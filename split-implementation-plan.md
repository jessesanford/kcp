# Split Implementation Plan for p7w1-sync-engine

## Current Size Violation
- Current: 804 lines
- Target: <700 lines per branch
- Overage: 4 lines (minimal violation)

## Analysis
This branch is only 4 lines over the limit. The implementation consists of:
- `engine.go`: 429 lines (main engine implementation)
- `resource_syncer.go`: 309 lines (resource synchronization logic)
- `types.go`: 66 lines (type definitions)
- `engine_test.go`: 269 lines (test file)

## Recommended Approach: Minor Refactoring

Since this branch is only 4 lines over the limit, a full split would be counterproductive. Instead, we recommend a minor refactoring to bring it under the limit.

### Option 1: Extract Constants and Configuration (Recommended)
Create a separate `config.go` file to hold:
- Default configuration values
- Constants used across the engine
- Configuration validation logic

This would move approximately 30-40 lines from `engine.go`, bringing the total well under 800 lines.

### Option 2: Move Test Helpers
If test coverage needs to be maintained in the same PR, consider:
- Moving test helper functions to a separate `testing_utils.go` file
- This is a common pattern in Kubernetes codebases

## Migration Strategy

### For Option 1 (Recommended):
1. Create `config.go` file in the same package
2. Move `DefaultEngineConfig()` from `types.go` (lines 57-66)
3. Extract configuration-related constants from `engine.go`
4. Extract any configuration validation logic from `engine.go`
5. Update imports in affected files
6. Run tests to ensure no regressions

### Expected Result:
- **engine.go**: ~410 lines (reduced by ~19)
- **resource_syncer.go**: 309 lines (unchanged)
- **types.go**: ~56 lines (reduced by ~10)
- **config.go**: ~29 lines (new file)
- **Total**: ~804 → ~775 lines (well under limit)

## Validation Checklist
- [ ] Total implementation lines under 800
- [ ] All tests pass
- [ ] No circular dependencies introduced
- [ ] Configuration logic properly isolated
- [ ] Code remains atomic and coherent

## Alternative: Test Split (Not Recommended)
If the above refactoring is not feasible, tests could be moved to a separate PR:
- **Branch 1**: Implementation only (535 lines)
- **Branch 2**: Tests only (269 lines)

However, this violates the principle of atomic PRs with tests, so it should only be used as a last resort.

## Recommendation
Given the minimal overage (4 lines), proceed with Option 1 to extract configuration logic. This maintains the atomic nature of the PR while bringing it under the size limit through a logical refactoring that improves code organization.