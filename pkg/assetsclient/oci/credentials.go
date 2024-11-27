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

// Package oci provides utilities for comunicating with the OCI registry.
package oci

import (
	"encoding/base64"
	"fmt"
	"os"
)

const (
	envOCIRegistry    = "OCI_REGISTRY"
	envOCIRepository  = "OCI_REPOSITORY"
	envOCIAccessToken = "OCI_ACCESS_TOKEN"
	envOCIUsername    = "OCI_USERNAME"
	envOCIPassword    = "OCI_PASSWORD"
)

type ociConfig struct {
	registry    string
	repository  string
	accessToken string
	username    string
	password    string
}

func newOCIConfig() (ociConfig, error) {
	var config ociConfig

	val := os.Getenv(envOCIRegistry)
	if val == "" {
		return ociConfig{}, fmt.Errorf("environment variable %s is not set", envOCIRegistry)
	}
	config.registry = val

	val = os.Getenv(envOCIRepository)
	if val == "" {
		return ociConfig{}, fmt.Errorf("environment variable %s is not set", envOCIRepository)
	}
	config.repository = val

	val = os.Getenv(envOCIAccessToken)
	base64AccessToken := base64.StdEncoding.EncodeToString([]byte(val))
	config.accessToken = base64AccessToken

	val = os.Getenv(envOCIUsername)
	config.username = val

	val = os.Getenv(envOCIPassword)
	config.password = val

	return config, nil
}
