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

// Package controller contains the cluster stack controllers.
package controller

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	csov1alpha1 "github.com/SovereignCloudStack/cluster-stack-operator/api/v1alpha1"
	"github.com/SovereignCloudStack/cluster-stack-operator/internal/clusterstackrelease"
	"github.com/SovereignCloudStack/cluster-stack-operator/pkg/assetsclient"
	"github.com/SovereignCloudStack/cluster-stack-operator/pkg/clusterstack"
	"github.com/SovereignCloudStack/cluster-stack-operator/pkg/version"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/external"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	"sigs.k8s.io/cluster-api/util/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// ClusterStackReconciler reconciles a ClusterStack object.
type ClusterStackReconciler struct {
	client.Client
	AssetsClientFactory assetsclient.Factory
	ReleaseDirectory    string
	WatchFilterValue    string
}

//+kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;update
//+kubebuilder:rbac:groups=clusterstack.x-k8s.io,resources=clusterstacks,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=clusterstack.x-k8s.io,resources=clusterstacks/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=clusterstack.x-k8s.io,resources=clusterstacks/finalizers,verbs=update;delete
//+kubebuilder:rbac:groups=cluster.x-k8s.io;infrastructure.cluster.x-k8s.io;bootstrap.cluster.x-k8s.io;controlplane.cluster.x-k8s.io,resources=*,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.clusterstack.x-k8s.io,resources=*,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the ClusterStack closer to the desired state.
func (r *ClusterStackReconciler) Reconcile(ctx context.Context, req reconcile.Request) (_ reconcile.Result, reterr error) {
	logger := log.FromContext(ctx)

	clusterStack := &csov1alpha1.ClusterStack{}
	if err := r.Get(ctx, req.NamespacedName, clusterStack); err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("failed to get ClusterStack %s/%s: %w", req.Namespace, req.Name, err)
	}

	patchHelper, err := patch.NewHelper(clusterStack, r.Client)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to init patch helper: %w", err)
	}

	defer func() {
		conditions.SetSummary(clusterStack)

		if err := patchHelper.Patch(ctx, clusterStack); err != nil {
			reterr = fmt.Errorf("failed to patch clusterstack: %w", err)
		}
	}()

	// nothing to do if deletion timestamp set
	if !clusterStack.DeletionTimestamp.IsZero() {
		return reconcile.Result{}, nil
	}

	existingClusterStackReleases, err := r.getExistingClusterStackReleases(ctx, clusterStack)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get existing ClusterStackReleases: %w", err)
	}

	latestReady, k8sVersionOfLatest, err := getLatestReadyClusterStackRelease(existingClusterStackReleases)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get latest ready ClusterStackRelease: %w", err)
	}

	var latest *string

	if clusterStack.Spec.AutoSubscribe {
		ac, err := r.AssetsClientFactory.NewClient(ctx)
		if err != nil {
			isSet := conditions.IsFalse(clusterStack, csov1alpha1.AssetsClientAPIAvailableCondition)
			conditions.MarkFalse(clusterStack,
				csov1alpha1.AssetsClientAPIAvailableCondition,
				csov1alpha1.FailedCreateAssetsClientReason,
				clusterv1.ConditionSeverityError,
				"%s", err.Error(),
			)
			record.Warn(clusterStack, "FailedCreateAssetsClient", err.Error())

			// give the assets client a second change
			if isSet {
				return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
			}
			return reconcile.Result{}, nil
		}

		conditions.MarkTrue(clusterStack, csov1alpha1.AssetsClientAPIAvailableCondition)

		latest, err = getLatestReleaseFromRemoteRepository(ctx, clusterStack, ac)
		if err != nil {
			// only log error and mark condition as false, but continue
			conditions.MarkFalse(clusterStack,
				csov1alpha1.ReleasesSyncedCondition,
				csov1alpha1.FailedToSyncReason,
				clusterv1.ConditionSeverityWarning,
				"%s", err.Error(),
			)
			logger.Error(err, "failed to get latest release from remote repository")
		}

		conditions.MarkTrue(clusterStack, csov1alpha1.ReleasesSyncedCondition)
	}

	inUse, err := r.getClusterStackReleasesInUse(ctx, req.Namespace)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get ClusterStackRelease objects that are in use: %w", err)
	}

	inSpec, err := getClusterStackReleasesInSpec(&clusterStack.Spec)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get ClusterStackReleases from clusterstack.spec.versions: %w", err)
	}

	toCreate, toDelete := makeDiff(existingClusterStackReleases, latest, latestReady, inSpec, inUse)

	// delete all cluster stack releases
	for i, csr := range toDelete {
		if err := r.Delete(ctx, toDelete[i]); err != nil && !apierrors.IsNotFound(err) {
			// ignore not found errors when deleting
			reterr := fmt.Errorf("failed to delete cluster stack release %s: %w", csr.Name, err)
			record.Event(clusterStack, "FailedToDeleteClusterStackRelease", reterr.Error())
			return reconcile.Result{}, reterr
		}
	}

	ownerRef := generateOwnerReference(clusterStack)

	summary := make([]csov1alpha1.ClusterStackReleaseSummary, len(toCreate))

	for i, csr := range toCreate {
		summary[i], err = clusterstackrelease.Summary(csr)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to create summary from ClusterStackRelease %q: %w", csr.Name, err)
		}

		var providerRef *corev1.ObjectReference

		if !clusterStack.Spec.NoProvider {
			providerRef, err = r.createOrUpdateProviderClusterStackRelease(ctx, csr.Name, clusterStack)
			if err != nil {
				// set appropriate condition
				if apierrors.IsNotFound(err) {
					conditions.MarkFalse(clusterStack,
						csov1alpha1.ProviderClusterStackReleasesSyncedCondition,
						csov1alpha1.ProviderTemplateNotFoundReason,
						clusterv1.ConditionSeverityError,
						"provider template could not be found - check provider reference of cluster stack",
					)
				} else {
					conditions.MarkFalse(clusterStack,
						csov1alpha1.ProviderClusterStackReleasesSyncedCondition,
						csov1alpha1.FailedToCreateOrUpdateReason,
						clusterv1.ConditionSeverityWarning,
						"%s", err.Error(),
					)
				}
				return reconcile.Result{}, fmt.Errorf("failed to create or update provider specific ClusterStackRelease %s/%s: %w", req.Namespace, csr.Name, err)
			}
		}

		if err := r.getOrCreateClusterStackRelease(ctx, csr.Name, req.Namespace, ownerRef, providerRef); err != nil {
			conditions.MarkFalse(clusterStack,
				csov1alpha1.ClusterStackReleasesSyncedCondition,
				csov1alpha1.FailedToCreateOrUpdateReason,
				clusterv1.ConditionSeverityWarning,
				"%s", err.Error(),
			)
			return reconcile.Result{}, fmt.Errorf("failed to get or create ClusterStackRelease %s/%s: %w", req.Namespace, csr.Name, err)
		}
	}

	conditions.MarkTrue(clusterStack, csov1alpha1.ClusterStackReleasesSyncedCondition)

	if !clusterStack.Spec.NoProvider {
		conditions.MarkTrue(clusterStack, csov1alpha1.ProviderClusterStackReleasesSyncedCondition)
	}

	clusterStack.Status.Summary = summary

	if latestReady != nil {
		clusterStack.Status.LatestRelease = fmt.Sprintf("%s | %s", *latestReady, k8sVersionOfLatest)
		conditions.MarkTrue(clusterStack, csov1alpha1.ClusterStackReleaseAvailableCondition)
	}

	usableVersions, err := getUsableClusterStackReleaseVersions(existingClusterStackReleases)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get usable versions from list of ClusterStackReleases: %w", err)
	}

	clusterStack.Status.UsableVersions = strings.Join(usableVersions, ", ")

	return reconcile.Result{}, nil
}

