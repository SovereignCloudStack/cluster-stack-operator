/*
Copyright 2024 The Kubernetes Authors.

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

package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/logs"
	logsv1 "k8s.io/component-base/logs/api/v1"
	_ "k8s.io/component-base/logs/json/register"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	csov1alpha1 "github.com/SovereignCloudStack/cluster-stack-operator/api/v1alpha1"
	"github.com/SovereignCloudStack/cluster-stack-operator/extension/handlers"
	"sigs.k8s.io/cluster-api/controllers/remote"
	runtimecatalog "sigs.k8s.io/cluster-api/exp/runtime/catalog"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	"sigs.k8s.io/cluster-api/exp/runtime/server"
	"sigs.k8s.io/cluster-api/version"
)

var (
	// catalog contains all information about RuntimeHooks.
	catalog = runtimecatalog.New()

	// scheme is a Kubernetes runtime scheme containing all the information about API types used by the test extension.
	// NOTE: it is not mandatory to use scheme in custom RuntimeExtension, but working with typed API objects makes code
	// easier to read and less error prone than using unstructured or working with raw json/yaml.
	scheme         = runtime.NewScheme()
	controllerName = "cluster-stack-operator-extension-manager"

	// Flags.
	profilerAddress string
	webhookPort     int
	webhookCertDir  string
	logOptions      = logs.NewOptions()
)

func init() {
	// Adds to the catalog all the RuntimeHooks defined in cluster API.
	_ = runtimehooksv1.AddToCatalog(catalog)

	// Adds to the scheme all the API types we used by the test extension.
	_ = csov1alpha1.AddToScheme(scheme)
}

// InitFlags initializes the flags.
func InitFlags(fs *pflag.FlagSet) {
	// Initialize logs flags using Kubernetes component-base machinery.
	logsv1.AddFlags(logOptions, fs)

	// Add test-extension specific flags
	fs.StringVar(&profilerAddress, "profiler-address", "localhost:6060",
		"Bind address to expose the pprof profiler (e.g. localhost:6060)")

	fs.IntVar(&webhookPort, "webhook-port", 9443,
		"Webhook Server port")

	fs.StringVar(&webhookCertDir, "webhook-cert-dir", "/tmp/k8s-webhook-server/serving-certs/",
		"Webhook cert dir, only used when webhook-port is specified.")
}

func main() {
	// Creates a logger to be used during the main func using controller runtime utilities
	setupLog := ctrl.Log.WithName("main")

	// Initialize and parse command line flags.
	InitFlags(pflag.CommandLine)
	pflag.CommandLine.SetNormalizeFunc(cliflag.WordSepNormalizeFunc)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	// Set log level 2 as default.
	if err := pflag.CommandLine.Set("v", "2"); err != nil {
		setupLog.Error(err, "failed to set log level: %v")
		os.Exit(1)
	}
	pflag.Parse()

	// Validates logs flags using Kubernetes component-base machinery and apply them
	// so klog will automatically use the right logger.
	if err := logsv1.ValidateAndApply(logOptions, nil); err != nil {
		setupLog.Error(err, "unable to start extension")
		os.Exit(1)
	}

	// Add the klog logger in the context.
	ctrl.SetLogger(klog.Background())

	// Initialize the golang profiler server, if required.
	if profilerAddress != "" {
		setupLog.Info(fmt.Sprintf("Profiler listening for requests at %s", profilerAddress))
		go func() {
			srv := http.Server{Addr: profilerAddress, ReadHeaderTimeout: 2 * time.Second}
			if err := srv.ListenAndServe(); err != nil {
				setupLog.Error(err, "problem running profiler server")
			}
		}()
	}

	// Create a http server for serving runtime extensions
	webhookServer, err := server.New(server.Options{
		Catalog: catalog,
		Port:    webhookPort,
		CertDir: webhookCertDir,
	})
	if err != nil {
		setupLog.Error(err, "error creating webhook server")
		os.Exit(1)
	}

	// Lifecycle Hooks
	// Gets a client to access the Kubernetes cluster where this RuntimeExtension will be deployed to;
	// this is a requirement specific of the lifecycle hooks implementation for Cluster APIs E2E tests.
	restConfig, err := ctrl.GetConfig()
	if err != nil {
		setupLog.Error(err, "error getting config for the cluster")
		os.Exit(1)
	}
	restConfig.UserAgent = remote.DefaultClusterAPIUserAgent(controllerName)

	client, err := client.New(restConfig, client.Options{Scheme: scheme})
	if err != nil {
		setupLog.Error(err, "error creating client to the cluster")
		os.Exit(1)
	}

	// Create the ExtensionHandlers for the lifecycle hooks
	lifecycleExtensionHandlers := handlers.NewExtensionHandlers(client, scheme)

	if err := webhookServer.AddExtensionHandler(server.ExtensionHandler{
		Hook:        runtimehooksv1.BeforeClusterUpgrade,
		Name:        "before-cluster-upgrade",
		HandlerFunc: lifecycleExtensionHandlers.DoBeforeClusterUpgrade,
	}); err != nil {
		setupLog.Error(err, "error adding handler")
		os.Exit(1)
	}

	if err := webhookServer.AddExtensionHandler(server.ExtensionHandler{
		Hook:        runtimehooksv1.AfterClusterUpgrade,
		Name:        "after-cluster-upgrade",
		HandlerFunc: lifecycleExtensionHandlers.DoAfterClusterUpgrade,
	}); err != nil {
		setupLog.Error(err, "error adding handler")
		os.Exit(1)
	}

	if err := webhookServer.AddExtensionHandler(server.ExtensionHandler{
		Hook:        runtimehooksv1.AfterControlPlaneInitialized,
		Name:        "after-control-plane-initialized",
		HandlerFunc: lifecycleExtensionHandlers.DoAfterControlPlaneInitialized,
	}); err != nil {
		setupLog.Error(err, "error adding handler")
		os.Exit(1)
	}

	// Setup a context listening for SIGINT.
	ctx := ctrl.SetupSignalHandler()

	if err := webhookServer.Start(ctx); err != nil {
		setupLog.Error(err, "error running webhook server")
		os.Exit(1)
	}
	setupLog.Info("Started RuntimeExtension", "version", version.Get().String())
}
