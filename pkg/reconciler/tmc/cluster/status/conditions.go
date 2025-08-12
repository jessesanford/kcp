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

package status

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	conditionsapi "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
	conditionsutil "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/util/conditions"
)

// TMC-specific condition types for cluster status management.
const (
	// ClusterConnectionCondition indicates whether the cluster connection is healthy
	ClusterConnectionCondition conditionsapi.ConditionType = "ClusterConnection"
	
	// ClusterRegistrationCondition indicates cluster registration status
	ClusterRegistrationCondition conditionsapi.ConditionType = "ClusterRegistration"
	
	// PlacementAvailableCondition indicates if the cluster is available for placement
	PlacementAvailableCondition conditionsapi.ConditionType = "PlacementAvailable"
	
	// HeartbeatCondition indicates cluster heartbeat status
	HeartbeatCondition conditionsapi.ConditionType = "Heartbeat"
	
	// ResourcesAvailableCondition indicates cluster resource availability
	ResourcesAvailableCondition conditionsapi.ConditionType = "ResourcesAvailable"
	
	// SyncCondition indicates cluster synchronization status
	SyncCondition conditionsapi.ConditionType = "Sync"
)

// TMC-specific condition reasons.
const (
	// Connection reasons
	ClusterConnectedReason     = "ClusterConnected"
	ClusterDisconnectedReason = "ClusterDisconnected"
	ConnectionTimeoutReason    = "ConnectionTimeout"
	
	// Registration reasons  
	ClusterRegisteredReason   = "ClusterRegistered"
	ClusterUnregisteredReason = "ClusterUnregistered"
	RegistrationFailedReason  = "RegistrationFailed"
	
	// Placement reasons
	PlacementReadyReason        = "PlacementReady"
	PlacementUnavailableReason  = "PlacementUnavailable"
	InsufficientResourcesReason = "InsufficientResources"
	
	// Heartbeat reasons
	HeartbeatHealthyReason = "HeartbeatHealthy"
	HeartbeatMissedReason  = "HeartbeatMissed"
	HeartbeatStaleReason   = "HeartbeatStale"
	
	// Resource reasons
	ResourcesAdequateReason     = "ResourcesAdequate"
	ResourcesInsufficientReason = "ResourcesInsufficient"
	ResourcesUnknownReason      = "ResourcesUnknown"
	
	// Sync reasons
	SyncSuccessfulReason = "SyncSuccessful"
	SyncFailedReason     = "SyncFailed"
	SyncInProgressReason = "SyncInProgress"
)

// ConditionBuilder provides a fluent interface for creating TMC conditions.
type ConditionBuilder struct {
	condition conditionsapi.Condition
}

// NewConditionBuilder creates a new condition builder for the specified type.
func NewConditionBuilder(conditionType conditionsapi.ConditionType) *ConditionBuilder {
	return &ConditionBuilder{
		condition: conditionsapi.Condition{
			Type:               conditionType,
			LastTransitionTime: metav1.NewTime(metav1.Now().Truncate(1 * 1000000000)), // Truncate to seconds
		},
	}
}

// WithStatus sets the condition status.
func (b *ConditionBuilder) WithStatus(status corev1.ConditionStatus) *ConditionBuilder {
	b.condition.Status = status
	return b
}

// WithReason sets the condition reason.
func (b *ConditionBuilder) WithReason(reason string) *ConditionBuilder {
	b.condition.Reason = reason
	return b
}

// WithMessage sets the condition message.
func (b *ConditionBuilder) WithMessage(message string) *ConditionBuilder {
	b.condition.Message = message
	return b
}

// WithSeverity sets the condition severity.
func (b *ConditionBuilder) WithSeverity(severity conditionsapi.ConditionSeverity) *ConditionBuilder {
	b.condition.Severity = severity
	return b
}

// Build returns the constructed condition.
func (b *ConditionBuilder) Build() *conditionsapi.Condition {
	return &b.condition
}

// Predefined condition constructors for common TMC scenarios.

