# Split Implementation Plan for sync-10-status-aggregator

## Problem Statement

The current implementation of the status aggregator feature branch `feature/tmc-phase4-sync-10-status-aggregator` contains 1,627 lines of hand-written code, which exceeds the 800-line hard limit for PRs. This needs to be split into smaller, atomic PRs that each remain under the limit while maintaining functional independence.

## Current Structure Analysis

### File Breakdown
```
pkg/status/
├── interfaces/interfaces.go     (289 lines) - Core interfaces and types
├── collector/collector.go       (400 lines) - Status collection implementation
├── aggregator/
│   ├── aggregator.go            (323 lines) - Main aggregation logic
│   ├── merger.go                (300 lines) - Field-level merging
│   ├── cache.go                 (310 lines) - TTL-based caching
│   └── aggregator_test.go       (171 lines) - Unit tests
```

### Dependency Graph
```
interfaces/interfaces.go (no dependencies)
    ↓
collector/collector.go (depends on interfaces)
    ↓
aggregator/merger.go (depends on interfaces)
    ↓
aggregator/cache.go (depends on interfaces)
    ↓
aggregator/aggregator.go (depends on interfaces, uses merger & cache)
    ↓
aggregator/aggregator_test.go (tests aggregator)
```

### Key Components
1. **Core Interfaces** - Define contracts for StatusCollector, StatusAggregator, StatusMerger, StatusCache
2. **Status Collector** - Multi-source status collection with retry policies
3. **Status Merger** - Field-level merging with conflict resolution
4. **Status Cache** - TTL-based caching with statistics
5. **Status Aggregator** - Main orchestration with multiple strategies

## Recommended 3-Way Split

### Split 1: Core Interfaces and Status Collection (Feature Branch: sync-10a-interfaces-collector)
**Target: ~689 lines**

**Files:**
- `pkg/status/interfaces/interfaces.go` (289 lines) - All core interfaces and types
- `pkg/status/collector/collector.go` (400 lines) - Complete collector implementation

**Rationale:**
- Forms the foundation that other components depend on
- Collector is independent and can be tested standalone
- Provides complete status collection functionality

**Testing:**
- Add collector unit tests (~100 lines)
- Test source registration/unregistration
- Test concurrent collection
- Test retry policies

### Split 2: Status Merging and Caching (Feature Branch: sync-10b-merger-cache)
**Target: ~710 lines**

**Files:**
- `pkg/status/aggregator/merger.go` (300 lines) - Field-level merging
- `pkg/status/aggregator/cache.go` (310 lines) - TTL-based caching
- Basic tests for merger and cache (~100 lines)

**Dependencies:**
- Requires Split 1 (interfaces package)

**Rationale:**
- Merger and cache are supporting components used by aggregator
- Both are independent of each other but share interface dependencies
- Can be tested independently before aggregator integration

**Testing:**
- Test merge strategies (union, intersection, priority)
- Test conflict resolution
- Test cache TTL and eviction
- Test cache statistics

### Split 3: Status Aggregator with Integration (Feature Branch: sync-10c-aggregator)
**Target: ~494 lines**

**Files:**
- `pkg/status/aggregator/aggregator.go` (323 lines) - Main aggregation logic
- `pkg/status/aggregator/aggregator_test.go` (171 lines) - Comprehensive tests

**Dependencies:**
- Requires Split 1 (interfaces and collector)
- Requires Split 2 (merger and cache)

**Rationale:**
- Final orchestration layer that brings everything together
- Contains strategy implementations (priority-based, merge-all, custom)
- Includes comprehensive integration tests

**Testing:**
- Test all aggregation strategies
- Test priority-based selection
- Test merge-all with conflicts
- Test custom strategy registration
- Integration tests with collector, merger, and cache

## Implementation Order

### Phase 1: Create Split 1 (sync-10a-interfaces-collector)
1. Branch from main: `feature/tmc-phase4-sync-10a-interfaces-collector`
2. Copy interfaces and collector packages
3. Add collector unit tests
4. Ensure all tests pass
5. Create PR (~689 lines)

### Phase 2: Create Split 2 (sync-10b-merger-cache)
1. Branch from main: `feature/tmc-phase4-sync-10b-merger-cache`
2. Copy interfaces package (for compilation)
3. Add merger and cache implementations
4. Add unit tests for both components
5. Ensure all tests pass
6. Create PR (~710 lines)

### Phase 3: Create Split 3 (sync-10c-aggregator)
1. Branch from Split 2: `feature/tmc-phase4-sync-10c-aggregator`
   - This ensures merger and cache are available
2. Add aggregator implementation
3. Add comprehensive aggregator tests
4. Ensure all tests pass including integration
5. Create PR (~494 lines)

## Dependencies Between Splits

### Merge Order
```
1. sync-10a-interfaces-collector → main
2. sync-10b-merger-cache → main (after #1 merged)
3. sync-10c-aggregator → main (after #1 and #2 merged)
```

### Key Considerations
- Each split is independently functional and testable
- Split 1 provides the foundation with no external dependencies
- Split 2 can be developed in parallel after Split 1's interfaces are defined
- Split 3 requires both previous splits to be merged for full functionality
- All splits remain well under the 800-line limit
- Test coverage is distributed across all splits

## Risk Mitigation

### Potential Issues
1. **Interface changes** - If interfaces need modification after Split 1, coordinate updates
2. **Integration complexity** - Split 3 might reveal integration issues requiring backports
3. **Test dependencies** - Ensure each split has adequate standalone tests

### Mitigation Strategies
1. **Interface stability** - Review interfaces thoroughly before Split 1 PR
2. **Early integration testing** - Test locally with all components before final split
3. **Feature flags** - Each split can have sub-feature flags for gradual rollout

## Success Criteria

- [ ] Each PR is under 800 lines of hand-written code
- [ ] Each PR passes all tests independently
- [ ] Each PR is atomic and provides value
- [ ] Total functionality matches original implementation
- [ ] No regression in test coverage
- [ ] Clean commit history in each PR
- [ ] All PRs can be reviewed independently

## Notes

- The line counts exclude generated code (deepcopy, CRDs)
- Test files are included in the line count but distributed appropriately
- Each split maintains the original package structure
- Documentation and comments are preserved in each split