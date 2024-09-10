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
	"reflect"
	"sort"
	"testing"

	csov1alpha1 "github.com/SovereignCloudStack/cluster-stack-operator/api/v1alpha1"
	"github.com/SovereignCloudStack/cluster-stack-operator/pkg/clusterstack"
	"github.com/SovereignCloudStack/cluster-stack-operator/pkg/version"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/external"
	"sigs.k8s.io/cluster-api/util/patch"
)

func TestMakeDiff(t *testing.T) {
	csr1 := &csov1alpha1.ClusterStackRelease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "release1",
			Namespace: "ns1",
		},
		Spec: csov1alpha1.ClusterStackReleaseSpec{
			ProviderRef: &corev1.ObjectReference{
				APIVersion: "infrastructure.clusterstack.x-k8s.io/v1alpha1",
				Kind:       "TestInfrastructureProviderClusterStackRelease",
				Name:       "release1",
				Namespace:  "ns1",
			},
		},
	}
	csr2 := &csov1alpha1.ClusterStackRelease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "release2",
			Namespace: "ns2",
		},
		Spec: csov1alpha1.ClusterStackReleaseSpec{
			ProviderRef: &corev1.ObjectReference{
				APIVersion: "infrastructure.clusterstack.x-k8s.io/v1alpha1",
				Kind:       "TestInfrastructureProviderClusterStackRelease",
				Name:       "release2",
				Namespace:  "ns2",
			},
		},
	}
	csr3 := &csov1alpha1.ClusterStackRelease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "release3",
			Namespace: "ns3",
		},
		Spec: csov1alpha1.ClusterStackReleaseSpec{
			ProviderRef: &corev1.ObjectReference{
				APIVersion: "infrastructure.clusterstack.x-k8s.io/v1alpha1",
				Kind:       "TestInfrastructureProviderClusterStackRelease",
				Name:       "release3",
				Namespace:  "ns3",
			},
		},
	}
	csr4 := &csov1alpha1.ClusterStackRelease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "release4",
			Namespace: "ns4",
		},
		Spec: csov1alpha1.ClusterStackReleaseSpec{
			ProviderRef: &corev1.ObjectReference{
				APIVersion: "infrastructure.clusterstack.x-k8s.io/v1alpha1",
				Kind:       "TestInfrastructureProviderClusterStackRelease",
				Name:       "release4",
				Namespace:  "ns4",
			},
		},
	}

	// Test Case 1:
	// - latest and latestReady are different
	// - latestReady is in use
	// - new entry in spec
	// - expected to delete old entries not in use
	// - expected to create new entries in spec and in use
	clusterStackReleases := []*csov1alpha1.ClusterStackRelease{csr1, csr2, csr3}
	latest := csr3.Name
	latestReady := csr2.Name

	inSpec := map[string]struct{}{
		csr4.Name: {},
	}
	inUse := map[string]struct{}{
		csr2.Name: {},
	}

	toCreate, toDelete := makeDiff(clusterStackReleases, &latest, &latestReady, inSpec, inUse)
	expectedToCreate := []*csov1alpha1.ClusterStackRelease{csr2, csr3, csr4}
	expectedToDelete := []*csov1alpha1.ClusterStackRelease{csr1}

	if !reflect.DeepEqual(toString(toCreate), toString(expectedToCreate)) {
		t.Errorf("toCreate is not as expected")
	}

	if !reflect.DeepEqual(toString(toDelete), toString(expectedToDelete)) {
		t.Errorf("toDelete is not as expected")
	}

	// Test Case 2:
	// - latest and latestReady are same
	// - latestReady is in use
	// - new entry in spec
	// - expected to delete old entries not in use
	// - expected to create new entries in spec and in use
	clusterStackReleases = []*csov1alpha1.ClusterStackRelease{csr1, csr2, csr3}
	latest = csr3.Name
	latestReady = csr3.Name

	inSpec = map[string]struct{}{
		csr4.Name: {},
	}
	inUse = map[string]struct{}{
		csr3.Name: {},
	}

	toCreate, toDelete = makeDiff(clusterStackReleases, &latest, &latestReady, inSpec, inUse)
	expectedToCreate = []*csov1alpha1.ClusterStackRelease{csr3, csr4}
	expectedToDelete = []*csov1alpha1.ClusterStackRelease{csr1, csr2}

	if !reflect.DeepEqual(toString(toCreate), toString(expectedToCreate)) {
		t.Errorf("toCreate is not as expected. Expected %v, got %v", toString(expectedToCreate), toString(toCreate))
	}

	if !reflect.DeepEqual(toString(toDelete), toString(expectedToDelete)) {
		t.Errorf("toDelete is not as expected. Expected %v, got %v", toString(expectedToDelete), toString(toDelete))
	}

	// Test Case 3
	// - latest and latestReady are same
	// - latestReady is not in use
	// - no new entry in spec
	// - expected to delete old entries not in use
	// - expected to create new entries in spec and in use
	clusterStackReleases = []*csov1alpha1.ClusterStackRelease{csr1, csr2, csr3, csr4}
	latest = csr4.Name
	latestReady = csr4.Name

	inUse = map[string]struct{}{
		csr3.Name: {},
	}

	toCreate, toDelete = makeDiff(clusterStackReleases, &latest, &latestReady, nil, inUse)
	expectedToCreate = []*csov1alpha1.ClusterStackRelease{csr3, csr4}
	expectedToDelete = []*csov1alpha1.ClusterStackRelease{csr1, csr2}

	if !reflect.DeepEqual(toString(toCreate), toString(expectedToCreate)) {
		t.Errorf("toCreate is not as expected. Expected %v, got %v", toString(expectedToCreate), toString(toCreate))
	}

	if !reflect.DeepEqual(toString(toDelete), toString(expectedToDelete)) {
		t.Errorf("toDelete is not as expected. Expected %v, got %v", toString(expectedToDelete), toString(toDelete))
	}
}

