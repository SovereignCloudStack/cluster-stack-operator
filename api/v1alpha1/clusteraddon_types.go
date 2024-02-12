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
	// ClusterAddonFinalizer is the finalizer for ClusterAddon objects.
	ClusterAddonFinalizer = "clusteraddon.clusterstack.x-k8s.io"
)

// ClusterAddonSpec defines the desired state of a ClusterAddon object.
type ClusterAddonSpec struct {
	// ClusterStack is the full string <provider>-<name>-<Kubernetes version>-<version> that will be filled with the cluster stack that
	// the respective cluster uses currently. It always matches cluster.spec.topology.class if the work of this controller is done.
	// +optional
	ClusterStack string `json:"clusterStack,omitempty"`

	// Version is the version of the cluster addons that have been applied in the workload cluster.
	// +optional
	Version string `json:"version,omitempty"`

	// ClusterRef is the reference to the clusterv1.Cluster object that corresponds to the workload cluster where this
	// controller applies the cluster addons.
	ClusterRef *corev1.ObjectReference `json:"clusterRef"`
}

// ClusterAddonStatus defines the observed state of ClusterAddon.
type ClusterAddonStatus struct {
	// Resources specifies the status of the resources that this object administrates.
	// +optional
	Resources []*Resource `json:"resources,omitempty"`

	// CurrentHook specifies the current running Hook.
	// +optional
	CurrentHook string `json:"currentHook,omitempty"`

	// KubernetesVersion is the kubernetes version of the current cluster stack release.
	// +optional
	KubernetesVersion string `json:"kubernetesVersion,omitempty"`

	// HelmChartStatus defines the status of helm chart in the cluster addon.
	// +optional
	HelmChartStatus map[string]HelmChartStatusConditions `json:"helmChartStatus,omitempty"`

	// +optional
	// +kubebuilder:default:=false
	Ready bool `json:"ready"`

	// Conditions define the current service state of the ClusterAddon.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.ownerReferences[?(@.kind==\"Cluster\")].name"
// +kubebuilder:printcolumn:name="Ready",type="boolean",JSONPath=".status.ready"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of Cluster Addon"
// +kubebuilder:printcolumn:name="Reason",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].reason"
// +kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].message"

// ClusterAddon is the schema for the clusteraddons API.
type ClusterAddon struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterAddonSpec   `json:"spec,omitempty"`
	Status ClusterAddonStatus `json:"status,omitempty"`
}

// GetConditions returns the observations of the operational state of the ClusterAddon resource.
func (r *ClusterAddon) GetConditions() clusterv1.Conditions {
	return r.Status.Conditions
}

// SetConditions sets the underlying service state of the ClusterAddon to the predescribed clusterv1.Conditions.
func (r *ClusterAddon) SetConditions(conditions clusterv1.Conditions) {
	r.Status.Conditions = conditions
}

//+kubebuilder:object:root=true

// ClusterAddonList contains a list of ClusterAddon.
type ClusterAddonList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterAddon `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterAddon{}, &ClusterAddonList{})
}
