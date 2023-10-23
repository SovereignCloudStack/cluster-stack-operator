/*
Copyright 2023 The Kubernetes Authors.

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

package controller

import (
	"context"
	"fmt"

	csov1alpha1 "github.com/SovereignCloudStack/cluster-stack-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// ClusterAddonCreateReconciler reconciles a Cluster object.
type ClusterAddonCreateReconciler struct {
	client.Client
	WatchFilterValue string
}

//+kubebuilder:rbac:groups=clusterstack.x-k8s.io,resources=clusteraddons,verbs=create

// Reconcile is part of the main Kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ClusterAddonCreateReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	cluster := &clusterv1.Cluster{}
	if err := r.Get(ctx, req.NamespacedName, cluster); err != nil {
		// if the cluster is not found, exit the reconciliation
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("failed to get cluster: %w", err)
	}

	clusterAddonName := fmt.Sprintf("cluster-addon-%s", cluster.Name)

	var existingClusterAddon csov1alpha1.ClusterAddon
	err := r.Get(ctx, types.NamespacedName{Namespace: cluster.Namespace, Name: clusterAddonName}, &existingClusterAddon)
	// no error means that the object already exists - nothing to do
	if err == nil {
		return reconcile.Result{}, nil
	}
	// unexpected error - return it
	if !apierrors.IsNotFound(err) {
		return reconcile.Result{}, fmt.Errorf("failed to get ClusterAddon object: %w", err)
	}

	// clusterAddon could not be found - create it
	clusterAddon := &csov1alpha1.ClusterAddon{
		ObjectMeta: metav1.ObjectMeta{Name: clusterAddonName, Namespace: cluster.Namespace},
		TypeMeta:   metav1.TypeMeta{Kind: "ClusterAddon", APIVersion: "clusterstack.x-k8s.io/v1alpha1"},
		Spec: csov1alpha1.ClusterAddonSpec{
			ClusterRef: &corev1.ObjectReference{
				APIVersion: cluster.APIVersion,
				Kind:       cluster.Kind,
				Name:       cluster.Name,
				UID:        cluster.UID,
				Namespace:  cluster.Namespace,
			},
		},
	}

	clusterAddon.OwnerReferences = append(clusterAddon.OwnerReferences, metav1.OwnerReference{
		APIVersion: cluster.APIVersion,
		Kind:       cluster.Kind,
		Name:       cluster.Name,
		UID:        cluster.UID,
	})

	if err := r.Create(ctx, clusterAddon); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to create ClusterAddon object: %w", err)
	}
	return reconcile.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterAddonCreateReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&clusterv1.Cluster{}).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(log.FromContext(ctx), r.WatchFilterValue)).
		WithEventFilter(predicate.Funcs{
			// We're only interested in the create events for a cluster object
			DeleteFunc: func(e event.DeleteEvent) bool {
				return false
			},
			GenericFunc: func(e event.GenericEvent) bool {
				return false
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				return false
			},
		}).
		Complete(r)
}
