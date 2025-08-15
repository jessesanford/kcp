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

// ConflictResolver handles conflicts that occur during resource synchronization
// between logical and physical clusters. It provides workspace-aware conflict
// resolution strategies for TMC's multi-placement scenarios.
type ConflictResolver interface {
	// ResolveConflict attempts to resolve a sync conflict using configured strategies.
	//
	// Parameters:
	//   - ctx: Context for cancellation and deadlines
	//   - conflict: The conflict to resolve
	//
	// Returns resolution result or error if resolution fails.
	ResolveConflict(ctx context.Context, conflict SyncConflict) (*ConflictResolution, error)

	// CanResolve determines if this resolver can handle the given conflict type.
	//
	// Parameters:
	//   - conflictType: The type of conflict
	//   - workspace: Workspace context
	//
	// Returns true if the conflict can be resolved.
	CanResolve(conflictType ConflictType, workspace logicalcluster.Name) bool

	// GetResolutionStrategy returns the strategy that would be used for a conflict.
	//
	// Parameters:
	//   - conflict: The conflict to analyze
	//
	// Returns the resolution strategy name.
	GetResolutionStrategy(conflict SyncConflict) string
}

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

// ConflictDetector identifies conflicts during sync operations.
type ConflictDetector interface {
	// DetectConflict checks for conflicts between source and target resources.
	//
	// Parameters:
	//   - ctx: Context for cancellation and deadlines
	//   - operation: The sync operation context
	//   - sourceResource: Resource from source cluster
	//   - targetResource: Resource from target cluster (may be nil)
	//
	// Returns SyncConflict if detected, nil otherwise.
	DetectConflict(ctx context.Context, operation SyncOperation, sourceResource, targetResource *unstructured.Unstructured) *SyncConflict

	// SupportedConflictTypes returns the types of conflicts this detector can identify.
	SupportedConflictTypes() []ConflictType
}