func TestGetLatestReadyClusterStackRelease(t *testing.T) {
	clusterStackReleases := []*csov1alpha1.ClusterStackRelease{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "docker-ferrol-1-18-v1",
				Namespace: "ns1",
			},
			Spec: csov1alpha1.ClusterStackReleaseSpec{
				ProviderRef: &corev1.ObjectReference{
					APIVersion: "infrastructure.clusterstack.x-k8s.io/v1alpha1",
					Kind:       "TestInfrastructureProviderClusterStackRelease",
					Name:       "docker-ferrol-1-18-v1",
					Namespace:  "ns1",
				},
			},
			Status: csov1alpha1.ClusterStackReleaseStatus{Ready: true, KubernetesVersion: "v1.18"},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "docker-ferrol-1-18-v2",
				Namespace: "ns1",
			},
			Spec: csov1alpha1.ClusterStackReleaseSpec{
				ProviderRef: &corev1.ObjectReference{
					APIVersion: "infrastructure.clusterstack.x-k8s.io/v1alpha1",
					Kind:       "TestInfrastructureProviderClusterStackRelease",
					Name:       "docker-ferrol-1-18-v2",
					Namespace:  "ns1",
				},
			},
			Status: csov1alpha1.ClusterStackReleaseStatus{Ready: false},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "docker-ferrol-1-21-v3",
				Namespace: "ns1",
			},
			Spec: csov1alpha1.ClusterStackReleaseSpec{
				ProviderRef: &corev1.ObjectReference{
					APIVersion: "infrastructure.clusterstack.x-k8s.io/v1alpha1",
					Kind:       "TestInfrastructureProviderClusterStackRelease",
					Name:       "docker-ferrol-1-21-v3",
					Namespace:  "ns1",
				},
			},
			Status: csov1alpha1.ClusterStackReleaseStatus{Ready: true, KubernetesVersion: "v1.21"},
		},
	}

	// Test case 1
	// - a latest and ready cluster stack release is present in the list of cluster stack releases
	t.Run("a ready, latest cluster stack release present", func(t *testing.T) {
		// Call the function being tested
		latest, k8sVersion, err := getLatestReadyClusterStackRelease(clusterStackReleases)
		if err != nil {
			t.Errorf("Expected err to be nil, but got: %v", err)
		}
		expectedLatest := "docker-ferrol-1-21-v3"
		if latest == nil {
			t.Errorf("Expected latest to be non-nil, but got nil")
		} else if *latest != expectedLatest {
			t.Errorf("Expected latest to be %s, but got %s", expectedLatest, *latest)
		}
		expectedK8sVersion := "v1.21"
		if k8sVersion != expectedK8sVersion {
			t.Errorf("Expected k8sVersion to be %s, but got %s", expectedK8sVersion, k8sVersion)
		}
	})

	// Test case 2
	// - no latest cluster stack release is present in the list of cluster stack releases
	t.Run("No ready cluster stack releases", func(t *testing.T) {
		// Create an empty list of ClusterStackRelease objects
		emptyClusterStackReleases := []*csov1alpha1.ClusterStackRelease{}

		latest, k8sVersion, err := getLatestReadyClusterStackRelease(emptyClusterStackReleases)
		if err != nil {
			t.Errorf("Expected err to be nil, but got: %v", err)
		}
		if latest != nil {
			t.Errorf("Expected latest to be nil, but got %v", *latest)
		}
		if k8sVersion != "" {
			t.Errorf("Expected k8sVersion to be an empty string, but got %s", k8sVersion)
		}
	})
}

