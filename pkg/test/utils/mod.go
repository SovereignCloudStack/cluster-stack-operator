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

// Package utils contains important functions for envtest.
package utils

import (
	"fmt"
	"os"

	"golang.org/x/mod/modfile"
)

// Mod contains path and content for the module.
type Mod struct {
	path    string
	content []byte
}

// NewMod returns a new mod.
func NewMod(path string) (Mod, error) {
	var m Mod
	content, err := os.ReadFile(path) //#nosec
	if err != nil {
		return m, fmt.Errorf("failed to read file: %q: %w", path, err)
	}
	return Mod{
		path:    path,
		content: content,
	}, nil
}

// FindDependencyVersion return the version of the given dependency.
func (m Mod) FindDependencyVersion(dependency string) (string, error) {
	f, err := modfile.Parse(m.path, m.content, nil)
	if err != nil {
		return "", fmt.Errorf("failed to parse modfile %q: %w", m.path, err)
	}

	var version string
	for _, entry := range f.Require {
		if entry.Mod.Path == dependency {
			version = entry.Mod.Version
			break
		}
	}
	if version == "" {
		return version, fmt.Errorf("could not find required package: %s", dependency)
	}

	for _, entry := range f.Replace {
		if entry.New.Path == dependency && entry.New.Version != "" {
			version = entry.New.Version
		}
	}
	return version, nil
}
