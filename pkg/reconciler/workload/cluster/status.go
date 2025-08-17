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

package cluster

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// Condition types for ClusterRegistration status
	
	// ConditionRegistered indicates whether the cluster is registered in the TMC system.
	ConditionRegistered = "Registered"
	
	// ConditionReady indicates whether the cluster is ready for workload placement.
	ConditionReady = "Ready"
	
	// ConditionHealthy indicates whether the cluster is healthy and operational.
	ConditionHealthy = "Healthy"
	
	// ConditionSyncTargetReady indicates whether the associated SyncTarget is ready.
	ConditionSyncTargetReady = "SyncTargetReady"
	
	// ConditionValidated indicates whether the cluster registration has been validated.
	ConditionValidated = "Validated"
)

const (
	// Condition reasons for ClusterRegistration status
	
	// ReasonRegistrationComplete indicates successful cluster registration.
	ReasonRegistrationComplete = "RegistrationComplete"
	
	// ReasonValidationFailed indicates cluster registration validation failed.
	ReasonValidationFailed = "ValidationFailed"
	
	// ReasonSyncTargetCreated indicates SyncTarget was successfully created.
	ReasonSyncTargetCreated = "SyncTargetCreated"
	
	// ReasonSyncTargetFailed indicates SyncTarget creation failed.
	ReasonSyncTargetFailed = "SyncTargetFailed"
	
	// ReasonHealthCheckPassed indicates cluster health check passed.
	ReasonHealthCheckPassed = "HealthCheckPassed"
	
	// ReasonHealthCheckFailed indicates cluster health check failed.
	ReasonHealthCheckFailed = "HealthCheckFailed"
	
	// ReasonClusterReady indicates cluster is ready for workloads.
	ReasonClusterReady = "ClusterReady"
	
	// ReasonClusterNotReady indicates cluster is not ready for workloads.
	ReasonClusterNotReady = "ClusterNotReady"
)

// StatusManager provides utilities for managing ClusterRegistration status.
type StatusManager struct{}

// NewStatusManager creates a new status manager for cluster registration.
func NewStatusManager() *StatusManager {
	return &StatusManager{}
}

// InitializeStatus initializes the status of a new ClusterRegistration.
func (sm *StatusManager) InitializeStatus(cluster *ClusterRegistration) {
	// Set initial phase
	cluster.Status.Phase = ClusterRegistrationPhasePending
	
	// Initialize conditions slice if nil
	if cluster.Status.Conditions == nil {
		cluster.Status.Conditions = []metav1.Condition{}
	}
	
	// Set initial condition
	sm.SetCondition(cluster, metav1.Condition{
		Type:    ConditionValidated,
		Status:  metav1.ConditionUnknown,
		Reason:  "ValidationInProgress",
		Message: "Cluster registration validation in progress",
	})
}

// SetCondition sets or updates a condition in the cluster status.
func (sm *StatusManager) SetCondition(cluster *ClusterRegistration, condition metav1.Condition) {
	cluster.Status.Conditions = setCondition(cluster.Status.Conditions, condition)
}

// GetCondition retrieves a condition from the cluster status by type.
func (sm *StatusManager) GetCondition(cluster *ClusterRegistration, conditionType string) *metav1.Condition {
	return getCondition(cluster.Status.Conditions, conditionType)
}

// IsConditionTrue checks if a condition is true.
func (sm *StatusManager) IsConditionTrue(cluster *ClusterRegistration, conditionType string) bool {
	condition := sm.GetCondition(cluster, conditionType)
	return condition != nil && condition.Status == metav1.ConditionTrue
}

// IsConditionFalse checks if a condition is false.
func (sm *StatusManager) IsConditionFalse(cluster *ClusterRegistration, conditionType string) bool {
	condition := sm.GetCondition(cluster, conditionType)
	return condition != nil && condition.Status == metav1.ConditionFalse
}

// UpdatePhase updates the cluster registration phase and related conditions.
func (sm *StatusManager) UpdatePhase(cluster *ClusterRegistration, phase ClusterRegistrationPhase, reason, message string) {
	cluster.Status.Phase = phase
	
	switch phase {
	case ClusterRegistrationPhasePending:
		sm.SetCondition(cluster, metav1.Condition{
			Type:    ConditionRegistered,
			Status:  metav1.ConditionFalse,
			Reason:  reason,
			Message: message,
		})
		
	case ClusterRegistrationPhaseRegistered:
		sm.SetCondition(cluster, metav1.Condition{
			Type:    ConditionRegistered,
			Status:  metav1.ConditionTrue,
			Reason:  reason,
			Message: message,
		})
		sm.SetCondition(cluster, metav1.Condition{
			Type:    ConditionReady,
			Status:  metav1.ConditionFalse,
			Reason:  "PreparingCluster",
			Message: "Cluster is being prepared for workload placement",
		})
		
	case ClusterRegistrationPhaseReady:
		sm.SetCondition(cluster, metav1.Condition{
			Type:    ConditionReady,
			Status:  metav1.ConditionTrue,
			Reason:  reason,
			Message: message,
		})
		
	case ClusterRegistrationPhaseFailed:
		sm.SetCondition(cluster, metav1.Condition{
			Type:    ConditionRegistered,
			Status:  metav1.ConditionFalse,
			Reason:  reason,
			Message: message,
		})
		sm.SetCondition(cluster, metav1.Condition{
			Type:    ConditionReady,
			Status:  metav1.ConditionFalse,
			Reason:  "RegistrationFailed",
			Message: "Cluster registration failed",
		})
	}
}

