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

package v1alpha1

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:path=/validate-clusterstack-x-k8s-io-v1alpha1-clusteraddon,mutating=false,failurePolicy=fail,sideEffects=None,groups=clusterstack.x-k8s.io,resources=clusteraddons,verbs=create;update,versions=v1alpha1,name=validation.clusteraddon.clusterstack.x-k8s.io,admissionReviewVersions={v1}

var _ webhook.CustomValidator = &ClusterAddonWebhook{}

// ClusterAddonWebhook defines the webhook for ClusterAddon.
type ClusterAddonWebhook struct{}

// SetupWebhookWithManager initializes webhook manager for ClusterAddon.
func (w *ClusterAddonWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&ClusterAddon{}).
		WithValidator(w).
		Complete()
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (w *ClusterAddonWebhook) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	clusterAddon, ok := obj.(*ClusterAddon)
	if !ok {
		return admission.Warnings{}, apierrors.NewBadRequest(fmt.Sprintf("expected a ClusterAddon but got a %T", obj))
	}
	var allErrs field.ErrorList

	if clusterAddon.Spec.ClusterRef == nil {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "clusterRef"), clusterAddon.Spec.ClusterRef, "must not be empty"))
	} else if clusterAddon.Spec.ClusterRef.Kind != "Cluster" {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "clusterRef", "kind"), clusterAddon.Spec.ClusterRef.Kind, "kind must be Cluster"))
	}

	return nil, aggregateObjErrors(clusterAddon.GroupVersionKind().GroupKind(), clusterAddon.Name, allErrs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (w *ClusterAddonWebhook) ValidateUpdate(_ context.Context, old runtime.Object, new runtime.Object) (admission.Warnings, error) {
	oldM, ok := old.(*ClusterAddon)
	if !ok {
		return admission.Warnings{}, apierrors.NewBadRequest(fmt.Sprintf("expected an ClusterAddon but got a %T", old))
	}
	newM, ok := new.(*ClusterAddon)
	if !ok {
		return admission.Warnings{}, apierrors.NewBadRequest(fmt.Sprintf("expected a ClusterAddon but got a %T", new))
	}

	var allErrs field.ErrorList

	if newM.Spec.ClusterRef == nil {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "clusterRef"), newM.Spec.ClusterRef, "must not be empty"))
		return admission.Warnings{}, aggregateObjErrors(newM.GroupVersionKind().GroupKind(), newM.Name, allErrs)
	}

	// clusterRef.Name is immutable
	if oldM.Spec.ClusterRef.Name != newM.Spec.ClusterRef.Name {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "clusterRef", "name"), newM.Spec.ClusterRef.Name, "field is immutable"),
		)
	}

	// namespace needs to always be the same for clusterAddon and cluster
	if newM.Spec.ClusterRef.Namespace != newM.Namespace {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "clusterRef", "namespace"), newM.Spec.ClusterRef.Namespace, "cluster and clusterAddon need to be in same namespace"),
		)
	}

	// clusterRef.kind is immutable
	if oldM.Spec.ClusterRef.Kind != newM.Spec.ClusterRef.Kind {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "clusterRef", "kind"), newM.Spec.ClusterRef.Kind, "field is immutable"),
		)
	}

	return nil, aggregateObjErrors(newM.GroupVersionKind().GroupKind(), newM.Name, allErrs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (w *ClusterAddonWebhook) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
