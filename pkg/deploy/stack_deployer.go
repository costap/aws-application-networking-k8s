package deploy

import (
	"context"

	"github.com/aws/aws-application-networking-k8s/pkg/utils/gwlog"

	"sigs.k8s.io/controller-runtime/pkg/client"

	pkg_aws "github.com/aws/aws-application-networking-k8s/pkg/aws"
	"github.com/aws/aws-application-networking-k8s/pkg/deploy/externaldns"
	"github.com/aws/aws-application-networking-k8s/pkg/deploy/lattice"
	"github.com/aws/aws-application-networking-k8s/pkg/latticestore"
	"github.com/aws/aws-application-networking-k8s/pkg/model/core"
	model "github.com/aws/aws-application-networking-k8s/pkg/model/lattice"
)

// StackDeployer will deploy a resource stack into AWS and K8S.
type StackDeployer interface {
	// Deploy a resource stack.
	Deploy(ctx context.Context, stack core.Stack) error
}

//var _ StackDeployer = &defaultStackDeployer{}

// TODO,  later might have a single stack, righ now will have
// dedicated stack for serviceNetwork/service/targetgroup
type serviceNetworkStackDeployer struct {
	log                          gwlog.Logger
	cloud                        pkg_aws.Cloud
	k8sClient                    client.Client
	latticeServiceNetworkManager lattice.ServiceNetworkManager
}

type ResourceSynthesizer interface {
	Synthesize(ctx context.Context) error
	PostSynthesize(ctx context.Context) error
}

func NewServiceNetworkStackDeployer(log gwlog.Logger, cloud pkg_aws.Cloud, k8sClient client.Client) *serviceNetworkStackDeployer {
	return &serviceNetworkStackDeployer{
		log:                          log,
		cloud:                        cloud,
		k8sClient:                    k8sClient,
		latticeServiceNetworkManager: lattice.NewDefaultServiceNetworkManager(log, cloud),
	}
}

// Deploy a resource stack

func deploy(ctx context.Context, stack core.Stack, synthesizers []ResourceSynthesizer) error {
	for _, synthesizer := range synthesizers {
		if err := synthesizer.Synthesize(ctx); err != nil {
			return err
		}
	}
	for i := len(synthesizers) - 1; i >= 0; i-- {
		if err := synthesizers[i].PostSynthesize(ctx); err != nil {
			return err
		}
	}

	return nil
}

func (d *serviceNetworkStackDeployer) Deploy(ctx context.Context, stack core.Stack) error {
	synthesizers := []ResourceSynthesizer{
		lattice.NewServiceNetworkSynthesizer(d.log, d.k8sClient, d.latticeServiceNetworkManager, stack),
	}
	return deploy(ctx, stack, synthesizers)
}

type latticeServiceStackDeployer struct {
	log                   gwlog.Logger
	cloud                 pkg_aws.Cloud
	k8sClient             client.Client
	latticeServiceManager lattice.ServiceManager
	targetGroupManager    lattice.TargetGroupManager
	targetsManager        lattice.TargetsManager
	listenerManager       lattice.ListenerManager
	ruleManager           lattice.RuleManager
	dnsEndpointManager    externaldns.DnsEndpointManager
	latticeDataStore      *latticestore.LatticeDataStore
}

func NewLatticeServiceStackDeploy(
	log gwlog.Logger,
	cloud pkg_aws.Cloud,
	k8sClient client.Client,
	latticeDataStore *latticestore.LatticeDataStore,
) *latticeServiceStackDeployer {
	return &latticeServiceStackDeployer{
		log:                   log,
		cloud:                 cloud,
		k8sClient:             k8sClient,
		latticeServiceManager: lattice.NewServiceManager(cloud, latticeDataStore),
		targetGroupManager:    lattice.NewTargetGroupManager(log, cloud),
		targetsManager:        lattice.NewTargetsManager(log, cloud, latticeDataStore),
		listenerManager:       lattice.NewListenerManager(log, cloud, latticeDataStore),
		ruleManager:           lattice.NewRuleManager(log, cloud, latticeDataStore),
		dnsEndpointManager:    externaldns.NewDnsEndpointManager(log, k8sClient),
		latticeDataStore:      latticeDataStore,
	}
}

