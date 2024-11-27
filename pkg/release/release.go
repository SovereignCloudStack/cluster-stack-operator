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
	"github.com/SovereignCloudStack/cluster-stack-operator/pkg/version"
	"gopkg.in/yaml.v3"
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
	// ClusterStackSuffix is the directory name where cluster stacks are there.
	ClusterStackSuffix = "cluster-stacks"

	// ClusterAddonYamlName is the file name where clusteraddon config is there.
	ClusterAddonYamlName = "clusteraddon.yaml"

	metadataFileName = "metadata.yaml"

	// ClusterAddonValuesName constant for the file cluster-addon-values.yaml.
	ClusterAddonValuesName = "cluster-addon-values.yaml"

	// OverwriteYaml is the new cluster stack overwrite yaml.
	OverwriteYaml = "overwrite.yaml"
)

// New returns a new release.
// Error is returned if there is an error while processing.
// But not all errors indicate a need to download the release from github
// so, a bool is returned to indicate if the release needs to be downloaded.
func New(tag, downloadPath string) (Release, bool, error) {
	// downloadPath is the path where the release is downloaded.
	// The path is of the form: <downloadPath>/<clusterStackSuffix>/<tag>/
	// For example: /tmp/downloads/cluster-stacks/docker-ferrol-1-26-v2/
	downloadPath = filepath.Join(downloadPath, ClusterStackSuffix, tag)
	cs, err := clusterstack.NewFromClusterStackReleaseProperties(tag)
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

	metadata, err := ensureMetadata(downloadPath, metadataFileName)
	if err != nil {
		return Release{}, false, fmt.Errorf("failed to get metadata: %w", err)
	}

	rel.Meta = metadata

	// release object is populated, we can validate it now.
	if err := rel.Validate(); err != nil {
		return Release{}, false, fmt.Errorf("failed to validate release: %w", err)
	}

	return rel, false, nil
}

// ConvertFromClusterClassToClusterStackFormat converts `docker-ferrol-1-27-v0-sha.3960147` way to
// `docker-ferrol-1-27-v0-sha-3960147`.
func ConvertFromClusterClassToClusterStackFormat(input string) string {
	parts := strings.Split(input, ".")

	if len(parts) == 2 {
		return fmt.Sprintf("%s-%s", parts[0], parts[1])
	}

	return input
}

func ensureMetadata(downloadPath, metadataFileName string) (Metadata, error) {
	// Read the metadata.yaml file from the release.
	metadataPath := filepath.Join(downloadPath, metadataFileName)
	f, err := os.ReadFile(filepath.Clean(filepath.Join(downloadPath, metadataFileName)))
	if err != nil {
		return Metadata{}, fmt.Errorf("failed to read metadata file %s: %w", metadataPath, err)
	}

	metadata := Metadata{}
	// if unmarshal fails, it indicates incomplete metadata file.
	// But we don't want to enforce download again.
	if err = yaml.Unmarshal(f, &metadata); err != nil {
		return Metadata{}, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	// Normalize the versions of metadata from v1-alpha.1 to v1-alpha-1 format.
	metaClusterStackVersion, err := version.New(metadata.Versions.ClusterStack)
	if err != nil {
		return Metadata{}, fmt.Errorf("failed to parse ClusterStack version from metadata: %w", err)
	}
	metadata.Versions.ClusterStack = metaClusterStackVersion.String()

	metaClusterAddonVersion, err := version.New(metadata.Versions.Components.ClusterAddon)
	if err != nil {
		return Metadata{}, fmt.Errorf("failed to parse ClusterAddon version from metadata: %w", err)
	}
	metadata.Versions.Components.ClusterAddon = metaClusterAddonVersion.String()

	metaNodeImageVersion, err := version.New(metadata.Versions.Components.NodeImage)
	if err != nil {
		return Metadata{}, fmt.Errorf("failed to parse NodeImage version from metadata: %w", err)
	}
	metadata.Versions.Components.NodeImage = metaNodeImageVersion.String()

	return metadata, nil
}

// CheckHelmCharts checks all expected helm charts in the release directory.
// This is a separate method, since few controllers need to check for the
// presence of helm charts and few don't.
func (r *Release) CheckHelmCharts() error {
	// check if the cluster class chart is present.
	clusterClassChartName := r.clusterClassChartName()
	clusterClassChartPath, err := r.helmChartNamePath(clusterClassChartName)
	if err != nil {
		return fmt.Errorf("failed to get cluster class chart path: %w", err)
	}
	if _, err := os.Stat(clusterClassChartPath); err != nil {
		return fmt.Errorf("failed to verify the cluster addon chart path %s with error: %w", clusterClassChartPath, err)
	}

	clusterAddonPath := filepath.Join(r.LocalDownloadPath, ClusterAddonYamlName)
	if _, err := os.Stat(clusterAddonPath); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to verify the clusteraddon.yaml path %s with error: %w", clusterAddonPath, err)
		}

		// check if the cluster addon values file is present.
		valuesPath := filepath.Join(r.LocalDownloadPath, ClusterAddonValuesName)
		if _, err := os.Stat(valuesPath); err != nil {
			return fmt.Errorf("failed to verify the cluster addon values path %s with error: %w", valuesPath, err)
		}
	}

	// check if the cluster addon chart is present.
	clusterAddonChartName := r.clusterAddonChartName()
	clusterAddonChartPath, err := r.helmChartNamePath(clusterAddonChartName)
	if err != nil {
		return fmt.Errorf("failed to get cluster addon chart path: %w", err)
	}
	if _, err := os.Stat(clusterAddonChartPath); err != nil {
		return fmt.Errorf("failed to verify the cluster class chart path %s with error: %w", clusterAddonChartPath, err)
	}

	return nil
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
	clusterAddonVersion, _ := version.ParseVersionString(r.Meta.Versions.Components.ClusterAddon)
	return fmt.Sprintf("%s-%s-%s-cluster-addon-%s", r.ClusterStack.Provider, r.ClusterStack.Name, r.ClusterStack.KubernetesVersion, clusterAddonVersion.StringWithDot())
}

// ClusterAddonChartPath returns the helm chart name from the given path.
func (r *Release) ClusterAddonChartPath() string {
	// we ignore the error here, since we already checked for the presence of the chart.
	name := r.clusterAddonChartName()
	path, _ := r.helmChartNamePath(name)
	return path
}

// ClusterAddonValuesPath returns the path to the cluster addon values file.
func (r *Release) ClusterAddonValuesPath() string {
	return filepath.Join(r.LocalDownloadPath, ClusterAddonValuesName)
}

// clusterClassChartName returns the helm chart name for cluster class.
func (r *Release) clusterClassChartName() string {
	return fmt.Sprintf("%s-%s-%s-cluster-class-%s", r.ClusterStack.Provider, r.ClusterStack.Name, r.ClusterStack.KubernetesVersion, r.ClusterStack.Version.StringWithDot())
}

// ClusterClassChartPath returns the absolute helm chart path for cluster class.
func (r *Release) ClusterClassChartPath() string {
	nameFilter := r.clusterClassChartName()
	// we ignore the error here, since we already checked for the presence of the chart.
	path, _ := r.helmChartNamePath(nameFilter)
	return path
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
