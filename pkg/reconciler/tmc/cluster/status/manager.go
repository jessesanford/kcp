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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"

	conditionsapi "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
	conditionsutil "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/util/conditions"
)

// StatusManagerInterface defines the interface for managing TMC cluster status.
type StatusManagerInterface interface {
	// UpdateStatus updates the status of an object with conditions
	UpdateStatus(ctx context.Context, obj StatusUpdater, conditions []conditionsapi.Condition) error
	
	// RecordEvent records an event for status changes
	RecordEvent(obj metav1.Object, eventType, reason, message string)
	
	// ComputeReadyCondition computes the overall Ready condition based on dependent conditions
	ComputeReadyCondition(conditions []conditionsapi.Condition) *conditionsapi.Condition
	
	// SetTransitioningCondition sets a condition indicating a transition is in progress
	SetTransitioningCondition(obj StatusUpdater, conditionType conditionsapi.ConditionType, reason, message string)
}

// StatusUpdater interface defines methods for objects that can have their status updated.
type StatusUpdater interface {
	conditionsutil.Getter
	conditionsutil.Setter
	metav1.Object
}

// Manager implements StatusManagerInterface for TMC cluster status management.
type Manager struct {
	eventRecorder record.EventRecorder
	logger        klog.Logger
}

// NewManager creates a new status manager instance.
//
// Parameters:
//   - eventRecorder: Event recorder for Kubernetes events
//   - logger: Logger for status operations
//
// Returns:
//   - StatusManagerInterface: Configured status manager
func NewManager(eventRecorder record.EventRecorder, logger klog.Logger) StatusManagerInterface {
	return &Manager{
		eventRecorder: eventRecorder,
		logger:        logger.WithName("status-manager"),
	}
}

// UpdateStatus updates the status of an object with the provided conditions.
// It performs intelligent condition merging and deduplication.
func (m *Manager) UpdateStatus(ctx context.Context, obj StatusUpdater, conditions []conditionsapi.Condition) error {
	if obj == nil {
		return fmt.Errorf("object cannot be nil")
	}

	logger := m.logger.WithValues("object", klog.KObj(obj), "namespace", obj.GetNamespace())
	
	// Get current conditions
	currentConditions := obj.GetConditions()
	
	// Apply new conditions intelligently
	updatedConditions := m.mergeConditions(currentConditions, conditions)
	
	// Check if conditions actually changed to avoid unnecessary updates
	if conditionsEqual(currentConditions, updatedConditions) {
		logger.V(4).Info("Conditions unchanged, skipping status update")
		return nil
	}
	
	// Set the updated conditions
	obj.SetConditions(updatedConditions)
	
	logger.V(2).Info("Status updated with conditions", "conditionsCount", len(updatedConditions))
	
	// Record events for significant condition changes
	m.recordConditionEvents(obj, currentConditions, updatedConditions)
	
	return nil
}

// RecordEvent records a Kubernetes event for status-related changes.
func (m *Manager) RecordEvent(obj metav1.Object, eventType, reason, message string) {
	if m.eventRecorder != nil && obj != nil {
		m.eventRecorder.Event(obj, eventType, reason, message)
		m.logger.V(3).Info("Event recorded", 
			"object", klog.KObj(obj), 
			"type", eventType, 
			"reason", reason,
			"message", message)
	}
}

// ComputeReadyCondition computes the overall Ready condition based on dependent conditions.
// This follows Kubernetes patterns where Ready is True only when all critical conditions are True.
func (m *Manager) ComputeReadyCondition(conditions []conditionsapi.Condition) *conditionsapi.Condition {
	if len(conditions) == 0 {
		return conditionsutil.UnknownCondition(
			conditionsapi.ReadyCondition,
			"NoConditions",
			"No conditions available to determine readiness",
		)
	}

	// Find any False conditions with Error severity
	var errorConditions []conditionsapi.Condition
	var warningConditions []conditionsapi.Condition
	var unknownConditions []conditionsapi.Condition
	
	for _, condition := range conditions {
		// Skip the Ready condition itself to avoid infinite recursion
		if condition.Type == conditionsapi.ReadyCondition {
			continue
		}
		
		switch condition.Status {
		case corev1.ConditionFalse:
			if condition.Severity == conditionsapi.ConditionSeverityError {
				errorConditions = append(errorConditions, condition)
			} else {
				warningConditions = append(warningConditions, condition)
			}
		case corev1.ConditionUnknown:
			unknownConditions = append(unknownConditions, condition)
		}
	}

	// If there are error conditions, Ready is False
	if len(errorConditions) > 0 {
		return conditionsutil.FalseCondition(
			conditionsapi.ReadyCondition,
			"ComponentsNotReady",
			conditionsapi.ConditionSeverityError,
			"Critical components are not ready: %s",
			getConditionNames(errorConditions),
		)
	}

	// If there are unknown conditions, Ready is Unknown
	if len(unknownConditions) > 0 {
		return conditionsutil.UnknownCondition(
			conditionsapi.ReadyCondition,
			"ComponentsUnknown",
			"Some components have unknown status: %s",
			getConditionNames(unknownConditions),
		)
	}

	// If there are warning conditions, Ready is True but with a message
	message := "All components are ready"
	if len(warningConditions) > 0 {
		message = fmt.Sprintf("Ready with warnings: %s", getConditionNames(warningConditions))
	}

	return &conditionsapi.Condition{
		Type:    conditionsapi.ReadyCondition,
		Status:  corev1.ConditionTrue,
		Reason:  "ComponentsReady",
		Message: message,
	}
}

