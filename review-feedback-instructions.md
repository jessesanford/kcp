# Review Feedback Instructions - Wave 1-01: SyncTarget API Types

## Current State
- Branch: `feature/tmc2-impl2/phase2/wave1-01-split-from-api-foundation`
- Focus: SyncTarget API types with proper registration
- Current files: ~200 lines (estimated)

## Priority Issues (P0 - Must Fix)

### 1. Missing Test Coverage
**CRITICAL**: No tests exist for API types

#### Required Test Files to Create:
1. `pkg/apis/workload/v1alpha1/synctarget_types_test.go` (~150 lines)
   - Test defaulting behavior
   - Test validation logic
   - Test condition helpers

2. `pkg/apis/workload/v1alpha1/register_test.go` (~80 lines)
   - Test scheme registration
   - Test GVK correctness

### 2. CRD Generation
**CRITICAL**: No CRD manifest exists

#### Commands to Run:
```bash
# Generate CRDs (from worktree root)
make update-codegen-crds

# Verify CRD output
ls -la config/crds/
```

### 3. Complete API Implementation

#### File: `pkg/apis/workload/v1alpha1/synctarget_types.go`
Add missing helper methods (~100 lines):
```go
// GetCondition returns the condition with the given type
func (st *SyncTarget) GetCondition(conditionType ConditionType) *metav1.Condition {
    for i := range st.Status.Conditions {
        if st.Status.Conditions[i].Type == string(conditionType) {
            return &st.Status.Conditions[i]
        }
    }
    return nil
}

// SetCondition updates or adds a condition
func (st *SyncTarget) SetCondition(condition metav1.Condition) {
    existingIndex := -1
    for i := range st.Status.Conditions {
        if st.Status.Conditions[i].Type == condition.Type {
            existingIndex = i
            break
        }
    }
    
    if existingIndex != -1 {
        st.Status.Conditions[existingIndex] = condition
    } else {
        st.Status.Conditions = append(st.Status.Conditions, condition)
    }
}
```

### 4. Add Validation Webhooks Support

#### File: `pkg/apis/workload/v1alpha1/synctarget_validation.go` (NEW ~120 lines)
```go
package v1alpha1

import (
    "fmt"
    "k8s.io/apimachinery/pkg/runtime"
    "k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateCreate validates a SyncTarget on creation
func (st *SyncTarget) ValidateCreate() error {
    allErrs := field.ErrorList{}
    
    // Validate KubeConfig reference
    if st.Spec.KubeConfig == "" {
        allErrs = append(allErrs, field.Required(
            field.NewPath("spec", "kubeConfig"),
            "kubeConfig is required"))
    }
    
    // Validate cells if specified
    if len(st.Spec.Cells) > 0 {
        for i, cell := range st.Spec.Cells {
            if cell == "" {
                allErrs = append(allErrs, field.Invalid(
                    field.NewPath("spec", "cells").Index(i),
                    cell,
                    "cell name cannot be empty"))
            }
        }
    }
    
    if len(allErrs) > 0 {
        return allErrs.ToAggregate()
    }
    return nil
}

// ValidateUpdate validates a SyncTarget on update
func (st *SyncTarget) ValidateUpdate(old runtime.Object) error {
    // Add update validation logic
    return st.ValidateCreate()
}
```

## Line Count Analysis

### Current Estimate:
- Existing code: ~200 lines
- Required tests: ~230 lines  
- Helper methods: ~100 lines
- Validation: ~120 lines
- **Total after fixes: ~650 lines** ✅ WITHIN LIMIT

## Specific Tasks

### 1. Test Implementation Priority
1. Create `synctarget_types_test.go`:
   ```go
   func TestSyncTargetConditions(t *testing.T) {
       st := &SyncTarget{}
       
       // Test setting condition
       condition := metav1.Condition{
           Type:   string(SyncTargetReady),
           Status: metav1.ConditionTrue,
       }
       st.SetCondition(condition)
       
       // Test getting condition
       got := st.GetCondition(SyncTargetReady)
       require.NotNil(t, got)
       require.Equal(t, metav1.ConditionTrue, got.Status)
   }
   ```

2. Create `register_test.go`:
   ```go
   func TestSchemeRegistration(t *testing.T) {
       scheme := runtime.NewScheme()
       require.NoError(t, AddToScheme(scheme))
       
       // Verify GVK registration
       gvk, err := apiutil.GVKForObject(&SyncTarget{}, scheme)
       require.NoError(t, err)
       require.Equal(t, SchemeGroupVersion.WithKind("SyncTarget"), gvk)
   }
   ```

### 2. CRD Generation
```bash
# Run from worktree root
cd /workspaces/kcp-worktrees/phase2/wave1-01-split-from-api-foundation
make update-codegen-crds

# Commit generated files
git add config/crds/
git commit -s -S -m "chore: generate CRD manifests for SyncTarget"
```

### 3. Documentation
Add inline documentation for all exported types and methods:
- Document condition types and their meanings
- Document status fields and when they're updated
- Add examples in comments

## Testing Requirements

### Unit Test Coverage Target: 80%
1. **API Types Tests**:
   - Default values application
   - Condition management
   - Status updates
   
2. **Validation Tests**:
   - Required fields validation
   - Field format validation
   - Update restrictions

3. **Registration Tests**:
   - Scheme registration
   - GVK correctness
   - Deep copy generation

## Completion Checklist

- [ ] All test files created with >80% coverage
- [ ] CRD manifests generated and committed
- [ ] Helper methods implemented (GetCondition, SetCondition)
- [ ] Validation webhook support added
- [ ] All exported types/methods documented
- [ ] `make test` passes
- [ ] `make verify` passes
- [ ] Line count verified < 700 lines
- [ ] No placeholder TODOs remain
- [ ] Git history is clean and logical

## Notes
- This PR is self-contained and can merge independently
- Focus on API contract stability - this is foundation for other work
- Ensure backward compatibility considerations are documented