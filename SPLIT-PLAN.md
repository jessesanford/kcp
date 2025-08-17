# Split Plan for feature/phase7-syncer-impl/p7w1-sync-engine

## Current State
- **Total Lines**: 904 implementation lines (excluding tests)
- **With Tests**: 1,174 lines total
- **Status**: Safe to split - no external dependencies

## Dependency Analysis
✅ **SAFE TO SPLIT** - Analysis Results:
1. No other Phase 7 branches import the engine package directly
2. Wave 2 downstream-core uses only the transformation package (different branch)
3. The Engine is self-contained with no cross-package dependencies
4. Public APIs can be preserved across splits

## Split Strategy

### Split 1: p7w1-sync-engine-types (66 lines)
**Branch**: `feature/phase7-syncer-impl/p7w1-sync-engine-types`
**Content**: Core types and interfaces
- `pkg/reconciler/workload/syncer/engine/types.go` (66 lines)

**Rationale**: Establishes the type foundation that other splits will import

### Split 2: p7w1-sync-engine-core (522 lines)
**Branch**: `feature/phase7-syncer-impl/p7w1-sync-engine-core`
**Dependencies**: p7w1-sync-engine-types
**Content**: Core engine implementation
- `pkg/reconciler/workload/syncer/engine/engine.go` (522 lines)

**Rationale**: Main engine logic without resource-specific handling

### Split 3: p7w1-sync-engine-resource (586 lines)
**Branch**: `feature/phase7-syncer-impl/p7w1-sync-engine-resource`
**Dependencies**: p7w1-sync-engine-types, p7w1-sync-engine-core
**Content**: Resource syncer and tests
- `pkg/reconciler/workload/syncer/engine/resource_syncer.go` (316 lines)
- `pkg/reconciler/workload/syncer/engine/engine_test.go` (270 lines)

**Rationale**: Resource-specific logic and all tests together

## Implementation Order

1. **Create p7w1-sync-engine-types branch**
   - Base from: main
   - Copy only types.go
   - Ensure compilation
   - Create minimal test to verify types

2. **Create p7w1-sync-engine-core branch**
   - Base from: main (not from types branch)
   - Copy types.go + engine.go
   - Import path remains the same
   - Add basic engine tests

3. **Create p7w1-sync-engine-resource branch**
   - Base from: main
   - Copy all files (types.go, engine.go, resource_syncer.go, engine_test.go)
   - This becomes the "complete" branch with all functionality
   - Full test coverage

## Merge Strategy
PRs should be merged in this order:
1. p7w1-sync-engine-types (establishes foundation)
2. p7w1-sync-engine-core (adds engine)
3. p7w1-sync-engine-resource (completes implementation)

## Risk Mitigation
- Each split compiles independently
- Each split has its own tests
- No API changes between splits
- Import paths remain consistent
- Other waves can start importing after Split 1 merges

## Validation Checklist
- [ ] Split 1 compiles and tests pass
- [ ] Split 2 imports Split 1's types correctly
- [ ] Split 3 includes all original functionality
- [ ] Total lines across splits equals original
- [ ] No functionality lost in splitting
- [ ] All tests still pass