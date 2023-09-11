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

// Len returns the length of the slice.
func (r ClusterStacks) Len() int {
	return len(r)
}

// Swap swaps the elements with indexes i and j.
func (r ClusterStacks) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}

// Less reports whether the element with index i should sort before the element with index j.
func (r ClusterStacks) Less(i, j int) bool {
	cmp, err := r[i].Version.Compare(r[j].Version)
	if err != nil {
		// We can't compare due to whatever reason. Return false as default
		return false
	}
	if cmp == -1 {
		// This is the case where value at index i is less than index j.
		// so index i should sort before the element with index j.
		// hence true
		return true
	}
	return false
}
