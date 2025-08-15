package interfaces

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ResourceDiscoveryInterface handles resource discovery within virtual workspaces.
// It provides capabilities for discovering available resources, monitoring resource changes,
// and managing discovery caching for optimal performance.
type ResourceDiscoveryInterface interface {
	// Discover returns all available resources in the specified workspace.
	// This method enables clients to understand what resources are accessible
	// within a particular virtual workspace context.
	Discover(ctx context.Context, workspace string) ([]ResourceInfo, error)

	// GetOpenAPISchema returns the OpenAPI schema for resources in the workspace.
	// This provides detailed schema information for client code generation
	// and validation purposes.
	GetOpenAPISchema(ctx context.Context, workspace string) ([]byte, error)

	// Watch monitors for resource changes within a workspace, returning
	// a channel of discovery events for real-time resource updates.
	Watch(ctx context.Context, workspace string) (<-chan DiscoveryEvent, error)

	// IsResourceAvailable checks if a specific resource is available in the workspace.
	// This enables efficient resource validation before attempting operations.
	IsResourceAvailable(ctx context.Context, workspace string, gvr schema.GroupVersionResource) (bool, error)
}

// ResourceInfo describes an available resource within a virtual workspace.
// It extends the standard Kubernetes APIResource with virtual workspace-specific
// metadata and schema information.
type ResourceInfo struct {
	// Embed standard Kubernetes APIResource for compatibility
	metav1.APIResource

	// WorkspaceScoped indicates if this resource is scoped to a workspace.
	// Workspace-scoped resources are isolated within their workspace context.
	WorkspaceScoped bool

	// Schema contains the OpenAPI schema definition for this resource.
	// This enables client validation and code generation.
	Schema []byte
}

// DiscoveryEvent represents changes in available resources within a workspace.
// These events enable reactive discovery updates and resource monitoring.
type DiscoveryEvent struct {
	// Type categorizes the discovery event (added, modified, deleted, error).
	Type EventType

	// Resource contains the resource information associated with this event.
	Resource ResourceInfo

	// Error contains any error information for error-type events.
	Error error
}

// DiscoveryCache provides caching capabilities for discovery information.
// This interface enables efficient caching of expensive discovery operations
// to improve performance and reduce load on underlying systems.
type DiscoveryCache interface {
	// GetResources retrieves cached resources for a workspace.
	// Returns the resources and a boolean indicating if they were found in cache.
	GetResources(workspace string) ([]ResourceInfo, bool)

	// SetResources caches resources for a workspace with the specified TTL.
	// The TTL (time-to-live) is specified in seconds.
	SetResources(workspace string, resources []ResourceInfo, ttl int64)

	// InvalidateWorkspace removes all cached data for a specific workspace.
	// This is useful when workspace configuration changes.
	InvalidateWorkspace(workspace string)

	// Clear removes all cached discovery data across all workspaces.
	// This provides a mechanism for complete cache reset.
	Clear()
}