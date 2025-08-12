# Split from impl4-26-constraint-engine: Sticky Binding

## Purpose
This branch is part 2 of 3 splits from impl4-26-constraint-engine-to-be-split

## Target Size
~800 lines (currently empty, waiting for work to begin)

## What to Include
From the original impl4-26-constraint-engine branch, cherry-pick:
- Commit: b2b0e52b
- Files to include:
  - pkg/apis/tmc/v1alpha1/types_sticky_binding.go (807 lines - may need trimming)
  - pkg/apis/tmc/v1alpha1/types_sticky_binding_test.go (tests)

## Instructions for Agent
1. Cherry-pick the specified commit from feature/tmc-impl4/26-constraint-engine-to-be-split
2. The types_sticky_binding.go file is 807 lines alone - consider:
   - Removing verbose comments if needed
   - Splitting into multiple files if logical
   - Ensuring core functionality remains intact
3. Ensure the resulting PR stays at or under 800 lines
4. Run code generation if needed
5. Ensure all tests pass

## Dependencies
- May depend on impl4-26a-session-affinity-split-from-impl4-26 for shared types

## PR Description
Sticky Binding Implementation for TMC - provides sticky session binding and 
session binding constraints for advanced workload placement policies.