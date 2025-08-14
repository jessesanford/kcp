# Wave 2C: Upstream Sync Implementation Spec

## Overview
Implement the upstream syncer that pulls resources from physical clusters back to KCP.

**Branch**: `feature/tmc-syncer-02c-upstream-sync`  
**Base**: main (will cherry-pick Wave 1 API types)  
**Target Lines**: 500-600 (excluding generated code)  
**Worktree**: `/workspaces/kcp-worktrees/phase2/wave2c-upstream`

## Dependencies
- Must copy SyncTarget API types from Wave 1 branch
- Integrates with physical cluster clients
- Uses KCP syncer patterns

## TODO List

### 0. Copy Wave 1 API types (Required first)
- [ ] Copy `/pkg/apis/workload/v1alpha1/` from wave1-api-foundation worktree
- [ ] Run `make generate` to generate deepcopy
- [ ] Verify API types compile

### 1. Create upstream sync package (20 lines)
- [ ] Create `/pkg/reconciler/workload/syncer/upstream/` directory
- [ ] Add package documentation
- [ ] Create doc.go file

### 2. Implement upstream syncer struct (150 lines)
- [ ] Create `syncer.go`
- [ ] Define UpstreamSyncer struct with:
  - [ ] KCP cluster client
  - [ ] Physical cluster client
  - [ ] Resource informers
  - [ ] Work queue
- [ ] Add NewUpstreamSyncer function
- [ ] Implement Start method
- [ ] Add graceful shutdown

### 3. Add resource discovery (100 lines)
- [ ] Create `discovery.go`
- [ ] Discover available resources in physical cluster
- [ ] Filter resources to sync:
  - [ ] Based on SyncTarget configuration
  - [ ] Based on resource types
  - [ ] Based on namespaces
- [ ] Cache discovery results
- [ ] Handle discovery updates

### 4. Implement sync logic (100 lines)
- [ ] Create `sync.go`
- [ ] Pull resources from physical cluster
- [ ] Transform to KCP format:
  - [ ] Add workspace annotations
  - [ ] Convert cluster-specific fields
  - [ ] Handle UID mapping
- [ ] Apply to KCP workspace
- [ ] Handle sync errors

### 5. Add conflict resolution (80 lines)
- [ ] Create `conflict.go`
- [ ] Detect resource conflicts
- [ ] Implement merge strategies:
  - [ ] Server-side wins
  - [ ] Client-side wins
  - [ ] Three-way merge
- [ ] Track conflict history
- [ ] Report conflicts in status

### 6. Add status aggregation (50 lines)
- [ ] Create `status.go`
- [ ] Aggregate pod status from physical cluster
- [ ] Update deployment/statefulset status
- [ ] Track resource health
- [ ] Report to KCP resources

### 7. Write upstream sync tests (100 lines)
- [ ] Create `syncer_test.go`
- [ ] Test resource discovery
- [ ] Test sync logic
- [ ] Test conflict resolution
- [ ] Test status aggregation
- [ ] Test error scenarios

## Integration Requirements
- Must handle multiple resource types
- Must maintain consistency
- Must handle network failures gracefully
- Must respect rate limits

## Testing Requirements
- Unit tests for all sync logic
- Unit tests for conflict resolution
- Mock tests for cluster clients
- Integration test stubs

## Success Criteria
- [ ] Upstream sync pulls resources correctly
- [ ] Conflicts are handled properly
- [ ] Status is accurately aggregated
- [ ] Tests pass with good coverage
- [ ] Line count within target (500-600)
- [ ] Follows KCP syncer patterns

## Notes
- This enables observability of physical cluster state
- Must handle eventual consistency
- Consider performance with many resources
- Ensure idempotent operations