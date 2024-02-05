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

// Package version contains important structs and methods for cluster stack versions.
package version

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Version encapsulates a Version string
// into its constituent parts as a struct. Sample:
// Version string "v1-alpha-1"
// Major: "v1"
// major: 1 (int, after stripping "v" prefix)
// Channel: "alpha"
// Patch: 1.
type Version struct {
	Major   int
	Channel Channel
	Patch   int
}

// New returns a Version struct from a version string
// Sample allowed inputs: "v1-alpha-1", "v1", "v1-alpha-0"
// Sample disallowed inputs: "v1-alpha", "v1-alpha-1.0", "v1-alpha-1.0.0", "v1-alpha.", "v1.0-alpha.1".
func New(version string) (Version, error) {
	var major, patch int
	var err error
	channel := ChannelStable

	re := regexp.MustCompile(`^v\d+(-\b\w+\b-\d+)?$`)
	match := re.FindStringSubmatch(version)

	if len(match) == 0 {
		return Version{}, fmt.Errorf("invalid version string %s", version)
	}

	// match[0] is the entire string e.g "v1-alpha-1" or "v1"
	// split match[0] with "-" as the delimiter
	ver := strings.Split(match[0], "-")

	// ver[0] is the major version
	// trim the "v" prefix and then convert to int
	if major, err = strconv.Atoi(strings.TrimPrefix(ver[0], "v")); err != nil {
		return Version{}, fmt.Errorf("invalid major version %s", ver[0])
	}

	// If the length of ver is 3, then the version string is of the form "v1-alpha-1"
	// ver[1] is the channel
	// ver[2] is the patch
	if len(ver) == 3 {
		channel = Channel(ver[1])
		if patch, err = strconv.Atoi(ver[2]); err != nil {
			return Version{}, fmt.Errorf("invalid patch value in version %s", ver[2])
		}
	}

	clusterStackVersion := Version{
		Major:   major,
		Channel: channel,
		Patch:   patch,
	}
	if err := clusterStackVersion.Validate(); err != nil {
		return Version{}, err
	}
	return clusterStackVersion, nil
}

// FromReleaseTag returns a Version struct from a release tag string.
func FromReleaseTag(releaseTag string) (Version, error) {
	v := strings.Split(releaseTag, "-")
	if len(v) != 5 && len(v) != 6 {
		return Version{}, fmt.Errorf("invalid release tag %s", releaseTag)
	}
	if len(v) == 5 {
		return New(v[4])
	}
	return New(fmt.Sprintf("%s-%s", v[4], v[5]))
}

// Validate validates the version.
func (csv *Version) Validate() error {
	if csv.Major < 0 {
		return fmt.Errorf("major version should be a non-negative integer")
	}
	if !csv.Channel.IsValid() {
		return fmt.Errorf("invalid channel: %s", csv.Channel)
	}
	if csv.Patch < 0 {
		return fmt.Errorf("patch version should be a non-negative integer")
	}
	return nil
}

// Compare compares two Version structs
// Returns 1 if csv is greater than input
// Returns -1 if csv is less than input
// Returns 0 if csv is equal to input
// Returns error if the two versions are not comparable (different channels).
func (csv Version) Compare(input Version) (int, error) {
	if csv.Channel != input.Channel {
		return 0, fmt.Errorf("cannot compare versions with different channels %s and %s", csv.Channel, input.Channel)
	}

	switch {
	case csv.Major > input.Major:
		return 1, nil
	case csv.Major < input.Major:
		return -1, nil
	case csv.Major == input.Major:
		switch {
		case csv.Patch > input.Patch:
			return 1, nil
		case csv.Patch < input.Patch:
			return -1, nil
		case csv.Patch == input.Patch:
			return 0, nil
		}
	}

	return 0, nil
}

func (csv Version) String() string {
	if csv.Channel == ChannelStable {
		return fmt.Sprintf("v%d", csv.Major)
	}
	return fmt.Sprintf("v%d-%s-%d", csv.Major, csv.Channel, csv.Patch)
}
