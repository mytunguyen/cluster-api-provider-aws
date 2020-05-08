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

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	infrav1 "sigs.k8s.io/cluster-api-provider-aws/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-aws/pkg/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-aws/pkg/cloud/services/ec2/network"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// AWSManagedClusterReconciler reconciles a AWSManagedCluster object
type AWSManagedClusterReconciler struct {
	client.Client
	Recorder record.EventRecorder
	Log      logr.Logger
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=awsmanagedclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=awsmanagedclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch

// Reconcile is used to reconcile the state of teh AWSManagedCluster
func (r *AWSManagedClusterReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, reterr error) {
	ctx := context.TODO()
	log := r.Log.WithValues("namespace", req.Namespace, "awsManagedCluster", req.Name)

	// Fetch the AWSManagedCluster instance
	awsManagedCluster := &infrav1.AWSManagedCluster{}
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

	// Create the scope.
	clusterScope, err := scope.NewManagedClusterScope(scope.ManagedClusterScopeParams{
		Client:            r.Client,
		Logger:            log,
		Cluster:           cluster,
		AWSManagedCluster: awsManagedCluster,
	})
	if err != nil {
		return reconcile.Result{}, errors.Errorf("failed to create scope: %+v", err)
	}

	// Always close the scope when exiting this function so we can persist any AWSCluster changes.
	defer func() {
		if err := clusterScope.Close(); err != nil && reterr == nil {
			reterr = err
		}
	}()

	// Handle deleted clusters
	if !awsManagedCluster.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(clusterScope)
	}

	// Handle non-deleted clusters
	return r.reconcileNormal(clusterScope)
}

func (r *AWSManagedClusterReconciler) reconcileDelete(clusterScope *scope.ManagedClusterScope) (reconcile.Result, error) {
	clusterScope.Info("Reconciling AWSManagedCluster delete")

	//ec2svc := ec2.NewService(clusterScope)
	//elbsvc := elb.NewService(clusterScope)
	awsManagedCluster := clusterScope.AWSManagedCluster
	networkSvc := network.NewService(clusterScope.NetworkScope)

	// if err := elbsvc.DeleteLoadbalancers(); err != nil {
	// 	return reconcile.Result{}, errors.Wrapf(err, "error deleting load balancer for AWSCluster %s/%s", awsCluster.Namespace, awsCluster.Name)
	// }

	// if err := ec2svc.DeleteBastion(); err != nil {
	// 	return reconcile.Result{}, errors.Wrapf(err, "error deleting bastion for AWSCluster %s/%s", awsCluster.Namespace, awsCluster.Name)
	// }

	if err := networkSvc.DeleteNetwork(); err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "error deleting network for AWSManagedCluster %s/%s", awsManagedCluster.Namespace, awsManagedCluster.Name)
	}

	// Cluster is deleted so remove the finalizer.
	controllerutil.RemoveFinalizer(clusterScope.AWSManagedCluster, infrav1.ManagedClusterFinalizer)

	return reconcile.Result{}, nil
}

func (r *AWSManagedClusterReconciler) reconcileNormal(clusterScope *scope.ManagedClusterScope) (reconcile.Result, error) {
	clusterScope.Info("Reconciling AWSManagedCluster")

	awsManagedCluster := clusterScope.AWSManagedCluster

	// If the AWSManagedCluster doesn't have our finalizer, add it.
	controllerutil.AddFinalizer(awsManagedCluster, infrav1.ManagedClusterFinalizer)
	// Register the finalizer immediately to avoid orphaning AWS resources on delete
	if err := clusterScope.PatchObject(); err != nil {
		return reconcile.Result{}, err
	}

	//ec2Service := ec2.NewService(clusterScope)
	//elbService := elb.NewService(clusterScope)
	networkSvc := network.NewService(clusterScope.NetworkScope)

	if err := networkSvc.ReconcileManagedNetwork(); err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to reconcile network for AWSManagedCluster %s/%s", awsManagedCluster.Namespace, awsManagedCluster.Name)
	}

	// if err := ec2Service.ReconcileBastion(); err != nil {
	// 	return reconcile.Result{}, errors.Wrapf(err, "failed to reconcile bastion host for AWSCluster %s/%s", awsCluster.Namespace, awsCluster.Name)
	// }

	// if err := elbService.ReconcileLoadbalancers(); err != nil {
	// 	return reconcile.Result{}, errors.Wrapf(err, "failed to reconcile load balancers for AWSCluster %s/%s", awsCluster.Namespace, awsCluster.Name)
	// }

	//if awsCluster.Status.Network.APIServerELB.DNSName == "" {
	//	clusterScope.Info("Waiting on API server ELB DNS name")
	//	return reconcile.Result{RequeueAfter: 15 * time.Second}, nil
	//}

	//if _, err := net.LookupIP(awsCluster.Status.Network.APIServerELB.DNSName); err != nil {
	//	clusterScope.Info("Waiting on API server ELB DNS name to resolve")
	//	return reconcile.Result{RequeueAfter: 15 * time.Second}, nil
	//}

	//awsManagedCluster.Spec.ControlPlaneEndpoint = clusterv1.APIEndpoint{
	//	Host: awsCluster.Status.Network.APIServerELB.DNSName,
	//	Port: clusterScope.APIServerPort(),
	//}

	for _, subnet := range clusterScope.Subnets().FilterPrivate() {
		found := false
		for _, az := range awsManagedCluster.Status.Network.APIServerELB.AvailabilityZones {
			if az == subnet.AvailabilityZone {
				found = true
				break
			}
		}

		clusterScope.SetFailureDomain(subnet.AvailabilityZone, clusterv1.FailureDomainSpec{
			ControlPlane: found,
		})
	}

	awsManagedCluster.Status.Ready = true
	return reconcile.Result{}, nil
}

