/*
Copyright 2026 The Kubernetes Authors.

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

// Package clusterclass contains helpers to find out which clusters use ClusterClasses of a namespace.
package clusterclass

import (
	"context"
	"fmt"

	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Namespace returns the namespace of the ClusterClass that is used by the cluster.
// Since CAPI 1.11 a ClusterClass can live in a different namespace than the Cluster (spec.topology.classNamespace).
// It returns an empty string if the cluster does not use a ClusterClass.
func Namespace(cluster *clusterv1.Cluster) string {
	if cluster.Spec.Topology == nil {
		return ""
	}

	if cluster.Spec.Topology.ClassNamespace != "" {
		return cluster.Spec.Topology.ClassNamespace
	}

	return cluster.Namespace
}

// ListClustersUsingClassesOf returns all clusters, in any namespace, that use a ClusterClass of the given namespace.
func ListClustersUsingClassesOf(ctx context.Context, c client.Reader, namespace string) ([]clusterv1.Cluster, error) {
	clusterList := &clusterv1.ClusterList{}
	if err := c.List(ctx, clusterList); err != nil {
		return nil, fmt.Errorf("failed to list clusters: %w", err)
	}

	clusters := make([]clusterv1.Cluster, 0, len(clusterList.Items))
	for i := range clusterList.Items {
		if clusterList.Items[i].Spec.Topology == nil {
			continue
		}

		if Namespace(&clusterList.Items[i]) == namespace {
			clusters = append(clusters, clusterList.Items[i])
		}
	}

	return clusters, nil
}
