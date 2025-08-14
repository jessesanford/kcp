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
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

// statusUpdater implements the StatusUpdater interface
type statusUpdater struct {
	// Add fields as needed for status updates
}

// NewStatusUpdater creates a new StatusUpdater
func NewStatusUpdater() StatusUpdater {
	return &statusUpdater{}
}

// UpdateStatus updates the SyncTarget status based on deployment state
func (s *statusUpdater) UpdateStatus(ctx context.Context, target *SyncTarget, status *DeploymentStatus) error {
	klog.V(4).Infof("Updating status for SyncTarget %s", target.Name)

	// Update replicas status
	target.Status.Replicas = status.Replicas
	target.Status.ReadyReplicas = status.ReadyReplicas
	target.Status.AvailableReplicas = status.AvailableReplicas

	// Set condition based on deployment status
	setCondition(&target.Status.Conditions, status.Condition)

	return nil
}

// setCondition adds or updates a condition in the slice
func setCondition(conditions *[]metav1.Condition, newCondition metav1.Condition) {
	for i, condition := range *conditions {
		if condition.Type == newCondition.Type {
			(*conditions)[i] = newCondition
			return
		}
	}
	*conditions = append(*conditions, newCondition)
}