// SetupWithManager will setup the reconciler for AWSManagedCluster
func (r *AWSManagedClusterReconciler) SetupWithManager(mgr ctrl.Manager, options controller.Options) error {
	controller, err := ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&infrav1.AWSManagedCluster{}).
		WithEventFilter(pausedPredicates(r.Log)).
		Build(r)

	if err != nil {
		return errors.Wrap(err, "error creating controller")
	}

	return controller.Watch(
		&source.Kind{Type: &clusterv1.Cluster{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: handler.ToRequestsFunc(r.requeueAWSManagedClusterForUnpausedCluster),
		},
		predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool {
				oldCluster := e.ObjectOld.(*clusterv1.Cluster)
				newCluster := e.ObjectNew.(*clusterv1.Cluster)
				log := r.Log.WithValues("predicate", "updateEvent", "namespace", newCluster.Namespace, "cluster", newCluster.Name)
				switch {
				// return true if Cluster.Spec.Paused has changed from true to false
				case oldCluster.Spec.Paused && !newCluster.Spec.Paused:
					log.V(4).Info("Cluster was unpaused, will attempt to map associated AWSCluster.")
					return true
				// otherwise, return false
				default:
					log.V(4).Info("Cluster did not match expected conditions, will not attempt to map associated AWSCluster.")
					return false
				}
			},
			CreateFunc: func(e event.CreateEvent) bool {
				cluster := e.Object.(*clusterv1.Cluster)
				log := r.Log.WithValues("predicate", "createEvent", "namespace", cluster.Namespace, "cluster", cluster.Name)

				// Only need to trigger a reconcile if the Cluster.Spec.Paused is false
				if !cluster.Spec.Paused {
					log.V(4).Info("Cluster is not paused, will attempt to map associated AWSCluster.")
					return true
				}
				log.V(4).Info("Cluster did not match expected conditions, will not attempt to map associated AWSCluster.")
				return false
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				log := r.Log.WithValues("predicate", "deleteEvent", "namespace", e.Meta.GetNamespace(), "cluster", e.Meta.GetName())
				log.V(4).Info("Cluster did not match expected conditions, will not attempt to map associated AWSCluster.")
				return false
			},
			GenericFunc: func(e event.GenericEvent) bool {
				log := r.Log.WithValues("predicate", "genericEvent", "namespace", e.Meta.GetNamespace(), "cluster", e.Meta.GetName())
				log.V(4).Info("Cluster did not match expected conditions, will not attempt to map associated AWSCluster.")
				return false
			},
		},
	)
}

func (r *AWSManagedClusterReconciler) requeueAWSManagedClusterForUnpausedCluster(o handler.MapObject) []ctrl.Request {
	c := o.Object.(*clusterv1.Cluster)
	log := r.Log.WithValues("objectMapper", "clusterToAWSManagedCluster", "namespace", c.Namespace, "cluster", c.Name)

	// Don't handle deleted clusters
	if !c.ObjectMeta.DeletionTimestamp.IsZero() {
		log.V(4).Info("Cluster has a deletion timestamp, skipping mapping.")
		return nil
	}

	// Make sure the ref is set
	if c.Spec.InfrastructureRef == nil {
		log.V(4).Info("Cluster does not have an InfrastructureRef, skipping mapping.")
		return nil
	}

	if c.Spec.InfrastructureRef.GroupVersionKind().Kind != "AWSManagedCluster" {
		log.V(4).Info("Cluster has an InfrastructureRef for a different type, skipping mapping.")
		return nil
	}

	log.V(4).Info("Adding request.", "awsCluster", c.Spec.InfrastructureRef.Name)
	return []ctrl.Request{
		{
			NamespacedName: client.ObjectKey{Namespace: c.Namespace, Name: c.Spec.InfrastructureRef.Name},
		},
	}
}
