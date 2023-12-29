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

package workloadcluster

import (
	"fmt"

	csov1alpha1 "github.com/SovereignCloudStack/cluster-stack-operator/api/v1alpha1"
	"github.com/SovereignCloudStack/cluster-stack-operator/internal/test/helpers"
	"github.com/SovereignCloudStack/cluster-stack-operator/pkg/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	capisecret "sigs.k8s.io/cluster-api/util/secret"
)

const (
	kubeSystemNamespace     = "kube-system"
	metricsServerDeployment = "metrics-server"
	webDeployment           = "web"
	clusterAddonLabelKey    = "clusterAddonVersion"
)

var _ = Describe("ClusterAddonReconciler", func() {
	var (
		cluster             *clusterv1.Cluster
		clusterStackRelease *csov1alpha1.ClusterStackRelease

		testNs *corev1.Namespace
		secret *corev1.Secret

		key                    types.NamespacedName
		clusterStackReleaseKey types.NamespacedName
	)

	BeforeEach(func() {
		var err error
		testNs, err = testEnv.CreateNamespace(ctx, "cso-system")
		Expect(err).NotTo(HaveOccurred())

		key = types.NamespacedName{Name: fmt.Sprintf("cluster-addon-%s", helpers.DefaultKindClusterName), Namespace: testNs.Name}
		clusterStackReleaseKey = types.NamespacedName{Name: testClusterStackName, Namespace: testNs.Name}

		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-%s", helpers.DefaultKindClusterName, capisecret.Kubeconfig),
				Namespace: testNs.Name,
			},
			Data: map[string][]byte{
				"value": []byte(testEnv.KubeConfig),
			},
			Type: "cluster.x-k8s.io/secret",
		}
		Expect(testEnv.Create(ctx, secret)).To(Succeed())

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
				testEnv.GetLogger().Error(err, "failed to get clusterStackRelease", "key", clusterStackReleaseKey)
				return false
			}
			return true
		}, timeout, interval).Should(BeTrue())

		cluster = &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      helpers.DefaultKindClusterName,
				Namespace: testNs.Name,
			},
			Spec: clusterv1.ClusterSpec{
				Topology: &clusterv1.Topology{
					Class:   testClusterStackName,
					Version: "v1.27.3",
				},
			},
		}
		Expect(testEnv.Create(ctx, cluster)).To(Succeed())
	})

	AfterEach(func() {
		Expect(testEnv.Cleanup(ctx, cluster, clusterStackRelease)).To(Succeed())
	})

	Context("Basic test", func() {
		It("sets HelmChartAppliedCondition condition on true if cluster addon helm chart has been applied", func() {
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
					testEnv.GetLogger().Error(err, "failed to get clusterAddon", "key", key)
					return false
				}

				if foundClusterAddon.Spec.ClusterRef.Name != cluster.Name {
					testEnv.GetLogger().Info("wrong cluster ref name", "got", foundClusterAddon.Spec.ClusterRef.Name, "want", cluster.Name)
					return false
				}

				return utils.IsPresentAndTrue(ctx, testEnv.Client, key, &foundClusterAddon, clusterv1.ReadyCondition)
			}, timeout, interval).Should(BeTrue())
		})

		It("updates the resource status and successfully applies it", func() {
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
					testEnv.GetLogger().Error(err, "failed to get clusterAddon", "key", key)
					return false
				}

				if len(foundClusterAddon.Status.Resources) == 0 {
					testEnv.GetLogger().Info("no resources found in status")
					return false
				}
				resource := foundClusterAddon.Status.Resources[0]

				if resource.Status != csov1alpha1.ResourceStatusSynced {
					testEnv.GetLogger().Info("first resource not synced", "resource", resource)
					return false
				}

				if resource.Error != "" {
					testEnv.GetLogger().Info("first resource has error", "error", resource.Error)
					return false
				}

				return true
			}, timeout, interval).Should(BeTrue())
		})

		It("applies the helm chart", func() {
			ph, err := patch.NewHelper(cluster, testEnv)
			Expect(err).ShouldNot(HaveOccurred())

			By("patching the cluster")
			conditions.MarkTrue(cluster, clusterv1.ControlPlaneReadyCondition)

			Eventually(func() bool {
				if err := ph.Patch(ctx, cluster); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By("querying the object")
			Eventually(func() bool {
				deployment, err := testEnv.WorkloadClusterClient.AppsV1().Deployments(kubeSystemNamespace).Get(ctx, metricsServerDeployment, metav1.GetOptions{})
				if err != nil {
					return false
				}

				// check labels
				version := deployment.GetLabels()[clusterAddonLabelKey]
				if version == "v1" {
					return true
				}
				testEnv.GetLogger().Info("found wrong label in deployment", "want", "v1", "got", version)
				return false
			}, timeout, interval).Should(BeTrue())
		})

		It("updates the clusteraddon helm chart objects if cluster switches to new cluster stack with new clusteraddon version", func() {
			ph, err := patch.NewHelper(cluster, testEnv)
			Expect(err).ShouldNot(HaveOccurred())

			conditions.MarkTrue(cluster, clusterv1.ControlPlaneReadyCondition)

			Eventually(func() bool {
				if err := ph.Patch(ctx, cluster); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By("querying the object")
			Eventually(func() bool {
				deployment, err := testEnv.WorkloadClusterClient.AppsV1().Deployments(kubeSystemNamespace).Get(ctx, metricsServerDeployment, metav1.GetOptions{})
				if err != nil {
					return false
				}

				// check labels
				version := deployment.GetLabels()[clusterAddonLabelKey]
				if version == "v1" {
					return true
				}

				testEnv.GetLogger().Info("found wrong label in deployment", "want", "v1", "got", version)
				return false
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

			By("checking that the deployment in workload cluster has been updated")
			Eventually(func() bool {
				deployment, err := testEnv.WorkloadClusterClient.AppsV1().Deployments(kubeSystemNamespace).Get(ctx, metricsServerDeployment, metav1.GetOptions{})
				if err != nil {
					return false
				}

				// check labels
				version := deployment.GetLabels()[clusterAddonLabelKey]
				if version == "v2" {
					return true
				}
				testEnv.GetLogger().Info("found wrong label in deployment", "want", "v2", "got", version)
				return false
			}, timeout, interval).Should(BeTrue())
		})

		It("checks if resources are removed from status after ClusterClass update", func() {
			By("making the control plane ready")
			ph, err := patch.NewHelper(cluster, testEnv)
			Expect(err).ShouldNot(HaveOccurred())

			conditions.MarkTrue(cluster, clusterv1.ControlPlaneReadyCondition)

			Eventually(func() error {
				return ph.Patch(ctx, cluster)
			}, timeout, interval).Should(BeNil())

			By("ensuring that the deployment exist in the old version is there in the status")
			Eventually(func() bool {
				var foundClusterAddon csov1alpha1.ClusterAddon
				if err := testEnv.Get(ctx, key, &foundClusterAddon); err != nil {
					testEnv.GetLogger().Info(err.Error())
					return false
				}
				testEnv.GetLogger().Info("Status of ClusterAddon", "Status", foundClusterAddon.Status)

				found := false
				for _, resource := range foundClusterAddon.Status.Resources {
					if resource.Name == "web" && resource.Kind == "Deployment" {
						found = true
					}
				}

				return found
			}, timeout, interval).Should(BeTrue())

			By("updating the cluster class")
			ph, err = patch.NewHelper(cluster, testEnv)
			Expect(err).ShouldNot(HaveOccurred())

			cluster.Spec.Topology.Class = testClusterStackNameV2

			Eventually(func() error {
				return ph.Patch(ctx, cluster)
			}, timeout, interval).Should(BeNil())

			By("ensuring that the deployment that does not exist in the new version is deleted from the status")
			Eventually(func() bool {
				var foundClusterAddon csov1alpha1.ClusterAddon
				if err := testEnv.Get(ctx, key, &foundClusterAddon); err != nil {
					testEnv.GetLogger().Info(err.Error())
					return false
				}
				testEnv.GetLogger().Info("Status of ClusterAddon", "Status", len(foundClusterAddon.Status.Resources))

				found := false
				for _, resource := range foundClusterAddon.Status.Resources {
					if resource.Name == "web" && resource.Kind == "Deployment" {
						found = true
					}
				}
				return found
			}, timeout, interval).Should(BeFalse())
		})

		It("checks if resources are removed from workloadcluster after ClusterClass update", func() {
			By("making the control plane ready")
			ph, err := patch.NewHelper(cluster, testEnv)
			Expect(err).ShouldNot(HaveOccurred())

			conditions.MarkTrue(cluster, clusterv1.ControlPlaneReadyCondition)

			Eventually(func() error {
				return ph.Patch(ctx, cluster)
			}, timeout, interval).Should(BeNil())

			By("ensuring the deployment is there in the workloadcluster")
			Eventually(func() bool {
				if _, err := testEnv.WorkloadClusterClient.AppsV1().Deployments(kubeSystemNamespace).Get(ctx, webDeployment, metav1.GetOptions{}); err != nil {
					testEnv.GetLogger().Error(err, "couldn't find deployment")
					return false
				}

				return true
			}, timeout, interval).Should(BeTrue())

			By("updating the cluster class")
			ph, err = patch.NewHelper(cluster, testEnv)
			Expect(err).ShouldNot(HaveOccurred())

			cluster.Spec.Topology.Class = testClusterStackNameV2

			Eventually(func() error {
				return ph.Patch(ctx, cluster)
			}, timeout, interval).Should(BeNil())

			By("ensuring the deployment is not there in the workloadcluster after clusterclass update")
			Eventually(func() bool {
				if _, err := testEnv.WorkloadClusterClient.AppsV1().Deployments(kubeSystemNamespace).Get(ctx, webDeployment, metav1.GetOptions{}); err != nil {
					if apierrors.IsNotFound(err) {
						return true
					}
					testEnv.GetLogger().Error(err, "couldn't find deployment")
					return false
				}

				return false
			}, timeout, interval).Should(BeTrue())
		})
	})
})
