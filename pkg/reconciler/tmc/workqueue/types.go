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

package workqueue

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/workqueue"

	"github.com/kcp-dev/logicalcluster/v3"
)

// WorkItemType represents different types of work items that can be processed.
type WorkItemType string

const (
	// Cluster work item types
	WorkItemClusterRegistration   WorkItemType = "ClusterRegistration"
	WorkItemClusterDeregistration WorkItemType = "ClusterDeregistration"
	WorkItemClusterHealthCheck    WorkItemType = "ClusterHealthCheck"
	WorkItemClusterCapacityUpdate WorkItemType = "ClusterCapacityUpdate"

	// Placement work item types
	WorkItemPlacementScheduling WorkItemType = "PlacementScheduling"
	WorkItemPlacementUpdate     WorkItemType = "PlacementUpdate"
	WorkItemPlacementDeletion   WorkItemType = "PlacementDeletion"

	// Sync work item types
	WorkItemResourceSync WorkItemType = "ResourceSync"
	WorkItemStatusSync   WorkItemType = "StatusSync"
	WorkItemConfigSync   WorkItemType = "ConfigSync"

	// Maintenance work item types
	WorkItemCleanup     WorkItemType = "Cleanup"
	WorkItemValidation  WorkItemType = "Validation"
	WorkItemReconcile   WorkItemType = "Reconcile"
)

// Priority represents the priority level of a work item.
type Priority int

const (
	// Priority levels (lower number = higher priority)
	PriorityImmediate Priority = iota // Critical operations
	PriorityHigh                      // Important operations
	PriorityNormal                    // Standard operations
	PriorityLow                       // Background operations
	PriorityBulk                      // Batch operations
)

// WorkItem represents a unit of work to be processed by TMC controllers.
type WorkItem struct {
	// ID uniquely identifies this work item
	ID string

	// Type classifies the work item
	Type WorkItemType

	// Priority determines processing order
	Priority Priority

	// Workspace identifies the logical cluster context
	Workspace logicalcluster.Name

	// Object is the Kubernetes object being processed
	Object runtime.Object

	// Key is the workqueue key for this item
	Key string

	// Metadata contains additional data for processing
	Metadata map[string]interface{}

	// CreatedAt tracks when the work item was created
	CreatedAt time.Time

	// Attempts tracks how many times this item has been processed
	Attempts int

	// LastAttemptAt tracks the last processing attempt
	LastAttemptAt time.Time

	// LastError tracks the last error encountered
	LastError error

	// ProcessingTimeout sets maximum processing time
	ProcessingTimeout time.Duration

	// RetryAfter specifies when to retry after failure
	RetryAfter time.Duration
}

// TMCWorkQueue defines the interface for TMC-enhanced workqueue operations.
type TMCWorkQueue interface {
	// Add adds a work item to the queue
	Add(ctx context.Context, item *WorkItem) error

	// AddAfter adds a work item to be processed after the specified duration
	AddAfter(ctx context.Context, item *WorkItem, duration time.Duration) error

	// AddWithPriority adds a work item with specific priority
	AddWithPriority(ctx context.Context, item *WorkItem, priority Priority) error

	// Get retrieves the next work item from the queue
	Get() (*WorkItem, bool)

	// Done marks a work item as processed
	Done(item *WorkItem)

	// AddRateLimited adds a work item with rate limiting applied
	AddRateLimited(ctx context.Context, item *WorkItem) error

	// Forget removes rate limiting tracking for a work item
	Forget(item *WorkItem)

	// NumRequeues returns the number of times an item has been requeued
	NumRequeues(item *WorkItem) int

	// Len returns the current queue length
	Len() int

	// ShutDown shuts down the work queue
	ShutDown()

	// ShuttingDown returns true if the queue is shutting down
	ShuttingDown() bool
}

// ProcessorFunc defines the function signature for work item processors.
type ProcessorFunc func(ctx context.Context, item *WorkItem) error

// RetryPolicy defines how work items should be retried on failure.
type RetryPolicy struct {
	// MaxAttempts is the maximum number of retry attempts
	MaxAttempts int

	// BaseDelay is the base delay between retries
	BaseDelay time.Duration

	// MaxDelay is the maximum delay between retries
	MaxDelay time.Duration

	// BackoffFactor is the multiplier for exponential backoff
	BackoffFactor float64

	// Jitter adds randomness to retry delays
	Jitter bool

	// RetryableErrors defines which errors should trigger retries
	RetryableErrors []ErrorMatcher
}

// ErrorMatcher defines criteria for matching retryable errors.
type ErrorMatcher interface {
	// Matches returns true if the error should trigger a retry
	Matches(err error) bool

	// GetBackoffOverride returns custom backoff for this error type
	GetBackoffOverride() *time.Duration
}

