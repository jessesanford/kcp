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

package interfaces

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kcp-dev/logicalcluster/v3"
)

// SyncEngine is the core interface for TMC's sync engine that manages resource synchronization
// between logical and physical clusters. It provides a workspace-aware sync system that handles
// multi-placement scenarios and maintains consistency across distributed workloads.
//
// The sync engine operates on unstructured resources to provide maximum flexibility for
// different resource types and supports both upstream (physical->logical) and downstream
// (logical->physical) synchronization patterns.
type SyncEngine interface {
	// Start begins the sync engine operations. It initializes workers, starts processing
	// queues, and begins synchronization between clusters.
	//
	// Parameters:
	//   - ctx: Context for cancellation and deadlines
	//
	// Returns error if the engine fails to start properly.
	Start(ctx context.Context) error

	// Stop gracefully shuts down the sync engine, ensuring all in-flight operations
	// complete before returning.
	//
	// Parameters:
	//   - ctx: Context for cancellation and deadlines
	//
	// Returns error if the engine fails to stop cleanly.
	Stop(ctx context.Context) error

	// EnqueueSyncOperation adds a sync operation to the processing queue.
	// Operations are processed based on priority and FIFO within the same priority level.
	//
	// Parameters:
	//   - operation: The sync operation to enqueue
	//
	// Returns error if the operation cannot be enqueued.
	EnqueueSyncOperation(operation SyncOperation) error

	// ProcessSyncOperation executes a single sync operation, handling both upstream
	// and downstream synchronization. It applies transformations, resolves conflicts,
	// and updates status.
	//
	// Parameters:
	//   - ctx: Context for cancellation and deadlines
	//   - operation: The sync operation to process
	//
	// Returns SyncStatus containing the result of the operation.
	ProcessSyncOperation(ctx context.Context, operation SyncOperation) SyncStatus

	// GetSyncStatus retrieves the current status of a sync operation by its ID.
	//
	// Parameters:
	//   - operationID: Unique identifier of the sync operation
	//
	// Returns SyncStatus and bool indicating if the status was found.
	GetSyncStatus(operationID string) (SyncStatus, bool)

	// ListPendingOperations returns all sync operations currently pending in the queue.
	//
	// Parameters:
	//   - workspace: Optional workspace filter (empty for all workspaces)
	//   - direction: Optional direction filter (empty for all directions)
	//
	// Returns slice of pending SyncOperation objects.
	ListPendingOperations(workspace logicalcluster.Name, direction SyncDirection) []SyncOperation

	// GetMetrics returns current sync engine metrics including throughput,
	// success rates, and processing times.
	//
	// Returns SyncMetrics containing aggregated statistics.
	GetMetrics() SyncMetrics

	// IsHealthy performs a health check on the sync engine, verifying that
	// all components are operational and within acceptable performance bounds.
	//
	// Returns error if any health check fails.
	IsHealthy() error
}

// ResourceSyncHandler defines callbacks for handling different phases of resource synchronization.
// This allows external components to hook into the sync process for custom processing.
type ResourceSyncHandler interface {
	// OnBeforeSync is called before a sync operation begins.
	// It can be used for pre-processing or validation.
	//
	// Parameters:
	//   - ctx: Context for cancellation and deadlines
	//   - operation: The sync operation about to be processed
	//   - resource: The resource being synchronized
	//
	// Returns modified resource and error. If error is non-nil, sync is aborted.
	OnBeforeSync(ctx context.Context, operation SyncOperation, resource *unstructured.Unstructured) (*unstructured.Unstructured, error)

	// OnAfterSync is called after a sync operation completes successfully.
	// It can be used for post-processing or cleanup.
	//
	// Parameters:
	//   - ctx: Context for cancellation and deadlines
	//   - operation: The completed sync operation
	//   - resource: The synchronized resource
	//   - status: The sync status result
	OnAfterSync(ctx context.Context, operation SyncOperation, resource *unstructured.Unstructured, status SyncStatus)

	// OnSyncError is called when a sync operation encounters an error.
	// It can be used for error handling or recovery attempts.
	//
	// Parameters:
	//   - ctx: Context for cancellation and deadlines
	//   - operation: The failed sync operation
	//   - resource: The resource that failed to sync (may be nil)
	//   - err: The error that occurred
	//
	// Returns bool indicating whether to retry the operation.
	OnSyncError(ctx context.Context, operation SyncOperation, resource *unstructured.Unstructured, err error) bool
}