func (r *ClusterStackReconciler) getOrCreateClusterStackRelease(ctx context.Context, name, namespace string, ownerRef *metav1.OwnerReference, providerRef *corev1.ObjectReference) error {
	clusterStackRelease := &csov1alpha1.ClusterStackRelease{}

	err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, clusterStackRelease)

	// nothing to do if object exists
	if err == nil {
		return nil
	}

	// unexpected error
	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to get ClusterStackRelease: %w", err)
	}

	// object not found - create it
	clusterStackRelease.Name = name
	clusterStackRelease.Namespace = namespace
	clusterStackRelease.TypeMeta = metav1.TypeMeta{
		Kind:       "ClusterStackRelease",
		APIVersion: "clusterstack.x-k8s.io/v1alpha1",
	}
	clusterStackRelease.SetOwnerReferences([]metav1.OwnerReference{*ownerRef})
	clusterStackRelease.Spec.ProviderRef = providerRef

	if err := r.Create(ctx, clusterStackRelease); err != nil {
		record.Eventf(clusterStackRelease,
			"ErrorCreatingClusterStackRelease",
			"failed to create %s ClusterStackRelease: %s", name, err.Error(),
		)
		return fmt.Errorf("failed to create ClusterStackRelease: %w", err)
	}

	record.Eventf(clusterStackRelease, "ClusterStackReleaseCreated", "successfully created ClusterStackRelease object %q", name)
	return nil
}

