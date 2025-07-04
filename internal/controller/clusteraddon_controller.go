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
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"text/template"
	"time"

	csov1alpha1 "github.com/SovereignCloudStack/cluster-stack-operator/api/v1alpha1"
	"github.com/SovereignCloudStack/cluster-stack-operator/pkg/assetsclient"
	"github.com/SovereignCloudStack/cluster-stack-operator/pkg/clusteraddon"
	"github.com/SovereignCloudStack/cluster-stack-operator/pkg/kube"
	"github.com/SovereignCloudStack/cluster-stack-operator/pkg/release"
	"github.com/SovereignCloudStack/cluster-stack-operator/pkg/workloadcluster"
	sprig "github.com/go-task/slim-sprig"
	"github.com/google/cel-go/cel"
	celtypes "github.com/google/cel-go/common/types"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/external"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	"sigs.k8s.io/cluster-api/util/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const clusterAddonNamespace = "kube-system"

const (
	beforeClusterUpgradeHook     = "BeforeClusterUpgrade"
	afterControlPlaneInitialized = "AfterControlPlaneInitialized"
)

// RestConfigSettings contains Kubernetes rest config settings.
type RestConfigSettings struct {
	QPS   float32
	Burst int
}

// ClusterAddonReconciler reconciles a ClusterAddon object.
type ClusterAddonReconciler struct {
	client.Client
	RestConfigSettings
	ReleaseDirectory                      string
	KubeClientFactory                     kube.Factory
	AssetsClientFactory                   assetsclient.Factory
	WatchFilterValue                      string
	WorkloadClusterFactory                workloadcluster.Factory
	clusterStackRelDownloadDirectoryMutex sync.Mutex
}

//+kubebuilder:rbac:groups=clusterstack.x-k8s.io,resources=clusteraddons,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=clusterstack.x-k8s.io,resources=clusteraddons/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=clusterstack.x-k8s.io,resources=clusteraddons/finalizers,verbs=update
//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters,verbs=get

