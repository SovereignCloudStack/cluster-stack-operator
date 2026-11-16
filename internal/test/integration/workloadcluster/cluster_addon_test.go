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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	"sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions"
	"sigs.k8s.io/cluster-api/util/deprecated/v1beta1/patch"
	capisecret "sigs.k8s.io/cluster-api/util/secret"
)

const (
	kubeSystemNamespace      = "kube-system"
	metricsServerDeployment  = "metrics-server"
	ciliumOperatorDeployment = "cilium-operator"

	// gatewayAPIGroupVersion is installed by the gateway-api addon, which only the first release contains.
	gatewayAPIGroupVersion = "gateway.networking.k8s.io/v1"
	// domainLabelKey is set on the metrics-server from the control plane endpoint of the cluster,
	// see the overwrite.yaml of the metrics-server addon.
	domainLabelKey = "domain"
)

var _ = Describe("ClusterAddonReconciler", func() {
	var (
		cluster             *clusterv1.Cluster
		clusterStackRelease *csov1alpha1.ClusterStackRelease

		testNs *corev1.Namespace
		secret *corev1.Secret

		key types.NamespacedName
	)

	BeforeEach(func() {
		var err error
		testNs, err = testEnv.CreateNamespace(ctx, "cso-system")
		Expect(err).NotTo(HaveOccurred())

		key = types.NamespacedName{Name: fmt.Sprintf("cluster-addon-%s", helpers.DefaultKindClusterName), Namespace: testNs.Name}

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
			if err := testEnv.Get(ctx, types.NamespacedName{Name: testClusterStackName, Namespace: testNs.Name}, &foundClusterStackRelease); err != nil {
				testEnv.GetLogger().Error(err, "failed to get clusterStackRelease", "name", testClusterStackName)
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
				ControlPlaneEndpoint: clusterv1.APIEndpoint{Host: testControlPlaneHost, Port: 6443},
				Topology: &clusterv1.Topology{
					Class:   testClusterClassName,
					Version: testKubernetesVersion,
				},
			},
		}
		Expect(testEnv.Create(ctx, cluster)).To(Succeed())
	})

	AfterEach(func() {
		Expect(testEnv.Cleanup(ctx, cluster, clusterStackRelease)).To(Succeed())
	})

	// applyFirstRelease triggers the AfterControlPlaneInitialized hook, which the hook server of the
	// operator does for real clusters, and waits until the cluster addon is ready.
	applyFirstRelease := func() {
		var foundClusterAddon csov1alpha1.ClusterAddon

		Eventually(func() bool {
			if err := testEnv.Get(ctx, key, &foundClusterAddon); err != nil {
				testEnv.GetLogger().Error(err, "failed to get clusterAddon", "key", key)
				return false
			}
			return true
		}, timeout, interval).Should(BeTrue())

		ph, err := patch.NewHelper(&foundClusterAddon, testEnv)
		Expect(err).ShouldNot(HaveOccurred())

		foundClusterAddon.Spec.Hook = "AfterControlPlaneInitialized"
		Eventually(func() error {
			return ph.Patch(ctx, &foundClusterAddon)
		}, timeout, interval).Should(Succeed())

		Eventually(func() bool {
			return clusterAddonReady(key)
		}, applyTimeout, interval).Should(BeTrue())
	}

	// switchToSecondRelease updates the cluster class of the cluster to the second release.
	switchToSecondRelease := func() {
		By("creating the second cluster stack release")
		secondRelease := &csov1alpha1.ClusterStackRelease{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testClusterStackNameV2,
				Namespace: testNs.Name,
			},
		}
		Expect(testEnv.Create(ctx, secondRelease)).To(Succeed())
		DeferCleanup(func() {
			Expect(testEnv.Cleanup(ctx, secondRelease)).To(Succeed())
		})

		By("updating the cluster class of the cluster")
		ph, err := patch.NewHelper(cluster, testEnv)
		Expect(err).ShouldNot(HaveOccurred())

		cluster.Spec.Topology.Class = testClusterClassNameV2
		Eventually(func() error {
			return ph.Patch(ctx, cluster)
		}, timeout, interval).Should(Succeed())
	}

	Context("with the docker/scs cluster stack", func() {
		It("applies all addon stages and finishes the hook", func() {
			applyFirstRelease()

			var foundClusterAddon csov1alpha1.ClusterAddon
			Expect(testEnv.Get(ctx, key, &foundClusterAddon)).To(Succeed())

			Expect(conditions.IsTrue(&foundClusterAddon, csov1alpha1.HelmChartAppliedCondition)).To(BeTrue())
			Expect(foundClusterAddon.Spec.ClusterStack).To(Equal(testClusterClassName))
			Expect(foundClusterAddon.Spec.Hook).To(BeEmpty())
			Expect(foundClusterAddon.Status.Stages).To(BeEmpty())
		})

		It("applies the resources of all addons to the workload cluster", func() {
			applyFirstRelease()

			By("checking the gateway-api addon")
			_, err := testEnv.WorkloadClusterClient.Discovery().ServerResourcesForGroupVersion(gatewayAPIGroupVersion)
			Expect(err).NotTo(HaveOccurred())

			By("checking the cni addon")
			_, err = testEnv.WorkloadClusterClient.AppsV1().Deployments(kubeSystemNamespace).Get(ctx, ciliumOperatorDeployment, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("checking the metrics-server addon, which uses the control plane endpoint of the cluster")
			deployment, err := testEnv.WorkloadClusterClient.AppsV1().Deployments(kubeSystemNamespace).Get(ctx, metricsServerDeployment, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(deployment.GetLabels()).To(HaveKeyWithValue(domainLabelKey, testControlPlaneHost))
			Expect(*deployment.Spec.Replicas).To(Equal(int32(1)))
		})

		It("updates the objects if the cluster switches to a release with a changed addon", func() {
			applyFirstRelease()

			switchToSecondRelease()

			By("checking that the metrics-server in the workload cluster has been updated")
			Eventually(func() bool {
				deployment, err := testEnv.WorkloadClusterClient.AppsV1().Deployments(kubeSystemNamespace).Get(ctx, metricsServerDeployment, metav1.GetOptions{})
				if err != nil {
					testEnv.GetLogger().Error(err, "couldn't get deployment")
					return false
				}
				return *deployment.Spec.Replicas == 2
			}, applyTimeout, interval).Should(BeTrue())

			By("checking that the cluster addon is ready again with the new cluster class")
			Eventually(func() bool {
				var foundClusterAddon csov1alpha1.ClusterAddon
				if err := testEnv.Get(ctx, key, &foundClusterAddon); err != nil {
					return false
				}
				return foundClusterAddon.Spec.ClusterStack == testClusterClassNameV2 && clusterAddonReady(key)
			}, applyTimeout, interval).Should(BeTrue())
		})

		It("removes the objects of an addon that is not part of the new release", func() {
			applyFirstRelease()

			By("checking that the gateway-api addon of the first release is there")
			_, err := testEnv.WorkloadClusterClient.Discovery().ServerResourcesForGroupVersion(gatewayAPIGroupVersion)
			Expect(err).NotTo(HaveOccurred())

			switchToSecondRelease()

			By("checking that the objects of the gateway-api addon are removed from the workload cluster")
			Eventually(func() bool {
				_, err := testEnv.WorkloadClusterClient.Discovery().ServerResourcesForGroupVersion(gatewayAPIGroupVersion)
				return apierrors.IsNotFound(err)
			}, applyTimeout, interval).Should(BeTrue())

			By("checking that the objects of the other addons are still there")
			_, err = testEnv.WorkloadClusterClient.AppsV1().Deployments(kubeSystemNamespace).Get(ctx, ciliumOperatorDeployment, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

// clusterAddonReady returns whether the cluster addon is ready and its Helm charts are applied.
func clusterAddonReady(key types.NamespacedName) bool {
	var clusterAddon csov1alpha1.ClusterAddon
	if err := testEnv.Get(ctx, key, &clusterAddon); err != nil {
		testEnv.GetLogger().Error(err, "failed to get clusterAddon", "key", key)
		return false
	}

	testEnv.GetLogger().Info("Status of ClusterAddon", "ready", clusterAddon.Status.Ready, "hook", clusterAddon.Spec.Hook, "stages", clusterAddon.Status.Stages)

	return clusterAddon.Status.Ready && conditions.IsTrue(&clusterAddon, csov1alpha1.HelmChartAppliedCondition)
}
