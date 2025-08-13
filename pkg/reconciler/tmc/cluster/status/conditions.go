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

// Condition types for TMC cluster management
const (
	// ClusterConnectionCondition indicates cluster connectivity
	ClusterConnectionCondition conditionsapi.ConditionType = "ClusterConnection"
	
	// ClusterRegistrationCondition indicates cluster registration status
	ClusterRegistrationCondition conditionsapi.ConditionType = "ClusterRegistration"
	
	// HeartbeatCondition indicates cluster heartbeat health
	HeartbeatCondition conditionsapi.ConditionType = "Heartbeat"
	
	// PlacementAvailableCondition indicates placement availability
	PlacementAvailableCondition conditionsapi.ConditionType = "PlacementAvailable"
	
	// SyncCondition indicates sync status
	SyncCondition conditionsapi.ConditionType = "Sync"
)

// Condition reasons for cluster connection
const (
	ClusterConnectedReason    = "Connected"
	ClusterDisconnectedReason = "Disconnected"
	ClusterTimeoutReason      = "ConnectionTimeout"
)

// Condition reasons for cluster registration
const (
	ClusterRegisteredReason   = "Registered"
	ClusterUnregisteredReason = "Unregistered"
)

// Condition reasons for heartbeat
const (
	HeartbeatHealthyReason   = "Healthy"
	HeartbeatMissedReason    = "Missed"
	HeartbeatStaleReason     = "Stale"
)

// Condition reasons for placement
const (
	PlacementAvailableReason     = "Available"
	PlacementUnavailableReason   = "Unavailable"
	InsufficientResourcesReason  = "InsufficientResources"
)

// Condition reasons for sync
const (
	SyncSuccessReason = "Success"
	SyncFailedReason  = "Failed"
)

// ConditionBuilder helps build conditions with fluent interface
type ConditionBuilder struct {
	condition conditionsapi.Condition
}

// NewConditionBuilder creates a new condition builder
func NewConditionBuilder(conditionType conditionsapi.ConditionType) *ConditionBuilder {
	return &ConditionBuilder{
		condition: conditionsapi.Condition{
			Type:               conditionType,
			LastTransitionTime: metav1.Now(),
		},
	}
}

// WithStatus sets the condition status
func (b *ConditionBuilder) WithStatus(status corev1.ConditionStatus) *ConditionBuilder {
	b.condition.Status = status
	return b
}

// WithReason sets the condition reason
func (b *ConditionBuilder) WithReason(reason string) *ConditionBuilder {
	b.condition.Reason = reason
	return b
}

// WithMessage sets the condition message
func (b *ConditionBuilder) WithMessage(message string) *ConditionBuilder {
	b.condition.Message = message
	return b
}

// WithSeverity sets the condition severity
func (b *ConditionBuilder) WithSeverity(severity conditionsapi.ConditionSeverity) *ConditionBuilder {
	b.condition.Severity = severity
	return b
}

// Build returns the built condition
func (b *ConditionBuilder) Build() conditionsapi.Condition {
	return b.condition
}

// Prebuilt condition functions for common scenarios

// ClusterConnectedCondition creates a condition indicating cluster is connected
func ClusterConnectedCondition(message string) *conditionsapi.Condition {
	return &conditionsapi.Condition{
		Type:               ClusterConnectionCondition,
		Status:             corev1.ConditionTrue,
		Reason:             ClusterConnectedReason,
		Message:            message,
		Severity:           conditionsapi.ConditionSeverityInfo,
		LastTransitionTime: metav1.Now(),
	}
}

// ClusterDisconnectedCondition creates a condition indicating cluster is disconnected
func ClusterDisconnectedCondition(message string) *conditionsapi.Condition {
	return &conditionsapi.Condition{
		Type:               ClusterConnectionCondition,
		Status:             corev1.ConditionFalse,
		Reason:             ClusterDisconnectedReason,
		Message:            message,
		Severity:           conditionsapi.ConditionSeverityError,
		LastTransitionTime: metav1.Now(),
	}
}

// ClusterConnectionTimeoutCondition creates a condition indicating connection timeout
func ClusterConnectionTimeoutCondition(message string) *conditionsapi.Condition {
	return &conditionsapi.Condition{
		Type:               ClusterConnectionCondition,
		Status:             corev1.ConditionUnknown,
		Reason:             ClusterTimeoutReason,
		Message:            message,
		Severity:           conditionsapi.ConditionSeverityWarning,
		LastTransitionTime: metav1.Now(),
	}
}

// ClusterRegisteredCondition creates a condition indicating cluster is registered
func ClusterRegisteredCondition(message string) *conditionsapi.Condition {
	return &conditionsapi.Condition{
		Type:               ClusterRegistrationCondition,
		Status:             corev1.ConditionTrue,
		Reason:             ClusterRegisteredReason,
		Message:            message,
		Severity:           conditionsapi.ConditionSeverityInfo,
		LastTransitionTime: metav1.Now(),
	}
}

