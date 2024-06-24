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
	"strings"

	"github.com/SovereignCloudStack/cluster-stack-operator/pkg/clusterstack"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// ClusterStackReleaseWebhook implements validating and defaulting webhook for ClusterStackRelease.
// +k8s:deepcopy-gen=false
type ClusterStackReleaseWebhook struct {
	client.Client
}

// SetupWebhookWithManager initializes webhook manager for ClusterStackRelease.
func (r *ClusterStackReleaseWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&ClusterStackRelease{}).
		WithValidator(r).
		Complete()
}

// SetupWebhookWithManager initializes webhook manager for ClusterStackReleaseList.
func (r *ClusterStackReleaseList) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:path=/validate-clusterstack-x-k8s-io-v1alpha1-clusterstackrelease,mutating=false,failurePolicy=fail,sideEffects=None,groups=clusterstack.x-k8s.io,resources=clusterstackreleases,verbs=delete,versions=v1alpha1,name=validation.clusterstackrelease.clusterstack.x-k8s.io,admissionReviewVersions={v1,v1alpha1}

var _ webhook.CustomValidator = &ClusterStackReleaseWebhook{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (*ClusterStackReleaseWebhook) ValidateCreate(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (*ClusterStackReleaseWebhook) ValidateUpdate(_ context.Context, _, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (r *ClusterStackReleaseWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	clusterStackRelease, ok := obj.(*ClusterStackRelease)
	if !ok {
		return admission.Warnings{}, apierrors.NewBadRequest(fmt.Sprintf("expected a ClusterStackRelease but got a %T", obj))
	}

	clusters, err := r.getClustersUsingClusterStackRelease(ctx, clusterStackRelease)
	if err != nil {
		return admission.Warnings{fmt.Sprintf("cannot validate whether clusterstackrelease is still in use. Watch out for potentially orphaned resources: %s", err.Error())}, nil
	}
	if len(clusters) > 0 {
		return admission.Warnings{}, apierrors.NewBadRequest(fmt.Sprintf("can't delete ClusterStackRelease as there are clusters using it: [%q]", strings.Join(clusters, ", ")))
	}

	return nil, nil
}

func (r *ClusterStackReleaseWebhook) getClustersUsingClusterStackRelease(ctx context.Context, clusterStackRelease *ClusterStackRelease) ([]string, error) {
	clusterList := &clusterv1.ClusterList{}
	if err := r.List(ctx, clusterList, &client.ListOptions{Namespace: clusterStackRelease.Namespace}); err != nil {
		return nil, fmt.Errorf("failed to list clusters: %w", err)
	}

	clusters := make([]string, 0, len(clusterList.Items))

	// list the names of all ClusterClasses that are referenced in Cluster objects
	for i := range clusterList.Items {
		if clusterList.Items[i].Spec.Topology == nil {
			continue
		}

		clusterClass := clusterList.Items[i].Spec.Topology.Class
		clusterClassFormated, err := clusterstack.NewFromClusterClassProperties(clusterClass)
		if err != nil {
			return nil, fmt.Errorf("failed to read properties from clusterClass string: %w", err)
		}
		clusterStackReleaseFormatted, err := clusterstack.NewFromClusterStackReleaseProperties(clusterStackRelease.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to read properties from clusterStackRelease object name: %w", err)
		}

		if clusterClassFormated.Name == clusterStackReleaseFormatted.Name &&
			clusterClassFormated.Provider == clusterStackReleaseFormatted.Provider &&
			clusterClassFormated.KubernetesVersion == clusterStackReleaseFormatted.KubernetesVersion &&
			clusterClassFormated.Version.String() == clusterStackReleaseFormatted.Version.String() {
			clusters = append(clusters, clusterList.Items[i].Name)
		}
	}

	return clusters, nil
}
