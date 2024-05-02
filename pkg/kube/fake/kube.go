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

// Package fake implements important interface like kube client.
package fake

import (
	"context"

	csov1alpha1 "github.com/SovereignCloudStack/cluster-stack-operator/api/v1alpha1"
	kubeclient "github.com/SovereignCloudStack/cluster-stack-operator/pkg/kube"
	"k8s.io/client-go/rest"
)

// kube is the struct for kube client.
type kube struct{}

type factory struct{}

// NewClient gives reference to the fake yaml client.
func (*factory) NewClient(_ string, _ *rest.Config) kubeclient.Client {
	return &kube{}
}

// NewFactory creates new fake kube client factories.
func NewFactory() kubeclient.Factory {
	return &factory{}
}

// Apply applies the given yaml object.
func (*kube) Apply(_ context.Context, _ []byte, _ []*csov1alpha1.Resource, _ bool) (_ []*csov1alpha1.Resource, _ bool, _ error) {
	return nil, false, nil
}

// Delete deletes the given yaml object.
func (*kube) Delete(_ context.Context, _ []byte, _ []*csov1alpha1.Resource) (_ []*csov1alpha1.Resource, _ bool, _ error) {
	return nil, false, nil
}

func (*kube) ApplyNewClusterStack(_ context.Context, _, _ []byte) (_ []*csov1alpha1.Resource, _ bool, _ error) {
	return nil, false, nil
}

func (*kube) DeleteNewClusterStack(_ context.Context, _ []byte) (_ []*csov1alpha1.Resource, _ bool, _ error) {
	return nil, false, nil
}
