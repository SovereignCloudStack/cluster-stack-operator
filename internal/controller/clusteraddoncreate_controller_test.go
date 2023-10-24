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

package controller

import (
	"fmt"

	csov1alpha1 "github.com/SovereignCloudStack/cluster-stack-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

var _ = Describe("ClusterAddonCreateReconciler", func() {
	var (
		cluster             *clusterv1.Cluster
		clusterStackRelease *csov1alpha1.ClusterStackRelease
		testNs              *corev1.Namespace
		key                 types.NamespacedName
	)

	BeforeEach(func() {
		var err error
		testNs, err = testEnv.CreateNamespace(ctx, "clusteraddoncreate-reconciler")
		Expect(err).NotTo(HaveOccurred())

		clusterStackRelease = &csov1alpha1.ClusterStackRelease{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testClusterStackName,
				Namespace: testNs.Name,
			},
		}
		Expect(testEnv.Create(ctx, clusterStackRelease)).To(Succeed())

		cluster = &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "testcluster",
				Namespace:  testNs.Name,
				Finalizers: []string{clusterv1.ClusterFinalizer},
			},
			Spec: clusterv1.ClusterSpec{
				Topology: &clusterv1.Topology{
					Class: testClusterStackName,
				},
			},
		}
		Expect(testEnv.Create(ctx, cluster)).To(Succeed())

		key = types.NamespacedName{Name: fmt.Sprintf("cluster-addon-%s", cluster.Name), Namespace: testNs.Name}
	})

	AfterEach(func() {
		Expect(testEnv.Cleanup(ctx, testNs, cluster, clusterStackRelease)).To(Succeed())
	})

	Context("Basic test", func() {
		It("creates the clusterAddon object", func() {
			Eventually(func() bool {
				var foundClusterAddon csov1alpha1.ClusterAddon
				if err := testEnv.Get(ctx, key, &foundClusterAddon); err != nil {
					testEnv.GetLogger().Info(err.Error())
					return false
				}

				if foundClusterAddon.Spec.ClusterRef.Name != cluster.Name {
					testEnv.GetLogger().Info("wrong cluster ref name", "got", foundClusterAddon.Spec.ClusterRef.Name, "want", cluster.Name)
					return false
				}

				return true
			}, timeout, interval).Should(BeTrue())
		})
	})
})
