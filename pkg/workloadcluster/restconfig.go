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

// Package workloadcluster includes implementation of workload cluster interfaces.
package workloadcluster

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/cluster-api/util/secret"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type restClient struct {
	controllerClient client.Client
	clusterName      string
	namespace        string
}

// Client includes method to get rest config for a workload cluster.
type Client interface {
	RestConfig(ctx context.Context) (*rest.Config, error)
}

// Factory interface includes methods for gettong a rest client.
type Factory interface {
	NewClient(name, namespace string, controllerClient client.Client) Client
}

type factory struct{}

// NewClient creates new workload cluster clients.
func (*factory) NewClient(name, namespace string, controllerClient client.Client) Client {
	return &restClient{clusterName: name, namespace: namespace, controllerClient: controllerClient}
}

var _ = Factory(&factory{})

// NewFactory creates a new factory for workload cluster clients.
func NewFactory() Factory {
	return &factory{}
}

var _ Client = &restClient{}

func (r *restClient) RestConfig(ctx context.Context) (*rest.Config, error) {
	namespacedName := types.NamespacedName{
		Name:      fmt.Sprintf("%s-%s", r.clusterName, secret.Kubeconfig),
		Namespace: r.namespace,
	}
	restSecret := &corev1.Secret{}
	if err := r.controllerClient.Get(ctx, namespacedName, restSecret); err != nil {
		return nil, fmt.Errorf("failed to get secret from management cluster in %s/%s: %w", namespacedName.Name, namespacedName.Namespace, err)
	}
	kubeconfig := string(restSecret.Data["value"])

	clientCfg, err := clientcmd.NewClientConfigFromBytes([]byte(kubeconfig))
	if err != nil {
		return nil, fmt.Errorf("failed to create client config from secret: %w", err)
	}

	// Get the rest config from the kubeconfig
	restCfg, err := clientCfg.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get rest config from client config: %w", err)
	}

	return restCfg, nil
}
