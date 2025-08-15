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

package workspace

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/kcp-dev/kcp/pkg/authorization"
	"github.com/kcp-dev/logicalcluster/v3"
)

// WorkspaceProvider defines the core interface for virtual workspace management in KCP.
// Implementations handle workspace lifecycle, resource projection, and access control
// while maintaining compatibility with KCP's logical cluster architecture.
//
// Key Responsibilities:
// - Workspace discovery and enumeration
// - Client creation with proper workspace scoping
// - Authentication and authorization integration
// - Resource lifecycle management
// - Performance optimization through caching
//
// Thread Safety:
// All methods must be safe for concurrent use across multiple goroutines.
// Implementations should use appropriate synchronization mechanisms.
//
// Error Handling:
// Methods should return structured errors with sufficient context for
// debugging and user feedback. Transient errors should be distinguishable
// from permanent failures.
type WorkspaceProvider interface {
	// GetWorkspace returns a workspace client for the specified workspace reference.
	// The client provides scoped access to resources within that workspace.
	//
	// Parameters:
	//   - ctx: Context for cancellation and deadline control
	//   - ref: Unique workspace identifier with logical cluster information
	//
	// Returns:
	//   - WorkspaceClient: Scoped client for the workspace
	//   - error: Details about any access or configuration issues
	//
	// Error Conditions:
	//   - WorkspaceNotFoundError: Workspace does not exist
	//   - AccessDeniedError: Insufficient permissions
	//   - ConfigurationError: Invalid workspace state
	GetWorkspace(ctx context.Context, ref WorkspaceReference) (WorkspaceClient, error)

	// ListWorkspaces returns information about accessible workspaces.
	// Results are filtered based on the caller's authorization context.
	//
	// Parameters:
	//   - ctx: Context with authentication and authorization details
	//   - opts: Filtering and pagination options
	//
	// Returns:
	//   - []WorkspaceInfo: Metadata for accessible workspaces
	//   - error: Issues with workspace enumeration
	//
	// Behavior:
	//   - Returns only workspaces the caller can access
	//   - Respects pagination limits and filters
	//   - May return cached results for performance
	ListWorkspaces(ctx context.Context, opts WorkspaceListOptions) ([]WorkspaceInfo, error)

	// WatchWorkspaces provides real-time updates for workspace changes.
	// Essential for controllers and UIs that need to react to workspace lifecycle events.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout control
	//   - opts: Filtering and watch configuration options
	//
	// Returns:
	//   - watch.Interface: Stream of workspace change events
	//   - error: Issues with establishing the watch
	//
	// Events:
	//   - ADDED: New workspace created
	//   - MODIFIED: Workspace metadata or state changed
	//   - DELETED: Workspace removed
	WatchWorkspaces(ctx context.Context, opts WorkspaceWatchOptions) (watch.Interface, error)

	// CreateWorkspace provisions a new virtual workspace with the specified configuration.
	// The workspace becomes available for resource operations once creation completes.
	//
	// Parameters:
	//   - ctx: Context with authentication for ownership assignment
	//   - ref: Desired workspace identifier and configuration
	//   - opts: Additional creation options and policies
	//
	// Returns:
	//   - WorkspaceInfo: Metadata for the created workspace
	//   - error: Issues with workspace provisioning
	//
	// Behavior:
	//   - Validates workspace name uniqueness
	//   - Applies appropriate security policies
	//   - Initializes workspace-specific resources
	CreateWorkspace(ctx context.Context, ref WorkspaceReference, opts WorkspaceCreateOptions) (*WorkspaceInfo, error)

	// DeleteWorkspace removes a virtual workspace and all its resources.
	// This is a destructive operation that cannot be undone.
	//
	// Parameters:
	//   - ctx: Context with authentication for authorization
	//   - ref: Workspace to be deleted
	//   - opts: Deletion options and cleanup policies
	//
	// Returns:
	//   - error: Issues with workspace deletion
	//
	// Behavior:
	//   - Verifies deletion permissions
	//   - Cleanly removes all workspace resources
	//   - Updates related metadata and caches
	DeleteWorkspace(ctx context.Context, ref WorkspaceReference, opts WorkspaceDeleteOptions) error

	// UpdateWorkspace modifies workspace metadata and configuration.
	// Changes take effect immediately but may require time to propagate.
	//
	// Parameters:
	//   - ctx: Context with authentication for authorization
	//   - info: Updated workspace information
	//   - opts: Update options and conflict resolution
	//
	// Returns:
	//   - *WorkspaceInfo: Updated workspace metadata
	//   - error: Issues with workspace modification
	//
	// Behavior:
	//   - Validates changes against current state
	//   - Applies updates atomically where possible
	//   - Handles concurrent modification conflicts
	UpdateWorkspace(ctx context.Context, info *WorkspaceInfo, opts WorkspaceUpdateOptions) (*WorkspaceInfo, error)

	// CheckAccess verifies whether the current context can perform specific operations
	// on a workspace. Used for pre-flight authorization checks.
	//
	// Parameters:
	//   - ctx: Context with authentication information
	//   - ref: Target workspace for the access check
	//   - verb: Operation to be performed (e.g., "get", "create", "delete")
	//   - resource: Optional specific resource type
	//
	// Returns:
	//   - bool: Whether access is permitted
	//   - error: Issues with the access check itself
	//
	// Usage:
	//   This method enables UI and API layers to provide better user experience
	//   by showing only accessible operations.
	CheckAccess(ctx context.Context, ref WorkspaceReference, verb string, resource string) (bool, error)

	// GetWorkspaceInfo retrieves detailed metadata about a specific workspace
	// without creating a client connection. Useful for status checks and introspection.
	//
	// Parameters:
	//   - ctx: Context for authentication and cancellation
	//   - ref: Workspace to inspect
	//
	// Returns:
	//   - *WorkspaceInfo: Current workspace metadata
	//   - error: Issues with workspace lookup
	//
	// Behavior:
	//   - Returns current state and metadata
	//   - May use cached information for performance
	//   - Respects access control policies
	GetWorkspaceInfo(ctx context.Context, ref WorkspaceReference) (*WorkspaceInfo, error)
}

