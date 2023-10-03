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

package builder

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// InfrastructureGroupVersion is group version used for infrastructure objects.
	InfrastructureGroupVersion = schema.GroupVersion{Group: "infrastructure.clusterstack.x-k8s.io", Version: "v1alpha1"}

	// TestInfrastructureProviderClusterStackReleaseTemplateKind is the kind for the InfrastructureProviderClusterStackReleaseTemplate type.
	TestInfrastructureProviderClusterStackReleaseTemplateKind = "TestInfrastructureProviderClusterStackReleaseTemplate"
	// TestInfrastructureProviderClusterStackReleaseTemplateCRD is a test InfrastructureProviderClusterStackReleaseTemplate CRD.
	TestInfrastructureProviderClusterStackReleaseTemplateCRD = testInfrastructureProviderClusterStackReleaseTemplateCRD(InfrastructureGroupVersion.WithKind(TestInfrastructureProviderClusterStackReleaseTemplateKind))

	// TestInfrastructureProviderClusterStackReleaseKind is the kind for the InfrastructureProviderClusterStackRelease type.
	TestInfrastructureProviderClusterStackReleaseKind = "TestInfrastructureProviderClusterStackRelease"
	// TestInfrastructureProviderClusterStackReleaseCRD is a test InfrastructureProviderClusterStackRelease CRD.
	TestInfrastructureProviderClusterStackReleaseCRD = testInfrastructureProviderClusterStackReleaseCRD(InfrastructureGroupVersion.WithKind(TestInfrastructureProviderClusterStackReleaseKind))
)

func testInfrastructureProviderClusterStackReleaseTemplateCRD(gvk schema.GroupVersionKind) *apiextensionsv1.CustomResourceDefinition {
	return generateCRD(gvk, map[string]apiextensionsv1.JSONSchemaProps{
		"metadata": {
			Type: "object",
		},
		"spec": infrastructureProviderClusterStackReleaseTemplateSpecSchema,
	})
}

func testInfrastructureProviderClusterStackReleaseCRD(gvk schema.GroupVersionKind) *apiextensionsv1.CustomResourceDefinition {
	return generateCRD(gvk, map[string]apiextensionsv1.JSONSchemaProps{
		"metadata": {
			Type: "object",
		},
		"spec": infrastructureProviderClusterStackRelaseSpecSchema,
		"status": {
			Type: "object",
			Properties: map[string]apiextensionsv1.JSONSchemaProps{
				"ready": {Type: "boolean"},
			},
		},
	})
}

var (
	infrastructureProviderClusterStackReleaseTemplateSpecSchema = apiextensionsv1.JSONSchemaProps{
		Type: "object",
		Properties: map[string]apiextensionsv1.JSONSchemaProps{
			"template": {
				Type: "object",
				Properties: map[string]apiextensionsv1.JSONSchemaProps{
					"spec": infrastructureProviderClusterStackRelaseSpecSchema,
				},
			},
		},
	}

	infrastructureProviderClusterStackRelaseSpecSchema = apiextensionsv1.JSONSchemaProps{
		Type: "object",
		Properties: map[string]apiextensionsv1.JSONSchemaProps{
			"nodeImages": {
				Type: "array",
				Items: &apiextensionsv1.JSONSchemaPropsOrArray{
					Schema: &apiextensionsv1.JSONSchemaProps{
						Type: "string",
					},
				},
			},
		},
	}
)
