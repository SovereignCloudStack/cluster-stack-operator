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

// Package cluster addon contains function for cluster addon config operations.
package clusteraddon

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Action is the type for helm chart action (e.g. - apply, delete).
type Action string

// ConditionNotMatchError is used when the specified CEL expression doesn't match in the stage.
var ConditionNotMatchError = errors.New("condition don't match")

var (
	// Apply applies a helm chart.
	Apply = Action("apply")

	// Delete deletes a helm chart.
	Delete = Action("delete")
)

type Object struct {
	Key        string `yaml:"key"`
	APIVersion string `yaml:"apiVersion"`
	Name       string `yaml:"name"`
	Kind       string `yaml:"kind"`
	Namespace  string `yaml:"namespace"`
}

type WaitForCondition struct {
	Timeout    time.Duration `yaml:"timeout"`
	Objects    []Object      `yaml:"objects"`
	Conditions string        `yaml:"conditions"`
}

type Stage struct {
	HelmChartName        string           `yaml:"helmChartName"`
	Action               Action           `yaml:"action"`
	WaitForPreCondition  WaitForCondition `yaml:"waitForPreCondition,omitempty"`
	WaitForPostCondition WaitForCondition `yaml:"waitForPostCondition,omitempty"`
}

type ClusterAddonConfig struct {
	APIVersion          string             `yaml:"apiVersion"`
	ClusterAddonVersion string             `yaml:"clusterAddonVersion"`
	AddonStages         map[string][]Stage `yaml:"addonStages"`
}

func ParseClusterAddonConfig(path string) (ClusterAddonConfig, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return ClusterAddonConfig{}, fmt.Errorf("failed to read file: %q: %w", path, err)
	}

	var clusterAddon ClusterAddonConfig
	if err := yaml.Unmarshal(data, &clusterAddon); err != nil {
		return ClusterAddonConfig{}, fmt.Errorf("failed to parse cluster addon yaml: %w", err)
	}

	return clusterAddon, nil
}