// WorkspaceListOptions configures workspace enumeration operations.
// Provides filtering, pagination, and performance tuning capabilities.
type WorkspaceListOptions struct {
	// LabelSelector filters workspaces based on label matching.
	// Uses standard Kubernetes selector syntax.
	LabelSelector string `json:"labelSelector,omitempty"`

	// TypeFilter restricts results to specific workspace types.
	// Empty slice means all types are included.
	TypeFilter []WorkspaceType `json:"typeFilter,omitempty"`

	// LogicalCluster constrains results to a specific logical cluster.
	// Empty value means all accessible clusters are included.
	LogicalCluster logicalcluster.Name `json:"logicalCluster,omitempty"`

	// Limit restricts the number of results returned.
	// Used for pagination and resource management.
	Limit int `json:"limit,omitempty"`

	// Continue token for pagination through large result sets.
	// Obtained from previous list operations.
	Continue string `json:"continue,omitempty"`

	// IncludeInactive determines whether suspended or terminating
	// workspaces are included in results.
	IncludeInactive bool `json:"includeInactive,omitempty"`
}

// WorkspaceWatchOptions configures workspace change monitoring.
// Enables efficient real-time updates for workspace state changes.
type WorkspaceWatchOptions struct {
	// ResourceVersion specifies the starting point for the watch.
	// Only changes after this version will be reported.
	ResourceVersion string `json:"resourceVersion,omitempty"`

	// LabelSelector filters watched workspaces based on labels.
	LabelSelector string `json:"labelSelector,omitempty"`

	// TypeFilter limits watch events to specific workspace types.
	TypeFilter []WorkspaceType `json:"typeFilter,omitempty"`

	// TimeoutSeconds specifies how long the watch should remain open.
	// The watch will close after this duration even if no events occur.
	TimeoutSeconds *int64 `json:"timeoutSeconds,omitempty"`
}

// WorkspaceCreateOptions configures new workspace provisioning.
// Controls initialization behavior and applied policies.
type WorkspaceCreateOptions struct {
	// InitialResourceQuotas override default resource limits.
	// Applied immediately upon workspace creation.
	InitialResourceQuotas map[string]string `json:"initialResourceQuotas,omitempty"`

	// AccessPolicy defines custom authorization rules.
	// If not specified, defaults based on workspace type are applied.
	AccessPolicy *authorization.WorkspaceAccessPolicy `json:"accessPolicy,omitempty"`

	// DryRun simulates workspace creation without making changes.
	// Used for validation and cost estimation.
	DryRun bool `json:"dryRun,omitempty"`

	// WaitForReady blocks until the workspace is fully initialized.
	// Guarantees immediate usability but increases latency.
	WaitForReady bool `json:"waitForReady,omitempty"`
}

// WorkspaceDeleteOptions configures workspace removal behavior.
// Controls cleanup policies and data preservation.
type WorkspaceDeleteOptions struct {
	// GracePeriodSeconds specifies how long to wait for graceful termination.
	// Resources are force-deleted after this period expires.
	GracePeriodSeconds *int64 `json:"gracePeriodSeconds,omitempty"`

	// PreserveData indicates whether to keep workspace data for recovery.
	// May not be supported by all implementations.
	PreserveData bool `json:"preserveData,omitempty"`

	// DryRun simulates deletion without making changes.
	// Used for impact analysis and validation.
	DryRun bool `json:"dryRun,omitempty"`
}

// WorkspaceUpdateOptions configures workspace modification operations.
// Handles conflict resolution and update validation.
type WorkspaceUpdateOptions struct {
	// DryRun simulates updates without making changes.
	// Used for validation and preview.
	DryRun bool `json:"dryRun,omitempty"`

	// Force bypasses certain safety checks for emergency situations.
	// Should be used with extreme caution.
	Force bool `json:"force,omitempty"`
}