// Reconcile is part of the main Kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ClusterAddonReconciler) Reconcile(ctx context.Context, req reconcile.Request) (res reconcile.Result, reterr error) {
	clusterAddon := &csov1alpha1.ClusterAddon{}
	if err := r.Get(ctx, req.NamespacedName, clusterAddon); err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("failed to get cluster addon %s/%s: %w", req.Name, req.Namespace, err)
	}

	patchHelper, err := patch.NewHelper(clusterAddon, r.Client)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to init patch helper: %w", err)
	}

	defer func() {
		conditions.SetSummary(clusterAddon)

		if err := patchHelper.Patch(ctx, clusterAddon); err != nil {
			reterr = fmt.Errorf("failed to patch clusterAddon: %w", err)
		}
	}()

	controllerutil.AddFinalizer(clusterAddon, csov1alpha1.ClusterAddonFinalizer)

	// retrieve associated cluster object
	cluster := &clusterv1.Cluster{}
	clusterName := client.ObjectKey{
		Name:      clusterAddon.Spec.ClusterRef.Name,
		Namespace: clusterAddon.Spec.ClusterRef.Namespace,
	}

	if err := r.Get(ctx, clusterName, cluster); err != nil {
		if apierrors.IsNotFound(err) && !clusterAddon.DeletionTimestamp.IsZero() {
			controllerutil.RemoveFinalizer(clusterAddon, csov1alpha1.ClusterAddonFinalizer)

			return reconcile.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("failed to find cluster %v: %w", clusterName, err)
	}

	restConfigClient := r.WorkloadClusterFactory.NewClient(cluster.Name, req.Namespace, r.Client)

	restConfig, err := restConfigClient.RestConfig(ctx)
	if err != nil {
		conditions.MarkFalse(
			clusterAddon,
			csov1alpha1.ClusterReadyCondition,
			csov1alpha1.ControlPlaneNotReadyReason,
			clusterv1.ConditionSeverityWarning,
			"kubeconfig not there (yet)",
		)
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// usually this is only nil in unit tests
	if restConfig != nil {
		restConfig.QPS = r.QPS
		restConfig.Burst = r.Burst

		clientSet, err := kubernetes.NewForConfig(restConfig)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to create Kubernetes interface from config: %w", err)
		}

		if _, err := clientSet.Discovery().RESTClient().Get().AbsPath("/readyz").DoRaw(ctx); err != nil {
			conditions.MarkFalse(
				clusterAddon,
				csov1alpha1.ClusterReadyCondition,
				csov1alpha1.ControlPlaneNotReadyReason,
				clusterv1.ConditionSeverityInfo,
				"control plane not ready yet",
			)

			// wait for cluster to be ready
			return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
		}
	}

	// cluster is ready, so we set a condition and can continue as well
	conditions.MarkTrue(clusterAddon, csov1alpha1.ClusterReadyCondition)

	releaseAsset, download, err := release.New(release.ConvertFromClusterClassToClusterStackFormat(cluster.Spec.Topology.Class), r.ReleaseDirectory)
	if err != nil {
		conditions.MarkFalse(clusterAddon, csov1alpha1.ClusterStackReleaseAssetsReadyCondition, csov1alpha1.IssueWithReleaseAssetsReason, clusterv1.ConditionSeverityError, "%s", err.Error())
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}
	if download {
		conditions.MarkFalse(clusterAddon, csov1alpha1.ClusterStackReleaseAssetsReadyCondition, csov1alpha1.ReleaseAssetsNotDownloadedYetReason, clusterv1.ConditionSeverityInfo, "release assets not downloaded yet")
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// Check for helm charts in the release assets. If they are not present, then something went wrong.
	if err := releaseAsset.CheckHelmCharts(); err != nil {
		msg := fmt.Sprintf("failed to validate helm charts: %s", err.Error())
		conditions.MarkFalse(
			clusterAddon,
			csov1alpha1.ClusterStackReleaseAssetsReadyCondition,
			csov1alpha1.IssueWithReleaseAssetsReason,
			clusterv1.ConditionSeverityError,
			"%s", msg,
		)
		record.Warn(clusterAddon, "ValidateHelmChartFailed", msg)
		return reconcile.Result{}, nil
	}

	// set downloaded condition if able to read metadata file
	conditions.MarkTrue(clusterAddon, csov1alpha1.ClusterStackReleaseAssetsReadyCondition)

	in := &templateAndApplyClusterAddonInput{
		clusterAddonChartPath:  releaseAsset.ClusterAddonChartPath(),
		clusterAddonValuesPath: releaseAsset.ClusterAddonValuesPath(),
		kubernetesVersion:      releaseAsset.Meta.Versions.Kubernetes,
		clusterAddon:           clusterAddon,
		cluster:                cluster,
		restConfig:             restConfig,
	}

	in.clusterAddonConfigPath, err = r.getClusterAddonConfigPath(cluster.Spec.Topology.Class)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get cluster addon config path: %w", err)
	}

	// Check whether current Helm chart has been applied in the workload cluster. If not, then we need to apply the helm chart (again).
	// the spec.clusterStack is only set after a Helm chart from a ClusterStack has been applied successfully.
	// If it is not set, the Helm chart has never been applied.
	// If it is set and does not equal the ClusterClass of the cluster, then it is outdated and has to be updated.
	if in.clusterAddonConfigPath == "" {
		if clusterAddon.Spec.ClusterStack != cluster.Spec.Topology.Class {
			metadata := releaseAsset.Meta

			// only apply the Helm chart again if the Helm chart version has also changed from one cluster stack release to the other
			if clusterAddon.Spec.Version != metadata.Versions.Components.ClusterAddon {
				clusterAddon.Status.Ready = false

				shouldRequeue, err := r.templateAndApplyClusterAddonHelmChart(ctx, in)
				if err != nil {
					conditions.MarkFalse(clusterAddon, csov1alpha1.HelmChartAppliedCondition, csov1alpha1.FailedToApplyObjectsReason, clusterv1.ConditionSeverityError, "failed to apply: %s", err.Error())
					return ctrl.Result{}, fmt.Errorf("failed to apply helm chart: %w", err)
				}
				if shouldRequeue {
					// set latest version and requeue
					clusterAddon.Spec.Version = metadata.Versions.Components.ClusterAddon
					clusterAddon.Spec.ClusterStack = cluster.Spec.Topology.Class

					// set condition to false as we have not successfully applied Helm chart
					conditions.MarkFalse(
						clusterAddon,
						csov1alpha1.HelmChartAppliedCondition,
						csov1alpha1.FailedToApplyObjectsReason,
						clusterv1.ConditionSeverityInfo,
						"failed to successfully apply everything",
					)
					return ctrl.Result{RequeueAfter: 20 * time.Second}, nil
				}

				// Helm chart has been applied successfully
				clusterAddon.Spec.Version = metadata.Versions.Components.ClusterAddon
				conditions.MarkTrue(clusterAddon, csov1alpha1.HelmChartAppliedCondition)
			}

			clusterAddon.SetStageAnnotations(csov1alpha1.StageAnnotationValueCreated)
			clusterAddon.Spec.Hook = ""
			clusterAddon.Spec.ClusterStack = cluster.Spec.Topology.Class
			clusterAddon.Status.Ready = true
			return ctrl.Result{}, nil
		}

		// if condition is false we have not yet successfully applied the helm chart
		if conditions.IsFalse(clusterAddon, csov1alpha1.HelmChartAppliedCondition) {
			shouldRequeue, err := r.templateAndApplyClusterAddonHelmChart(ctx, in)
			if err != nil {
				conditions.MarkFalse(clusterAddon, csov1alpha1.HelmChartAppliedCondition, csov1alpha1.FailedToApplyObjectsReason, clusterv1.ConditionSeverityError, "failed to apply: %s", err.Error())
				return ctrl.Result{}, fmt.Errorf("failed to apply helm chart: %w", err)
			}
			if shouldRequeue {
				// set condition to false as we have not yet successfully applied helm chart
				conditions.MarkFalse(clusterAddon, csov1alpha1.HelmChartAppliedCondition, csov1alpha1.FailedToApplyObjectsReason, clusterv1.ConditionSeverityInfo, "failed to successfully apply everything")
				return ctrl.Result{RequeueAfter: 20 * time.Second}, nil
			}

			// set condition that helm chart has been applied successfully
			conditions.MarkTrue(clusterAddon, csov1alpha1.HelmChartAppliedCondition)
		}

		clusterAddon.Spec.Hook = ""
		clusterAddon.SetStageAnnotations(csov1alpha1.StageAnnotationValueCreated)
		clusterAddon.Status.Ready = true
		return ctrl.Result{}, nil
	}

	// multi-stage cluster addon flow
	in.addonStagesInput, err = r.getAddonStagesInput(in.restConfig, in.clusterAddonChartPath)
	if err != nil {
		conditions.MarkFalse(
			clusterAddon,
			csov1alpha1.ClusterAddonConfigValidatedCondition,
			csov1alpha1.ParsingClusterAddonConfigFailedReason,
			clusterv1.ConditionSeverityError,
			"cluster addon config (clusteraddon.yaml) is wrong: %s", err.Error(),
		)

		record.Warnf(
			clusterAddon,
			csov1alpha1.ParsingClusterAddonConfigFailedReason,
			"cluster addon config (clusteraddon.yaml) is wrong: %s", err.Error(),
		)

		return reconcile.Result{}, nil
	}
	conditions.MarkTrue(clusterAddon, csov1alpha1.ClusterAddonConfigValidatedCondition)

	// clusteraddon.yaml in the release.
	clusterAddonConfig, err := clusteraddon.ParseConfig(in.clusterAddonConfigPath)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to parse clusteraddon.yaml config: %w", err)
	}

	var (
		oldRelease *release.Release
		requeue    bool
	)

	if clusterAddon.Spec.ClusterStack != "" {
		oldRelease, requeue, err = r.downloadOldClusterStackRelease(ctx, clusterAddon)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to download old cluster stack releases: %w", err)
		}
		if requeue {
			return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
		}

		// src - /tmp/cluster-stacks/docker-ferrol-1-27-v1/docker-ferrol-1-27-cluster-addon-v1.tgz
		// dst - /tmp/cluster-stacks/docker-ferrol-1-27-v1/docker-ferrol-1-27-cluster-addon-v1/
		in.oldDestinationClusterAddonChartDir = strings.TrimSuffix(oldRelease.ClusterAddonChartPath(), ".tgz")
		in.oldKubernetesVersion = oldRelease.Meta.Versions.Kubernetes

		if err := unTarContent(oldRelease.ClusterAddonChartPath(), in.oldDestinationClusterAddonChartDir); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to untar old cluster stack cluster addon chart: %q: %w", oldRelease.ClusterAddonChartPath(), err)
		}
	}

	// if a hook is specified, we cannot be ready yet
	// if a hook is set, it is expected that HelmChartAppliedCondition is removed
	if clusterAddon.Spec.Hook != "" {
		// if the clusterAddon was ready before, it means this hook is fresh and we have to reset the status
		if clusterAddon.Status.Ready || len(clusterAddon.Status.Stages) == 0 {
			clusterAddon.Status.Stages = make([]csov1alpha1.StageStatus, len(clusterAddonConfig.AddonStages[clusterAddon.Spec.Hook]))
			for i, stage := range clusterAddonConfig.AddonStages[clusterAddon.Spec.Hook] {
				clusterAddon.Status.Stages[i].Name = stage.Name
				clusterAddon.Status.Stages[i].Action = stage.Action
				clusterAddon.Status.Stages[i].Phase = csov1alpha1.StagePhasePending
			}
		}
		clusterAddon.Status.Ready = false
	}

	// In case the Kubernetes version stays the same, the hook server does not trigger.
	// Therefore, we have to check whether the ClusterStack is upgraded and if that is the case, the ClusterAddons have to be upgraded as well.
	if clusterAddon.Spec.ClusterStack != cluster.Spec.Topology.Class && oldRelease != nil && oldRelease.Meta.Versions.Kubernetes == releaseAsset.Meta.Versions.Kubernetes {
		if clusterAddon.Spec.Version != releaseAsset.Meta.Versions.Components.ClusterAddon {
			if clusterAddon.Status.Ready || len(clusterAddon.Status.Stages) == 0 {
				clusterAddon.Status.Stages = make([]csov1alpha1.StageStatus, len(clusterAddonConfig.AddonStages[beforeClusterUpgradeHook]))
				for i, stage := range clusterAddonConfig.AddonStages[beforeClusterUpgradeHook] {
					clusterAddon.Status.Stages[i].Name = stage.Name
					clusterAddon.Status.Stages[i].Action = stage.Action
					clusterAddon.Status.Stages[i].Phase = csov1alpha1.StagePhasePending
				}
			}
			clusterAddon.Status.Ready = false
			conditions.Delete(clusterAddon, csov1alpha1.HelmChartAppliedCondition)
		} else {
			// If the cluster addon version don't change we don't want to apply helm charts again.
			clusterAddon.Spec.ClusterStack = cluster.Spec.Topology.Class
			clusterAddon.Status.Ready = true
		}
	}

	clusterAddon.Spec.Version = releaseAsset.Meta.Versions.Components.ClusterAddon

	if clusterAddon.Status.Ready {
		return reconcile.Result{}, nil
	}

	// In case the Kubernetes version stayed the same during an upgrade, the hook server does not trigger and
	// we just take the Helm charts that are supposed to be installed in the BeforeClusterUpgrade hook and apply them.
	if oldRelease != nil && oldRelease.Meta.Versions.Kubernetes == releaseAsset.Meta.Versions.Kubernetes {
		clusterAddon.Spec.Hook = beforeClusterUpgradeHook

		for _, stage := range clusterAddonConfig.AddonStages[beforeClusterUpgradeHook] {
			shouldRequeue, err := r.executeStage(ctx, stage, in)
			if err != nil {
				return reconcile.Result{}, fmt.Errorf("failed to execute stage: %w", err)
			}
			if shouldRequeue {
				return reconcile.Result{RequeueAfter: 20 * time.Second}, nil
			}
		}

		// create the list of old release objects
		oldClusterStackObjectList, err := r.getOldReleaseObjects(ctx, in, clusterAddonConfig, oldRelease)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to get old cluster stack object list from helm charts: %w", err)
		}

		newClusterStackObjectList, err := r.getNewReleaseObjects(ctx, in, clusterAddonConfig)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to get new cluster stack object list from helm charts: %w", err)
		}

		shouldRequeue, err := cleanUpResources(ctx, in, oldClusterStackObjectList, newClusterStackObjectList)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to clean up resources: %w", err)
		}
		if shouldRequeue {
			return reconcile.Result{RequeueAfter: 20 * time.Second}, nil
		}

		// set upgrade annotation once done
		clusterAddon.SetStageAnnotations(csov1alpha1.StageAnnotationValueUpgraded)

		// Helm chart has been applied successfully
		conditions.MarkTrue(clusterAddon, csov1alpha1.HelmChartAppliedCondition)

		// remove the status resource if hook is finished
		clusterAddon.Status.Resources = make([]*csov1alpha1.Resource, 0)

		// remove the helm chart status from the status.
		clusterAddon.Status.Stages = make([]csov1alpha1.StageStatus, 0)

		// update the latest cluster class
		clusterAddon.Spec.ClusterStack = cluster.Spec.Topology.Class
		clusterAddon.Status.Ready = true

		// unset spec hook and make cluster addon ready
		clusterAddon.Spec.Hook = ""

		return ctrl.Result{}, nil
	}

	// If hook is empty we can don't want to proceed executing staged according to current hook
	// hence we can return.
	if clusterAddon.Spec.Hook == "" {
		conditions.MarkFalse(clusterAddon,
			csov1alpha1.HookServerReadyCondition,
			csov1alpha1.HookServerUnresponsiveReason,
			clusterv1.ConditionSeverityInfo,
			"hook server hasn't updated the spec.hook yet",
		)
		return reconcile.Result{}, nil
	}
	conditions.MarkTrue(clusterAddon, csov1alpha1.HookServerReadyCondition)

	for _, stage := range clusterAddonConfig.AddonStages[clusterAddon.Spec.Hook] {
		shouldRequeue, err := r.executeStage(ctx, stage, in)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to execute stage: %q: %w", stage.Name, err)
		}
		if shouldRequeue {
			return reconcile.Result{RequeueAfter: 20 * time.Second}, nil
		}
	}

	if clusterAddon.Spec.Hook == afterControlPlaneInitialized || clusterAddon.Spec.Hook == beforeClusterUpgradeHook {
		if clusterAddon.Spec.Hook == beforeClusterUpgradeHook {
			// create the list of old release objects
			oldClusterStackObjectList, err := r.getOldReleaseObjects(ctx, in, clusterAddonConfig, oldRelease)
			if err != nil {
				return reconcile.Result{}, fmt.Errorf("failed to get old cluster stack object list from helm charts: %w", err)
			}

			newClusterStackObjectList, err := r.getNewReleaseObjects(ctx, in, clusterAddonConfig)
			if err != nil {
				return reconcile.Result{}, fmt.Errorf("failed to get new cluster stack object list from helm charts: %w", err)
			}

			shouldRequeue, err := cleanUpResources(ctx, in, oldClusterStackObjectList, newClusterStackObjectList)
			if err != nil {
				return reconcile.Result{}, fmt.Errorf("failed to clean up resources: %w", err)
			}
			if shouldRequeue {
				return reconcile.Result{RequeueAfter: 20 * time.Second}, nil
			}

			// set upgrade annotation once done
			clusterAddon.SetStageAnnotations(csov1alpha1.StageAnnotationValueUpgraded)
		}

		// if upgrade annotation is not present add the create annotation
		clusterAddon.SetStageAnnotations(csov1alpha1.StageAnnotationValueCreated)

		clusterAddon.Spec.ClusterStack = cluster.Spec.Topology.Class
		clusterAddon.Status.Ready = true
	}

	// Helm chart has been applied successfully
	// clusterAddon.Spec.Version = metadata.Versions.Components.ClusterAddon
	conditions.MarkTrue(clusterAddon, csov1alpha1.HelmChartAppliedCondition)

	// remove the helm chart status from the status.
	clusterAddon.Status.Stages = make([]csov1alpha1.StageStatus, 0)

	// remove the status resource if hook is finished
	clusterAddon.Status.Resources = make([]*csov1alpha1.Resource, 0)

	// unset spec hook
	clusterAddon.Spec.Hook = ""

	return ctrl.Result{}, nil
}

