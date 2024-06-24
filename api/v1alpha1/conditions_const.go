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

package v1alpha1

import clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

const (
	// ClusterReadyCondition reports on whether the associated cluster is ready.
	ClusterReadyCondition clusterv1.ConditionType = "ClusterReady"

	// ControlPlaneNotReadyReason is used when the control planes of a cluster are not ready yet.
	ControlPlaneNotReadyReason = "ControlPlaneNotReady"
)

const (
	// EvaluatedCELCondition reports on whether the CEL expression is evaluated properly.
	EvaluatedCELCondition clusterv1.ConditionType = "EvaluatedCEL"

	// FailedToEvaluatePreConditionReason is used when some pre CEL expression have been failed to evaluate.
	FailedToEvaluatePreConditionReason = "FailedToEvaluatePreCondition"

	// FailedToEvaluatePostConditionReason is used when some post CEL expression have been failed to evaluate.
	FailedToEvaluatePostConditionReason = "FailedToEvaluatePostCondition"
)

const (
	// HelmChartFoundCondition reports when mentioned helm chart is present in the cluster addon tar archive.
	HelmChartFoundCondition = "HelmChartFound"

	// HelmChartMissingReason is used when mentioned helm chart is missing in the cluster addon tar archive.
	HelmChartMissingReason = "HelmChartMissing"
)

const (
	// HelmChartTemplatedCondition reports on whether the relevant helm chart has been templated properly.
	HelmChartTemplatedCondition clusterv1.ConditionType = "HelmChartTemplated"

	// TemplateOldClusterStackOverwriteFailedReason is used when old cluster stack overwrite.yaml is wrong.
	TemplateOldClusterStackOverwriteFailedReason = "TemplateOldClusterStackOverwriteFailed"

	// TemplateOldClusterStackFailedReason is used when there is a issue doing helm template for the old cluster stack.
	TemplateOldClusterStackFailedReason = "TemplateOldClusterStackFailed"

	// TemplateNewClusterStackOverwriteFailedReason is used when new cluster stack overwrite.yaml is wrong.
	TemplateNewClusterStackOverwriteFailedReason = "TemplateNewClusterStackOverwriteFailed"

	// TemplateNewClusterStackFailedReason is used when there is a issue doing helm template for the new cluster stack.
	TemplateNewClusterStackFailedReason = "TemplateNewClusterStackFailed"
)

const (
	// HelmChartAppliedCondition reports on whether the relevant helm chart has been applied.
	HelmChartAppliedCondition clusterv1.ConditionType = "HelmChartApplied"

	// FailedToApplyObjectsReason is used when some objects have been failed to apply.
	FailedToApplyObjectsReason = "FailedToApplyObjects"

	// ObjectsApplyingOngoingReason is used when the objects are still being applied.
	ObjectsApplyingOngoingReason = "ObjectsApplyingOngoing"
)

const (
	// HookServerReadyCondition reports on whether hook server is ready or not.
	HookServerReadyCondition clusterv1.ConditionType = "HookServerReady"

	// HookServerUnresponsiveReason is used when hook server don't update the clusterAddon.Spec.Hook.
	HookServerUnresponsiveReason = "HookServerUnresponsive"
)

const (
	// HelmChartDeletedCondition reports on whether the relevant helm chart has been applied.
	HelmChartDeletedCondition clusterv1.ConditionType = "HelmChartDeleted"

	// FailedToDeleteObjectsReason is used when some objects have been failed to delete.
	FailedToDeleteObjectsReason = "FailedToDeleteObjects"
)

const (
	// ProviderClusterStackReleasesSyncedCondition reports on whether the ProviderClusterStackReleases are ready.
	ProviderClusterStackReleasesSyncedCondition = "ProviderClusterStackReleasesSynced"

	// ProviderTemplateNotFoundReason is used when providerTemplate is not found.
	ProviderTemplateNotFoundReason = "ProviderTemplateNotFound"

	// FailedToCreateOrUpdateReason is used when ProviderClusterStackRelease was failed to be created or updated.
	FailedToCreateOrUpdateReason = "FailedToCreateOrUpdate"
)

const (
	// ClusterStackReleasesSyncedCondition reports on whether the ClusterStackReleases are ready.
	ClusterStackReleasesSyncedCondition = "ClusterStackReleasesSynced" //#nosec
)

const (
	// ClusterStackReleaseAvailableCondition reports on whether there is at least one ClusterStackRelease available to use.
	ClusterStackReleaseAvailableCondition = "ClusterStackReleaseAvailable" //#nosec
)

const (
	// ClusterStackReleaseAssetsReadyCondition reports on whether the download of cluster stack release assets is complete.
	ClusterStackReleaseAssetsReadyCondition = "ClusterStackReleaseDownloaded"

	// ReleaseAssetsNotDownloadedYetReason is used when release assets are not yet downloaded.
	ReleaseAssetsNotDownloadedYetReason = "ReleaseAssetsNotDownloadedYet"

	// IssueWithReleaseAssetsReason is used when release assets have an issue.
	IssueWithReleaseAssetsReason = "IssueWithReleaseAssets"
)

const (
	// ProviderClusterStackReleaseReadyCondition reports on whether the relevant provider-specific object is ready.
	ProviderClusterStackReleaseReadyCondition clusterv1.ConditionType = "ProviderClusterStackReleaseReady"

	// ProcessOngoingReason is used when the process of the provider-specific object is still ongoing.
	ProcessOngoingReason = "ProcessOngoing"
)

const (
	// GitAPIAvailableCondition is used when Git API is available.
	GitAPIAvailableCondition clusterv1.ConditionType = "GitAPIAvailable"

	// GitTokenOrEnvVariableNotSetReason is used when user don't specify the token or environment variable.
	GitTokenOrEnvVariableNotSetReason = "GitTokenOrEnvVariableNotSet" //#nosec

	// FailedCreateGithubClientReason is used when user don't specify the token or environment variable.
	FailedCreateGithubClientReason = "FailedCreateGithubClient" //#nosec
)

const (
	// GitReleasesSyncedCondition is used when Git releases have been synced successfully.
	GitReleasesSyncedCondition clusterv1.ConditionType = "GitReleasesSynced"

	// FailedToSyncReason is used when Git releases could not be synced.
	FailedToSyncReason = "FailedToSync"
)
