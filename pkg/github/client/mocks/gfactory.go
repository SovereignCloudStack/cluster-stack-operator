// Package mocks implement important mocking interface of packer.
package mocks

import (
	"context"

	githubclient "github.com/SovereignCloudStack/cluster-stack-operator/pkg/github/client"
)

type githubFactory struct {
	client *Client
}

// NewGitHubFactory returns a mocked Github client.
func NewGitHubFactory(client *Client) githubclient.Factory {
	return &githubFactory{client: client}
}

var _ = githubclient.Factory(&githubFactory{})

func (f *githubFactory) NewClient(_ context.Context) (githubclient.Client, error) {
	return f.client, nil
}