func TestGetUsableClusterStackReleaseVersions(t *testing.T) {
	clusterStackReleases := []*csov1alpha1.ClusterStackRelease{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "docker-ferrol-1-18-v1",
				Namespace: "ns1",
			},
			Spec: csov1alpha1.ClusterStackReleaseSpec{
				ProviderRef: &corev1.ObjectReference{
					APIVersion: "infrastructure.clusterstack.x-k8s.io/v1alpha1",
					Kind:       "TestInfrastructureProviderClusterStackRelease",
					Name:       "docker-ferrol-1-18-v1",
					Namespace:  "ns1",
				},
			},
			Status: csov1alpha1.ClusterStackReleaseStatus{Ready: true},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "docker-ferrol-1-18-v2",
				Namespace: "ns1",
			},
			Spec: csov1alpha1.ClusterStackReleaseSpec{
				ProviderRef: &corev1.ObjectReference{
					APIVersion: "infrastructure.clusterstack.x-k8s.io/v1alpha1",
					Kind:       "TestInfrastructureProviderClusterStackRelease",
					Name:       "docker-ferrol-1-18-v2",
					Namespace:  "ns1",
				},
			},
			Status: csov1alpha1.ClusterStackReleaseStatus{Ready: false},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "docker-ferrol-1-21-v3",
				Namespace: "ns1",
			},
			Spec: csov1alpha1.ClusterStackReleaseSpec{
				ProviderRef: &corev1.ObjectReference{
					APIVersion: "infrastructure.clusterstack.x-k8s.io/v1alpha1",
					Kind:       "TestInfrastructureProviderClusterStackRelease",
					Name:       "docker-ferrol-1-21-v3",
					Namespace:  "ns1",
				},
			},
			Status: csov1alpha1.ClusterStackReleaseStatus{Ready: true},
		},
	}

	// test case 1
	// - two ready cluster stack releases are present in the list of cluster stack releases
	t.Run("two ready cluster stack releases", func(t *testing.T) {
		// Call the function
		usableVersions, err := getUsableClusterStackReleaseVersions(clusterStackReleases)
		if err != nil {
			t.Errorf("Expected no error, but got %v", err)
		}

		// Expected result
		expectedVersions := []string{"v1", "v3"}

		// Check if the result matches the expected versions
		if !equalStringSlices(usableVersions, expectedVersions) {
			t.Errorf("Expected versions %v, but got %v", expectedVersions, usableVersions)
		}
	})

	// test case 2
	// - no ready cluster stack releases are present in the list of cluster stack releases
	t.Run("no ready cluster stack releases", func(t *testing.T) {
		// Call the function
		usableVersions, err := getUsableClusterStackReleaseVersions(nil)
		if err != nil {
			t.Errorf("Expected no error, but got %v", err)
		}

		// Expected result
		expectedVersions := []string{}

		// Check if the result matches the expected versions
		if !equalStringSlices(usableVersions, expectedVersions) {
			t.Errorf("Expected versions %v, but got %v", expectedVersions, usableVersions)
		}
	})
}

