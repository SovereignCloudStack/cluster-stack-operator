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

package kube

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"

	csov1alpha1 "github.com/SovereignCloudStack/cluster-stack-operator/api/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	cliresource "k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
)

const (
	// DeploymentKind is the Deployment object.
	DeploymentKind = "Deployment"

	// ReplicaSetKind is the ReplicaSet object.
	ReplicaSetKind = "ReplicaSet"

	// StatefulSetKind is the StatefulSet object.
	StatefulSetKind = "StatefulSet"

	// DaemonSetKind is the DaemonSet object.
	DaemonSetKind = "DaemonSet"

	// IngressKind is the Ingress object.
	IngressKind = "Ingress"

	// JobKind is the Job object.
	JobKind = "Job"

	// PodKind is the Pod object.
	PodKind = "Pod"

	// ObjectLabelKeyOwned is the label for objects.
	ObjectLabelKeyOwned = "clusterstack.x-k8s.io/instance"

	// ObjectLabelValueOwned is the object owned.
	ObjectLabelValueOwned = "owned"
)

func parseK8sYaml(template []byte) ([]*unstructured.Unstructured, error) {
	multidocReader := utilyaml.NewYAMLReader(bufio.NewReader(bytes.NewReader(template)))
	var objs []*unstructured.Unstructured

	for {
		buf, err := multidocReader.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
		}

		jsonbuf, err := utilyaml.ToJSON(buf)
		if err != nil {
			return nil, fmt.Errorf("failed to convert yaml to json: %w", err)
		}

		// ignore empty json
		if string(jsonbuf) == "null" {
			continue
		}

		obj, err := runtime.Decode(unstructured.UnstructuredJSONScheme, jsonbuf)
		if err != nil {
			return nil, fmt.Errorf("failed to decode YAML object: %w", err)
		}
		uobj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
		if err != nil {
			return nil, fmt.Errorf("failed to convert YAML to unstructured object: %w", err)
		}
		objs = append(objs, &unstructured.Unstructured{Object: uobj})
	}
	return objs, nil
}

func getResourceMap(resources []*csov1alpha1.Resource) map[types.NamespacedName]*csov1alpha1.Resource {
	resourceMap := make(map[types.NamespacedName]*csov1alpha1.Resource)

	for i, resource := range resources {
		resourceMap[types.NamespacedName{Name: resource.Name, Namespace: resource.Namespace}] = resources[i]
	}
	return resourceMap
}

func getResourceMapOfUnstructuredObjects(objects []*unstructured.Unstructured) map[types.NamespacedName]*unstructured.Unstructured {
	objectMap := make(map[types.NamespacedName]*unstructured.Unstructured)

	for i, object := range objects {
		objectMap[types.NamespacedName{Name: object.GetName(), Namespace: object.GetNamespace()}] = objects[i]
	}

	return objectMap
}

func setLabel(target *unstructured.Unstructured, key, val string) error {
	labels, _, err := unstructured.NestedStringMap(target.Object, "metadata", "labels")
	if err != nil {
		return fmt.Errorf("failed to get labels from target object %s %s/%s: %w", target.GroupVersionKind().String(), target.GetNamespace(), target.GetName(), err)
	}
	if labels == nil {
		labels = make(map[string]string)
	}
	labels[key] = val
	target.SetLabels(labels)

	gvk := schema.FromAPIVersionAndKind(target.GetAPIVersion(), target.GetKind())
	// special case for deployment and job types: make sure that derived replicaset, and pod has
	// the application label
	switch gvk.Group {
	case "apps", "extensions":
		switch gvk.Kind {
		case DeploymentKind, ReplicaSetKind, StatefulSetKind, DaemonSetKind:
			templateLabels, ok, err := unstructured.NestedMap(target.UnstructuredContent(), "spec", "template", "metadata", "labels")
			if err != nil {
				return fmt.Errorf("error setting labels under spec.template for apps/extensions group: %w", err)
			}
			if !ok || templateLabels == nil {
				templateLabels = make(map[string]interface{})
			}
			templateLabels[key] = val
			err = unstructured.SetNestedMap(target.UnstructuredContent(), templateLabels, "spec", "template", "metadata", "labels")
			if err != nil {
				return fmt.Errorf("error setting labels under spec.template for apps/extensions group: %w", err)
			}
			switch target.GetAPIVersion() {
			case "apps/v1beta1", "extensions/v1beta1":
				selector, _, err := unstructured.NestedMap(target.UnstructuredContent(), "spec", "selector")
				if err != nil {
					return fmt.Errorf("error setting labels on .spec.selector for apps/extensions group: %w", err)
				}
				if len(selector) == 0 {
					delete(templateLabels, key)
					err = unstructured.SetNestedMap(target.UnstructuredContent(), templateLabels, "spec", "selector", "matchLabels")
					if err != nil {
						return fmt.Errorf("error setting labels on .spec.selector for apps/extensions group: %w", err)
					}
				}
			}
		}
	case "batch":
		if gvk.Kind == JobKind {
			templateLabels, ok, err := unstructured.NestedMap(target.UnstructuredContent(), "spec", "template", "metadata", "labels")
			if err != nil {
				return fmt.Errorf("error setting labels under spec.template: %w", err)
			}
			if !ok || templateLabels == nil {
				templateLabels = make(map[string]interface{})
			}
			templateLabels[key] = val
			err = unstructured.SetNestedMap(target.UnstructuredContent(), templateLabels, "spec", "template", "metadata", "labels")
			if err != nil {
				return fmt.Errorf("error setting labels under spec.template for batch group: %w", err)
			}
		}
	}
	return nil
}

