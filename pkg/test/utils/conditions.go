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

//revive:disable:var-naming
package utils

//revive:enable:var-naming
import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	capiConditions "sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// IsPresentAndFalseWithReason returns if condition is present in the object status with false and a reason or not.
func IsPresentAndFalseWithReason(ctx context.Context, c client.Client, key types.NamespacedName, obj client.Object, condition, reason string) bool {
	if err := c.Get(ctx, key, obj); err != nil {
		return false
	}
	getter, ok := obj.(capiConditions.Getter)
	if !ok {
		return false
	}
	if !capiConditions.Has(getter, condition) {
		return false
	}
	objectCondition := capiConditions.Get(getter, condition)
	return objectCondition.Status == metav1.ConditionFalse &&
		objectCondition.Reason == reason
}

// IsPresentAndTrue returns if condition is present in the object status with true or not.
func IsPresentAndTrue(ctx context.Context, c client.Client, key types.NamespacedName, obj client.Object, condition string) bool {
	if err := c.Get(ctx, key, obj); err != nil {
		return false
	}
	getter, ok := obj.(capiConditions.Getter)
	if !ok {
		return false
	}
	if !capiConditions.Has(getter, condition) {
		return false
	}
	objectCondition := capiConditions.Get(getter, condition)
	return objectCondition.Status == metav1.ConditionTrue
}