func TestGetClusterStackReleasesInSpec(t *testing.T) {
	spec := &csov1alpha1.ClusterStackSpec{
		Provider:          "docker",
		Name:              "ferrol",
		KubernetesVersion: "1.21",
		Versions:          []string{"v1", "v2", "v3-alpha.0"},
	}

	result, err := getClusterStackReleasesInSpec(spec)
	assert.NoError(t, err, "Expected no error")

	expectedResult := map[string]struct{}{
		"docker-ferrol-1-21-v1":         {},
		"docker-ferrol-1-21-v2":         {},
		"docker-ferrol-1-21-v3-alpha-0": {},
	}

	// check if all the three versions given in the spec are present in the result
	for k := range result {
		if _, ok := expectedResult[k]; !ok {
			t.Errorf("Expected result %v, not found", k)
		}
	}
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func toString(arr1 []*csov1alpha1.ClusterStackRelease) []string {
	arr2 := make([]string, 0, len(arr1))
	for _, v := range arr1 {
		arr2 = append(arr2, v.Name)
	}
	sort.Strings(arr2)
	return arr2
}

var _ = Describe("ClusterStackReconciler", func() {
	var (
		clusterStack *csov1alpha1.ClusterStack
		testNs       *corev1.Namespace
	)

	BeforeEach(func() {
		var err error
		testNs, err = testEnv.CreateNamespace(ctx, "clusterstack-reconciler")
		Expect(err).NotTo(HaveOccurred())

		// clusterStack with auto subscribe false
		clusterStack = &csov1alpha1.ClusterStack{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test1",
				Namespace: testNs.Name,
			},
			Spec: csov1alpha1.ClusterStackSpec{
				Provider:          "docker",
				Name:              "ferrol",
				KubernetesVersion: "1.27",
				Versions:          []string{"v1"},
				AutoSubscribe:     false,
			},
		}
	})

	AfterEach(func() {
		Expect(testEnv.Cleanup(ctx, testNs)).To(Succeed())
	})

	Context("Test with provider", func() {
		var providerClusterStackReleaseTemplate *unstructured.Unstructured

		BeforeEach(func() {
			providerClusterStackReleaseTemplate = &unstructured.Unstructured{
				Object: map[string]interface{}{
					"kind":       "TestInfrastructureProviderClusterStackReleaseTemplate",
					"apiVersion": "infrastructure.clusterstack.x-k8s.io/v1alpha1",
					"metadata": map[string]interface{}{
						"name":      "provider-test1",
						"namespace": testNs.Name,
					},
					"spec": map[string]interface{}{
						"template": map[string]interface{}{
							"spec": map[string]interface{}{
								"nodeImages": []string{"controlplaneamd64provider", "workeramd64provider"},
							},
						},
					},
				},
			}
			Expect(testEnv.Create(ctx, providerClusterStackReleaseTemplate)).To(Succeed())
		})

		AfterEach(func() {
			Expect(testEnv.Cleanup(ctx, providerClusterStackReleaseTemplate)).To(Succeed())
		})

		Context("Basic test", func() {
			var clusterStackReleaseName string
			BeforeEach(func() {
				clusterStack.Spec.ProviderRef = &corev1.ObjectReference{
					APIVersion: "infrastructure.clusterstack.x-k8s.io/v1alpha1",
					Kind:       "TestInfrastructureProviderClusterStackReleaseTemplate",
					Name:       "provider-test1",
					Namespace:  testNs.Name,
				}
				Expect(testEnv.Create(ctx, clusterStack)).To(Succeed())

				cs, err := clusterstack.New(clusterStack.Spec.Provider, clusterStack.Spec.Name, clusterStack.Spec.KubernetesVersion, "v1")
				Expect(err).To(BeNil())
				clusterStackReleaseName = cs.String()
			})

			AfterEach(func() {
				Expect(testEnv.Cleanup(ctx, clusterStack)).To(Succeed())
			})

			It("creates the cluster stack release object with cluster stack auto subscribe false", func() {
				key := types.NamespacedName{Name: clusterStackReleaseName, Namespace: testNs.Name}
				Eventually(func() error {
					return testEnv.Get(ctx, key, &csov1alpha1.ClusterStackRelease{})
				}, timeout, interval).Should(BeNil())
			})

			It("creates the provider cluster stack release object", func() {
				foundProviderclusterStackReleaseRef := &corev1.ObjectReference{
					APIVersion: "infrastructure.clusterstack.x-k8s.io/v1alpha1",
					Kind:       "TestInfrastructureProviderClusterStackRelease",
					Name:       clusterStackReleaseName,
					Namespace:  testNs.Name,
				}

				Eventually(func() error {
					_, err := external.Get(ctx, testEnv.GetClient(), foundProviderclusterStackReleaseRef, testNs.Name)
					return err
				}, timeout).Should(BeNil())
			})
		})

		Context("Tests with multiple versions", func() {
			var (
				clusterStackReleaseTagV1    string
				clusterStackReleaseTagV2    string
				clusterStackReleaseTagV1Key types.NamespacedName
				clusterStackReleaseTagV2Key types.NamespacedName
			)
			BeforeEach(func() {
				clusterStack.Spec = csov1alpha1.ClusterStackSpec{
					Provider:          "docker",
					Name:              "ferrol",
					KubernetesVersion: "1.27",
					Versions:          []string{"v1", "v2"},
					AutoSubscribe:     false,
					ProviderRef: &corev1.ObjectReference{
						APIVersion: "infrastructure.clusterstack.x-k8s.io/v1alpha1",
						Kind:       "TestInfrastructureProviderClusterStackReleaseTemplate",
						Name:       "provider-test1",
						Namespace:  testNs.Name,
					},
				}
				Expect(testEnv.Create(ctx, clusterStack)).To(Succeed())

				cs, err := clusterstack.New(clusterStack.Spec.Provider, clusterStack.Spec.Name, clusterStack.Spec.KubernetesVersion, "v1")
				Expect(err).To(BeNil())
				clusterStackReleaseTagV1 = cs.String()

				cs, err = clusterstack.New(clusterStack.Spec.Provider, clusterStack.Spec.Name, clusterStack.Spec.KubernetesVersion, "v2")
				Expect(err).To(BeNil())
				clusterStackReleaseTagV2 = cs.String()

				clusterStackReleaseTagV1Key = types.NamespacedName{Name: clusterStackReleaseTagV1, Namespace: testNs.Name}
				clusterStackReleaseTagV2Key = types.NamespacedName{Name: clusterStackReleaseTagV2, Namespace: testNs.Name}
				waitUntilChildObjectsAreCreated(clusterStack)
			})

			AfterEach(func() {
				Expect(testEnv.Cleanup(ctx, clusterStack)).To(Succeed())
			})

			It("creates multiple ClusterStackRelease and ProviderClusterStackRelease", func() {
				By("checking that ClusterStackRelease objects get created")

				Eventually(func() error {
					var clusterStackRelease csov1alpha1.ClusterStackRelease

					if err := testEnv.Get(ctx, clusterStackReleaseTagV1Key, &clusterStackRelease); err != nil {
						testEnv.GetLogger().Error(err, "failed to get clusterStackRelease", "key", clusterStackReleaseTagV1Key)
						return err
					}
					if err := testEnv.Get(ctx, clusterStackReleaseTagV2Key, &clusterStackRelease); err != nil {
						testEnv.GetLogger().Error(err, "failed to get clusterStackRelease", "key", clusterStackReleaseTagV2Key)
						return err
					}

					return nil
				}, timeout, interval).Should(BeNil())

				By("checking that ProviderClusterStackRelease objects get created")

				Eventually(func() error {
					foundProviderclusterStackReleaseRef := &corev1.ObjectReference{
						APIVersion: "infrastructure.clusterstack.x-k8s.io/v1alpha1",
						Kind:       "TestInfrastructureProviderClusterStackRelease",
						Name:       clusterStackReleaseTagV1,
						Namespace:  testNs.Name,
					}

					if _, err := external.Get(ctx, testEnv.GetClient(), foundProviderclusterStackReleaseRef, testNs.Name); err != nil {
						testEnv.GetLogger().Error(err, "failed to get providerClusterStackRelease", "ref", foundProviderclusterStackReleaseRef)
						return err
					}

					foundProviderclusterStackReleaseRef = &corev1.ObjectReference{
						APIVersion: "infrastructure.clusterstack.x-k8s.io/v1alpha1",
						Kind:       "TestInfrastructureProviderClusterStackRelease",
						Name:       clusterStackReleaseTagV2,
						Namespace:  testNs.Name,
					}

					if _, err := external.Get(ctx, testEnv.GetClient(), foundProviderclusterStackReleaseRef, testNs.Name); err != nil {
						testEnv.GetLogger().Error(err, "failed to get providerClusterStackRelease", "ref", foundProviderclusterStackReleaseRef)
						return err
					}

					return nil
				}, timeout, interval).Should(BeNil())
			})

			/////////////////////////////////////////////////////////////////////
			// Flaky test
			FIt("checks ProviderClusterstackrelease is deleted when version is removed from spec", func() {
				fmt.Println("itttttttttttttttttttttttttttttttttttttttttt")
				ph, err := patch.NewHelper(clusterStack, testEnv)
				Expect(err).ShouldNot(HaveOccurred())

				providerclusterStackReleaseRefV2 := &corev1.ObjectReference{
					APIVersion: "infrastructure.clusterstack.x-k8s.io/v1alpha1",
					Kind:       "TestInfrastructureProviderClusterStackRelease",
					Name:       clusterStackReleaseTagV2,
					Namespace:  testNs.Name,
				}

				// Wait until the ProviderClusterStackRelease is created
				// Check for the correct ownerRef
				Eventually(func() bool {
					obj, err := external.Get(ctx, testEnv.GetClient(), providerclusterStackReleaseRefV2, testNs.Name)
					if apierrors.IsNotFound(err) {
						fmt.Printf("   providerclusterStackReleaseRefV2 not found (retry). %s\n", err.Error())
						return false
					}
					if err != nil {
						fmt.Printf("   get providerclusterStackReleaseRefV2: error (retry). %s\n", err.Error())
						return false
					}
					if obj.GetDeletionTimestamp() != nil {
						// in envTests there is not GC: If the parent-objects gets deleted, the child-objects are not updated..
						fmt.Printf("   I am confused. providerclusterStackReleaseRefV2 has a deletionTimestamp? (retry). %s\n", obj.GetDeletionTimestamp())
						return false
					}
					owners := obj.GetOwnerReferences()
					fmt.Printf("    providerclusterStackReleaseRefV2 found: Finalizers %+v OwnerRefs: %+v Kind: %s, uuid: %s\n", obj.GetFinalizers(), owners,
						obj.GetKind(), obj.GetUID())
					if len(obj.GetOwnerReferences()) == 0 {
						fmt.Printf("    Missing finalizer/ownerRefs\n")
						return false
					}
					if len(obj.GetOwnerReferences()) > 1 {
						fmt.Printf("    Too many ownerRefs?\n")
						return false
					}
					ownerRef := obj.GetOwnerReferences()[0]
					if ownerRef.APIVersion != "clusterstack.x-k8s.io/v1alpha1" || ownerRef.Kind != "ClusterStackRelease" ||
						ownerRef.Name != "docker-ferrol-1-27-v2" || !*ownerRef.Controller || !*ownerRef.BlockOwnerDeletion {
						fmt.Printf("    I am confused, wrong ownerRef: %+v\n", ownerRef)
						return false
					}
					return true
				}, timeout, interval).Should(BeTrue())

				fmt.Printf("old %+v new %+v\n", clusterStack.Spec.Versions, []string{"v1"})
				clusterStack.Spec.Versions = []string{"v1"}

				// remove v2 from CS
				Eventually(func() error {
					err := ph.Patch(ctx, clusterStack)
					return err
				}, timeout, interval).Should(BeNil())

				// wait until clusterStackRelease V2 is deleted
				// This delete propagation (From CS spec.versions --> ClusterStackRelease) is done by our controller (not Kubernetes GC)
				Eventually(func() bool {
					obj := csov1alpha1.ClusterStackRelease{}
					err := testEnv.Get(ctx, clusterStackReleaseTagV2Key, &obj)

					if apierrors.IsNotFound(err) {
						fmt.Printf("    clusterStackReleaseTagV2Key was deleted (good): %s\n", err.Error())
						return true
					}

					if err != nil {
						fmt.Printf("    clusterStackReleaseTagV2Key error (retry). %s\n", err.Error())
						return false
					}
					fmt.Printf("    clusterStackReleaseTagV2Key is found (will retry). %s Finalizers: %+v OwnerRefs %+v DelTimestamp: %v\n", obj.GetName(), obj.GetFinalizers(), obj.GetOwnerReferences(),
						obj.GetDeletionTimestamp())
					return false
				}, timeout, interval).Should(BeTrue())
			})
		})
	})

	Context("Tests without provider", func() {
		var (
			clusterStackReleaseTagV1Name string
			key                          types.NamespacedName
		)

		BeforeEach(func() {
			clusterStack.Spec.NoProvider = true
			Expect(testEnv.Create(ctx, clusterStack)).To(Succeed())

			cs, err := clusterstack.New(clusterStack.Spec.Provider, clusterStack.Spec.Name, clusterStack.Spec.KubernetesVersion, "v1")
			clusterStackReleaseTagV1Name = cs.String()
			Expect(err).To(BeNil())

			key = types.NamespacedName{Name: clusterStackReleaseTagV1Name, Namespace: testNs.Name}
		})

		AfterEach(func() {
			Expect(testEnv.Cleanup(ctx, clusterStack)).To(Succeed())
		})

		It("creates the cluster stack release object with providerRef empty", func() {
			Eventually(func() bool {
				var clusterStackRelease csov1alpha1.ClusterStackRelease
				if err := testEnv.Get(ctx, key, &clusterStackRelease); err != nil {
					testEnv.GetLogger().Error(err, "failed to get clusterStackRelease", "key", key)
					return false
				}

				return clusterStackRelease.Spec.ProviderRef == nil
			}, timeout, interval).Should(BeTrue())
		})
	})
})