func getDynamicResourceInterface(namespace string, restConfig *rest.Config, group schema.GroupVersionKind) (dynamic.ResourceInterface, error) {
	dclient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("error creating dynamic client: %w", err)
	}

	dc, err := discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("error creating discovery client: %w", err)
	}

	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(dc))
	mapping, err := mapper.RESTMapping(group.GroupKind(), group.Version)
	if err != nil {
		return nil, fmt.Errorf("error creating rest mapping: %w", err)
	}

	var dr dynamic.ResourceInterface

	if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		dr = dclient.Resource(mapping.Resource).Namespace(namespace)
	} else {
		dr = dclient.Resource(mapping.Resource)
	}

	return dr, nil
}

func resourcesToBeDeleted(oldResources []*csov1alpha1.Resource, currentObjs []*unstructured.Unstructured) []*csov1alpha1.Resource {
	toBeDeleted := make([]*csov1alpha1.Resource, 0, len(oldResources))

	// define object map
	objMap := make(map[types.NamespacedName]struct{})
	for _, obj := range currentObjs {
		namespacedName := types.NamespacedName{
			Namespace: obj.GetNamespace(),
			Name:      obj.GetName(),
		}
		objMap[namespacedName] = struct{}{}
	}

	for i, oldResource := range oldResources {
		if oldResource.Status == csov1alpha1.ResourceStatusSynced {
			namespacedName := types.NamespacedName{
				Namespace: oldResource.Namespace,
				Name:      oldResource.Name,
			}

			// delete resources that are not listed in the new objects anymore
			if _, found := objMap[namespacedName]; !found {
				toBeDeleted = append(toBeDeleted, oldResources[i])
			}
		}
	}
	return toBeDeleted
}

func resourcesToBeDeletedFromUnstructuredObjects(oldObjects, newObjects []*unstructured.Unstructured) []*unstructured.Unstructured {
	toBeDeleted := make([]*unstructured.Unstructured, 0, len(oldObjects))

	newObjectMap := make(map[types.NamespacedName]struct{})
	for _, newObj := range newObjects {
		newObjectMap[types.NamespacedName{Name: newObj.GetName(), Namespace: newObj.GetNamespace()}] = struct{}{}
	}

	for i, oldObj := range oldObjects {
		nameSpacedName := types.NamespacedName{
			Name:      oldObj.GetName(),
			Namespace: oldObj.GetNamespace(),
		}

		// delete resources that are not listed in the new objects anymore
		if _, found := newObjectMap[nameSpacedName]; !found {
			toBeDeleted = append(toBeDeleted, oldObjects[i])
		}
	}

	return toBeDeleted
}

func deleteObject(kubeClientset kubernetes.Interface, namespace string, restConfig *rest.Config, obj runtime.Object) error {
	// Create a REST mapper that tracks information about the available resources in the cluster.
	groupResources, err := restmapper.GetAPIGroupResources(kubeClientset.Discovery())
	if err != nil {
		return fmt.Errorf("failed to get API group resources: %w", err)
	}
	rm := restmapper.NewDiscoveryRESTMapper(groupResources)

	// Get some metadata needed to make the REST request.
	gvk := obj.GetObjectKind().GroupVersionKind()
	gk := schema.GroupKind{Group: gvk.Group, Kind: gvk.Kind}
	mapping, err := rm.RESTMapping(gk, gvk.Version)
	if err != nil {
		return fmt.Errorf("failed to create REST mapping: %w", err)
	}

	// Create a client specifically for deleting the object.
	restClient, err := newRestClient(restConfig, mapping.GroupVersionKind.GroupVersion())
	if err != nil {
		return fmt.Errorf("failed to create new REST client: %w", err)
	}

	objName, err := getName(obj)
	if err != nil {
		return fmt.Errorf("failed to get name: %w", err)
	}

	// Use the REST helper to delete the object from the specified namespace.
	restHelper := cliresource.NewHelper(restClient, mapping)
	_, err = restHelper.Delete(namespace, objName)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete object: %w", err)
	}

	return nil
}

func getName(obj runtime.Object) (string, error) {
	switch t := obj.(type) {
	case metav1.Object:
		return t.GetName(), nil
	default:
		return "", fmt.Errorf("object does not implement GetName()")
	}
}

func newRestClient(restConfig *rest.Config, gv schema.GroupVersion) (rest.Interface, error) {
	restConfig.ContentConfig = cliresource.UnstructuredPlusDefaultContentConfig()
	restConfig.GroupVersion = &gv
	if gv.Group == "" {
		restConfig.APIPath = "/api"
	} else {
		restConfig.APIPath = "/apis"
	}

	restInterface, err := rest.RESTClientFor(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to get REST client: %w", err)
	}

	return restInterface, nil
}
