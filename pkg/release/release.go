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

// Package release contains important structs and methods for a cluster stack release.
package release

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/SovereignCloudStack/cluster-stack-operator/pkg/clusterstack"
	"gopkg.in/yaml.v2"
)

// Release contains information for ClusterStack release.
type Release struct {
	Tag               string                    `yaml:"name"`
	Meta              Metadata                  `yaml:"metadata"`
	ClusterStack      clusterstack.ClusterStack `yaml:"clusterStack"`
	LocalDownloadPath string                    `yaml:"downloadPath"`
}

var (
	// ErrEmptyReleaseName indicates release name is not provided.
	ErrEmptyReleaseName = fmt.Errorf("name is empty")
	// ErrEmptyReleaseCSR indicates cluster stack is not provided.
	ErrEmptyReleaseCSR = fmt.Errorf("cluster stack is empty")
	// ErrEmptyReleaseDownloadPath indicates download path is not provided.
	ErrEmptyReleaseDownloadPath = fmt.Errorf("local download path is empty")
)

const (
	clusterStackSuffix     = "cluster-stacks"
	metadataFileName       = "metadata.yaml"
	clusterAddonValuesName = "cluster-addon-values.yaml"
)

// New returns a new release.
// Error is returned if there is an error while processing.
// But not all errors indicate a need to download the release from github
// so, a bool is returned to indicate if the release needs to be downloaded.
func New(tag, downloadPath string) (Release, bool, error) {
	// downloadPath is the path where the release is downloaded.
	// The path is of the form: <downloadPath>/<clusterStackSuffix>/<tag>/
	downloadPath = filepath.Join(downloadPath, clusterStackSuffix, tag)
	cs, err := clusterstack.NewFromString(tag)
	if err != nil {
		return Release{}, false, fmt.Errorf("failed to parse cluster stack release: %w", err)
	}

	rel := Release{
		Tag:               tag,
		ClusterStack:      cs,
		LocalDownloadPath: downloadPath,
	}

	// Check if the release path is present.
	if _, err = os.Stat(downloadPath); err != nil {
		if os.IsNotExist(err) {
			return rel, true, nil
		}
		return Release{}, false, fmt.Errorf("couldn't verify the download path %s with error: %w", downloadPath, err)
	}

	// Read the metadata.yaml file from the release.
	metadataPath := filepath.Join(downloadPath, metadataFileName)
	f, err := os.ReadFile(filepath.Clean(filepath.Join(downloadPath, metadataFileName)))
	if err != nil {
		return Release{}, false, fmt.Errorf("failed to read metadata file %s: %w", metadataPath, err)
	}

	metadata := Metadata{}
	// if unmarshal fails, it indicates incomplete metadata file.
	// But we don't want to enforce download again.
	if err = yaml.Unmarshal(f, &metadata); err != nil {
		return Release{}, false, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	rel.Meta = metadata

	if err := rel.Validate(); err != nil {
		return Release{}, false, fmt.Errorf("failed to validate release: %w", err)
	}

	return rel, false, nil
}

// Validate validates the release.
func (r *Release) Validate() error {
	if r.Tag == "" {
		return ErrEmptyReleaseName
	}
	if r.LocalDownloadPath == "" {
		return ErrEmptyReleaseDownloadPath
	}
	if r.ClusterStack == (clusterstack.ClusterStack{}) {
		return ErrEmptyReleaseCSR
	}
	if err := r.ClusterStack.Version.Validate(); err != nil {
		return fmt.Errorf("failed to validate cluster stack object: %w", err)
	}
	if err := r.Meta.Validate(); err != nil {
		return fmt.Errorf("failed to validate metadata: %w", err)
	}
	return nil
}

// clusterAddonChartName returns the helm chart name for cluster addon.
func (r *Release) clusterAddonChartName() string {
	return fmt.Sprintf("%s-%s-%s-cluster-addon-%s", r.ClusterStack.Provider, r.ClusterStack.Name, r.ClusterStack.KubernetesVersion, r.Meta.Versions.Components.ClusterAddon)
}

// ClusterAddonChartPath returns the helm chart name from the given path.
func (r *Release) ClusterAddonChartPath() (string, error) {
	return r.helmChartNamePath(r.clusterAddonChartName())
}

// ClusterAddonValuesPath returns the path to the cluster addon values file.
func (r *Release) ClusterAddonValuesPath() string {
	return filepath.Join(r.LocalDownloadPath, clusterAddonValuesName)
}

// clusterClassChartName returns the helm chart name for cluster class.
func (r *Release) clusterClassChartName() string {
	return fmt.Sprintf("%s-%s-%s-cluster-class-%s", r.ClusterStack.Provider, r.ClusterStack.Name, r.ClusterStack.KubernetesVersion, r.ClusterStack.Version.String())
}

// ClusterClassChartPath returns the absolute helm chart path for cluster class.
func (r *Release) ClusterClassChartPath() (string, error) {
	nameFilter := r.clusterClassChartName()
	return r.helmChartNamePath(nameFilter)
}

// helmChartNamePath returns the helm chart name from the given path.
func (r *Release) helmChartNamePath(nameFilter string) (string, error) {
	files, err := os.ReadDir(r.LocalDownloadPath)
	if err != nil {
		return "", fmt.Errorf("failed to read directory %s: %w", r.LocalDownloadPath, err)
	}
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		if nameFilter != "" && strings.Contains(file.Name(), nameFilter) {
			// This assumes that helm charts are the only files with .tgz extension in the given path
			if filepath.Ext(file.Name()) == ".tgz" {
				return filepath.Join(r.LocalDownloadPath, file.Name()), nil
			}
		}
	}
	return "", fmt.Errorf("helm chart matching the name filter %s not found in %s", nameFilter, r.LocalDownloadPath)
}
