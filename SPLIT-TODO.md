# Split from impl4-26-constraint-engine: Session Affinity

## Purpose
This branch is part 1 of 3 splits from impl4-26-constraint-engine-to-be-split

## Target Size
~400 lines (currently empty, waiting for work to begin)

## What to Include
From the original impl4-26-constraint-engine branch, cherry-pick:
- Commits: f5452b40, 131ed13d
- Files to include:
  - pkg/apis/tmc/v1alpha1/types_session_affinity.go (292 lines)
  - Part of pkg/apis/tmc/v1alpha1/types_shared.go (relevant portions)
  - pkg/apis/tmc/v1alpha1/types_session_affinity_test.go (tests)

## Instructions for Agent
1. Cherry-pick the specified commits from feature/tmc-impl4/26-constraint-engine-to-be-split
2. Remove any files not related to session affinity
3. Ensure the resulting PR is under 400 lines
4. Run code generation if needed
5. Ensure all tests pass

## Dependencies
- None (can be merged independently)

## PR Description
Session Affinity Foundation for TMC - provides the basic API types and validation
for session-based placement policies in the Traffic Management Controller.