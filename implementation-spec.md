# Wave 1: API Foundation Implementation Spec

## Overview
Implement the core SyncTarget API types and validation for the TMC syncer.

**Branch**: `feature/tmc-syncer-01-api-foundation`  
**Base**: main  
**Target Lines**: ~400-600 (excluding generated code)  
**Worktree**: `/workspaces/kcp-worktrees/phase2/wave1-api-foundation`

## Dependencies
- Uses standard Kubernetes API conventions
- Integrates with KCP API machinery
- Requires generated code

## TODO List

### 1. Create workload API package (50 lines)
- [ ] Create `/pkg/apis/workload/` directory
- [ ] Add group registration
- [ ] Create v1alpha1 package

### 2. Implement SyncTarget API types (300 lines)
- [ ] Create `synctarget_types.go`
- [ ] Define SyncTarget struct
- [ ] Define SyncTargetSpec:
  - [ ] Cluster endpoint
  - [ ] Authentication config
  - [ ] Resource filters
- [ ] Define SyncTargetStatus:
  - [ ] Connection status
  - [ ] Last sync time
  - [ ] Conditions

### 3. Add validation (200 lines)
- [ ] Create `synctarget_validation.go`
- [ ] Validate spec fields
- [ ] Check cluster endpoint format
- [ ] Validate authentication config

### 4. Add defaults (100 lines)
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
=======
# Wave 1: SyncTarget API Foundation Implementation Spec

## Overview
Create the SyncTarget API that all other syncer components depend on. This is the critical foundation that blocks all Wave 2 work.

**Branch**: `feature/tmc-syncer-01-api-foundation`  
**Base**: main  
**Target Lines**: 400-500 (excluding generated code)  
**Worktree**: `/workspaces/kcp-worktrees/phase2/wave1-api-foundation`

## TODO List

### 1. Create API package structure (20 lines)
- [ ] Create directory `/pkg/apis/workload/v1alpha1/`
- [ ] Create `doc.go` with package documentation
- [ ] Create `register.go` for API registration

### 2. Define SyncTarget types (150 lines)
- [ ] Create `synctarget_types.go`
- [ ] Define `SyncTarget` struct with TypeMeta and ObjectMeta
- [ ] Define `SyncTargetSpec` with fields:
  - [ ] ClusterRef (reference to ClusterRegistration)
  - [ ] SyncerConfig (configuration for syncer)
  - [ ] ResourceQuotas (capacity limits)
  - [ ] Selector (workload selection criteria)
- [ ] Define `SyncTargetStatus` with:
  - [ ] Conditions (using KCP conditions API)
  - [ ] Capacity information
  - [ ] LastSyncTime
  - [ ] SyncerVersion
- [ ] Define `SyncTargetList` type

### 3. Create validation logic (80 lines)
- [ ] Create `synctarget_validation.go`
- [ ] Implement ValidateSyncTarget function
- [ ] Implement ValidateSyncTargetUpdate function
- [ ] Add webhook validation markers
- [ ] Validate ClusterRef exists
- [ ] Validate ResourceQuotas are positive
- [ ] Validate Selector syntax

### 4. Generate CRD and deepcopy (Generated - not counted)
- [ ] Run `make generate` for deepcopy functions
- [ ] Run `make manifests` for CRD generation
- [ ] Verify generated files are correct

### 5. Add conversion hooks (60 lines)
- [ ] Create `synctarget_conversion.go`
- [ ] Implement hub version markers
- [ ] Add conversion test stubs
- [ ] Ensure forward compatibility

### 6. Create helper functions (50 lines)
- [ ] Create `synctarget_helpers.go`
- [ ] Add GetCondition helper
- [ ] Add SetCondition helper
- [ ] Add IsReady helper
- [ ] Add capacity calculation helpers
- [ ] Add selector matching helpers

### 7. Add defaulting logic (40 lines)
- [ ] Create `synctarget_defaults.go`
- [ ] Set default ResourceQuotas if not specified
- [ ] Set default SyncerConfig values
- [ ] Add defaulting webhook markers

### 8. Write comprehensive unit tests (100 lines)
- [ ] Create `synctarget_types_test.go`
- [ ] Test validation logic with valid/invalid inputs
- [ ] Test defaulting behavior
- [ ] Test helper functions
- [ ] Test condition management
- [ ] Achieve >80% code coverage

## Success Criteria
- [ ] All files created and properly integrated
- [ ] Tests pass with >80% coverage
- [ ] Generated code (deepcopy, CRD) works correctly
- [ ] Linting passes
- [ ] Line count within target (400-500 excluding generated)
- [ ] Follows KCP API patterns exactly
- [ ] Ready for Wave 2 branches to build upon

## Integration Points
- Must reference `ClusterRegistration` from TMC APIs
- Must use KCP conditions API from SDK
- Must follow workspace-aware patterns
- Must integrate with existing TMC types

## Testing Requirements
- Unit tests for all validation logic
- Unit tests for all helper functions
- Integration test stubs for controller testing
- Example YAML manifests for documentation

## Notes
- This is the CRITICAL PATH - Wave 2 cannot start until this completes
- Focus on clean, extensible API design
- Follow existing KCP patterns from apis.kcp.io
- Ensure workspace isolation is maintained
- Consider multi-tenancy from the start
>>>>>>> feature/tmc-syncer-01-api-foundation
