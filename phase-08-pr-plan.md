# Phase 8: Cross-Workspace Runtime - PR Plan

## Executive Summary

**Phase Status**: 87.5% Complete (7 of 8 components implemented)  
**Total Branches**: 8 feature branches created  
**PR Messages**: 3 created, 5 pending  
**Size Issues**: 2 branches need splitting (Cross-Workspace Controller, Decision Maker)  
**Blocker**: Status Aggregation (8.3.3) blocked on Phase 7 syncer completion

## Implementation Status

### Wave 1: Discovery Foundation ✅ COMPLETE

| Component | Branch | Lines | PR Message | Status |
|-----------|--------|-------|------------|--------|
| 8.1.1 Workspace Discovery | `feature/tmc-completion/p8w1-workspace-discovery` | 730 | ✅ Created | ✅ Ready for PR |
| 8.1.2 API Discovery | `feature/tmc-completion/p8w1-api-discovery` | 730 | ❌ Missing | ✅ Implementation Complete |

### Wave 2: Decision Engine ✅ COMPLETE

| Component | Branch | Lines | PR Message | Status |
|-----------|--------|-------|------------|--------|
| 8.2.1 Placement Scheduler | `feature/tmc-completion/p8w2-scheduler` | ~1000 | ❌ Missing | ✅ Foundation Complete |
| 8.2.2 CEL Evaluator | `feature/tmc-completion/p8w2-cel-evaluator` | 650 | ❌ Missing | ✅ Ready for PR |
| 8.2.3 Decision Maker | `feature/tmc-completion/p8w2-decision-maker` | 2688 | ❌ Missing | ⚠️ NEEDS SPLITTING |

### Wave 3: Execution Layer ⚠️ PARTIALLY COMPLETE

| Component | Branch | Lines | PR Message | Status |
|-----------|--------|-------|------------|--------|
| 8.3.1 Cross-Workspace Controller | `feature/tmc-completion/p8w3-controller` | 2230 | ❌ Missing | ⚠️ NEEDS SPLITTING |
| 8.3.2 Placement Binding | `feature/tmc-completion/p8w3-binding` | 702 | ✅ Created | ✅ Ready for PR |
| 8.3.3 Status Aggregation | Not created | - | - | 🔴 BLOCKED on Phase 7 |

## PR Splitting Requirements

### Decision Maker (8.2.3) - Split into 5 PRs:
1. **PR1: Types & Interfaces** (~450 lines)
   - `types.go`
   - Branch: `feature/tmc-completion/p8w2-decision-types`

2. **PR2: Core Decision Logic** (~700 lines)
   - `decision_maker.go`
   - Branch: `feature/tmc-completion/p8w2-decision-core`

3. **PR3: Validation** (~450 lines)
   - `validator.go`
   - Branch: `feature/tmc-completion/p8w2-decision-validator`

4. **PR4: Recording & History** (~450 lines)
   - `recorder.go`
   - Branch: `feature/tmc-completion/p8w2-decision-recorder`

5. **PR5: Override System** (~650 lines)
   - `override.go`
   - Branch: `feature/tmc-completion/p8w2-decision-override`

### Cross-Workspace Controller (8.3.1) - Split into 3 PRs:
1. **PR1: Foundation** (~700 lines)
   - `controller.go` + `watcher.go`
   - Branch: `feature/tmc-completion/p8w3-controller-foundation`

2. **PR2: Reconciliation** (~750 lines)
   - `reconciler.go` + `status.go`
   - Branch: `feature/tmc-completion/p8w3-controller-reconciler`

3. **PR3: Integration** (~780 lines)
   - `placement_handler.go` + tests
   - Branch: `feature/tmc-completion/p8w3-controller-integration`

## Merge Order

### Prerequisites
- Phase 5: Placement interfaces (must be merged first)
- Phase 6: Virtual Workspace infrastructure (must be merged first)
- Phase 7: Syncer (required for 8.3.3 only)

### Phase 8 Merge Sequence

1. **Wave 1 (Parallel)**
   - `p8w1-workspace-discovery`
   - `p8w1-api-discovery`

2. **Wave 2 (Sequential with some parallelism)**
   - `p8w2-scheduler` (first)
   - `p8w2-cel-evaluator` (parallel with scheduler)
   - `p8w2-decision-types` (after scheduler)
   - `p8w2-decision-core` (after types)
   - `p8w2-decision-validator` (parallel with core)
   - `p8w2-decision-recorder` (parallel with core)
   - `p8w2-decision-override` (after core)

3. **Wave 3 (Sequential)**
   - `p8w3-controller-foundation` (first)
   - `p8w3-controller-reconciler` (after foundation)
   - `p8w3-controller-integration` (after reconciler)
   - `p8w3-binding` (parallel with controller PRs)
   - `p8w3-status-aggregation` (when Phase 7 complete)

## Action Items

### Immediate (Before Any PRs)
1. [ ] Create PR messages for Wave 1 API Discovery
2. [ ] Create PR messages for all Wave 2 components
3. [ ] Create PR messages for Wave 3 Controller
4. [ ] Split Decision Maker into 5 branches
5. [ ] Split Cross-Workspace Controller into 3 branches

### PR Creation Order
1. [ ] Wave 1: Create 2 PRs (can be parallel)
2. [ ] Wave 2: Create 6 PRs (scheduler first, then others)
3. [ ] Wave 3: Create 4 PRs (controller foundation first)

### Documentation
1. [ ] Update each PR with comprehensive description
2. [ ] Include testing instructions in each PR
3. [ ] Add integration notes for dependent components
4. [ ] Document any API changes

## Risk Assessment

### High Risk
- **Size violations**: 2 components need splitting before PR creation
- **Missing PR messages**: 5 branches lack PR descriptions

### Medium Risk
- **Integration complexity**: Wave 3 controller has many dependencies
- **Testing coverage**: Some components have minimal tests

### Low Risk
- **Code quality**: All implementations follow KCP patterns
- **Compilation**: All branches compile successfully

## Success Criteria

- [ ] All 8 components implemented (7/8 done, 1 blocked)
- [ ] All branches under 800 lines (after splitting)
- [ ] All PR messages created
- [ ] All tests passing
- [ ] Integration verified between waves
- [ ] Documentation complete

## Notes

1. **Status Aggregation (8.3.3)** cannot be implemented until Phase 7's syncer is complete
2. **Decision Maker** significantly exceeded size estimates but provides comprehensive functionality
3. **Cross-Workspace Controller** complexity justified by the need for robust state management
4. All implementations follow KCP patterns and integrate properly with existing phases