var _ = Describe("clusterStack validation", func() {
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

	Context("validate create", func() {
		var clusterStack *csov1alpha1.ClusterStack

		BeforeEach(func() {
			clusterStack = &csov1alpha1.ClusterStack{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-stack",
					Namespace: testNs.Name,
				},
				Spec: csov1alpha1.ClusterStackSpec{
					Provider:          "docker",
					Name:              "testclusterstack",
					KubernetesVersion: "1.27",
					NoProvider:        true,
					AutoSubscribe:     false,
					ProviderRef: &corev1.ObjectReference{
						APIVersion: "infrastructure.clusterstack.x-k8s.io/v1alpha1",
						Kind:       "DockerClusterStackReleaseTemplate",
						Name:       "mytemplate",
						Namespace:  testNs.Name,
					},
				},
			}
		})

		AfterEach(func() {
			Eventually(func() error {
				return testEnv.Cleanup(ctx, clusterStack)
			}, timeout, interval).Should(BeNil())
		})

		It("should succeed with correct spec", func() {
			Expect(testEnv.Create(ctx, clusterStack)).To(Succeed())
		})

		It("should succeed with no provider", func() {
			clusterStack.Spec.ProviderRef = nil
			Expect(testEnv.Create(ctx, clusterStack)).To(Succeed())
		})

		It("should fail with a wrong version tag", func() {
			clusterStack.Spec.Versions = append(clusterStack.Spec.Versions, "v1-alpha")
			Expect(testEnv.Create(ctx, clusterStack)).NotTo(Succeed())
		})

		It("should fail with empty provider", func() {
			clusterStack.Spec.Provider = ""
			Expect(testEnv.Create(ctx, clusterStack)).NotTo(Succeed())
		})

		It("should fail with empty clusterStack name", func() {
			clusterStack.Spec.Name = ""
			Expect(testEnv.Create(ctx, clusterStack)).NotTo(Succeed())
		})

		It("providerRef is nil and no provider is false", func() {
			clusterStack.Spec.ProviderRef = nil
			clusterStack.Spec.NoProvider = false
			Expect(testEnv.Create(ctx, clusterStack)).ToNot(Succeed())
		})
	})

	Context("validate update", func() {
		var (
			clusterStack      *csov1alpha1.ClusterStack
			key               types.NamespacedName
			foundClusterStack csov1alpha1.ClusterStack
		)

		BeforeEach(func() {
			key = types.NamespacedName{Namespace: testNs.Name, Name: "cluster-stack"}
			clusterStack = &csov1alpha1.ClusterStack{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-stack",
					Namespace: testNs.Name,
				},
				Spec: csov1alpha1.ClusterStackSpec{
					Provider:          "docker",
					Name:              "testclusterstack",
					Channel:           version.ChannelStable,
					KubernetesVersion: "1.27",
					NoProvider:        true,
					AutoSubscribe:     false,
				},
			}
			Expect(testEnv.Create(ctx, clusterStack)).To(Succeed())

			Eventually(func() bool {
				if err := testEnv.Get(ctx, key, &foundClusterStack); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())
		})

		AfterEach(func() {
			Eventually(func() error {
				return testEnv.Cleanup(ctx, clusterStack)
			}, timeout, interval).Should(BeNil())
		})

		It("should have correct specs", func() {
			Expect(foundClusterStack.Spec.AutoSubscribe).To(Equal(false))
			Expect(foundClusterStack.Spec.NoProvider).To(Equal(true))
			Expect(foundClusterStack.Spec.Provider).To(Equal("docker"))
			Expect(foundClusterStack.Spec.Name).To(Equal("testclusterstack"))
			Expect(foundClusterStack.Spec.KubernetesVersion).To(Equal("1.27"))
			Expect(foundClusterStack.Spec.Channel).To(Equal(version.ChannelStable))
		})

		It("Should not allow an update of ClusterStack.Spec.Provider", func() {
			foundClusterStack.Spec.Provider = "otherprovider"
			Expect(testEnv.Update(ctx, &foundClusterStack)).NotTo(Succeed())
		})

		It("Should not allow an update of ClusterStack.Spec.Name", func() {
			foundClusterStack.Spec.Name = "testclusterstack2"
			Expect(testEnv.Update(ctx, &foundClusterStack)).NotTo(Succeed())
		})

		It("Should not allow an update of ClusterStack.Spec.Channel", func() {
			foundClusterStack.Spec.Channel = version.ChannelCustom
			Expect(testEnv.Update(ctx, &foundClusterStack)).NotTo(Succeed())
		})

		It("Should not allow an update of ClusterStack.Spec.KubernetesVersion", func() {
			foundClusterStack.Spec.KubernetesVersion = "1.25"
			Expect(testEnv.Update(ctx, &foundClusterStack)).NotTo(Succeed())
		})
	})

	Context("validate delete", func() {
		var (
			clusterStack      *csov1alpha1.ClusterStack
			key               types.NamespacedName
			foundClusterStack csov1alpha1.ClusterStack
			cluster           clusterv1.Cluster
		)

		BeforeEach(func() {
			key = types.NamespacedName{Namespace: testNs.Name, Name: "cluster-stack"}
			By("creating clusterstack")
			clusterStack = &csov1alpha1.ClusterStack{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "cluster-stack",
					Namespace:  testNs.Name,
					Finalizers: []string{clusterv1.ClusterFinalizer},
				},
				Spec: csov1alpha1.ClusterStackSpec{
					Provider:          "docker",
					Name:              "ferrol",
					Channel:           version.ChannelStable,
					KubernetesVersion: "1.27",
					NoProvider:        true,
					AutoSubscribe:     false,
				},
			}
			Expect(testEnv.Create(ctx, clusterStack)).To(Succeed())

			By("checking if clusterstack is created properly")
			Eventually(func() error {
				return testEnv.Get(ctx, key, &foundClusterStack)
			}, timeout, interval).Should(BeNil())

			By("creating cluster")
			cluster = clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster",
					Namespace: testNs.Name,
				},
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{
						Class: "docker-ferrol-1-27-v6",
					},
				},
			}
		})

		AfterEach(func() {
			Eventually(func() error {
				return testEnv.Cleanup(ctx, &cluster, clusterStack)
			}, timeout, interval).Should(BeNil())
		})

		It("should not allow delete if ClusterStack is in use by Cluster", func() {
			Expect(testEnv.Create(ctx, &cluster)).To(Succeed())
			Expect(testEnv.Delete(ctx, clusterStack)).ToNot(Succeed())
		})

		It("should allow delete if existing Clusters reference ClusterClasses that do not follow the cluster stack naming convention", func() {
			cluster.Spec.Topology.Class = "test-cluster-class"
			Expect(testEnv.Create(ctx, &cluster)).To(Succeed())

			Expect(testEnv.Delete(ctx, clusterStack)).To(Succeed())
		})

		It("should allow delete if existing Clusters reference different ClusterClasses", func() {
			cluster.Spec.Topology.Class = "docker-ferrol-1-25-v1"
			Expect(testEnv.Create(ctx, &cluster)).To(Succeed())

			Expect(testEnv.Delete(ctx, clusterStack)).To(Succeed())
		})
	})
})

