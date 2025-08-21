# Effort E1.1.5 Implementation Plan
Generated: 2025-08-21 04:45:00 UTC
Created by: TMC Orchestrator Planning Agent
Reviewed Phase Plan: PHASE1-SPECIFIC-IMPL-PLAN-8-20-25.md

## Context Analysis
### Completed Efforts in Current Wave
From orchestrator-state.yaml:
- E1.1.1 (api-types-core): Complete - Created base KCP API types (6 branches, 2854 lines total)
- E1.1.2 (synctarget-types): Complete - SyncTarget API types (3 branches, 1864 lines total)  
- E1.1.3 (placement-types): Complete - Created placement and initial scheduling types:
  - phase1/wave1/effort3-placement-core (417 lines)
  - phase1/wave1/effort3-placement-validation (~650 lines)
  - phase1/wave1/effort3-tmc-scheduling (~850 lines) - **Created Location and Placement types**

### Parallel Effort
- E1.1.4 (workload-types): In progress - Creating workload-related types

### Adjustments Based on Progress
Analysis of what's been implemented by E1.1.3:
- E1.1.3's effort3-tmc-scheduling already created:
  - `Location` type - Represents a set of scheduling resource instances
  - `Placement` type - Selection rule to choose location for namespaces
- E1.1.5 must add COMPLEMENTARY scheduling types, NOT duplicate these
- Focus on the remaining scheduling types specified in the phase plan:
  - `SchedulingPolicy` - Scheduling rules and constraints
  - `ResourceQuota` - Quota specifications for scheduling
  - `Priority` - Priority classes for scheduling decisions

## Effort Overview
- Phase: 1
- Wave: 1  
- Effort: 5
- Name: scheduling-types
- Base Branch: main
- Working Copy: /workspaces/efforts/phase1/wave1/effort5-scheduling-types
- Dependencies: E1.1.1, E1.1.2, E1.1.3 complete (E1.1.4 parallel)

## Specific Requirements
From phase plan with adjustments:
1. **MUST** implement remaining scheduling types NOT created by E1.1.3:
   - `SchedulingPolicy` - Rules for how workloads are scheduled
   - `ResourceQuota` - Quota specifications across locations
   - `Priority` - Priority classes for scheduling precedence
2. **MUST** integrate with existing Location and Placement types from E1.1.3
3. **MUST** follow KCP API conventions and patterns
4. **MUST** include validation logic and status conditions
5. **MUST** achieve 80% test coverage
6. **MUST** stay under 800 lines (target 700)

## Implementation Steps
1. Create scheduling API group structure (if not exists)
2. Implement SchedulingPolicy type with:
   - Rules for affinity/anti-affinity
   - Weight-based preferences
   - Constraint expressions
3. Implement ResourceQuota type with:
   - Resource limits per location
   - Aggregation logic
   - Status tracking
4. Implement Priority type with:
   - Priority classes
   - Preemption policies
   - Default priorities
5. Add validation webhooks for each type
6. Create comprehensive unit tests
7. Generate deepcopy methods
8. Add integration test scenarios

## Files to Create/Modify
### New Files to Create:
- `contrib-tmc/apis/scheduling/v1alpha1/types_schedulingpolicy.go` - SchedulingPolicy API type
- `contrib-tmc/apis/scheduling/v1alpha1/types_resourcequota.go` - ResourceQuota API type  
- `contrib-tmc/apis/scheduling/v1alpha1/types_priority.go` - Priority API type
- `contrib-tmc/apis/scheduling/v1alpha1/validation.go` - Validation logic for new types
- `test/apis/scheduling_types_test.go` - Unit tests for scheduling types

### Files to Modify:
- `contrib-tmc/apis/scheduling/v1alpha1/register.go` - Register new types
- `contrib-tmc/apis/scheduling/v1alpha1/doc.go` - Update package documentation
- `contrib-tmc/apis/scheduling/v1alpha1/zz_generated.deepcopy.go` - Will be regenerated

## Cherry-Pick Instructions
```bash
# No cherry-picks needed - starting fresh with new types
# May need to cherry-pick base setup from E1.1.3 if not in main
```

## Test Requirements
- Coverage: 80% minimum
- Specific tests needed:
  - SchedulingPolicy validation tests (valid/invalid rules, weight ranges)
  - ResourceQuota aggregation tests (sum calculation, status updates)
  - Priority ordering tests (precedence, preemption logic)
  - Integration tests with Location/Placement from E1.1.3
  - Validation webhook tests for all new types
  - DeepCopy tests for complex nested structures

## Size Constraints
- Target: 700 lines
- Maximum: 800 lines (measured by tmc-pr-line-counter.sh)
- Split strategy if exceeded:
  - Part 1: Core types (SchedulingPolicy, Priority)
  - Part 2: ResourceQuota and validation
  - Part 3: Tests and integration

## Success Criteria
- [ ] SchedulingPolicy type implemented with validation
- [ ] ResourceQuota type implemented with aggregation logic
- [ ] Priority type implemented with ordering logic
- [ ] All types registered and deepcopy generated
- [ ] Validation webhooks implemented
- [ ] Tests pass with 80%+ coverage
- [ ] Size under 800 lines per tmc-pr-line-counter.sh
- [ ] No hardcoded values
- [ ] Follows KCP style guide
- [ ] Integrates cleanly with E1.1.3's Location/Placement types

## Dependencies
- Depends on: E1.1.1 (api-types-core), E1.1.2 (synctarget-types), E1.1.3 (placement-types)
- Blocks: E1.2.1 (crd-generation), E1.2.2 (openapi-schemas), E1.2.3 (validation-rules)

## Implementation Notes
1. **Coordination with E1.1.3**: Since E1.1.3 already created Location and Placement in the scheduling API group, we must ensure our new types complement these existing types
2. **Source Material**: Reference `contrib-tmc/apis/scheduling/v1alpha1/` from the original TMC implementation
3. **API Conventions**: Follow Kubernetes API conventions for status conditions, validation, and printer columns
4. **Testing Strategy**: Focus on edge cases for scheduling rules, quota calculations, and priority conflicts