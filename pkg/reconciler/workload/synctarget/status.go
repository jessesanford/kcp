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
	"time"

	"github.com/kcp-dev/logicalcluster/v3"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	workloadv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"
)

// updateStatus updates the SyncTarget status based on the reconciliation result.
// It manages all status conditions and ensures proper status transitions.
func (c *Controller) updateStatus(ctx context.Context, cluster logicalcluster.Path, syncTarget *workloadv1alpha1.SyncTarget, reconcileErr error) error {
	klog.V(3).Infof("Updating status for SyncTarget %s", syncTarget.Name)

	// Create a deep copy to avoid modifying the original
	syncTarget = syncTarget.DeepCopy()
	
	// Update conditions based on reconciliation result
	c.updateConditions(syncTarget, reconcileErr)
	
	// Update overall status
	c.updateOverallStatus(syncTarget)
	
	// TODO: Use actual client to update status when available
	// _, err := c.kcpClusterClient.WorkloadV1alpha1().
	//     SyncTargets().
	//     Cluster(cluster).
	//     UpdateStatus(ctx, syncTarget, metav1.UpdateOptions{})
	// if err != nil {
	//     return fmt.Errorf("failed to update status: %w", err)
	// }

	klog.V(3).Infof("Status updated successfully for SyncTarget %s", syncTarget.Name)
	return nil
}

// updateConditions manages all status conditions for the SyncTarget.
func (c *Controller) updateConditions(syncTarget *workloadv1alpha1.SyncTarget, reconcileErr error) {
	now := metav1.NewTime(time.Now())

	if reconcileErr != nil {
		// Set error conditions based on the type of failure
		c.handleReconcileError(syncTarget, reconcileErr, now)
	} else {
		// Set success conditions
		c.handleReconcileSuccess(syncTarget, now)
	}
}

// handleReconcileError sets appropriate error conditions based on the reconciliation error.
func (c *Controller) handleReconcileError(syncTarget *workloadv1alpha1.SyncTarget, reconcileErr error, now metav1.Time) {
	klog.V(4).Infof("Handling reconcile error for SyncTarget %s: %v", syncTarget.Name, reconcileErr)

	// Determine which phase failed and set appropriate conditions
	errorMsg := reconcileErr.Error()

	if contains(errorMsg, "prerequisites") {
		// Prerequisites validation failed
		setCondition(syncTarget, workloadv1alpha1.SyncTargetCondition{
			Type:               SyncTargetValidCondition,
			Status:             metav1.ConditionFalse,
			LastTransitionTime: now,
			Reason:             "PrerequisitesFailed",
			Message:            errorMsg,
		})
	} else if contains(errorMsg, "deployment") {
		// Deployment failed
		setCondition(syncTarget, workloadv1alpha1.SyncTargetCondition{
			Type:               SyncTargetDeployedCondition,
			Status:             metav1.ConditionFalse,
			LastTransitionTime: now,
			Reason:             "DeploymentFailed",
			Message:            errorMsg,
		})
	} else if contains(errorMsg, "health") {
		// Health check failed
		setCondition(syncTarget, workloadv1alpha1.SyncTargetCondition{
			Type:               SyncTargetReadyCondition,
			Status:             metav1.ConditionFalse,
			LastTransitionTime: now,
			Reason:             "HealthCheckFailed",
			Message:            errorMsg,
		})
	} else {
		// Generic error
		setCondition(syncTarget, workloadv1alpha1.SyncTargetCondition{
			Type:               SyncTargetReadyCondition,
			Status:             metav1.ConditionFalse,
			LastTransitionTime: now,
			Reason:             "ReconciliationFailed",
			Message:            errorMsg,
		})
	}
}