func waitUntilChildObjectsAreCreated(cs *csov1alpha1.ClusterStack) {
	providerKind := "TestInfrastructureProviderClusterStackRelease"
	for _, csrVersion := range cs.Spec.Versions {
		csoClusterStack, err := clusterstack.New(cs.Spec.Provider, cs.Spec.Name, cs.Spec.KubernetesVersion, csrVersion)
		Expect(err).To(BeNil())
		key := types.NamespacedName{Name: csoClusterStack.String(), Namespace: cs.Namespace}
		Eventually(func() error {
			return testEnv.Get(ctx, key, &csov1alpha1.ClusterStackRelease{})
		}, 3*timeout, interval).Should(BeNil())
		Eventually(func() bool {
			providerObj, err := external.Get(ctx, testEnv.GetClient(), &corev1.ObjectReference{
				APIVersion: "infrastructure.clusterstack.x-k8s.io/v1alpha1",
				Kind:       providerKind,
				Name:       csoClusterStack.String(),
				Namespace:  cs.Namespace,
			}, cs.Namespace)
			if err != nil {
				fmt.Printf("%s %s not found. %s\n", providerKind, csoClusterStack.String(), err.Error())
				return false
			}
			fmt.Printf("foundProviderclusterStackReleaseRef is found. %s Finalizers: %+v OwnerRefs: %+v\n",
				providerObj.GetName(), providerObj.GetFinalizers(), providerObj.GetOwnerReferences())
			if len(providerObj.GetOwnerReferences()) == 0 {
				fmt.Printf("    Missing ownerRefs\n")
				return false
			}
			csoObj := &csov1alpha1.ClusterStackRelease{}
			err = testEnv.Get(ctx, key, csoObj)
			if err != nil {
				fmt.Printf("ClusterStackRelease %s not found. %s\n", csoClusterStack.Name, err.Error())
				return false
			}
			return true
		}, timeout, interval).Should(BeTrue())
	}
}
