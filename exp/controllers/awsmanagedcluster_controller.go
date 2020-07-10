/*

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
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"

	"sigs.k8s.io/cluster-api/util"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"

	infrav1exp "sigs.k8s.io/cluster-api-provider-aws/exp/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-aws/pkg/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-aws/pkg/cloud/services/ec2"
)

// AWSManagedClusterReconciler reconciles AWSManagedCluster
type AWSManagedClusterReconciler struct {
	client.Client
	Log              logr.Logger
	Recorder         record.EventRecorder
	ReconcileTimeout time.Duration
}

func (r *AWSManagedClusterReconciler) SetupWithManager(mgr ctrl.Manager, options controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&infrav1exp.AWSManagedCluster{}).
		Complete(r)
	//TODO: need to change this to match the AWSClusterSetup
}

// +kubebuilder:rbac:groups=exp.infrastructure.cluster.x-k8s.io,resources=awsmanagedclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=exp.infrastructure.cluster.x-k8s.io,resources=awsmanagedclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete

func (r *AWSManagedClusterReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, reterr error) {
	ctx := context.Background()
	log := r.Log.WithValues("namespace", req.Namespace, "awsManagedCluster", req.Name)

	// Fetch the AWSManagedCluster instance
	awsManagedCluster := &infrav1exp.AWSManagedCluster{}
	err := r.Get(ctx, req.NamespacedName, awsManagedCluster)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Fetch the Cluster.
	cluster, err := util.GetOwnerCluster(ctx, r.Client, awsManagedCluster.ObjectMeta)
	if err != nil {
		return reconcile.Result{}, err
	}
	if cluster == nil {
		log.Info("Cluster Controller has not yet set OwnerRef")
		return reconcile.Result{}, nil
	}

	if util.IsPaused(cluster, awsManagedCluster) {
		log.Info("AWSManagedCluster or linked Cluster is marked as paused. Won't reconcile")
		return reconcile.Result{}, nil
	}

	log = log.WithValues("cluster", cluster.Name)

	// Fetch the control plane
	controlPlane := &infrav1exp.AWSManagedControlPlane{}
	controlPlaneRef := types.NamespacedName{
		Name:      cluster.Spec.ControlPlaneRef.Name,
		Namespace: cluster.Namespace,
	}

	if err := r.Get(ctx, controlPlaneRef, controlPlane); err != nil {
		return reconcile.Result{}, errors.Wrap(err, "failed to get control plane ref")
	}

	managedClusterScope, err := scope.NewManagedClusterScope(scope.ManagedClusterScopeParams{
		Client:            r.Client,
		Logger:            log,
		Cluster:           cluster,
		AWSManagedCluster: awsManagedCluster,
		Controlplane:      controlPlane,
	})
	if err != nil {
		return reconcile.Result{}, errors.Errorf("failed to create managed cluster scope: %+v", err)
	}

	defer func() {
		if err := managedClusterScope.Close(); err != nil && reterr == nil {
			reterr = err
		}
	}()

	// Handle deleted clusters
	if !awsManagedCluster.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(managedClusterScope)
	}

	// Handle non-deleted clusters
	return r.reconcileNormal(ctx, managedClusterScope)
}

func (r *AWSManagedClusterReconciler) reconcileNormal(ctx context.Context, managedClusterScope *scope.ManagedClusterScope) (reconcile.Result, error) {
	managedClusterScope.Info("Reconciling AWSManagedCluster")

	awsManagedCluster := managedClusterScope.InfraCluster().(*infrav1exp.AWSManagedCluster)

	// If the AWSManagedCluster doesn't have our finalizer, add it.
	controllerutil.AddFinalizer(awsManagedCluster, infrav1exp.ManagedClusterFinalizer)
	// Register the finalizer immediately to avoid orphaning AWS resources on delete
	if err := managedClusterScope.PatchObject(); err != nil {
		return reconcile.Result{}, err
	}

	ec2Service := ec2.NewService(managedClusterScope)

	if err := ec2Service.ReconcileNetwork(); err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to reconcile network for AWSManagedCluster %s/%s", awsManagedCluster.Namespace, awsManagedCluster.Name)
	}

	if err := ec2Service.ReconcileBastion(); err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to reconcile bastion host for AWSManagedCluster %s/%s", awsManagedCluster.Namespace, awsManagedCluster.Name)
	}

	for _, subnet := range managedClusterScope.Subnets().FilterPrivate() {
		managedClusterScope.SetFailureDomain(subnet.AvailabilityZone, clusterv1.FailureDomainSpec{
			ControlPlane: true,
		})
	}

	// We have initialized the infra
	awsManagedCluster.Status.Initialized = true

	// Check the control plane and see if we are ready
	controlPlane := managedClusterScope.ControlPlane()
	if controlPlane.Status.Ready {
		awsManagedCluster.Spec.ControlPlaneEndpoint = awsManagedCluster.Spec.ControlPlaneEndpoint.DeepCopy()
		if awsManagedCluster.Spec.ControlPlaneEndpoint != nil {
			awsManagedCluster.Status.Ready = true
		}
	}

	return reconcile.Result{}, nil
}

func (r *AWSManagedClusterReconciler) reconcileDelete(managedClusterScope *scope.ManagedClusterScope) (reconcile.Result, error) {
	managedClusterScope.Info("Reconciling AWSManagedCluster delete")

	ec2svc := ec2.NewService(managedClusterScope)
	awsManagedCluster := managedClusterScope.InfraCluster().(*infrav1exp.AWSManagedCluster)

	//TODO: check that the Controlplane is deleted first

	if err := ec2svc.DeleteBastion(); err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "error deleting bastion for AWSManagedCluster %s/%s", awsManagedCluster.Namespace, awsManagedCluster.Name)
	}

	if err := ec2svc.DeleteNetwork(); err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "error deleting network for AWSManagedCluster %s/%s", awsManagedCluster.Namespace, awsManagedCluster.Name)
	}

	// Cluster is deleted so remove the finalizer.
	controllerutil.RemoveFinalizer(awsManagedCluster, infrav1exp.ManagedClusterFinalizer)

	return reconcile.Result{}, nil
}