// UpdateSyncTargetCondition updates the SyncTarget related condition.
func (sm *StatusManager) UpdateSyncTargetCondition(cluster *ClusterRegistration, ready bool, reason, message string) {
	status := metav1.ConditionFalse
	if ready {
		status = metav1.ConditionTrue
	}
	
	sm.SetCondition(cluster, metav1.Condition{
		Type:    ConditionSyncTargetReady,
		Status:  status,
		Reason:  reason,
		Message: message,
	})
	
	// Update sync target reference if ready and not already set
	if ready && cluster.Status.SyncTargetRef == nil {
		cluster.Status.SyncTargetRef = &ClusterReference{
			Name:    "synctarget-" + cluster.Name,
			Cluster: cluster.Namespace, // Use namespace as cluster reference
		}
	}
}

// UpdateHealthCondition updates the health condition based on health status.
func (sm *StatusManager) UpdateHealthCondition(cluster *ClusterRegistration, health HealthStatus) {
	var status metav1.ConditionStatus
	var reason, message string
	
	switch health.Overall {
	case HealthStatusHealthy:
		status = metav1.ConditionTrue
		reason = ReasonHealthCheckPassed
		message = "Cluster health check passed"
		
	case HealthStatusDegraded:
		status = metav1.ConditionTrue
		reason = "HealthCheckDegraded"
		message = "Cluster is operational but degraded"
		
	case HealthStatusUnhealthy:
		status = metav1.ConditionFalse
		reason = ReasonHealthCheckFailed
		message = "Cluster health check failed"
	}
	
	sm.SetCondition(cluster, metav1.Condition{
		Type:    ConditionHealthy,
		Status:  status,
		Reason:  reason,
		Message: message,
	})
}

// IsClusterReady checks if the cluster is ready for workload placement.
func (sm *StatusManager) IsClusterReady(cluster *ClusterRegistration) bool {
	return cluster.Status.Phase == ClusterRegistrationPhaseReady &&
		sm.IsConditionTrue(cluster, ConditionRegistered) &&
		sm.IsConditionTrue(cluster, ConditionReady) &&
		sm.IsConditionTrue(cluster, ConditionSyncTargetReady)
}

// IsClusterHealthy checks if the cluster is healthy.
func (sm *StatusManager) IsClusterHealthy(cluster *ClusterRegistration) bool {
	return sm.IsConditionTrue(cluster, ConditionHealthy)
}

// GetReadyConditionsCount returns the number of ready conditions.
func (sm *StatusManager) GetReadyConditionsCount(cluster *ClusterRegistration) (ready, total int) {
	requiredConditions := []string{
		ConditionValidated,
		ConditionRegistered,
		ConditionSyncTargetReady,
		ConditionReady,
	}
	
	total = len(requiredConditions)
	ready = 0
	
	for _, conditionType := range requiredConditions {
		if sm.IsConditionTrue(cluster, conditionType) {
			ready++
		}
	}
	
	return ready, total
}

// Helper functions for condition management

// setCondition sets or updates a condition in the conditions slice.
func setCondition(conditions []metav1.Condition, newCondition metav1.Condition) []metav1.Condition {
	if newCondition.LastTransitionTime.IsZero() {
		newCondition.LastTransitionTime = metav1.NewTime(time.Now())
	}
	
	for i, existingCondition := range conditions {
		if existingCondition.Type == newCondition.Type {
			// Update existing condition
			if existingCondition.Status != newCondition.Status {
				newCondition.LastTransitionTime = metav1.NewTime(time.Now())
			} else {
				// Keep the original transition time if status hasn't changed
				newCondition.LastTransitionTime = existingCondition.LastTransitionTime
			}
			conditions[i] = newCondition
			return conditions
		}
	}
	
	// Add new condition
	return append(conditions, newCondition)
}

// getCondition retrieves a condition by type from the conditions slice.
func getCondition(conditions []metav1.Condition, conditionType string) *metav1.Condition {
	for i, condition := range conditions {
		if condition.Type == conditionType {
			return &conditions[i]
		}
	}
	return nil
}

// removeCondition removes a condition from the conditions slice.
func removeCondition(conditions []metav1.Condition, conditionType string) []metav1.Condition {
	for i, condition := range conditions {
		if condition.Type == conditionType {
			return append(conditions[:i], conditions[i+1:]...)
		}
	}
	return conditions
}

// hasCondition checks if a condition exists in the conditions slice.
func hasCondition(conditions []metav1.Condition, conditionType string) bool {
	return getCondition(conditions, conditionType) != nil
}

// isConditionStatusTrue checks if a condition is true.
func isConditionStatusTrue(conditions []metav1.Condition, conditionType string) bool {
	condition := getCondition(conditions, conditionType)
	return condition != nil && condition.Status == metav1.ConditionTrue
}

// getConditionMessage gets the message from a condition.
func getConditionMessage(conditions []metav1.Condition, conditionType string) string {
	condition := getCondition(conditions, conditionType)
	if condition != nil {
		return condition.Message
	}
	return ""
}

// getConditionReason gets the reason from a condition.
func getConditionReason(conditions []metav1.Condition, conditionType string) string {
	condition := getCondition(conditions, conditionType)
	if condition != nil {
		return condition.Reason
	}
	return ""
}