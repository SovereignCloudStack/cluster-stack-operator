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

import (
	"github.com/SovereignCloudStack/cluster-stack-operator/pkg/version"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// ClusterStackSpec defines the desired state of ClusterStack.
type ClusterStackSpec struct {
	// Provider is the name of the cluster stack provider.
	// +kubebuilder:validation:MinLength=1
	Provider string `json:"provider"`

	// Name is the name of the cluster stack.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// KubernetesVersion is the Kubernetes version in the format '<majorVersion>.<minorVersion>', e.g. 1.26.
	// +kubebuilder:validation:Pattern=`^\d\.\d+$`
	KubernetesVersion string `json:"kubernetesVersion"`

	// Channel specifies the release channel of the cluster stack. Defaults to 'stable'.
	// +kubebuilder:default:=stable
	// +kubebuilder:validation:enum=stable;alpha;beta;rc
	Channel version.Channel `json:"channel,omitempty"`

	// Versions is a list of version of the cluster stack that should be available in the management cluster.
	// A version has to have the format 'v<versionNumber>', e.g. v1 for stable channel or, v1-alpha.1 for alpha channel.
	// The versions have to correspond to the channel property.
	// +optional
	Versions []string `json:"versions"`

	// AutoSubscribe is a feature where the operator checks automatically if there are new versions of this cluster stack available.
	// +optional
	// +kubebuilder:default:=true
	AutoSubscribe bool `json:"autoSubscribe"`

	// NoProvider indicates if set on true that there is no provider-specific implementation and operator.
	// +optional
	// +kubebuilder:default:=false
	NoProvider bool `json:"noProvider,omitempty"`

	// ProviderRef has to reference the ProviderClusterStackReleaseTemplate that contains all provider-specific information.
	// +optional
	ProviderRef *corev1.ObjectReference `json:"providerRef,omitempty"`
}

// ClusterStackStatus defines the observed state of ClusterStack.
type ClusterStackStatus struct {
	// +optional
	LatestRelease string `json:"latestRelease"`

	// +optional
	Summary []ClusterStackReleaseSummary `json:"summary,omitempty"`

	// +optional
	UsableVersions string `json:"usableVersions,omitempty"`

	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

// ClusterStackReleaseSummary gives the summary of the status of a ClusterStackRelease object.
type ClusterStackReleaseSummary struct {
	Name  string                   `json:"name"`
	Ready bool                     `json:"ready"`
	Phase ClusterStackReleasePhase `json:"phase"`

	// +optional
	Message string `json:"message,omitempty"`
}

// ClusterStackReleasePhase is the phase of a ClusterStackRelease object.
type ClusterStackReleasePhase string

const (
	// ClusterStackReleasePhaseNone is the default phase.
	ClusterStackReleasePhaseNone = ClusterStackReleasePhase("")

	// ClusterStackReleasePhaseDownloadingAssets is the phase where assets are downloaded from git provider.
	ClusterStackReleasePhaseDownloadingAssets = ClusterStackReleasePhase("downloading release assets")

	// ClusterStackReleasePhaseProviderSpecificWork is the phase where provider-specific work is done.
	ClusterStackReleasePhaseProviderSpecificWork = ClusterStackReleasePhase("provider-specific work")

	// ClusterStackReleasePhaseApplyingObjects is the phase where objects are applied to the management cluster.
	ClusterStackReleasePhaseApplyingObjects = ClusterStackReleasePhase("applying objects")

	// ClusterStackReleasePhaseDone is the phase where all jobs are done.
	ClusterStackReleasePhaseDone = ClusterStackReleasePhase("done")
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Provider",type="string",JSONPath=".spec.provider"
// +kubebuilder:printcolumn:name="ClusterStack",type="string",JSONPath=".spec.name"
// +kubebuilder:printcolumn:name="K8s",type="string",JSONPath=".spec.kubernetesVersion"
// +kubebuilder:printcolumn:name="Channel",type="string",JSONPath=".spec.channel"
// +kubebuilder:printcolumn:name="Autosubscribe",type="string",JSONPath=".spec.autoSubscribe"
// +kubebuilder:printcolumn:name="Usable",type="string",JSONPath=".status.usableVersions"
// +kubebuilder:printcolumn:name="Latest",type="string",JSONPath=".status.latestRelease"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of ClusterStack"
// +kubebuilder:printcolumn:name="Reason",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].reason"
// +kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].message"

// ClusterStack is the Schema for the clusterstacks API.
type ClusterStack struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterStackSpec   `json:"spec,omitempty"`
	Status ClusterStackStatus `json:"status,omitempty"`
}

// GetConditions returns the observations of the operational state of the ClusterAddon resource.
func (r *ClusterStack) GetConditions() clusterv1.Conditions {
	return r.Status.Conditions
}

// SetConditions sets the underlying service state of the ClusterAddon to the predescribed clusterv1.Conditions.
func (r *ClusterStack) SetConditions(conditions clusterv1.Conditions) {
	r.Status.Conditions = conditions
}

//+kubebuilder:object:root=true

// ClusterStackList contains a list of ClusterStack.
type ClusterStackList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterStack `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterStack{}, &ClusterStackList{})
}
