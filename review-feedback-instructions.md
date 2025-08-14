# Review Feedback Instructions - Wave 1-03: ClusterWorkloadPlacement API Types

## Current State
- Branch: `feature/tmc2-impl2/phase2/wave1-03-split-from-api-foundation`
- Focus: ClusterWorkloadPlacement API for placement policies
- Estimated current lines: ~220 lines

## Priority Issues (P0 - Must Fix)

### 1. Missing Test Coverage
**CRITICAL**: No tests for placement logic

#### Required Test Files to Create:
1. `pkg/apis/workload/v1alpha1/clusterworkloadplacement_types_test.go` (~180 lines)
   - Test placement selector logic
   - Test status aggregation
   - Test condition management

2. `pkg/apis/workload/v1alpha1/placement_selector_test.go` (~120 lines)
   - Test label selector matching
   - Test affinity/anti-affinity rules

### 2. CRD Generation
**CRITICAL**: Generate CRD manifests

#### Commands to Run:
```bash
# Generate CRDs
make update-codegen-crds

# Verify CRD structure
kubectl explain clusterworkloadplacement --recursive=false
```

### 3. Complete Placement Logic Implementation

#### File: `pkg/apis/workload/v1alpha1/clusterworkloadplacement_types.go`
Add placement evaluation methods (~150 lines):
```go
// EvaluatePlacement checks if a target matches placement rules
func (cwp *ClusterWorkloadPlacement) EvaluatePlacement(target *SyncTarget) (bool, string) {
    // Check namespace selector
    if cwp.Spec.NamespaceSelector != nil {
        selector, err := metav1.LabelSelectorAsSelector(cwp.Spec.NamespaceSelector)
        if err != nil {
            return false, fmt.Sprintf("invalid namespace selector: %v", err)
        }
        
        if !selector.Matches(labels.Set(target.Labels)) {
            return false, "namespace selector does not match"
        }
    }
    
    // Check location selector
    if cwp.Spec.LocationSelector != nil {
        if !cwp.evaluateLocationSelector(target) {
            return false, "location requirements not met"
        }
    }
    
    // Check resource requirements
    if cwp.Spec.ResourceRequirements != nil {
        if !cwp.evaluateResourceRequirements(target) {
            return false, "insufficient resources"
        }
    }
    
    return true, ""
}

// evaluateLocationSelector checks location constraints
func (cwp *ClusterWorkloadPlacement) evaluateLocationSelector(target *SyncTarget) bool {
    if cwp.Spec.LocationSelector == nil {
        return true
    }
    
    // Check required locations
    for _, required := range cwp.Spec.LocationSelector.RequiredLocations {
        if target.Spec.Location != required {
            return false
        }
    }
    
    // Check preferred locations (for scoring, not filtering)
    // This affects ranking but not eligibility
    
    return true
}

// evaluateResourceRequirements checks resource availability
func (cwp *ClusterWorkloadPlacement) evaluateResourceRequirements(target *SyncTarget) bool {
    if cwp.Spec.ResourceRequirements == nil {
        return true
    }
    
    // Check minimum resource requirements
    // This would integrate with ResourceQuota in practice
    
    return true // Simplified for now
}

// ScorePlacement ranks a target for placement
func (cwp *ClusterWorkloadPlacement) ScorePlacement(target *SyncTarget) int32 {
    score := int32(0)
    
    // Score based on preferred locations
    if cwp.Spec.LocationSelector != nil {
        for i, preferred := range cwp.Spec.LocationSelector.PreferredLocations {
            if target.Spec.Location == preferred {
                score += int32(100 - i*10) // Higher score for earlier preferences
            }
        }
    }
    
    // Additional scoring factors can be added
    
    return score
}
```

### 4. Add Placement Strategy Support

