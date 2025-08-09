# Branch Split Analysis: feature/tmc2-impl2/05a2-decision-processing

## Executive Summary

**DECISION: MANDATORY SPLIT REQUIRED**

The `feature/tmc2-impl2/05a2-decision-processing` branch contains **3317 lines** of implementation code, which is **373% over the 700-line target** and **414% over the 800-line maximum**. This branch cannot be submitted as a single atomic PR and must be split.

## Current State Analysis

### Line Count Breakdown

| Component | File | Lines | Percentage of Total |
|-----------|------|-------|-------------------|
| **API Types** | `sdk/apis/workload/v1alpha1/types.go` | 1158 | 35% |
| **Decision Engine** | `pkg/reconciler/workload/placement/decision.go` | 845 | 25% |
| **REST Mapper** | `pkg/reconciler/dynamicrestmapper/defaultrestmapper_kcp.go` | 536 | 16% |
| **Controller** | `pkg/reconciler/workload/placement/controller.go` | 299 | 9% |
| **Reconciler** | `pkg/reconciler/workload/placement/reconciler.go` | 197 | 6% |
| **Features** | `pkg/features/kcp_features.go` | 151 | 5% |
| **API Support** | Other API files | 131 | 4% |
| **TOTAL** | **Implementation Files** | **3317** | **100%** |

### Test Coverage Analysis

- **Test Files**: 877 lines (26% coverage ratio)
- **Decision Engine Tests**: 658 lines
- **Simple Tests**: 219 lines

## Recommended Split Strategy

### Option 1: Four-Branch Split (RECOMMENDED)

This approach creates manageable, atomic PRs with clear business boundaries:

#### Branch 1: 05a2a-api-foundation (~680 lines)
- **Files**: API types, registration, install files
- **Purpose**: Establish workload API foundation
- **Dependencies**: None (builds on main)
- **Estimated Lines**: 680

#### Branch 2: 05a2b-decision-engine (~845 lines)  
- **Files**: Decision engine core logic and tests
- **Purpose**: Advanced placement decision processing
- **Dependencies**: 05a2a-api-foundation
- **Estimated Lines**: 845

#### Branch 3: 05a2c-controller-integration (~650 lines)
- **Files**: Controller, reconciler, integration logic  
- **Purpose**: Wire decision engine into placement controller
- **Dependencies**: 05a2b-decision-engine
- **Estimated Lines**: 650

#### Branch 4: 05a2d-rest-mapper (~600 lines)
- **Files**: Dynamic REST mapper support
- **Purpose**: Enhanced resource mapping for placement
- **Dependencies**: 05a2c-controller-integration  
- **Estimated Lines**: 600

### Option 2: Six-Branch Split (Alternative)

For maximum compliance with line limits:

1. **05a2a-placement-api** (~400 lines) - Placement API types only
2. **05a2b-location-api** (~300 lines) - Location API types  
3. **05a2c-decision-core** (~400 lines) - Core decision logic
4. **05a2d-decision-strategies** (~400 lines) - Selection strategies
5. **05a2e-controller-logic** (~400 lines) - Controller integration
6. **05a2f-rest-mapper** (~600 lines) - REST mapper support

## Risk Assessment

### Current Risks (Status Quo)
- **HIGH**: 3317-line PR impossible to review effectively
- **HIGH**: Violates all size guidelines (700 target, 800 maximum)
- **MEDIUM**: Complex functionality makes rollback difficult
- **MEDIUM**: Testing coverage may be insufficient for size

### Split Approach Risks
- **LOW**: Dependency chain requires careful ordering
- **LOW**: Individual branches may have incomplete functionality
- **MINIMAL**: Each branch will have focused, testable scope

## Implementation Plan

### Phase 1: Branch Creation
1. Create base branches from current branch
2. Split files logically into separate commits
3. Ensure each branch compiles and tests pass
4. Verify line counts meet requirements

### Phase 2: Testing & Validation  
1. Run full test suite on each branch
2. Validate generated code consistency
3. Check integration points between branches
4. Verify feature flag compatibility

### Phase 3: PR Preparation
1. Create comprehensive PR messages for each branch
2. Update TMC PR plan with dependencies
3. Ensure proper commit signing and messages
4. Prepare for sequential merge process

## Justification for Split

### Why Split is Necessary

1. **Size Compliance**: Current branch is 373% over target, 414% over maximum
2. **Review Feasibility**: 3317 lines impossible for effective code review
3. **Atomic Principles**: Multiple distinct features bundled inappropriately  
4. **Risk Management**: Large monolithic changes increase rollback risk
5. **CI/CD Performance**: Smaller PRs enable faster build/test cycles

### Why Single PR is Not Viable

The PR message claims this is "the minimum atomic unit" but analysis shows:

- **API types** (1158 lines) are completely separable from implementation
- **Decision engine** (845 lines) is self-contained business logic
- **REST mapper** (536 lines) is infrastructure separate from placement logic
- **Controller integration** (496 lines) is wiring/coordination code

Each component serves a distinct purpose and can be developed/tested independently.

## Recommended Action

**Immediate**: Proceed with **Option 1: Four-Branch Split**
- Provides optimal balance between manageability and simplicity
- Creates clear business logic boundaries  
- Minimizes dependency complexity
- Each branch stays within or close to guidelines
- Maintains atomic functionality per branch

## Next Steps

1. ✅ **COMPLETED**: Analysis of current branch structure
2. 🔄 **IN PROGRESS**: Create 05a2a-api-foundation branch
3. ⏳ **PENDING**: Create remaining split branches
4. ⏳ **PENDING**: Validate each branch independently
5. ⏳ **PENDING**: Create PR messages for each branch
6. ⏳ **PENDING**: Update TMC PR plan with new dependencies

---

**Analysis Date**: 2025-08-08  
**Analyst**: Claude Code Agent 2  
**Branch**: feature/tmc2-impl2/05a2-decision-processing  
**Status**: MANDATORY SPLIT REQUIRED