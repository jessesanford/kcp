// Package interfaces defines the core interfaces and contracts for virtual workspace operations.
// These interfaces establish the foundation for all virtual workspace implementations in KCP,
// providing clear contracts for provider implementations, resource discovery, and workspace management.
package interfaces

import (
	"context"
	"net/http"
)

// VirtualWorkspaceProvider is the main interface for virtual workspace operations.
// It defines the core contract that all virtual workspace implementations must satisfy,
// providing HTTP serving capabilities, workspace management, and lifecycle operations.
type VirtualWorkspaceProvider interface {
	// ServeHTTP handles incoming HTTP requests to the virtual workspace.
	// This method integrates with KCP's serving infrastructure to provide
	// RESTful API access to virtual workspace resources.
	ServeHTTP(w http.ResponseWriter, r *http.Request)

	// Initialize sets up the provider with necessary configurations.
	// This method is called during startup to configure the provider
	// with cluster information, API exports, and other required settings.
	Initialize(ctx context.Context, config *ProviderConfig) error

	// GetWorkspace retrieves a specific workspace configuration by name.
	// Returns the workspace definition including available resources and status.
	GetWorkspace(ctx context.Context, name string) (*VirtualWorkspace, error)

	// ListWorkspaces returns all available workspaces managed by this provider.
	// This enables workspace discovery and enumeration operations.
	ListWorkspaces(ctx context.Context) ([]*VirtualWorkspace, error)

	// Watch returns a channel for workspace events, enabling real-time
	// monitoring of workspace changes, additions, and deletions.
	Watch(ctx context.Context) (<-chan WorkspaceEvent, error)

	// Shutdown gracefully stops the provider, cleaning up resources
	// and ensuring orderly termination of ongoing operations.
	Shutdown(ctx context.Context) error
}

// ProviderConfig contains configuration for initializing a virtual workspace provider.
// This configuration establishes the provider's identity within KCP and defines
// its operational parameters for serving virtual workspaces.
type ProviderConfig struct {
	// ClusterName is the logical cluster this provider serves.
	// This establishes the provider's scope within KCP's multi-cluster architecture.
	ClusterName string

	// APIExportName references the APIExport being served by this provider.
	// This connects the virtual workspace to KCP's API export mechanism.
	APIExportName string

	// AuthConfig contains authentication configuration for the provider.
	// This defines how the provider handles authentication and user identity.
	AuthConfig *AuthConfig

	// DiscoveryConfig contains resource discovery configuration.
	// This configures how the provider discovers and exposes available resources.
	DiscoveryConfig *DiscoveryConfig
}

// VirtualWorkspace represents a single virtual workspace instance.
// It encapsulates the workspace identity, available resources, and current status.
type VirtualWorkspace struct {
	// Name is the unique identifier for this workspace within the provider.
	Name string

	// APIVersion specifies the API version for workspace compatibility.
	APIVersion string

	// Resources contains the list of resources available in this workspace.
	// This defines the API surface exposed by the workspace.
	Resources []ResourceInfo

	// Status indicates the current operational status of the workspace.
	Status WorkspaceStatus
}

// WorkspaceEvent represents changes to virtual workspaces.
// These events enable reactive programming patterns and real-time monitoring
// of workspace lifecycle changes.
type WorkspaceEvent struct {
	// Type categorizes the event (added, modified, deleted, error).
	Type EventType

	// Workspace contains the workspace data associated with this event.
	// May be nil for error events or deletion events.
	Workspace *VirtualWorkspace

	// Error contains any error information associated with the event.
	// Only populated for error-type events.
	Error error
}

// EventType categorizes workspace events for proper handling.
type EventType string

const (
	// EventTypeAdded indicates a new workspace has been created.
	EventTypeAdded EventType = "ADDED"
	// EventTypeModified indicates an existing workspace has been updated.
	EventTypeModified EventType = "MODIFIED"
	// EventTypeDeleted indicates a workspace has been removed.
	EventTypeDeleted EventType = "DELETED"
	// EventTypeError indicates an error occurred during workspace operations.
	EventTypeError EventType = "ERROR"
)

// WorkspaceStatus indicates the operational state of a virtual workspace.
type WorkspaceStatus string

const (
	// WorkspaceStatusReady indicates the workspace is fully operational.
	WorkspaceStatusReady WorkspaceStatus = "Ready"
	// WorkspaceStatusPending indicates the workspace is being initialized.
	WorkspaceStatusPending WorkspaceStatus = "Pending"
	// WorkspaceStatusError indicates the workspace has encountered an error.
	WorkspaceStatusError WorkspaceStatus = "Error"
)

// AuthConfig defines authentication configuration for virtual workspace providers.
// This configuration determines how the provider authenticates requests and
// establishes user identity within the workspace context.
type AuthConfig struct {
	// Enabled indicates whether authentication is required for this provider.
	Enabled bool

	// TokenValidationURL specifies the endpoint for validating authentication tokens.
	TokenValidationURL string

	// AllowAnonymous indicates whether anonymous access is permitted.
	AllowAnonymous bool
}

// DiscoveryConfig defines resource discovery configuration for virtual workspace providers.
// This configuration controls how the provider discovers and exposes available resources
// within the workspace context.
type DiscoveryConfig struct {
	// RefreshInterval specifies how often to refresh resource discovery information.
	RefreshInterval int64

	// CacheEnabled indicates whether discovery results should be cached.
	CacheEnabled bool

	// CacheTTL specifies the time-to-live for cached discovery information.
	CacheTTL int64
}