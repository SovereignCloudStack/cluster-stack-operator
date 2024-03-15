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
	"reflect"
	"strings"

	"github.com/SovereignCloudStack/cluster-stack-operator/pkg/clusterstack"
	"github.com/SovereignCloudStack/cluster-stack-operator/pkg/version"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// ClusterStackWebhook implements validating and defaulting webhook for clusterstack.
// +k8s:deepcopy-gen=false
type ClusterStackWebhook struct {
	LocalMode bool
	client.Client
}

// SetupWebhookWithManager initializes webhook manager for ClusterStack.
func (r *ClusterStackWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&ClusterStack{}).
		WithValidator(r).
		Complete()
}

// SetupWebhookWithManager initializes webhook manager for ClusterStackList.
func (r *ClusterStackList) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:path=/validate-clusterstack-x-k8s-io-v1alpha1-clusterstack,mutating=false,failurePolicy=fail,sideEffects=None,groups=clusterstack.x-k8s.io,resources=clusterstacks,verbs=create;update;delete,versions=v1alpha1,name=validation.clusterstack.clusterstack.x-k8s.io,admissionReviewVersions={v1,v1alpha1}

var _ webhook.CustomValidator = &ClusterStackWebhook{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (r *ClusterStackWebhook) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	clusterStack, ok := obj.(*ClusterStack)
	if !ok {
		return admission.Warnings{}, apierrors.NewBadRequest(fmt.Sprintf("expected a Cluster but got a %T", obj))
	}

	var allErrs field.ErrorList

	if r.LocalMode && clusterStack.Spec.AutoSubscribe {
		field.Invalid(field.NewPath("spec", "autosubscribe"), clusterStack.Spec.AutoSubscribe, "can't autosubscribe in localMode")
	}

	// validate versions and validate that versions match with the channel specified.
	for _, v := range clusterStack.Spec.Versions {
		if _, err := version.New(v); err != nil {
			allErrs = append(allErrs,
				field.Invalid(field.NewPath("spec", "versions"), clusterStack.Spec.Versions, fmt.Sprintf("invalid version: %s", err.Error())),
			)
		}
	}

	if clusterStack.Spec.ProviderRef == nil && !clusterStack.Spec.NoProvider {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "providerRef"), clusterStack.Spec.ProviderRef, "empty provider ref, even though noProvider mode is turned off"),
		)
	}

	return nil, aggregateObjErrors(clusterStack.GroupVersionKind().GroupKind(), clusterStack.Name, allErrs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (r *ClusterStackWebhook) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	oldClusterStack, ok := oldObj.(*ClusterStack)
	if !ok {
		return admission.Warnings{}, apierrors.NewBadRequest(fmt.Sprintf("expected an ClusterStack but got a %T", oldObj))
	}

	newClusterStack, ok := newObj.(*ClusterStack)
	if !ok {
		return admission.Warnings{}, apierrors.NewBadRequest(fmt.Sprintf("expected an ClusterStack but got a %T", newClusterStack))
	}

	var allErrs field.ErrorList

	if r.LocalMode && newClusterStack.Spec.AutoSubscribe {
		field.Invalid(field.NewPath("spec", "autosubscribe"), newClusterStack.Spec.AutoSubscribe, "can't autosubscribe in localMode")
	}

	// provider is immutable
	if !reflect.DeepEqual(oldClusterStack.Spec.Provider, newClusterStack.Spec.Provider) {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "provider"), newClusterStack.Spec.Provider, "field is immutable"),
		)
	}

	// name is immutable
	if !reflect.DeepEqual(oldClusterStack.Spec.Name, newClusterStack.Spec.Name) {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "name"), newClusterStack.Spec.Name, "field is immutable"),
		)
	}

	// channel is immutable
	if !reflect.DeepEqual(oldClusterStack.Spec.Channel, newClusterStack.Spec.Channel) {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "channel"), newClusterStack.Spec.Channel, "field is immutable"),
		)
	}

	// KubernetesVersion is immutable
	if !reflect.DeepEqual(oldClusterStack.Spec.KubernetesVersion, newClusterStack.Spec.KubernetesVersion) {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "kubernetesVersion"), newClusterStack.Spec.KubernetesVersion, "field is immutable"),
		)
	}

	return nil, aggregateObjErrors(newClusterStack.GroupVersionKind().GroupKind(), newClusterStack.Name, allErrs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (r *ClusterStackWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	clusterStack, ok := obj.(*ClusterStack)
	if !ok {
		return admission.Warnings{}, apierrors.NewBadRequest(fmt.Sprintf("expected a ClusterStack but got a %T", obj))
	}

	clusters, err := r.getClustersUsingClusterStack(ctx, clusterStack)
	if err != nil {
		return admission.Warnings{fmt.Sprintf("cannot validate whether clusterstack is still in use. Watch out for potentially orphaned resources: %s", err.Error())}, nil
	}
	if len(clusters) > 0 {
		return admission.Warnings{}, apierrors.NewBadRequest(fmt.Sprintf("can't delete ClusterStack as there are clusters using it: [%q]", strings.Join(clusters, ", ")))
	}

	return nil, nil
}

func (r *ClusterStackWebhook) getClustersUsingClusterStack(ctx context.Context, clusterStack *ClusterStack) ([]string, error) {
	clusterList := &clusterv1.ClusterList{}
	if err := r.List(ctx, clusterList, &client.ListOptions{Namespace: clusterStack.Namespace}); err != nil {
		return nil, fmt.Errorf("failed to list clusters: %w", err)
	}

	clusters := make([]string, 0, len(clusterList.Items))

	// list the names of all ClusterClasses that are referenced in Cluster objects
	for i := range clusterList.Items {
		if clusterList.Items[i].Spec.Topology == nil {
			continue
		}

		clusterClass := clusterList.Items[i].Spec.Topology.Class
		if clusterClass == "" {
			continue
		}

		cs, err := clusterstack.NewFromClusterClassProperties(clusterClass)
		if err != nil {
			continue
		}
		if clusterStack.Spec.Provider == cs.Provider &&
			clusterStack.Spec.Channel == cs.Version.Channel &&
			clusterStack.Spec.Name == cs.Name &&
			clusterStack.Spec.KubernetesVersion == cs.KubernetesVersion.StringWithDot() {
			clusters = append(clusters, clusterList.Items[i].Name)
		}
	}

	return clusters, nil
}
