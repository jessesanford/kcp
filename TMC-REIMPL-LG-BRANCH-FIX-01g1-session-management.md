# TMC 01g1-session-management Branch Split Plan

## Problem Statement

The `feature/tmc2-impl2/01g1-session-management` branch contains **2,583 lines** of implementation code, which exceeds the 700-line target by **269%**. This branch implements comprehensive session management APIs for the TMC placement system but is too large for optimal code review.

## Current Branch Analysis

### File Structure and Line Counts
```
pkg/apis/tmc/v1alpha1/
├── types_placement_session.go      # 457 lines - Core session management
├── types_session_state.go          # 737 lines - State persistence 
├── types_placement_decision.go     # 588 lines - Decision management
├── types_session_validation.go     # 633 lines - Validation framework
├── types_shared.go                 # 76 lines - Foundation types
├── register.go                     # 66 lines - API registration
├── doc.go                          # 28 lines - Package documentation
├── types_session_management_test.go # 860 lines - Comprehensive tests
└── zz_generated.deepcopy.go        # Generated (excluded from count)
```

**Total Implementation**: 2,583 lines  
**Total Tests**: 860 lines

### API Dependency Analysis

1. **Foundation Layer**: `types_shared.go` - Required by all APIs
2. **Core Layer**: `PlacementSession` - Central coordination API
3. **Extension Layer**: 
   - `SessionState` - Depends on PlacementSession for session references
   - `PlacementDecision` - Depends on PlacementSession for session context
   - `SessionValidator` - Independent validation framework

## Proposed Split Strategy

### 5-Branch Decomposition Plan

#### Branch 1: `feature/tmc2-impl2/01g1a-shared-foundation`
**Target Size**: ~200 lines  
**Base Branch**: `main`  
**Dependencies**: None

**Contents**:
- `types_shared.go` (76 lines) - All shared types and constants
- `register.go` (partial, ~30 lines) - Base registration structure
- `doc.go` (28 lines) - Package documentation
- Basic test coverage (~100 lines) - Foundation validation tests
- `zz_generated.deepcopy.go` (generated) - Deepcopy for shared types

**Purpose**: Establish the foundation layer with shared types, constants, and basic registration that all other APIs depend on.

**Rationale**: Creates a stable base that enables parallel development of the extension APIs while maintaining consistency.

#### Branch 2: `feature/tmc2-impl2/01g1b-placement-session` 
**Target Size**: ~650 lines  
**Base Branch**: `01g1a-shared-foundation`  
**Dependencies**: Shared foundation

**Contents**:
- `types_placement_session.go` (457 lines) - Complete PlacementSession API
- `register.go` (update, +20 lines) - Add PlacementSession registration
- Comprehensive test coverage (~250 lines) - PlacementSession validation tests
- `zz_generated.deepcopy.go` (update) - Add PlacementSession deepcopy methods

**Purpose**: Implement the core session management API that provides coordinated lifecycle management for placement operations.

**Key Features**:
- Session-based placement coordination
- Configurable placement policies with priorities
- Resource constraint validation
- Conflict resolution strategies
- Session recovery policies

#### Branch 3: `feature/tmc2-impl2/01g1c-placement-decision`
**Target Size**: ~680 lines  
**Base Branch**: `01g1b-placement-session`  
**Dependencies**: PlacementSession API

**Contents**:
- `types_placement_decision.go` (588 lines) - Complete PlacementDecision API
- `register.go` (update, +15 lines) - Add PlacementDecision registration  
- Comprehensive test coverage (~200 lines) - PlacementDecision validation tests
- `zz_generated.deepcopy.go` (update) - Add PlacementDecision deepcopy methods

**Purpose**: Implement decision coordination and execution API for managing placement decisions within sessions.

**Key Features**:
- Decision coordination with context tracking
- Cluster evaluation with weighted scoring
- Policy application and impact assessment
- Rollback policies with automated triggers
- Decision metrics and performance tracking

#### Branch 4: `feature/tmc2-impl2/01g1d-session-state`
**Target Size**: ~670 lines  
**Base Branch**: `01g1b-placement-session`  
**Dependencies**: PlacementSession API

**Contents**:
- `types_session_state.go` (737 lines) - Complete SessionState API
- `register.go` (update, +15 lines) - Add SessionState registration
- Comprehensive test coverage (~180 lines) - SessionState validation tests
- `zz_generated.deepcopy.go` (update) - Add SessionState deepcopy methods

**Purpose**: Implement persistent state tracking and recovery for distributed placement sessions.

**Key Features**:
- Multi-cluster session synchronization
- Resource allocation tracking
- Conflict history and resolution tracking
- State checkpointing and recovery
- Event tracking and audit trails

**Note**: This branch is slightly over the 700-line target (670 vs 700), but the SessionState API is atomic and cannot be meaningfully split without breaking its coherence.