func (r *ClusterAddonReconciler) getNewReleaseObjects(ctx context.Context, in *templateAndApplyClusterAddonInput, clusterAddonConfig clusteraddon.Config) ([]*csov1alpha1.Resource, error) {
	var (
		newBuildTemplate []byte
		resources        []*csov1alpha1.Resource
	)

	for _, stage := range clusterAddonConfig.AddonStages[in.clusterAddon.Spec.Hook] {
		if _, err := os.Stat(filepath.Join(in.newDestinationClusterAddonChartDir, stage.Name, release.OverwriteYaml)); err == nil {
			newBuildTemplate, err = buildTemplateFromClusterAddonValues(ctx, filepath.Join(in.newDestinationClusterAddonChartDir, stage.Name, release.OverwriteYaml), in.cluster, r.Client)
			if err != nil {
				return nil, fmt.Errorf("failed to build template from new cluster addon values of the latest cluster stack: %w", err)
			}
		}

		helmTemplate, err := helmTemplateClusterAddon(filepath.Join(in.newDestinationClusterAddonChartDir, stage.Name), newBuildTemplate, in.kubernetesVersion)
		if err != nil {
			return nil, fmt.Errorf("failed to template new helm chart of the latest cluster stack: %w", err)
		}

		resource, err := kube.GetResourcesFromHelmTemplate(helmTemplate)
		if err != nil {
			return nil, fmt.Errorf("failed to get resources form old cluster stack helm template of the latest cluster stack: %w", err)
		}

		if stage.Action == clusteraddon.Apply {
			resources = append(resources, resource...)
		} else {
			resources = removeResourcesFromCurrentListOfObjects(resources, resource)
		}
	}

	return resources, nil
}

