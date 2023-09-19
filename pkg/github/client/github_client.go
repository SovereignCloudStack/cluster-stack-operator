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

package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/go-github/v52/github"
	"golang.org/x/oauth2"
)

// Client contains all functions to talk to Github API.
type Client interface {
	DownloadReleaseAssets(ctx context.Context, release *github.RepositoryRelease, path string, assetlist []string) error
	GetReleaseByTag(ctx context.Context, tag string) (*github.RepositoryRelease, *github.Response, error)
	ListRelease(ctx context.Context) ([]*github.RepositoryRelease, *github.Response, error)
}

// Factory is a factory to generate Github clients.
type Factory interface {
	NewClient(ctx context.Context) (Client, error)
}

type realGhClient struct {
	client     *github.Client
	httpclient *http.Client
	orgName    string
	repoName   string
}

type factory struct{}

var _ = Client(&realGhClient{})

var _ = Factory(&factory{})

// NewFactory returns a new factory for Github clients.
func NewFactory() Factory {
	return &factory{}
}

var _ = Client(&realGhClient{})

func (*factory) NewClient(ctx context.Context) (Client, error) {
	creds, err := NewGitConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create git config: %w", err)
	}
	ghclient, oAuthClient, err := githubAndOAuthClientWithToken(ctx, creds)
	if err != nil {
		return nil, fmt.Errorf("failed to create github client: %w", err)
	}

	if oAuthClient == nil {
		oAuthClient = http.DefaultClient
	}

	return &realGhClient{
		client:     ghclient,
		httpclient: oAuthClient,
		orgName:    creds.GitOrgName,
		repoName:   creds.GitRepoName,
	}, nil
}

func (c *realGhClient) ListRelease(ctx context.Context) ([]*github.RepositoryRelease, *github.Response, error) {
	repoRelease, response, err := c.client.Repositories.ListReleases(ctx, c.orgName, c.repoName, &github.ListOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list releases: %w", err)
	}

	return repoRelease, response, nil
}

func (c *realGhClient) GetReleaseByTag(ctx context.Context, tag string) (*github.RepositoryRelease, *github.Response, error) {
	repoRelease, response, err := c.client.Repositories.GetReleaseByTag(ctx, c.orgName, c.repoName, tag)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get release tag: %w", err)
	}

	return repoRelease, response, nil
}

// DownloadReleaseAssets downloads a list of release assets.
func (c *realGhClient) DownloadReleaseAssets(ctx context.Context, release *github.RepositoryRelease, path string, assetlist []string) error {
	if err := os.MkdirAll(path, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}
	// Extract the release assets
	for _, asset := range release.Assets {
		if !contains(assetlist, asset.GetName()) {
			continue
		}
		assetPath := filepath.Join(path, asset.GetName())
		// Create a temporary file (inside the dest dir) to save the downloaded asset file
		assetFile, err := os.Create(filepath.Clean(assetPath))
		if err != nil {
			return fmt.Errorf("failed to create temporary asset file: %w", err)
		}

		resp, redirectURL, err := c.client.Repositories.DownloadReleaseAsset(ctx, c.orgName, c.repoName, asset.GetID(), nil)
		if err != nil {
			return fmt.Errorf("failed to download the release asset from URL %s: %w", *asset.BrowserDownloadURL, err)
		}

		// if redirectURL is set, then response is nil and vice versa
		if redirectURL != "" {
			if err := c.handleRedirect(ctx, redirectURL, assetFile); err != nil {
				return fmt.Errorf("failed to handle redirect: %w", err)
			}
		} else {
			if _, err = io.Copy(assetFile, resp); err != nil {
				return fmt.Errorf("failed to save asset file %s from HTTP response: %w", assetPath, err)
			}

			if err := resp.Close(); err != nil {
				return fmt.Errorf("failed to close response: %w", err)
			}
		}

		if err := assetFile.Close(); err != nil {
			return fmt.Errorf("failed to close asset file: %w", err)
		}
	}
	return nil
}

func (c *realGhClient) handleRedirect(ctx context.Context, url string, assetFile *os.File) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return fmt.Errorf("failed to define http get request: %w", err)
	}

	resp, err := c.httpclient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to get URL %q: %w", url, err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download asset, HTTP status code: %d", resp.StatusCode)
	}

	if _, err := io.Copy(assetFile, resp.Body); err != nil {
		return fmt.Errorf("failed to copy http response in file: %w", err)
	}

	if err := resp.Body.Close(); err != nil {
		return fmt.Errorf("failed to close body of response: %w", err)
	}
	return nil
}

func githubAndOAuthClientWithToken(ctx context.Context, creds GitConfig) (githubClient *github.Client, oauthClient *http.Client, err error) {
	if creds.GitAccessToken == "" {
		githubClient = github.NewClient(nil)
	} else {
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: creds.GitAccessToken},
		)

		oauthClient = oauth2.NewClient(ctx, ts)
		githubClient = github.NewClient(oauthClient)
	}

	if err := verifyAccess(ctx, githubClient, creds); err != nil {
		return nil, &http.Client{}, fmt.Errorf("failed to access Git API: %w", err)
	}

	return githubClient, oauthClient, nil
}

func verifyAccess(ctx context.Context, client *github.Client, creds GitConfig) error {
	_, _, err := client.Repositories.Get(ctx, creds.GitOrgName, creds.GitRepoName)
	if err != nil {
		return fmt.Errorf("failed to get repository: %w", err)
	}
	return nil
}

func contains(source []string, ghAsset string) bool {
	for _, a := range source {
		if a == ghAsset || strings.Contains(ghAsset, a) {
			return true
		}
	}
	return false
}
