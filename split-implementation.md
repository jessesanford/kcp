# Split Implementation: Wave1-03 - API Helpers, Conversion & Tests

## Overview
**Branch:** `feature/tmc-syncer-01c-api-helpers`  
**Target Size:** 590 lines  
**Dependencies:** Wave1-01 (API Types) must be merged first  
**Can Run In Parallel:** Yes, with Wave1-02 after Wave1-01 merges

## Files to Copy/Create

### Required Files from Wave1-01
This split depends on having the API types from Wave1-01:
```bash
# Ensure Wave1-01 changes are available
git fetch origin
git merge origin/main  # After Wave1-01 is merged
```

### Files to Add

1. **pkg/apis/workload/v1alpha1/synctarget_helpers.go** (309 lines)
   - Helper functions for SyncTarget manipulation
   - Condition management helpers
   - Status update utilities
   - Label and annotation helpers
   - Capacity calculation functions

2. **pkg/apis/workload/v1alpha1/synctarget_conversion.go** (48 lines)
   - Conversion functions between versions
   - Hub version markers
   - Conversion webhook setup

3. **pkg/apis/workload/v1alpha1/synctarget_types_test.go** (233 lines)
   - Comprehensive API tests
   - Helper function tests
   - Validation tests
   - Default value tests

## Implementation Checklist

### Pre-Implementation
- [ ] Verify Wave1-01 is merged to main
- [ ] Pull latest main branch
- [ ] Create feature branch: `feature/tmc-syncer-01c-api-helpers`
- [ ] Verify API types exist from Wave1-01

### Implementation Steps

1. **Verify Prerequisites**
   ```bash
   # Check API types exist
   ls -la pkg/apis/workload/v1alpha1/synctarget_types.go
   
   # Check if validation/defaults exist (from Wave1-02, optional)
   ls -la pkg/apis/workload/v1alpha1/synctarget_validation.go 2>/dev/null || echo "Validation not yet added"
   ```

2. **Copy Helper Functions**
   ```bash
   cp /workspaces/kcp-worktrees/phase2/wave1-api-foundation-to-be-split/pkg/apis/workload/v1alpha1/synctarget_helpers.go \
      pkg/apis/workload/v1alpha1/synctarget_helpers.go
   ```

3. **Copy Conversion Functions**
   ```bash
   cp /workspaces/kcp-worktrees/phase2/wave1-api-foundation-to-be-split/pkg/apis/workload/v1alpha1/synctarget_conversion.go \
      pkg/apis/workload/v1alpha1/synctarget_conversion.go
   ```

4. **Copy Test File**
   ```bash
   cp /workspaces/kcp-worktrees/phase2/wave1-api-foundation-to-be-split/pkg/apis/workload/v1alpha1/synctarget_types_test.go \
      pkg/apis/workload/v1alpha1/synctarget_types_test.go
   ```

5. **Verify Helper Functions**
   Review helpers for:
   - Condition management (GetCondition, SetCondition, etc.)
   - Status helpers (IsReady, SetReady, etc.)
   - Capacity helpers (GetTotalCapacity, etc.)

6. **Test Compilation**
   ```bash
   go build ./pkg/apis/workload/v1alpha1/...
   ```

7. **Run Tests**
   ```bash
   go test ./pkg/apis/workload/v1alpha1/... -v
   ```

### Key Helper Functions to Verify

```go
// Condition Helpers - should be present
- GetCondition(conditions []Condition, conditionType ConditionType) *Condition
- SetCondition(conditions []Condition, newCondition Condition) []Condition
- RemoveCondition(conditions []Condition, conditionType ConditionType) []Condition

// Status Helpers
- (st *SyncTarget) IsReady() bool
- (st *SyncTarget) SetReady(ready bool)
- (st *SyncTarget) GetHeartbeatTime() *metav1.Time

// Capacity Helpers  
- (st *SyncTarget) GetTotalCapacity() resource.Quantity
- (st *SyncTarget) GetAvailableCapacity() resource.Quantity
```

### Validation Steps

1. **Run All Tests**
   ```bash
   go test ./pkg/apis/workload/v1alpha1/... -v -count=1
   ```

2. **Check Test Coverage**
   ```bash
   go test ./pkg/apis/workload/v1alpha1/... -cover
   ```

3. **Verify Line Count**
   ```bash
   /workspaces/kcp-shared-tools/tmc-pr-line-counter.sh -c feature/tmc-syncer-01c-api-helpers
   ```
   - Should be ~590 lines

4. **Check Helper Functions Work**
   ```go
   // Quick test
   st := &SyncTarget{}
   st.SetReady(true)
   if !st.IsReady() {
       panic("Helper not working")
   }
   ```

### Commit Strategy

```bash
# Stage helper files
git add pkg/apis/workload/v1alpha1/synctarget_helpers.go
git commit -s -S -m "feat(api): add helper functions for SyncTarget management

- Add condition management helpers
- Add status update utilities
- Add capacity calculation helpers
- Add label and annotation helpers
- Provide convenience methods for common operations"

# Stage conversion
git add pkg/apis/workload/v1alpha1/synctarget_conversion.go
git commit -s -S -m "feat(api): add conversion support for SyncTarget

- Set up hub version markers
- Add conversion webhook framework
- Prepare for future API versioning"

# Stage tests
git add pkg/apis/workload/v1alpha1/synctarget_types_test.go
git commit -s -S -m "test: add comprehensive tests for SyncTarget API

- Test helper functions
- Validate condition management
- Test capacity calculations
- Ensure defaults and validation work correctly"
```

### Post-Implementation
- [ ] All tests pass
- [ ] Helpers are properly documented
- [ ] Line count under 800
- [ ] No compilation errors
- [ ] Clean git status
- [ ] Push branch and create PR

## Success Criteria

1. ✅ All helper functions compile
2. ✅ Tests achieve good coverage (>80%)
3. ✅ Under 800 lines total
4. ✅ Conversion framework in place
5. ✅ No runtime panics in helpers
6. ✅ Documentation for all exported functions

## Potential Issues & Solutions

1. **Test Failures**
   - May need Wave1-02 (validation/defaults) for some tests
   - Can mock or skip those tests temporarily

2. **Missing Dependencies**
   - Ensure Wave1-01 types are available
   - May need to adjust imports

3. **Conversion Setup**
   - Conversion may need webhook configuration
   - Can be added in later PR if needed

## Dependencies on Other Splits

- **Requires:** Wave1-01 (API Types)
- **Optional:** Wave1-02 (Validation) - some tests may use it
- **Required By:** Controllers will use these helpers
- **Can Parallel With:** Wave1-02

## Notes for Parallel Agents

- Can work simultaneously with Wave1-02
- Share no files with Wave1-02
- Both depend on Wave1-01 types
- Helpers will be used by future controller implementations
- Branch: `feature/tmc-syncer-01c-api-helpers`

## Testing Focus Areas

1. **Condition Management**
   - Setting conditions
   - Updating existing conditions
   - Removing conditions
   - Finding specific conditions

2. **Status Helpers**
   - Ready state management
   - Heartbeat tracking
   - Phase transitions

3. **Capacity Calculations**
   - Total capacity
   - Available capacity
   - Resource arithmetic

4. **Edge Cases**
   - Nil pointer handling
   - Empty slices
   - Invalid inputs
