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

package clusterstack

// ClusterStacks implements sort.Interface for []ClusterStack based on the release name / tag.
type ClusterStacks []ClusterStack

// Contains return if releases slice contains the given name or not.
func (r ClusterStacks) Contains(name string) bool {
	for _, release := range r {
		if release.String() == name {
			return true
		}
	}
	return false
}

// Latest returns the latest release from the slice.
// If the slice is empty, it returns nil.
// The slice is expected to be sorted.
func (r ClusterStacks) Latest() *ClusterStack {
	if len(r) > 0 {
		return &r[len(r)-1]
	}
	return nil
}
