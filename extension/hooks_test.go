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

package extension

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

func newCluster(name, namespace, class string) *clusterv1.Cluster {
	return &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: clusterv1.ClusterSpec{
			Topology: clusterv1.Topology{
				ClassRef: clusterv1.ClusterClassRef{Name: class},
			},
		},
	}
}

func TestGetHookRecordKey(t *testing.T) {
	t.Parallel()

	got := getHookRecordKey(newCluster("my-cluster", "my-namespace", "openstack-scs2-1-36-v1"))
	want := "my-cluster/my-namespace/openstack-scs2-1-36-v1"

	if got != want {
		t.Fatalf("getHookRecordKey() = %q, want %q", got, want)
	}
}

// A cluster that is upgraded twice in a row (new ClusterClass each time) must get a new hook record key,
// otherwise the second BeforeClusterUpgrade hook is skipped and the cluster addons are not updated.
func TestHookRecordIsPerClusterClass(t *testing.T) {
	t.Parallel()

	h := NewHandler(nil)

	first := getHookRecordKey(newCluster("c", "ns", "openstack-scs2-1-35-v1"))
	second := getHookRecordKey(newCluster("c", "ns", "openstack-scs2-1-36-v1"))

	if first == second {
		t.Fatalf("hook record keys of different cluster classes must differ, both are %q", first)
	}

	h.HookRecord[first] = BeforeClusterUpgradeHook

	if !h.hasAlreadyPatchedClusterAddon(first, BeforeClusterUpgradeHook) {
		t.Fatal("the recorded hook should be reported as already patched")
	}

	if h.hasAlreadyPatchedClusterAddon(second, BeforeClusterUpgradeHook) {
		t.Fatal("the hook for the next cluster class must not be reported as already patched")
	}
}
