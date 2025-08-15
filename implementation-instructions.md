# Implementation Instructions: API Contracts & Core Interfaces

## Overview
- **Branch**: feature/tmc-phase4-vw-01-api-contracts
- **Purpose**: Define all core interfaces and contracts for the virtual workspace system, establishing the foundation for all future implementations
- **Target Lines**: 350
- **Dependencies**: None (Foundation branch)
- **Estimated Time**: 2 days

## Files to Create

### 1. pkg/virtual/interfaces/provider.go (80 lines)
**Purpose**: Define the main VirtualWorkspaceProvider interface that all implementations must satisfy

**Interfaces/Types to Define**:
```go
package interfaces

import (
    "context"
    "net/http"
    
    "k8s.io/apimachinery/pkg/runtime"
    "github.com/kcp-dev/kcp/sdk/apis/core"
)

// VirtualWorkspaceProvider is the main interface for virtual workspace operations
type VirtualWorkspaceProvider interface {
    // ServeHTTP handles incoming requests to the virtual workspace
    ServeHTTP(w http.ResponseWriter, r *http.Request)
    
    // Initialize sets up the provider with necessary configurations
    Initialize(ctx context.Context, config *ProviderConfig) error
    
    // GetWorkspace retrieves a specific workspace configuration
    GetWorkspace(ctx context.Context, name string) (*VirtualWorkspace, error)
    
    // ListWorkspaces returns all available workspaces
    ListWorkspaces(ctx context.Context) ([]*VirtualWorkspace, error)
    
    // Watch returns a channel for workspace events
    Watch(ctx context.Context) (<-chan WorkspaceEvent, error)
    
    // Shutdown gracefully stops the provider
    Shutdown(ctx context.Context) error
}

// ProviderConfig contains configuration for initializing a provider
type ProviderConfig struct {
    // ClusterName is the logical cluster this provider serves
    ClusterName string
    
    // APIExportName references the APIExport being served
    APIExportName string
    
    // AuthConfig contains authentication configuration
    AuthConfig *AuthConfig
    
    // DiscoveryConfig contains resource discovery configuration
    DiscoveryConfig *DiscoveryConfig
}

// VirtualWorkspace represents a single virtual workspace
type VirtualWorkspace struct {
    // Name is the unique identifier for this workspace
    Name string
    
    // APIVersion for versioning
    APIVersion string
    
    // Resources available in this workspace
    Resources []ResourceInfo
    
    // Status of the workspace
    Status WorkspaceStatus
}

// WorkspaceEvent represents changes to workspaces
type WorkspaceEvent struct {
    Type      EventType
    Workspace *VirtualWorkspace
    Error     error
}

type EventType string

const (
    EventTypeAdded   EventType = "ADDED"
    EventTypeModified EventType = "MODIFIED"
    EventTypeDeleted  EventType = "DELETED"
    EventTypeError    EventType = "ERROR"
)

type WorkspaceStatus string

const (
    WorkspaceStatusReady    WorkspaceStatus = "Ready"
    WorkspaceStatusPending  WorkspaceStatus = "Pending"
    WorkspaceStatusError    WorkspaceStatus = "Error"
)
```

### 2. pkg/virtual/interfaces/discovery.go (60 lines)
**Purpose**: Define resource discovery interfaces for virtual workspaces

**Interfaces/Types to Define**:
```go
package interfaces

import (
    "context"
    
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/runtime/schema"
)

// ResourceDiscoveryInterface handles resource discovery within virtual workspaces
type ResourceDiscoveryInterface interface {
    // Discover returns available resources in a workspace
    Discover(ctx context.Context, workspace string) ([]ResourceInfo, error)
    
    // GetOpenAPISchema returns the OpenAPI schema for resources
    GetOpenAPISchema(ctx context.Context, workspace string) ([]byte, error)
    
    // Watch monitors for resource changes
    Watch(ctx context.Context, workspace string) (<-chan DiscoveryEvent, error)
    
    // IsResourceAvailable checks if a specific resource is available
    IsResourceAvailable(ctx context.Context, workspace string, gvr schema.GroupVersionResource) (bool, error)
}

// ResourceInfo describes an available resource
type ResourceInfo struct {
    metav1.APIResource
    
    // WorkspaceScoped indicates if this resource is workspace-scoped
    WorkspaceScoped bool
    
    // Schema contains OpenAPI schema for this resource
    Schema []byte
}

// DiscoveryEvent represents changes in available resources
type DiscoveryEvent struct {
    Type     EventType
    Resource ResourceInfo
    Error    error
}

// DiscoveryCache provides caching for discovery information
type DiscoveryCache interface {
    // GetResources retrieves cached resources for a workspace
    GetResources(workspace string) ([]ResourceInfo, bool)
    
    // SetResources caches resources for a workspace
    SetResources(workspace string, resources []ResourceInfo, ttl int64)
    
    // InvalidateWorkspace removes cached data for a workspace
    InvalidateWorkspace(workspace string)
    
    // Clear removes all cached data
    Clear()
}
```

