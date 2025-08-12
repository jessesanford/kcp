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
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"

	conditionsapi "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
	
	tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
)

// mockStatusUpdater implements StatusUpdater interface for testing.
type mockStatusUpdater struct {
	metav1.ObjectMeta
	conditions conditionsapi.Conditions
}

func (m *mockStatusUpdater) GetConditions() conditionsapi.Conditions {
	return m.conditions
}

func (m *mockStatusUpdater) SetConditions(conditions conditionsapi.Conditions) {
	m.conditions = conditions
}

func (m *mockStatusUpdater) DeepCopyObject() runtime.Object {
	copy := &mockStatusUpdater{
		ObjectMeta: *m.ObjectMeta.DeepCopy(),
		conditions: make(conditionsapi.Conditions, len(m.conditions)),
	}
	for i, condition := range m.conditions {
		copy.conditions[i] = *condition.DeepCopy()
	}
	return copy
}

func (m *mockStatusUpdater) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

// mockEventRecorder implements record.EventRecorder for testing.
type mockEventRecorder struct {
	events []mockEvent
}

type mockEvent struct {
	object    interface{}
	eventType string
	reason    string
	message   string
}

func (m *mockEventRecorder) Event(object runtime.Object, eventType, reason, message string) {
	m.events = append(m.events, mockEvent{
		object:    object,
		eventType: eventType,
		reason:    reason,
		message:   message,
	})
}

func (m *mockEventRecorder) Eventf(object runtime.Object, eventType, reason, messageFmt string, args ...interface{}) {
	m.Event(object, eventType, reason, fmt.Sprintf(messageFmt, args...))
}

func (m *mockEventRecorder) AnnotatedEventf(object runtime.Object, annotations map[string]string, eventType, reason, messageFmt string, args ...interface{}) {
	m.Event(object, eventType, reason, fmt.Sprintf(messageFmt, args...))
}

func TestNewManager(t *testing.T) {
	recorder := &mockEventRecorder{}
	logger := klog.NewKlogr()

	manager := NewManager(recorder, logger)

	if manager == nil {
		t.Fatal("NewManager returned nil")
	}

	// Verify it implements the interface
	var _ StatusManagerInterface = manager
}

func TestManager_UpdateStatus(t *testing.T) {
	recorder := &mockEventRecorder{}
	logger := klog.NewKlogr()
	manager := NewManager(recorder, logger).(*Manager)

	tests := map[string]struct {
		obj           StatusUpdater
		conditions    []conditionsapi.Condition
		wantError     bool
		wantConditions int
		wantEvents    int
	}{
		"nil object": {
			obj:       nil,
			wantError: true,
		},
		"empty conditions": {
			obj: &mockStatusUpdater{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-namespace",
				},
			},
			conditions:     []conditionsapi.Condition{},
			wantConditions: 0,
			wantEvents:     0,
		},
		"new conditions": {
			obj: &mockStatusUpdater{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-namespace",
				},
			},
			conditions: conditionsapi.Conditions{
				*ClusterConnectedCondition("Cluster is connected"),
				*HeartbeatHealthyCondition("Heartbeat is healthy"),
			},
			wantConditions: 2,
			wantEvents:     2,
		},
		"update existing condition": {
			obj: &mockStatusUpdater{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-namespace",
				},
				conditions: conditionsapi.Conditions{
					*ClusterConnectedCondition("Initially connected"),
				},
			},
			conditions: conditionsapi.Conditions{
				*ClusterDisconnectedCondition("Connection lost"),
			},
			wantConditions: 1,
			wantEvents:     1,
		},
		"no change in conditions": {
			obj: &mockStatusUpdater{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-namespace",
				},
				conditions: conditionsapi.Conditions{
					*ClusterConnectedCondition("Cluster is connected"),
				},
			},
			conditions: conditionsapi.Conditions{
				*ClusterConnectedCondition("Cluster is connected"),
			},
			wantConditions: 1,
			wantEvents:     0, // No events for unchanged conditions
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Reset recorder for each test
			recorder.events = nil

			err := manager.UpdateStatus(context.Background(), tc.obj, tc.conditions)

			if tc.wantError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if tc.obj != nil {
				if len(tc.obj.GetConditions()) != tc.wantConditions {
					t.Errorf("Expected %d conditions, got %d", tc.wantConditions, len(tc.obj.GetConditions()))
				}
			}

			if len(recorder.events) != tc.wantEvents {
				t.Errorf("Expected %d events, got %d", tc.wantEvents, len(recorder.events))
			}
		})
	}
}

