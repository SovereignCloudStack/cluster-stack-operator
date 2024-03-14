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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
	Apply(ctx context.Context, template []byte, oldResources []*csov1alpha1.Resource, shouldDelete bool) (newResources []*csov1alpha1.Resource, shouldRequeue bool, err error)
	Delete(ctx context.Context, template []byte, oldResources []*csov1alpha1.Resource) (newResources []*csov1alpha1.Resource, shouldRequeue bool, err error)

	ApplyNewClusterStack(ctx context.Context, oldTemplate, newTemplate []byte) (shouldRequeue bool, err error)
	DeleteNewClusterStack(ctx context.Context, template []byte) (shouldRequeue bool, err error)
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

func (k *kube) ApplyNewClusterStack(ctx context.Context, oldTemplate, newTemplate []byte) (shouldRequeue bool, err error) {
	logger := log.FromContext(ctx)

	oldObjects, err := parseK8sYaml(oldTemplate)
	if err != nil {
		return false, fmt.Errorf("failed to parse old cluster stack template: %w", err)
	}
	logger.Info("oldObjects value", "val", oldObjects)

	newObjects, err := parseK8sYaml(newTemplate)
	if err != nil {
		return false, fmt.Errorf("failed to parse new cluster stack template: %w", err)
	}

	// oldObjectMap := getResourceMapOfUnstructuredObjects(oldObjects)
	for _, newObject := range newObjects {
		// do nothing if found
		// if _, found := oldObjectMap[types.NamespacedName{Name: newObject.GetName(), Namespace: newObject.GetNamespace()}]; found {
		// 	continue
		// }

		logger.Info("object to be applied", "object", newObject.GetName(), "kind", newObject.GetKind())

		if err := setLabel(newObject, ObjectLabelKeyOwned, ObjectLabelValueOwned); err != nil {
			return false, fmt.Errorf("error setting label: %w", err)
		}

		// call the function and get dynamic.ResourceInterface
		// getDynamicResourceInterface
		dr, err := getDynamicResourceInterface(k.Namespace, k.RestConfig, newObject.GroupVersionKind())
		if err != nil {
			reterr := fmt.Errorf("failed to get dynamic resource interface: %w", err)
			logger.Error(reterr, "failed to get dynamic resource interface", "obj", newObject.GetObjectKind().GroupVersionKind())
			shouldRequeue = true
			continue
		}

		if _, err := dr.Apply(ctx, newObject.GetName(), newObject, metav1.ApplyOptions{FieldManager: "kubectl", Force: true}); err != nil {
			reterr := fmt.Errorf("failed to apply object: %w", err)
			logger.Error(reterr, "failed to apply object", "obj", newObject.GetObjectKind().GroupVersionKind(), "name", newObject.GetName(), "namespace", newObject.GetNamespace())
			shouldRequeue = true
		}
	}

	for _, object := range resourcesToBeDeletedFromUnstructuredObjects(oldObjects, newObjects) {
		logger.Info("resource are being deleted", "kind", object.GetKind(), "name", object.GetName(), "namespace", object.GetNamespace())

		dr, err := getDynamicResourceInterface(k.Namespace, k.RestConfig, object.GroupVersionKind())
		if err != nil {
			return false, fmt.Errorf("failed to get dynamic resource interface: %w", err)
		}

		if err := dr.Delete(ctx, object.GetName(), metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			reterr := fmt.Errorf("failed to delete object: %w", err)
			logger.Error(reterr, "failed to delete object", "obj", object.GroupVersionKind(), "namespacedName", fmt.Sprintf("%s/%s", object.GetNamespace(), object.GetName()))
			// append resource to status and requeue again to be able to retry deletion
			shouldRequeue = true
		}
	}

	return shouldRequeue, nil
}

