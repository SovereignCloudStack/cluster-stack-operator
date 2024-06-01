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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	// ClusterStackReleaseFinalizer is the finalizer for ClusterStackRelease objects.
	ClusterStackReleaseFinalizer = "clusterstackrelease.clusterstack.x-k8s.io"
)

// ClusterStackReleaseSpec defines the desired state of ClusterStackRelease.
type ClusterStackReleaseSpec struct {
	// ProviderRef specifies the reference to the ProviderClusterStackRelease object.
	// It has to be set only if the object exists, i.e. if the noProvider mode is turned off.
	// +optional
	ProviderRef *corev1.ObjectReference `json:"providerRef,omitempty"`
}

// ClusterStackReleaseStatus defines the observed state of ClusterStackRelease.
type ClusterStackReleaseStatus struct {
	// Resources specifies the status of the resources that this object administrates.
	// +optional
	Resources []*Resource `json:"resources,omitempty"`

	// KubernetesVersion is the Kubernetes version incl. patch version, e.g. 1.26.6.
	// The controller fetches the version from the release assets of the cluster stack.
	// +optional
	KubernetesVersion string `json:"kubernetesVersion,omitempty"`

	// +optional
	// +kubebuilder:default:=false
	Ready bool `json:"ready,omitempty"`

	// Conditions defines current service state of the ClusterAddon.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="K8s Version",type="string",JSONPath=".status.kubernetesVersion"
// +kubebuilder:printcolumn:name="Ready",type="boolean",JSONPath=".status.ready"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of ClusterStackRelease"
// +kubebuilder:printcolumn:name="Reason",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].reason"
// +kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].message"
// +kubebuilder:resource:shortName=cskr

// ClusterStackRelease is the Schema for the clusterstackreleases API.
type ClusterStackRelease struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterStackReleaseSpec   `json:"spec,omitempty"`
	Status ClusterStackReleaseStatus `json:"status,omitempty"`
}

// GetConditions returns the observations of the operational state of the ClusterAddon resource.
func (r *ClusterStackRelease) GetConditions() clusterv1.Conditions {
	return r.Status.Conditions
}

// SetConditions sets the underlying service state of the ClusterAddon to the predescribed clusterv1.Conditions.
func (r *ClusterStackRelease) SetConditions(conditions clusterv1.Conditions) {
	r.Status.Conditions = conditions
}

//+kubebuilder:object:root=true

// ClusterStackReleaseList contains a list of ClusterStackRelease.
type ClusterStackReleaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterStackRelease `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterStackRelease{}, &ClusterStackReleaseList{})
}
