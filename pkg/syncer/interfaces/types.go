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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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