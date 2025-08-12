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
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"

	conditionsapi "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
	conditionsutil "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/util/conditions"

	tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
)

// StatusManagerInterface defines the interface for managing TMC resource status updates.
// This interface provides a standardized way to handle status conditions, events,
// and health monitoring across TMC resources.
type StatusManagerInterface interface {
	// UpdateStatus updates the status conditions on a resource and records appropriate events
	UpdateStatus(ctx context.Context, obj StatusUpdater, conditions []conditionsapi.Condition) error
	
	// ComputeReadyCondition computes the overall Ready condition based on other conditions
	ComputeReadyCondition(conditions []conditionsapi.Condition) *conditionsapi.Condition
	
	// SetTransitioningCondition sets a condition to Unknown status with transitioning message
	SetTransitioningCondition(obj StatusUpdater, conditionType conditionsapi.ConditionType, reason, message string)
	
	// RecordEvent records an event for the given object
	RecordEvent(obj StatusUpdater, eventType, reason, message string)
}

// StatusUpdater defines the interface for objects that can have their status updated.
// This interface allows the status manager to work with different TMC resource types.
type StatusUpdater interface {
	GetConditions() conditionsapi.Conditions
	SetConditions(conditions conditionsapi.Conditions)
	GetObjectKind() metav1.Object
}

// Manager implements StatusManagerInterface for TMC resource status management.
// It provides comprehensive status management capabilities including condition updates,
// event recording, and health monitoring.
type Manager struct {
	recorder record.EventRecorder
	logger   logr.Logger
}

// NewManager creates a new status manager with the given event recorder and logger.
func NewManager(recorder record.EventRecorder, logger logr.Logger) StatusManagerInterface {
	return &Manager{
		recorder: recorder,
		logger:   logger,
	}
}

// UpdateStatus updates the status conditions on a resource and records appropriate events.
// It performs intelligent condition merging and only records events for condition changes.
func (m *Manager) UpdateStatus(ctx context.Context, obj StatusUpdater, newConditions []conditionsapi.Condition) error {
	if obj == nil {
		return fmt.Errorf("cannot update status on nil object")
	}

	logger := m.logger.WithValues("name", obj.GetObjectKind().GetName(), "namespace", obj.GetObjectKind().GetNamespace())
	
	currentConditions := obj.GetConditions()
	updatedConditions := make([]conditionsapi.Condition, len(currentConditions))
	copy(updatedConditions, currentConditions)
	
	// Track condition changes for event generation
	var changedConditions []conditionsapi.Condition
	
	// Update or add new conditions
	for _, newCondition := range newConditions {
		oldCondition := conditionsutil.FindStatusCondition(updatedConditions, newCondition.Type)
		
		// Set transition time if this is a new condition or status changed
		if oldCondition == nil || oldCondition.Status != newCondition.Status {
			newCondition.LastTransitionTime = metav1.NewTime(time.Now())
			changedConditions = append(changedConditions, newCondition)
		} else if oldCondition != nil {
			// Preserve transition time for unchanged status
			newCondition.LastTransitionTime = oldCondition.LastTransitionTime
		}
		
		// Update or append the condition
		conditionsutil.SetStatusCondition(&updatedConditions, newCondition)
	}
	
	// Update the object's conditions
	obj.SetConditions(updatedConditions)
	
	// Record events for changed conditions
	for _, condition := range changedConditions {
		eventType := corev1.EventTypeNormal
		if condition.Status == corev1.ConditionFalse && 
		   condition.Severity == conditionsapi.ConditionSeverityError {
			eventType = corev1.EventTypeWarning
		}
		
		message := fmt.Sprintf("Condition %s is now %s: %s", condition.Type, condition.Status, condition.Message)
		m.RecordEvent(obj, eventType, string(condition.Reason), message)
		
		logger.V(2).Info("Updated condition",
			"conditionType", condition.Type,
			"status", condition.Status,
			"reason", condition.Reason,
			"message", condition.Message)
	}
	
	return nil
}

