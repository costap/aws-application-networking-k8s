/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/source"
	mcsv1alpha1 "sigs.k8s.io/mcs-api/pkg/apis/v1alpha1"

	"github.com/aws/aws-application-networking-k8s/controllers/eventhandlers"

	anv1alpha1 "github.com/aws/aws-application-networking-k8s/pkg/apis/applicationnetworking/v1alpha1"
	"github.com/aws/aws-application-networking-k8s/pkg/aws"
	"github.com/aws/aws-application-networking-k8s/pkg/deploy"
	"github.com/aws/aws-application-networking-k8s/pkg/gateway"
	"github.com/aws/aws-application-networking-k8s/pkg/k8s"
	"github.com/aws/aws-application-networking-k8s/pkg/latticestore"
	lattice_runtime "github.com/aws/aws-application-networking-k8s/pkg/runtime"
	"github.com/aws/aws-application-networking-k8s/pkg/utils/gwlog"
)

type serviceExportReconciler struct {
	log              gwlog.Logger
	client           client.Client
	Scheme           *runtime.Scheme
	finalizerManager k8s.FinalizerManager
	eventRecorder    record.EventRecorder
	modelBuilder     gateway.SvcExportTargetGroupModelBuilder
	stackDeployer    deploy.StackDeployer
	latticeDataStore *latticestore.LatticeDataStore
	stackMarshaller  deploy.StackMarshaller
}

const (
	serviceExportFinalizer = "serviceexport.k8s.aws/resources"
)

func RegisterServiceExportController(
	log gwlog.Logger,
	cloud aws.Cloud,
	latticeDataStore *latticestore.LatticeDataStore,
	finalizerManager k8s.FinalizerManager,
	mgr ctrl.Manager,
) error {
	mgrClient := mgr.GetClient()
	scheme := mgr.GetScheme()
	eventRecorder := mgr.GetEventRecorderFor("serviceExport")

	modelBuilder := gateway.NewSvcExportTargetGroupBuilder(log, mgrClient, latticeDataStore, cloud)
	stackDeploy := deploy.NewTargetGroupStackDeploy(log, cloud, mgrClient, latticeDataStore)
	stackMarshaller := deploy.NewDefaultStackMarshaller()

	r := &serviceExportReconciler{
		log:              log,
		client:           mgrClient,
		Scheme:           scheme,
		finalizerManager: finalizerManager,
		modelBuilder:     modelBuilder,
		stackDeployer:    stackDeploy,
		eventRecorder:    eventRecorder,
		latticeDataStore: latticeDataStore,
		stackMarshaller:  stackMarshaller,
	}

	svcEventHandler := eventhandlers.NewServiceEventHandler(log, r.client)

	builder := ctrl.NewControllerManagedBy(mgr).
		For(&mcsv1alpha1.ServiceExport{}).
		Watches(&source.Kind{Type: &corev1.Service{}}, svcEventHandler.MapToServiceExport())

	if ok, err := k8s.IsGVKSupported(mgr, anv1alpha1.GroupVersion.String(), anv1alpha1.TargetGroupPolicyKind); ok {
		builder.Watches(&source.Kind{Type: &anv1alpha1.TargetGroupPolicy{}}, svcEventHandler.MapToServiceExport())
	} else {
		if err != nil {
			return err
		}
		log.Infof("TargetGroupPolicy CRD is not installed, skipping watch")
	}

	return builder.Complete(r)
}

//+kubebuilder:rbac:groups=multicluster.x-k8s.io,resources=serviceexports,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=multicluster.x-k8s.io,resources=serviceexports/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=multicluster.x-k8s.io,resources=serviceexports/finalizers,verbs=update

func (r *serviceExportReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return lattice_runtime.HandleReconcileError(r.reconcile(ctx, req))
}

func (r *serviceExportReconciler) reconcile(ctx context.Context, req ctrl.Request) error {
	srvExport := &mcsv1alpha1.ServiceExport{}

	if err := r.client.Get(ctx, req.NamespacedName, srvExport); err != nil {
		return client.IgnoreNotFound(err)
	}

	if srvExport.ObjectMeta.Annotations["multicluster.x-k8s.io/federation"] != "amazon-vpc-lattice" {
		return nil
	}
	r.log.Debugf("Found matching service export %s-%s", srvExport.Name, srvExport.Namespace)

	if !srvExport.DeletionTimestamp.IsZero() {
		if err := r.buildAndDeployModel(ctx, srvExport); err != nil {
			return err
		}
		err := r.finalizerManager.RemoveFinalizers(ctx, srvExport, serviceExportFinalizer)
		if err != nil {
			r.log.Errorf("Failed to remove finalizers for service export %s-%s due to %s",
				srvExport.Name, srvExport.Namespace, err)
		}
		return nil
	} else {
		if err := r.finalizerManager.AddFinalizers(ctx, srvExport, serviceExportFinalizer); err != nil {
			r.eventRecorder.Event(srvExport, corev1.EventTypeWarning, k8s.GatewayEventReasonFailedAddFinalizer, fmt.Sprintf("Failed add finalizer due to %v", err))
			return errors.New("TODO")
		}

		err := r.buildAndDeployModel(ctx, srvExport)
		return err
	}
}

func (r *serviceExportReconciler) buildAndDeployModel(
	ctx context.Context,
	srvExport *mcsv1alpha1.ServiceExport,
) error {
	stack, _, err := r.modelBuilder.Build(ctx, srvExport)

	if err != nil {
		r.log.Debugf("Failed to buildAndDeployModel for service export %s-%s due to %s",
			srvExport.Name, srvExport.Namespace, err)

		r.eventRecorder.Event(srvExport, corev1.EventTypeWarning,
			k8s.GatewayEventReasonFailedBuildModel,
			fmt.Sprintf("Failed BuildModel due to %s", err))

		// Build failed means the K8S serviceexport, service are NOT ready to be deployed to lattice
		// return nil  to complete controller loop for current change.
		// TODO continue deploy to trigger reconcile of stale SDK objects
		//return stack, targetGroup, nil
	}

	_, err = r.stackMarshaller.Marshal(stack)
	if err != nil {
		r.log.Errorf("Error on marshalling model for service export %s-%s", srvExport.Name, srvExport.Namespace)
	}

	if err := r.stackDeployer.Deploy(ctx, stack); err != nil {
		r.eventRecorder.Event(srvExport, corev1.EventTypeWarning,
			k8s.ServiceExportEventReasonFailedDeployModel, fmt.Sprintf("Failed deploy model due to %s", err))
		return err
	}

	r.log.Debugf("Successfully deployed model for service export %s-%s", srvExport.Name, srvExport.Namespace)
	return err
}