// getOldReleaseObjects returns the old cluster stack objects in the workload cluster.
func (r *ClusterAddonReconciler) getOldReleaseObjects(ctx context.Context, in *templateAndApplyClusterAddonInput, clusterAddonConfig clusteraddon.Config, oldRelease *release.Release) ([]*csov1alpha1.Resource, error) {
	// clusteraddon.yaml
	clusterAddonConfigPath, err := r.getClusterAddonConfigPath(in.clusterAddon.Spec.ClusterStack)
	if err != nil {
		return nil, fmt.Errorf("failed to get old cluster stack cluster addon config path: %w", err)
	}

	if _, err := os.Stat(clusterAddonConfigPath); err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to verify the clusteraddon.yaml on old cluster stack release path %q with error: %w", clusterAddonConfigPath, err)
		}

		// this is the old way
		buildTemplate, err := buildTemplateFromClusterAddonValues(ctx, oldRelease.ClusterAddonValuesPath(), in.cluster, r.Client)
		if err != nil {
			return nil, fmt.Errorf("failed to build template from the old cluster stack cluster addon values: %w", err)
		}

		helmTemplate, err := helmTemplateClusterAddon(oldRelease.ClusterAddonChartPath(), buildTemplate, oldRelease.Meta.Versions.Kubernetes)
		if err != nil {
			return nil, fmt.Errorf("failed to template helm chart: %w", err)
		}

		resources, err := kube.GetResourcesFromHelmTemplate(helmTemplate)
		if err != nil {
			return nil, fmt.Errorf("failed to get resources form old cluster stack helm template: %w", err)
		}

		return resources, nil
	}

	// this is the new way
	// Read all the helm charts in new the un-tared cluster addon.

	var (
		newBuildTemplate []byte
		resources        []*csov1alpha1.Resource

		hook string
	)

	if in.clusterAddon.HasStageAnnotation(csov1alpha1.StageAnnotationValueCreated) {
		hook = afterControlPlaneInitialized
	} else {
		hook = beforeClusterUpgradeHook
	}

	for _, stage := range clusterAddonConfig.AddonStages[hook] {
		if _, err := os.Stat(filepath.Join(in.oldDestinationClusterAddonChartDir, stage.Name, release.OverwriteYaml)); err == nil {
			newBuildTemplate, err = buildTemplateFromClusterAddonValues(ctx, filepath.Join(in.oldDestinationClusterAddonChartDir, stage.Name, release.OverwriteYaml), in.cluster, r.Client)
			if err != nil {
				return nil, fmt.Errorf("failed to build template from new cluster addon values: %w", err)
			}
		}

		helmTemplate, err := helmTemplateClusterAddon(filepath.Join(in.oldDestinationClusterAddonChartDir, stage.Name), newBuildTemplate, oldRelease.Meta.Versions.Kubernetes)
		if err != nil {
			return nil, fmt.Errorf("failed to template new helm chart: %w", err)
		}

		resource, err := kube.GetResourcesFromHelmTemplate(helmTemplate)
		if err != nil {
			return nil, fmt.Errorf("failed to get resources form old cluster stack helm template: %w", err)
		}

		if stage.Action == clusteraddon.Apply {
			resources = append(resources, resource...)
		} else {
			resources = removeResourcesFromCurrentListOfObjects(resources, resource)
		}
	}

	return resources, nil
}

func cleanUpResources(ctx context.Context, in *templateAndApplyClusterAddonInput, oldList, newList []*csov1alpha1.Resource) (shouldRequeue bool, err error) {
	// Create a map of items in the new slice for faster lookup
	newMap := make(map[csov1alpha1.Resource]bool)
	for i := range newList {
		newMap[*newList[i]] = true
	}

	// Find extra objects in the old slice
	var extraResources []csov1alpha1.Resource
	for i, item := range oldList {
		if !newMap[*item] {
			extraResources = append(extraResources, *oldList[i])
		}
	}

	for _, resource := range extraResources {
		if resource.Namespace == "" {
			resource.Namespace = clusterAddonNamespace
		}
		dr, err := kube.GetDynamicResourceInterface(resource.Namespace, in.restConfig, resource.GroupVersionKind())
		if err != nil {
			return false, fmt.Errorf("failed to get dynamic resource interface: %w", err)
		}

		if err := dr.Delete(ctx, resource.Name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			reterr := fmt.Errorf("failed to delete object %q: %w", resource.GroupVersionKind(), err)
			resource.Error = reterr.Error()
			shouldRequeue = true
		}
	}

	return shouldRequeue, nil
}

func (r *ClusterAddonReconciler) getClusterAddonConfigPath(clusterClassName string) (string, error) {
	// path to the clusteraddon config /tmp/cluster-stacks/docker-ferrol-1-27-v1/clusteraddon.yaml
	// if present then new way of cluster stack otherwise old way.
	clusterAddonConfigPath := filepath.Join(r.ReleaseDirectory, release.ClusterStackSuffix, release.ConvertFromClusterClassToClusterStackFormat(clusterClassName), release.ClusterAddonYamlName)
	if _, err := os.Stat(clusterAddonConfigPath); err != nil {
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("failed to verify the clusteraddon.yaml path %s with error: %w", clusterAddonConfigPath, err)
		}

		return "", nil
	}

	return clusterAddonConfigPath, nil
}

type templateAndApplyClusterAddonInput struct {
	clusterAddonChartPath string
	// cluster-addon-values.yaml
	clusterAddonValuesPath string
	// clusteraddon.yaml
	clusterAddonConfigPath string
	clusterAddon           *csov1alpha1.ClusterAddon
	kubernetesVersion      string
	oldKubernetesVersion   string
	cluster                *clusterv1.Cluster
	restConfig             *rest.Config
	addonStagesInput
}

type addonStagesInput struct {
	kubeClient                         kube.Client
	dynamicClient                      *dynamic.DynamicClient
	discoverClient                     *discovery.DiscoveryClient
	chartMap                           map[string]os.DirEntry
	newDestinationClusterAddonChartDir string
	oldDestinationClusterAddonChartDir string
}

func (r *ClusterAddonReconciler) getAddonStagesInput(restConfig *rest.Config, clusterAddonChart string) (addonStagesInput, error) {
	var (
		addonStages addonStagesInput
		err         error
	)
	addonStages.kubeClient = r.KubeClientFactory.NewClient(clusterAddonNamespace, restConfig)

	addonStages.dynamicClient, err = dynamic.NewForConfig(restConfig)
	if err != nil {
		return addonStagesInput{}, fmt.Errorf("failed to build dynamic client from restConfig: %w", err)
	}

	addonStages.discoverClient, err = discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		return addonStagesInput{}, fmt.Errorf("error creating discovery client: %w", err)
	}

	// src - /tmp/cluster-stacks/docker-ferrol1-27-v1/docker-ferrol-27-cluster-addon-v1.tgz
	// dst - /tmp/cluster-stacks/docker-ferrol-1-27-v1/docker-ferrol-1-27-cluster-addon-v1/
	addonStages.newDestinationClusterAddonChartDir = strings.TrimSuffix(clusterAddonChart, ".tgz")
	if err := unTarContent(clusterAddonChart, addonStages.newDestinationClusterAddonChartDir); err != nil {
		return addonStagesInput{}, fmt.Errorf("failed to untar new cluster stack cluster addon chart: %q: %w", clusterAddonChart, err)
	}

	// Read all the helm charts in the un-tared cluster addon.
	subDirs, err := os.ReadDir(addonStages.newDestinationClusterAddonChartDir)
	if err != nil {
		return addonStagesInput{}, fmt.Errorf("failed to read directories inside: %q: %w", addonStages.newDestinationClusterAddonChartDir, err)
	}

	// Create a map for faster lookup
	chartMap := make(map[string]os.DirEntry)
	for _, subDir := range subDirs {
		chartMap[subDir.Name()] = subDir
	}
	addonStages.chartMap = chartMap

	return addonStages, nil
}

