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

package utils

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// IsPresentAndFalseWithReason returns if condition is present in the object status with false and a reason or not.
func IsPresentAndFalseWithReason(ctx context.Context, c client.Client, key types.NamespacedName, getter conditions.Getter, condition clusterv1.ConditionType, reason string) bool {
	if err := c.Get(ctx, key, getter); err != nil {
		return false
	}
	if !conditions.Has(getter, condition) {
		return false
	}
	objectCondition := conditions.Get(getter, condition)
	return objectCondition.Status == corev1.ConditionFalse &&
		objectCondition.Reason == reason
}

// IsPresentAndTrue returns if condition is present in the object status with true or not.
func IsPresentAndTrue(ctx context.Context, c client.Client, key types.NamespacedName, getter conditions.Getter, condition clusterv1.ConditionType) bool {
	if err := c.Get(ctx, key, getter); err != nil {
		return false
	}
	if !conditions.Has(getter, condition) {
		return false
	}
	objectCondition := conditions.Get(getter, condition)
	return objectCondition.Status == corev1.ConditionTrue
}
