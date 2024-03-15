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
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// SetupWebhookWithManager initializes webhook manager for ClusterAddon.
func (r *ClusterAddon) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// SetupWebhookWithManager initializes webhook manager for ClusterAddonList.
func (r *ClusterAddonList) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/validate-clusterstack-x-k8s-io-v1alpha1-clusteraddon,mutating=false,failurePolicy=fail,sideEffects=None,groups=clusterstack.x-k8s.io,resources=clusteraddons,verbs=create;update,versions=v1alpha1,name=validation.clusteraddon.clusterstack.x-k8s.io,admissionReviewVersions={v1,v1alpha1}

var _ webhook.Validator = &ClusterAddon{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (r *ClusterAddon) ValidateCreate() (admission.Warnings, error) {
	var allErrs field.ErrorList

	if r.Spec.ClusterRef == nil {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "clusterRef"), r.Spec.ClusterRef, "must not be empty"))
	} else if r.Spec.ClusterRef.Kind != "Cluster" {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "clusterRef", "kind"), r.Spec.ClusterRef.Kind, "kind must be cluster"))
	}

	return nil, aggregateObjErrors(r.GroupVersionKind().GroupKind(), r.Name, allErrs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (r *ClusterAddon) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	oldM, ok := old.(*ClusterAddon)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected an ClusterAddon but got a %T", old))
	}

	var allErrs field.ErrorList

	if r.Spec.ClusterRef == nil {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "clusterRef"), r.Spec.ClusterRef, "must not be empty"))
		return nil, aggregateObjErrors(r.GroupVersionKind().GroupKind(), r.Name, allErrs)
	}

	// clusterRef.Name is immutable
	if oldM.Spec.ClusterRef.Name != r.Spec.ClusterRef.Name {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "clusterRef", "name"), r.Spec.ClusterRef.Name, "field is immutable"),
		)
	}

	// namespace needs to always be the same for clusterAddon and cluster
	if r.Spec.ClusterRef.Namespace != r.Namespace {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "clusterRef", "namespace"), r.Spec.ClusterRef.Namespace, "cluster and clusterAddon need to be in same namespace"),
		)
	}

	// clusterRef.kind is immutable
	if oldM.Spec.ClusterRef.Kind != r.Spec.ClusterRef.Kind {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "clusterRef", "kind"), r.Spec.ClusterRef.Kind, "field is immutable"),
		)
	}

	return nil, aggregateObjErrors(r.GroupVersionKind().GroupKind(), r.Name, allErrs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (*ClusterAddon) ValidateDelete() (admission.Warnings, error) {
	return nil, nil
}
