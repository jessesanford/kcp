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
	"time"

	"k8s.io/apimachinery/pkg/runtime"
)

// ReconcilerFactory creates reconcilers for different resource types.
// This factory pattern allows different implementations for different environments
// (e.g., testing vs production) and supports dependency injection.
type ReconcilerFactory interface {
	// NewReconciler creates a new reconciler instance for the given manager.
	// The factory MUST configure the reconciler with appropriate:
	// - Client for API access
	// - Logger for structured logging  
	// - Event recorder for Kubernetes events
	// - Any resource-specific dependencies
	NewReconciler(mgr Manager) (Reconciler, error)

	// SupportsType returns true if this factory can create reconcilers
	// for the given resource type. This enables dynamic reconciler selection.
	SupportsType(resourceType string) bool

	// GetPriority returns the priority of this factory for a given resource type.
	// Higher values indicate higher priority. Used for factory selection when
	// multiple factories support the same type.
	GetPriority(resourceType string) int
}

// ReconcileResult represents the outcome of a reconciliation attempt.
// It encapsulates both the success/failure state and any retry behavior needed.
type ReconcileResult interface {
	// ShouldRequeue indicates if the item should be requeued for another attempt.
	// True means the reconciliation should be retried after some delay.
	ShouldRequeue() bool

	// RequeueAfter returns the duration to wait before requeuing.
	// Only meaningful when ShouldRequeue returns true.
	// A zero duration means requeue immediately.
	RequeueAfter() time.Duration

	// Error returns any error that occurred during reconciliation.
	// A non-nil error typically means ShouldRequeue should return true.
	Error() error

	// IsSuccess returns true if the reconciliation completed successfully.
	// This is the inverse of having an error, but provides clearer semantics.
	IsSuccess() bool
}

// ReconcileContext provides contextual information about the reconciliation environment.
// This is particularly important in KCP where resources exist in logical clusters
// and may need workspace-aware processing.
type ReconcileContext interface {
	// GetWorkspace returns the logical cluster name where the resource exists.
	// In KCP, this represents the workspace containing the resource being reconciled.
	// This is crucial for workspace isolation and proper API client configuration.
	GetWorkspace() string

	// GetClusterName returns the physical cluster name where processing occurs.
	// This may differ from the logical workspace in multi-cluster scenarios.
	GetClusterName() string

	// GetNamespace returns the namespace of the resource being reconciled.
	// Returns empty string for cluster-scoped resources.
	GetNamespace() string

	// GetResourceName returns the name of the resource being reconciled.
	GetResourceName() string

	// GetResourceVersion returns the resource version of the object.
	// This can be used for optimistic locking and conflict detection.
	GetResourceVersion() string

	// GetUID returns the unique identifier of the resource.
	// This is stable across updates and useful for correlation.
	GetUID() string
}

// EventRecorder provides an interface for recording Kubernetes events.
// This abstracts the event recording to enable testing and different backends.
type EventRecorder interface {
	// Event records an event with the given object, type, reason, and message.
	// The eventType should be "Normal" or "Warning".
	// The reason should be a short, machine-readable string.
	// The message should be a human-readable description.
	Event(object runtime.Object, eventType, reason, message string)

	// Eventf records an event with formatted message string.
	// Similar to Event but supports printf-style formatting.
	Eventf(object runtime.Object, eventType, reason, format string, args ...interface{})

	// AnnotatedEventf records an event with additional annotations.
	// Annotations provide structured metadata about the event.
	AnnotatedEventf(object runtime.Object, annotations map[string]string, eventType, reason, format string, args ...interface{})
}