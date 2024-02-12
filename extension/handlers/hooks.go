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

package handlers

import (
	"context"
	"fmt"

	csov1alpha1 "github.com/SovereignCloudStack/cluster-stack-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ExtensionHandler struct {
	client client.Client
}

func NewExtensionHandlers(client client.Client) *ExtensionHandler {
	return &ExtensionHandler{
		client: client,
	}
}

func (e *ExtensionHandler) DoBeforeClusterUpgrade(ctx context.Context, request *runtimehooksv1.BeforeClusterUpgradeRequest, response *runtimehooksv1.BeforeClusterUpgradeResponse) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("DoBeforeClusterUpgrade is called")

	// Get the ClusterAddon
	key := types.NamespacedName{Name: fmt.Sprintf("cluster-addon-%s", request.Cluster.GetName()), Namespace: request.Cluster.GetNamespace()}
	clusterAddon := &csov1alpha1.ClusterAddon{}
	if err := e.client.Get(ctx, key, clusterAddon); err != nil {
		log.Error(err, "failed to get cluster addon")
		response.SetStatus(runtimehooksv1.ResponseStatusFailure)
		response.SetMessage(err.Error())
		return
	}

	if clusterAddon.Spec.Hook != "BeforeClusterUpgrade" {
		patchHelper, err := patch.NewHelper(clusterAddon, e.client)
		if err != nil {
			log.Error(err, "failed to create patch helper")
			response.SetStatus(runtimehooksv1.ResponseStatusFailure)
			response.SetMessage(err.Error())
		}

		clusterAddon.Spec.Hook = "BeforeClusterUpgrade"
		conditions.Delete(clusterAddon, csov1alpha1.HelmChartAppliedCondition)

		if err := patchHelper.Patch(ctx, clusterAddon); err != nil {
			log.Error(err, "failed to patch cluster addon")
			response.SetStatus(runtimehooksv1.ResponseStatusFailure)
			response.SetMessage(err.Error())
		}

		response.SetRetryAfterSeconds(10)
		return
	}

	if e.isTimeoutConditionPresent(ctx, key, clusterAddon) {
		message := fmt.Sprintf("timeout occurred while evaluating CEL expression: %s", conditions.GetMessage(clusterAddon, csov1alpha1.EvaluatedCELCondition))
		log.Info(message)
		response.SetStatus(runtimehooksv1.ResponseStatusFailure)
		response.SetMessage(message)
		return
	}

	if !utils.IsPresentAndTrue(ctx, e.client, key, clusterAddon, csov1alpha1.HelmChartAppliedCondition) {
		response.SetRetryAfterSeconds(5)
		return
	}

	response.SetStatus(runtimehooksv1.ResponseStatusSuccess)
	return
}

func (e *ExtensionHandler) DoAfterClusterUpgrade(ctx context.Context, request *runtimehooksv1.AfterClusterUpgradeRequest, response *runtimehooksv1.AfterClusterUpgradeResponse) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("DoAfterClusterUpgrade is called", "ClusterName", request.Cluster.GetName())

	response.SetStatus(runtimehooksv1.ResponseStatusSuccess)
	return
}

func (e *ExtensionHandler) DoAfterControlPlaneInitialized(ctx context.Context, request *runtimehooksv1.AfterControlPlaneInitializedRequest, response *runtimehooksv1.AfterControlPlaneInitializedResponse) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("DoAfterControlPlaneInitialized is called", "ClusterName", request.Cluster.GetName())

	// Get the ClusterAddon
	key := types.NamespacedName{Name: fmt.Sprintf("cluster-addon-%s", request.Cluster.GetName()), Namespace: request.Cluster.GetNamespace()}
	clusterAddon := &csov1alpha1.ClusterAddon{}
	if err := e.client.Get(ctx, key, clusterAddon); err != nil {
		response.SetStatus(runtimehooksv1.ResponseStatusFailure)
		response.SetMessage(err.Error())
		return
	}

	if clusterAddon.Spec.Hook != "AfterControlPlaneInitialized" {
		patchHelper, err := patch.NewHelper(clusterAddon, e.client)
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

	if e.isTimeoutConditionPresent(ctx, key, clusterAddon) {
		message := fmt.Sprintf("timeout occurred while evaluating CEL expression: %s", conditions.GetMessage(clusterAddon, csov1alpha1.EvaluatedCELCondition))
		log.Info(message)
		response.SetStatus(runtimehooksv1.ResponseStatusFailure)
		response.SetMessage(message)
		return
	}

	response.SetStatus(runtimehooksv1.ResponseStatusSuccess)
	return
}

func (e *ExtensionHandler) isTimeoutConditionPresent(ctx context.Context, key types.NamespacedName, getter conditions.Getter) bool {
	return utils.IsPresentAndFalseWithReason(ctx, e.client, key, getter, csov1alpha1.EvaluatedCELCondition, csov1alpha1.CELEvaluationTimeoutReason)
}
