# Implementation Instructions: Upstream Status Syncer

## Branch: `feature/phase7-syncer-impl/p7w3-upstream-status`

## Overview
This branch implements the upstream status synchronization that syncs resource status from physical clusters back to KCP. It handles status extraction, transformation, aggregation from multiple clusters, and patching back to KCP resources.

**Target Size**: ~650 lines  
**Complexity**: Medium-High  
**Priority**: High (completes bi-directional sync)

## Dependencies
- **Phase 5 APIs**: Uses status syncer interfaces
- **Phase 6 Infrastructure**: Virtual workspace APIs
- **Wave 1 Sync Engine**: Integrates with engine
- **Wave 1 Transformation**: Reverse transformations

## Files to Create

### 1. Status Syncer Core (~250 lines)
**File**: `pkg/reconciler/workload/syncer/upstream/status_syncer.go`
- Main status syncer struct
- Status extraction logic
- Status patching to KCP
- Queue management

### 2. Status Aggregation (~150 lines)
**File**: `pkg/reconciler/workload/syncer/upstream/aggregation.go`
- Multi-cluster status aggregation
- Status merging strategies
- Condition aggregation
- Summary generation

### 3. Status Extractors (~100 lines)
**File**: `pkg/reconciler/workload/syncer/upstream/extractors.go`
- Resource-specific extractors
- Default status extraction
- Field mapping
- Status validation

### 4. Status Transform (~80 lines)
**File**: `pkg/reconciler/workload/syncer/upstream/transform.go`
- Reverse namespace transformation
- Label/annotation cleanup
- Reference adjustment
- Status sanitization

### 5. Status Tests (~70 lines)
**File**: `pkg/reconciler/workload/syncer/upstream/status_syncer_test.go`
- Unit tests for status sync
- Aggregation tests
- Transform tests

## Step-by-Step Implementation Guide

### Step 1: Create Package Structure
```bash
mkdir -p pkg/reconciler/workload/syncer/upstream
```

### Step 2: Define Status Syncer Core
Create `status_syncer.go` with:

```go
package upstream

import (
    "context"
    "encoding/json"
    "fmt"
    
    workloadv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"
    "github.com/kcp-dev/kcp/pkg/syncer/interfaces"
    
    "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
    "k8s.io/apimachinery/pkg/runtime/schema"
    "k8s.io/apimachinery/pkg/types"
    "k8s.io/client-go/dynamic"
    "k8s.io/klog/v2"
    "github.com/kcp-dev/logicalcluster/v3"
)

// StatusSyncer syncs status from downstream to KCP
type StatusSyncer struct {
    kcpClient        dynamic.ClusterInterface
    downstreamClient dynamic.Interface
    
    syncTarget       *workloadv1alpha1.SyncTarget
    workspace        logicalcluster.Name
    
    extractors       map[schema.GroupVersionResource]interfaces.StatusExtractor
    aggregator       *StatusAggregator
    transformer      *StatusTransformer
    
    // Caching
    statusCache      map[string]interface{}
    mu               sync.RWMutex
}

// NewStatusSyncer creates a new status syncer
func NewStatusSyncer(
    kcpClient dynamic.ClusterInterface,
    downstreamClient dynamic.Interface,
    syncTarget *workloadv1alpha1.SyncTarget,
    workspace logicalcluster.Name,
) *StatusSyncer {
    return &StatusSyncer{
        kcpClient:        kcpClient,
        downstreamClient: downstreamClient,
        syncTarget:       syncTarget,
        workspace:        workspace,
        extractors:       make(map[schema.GroupVersionResource]interfaces.StatusExtractor),
        aggregator:       NewStatusAggregator(),
        transformer:      NewStatusTransformer(workspace),
        statusCache:      make(map[string]interface{}),
    }
}

// SyncStatusToKCP syncs resource status from downstream to KCP
func (s *StatusSyncer) SyncStatusToKCP(ctx context.Context, downstreamObj *unstructured.Unstructured) error {
    logger := klog.FromContext(ctx)
    
    // Extract status from downstream object
    status, err := s.extractStatus(downstreamObj)
    if err != nil {
        return fmt.Errorf("failed to extract status: %w", err)
    }
    
    if status == nil {
        logger.V(4).Info("No status to sync", "resource", downstreamObj.GetName())
        return nil
    }
    
    // Transform for upstream
    transformedStatus, err := s.transformer.TransformForUpstream(status, downstreamObj)
    if err != nil {
        return fmt.Errorf("failed to transform status: %w", err)
    }
    
    // Get the KCP resource
    gvr := s.getGVR(downstreamObj)
    namespace := s.reverseNamespaceTransform(downstreamObj.GetNamespace())
    name := downstreamObj.GetName()
    
    // Check if status has changed
    cacheKey := fmt.Sprintf("%s/%s/%s", gvr, namespace, name)
    if s.isStatusUnchanged(cacheKey, transformedStatus) {
        logger.V(5).Info("Status unchanged, skipping update", "resource", name)
        return nil
    }
    
    // Patch status to KCP
    if err := s.patchStatus(ctx, gvr, namespace, name, transformedStatus); err != nil {
        return fmt.Errorf("failed to patch status: %w", err)
    }
    
    // Update cache
    s.updateStatusCache(cacheKey, transformedStatus)
    
    logger.V(4).Info("Successfully synced status to KCP", "resource", name)
    return nil
}

// extractStatus extracts status from a downstream resource
func (s *StatusSyncer) extractStatus(obj *unstructured.Unstructured) (interface{}, error) {
    gvr := s.getGVR(obj)
    
    // Use custom extractor if available
    if extractor, exists := s.extractors[gvr]; exists {
        return extractor.ExtractStatus(obj)
    }
    
    // Use default extraction
    status, found, err := unstructured.NestedFieldNoCopy(obj.Object, "status")
    if err != nil {
        return nil, err
    }
    if !found {
        return nil, nil
    }
    
    return status, nil
}

// patchStatus patches the status to the KCP resource
func (s *StatusSyncer) patchStatus(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string, status interface{}) error {
    patch := map[string]interface{}{
        "status": status,
    }
    
    patchData, err := json.Marshal(patch)
    if err != nil {
        return err
    }
    
    _, err = s.kcpClient.
        Cluster(s.workspace).
        Resource(gvr).
        Namespace(namespace).
        Patch(ctx, name, types.MergePatchType, patchData, metav1.PatchOptions{}, "status")
    
    return err
}

// RegisterExtractor registers a custom status extractor
func (s *StatusSyncer) RegisterExtractor(gvr schema.GroupVersionResource, extractor interfaces.StatusExtractor) {
    s.extractors[gvr] = extractor
}
```

