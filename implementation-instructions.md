# Implementation Instructions: Resource Applier

## Branch: `feature/phase7-syncer-impl/p7w2-applier`

## Overview
This branch implements the resource applier that executes the actual API calls to create, update, and delete resources in the downstream cluster. It handles retry logic, optimistic concurrency control, and provides detailed operation results.

**Target Size**: ~600 lines  
**Complexity**: Medium  
**Priority**: High (executes actual sync operations)

## Dependencies
- **Phase 5 APIs**: Uses applier interfaces
- **Phase 6 Infrastructure**: Controller utilities
- **Wave 2 Downstream Core**: Called by downstream syncer
- **Wave 2 Conflict Resolution**: Uses conflict resolver

## Files to Create

### 1. Resource Applier Core (~250 lines)
**File**: `pkg/reconciler/workload/syncer/applier/applier.go`
- Main applier struct
- Apply method with retry logic
- Delete method with propagation
- Patch method for partial updates

### 2. Retry Strategy (~100 lines)
**File**: `pkg/reconciler/workload/syncer/applier/retry.go`
- Exponential backoff implementation
- Retry condition evaluation
- Jitter for distributed systems
- Circuit breaker pattern

### 3. Apply Strategy (~100 lines)
**File**: `pkg/reconciler/workload/syncer/applier/strategy.go`
- Server-side apply implementation
- Strategic merge patch
- JSON merge patch
- Replace strategy

### 4. Result Aggregation (~50 lines)
**File**: `pkg/reconciler/workload/syncer/applier/results.go`
- Operation result types
- Result aggregation
- Error classification
- Success metrics

### 5. Applier Tests (~100 lines)
**File**: `pkg/reconciler/workload/syncer/applier/applier_test.go`
- Unit tests for apply operations
- Retry logic tests
- Strategy tests

## Step-by-Step Implementation Guide

### Step 1: Create Package Structure
```bash
mkdir -p pkg/reconciler/workload/syncer/applier
```

### Step 2: Define Core Applier
Create `applier.go` with:

```go
package applier

import (
    "context"
    "fmt"
    
    "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
    "k8s.io/apimachinery/pkg/runtime/schema"
    "k8s.io/client-go/dynamic"
    "k8s.io/klog/v2"
)

// Applier handles resource application to clusters
type Applier struct {
    client          dynamic.Interface
    retryStrategy   *RetryStrategy
    applyStrategy   ApplyStrategy
    fieldManager    string
    forceConflicts  bool
}

// NewApplier creates a new resource applier
func NewApplier(client dynamic.Interface, fieldManager string) *Applier {
    return &Applier{
        client:        client,
        retryStrategy: NewDefaultRetryStrategy(),
        applyStrategy: ServerSideApply,
        fieldManager:  fieldManager,
    }
}

// Apply creates or updates a resource with retry logic
func (a *Applier) Apply(ctx context.Context, obj *unstructured.Unstructured) (*ApplyResult, error) {
    logger := klog.FromContext(ctx)
    gvr := a.getGVR(obj)
    
    result := &ApplyResult{
        GVR:       gvr,
        Namespace: obj.GetNamespace(),
        Name:      obj.GetName(),
    }
    
    err := a.retryStrategy.Execute(ctx, func() error {
        switch a.applyStrategy {
        case ServerSideApply:
            return a.serverSideApply(ctx, gvr, obj, result)
        case StrategicMerge:
            return a.strategicMerge(ctx, gvr, obj, result)
        case Replace:
            return a.replace(ctx, gvr, obj, result)
        default:
            return fmt.Errorf("unknown apply strategy: %v", a.applyStrategy)
        }
    })
    
    if err != nil {
        result.Success = false
        result.Error = err
        logger.Error(err, "Failed to apply resource", "gvr", gvr, "name", obj.GetName())
    } else {
        result.Success = true
        logger.V(4).Info("Successfully applied resource", "gvr", gvr, "name", obj.GetName())
    }
    
    return result, err
}

// Delete removes a resource with configurable propagation
func (a *Applier) Delete(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string, options metav1.DeleteOptions) error {
    logger := klog.FromContext(ctx)
    
    return a.retryStrategy.Execute(ctx, func() error {
        err := a.client.Resource(gvr).Namespace(namespace).Delete(ctx, name, options)
        if err != nil {
            if errors.IsNotFound(err) {
                logger.V(4).Info("Resource already deleted", "gvr", gvr, "name", name)
                return nil
            }
            return err
        }
        
        logger.V(4).Info("Deleted resource", "gvr", gvr, "name", name)
        return nil
    })
}
```

