/*
Copyright 2024 The KCP Authors.

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

package synctarget

import (
	"fmt"
	"sort"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	workloadv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/workload/v1alpha1"
)

// setCondition adds or updates a condition in the conditions slice.
// If a condition with the same type already exists, it will be updated.
// Otherwise, a new condition will be added to the slice.
func setCondition(conditions *[]metav1.Condition, newCondition metav1.Condition) {
	if conditions == nil {
		*conditions = []metav1.Condition{}
	}

	for i, condition := range *conditions {
		if condition.Type == newCondition.Type {
			// Update existing condition
			(*conditions)[i] = newCondition
			return
		}
	}

	// Add new condition
	*conditions = append(*conditions, newCondition)
}

// getCondition retrieves a condition by type from the conditions slice.
// Returns the condition and true if found, nil and false otherwise.
func getCondition(conditions []metav1.Condition, conditionType string) (*metav1.Condition, bool) {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			return &condition, true
		}
	}
	return nil, false
}

// isConditionTrue checks if a condition with the given type exists and has status True.
func isConditionTrue(conditions []metav1.Condition, conditionType string) bool {
	condition, found := getCondition(conditions, conditionType)
	return found && condition.Status == metav1.ConditionTrue
}

// isConditionFalse checks if a condition with the given type exists and has status False.
func isConditionFalse(conditions []metav1.Condition, conditionType string) bool {
	condition, found := getCondition(conditions, conditionType)
	return found && condition.Status == metav1.ConditionFalse
}

// calculateResourceUtilization calculates utilization percentage.
func calculateResourceUtilization(allocated, available resource.Quantity) (float64, error) {
	if available.IsZero() {
		return 0, fmt.Errorf("available resource is zero")
	}
	total := allocated.DeepCopy()
	total.Add(available)
	if total.IsZero() {
		return 0, nil
	}
	return float64(allocated.MilliValue()) / float64(total.MilliValue()) * 100, nil
}

// getSyncTargetHealthScore calculates overall health score (0-100).
func getSyncTargetHealthScore(syncTarget *workloadv1alpha1.SyncTarget) int {
	if syncTarget == nil {
		return 0
	}
	score := 100
	for _, condition := range syncTarget.Status.Conditions {
		if condition.Status != metav1.ConditionTrue {
			if condition.Type == "Ready" || condition.Type == "HeartbeatHealthy" {
				score -= 30
			} else {
				score -= 15
			}
		}
	}
	if score < 0 {
		score = 0
	}
	return score
}