// ComputeReadyCondition computes the overall Ready condition based on other conditions.
// It follows KCP patterns for condition aggregation and severity-based ready computation.
func (m *Manager) ComputeReadyCondition(conditions []conditionsapi.Condition) *conditionsapi.Condition {
	if len(conditions) == 0 {
		return NewConditionBuilder(conditionsapi.ReadyCondition).
			WithStatus(corev1.ConditionUnknown).
			WithReason("NoConditions").
			WithMessage("No status conditions available").
			WithSeverity(conditionsapi.ConditionSeverityInfo).
			Build()
	}
	
	// Check for critical errors (Error severity, False status)
	criticalConditions := GetCriticalConditions(conditions)
	if len(criticalConditions) > 0 {
		messages := make([]string, len(criticalConditions))
		for i, condition := range criticalConditions {
			messages[i] = fmt.Sprintf("%s: %s", condition.Type, condition.Message)
		}
		
		return NewConditionBuilder(conditionsapi.ReadyCondition).
			WithStatus(corev1.ConditionFalse).
			WithReason("ComponentsNotReady").
			WithMessage(fmt.Sprintf("Critical conditions not ready: %v", messages)).
			WithSeverity(conditionsapi.ConditionSeverityError).
			Build()
	}
	
	// Check for unknown conditions
	unknownCount := 0
	for _, condition := range conditions {
		if condition.Status == corev1.ConditionUnknown {
			unknownCount++
		}
	}
	
	if unknownCount > 0 {
		return NewConditionBuilder(conditionsapi.ReadyCondition).
			WithStatus(corev1.ConditionUnknown).
			WithReason("ComponentsUnknown").
			WithMessage(fmt.Sprintf("%d conditions have unknown status", unknownCount)).
			WithSeverity(conditionsapi.ConditionSeverityInfo).
			Build()
	}
	
	// All conditions are True or Warning severity False
	return NewConditionBuilder(conditionsapi.ReadyCondition).
		WithStatus(corev1.ConditionTrue).
		WithReason("ComponentsReady").
		WithMessage("All components are ready").
		WithSeverity(conditionsapi.ConditionSeverityInfo).
		Build()
}

// SetTransitioningCondition sets a condition to Unknown status indicating a transition is in progress.
func (m *Manager) SetTransitioningCondition(obj StatusUpdater, conditionType conditionsapi.ConditionType, reason, message string) {
	condition := NewConditionBuilder(conditionType).
		WithStatus(corev1.ConditionUnknown).
		WithReason(reason).
		WithMessage(message).
		WithSeverity(conditionsapi.ConditionSeverityInfo).
		Build()
	
	conditions := obj.GetConditions()
	conditionsutil.SetStatusCondition(&conditions, *condition)
	obj.SetConditions(conditions)
}

// RecordEvent records an event for the given object.
func (m *Manager) RecordEvent(obj StatusUpdater, eventType, reason, message string) {
	if m.recorder != nil {
		m.recorder.Event(obj.(metav1.Object), eventType, reason, message)
	}
}

// ConditionBuilder provides a fluent interface for building status conditions.
type ConditionBuilder struct {
	condition *conditionsapi.Condition
}

