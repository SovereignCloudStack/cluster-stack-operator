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

// Package version contains important methods and constants for github release.
package version

// Channel is the release channel of a cluster stack.
type Channel string

const (
	// ChannelStable is the stable channel.
	ChannelStable = Channel("stable")
	// ChannelAlpha is the alpha channel.
	ChannelAlpha = Channel("alpha")
	// ChannelBeta is the beta channel.
	ChannelBeta = Channel("beta")
	// ChannelRC is the rc channel.
	ChannelRC = Channel("rc")
)

// IsValid returns true if the release channel is valid.
func (c Channel) IsValid() bool {
	return c == ChannelStable ||
		c == ChannelAlpha ||
		c == ChannelBeta ||
		c == ChannelRC
}
