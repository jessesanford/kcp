# Wave Plan: Workspace Discovery Implementation
## Branch: feature/tmc-phase4-16-workspace-discovery

This wave plan breaks down the workspace discovery implementation into manageable waves, each building on the previous one.

---

## 🌊 Wave 1: Core Discovery Types (150 lines)
**Goal**: Establish the foundational types and structures

### Files to Create:
1. **`pkg/placement/discovery/types.go`** (80 lines)
   - Define WorkspaceInfo struct
   - Define ClusterInfo struct  
   - Define DiscoveryOptions struct
   - Define DiscoveryResult struct
   - Add helper methods for type conversions

2. **`pkg/placement/discovery/errors.go`** (30 lines)
   - Custom error types for discovery failures
   - Permission denied errors
   - Workspace not found errors
   - Timeout errors

3. **`pkg/placement/discovery/doc.go`** (10 lines)
   - Package documentation

4. **`pkg/placement/discovery/constants.go`** (30 lines)
   - Discovery timeouts
   - Cache durations
   - Label constants
   - Annotation constants

### Validation:
- [ ] All types compile
- [ ] Types follow KCP conventions
- [ ] Documentation complete

---

## 🌊 Wave 2: Permission Checking (120 lines)
**Goal**: Implement permission verification for workspace access

### Files to Create:
1. **`pkg/placement/discovery/permissions.go`** (120 lines)
   - PermissionChecker struct
   - CheckWorkspaceAccess method
   - CheckClusterAccess method
   - Permission caching logic
   - RBAC integration

### Validation:
- [ ] Permission checks work correctly
- [ ] Caching reduces API calls
- [ ] Handles missing permissions gracefully

---

## 🌊 Wave 3: Cache Implementation (130 lines)
**Goal**: Add caching layer for performance

### Files to Create:
1. **`pkg/placement/discovery/cache.go`** (100 lines)
   - HierarchyCache struct
   - TTL-based cache expiration
   - Cache invalidation methods
   - Thread-safe operations

2. **`pkg/placement/discovery/cache_test.go`** (30 lines)
   - Basic cache tests
   - Expiration tests
   - Concurrency tests

### Validation:
- [ ] Cache improves performance
- [ ] Thread-safe under concurrent access
- [ ] Proper expiration handling

---

## 🌊 Wave 4: Workspace Traversal (120 lines)
**Goal**: Implement the core traversal logic

### Files to Create:
1. **`pkg/placement/discovery/traverser.go`** (120 lines)
   - WorkspaceTraverser struct
   - ListWorkspaces method
   - Recursive traversal logic
   - Integration with cache and permissions

### Validation:
- [ ] Can traverse workspace hierarchy
- [ ] Respects permissions
- [ ] Uses cache effectively

---

## 🌊 Wave 5: Cluster Discovery (100 lines)
**Goal**: Discover clusters within workspaces

### Files to Create:
1. **`pkg/placement/discovery/clusters.go`** (100 lines)
   - ClusterDiscoverer struct
   - DiscoverClusters method
   - Filter by labels/annotations
   - Health status checking

### Validation:
- [ ] Discovers all accessible clusters
- [ ] Filters work correctly
- [ ] Health status accurate

---

## 🌊 Wave 6: Interface Implementation (80 lines)
**Goal**: Implement the WorkspaceDiscovery interface

### Files to Create:
1. **`pkg/placement/discovery/impl.go`** (80 lines)
   - WorkspaceDiscoveryImpl struct
   - Implement all interface methods
   - Orchestrate traverser and cluster discovery
   - Error handling and logging

### Validation:
- [ ] Satisfies interfaces.WorkspaceDiscovery
- [ ] All methods implemented correctly
- [ ] Proper error propagation

---

## 🌊 Wave 7: Testing & Documentation (100 lines)
**Goal**: Comprehensive testing and documentation

### Files to Create:
1. **`pkg/placement/discovery/impl_test.go`** (60 lines)
   - Unit tests for main implementation
   - Mock clients
   - Edge case testing

2. **`pkg/placement/discovery/traverser_test.go`** (40 lines)
   - Traversal logic tests
   - Permission integration tests

### Documentation:
- [ ] Update README with usage examples
- [ ] Add inline code examples
- [ ] Document performance characteristics

### Validation:
- [ ] All tests pass
- [ ] Code coverage >80%
- [ ] Documentation complete

---

## 📊 Progress Tracking

| Wave | Description | Lines | Status | Commit |
|------|------------|-------|--------|--------|
| 1 | Core Types | 150 | ⏳ Pending | - |
| 2 | Permissions | 120 | ⏳ Pending | - |
| 3 | Cache | 130 | ⏳ Pending | - |
| 4 | Traversal | 120 | ⏳ Pending | - |
| 5 | Clusters | 100 | ⏳ Pending | - |
| 6 | Interface | 80 | ⏳ Pending | - |
| 7 | Testing | 100 | ⏳ Pending | - |
| **Total** | | **800** | | |

---

## 🎯 Success Criteria

### Per Wave:
- [ ] Implements specified functionality
- [ ] Passes all tests
- [ ] Follows KCP patterns
- [ ] Clean commit with descriptive message

### Overall:
- [ ] Total lines: ~800 (within PR limit)
- [ ] Satisfies all interface requirements
- [ ] Integrates with placement interfaces (Branch 13)
- [ ] Performance: <100ms for typical queries
- [ ] Memory: Efficient caching without leaks
- [ ] Documentation: Complete with examples

---

## 🚀 Implementation Order

1. Start with Wave 1 (types) - foundation
2. Wave 2 (permissions) - security layer
3. Wave 3 (cache) - performance layer
4. Wave 4 (traversal) - core logic
5. Wave 5 (clusters) - discovery logic
6. Wave 6 (interface) - integration
7. Wave 7 (testing) - validation

Each wave should be committed separately with a clear message describing what was implemented.

---

## ⚠️ Critical Considerations

1. **Workspace Isolation**: Never leak information across workspace boundaries
2. **Permission Checks**: Always verify access before returning data
3. **Cache Invalidation**: Ensure cache doesn't serve stale data
4. **Performance**: Minimize API calls through intelligent caching
5. **Error Handling**: Graceful degradation when workspaces unavailable
6. **Concurrency**: Thread-safe operations for parallel discovery

---

## 📝 Notes for Implementer

- Start each wave by reading the implementation-instructions.md for full context
- Keep each wave atomic and testable
- Use git commits to mark wave completion
- If a wave exceeds its line estimate, consider splitting it
- Run tests after each wave to ensure nothing breaks
- Update this plan with actual line counts and commit SHAs as you progress