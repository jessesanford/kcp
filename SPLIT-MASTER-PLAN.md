# Split Master Plan: Wave 1 - API Foundation

## Current State
- **Total Lines:** 1,184 lines (excluding generated code)
- **Test Lines:** 234 lines
- **Files:** 10 files (including generated)
- **Status:** NEEDS SPLIT - Exceeds 800 line limit by 384 lines

## Split Strategy

### Split 1: Core API Types & Registration (wave1-01)
**Target Size:** ~450 lines  
**Branch:** `feature/tmc-syncer-01a-api-types`
**Worktree:** `/workspaces/kcp-worktrees/phase2/wave1-01-split-from-api-foundation`

**Files to Include:**
- `pkg/apis/workload/group.go` (~15 lines)
- `pkg/apis/workload/v1alpha1/doc.go` (22 lines)
- `pkg/apis/workload/v1alpha1/register.go` (54 lines)
- `pkg/apis/workload/v1alpha1/synctarget_types.go` (333 lines)
- `pkg/apis/workload/v1alpha1/zz_generated.deepcopy.go` (generated - not counted)

**Total:** ~424 lines + generated deepcopy

**Rationale:** Core API types must be established first as they form the foundation for all other functionality.

### Split 2: API Validation & Defaults (wave1-02)
**Target Size:** ~389 lines  
**Branch:** `feature/tmc-syncer-01b-api-validation`
**Worktree:** `/workspaces/kcp-worktrees/phase2/wave1-02-split-from-api-foundation`

**Files to Include:**
- `pkg/apis/workload/v1alpha1/synctarget_validation.go` (276 lines)
- `pkg/apis/workload/v1alpha1/synctarget_defaults.go` (113 lines)

**Total:** 389 lines

**Rationale:** Validation and defaulting logic can be added after types are defined. These are cohesive units that work together.

### Split 3: API Helpers, Conversion & Tests (wave1-03)
**Target Size:** ~590 lines  
**Branch:** `feature/tmc-syncer-01c-api-helpers`
**Worktree:** `/workspaces/kcp-worktrees/phase2/wave1-03-split-from-api-foundation`

**Files to Include:**
- `pkg/apis/workload/v1alpha1/synctarget_helpers.go` (309 lines)
- `pkg/apis/workload/v1alpha1/synctarget_conversion.go` (48 lines)
- `pkg/apis/workload/v1alpha1/synctarget_types_test.go` (233 lines)

**Total:** 590 lines

**Rationale:** Helpers and conversion are utility functions that depend on types but are independent of validation. Tests validate the entire API surface.
>>>>>>> feature/tmc-syncer-01-api-foundation

## Dependencies

```mermaid
graph TD
<<<<<<< HEAD
    W1[Wave 1: API Types] --> VW1[Split 1: Virtual Base]
    VW1 --> VW2[Split 2: Auth & Storage]
    VW1 --> VW3[Split 3: Transformation]
    VW2 --> T[Integration Tests]
    VW3 --> T
```

- **Wave 1 API Types** must be available (separate PR)
- **Split 1** establishes virtual workspace foundation
- **Split 2** and **Split 3** can proceed in parallel after Split 1
=======
    A[Split 1: API Types] --> B[Split 2: Validation]
    A --> C[Split 3: Helpers]
    B --> D[Tests Pass]
    C --> D
