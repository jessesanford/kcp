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

package controller

import (
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
)

// mockReconcilerFactory implements the ReconcilerFactory interface for testing
type mockReconcilerFactory struct {
	supportedTypes map[string]int // type -> priority
}

func (m *mockReconcilerFactory) NewReconciler(mgr Manager) (Reconciler, error) {
	return &mockReconciler{}, nil
}

func (m *mockReconcilerFactory) SupportsType(resourceType string) bool {
	_, exists := m.supportedTypes[resourceType]
	return exists
}

func (m *mockReconcilerFactory) GetPriority(resourceType string) int {
	return m.supportedTypes[resourceType]
}

func TestReconcilerFactoryInterface(t *testing.T) {
	// Verify that mockReconcilerFactory implements the ReconcilerFactory interface
	var _ ReconcilerFactory = &mockReconcilerFactory{}

	factory := &mockReconcilerFactory{
		supportedTypes: map[string]int{
			"Pod":     10,
			"Service": 5,
		},
	}

	// Test SupportsType
	if !factory.SupportsType("Pod") {
		t.Error("expected factory to support Pod type")
	}

	if factory.SupportsType("UnknownType") {
		t.Error("expected factory to not support UnknownType")
	}

	// Test GetPriority
	if priority := factory.GetPriority("Pod"); priority != 10 {
		t.Errorf("expected priority 10 for Pod, got %d", priority)
	}

	if priority := factory.GetPriority("Service"); priority != 5 {
		t.Errorf("expected priority 5 for Service, got %d", priority)
	}

	// Test NewReconciler
	manager := &mockManager{}
	reconciler, err := factory.NewReconciler(manager)
	if err != nil {
		t.Errorf("unexpected error creating reconciler: %v", err)
	}
	if reconciler == nil {
		t.Error("expected non-nil reconciler")
	}
}

// mockReconcileResult implements the ReconcileResult interface for testing
type mockReconcileResult struct {
	requeue      bool
	requeueAfter time.Duration
	err          error
}

func (m *mockReconcileResult) ShouldRequeue() bool {
	return m.requeue
}

func (m *mockReconcileResult) RequeueAfter() time.Duration {
	return m.requeueAfter
}

func (m *mockReconcileResult) Error() error {
	return m.err
}

func (m *mockReconcileResult) IsSuccess() bool {
	return m.err == nil
}

func TestReconcileResultInterface(t *testing.T) {
	// Verify that mockReconcileResult implements the ReconcileResult interface
	var _ ReconcileResult = &mockReconcileResult{}

	// Test successful result
	successResult := &mockReconcileResult{
		requeue:      false,
		requeueAfter: 0,
		err:          nil,
	}

	if successResult.ShouldRequeue() {
		t.Error("expected successful result to not requeue")
	}

	if !successResult.IsSuccess() {
		t.Error("expected result to be successful")
	}

	if successResult.Error() != nil {
		t.Error("expected no error for successful result")
	}

	// Test error result with requeue
	errorResult := &mockReconcileResult{
		requeue:      true,
		requeueAfter: 5 * time.Minute,
		err:          &testError{message: "test error"},
	}

	if !errorResult.ShouldRequeue() {
		t.Error("expected error result to requeue")
	}

	if errorResult.IsSuccess() {
		t.Error("expected result to not be successful")
	}

	if errorResult.RequeueAfter() != 5*time.Minute {
		t.Errorf("expected requeue after 5 minutes, got %v", errorResult.RequeueAfter())
	}

	if errorResult.Error() == nil {
		t.Error("expected error to be present")
	}
}

// mockReconcileContext implements the ReconcileContext interface for testing
type mockReconcileContext struct {
	workspace       string
	clusterName     string
	namespace       string
	resourceName    string
	resourceVersion string
	uid             string
}

func (m *mockReconcileContext) GetWorkspace() string {
	return m.workspace
}

func (m *mockReconcileContext) GetClusterName() string {
	return m.clusterName
}

func (m *mockReconcileContext) GetNamespace() string {
	return m.namespace
}

func (m *mockReconcileContext) GetResourceName() string {
	return m.resourceName
}

func (m *mockReconcileContext) GetResourceVersion() string {
	return m.resourceVersion
}

