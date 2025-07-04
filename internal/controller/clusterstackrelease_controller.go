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
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	csov1alpha1 "github.com/SovereignCloudStack/cluster-stack-operator/api/v1alpha1"
	"github.com/SovereignCloudStack/cluster-stack-operator/pkg/assetsclient"
	"github.com/SovereignCloudStack/cluster-stack-operator/pkg/clusterstack"
	"github.com/SovereignCloudStack/cluster-stack-operator/pkg/kube"
	"github.com/SovereignCloudStack/cluster-stack-operator/pkg/release"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/rest"
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
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// ClusterStackReleaseReconciler reconciles a ClusterStackRelease object.
type ClusterStackReleaseReconciler struct {
	client.Client
	RESTConfig                            *rest.Config
	ReleaseDirectory                      string
	KubeClientFactory                     kube.Factory
	AssetsClientFactory                   assetsclient.Factory
	externalTracker                       external.ObjectTracker
	clusterStackRelDownloadDirectoryMutex sync.Mutex
	WatchFilterValue                      string
}

//+kubebuilder:rbac:groups=clusterstack.x-k8s.io,resources=clusterstackreleases,verbs=get;list;watch;create;patch;delete
//+kubebuilder:rbac:groups=clusterstack.x-k8s.io,resources=clusterstackreleases/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=clusterstack.x-k8s.io,resources=clusterstackreleases/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ClusterStackReleaseReconciler) Reconcile(ctx context.Context, req reconcile.Request) (_ reconcile.Result, reterr error) {
	clusterStackRelease := &csov1alpha1.ClusterStackRelease{}
	if err := r.Get(ctx, req.NamespacedName, clusterStackRelease); err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("failed to get clusterStackRelease %s/%s: %w", req.Namespace, req.Name, err)
	}

	// noProvider mode has to be handled differently
	noProvider := clusterStackRelease.Spec.ProviderRef == nil

	patchHelper, err := patch.NewHelper(clusterStackRelease, r.Client)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to init patch helper for ClusterStackRelease: %w", err)
	}

	defer func() {
		conditions.SetSummary(clusterStackRelease)

		// Check if the object has a deletion timestamp and release assets have not been downloaded properly.
		// In that case, the controller cannot perform a proper reconcileDelete and we just remove the finalizer.
		if !clusterStackRelease.DeletionTimestamp.IsZero() &&
			conditions.IsFalse(clusterStackRelease, csov1alpha1.ClusterStackReleaseAssetsReadyCondition) {
			controllerutil.RemoveFinalizer(clusterStackRelease, csov1alpha1.ClusterStackReleaseFinalizer)
		}

		if err := patchHelper.Patch(ctx, clusterStackRelease); err != nil {
			reterr = fmt.Errorf("failed to patch ClusterStackRelease: %w", err)
		}
	}()

	controllerutil.AddFinalizer(clusterStackRelease, csov1alpha1.ClusterStackReleaseFinalizer)

	// name of ClusterStackRelease object is same as the release tag
	releaseTag := clusterStackRelease.Name

	releaseAssets, download, err := release.New(releaseTag, r.ReleaseDirectory)
	if err != nil {
		conditions.MarkFalse(clusterStackRelease,
			csov1alpha1.ClusterStackReleaseAssetsReadyCondition,
			csov1alpha1.IssueWithReleaseAssetsReason,
			clusterv1.ConditionSeverityError, "%s", err.Error())
		return reconcile.Result{RequeueAfter: 1 * time.Minute}, fmt.Errorf("failed to create release: %w", err)
	}

	// if download is true, it means that the release assets have not been downloaded yet
	if download {
		conditions.MarkFalse(clusterStackRelease, csov1alpha1.ClusterStackReleaseAssetsReadyCondition, csov1alpha1.ReleaseAssetsNotDownloadedYetReason, clusterv1.ConditionSeverityInfo, "assets not downloaded yet")

		// this is the point where we download the release
		// acquire lock so that only one reconcile loop can download the release
		r.clusterStackRelDownloadDirectoryMutex.Lock()
		defer r.clusterStackRelDownloadDirectoryMutex.Unlock()

		ac, err := r.AssetsClientFactory.NewClient(ctx)
		if err != nil {
			isSet := conditions.IsFalse(clusterStackRelease, csov1alpha1.AssetsClientAPIAvailableCondition)
			conditions.MarkFalse(clusterStackRelease,
				csov1alpha1.AssetsClientAPIAvailableCondition,
				csov1alpha1.FailedCreateAssetsClientReason,
				clusterv1.ConditionSeverityError,
				"%s", err.Error(),
			)
			record.Warn(clusterStackRelease, "FailedCreateAssetsClient", err.Error())

			// give the assets client a second change
			if isSet {
				return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
			}
			return reconcile.Result{}, nil
		}

		conditions.MarkTrue(clusterStackRelease, csov1alpha1.AssetsClientAPIAvailableCondition)

		if err := downloadReleaseAssets(ctx, releaseTag, releaseAssets.LocalDownloadPath, ac); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to download release assets: %w", err)
		}

		// requeue to make sure release assets can be accessed
		return reconcile.Result{Requeue: true}, nil
	}

	// Check for helm charts in the release assets. If they are not present, then something went wrong.
	if err := releaseAssets.CheckHelmCharts(); err != nil {
		msg := fmt.Sprintf("failed to validate helm charts: %s", err.Error())
		conditions.MarkFalse(
			clusterStackRelease,
			csov1alpha1.ClusterStackReleaseAssetsReadyCondition,
			csov1alpha1.IssueWithReleaseAssetsReason,
			clusterv1.ConditionSeverityError,
			"%s", msg,
		)
		record.Warn(clusterStackRelease, "ValidateHelmChartFailed", msg)
		return reconcile.Result{}, nil
	}

	conditions.MarkTrue(clusterStackRelease, csov1alpha1.ClusterStackReleaseAssetsReadyCondition)

	kubeClient := r.KubeClientFactory.NewClient(req.Namespace, r.RESTConfig)

	if !clusterStackRelease.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, &releaseAssets, req.Namespace, kubeClient, clusterStackRelease)
	}

	clusterStackRelease.Status.KubernetesVersion = releaseAssets.Meta.Versions.Kubernetes

	if !noProvider {
		ready, err := r.updateProviderClusterStackRelease(ctx, clusterStackRelease)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to update ProviderClusterStackRelease: %w", err)
		}

		// If not ready, then we wait. We don't have to requeue because an update of the provider object triggers it anyway.
		if !ready {
			return reconcile.Result{}, nil
		}
	}

	conditions.MarkTrue(clusterStackRelease, csov1alpha1.ProviderClusterStackReleaseReadyCondition)

	// if objects have been applied already, we don't have to do anything
	if conditions.IsTrue(clusterStackRelease, csov1alpha1.HelmChartAppliedCondition) {
		return reconcile.Result{}, nil
	}

	shouldRequeue, err := r.templateAndApply(ctx, &releaseAssets, clusterStackRelease, kubeClient)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to template and apply: %w", err)
	}

	if shouldRequeue {
		conditions.MarkFalse(clusterStackRelease, csov1alpha1.HelmChartAppliedCondition, csov1alpha1.ObjectsApplyingOngoingReason, clusterv1.ConditionSeverityWarning, "failed to successfully apply everything")
		return reconcile.Result{RequeueAfter: 20 * time.Second}, nil
	}

	conditions.MarkTrue(clusterStackRelease, csov1alpha1.HelmChartAppliedCondition)
	clusterStackRelease.Status.Ready = true

	return reconcile.Result{}, nil
}

