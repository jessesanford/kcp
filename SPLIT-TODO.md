# Split from impl4-26-constraint-engine: Constraint Core

## Purpose
This branch is part 3 of 3 splits from impl4-26-constraint-engine-to-be-split

## Target Size
~350 lines (currently empty, waiting for work to begin)

## What to Include
From the original impl4-26-constraint-engine branch, cherry-pick:
- Commits: e6e98183, 44d367ce
- Files to include:
  - pkg/apis/tmc/v1alpha1/types_constraints_core.go (355 lines)
  - pkg/apis/tmc/v1alpha1/doc.go (32 lines)
  - pkg/apis/tmc/v1alpha1/register.go (66 lines)
  - pkg/apis/tmc/v1alpha1/types_constraints_core_test.go (tests)
  - Wildwest informers (if needed for tests)

## Instructions for Agent
1. Cherry-pick the specified commits from feature/tmc-impl4/26-constraint-engine-to-be-split
2. Include the core constraint evaluation engine API
3. Ensure doc.go and register.go are included for proper package setup
4. Ensure the resulting PR is under 400 lines
5. Run code generation if needed
6. Ensure all tests pass

## Dependencies
- Can be independent or may depend on session affinity types

## PR Description
Constraint Evaluation Engine for TMC - provides the core constraint evaluation
engine API for advanced placement decision making in the Traffic Management Controller.