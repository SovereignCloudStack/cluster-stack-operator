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

// Package kubernetesversion contains important structs and method for kubernetes version.
package kubernetesversion

import (
	"fmt"
	"strconv"
	"strings"
)

// KubernetesVersion contains major and minor version.
type KubernetesVersion struct {
	Major int
	Minor int
}

var (
	// ErrInvalidFormat is used for invalid format.
	ErrInvalidFormat = fmt.Errorf("invalid format")

	// ErrInvalidMajorVersion is used for invalid major version.
	ErrInvalidMajorVersion = fmt.Errorf("invalid major version")

	// ErrInvalidMinorVersion is used for invalid minor version.
	ErrInvalidMinorVersion = fmt.Errorf("invalid minor version")
)

// New returns a kubernetes version from specified major and minor version.
func New(majorStr, minorStr string) (KubernetesVersion, error) {
	major, err := strconv.Atoi(majorStr)
	if err != nil {
		return KubernetesVersion{}, ErrInvalidMajorVersion
	}

	minor, err := strconv.Atoi(minorStr)
	if err != nil {
		return KubernetesVersion{}, ErrInvalidMinorVersion
	}

	return KubernetesVersion{
		Major: major,
		Minor: minor,
	}, nil
}

// NewFromString returns a kubernetes version from a specified input.
func NewFromString(str string) (KubernetesVersion, error) {
	splitted := strings.Split(str, ".")
	if len(splitted) != 2 {
		return KubernetesVersion{}, ErrInvalidFormat
	}

	major, err := strconv.Atoi(splitted[0])
	if err != nil {
		return KubernetesVersion{}, ErrInvalidMajorVersion
	}

	minor, err := strconv.Atoi(splitted[1])
	if err != nil {
		return KubernetesVersion{}, ErrInvalidMinorVersion
	}

	return KubernetesVersion{
		Major: major,
		Minor: minor,
	}, nil
}

// Validate validates a kubernetes version.
func (kv KubernetesVersion) Validate() error {
	if kv.Major == 0 {
		return ErrInvalidMajorVersion
	}

	if kv.Minor == 0 {
		return ErrInvalidMinorVersion
	}
	return nil
}

func (kv KubernetesVersion) String() string {
	return fmt.Sprintf("%v-%v", kv.Major, kv.Minor)
}

// StringWithDot returns the Kubernetes version in the format <major>.<minor>, e.g. 1.27.
func (kv KubernetesVersion) StringWithDot() string {
	return fmt.Sprintf("%v.%v", kv.Major, kv.Minor)
}
