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
	"context"
	"errors"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// fakeReader answers Get with the given clusters, one per call, and counts the calls.
type fakeReader struct {
	clusters []*clusterv1.Cluster
	err      error
	gets     int
}

func (f *fakeReader) Get(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
	f.gets++

	if f.err != nil {
		return f.err
	}

	cluster, ok := obj.(*clusterv1.Cluster)
	if !ok {
		return errors.New("unexpected object type")
	}

	i := f.gets - 1
	if i >= len(f.clusters) {
		i = len(f.clusters) - 1
	}

	f.clusters[i].DeepCopyInto(cluster)

	return nil
}

func (f *fakeReader) List(context.Context, client.ObjectList, ...client.ListOption) error {
	return errors.New("not implemented")
}

// cachedClient must not be used to determine the class: its cache can be stale when the hook is called.
type cachedClient struct {
	client.Client
}

func (cachedClient) Get(context.Context, client.ObjectKey, client.Object, ...client.GetOption) error {
	panic("the hook record key must not be read from the cache")
}

func newCluster(class string) *clusterv1.Cluster {
	return &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "my-cluster", Namespace: "my-namespace"},
		Spec:       clusterv1.ClusterSpec{Topology: &clusterv1.Topology{Class: class}},
	}
}

func TestGetHookRecordKeyWithClassInRequest(t *testing.T) {
	t.Parallel()

	// Cluster API 1.11 and 1.12 send a v1beta1 Cluster, the class is part of the request.
	fc := &fakeReader{}
	h := NewHandler(cachedClient{}, fc)

	got := h.getHookRecordKey(context.Background(), newCluster("openstack-scs2-1-36-v1"))
	want := "my-cluster/my-namespace/openstack-scs2-1-36-v1"

	if got != want {
		t.Fatalf("getHookRecordKey() = %q, want %q", got, want)
	}

	if fc.gets != 0 {
		t.Fatalf("the cluster must not be read from the API if the request has the class, got %d calls", fc.gets)
	}
}

func TestGetHookRecordKeyWithoutClassInRequest(t *testing.T) {
	t.Parallel()

	// Cluster API >= 1.13 sends a v1beta2 Cluster: after decoding as v1beta1 the class is empty.
	fc := &fakeReader{clusters: []*clusterv1.Cluster{newCluster("openstack-scs2-1-36-v1")}}
	h := NewHandler(cachedClient{}, fc)

	got := h.getHookRecordKey(context.Background(), newCluster(""))
	want := "my-cluster/my-namespace/openstack-scs2-1-36-v1"

	if got != want {
		t.Fatalf("getHookRecordKey() = %q, want %q", got, want)
	}

	if fc.gets != 1 {
		t.Fatalf("expected one read of the cluster, got %d", fc.gets)
	}
}

func TestGetHookRecordKeyWithoutTopologyOrAPI(t *testing.T) {
	t.Parallel()

	noTopology := &clusterv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "my-cluster", Namespace: "my-namespace"}}

	for name, fc := range map[string]*fakeReader{
		"api returns an error":           {err: errors.New("not found")},
		"cluster in api has no topology": {clusters: []*clusterv1.Cluster{noTopology}},
	} {
		got := NewHandler(cachedClient{}, fc).getHookRecordKey(context.Background(), noTopology)
		if want := "my-cluster/my-namespace/"; got != want {
			t.Fatalf("%s: getHookRecordKey() = %q, want %q", name, got, want)
		}
	}
}

// A cluster that is upgraded twice in a row (new ClusterClass each time) must get a new hook record key,
// otherwise the second BeforeClusterUpgrade hook is skipped and the cluster addons are not updated.
func TestHookRecordIsPerClusterClass(t *testing.T) {
	t.Parallel()

	fc := &fakeReader{clusters: []*clusterv1.Cluster{
		newCluster("openstack-scs2-1-35-v1"),
		newCluster("openstack-scs2-1-36-v1"),
	}}
	h := NewHandler(cachedClient{}, fc)

	first := h.getHookRecordKey(context.Background(), newCluster(""))
	second := h.getHookRecordKey(context.Background(), newCluster(""))

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