func (r *ClusterAddonReconciler) templateAndApplyClusterAddonHelmChart(ctx context.Context, in *templateAndApplyClusterAddonInput) (bool, error) {
	clusterAddonChart := in.clusterAddonChartPath
	var shouldRequeue bool

	buildTemplate, err := buildTemplateFromClusterAddonValues(ctx, in.clusterAddonValuesPath, in.cluster, r.Client)
	if err != nil {
		return false, fmt.Errorf("failed to build template from cluster addon values: %w", err)
	}

	helmTemplate, err := helmTemplateClusterAddon(clusterAddonChart, buildTemplate, in.kubernetesVersion)
	if err != nil {
		return false, fmt.Errorf("failed to template helm chart: %w", err)
	}

	kubeClient := r.KubeClientFactory.NewClient(clusterAddonNamespace, in.restConfig)

	newResources, shouldRequeue, err := kubeClient.Apply(ctx, helmTemplate, in.clusterAddon.Status.Resources)
	if err != nil {
		return false, fmt.Errorf("failed to apply objects from cluster addon Helm chart: %w", err)
	}

	in.clusterAddon.Status.Resources = newResources

	return shouldRequeue, nil
}

func (r *ClusterAddonReconciler) executeStage(ctx context.Context, stage *clusteraddon.Stage, in *templateAndApplyClusterAddonInput) (bool, error) {
	logger := log.FromContext(ctx)

	_, exists := in.chartMap[stage.Name]
	if !exists {
		// do not reconcile by returning error, just create an event.
		conditions.MarkFalse(
			in.clusterAddon,
			csov1alpha1.HelmChartFoundCondition,
			csov1alpha1.HelmChartMissingReason,
			clusterv1.ConditionSeverityInfo,
			"helm chart name doesn't exists in the cluster addon helm chart: %q",
			stage.Name,
		)
		return false, nil
	}

check:
	switch in.clusterAddon.GetStagePhase(stage.Name, stage.Action) {
	case csov1alpha1.StagePhasePending, csov1alpha1.StagePhaseWaitingForPreCondition:
		// If WaitForPreCondition is mentioned.
		if !reflect.DeepEqual(stage.WaitForPreCondition, clusteraddon.WaitForCondition{}) {
			// Evaluate the condition.
			logger.V(1).Info("starting to evaluate pre condition", "clusterStack", in.clusterAddon.Spec.ClusterStack, "name", stage.Name, "hook", in.clusterAddon.Spec.Hook)
			if err := getDynamicResourceAndEvaluateCEL(ctx, in.dynamicClient, in.discoverClient, stage.WaitForPreCondition); err != nil {
				if errors.Is(err, clusteraddon.ErrConditionNotMatch) {
					conditions.MarkFalse(
						in.clusterAddon,
						csov1alpha1.EvaluatedCELCondition,
						csov1alpha1.FailedToEvaluatePreConditionReason,
						clusterv1.ConditionSeverityInfo,
						"failed to successfully evaluate pre condition: %q: %s", stage.Name, err.Error(),
					)

					in.clusterAddon.SetStagePhase(stage.Name, stage.Action, csov1alpha1.StagePhaseWaitingForPreCondition)

					return true, nil
				}
				return false, fmt.Errorf("failed to get dynamic resource and evaluate cel expression for pre condition: %w", err)
			}

			conditions.Delete(in.clusterAddon, csov1alpha1.EvaluatedCELCondition)
			logger.V(1).Info("finished evaluating pre condition", "clusterStack", in.clusterAddon.Spec.ClusterStack, "name", stage.Name, "hook", in.clusterAddon.Spec.Hook)
		}

		in.clusterAddon.SetStagePhase(stage.Name, stage.Action, csov1alpha1.StagePhaseApplyingOrDeleting)
		goto check

	case csov1alpha1.StagePhaseApplyingOrDeleting:
		if stage.Action == clusteraddon.Apply {
			logger.V(1).Info("starting to template helm chart", "clusterStack", in.clusterAddon.Spec.ClusterStack, "name", stage.Name, "hook", in.clusterAddon.Spec.Hook)
			shouldReturn, oldTemplate, newTemplate, err := r.templateNewClusterStackAddonHelmChart(ctx, in, stage.Name)
			if err != nil {
				return false, fmt.Errorf("failed to helm template: %w", err)
			}
			if shouldReturn {
				return false, nil
			}

			logger.V(1).Info("finished templating helm chart and starting to apply helm chart", "clusterStack", in.clusterAddon.Spec.ClusterStack, "name", stage.Name, "hook", in.clusterAddon.Spec.Hook)

			newResources, shouldRequeue, err := in.kubeClient.ApplyNewClusterStack(ctx, oldTemplate, newTemplate)
			if err != nil {
				conditions.MarkFalse(
					in.clusterAddon,
					csov1alpha1.HelmChartAppliedCondition,
					csov1alpha1.FailedToApplyObjectsReason,
					clusterv1.ConditionSeverityInfo,
					"failed to successfully apply helm chart: %q: %s", stage.Name, err.Error(),
				)

				return false, fmt.Errorf("failed to apply objects from cluster addon Helm chart: %w", err)
			}
			if shouldRequeue {
				conditions.MarkFalse(
					in.clusterAddon,
					csov1alpha1.HelmChartAppliedCondition,
					csov1alpha1.FailedToApplyObjectsReason,
					clusterv1.ConditionSeverityInfo,
					"failed to successfully apply helm chart: %q", stage.Name,
				)

				return true, nil
			}

			// This is for the current stage objects and will be removed once done.
			in.clusterAddon.Status.Resources = newResources
			logger.V(1).Info("finished applying helm chart", "clusterStack", in.clusterAddon.Spec.ClusterStack, "name", stage.Name, "hook", in.clusterAddon.Spec.Hook)

			// delete the false condition with failed to apply reason
			conditions.Delete(in.clusterAddon, csov1alpha1.HelmChartAppliedCondition)

			// remove status resource if applied successfully
			in.clusterAddon.Status.Resources = make([]*csov1alpha1.Resource, 0)

			in.clusterAddon.SetStagePhase(stage.Name, stage.Action, csov1alpha1.StagePhaseWaitingForPostCondition)
			goto check
		}
		// Delete part
		logger.V(1).Info("starting to template helm chart", "clusterStack", in.clusterAddon.Spec.ClusterStack, "name", stage.Name, "hook", in.clusterAddon.Spec.Hook)
		helmTemplate, err := helmTemplateNewClusterStack(in, stage.Name)
		if err != nil {
			conditions.MarkFalse(
				in.clusterAddon,
				csov1alpha1.HelmChartTemplatedCondition,
				csov1alpha1.TemplateNewClusterStackFailedReason,
				clusterv1.ConditionSeverityError,
				"failed to template new helm chart: %s", err.Error(),
			)

			return false, nil
		}
		logger.V(1).Info("finished templating helm chart and starting to delete helm chart", "clusterStack", in.clusterAddon.Spec.ClusterStack, "name", stage.Name, "hook", in.clusterAddon.Spec.Hook)

		deletedResources, shouldRequeue, err := in.kubeClient.DeleteNewClusterStack(ctx, helmTemplate)
		if err != nil {
			conditions.MarkFalse(
				in.clusterAddon,
				csov1alpha1.HelmChartDeletedCondition,
				csov1alpha1.FailedToDeleteObjectsReason,
				clusterv1.ConditionSeverityInfo,
				"failed to successfully delete helm chart: %q", stage.Name,
			)

			return false, fmt.Errorf("failed to delete objects from cluster addon Helm chart: %w", err)
		}
		if shouldRequeue {
			return true, nil
		}

		// This is for the current stage objects and will be removed once done.
		in.clusterAddon.Status.Resources = deletedResources
		logger.V(1).Info("finished deleting helm chart", "clusterStack", in.clusterAddon.Spec.ClusterStack, "name", stage.Name, "hook", in.clusterAddon.Spec.Hook)

		// remove status resource if deleted successfully
		in.clusterAddon.Status.Resources = make([]*csov1alpha1.Resource, 0)

		// delete the false condition with failed to apply reason
		conditions.Delete(in.clusterAddon, csov1alpha1.HelmChartDeletedCondition)

		in.clusterAddon.SetStagePhase(stage.Name, stage.Action, csov1alpha1.StagePhaseWaitingForPostCondition)
		goto check

	case csov1alpha1.StagePhaseWaitingForPostCondition:
		// If WaitForPostCondition is mentioned.
		if !reflect.DeepEqual(stage.WaitForPostCondition, clusteraddon.WaitForCondition{}) {
			// Evaluate the condition.
			logger.V(1).Info("starting to evaluate post condition", "clusterStack", in.clusterAddon.Spec.ClusterStack, "name", stage.Name, "hook", in.clusterAddon.Spec.Hook)
			if err := getDynamicResourceAndEvaluateCEL(ctx, in.dynamicClient, in.discoverClient, stage.WaitForPostCondition); err != nil {
				if errors.Is(err, clusteraddon.ErrConditionNotMatch) {
					conditions.MarkFalse(
						in.clusterAddon,
						csov1alpha1.EvaluatedCELCondition,
						csov1alpha1.FailedToEvaluatePostConditionReason,
						clusterv1.ConditionSeverityInfo,
						"failed to successfully evaluate post condition: %q: %s", stage.Name, err.Error(),
					)

					return true, nil
				}
				return false, fmt.Errorf("failed to get dynamic resource and evaluate cel expression for post condition: %w", err)
			}

			conditions.Delete(in.clusterAddon, csov1alpha1.EvaluatedCELCondition)
			logger.V(1).Info("finished evaluating post condition", "clusterStack", in.clusterAddon.Spec.ClusterStack, "name", stage.Name, "hook", in.clusterAddon.Spec.Hook)
		}

		in.clusterAddon.SetStagePhase(stage.Name, stage.Action, csov1alpha1.StagePhaseDone)
	}

	return false, nil
}