// NewConditionBuilder creates a new condition builder for the given condition type.
func NewConditionBuilder(conditionType conditionsapi.ConditionType) *ConditionBuilder {
	return &ConditionBuilder{
		condition: &conditionsapi.Condition{
			Type:               conditionType,
			LastTransitionTime: metav1.NewTime(time.Now()),
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
	return b.condition
}

// Convenience functions for common TMC conditions

// ClusterConnectedCondition returns a condition indicating the cluster is connected.
func ClusterConnectedCondition(message string) *conditionsapi.Condition {
	return NewConditionBuilder(tmcv1alpha1.ClusterConnectionCondition).
		WithStatus(corev1.ConditionTrue).
		WithReason(tmcv1alpha1.ClusterConnectedReason).
		WithMessage(message).
		WithSeverity(conditionsapi.ConditionSeverityInfo).
		Build()
}

// ClusterDisconnectedCondition returns a condition indicating the cluster is disconnected.
func ClusterDisconnectedCondition(message string) *conditionsapi.Condition {
	return NewConditionBuilder(tmcv1alpha1.ClusterConnectionCondition).
		WithStatus(corev1.ConditionFalse).
		WithReason(tmcv1alpha1.ClusterDisconnectedReason).
		WithMessage(message).
		WithSeverity(conditionsapi.ConditionSeverityError).
		Build()
}

// ClusterConnectionTimeoutCondition returns a condition indicating connection timeout.
func ClusterConnectionTimeoutCondition(message string) *conditionsapi.Condition {
	return NewConditionBuilder(tmcv1alpha1.ClusterConnectionCondition).
		WithStatus(corev1.ConditionUnknown).
		WithReason(tmcv1alpha1.ClusterConnectionTimeoutReason).
		WithMessage(message).
		WithSeverity(conditionsapi.ConditionSeverityWarning).
		Build()
}

// ClusterRegisteredCondition returns a condition indicating the cluster is registered.
func ClusterRegisteredCondition(message string) *conditionsapi.Condition {
	return NewConditionBuilder(tmcv1alpha1.ClusterRegistrationCondition).
		WithStatus(corev1.ConditionTrue).
		WithReason(tmcv1alpha1.ClusterRegisteredReason).
		WithMessage(message).
		WithSeverity(conditionsapi.ConditionSeverityInfo).
		Build()
}

// HeartbeatHealthyCondition returns a condition indicating healthy heartbeat.
func HeartbeatHealthyCondition(message string) *conditionsapi.Condition {
	return NewConditionBuilder(tmcv1alpha1.HeartbeatCondition).
		WithStatus(corev1.ConditionTrue).
		WithReason(tmcv1alpha1.HeartbeatHealthyReason).
		WithMessage(message).
		WithSeverity(conditionsapi.ConditionSeverityInfo).
		Build()
}

// HeartbeatUnhealthyCondition returns a condition indicating unhealthy heartbeat with dynamic severity.
func HeartbeatUnhealthyCondition(reason, message string) *conditionsapi.Condition {
	severity := conditionsapi.ConditionSeverityError
	if reason == tmcv1alpha1.HeartbeatStaleReason {
		severity = conditionsapi.ConditionSeverityWarning
	}
	
	return NewConditionBuilder(tmcv1alpha1.HeartbeatCondition).
		WithStatus(corev1.ConditionFalse).
		WithReason(reason).
		WithMessage(message).
		WithSeverity(severity).
		Build()
}

// ResourcesAvailableCondition returns a condition indicating resources are available.
func ResourcesAvailableCondition(message string) *conditionsapi.Condition {
	return NewConditionBuilder(tmcv1alpha1.ResourcesAvailableCondition).
		WithStatus(corev1.ConditionTrue).
		WithReason(tmcv1alpha1.ResourcesAvailableReason).
		WithMessage(message).
		WithSeverity(conditionsapi.ConditionSeverityInfo).
		Build()
}

// ResourcesInsufficientCondition returns a condition indicating insufficient resources.
func ResourcesInsufficientCondition(message string) *conditionsapi.Condition {
	return NewConditionBuilder(tmcv1alpha1.ResourcesAvailableCondition).
		WithStatus(corev1.ConditionFalse).
		WithReason(tmcv1alpha1.InsufficientResourcesReason).
		WithMessage(message).
		WithSeverity(conditionsapi.ConditionSeverityWarning).
		Build()
}

// PlacementUnavailableCondition returns a placement unavailable condition with dynamic severity.
func PlacementUnavailableCondition(reason, message string) *conditionsapi.Condition {
	severity := conditionsapi.ConditionSeverityWarning
	if reason == tmcv1alpha1.InsufficientResourcesReason {
		severity = conditionsapi.ConditionSeverityError
	}
	
	return NewConditionBuilder(tmcv1alpha1.PlacementAvailableCondition).
		WithStatus(corev1.ConditionFalse).
		WithReason(reason).
		WithMessage(message).
		WithSeverity(severity).
		Build()
}

// SyncFailedCondition returns a condition indicating sync failure.
func SyncFailedCondition(message string) *conditionsapi.Condition {
	return NewConditionBuilder(tmcv1alpha1.SyncedCondition).
		WithStatus(corev1.ConditionFalse).
		WithReason(tmcv1alpha1.SyncFailedReason).
		WithMessage(message).
		WithSeverity(conditionsapi.ConditionSeverityError).
		Build()
}

// Helper functions for condition analysis

// IsConditionTrue returns true if the specified condition is True.
func IsConditionTrue(obj conditionsutil.Getter, conditionType conditionsapi.ConditionType) bool {
	return conditionsutil.IsStatusConditionTrue(obj.GetConditions(), conditionType)
}

// IsConditionFalse returns true if the specified condition is False.
func IsConditionFalse(obj conditionsutil.Getter, conditionType conditionsapi.ConditionType) bool {
	return conditionsutil.IsStatusConditionFalse(obj.GetConditions(), conditionType)
}

// IsConditionUnknown returns true if the specified condition is Unknown.
func IsConditionUnknown(obj conditionsutil.Getter, conditionType conditionsapi.ConditionType) bool {
	condition := conditionsutil.FindStatusCondition(obj.GetConditions(), conditionType)
	return condition != nil && condition.Status == corev1.ConditionUnknown
}

// GetConditionReason returns the reason for the specified condition, or empty string if not found.
func GetConditionReason(obj conditionsutil.Getter, conditionType conditionsapi.ConditionType) string {
	condition := conditionsutil.FindStatusCondition(obj.GetConditions(), conditionType)
	if condition != nil {
		return condition.Reason
	}
	return ""
}

// HasCriticalConditionError returns true if any condition has Error severity and False status.
func HasCriticalConditionError(conditions []conditionsapi.Condition) bool {
	for _, condition := range conditions {
		if condition.Status == corev1.ConditionFalse && condition.Severity == conditionsapi.ConditionSeverityError {
			return true
		}
	}
	return false
}

// GetCriticalConditions returns all conditions with Error severity and False status.
func GetCriticalConditions(conditions []conditionsapi.Condition) []conditionsapi.Condition {
	var critical []conditionsapi.Condition
	for _, condition := range conditions {
		if condition.Status == corev1.ConditionFalse && condition.Severity == conditionsapi.ConditionSeverityError {
			critical = append(critical, condition)
		}
	}
	return critical
}

// GetWarningConditions returns all conditions with Warning severity and False status.
func GetWarningConditions(conditions []conditionsapi.Condition) []conditionsapi.Condition {
	var warnings []conditionsapi.Condition
	for _, condition := range conditions {
		if condition.Status == corev1.ConditionFalse && condition.Severity == conditionsapi.ConditionSeverityWarning {
			warnings = append(warnings, condition)
		}
	}
	return warnings
}