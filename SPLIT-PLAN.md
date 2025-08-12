# Split Plan for impl4-26-constraint-engine

## Status
**Branch renamed to**: `feature/tmc-impl4/26-constraint-engine-to-be-split`
**Reason**: Branch exceeds PR size limit by 874 lines (1674 total vs 800 max)

## Current Size
- Hand-written code: 1,674 lines
- Test code: 1,550 lines  
- Generated code: 4,373 lines (excluded from count)

## Files to Split
```
32   pkg/apis/tmc/v1alpha1/doc.go
66   pkg/apis/tmc/v1alpha1/register.go
355  pkg/apis/tmc/v1alpha1/types_constraints_core.go
292  pkg/apis/tmc/v1alpha1/types_session_affinity.go
97   pkg/apis/tmc/v1alpha1/types_shared.go
807  pkg/apis/tmc/v1alpha1/types_sticky_binding.go (this file alone exceeds limit!)
```

## Recommended Split into 3 PRs

### PR 1: Session Affinity Foundation (~400 lines)
- Commits: f5452b40, 131ed13d
- Files:
  - types_session_affinity.go (292 lines)
  - Part of types_shared.go
  - Related tests

### PR 2: Sticky Binding Implementation (~800 lines)
- Commit: b2b0e52b
- Files:
  - types_sticky_binding.go (807 lines - may need further splitting)
  - Related tests

### PR 3: Constraint Evaluation Engine (~350 lines)
- Commits: e6e98183, 44d367ce
- Files:
  - types_constraints_core.go (355 lines)
  - doc.go, register.go
  - Related tests

## Next Steps
1. Create three new branches from main
2. Cherry-pick appropriate commits to each branch
3. Ensure each PR is under 800 lines
4. Update TMC-IMPL4-PR-PLAN.md to reflect the split
5. Archive this branch once splits are complete