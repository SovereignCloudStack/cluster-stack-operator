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

package release

import (
	"fmt"
)

// ErrEmptyVersion indicates that the version is empty.
var ErrEmptyVersion = fmt.Errorf("empty version")

// Metadata is the metadata for cluster stacks.
type Metadata struct {
	Versions Versions `yaml:"versions"`
}

// Versions is the cluster stack versions.
type Versions struct {
	ClusterStack string     `yaml:"clusterStack"`
	Kubernetes   string     `yaml:"kubernetes"`
	Components   Components `yaml:"components"`
}

// Components is the cluster stack components.
type Components struct {
	ClusterAddon string `yaml:"clusterAddon"`
	NodeImage    string `yaml:"nodeImage"`
}

// Validate validates metadata.
func (m Metadata) Validate() error {
	if m.Versions.ClusterStack == "" {
		return fmt.Errorf("clusterstack version empty: %w", ErrEmptyVersion)
	}

	if m.Versions.Kubernetes == "" {
		return fmt.Errorf("kubernetes version empty: %w", ErrEmptyVersion)
	}

	if m.Versions.Components.ClusterAddon == "" {
		return fmt.Errorf("cluster addon version empty: %w", ErrEmptyVersion)
	}

	return nil
}
