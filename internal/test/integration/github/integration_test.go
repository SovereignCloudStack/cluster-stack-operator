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

package github

import (
	csov1alpha1 "github.com/SovereignCloudStack/cluster-stack-operator/api/v1alpha1"
	"github.com/SovereignCloudStack/cluster-stack-operator/pkg/clusterstack"
	"github.com/SovereignCloudStack/cluster-stack-operator/pkg/test/utils"
	csv "github.com/SovereignCloudStack/cluster-stack-operator/pkg/version"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	provider          = "docker"
	name              = "ferrol"
	kubernetesVersion = "1.27"
	version           = "v1"
)

var _ = Describe("ClusterStackReconciler", func() {
	Context("auto subscribe false", func() {
		var (
			clusterStack           *csov1alpha1.ClusterStack
			testNs                 *corev1.Namespace
			clusterStackReleaseKey types.NamespacedName
		)

		BeforeEach(func() {
			var err error
			testNs, err = testEnv.CreateNamespace(ctx, "clusterstack-integration")
			Expect(err).NotTo(HaveOccurred())

			clusterStack = &csov1alpha1.ClusterStack{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test1",
					Namespace: testNs.Name,
				},
				Spec: csov1alpha1.ClusterStackSpec{
					Provider:          provider,
					Name:              name,
					KubernetesVersion: kubernetesVersion,
					Versions:          []string{version},
					AutoSubscribe:     false,
					NoProvider:        true,
				},
			}
			Expect(testEnv.Create(ctx, clusterStack)).To(Succeed())

			cs, err := clusterstack.New(clusterStack.Spec.Provider, clusterStack.Spec.Name, clusterStack.Spec.KubernetesVersion, version)
			Expect(err).To(BeNil())

			clusterStackReleaseKey = types.NamespacedName{Name: cs.String(), Namespace: testNs.Name}
		})

		AfterEach(func() {
			Eventually(func() error {
				return testEnv.Cleanup(ctx, testNs, clusterStack)
			}, timeout, interval).Should(BeNil())
		})

		It("checks if the AssetsClientAPIAvailableCondition condition is true", func() {
			Eventually(func() bool {
				var foundClusterStackRelease csov1alpha1.ClusterStackRelease
				if err := testEnv.Get(ctx, clusterStackReleaseKey, &foundClusterStackRelease); err != nil {
					testEnv.GetLogger().Error(err, "failed to get clusterStackRelease", "key", clusterStackReleaseKey)
					return false
				}

				testEnv.GetLogger().Info("status condition of cluster stack release", "key", clusterStackReleaseKey, "status condition", foundClusterStackRelease.Status.Conditions)
				return utils.IsPresentAndTrue(ctx, testEnv.Client, clusterStackReleaseKey, &foundClusterStackRelease, csov1alpha1.AssetsClientAPIAvailableCondition)
			}, timeout, interval).Should(BeTrue())
		})

		It("creates the cluster stack release object", func() {
			Eventually(func() error {
				var clusterStackRelease csov1alpha1.ClusterStackRelease
				return testEnv.Get(ctx, clusterStackReleaseKey, &clusterStackRelease)
			}, timeout, interval).Should(BeNil())
		})

		It("sets ClusterStackReleaseDownloaded condition once ClusterStackRelease object is created", func() {
			Eventually(func() bool {
				var clusterStackRelease csov1alpha1.ClusterStackRelease
				return utils.IsPresentAndTrue(ctx, testEnv.Client, clusterStackReleaseKey, &clusterStackRelease, csov1alpha1.ClusterStackReleaseAssetsReadyCondition)
			}, timeout, interval).Should(BeTrue())
		})

		It("sets ClusterStackRelease Status ready after ClusterStackRelease object is created", func() {
			Eventually(func() bool {
				var foundClusterStackRelease csov1alpha1.ClusterStackRelease
				if err := testEnv.Get(ctx, clusterStackReleaseKey, &foundClusterStackRelease); err == nil {
					return foundClusterStackRelease.Status.Ready
				}
				return false
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("auto subscribe true", func() {
		var (
			clusterStack           *csov1alpha1.ClusterStack
			testNs                 *corev1.Namespace
			clusterStackKey        types.NamespacedName
			clusterStackReleaseKey types.NamespacedName
		)

		BeforeEach(func() {
			var err error
			testNs, err = testEnv.CreateNamespace(ctx, "clusterstack-integration")
			Expect(err).NotTo(HaveOccurred())

			clusterStack = &csov1alpha1.ClusterStack{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test2",
					Namespace: testNs.Name,
				},
				Spec: csov1alpha1.ClusterStackSpec{
					Provider:          provider,
					Name:              name,
					KubernetesVersion: kubernetesVersion,
					Versions:          []string{},
					AutoSubscribe:     true,
					NoProvider:        true,
					ProviderRef:       nil,
				},
			}
			Expect(testEnv.Create(ctx, clusterStack)).To(Succeed())

			clusterStackKey = types.NamespacedName{Name: clusterStack.Name, Namespace: testNs.Name}

			cs, err := clusterstack.New(clusterStack.Spec.Provider, clusterStack.Spec.Name, clusterStack.Spec.KubernetesVersion, version)
			Expect(err).To(BeNil())

			clusterStackReleaseKey = types.NamespacedName{Name: cs.String(), Namespace: testNs.Name}
		})

		AfterEach(func() {
			Expect(testEnv.Cleanup(ctx, testNs, clusterStack)).To(Succeed())
		})

		It("finds the new cluster stack release", func() {
			Eventually(func() bool {
				var clusterStackReleaseList csov1alpha1.ClusterStackReleaseList
				if err := testEnv.List(ctx, &clusterStackReleaseList, &client.ListOptions{Namespace: testNs.Name}); err != nil {
					testEnv.GetLogger().Error(err, "failed to get clusterStackList")
					return false
				}

				var clusterStack csov1alpha1.ClusterStack
				if err := testEnv.Get(ctx, clusterStackKey, &clusterStack); err != nil {
					testEnv.GetLogger().Error(err, "failed to get clusterStack", "key", clusterStackKey)
					return false
				}

				for _, csrSummary := range clusterStack.Status.Summary {
					v, err := csv.New(csrSummary.Name)
					Expect(err).To(BeNil())

					oldVersion, err := csv.New(version)
					Expect(err).To(BeNil())

					cmp, err := v.Compare(oldVersion)
					Expect(err).To(BeNil())

					if cmp >= 0 {
						return true
					}
				}
				return false
			}, timeout, interval).Should(BeTrue())
		})

		It("creates the new cluster stack release", func() {
			Eventually(func() bool {
				var clusterStackRelease csov1alpha1.ClusterStackRelease
				if err := testEnv.Get(ctx, clusterStackReleaseKey, &clusterStackRelease); err != nil {
					testEnv.GetLogger().Error(err, "failed to get clusterStackRelease", "key", clusterStackReleaseKey)
					return false
				}

				return true
			}, timeout, interval).Should(BeTrue())
		})
	})
})
