/*
Copyright 2020 The Kubernetes Authors.

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
	"fmt"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	controlplanev1 "sigs.k8s.io/cluster-api-provider-aws/controlplane/eks/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-aws/pkg/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-aws/pkg/cloud/services/eks"
)

// +kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;patch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;patch
// +kubebuilder:rbac:groups=core,resources=configmaps,namespace=kube-system,verbs=get;list;watch;create
// +kubebuilder:rbac:groups=rbac,resources=roles,namespace=kube-system,verbs=get;list;watch;create
// +kubebuilder:rbac:groups=rbac,resources=rolebindings,namespace=kube-system,verbs=get;list;watch;create
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io;bootstrap.cluster.x-k8s.io;controlplane.cluster.x-k8s.io,resources=*,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines;machines/status,verbs=get;list;watch;create;update;patch;delete

// EKSControlPlaneReconciler will reconcile a EKSControlPlane object
type EKSControlPlaneReconciler struct {
	client.Client
	Log      logr.Logger
	Recorder record.EventRecorder

	scheme     *runtime.Scheme
	controller controller.Controller
}

func (r *EKSControlPlaneReconciler) SetupWithManager(mgr ctrl.Manager, options controller.Options) error {
	c, err := ctrl.NewControllerManagedBy(mgr).
		For(&controlplanev1.EKSControlPlane{}).
		Watches(
			&source.Kind{Type: &clusterv1.Cluster{}},
			&handler.EnqueueRequestsFromMapFunc{ToRequests: handler.ToRequestsFunc(r.ClusterToEKSControlPlane)},
		).
		WithOptions(options).
		Build(r)

	if err != nil {
		return errors.Wrap(err, "failed setting up with a controller manager")
	}

	r.scheme = mgr.GetScheme()
	r.controller = c

	//if r.managementCluster == nil {
	//	r.managementCluster = &internal.Management{Client: r.Client}
	//}

	return nil
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=ekscontrolplane,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch

func (r *EKSControlPlaneReconciler) Reconcile(req ctrl.Request) (res ctrl.Result, reterr error) {
	logger := r.Log.WithValues("namespace", req.Namespace, "eksControlPlane", req.Name)
	ctx := context.Background()

	// Get the control plane instance
	ecp := &controlplanev1.EKSControlPlane{}
	if err := r.Client.Get(ctx, req.NamespacedName, ecp); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Get the cluster
	cluster, err := util.GetOwnerCluster(ctx, r.Client, ecp.ObjectMeta)
	if err != nil {
		logger.Error(err, "Failed to retrieve owner Cluster from the API Server")
		return ctrl.Result{}, err
	}
	if cluster == nil {
		logger.Info("Cluster Controller has not yet set OwnerRef")
		return ctrl.Result{}, nil
	}

	if util.IsPaused(cluster, ecp) {
		logger.Info("Reconciliation is paused for this object")
		return ctrl.Result{}, nil
	}

	logger = logger.WithValues("cluster", cluster.Name)

	managedScope, err := scope.NewManagedControlPlaneScope(scope.ManagedControlPlaneScopeParams{
		Client:          r.Client,
		Logger:          logger,
		Cluster:         cluster,
		EKSControlPlane: ecp,
	})
	if err != nil {
		return reconcile.Result{}, errors.Errorf("failed to create scope: %+v", err)
	}

	// Always close the scope
	defer func() {
		if err := managedScope.Close(); err != nil && reterr == nil {
			reterr = err
		}
	}()

	if !ecp.ObjectMeta.DeletionTimestamp.IsZero() {
		// Handle deletion reconciliation loop.
		return r.reconcileDelete(ctx, managedScope)
	}

	// Handle normal reconciliation loop.
	return r.reconcileNormal(ctx, cluster, ecp)
}

func (r *EKSControlPlaneReconciler) reconcileNormal(ctx context.Context, cluster *clusterv1.Cluster, ekp *controlplanev1.EKSControlPlane) (res ctrl.Result, reterr error) {
	return
}

func (r *EKSControlPlaneReconciler) reconcileDelete(ctx context.Context, managedScope *scope.ManagedControlPlaneScope) (_ ctrl.Result, reterr error) {
	managedScope.Info("Reconciling EKSClusterPlane delete")

	ekssvc := eks.NewService(managedScope)
	controlPlane := managedScope.EKSControlPlane

	if err := ekssvc.DeleteCluster(); err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "error deleteing EKS cluster for EKS control plane %s/%s", managedScope.Namespace, managedScope.Name)
	}

	controllerutil.RemoveFinalizer(controlPlane, controlplanev1.EKSControlPlaneFinalizer)

	return reconcile.Result{}, nil

	return
}

func (r *EKSControlPlaneReconciler) reconcilHealth(ctx context.Context, cluster *clusterv1.Cluster, ekp *controlplanev1.EKSControlPlane) (_ ctrl.Result, reterr error) {
	return
}

// ClusterToEKSControlPlane is a handler.ToRequestsFunc to be used to enqueue requests for reconciliation
// for EKSControlPlane based on updates to a Cluster.
func (r *EKSControlPlaneReconciler) ClusterToEKSControlPlane(o handler.MapObject) []ctrl.Request {
	c, ok := o.Object.(*clusterv1.Cluster)
	if !ok {
		r.Log.Error(nil, fmt.Sprintf("Expected a Cluster but got a %T", o.Object))
		return nil
	}

	controlPlaneRef := c.Spec.ControlPlaneRef
	if controlPlaneRef != nil && controlPlaneRef.Kind == "EKSControlPlane" {
		return []ctrl.Request{{NamespacedName: client.ObjectKey{Namespace: controlPlaneRef.Namespace, Name: controlPlaneRef.Name}}}
	}

	return nil
}
