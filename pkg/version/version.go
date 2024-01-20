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
	Patch   string
}

// ParseVersionString returns a Version struct from a version string like -
// "v1", "v1-alpha-1", "v1-beta-3", etc.
func ParseVersionString(version string) (Version, error) {
	var (
		major int
		patch string
		err   error
	)
	channel := ChannelStable

	re := regexp.MustCompile(`^v\d+(-\b\w+\b\-\w+)?$`)
	match := re.MatchString(version)
	if !match {
		return Version{}, fmt.Errorf("invalid version string %s", version)
	}

	// match[0] is the entire string e.g "v1-alpha-1" or "v1"
	// split match[0] with "-" as the delimiter
	ver := strings.Split(version, "-")

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
		patch = ver[2]
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

// New returns a Version struct from a version string
// Sample allowed inputs: "v1-alpha.1", "v1", "v1-alpha.0"
// Sample disallowed inputs: "v1-alpha", "v1-alpha-1.0", "v1-alpha-1.0.0", "v1-alpha.", "v1.0-alpha.1".
func New(version string) (Version, error) {
	var (
		major int
		patch string
		err   error
	)
	channel := ChannelStable

	re := regexp.MustCompile(`^v\d+(-\b\w+\b\.\w+)?$`)
	match := re.MatchString(version)
	if !match {
		return Version{}, fmt.Errorf("invalid version string %s", version)
	}

	// match[0] is the entire string e.g "v1-alpha.1" or "v1"
	// split match[0] with "-" as the delimiter
	ver := strings.Split(version, "-")

	// ver[0] is the major version
	// trim the "v" prefix and then convert to int
	if major, err = strconv.Atoi(strings.TrimPrefix(ver[0], "v")); err != nil {
		return Version{}, fmt.Errorf("invalid major version %s", ver[0])
	}

	// If the length of ver is 2, then the version string is of the form "v1-alpha.1", and split it -
	// ver[0] is the channel
	// ver[1] is the patch
	if len(ver) == 2 {
		splittedChannelPatch := strings.Split(ver[1], ".")
		if len(splittedChannelPatch) != 2 {
			return Version{}, fmt.Errorf("invalid version string %s", version)
		}
		channel = Channel(splittedChannelPatch[0])
		patch = splittedChannelPatch[1]
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
	if len(v) != 5 && len(v) != 7 {
		return Version{}, fmt.Errorf("invalid release tag %s", releaseTag)
	}
	// for docker-ferrol-1-26-v1 type tag, v[4] is the version
	if len(v) == 5 {
		return ParseVersionString(v[4])
	}
	// for docker-ferrol-1-26-v1-alpha-0 type tag, v[4] is the version and v[5] is the release channel + patch version
	return ParseVersionString(fmt.Sprintf("%s-%s-%s", v[4], v[5], v[6]))
}

// Validate validates the version.
func (csv *Version) Validate() error {
	if csv.Major < 0 {
		return fmt.Errorf("major version should be a non-negative integer")
	}

	if csv.Channel != ChannelStable {
		// Check if the patch is a valid integer
		if isInteger(csv.Patch) {
			// If it's an integer, check if it's greater than 0
			patchInt, _ := strconv.Atoi(csv.Patch)
			if patchInt < 0 {
				return fmt.Errorf("patch version should be a non-negative integer")
			}
		}

		// If it's alpha numeric, check if it's empty
		if csv.Patch == "" {
			return fmt.Errorf("patch can't empty")
		}
	}

	return nil
}

// isInteger checks if the given string is a valid integer.
func isInteger(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
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

// String converts a Version struct to a string representation.
// If the channel is stable, it returns the version in the format "vMajor".
// Otherwise, it returns the version in the format "vMajor-Channel-Patch".
func (csv Version) String() string {
	if csv.Channel == ChannelStable {
		return fmt.Sprintf("v%d", csv.Major)
	}
	return fmt.Sprintf("v%d-%s-%s", csv.Major, csv.Channel, csv.Patch)
}

// StringWithDot converts a Version struct to a string representation.
// If the channel is stable, it returns the version in the format "vMajor".
// Otherwise, it returns the version in the format "vMajor-Channel.Patch",
// similar to String but with a dot separating channel and patch.
func (csv Version) StringWithDot() string {
	if csv.Channel == ChannelStable {
		return fmt.Sprintf("v%d", csv.Major)
	}
	return fmt.Sprintf("v%d-%s.%s", csv.Major, csv.Channel, csv.Patch)
}
