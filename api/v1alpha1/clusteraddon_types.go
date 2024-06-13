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
	"github.com/SovereignCloudStack/cluster-stack-operator/pkg/clusteraddon"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	// ClusterAddonFinalizer is the finalizer for ClusterAddon objects.
	ClusterAddonFinalizer = "clusteraddon.clusterstack.x-k8s.io"
)

// StageAnnotation is the annotation name and key for the stage annotation.
const StageAnnotation = "ClusterAddonStage"

// StageAnnotationValue is the value of the stage annotation.
type StageAnnotationValue string

const (
	// StageAnnotationValueCreated signifies the stage annotation created.
	StageAnnotationValueCreated = StageAnnotationValue("created")
	// StageAnnotationValueUpgraded signifies the stage annotation upgraded.
	StageAnnotationValueUpgraded = StageAnnotationValue("upgraded")
)

// StagePhase defines the status of helm chart in the cluster addon.
type StagePhase string

var (
	// StagePhaseNone signifies the empty stage phase.
	StagePhaseNone = StagePhase("")
	// StagePhasePending signifies the stage phase 'pending'.
	StagePhasePending = StagePhase("Pending")
	// StagePhaseWaitingForPreCondition signifies the stage phase 'waitingForPreCondition'.
	StagePhaseWaitingForPreCondition = StagePhase("waitingForPreCondition")
	// StagePhaseApplyingOrDeleting signifies the stage phase 'applyingOrDeleting'.
	StagePhaseApplyingOrDeleting = StagePhase("applyingOrDeleting")
	// StagePhaseWaitingForPostCondition signifies the stage phase 'waitingForPostCondition'.
	StagePhaseWaitingForPostCondition = StagePhase("waitingForPostCondition")
	// StagePhaseDone signifies the stage phase 'done'.
	StagePhaseDone = StagePhase("done")
)

// StageStatus represents the helm charts of the hook and it's phases.
type StageStatus struct {
	// Name represent name of the helm chart
	// +optional
	Name string `json:"name"`

	// Action is the action of the helm chart. e.g. - apply and delete.
	// +optional
	Action clusteraddon.Action `json:"action,omitempty"`

	// Phase is the current phase of the helm chart.
	// +optional
	Phase StagePhase `json:"phase"`
}

// ClusterAddonSpec defines the desired state of a ClusterAddon object.
type ClusterAddonSpec struct {
	// ClusterStack is the full string <provider>-<name>-<Kubernetes version>-<version> that will be filled with the cluster stack that
	// the respective cluster uses currently. It always matches cluster.spec.topology.class if the work of this controller is done.
	// +optional
	ClusterStack string `json:"clusterStack,omitempty"`

	// Version is the version of the cluster addons that have been applied in the workload cluster.
	// +optional
	Version string `json:"version,omitempty"`

	// Hook specifies the runtime hook for the Cluster event.
	// +optional
	Hook string `json:"hook,omitempty"`

	// ClusterRef is the reference to the clusterv1.Cluster object that corresponds to the workload cluster where this
	// controller applies the cluster addons.
	ClusterRef *corev1.ObjectReference `json:"clusterRef"`
}

// ClusterAddonStatus defines the observed state of ClusterAddon.
type ClusterAddonStatus struct {
	// Resources specifies the status of the resources that this object administrates.
	// +optional
	Resources []*Resource `json:"resources,omitempty"`

	// Stages shows the state of all stages in the current running hook.
	// +optional
	Stages []StageStatus `json:"stages,omitempty"`

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
// +kubebuilder:printcolumn:name="Hook",type="string",JSONPath=".spec.hook",description="Present running hook"
// +kubebuilder:printcolumn:name="Ready",type="boolean",JSONPath=".status.ready"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of Cluster Addon"
// +kubebuilder:printcolumn:name="Reason",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].reason"
// +kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].message"
// +kubebuilder:resource:shortName=caddon

// ClusterAddon is the schema for the clusteraddons API.
type ClusterAddon struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterAddonSpec   `json:"spec,omitempty"`
	Status ClusterAddonStatus `json:"status,omitempty"`
}

// GetStagePhase returns helm chart status for the helm chart.
func (r *ClusterAddon) GetStagePhase(stageName string, action clusteraddon.Action) StagePhase {
	for _, stage := range r.Status.Stages {
		if stage.Name == stageName && stage.Action == action {
			return stage.Phase
		}
	}

	// This cannot occur as we populate phase value with "pending".
	return StagePhaseNone
}

// SetStagePhase sets the helm chart status phase.
func (r *ClusterAddon) SetStagePhase(stageName string, action clusteraddon.Action, phase StagePhase) {
	for i := range r.Status.Stages {
		if r.Status.Stages[i].Name == stageName && r.Status.Stages[i].Action == action {
			r.Status.Stages[i].Phase = phase
		}
	}
}

// SetStageAnnotations sets the annotation whether the cluster got created or upgraded.
func (r *ClusterAddon) SetStageAnnotations(value StageAnnotationValue) {
	if r.Annotations == nil {
		r.Annotations = make(map[string]string, 0)
	}
	_, found := r.Annotations[StageAnnotation]
	if !found {
		r.Annotations[StageAnnotation] = string(value)
	}
}

// HasStageAnnotation returns whether the stage annotation exists with a certain value.
func (r *ClusterAddon) HasStageAnnotation(value StageAnnotationValue) bool {
	val, found := r.Annotations[StageAnnotation]
	if found && val == string(value) {
		return true
	}

	return false
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
