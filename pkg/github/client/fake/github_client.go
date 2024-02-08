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

// Package fake defines a fake Gitub client.
package fake

import (
	"context"
	"net/http"

	"github.com/SovereignCloudStack/cluster-stack-operator/pkg/github/client"
	"github.com/go-logr/logr"
	"github.com/google/go-github/v52/github"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type fakeClient struct {
	log logr.Logger
}

type factory struct{}

var _ = client.Client(&fakeClient{})

var _ = client.Factory(&factory{})

// NewFactory returns a new factory for Github clients.
func NewFactory() client.Factory {
	return &factory{}
}

func (*factory) NewClient(ctx context.Context) (client.Client, error) {
	logger := log.FromContext(ctx)

	return &fakeClient{
		log: logger,
	}, nil
}

func (c *fakeClient) ListRelease(_ context.Context) ([]*github.RepositoryRelease, *github.Response, error) {
	c.log.Info("WARNING: called ListRelease of fake Github client")
	resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
	return nil, resp, nil
}

func (c *fakeClient) GetReleaseByTag(_ context.Context, _ string) (*github.RepositoryRelease, *github.Response, error) {
	c.log.Info("WARNING: called GetReleaseByTag of fake Github client")
	resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
	return nil, resp, nil
}

// DownloadReleaseAssets downloads a list of release assets.
func (c *fakeClient) DownloadReleaseAssets(_ context.Context, _ *github.RepositoryRelease, _ string, _ []string) error {
	c.log.Info("WARNING: called DownloadReleaseAssets of fake Github client")
	return nil
}
