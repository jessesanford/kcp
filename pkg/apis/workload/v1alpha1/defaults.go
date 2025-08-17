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

package v1alpha1

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SetDefaults_SyncTarget sets default values for SyncTarget objects.
func SetDefaults_SyncTarget(obj *SyncTarget) {
	// Set default eviction timeout if not specified
	if obj.Spec.EvictAfter == nil {
		// Default to 5 minutes to allow for reasonable recovery time
		// but not too long to avoid extended unavailability
		defaultEvictAfter := metav1.Duration{Duration: 5 * time.Minute}
		obj.Spec.EvictAfter = &defaultEvictAfter
	}

	// Ensure unschedulable is explicitly set to false if not provided
	// This makes the default behavior clear
	if obj.Spec.Unschedulable {
		// If unschedulable is true, we don't override it
		// This preserves explicit user intent
	} else {
		// Explicitly set to false to make default clear
		obj.Spec.Unschedulable = false
	}

	// Set default labels for cells if they don't have any
	for i := range obj.Spec.Cells {
		cell := &obj.Spec.Cells[i]
		if cell.Labels == nil {
			cell.Labels = make(map[string]string)
		}

		// Add default location label if not present
		if _, exists := cell.Labels["location"]; !exists {
			cell.Labels["location"] = cell.Name
		}
	}
}