### Step 3: Implement Status Aggregation
Create `aggregation.go` with:

1. **StatusAggregator struct**:
```go
type StatusAggregator struct {
    strategies map[string]AggregationStrategy
}
```

2. **Aggregation strategies**:
   - **Union**: Combine all statuses
   - **Latest**: Use most recent status
   - **Quorum**: Majority consensus
   - **All**: Require all clusters agree

3. **Condition aggregation**:
   - Merge conditions from multiple sources
   - Calculate aggregate condition status
   - Generate summary messages

4. **Resource-specific aggregation**:
   - Deployment: Aggregate replicas
   - Service: Combine endpoints
   - Pod: Status from primary cluster

### Step 4: Implement Status Extractors
Create `extractors.go` with:

1. **Default extractor**:
```go
type DefaultExtractor struct{}

func (e *DefaultExtractor) ExtractStatus(obj *unstructured.Unstructured) (interface{}, error) {
    status, found, err := unstructured.NestedFieldNoCopy(obj.Object, "status")
    if err != nil || !found {
        return nil, err
    }
    return status, nil
}
```

2. **Pod extractor**:
   - Extract pod phase
   - Container statuses
   - Conditions
   - IP addresses

3. **Deployment extractor**:
   - Replica counts
   - Conditions
   - Observed generation

4. **Service extractor**:
   - LoadBalancer status
   - Ingress points

### Step 5: Implement Status Transformation
Create `transform.go` with:

1. **Reverse namespace transformation**:
```go
func (t *StatusTransformer) reverseNamespaceTransform(downstream string) string {
    // Remove workspace prefix
    prefix := fmt.Sprintf("kcp-%s-", t.workspace)
    if strings.HasPrefix(downstream, prefix) {
        return strings.TrimPrefix(downstream, prefix)
    }
    return downstream
}
```

2. **Reference transformation**:
   - Update object references
   - Fix namespace references
   - Adjust UIDs if needed

3. **Label/annotation cleanup**:
   - Remove downstream-specific labels
   - Clean up sync annotations
   - Preserve user annotations

### Step 6: Add Status Watching

1. **Watch for status changes**:
   - Setup informer for downstream resources
   - Filter to status-only updates
   - Enqueue for upstream sync

2. **Batch status updates**:
   - Collect multiple updates
   - Batch patch to KCP
   - Reduce API calls

3. **Rate limiting**:
   - Limit status update frequency
   - Coalesce rapid changes
   - Prevent API overload

### Step 7: Add Comprehensive Tests
Create test files covering:

1. **Status extraction**:
   - Various resource types
   - Missing status handling
   - Complex status structures

2. **Aggregation**:
   - Multiple cluster scenarios
   - Different strategies
   - Condition merging

3. **Transformation**:
   - Namespace reversal
   - Reference updates
   - Label cleanup

## Testing Requirements

### Unit Tests:
- Status extraction for various resources
- Aggregation strategies
- Transformation logic
- Cache management
- Error handling

### Integration Tests:
- Full status sync flow
- Multi-cluster aggregation
- Large status objects
- Rapid status changes

## Validation Checklist

- [ ] Status extracted correctly from all resource types
- [ ] Aggregation strategies work properly
- [ ] Transformations are reversible
- [ ] Cache prevents unnecessary updates
- [ ] Rate limiting prevents API overload
- [ ] Comprehensive logging
- [ ] Metrics for status sync
- [ ] Tests achieve >70% coverage
- [ ] Code follows KCP patterns
- [ ] Feature flags integrated
- [ ] Under 650 lines (excluding tests)

## Common Pitfalls to Avoid

1. **Don't sync status too frequently** - use caching and rate limiting
2. **Handle missing status gracefully** - not all resources have status
3. **Preserve status subresources** - some resources have complex status
4. **Avoid status loops** - don't trigger downstream updates
5. **Clean up properly** - remove workspace-specific fields

## Integration Notes

This component:
- Receives events from Wave 1 sync engine
- Uses Wave 1 transformation pipeline (reverse)
- May coordinate with Wave 3 event syncer
- Provides status for monitoring

Should provide:
- Configurable aggregation strategies
- Custom status extractors
- Rate limiting controls
- Status sync metrics

## Success Criteria

The implementation is complete when:
1. Status syncs from downstream to KCP
2. Multi-cluster aggregation works
3. Transformations are correctly reversed
4. Caching prevents unnecessary updates
5. All tests pass
6. Can handle 100+ status updates per second