func (r *ClusterStackReconciler) createOrUpdateProviderClusterStackRelease(ctx context.Context, name string, clusterStack *csov1alpha1.ClusterStack) (*corev1.ObjectReference, error) {
	// get template object where the object is based on
	from, err := external.Get(ctx, r.Client, clusterStack.Spec.ProviderRef, clusterStack.Namespace)
	if err != nil {
		return nil, fmt.Errorf("ProviderClusterStackReleaseTemplate %q for kind %q not found: %w", clusterStack.Spec.ProviderRef.Name, clusterStack.Spec.ProviderRef.GetObjectKind(), err)
	}

	ref := &corev1.ObjectReference{
		APIVersion: from.GetAPIVersion(),
		Kind:       strings.TrimSuffix(from.GetKind(), "Template"),
		Name:       name,
	}

	existingObject, err := external.Get(ctx, r.Client, ref, clusterStack.Namespace)

	// handle unexpected errors
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, fmt.Errorf("failed to get existing ClusterStackRelease: %w", err)
	}

	var existsAlready bool
	if err == nil {
		existsAlready = true
	}

	input := &generateProviderObjectInput{
		ExistingObject: existingObject,
		Template:       from,
		TemplateRef:    clusterStack.Spec.ProviderRef,
		Namespace:      clusterStack.Namespace,
		Name:           name,
		Labels:         clusterStack.Labels,
		Annotations:    clusterStack.Annotations,
	}

	to, shouldUpdate, err := generateProviderObject(input)
	if err != nil {
		return nil, fmt.Errorf("failed to generate template: %w", err)
	}

	objectRef := external.GetObjectReference(to)

	// update if it exists already and should be updated
	if existsAlready {
		if shouldUpdate {
			if err := r.Update(ctx, to); err != nil {
				return nil, fmt.Errorf("failed to update object: %w", err)
			}
			record.Eventf(clusterStack, "UpdateProviderStackRelease", "Updated ProviderClusterStackRelease %s", name)
		}
		return objectRef, nil
	}

	// object does not exist yet - create it
	if err := r.Create(ctx, to); err != nil {
		return nil, fmt.Errorf("failed to create object: %w", err)
	}

	record.Eventf(clusterStack, "CreateProviderStackRelease", "Created ProviderClusterStackRelease %s", name)

	return objectRef, nil
}

// generateProviderObjectInput is the input needed to generate a new template.
type generateProviderObjectInput struct {
	// ExistingObject is the existing object if it exists already.
	ExistingObject *unstructured.Unstructured

	// Template is the TemplateRef turned into an unstructured.
	Template *unstructured.Unstructured

	// TemplateRef is a reference to the template that needs to be cloned.
	TemplateRef *corev1.ObjectReference

	// Namespace is the Kubernetes namespace the cloned object should be created into.
	Namespace string

	// Name is the Kubernetes name that the cloned object should get.
	Name string

	// Labels is an optional map of labels to be added to the object.
	// +optional
	Labels map[string]string

	// Annotations is an optional map of annotations to be added to the object.
	// +optional
	Annotations map[string]string
}