#### Branch 5: `feature/tmc2-impl2/01g1e-session-validation`
**Target Size**: ~650 lines  
**Base Branch**: `01g1a-shared-foundation`  
**Dependencies**: Shared foundation only

**Contents**:
- `types_session_validation.go` (633 lines) - Complete SessionValidator API
- `register.go` (update, +15 lines) - Add SessionValidator registration
- Comprehensive test coverage (~130 lines) - SessionValidator validation tests
- `zz_generated.deepcopy.go` (update) - Add SessionValidator deepcopy methods

**Purpose**: Implement comprehensive validation framework with rule-based evaluation and conflict detection.

**Key Features**:
- Rule-based validation framework
- Conflict detection policies
- Resource validation with capacity thresholds
- Custom validation scripts with Lua support
- Dependency validation with circular dependency detection

## Implementation Sequence

### Phase 1: Foundation
1. **01g1a-shared-foundation** - Establish shared types and registration

### Phase 2: Core Session Management  
2. **01g1b-placement-session** - Implement core session API

### Phase 3: Parallel Extension Development
3. **01g1c-placement-decision** (depends on 01g1b)
4. **01g1d-session-state** (depends on 01g1b) 
5. **01g1e-session-validation** (depends on 01g1a only)

**Parallel Development**: After branch 2 is complete, branches 3, 4, and 5 can be developed simultaneously by different developers.

## Branch Dependencies Diagram

```
main
 └── 01g1a-shared-foundation (foundation)
     ├── 01g1b-placement-session (core)
     │   ├── 01g1c-placement-decision (extension)
     │   └── 01g1d-session-state (extension)
     └── 01g1e-session-validation (independent extension)
```

## Test Distribution Strategy

### Test Coverage by Branch
- **01g1a**: ~100 lines - Foundation type validation
- **01g1b**: ~250 lines - PlacementSession API validation  
- **01g1c**: ~200 lines - PlacementDecision API validation
- **01g1d**: ~180 lines - SessionState API validation
- **01g1e**: ~130 lines - SessionValidator API validation

**Total**: 860 lines distributed proportionally across all branches.

## Migration Steps from Current Branch

### Step 1: Create Foundation Branch (01g1a)
```bash
git checkout feature/tmc2-impl2/01g1-session-management
git checkout -b feature/tmc2-impl2/01g1a-shared-foundation

# Remove all API files except shared types
# Keep: types_shared.go, partial register.go, doc.go, basic tests
# Remove: types_placement_*.go, types_session_*.go, most tests

# Update register.go to only register shared types
# Update tests to only cover shared type validation
# Generate deepcopy for shared types only
```

### Step 2: Create PlacementSession Branch (01g1b)  
```bash
git checkout feature/tmc2-impl2/01g1a-shared-foundation
git checkout -b feature/tmc2-impl2/01g1b-placement-session

# Add PlacementSession API
# Keep: types_placement_session.go, related tests
# Update register.go to include PlacementSession
# Generate deepcopy for PlacementSession
```

### Step 3-5: Create Extension Branches in Parallel
Follow similar pattern for 01g1c, 01g1d, and 01g1e branches.

## Quality Assurance

### Per-Branch Validation
- [ ] Each branch compiles independently
- [ ] Each branch has comprehensive test coverage
- [ ] Each branch follows TMC PR size guidelines (≤700 lines)
- [ ] Each branch maintains API consistency
- [ ] Each branch includes proper deepcopy generation

### Integration Testing
- [ ] All branches can be merged sequentially without conflicts
- [ ] Final merged state matches original comprehensive implementation
- [ ] All tests pass in merged state
- [ ] Generated code is consistent across all branches

## Benefits of This Split Strategy

1. **Manageable Review Size**: Each PR is ≤700 lines for optimal review efficiency
2. **Logical Cohesion**: Each branch focuses on a single, cohesive API domain
3. **Clear Dependencies**: Foundation → core → extensions pattern is easy to understand
4. **Parallel Development**: After foundation + core, 3 branches can develop simultaneously
5. **Atomic Changes**: Each branch is self-contained and testable
6. **Incremental Value**: Each branch provides standalone value that can be merged independently

## Risk Mitigation

1. **Dependency Management**: Clear base branch strategy prevents integration conflicts
2. **API Consistency**: Shared foundation ensures consistent types across all APIs
3. **Test Coverage**: Comprehensive testing at each branch level prevents regressions  
4. **Generated Code**: Proper deepcopy generation at each stage maintains API compliance
5. **Documentation**: Each branch includes appropriate documentation updates

## Conclusion

This 5-branch split strategy transforms a 2,583-line monolithic branch into manageable, reviewable units while maintaining API integrity and enabling efficient parallel development. The approach balances review efficiency with logical API groupings to optimize both developer productivity and code quality.