// reconcileDelete controls the deletion of clusterstackrelease objects.
func (r *ClusterStackReleaseReconciler) reconcileDelete(ctx context.Context, releaseAssets *release.Release, namespace string, kubeClient kube.Client, clusterStackReleaseCR *csov1alpha1.ClusterStackRelease) (reconcile.Result, error) {
	presentClusterClasses, err := getUsedClusterClasses(ctx, r.Client, namespace)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get usedClusterClass: %w", err)
	}
	for _, clusterstackrelease := range presentClusterClasses {
		if clusterStackReleaseCR.Name == clusterstackrelease {
			// if it's in use then do nothing and check again after 1 minute.
			return reconcile.Result{RequeueAfter: time.Minute}, nil
		}
	}

	template, err := r.templateClusterClassHelmChart(releaseAssets, clusterStackReleaseCR.Name, clusterStackReleaseCR.Namespace)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to perform helm template: %w", err)
	}

	_, shouldRequeue, err := kubeClient.Delete(ctx, template, clusterStackReleaseCR.Status.Resources)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to delete helm chart: %w", err)
	}
	if shouldRequeue {
		return reconcile.Result{Requeue: true, RequeueAfter: 20 * time.Second}, nil
	}

	controllerutil.RemoveFinalizer(clusterStackReleaseCR, csov1alpha1.ClusterStackReleaseFinalizer)
	return reconcile.Result{}, nil
}

func downloadReleaseAssets(ctx context.Context, releaseTag, downloadPath string, ac assetsclient.Client) error {
	if err := ac.DownloadReleaseAssets(ctx, releaseTag, downloadPath); err != nil {
		// if download failed for some reason, delete the release directory so that it can be retried in the next reconciliation
		if err := os.RemoveAll(downloadPath); err != nil {
			return fmt.Errorf("failed to remove release: %w", err)
		}
		return fmt.Errorf("failed to download release assets: %w", err)
	}

	return nil
}

