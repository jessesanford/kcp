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
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/record"
)

// Controller defines the base controller interface for TMC.
// All TMC controllers MUST implement this interface to participate
// in the controller lifecycle management.
type Controller interface {
	// Start begins the controller's reconciliation loop.
	// It MUST be non-blocking and return quickly.
	// The provided context will be canceled when the controller should stop.
	Start(ctx context.Context) error

	// Stop gracefully shuts down the controller.
	// It MUST wait for in-flight reconciliations to complete.
	// This method should be idempotent and safe to call multiple times.
	Stop() error

	// Name returns the controller's unique identifier.
	// This name is used for logging, metrics, and debugging.
	// It MUST be unique within a controller manager instance.
	Name() string

	// Ready indicates if the controller is ready to process items.
	// Returns false during startup/shutdown or when experiencing issues.
	Ready() bool
}

// Reconciler processes individual work items from a work queue.
// Implementations MUST be idempotent and reentrant since the same
// item may be processed multiple times.
type Reconciler interface {
	// Reconcile processes a single work item identified by key.
	// The key format is typically "namespace/name" or just "name" for cluster-scoped resources.
	// Returns an error if the item should be requeued for retry.
	// A nil return indicates successful processing.
	Reconcile(ctx context.Context, key string) error

	// SetupWithManager configures the reconciler with the controller manager.
	// This is called during controller initialization to establish
	// watches, event handlers, and other manager integrations.
	SetupWithManager(mgr Manager) error

	// GetLogger returns the reconciler's logger for structured logging.
	// The logger should include contextual information like controller name.
	GetLogger() logr.Logger
}

// WorkQueue manages the work items for a controller.
// It provides a thread-safe way to queue and process work items
// with support for rate limiting and retry logic.
type WorkQueue interface {
	// Add enqueues an item for processing.
	// The item will be processed by the reconciler.
	// Duplicate items are deduplicated automatically.
	Add(item interface{})

	// Get retrieves the next item to process.
	// Returns the item and a boolean indicating if the queue is shutting down.
	// Blocks until an item is available or shutdown is signaled.
	Get() (interface{}, bool)

	// Done marks an item as completed processing.
	// This MUST be called for every item retrieved with Get().
	// Failure to call Done will cause the item to be stuck.
	Done(item interface{})

	// Forget removes an item from the rate limiter.
	// This should be called when an item is successfully processed
	// or when giving up on retries.
	Forget(item interface{})

	// Len returns the current queue depth.
	// This is useful for monitoring and metrics.
	Len() int

	// ShuttingDown returns true if the queue is shutting down.
	// No new items should be added when shutting down.
	ShuttingDown() bool

	// ShutDown signals the queue to shut down and stop processing.
	// This causes Get() to return false for shutdown indication.
	ShutDown()
}

// Manager provides shared dependencies and lifecycle management for controllers.
// It acts as a central hub for controller coordination and resource sharing.
type Manager interface {
	// AddController registers a new controller with the manager.
	// The manager will handle the controller's lifecycle (start/stop).
	// Returns an error if the controller cannot be registered.
	AddController(ctrl Controller) error

	// GetClient returns the dynamic client for API operations.
	// The client is pre-configured with appropriate authentication
	// and is safe for concurrent use across multiple controllers.
	GetClient() dynamic.Interface

	// GetScheme returns the runtime scheme for object serialization.
	// This includes all the types that controllers need to work with.
	GetScheme() *runtime.Scheme

	// GetEventRecorder returns the event recorder for Kubernetes events.
	// Controllers should use this to record events on objects they manage.
	GetEventRecorder() record.EventRecorder

	// GetLogger returns a logger for the manager.
	// Controllers can derive their own loggers from this base logger.
	GetLogger() logr.Logger

	// Start starts all registered controllers.
	// This blocks until the provided context is canceled.
	// All controllers are started concurrently.
	Start(ctx context.Context) error

	// GetControllers returns all registered controllers.
	// This is primarily used for testing and debugging.
	GetControllers() []Controller
}