// downloadOldClusterStackRelease downloads the old cluster stack if not present and returns release clusterAddon chart path if requeue and error.
func (r *ClusterAddonReconciler) downloadOldClusterStackRelease(ctx context.Context, clusterAddon *csov1alpha1.ClusterAddon) (*release.Release, bool, error) {
	// initiate assets client.
	gc, err := r.AssetsClientFactory.NewClient(ctx)
	if err != nil {
		isSet := conditions.IsFalse(clusterAddon, csov1alpha1.AssetsClientAPIAvailableCondition)
		conditions.MarkFalse(clusterAddon,
			csov1alpha1.AssetsClientAPIAvailableCondition,
			csov1alpha1.FailedCreateAssetsClientReason,
			clusterv1.ConditionSeverityError,
			"%s", err.Error(),
		)
		record.Warn(clusterAddon, "FailedCreateAssetsClient", err.Error())

		// give the assets client a second change
		if isSet {
			return nil, true, nil
		}
		return nil, false, nil
	}

	conditions.MarkTrue(clusterAddon, csov1alpha1.AssetsClientAPIAvailableCondition)

	// check if old cluster stack release is present or not.
	releaseAsset, download, err := release.New(release.ConvertFromClusterClassToClusterStackFormat(clusterAddon.Spec.ClusterStack), r.ReleaseDirectory)
	if err != nil {
		conditions.MarkFalse(clusterAddon,
			csov1alpha1.ClusterStackReleaseAssetsReadyCondition,
			csov1alpha1.IssueWithReleaseAssetsReason,
			clusterv1.ConditionSeverityError, "%s", err.Error())
		return nil, true, nil
	}
	if download {
		// if download is true, it means that the release assets have not been downloaded yet
		conditions.MarkFalse(clusterAddon, csov1alpha1.ClusterStackReleaseAssetsReadyCondition, csov1alpha1.ReleaseAssetsNotDownloadedYetReason, clusterv1.ConditionSeverityInfo, "assets not downloaded yet")

		// this is the point where we download the release.
		// acquire lock so that only one reconcile loop can download the release
		r.clusterStackRelDownloadDirectoryMutex.Lock()

		if err := downloadReleaseAssets(ctx, release.ConvertFromClusterClassToClusterStackFormat(clusterAddon.Spec.ClusterStack), releaseAsset.LocalDownloadPath, gc); err != nil {
			return nil, false, fmt.Errorf("failed to download release assets: %w", err)
		}

		r.clusterStackRelDownloadDirectoryMutex.Unlock()

		// requeue to make sure release assets can be accessed
		return nil, true, nil
	}

	if err := releaseAsset.CheckHelmCharts(); err != nil {
		msg := fmt.Sprintf("failed to validate helm charts: %s", err.Error())
		conditions.MarkFalse(
			clusterAddon,
			csov1alpha1.ClusterStackReleaseAssetsReadyCondition,
			csov1alpha1.IssueWithReleaseAssetsReason,
			clusterv1.ConditionSeverityError,
			"%s", msg,
		)
		record.Warn(clusterAddon, "ValidateHelmChartFailed", msg)
		return nil, false, nil
	}

	// set downloaded condition if able to read metadata file
	conditions.MarkTrue(clusterAddon, csov1alpha1.ClusterStackReleaseAssetsReadyCondition)

	return &releaseAsset, false, nil
}

