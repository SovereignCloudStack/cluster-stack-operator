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

// Package main is the main package to run the cluster-stack-operator.
package main

import (
	"flag"
	"fmt"
	"os"
	"sync"
	"time"

	//+kubebuilder:scaffold:imports
	csov1alpha1 "github.com/SovereignCloudStack/cluster-stack-operator/api/v1alpha1"
	"github.com/SovereignCloudStack/cluster-stack-operator/extension"
	"github.com/SovereignCloudStack/cluster-stack-operator/internal/controller"
	"github.com/SovereignCloudStack/cluster-stack-operator/pkg/assetsclient"
	"github.com/SovereignCloudStack/cluster-stack-operator/pkg/assetsclient/fake"
	"github.com/SovereignCloudStack/cluster-stack-operator/pkg/assetsclient/github"
	"github.com/SovereignCloudStack/cluster-stack-operator/pkg/csoversion"
	"github.com/SovereignCloudStack/cluster-stack-operator/pkg/kube"
	"github.com/SovereignCloudStack/cluster-stack-operator/pkg/utillog"
	"github.com/SovereignCloudStack/cluster-stack-operator/pkg/workloadcluster"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/remote"
	runtimecatalog "sigs.k8s.io/cluster-api/exp/runtime/catalog"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	"sigs.k8s.io/cluster-api/exp/runtime/server"
	"sigs.k8s.io/cluster-api/util/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	controllerruntimecontroller "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")

	// catalog contains all information about RuntimeHooks.
	catalog = runtimecatalog.New()
)

