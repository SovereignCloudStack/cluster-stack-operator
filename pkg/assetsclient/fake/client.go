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

	"github.com/SovereignCloudStack/cluster-stack-operator/pkg/assetsclient"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type fakeClient struct {
	log logr.Logger
}

type factory struct{}

var _ = assetsclient.Client(&fakeClient{})

var _ = assetsclient.Factory(&factory{})

// NewFactory returns a new factory for assets clients.
func NewFactory() assetsclient.Factory {
	return &factory{}
}

func (*factory) NewClient(ctx context.Context) (assetsclient.Client, error) {
	logger := log.FromContext(ctx)

	return &fakeClient{
		log: logger,
	}, nil
}

func (c *fakeClient) ListRelease(_ context.Context) ([]string, error) {
	c.log.Info("WARNING: called ListRelease of fake assets client")
	return nil, nil
}

// DownloadReleaseAssets downloads a list of release assets.
func (c *fakeClient) DownloadReleaseAssets(_ context.Context, _, _ string) error {
	c.log.Info("WARNING: called DownloadReleaseAssets of fake assets client")
	return nil
}
