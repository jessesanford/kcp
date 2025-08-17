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
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	kcpfake "github.com/kcp-dev/client-go/kubernetes/fake"
	kcptesting "github.com/kcp-dev/client-go/third_party/k8s.io/client-go/testing"
	"github.com/kcp-dev/logicalcluster/v3"
)

func TestEventFilter_ShouldSync(t *testing.T) {
	tests := []struct {
		name     string
		config   EventSyncConfig
		event    *corev1.Event
		expected bool
	}{
		{
			name: "normal event should sync",
			config: EventSyncConfig{
				IncludeTypes: []string{"Normal", "Warning"},
			},
			event: &corev1.Event{
				Type:   corev1.EventTypeNormal,
				Reason: "Scheduled",
			},
			expected: true,
		},
		{
			name: "filtered event should not sync",
			config: EventSyncConfig{
				ExcludeTypes: []string{"Normal"},
			},
			event: &corev1.Event{
				Type:   corev1.EventTypeNormal,
				Reason: "Pulled",
			},
			expected: false,
		},
		{
			name: "noisy event should be filtered",
			config: EventSyncConfig{},
			event: &corev1.Event{
				Type:   corev1.EventTypeNormal,
				Reason: "SuccessfulMount",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := NewEventFilter(tt.config)
			result := filter.ShouldSync(tt.event)
			if result != tt.expected {
				t.Errorf("ShouldSync() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestEventAggregator_ShouldAggregate(t *testing.T) {
	aggregator := NewEventAggregator(EventSyncConfig{})

	tests := []struct {
		name     string
		event    *corev1.Event
		expected bool
	}{
		{
			name: "warning event should not aggregate",
			event: &corev1.Event{
				Type: corev1.EventTypeWarning,
			},
			expected: false,
		},
		{
			name: "normal event should aggregate",
			event: &corev1.Event{
				Type:   corev1.EventTypeNormal,
				Reason: "SomeReason",
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := aggregator.ShouldAggregate(tt.event)
			if result != tt.expected {
				t.Errorf("ShouldAggregate() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestEventSyncer_transformEvent(t *testing.T) {
	syncer := &EventSyncer{
		syncTargetName: "test-cluster",
		workspace:      logicalcluster.Name("root:test"),
		config:         EventSyncConfig{},
	}

	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-event",
			Namespace: "default",
		},
		InvolvedObject: corev1.ObjectReference{
			Kind:      "Pod",
			Name:      "test-pod",
			Namespace: "default",
		},
	}

	transformed := syncer.transformEvent(event)

	// Check that sync target metadata was added
	if transformed.Labels["kcp.io/sync-target"] != "test-cluster" {
		t.Errorf("Expected sync-target label to be 'test-cluster', got %v", transformed.Labels["kcp.io/sync-target"])
	}

	if transformed.Labels["kcp.io/workspace"] != "root:test" {
		t.Errorf("Expected workspace label to be 'root:test', got %v", transformed.Labels["kcp.io/workspace"])
	}

	if transformed.Annotations["kcp.io/source-cluster"] != "test-cluster" {
		t.Errorf("Expected source-cluster annotation to be 'test-cluster', got %v", transformed.Annotations["kcp.io/source-cluster"])
	}

	// Check that name was modified to prevent conflicts
	expectedName := "test-cluster-test-event"
	if transformed.Name != expectedName {
		t.Errorf("Expected transformed name to be %v, got %v", expectedName, transformed.Name)
	}
}

func TestEventSyncer_syncToKCP(t *testing.T) {
	kcpClient := kcpfake.NewSimpleClientset()
	downstreamClient := fake.NewSimpleClientset()

	syncer := NewEventSyncer(
		kcpClient,
		downstreamClient,
		"test-cluster",
		logicalcluster.Name("root:test"),
		EventSyncConfig{},
	)

	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-event",
			Namespace: "default",
		},
		Type:   corev1.EventTypeNormal,
		Reason: "Test",
	}

	ctx := context.TODO()
	err := syncer.syncToKCP(ctx, event)
	if err != nil {
		t.Errorf("syncToKCP() returned error: %v", err)
	}

	// Verify the event was created in KCP
	actions := kcpClient.Actions()
	if len(actions) != 1 {
		t.Errorf("Expected 1 action, got %d", len(actions))
	}

	createAction := actions[0].(kcptesting.CreateAction)
	if createAction.GetVerb() != "create" {
		t.Errorf("Expected create action, got %v", createAction.GetVerb())
	}
}

func TestEventSyncer_isDuplicate(t *testing.T) {
	syncer := &EventSyncer{
		seenEvents: make(map[string]time.Time),
	}

	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
		},
		InvolvedObject: corev1.ObjectReference{
			Kind: "Pod",
			Name: "test-pod",
		},
		Reason:  "Test",
		Message: "Test message",
	}

	// First check should not be duplicate
	if syncer.isDuplicate(event) {
		t.Error("First check should not be duplicate")
	}

	// Mark as seen
	syncer.markSeen(event)

	// Second check should be duplicate
	if !syncer.isDuplicate(event) {
		t.Error("Second check should be duplicate")
	}
}

func TestEventAggregator_AddEvent(t *testing.T) {
	config := EventSyncConfig{
		MaxAggregatedEvents: 10,
		AggregationWindow:   time.Minute,
	}
	aggregator := NewEventAggregator(config)

	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-event",
			Namespace: "default",
		},
		InvolvedObject: corev1.ObjectReference{
			Kind: "Pod",
			Name: "test-pod",
		},
		Type:           corev1.EventTypeNormal,
		Reason:         "Test",
		Message:        "Test message",
		FirstTimestamp: metav1.Now(),
		LastTimestamp:  metav1.Now(),
	}

	// Add first event
	aggregator.AddEvent(event)

	stats := aggregator.GetAggregationStats()
	if stats["active_groups"] != 1 {
		t.Errorf("Expected 1 active group, got %v", stats["active_groups"])
	}

	// Add similar event
	aggregator.AddEvent(event)

	stats = aggregator.GetAggregationStats()
	if stats["total_aggregated_events"].(int32) != 2 {
		t.Errorf("Expected 2 total aggregated events, got %v", stats["total_aggregated_events"])
	}
}