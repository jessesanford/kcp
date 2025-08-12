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

package events

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"
)

// mockEventRecorder is a test implementation of record.EventRecorder.
type mockEventRecorder struct {
	events []mockEvent
}

type mockEvent struct {
	object    runtime.Object
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
	// Not implemented for this test
}

func (m *mockEventRecorder) AnnotatedEventf(object runtime.Object, annotations map[string]string, eventType, reason, messageFmt string, args ...interface{}) {
	// Not implemented for this test
}

func (m *mockEventRecorder) getEvents() []mockEvent {
	return m.events
}

func (m *mockEventRecorder) reset() {
	m.events = nil
}

// createTestPod creates a test pod for use in event recording tests.
func createTestPod(name, namespace string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "test",
					Image: "test:latest",
				},
			},
		},
	}
}

func TestNewClusterEventRecorder(t *testing.T) {
	mockRecorder := &mockEventRecorder{}
	workspace := logicalcluster.Name("root:test")
	logger := klog.Background()

	recorder := NewClusterEventRecorder(mockRecorder, workspace, logger)

	if recorder == nil {
		t.Fatal("Expected non-nil ClusterEventRecorder")
	}
	if recorder.GetWorkspace() != workspace {
		t.Errorf("GetWorkspace() returned %s, expected %s", recorder.GetWorkspace(), workspace)
	}
}

func TestClusterEventRecorder_RecordClusterEvent(t *testing.T) {
	ctx := context.Background()
	testPod := createTestPod("test-pod", "default")
	workspace := logicalcluster.Name("root:test")
	mockRecorder := &mockEventRecorder{}
	recorder := NewClusterEventRecorder(mockRecorder, workspace, klog.Background())

	recorder.RecordClusterEvent(ctx, testPod, EventTypeClusterHealthy, ReasonHealthCheckPassed, "Health check passed")

	events := mockRecorder.getEvents()
	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	event := events[0]
	if event.eventType != corev1.EventTypeNormal {
		t.Errorf("Expected eventType %s, got %s", corev1.EventTypeNormal, event.eventType)
	}
	if event.reason != "HealthCheckPassed" {
		t.Errorf("Expected reason HealthCheckPassed, got %s", event.reason)
	}
}

func TestClusterEventRecorder_RecordClusterEvent_NilRecorder(t *testing.T) {
	ctx := context.Background()
	testPod := createTestPod("test-pod", "default")
	workspace := logicalcluster.Name("root:test")

	recorder := NewClusterEventRecorder(nil, workspace, klog.Background())
	
	// This should not panic
	recorder.RecordClusterEvent(ctx, testPod, EventTypeClusterHealthy, ReasonHealthCheckPassed, "test message")
}


func TestClusterEventRecorder_RecordClusterHealthEvent(t *testing.T) {
	ctx := context.Background()
	testPod := createTestPod("test-pod", "default")
	workspace := logicalcluster.Name("root:test")
	mockRecorder := &mockEventRecorder{}
	recorder := NewClusterEventRecorder(mockRecorder, workspace, klog.Background())

	recorder.RecordClusterHealthEvent(ctx, testPod, true, "All systems operational")

	events := mockRecorder.getEvents()
	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	event := events[0]
	if event.eventType != corev1.EventTypeNormal {
		t.Errorf("Expected eventType %s, got %s", corev1.EventTypeNormal, event.eventType)
	}
	if event.reason != string(ReasonHealthCheckPassed) {
		t.Errorf("Expected reason %s, got %s", ReasonHealthCheckPassed, event.reason)
	}
}


func TestClusterEventRecorder_WithLogger(t *testing.T) {
	originalRecorder := &mockEventRecorder{}
	workspace := logicalcluster.Name("root:test")
	originalLogger := klog.Background()
	newLogger := klog.Background()

	recorder := NewClusterEventRecorder(originalRecorder, workspace, originalLogger)
	newRecorder := recorder.WithLogger(newLogger)

	// Should be a new instance
	if recorder == newRecorder {
		t.Error("WithLogger should return a new instance")
	}

	// Should preserve other fields
	if newRecorder.recorder != originalRecorder {
		t.Error("WithLogger should preserve recorder")
	}
	if newRecorder.workspace != workspace {
		t.Error("WithLogger should preserve workspace")
	}
}