func init() {
	// Adds to the catalog all the RuntimeHooks defined in cluster API.
	_ = runtimehooksv1.AddToCatalog(catalog)

	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(csov1alpha1.AddToScheme(scheme))
	utilruntime.Must(clusterv1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

var (
	metricsAddr                    string
	probeAddr                      string
	enableLeaderElection           bool
	leaderElectionNamespace        string
	watchFilterValue               string
	watchNamespace                 string
	clusterStackConcurrency        int
	clusterStackReleaseConcurrency int
	clusterAddonConcurrency        int
	logLevel                       string
	releaseDir                     string
	localMode                      bool
	qps                            float64
	burst                          int
	hookPort                       int
	hookCertDir                    string
)

func main() {
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":9440", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", true, "Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&leaderElectionNamespace, "leader-elect-namespace", "", "Namespace that the controller performs leader election in. If unspecified, the controller will discover which namespace it is running in.")
	flag.StringVar(&watchFilterValue, "watch-filter", "", fmt.Sprintf("Label value that the controller watches to reconcile cluster-api objects. Label key is always %s. If unspecified, the controller watches for all cluster-api objects.", clusterv1.WatchLabel))
	flag.StringVar(&watchNamespace, "namespace", "", "Namespace that the controller watches to reconcile cluster-api objects. If unspecified, the controller watches for cluster-api objects across all namespaces.")
	flag.IntVar(&clusterStackConcurrency, "clusterstack-concurrency", 1, "Number of ClusterStacks to process simultaneously")
	flag.IntVar(&clusterStackReleaseConcurrency, "clusterstackrelease-concurrency", 1, "Number of ClusterStackReleases to process simultaneously")
	flag.IntVar(&clusterAddonConcurrency, "clusteraddon-concurrency", 1, "Number of ClusterAddons to process simultaneously")
	flag.StringVar(&logLevel, "log-level", "info", "Specifies log level. Options are 'debug', 'info' and 'error'")
	flag.StringVar(&releaseDir, "release-dir", "/tmp/downloads/", "Specify release directory for cluster-stack releases")
	flag.BoolVar(&localMode, "local", false, "Enable local mode where no release assets will be downloaded from a remote Git repository. Useful for implementing cluster stacks.")
	flag.Float64Var(&qps, "qps", 50, "Enable custom query per second for kubernetes API server")
	flag.IntVar(&burst, "burst", 100, "Enable custom burst defines how many queries the API server will accept before enforcing the limit established by qps")
	flag.IntVar(&hookPort, "hook-port", 9442, "hook server port")
	flag.StringVar(&hookCertDir, "hook-cert-dir", "/tmp/k8s-hook-server/serving-certs/", "hook cert dir, only used when hook-port is specified.")
	flag.Parse()

	ctrl.SetLogger(utillog.GetDefaultLogger(logLevel))

	syncPeriod := 5 * time.Minute

	var watchNamespaces map[string]cache.Config
	if watchNamespace != "" {
		watchNamespaces = map[string]cache.Config{
			watchNamespace: {},
		}
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		HealthProbeBindAddress: probeAddr,
		WebhookServer: webhook.NewServer(webhook.Options{
			Port: 9443,
		}),
		LeaderElection:                enableLeaderElection,
		LeaderElectionID:              "clusterstack.x-k8s.io",
		LeaderElectionNamespace:       leaderElectionNamespace,
		LeaderElectionResourceLock:    "leases",
		LeaderElectionReleaseOnCancel: true,
		Cache: cache.Options{
			SyncPeriod:        &syncPeriod,
			DefaultNamespaces: watchNamespaces,
		},
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Initialize event recorder.
	record.InitFromRecorder(mgr.GetEventRecorderFor("clusterstack-controller"))

	// Setup the context that's going to be used in controllers and for the manager.
	ctx := ctrl.SetupSignalHandler()

	var assetsClientFactory assetsclient.Factory
	if localMode {
		assetsClientFactory = fake.NewFactory()
	} else {
		assetsClientFactory = github.NewFactory()
	}

	restConfig := mgr.GetConfig()
	restConfig.QPS = float32(qps)
	restConfig.Burst = burst

	restConfigSettings := controller.RestConfigSettings{
		QPS:   float32(qps),
		Burst: burst,
	}

	var wg sync.WaitGroup

	if err = (&controller.ClusterStackReconciler{
		Client:              mgr.GetClient(),
		ReleaseDirectory:    releaseDir,
		AssetsClientFactory: assetsClientFactory,
		WatchFilterValue:    watchFilterValue,
	}).SetupWithManager(ctx, mgr, controllerruntimecontroller.Options{MaxConcurrentReconciles: clusterStackConcurrency}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ClusterStack")
		os.Exit(1)
	}

	if err = (&controller.ClusterStackReleaseReconciler{
		Client:              mgr.GetClient(),
		RESTConfig:          restConfig,
		ReleaseDirectory:    releaseDir,
		WatchFilterValue:    watchFilterValue,
		KubeClientFactory:   kube.NewFactory(),
		AssetsClientFactory: assetsClientFactory,
	}).SetupWithManager(ctx, mgr, controllerruntimecontroller.Options{MaxConcurrentReconciles: clusterStackReleaseConcurrency}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ClusterStackRelease")
		os.Exit(1)
	}

	if err = (&controller.ClusterAddonReconciler{
		Client:                 mgr.GetClient(),
		ReleaseDirectory:       releaseDir,
		RestConfigSettings:     restConfigSettings,
		WatchFilterValue:       watchFilterValue,
		KubeClientFactory:      kube.NewFactory(),
		WorkloadClusterFactory: workloadcluster.NewFactory(),
		AssetsClientFactory:    assetsClientFactory,
	}).SetupWithManager(ctx, mgr, controllerruntimecontroller.Options{MaxConcurrentReconciles: clusterAddonConcurrency}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ClusterAddon")
		os.Exit(1)
	}

	if err = (&controller.ClusterAddonCreateReconciler{
		Client: mgr.GetClient(),
	}).SetupWithManager(ctx, mgr, controllerruntimecontroller.Options{}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ClusterAddonCreate")
		os.Exit(1)
	}

	setUpWebhookWithManager(mgr)
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	// Create a http server for serving runtime extensions
	hookServer, err := server.New(server.Options{
		Catalog: catalog,
		Port:    hookPort,
		CertDir: hookCertDir,
	})
	if err != nil {
		setupLog.Error(err, "error creating hook server")
		os.Exit(1)
	}

	// Lifecycle Hooks
	// Gets a client to access the Kubernetes cluster where this RuntimeExtension will be deployed to;
	// this is a requirement specific of the lifecycle hooks implementation for Cluster APIs E2E tests.
	restConfig.UserAgent = remote.DefaultClusterAPIUserAgent("cluster-stack-operator-extension-manager")

	// Create the ExtensionHandlers for the lifecycle hooks
	lifecycleExtensionHandlers := extension.NewHandler(mgr.GetClient())

	setupLog.Info("Add extension handlers")
	if err := hookServer.AddExtensionHandler(server.ExtensionHandler{
		Hook:        runtimehooksv1.BeforeClusterUpgrade,
		Name:        "before-cluster-upgrade",
		HandlerFunc: lifecycleExtensionHandlers.DoBeforeClusterUpgrade,
	}); err != nil {
		setupLog.Error(err, "error adding handler")
		os.Exit(1)
	}

	if err := hookServer.AddExtensionHandler(server.ExtensionHandler{
		Hook:        runtimehooksv1.AfterClusterUpgrade,
		Name:        "after-cluster-upgrade",
		HandlerFunc: lifecycleExtensionHandlers.DoAfterClusterUpgrade,
	}); err != nil {
		setupLog.Error(err, "error adding handler")
		os.Exit(1)
	}

	if err := hookServer.AddExtensionHandler(server.ExtensionHandler{
		Hook:        runtimehooksv1.AfterControlPlaneInitialized,
		Name:        "after-control-plane-initialized",
		HandlerFunc: lifecycleExtensionHandlers.DoAfterControlPlaneInitialized,
	}); err != nil {
		setupLog.Error(err, "error adding handler")
		os.Exit(1)
	}

	errChan := make(chan error, 1)

	wg.Add(1)

	go func() {
		setupLog.Info("starting manager", "version", csoversion.Get().String())
		if err := mgr.Start(ctx); err != nil {
			setupLog.Error(err, "problem running manager")
			errChan <- err
		}
		wg.Done()
	}()

	wg.Add(1)

	go func() {
		setupLog.Info("starting hook server")
		if err := hookServer.Start(ctx); err != nil {
			setupLog.Error(err, "problem running hook server")
			errChan <- err
		}
		wg.Done()
	}()

	go func() {
		select {
		case err := <-errChan:
			setupLog.Error(err, "Received error")
			os.Exit(1)
		case <-ctx.Done():
			setupLog.Info("shutting down")
		}
	}()

	// wait for all processes to shut down
	wg.Wait()
}

func setUpWebhookWithManager(mgr ctrl.Manager) {
	if err := (&csov1alpha1.ClusterStackWebhook{
		LocalMode: localMode,
		Client:    mgr.GetClient(),
	}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "ClusterStack")
		os.Exit(1)
	}
	if err := (&csov1alpha1.ClusterAddon{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "ClusterAddon")
		os.Exit(1)
	}
	if err := (&csov1alpha1.Cluster{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "Cluster")
		os.Exit(1)
	}
	if err := (&csov1alpha1.ClusterStackReleaseWebhook{Client: mgr.GetClient()}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "ClusterStackRelease")
		os.Exit(1)
	}
}
