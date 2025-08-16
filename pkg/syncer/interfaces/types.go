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
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/kcp-dev/logicalcluster/v3"
)

// SyncDirection represents the direction of synchronization.
type SyncDirection string

const (
	// SyncDirectionUpstream indicates resources are being synced from physical cluster to logical cluster.
	SyncDirectionUpstream SyncDirection = "upstream"
	// SyncDirectionDownstream indicates resources are being synced from logical cluster to physical cluster.
	SyncDirectionDownstream SyncDirection = "downstream"
)

// SyncResult represents the outcome of a sync operation.
type SyncResult string

const (
	// SyncResultSuccess indicates the sync operation completed successfully.
	SyncResultSuccess SyncResult = "success"
	// SyncResultError indicates the sync operation encountered an error.
	SyncResultError SyncResult = "error"
	// SyncResultConflict indicates the sync operation encountered a conflict.
	SyncResultConflict SyncResult = "conflict"
	// SyncResultSkipped indicates the sync operation was skipped.
	SyncResultSkipped SyncResult = "skipped"
	// SyncResultRetry indicates the sync operation should be retried.
	SyncResultRetry SyncResult = "retry"
)

// ConflictType represents the type of conflict that occurred during sync.
type ConflictType string

const (
	// ConflictTypeResourceVersion indicates a conflict due to resource version mismatch.
	ConflictTypeResourceVersion ConflictType = "resource-version"
	// ConflictTypeOwnership indicates a conflict due to ownership issues.
	ConflictTypeOwnership ConflictType = "ownership"
	// ConflictTypeFieldManager indicates a conflict with field managers.
	ConflictTypeFieldManager ConflictType = "field-manager"
	// ConflictTypeAnnotation indicates a conflict with annotations.
	ConflictTypeAnnotation ConflictType = "annotation"
)

// SyncOperation represents a single sync operation to be performed.
type SyncOperation struct {
	// ID is a unique identifier for this sync operation.
	ID string
	// Direction indicates whether this is upstream or downstream sync.
	Direction SyncDirection
	// SourceCluster is the logical cluster where the resource originates.
	SourceCluster logicalcluster.Name
	// TargetCluster is the logical cluster where the resource should be synchronized.
	TargetCluster logicalcluster.Name
	// GVR is the Group/Version/Resource for the operation.
	GVR schema.GroupVersionResource
	// Namespace is the namespace of the resource (empty for cluster-scoped).
	Namespace string
	// Name is the name of the resource.
	Name string
	// Priority indicates the priority of this operation (higher values = higher priority).
	Priority int32
	// Timestamp is when this operation was queued.
	Timestamp metav1.Time
}

// SyncStatus represents the status of a sync operation.
type SyncStatus struct {
	// Operation is the sync operation this status refers to.
	Operation SyncOperation
	// Result is the outcome of the sync operation.
	Result SyncResult
	// Message provides additional details about the result.
	Message string
	// Error contains error details if Result is SyncResultError.
	Error error
	// ConflictType indicates the type of conflict if Result is SyncResultConflict.
	ConflictType ConflictType
	// RetryAfter indicates when to retry if Result is SyncResultRetry.
	RetryAfter *time.Duration
	// ProcessingTime is how long the operation took to process.
	ProcessingTime time.Duration
	// Timestamp is when this status was recorded.
	Timestamp metav1.Time
}

// SyncMetrics contains metrics about sync operations.
type SyncMetrics struct {
	// TotalOperations is the total number of sync operations processed.
	TotalOperations int64
	// SuccessfulOperations is the number of successful sync operations.
	SuccessfulOperations int64
	// FailedOperations is the number of failed sync operations.
	FailedOperations int64
	// ConflictedOperations is the number of conflicted sync operations.
	ConflictedOperations int64
	// AverageProcessingTime is the average time to process sync operations.
	AverageProcessingTime time.Duration
	// LastSyncTime is the timestamp of the last sync operation.
	LastSyncTime metav1.Time
}

// Common annotations used by the syncer for resource tracking and management.
const (
	// SyncSourceAnnotation indicates the source cluster for a synced resource.
	SyncSourceAnnotation = "tmc.kcp.io/sync-source"

	// WorkspaceOriginAnnotation indicates the workspace a resource originated from.
	WorkspaceOriginAnnotation = "tmc.kcp.io/workspace-origin"

	// PlacementAnnotation indicates which placement policy selected this resource.
	PlacementAnnotation = "tmc.kcp.io/placement"

	// SyncTargetAnnotation indicates the target cluster for synchronization.
	SyncTargetAnnotation = "tmc.kcp.io/sync-target"

	// SyncGenerationAnnotation tracks the generation of the synced resource.
	SyncGenerationAnnotation = "tmc.kcp.io/sync-generation"

	// SyncTimestampAnnotation records when the resource was last synchronized.
	SyncTimestampAnnotation = "tmc.kcp.io/sync-timestamp"

	// ConflictResolutionAnnotation indicates how conflicts were resolved.
	ConflictResolutionAnnotation = "tmc.kcp.io/conflict-resolution"
)

