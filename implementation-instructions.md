# Implementation Instructions: Conflict Resolution

## Branch: `feature/phase7-syncer-impl/p7w2-conflict`

## Overview
This branch implements the conflict resolution system that handles conflicts when syncing resources between KCP and physical clusters. It provides multiple resolution strategies and maintains consistency across clusters.

**Target Size**: ~500 lines  
**Complexity**: Medium-High  
**Priority**: High (ensures sync reliability)

## Dependencies
- **Phase 5 APIs**: Implements conflict resolver interface
- **Phase 6 Infrastructure**: Uses controller patterns
- **Wave 2 Downstream Core**: Used by downstream syncer
- **Wave 2 Applier**: Coordinates with applier

## Files to Create

### 1. Conflict Resolver Core (~200 lines)
**File**: `pkg/reconciler/workload/syncer/conflict/resolver.go`
- Main resolver struct
- Conflict detection logic
- Resolution strategy selection
- Conflict resolution execution

### 2. Resolution Strategies (~150 lines)
**File**: `pkg/reconciler/workload/syncer/conflict/strategies.go`
- KCP wins strategy
- Downstream wins strategy
- Merge strategy
- Manual resolution marker

### 3. Conflict Detection (~80 lines)
**File**: `pkg/reconciler/workload/syncer/conflict/detection.go`
- Version conflict detection
- Semantic conflict detection
- Field-level conflict identification
- Conflict severity assessment

### 4. Conflict Types (~30 lines)
**File**: `pkg/reconciler/workload/syncer/conflict/types.go`
- Conflict struct definitions
- Resolution result types
- Strategy configuration

### 5. Conflict Tests (~40 lines)
**File**: `pkg/reconciler/workload/syncer/conflict/resolver_test.go`
- Unit tests for resolution strategies
- Conflict detection tests
- Integration scenarios

## Step-by-Step Implementation Guide

### Step 1: Create Package Structure
```bash
mkdir -p pkg/reconciler/workload/syncer/conflict
```

### Step 2: Define Types
Create `types.go` with:

```go
package conflict

import (
    "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
    "k8s.io/apimachinery/pkg/runtime/schema"
)

// Conflict represents a synchronization conflict
type Conflict struct {
    GVR              schema.GroupVersionResource
    Namespace        string
    Name             string
    Type             ConflictType
    Severity         ConflictSeverity
    KCPVersion       string
    DownstreamVersion string
    Fields           []FieldConflict
    DetectedAt       metav1.Time
}

// ConflictType categorizes conflicts
type ConflictType string

const (
    VersionConflict   ConflictType = "version"
    SemanticConflict  ConflictType = "semantic"
    DeletedConflict   ConflictType = "deleted"
    OwnershipConflict ConflictType = "ownership"
)

// ConflictSeverity indicates conflict importance
type ConflictSeverity string

const (
    LowSeverity    ConflictSeverity = "low"
    MediumSeverity ConflictSeverity = "medium"
    HighSeverity   ConflictSeverity = "high"
    CriticalSeverity ConflictSeverity = "critical"
)

// ResolutionStrategy defines how to resolve conflicts
type ResolutionStrategy string

const (
    KCPWins       ResolutionStrategy = "kcp-wins"
    DownstreamWins ResolutionStrategy = "downstream-wins"
    Merge         ResolutionStrategy = "merge"
    Manual        ResolutionStrategy = "manual"
)

// ResolutionResult contains the outcome of conflict resolution
type ResolutionResult struct {
    Resolved  bool
    Strategy  ResolutionStrategy
    Merged    *unstructured.Unstructured
    Error     error
    Conflicts []FieldConflict
}

// FieldConflict represents a field-level conflict
type FieldConflict struct {
    Path          string
    KCPValue      interface{}
    DownstreamValue interface{}
    Resolution    string
}
```

### Step 3: Implement Conflict Resolver
Create `resolver.go` with:

