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
	timeout  = time.Second * 3
	interval = 100 * time.Millisecond

	testClusterStackName   = "docker-ferrol-1-27-v1"
	testClusterStackNameV2 = "docker-ferrol-1-27-v2"
	testClusterStackNameV3 = "docker-ferrol-1-27-v3"

	testNewWayClusterStackName = "docker-ferrol-1-27-v0-sha-hipstsw"
	testNewWayClusterClassName = "docker-ferrol-1-27-v0-sha.hipstsw"
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
		GitHubClientFactory: testEnv.GitHubClientFactory,
	}).SetupWithManager(ctx, testEnv.Manager, controllerruntimecontroller.Options{})).To(Succeed())

	Expect((&controller.ClusterStackReleaseReconciler{
		Client:              testEnv.Manager.GetClient(),
		RESTConfig:          testEnv.Manager.GetConfig(),
		KubeClientFactory:   kube.NewFactory(),
		GitHubClientFactory: testEnv.GitHubClientFactory,
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
	}).SetupWithManager(ctx, testEnv.Manager, controllerruntimecontroller.Options{})).To(Succeed())

	go func() {
		defer GinkgoRecover()
		Expect(testEnv.StartManager(ctx)).To(Succeed())
	}()

	<-testEnv.Manager.Elected()
})

var _ = AfterSuite(func() {
	Expect(testEnv.Stop()).To(Succeed())
})
