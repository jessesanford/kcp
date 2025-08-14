# Wave 2A: SyncTarget Controller Implementation Spec

## Overview
Implement the controller to manage SyncTarget lifecycle. This controller handles syncer deployment and status management.

**Branch**: `feature/tmc-syncer-02a-controller`  
**Base**: main (will cherry-pick Wave 1 API types)  
**Target Lines**: 600-700 (excluding generated code)  
**Worktree**: `/workspaces/kcp-worktrees/phase2/wave2a-controller`

## Dependencies
- Must copy SyncTarget API types from Wave 1 branch
- Integrates with existing ClusterRegistration API
- Uses KCP controller patterns

## TODO List

### 0. Copy Wave 1 API types (Required first)
- [ ] Copy `/pkg/apis/workload/v1alpha1/` from wave1-api-foundation worktree
- [ ] Run `make generate` to generate deepcopy
- [ ] Verify API types compile

### 1. Create controller package structure (20 lines)
- [ ] Create `/pkg/reconciler/workload/synctarget/` directory
- [ ] Add package documentation
- [ ] Create doc.go file

### 2. Implement controller struct (100 lines)
- [ ] Create `controller.go`
- [ ] Define Controller struct with:
  - [ ] KCP cluster-aware clients
  - [ ] TMC clientset for ClusterRegistration access
  - [ ] Workspace-aware informers
  - [ ] Work queue setup
- [ ] Add NewController function
- [ ] Implement workspace isolation

### 3. Implement reconciliation logic (200 lines)
- [ ] Create `reconcile.go`
- [ ] Implement Reconcile method
- [ ] Handle SyncTarget creation:
  - [ ] Validate ClusterRef exists
  - [ ] Create syncer configuration
  - [ ] Deploy syncer to physical cluster
- [ ] Handle SyncTarget update:
  - [ ] Update syncer configuration
  - [ ] Handle capacity changes
- [ ] Handle SyncTarget deletion:
  - [ ] Clean up syncer deployment
  - [ ] Remove finalizers

### 4. Add status management (80 lines)
- [ ] Create `status.go`
- [ ] Implement updateStatus method
- [ ] Update conditions:
  - [ ] Ready condition
  - [ ] SyncerDeployed condition
  - [ ] ClusterConnected condition
- [ ] Report capacity from physical cluster
- [ ] Track LastSyncTime

### 5. Implement syncer deployment (100 lines)
- [ ] Create `deployment.go`
- [ ] Generate syncer manifests
- [ ] Create ServiceAccount for syncer
- [ ] Generate RBAC for syncer
- [ ] Configure authentication:
  - [ ] Generate certificates
  - [ ] Create kubeconfig
- [ ] Deploy syncer pod/deployment

### 6. Add event handling (50 lines)
- [ ] Create `events.go`
- [ ] Add informer event handlers
- [ ] Implement retry logic with backoff
- [ ] Handle transient errors
- [ ] Record events for important state changes

### 7. Write controller tests (150 lines)
- [ ] Create `controller_test.go`
- [ ] Test reconciliation scenarios:
  - [ ] Create new SyncTarget
  - [ ] Update existing SyncTarget
  - [ ] Delete SyncTarget
- [ ] Test status updates
- [ ] Test error handling
- [ ] Test workspace isolation

## Integration Requirements
- Must integrate with ClusterRegistration for cluster access
- Must follow KCP controller patterns
- Must maintain workspace isolation
- Must handle multi-tenancy correctly

## Testing Requirements
- Unit tests for all reconciliation logic
- Unit tests for status management
- Mock tests for syncer deployment
- Integration test stubs

## Success Criteria
- [ ] Controller properly manages SyncTarget lifecycle
- [ ] Syncer deployment logic is complete
- [ ] Status accurately reflects syncer state
- [ ] Tests pass with good coverage
- [ ] Line count within target (600-700)
- [ ] Follows KCP patterns exactly

## Notes
- This enables syncer deployment to physical clusters
- Must handle authentication securely
- Consider failure recovery scenarios
- Ensure proper cleanup on deletion