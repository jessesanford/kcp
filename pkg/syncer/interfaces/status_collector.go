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
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kcp-dev/logicalcluster/v3"
)

// StatusCollector collects and aggregates status information from sync operations
// across multiple clusters and workspaces. It provides workspace-aware status
// tracking for TMC's multi-placement synchronization scenarios.
type StatusCollector interface {
	// RecordSyncStatus records the status of a completed sync operation.
	//
	// Parameters:
	//   - ctx: Context for cancellation and deadlines
	//   - status: The sync status to record
	RecordSyncStatus(ctx context.Context, status SyncStatus) error

	// GetOperationStatus retrieves the latest status for a specific operation.
	//
	// Parameters:
	//   - operationID: Unique identifier of the sync operation
	//
	// Returns SyncStatus and bool indicating if found.
	GetOperationStatus(operationID string) (SyncStatus, bool)

	// GetWorkspaceMetrics returns aggregated metrics for a specific workspace.
	//
	// Parameters:
	//   - workspace: Logical cluster name
	//   - since: Optional time filter for metrics (nil for all time)
	//
	// Returns SyncMetrics for the workspace.
	GetWorkspaceMetrics(workspace logicalcluster.Name, since *time.Time) SyncMetrics

	// GetGlobalMetrics returns aggregated metrics across all workspaces.
	//
	// Parameters:
	//   - since: Optional time filter for metrics (nil for all time)
	//
	// Returns global SyncMetrics.
	GetGlobalMetrics(since *time.Time) SyncMetrics

	// ListFailedOperations returns operations that have failed and may need attention.
	//
	// Parameters:
	//   - workspace: Optional workspace filter (empty for all)
	//   - limit: Maximum number of results to return
	//
	// Returns slice of failed SyncStatus entries.
	ListFailedOperations(workspace logicalcluster.Name, limit int) []SyncStatus
}

// StatusReporter provides callbacks for external components to receive
// status updates about sync operations.
type StatusReporter interface {
	// OnStatusChange is called when a sync operation status changes.
	//
	// Parameters:
	//   - ctx: Context for cancellation and deadlines
	//   - oldStatus: Previous status (nil for new operations)
	//   - newStatus: Updated status
	OnStatusChange(ctx context.Context, oldStatus, newStatus *SyncStatus)

	// OnMetricsUpdate is called when aggregated metrics are updated.
	//
	// Parameters:
	//   - ctx: Context for cancellation and deadlines
	//   - workspace: Workspace the metrics apply to
	//   - metrics: Updated metrics
	OnMetricsUpdate(ctx context.Context, workspace logicalcluster.Name, metrics SyncMetrics)
}

// ResourceStatusCollector collects status from individual resources being synced.
type ResourceStatusCollector interface {
	// CollectResourceStatus extracts status information from a resource.
	//
	// Parameters:
	//   - ctx: Context for cancellation and deadlines
	//   - resource: Resource to collect status from
	//   - operation: The sync operation context
	//
	// Returns extracted status information or error.
	CollectResourceStatus(ctx context.Context, resource *unstructured.Unstructured, operation SyncOperation) (map[string]interface{}, error)

	// SupportsResource determines if this collector can handle the given resource type.
	//
	// Parameters:
	//   - resource: Resource to evaluate
	//
	// Returns true if the resource is supported.
	SupportsResource(resource *unstructured.Unstructured) bool
}