```go
package conflict

import (
    "context"
    "fmt"
    
    "github.com/kcp-dev/kcp/pkg/syncer/interfaces"
    "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
    "k8s.io/klog/v2"
)

// Resolver handles conflict resolution
type Resolver struct {
    defaultStrategy ResolutionStrategy
    strategies      map[ResolutionStrategy]interfaces.ResolutionStrategy
    detector        *ConflictDetector
}

// NewResolver creates a new conflict resolver
func NewResolver(defaultStrategy ResolutionStrategy) *Resolver {
    r := &Resolver{
        defaultStrategy: defaultStrategy,
        strategies:      make(map[ResolutionStrategy]interfaces.ResolutionStrategy),
        detector:        NewConflictDetector(),
    }
    
    // Register default strategies
    r.strategies[KCPWins] = &KCPWinsStrategy{}
    r.strategies[DownstreamWins] = &DownstreamWinsStrategy{}
    r.strategies[Merge] = &MergeStrategy{}
    r.strategies[Manual] = &ManualStrategy{}
    
    return r
}

// ResolveConflict attempts to resolve a synchronization conflict
func (r *Resolver) ResolveConflict(ctx context.Context, kcp, downstream *unstructured.Unstructured) (*ResolutionResult, error) {
    logger := klog.FromContext(ctx)
    
    // Detect conflicts
    conflict := r.detector.DetectConflict(kcp, downstream)
    if conflict == nil {
        return &ResolutionResult{Resolved: true}, nil
    }
    
    logger.V(2).Info("Conflict detected", 
        "type", conflict.Type,
        "severity", conflict.Severity,
        "resource", fmt.Sprintf("%s/%s", conflict.Namespace, conflict.Name))
    
    // Select resolution strategy
    strategy := r.selectStrategy(conflict)
    
    // Apply resolution strategy
    resolver, exists := r.strategies[strategy]
    if !exists {
        return nil, fmt.Errorf("unknown resolution strategy: %s", strategy)
    }
    
    result, err := resolver.Resolve(ctx, kcp, downstream, conflict)
    if err != nil {
        logger.Error(err, "Failed to resolve conflict", "strategy", strategy)
        return nil, err
    }
    
    result.Strategy = strategy
    
    if result.Resolved {
        logger.V(2).Info("Conflict resolved", "strategy", strategy)
    } else {
        logger.Warning("Conflict requires manual resolution", "conflicts", result.Conflicts)
    }
    
    return result, nil
}

// selectStrategy chooses the appropriate resolution strategy
func (r *Resolver) selectStrategy(conflict *Conflict) ResolutionStrategy {
    // Critical conflicts require manual resolution
    if conflict.Severity == CriticalSeverity {
        return Manual
    }
    
    // Use type-specific strategies
    switch conflict.Type {
    case DeletedConflict:
        return KCPWins // Recreate if deleted downstream
    case OwnershipConflict:
        return Manual // Ownership conflicts need investigation
    case VersionConflict:
        if conflict.Severity == HighSeverity {
            return Manual
        }
        return r.defaultStrategy
    default:
        return r.defaultStrategy
    }
}
```

### Step 4: Implement Resolution Strategies
Create `strategies.go` with:

1. **KCP Wins Strategy**:
```go
type KCPWinsStrategy struct{}

func (s *KCPWinsStrategy) Resolve(ctx context.Context, kcp, downstream *unstructured.Unstructured, conflict *Conflict) (*ResolutionResult, error) {
    // KCP version takes precedence
    merged := kcp.DeepCopy()
    
    // Preserve downstream-only fields
    preserveDownstreamOnlyFields(downstream, merged)
    
    return &ResolutionResult{
        Resolved: true,
        Merged:   merged,
    }, nil
}
```

2. **Downstream Wins Strategy**:
   - Preserve downstream changes
   - Update KCP metadata
   - Mark for upstream sync

3. **Merge Strategy**:
   - Three-way merge
   - Field-level merging
   - Conflict markers for unresolvable fields

4. **Manual Strategy**:
   - Mark resource for manual review
   - Add conflict annotations
   - Pause sync for resource

### Step 5: Implement Conflict Detection
Create `detection.go` with:

1. **ConflictDetector struct**:
```go
type ConflictDetector struct {
    ignoreFields []string
}
```

2. **DetectConflict method**:
   - Compare resource versions
   - Check for semantic differences
   - Identify conflicting fields
   - Assess conflict severity

3. **Field comparison**:
   - Deep equality check
   - Ignore server-managed fields
   - Detect meaningful changes

### Step 6: Add Advanced Features

1. **Conflict history**:
   - Track resolution history
   - Learn from patterns
   - Suggest resolutions

2. **Custom strategies**:
   - Plugin architecture
   - Resource-specific strategies
   - User-defined rules

3. **Notifications**:
   - Alert on critical conflicts
   - Resolution reports
   - Audit logging

### Step 7: Add Comprehensive Tests
Create test files covering:

1. **Detection tests**:
   - Version conflicts
   - Field conflicts
   - Severity assessment

2. **Resolution tests**:
   - Each strategy
   - Complex scenarios
   - Edge cases

## Testing Requirements

### Unit Tests:
- Conflict detection accuracy
- Each resolution strategy
- Strategy selection logic
- Field preservation
- Error handling

### Integration Tests:
- Real resource conflicts
- Multiple conflict types
- Resolution chains
- Performance under load

## Validation Checklist

- [ ] All conflict types detected correctly
- [ ] Resolution strategies work as expected
- [ ] Field preservation maintains integrity
- [ ] Manual conflicts properly marked
- [ ] Comprehensive logging
- [ ] Metrics for conflict tracking
- [ ] Tests achieve >75% coverage
- [ ] Code follows KCP patterns
- [ ] Feature flags integrated
- [ ] Under 500 lines (excluding tests)

## Common Pitfalls to Avoid

1. **Don't lose data** - always preserve important fields
2. **Avoid infinite loops** - detect resolution cycles
3. **Handle edge cases** - nil values, missing fields
4. **Document decisions** - log why strategies were chosen
5. **Maintain consistency** - ensure resolved state is valid

## Integration Notes

This component:
- Is used by Wave 2 downstream core
- Coordinates with Wave 2 applier
- May trigger Wave 3 upstream sync
- Reports conflicts for monitoring

Should provide:
- Multiple resolution strategies
- Extensible strategy system
- Detailed conflict information
- Resolution metrics

## Success Criteria

The implementation is complete when:
1. Conflicts are accurately detected
2. Multiple resolution strategies work
3. Field preservation maintains data integrity
4. Manual conflicts are properly handled
5. All tests pass
6. Can resolve 95% of conflicts automatically