// Package mocks implement important mocking interface of kube.
package mocks

import (
	"github.com/SovereignCloudStack/cluster-stack-operator/pkg/kube"
	"k8s.io/client-go/rest"
)

type kubeFactory struct {
	client *Client
}

// NewKubeFactory returns packer factory interface.
func NewKubeFactory(client *Client) kube.Factory {
	return &kubeFactory{client: client}
}

var _ = kube.Factory(&kubeFactory{})

func (f *kubeFactory) NewClient(_ string, _ *rest.Config) kube.Client {
	return f.client
}