func (r *ClusterStackReleaseReconciler) updateProviderClusterStackRelease(ctx context.Context, clusterStackRelease *csov1alpha1.ClusterStackRelease) (bool, error) {
	// fetch providerClusterStackReleaseObject to update that object accordingly and get information from it
	providerClusterStackRelease, err := external.Get(ctx, r.Client, clusterStackRelease.Spec.ProviderRef, clusterStackRelease.Namespace)
	if err != nil {
		return false, fmt.Errorf("failed to get ProviderClusterStackRelease object: %w", err)
	}

	patchHelperProviderObject, err := patch.NewHelper(providerClusterStackRelease, r.Client)
	if err != nil {
		return false, fmt.Errorf("failed to create patch helper for ProviderClusterStackRelease: %w", err)
	}

	if err := controllerutil.SetControllerReference(clusterStackRelease, providerClusterStackRelease, r.Scheme()); err != nil {
		return false, fmt.Errorf("failed to set owner reference to ProviderClusterStackRelease: %w", err)
	}

	if err := patchHelperProviderObject.Patch(ctx, providerClusterStackRelease); err != nil {
		return false, fmt.Errorf("failed to patch ProviderClusterStackRelease: %w", err)
	}

	// ensure we add a watch to the external object, if there isn't one already
	eventHandler := handler.EnqueueRequestForOwner(r.Scheme(), r.RESTMapper(), &csov1alpha1.ClusterStackRelease{})
	if err := r.externalTracker.Watch(log.FromContext(ctx), providerClusterStackRelease, eventHandler); err != nil {
		return false, fmt.Errorf("failed to add external watch to ProviderClusterStackRelease: %w", err)
	}

	// set condition and wait if providerClusterStackRelease is not ready yet
	ready, _, err := unstructured.NestedBool(providerClusterStackRelease.Object, "status", "ready")
	if err != nil {
		return false, fmt.Errorf("failed to find status.ready in providerClusterStackRelease object: %w", err)
	}

	if !ready {
		conditions.MarkFalse(clusterStackRelease,
			csov1alpha1.ProviderClusterStackReleaseReadyCondition,
			csov1alpha1.ProcessOngoingReason, clusterv1.ConditionSeverityInfo,
			"providerClusterStackRelease not ready yet",
		)
	}
	return ready, nil
}

func (r *ClusterStackReleaseReconciler) templateAndApply(ctx context.Context, releaseAssets *release.Release, clusterStackRelease *csov1alpha1.ClusterStackRelease, kubeClient kube.Client) (bool, error) {
	// template helm chart and apply objects
	template, err := r.templateClusterClassHelmChart(releaseAssets, clusterStackRelease.Name, clusterStackRelease.Namespace)
	if err != nil {
		return false, fmt.Errorf("failed to template clusterClass helm chart: %w", err)
	}

	if template == nil {
		return false, errors.New("template is empty")
	}

	newResources, shouldRequeue, err := kubeClient.Apply(ctx, template, clusterStackRelease.Status.Resources)
	if err != nil {
		conditions.MarkFalse(clusterStackRelease, csov1alpha1.HelmChartAppliedCondition, csov1alpha1.FailedToApplyObjectsReason, clusterv1.ConditionSeverityError, "failed to apply")
		return false, fmt.Errorf("failed to apply cluster class helm chart: %w", err)
	}

	clusterStackRelease.Status.Resources = newResources

	return shouldRequeue, nil
}

// templateClusterClassHelmChart templates the clusterClass helm chart.
func (*ClusterStackReleaseReconciler) templateClusterClassHelmChart(releaseAssets *release.Release, name, namespace string) ([]byte, error) {
	clusterClassChart, e := releaseAssets.ClusterClassChartPath()
	if e != nil {
		return nil, fmt.Errorf("failed to template clusterClass helm chart: %w", e)
	}

	splittedName := strings.Split(name, clusterstack.Separator)
	releaseName := strings.Join(splittedName[0:4], clusterstack.Separator)

	template, err := helmTemplate(clusterClassChart, releaseName, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to template clusterClass helm chart: %w", err)
	}

	return template, nil
}

func helmTemplate(chartPath, releaseName, namespace string) ([]byte, error) {
	helmCommand := "helm"
	helmArgs := []string{"template"}

	var cmdOutput bytes.Buffer

	helmArgs = append(helmArgs, releaseName, filepath.Base(chartPath), "--namespace", namespace)
	helmTemplateCmd := exec.Command(helmCommand, helmArgs...)
	helmTemplateCmd.Stderr = os.Stderr
	helmTemplateCmd.Dir = filepath.Dir(chartPath)
	helmTemplateCmd.Stdout = &cmdOutput

	if err := helmTemplateCmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to run helm template for %q: %w", chartPath, err)
	}

	return cmdOutput.Bytes(), nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterStackReleaseReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	c, err := ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&csov1alpha1.ClusterStackRelease{}).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(log.FromContext(ctx), r.WatchFilterValue)).
		Build(r)
	if err != nil {
		return fmt.Errorf("failed to set up with a controller manager: %w", err)
	}

	r.externalTracker = external.ObjectTracker{
		Controller: c,
		Cache:      mgr.GetCache(),
	}
	return nil
}
