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
	"github.com/SovereignCloudStack/cluster-stack-operator/pkg/release"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// Cluster implements a validating and defaulting webhook for Cluster.
// +k8s:deepcopy-gen=false
type Cluster struct {
	Client client.Client
}

// SetupWebhookWithManager initializes webhook manager for ClusterStack.
func (r *Cluster) SetupWebhookWithManager(mgr ctrl.Manager) error {
	r.Client = mgr.GetClient()
	return ctrl.NewWebhookManagedBy(mgr).
		For(&clusterv1.Cluster{}).
		WithValidator(r).
		Complete()
}

//+kubebuilder:webhook:path=/validate-cluster-x-k8s-io-v1beta2-cluster,mutating=false,failurePolicy=fail,sideEffects=None,groups=cluster.x-k8s.io,resources=clusters,verbs=create;update,versions=v1beta2,name=validation.cluster.cluster.x-k8s.io,admissionReviewVersions={v1,v1beta1}

var _ admission.CustomValidator = &Cluster{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (r *Cluster) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	cluster, ok := obj.(*clusterv1.Cluster)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a Cluster but got a %T", obj))
	}

	return r.isVersionCorrect(ctx, cluster)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (r *Cluster) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	oldCluster, ok := oldObj.(*clusterv1.Cluster)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected an Cluster but got a %T", oldCluster))
	}

	newCluster, ok := newObj.(*clusterv1.Cluster)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected an Cluster but got a %T", newCluster))
	}

	var allErrs field.ErrorList

	warnings, err := r.isVersionCorrect(ctx, newCluster)
	if len(warnings) > 0 || err != nil {
		return warnings, err
	}

	csOld, err := clusterstack.NewFromClusterClassProperties(oldCluster.Spec.Topology.ClassRef.Name)
	if err != nil {
		return nil, fmt.Errorf("expected a clusterclass of form <provider>-<clusterStackName>-<kubernetesVersion>-<clusterStackVersion> but got %s: %w", oldCluster.Spec.Topology.ClassRef.Name, err)
	}

	csNew, err := clusterstack.NewFromClusterClassProperties(newCluster.Spec.Topology.ClassRef.Name)
	if err != nil {
		return nil, fmt.Errorf("expected a clusterclass of form <provider>-<clusterStackName>-<kubernetesVersion>-<clusterStackVersion> but got %s: %w", newCluster.Spec.Topology.ClassRef.Name, err)
	}

	// provider must not change
	if csOld.Provider != csNew.Provider {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "topology", "classRef"), newCluster.Spec.Topology.ClassRef.Name, fmt.Sprintf("provider name must not change. Got %s, want %s", csNew.Provider, csOld.Provider)))
	}

	// cluster stack name must not change
	if csOld.Name != csNew.Name {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "topology", "classRef"), newCluster.Spec.Topology.ClassRef.Name, fmt.Sprintf("cluster stack name must not change. Got %s, want %s", csNew.Name, csOld.Name)))
	}

	// kubernetes version must be the same or higher by one
	if csNew.KubernetesVersion.Minor-csOld.KubernetesVersion.Minor != 1 && csNew.KubernetesVersion.Minor-csOld.KubernetesVersion.Minor != 0 {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "topology", "classRef"), newCluster.Spec.Topology.ClassRef.Name, fmt.Sprintf("kubernetes version must be the same or higher by one. Got %s, want %s or 1-%d", csNew.KubernetesVersion, csOld.KubernetesVersion, csOld.KubernetesVersion.Minor+1)))
	}

	return nil, aggregateObjErrors(oldCluster.GroupVersionKind().GroupKind(), oldCluster.Name, allErrs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (*Cluster) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (r *Cluster) isVersionCorrect(ctx context.Context, cluster *clusterv1.Cluster) (admission.Warnings, error) {
	if cluster.Spec.Topology.ClassRef.Name == "" {
		return nil, field.Invalid(field.NewPath("spec", "topology", "classRef"), cluster.Spec.Topology.ClassRef.Name, "classRef field cannot be empty")
	}
	
	if cluster.Spec.Topology.Version == "" {
		return nil, field.Invalid(field.NewPath("spec", "topology", "version"), cluster.Spec.Topology.Version, "version field cannot be empty")
	}

	namespace := cluster.Namespace
	if cluster.Spec.Topology.ClassRef.Namespace != "" {
		namespace = cluster.Spec.Topology.ClassRef.Namespace
	}
	wantKubernetesVersion, err := r.getClusterStackReleaseVersion(ctx, release.ConvertFromClusterClassToClusterStackFormat(cluster.Spec.Topology.ClassRef.Name), namespace)
	if err != nil {
		return admission.Warnings{fmt.Sprintf("cannot validate clusterClass and Kubernetes version. Getting clusterStackRelease object failed: %s", err.Error())}, nil
	}

	if wantKubernetesVersion == "" {
		return admission.Warnings{fmt.Sprintf("no Kubernetes version set in status of clusterStackRelease object. Cannot validate Kubernetes version. Check out the ClusterStackReleaseObject %s/%s manually", cluster.Namespace, cluster.Spec.Topology.ClassRef.Name)}, nil
	}

	if cluster.Spec.Topology.Version != wantKubernetesVersion {
		return nil, field.Invalid(field.NewPath("spec", "topology", "version"), cluster.Spec.Topology.Version, fmt.Sprintf("clusterClass %s expects Kubernetes version %s, but got %s", cluster.Spec.Topology.ClassRef.Name, wantKubernetesVersion, cluster.Spec.Topology.Version))
	}
	return nil, nil
}

func (r *Cluster) getClusterStackReleaseVersion(ctx context.Context, name, namespace string) (string, error) {
	clusterStackRelCR := &ClusterStackRelease{}
	namespacedName := types.NamespacedName{Name: name, Namespace: namespace}

	// ponytail: the ClusterStackRelease CRD may not yet be registered when the
	// Cluster webhook fires (e.g. during envtest startup or rapid create/delete
	// sequences). In that case the API server returns "the server could not
	// find the requested resource" instead of a normal 404. Treat this as
	// "no ClusterStackRelease exists" and skip version validation gracefully.
	if err := r.Client.Get(ctx, namespacedName, clusterStackRelCR); err != nil {
		if apierrors.IsNotFound(err) || strings.Contains(err.Error(), "the server could not find the requested resource") {
			return "", nil
		}
		return "", fmt.Errorf("failed to get ClusterStackRelease - cannot validate Kubernetes version: %w", err)
	}
	return clusterStackRelCR.Status.KubernetesVersion, nil
}