// ClusterConnectedCondition creates a condition indicating the cluster is connected.
func ClusterConnectedCondition(message string) *conditionsapi.Condition {
	return NewConditionBuilder(ClusterConnectionCondition).
		WithStatus(corev1.ConditionTrue).
		WithReason(ClusterConnectedReason).
		WithMessage(message).
		Build()
}

// ClusterDisconnectedCondition creates a condition indicating the cluster is disconnected.
func ClusterDisconnectedCondition(message string) *conditionsapi.Condition {
	return NewConditionBuilder(ClusterConnectionCondition).
		WithStatus(corev1.ConditionFalse).
		WithReason(ClusterDisconnectedReason).
		WithMessage(message).
		WithSeverity(conditionsapi.ConditionSeverityError).
		Build()
}

// ClusterConnectionTimeoutCondition creates a condition for connection timeout.
func ClusterConnectionTimeoutCondition(message string) *conditionsapi.Condition {
	return NewConditionBuilder(ClusterConnectionCondition).
		WithStatus(corev1.ConditionUnknown).
		WithReason(ConnectionTimeoutReason).
		WithMessage(message).
		WithSeverity(conditionsapi.ConditionSeverityWarning).
		Build()
}

// ClusterRegisteredCondition creates a condition indicating successful registration.
func ClusterRegisteredCondition(message string) *conditionsapi.Condition {
	return NewConditionBuilder(ClusterRegistrationCondition).
		WithStatus(corev1.ConditionTrue).
		WithReason(ClusterRegisteredReason).
		WithMessage(message).
		Build()
}

// ClusterRegistrationFailedCondition creates a condition indicating registration failure.
func ClusterRegistrationFailedCondition(message string) *conditionsapi.Condition {
	return NewConditionBuilder(ClusterRegistrationCondition).
		WithStatus(corev1.ConditionFalse).
		WithReason(RegistrationFailedReason).
		WithMessage(message).
		WithSeverity(conditionsapi.ConditionSeverityError).
		Build()
}

// PlacementReadyCondition creates a condition indicating the cluster is ready for placement.
func PlacementReadyCondition(message string) *conditionsapi.Condition {
	return NewConditionBuilder(PlacementAvailableCondition).
		WithStatus(corev1.ConditionTrue).
		WithReason(PlacementReadyReason).
		WithMessage(message).
		Build()
}

// PlacementUnavailableCondition creates a condition indicating placement unavailability.
func PlacementUnavailableCondition(reason, message string) *conditionsapi.Condition {
	severity := conditionsapi.ConditionSeverityWarning
	if reason == InsufficientResourcesReason {
		severity = conditionsapi.ConditionSeverityError
	}
	
	return NewConditionBuilder(PlacementAvailableCondition).
		WithStatus(corev1.ConditionFalse).
		WithReason(reason).
		WithMessage(message).
		WithSeverity(severity).
		Build()
}

// HeartbeatHealthyCondition creates a condition indicating healthy heartbeat.
func HeartbeatHealthyCondition(message string) *conditionsapi.Condition {
	return NewConditionBuilder(HeartbeatCondition).
		WithStatus(corev1.ConditionTrue).
		WithReason(HeartbeatHealthyReason).
		WithMessage(message).
		Build()
}

// HeartbeatUnhealthyCondition creates a condition for unhealthy heartbeat.
func HeartbeatUnhealthyCondition(reason, message string) *conditionsapi.Condition {
	severity := conditionsapi.ConditionSeverityWarning
	if reason == HeartbeatMissedReason {
		severity = conditionsapi.ConditionSeverityError
	}
	
	return NewConditionBuilder(HeartbeatCondition).
		WithStatus(corev1.ConditionFalse).
		WithReason(reason).
		WithMessage(message).
		WithSeverity(severity).
		Build()
}

// ResourcesAdequateCondition creates a condition indicating adequate resources.
func ResourcesAdequateCondition(message string) *conditionsapi.Condition {
	return NewConditionBuilder(ResourcesAvailableCondition).
		WithStatus(corev1.ConditionTrue).
		WithReason(ResourcesAdequateReason).
		WithMessage(message).
		Build()
}

