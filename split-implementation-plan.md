# Split Implementation Plan for p7w1-transform

## Current Size Violation
- Current: 1410 lines
- Target: <700 lines per branch
- Overage: 610 lines (significant violation requiring 2-3 branches)

## Analysis
The transformation package implements distinct transformation concerns:
- `pipeline.go`: 246 lines (orchestration and interfaces)
- `namespace.go`: 252 lines (namespace mapping logic)
- `metadata.go`: 296 lines (metadata transformation)
- `secret.go`: 339 lines (secret handling and security)
- `ownership.go`: 277 lines (ownership and finalizer management)
- `pipeline_test.go`: 337 lines (test coverage)

## Proposed Split Structure

### Branch 1: p7w1-transform-core (~550 lines)
**Purpose**: Core transformation pipeline and namespace handling
- Files:
  - `pipeline.go`: 246 lines (defines interfaces and orchestration)
  - `namespace.go`: 252 lines (workspace-aware namespace mapping)
  - Basic test stubs: ~52 lines
- Estimated lines: 550 lines
- Dependencies: None (foundational branch)
- Rationale: These form the core transformation framework that other transformers depend on

### Branch 2: p7w1-transform-metadata (~635 lines)
**Purpose**: Metadata and ownership transformation logic
- Files:
  - `metadata.go`: 296 lines (label, annotation, and metadata handling)
  - `ownership.go`: 277 lines (owner references and finalizers)
  - Basic test stubs: ~62 lines
- Estimated lines: 635 lines
- Dependencies: Branch 1 (requires pipeline interfaces)
- Rationale: These handle resource metadata transformations as a cohesive unit

### Branch 3: p7w1-transform-security (~576 lines)
**Purpose**: Security-sensitive transformations and comprehensive testing
- Files:
  - `secret.go`: 339 lines (secret transformation and security)
  - `pipeline_test.go`: 337 lines (comprehensive test suite)
  - Integration test helpers: ~100 lines (estimated for test completeness)
- Estimated lines: 576 lines (excluding helper code counted elsewhere)
- Dependencies: Branches 1 and 2 (tests all transformers)
- Rationale: Groups security-critical code with comprehensive testing

## Migration Strategy

### Phase 1: Create p7w1-transform-core
1. Create new branch `feature/phase7-syncer-impl/p7w1-transform-core` from main
2. Create new worktree at `/workspaces/kcp-worktrees/phase7/syncer-impl/worktrees/p7w1-transform-core`
3. Copy from original branch:
   - `pkg/reconciler/workload/syncer/transformation/pipeline.go`
   - `pkg/reconciler/workload/syncer/transformation/namespace.go`
4. Add minimal test stubs to ensure interfaces are testable
5. Ensure package compiles and basic tests pass
6. Commit with clear message about core transformation framework

### Phase 2: Create p7w1-transform-metadata
1. Create new branch `feature/phase7-syncer-impl/p7w1-transform-metadata` from p7w1-transform-core
2. Create new worktree at `/workspaces/kcp-worktrees/phase7/syncer-impl/worktrees/p7w1-transform-metadata`
3. Copy from original branch:
   - `pkg/reconciler/workload/syncer/transformation/metadata.go`
   - `pkg/reconciler/workload/syncer/transformation/ownership.go`
4. Add basic unit tests for metadata and ownership transformers
5. Ensure all transformers implement the ResourceTransformer interface
6. Verify compilation against core interfaces
7. Commit with clear message about metadata transformation capabilities

### Phase 3: Create p7w1-transform-security
1. Create new branch `feature/phase7-syncer-impl/p7w1-transform-security` from p7w1-transform-metadata
2. Create new worktree at `/workspaces/kcp-worktrees/phase7/syncer-impl/worktrees/p7w1-transform-security`
3. Copy from original branch:
   - `pkg/reconciler/workload/syncer/transformation/secret.go`
   - `pkg/reconciler/workload/syncer/transformation/pipeline_test.go`
4. Ensure comprehensive test coverage for all transformers
5. Add security-specific validation tests
6. Verify all tests pass with complete transformer suite
7. Commit with clear message about security transformations and testing

### Phase 4: Archive Original Branch
1. Tag the original branch for reference: `git tag archive/p7w1-transform-original`
2. Delete the original oversized branch
3. Remove the original worktree

## Validation Checklist

### For Each Split Branch:
- [ ] Lines of code under 700 (use tmc-pr-line-counter.sh)
- [ ] Compiles independently
- [ ] Has appropriate test coverage
- [ ] No circular dependencies
- [ ] Clear commit messages
- [ ] Follows KCP patterns

### Integration Validation:
- [ ] Branch 1 provides complete interfaces
- [ ] Branch 2 builds on Branch 1 correctly
- [ ] Branch 3 tests entire transformation suite
- [ ] All transformers implement ResourceTransformer interface
- [ ] Pipeline can orchestrate all transformers

## Dependency Graph
```
main
  └── p7w1-transform-core (interfaces & namespace)
      └── p7w1-transform-metadata (metadata & ownership)
          └── p7w1-transform-security (secrets & full tests)
```

## Risk Mitigation
- Each branch is independently compilable and testable
- Core interfaces are established first to prevent API changes
- Test coverage is maintained throughout the split
- Security-sensitive code remains together in one PR
- Original branch is tagged before deletion for recovery if needed

## Expected Outcome
Three atomic, well-sized PRs that:
1. Establish the transformation framework (550 lines)
2. Add metadata transformation capabilities (635 lines)
3. Complete security transformations with full testing (576 lines)

Total: 1761 lines (including test improvements) vs original 1410 lines
The slight increase accounts for proper test stubs and integration helpers to ensure each PR is complete and testable.