// CircuitBreakerConfig configures circuit breaker behavior.
type CircuitBreakerConfig struct {
	// MaxFailures is the number of failures before opening the circuit
	MaxFailures int

	// Timeout is how long to wait before attempting to close the circuit
	Timeout time.Duration

	// SuccessThreshold is the number of successes needed to close the circuit
	SuccessThreshold int

	// OnStateChange is called when the circuit breaker state changes
	OnStateChange func(from, to CircuitBreakerState)
}

// CircuitBreakerState represents the state of a circuit breaker.
type CircuitBreakerState int

const (
	CircuitBreakerClosed CircuitBreakerState = iota
	CircuitBreakerOpen
	CircuitBreakerHalfOpen
)

// RateLimiterConfig configures rate limiting behavior.
type RateLimiterConfig struct {
	// BaseDelay is the base delay for rate limiting
	BaseDelay time.Duration

	// MaxDelay is the maximum delay for rate limiting
	MaxDelay time.Duration

	// PerWorkspaceLimit enables per-workspace rate limiting
	PerWorkspaceLimit bool

	// WorkspaceQPS sets queries per second per workspace
	WorkspaceQPS float32

	// WorkspaceBurst sets burst capacity per workspace
	WorkspaceBurst int

	// GlobalQPS sets global queries per second
	GlobalQPS float32

	// GlobalBurst sets global burst capacity
	GlobalBurst int
}

// WorkQueueOptions configures TMC workqueue behavior.
type WorkQueueOptions struct {
	// Name identifies the workqueue for metrics and logging
	Name string

	// RetryPolicy configures retry behavior
	RetryPolicy RetryPolicy

	// RateLimiter configures rate limiting
	RateLimiter RateLimiterConfig

	// CircuitBreaker configures circuit breaker behavior
	CircuitBreaker *CircuitBreakerConfig

	// EnableMetrics enables metrics collection
	EnableMetrics bool

	// MetricsNamespace sets the namespace for metrics
	MetricsNamespace string

	// WorkerCount sets the number of worker goroutines
	WorkerCount int

	// ProcessingTimeout sets the default processing timeout
	ProcessingTimeout time.Duration

	// ShutdownTimeout sets the graceful shutdown timeout
	ShutdownTimeout time.Duration
}

// WorkQueueMetrics provides metrics about workqueue performance.
type WorkQueueMetrics struct {
	// QueueLength is the current number of items in the queue
	QueueLength int

	// ProcessingDuration is the average processing duration
	ProcessingDuration time.Duration

	// TotalProcessed is the total number of items processed
	TotalProcessed int64

	// TotalFailed is the total number of items that failed
	TotalFailed int64

	// TotalRetried is the total number of items retried
	TotalRetried int64

	// AverageRetries is the average number of retries per item
	AverageRetries float64

	// WorkerUtilization is the percentage of time workers are busy
	WorkerUtilization float64

	// LastProcessedAt is when the last item was processed
	LastProcessedAt time.Time

	// CircuitBreakerState is the current circuit breaker state
	CircuitBreakerState CircuitBreakerState

	// ActiveWorkers is the current number of active workers
	ActiveWorkers int
}

// WorkerPool manages a pool of workers for processing work items.
type WorkerPool interface {
	// Start starts the specified number of workers
	Start(ctx context.Context, workers int) error

	// Stop stops all workers gracefully
	Stop(ctx context.Context) error

	// AddWorker adds a new worker to the pool
	AddWorker(ctx context.Context) error

	// RemoveWorker removes a worker from the pool
	RemoveWorker(ctx context.Context) error

	// GetWorkerCount returns the current number of workers
	GetWorkerCount() int

	// GetActiveWorkers returns the number of currently active workers
	GetActiveWorkers() int

	// GetMetrics returns worker pool metrics
	GetMetrics() WorkerPoolMetrics
}

// WorkerPoolMetrics provides metrics about worker pool performance.
type WorkerPoolMetrics struct {
	// TotalWorkers is the total number of workers
	TotalWorkers int

	// ActiveWorkers is the number of currently active workers
	ActiveWorkers int

	// IdleWorkers is the number of idle workers
	IdleWorkers int

	// WorkerUtilization is the average worker utilization
	WorkerUtilization float64

	// AverageProcessingTime is the average time spent processing items
	AverageProcessingTime time.Duration

	// TotalItemsProcessed is the total number of items processed by all workers
	TotalItemsProcessed int64

	// WorkerErrors is the number of worker-level errors
	WorkerErrors int64
}