// ResourcesInsufficientCondition creates a condition indicating insufficient resources.
func ResourcesInsufficientCondition(message string) *conditionsapi.Condition {
	return NewConditionBuilder(ResourcesAvailableCondition).
		WithStatus(corev1.ConditionFalse).
		WithReason(ResourcesInsufficientReason).
		WithMessage(message).
		WithSeverity(conditionsapi.ConditionSeverityWarning).
		Build()
}

// ResourcesUnknownCondition creates a condition when resource status is unknown.
func ResourcesUnknownCondition(message string) *conditionsapi.Condition {
	return NewConditionBuilder(ResourcesAvailableCondition).
		WithStatus(corev1.ConditionUnknown).
		WithReason(ResourcesUnknownReason).
		WithMessage(message).
		Build()
}

// SyncSuccessfulCondition creates a condition indicating successful synchronization.
func SyncSuccessfulCondition(message string) *conditionsapi.Condition {
	return NewConditionBuilder(SyncCondition).
		WithStatus(corev1.ConditionTrue).
		WithReason(SyncSuccessfulReason).
		WithMessage(message).
		Build()
}

// SyncFailedCondition creates a condition indicating synchronization failure.
func SyncFailedCondition(message string) *conditionsapi.Condition {
	return NewConditionBuilder(SyncCondition).
		WithStatus(corev1.ConditionFalse).
		WithReason(SyncFailedReason).
		WithMessage(message).
		WithSeverity(conditionsapi.ConditionSeverityError).
		Build()
}

// SyncInProgressCondition creates a condition indicating synchronization is in progress.
func SyncInProgressCondition(message string) *conditionsapi.Condition {
	return NewConditionBuilder(SyncCondition).
		WithStatus(corev1.ConditionUnknown).
		WithReason(SyncInProgressReason).
		WithMessage(message).
		Build()
}

// Condition helper functions

// IsConditionTrue checks if a condition is True.
func IsConditionTrue(obj conditionsutil.Getter, conditionType conditionsapi.ConditionType) bool {
	condition := conditionsutil.Get(obj, conditionType)
	return condition != nil && condition.Status == corev1.ConditionTrue
}

// IsConditionFalse checks if a condition is False.
func IsConditionFalse(obj conditionsutil.Getter, conditionType conditionsapi.ConditionType) bool {
	condition := conditionsutil.Get(obj, conditionType)
	return condition != nil && condition.Status == corev1.ConditionFalse
}

// IsConditionUnknown checks if a condition is Unknown or doesn't exist.
func IsConditionUnknown(obj conditionsutil.Getter, conditionType conditionsapi.ConditionType) bool {
	condition := conditionsutil.Get(obj, conditionType)
	return condition == nil || condition.Status == corev1.ConditionUnknown
}

// GetConditionReason returns the reason for a condition, or empty string if not found.
func GetConditionReason(obj conditionsutil.Getter, conditionType conditionsapi.ConditionType) string {
	condition := conditionsutil.Get(obj, conditionType)
	if condition != nil {
		return condition.Reason
	}
	return ""
}

// HasCriticalConditionError checks if there are any conditions with Error severity and False status.
func HasCriticalConditionError(conditions []conditionsapi.Condition) bool {
	for _, condition := range conditions {
		if condition.Status == corev1.ConditionFalse && condition.Severity == conditionsapi.ConditionSeverityError {
			return true
		}
	}
	return false
}

// GetCriticalConditions returns conditions that are False with Error severity.
func GetCriticalConditions(conditions []conditionsapi.Condition) []conditionsapi.Condition {
	var critical []conditionsapi.Condition
	for _, condition := range conditions {
		if condition.Status == corev1.ConditionFalse && condition.Severity == conditionsapi.ConditionSeverityError {
			critical = append(critical, condition)
		}
	}
	return critical
}

// GetWarningConditions returns conditions that are False with Warning severity.
func GetWarningConditions(conditions []conditionsapi.Condition) []conditionsapi.Condition {
	var warnings []conditionsapi.Condition
	for _, condition := range conditions {
		if condition.Status == corev1.ConditionFalse && condition.Severity == conditionsapi.ConditionSeverityWarning {
			warnings = append(warnings, condition)
		}
	}
	return warnings
}