// generateProviderObject generates an object with the given template input.
func generateProviderObject(in *generateProviderObjectInput) (to *unstructured.Unstructured, updated bool, err error) {
	template, found, err := unstructured.NestedMap(in.Template.Object, "spec", "template")
	if err != nil {
		return nil, false, fmt.Errorf("failed to retrieve Spec.Template map on %v %q: %w", in.Template.GroupVersionKind(), in.Template.GetName(), err)
	}
	if !found {
		return nil, false, fmt.Errorf("missing Spec.Template on %v %q", in.Template.GroupVersionKind(), in.Template.GetName())
	}

	to = &unstructured.Unstructured{Object: template}

	if in.ExistingObject != nil {
		// re-use existing object
		to = in.ExistingObject
		newSpec, isEqual, err := unstructuredSpecEqual(in.ExistingObject.Object, template)
		if err != nil {
			return nil, false, fmt.Errorf("failed to compare spec of objects %q and %q: %w", in.ExistingObject.GetName(), in.Template.GetName(), err)
		}

		if !isEqual {
			if err := unstructured.SetNestedMap(to.Object, newSpec, "spec"); err != nil {
				return nil, false, fmt.Errorf("failed to set new spec from template: %w", err)
			}
			updated = true
		}
	} else {
		// create new object
		to.SetResourceVersion("")
		to.SetFinalizers(nil)
		to.SetUID("")
		to.SetSelfLink("")
		to.SetName(in.Name)
		to.SetNamespace(in.Namespace)
	}

	// set annotations
	annotations := to.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	for key, value := range in.Annotations {
		annotations[key] = value
	}
	to.SetAnnotations(annotations)

	// set labels
	labels := to.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	for key, value := range in.Labels {
		labels[key] = value
	}
	to.SetLabels(labels)

	// set APIVersion
	if to.GetAPIVersion() == "" {
		to.SetAPIVersion(in.Template.GetAPIVersion())
	}

	// set kind, e.g. by ProviderClusterStackReleaseTemplate -> ProviderClusterStackRelease
	if to.GetKind() == "" {
		to.SetKind(strings.TrimSuffix(in.Template.GetKind(), "Template"))
	}
	return to, updated, nil
}

func (r *ClusterStackReconciler) getExistingClusterStackReleases(ctx context.Context, clusterStack *csov1alpha1.ClusterStack) ([]*csov1alpha1.ClusterStackRelease, error) {
	csrList := &csov1alpha1.ClusterStackReleaseList{}

	if err := r.List(ctx, csrList, client.InNamespace(clusterStack.Namespace)); err != nil {
		return nil, fmt.Errorf("failed to list ClusterStackReleases: %w", err)
	}

	existingClusterStackReleases := make([]*csov1alpha1.ClusterStackRelease, 0, len(csrList.Items))

	for i := range csrList.Items {
		csr := csrList.Items[i]
		for j := range csr.GetOwnerReferences() {
			ownerRef := csr.GetOwnerReferences()[j]
			if matchesOwnerRef(&ownerRef, clusterStack) {
				existingClusterStackReleases = append(existingClusterStackReleases, &csrList.Items[i])
				break
			}
		}
	}

	return existingClusterStackReleases, nil
}

func makeDiff(clusterStackReleases []*csov1alpha1.ClusterStackRelease, latest, latestReady *string, inSpec, inUse map[string]struct{}) (toCreate, toDelete []*csov1alpha1.ClusterStackRelease) {
	toCreate = make([]*csov1alpha1.ClusterStackRelease, 0, len(clusterStackReleases))
	toDelete = make([]*csov1alpha1.ClusterStackRelease, 0, len(clusterStackReleases))

	mapToCreate := make(map[string]struct{})

	// decide whether existing clusterStackReleases should be kept or not
	for i, cs := range clusterStackReleases {
		var shouldCreate bool

		// if clusterStackRelease is either the latest or the latest that is ready, we want to have it
		if latest != nil && cs.Name == *latest || latestReady != nil && cs.Name == *latestReady {
			shouldCreate = true
		}

		// if the clusterStackRelease is listed in spec, then we want to keep it
		if _, found := inSpec[cs.Name]; found {
			shouldCreate = true
		}

		// if the clusterStackRelease is in use, then we want to keep it
		if _, found := inUse[cs.Name]; found {
			shouldCreate = true
		}

		if shouldCreate {
			toCreate = append(toCreate, clusterStackReleases[i])
			mapToCreate[cs.Name] = struct{}{}
		} else {
			toDelete = append(toDelete, clusterStackReleases[i])
		}
	}

	// make sure that the latest release found by autosubscribe is in the list
	if latest != nil {
		if _, found := mapToCreate[*latest]; !found {
			latestCSR := &csov1alpha1.ClusterStackRelease{}
			latestCSR.Name = *latest
			toCreate = append(toCreate, latestCSR)
		}
	}

	// make sure that all releases from the spec are all in the list
	for cs := range inSpec {
		if _, found := mapToCreate[cs]; !found {
			csr := &csov1alpha1.ClusterStackRelease{}
			csr.Name = cs
			toCreate = append(toCreate, csr)
		}
	}

	return toCreate, toDelete
}

