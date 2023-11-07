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
	"github.com/SovereignCloudStack/cluster-stack-operator/internal/controller"
	"github.com/SovereignCloudStack/cluster-stack-operator/pkg/csoversion"
	githubclient "github.com/SovereignCloudStack/cluster-stack-operator/pkg/github/client"
	"github.com/SovereignCloudStack/cluster-stack-operator/pkg/kube"
	"github.com/SovereignCloudStack/cluster-stack-operator/pkg/utillog"
	"github.com/SovereignCloudStack/cluster-stack-operator/pkg/workloadcluster"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/record"
	ctrl "sigs.k8s.io/controller-runtime"
	controllerruntimecontroller "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
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
	qps                            float64
	burst                          int
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
	flag.Float64Var(&qps, "qps", 50, "Enable custom query per second for kubernetes API server")
	flag.IntVar(&burst, "burst", 100, "Enable custom burst defines how many queries the API server will accept before enforcing the limit established by qps")

	flag.Parse()

	ctrl.SetLogger(utillog.GetDefaultLogger(logLevel))

	syncPeriod := 5 * time.Minute

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                        scheme,
		MetricsBindAddress:            metricsAddr,
		Port:                          9443,
		HealthProbeBindAddress:        probeAddr,
		LeaderElection:                enableLeaderElection,
		LeaderElectionID:              "clusterstack.x-k8s.io",
		LeaderElectionNamespace:       leaderElectionNamespace,
		LeaderElectionResourceLock:    "leases",
		LeaderElectionReleaseOnCancel: true,
		Namespace:                     watchNamespace,
		SyncPeriod:                    &syncPeriod,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Initialize event recorder.
	record.InitFromRecorder(mgr.GetEventRecorderFor("clusterstack-controller"))

	// Setup the context that's going to be used in controllers and for the manager.
	ctx := ctrl.SetupSignalHandler()

	gitFactory := githubclient.NewFactory()

	restConfig := mgr.GetConfig()
	restConfig.QPS = float32(qps)
	restConfig.Burst = burst

	restConfigSettings := controller.RestConfigSettings{
		QPS:   float32(qps),
		Burst: burst,
	}

	var wg sync.WaitGroup
	wg.Add(1)

	if err = (&controller.ClusterStackReconciler{
		Client:              mgr.GetClient(),
		ReleaseDirectory:    releaseDir,
		GitHubClientFactory: gitFactory,
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
		GitHubClientFactory: gitFactory,
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

	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager", "version", csoversion.Get().String())
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}

	wg.Done()
	// Wait for all target cluster managers to gracefully shut down.
	wg.Wait()
}