// HeartbeatHealthyCondition creates a condition indicating healthy heartbeat
func HeartbeatHealthyCondition(message string) *conditionsapi.Condition {
	return &conditionsapi.Condition{
		Type:               HeartbeatCondition,
		Status:             corev1.ConditionTrue,
		Reason:             HeartbeatHealthyReason,
		Message:            message,
		Severity:           conditionsapi.ConditionSeverityInfo,
		LastTransitionTime: metav1.Now(),
	}
}

// HeartbeatUnhealthyCondition creates a condition indicating unhealthy heartbeat
func HeartbeatUnhealthyCondition(reason, message string) *conditionsapi.Condition {
	severity := conditionsapi.ConditionSeverityError
	if reason == HeartbeatStaleReason {
		severity = conditionsapi.ConditionSeverityWarning
	}
	
	return &conditionsapi.Condition{
		Type:               HeartbeatCondition,
		Status:             corev1.ConditionFalse,
		Reason:             reason,
		Message:            message,
		Severity:           severity,
		LastTransitionTime: metav1.Now(),
	}
}

// PlacementUnavailableCondition creates a condition indicating placement unavailability
func PlacementUnavailableCondition(reason, message string) *conditionsapi.Condition {
	severity := conditionsapi.ConditionSeverityWarning
	if reason == InsufficientResourcesReason {
		severity = conditionsapi.ConditionSeverityError
	}
	
	return &conditionsapi.Condition{
		Type:               PlacementAvailableCondition,
		Status:             corev1.ConditionFalse,
		Reason:             reason,
		Message:            message,
		Severity:           severity,
		LastTransitionTime: metav1.Now(),
	}
}

// ResourcesInsufficientCondition creates a condition indicating insufficient resources
func ResourcesInsufficientCondition(message string) *conditionsapi.Condition {
	return &conditionsapi.Condition{
		Type:               PlacementAvailableCondition,
		Status:             corev1.ConditionFalse,
		Reason:             InsufficientResourcesReason,
		Message:            message,
		Severity:           conditionsapi.ConditionSeverityWarning,
		LastTransitionTime: metav1.Now(),
	}
}

// SyncFailedCondition creates a condition indicating sync failure
func SyncFailedCondition(message string) *conditionsapi.Condition {
	return &conditionsapi.Condition{
		Type:               SyncCondition,
		Status:             corev1.ConditionFalse,
		Reason:             SyncFailedReason,
		Message:            message,
		Severity:           conditionsapi.ConditionSeverityError,
		LastTransitionTime: metav1.Now(),
	}
}

// Helper functions for condition checking

// IsConditionTrue returns true if the specified condition is True
func IsConditionTrue(obj conditionsutil.Getter, conditionType conditionsapi.ConditionType) bool {
	condition := conditionsutil.Get(obj, conditionType)
	return condition != nil && condition.Status == corev1.ConditionTrue
}

// IsConditionFalse returns true if the specified condition is False
func IsConditionFalse(obj conditionsutil.Getter, conditionType conditionsapi.ConditionType) bool {
	condition := conditionsutil.Get(obj, conditionType)
	return condition != nil && condition.Status == corev1.ConditionFalse
}

// IsConditionUnknown returns true if the specified condition is Unknown
func IsConditionUnknown(obj conditionsutil.Getter, conditionType conditionsapi.ConditionType) bool {
	condition := conditionsutil.Get(obj, conditionType)
	return condition != nil && condition.Status == corev1.ConditionUnknown
}

// GetConditionReason returns the reason of the specified condition
func GetConditionReason(obj conditionsutil.Getter, conditionType conditionsapi.ConditionType) string {
	condition := conditionsutil.Get(obj, conditionType)
	if condition == nil {
		return ""
	}
	return condition.Reason
}

// HasCriticalConditionError returns true if any condition has critical (Error) severity
func HasCriticalConditionError(conditions conditionsapi.Conditions) bool {
	for _, condition := range conditions {
		if condition.Status == corev1.ConditionFalse && condition.Severity == conditionsapi.ConditionSeverityError {
			return true
		}
	}
	return false
}

// GetCriticalConditions returns all conditions with Error severity and False status
func GetCriticalConditions(conditions conditionsapi.Conditions) []conditionsapi.Condition {
	var critical []conditionsapi.Condition
	for _, condition := range conditions {
		if condition.Status == corev1.ConditionFalse && condition.Severity == conditionsapi.ConditionSeverityError {
			critical = append(critical, condition)
		}
	}
	return critical
}

// GetWarningConditions returns all conditions with Warning severity and False status
func GetWarningConditions(conditions conditionsapi.Conditions) []conditionsapi.Condition {
	var warnings []conditionsapi.Condition
	for _, condition := range conditions {
		if condition.Status == corev1.ConditionFalse && condition.Severity == conditionsapi.ConditionSeverityWarning {
			warnings = append(warnings, condition)
		}
	}
	return warnings
}