```

- **Split 1** must be merged first (establishes types)
- **Split 2** and **Split 3** can be done in parallel after Split 1
- All splits must maintain compilation and test success
>>>>>>> feature/tmc-syncer-01-api-foundation

## Execution Order

### Sequential Requirements:
<<<<<<< HEAD
1. **Split 1** (Virtual Base) - MUST be first, establishes foundation
2. **Split 2** and **Split 3** can proceed in parallel

### Parallel Opportunities:
- After Split 1 merges:
  - Agent A: Split 2 (Auth & Storage)
  - Agent B: Split 3 (Transformation)

## Critical Integration Points

### Split 1 → Split 2 Interface
- Virtual workspace registration
- Discovery provider interface
- Context propagation patterns

### Split 1 → Split 3 Interface
- Resource transformation hooks
- Virtual resource definitions
- Type conversion interfaces
=======
1. **Split 1** (API Types) - MUST be first
2. **Split 2** and **Split 3** can proceed in parallel after Split 1

### Parallel Opportunities:
- After Split 1 is complete:
  - Agent A can work on Split 2 (Validation)
  - Agent B can work on Split 3 (Helpers)
>>>>>>> feature/tmc-syncer-01-api-foundation

## Success Criteria

Each split must:
<<<<<<< HEAD
1. ✅ Stay under 800 lines (strictly enforced)
2. ✅ Be independently compilable
3. ✅ Maintain virtual workspace isolation
4. ✅ Follow KCP virtual workspace patterns
5. ✅ Include appropriate test coverage
6. ✅ Handle multi-tenancy correctly

## Risk Mitigation

1. **Virtual Workspace Pattern**: Must follow KCP's established patterns exactly
2. **Security**: Authentication must be properly isolated per workspace
3. **Resource Transformation**: Must handle all edge cases in conversion
4. **Discovery**: Must integrate with KCP's discovery mechanism
5. **Testing**: Virtual workspaces are complex - need thorough testing

## Verification Steps

```bash
# For each split:

# 1. Verify compilation
make build

# 2. Run code generation if needed
make codegen

# 3. Run unit tests
make test

# 4. Check virtual workspace registration
kubectl ws virtual

# 5. Test authentication
kubectl --context admin get --raw /services/syncer/clusters/

# 6. Verify line count
/workspaces/kcp-shared-tools/tmc-pr-line-counter.sh -c <branch-name>

# 7. Integration test
make test-e2e-shared-minimal
```

## Implementation Notes

### Virtual Workspace Patterns
- Must register with APIExport/APIBinding system
- Proper workspace isolation is critical
- Authentication must use KCP's auth patterns
- Discovery must integrate with aggregated API server

### Security Considerations
- Each virtual workspace must be isolated
- Authentication tokens must be workspace-scoped
- No cross-workspace data leakage
- Proper RBAC integration

### Performance Considerations
- Virtual workspaces add overhead
- Caching strategy needed for transformation
- Efficient discovery mechanism required

## Parallelization Strategy

### Independent Development Paths
1. **Path 1**: Virtual Base → Auth & Storage
2. **Path 2**: Virtual Base → Transformation

### Coordination Points
- Both paths depend on Split 1 (Virtual Base)
- Final integration testing requires all splits
- Performance testing after all components integrated

## Notes

- Virtual workspaces are a complex KCP feature requiring careful implementation
- Each split must maintain the security boundary
- The transformation logic is critical for resource compatibility
- Discovery mechanism must be efficient to avoid performance issues
- Test coverage is essential given the complexity
=======
1. ✅ Stay under 800 lines (excluding generated code)
2. ✅ Be atomic and compilable
3. ✅ Pass all tests
4. ✅ Include proper documentation
5. ✅ Follow KCP coding standards
6. ✅ Be independently reviewable

## Risk Mitigation

1. **Compilation Issues**: Each split includes minimal dependencies to ensure compilation
2. **Test Coverage**: Tests are included in Split 3 to validate all API functionality
3. **Generated Code**: Deepcopy generation must run after Split 1
4. **Import Cycles**: Careful package structure to avoid circular dependencies

## Verification Steps

Before creating PR for each split:
```bash
# 1. Verify compilation
make build

# 2. Run code generation
make codegen

# 3. Run tests
make test

# 4. Check line count
/workspaces/kcp-shared-tools/tmc-pr-line-counter.sh -c <branch-name>

# 5. Verify no uncommitted files
git status
```

## Notes

- Each split maintains the full package structure
- Generated code (deepcopy) is included but not counted in line limits
- CRD generation may be needed after API types are added
- All splits target merging to `main` independently
>>>>>>> feature/tmc-syncer-01-api-foundation