### 3. pkg/virtual/interfaces/authorization.go (50 lines)
**Purpose**: Define authorization interfaces for virtual workspace access control

**Interfaces/Types to Define**:
```go
package interfaces

import (
    "context"
    
    authv1 "k8s.io/api/authorization/v1"
    "k8s.io/apimachinery/pkg/runtime/schema"
)

// AuthorizationProvider handles authorization for virtual workspace operations
type AuthorizationProvider interface {
    // Authorize determines if a request is allowed
    Authorize(ctx context.Context, req *AuthorizationRequest) (*AuthorizationDecision, error)
    
    // GetPermissions returns permissions for a user in a workspace
    GetPermissions(ctx context.Context, workspace, user string) ([]Permission, error)
    
    // RefreshCache updates cached authorization data
    RefreshCache(ctx context.Context, workspace string) error
}

// AuthorizationRequest contains details for an authorization check
type AuthorizationRequest struct {
    // User making the request
    User string
    
    // Groups the user belongs to
    Groups []string
    
    // Workspace being accessed
    Workspace string
    
    // Resource being accessed
    Resource schema.GroupVersionResource
    
    // Verb being performed
    Verb string
    
    // ResourceName if accessing a specific resource
    ResourceName string
}

// AuthorizationDecision contains the authorization result
type AuthorizationDecision struct {
    // Allowed indicates if the request is authorized
    Allowed bool
    
    // Reason provides explanation for the decision
    Reason string
    
    // EvaluationError contains any error during evaluation
    EvaluationError error
}

// Permission represents a granted permission
type Permission struct {
    Resource schema.GroupVersionResource
    Verbs    []string
    ResourceNames []string
}
```

### 4. pkg/virtual/interfaces/storage.go (40 lines)
**Purpose**: Define storage interfaces for virtual workspace persistence

**Interfaces/Types to Define**:
```go
package interfaces

import (
    "context"
    
    "k8s.io/apimachinery/pkg/runtime"
)

// StorageInterface provides persistence for virtual workspace data
type StorageInterface interface {
    // Get retrieves an object from storage
    Get(ctx context.Context, key string, obj runtime.Object) error
    
    // Create stores a new object
    Create(ctx context.Context, key string, obj runtime.Object) error
    
    // Update modifies an existing object
    Update(ctx context.Context, key string, obj runtime.Object) error
    
    // Delete removes an object from storage
    Delete(ctx context.Context, key string) error
    
    // List returns objects matching the provided options
    List(ctx context.Context, opts ListOptions) ([]runtime.Object, error)
    
    // Watch monitors for changes to objects
    Watch(ctx context.Context, opts WatchOptions) (<-chan WatchEvent, error)
}

// ListOptions configures a list operation
type ListOptions struct {
    Prefix string
    Limit  int
    Continue string
}

// WatchOptions configures a watch operation
type WatchOptions struct {
    Prefix string
    ResourceVersion string
}

// WatchEvent represents a change to stored data
type WatchEvent struct {
    Type   EventType
    Object runtime.Object
}
```

### 5. pkg/virtual/contracts/request.go (40 lines)
**Purpose**: Define request contracts for virtual workspace operations

**Interfaces/Types to Define**:
```go
package contracts

import (
    "net/http"
    
    "k8s.io/apimachinery/pkg/runtime/schema"
)

// VirtualWorkspaceRequest wraps an HTTP request with workspace context
type VirtualWorkspaceRequest struct {
    // Original HTTP request
    *http.Request
    
    // Workspace being accessed
    Workspace string
    
    // Resource being accessed
    Resource schema.GroupVersionResource
    
    // Action being performed
    Action string
    
    // User information
    User UserInfo
}

// UserInfo contains authenticated user information
type UserInfo struct {
    // Username of the authenticated user
    Username string
    
    // Groups the user belongs to
    Groups []string
    
    // Extra information from authentication
    Extra map[string][]string
}

// RequestContext provides additional request context
type RequestContext struct {
    // RequestID for tracking
    RequestID string
    
    // CorrelationID for distributed tracing
    CorrelationID string
}
```

### 6. pkg/virtual/contracts/response.go (40 lines)
**Purpose**: Define response contracts for virtual workspace operations

