/*
Copyright 2024 The Kubernetes Authors.

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

package oci

import (
	"context"
	"errors"
	"fmt"

	"github.com/SovereignCloudStack/cluster-stack-operator/pkg/assetsclient"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
)

type ociClient struct {
	repository *remote.Repository
}

type factory struct{}

// NewFactory returns a new factory for OCI clients.
func NewFactory() assetsclient.Factory {
	return &factory{}
}

var _ = assetsclient.Factory(&factory{})

var _ = assetsclient.Client(&ociClient{})

func (*factory) NewClient(ctx context.Context) (assetsclient.Client, error) {
	_ = ctx
	config, err := newOCIConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create OCI config: %w", err)
	}

	client := auth.Client{
		Credential: auth.StaticCredential(config.registry, auth.Credential{
			AccessToken: config.accessToken,
			Username:    config.username,
			Password:    config.password,
		}),
	}

	repository, err := remote.NewRepository(config.repository)
	if err != nil {
		return nil, fmt.Errorf("failed to create OCI client to remote repository %s: %w", config.repository, err)
	}

	repository.Client = &client
	return &ociClient{repository: repository}, nil
}

func (c *ociClient) ListRelease(ctx context.Context) ([]string, error) {
	tags, err := registry.Tags(ctx, c.repository)
	if err != nil {
		return nil, fmt.Errorf("failed to list tags: %w", err)
	}

	return tags, nil
}

func (c *ociClient) DownloadReleaseAssets(ctx context.Context, tag, path string) (reterr error) {
	dest, err := file.New(path)
	if err != nil {
		return fmt.Errorf("failed to create file store: %w", err)
	}

	defer func() {
		err := dest.Close()
		if err != nil {
			reterr = errors.Join(reterr, err)
		}
	}()

	_, err = oras.Copy(ctx, c.repository, tag, dest, tag, oras.DefaultCopyOptions)
	if err != nil {
		return fmt.Errorf("failed to copy repository artifacts to path %s: %w", path, err)
	}

	return nil
}
