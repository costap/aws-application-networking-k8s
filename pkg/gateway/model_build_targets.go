package gateway

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	mcsv1alpha1 "sigs.k8s.io/mcs-api/pkg/apis/v1alpha1"

	pkg_aws "github.com/aws/aws-application-networking-k8s/pkg/aws"
	"github.com/aws/aws-application-networking-k8s/pkg/k8s"
	"github.com/aws/aws-application-networking-k8s/pkg/latticestore"
	"github.com/aws/aws-application-networking-k8s/pkg/model/core"
	model "github.com/aws/aws-application-networking-k8s/pkg/model/lattice"
	"github.com/aws/aws-application-networking-k8s/pkg/utils/gwlog"
)

const (
	portAnnotationsKey = "multicluster.x-k8s.io/port"
	undefinedPort      = int32(0)
)

type LatticeTargetsBuilder interface {
	Build(ctx context.Context, service *corev1.Service, routeName string) (core.Stack, *model.Targets, error)
}

type LatticeTargetsModelBuilder struct {
	log         gwlog.Logger
	client      client.Client
	defaultTags map[string]string
	datastore   *latticestore.LatticeDataStore
	cloud       pkg_aws.Cloud
}

func NewTargetsBuilder(
	log gwlog.Logger,
	client client.Client,
	cloud pkg_aws.Cloud,
	datastore *latticestore.LatticeDataStore,
) *LatticeTargetsModelBuilder {
	return &LatticeTargetsModelBuilder{
		log:       log,
		client:    client,
		cloud:     cloud,
		datastore: datastore,
	}
}

func (b *LatticeTargetsModelBuilder) Build(ctx context.Context, service *corev1.Service, routeName string) (core.Stack, *model.Targets, error) {
	stack := core.NewDefaultStack(core.StackID(k8s.NamespacedName(service)))

	task := &latticeTargetsModelBuildTask{
		log:         b.log,
		client:      b.client,
		tgName:      service.Name,
		tgNamespace: service.Namespace,
		routeName:   routeName,
		stack:       stack,
		datastore:   b.datastore,
	}

	if err := task.run(ctx); err != nil {
		return nil, nil, corev1.ErrIntOverflowGenerated
	}

	return task.stack, task.latticeTargets, nil
}

func (t *latticeTargetsModelBuildTask) run(ctx context.Context) error {
	return t.buildLatticeTargets(ctx)
}

func (t *latticeTargetsModelBuildTask) buildLatticeTargets(ctx context.Context) error {
	ds := t.datastore
	tgName := latticestore.TargetGroupName(t.tgName, t.tgNamespace)
	tg, err := ds.GetTargetGroup(tgName, t.routeName, false) // isServiceImport= false

	if err != nil {
		errmsg := fmt.Sprintf("Build Targets failed because target group (name=%s, namespace=%s found not in datastore)", t.tgName, t.tgNamespace)
		return errors.New(errmsg)
	}

	if !tg.ByBackendRef && !tg.ByServiceExport {
		errmsg := fmt.Sprintf("Build Targets failed because its target Group name=%s, namespace=%s is no longer referenced", t.tgName, t.tgNamespace)
		return errors.New(errmsg)
	}

	svc := &corev1.Service{}
	namespacedName := types.NamespacedName{
		Namespace: t.tgNamespace,
		Name:      t.tgName,
	}

	if err := t.client.Get(ctx, namespacedName, svc); err != nil {
		return fmt.Errorf("Build Targets failed because K8S service %s does not exist", namespacedName)
	}

	definedPorts := make(map[int32]struct{})

	if tg.ByServiceExport {
		serviceExport := &mcsv1alpha1.ServiceExport{}
		err = t.client.Get(ctx, namespacedName, serviceExport)
		if err != nil {
			t.log.Errorf("Failed to find service export %s-%s in datastore due to %s", t.tgName, t.tgNamespace, err)
		} else {
			portsAnnotations := strings.Split(serviceExport.ObjectMeta.Annotations[portAnnotationsKey], ",")

			for _, portAnnotation := range portsAnnotations {
				definedPort, err := strconv.ParseInt(portAnnotation, 10, 32)
				if err != nil {
					t.log.Errorf("Failed to read Annotations/Port: %s due to %s",
						serviceExport.ObjectMeta.Annotations[portAnnotationsKey], err)
				} else {
					definedPorts[int32(definedPort)] = struct{}{}
				}
			}
		}
	} else if tg.ByBackendRef && t.backendRefPort != undefinedPort {
		definedPorts[t.backendRefPort] = struct{}{}
	}

	// A service port MUST have a name if there are multiple ports exposed from a service.
	// Therefore, if a port is named, endpoint port is only relevant if it has the same name.
	//
	// If a service port is unnamed, it MUST be the only port that is exposed from a service.
	// In this case, as long as the service port is matching with backendRef/annotations,
	// we can consider all endpoints valid.

	servicePortNames := make(map[string]struct{})
	skipMatch := false

	for _, port := range svc.Spec.Ports {
		if _, ok := definedPorts[port.Port]; ok {
			if port.Name != "" {
				servicePortNames[port.Name] = struct{}{}
			} else {
				// Unnamed, consider all endpoints valid
				skipMatch = true
			}
		}
	}

	// Having no backendRef port makes all endpoints valid - this is mainly for backwards compatibility.
	if len(definedPorts) == 0 {
		skipMatch = true
	}

	var targetList []model.Target
	endpoints := &corev1.Endpoints{}

	if svc.DeletionTimestamp.IsZero() {
		if err := t.client.Get(ctx, namespacedName, endpoints); err != nil {
			return fmt.Errorf("build targets failed because K8S service %s does not exist", namespacedName)
		}

		for _, endPoint := range endpoints.Subsets {
			for _, address := range endPoint.Addresses {
				for _, port := range endPoint.Ports {
					target := model.Target{
						TargetIP: address.IP,
						Port:     int64(port.Port),
					}
					// Note that the Endpoint's port name is from ServicePort, but the actual registered port
					// is from Pods(targets).
					if _, ok := servicePortNames[port.Name]; ok || skipMatch {
						targetList = append(targetList, target)
					}
				}
			}
		}
	}

	spec := model.TargetsSpec{
		Name:         t.tgName,
		Namespace:    t.tgNamespace,
		RouteName:    t.routeName,
		TargetIPList: targetList,
	}

	t.latticeTargets = model.NewTargets(t.stack, tgName, spec)

	return nil
}

type latticeTargetsModelBuildTask struct {
	log            gwlog.Logger
	client         client.Client
	tgName         string
	tgNamespace    string
	routeName      string
	backendRefPort int32
	latticeTargets *model.Targets
	stack          core.Stack
	datastore      *latticestore.LatticeDataStore
	route          core.Route
}