func TestManager_ComputeReadyCondition(t *testing.T) {
	recorder := &mockEventRecorder{}
	logger := klog.NewKlogr()
	manager := NewManager(recorder, logger).(*Manager)

	tests := map[string]struct {
		conditions []conditionsapi.Condition
		wantStatus corev1.ConditionStatus
		wantReason string
	}{
		"no conditions": {
			conditions: conditionsapi.Conditions{},
			wantStatus: corev1.ConditionUnknown,
			wantReason: "NoConditions",
		},
		"all conditions true": {
			conditions: conditionsapi.Conditions{
				*ClusterConnectedCondition("Connected"),
				*HeartbeatHealthyCondition("Healthy"),
			},
			wantStatus: corev1.ConditionTrue,
			wantReason: "ComponentsReady",
		},
		"critical condition false": {
			conditions: conditionsapi.Conditions{
				*ClusterDisconnectedCondition("Disconnected"),
				*HeartbeatHealthyCondition("Healthy"),
			},
			wantStatus: corev1.ConditionFalse,
			wantReason: "ComponentsNotReady",
		},
		"unknown conditions": {
			conditions: conditionsapi.Conditions{
				*ClusterConnectionTimeoutCondition("Timeout"),
				*HeartbeatHealthyCondition("Healthy"),
			},
			wantStatus: corev1.ConditionUnknown,
			wantReason: "ComponentsUnknown",
		},
		"warning conditions only": {
			conditions: conditionsapi.Conditions{
				*ClusterConnectedCondition("Connected"),
				*ResourcesInsufficientCondition("Low resources"),
			},
			wantStatus: corev1.ConditionTrue,
			wantReason: "ComponentsReady",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			condition := manager.ComputeReadyCondition(tc.conditions)

			if condition.Status != tc.wantStatus {
				t.Errorf("Expected status %v, got %v", tc.wantStatus, condition.Status)
			}

			if condition.Reason != tc.wantReason {
				t.Errorf("Expected reason %q, got %q", tc.wantReason, condition.Reason)
			}

			if condition.Type != conditionsapi.ReadyCondition {
				t.Errorf("Expected condition type %v, got %v", conditionsapi.ReadyCondition, condition.Type)
			}
		})
	}
}

func TestManager_SetTransitioningCondition(t *testing.T) {
	recorder := &mockEventRecorder{}
	logger := klog.NewKlogr()
	manager := NewManager(recorder, logger).(*Manager)

	obj := &mockStatusUpdater{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "test-namespace",
		},
	}

	conditionType := tmcv1alpha1.ClusterConnectionCondition
	reason := "Connecting"
	message := "Attempting to connect to cluster"

	manager.SetTransitioningCondition(obj, conditionType, reason, message)

	conditions := obj.GetConditions()
	if len(conditions) != 1 {
		t.Fatalf("Expected 1 condition, got %d", len(conditions))
	}

	condition := conditions[0]
	if condition.Type != conditionType {
		t.Errorf("Expected condition type %v, got %v", conditionType, condition.Type)
	}

	if condition.Status != corev1.ConditionUnknown {
		t.Errorf("Expected status Unknown, got %v", condition.Status)
	}

	if condition.Reason != reason {
		t.Errorf("Expected reason %q, got %q", reason, condition.Reason)
	}

	if condition.Message != message {
		t.Errorf("Expected message %q, got %q", message, condition.Message)
	}
}