### Step 3: Implement Retry Strategy
Create `retry.go` with:

1. **RetryStrategy struct**:
```go
type RetryStrategy struct {
    MaxRetries     int
    InitialDelay   time.Duration
    MaxDelay       time.Duration
    Factor         float64
    Jitter         float64
    RetryCondition func(error) bool
}
```

2. **Execute method with backoff**:
   - Exponential backoff calculation
   - Jitter addition
   - Retry condition checking
   - Context cancellation handling

3. **Default retry conditions**:
   - Retry on conflicts
   - Retry on temporary errors
   - Don't retry on validation errors
   - Circuit breaker for repeated failures

### Step 4: Implement Apply Strategies
Create `strategy.go` with:

1. **Server-side apply**:
```go
func (a *Applier) serverSideApply(ctx context.Context, gvr schema.GroupVersionResource, obj *unstructured.Unstructured, result *ApplyResult) error {
    data, err := json.Marshal(obj)
    if err != nil {
        return err
    }
    
    applied, err := a.client.Resource(gvr).
        Namespace(obj.GetNamespace()).
        Patch(ctx, obj.GetName(), types.ApplyPatchType, data, metav1.PatchOptions{
            FieldManager: a.fieldManager,
            Force:        &a.forceConflicts,
        })
    
    if err != nil {
        return err
    }
    
    result.Applied = applied
    result.Operation = "apply"
    return nil
}
```

2. **Strategic merge patch**:
   - Calculate patch from current state
   - Apply strategic merge
   - Handle conflicts

3. **Replace strategy**:
   - Get current resource
   - Update with new spec
   - Replace entire resource

### Step 5: Implement Result Aggregation
Create `results.go` with:

1. **ApplyResult struct**:
```go
type ApplyResult struct {
    GVR         schema.GroupVersionResource
    Namespace   string
    Name        string
    Operation   string // create, update, apply, noop
    Success     bool
    Error       error
    Applied     *unstructured.Unstructured
    Attempts    int
    Duration    time.Duration
}
```

2. **BatchResult for multiple operations**:
   - Aggregate multiple results
   - Calculate success rate
   - Categorize errors
   - Generate summary

### Step 6: Add Optimizations

1. **Batch operations**:
   - Group similar operations
   - Parallel execution with limits
   - Result aggregation

2. **Caching**:
   - Cache discovery information
   - Cache field managers
   - Reuse clients

3. **Performance**:
   - Connection pooling
   - Request coalescing
   - Minimal API calls

### Step 7: Add Comprehensive Tests
Create test files covering:

1. **Apply operations**:
   - Create new resources
   - Update existing resources
   - Server-side apply
   - Conflict handling

2. **Delete operations**:
   - Simple deletion
   - Cascade deletion
   - Orphan deletion

3. **Retry logic**:
   - Exponential backoff
   - Retry conditions
   - Maximum retries

## Testing Requirements

### Unit Tests:
- Apply with different strategies
- Delete with different propagation policies
- Retry logic with various errors
- Result aggregation
- Error classification

### Integration Tests:
- Real resource application
- Conflict scenarios
- Large batch operations
- Network failure handling

## Validation Checklist

- [ ] All apply strategies implemented correctly
- [ ] Retry logic works with backoff
- [ ] Proper error classification
- [ ] Results accurately reported
- [ ] Performance optimizations in place
- [ ] Comprehensive logging
- [ ] Metrics for monitoring
- [ ] Tests achieve >75% coverage
- [ ] Code follows KCP patterns
- [ ] Feature flags integrated
- [ ] Under 600 lines (excluding tests)

## Common Pitfalls to Avoid

1. **Don't retry validation errors** - they won't succeed
2. **Add jitter to retries** - avoid thundering herd
3. **Handle partial failures** - in batch operations
4. **Clean up on failures** - don't leave orphans
5. **Log with context** - include GVR and names

## Integration Notes

This component:
- Is called by Wave 2 downstream core
- Uses Wave 2 conflict resolver
- Reports results for monitoring
- Provides operation metrics

Should expose:
- Multiple apply strategies
- Configurable retry logic
- Batch operation support
- Detailed result information

## Success Criteria

The implementation is complete when:
1. Resources can be applied with multiple strategies
2. Retry logic handles transient failures
3. Deletions work with proper propagation
4. Batch operations are efficient
5. All tests pass
6. Can handle 100+ operations per second