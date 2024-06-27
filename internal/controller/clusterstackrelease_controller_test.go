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
	"errors"

	csov1alpha1 "github.com/SovereignCloudStack/cluster-stack-operator/api/v1alpha1"
	"github.com/SovereignCloudStack/cluster-stack-operator/pkg/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ObjectLabelKeyOwned   = "clusterstack.x-k8s.io/instance"
	ObjectLabelValueOwned = "owned"
)

var _ = Describe("ClusterStackReleaseReconciler", func() {
	var (
		providerClusterStackRelease *unstructured.Unstructured
		clusterStackRelease         *csov1alpha1.ClusterStackRelease
		testNs                      *corev1.Namespace
		key                         types.NamespacedName
	)

	BeforeEach(func() {
		var err error
		testNs, err = testEnv.CreateNamespace(ctx, "clusterstackrelease-reconciler")
		Expect(err).NotTo(HaveOccurred())

		providerClusterStackRelease = &unstructured.Unstructured{
			Object: map[string]interface{}{
				"kind":       "TestInfrastructureProviderClusterStackRelease",
				"apiVersion": "infrastructure.clusterstack.x-k8s.io/v1alpha1",
				"metadata": map[string]interface{}{
					"name":      testClusterStackName,
					"namespace": testNs.Name,
				},
				"spec": map[string]interface{}{
					"NodeImages": []string{"nodeImage1"},
				},
			},
		}
		Expect(testEnv.Create(ctx, providerClusterStackRelease)).To(Succeed())

		clusterStackRelease = &csov1alpha1.ClusterStackRelease{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testClusterStackName,
				Namespace: testNs.Name,
			},
			Spec: csov1alpha1.ClusterStackReleaseSpec{
				ProviderRef: &corev1.ObjectReference{
					APIVersion: "infrastructure.clusterstack.x-k8s.io/v1alpha1",
					Kind:       "TestInfrastructureProviderClusterStackRelease",
					Name:       testClusterStackName,
					Namespace:  testNs.Name,
				},
			},
		}
		Expect(testEnv.Create(ctx, clusterStackRelease)).To(Succeed())

		key = types.NamespacedName{Name: clusterStackRelease.Name, Namespace: testNs.Name}
	})

	AfterEach(func() {
		Expect(testEnv.Cleanup(ctx, testNs, clusterStackRelease)).To(Succeed())
	})

	Context("provider object is ready", func() {
		BeforeEach(func() {
			providerClusterStackReleasePatch := client.MergeFrom(providerClusterStackRelease.DeepCopy())
			Expect(unstructured.SetNestedField(providerClusterStackRelease.Object, true, "status", "ready")).ToNot(HaveOccurred())
			Expect(testEnv.Status().Patch(ctx, providerClusterStackRelease, providerClusterStackReleasePatch)).To(Succeed())
		})

		It("checks the presence of clusterclass", func() {
			Eventually(func() bool {
				if err := testEnv.Get(ctx, key, &clusterv1.ClusterClass{}); err != nil {
					testEnv.GetLogger().Error(err, "failed to get clusterclass", "key", key)
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())
		})

		It("sets ClusterStackReleaseDownloaded condition once ClusterStackRelease object is created", func() {
			err := testEnv.Delete(ctx, clusterStackRelease)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() bool {
				if err := testEnv.Get(ctx, key, &clusterv1.ClusterClass{}); err != nil {
					return true
				}
				testEnv.GetLogger().Error(errors.New("cluster class found"), "clusterclass shouldn't be there", "key", key)
				return false
			}, timeout, interval).Should(BeTrue())
		})

		It("checks the kubernetes version of the ClusterStackRelease", func() {
			Eventually(func() bool {
				csr := &csov1alpha1.ClusterStackRelease{}
				if err := testEnv.Get(ctx, key, csr); err != nil {
					return false
				}

				testEnv.GetLogger().Info("ClusterStackRelease status", "status", csr.Status.KubernetesVersion)
				return csr.Status.KubernetesVersion != "v1.26.6"
			}, timeout, interval).Should(BeTrue())
		})

		It("check if helmchartapplied condition is there", func() {
			Eventually(func() bool {
				var foundClusterStackRelease csov1alpha1.ClusterStackRelease
				if err := testEnv.Get(ctx, key, &foundClusterStackRelease); err != nil {
					testEnv.GetLogger().Error(err, "clusterStackRelease not found", "key", key)
					return false
				}
				testEnv.GetLogger().Info("clusterStackRelease condition", "condition", foundClusterStackRelease.Status.Conditions)

				return utils.IsPresentAndTrue(ctx, testEnv, key, &foundClusterStackRelease, csov1alpha1.HelmChartAppliedCondition)
			}, timeout, interval).Should(BeTrue())
		})

		It("checks the presence of labels", func() {
			Eventually(func() bool {
				clusterclass := clusterv1.ClusterClass{}
				if err := testEnv.Get(ctx, key, &clusterclass); err != nil {
					testEnv.GetLogger().Error(err, "failed to get clusterclass", "key", key)
					return false
				}
				// check if labels are present
				if clusterclass.ObjectMeta.Labels == nil {
					testEnv.GetLogger().Error(nil, "no labels present on clusterclass object")
					return false
				}

				label, ok := clusterclass.ObjectMeta.Labels[ObjectLabelKeyOwned]
				if !ok {
					testEnv.GetLogger().Error(nil, "key was not found in labels", "key", ObjectLabelKeyOwned)
					return false
				}

				if label != ObjectLabelValueOwned {
					testEnv.GetLogger().Error(nil, "wrong value for label", "label", label)
					return false
				}

				return true
			}, timeout).Should(BeTrue())
		})

		It("updates the status of clusterStackResource with the clusterClass resource synced", func() {
			Eventually(func() bool {
				var foundClusterStackRelease csov1alpha1.ClusterStackRelease

				if err := testEnv.Get(ctx, key, &foundClusterStackRelease); err != nil {
					testEnv.GetLogger().Error(err, "failed to get clusterStackRelease", "key", key)
					return false
				}

				resources := foundClusterStackRelease.Status.Resources

				if len(resources) == 0 {
					testEnv.GetLogger().Info("no resources in status of clusterStackRelease", "key", key)
					return false
				}

				for _, resource := range resources {
					if resource.Name == clusterStackRelease.Name &&
						resource.Kind == "ClusterClass" &&
						resource.Namespace == testNs.Name {
						if resource.Status == csov1alpha1.ResourceStatusSynced {
							return true
						}
						testEnv.GetLogger().Info("clusterclass with status not synced in clusterStackRelease", "key", key)
						return false
					}
				}

				// not found
				testEnv.GetLogger().Info("clusterclass not found in status of clusterStackRelease", "key", key)
				return false
			}, timeout).Should(BeTrue())
		})

		It("syncs all resources successfully", func() {
			Eventually(func() bool {
				var foundClusterStackRelease csov1alpha1.ClusterStackRelease

				if err := testEnv.Get(ctx, key, &foundClusterStackRelease); err != nil {
					testEnv.GetLogger().Error(err, "failed to get clusterStackRelease", "key", key)
					return false
				}

				resources := foundClusterStackRelease.Status.Resources

				if len(resources) == 0 {
					testEnv.GetLogger().Info("no resources in status of clusterStackRelease", "key", key)
					return false
				}

				for _, resource := range resources {
					if resource.Status != csov1alpha1.ResourceStatusSynced {
						testEnv.GetLogger().Info("resource not synced", "resource", resource)
						return false
					}
				}

				return true
			}, timeout).Should(BeTrue())
		})

		It("checks that clusterStackRelease gets ready if providerclusterStackRelease is ready", func() {
			Eventually(func() bool {
				var foundClusterStackRelease csov1alpha1.ClusterStackRelease
				if err := testEnv.Get(ctx, key, &foundClusterStackRelease); err != nil {
					testEnv.GetLogger().Error(err, "failed to get clusterStackRelease", "key", key)
					return false
				}
				return utils.IsPresentAndTrue(ctx, testEnv.Client, key, &foundClusterStackRelease, csov1alpha1.ProviderClusterStackReleaseReadyCondition)
			}, timeout, interval).Should(BeTrue())
		})
	})
	Context("provider is not ready", func() {
		It("check clusterStackRelease is not ready if providerclusterStackRelease ready is false", func() {
			Eventually(func() bool {
				var foundClusterStackRelease csov1alpha1.ClusterStackRelease
				if err := testEnv.Get(ctx, key, &foundClusterStackRelease); err != nil {
					testEnv.GetLogger().Error(err, "failed to get clusterStackRelease", "key", key)
					return false
				}
				testEnv.GetLogger().Info("status of ClusterStackRelease", "status", foundClusterStackRelease.Status)
				return utils.IsPresentAndFalseWithReason(ctx, testEnv.Client, key, &foundClusterStackRelease, csov1alpha1.ProviderClusterStackReleaseReadyCondition, csov1alpha1.ProcessOngoingReason)
			}, timeout, interval).Should(BeTrue())
		})
	})
})