// SetTransitioningCondition sets a condition to indicate that a transition is in progress.
func (m *Manager) SetTransitioningCondition(obj StatusUpdater, conditionType conditionsapi.ConditionType, reason, message string) {
	if obj == nil {
		return
	}
	
	condition := conditionsutil.UnknownCondition(conditionType, reason, message)
	conditionsutil.Set(obj, condition)
	
	m.logger.V(3).Info("Transitioning condition set", 
		"object", klog.KObj(obj),
		"type", string(conditionType),
		"reason", reason)
}

// mergeConditions intelligently merges new conditions with existing ones.
func (m *Manager) mergeConditions(current, new []conditionsapi.Condition) []conditionsapi.Condition {
	if len(new) == 0 {
		return current
	}
	
	// Create a map of new conditions by type for efficient lookup
	newConditionsByType := make(map[conditionsapi.ConditionType]conditionsapi.Condition)
	for _, condition := range new {
		newConditionsByType[condition.Type] = condition
	}
	
	// Update existing conditions and collect unchanged ones
	var result []conditionsapi.Condition
	for _, currentCondition := range current {
		if newCondition, exists := newConditionsByType[currentCondition.Type]; exists {
			// Use the new condition, but preserve LastTransitionTime if state hasn't changed
			if hasSameState(&currentCondition, &newCondition) {
				newCondition.LastTransitionTime = currentCondition.LastTransitionTime
			} else {
				if newCondition.LastTransitionTime.IsZero() {
					newCondition.LastTransitionTime = metav1.NewTime(time.Now().UTC().Truncate(time.Second))
				}
			}
			result = append(result, newCondition)
			delete(newConditionsByType, currentCondition.Type)
		} else {
			// Keep existing condition that wasn't updated
			result = append(result, currentCondition)
		}
	}
	
	// Add completely new conditions
	for _, condition := range newConditionsByType {
		if condition.LastTransitionTime.IsZero() {
			condition.LastTransitionTime = metav1.NewTime(time.Now().UTC().Truncate(time.Second))
		}
		result = append(result, condition)
	}
	
	return result
}

// recordConditionEvents records events for significant condition changes.
func (m *Manager) recordConditionEvents(obj metav1.Object, oldConditions, newConditions []conditionsapi.Condition) {
	if m.eventRecorder == nil {
		return
	}
	
	// Create maps for efficient comparison
	oldMap := make(map[conditionsapi.ConditionType]conditionsapi.Condition)
	for _, condition := range oldConditions {
		oldMap[condition.Type] = condition
	}
	
	// Check for condition changes that should trigger events
	for _, newCondition := range newConditions {
		oldCondition, existed := oldMap[newCondition.Type]
		
		// Record event for new conditions or status changes
		if !existed {
			m.RecordEvent(obj, corev1.EventTypeNormal, "ConditionAdded", 
				fmt.Sprintf("Condition %s added with status %s", newCondition.Type, newCondition.Status))
		} else if oldCondition.Status != newCondition.Status {
			eventType := corev1.EventTypeNormal
			if newCondition.Status == corev1.ConditionFalse && newCondition.Severity == conditionsapi.ConditionSeverityError {
				eventType = corev1.EventTypeWarning
			}
			
			m.RecordEvent(obj, eventType, "ConditionChanged", 
				fmt.Sprintf("Condition %s changed from %s to %s: %s", 
					newCondition.Type, oldCondition.Status, newCondition.Status, newCondition.Message))
		}
	}
}

// Helper functions

// conditionsEqual compares two condition slices for equality.
func conditionsEqual(a, b []conditionsapi.Condition) bool {
	if len(a) != len(b) {
		return false
	}
	
	// Create maps for comparison
	aMap := make(map[conditionsapi.ConditionType]conditionsapi.Condition)
	for _, condition := range a {
		aMap[condition.Type] = condition
	}
	
	for _, bCondition := range b {
		aCondition, exists := aMap[bCondition.Type]
		if !exists || !equality.Semantic.DeepEqual(aCondition, bCondition) {
			return false
		}
	}
	
	return true
}

// hasSameState compares the state-relevant fields of two conditions.
func hasSameState(a, b *conditionsapi.Condition) bool {
	return a.Type == b.Type &&
		a.Status == b.Status &&
		a.Reason == b.Reason &&
		a.Severity == b.Severity &&
		a.Message == b.Message
}

// getConditionNames returns a comma-separated list of condition type names.
func getConditionNames(conditions []conditionsapi.Condition) string {
	if len(conditions) == 0 {
		return ""
	}
	
	names := make([]string, len(conditions))
	for i, condition := range conditions {
		names[i] = string(condition.Type)
	}
	
	if len(names) == 1 {
		return names[0]
	}
	
	// For multiple names, format as "A, B and C"
	if len(names) == 2 {
		return fmt.Sprintf("%s and %s", names[0], names[1])
	}
	
	result := ""
	for i, name := range names[:len(names)-1] {
		if i > 0 {
			result += ", "
		}
		result += name
	}
	result += " and " + names[len(names)-1]
	
	return result
}