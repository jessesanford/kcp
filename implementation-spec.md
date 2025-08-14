# Wave 2B: Virtual Workspace Implementation Spec

## Overview
Implement the virtual workspace that provides the API endpoint for syncers to connect to KCP.

**Branch**: `feature/tmc-syncer-02b-virtual-workspace`  
**Base**: main (will cherry-pick Wave 1 API types)  
**Target Lines**: 500-600 (excluding generated code)  
**Worktree**: `/workspaces/kcp-worktrees/phase2/wave2b-virtual`

## Dependencies
- Must copy SyncTarget API types from Wave 1 branch
- Uses KCP virtual workspace patterns
- Integrates with authentication system

## TODO List

### 0. Copy Wave 1 API types (Required first)
- [ ] Copy `/pkg/apis/workload/v1alpha1/` from wave1-api-foundation worktree
- [ ] Run `make generate` to generate deepcopy
- [ ] Verify API types compile

### 1. Create virtual workspace package (20 lines)
- [ ] Create `/pkg/virtual/syncer/` directory
- [ ] Add package documentation
- [ ] Create doc.go file

### 2. Implement VirtualWorkspace interface (150 lines)
- [ ] Create `virtual_workspace.go`
- [ ] Define VirtualWorkspace struct
- [ ] Implement RootPathResolver:
  - [ ] Resolve syncer paths
  - [ ] Handle workspace routing
- [ ] Implement ReadyChecker:
  - [ ] Check backend availability
  - [ ] Verify authentication
- [ ] Implement Authorizer:
  - [ ] Validate syncer certificates
  - [ ] Check permissions

### 3. Add REST storage implementation (100 lines)
- [ ] Create `rest_storage.go`
- [ ] Implement resource storage interface
- [ ] Handle GET requests for resources
- [ ] Handle LIST with filtering
- [ ] Handle WATCH for changes
- [ ] Apply workspace scoping

### 4. Implement authentication (80 lines)
- [ ] Create `auth.go`
- [ ] Handle syncer certificate validation
- [ ] Extract syncer identity from cert
- [ ] Map syncer to SyncTarget
- [ ] Validate syncer permissions
- [ ] Handle authentication errors

### 5. Add resource transformation (100 lines)
- [ ] Create `transformation.go`
- [ ] Transform KCP resources for syncers:
  - [ ] Remove internal annotations
  - [ ] Add syncer-specific metadata
  - [ ] Handle namespace mapping
- [ ] Transform syncer resources to KCP:
  - [ ] Add workspace metadata
  - [ ] Apply placement labels
- [ ] Handle resource versioning

### 6. Implement discovery (50 lines)
- [ ] Create `discovery.go`
- [ ] Provide API discovery endpoint
- [ ] Filter APIs based on syncer permissions
- [ ] Generate OpenAPI spec
- [ ] Handle version negotiation

### 7. Write virtual workspace tests (100 lines)
- [ ] Create `virtual_workspace_test.go`
- [ ] Test authentication flows
- [ ] Test resource access control
- [ ] Test transformation logic
- [ ] Test discovery endpoint
- [ ] Test error handling

## Integration Requirements
- Must integrate with KCP authentication
- Must maintain workspace isolation
- Must handle multi-tenancy
- Must support bidirectional sync

## Testing Requirements
- Unit tests for all authentication logic
- Unit tests for transformation
- Mock tests for REST storage
- Integration test stubs

## Success Criteria
- [ ] Virtual workspace provides syncer endpoint
- [ ] Authentication works correctly
- [ ] Resources are properly transformed
- [ ] Tests pass with good coverage
- [ ] Line count within target (500-600)
- [ ] Follows KCP virtual workspace patterns

## Notes
- This is the API endpoint syncers connect to
- Security is critical - proper authentication required
- Must handle high-frequency polling from syncers
- Consider performance implications