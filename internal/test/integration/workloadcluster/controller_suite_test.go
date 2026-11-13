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
	"testing"
	"time"

	"github.com/SovereignCloudStack/cluster-stack-operator/internal/controller"
	"github.com/SovereignCloudStack/cluster-stack-operator/internal/test/helpers"
	"github.com/SovereignCloudStack/cluster-stack-operator/pkg/kube"
	"github.com/SovereignCloudStack/cluster-stack-operator/pkg/workloadcluster"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	ctrl "sigs.k8s.io/controller-runtime"
	controllerruntimecontroller "sigs.k8s.io/controller-runtime/pkg/controller"
)

const (
	timeout  = time.Second * 30
	interval = 100 * time.Millisecond

	// applyTimeout is the time for specs in which the cluster addon charts are templated and applied
	// to the workload cluster, which takes a few reconciliations.
	applyTimeout = time.Minute * 3

	// The cluster stack releases in test/releases are built from the docker/scs cluster stack of
	// https://github.com/SovereignCloudStack/cluster-stacks, see test/releases/README.md.
	//
	// Name of the release (the cluster stack release object) and of the cluster class of the first release.
	testClusterStackName = "docker-scs-1-35-v0-ttl-1792740950"
	testClusterClassName = "docker-scs-1-35-v0-ttl.1792740950"
	// Name of the release and of the cluster class of the second release, which has the same Kubernetes version.
	// It configures two replicas for the metrics-server and does not contain the gateway-api addon.
	testClusterStackNameV2 = "docker-scs-1-35-v0-ttl-1792740963"
	testClusterClassNameV2 = "docker-scs-1-35-v0-ttl.1792740963"

	testKubernetesVersion = "v1.35.5"
	testControlPlaneHost  = "test-cluster.example.com"
)

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Suite")
}

var (
	ctx     = ctrl.SetupSignalHandler()
	testEnv *helpers.TestEnvironment
)

var _ = BeforeSuite(func() {
	testEnv = helpers.NewTestEnvironment()

	Expect((&controller.ClusterStackReconciler{
		Client:              testEnv.Manager.GetClient(),
		ReleaseDirectory:    "./../../../../test/releases",
		AssetsClientFactory: testEnv.AssetsClientFactory,
	}).SetupWithManager(ctx, testEnv.Manager, controllerruntimecontroller.Options{})).To(Succeed())

	Expect((&controller.ClusterStackReleaseReconciler{
		Client:              testEnv.Manager.GetClient(),
		RESTConfig:          testEnv.Manager.GetConfig(),
		KubeClientFactory:   kube.NewFactory(),
		AssetsClientFactory: testEnv.AssetsClientFactory,
		ReleaseDirectory:    "./../../../../test/releases",
	}).SetupWithManager(ctx, testEnv.Manager, controllerruntimecontroller.Options{})).To(Succeed())

	Expect((&controller.ClusterAddonCreateReconciler{
		Client: testEnv.Manager.GetClient(),
	}).SetupWithManager(ctx, testEnv.Manager, controllerruntimecontroller.Options{})).To(Succeed())

	Expect((&controller.ClusterAddonReconciler{
		Client:                 testEnv.Manager.GetClient(),
		ReleaseDirectory:       "./../../../../test/releases",
		KubeClientFactory:      kube.NewFactory(),
		WorkloadClusterFactory: workloadcluster.NewFactory(),
		AssetsClientFactory:    testEnv.AssetsClientFactory,
	}).SetupWithManager(ctx, testEnv.Manager, controllerruntimecontroller.Options{})).To(Succeed())

	go func() {
		defer GinkgoRecover()
		Expect(testEnv.StartManager(ctx)).To(Succeed())
	}()

	<-testEnv.Elected()
})

var _ = AfterSuite(func() {
	Expect(testEnv.Stop()).To(Succeed())
})
