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

package controller

import (
	"context"
	"fmt"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func getUsedClusterClasses(ctx context.Context, c client.Client, namespace string) ([]string, error) {
	clusterList := &clusterv1.ClusterList{}
	if err := c.List(ctx, clusterList, &client.ListOptions{Namespace: namespace}); err != nil {
		return nil, fmt.Errorf("failed to list clusters: %w", err)
	}

	usedClusterClasses := make([]string, 0, len(clusterList.Items))

	// list the names of all ClusterClasses that are referenced in Cluster objects
	for _, cluster := range clusterList.Items {
		if cluster.Spec.Topology == nil {
			continue
		}
		if cluster.Spec.Topology.Class != "" {
			usedClusterClasses = append(usedClusterClasses, cluster.Spec.Topology.Class)
		}
	}

	return usedClusterClasses, nil
}