// getLatestReadyClusterStackRelease returns the latest ready clusterStackRelease or nil.
// If one has been found, it returns additionally the Kubernetes version found in the objects's status.
func getLatestReadyClusterStackRelease(clusterStackReleases []*csov1alpha1.ClusterStackRelease) (latest *string, k8sversion string, err error) {
	clusterStackObjects := make(clusterstack.ClusterStacks, 0, len(clusterStackReleases))

	mapKubernetesVersions := make(map[string]string)

	// filter the ones that are ready
	for _, csr := range clusterStackReleases {
		if csr.Status.Ready {
			cs, err := clusterstack.NewFromClusterStackReleaseProperties(csr.Name)
			if err != nil {
				return nil, "", fmt.Errorf("failed to get clusterstack from ClusterStackRelease.Name %q: %w", csr.Name, err)
			}
			clusterStackObjects = append(clusterStackObjects, cs)
			mapKubernetesVersions[cs.StringWithDot()] = csr.Status.KubernetesVersion
		}
	}

	// if no ready cluster stack releases exist, then return an empty string
	if len(clusterStackObjects) == 0 {
		return nil, "", nil
	}

	// sort them according to version
	sort.Sort((clusterStackObjects))

	// return the latest one
	cs := clusterStackObjects[len(clusterStackObjects)-1].StringWithDot()
	latest = &cs
	k8sversion = mapKubernetesVersions[*latest]
	return latest, k8sversion, nil
}

func getLatestReleaseFromRemoteRepository(ctx context.Context, clusterStack *csov1alpha1.ClusterStack, ac assetsclient.Client) (*string, error) {
	releases, err := ac.ListRelease(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list releases on remote repository: %w", err)
	}

	var clusterStacks clusterstack.ClusterStacks

	for _, release := range releases {
		clusterStackObject, matches := matchesSpec(release, clusterStack)
		if matches {
			clusterStacks = append(clusterStacks, clusterStackObject)
		}
	}

	if len(clusterStacks) == 0 {
		return nil, nil
	}

	sort.Sort(clusterStacks)

	str := clusterStacks.Latest().String()
	return &str, nil
}

func (r *ClusterStackReconciler) getClusterStackReleasesInUse(ctx context.Context, namespace string) (map[string]struct{}, error) {
	usedClusterClasses, err := getUsedClusterClasses(ctx, r.Client, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get used ClusterClasses: %w", err)
	}

	mapUsedClusterClasses := make(map[string]struct{})
	for _, clusterClass := range usedClusterClasses {
		mapUsedClusterClasses[clusterClass] = struct{}{}
	}

	return mapUsedClusterClasses, nil
}

func getClusterStackReleasesInSpec(spec *csov1alpha1.ClusterStackSpec) (map[string]struct{}, error) {
	clusterStackReleaseMap := make(map[string]struct{})

	for _, v := range spec.Versions {
		cs, err := clusterstack.New(spec.Provider, spec.Name, spec.KubernetesVersion, v)
		if err != nil {
			return nil, fmt.Errorf("failed to create new cluster stack object from spec: %w", err)
		}

		clusterStackReleaseMap[cs.String()] = struct{}{}
	}
	return clusterStackReleaseMap, nil
}

func getUsableClusterStackReleaseVersions(clusterStackReleases []*csov1alpha1.ClusterStackRelease) ([]string, error) {
	usableVersions := make([]string, 0, len(clusterStackReleases))

	for _, csr := range clusterStackReleases {
		if csr.Status.Ready {
			v, err := version.FromReleaseTag(csr.Name)
			if err != nil {
				return nil, fmt.Errorf("failed to construct version from ClusterStackRelease.Name %q: %w", csr.Name, err)
			}

			usableVersions = append(usableVersions, v.StringWithDot())
		}
	}
	return usableVersions, nil
}

func matchesOwnerRef(a *metav1.OwnerReference, clusterStack *csov1alpha1.ClusterStack) bool {
	aGV, err := schema.ParseGroupVersion(a.APIVersion)
	if err != nil {
		return false
	}

	return aGV.Group == clusterStack.GroupVersionKind().Group && a.Kind == clusterStack.Kind && a.Name == clusterStack.Name
}