func (r *ClusterAddonReconciler) templateNewClusterStackAddonHelmChart(ctx context.Context, in *templateAndApplyClusterAddonInput, name string) (shouldReturn bool, oldTemplate, newTemplate []byte, err error) { //nolint:revive // ignoring the 4 return values
	var (
		oldBuildTemplate []byte
		oldHelmTemplate  []byte

		newBuildTemplate []byte
		newHelmTemplate  []byte
	)

	if in.oldDestinationClusterAddonChartDir != "" {
		oldClusterStackSubDirPath := filepath.Join(in.oldDestinationClusterAddonChartDir, name)

		// we skip helm templating if last cluster stack don't follow the new convention.
		if _, err := os.Stat(filepath.Join(oldClusterStackSubDirPath, release.OverwriteYaml)); err == nil {
			oldBuildTemplate, err = buildTemplateFromClusterAddonValues(ctx, filepath.Join(oldClusterStackSubDirPath, release.OverwriteYaml), in.cluster, r.Client)
			if err != nil {
				conditions.MarkFalse(
					in.clusterAddon,
					csov1alpha1.HelmChartTemplatedCondition,
					csov1alpha1.TemplateOldClusterStackOverwriteFailedReason,
					clusterv1.ConditionSeverityError,
					"failed to build template from old cluster addon values: %s", err.Error(),
				)

				record.Warnf(
					in.clusterAddon,
					csov1alpha1.TemplateOldClusterStackOverwriteFailedReason,
					"failed to build template from old cluster addon values: %s", err.Error(),
				)

				return true, nil, nil, nil
			}

			oldHelmTemplate, err = helmTemplateClusterAddon(oldClusterStackSubDirPath, oldBuildTemplate, in.oldKubernetesVersion)
			if err != nil {
				conditions.MarkFalse(
					in.clusterAddon,
					csov1alpha1.HelmChartTemplatedCondition,
					csov1alpha1.TemplateOldClusterStackFailedReason,
					clusterv1.ConditionSeverityError,
					"failed to template old helm chart: %s", err.Error(),
				)

				record.Warnf(
					in.clusterAddon,
					csov1alpha1.TemplateOldClusterStackFailedReason,
					"failed to template old helm chart: %s", err.Error(),
				)

				return true, nil, nil, nil
			}
		}
	}

	newClusterStackSubDirPath := filepath.Join(in.newDestinationClusterAddonChartDir, name)

	if _, err := os.Stat(filepath.Join(newClusterStackSubDirPath, release.OverwriteYaml)); err == nil {
		newBuildTemplate, err = buildTemplateFromClusterAddonValues(ctx, filepath.Join(newClusterStackSubDirPath, release.OverwriteYaml), in.cluster, r.Client)
		if err != nil {
			conditions.MarkFalse(
				in.clusterAddon,
				csov1alpha1.HelmChartTemplatedCondition,
				csov1alpha1.TemplateNewClusterStackOverwriteFailedReason,
				clusterv1.ConditionSeverityError,
				"failed to build template from new cluster addon values: %s", err.Error(),
			)

			record.Eventf(
				in.clusterAddon,
				csov1alpha1.TemplateNewClusterStackOverwriteFailedReason,
				"failed to build template from new cluster addon values: %s", err.Error(),
			)

			return true, nil, nil, nil
		}
	}

	newHelmTemplate, err = helmTemplateClusterAddon(newClusterStackSubDirPath, newBuildTemplate, in.kubernetesVersion)
	if err != nil {
		conditions.MarkFalse(
			in.clusterAddon,
			csov1alpha1.HelmChartTemplatedCondition,
			csov1alpha1.TemplateNewClusterStackFailedReason,
			clusterv1.ConditionSeverityError,
			"failed to template new helm chart: %s", err.Error(),
		)

		record.Eventf(
			in.clusterAddon,
			csov1alpha1.TemplateNewClusterStackFailedReason,
			"failed to template new helm chart: %s", err.Error(),
		)

		return true, nil, nil, nil
	}
	conditions.Delete(in.clusterAddon, csov1alpha1.HelmChartTemplatedCondition)

	return false, oldHelmTemplate, newHelmTemplate, nil
}

func helmTemplateNewClusterStack(in *templateAndApplyClusterAddonInput, name string) (newTemplate []byte, err error) {
	var buildTemplate []byte

	newClusterStackSubDirPath := filepath.Join(in.newDestinationClusterAddonChartDir, name)
	newHelmTemplate, err := helmTemplateClusterAddon(newClusterStackSubDirPath, buildTemplate, in.kubernetesVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to template new helm chart: %w", err)
	}

	return newHelmTemplate, nil
}

func getDynamicResourceAndEvaluateCEL(ctx context.Context, dynamicClient *dynamic.DynamicClient, discoveryClient *discovery.DiscoveryClient, waitCondition clusteraddon.WaitForCondition) error {
	evalMap := make(map[string]interface{})

	env, err := cel.NewEnv()
	if err != nil {
		return fmt.Errorf("environment creation error: %w", err)
	}

	for _, object := range waitCondition.Objects {
		env, err = env.Extend(
			cel.Variable(object.Key, celtypes.NewMapType(cel.StringType, cel.AnyType)),
		)
		if err != nil {
			return fmt.Errorf("failed to extend the current cel environment with %q: %w", object.Key, err)
		}
	}

	ast, iss := env.Compile(waitCondition.Conditions)
	if !errors.Is(iss.Err(), nil) {
		return fmt.Errorf("failed to compile cel expression: %q: %w", waitCondition.Conditions, err)
	}

	prg, err := env.Program(ast)
	if err != nil {
		return fmt.Errorf("failed to generate an evaluable instance of the Ast within the environment: %w", err)
	}

	for _, object := range waitCondition.Objects {
		splittedAPIVersion := strings.Split(object.APIVersion, "/")

		gvr := schema.GroupVersionKind{
			Kind: object.Kind,
		}
		if len(splittedAPIVersion) == 2 {
			gvr.Group = splittedAPIVersion[0]
			gvr.Version = splittedAPIVersion[1]
		} else {
			gvr.Group = ""
			gvr.Version = splittedAPIVersion[0]
		}

		mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(discoveryClient))
		mapping, err := mapper.RESTMapping(schema.GroupKind{Group: gvr.Group, Kind: gvr.Kind}, gvr.Version)
		if err != nil {
			return fmt.Errorf("error creating rest mapping: %w", err)
		}

		resource := dynamicClient.Resource(mapping.Resource)

		unstructuredObj, err := resource.Namespace(object.Namespace).Get(ctx, object.Name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get object(name: %s, namespace: %s) from dynamic client: %w", object.Name, object.Namespace, err)
		}

		evalMap[object.Key] = unstructuredObj.Object
	}

	out, _, err := prg.Eval(evalMap)
	if err != nil {
		return fmt.Errorf("failed to evaluate the Ast and environment against the input vars: %w", err)
	}

	boolVal, ok := (out.Value()).(bool)
	if !ok {
		return fmt.Errorf("failed to evaluate the cel expression, its value is no boolean: %w", clusteraddon.ErrConditionNotMatch)
	}

	if !boolVal {
		return fmt.Errorf("failed to evaluate the cel expression, please check again: %w", clusteraddon.ErrConditionNotMatch)
	}

	return nil
}

