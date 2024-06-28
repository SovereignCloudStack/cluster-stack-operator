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

// Package clusteraddon contains function for cluster addon config operations.
package clusteraddon

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Action is the type for helm chart action (e.g. - apply, delete).
type Action string

// ErrConditionNotMatch is used when the specified CEL expression doesn't match in the stage.
var ErrConditionNotMatch = errors.New("condition don't match")

var (
	// Apply applies a helm chart.
	Apply = Action("apply")

	// Delete deletes a helm chart.
	Delete = Action("delete")
)

// Object is a representation of a Kubernetes object plus a key for evaluating CEL expressions with that object.
type Object struct {
	Key        string `yaml:"key"`
	APIVersion string `yaml:"apiVersion"`
	Name       string `yaml:"name"`
	Kind       string `yaml:"kind"`
	Namespace  string `yaml:"namespace"`
}

// WaitForCondition contains objects and conditions to use in CEL expressions.
type WaitForCondition struct {
	Objects    []Object `yaml:"objects"`
	Conditions string   `yaml:"conditions"`
}

// Stage is a stage of a hook in which a certain Helm chart is applied and pre- and post-conditions are evaluated if they exist.
type Stage struct {
	Name                 string           `yaml:"name"`
	Action               Action           `yaml:"action"`
	WaitForPreCondition  WaitForCondition `yaml:"waitForPreCondition,omitempty"`
	WaitForPostCondition WaitForCondition `yaml:"waitForPostCondition,omitempty"`
}

// Config is a configuration for multi-stage cluster addons.
type Config struct {
	APIVersion          string              `yaml:"apiVersion"`
	ClusterAddonVersion string              `yaml:"clusterAddonVersion"`
	AddonStages         map[string][]*Stage `yaml:"addonStages"`
}

// ParseConfig parses a file whose path is given and returns a config struct.
func ParseConfig(path string) (Config, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return Config{}, fmt.Errorf("failed to read file: %q: %w", path, err)
	}

	var clusterAddon Config
	if err := yaml.Unmarshal(data, &clusterAddon); err != nil {
		return Config{}, fmt.Errorf("failed to parse cluster addon yaml: %w", err)
	}

	return clusterAddon, nil
}
