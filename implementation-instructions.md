# Implementation Instructions: Basic Virtual Workspace Provider

## Overview
- **Branch**: feature/tmc-phase4-vw-06-basic-provider
- **Purpose**: Implement basic virtual workspace provider with core functionality, request routing, and workspace isolation
- **Target Lines**: 500
- **Dependencies**: Branch vw-05 (auth contracts)
- **Estimated Time**: 3 days

## Files to Create

### 1. pkg/virtual/provider/workspace.go (200 lines)
**Purpose**: Implement the main virtual workspace provider

**Implementation Structure**:
```go
package provider

import (
    "context"
    "fmt"
    "net/http"
    "sync"
    
    "github.com/kcp-dev/kcp/pkg/virtual/interfaces"
    "github.com/kcp-dev/kcp/pkg/virtual/workspace"
    "github.com/kcp-dev/kcp/pkg/virtual/discovery"
    "github.com/kcp-dev/kcp/pkg/virtual/auth"
    virtualv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/virtual/v1alpha1"
)

// BasicProvider implements a basic virtual workspace provider
type BasicProvider struct {
    mu               sync.RWMutex
    config           *interfaces.ProviderConfig
    workspaceManager workspace.Manager
    discoveryProvider discovery.Provider
    authProvider     auth.Provider
    router          *Router
    handlers        map[string]http.Handler
    initialized     bool
}

// Key methods to implement:
// - NewBasicProvider() - Constructor
// - ServeHTTP() - Main HTTP handler
// - Initialize() - Setup provider
// - GetWorkspace() - Retrieve workspace config
// - ListWorkspaces() - List all workspaces
// - Watch() - Monitor workspace changes
// - Shutdown() - Cleanup resources
// - registerHandlers() - Setup HTTP handlers
// - handleWorkspaceRequest() - Process workspace requests
```

### 2. pkg/virtual/provider/router.go (100 lines)
**Purpose**: Implement request routing logic

**Implementation Structure**:
```go
package provider

// Router handles routing requests to appropriate handlers
type Router struct {
    routes map[string]RouteHandler
    middleware []Middleware
}

// Key methods:
// - NewRouter() - Create router
// - AddRoute() - Register route
// - AddMiddleware() - Add middleware
// - Route() - Route request to handler
// - extractWorkspace() - Extract workspace from path
// - extractResource() - Extract resource from path
```

### 3. pkg/virtual/provider/handler.go (100 lines)
**Purpose**: Implement request handlers

**Implementation Structure**:
```go
package provider

// Handler processes virtual workspace requests
type Handler struct {
    provider *BasicProvider
}

// Key methods:
// - HandleList() - Handle list requests
// - HandleGet() - Handle get requests
// - HandleCreate() - Handle create requests
// - HandleUpdate() - Handle update requests
// - HandleDelete() - Handle delete requests
// - HandleWatch() - Handle watch requests
```

### 4. pkg/virtual/provider/workspace_test.go (100 lines)
**Purpose**: Test the basic provider implementation

## Implementation Steps

1. **Implement core provider**:
   - Create BasicProvider struct
   - Implement interfaces.VirtualWorkspaceProvider
   - Add workspace isolation logic

2. **Add routing**:
   - Implement path-based routing
   - Extract workspace and resource info
   - Support middleware chain

3. **Create handlers**:
   - Implement CRUD operations
   - Add watch support
   - Handle errors properly

4. **Add tests**:
   - Test provider initialization
   - Test request routing
   - Test workspace isolation

## Testing Requirements
- Unit test coverage: >80%
- Test scenarios:
  - Provider initialization
  - Request routing
  - Workspace isolation
  - Error handling
  - Concurrent access

## Integration Points
- Uses: All previous branch interfaces
- Provides: Basic working virtual workspace system

## Acceptance Criteria
- [ ] Provider implements all required interfaces
- [ ] Request routing works correctly
- [ ] Workspace isolation enforced
- [ ] Error handling comprehensive
- [ ] Tests pass with good coverage
- [ ] Follows KCP patterns
- [ ] No linting errors

## Common Pitfalls
- **Ensure workspace isolation**: Critical for multi-tenancy
- **Handle concurrent requests**: Thread-safe operations
- **Validate all inputs**: Security is paramount
- **Clean error messages**: Help debugging
- **Resource cleanup**: Prevent leaks
- **Test edge cases**: Empty workspaces, invalid paths

## Code Review Focus
- Workspace isolation correctness
- Thread safety
- Error handling completeness
- Resource lifecycle management
- Performance under load