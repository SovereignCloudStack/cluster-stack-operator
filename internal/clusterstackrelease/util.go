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
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
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
	if conditions.IsTrue(csr, clusterv1.ReadyCondition) {
		summary.Ready = true
		summary.Phase = csov1alpha1.ClusterStackReleasePhaseDone
		return summary, nil
	} else if conditions.IsFalse(csr, clusterv1.ReadyCondition) {
		// if it is not ready, then we need to give a reason
		summary.Message = conditions.GetReason(csr, clusterv1.ReadyCondition)
	}

	// if provider-specific work is done, we are left with applying objects
	// We don't expect the condition to be not set at all, hence no else case here
	switch {
	case conditions.IsTrue(csr, csov1alpha1.ProviderClusterStackReleaseReadyCondition):
		summary.Phase = csov1alpha1.ClusterStackReleasePhaseApplyingObjects
	case conditions.IsTrue(csr, csov1alpha1.ClusterStackReleaseAssetsReadyCondition):
		summary.Phase = csov1alpha1.ClusterStackReleasePhaseProviderSpecificWork
	case conditions.IsFalse(csr, csov1alpha1.ClusterStackReleaseAssetsReadyCondition):
		summary.Phase = csov1alpha1.ClusterStackReleasePhaseDownloadingAssets
	}

	return summary, nil
}
