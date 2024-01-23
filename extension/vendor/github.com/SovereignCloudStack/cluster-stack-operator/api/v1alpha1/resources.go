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
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ErrNilInput indicates a nil input.
var ErrNilInput = fmt.Errorf("nil input")

// Resource defines the status of a resource.
type Resource struct {
	// Group specifies the group of the object.
	Group string `json:"group,omitempty"`
	// Version specifies the version of the object.
	Version string `json:"version,omitempty"`
	// Kind specifies the kind of the object.
	Kind string `json:"kind,omitempty"`
	// Namespace specifies the namespace of the object.
	Namespace string `json:"namespace,omitempty"`
	// Name specifies the name of the object.
	Name string `json:"name,omitempty"`
	// Status specifies the status of the object being applied.
	Status ResourceStatus `json:"status,omitempty"`
	// Error specifies the error of the last time this object has been applied.
	Error string `json:"error,omitempty"`
}

// ResourceStatus defines the status of a resource.
type ResourceStatus string

const (
	// ResourceStatusSynced means a resource is synced.
	ResourceStatusSynced = ResourceStatus("synced")
	// ResourceStatusNotSynced means a resource is not synced.
	ResourceStatusNotSynced = ResourceStatus("not-synced")
)

// NamespacedName returns a string of the form <namespace>/<name>.
func (r *Resource) NamespacedName() string {
	return r.Namespace + "/" + r.Name
}

// NewResourceFromUnstructured defines a new resource based on an unstructured object. For a nil input it returns a nil resource.
func NewResourceFromUnstructured(u *unstructured.Unstructured) *Resource {
	if u == nil {
		return nil
	}

	return &Resource{
		Group:     u.GetObjectKind().GroupVersionKind().Group,
		Version:   u.GetObjectKind().GroupVersionKind().Version,
		Kind:      u.GetObjectKind().GroupVersionKind().Kind,
		Namespace: u.GetNamespace(),
		Name:      u.GetName(),
	}
}

// GroupVersionKind returns a schema.GroupVersionKind.
func (r *Resource) GroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   r.Group,
		Version: r.Version,
		Kind:    r.Kind,
	}
}
