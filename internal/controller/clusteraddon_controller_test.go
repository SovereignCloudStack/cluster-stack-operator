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
	csov1alpha1 "github.com/SovereignCloudStack/cluster-stack-operator/api/v1alpha1"
	"github.com/SovereignCloudStack/cluster-stack-operator/pkg/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
)

var _ = Describe("ClusterAddonReconciler", func() {
	var (
		cluster             *clusterv1.Cluster
		clusterStackRelease *csov1alpha1.ClusterStackRelease

		testNs *corev1.Namespace
		key    types.NamespacedName
	)

	const clusterAddonName = "cluster-addon-testcluster"

	BeforeEach(func() {
		var err error
		testNs, err = testEnv.CreateNamespace(ctx, "clusteraddon-reconciler")
		Expect(err).NotTo(HaveOccurred())

		key = types.NamespacedName{Name: clusterAddonName, Namespace: testNs.Name}

		cluster = &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "testcluster",
				Namespace: testNs.Name,
			},
			Spec: clusterv1.ClusterSpec{
				Topology: &clusterv1.Topology{
					Class:   testClusterStackName,
					Version: testKubernetesVersion,
				},
			},
		}

		testEnv.KubeClient.On("Apply", mock.Anything, mock.Anything, mock.Anything).Return([]*csov1alpha1.Resource{}, false, nil)
	})

	AfterEach(func() {
		Eventually(func() error {
			return testEnv.Cleanup(ctx, testNs, cluster, clusterStackRelease)
		}, timeout, interval).Should(BeNil())
	})

	Context("Basic test", func() {
		var clusterStackReleaseKey types.NamespacedName

		BeforeEach(func() {
			clusterStackReleaseKey = types.NamespacedName{Name: testClusterStackName, Namespace: testNs.Name}

			clusterStackRelease = &csov1alpha1.ClusterStackRelease{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testClusterStackName,
					Namespace: testNs.Name,
				},
			}
			Expect(testEnv.Create(ctx, clusterStackRelease)).To(Succeed())

			Eventually(func() bool {
				var foundClusterStackRelease csov1alpha1.ClusterStackRelease
				if err := testEnv.Get(ctx, clusterStackReleaseKey, &foundClusterStackRelease); err != nil {
					testEnv.GetLogger().Info(err.Error())
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())
		})

		It("creates the clusterAddon object", func() {
			Expect(testEnv.Create(ctx, cluster)).To(Succeed())

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

		It("sets ClusterReady condition if cluster has ControlPlaneReadyCondition", func() {
			Expect(testEnv.Create(ctx, cluster)).To(Succeed())

			ph, err := patch.NewHelper(cluster, testEnv)
			Expect(err).ShouldNot(HaveOccurred())

			conditions.MarkTrue(cluster, clusterv1.ControlPlaneReadyCondition)

			Eventually(func() bool {
				if err := ph.Patch(ctx, cluster); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

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

				return utils.IsPresentAndTrue(ctx, testEnv.Client, key, &foundClusterAddon, csov1alpha1.ClusterReadyCondition)
			}, timeout, interval).Should(BeTrue())
		})

		It("updates the clusteraddon helm chart objects if cluster switches to new cluster stack with new clusteraddon version", func() {
			Expect(testEnv.Create(ctx, cluster)).To(Succeed())

			ph, err := patch.NewHelper(cluster, testEnv)
			Expect(err).ShouldNot(HaveOccurred())

			conditions.MarkTrue(cluster, clusterv1.ControlPlaneReadyCondition)

			Eventually(func() bool {
				if err := ph.Patch(ctx, cluster); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By("updating the clusterclass in the cluster")

			ph, err = patch.NewHelper(cluster, testEnv)
			Expect(err).ShouldNot(HaveOccurred())

			cluster.Spec.Topology.Class = testClusterStackNameV2

			Eventually(func() bool {
				if err := ph.Patch(ctx, cluster); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By("ensuring that the spec of clusteraddon is updated and status still synced")
			Eventually(func() bool {
				var foundClusterAddon csov1alpha1.ClusterAddon
				if err := testEnv.Get(ctx, key, &foundClusterAddon); err != nil {
					testEnv.GetLogger().Info(err.Error())
					return false
				}

				if foundClusterAddon.Spec.ClusterStack != testClusterStackNameV2 {
					testEnv.GetLogger().Info("found wrong cluster stack", "want", testClusterStackNameV2, "got", foundClusterAddon.Spec.ClusterStack)
					return false
				}

				if foundClusterAddon.Spec.Version != "v2" {
					testEnv.GetLogger().Info("found wrong cluster addon version", "want", "v2", "got", foundClusterAddon.Spec.Version)
					return false
				}

				return true
			}, timeout, interval).Should(BeTrue())
		})

		It("should call update if the cluster addon version changes in the ClusterClass update", func() {
			Expect(testEnv.Create(ctx, cluster)).To(Succeed())

			By("making the control plane ready")
			ph, err := patch.NewHelper(cluster, testEnv)
			Expect(err).ShouldNot(HaveOccurred())

			conditions.MarkTrue(cluster, clusterv1.ControlPlaneReadyCondition)

			Eventually(func() error {
				return ph.Patch(ctx, cluster)
			}, timeout, interval).Should(BeNil())

			By("ensuring helm chart is applied")
			Eventually(func() bool {
				var foundClusterAddon csov1alpha1.ClusterAddon
				if err := testEnv.Get(ctx, key, &foundClusterAddon); err != nil {
					testEnv.GetLogger().Info(err.Error())
					return false
				}

				return utils.IsPresentAndTrue(ctx, testEnv.GetClient(), key, &foundClusterAddon, csov1alpha1.HelmChartAppliedCondition)
			}, timeout, interval).Should(BeTrue())

			By("updating the cluster class")
			ph, err = patch.NewHelper(cluster, testEnv)
			Expect(err).ShouldNot(HaveOccurred())

			cluster.Spec.Topology.Class = testClusterStackNameV2

			Eventually(func() error {
				return ph.Patch(ctx, cluster)
			}, timeout, interval).Should(BeNil())

			By("ensuring cluster addon and cluster stack version is updated")
			Eventually(func() bool {
				var foundClusterAddon csov1alpha1.ClusterAddon
				if err := testEnv.Get(ctx, key, &foundClusterAddon); err != nil {
					testEnv.GetLogger().Info(err.Error())
					return false
				}

				return foundClusterAddon.Spec.Version == "v2" && foundClusterAddon.Spec.ClusterStack == "docker-ferrol-1-27-v2"
			}, timeout, interval).Should(BeTrue())

			By("checking Update method was called")
			Expect(testEnv.KubeClient.AssertCalled(GinkgoT(), "Apply", mock.Anything, mock.Anything, mock.Anything)).To(BeTrue())
		})

		It("should not call update if the ClusterAddon version does not change in the ClusterClass update", func() {
			cluster.Spec.Topology.Class = testClusterStackNameV2
			Expect(testEnv.Create(ctx, cluster)).To(Succeed())

			By("making the control plane ready")
			ph, err := patch.NewHelper(cluster, testEnv)
			Expect(err).ShouldNot(HaveOccurred())

			conditions.MarkTrue(cluster, clusterv1.ControlPlaneReadyCondition)

			Eventually(func() error {
				return ph.Patch(ctx, cluster)
			}, timeout, interval).Should(BeNil())

			By("ensuring helm chart is applied")
			Eventually(func() bool {
				var foundClusterAddon csov1alpha1.ClusterAddon
				if err := testEnv.Get(ctx, key, &foundClusterAddon); err != nil {
					testEnv.GetLogger().Info(err.Error())
					return false
				}

				return utils.IsPresentAndTrue(ctx, testEnv.GetClient(), key, &foundClusterAddon, csov1alpha1.HelmChartAppliedCondition)
			}, timeout, interval).Should(BeTrue())

			By("updating the cluster class")
			ph, err = patch.NewHelper(cluster, testEnv)
			Expect(err).ShouldNot(HaveOccurred())

			cluster.Spec.Topology.Class = testClusterStackNameV3

			Eventually(func() error {
				return ph.Patch(ctx, cluster)
			}, timeout, interval).Should(BeNil())

			By("ensuring cluster addon and cluster stack version is updated")
			Eventually(func() bool {
				var foundClusterAddon csov1alpha1.ClusterAddon
				if err := testEnv.Get(ctx, key, &foundClusterAddon); err != nil {
					testEnv.GetLogger().Info(err.Error())
					return false
				}

				return foundClusterAddon.Spec.Version == "v2" && foundClusterAddon.Spec.ClusterStack == "docker-ferrol-1-27-v3"
			}, timeout, interval).Should(BeTrue())

			By("checking Update method was not called")
			Expect(testEnv.KubeClient.AssertNotCalled(GinkgoT(), "Apply")).To(BeTrue())
		})

		It("check that specs are set accordingly after helmchart has been applied", func() {
			Expect(testEnv.Create(ctx, cluster)).To(Succeed())

			By("making the control plane ready")
			ph, err := patch.NewHelper(cluster, testEnv)
			Expect(err).ShouldNot(HaveOccurred())

			conditions.MarkTrue(cluster, clusterv1.ControlPlaneReadyCondition)

			Eventually(func() error {
				return ph.Patch(ctx, cluster)
			}, timeout, interval).Should(BeNil())

			By("ensuring helm chart is applied and specs are set")
			Eventually(func() bool {
				var foundClusterAddon csov1alpha1.ClusterAddon
				if err := testEnv.Get(ctx, key, &foundClusterAddon); err != nil {
					testEnv.GetLogger().Info(err.Error())
					return false
				}

				return utils.IsPresentAndTrue(ctx, testEnv.GetClient(), key, &foundClusterAddon, csov1alpha1.HelmChartAppliedCondition) &&
					foundClusterAddon.Status.Ready && foundClusterAddon.Spec.ClusterStack == testClusterStackName
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("Update test", func() {
		var (
			clusterStackRelease    *csov1alpha1.ClusterStackRelease
			clusterStackReleaseKey types.NamespacedName
		)

		BeforeEach(func() {
			clusterStackReleaseKey = types.NamespacedName{Name: testClusterStackNameV2, Namespace: testNs.Name}

			clusterStackRelease = &csov1alpha1.ClusterStackRelease{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testClusterStackNameV2,
					Namespace: testNs.Name,
				},
			}
			Expect(testEnv.Create(ctx, clusterStackRelease)).To(Succeed())

			Eventually(func() bool {
				var foundClusterStackRelease csov1alpha1.ClusterStackRelease
				if err := testEnv.Get(ctx, clusterStackReleaseKey, &foundClusterStackRelease); err != nil {
					testEnv.GetLogger().Info(err.Error())
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())
		})

		AfterEach(func() {
			Eventually(func() error {
				return testEnv.Cleanup(ctx, clusterStackRelease)
			}, timeout, interval).Should(BeNil())
		})

		It("does not update the clusteraddon helm chart objects if cluster switches to new cluster stack without new clusteraddon version", func() {
			cluster.Spec.Topology.Class = testClusterStackNameV2
			Expect(testEnv.Create(ctx, cluster)).To(Succeed())

			ph, err := patch.NewHelper(cluster, testEnv)
			Expect(err).ShouldNot(HaveOccurred())

			conditions.MarkTrue(cluster, clusterv1.ControlPlaneReadyCondition)

			Eventually(func() bool {
				if err := ph.Patch(ctx, cluster); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By("updating the clusterclass in the cluster")

			ph, err = patch.NewHelper(cluster, testEnv)
			Expect(err).ShouldNot(HaveOccurred())

			cluster.Spec.Topology.Class = testClusterStackNameV3

			Eventually(func() bool {
				if err := ph.Patch(ctx, cluster); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By("ensuring that the spec of clusteraddon is updated and status still synced")
			Eventually(func() bool {
				var foundClusterAddon csov1alpha1.ClusterAddon
				if err := testEnv.Get(ctx, key, &foundClusterAddon); err != nil {
					testEnv.GetLogger().Info(err.Error())
					return false
				}

				if foundClusterAddon.Spec.ClusterStack != testClusterStackNameV3 {
					testEnv.GetLogger().Info("found wrong cluster stack", "want", testClusterStackNameV3, "got", foundClusterAddon.Spec.ClusterStack)
					return false
				}

				if foundClusterAddon.Spec.Version != "v2" {
					testEnv.GetLogger().Info("found wrong cluster addon version", "want", "v2", "got", foundClusterAddon.Spec.Version)
					return false
				}

				return true
			}, timeout, interval).Should(BeTrue())

			// TODO: Check that mocked function has not been called
		})
	})
})

var _ = Describe("ClusterAddon validation", func() {
	var (
		clusteraddon *csov1alpha1.ClusterAddon
		testNs       *corev1.Namespace
		key          types.NamespacedName
	)

	const clusterAddonName = "cluster-addon-testcluster"

	BeforeEach(func() {
		var err error
		testNs, err = testEnv.CreateNamespace(ctx, "clusteraddon-validation")
		Expect(err).NotTo(HaveOccurred())

		key = types.NamespacedName{Namespace: testNs.Name, Name: clusterAddonName}

		clusteraddon = &csov1alpha1.ClusterAddon{
			ObjectMeta: metav1.ObjectMeta{
				Name:      clusterAddonName,
				Namespace: testNs.Name,
			},
			Spec: csov1alpha1.ClusterAddonSpec{
				ClusterRef: &corev1.ObjectReference{
					Name: "test",
					Kind: "Cluster",
				},
			},
		}
		Expect(testEnv.Create(ctx, clusteraddon)).To(Succeed())
	})

	AfterEach(func() {
		Eventually(func() error {
			return testEnv.Cleanup(ctx, testNs, clusteraddon)
		}, timeout, interval).Should(BeNil())
	})

	Context("validate update", func() {
		It("Should not allow an update of clusterAddon.Spec.ClusterRef", func() {
			var foundClusterAddon csov1alpha1.ClusterAddon
			Eventually(func() bool {
				if err := testEnv.Get(ctx, key, &foundClusterAddon); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			foundClusterAddon.Spec.ClusterRef.Name = "test2"
			Expect(testEnv.Update(ctx, &foundClusterAddon)).NotTo(Succeed())
		})
	})
})