**Interfaces/Types to Define**:
```go
package contracts

import (
    "k8s.io/apimachinery/pkg/runtime"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// VirtualWorkspaceResponse represents a response from virtual workspace operations
type VirtualWorkspaceResponse struct {
    // Object being returned
    Object runtime.Object
    
    // Status of the operation
    Status ResponseStatus
    
    // Metadata about the response
    Metadata ResponseMetadata
}

// ResponseStatus indicates the result of an operation
type ResponseStatus string

const (
    ResponseStatusSuccess ResponseStatus = "Success"
    ResponseStatusPartial ResponseStatus = "Partial"
    ResponseStatusError   ResponseStatus = "Error"
)

// ResponseMetadata contains metadata about the response
type ResponseMetadata struct {
    // ResourceVersion for optimistic concurrency
    ResourceVersion string
    
    // Continue token for pagination
    Continue string
    
    // RemainingItemCount for pagination
    RemainingItemCount *int64
    
    // Warnings encountered during processing
    Warnings []string
}

// ErrorResponse represents an error response
type ErrorResponse struct {
    metav1.Status
    Details string
}
```

### 7. pkg/virtual/contracts/errors.go (40 lines)
**Purpose**: Define error types and contracts for virtual workspace operations

**Interfaces/Types to Define**:
```go
package contracts

import (
    "fmt"
    
    "k8s.io/apimachinery/pkg/api/errors"
)

// VirtualWorkspaceError represents errors in virtual workspace operations
type VirtualWorkspaceError struct {
    // Type categorizes the error
    Type ErrorType
    
    // Message provides human-readable description
    Message string
    
    // Workspace where error occurred
    Workspace string
    
    // Cause is the underlying error if any
    Cause error
}

// ErrorType categorizes virtual workspace errors
type ErrorType string

const (
    ErrorTypeNotFound      ErrorType = "NotFound"
    ErrorTypeUnauthorized  ErrorType = "Unauthorized"
    ErrorTypeInvalid       ErrorType = "Invalid"
    ErrorTypeConflict      ErrorType = "Conflict"
    ErrorTypeInternal      ErrorType = "Internal"
    ErrorTypeTimeout       ErrorType = "Timeout"
)

// Error implements the error interface
func (e *VirtualWorkspaceError) Error() string {
    if e.Cause != nil {
        return fmt.Sprintf("%s: %s (workspace: %s): %v", e.Type, e.Message, e.Workspace, e.Cause)
    }
    return fmt.Sprintf("%s: %s (workspace: %s)", e.Type, e.Message, e.Workspace)
}

// IsRetryable indicates if the error is transient
func (e *VirtualWorkspaceError) IsRetryable() bool {
    return e.Type == ErrorTypeTimeout || e.Type == ErrorTypeInternal
}
```

## Implementation Steps

1. **Create package structure**:
   - Create `pkg/virtual/interfaces/` directory
   - Create `pkg/virtual/contracts/` directory

2. **Implement core interfaces**:
   - Start with provider.go to establish main contract
   - Add discovery.go for resource discovery
   - Add authorization.go for access control
   - Add storage.go for persistence

3. **Define request/response contracts**:
   - Implement request.go for request handling
   - Implement response.go for response formatting
   - Add errors.go for error handling

4. **Add documentation**:
   - Comprehensive godoc for all interfaces
   - Example usage in comments
   - Design rationale documentation

## Testing Requirements
- Unit test coverage: N/A (interfaces only)
- Test scenarios:
  - Compilation tests to ensure interfaces are valid
  - Mock implementations for testing other components
  - Example usage tests

## Integration Points
- Uses: Standard Kubernetes API types
- Provides: Foundation interfaces for all virtual workspace implementations

## Acceptance Criteria
- [ ] All interfaces defined with clear contracts
- [ ] Comprehensive godoc comments on all types and methods
- [ ] No implementation code (interfaces only)
- [ ] Follows KCP patterns and conventions
- [ ] Compiles without errors
- [ ] Mock implementations provided for testing
- [ ] No linting errors

## Common Pitfalls
- **Don't include implementations**: This branch is interfaces only
- **Avoid circular dependencies**: Keep interfaces minimal and focused
- **Don't over-specify**: Allow flexibility for different implementations
- **Follow KCP patterns**: Study existing KCP interfaces for consistency
- **Keep interfaces small**: Prefer multiple focused interfaces over large ones
- **Use standard Kubernetes types**: Leverage existing API machinery types

## Code Review Focus
- Interface design and extensibility
- Consistency with KCP patterns
- Clear separation of concerns
- Future-proofing for extensions
- Documentation completeness