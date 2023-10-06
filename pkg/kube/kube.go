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

// Package kube implements important interfaces like the Kube.
package kube

import (
	"context"
	"fmt"

	csov1alpha1 "github.com/SovereignCloudStack/cluster-stack-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type kube struct {
	RestConfig *rest.Config
	Namespace  string
}

// Client has all the meathod for helm chart kube operation.
type Client interface {
	Apply(ctx context.Context, template []byte, oldResources []*csov1alpha1.Resource) (newResources []*csov1alpha1.Resource, shouldRequeue bool, err error)
	Update(ctx context.Context, template []byte, oldResources []*csov1alpha1.Resource) (newResources []*csov1alpha1.Resource, shouldRequeue bool, err error)
	Delete(template []byte) error
}

// Factory creates new fake kube client factories.
type Factory interface {
	NewClient(namespace string, resCfg *rest.Config) Client
}

type factory struct{}

var _ = Factory(&factory{})

// NewFactory has method to create a new factory for kube clients.
func NewFactory() Factory {
	return &factory{}
}

var _ Client = &kube{}

func (*factory) NewClient(namespace string, resCfg *rest.Config) Client {
	return &kube{
		Namespace:  namespace,
		RestConfig: resCfg,
	}
}

func (k *kube) Apply(ctx context.Context, template []byte, oldResources []*csov1alpha1.Resource) (newResources []*csov1alpha1.Resource, shouldRequeue bool, err error) {
	logger := log.FromContext(ctx)

	objs, err := parseK8sYaml(template)
	if err != nil {
		return nil, false, fmt.Errorf("couldn't parse k8s yaml: %w", err)
	}

	resourceMap, newResources := getResourceMap(oldResources)
	for _, obj := range objs {
		oldResource, found := resourceMap[types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}]

		// do nothing if synced
		if found && oldResource.Status == csov1alpha1.ResourceStatusSynced {
			continue
		}

		if err := setLabel(obj, ObjectLabelKeyOwned, ObjectLabelValueOwned); err != nil {
			return nil, false, fmt.Errorf("error setting label: %w", err)
		}

		resource := csov1alpha1.NewResourceFromUnstructured(obj)

		// call the function and get dynamic.ResourceInterface
		// getDynamicResourceInterface
		dr, err := getDynamicResourceInterface(k.Namespace, k.RestConfig, obj.GroupVersionKind())
		if err != nil {
			reterr := fmt.Errorf("failed to get dynamic resource interface: %w", err)
			resource.Error = reterr.Error()
			resource.Status = csov1alpha1.ResourceStatusNotSynced
			logger.Error(reterr, "failed to get dynamic resource interface", "obj", obj.GetObjectKind().GroupVersionKind())
			shouldRequeue = true
			continue
		}

		if _, err := dr.Apply(ctx, obj.GetName(), obj, metav1.ApplyOptions{FieldManager: "kubectl", Force: true}); err != nil {
			reterr := fmt.Errorf("failed to apply object: %w", err)
			resource.Error = reterr.Error()
			resource.Status = csov1alpha1.ResourceStatusNotSynced
			logger.Error(reterr, "failed to apply object", "obj", obj.GetObjectKind().GroupVersionKind())
			shouldRequeue = true
		} else {
			resource.Status = csov1alpha1.ResourceStatusSynced
		}

		newResources = append(newResources, resource)
	}

	return newResources, shouldRequeue, nil
}

func (k *kube) Update(ctx context.Context, template []byte, oldResources []*csov1alpha1.Resource) (newResources []*csov1alpha1.Resource, shouldRequeue bool, err error) {
	logger := log.FromContext(ctx)

	objs, err := parseK8sYaml(template)
	if err != nil {
		return nil, false, fmt.Errorf("couldn't parse k8s yaml: %w", err)
	}

	for _, obj := range objs {
		if err := setLabel(obj, ObjectLabelKeyOwned, ObjectLabelValueOwned); err != nil {
			return nil, false, fmt.Errorf("error setting label: %w", err)
		}

		// call the function and get dynamic.ResourceInterface
		// getDynamicResourceInterface
		dr, err := getDynamicResourceInterface(k.Namespace, k.RestConfig, obj.GroupVersionKind())
		if err != nil {
			return nil, false, fmt.Errorf("failed to get dynamic resource interface: %w", err)
		}

		resource := csov1alpha1.NewResourceFromUnstructured(obj)

		if _, err := dr.Apply(ctx, obj.GetName(), obj, metav1.ApplyOptions{FieldManager: "kubectl", Force: true}); err != nil {
			reterr := fmt.Errorf("failed to apply object: %w", err)
			resource.Error = reterr.Error()
			resource.Status = csov1alpha1.ResourceStatusNotSynced
			logger.Error(reterr, "failed to apply object", "obj", obj.GetObjectKind().GroupVersionKind())
			shouldRequeue = true
		} else {
			resource.Status = csov1alpha1.ResourceStatusSynced
		}

		newResources = append(newResources, resource)
	}

	// make a diff between new objs and oldResources to find out
	// a) if an object is in oldResources and synced and not in new objs, then delete should be attempted
	// then, all objs should be applied by create or update
	// at the end, we should delete objects that are supposed to be deleted
	for _, resource := range resourcesToBeDeleted(oldResources, objs) {
		// call the function and get dynamic.ResourceInterface
		// getDynamicResourceInterface

		dr, err := getDynamicResourceInterface(k.Namespace, k.RestConfig, resource.GroupVersionKind())
		if err != nil {
			return nil, false, fmt.Errorf("failed to get dynamic resource interface: %w", err)
		}

		if err := dr.Delete(ctx, resource.Name, metav1.DeleteOptions{}); err == nil {
			return nil, false, fmt.Errorf("failed to delete object %q of namespace %q: %w", resource.Name, resource.Namespace, err)
		}
	}
	return newResources, shouldRequeue, nil
}

func (k *kube) Delete(template []byte) error {
	clientset, err := kubernetes.NewForConfig(k.RestConfig)
	if err != nil {
		return fmt.Errorf("failed to create clientset: %w", err)
	}

	objs, err := parseK8sYaml(template)
	if err != nil {
		return fmt.Errorf("couldn't parse k8s yaml: %w", err)
	}

	for _, obj := range objs {
		if err := deleteObject(clientset, k.Namespace, k.RestConfig, obj); err != nil {
			return fmt.Errorf("failed to delete object %q: %w", obj.GetObjectKind().GroupVersionKind(), err)
		}
	}
	return nil
}
