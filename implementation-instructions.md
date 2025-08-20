# Implementation Instructions: Downstream Syncer Core

## Branch: `feature/phase7-syncer-impl/p7w2-downstream-core`

## Overview
This branch implements the core downstream synchronization logic that handles the actual application of resources from KCP to physical clusters. It manages create, update, and delete operations while handling conflicts and preserving downstream-specific fields.

**Target Size**: ~700 lines  
**Complexity**: High  
**Priority**: Critical (core sync functionality)

## Dependencies
- **Phase 5 APIs**: Uses syncer interfaces
- **Phase 6 Infrastructure**: Controller patterns
- **Wave 1 Sync Engine**: Integrates with engine
- **Wave 1 Transformation**: Uses transformation pipeline

## Files to Create

### 1. Downstream Syncer Core (~300 lines)
**File**: `pkg/reconciler/workload/syncer/downstream/syncer.go`
- Main downstream syncer struct
- Apply/delete methods
- Conflict detection
- Field preservation logic

### 2. Resource Differ (~150 lines)
**File**: `pkg/reconciler/workload/syncer/downstream/differ.go`
- Resource comparison logic
- Field-level diff generation
- Meaningful change detection
- Ignore rules for server-side fields

### 3. Field Preservation (~100 lines)
**File**: `pkg/reconciler/workload/syncer/downstream/preservation.go`
- Downstream field preservation
- Status field handling
- Server-managed field preservation
- Merge strategies

### 4. Downstream Types (~50 lines)
**File**: `pkg/reconciler/workload/syncer/downstream/types.go`
- DownstreamSyncer configuration
- Sync result types
- Error types
- Helper structures

### 5. Downstream Tests (~100 lines)
**File**: `pkg/reconciler/workload/syncer/downstream/syncer_test.go`
- Unit tests for sync operations
- Conflict resolution tests
- Field preservation tests

## Step-by-Step Implementation Guide

### Step 1: Create Package Structure
```bash
mkdir -p pkg/reconciler/workload/syncer/downstream
```

### Step 2: Define Types
Create `types.go` with:

```go
package downstream

import (
    "k8s.io/apimachinery/pkg/runtime/schema"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SyncResult represents the result of a sync operation
type SyncResult struct {
    Operation   string // create, update, delete, noop
    Success     bool
    Error       error
    RetryAfter  *time.Duration
    Conflicts   []string
}

// DownstreamConfig holds downstream syncer configuration
type DownstreamConfig struct {
    ConflictRetries    int
    UpdateStrategy     string // replace, merge, strategic-merge
    PreserveFields     []string
    IgnoreFields       []string
    DeletionPropagation metav1.DeletionPropagation
}

// ResourceState tracks downstream resource state
type ResourceState struct {
    GVR              schema.GroupVersionResource
    Namespace        string
    Name             string
    ResourceVersion  string
    Generation       int64
    LastSyncTime     metav1.Time
    Hash             string
}
```

### Step 3: Implement Core Syncer
Create `syncer.go` with:

1. **Syncer struct**:
```go
type Syncer struct {
    kcpClient        dynamic.ClusterInterface
    downstreamClient dynamic.Interface
    syncTarget       *workloadv1alpha1.SyncTarget
    
    transformer      interfaces.ResourceTransformer
    conflictResolver interfaces.ConflictResolver
    differ           *ResourceDiffer
    
    config          DownstreamConfig
    stateCache      map[string]*ResourceState
    mu              sync.RWMutex
}
```

2. **ApplyToDownstream method**:
   - Check if resource exists
   - Determine if create or update
   - Apply transformations
   - Handle conflicts
   - Preserve downstream fields
   - Execute operation
   - Update state cache

3. **DeleteFromDownstream method**:
   - Verify resource exists
   - Check deletion conditions
   - Apply deletion propagation
   - Clean up state cache
   - Handle finalizers

4. **Conflict handling**:
   - Detect conflicts via resource version
   - Attempt automatic resolution
   - Retry with backoff
   - Report unresolvable conflicts

### Step 4: Implement Resource Differ
Create `differ.go` with:

1. **ResourceDiffer struct**:
```go
type ResourceDiffer struct {
    ignoreFields []string
    significantFields []string
}
```

2. **Diff method**:
   - Deep comparison of objects
   - Ignore server-managed fields
   - Detect meaningful changes
   - Generate change summary

3. **HasSignificantChanges method**:
   - Check if changes require update
   - Filter out status-only changes
   - Consider generation changes

### Step 5: Implement Field Preservation
Create `preservation.go` with:

1. **PreserveDownstreamFields function**:
```go
func PreserveDownstreamFields(existing, desired *unstructured.Unstructured) *unstructured.Unstructured {
    merged := desired.DeepCopy()
    
    // Preserve resource version for updates
    merged.SetResourceVersion(existing.GetResourceVersion())
    
    // Preserve status (synced separately)
    if status, found, _ := unstructured.NestedFieldNoCopy(existing.Object, "status"); found {
        unstructured.SetNestedField(merged.Object, status, "status")
    }
    
    // Preserve other downstream-managed fields
    preserveServerManagedFields(existing, merged)
    
    return merged
}
```

2. **Field preservation rules**:
   - Status field preservation
   - Server-side default preservation
   - Finalizer coordination
   - Annotation merging

### Step 6: Handle Special Resources
Add special handling for:

1. **ConfigMaps and Secrets**:
   - Binary data handling
   - Size validation
   - Immutable field checks

2. **Services**:
   - ClusterIP preservation
   - NodePort preservation
   - LoadBalancer status

3. **PersistentVolumes**:
   - Binding preservation
   - Reclaim policy handling

### Step 7: Add Comprehensive Tests
Create test files covering:

1. **Create operations**:
   - Simple resource creation
   - Complex resource with status
   - Creation with conflicts

2. **Update operations**:
   - Simple updates
   - Conflict resolution
   - Field preservation

3. **Delete operations**:
   - Simple deletion
   - Cascade deletion
   - Finalizer handling

## Testing Requirements

### Unit Tests:
- Create, update, delete operations
- Conflict detection and resolution
- Field preservation logic
- Differ functionality
- State cache management
- Error handling

### Integration Tests:
- Full sync flow with real resources
- Conflict scenarios
- Large resource handling
- Concurrent operations

## Validation Checklist

- [ ] Proper error handling for all operations
- [ ] Conflicts detected and handled correctly
- [ ] Downstream fields preserved appropriately
- [ ] State cache maintains consistency
- [ ] No data loss during sync
- [ ] Proper cleanup on deletion
- [ ] Comprehensive logging
- [ ] Metrics for operations
- [ ] Tests achieve >70% coverage
- [ ] Code follows KCP patterns
- [ ] Feature flags integrated
- [ ] Under 700 lines (excluding tests)

## Common Pitfalls to Avoid

1. **Don't overwrite downstream changes** - preserve legitimate fields
2. **Handle conflicts gracefully** - don't fail immediately
3. **Clean up properly** - remove from cache on deletion
4. **Validate before applying** - check resource validity
5. **Log appropriately** - include context in errors

## Integration Notes

This component:
- Receives work from Wave 1 sync engine
- Uses Wave 1 transformation pipeline
- Works with Wave 2 applier for execution
- Coordinates with Wave 2 conflict resolver

Should provide:
- Sync operation results
- Conflict information
- Operation metrics
- State query interface

## Success Criteria

The implementation is complete when:
1. Resources can be created in downstream cluster
2. Updates are applied correctly with field preservation
3. Deletions are handled properly
4. Conflicts are detected and resolved
5. All tests pass
6. Can handle 100+ resources without issues