/*
Copyright 2026 The Kubernetes Authors.

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

package controller

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("getUsedClusterClasses", func() {
	var (
		nsA, nsB *corev1.Namespace
		clusters []*clusterv1.Cluster
	)

	newCluster := func(name string, ns *corev1.Namespace, class, classNamespace string) *clusterv1.Cluster {
		return &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:       name,
				Namespace:  ns.Name,
				Finalizers: []string{clusterv1.ClusterFinalizer},
			},
			Spec: clusterv1.ClusterSpec{
				Topology: &clusterv1.Topology{
					Class:          class,
					ClassNamespace: classNamespace,
					Version:        testKubernetesVersion,
				},
			},
		}
	}

	BeforeEach(func() {
		var err error
		nsA, err = testEnv.CreateNamespace(ctx, "used-classes-a")
		Expect(err).NotTo(HaveOccurred())
		nsB, err = testEnv.CreateNamespace(ctx, "used-classes-b")
		Expect(err).NotTo(HaveOccurred())

		clusters = []*clusterv1.Cluster{
			// uses the ClusterClass of its own namespace A
			newCluster("local-a", nsA, "docker-ferrol-1-27-v1", ""),
			// lives in A, but uses the ClusterClass of namespace B
			newCluster("cross-a-to-b", nsA, "docker-ferrol-1-27-v2", nsB.Name),
			// lives in B and uses the ClusterClass of its own namespace B
			newCluster("local-b", nsB, "docker-ferrol-1-27-v3", ""),
			// lives in B, but uses the ClusterClass of namespace A
			newCluster("cross-b-to-a", nsB, "docker-ferrol-1-27-v4", nsA.Name),
		}
		for _, cluster := range clusters {
			Expect(testEnv.Create(ctx, cluster)).To(Succeed())
		}
	})

	AfterEach(func() {
		Eventually(func() error {
			objs := []client.Object{nsA, nsB}
			for _, cluster := range clusters {
				objs = append(objs, cluster)
			}
			return testEnv.Cleanup(ctx, objs...)
		}, timeout, interval).Should(Succeed())
	})

	It("takes the classNamespace of the clusters into account", func() {
		Eventually(func() ([]string, error) {
			return getUsedClusterClasses(ctx, testEnv, nsA.Name)
		}, timeout, interval).Should(ConsistOf("docker-ferrol-1-27-v1", "docker-ferrol-1-27-v4"))

		Eventually(func() ([]string, error) {
			return getUsedClusterClasses(ctx, testEnv, nsB.Name)
		}, timeout, interval).Should(ConsistOf("docker-ferrol-1-27-v2", "docker-ferrol-1-27-v3"))
	})
})
