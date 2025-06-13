/*
Copyright 2022 The Kubernetes Authors.

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

// Package helpers includes helper functions important for unit and integration testing.
package helpers

import (
	"context"
	"fmt"
	"go/build"
	"os"
	"path"
	"path/filepath"
	goruntime "runtime"
	"strconv"
	"strings"
	"time"

	csov1alpha1 "github.com/SovereignCloudStack/cluster-stack-operator/api/v1alpha1"
	"github.com/SovereignCloudStack/cluster-stack-operator/internal/test/helpers/builder"
	"github.com/SovereignCloudStack/cluster-stack-operator/pkg/assetsclient"
	assetsclientmocks "github.com/SovereignCloudStack/cluster-stack-operator/pkg/assetsclient/mocks"
	kubeclient "github.com/SovereignCloudStack/cluster-stack-operator/pkg/kube"
	kubemocks "github.com/SovereignCloudStack/cluster-stack-operator/pkg/kube/mocks"
	"github.com/SovereignCloudStack/cluster-stack-operator/pkg/test/utils"
	g "github.com/onsi/ginkgo/v2"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/textlogger"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/cmd/clusterctl/log"
	dockerv1 "sigs.k8s.io/cluster-api/test/infrastructure/docker/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/kubeconfig"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	kind "sigs.k8s.io/kind/pkg/cluster"
)

func init() {
	klog.InitFlags(nil)
	logger := textlogger.NewLogger(textlogger.NewConfig())

	// use klog as the internal logger for this envtest environment.
	log.SetLogger(logger)
	// additionally force all of the controllers to use the Ginkgo logger.
	ctrl.SetLogger(logger)
	// add logger for ginkgo
	klog.SetOutput(g.GinkgoWriter)
}

var (
	scheme = runtime.NewScheme()
	env    *envtest.Environment
	ctx    = context.Background()
)

const (
	// DefaultKindClusterName is the name for workload kind cluster.
	DefaultKindClusterName = "cso-kind-workload-cluster"

	// DefaultPodNamespace is default the namespace for the envtest resources.
	DefaultPodNamespace = "cso-system"

	// defaultKindClusterNodeImage is default node image for kind cluster.
	defaultKindClusterNodeImage = "kindest/node:v1.26.6@sha256:6e2d8b28a5b601defe327b98bd1c2d1930b49e5d8c512e1895099e4504007adb" //#nosec
)

const (
	packageName = "sigs.k8s.io/cluster-api"
)

func init() {
	// Calculate the scheme.
	utilruntime.Must(clusterv1.AddToScheme(scheme))
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(csov1alpha1.AddToScheme(scheme))
	utilruntime.Must(dockerv1.AddToScheme(scheme))

	// Get the root of the current file to use in CRD paths.
	_, filename, _, _ := goruntime.Caller(0) //nolint:dogsled // external function
	root := path.Join(path.Dir(filename), "..", "..", "..")

	crdPaths := []string{
		filepath.Join(root, "config", "crd", "bases"),
	}

	if capiBootstrapPath := getFilePathToCAPIBootstrapCRDs(root); capiBootstrapPath != "" {
		crdPaths = append(crdPaths, capiBootstrapPath)
	}

	if capiControlPlanePath := getFilePathToCAPIControlPlaneCRDs(root); capiControlPlanePath != "" {
		crdPaths = append(crdPaths, capiControlPlanePath)
	}

	if capiPath := getFilePathToCAPICRDs(root); capiPath != "" {
		crdPaths = append(crdPaths, capiPath)
	}

	if capiDockerPath := getFilePathToCAPIDockerCRDs(root); capiDockerPath != "" {
		crdPaths = append(crdPaths, capiDockerPath)
	}

	// Create the test environment.
	env = &envtest.Environment{
		Scheme:                scheme,
		ErrorIfCRDPathMissing: true,
		CRDDirectoryPaths:     crdPaths,
		CRDs: []*apiextensionsv1.CustomResourceDefinition{
			builder.TestInfrastructureProviderClusterStackReleaseTemplateCRD.DeepCopy(),
			builder.TestInfrastructureProviderClusterStackReleaseCRD.DeepCopy(),
		},
	}
}

type (
	// TestEnvironment encapsulates a Kubernetes local test environment.
	TestEnvironment struct {
		ctrl.Manager
		client.Client
		KubeConfig            string
		Config                *rest.Config
		cancel                context.CancelFunc
		kind                  *kind.Provider
		WorkloadClusterClient *kubernetes.Clientset
		AssetsClientFactory   assetsclient.Factory
		KubeClientFactory     kubeclient.Factory
		AssetsClient          *assetsclientmocks.Client
		KubeClient            *kubemocks.Client
	}
)

// NewTestEnvironment creates a new environment spinning up a local api-server.
func NewTestEnvironment() *TestEnvironment {
	// initialize webhook here to be able to test the envtest install via webhookOptions
	initializeWebhookInEnvironment()

	config, err := env.Start()
	if err != nil {
		klog.Fatalf("unable to start env: %s", err)
	}

	// Build the controller manager.
	mgr, err := ctrl.NewManager(config, ctrl.Options{
		Scheme: env.Scheme,
		WebhookServer: webhook.NewServer(
			webhook.Options{
				Port:    env.WebhookInstallOptions.LocalServingPort,
				CertDir: env.WebhookInstallOptions.LocalServingCertDir,
				Host:    "localhost",
			},
		),
	})
	if err != nil {
		klog.Fatalf("unable to create manager: %s", err)
	}

	if err := (&csov1alpha1.ClusterStackWebhook{Client: mgr.GetClient()}).SetupWebhookWithManager(mgr); err != nil {
		klog.Fatalf("failed to set up webhook with manager for ClusterStack: %s", err)
	}
	if err := (&csov1alpha1.ClusterAddon{}).SetupWebhookWithManager(mgr); err != nil {
		klog.Fatalf("failed to set up webhook with manager for ClusterAddon: %s", err)
	}
	if err := (&csov1alpha1.Cluster{}).SetupWebhookWithManager(mgr); err != nil {
		klog.Fatalf("failed to set up webhook with manager for Cluster: %s", err)
	}
	if err := (&csov1alpha1.ClusterStackReleaseWebhook{Client: mgr.GetClient()}).SetupWebhookWithManager(mgr); err != nil {
		klog.Fatalf("failed to set up webhook with manager for ClusterStackRelease: %s", err)
	}

	// create manager pod namespace
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: DefaultPodNamespace,
		},
	}

	if err = mgr.GetClient().Create(ctx, ns); err != nil {
		klog.Fatalf("unable to create manager pod namespace: %s", err)
	}

	assetsClient := &assetsclientmocks.Client{}
	kubeClient := &kubemocks.Client{}

	testEnv := &TestEnvironment{
		Manager:             mgr,
		Client:              mgr.GetClient(),
		Config:              mgr.GetConfig(),
		AssetsClientFactory: assetsclientmocks.NewAssetsClientFactory(assetsClient),
		KubeClientFactory:   kubemocks.NewKubeFactory(kubeClient),
		AssetsClient:        assetsClient,
		KubeClient:          kubeClient,
	}

	if ifCreateKind() {
		// Create kind cluster
		klog.Info("creating kind cluster")

		cluster := kind.NewProvider(kind.ProviderWithDocker())
		err = cluster.Create(
			DefaultKindClusterName,
			kind.CreateWithWaitForReady(time.Minute*2),
			kind.CreateWithNodeImage(defaultKindClusterNodeImage),
		)
		if err != nil {
			klog.Fatalf("unable to create kind cluster: %s", err)
		}
		klog.Infof("kind cluster created: %s", DefaultKindClusterName)

		// Get kind cluster kubeconfig
		testEnv.KubeConfig, err = cluster.KubeConfig(DefaultKindClusterName, false)
		if err != nil {
			klog.Fatalf("unable to get kubeconfig: %s", err)
		}

		testEnv.kind = cluster

		clientCfg, err := clientcmd.NewClientConfigFromBytes([]byte(testEnv.KubeConfig))
		if err != nil {
			klog.Fatalf("failed to create client config from secret: %v", err)
		}

		restConfig, err := clientCfg.ClientConfig()
		if err != nil {
			klog.Fatalf("failed to get rest config from client config: %v", err)
		}

		testEnv.WorkloadClusterClient, err = kubernetes.NewForConfig(restConfig)
		if err != nil {
			klog.Fatalf("failed to get client set from rest config: %v", err)
		}
	}

	return testEnv
}

// StartManager starts the manager and sets a cancel function into the testEnv object.
func (t *TestEnvironment) StartManager(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	t.cancel = cancel
	if err := t.Start(ctx); err != nil {
		return fmt.Errorf("failed to start manager: %w", err)
	}
	return nil
}

// Stop stops the manager and cancels the context.
func (t *TestEnvironment) Stop() error {
	if ifCreateKind() {
		klog.Info("Deleting kind cluster")
		err := t.kind.Delete(DefaultKindClusterName, "./kubeconfig")
		if err != nil {
			if !strings.Contains(err.Error(), "failed to update kubeconfig:") {
				klog.Errorf("unable to delete kind cluster: %s", err)
			}
		}
		klog.Info("successfully deleted kind cluster")
	}

	t.cancel()
	if err := env.Stop(); err != nil {
		return fmt.Errorf("failed to stop environment; %w", err)
	}
	return nil
}

// Cleanup deletes client objects.
func (t *TestEnvironment) Cleanup(ctx context.Context, objs ...client.Object) error {
	errs := make([]error, 0, len(objs))
	for _, o := range objs {
		err := t.Delete(ctx, o)
		if apierrors.IsNotFound(err) {
			// If the object is not found, it must've been garbage collected
			// already. For example, if we delete namespace first and then
			// objects within it.
			continue
		}
		errs = append(errs, err)
	}
	return kerrors.NewAggregate(errs)
}

// CreateNamespace creates a namespace.
func (t *TestEnvironment) CreateNamespace(ctx context.Context, generateName string) (*corev1.Namespace, error) {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", generateName),
			Labels: map[string]string{
				"testenv/original-name": generateName,
			},
		},
	}
	if err := t.Create(ctx, ns); err != nil {
		return nil, fmt.Errorf("failed to create namespace: %w", err)
	}

	return ns, nil
}

// CreateKubeconfigSecret generates a kubeconfig secret in a given capi cluster.
func (t *TestEnvironment) CreateKubeconfigSecret(ctx context.Context, cluster *clusterv1.Cluster) error {
	if err := t.Create(ctx, kubeconfig.GenerateSecret(cluster, kubeconfig.FromEnvTestConfig(t.Config, cluster))); err != nil {
		return fmt.Errorf("failed to create secret: %w", err)
	}
	return nil
}

func getFilePathToCAPICRDs(root string) string {
	mod, err := utils.NewMod(filepath.Join(root, "go.mod"))
	if err != nil {
		return ""
	}

	clusterAPIVersion, err := mod.FindDependencyVersion(packageName)
	if err != nil {
		return ""
	}

	gopath := envOr("GOPATH", build.Default.GOPATH)
	return filepath.Join(gopath, "pkg", "mod", "sigs.k8s.io", fmt.Sprintf("cluster-api@%s", clusterAPIVersion), "config", "crd", "bases")
}

func getFilePathToCAPIDockerCRDs(root string) string {
	mod, err := utils.NewMod(filepath.Join(root, "go.mod"))
	if err != nil {
		return ""
	}

	clusterAPIVersion, err := mod.FindDependencyVersion(packageName)
	if err != nil {
		return ""
	}

	gopath := envOr("GOPATH", build.Default.GOPATH)
	return filepath.Join(gopath, "pkg", "mod", "sigs.k8s.io", "cluster-api", fmt.Sprintf("test@%s", clusterAPIVersion), "infrastructure", "docker", "config", "crd", "bases")
}

func getFilePathToCAPIControlPlaneCRDs(root string) string {
	mod, err := utils.NewMod(filepath.Join(root, "go.mod"))
	if err != nil {
		return ""
	}

	clusterAPIVersion, err := mod.FindDependencyVersion(packageName)
	if err != nil {
		return ""
	}

	gopath := envOr("GOPATH", build.Default.GOPATH)
	return filepath.Join(gopath, "pkg", "mod", "sigs.k8s.io", fmt.Sprintf("cluster-api@%s", clusterAPIVersion), "controlplane", "kubeadm", "config", "crd", "bases")
}

func getFilePathToCAPIBootstrapCRDs(root string) string {
	mod, err := utils.NewMod(filepath.Join(root, "go.mod"))
	if err != nil {
		return ""
	}

	clusterAPIVersion, err := mod.FindDependencyVersion(packageName)
	if err != nil {
		return ""
	}

	gopath := envOr("GOPATH", build.Default.GOPATH)
	return filepath.Join(gopath, "pkg", "mod", "sigs.k8s.io", fmt.Sprintf("cluster-api@%s", clusterAPIVersion), "bootstrap", "kubeadm", "config", "crd", "bases")
}

func envOr(envKey, defaultValue string) string {
	if value, ok := os.LookupEnv(envKey); ok {
		return value
	}
	return defaultValue
}

// IfCreateKind returns that to create kind cluster or not.
func ifCreateKind() bool {
	createKind, ok := os.LookupEnv("CREATE_KIND_CLUSTER")
	// default to true if not set
	if createKind == "" || !ok {
		createKind = "true"
	}
	ifCreate, err := strconv.ParseBool(createKind)
	if err != nil {
		klog.Fatalf("unable to parse CREATE_KIND_CLUSTER value: %s", err)
	}

	return ifCreate
}