func TestConditionHelpers(t *testing.T) {
	obj := &mockStatusUpdater{
		conditions: []conditionsapi.Condition{
			*ClusterConnectionTimeoutCondition("Timeout"),
		},
	}

	tests := map[string]struct {
		helperFunc func(conditionsutil.Getter, conditionsapi.ConditionType) bool
		conditionType conditionsapi.ConditionType
		expected bool
	}{
		"IsConditionTrue - false condition": {
			helperFunc:    IsConditionTrue,
			conditionType: tmcv1alpha1.ClusterConnectionCondition,
			expected:      false, // Will be false because we set a timeout (unknown) condition
		},
		"IsConditionFalse - false condition": {
			helperFunc:    IsConditionFalse,
			conditionType: tmcv1alpha1.ClusterRegistrationCondition,
			expected:      false, // Condition doesn't exist
		},
		"IsConditionUnknown - unknown condition": {
			helperFunc:    IsConditionUnknown,
			conditionType: tmcv1alpha1.ClusterConnectionCondition,
			expected:      true, // We set a timeout (unknown) condition
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result := tc.helperFunc(obj, tc.conditionType)
			if result != tc.expected {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestHasCriticalConditionError(t *testing.T) {
	tests := map[string]struct {
		conditions []conditionsapi.Condition
		expected   bool
	}{
		"no conditions": {
			conditions: conditionsapi.Conditions{},
			expected:   false,
		},
		"no critical errors": {
			conditions: conditionsapi.Conditions{
				*ClusterConnectedCondition("Connected"),
				*ResourcesInsufficientCondition("Low resources"), // Warning, not error
			},
			expected: false,
		},
		"has critical error": {
			conditions: conditionsapi.Conditions{
				*ClusterConnectedCondition("Connected"),
				*ClusterDisconnectedCondition("Disconnected"), // Error severity
			},
			expected: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result := HasCriticalConditionError(tc.conditions)
			if result != tc.expected {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestPrebuiltConditions(t *testing.T) {
	tests := map[string]struct {
		conditionFunc func(string) *conditionsapi.Condition
		expectedType  conditionsapi.ConditionType
		expectedStatus corev1.ConditionStatus
		expectedReason string
	}{
		"ClusterConnectedCondition": {
			conditionFunc:  ClusterConnectedCondition,
			expectedType:   tmcv1alpha1.ClusterConnectionCondition,
			expectedStatus: corev1.ConditionTrue,
			expectedReason: tmcv1alpha1.ClusterConnectedReason,
		},
		"ClusterDisconnectedCondition": {
			conditionFunc:  ClusterDisconnectedCondition,
			expectedType:   tmcv1alpha1.ClusterConnectionCondition,
			expectedStatus: corev1.ConditionFalse,
			expectedReason: tmcv1alpha1.ClusterDisconnectedReason,
		},
		"ClusterRegisteredCondition": {
			conditionFunc:  ClusterRegisteredCondition,
			expectedType:   tmcv1alpha1.ClusterRegistrationCondition,
			expectedStatus: corev1.ConditionTrue,
			expectedReason: tmcv1alpha1.ClusterRegisteredReason,
		},
		"HeartbeatHealthyCondition": {
			conditionFunc:  HeartbeatHealthyCondition,
			expectedType:   tmcv1alpha1.HeartbeatCondition,
			expectedStatus: corev1.ConditionTrue,
			expectedReason: tmcv1alpha1.HeartbeatHealthyReason,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			message := "test message"
			condition := tc.conditionFunc(message)

			if condition.Type != tc.expectedType {
				t.Errorf("Expected type %v, got %v", tc.expectedType, condition.Type)
			}

			if condition.Status != tc.expectedStatus {
				t.Errorf("Expected status %v, got %v", tc.expectedStatus, condition.Status)
			}

			if condition.Reason != tc.expectedReason {
				t.Errorf("Expected reason %q, got %q", tc.expectedReason, condition.Reason)
			}

			if condition.Message != message {
				t.Errorf("Expected message %q, got %q", message, condition.Message)
			}
		})
	}
}