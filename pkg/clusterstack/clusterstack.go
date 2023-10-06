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

// Package clusterstack contains functions related to clusterstacks.
package clusterstack

import (
	"fmt"
	"strings"

	"github.com/SovereignCloudStack/cluster-stack-operator/pkg/kubernetesversion"
	"github.com/SovereignCloudStack/cluster-stack-operator/pkg/version"
)

const (
	// Separator defines the separator for cluster stack strings.
	Separator = "-"
)

const (
	// ClusterStacksDownloadDirectory is the cluster stack download directory.
	ClusterStacksDownloadDirectory = "cluster-stacks"

	// NodeImageDownloadDirectory is the node image untar directory.
	NodeImageDownloadDirectory = "node-images"
)

// ClusterStack contains all properties defining a cluster stack.
type ClusterStack struct {
	Provider          string
	Name              string
	KubernetesVersion kubernetesversion.KubernetesVersion
	Version           version.Version
}

var (
	// ErrInvalidFormat indicates a cluster stack string has an invalid format.
	ErrInvalidFormat = fmt.Errorf("invalid format")
	// ErrInvalidProvider indicates a cluster stack string has an invalid provider.
	ErrInvalidProvider = fmt.Errorf("invalid provider")
	// ErrInvalidName indicates a cluster stack string has an invalid name.
	ErrInvalidName = fmt.Errorf("invalid name")
)

// NewFromString returns a ClusterStack based on a cluster stack string.
func NewFromString(str string) (ClusterStack, error) {
	splitted := strings.Split(str, Separator)
	if len(splitted) != 5 && len(splitted) != 7 {
		return ClusterStack{}, ErrInvalidFormat
	}

	clusterStack := ClusterStack{
		Provider: splitted[0],
		Name:     splitted[1],
	}

	if clusterStack.Provider == "" {
		return ClusterStack{}, ErrInvalidProvider
	}

	if clusterStack.Name == "" {
		return ClusterStack{}, ErrInvalidName
	}

	var err error

	clusterStack.KubernetesVersion, err = kubernetesversion.New(splitted[2], splitted[3])
	if err != nil {
		return ClusterStack{}, fmt.Errorf("failed to create Kubernetes version from %s-%s: %w", splitted[2], splitted[3], err)
	}

	var versionString string
	if len(splitted) == 5 {
		// e.g. myprovider-myclusterstack-1-26-v1
		versionString = splitted[4]
	} else if len(splitted) == 7 {
		// e.g. myprovider-myclusterstack-1-26-v1-alpha-0
		versionString = strings.Join(splitted[4:7], Separator)
	}

	v, err := version.New(versionString)
	if err != nil {
		return ClusterStack{}, fmt.Errorf("failed to create version from %s: %w", versionString, err)
	}

	clusterStack.Version = v

	return clusterStack, nil
}

// New returns a ClusterStack based on a cluster stack string.
func New(provider, name, kubernetesVersion, csVersion string) (ClusterStack, error) {
	k8sVersion, err := kubernetesversion.NewFromString(kubernetesVersion)
	if err != nil {
		return ClusterStack{}, fmt.Errorf("failed to create Kubernetes version from %s: %w", kubernetesVersion, err)
	}

	v, err := version.New(csVersion)
	if err != nil {
		return ClusterStack{}, fmt.Errorf("failed to create version from %s: %w", csVersion, err)
	}

	clusterStack := ClusterStack{
		Provider:          provider,
		Name:              name,
		KubernetesVersion: k8sVersion,
		Version:           v,
	}

	if err := clusterStack.Validate(); err != nil {
		return ClusterStack{}, fmt.Errorf("failed to validate clusterStack: %w", err)
	}

	return clusterStack, nil
}

// Validate validates a given ClusterStack.
func (cs *ClusterStack) Validate() error {
	if cs.Provider == "" {
		return ErrInvalidProvider
	}

	if cs.Name == "" {
		return ErrInvalidName
	}

	if err := cs.Version.Validate(); err != nil {
		return fmt.Errorf("failed to validate version: %w", err)
	}

	if err := cs.KubernetesVersion.Validate(); err != nil {
		return fmt.Errorf("failed to validate Kubernetes version: %w", err)
	}
	return nil
}

func (cs *ClusterStack) String() string {
	// release tag: myprovider-myclusterstack-1-26-v1
	return strings.Join([]string{cs.Provider, cs.Name, cs.KubernetesVersion.String(), cs.Version.String()}, Separator)
}