func (d *latticeServiceStackDeployer) Deploy(ctx context.Context, stack core.Stack) error {
	targetGroupSynthesizer := lattice.NewTargetGroupSynthesizer(d.log, d.cloud, d.k8sClient, d.targetGroupManager, stack, d.latticeDataStore)
	targetsSynthesizer := lattice.NewTargetsSynthesizer(d.log, d.cloud, d.targetsManager, stack, d.latticeDataStore)
	serviceSynthesizer := lattice.NewServiceSynthesizer(d.log, d.latticeServiceManager, d.dnsEndpointManager, stack, d.latticeDataStore)
	listenerSynthesizer := lattice.NewListenerSynthesizer(d.log, d.listenerManager, stack, d.latticeDataStore)
	ruleSynthesizer := lattice.NewRuleSynthesizer(d.log, d.ruleManager, stack, d.latticeDataStore)

	//Handle targetGroups creation request
	if err := targetGroupSynthesizer.SynthesizeTriggeredTargetGroupsCreation(ctx); err != nil {
		return err
	}

	//Handle targets "reconciliation" request (register intend-to-be-registered targets and deregister intend-to-be-registered targets)
	if err := targetsSynthesizer.Synthesize(ctx); err != nil {
		return err
	}

	// Handle latticeService "reconciliation" request
	if err := serviceSynthesizer.Synthesize(ctx); err != nil {
		return err
	}

	//Handle latticeService listeners "reconciliation" request
	if err := listenerSynthesizer.Synthesize(ctx); err != nil {
		return err
	}

	//Handle latticeService listener's rules "reconciliation" request
	if err := ruleSynthesizer.Synthesize(ctx); err != nil {
		return err
	}

	//Handle targetGroup deletion request
	if err := targetGroupSynthesizer.SynthesizeTriggeredTargetGroupsDeletion(ctx); err != nil {
		return err
	}

	// Do garbage collection for not-in-use targetGroups
	//TODO: run SynthesizeSDKTargetGroups(ctx) as a global garbage collector scheduled backgroud task (i.e., run it as a goroutine in main.go)
	if err := targetGroupSynthesizer.SynthesizeSDKTargetGroups(ctx); err != nil {
		return err
	}

	return nil
}

type latticeTargetGroupStackDeployer struct {
	log                gwlog.Logger
	cloud              pkg_aws.Cloud
	k8sclient          client.Client
	targetGroupManager lattice.TargetGroupManager
	latticeDatastore   *latticestore.LatticeDataStore
}

// triggered by service export
func NewTargetGroupStackDeploy(
	log gwlog.Logger,
	cloud pkg_aws.Cloud,
	k8sClient client.Client,
	latticeDataStore *latticestore.LatticeDataStore,
) *latticeTargetGroupStackDeployer {
	return &latticeTargetGroupStackDeployer{
		log:                log,
		cloud:              cloud,
		k8sclient:          k8sClient,
		targetGroupManager: lattice.NewTargetGroupManager(log, cloud),
		latticeDatastore:   latticeDataStore,
	}
}

func (d *latticeTargetGroupStackDeployer) Deploy(ctx context.Context, stack core.Stack) error {
	synthesizers := []ResourceSynthesizer{
		lattice.NewTargetGroupSynthesizer(d.log, d.cloud, d.k8sclient, d.targetGroupManager, stack, d.latticeDatastore),
		lattice.NewTargetsSynthesizer(d.log, d.cloud, lattice.NewTargetsManager(d.log, d.cloud, d.latticeDatastore), stack, d.latticeDatastore),
	}
	return deploy(ctx, stack, synthesizers)
}

type latticeTargetsStackDeployer struct {
	log              gwlog.Logger
	k8sClient        client.Client
	stack            core.Stack
	targetsManager   lattice.TargetsManager
	latticeDataStore *latticestore.LatticeDataStore
}

func NewTargetsStackDeployer(
	log gwlog.Logger,
	cloud pkg_aws.Cloud,
	k8sClient client.Client,
	latticeDataStore *latticestore.LatticeDataStore,
) *latticeTargetsStackDeployer {
	return &latticeTargetsStackDeployer{
		k8sClient:        k8sClient,
		targetsManager:   lattice.NewTargetsManager(log, cloud, latticeDataStore),
		latticeDataStore: latticeDataStore,
	}
}

func (d *latticeTargetsStackDeployer) Deploy(ctx context.Context, stack core.Stack) error {
	var resTargets []*model.Targets

	d.stack = stack

	err := d.stack.ListResources(&resTargets)
	if err != nil {
		d.log.Errorf("Failed to list targets due to %s", err)
	}

	for _, targets := range resTargets {
		err := d.targetsManager.Create(ctx, targets)
		if err == nil {
			tgName := latticestore.TargetGroupName(targets.Spec.Name, targets.Spec.Namespace)

			var targetList []latticestore.Target
			for _, target := range targetList {
				t := latticestore.Target{
					TargetIP:   target.TargetIP,
					TargetPort: target.TargetPort,
				}
				targetList = append(targetList, t)
			}
			err := d.latticeDataStore.UpdateTargetsForTargetGroup(tgName, targets.Spec.RouteName, targetList)
			if err != nil {
				d.log.Errorf("Failed to update targets for target group %s due to %s", tgName, err)
			}
		}

	}
	return nil
}

type accessLogSubscriptionStackDeployer struct {
	log       gwlog.Logger
	k8sClient client.Client
	stack     core.Stack
	manager   lattice.AccessLogSubscriptionManager
}

func NewAccessLogSubscriptionStackDeployer(
	log gwlog.Logger,
	cloud pkg_aws.Cloud,
	k8sClient client.Client,
) *accessLogSubscriptionStackDeployer {
	return &accessLogSubscriptionStackDeployer{
		log:       log,
		k8sClient: k8sClient,
		manager:   lattice.NewAccessLogSubscriptionManager(log, cloud),
	}
}

func (d *accessLogSubscriptionStackDeployer) Deploy(ctx context.Context, stack core.Stack) error {
	synthesizers := []ResourceSynthesizer{
		lattice.NewAccessLogSubscriptionSynthesizer(d.log, d.k8sClient, d.manager, stack),
	}
	return deploy(ctx, stack, synthesizers)
}