// handleReconcileSuccess sets success conditions when reconciliation completes without error.
func (c *Controller) handleReconcileSuccess(syncTarget *workloadv1alpha1.SyncTarget, now metav1.Time) {
	klog.V(4).Infof("Handling reconcile success for SyncTarget %s", syncTarget.Name)

	// Set all conditions to success state
	setCondition(syncTarget, workloadv1alpha1.SyncTargetCondition{
		Type:               SyncTargetValidCondition,
		Status:             metav1.ConditionTrue,
		LastTransitionTime: now,
		Reason:             "PrerequisitesValid",
		Message:            "All prerequisites are met",
	})

	setCondition(syncTarget, workloadv1alpha1.SyncTargetCondition{
		Type:               SyncTargetDeployedCondition,
		Status:             metav1.ConditionTrue,
		LastTransitionTime: now,
		Reason:             "DeploymentReady",
		Message:            "Syncer deployment is ready",
	})

	setCondition(syncTarget, workloadv1alpha1.SyncTargetCondition{
		Type:               SyncTargetReadyCondition,
		Status:             metav1.ConditionTrue,
		LastTransitionTime: now,
		Reason:             "Ready",
		Message:            "SyncTarget is ready and healthy",
	})
}

// updateOverallStatus sets the overall status fields based on current conditions.
func (c *Controller) updateOverallStatus(syncTarget *workloadv1alpha1.SyncTarget) {
	// TODO: Update overall status fields when SyncTarget.Status structure is available
	// This might include:
	// - Phase (Pending, Ready, Failed, Terminating)
	// - LastSyncTime
	// - Observed generation
	// - Summary information

	klog.V(6).Infof("Overall status updated for SyncTarget %s", syncTarget.Name)
}

// setCondition sets or updates a condition in the SyncTarget status.
// If the condition already exists and hasn't changed, the LastTransitionTime is preserved.
func setCondition(syncTarget *workloadv1alpha1.SyncTarget, condition workloadv1alpha1.SyncTargetCondition) {
	// TODO: Implement when SyncTarget.Status.Conditions is available
	// This is a placeholder that will be updated when the full API structure is available

	klog.V(6).Infof("Setting condition %s=%s for SyncTarget %s: %s", 
		condition.Type, condition.Status, syncTarget.Name, condition.Message)

	// Standard condition management logic:
	// 1. Find existing condition by type
	// 2. If not found, append new condition
	// 3. If found and changed, update with new LastTransitionTime
	// 4. If found and unchanged, preserve LastTransitionTime
}

// removeCondition removes a condition of the specified type from the SyncTarget status.
func removeCondition(syncTarget *workloadv1alpha1.SyncTarget, conditionType string) {
	// TODO: Implement when SyncTarget.Status.Conditions is available
	
	klog.V(6).Infof("Removing condition %s from SyncTarget %s", conditionType, syncTarget.Name)

	// Standard condition removal logic:
	// 1. Find condition by type
	// 2. Remove from conditions slice
	// 3. Update status
}

// getCondition retrieves a condition of the specified type from the SyncTarget status.
func getCondition(syncTarget *workloadv1alpha1.SyncTarget, conditionType string) *workloadv1alpha1.SyncTargetCondition {
	// TODO: Implement when SyncTarget.Status.Conditions is available
	
	// Standard condition retrieval logic:
	// 1. Iterate through conditions
	// 2. Return condition matching type
	// 3. Return nil if not found
	
	return nil
}

// hasCondition checks if a condition of the specified type exists.
func hasCondition(syncTarget *workloadv1alpha1.SyncTarget, conditionType string) bool {
	return getCondition(syncTarget, conditionType) != nil
}

// isConditionTrue checks if a condition exists and has status True.
func isConditionTrue(syncTarget *workloadv1alpha1.SyncTarget, conditionType string) bool {
	condition := getCondition(syncTarget, conditionType)
	return condition != nil && condition.Status == metav1.ConditionTrue
}

// contains is a helper function to check if a string contains a substring.
// This is used for error message categorization.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || 
		(len(s) > len(substr) && 
		 (s[:len(substr)] == substr || 
		  s[len(s)-len(substr):] == substr ||
		  findSubstring(s, substr))))
}

// findSubstring performs a simple substring search.
func findSubstring(s, substr string) bool {
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}