func (m *mockReconcileContext) GetUID() string {
	return m.uid
}

func TestReconcileContextInterface(t *testing.T) {
	// Verify that mockReconcileContext implements the ReconcileContext interface
	var _ ReconcileContext = &mockReconcileContext{}

	ctx := &mockReconcileContext{
		workspace:       "root:test-workspace",
		clusterName:     "test-cluster",
		namespace:       "test-namespace",
		resourceName:    "test-resource",
		resourceVersion: "12345",
		uid:             "abc-123-def",
	}

	if ctx.GetWorkspace() != "root:test-workspace" {
		t.Errorf("expected workspace 'root:test-workspace', got %s", ctx.GetWorkspace())
	}

	if ctx.GetClusterName() != "test-cluster" {
		t.Errorf("expected cluster name 'test-cluster', got %s", ctx.GetClusterName())
	}

	if ctx.GetNamespace() != "test-namespace" {
		t.Errorf("expected namespace 'test-namespace', got %s", ctx.GetNamespace())
	}

	if ctx.GetResourceName() != "test-resource" {
		t.Errorf("expected resource name 'test-resource', got %s", ctx.GetResourceName())
	}

	if ctx.GetResourceVersion() != "12345" {
		t.Errorf("expected resource version '12345', got %s", ctx.GetResourceVersion())
	}

	if ctx.GetUID() != "abc-123-def" {
		t.Errorf("expected UID 'abc-123-def', got %s", ctx.GetUID())
	}
}

// mockEventRecorder implements the EventRecorder interface for testing
type mockEventRecorder struct {
	events []eventRecord
}

type eventRecord struct {
	object      runtime.Object
	eventType   string
	reason      string
	message     string
	annotations map[string]string
}

func (m *mockEventRecorder) Event(object runtime.Object, eventType, reason, message string) {
	m.events = append(m.events, eventRecord{
		object:    object,
		eventType: eventType,
		reason:    reason,
		message:   message,
	})
}

func (m *mockEventRecorder) Eventf(object runtime.Object, eventType, reason, format string, args ...interface{}) {
	message := format // Simple implementation for testing
	if len(args) > 0 {
		// In real implementation, this would use fmt.Sprintf
		message = format + " (formatted)"
	}
	m.Event(object, eventType, reason, message)
}

func (m *mockEventRecorder) AnnotatedEventf(object runtime.Object, annotations map[string]string, eventType, reason, format string, args ...interface{}) {
	message := format // Simple implementation for testing
	if len(args) > 0 {
		message = format + " (formatted)"
	}
	m.events = append(m.events, eventRecord{
		object:      object,
		eventType:   eventType,
		reason:      reason,
		message:     message,
		annotations: annotations,
	})
}

func TestEventRecorderInterface(t *testing.T) {
	// Verify that mockEventRecorder implements the EventRecorder interface
	var _ EventRecorder = &mockEventRecorder{}

	recorder := &mockEventRecorder{}

	// Test Event
	recorder.Event(nil, "Normal", "Created", "Resource created successfully")
	if len(recorder.events) != 1 {
		t.Errorf("expected 1 event, got %d", len(recorder.events))
	}

	event := recorder.events[0]
	if event.eventType != "Normal" {
		t.Errorf("expected event type 'Normal', got %s", event.eventType)
	}
	if event.reason != "Created" {
		t.Errorf("expected reason 'Created', got %s", event.reason)
	}

	// Test Eventf
	recorder.Eventf(nil, "Warning", "Failed", "Failed to create resource")
	if len(recorder.events) != 2 {
		t.Errorf("expected 2 events, got %d", len(recorder.events))
	}

	// Test AnnotatedEventf
	annotations := map[string]string{"component": "test"}
	recorder.AnnotatedEventf(nil, annotations, "Normal", "Updated", "Resource updated")
	if len(recorder.events) != 3 {
		t.Errorf("expected 3 events, got %d", len(recorder.events))
	}

	annotatedEvent := recorder.events[2]
	if annotatedEvent.annotations["component"] != "test" {
		t.Error("expected annotation 'component=test'")
	}
}

// testError is a simple error implementation for testing
type testError struct {
	message string
}

func (e *testError) Error() string {
	return e.message
}