#### File: `pkg/apis/workload/v1alpha1/placement_strategy.go` (NEW ~100 lines)
```go
package v1alpha1

// PlacementStrategy defines how workloads are distributed
type PlacementStrategy string

const (
    // PlacementStrategyBinPack minimizes number of targets used
    PlacementStrategyBinPack PlacementStrategy = "BinPack"
    
    // PlacementStrategySpread distributes across targets
    PlacementStrategySpread PlacementStrategy = "Spread"
    
    // PlacementStrategyRandom uses random selection
    PlacementStrategyRandom PlacementStrategy = "Random"
)

// PlacementPolicy defines placement behavior
type PlacementPolicy struct {
    // Strategy for workload distribution
    Strategy PlacementStrategy `json:"strategy,omitempty"`
    
    // MaxTargets limits number of placement targets
    MaxTargets *int32 `json:"maxTargets,omitempty"`
    
    // MinTargets ensures minimum redundancy
    MinTargets *int32 `json:"minTargets,omitempty"`
}

// Add to ClusterWorkloadPlacementSpec:
// PlacementPolicy defines distribution strategy
// PlacementPolicy *PlacementPolicy `json:"placementPolicy,omitempty"`
```

## Line Count Analysis

### Current Estimate:
- Existing code: ~220 lines
- Required tests: ~300 lines
- Placement evaluation: ~150 lines
- Strategy support: ~100 lines
- **Total after fixes: ~770 lines** ⚠️ OVER LIMIT

### NEEDS SPLIT Strategy:
Split into 2 PRs:
1. **Current PR**: Core placement types + basic tests (~500 lines)
2. **Follow-up PR**: Advanced placement strategies + comprehensive tests (~400 lines)

## Specific Tasks for THIS PR

### 1. Focus on Core Functionality Only
1. Create minimal `clusterworkloadplacement_types_test.go`:
   ```go
   func TestBasicPlacementEvaluation(t *testing.T) {
       cwp := &ClusterWorkloadPlacement{
           Spec: ClusterWorkloadPlacementSpec{
               NamespaceSelector: &metav1.LabelSelector{
                   MatchLabels: map[string]string{
                       "env": "production",
                   },
               },
           },
       }
       
       target := &SyncTarget{
           ObjectMeta: metav1.ObjectMeta{
               Labels: map[string]string{
                   "env": "production",
               },
           },
       }
       
       match, reason := cwp.EvaluatePlacement(target)
       require.True(t, match)
       require.Empty(t, reason)
   }
   ```

2. Implement only `EvaluatePlacement` method (skip scoring for now)

### 2. CRD Generation
```bash
cd /workspaces/kcp-worktrees/phase2/wave1-03-split-from-api-foundation
make update-codegen-crds
git add config/crds/
git commit -s -S -m "chore: generate ClusterWorkloadPlacement CRD"
```

### 3. Defer Advanced Features
Move to follow-up PR:
- Scoring logic
- Placement strategies
- Resource requirement evaluation
- Comprehensive test coverage

## Testing Requirements (This PR)

### Unit Test Coverage Target: 70%
1. **Basic Placement Tests**:
   - Namespace selector matching
   - Label selector evaluation
   - Condition updates

2. **Validation Tests**:
   - Selector validation
   - Basic field validation

## Completion Checklist (This PR)

- [ ] Basic test file created (70% coverage)
- [ ] CRD manifest generated
- [ ] `EvaluatePlacement` method implemented
- [ ] Basic validation added
- [ ] API documentation complete
- [ ] `make test` passes
- [ ] `make verify` passes
- [ ] Line count < 700 lines
- [ ] TODO added for follow-up PR
- [ ] Clean commit history

## Follow-up PR Planning
Create new TODO for Wave 1-03b with:
- Placement scoring implementation
- Strategy support
- Resource evaluation
- Comprehensive test suite
- Performance optimization

## Notes
- Keep this PR focused on core placement contract
- Document deferred features in code comments
- Ensure API stability for future extensions
- Create issue for tracking follow-up work