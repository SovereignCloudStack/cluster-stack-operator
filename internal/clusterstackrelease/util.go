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

package clusterstackrelease

import (
	"fmt"

	csov1alpha1 "github.com/SovereignCloudStack/cluster-stack-operator/api/v1alpha1"
	"github.com/SovereignCloudStack/cluster-stack-operator/pkg/clusterstack"
	"sigs.k8s.io/cluster-api/util/conditions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Summary returns a ClusterStackReleaseSummary object from a clusterStackRelease.
func Summary(csr *csov1alpha1.ClusterStackRelease) (csov1alpha1.ClusterStackReleaseSummary, error) {
	clusterStack, err := clusterstack.NewFromClusterStackReleaseProperties(csr.Name)
	if err != nil {
		return csov1alpha1.ClusterStackReleaseSummary{}, fmt.Errorf("failed to create clusterStack from string %s: %w", csr.Name, err)
	}

	summary := csov1alpha1.ClusterStackReleaseSummary{
		Name: clusterStack.Version.String(),
	}

	// if csr is ready, we mark that in summary
	if conditions.Get(csr, string(csov1alpha1.ClusterStackReleaseAvailableCondition)) != nil && conditions.Get(csr, string(csov1alpha1.ClusterStackReleaseAvailableCondition)).Status == metav1.ConditionTrue {
		summary.Ready = true
		summary.Phase = csov1alpha1.ClusterStackReleasePhaseDone
		return summary, nil
	} else if conditions.Get(csr, string(csov1alpha1.ClusterStackReleaseAvailableCondition)) != nil && conditions.Get(csr, string(csov1alpha1.ClusterStackReleaseAvailableCondition)).Status == metav1.ConditionFalse {
		// if it is not ready, then we need to give a reason
		summary.Message = conditions.Get(csr, string(csov1alpha1.ClusterStackReleaseAvailableCondition)).Reason
	}

	// if provider-specific work is done, we are left with applying objects
	// We don't expect the condition to be not set at all, hence no else case here
	switch {
	case conditions.Get(csr, string(csov1alpha1.ProviderClusterStackReleaseReadyCondition)) != nil && conditions.Get(csr, string(csov1alpha1.ProviderClusterStackReleaseReadyCondition)).Status == metav1.ConditionTrue:
		summary.Phase = csov1alpha1.ClusterStackReleasePhaseApplyingObjects
	case conditions.Get(csr, string(csov1alpha1.ClusterStackReleaseAssetsReadyCondition)) != nil && conditions.Get(csr, string(csov1alpha1.ClusterStackReleaseAssetsReadyCondition)).Status == metav1.ConditionTrue:
		summary.Phase = csov1alpha1.ClusterStackReleasePhaseProviderSpecificWork
	case conditions.Get(csr, string(csov1alpha1.ClusterStackReleaseAssetsReadyCondition)) != nil && conditions.Get(csr, string(csov1alpha1.ClusterStackReleaseAssetsReadyCondition)).Status == metav1.ConditionFalse:
		summary.Phase = csov1alpha1.ClusterStackReleasePhaseDownloadingAssets
	}

	return summary, nil
}
