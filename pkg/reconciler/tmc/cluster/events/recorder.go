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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"
)

// EventType represents the type of cluster event being recorded.
type EventType string

const (
	// EventTypeClusterRegistered indicates a cluster has been registered.
	EventTypeClusterRegistered EventType = "ClusterRegistered"

	// EventTypeClusterHealthy indicates a cluster is healthy.
	EventTypeClusterHealthy EventType = "ClusterHealthy"

	// EventTypeClusterUnhealthy indicates a cluster is unhealthy.
	EventTypeClusterUnhealthy EventType = "ClusterUnhealthy"

	// EventTypeClusterDisconnected indicates a cluster has disconnected.
	EventTypeClusterDisconnected EventType = "ClusterDisconnected"

	// EventTypeClusterReconnected indicates a cluster has reconnected.
	EventTypeClusterReconnected EventType = "ClusterReconnected"

	// EventTypeClusterCapacityUpdated indicates cluster capacity information was updated.
	EventTypeClusterCapacityUpdated EventType = "CapacityUpdated"

	// EventTypeClusterDeleted indicates a cluster has been deleted.
	EventTypeClusterDeleted EventType = "ClusterDeleted"
)

// EventReason represents the reason for a cluster event.
type EventReason string

const (
	// ReasonRegistrationSuccessful indicates cluster registration was successful.
	ReasonRegistrationSuccessful EventReason = "RegistrationSuccessful"

	// ReasonRegistrationFailed indicates cluster registration failed.
	ReasonRegistrationFailed EventReason = "RegistrationFailed"

	// ReasonHealthCheckPassed indicates health check passed.
	ReasonHealthCheckPassed EventReason = "HealthCheckPassed"

	// ReasonHealthCheckFailed indicates health check failed.
	ReasonHealthCheckFailed EventReason = "HealthCheckFailed"

	// ReasonConnectionLost indicates connection to cluster was lost.
	ReasonConnectionLost EventReason = "ConnectionLost"

	// ReasonConnectionRestored indicates connection to cluster was restored.
	ReasonConnectionRestored EventReason = "ConnectionRestored"

	// ReasonCapacityReported indicates cluster reported its capacity.
	ReasonCapacityReported EventReason = "CapacityReported"

	// ReasonDeletionRequested indicates cluster deletion was requested.
	ReasonDeletionRequested EventReason = "DeletionRequested"

	// ReasonDeletionCompleted indicates cluster deletion was completed.
	ReasonDeletionCompleted EventReason = "DeletionCompleted"
)

// ClusterEventRecorder is responsible for recording events related to TMC cluster management.
// It provides workspace-aware event recording that maintains proper isolation
// and follows KCP patterns for distributed multi-tenancy.
type ClusterEventRecorder struct {
	// recorder is the underlying Kubernetes event recorder.
	recorder record.EventRecorder

	// workspace represents the logical cluster context for event recording.
	workspace logicalcluster.Name

	// logger provides structured logging for event recording operations.
	logger klog.Logger
}

// NewClusterEventRecorder creates a new ClusterEventRecorder for the specified workspace.
//
// Parameters:
//   - recorder: Kubernetes event recorder instance
//   - workspace: Logical cluster name for workspace isolation
//   - logger: Structured logger for event recording operations
//
// Returns:
//   - *ClusterEventRecorder: Configured event recorder ready for use
func NewClusterEventRecorder(
	recorder record.EventRecorder,
	workspace logicalcluster.Name,
	logger klog.Logger,
) *ClusterEventRecorder {
	return &ClusterEventRecorder{
		recorder:  recorder,
		workspace: workspace,
		logger:    logger.WithName("cluster-event-recorder").WithValues("workspace", workspace),
	}
}

// RecordClusterEvent records an event for a cluster object with proper workspace isolation.
//
// Parameters:
//   - ctx: Context for the operation
//   - obj: The object to record the event for
//   - eventType: Type of event being recorded
//   - reason: Reason for the event
//   - message: Human-readable message describing the event
func (r *ClusterEventRecorder) RecordClusterEvent(
	ctx context.Context,
	obj runtime.Object,
	eventType EventType,
	reason EventReason,
	message string,
) {
	if r.recorder == nil {
		r.logger.V(2).Info("Event recorder not available, skipping event", 
			"eventType", eventType, "reason", reason)
		return
	}

	// Enrich the message with workspace context
	enrichedMessage := fmt.Sprintf("[%s] %s", r.workspace, message)

	// Record the event with appropriate type mapping
	switch eventType {
	case EventTypeClusterRegistered, EventTypeClusterHealthy, EventTypeClusterReconnected, EventTypeClusterCapacityUpdated:
		r.recorder.Event(obj, corev1.EventTypeNormal, string(reason), enrichedMessage)
	case EventTypeClusterUnhealthy, EventTypeClusterDisconnected, EventTypeClusterDeleted:
		r.recorder.Event(obj, corev1.EventTypeWarning, string(reason), enrichedMessage)
	default:
		r.recorder.Event(obj, corev1.EventTypeNormal, string(reason), enrichedMessage)
	}

	r.logger.V(4).Info("Recorded cluster event",
		"eventType", eventType,
		"reason", reason,
		"message", message,
		"workspace", r.workspace,
	)
}

// RecordClusterHealthEvent records events related to cluster health status changes.
func (r *ClusterEventRecorder) RecordClusterHealthEvent(
	ctx context.Context,
	obj runtime.Object,
	healthy bool,
	details string,
) {
	if healthy {
		r.RecordClusterEvent(ctx, obj, EventTypeClusterHealthy, 
			ReasonHealthCheckPassed, details)
	} else {
		r.RecordClusterEvent(ctx, obj, EventTypeClusterUnhealthy, 
			ReasonHealthCheckFailed, details)
	}
}

// GetWorkspace returns the workspace this event recorder is associated with.
func (r *ClusterEventRecorder) GetWorkspace() logicalcluster.Name {
	return r.workspace
}

// WithLogger returns a new ClusterEventRecorder with the specified logger.
func (r *ClusterEventRecorder) WithLogger(logger klog.Logger) *ClusterEventRecorder {
	return &ClusterEventRecorder{
		recorder:  r.recorder,
		workspace: r.workspace,
		logger:    logger.WithName("cluster-event-recorder").WithValues("workspace", r.workspace),
	}
}