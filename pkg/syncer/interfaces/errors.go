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

	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/kcp-dev/logicalcluster/v3"
)

// SyncError represents an error that occurred during synchronization.
type SyncError struct {
	Operation SyncOperation
	Cause     error
	Message   string
}

func (e *SyncError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("sync error for %s/%s in %s: %s: %v",
			e.Operation.Namespace, e.Operation.Name,
			e.Operation.SourceCluster, e.Message, e.Cause)
	}
	return fmt.Sprintf("sync error for %s/%s in %s: %s",
		e.Operation.Namespace, e.Operation.Name,
		e.Operation.SourceCluster, e.Message)
}

func (e *SyncError) Unwrap() error {
	return e.Cause
}

// TransformationError represents an error during resource transformation.
type TransformationError struct {
	GVR       schema.GroupVersionResource
	Namespace string
	Name      string
	Direction SyncDirection
	Workspace logicalcluster.Name
	Cause     error
}

func (e *TransformationError) Error() string {
	return fmt.Sprintf("transformation error for %s %s/%s (direction: %s, workspace: %s): %v",
		e.GVR.String(), e.Namespace, e.Name, e.Direction, e.Workspace, e.Cause)
}

func (e *TransformationError) Unwrap() error {
	return e.Cause
}

// ConflictError represents a conflict that could not be resolved.
type ConflictError struct {
	Conflict   SyncConflict
	Resolution *ConflictResolution
	Cause      error
}

func (e *ConflictError) Error() string {
	if e.Resolution != nil {
		return fmt.Sprintf("conflict resolution failed (type: %s, strategy: %s): %v",
			e.Conflict.ConflictType, e.Resolution.Strategy, e.Cause)
	}
	return fmt.Sprintf("conflict detected (type: %s): %v",
		e.Conflict.ConflictType, e.Cause)
}

func (e *ConflictError) Unwrap() error {
	return e.Cause
}

// Common error values.
var (
	// ErrSyncEngineStopped indicates the sync engine has been stopped.
	ErrSyncEngineStopped = fmt.Errorf("sync engine stopped")

	// ErrQueueFull indicates the sync queue is full and cannot accept more operations.
	ErrQueueFull = fmt.Errorf("sync queue full")

	// ErrOperationNotFound indicates the requested operation was not found.
	ErrOperationNotFound = fmt.Errorf("operation not found")

	// ErrInvalidDirection indicates an invalid sync direction was specified.
	ErrInvalidDirection = fmt.Errorf("invalid sync direction")

	// ErrWorkspaceNotFound indicates the specified workspace does not exist.
	ErrWorkspaceNotFound = fmt.Errorf("workspace not found")

	// ErrTransformationFailed indicates resource transformation failed.
	ErrTransformationFailed = fmt.Errorf("transformation failed")

	// ErrConflictUnresolvable indicates a conflict could not be resolved.
	ErrConflictUnresolvable = fmt.Errorf("conflict unresolvable")

	// ErrInvalidResource indicates the resource is invalid for synchronization.
	ErrInvalidResource = fmt.Errorf("invalid resource")
)

// IsRetryableError determines if an error should trigger a retry.
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check for specific non-retryable errors
	switch err {
	case ErrSyncEngineStopped, ErrInvalidDirection, ErrInvalidResource:
		return false
	}

	// Check for conflict errors - some may be retryable
	if conflictErr, ok := err.(*ConflictError); ok {
		return conflictErr.Conflict.ConflictType == ConflictTypeResourceVersion
	}

	// Default to retryable for unknown errors
	return true
}
