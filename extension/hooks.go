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

// Package extension defines a hook server for multi-stage cluster addons.
package extension

import (
	"context"
	"fmt"

	csov1alpha1 "github.com/SovereignCloudStack/cluster-stack-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	"sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions"
	"sigs.k8s.io/cluster-api/util/deprecated/v1beta1/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// BeforeClusterUpgradeHook is the hook for before cluster upgrade.
	BeforeClusterUpgradeHook = "BeforeClusterUpgrade"
)

// Handler is the handler object with client.Client for the hook server.
type Handler struct {
	client.Client
	// APIReader reads directly from the API server, not from the cache.
	APIReader  client.Reader
	HookRecord map[string]string
}

// NewHandler gives an extension handler.
func NewHandler(c client.Client, apiReader client.Reader) *Handler {
	return &Handler{
		Client:     c,
		APIReader:  apiReader,
		HookRecord: make(map[string]string),
	}
}

// DoBeforeClusterUpgrade satisfies the CAPI interface for hook server.
func (e *Handler) DoBeforeClusterUpgrade(ctx context.Context, request *runtimehooksv1.BeforeClusterUpgradeRequest, response *runtimehooksv1.BeforeClusterUpgradeResponse) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("DoBeforeClusterUpgrade is called")

	// Get the ClusterAddon
	key := types.NamespacedName{Name: fmt.Sprintf("cluster-addon-%s", request.Cluster.GetName()), Namespace: request.Cluster.GetNamespace()}
	clusterAddon := &csov1alpha1.ClusterAddon{}
	if err := e.Get(ctx, key, clusterAddon); err != nil {
		log.Error(err, "failed to get cluster addon")
		response.SetStatus(runtimehooksv1.ResponseStatusFailure)
		response.SetMessage(err.Error())
		return
	}

	// the key consists of (name/namespace/clusterclass)
	hookRecordKey := e.getHookRecordKey(ctx, &request.Cluster)

	if clusterAddon.Spec.Hook != "BeforeClusterUpgrade" && !e.hasAlreadyPatchedClusterAddon(hookRecordKey, BeforeClusterUpgradeHook) {
		patchHelper, err := patch.NewHelper(clusterAddon, e)
		if err != nil {
			log.Error(err, "failed to create patch helper")
			response.SetStatus(runtimehooksv1.ResponseStatusFailure)
			response.SetMessage(err.Error())
		}

		clusterAddon.Spec.Hook = BeforeClusterUpgradeHook
		conditions.Delete(clusterAddon, csov1alpha1.HelmChartAppliedCondition)

		if err := patchHelper.Patch(ctx, clusterAddon); err != nil {
			log.Error(err, "failed to patch cluster addon")
			response.SetStatus(runtimehooksv1.ResponseStatusFailure)
			response.SetMessage(err.Error())
		}

		e.HookRecord[hookRecordKey] = BeforeClusterUpgradeHook
	}

	// check if hook is completed
	if !conditions.IsTrue(clusterAddon, csov1alpha1.HelmChartAppliedCondition) {
		response.SetRetryAfterSeconds(10)
	}

	response.SetStatus(runtimehooksv1.ResponseStatusSuccess)
}

// DoAfterClusterUpgrade satisfies the CAPI interface for hook server.
func (*Handler) DoAfterClusterUpgrade(ctx context.Context, request *runtimehooksv1.AfterClusterUpgradeRequest, response *runtimehooksv1.AfterClusterUpgradeResponse) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("DoAfterClusterUpgrade is called", "ClusterName", request.Cluster.GetName())

	response.SetStatus(runtimehooksv1.ResponseStatusSuccess)
}

// DoAfterControlPlaneInitialized satisfies the CAPI interface for hook server.
func (e *Handler) DoAfterControlPlaneInitialized(ctx context.Context, request *runtimehooksv1.AfterControlPlaneInitializedRequest, response *runtimehooksv1.AfterControlPlaneInitializedResponse) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("DoAfterControlPlaneInitialized is called", "ClusterName", request.Cluster.GetName())

	// Get the ClusterAddon
	key := types.NamespacedName{Name: fmt.Sprintf("cluster-addon-%s", request.Cluster.GetName()), Namespace: request.Cluster.GetNamespace()}
	clusterAddon := &csov1alpha1.ClusterAddon{}
	if err := e.Get(ctx, key, clusterAddon); err != nil {
		log.Error(err, "failed to get the cluster addon")
		response.SetStatus(runtimehooksv1.ResponseStatusFailure)
		response.SetMessage(err.Error())
		return
	}

	if clusterAddon.Spec.Hook != "AfterControlPlaneInitialized" {
		patchHelper, err := patch.NewHelper(clusterAddon, e)
		if err != nil {
			log.Error(err, "failed to create patch helper")
			response.SetStatus(runtimehooksv1.ResponseStatusFailure)
			response.SetMessage(fmt.Errorf("failed to init patch helper: %w", err).Error())
		}

		clusterAddon.Spec.Hook = "AfterControlPlaneInitialized"
		conditions.Delete(clusterAddon, csov1alpha1.HelmChartAppliedCondition)

		if err := patchHelper.Patch(ctx, clusterAddon, patch.WithForceOverwriteConditions{}); err != nil {
			log.Error(err, "failed to patch cluster addon")
			response.SetStatus(runtimehooksv1.ResponseStatusFailure)
			response.SetMessage(fmt.Errorf("failed to patch clusterAddon: %w", err).Error())
		}
	}

	response.SetStatus(runtimehooksv1.ResponseStatusSuccess)
}

func (e *Handler) getHookRecordKey(ctx context.Context, cluster *clusterv1.Cluster) string {
	return fmt.Sprintf("%s/%s/%s", cluster.GetName(), cluster.GetNamespace(), e.getClusterClass(ctx, cluster))
}

// getClusterClass returns the name of the ClusterClass of the cluster from the hook request.
// Cluster API >= 1.13 sends the request as v1beta2 Cluster, where the class is in spec.topology.classRef.name
// and not in spec.topology.class, so it is empty after decoding the request as v1beta1. In that case the
// class is read from the API server, which serves the Cluster as v1beta1 as well. This must not use the
// cache: Cluster API calls the hook right after it has seen the new class, when the cache of the CSO can
// still have the previous one.
func (e *Handler) getClusterClass(ctx context.Context, cluster *clusterv1.Cluster) string {
	if cluster.Spec.Topology != nil && cluster.Spec.Topology.Class != "" {
		return cluster.Spec.Topology.Class
	}

	current := &clusterv1.Cluster{}
	if err := e.APIReader.Get(ctx, client.ObjectKeyFromObject(cluster), current); err != nil {
		ctrl.LoggerFrom(ctx).Error(err, "failed to get the cluster to determine its cluster class")
		return ""
	}

	if current.Spec.Topology == nil {
		return ""
	}

	return current.Spec.Topology.Class
}

func (e *Handler) hasAlreadyPatchedClusterAddon(key, hook string) bool {
	val, ok := e.HookRecord[key]
	if !ok {
		return false
	}

	return val == hook
}