var _ = Describe("ClusterStackRelease validation", func() {
	var testNs *corev1.Namespace

	BeforeEach(func() {
		var err error
		testNs, err = testEnv.CreateNamespace(ctx, "clusterstack-validation")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Eventually(func() error {
			return testEnv.Cleanup(ctx, testNs)
		}, timeout, interval).Should(BeNil())
	})

	Context("validate delete", func() {
		var (
			clusterStackRelease      *csov1alpha1.ClusterStackRelease
			key                      types.NamespacedName
			foundClusterStackRelease csov1alpha1.ClusterStackRelease
			cluster                  clusterv1.Cluster
		)

		BeforeEach(func() {
			key = types.NamespacedName{Namespace: testNs.Name, Name: "docker-ferrol-1-27-v1"}
			By("creating ClusterStackRelease")
			clusterStackRelease = &csov1alpha1.ClusterStackRelease{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "docker-ferrol-1-27-v1",
					Namespace: testNs.Name,
				},
			}
			Expect(testEnv.Create(ctx, clusterStackRelease)).To(Succeed())

			By("checking if ClusterStackRelease is created properly")
			Eventually(func() error {
				return testEnv.Get(ctx, key, &foundClusterStackRelease)
			}, timeout, interval).Should(BeNil())

			By("creating cluster")
			cluster = clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster",
					Namespace: testNs.Name,
				},
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{
						Class:   "docker-ferrol-1-27-v1",
						Version: "v1.27.3",
					},
				},
			}

			testEnv.KubeClient.On("Apply", mock.Anything, mock.Anything, mock.Anything).Return([]*csov1alpha1.Resource{}, false, nil)
		})

		AfterEach(func() {
			Eventually(func() error {
				return testEnv.Cleanup(ctx, &cluster, clusterStackRelease)
			}, timeout, interval).Should(BeNil())
		})

		It("should not allow delete if ClusterStackRelease is in use by Cluster", func() {
			Expect(testEnv.Create(ctx, &cluster)).To(Succeed())
			Expect(testEnv.Delete(ctx, clusterStackRelease)).ToNot(Succeed())
		})

		It("should allow delete if existing Clusters reference ClusterClasses that do not follow the cluster stack naming convention", func() {
			cluster.Spec.Topology.Class = "test-cluster-class"
			Expect(testEnv.Create(ctx, &cluster)).To(Succeed())

			Expect(testEnv.Delete(ctx, clusterStackRelease)).To(Succeed())
		})

		It("should allow delete if existing Clusters reference different ClusterClasses", func() {
			cluster.Spec.Topology.Class = "docker-ferrol-1-26-v5"
			Expect(testEnv.Create(ctx, &cluster)).To(Succeed())

			Expect(testEnv.Delete(ctx, clusterStackRelease)).To(Succeed())
		})
	})
})