// Common labels used by the syncer.
const (
	// SyncerManagedLabel indicates a resource is managed by the syncer.
	SyncerManagedLabel = "tmc.kcp.io/syncer-managed"

	// WorkspaceLabel indicates the workspace a resource belongs to.
	WorkspaceLabel = "tmc.kcp.io/workspace"

	// PlacementLabel indicates the placement that selected this resource.
	PlacementLabel = "tmc.kcp.io/placement"
)

// Feature gates for syncer functionality.
const (
	// FeatureGateTMCSyncer enables the TMC syncer functionality.
	FeatureGateTMCSyncer = "TMCSyncer"

	// FeatureGateAdvancedConflictResolution enables advanced conflict resolution strategies.
	FeatureGateAdvancedConflictResolution = "TMCAdvancedConflictResolution"

	// FeatureGateStatusAggregation enables status aggregation across placements.
	FeatureGateStatusAggregation = "TMCStatusAggregation"
)

// SyncConflict represents a conflict that occurred during synchronization.
type SyncConflict struct {
	// Operation is the sync operation that encountered the conflict.
	Operation SyncOperation
	// ConflictType indicates the type of conflict.
	ConflictType ConflictType
	// SourceResource is the resource from the source cluster.
	SourceResource *unstructured.Unstructured
	// TargetResource is the conflicting resource from the target cluster.
	TargetResource *unstructured.Unstructured
	// ConflictDetails provides additional information about the conflict.
	ConflictDetails map[string]interface{}
	// DetectedAt is when the conflict was first detected.
	DetectedAt time.Time
}

// ConflictResolution represents the result of conflict resolution.
type ConflictResolution struct {
	// Resolved indicates whether the conflict was successfully resolved.
	Resolved bool
	// Resolution is the resolved resource (if applicable).
	Resolution *unstructured.Unstructured
	// Strategy is the name of the strategy used for resolution.
	Strategy string
	// Message provides details about the resolution process.
	Message string
	// Retry indicates whether the operation should be retried.
	Retry bool
	// RetryAfter specifies when to retry (if Retry is true).
	RetryAfter *time.Duration
}

// TransformationContext provides additional context for resource transformations.
type TransformationContext struct {
	// SourceWorkspace is the workspace the resource is coming from.
	SourceWorkspace logicalcluster.Name

	// TargetWorkspace is the workspace the resource is going to.
	TargetWorkspace logicalcluster.Name

	// Direction indicates the direction of synchronization.
	Direction SyncDirection

	// PlacementName is the name of the placement that triggered this sync.
	PlacementName string

	// SyncTargetName is the name of the sync target for physical cluster operations.
	SyncTargetName string

	// Annotations contains additional metadata for the transformation.
	Annotations map[string]string
}

// SyncEngineConfig contains configuration options for creating a sync engine instance.
type SyncEngineConfig struct {
	// WorkerCount is the number of worker goroutines to process sync operations.
	WorkerCount int

	// QueueDepth is the maximum number of operations that can be queued.
	QueueDepth int

	// Workspace is the logical cluster name this engine is associated with.
	Workspace logicalcluster.Name

	// ResourceTransformer handles resource transformations during sync.
	ResourceTransformer ResourceTransformer

	// StatusCollector collects and reports sync operation status.
	StatusCollector StatusCollector

	// ConflictResolver resolves conflicts during sync operations.
	ConflictResolver ConflictResolver

	// SupportedGVRs is the list of Group/Version/Resource types this engine can sync.
	// If empty, all resource types are supported.
	SupportedGVRs []schema.GroupVersionResource
}

// Validate validates the SyncOperation.
func (o *SyncOperation) Validate() error {
	if o.ID == "" {
		return fmt.Errorf("operation ID is required")
	}
	if o.Direction != SyncDirectionUpstream && o.Direction != SyncDirectionDownstream {
		return fmt.Errorf("invalid sync direction: %s", o.Direction)
	}
	if o.SourceCluster.Empty() {
		return fmt.Errorf("source cluster is required")
	}
	if o.TargetCluster.Empty() {
		return fmt.Errorf("target cluster is required")
	}
	if o.GVR.Resource == "" {
		return fmt.Errorf("resource is required in GVR")
	}
	if o.Name == "" {
		return fmt.Errorf("resource name is required")
	}
	return nil
}

// String returns a string representation of the SyncOperation.
func (o *SyncOperation) String() string {
	return fmt.Sprintf("SyncOperation{ID: %s, Direction: %s, %s/%s, %s -> %s}",
		o.ID, o.Direction, o.Namespace, o.Name, o.SourceCluster, o.TargetCluster)
}

// String returns a string representation of the SyncStatus.
func (s *SyncStatus) String() string {
	return fmt.Sprintf("SyncStatus{Operation: %s, Result: %s, Message: %s}",
		s.Operation.ID, s.Result, s.Message)
}