func matchesSpec(str string, clusterStack *csov1alpha1.ClusterStack) (clusterstack.ClusterStack, bool) {
	csObject, err := clusterstack.NewFromClusterStackReleaseProperties(str)
	if err != nil {
		record.Warnf(
			clusterStack,
			"FailedToParseClusterStackRelease",
			"failed to get clusterstack object from string %q: %s", str, err.Error(),
		)

		return clusterstack.ClusterStack{}, false
	}

	return csObject, csObject.Version.Channel == clusterStack.Spec.Channel &&
		csObject.KubernetesVersion.StringWithDot() == clusterStack.Spec.KubernetesVersion &&
		csObject.Name == clusterStack.Spec.Name &&
		csObject.Provider == clusterStack.Spec.Provider
}

func unstructuredSpecEqual(oldObj, newObj map[string]interface{}) (newSpec map[string]interface{}, isEqual bool, err error) {
	oldSpec, isEqual, err := unstructured.NestedMap(oldObj, "spec")
	if err != nil {
		return nil, false, fmt.Errorf("failed to retrieve spec map of object: %w", err)
	}
	if !isEqual {
		return nil, false, errors.New("missing spec")
	}

	newSpec, isEqual, err = unstructured.NestedMap(newObj, "spec")
	if err != nil {
		return nil, false, fmt.Errorf("failed to retrieve spec map of object: %w", err)
	}
	if !isEqual {
		return nil, false, errors.New("missing spec")
	}

	return newSpec, reflect.DeepEqual(oldSpec, newSpec), nil
}

func generateOwnerReference(clusterStack *csov1alpha1.ClusterStack) *metav1.OwnerReference {
	return &metav1.OwnerReference{
		APIVersion: clusterStack.APIVersion,
		Kind:       clusterStack.Kind,
		Name:       clusterStack.Name,
		UID:        clusterStack.UID,
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterStackReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&csov1alpha1.ClusterStack{}).
		Owns(&csov1alpha1.ClusterStackRelease{}).
		Watches(
			&csov1alpha1.ClusterStackRelease{},
			handler.EnqueueRequestsFromMapFunc(r.ClusterStackReleaseToClusterStack(ctx)),
			builder.WithPredicates(predicate.Funcs{
				// we are only interested in the update events of a cluster object where cluster.spec.topology.class changed
				UpdateFunc: func(e event.UpdateEvent) bool {
					oldClusterStackRelease, ok := e.ObjectOld.(*csov1alpha1.ClusterStackRelease)
					if !ok {
						return false
					}

					newClusterStackRelease, ok := e.ObjectNew.(*csov1alpha1.ClusterStackRelease)
					if !ok {
						return false
					}

					if !reflect.DeepEqual(oldClusterStackRelease.Status, newClusterStackRelease.Status) {
						return true
					}

					return false
				},
				GenericFunc: func(_ event.GenericEvent) bool {
					return false
				},
				CreateFunc: func(_ event.CreateEvent) bool {
					return false
				},
				DeleteFunc: func(_ event.DeleteEvent) bool {
					return true
				},
			}),
		).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(ctrl.LoggerFrom(ctx), r.WatchFilterValue)).
		Complete(r)
}

// ClusterStackReleaseToClusterStack is a handler.ToRequestsFunc to be used to enqueue requests for reconciliation
// for ClusterStacks that might get updated by changes in ClusterStackReleases.
func (*ClusterStackReconciler) ClusterStackReleaseToClusterStack(ctx context.Context) handler.MapFunc {
	logger := log.FromContext(ctx)
	return func(_ context.Context, o client.Object) []reconcile.Request {
		result := []reconcile.Request{}

		m, ok := o.(*csov1alpha1.ClusterStackRelease)
		if !ok {
			logger.Error(fmt.Errorf("expected a ClusterStackRelease but got a %T", o), "failed to get clusterStackRelease for clusterStack")
			return nil
		}

		// check if the controller reference is already set and return an empty result when one is found.
		for _, ref := range m.GetOwnerReferences() {
			result = append(result, reconcile.Request{NamespacedName: types.NamespacedName{Name: ref.Name, Namespace: m.Namespace}})
		}

		return result
	}
}