func buildTemplateFromClusterAddonValues(ctx context.Context, addonValuePath string, cluster *clusterv1.Cluster, c client.Client) ([]byte, error) {
	data, err := os.ReadFile(filepath.Clean(addonValuePath))
	if err != nil {
		return nil, fmt.Errorf("failed to read the file %s: %w", addonValuePath, err)
	}

	references := map[string]corev1.ObjectReference{
		"Cluster": {
			APIVersion: cluster.APIVersion,
			Kind:       cluster.Kind,
			Namespace:  cluster.Namespace,
			Name:       cluster.Name,
		},
	}

	if cluster.Spec.ControlPlaneRef != nil {
		references["ControlPlane"] = *cluster.Spec.ControlPlaneRef
	}

	if cluster.Spec.InfrastructureRef != nil {
		references["InfraCluster"] = *cluster.Spec.InfrastructureRef
	}

	valueLookUp, err := initializeBuiltins(ctx, c, references, cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize builtins: %w", err)
	}

	tmpl, err := template.New("cluster-addon-values" + "-" + cluster.GetName()).
		Funcs(sprig.TxtFuncMap()).
		Parse(string(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create new template: %w", err)
	}

	var buffer bytes.Buffer

	if err := tmpl.Execute(&buffer, valueLookUp); err != nil {
		return nil, fmt.Errorf("failed to execute template string %q on cluster %q: %w", string(data), cluster.GetName(), err)
	}

	var unmarshalData interface{}
	err = yaml.Unmarshal(buffer.Bytes(), &unmarshalData)
	if err != nil {
		return nil, fmt.Errorf("unmarshal expanded template: %w", err)
	}

	values, ok := unmarshalData.(map[string]interface{})["values"].(string)
	// we ignore if values prefix is not there
	if !ok {
		return buffer.Bytes(), nil
	}

	return []byte(values), nil
}

// helmTemplateClusterAddon takes the helm chart path and cluster addon values and generates template yaml file
// in the same path as the chart.
// Then it returns the path of the generated yaml file.
// Example: helm template /tmp/downloads/cluster-stacks/myprovider-myclusterstack-1-26-v2/myprovider-myclusterstack-1-26-v2.tgz
// The return yaml file path will be /tmp/downloads/cluster-stacks/myprovider-myclusterstack-1-26-v2/myprovider-myclusterstack-1-26-v2.tgz.yaml.
func helmTemplateClusterAddon(chartPath string, helmTemplate []byte, kubernetesVersion string) ([]byte, error) {
	helmCommand := "helm"
	helmArgs := []string{"template", "--include-crds"}

	input := bytes.NewBuffer(helmTemplate)

	var cmdOutput bytes.Buffer

	helmArgs = append(helmArgs, "--kube-version", kubernetesVersion, "cluster-addon", filepath.Base(chartPath), "--namespace", clusterAddonNamespace, "-f", "-")
	helmTemplateCmd := exec.Command(helmCommand, helmArgs...)
	helmTemplateCmd.Stderr = os.Stderr
	helmTemplateCmd.Dir = filepath.Dir(chartPath)
	helmTemplateCmd.Stdout = &cmdOutput
	helmTemplateCmd.Stdin = input

	if err := helmTemplateCmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to run helm template for %q: %w", chartPath, err)
	}

	return cmdOutput.Bytes(), nil
}

// initializeBuiltins takes a map of keys to object references, attempts to get the referenced objects, and returns a map of keys to the actual objects.
// These objects are a map[string]interface{} so that they can be used as values in the template.
func initializeBuiltins(ctx context.Context, c client.Client, referenceMap map[string]corev1.ObjectReference, cluster *clusterv1.Cluster) (map[string]interface{}, error) {
	valueLookUp := make(map[string]interface{})

	for name, ref := range referenceMap {
		objectRef := referenceMap[name]
		obj, err := external.Get(ctx, c, &objectRef, cluster.Namespace)
		if err != nil {
			return nil, fmt.Errorf("failed to get object %s: %w", ref.Name, err)
		}
		valueLookUp[name] = obj.Object
	}

	return valueLookUp, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterAddonReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	logger := ctrl.LoggerFrom(ctx)
	blder := ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&csov1alpha1.ClusterAddon{}).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(logger, r.WatchFilterValue))

	// check also for updates in cluster objects
	return blder.WatchesRawSource(
		source.Kind(mgr.GetCache(), &clusterv1.Cluster{},
			handler.TypedEnqueueRequestsFromMapFunc(clusterToClusterAddon),
			predicate.TypedFuncs[*clusterv1.Cluster]{
				// We're only interested in the update events for a cluster object where cluster.spec.topology.class changed
				UpdateFunc: func(e event.TypedUpdateEvent[*clusterv1.Cluster]) bool {
					oldCluster := e.ObjectOld
					newCluster := e.ObjectNew
					if oldCluster.Spec.Topology != nil && newCluster.Spec.Topology != nil &&
						oldCluster.Spec.Topology.Class != newCluster.Spec.Topology.Class {
						return true
					}
					return false
				},
				GenericFunc: func(_ event.TypedGenericEvent[*clusterv1.Cluster]) bool {
					return false
				},
				CreateFunc: func(_ event.TypedCreateEvent[*clusterv1.Cluster]) bool {
					return false
				},
				DeleteFunc: func(_ event.TypedDeleteEvent[*clusterv1.Cluster]) bool {
					return false
				},
			},
		)).Complete(r)
}

// clusterToClusterAddon enqueues requests for clusterAddons on change of cluster objects.
func clusterToClusterAddon(_ context.Context, c *clusterv1.Cluster) []ctrl.Request {
	clusterAddonName := types.NamespacedName{
		Namespace: c.GetNamespace(),
		Name:      fmt.Sprintf("cluster-addon-%s", c.GetName()),
	}
	return []reconcile.Request{{NamespacedName: clusterAddonName}}
}

func unTarContent(src, dst string) error {
	// Create the target directory if it doesn't exist
	if err := os.MkdirAll(dst, os.ModePerm); err != nil { //nolint:gosec // ignore permissions
		return fmt.Errorf("%q: creating directory: %w", dst, err)
	}

	// Open the tar file
	tarFile, err := os.Open(filepath.Clean(src))
	if err != nil {
		return fmt.Errorf("%q: opening file: %w", src, err)
	}

	uncompressedStream, err := gzip.NewReader(tarFile)
	if err != nil {
		return fmt.Errorf("failed to init gzip.NewReader for %q: %w", src, err)
	}

	// Create a tar reader
	tarReader := tar.NewReader(uncompressedStream)

	// Iterate over the tar entries
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break // Reached end of tar file
		}
		if err != nil {
			return fmt.Errorf("reading tar archive: %w", err)
		}

		// Calculate the target file path
		targetPath := filepath.Join(dst, header.Name) // #nosec

		switch header.Typeflag {
		case tar.TypeDir:
			// Create directories
			if err := os.MkdirAll(targetPath, os.ModePerm); err != nil { //nolint:gosec // ignore permissions
				return fmt.Errorf("%q: creating directory: %w", targetPath, err)
			}
		case tar.TypeReg:
			// Create regular files
			if err := os.MkdirAll(filepath.Dir(targetPath), os.ModePerm); err != nil { //nolint:gosec // ignore permissions
				return fmt.Errorf("%q: creating directory: %w", filepath.Dir(targetPath), err)
			}

			outputFile, err := os.Create(filepath.Clean(targetPath))
			if err != nil {
				return fmt.Errorf("%q: creating file: %w", targetPath, err)
			}

			if _, err = io.Copy(outputFile, tarReader); err != nil /* #nosec */ {
				return fmt.Errorf("reading output file: %w", err)
			}

			if err := outputFile.Close(); err != nil {
				return fmt.Errorf("failed to close output file: %w", err)
			}
		default:
			return fmt.Errorf("unsupported type: %c in file %s", header.Typeflag, header.Name)
		}
	}
	if err := tarFile.Close(); err != nil {
		return fmt.Errorf("failed to close tar file: %w", err)
	}

	return nil
}

func removeResourcesFromCurrentListOfObjects(baseList, itemsToRemove []*csov1alpha1.Resource) []*csov1alpha1.Resource {
	// Create a map of items to remove for faster lookup
	itemsMap := make(map[*csov1alpha1.Resource]bool)
	for _, item := range itemsToRemove {
		itemsMap[item] = true
	}

	// Create a new list without the items to remove
	var newList []*csov1alpha1.Resource
	for _, item := range baseList {
		if !itemsMap[item] {
			newList = append(newList, item)
		}
	}

	return newList
}
