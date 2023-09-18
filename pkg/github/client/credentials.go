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

// Package client contains interface for github client.
package client

import (
	"fmt"
	"os"
)

const (
	// EnvGitProvider is the provider name.
	EnvGitProvider = "GIT_PROVIDER"

	// EnvGitOrgName is the git org name.
	EnvGitOrgName = "GIT_ORG_NAME"

	// EnvGitRepositoryName is the repository name.
	EnvGitRepositoryName = "GIT_REPOSITORY_NAME"

	// EnvGitAccessToken used for accessing private repositories.
	EnvGitAccessToken = "GIT_ACCESS_TOKEN"
)

// GitConfig contains necessary data to connect to github.
type GitConfig struct {
	GitProvider    string
	GitOrgName     string
	GitRepoName    string
	GitAccessToken string
}

// NewGitConfig ensures the environment variables required for the operator to run
// are set. Returns false if any of the required environment variables are not set.
func NewGitConfig() (GitConfig, error) {
	var gitCfg GitConfig

	val, ok := os.LookupEnv(EnvGitProvider)
	if val == "" || !ok {
		return GitConfig{}, fmt.Errorf("environment variable %s is not set", EnvGitProvider)
	} else if val != "github" {
		return GitConfig{}, fmt.Errorf("only github is supported as %s", EnvGitProvider)
	}
	gitCfg.GitProvider = val

	val, ok = os.LookupEnv(EnvGitOrgName)
	if val == "" || !ok {
		return GitConfig{}, fmt.Errorf("environment variable %s is not set", EnvGitOrgName)
	}
	gitCfg.GitOrgName = val

	val, ok = os.LookupEnv(EnvGitRepositoryName)
	if val == "" || !ok {
		return GitConfig{}, fmt.Errorf("environment variable %s is not set", EnvGitRepositoryName)
	}
	gitCfg.GitRepoName = val

	gitCfg.GitAccessToken = os.Getenv(EnvGitAccessToken)

	return gitCfg, nil
}