func (k *kube) DeleteNewClusterStack(ctx context.Context, template []byte) (shouldRequeue bool, err error) {
	logger := log.FromContext(ctx)

	// oldObjects, err := parseK8sYaml(oldTemplate)
	// if err != nil {
	// 	return false, fmt.Errorf("failed to parse old cluster stack template: %w", err)
	// }

	objects, err := parseK8sYaml(template)
	if err != nil {
		return false, fmt.Errorf("failed to parse new cluster stack template: %w", err)
	}

	// oldObjectMap := getResourceMapOfUnstructuredObjects(objects)
	for _, object := range objects {
		// do nothing if synced
		// if _, found := oldObjectMap[types.NamespacedName{Name: newObject.GetName(), Namespace: newObject.GetNamespace()}]; found {
		// 	continue
		// }

		logger.Info("object to be deleted", "object", object.GetName(), "kind", object.GetKind())

		if err := setLabel(object, ObjectLabelKeyOwned, ObjectLabelValueOwned); err != nil {
			return false, fmt.Errorf("error setting label: %w", err)
		}

		dr, err := getDynamicResourceInterface(k.Namespace, k.RestConfig, object.GroupVersionKind())
		if err != nil {
			return false, fmt.Errorf("failed to get dynamic resource interface: %w", err)
		}

		if err := dr.Delete(ctx, object.GetName(), metav1.DeleteOptions{}); err != nil {
			return true, fmt.Errorf("failed to delete object %q: %w", object.GetObjectKind().GroupVersionKind(), err)
		}
	}

	return shouldRequeue, nil
}

func (k *kube) Apply(ctx context.Context, template []byte, oldResources []*csov1alpha1.Resource, shouldDelete bool) (newResources []*csov1alpha1.Resource, shouldRequeue bool, err error) {
	logger := log.FromContext(ctx)

	objs, err := parseK8sYaml(template)
	if err != nil {
		return nil, false, fmt.Errorf("couldn't parse k8s yaml: %w", err)
	}

	resourceMap := getResourceMap(oldResources)
	for _, obj := range objs {
		oldResource, found := resourceMap[types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}]

		// do nothing if synced
		if found && oldResource.Status == csov1alpha1.ResourceStatusSynced {
			newResources = append(newResources, oldResource)
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
			logger.Error(reterr, "failed to apply object", "obj", obj.GetObjectKind().GroupVersionKind(), "name", obj.GetName(), "namespace", obj.GetNamespace())
			shouldRequeue = true
		} else {
			resource.Status = csov1alpha1.ResourceStatusSynced
		}

		newResources = append(newResources, resource)
	}

	if shouldDelete {
		// make a diff between new objs and oldResources to find out
		// a) if an object is in oldResources and synced and not in new objs, then delete should be attempted
		// then, all objs should be applied by create or update
		// at the end, we should delete objects that are supposed to be deleted
		for _, resource := range resourcesToBeDeleted(oldResources, objs) {
			// call the function and get dynamic.ResourceInterface
			// getDynamicResourceInterface
			logger.Info("resource are being deleted", "kind", resource.Kind, "name", resource.Name, "namespace", resource.Namespace)

			dr, err := getDynamicResourceInterface(k.Namespace, k.RestConfig, resource.GroupVersionKind())
			if err != nil {
				return nil, false, fmt.Errorf("failed to get dynamic resource interface: %w", err)
			}

			if err := dr.Delete(ctx, resource.Name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
				reterr := fmt.Errorf("failed to delete object: %w", err)
				logger.Error(reterr, "failed to delete object", "obj", resource.GroupVersionKind(), "namespacedName", resource.NamespacedName())

				// append resource to status and requeue again to be able to retry deletion
				resource.Status = csov1alpha1.ResourceStatusNotSynced
				resource.Error = reterr.Error()
				newResources = append(newResources, resource)
				shouldRequeue = true
			}
		}
	}

	return newResources, shouldRequeue, nil
}

func (k *kube) Delete(ctx context.Context, template []byte, oldResources []*csov1alpha1.Resource) (newResources []*csov1alpha1.Resource, shouldRequeue bool, err error) {
	clientset, err := kubernetes.NewForConfig(k.RestConfig)
	if err != nil {
		return nil, false, fmt.Errorf("failed to create clientset: %w", err)
	}

	objs, err := parseK8sYaml(template)
	if err != nil {
		return nil, false, fmt.Errorf("couldn't parse k8s yaml: %w", err)
	}

	resourceMap := getResourceMap(oldResources)
	for _, obj := range objs {
		oldResource, found := resourceMap[types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}]

		// do nothing if synced
		if found && oldResource.Status == csov1alpha1.ResourceStatusSynced {
			newResources = append(newResources, oldResource)
			continue
		}

		if err := setLabel(obj, ObjectLabelKeyOwned, ObjectLabelValueOwned); err != nil {
			return nil, false, fmt.Errorf("error setting label: %w", err)
		}

		if err := deleteObject(clientset, k.Namespace, k.RestConfig, obj); err != nil {
			return nil, true, fmt.Errorf("failed to delete object %q: %w", obj.GetObjectKind().GroupVersionKind(), err)
		}
	}

	return newResources, shouldRequeue, nil
}
