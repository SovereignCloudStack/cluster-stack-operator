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
	"testing"
	"time"

	"github.com/SovereignCloudStack/cluster-stack-operator/internal/controller"
	"github.com/SovereignCloudStack/cluster-stack-operator/internal/test/helpers"
	githubclient "github.com/SovereignCloudStack/cluster-stack-operator/pkg/assetsclient/github"
	"github.com/SovereignCloudStack/cluster-stack-operator/pkg/kube"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	ctrl "sigs.k8s.io/controller-runtime"
	controllerruntimecontroller "sigs.k8s.io/controller-runtime/pkg/controller"
)

const (
	timeout  = time.Second * 20
	interval = 1000 * time.Millisecond
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
		AssetsClientFactory: githubclient.NewFactory(),
		ReleaseDirectory:    "/tmp/downloads",
	}).SetupWithManager(ctx, testEnv.Manager, controllerruntimecontroller.Options{})).To(Succeed())
	Expect((&controller.ClusterStackReleaseReconciler{
		Client:              testEnv.Manager.GetClient(),
		RESTConfig:          testEnv.Manager.GetConfig(),
		KubeClientFactory:   kube.NewFactory(),
		AssetsClientFactory: githubclient.NewFactory(),
		ReleaseDirectory:    "/tmp/downloads",
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
