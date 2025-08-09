/*
Copyright 2025 The KCP Authors.

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
	"fmt"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"
)

// EventType represents different types of TMC events.
type EventType string

const (
	// Cluster events
	EventClusterRegistered   EventType = "ClusterRegistered"
	EventClusterDeregistered EventType = "ClusterDeregistered"
	EventClusterHealthy      EventType = "ClusterHealthy"
	EventClusterUnhealthy    EventType = "ClusterUnhealthy"

	// Placement events
	EventPlacementScheduled EventType = "PlacementScheduled"
	EventPlacementFailed    EventType = "PlacementFailed"
	EventPlacementUpdated   EventType = "PlacementUpdated"
	EventPlacementDeleted   EventType = "PlacementDeleted"

	// Sync events
	EventSyncStarted   EventType = "SyncStarted"
	EventSyncCompleted EventType = "SyncCompleted"
	EventSyncFailed    EventType = "SyncFailed"
)

// EventReason provides standardized reason codes for TMC events.
type EventReason string

const (
	// Success reasons
	ReasonRegistrationSucceeded EventReason = "RegistrationSucceeded"
	ReasonPlacementSucceeded    EventReason = "PlacementSucceeded"
	ReasonSyncSucceeded         EventReason = "SyncSucceeded"
	ReasonHealthCheckPassed     EventReason = "HealthCheckPassed"

	// Failure reasons
	ReasonRegistrationFailed   EventReason = "RegistrationFailed"
	ReasonPlacementFailed      EventReason = "PlacementFailed"
	ReasonSyncFailed           EventReason = "SyncFailed"
	ReasonHealthCheckFailed    EventReason = "HealthCheckFailed"
	ReasonInvalidConfiguration EventReason = "InvalidConfiguration"
	ReasonTimeoutExceeded      EventReason = "TimeoutExceeded"

	// Warning reasons
	ReasonCapacityWarning     EventReason = "CapacityWarning"
	ReasonConnectivityIssue   EventReason = "ConnectivityIssue"
	ReasonPerformanceDegraded EventReason = "PerformanceDegraded"
)

// TMCEvent represents a structured TMC event with workspace context.
type TMCEvent struct {
	Type        EventType
	Reason      EventReason
	Message     string
	Workspace   logicalcluster.Name
	Object      runtime.Object
	Timestamp   time.Time
	Source      string
	EventType   string
	Count       int32
	Metadata    map[string]interface{}
}

// EventHandler defines the interface for handling TMC events.
type EventHandler interface {
	// RecordEvent records a TMC event with workspace context
	RecordEvent(ctx context.Context, event *TMCEvent) error

	// RecordEventf records a formatted TMC event
	RecordEventf(ctx context.Context, object runtime.Object, workspace logicalcluster.Name, 
		eventType string, reason EventReason, messageFmt string, args ...interface{}) error

	// GetEvents retrieves events for a specific object in a workspace
	GetEvents(ctx context.Context, object runtime.Object, workspace logicalcluster.Name) ([]*TMCEvent, error)
}

// EventNotifier defines the interface for event notifications.
type EventNotifier interface {
	// NotifyEvent sends notifications for significant TMC events
	NotifyEvent(ctx context.Context, event *TMCEvent) error

	// AddListener adds an event listener for specific event types
	AddListener(eventTypes []EventType, listener EventListener) error

	// RemoveListener removes an event listener
	RemoveListener(listener EventListener) error
}

// EventListener defines the interface for event listeners.
type EventListener interface {
	// OnEvent is called when a matching event occurs
	OnEvent(ctx context.Context, event *TMCEvent) error

	// GetEventTypes returns the event types this listener is interested in
	GetEventTypes() []EventType

	// GetID returns a unique identifier for this listener
	GetID() string
}

// EventRecorderOptions configures the TMC event recorder.
type EventRecorderOptions struct {
	Source                  string
	EnableNotifications     bool
	NotificationTimeout     time.Duration
	MaxEventAge             time.Duration
	EventBatchSize          int
}

// tmcEventRecorder implements EventHandler for recording TMC events.
type tmcEventRecorder struct {
	eventRecorder record.EventRecorder
	options       EventRecorderOptions
	notifier      EventNotifier
	eventStore    *eventStore
	mu            sync.RWMutex
}

// NewEventRecorder creates a new TMC event recorder.
func NewEventRecorder(eventRecorder record.EventRecorder, options EventRecorderOptions) (EventHandler, error) {
	if options.Source == "" {
		options.Source = "tmc-controller"
	}
	if options.NotificationTimeout == 0 {
		options.NotificationTimeout = 30 * time.Second
	}
	if options.MaxEventAge == 0 {
		options.MaxEventAge = 24 * time.Hour
	}

	recorder := &tmcEventRecorder{
		eventRecorder: eventRecorder,
		options:       options,
		eventStore:    newEventStore(options.MaxEventAge),
	}

	if options.EnableNotifications {
		notifier, err := newEventNotifier(options.NotificationTimeout)
		if err != nil {
			return nil, fmt.Errorf("failed to create event notifier: %w", err)
		}
		recorder.notifier = notifier
	}

	return recorder, nil
}

// RecordEvent records a TMC event with workspace context.
func (r *tmcEventRecorder) RecordEvent(ctx context.Context, event *TMCEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.validateEvent(event); err != nil {
		return fmt.Errorf("invalid event: %w", err)
	}

	// Set defaults
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	if event.Source == "" {
		event.Source = r.options.Source
	}
	if event.Count == 0 {
		event.Count = 1
	}

	// Store in local event store
	r.eventStore.addEvent(event)

	// Send notifications if enabled
	if r.notifier != nil {
		if err := r.notifier.NotifyEvent(ctx, event); err != nil {
			klog.V(4).Infof("Failed to send event notification: %v", err)
		}
	}

	klog.V(6).Infof("Recorded TMC event: %s/%s in workspace %s", 
		event.Type, event.Reason, event.Workspace)

	return nil
}

// RecordEventf records a formatted TMC event.
func (r *tmcEventRecorder) RecordEventf(
	ctx context.Context,
	object runtime.Object,
	workspace logicalcluster.Name,
	eventType string,
	reason EventReason,
	messageFmt string,
	args ...interface{},
) error {
	event := &TMCEvent{
		Type:      EventType(fmt.Sprintf("TMC%s", eventType)),
		Reason:    reason,
		Message:   fmt.Sprintf(messageFmt, args...),
		Workspace: workspace,
		Object:    object,
		EventType: eventType,
		Source:    r.options.Source,
	}
	return r.RecordEvent(ctx, event)
}

// GetEvents retrieves events for a specific object in a workspace.
func (r *tmcEventRecorder) GetEvents(
	ctx context.Context,
	object runtime.Object,
	workspace logicalcluster.Name,
) ([]*TMCEvent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.eventStore.getEventsForWorkspace(workspace), nil
}

// validateEvent validates the event data.
func (r *tmcEventRecorder) validateEvent(event *TMCEvent) error {
	if event == nil {
		return fmt.Errorf("event cannot be nil")
	}
	if event.Type == "" {
		return fmt.Errorf("event type cannot be empty")
	}
	if event.Reason == "" {
		return fmt.Errorf("event reason cannot be empty")
	}
	if event.Message == "" {
		return fmt.Errorf("event message cannot be empty")
	}
	if event.Workspace.Empty() {
		return fmt.Errorf("workspace cannot be empty")
	}
	return nil
}