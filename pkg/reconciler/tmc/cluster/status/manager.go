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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"

	conditionsapi "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
	conditionsutil "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/util/conditions"
)

// StatusUpdater defines the interface for objects that can have their status updated
type StatusUpdater interface {
	runtime.Object
	metav1.Object
	GetConditions() conditionsapi.Conditions
	SetConditions(conditionsapi.Conditions)
}

// StatusManagerInterface defines the interface for managing status updates
type StatusManagerInterface interface {
	UpdateStatus(ctx context.Context, obj StatusUpdater, conditions conditionsapi.Conditions) error
	ComputeReadyCondition(conditions conditionsapi.Conditions) conditionsapi.Condition
	SetTransitioningCondition(obj StatusUpdater, conditionType conditionsapi.ConditionType, reason, message string)
	RecordEvent(obj runtime.Object, eventType, reason, message string)
}

// Manager implements StatusManagerInterface
type Manager struct {
	recorder record.EventRecorder
	logger   klog.Logger
}

// NewManager creates a new status manager
func NewManager(recorder record.EventRecorder, logger klog.Logger) StatusManagerInterface {
	return &Manager{
		recorder: recorder,
		logger:   logger,
	}
}

// UpdateStatus updates the status conditions on an object
func (m *Manager) UpdateStatus(ctx context.Context, obj StatusUpdater, conditions conditionsapi.Conditions) error {
	if obj == nil {
		return fmt.Errorf("object cannot be nil")
	}

	// Track if any conditions changed
	changed := false

	// Update or add new conditions
	for _, newCondition := range conditions {
		existingCondition := conditionsutil.Get(obj, newCondition.Type)
		
		// Check if condition changed
		if existingCondition == nil || 
			existingCondition.Status != newCondition.Status ||
			existingCondition.Reason != newCondition.Reason ||
			existingCondition.Message != newCondition.Message {
			
			conditionsutil.Set(obj, &newCondition)
			changed = true
			
			// Record event for condition changes
			if existingCondition == nil {
				m.recorder.Eventf(obj, corev1.EventTypeNormal, "ConditionAdded", 
					"Added condition %s: %s", newCondition.Type, newCondition.Message)
			} else {
				m.recorder.Eventf(obj, corev1.EventTypeNormal, "ConditionUpdated", 
					"Updated condition %s: %s", newCondition.Type, newCondition.Message)
			}
		}
	}

	// Compute and set the Ready condition if conditions changed
	if changed {
		readyCondition := m.ComputeReadyCondition(obj.GetConditions())
		conditionsutil.Set(obj, &readyCondition)
	}

	return nil
}

// ComputeReadyCondition computes the overall Ready condition based on other conditions
func (m *Manager) ComputeReadyCondition(conditions conditionsapi.Conditions) conditionsapi.Condition {
	if len(conditions) == 0 {
		return conditionsapi.Condition{
			Type:               conditionsapi.ReadyCondition,
			Status:             corev1.ConditionUnknown,
			Reason:             "NoConditions",
			Message:            "No conditions available",
			LastTransitionTime: metav1.Now(),
		}
	}

	// Check for critical (Error severity) conditions
	critical := GetCriticalConditions(conditions)
	if len(critical) > 0 {
		return conditionsapi.Condition{
			Type:               conditionsapi.ReadyCondition,
			Status:             corev1.ConditionFalse,
			Reason:             "ComponentsNotReady",
			Message:            fmt.Sprintf("Critical conditions: %d", len(critical)),
			LastTransitionTime: metav1.Now(),
		}
	}

	// Check for unknown conditions
	hasUnknown := false
	for _, condition := range conditions {
		if condition.Type == conditionsapi.ReadyCondition {
			continue // Skip the Ready condition itself
		}
		if condition.Status == corev1.ConditionUnknown {
			hasUnknown = true
			break
		}
	}

	if hasUnknown {
		return conditionsapi.Condition{
			Type:               conditionsapi.ReadyCondition,
			Status:             corev1.ConditionUnknown,
			Reason:             "ComponentsUnknown",
			Message:            "Some components have unknown status",
			LastTransitionTime: metav1.Now(),
		}
	}

	// All conditions are either True or Warning level False
	return conditionsapi.Condition{
		Type:               conditionsapi.ReadyCondition,
		Status:             corev1.ConditionTrue,
		Reason:             "ComponentsReady",
		Message:            "All components are ready",
		LastTransitionTime: metav1.Now(),
	}
}

// SetTransitioningCondition sets a condition to Unknown status while transitioning
func (m *Manager) SetTransitioningCondition(obj StatusUpdater, conditionType conditionsapi.ConditionType, reason, message string) {
	condition := conditionsapi.Condition{
		Type:               conditionType,
		Status:             corev1.ConditionUnknown,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
		Severity:           conditionsapi.ConditionSeverityInfo,
	}
	
	conditionsutil.Set(obj, &condition)
}

// RecordEvent records an event for the object
func (m *Manager) RecordEvent(obj runtime.Object, eventType, reason, message string) {
	m.recorder.Event(obj, eventType, reason, message)
}