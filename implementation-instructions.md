# Implementation Instructions: Sync Engine Core

## Branch: `feature/phase7-syncer-impl/p7w1-sync-engine`

## Overview
This branch implements the core synchronization engine that manages the lifecycle of resource synchronization between KCP and physical clusters. The sync engine is the heart of the syncer, coordinating all synchronization activities through a work queue pattern.

**Target Size**: ~750 lines  
**Complexity**: High  
**Priority**: Critical (blocks all other Wave 2-4 efforts)

## Dependencies
- **Phase 5 APIs**: Uses syncer interfaces from `pkg/apis/syncer/v1alpha1`
- **Phase 6 Infrastructure**: Leverages virtual workspace and controller patterns
- **External**: None - this is the foundational component

## Files to Create

### 1. Core Engine Implementation (~350 lines)
**File**: `pkg/reconciler/workload/syncer/engine/engine.go`
- Main sync engine struct and initialization
- Work queue management
- Resource syncer registration
- Event handling from informers
- Status tracking and reporting

### 2. Resource Syncer (~200 lines)
**File**: `pkg/reconciler/workload/syncer/engine/resource_syncer.go`
- Individual resource type synchronization
- Bi-directional sync coordination
- Resource-specific logic handling
- Cache management per resource type

### 3. Sync Item Types (~50 lines)
**File**: `pkg/reconciler/workload/syncer/engine/types.go`
- SyncItem struct definition
- SyncStatus struct
- Engine configuration types
- Helper type definitions

### 4. Engine Tests (~150 lines)
**File**: `pkg/reconciler/workload/syncer/engine/engine_test.go`
- Unit tests for engine lifecycle
- Work queue processing tests
- Resource syncer registration tests
- Mock implementations

## Step-by-Step Implementation Guide

### Step 1: Create Package Structure
```bash
mkdir -p pkg/reconciler/workload/syncer/engine
```

### Step 2: Define Types
Create `types.go` with:
```go
package engine

import (
    "k8s.io/apimachinery/pkg/runtime/schema"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SyncItem represents a work item in the sync queue
type SyncItem struct {
    GVR       schema.GroupVersionResource
    Key       string      // namespace/name
    Action    string      // add, update, delete, status
    Object    interface{} // The actual object
    Retries   int
    Timestamp metav1.Time
}

// SyncStatus tracks synchronization state
type SyncStatus struct {
    Connected        bool
    LastSyncTime     *metav1.Time
    SyncedResources  map[schema.GroupVersionResource]int
    PendingResources map[schema.GroupVersionResource]int
    FailedResources  map[schema.GroupVersionResource]int
    ErrorMessage     string
}

// EngineConfig holds engine configuration
type EngineConfig struct {
    WorkerCount       int
    ResyncPeriod      time.Duration
    MaxRetries        int
    RateLimitPerSec   int
    QueueDepth        int
    EnableProfiling   bool
}
```

### Step 3: Implement Core Engine
Create `engine.go` with the following structure:

1. **Engine struct definition**:
   - KCP and downstream clients
   - Informer factories
   - Resource syncers map
   - Work queue
   - Transformation pipeline reference
   - Filter chain
   - Status tracking

2. **NewEngine constructor**:
   - Initialize clients
   - Create work queue with rate limiting
   - Setup informer factories
   - Initialize resource syncer map
   - Configure default transformers and filters

3. **Start method**:
   - Start informer factories
   - Wait for cache sync
   - Start worker goroutines
   - Begin status reporting

4. **setupResourceSyncer method**:
   - Create informer for resource type
   - Add event handlers (add/update/delete)
   - Register resource syncer
   - Configure bi-directional sync

5. **Work queue processing**:
   - processNextWorkItem method
   - Handle retries with exponential backoff
   - Update status counters
   - Log processing results

### Step 4: Implement Resource Syncer
Create `resource_syncer.go` with:

1. **ResourceSyncer struct**:
   - GVR identification
   - Engine reference
   - KCP and downstream informers
   - Transformation hooks

2. **ApplyToDownstream method**:
   - Get object from KCP
   - Apply transformations
   - Check filters
   - Create/update in downstream

3. **DeleteFromDownstream method**:
   - Verify deletion is allowed
   - Remove from downstream
   - Clean up any resources

4. **SyncStatusToKCP method**:
   - Extract status from downstream
   - Transform if needed
   - Patch to KCP object

### Step 5: Add Test Coverage
Create comprehensive tests in `engine_test.go`:

1. **Engine lifecycle tests**:
   - Test engine creation
   - Test start/stop
   - Test graceful shutdown

2. **Resource syncer tests**:
   - Test syncer registration
   - Test event handling
   - Test queue processing

3. **Mock implementations**:
   - Mock KCP client
   - Mock downstream client
   - Mock transformers

### Step 6: Integration Points

#### With Phase 5 Interfaces:
```go
import (
    syncerv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/syncer/v1alpha1"
    "github.com/kcp-dev/kcp/pkg/syncer/interfaces"
)
```

#### With Phase 6 Controllers:
- Use controller patterns from Phase 6
- Integrate with virtual workspace APIs
- Leverage existing reconciliation helpers

### Step 7: Feature Flag Integration
```go
if features.TMCEnabled() && features.SyncEngineEnabled() {
    // Enable sync engine
}
```

## Testing Requirements

### Unit Tests:
- Engine initialization and lifecycle
- Work queue operations
- Resource syncer registration
- Event handler logic
- Status tracking
- Error handling and retries

### Integration Tests:
- End-to-end sync flow
- Multiple resource types
- Conflict scenarios
- Connection loss/recovery

## Validation Checklist

- [ ] All imports resolve correctly
- [ ] Interfaces from Phase 5 are properly implemented
- [ ] Work queue uses proper Kubernetes patterns
- [ ] Informers are correctly configured
- [ ] Event handlers don't block
- [ ] Proper error handling throughout
- [ ] Comprehensive logging with appropriate levels
- [ ] Metrics exposed for monitoring
- [ ] Status is accurately tracked and reported
- [ ] Resource cleanup on shutdown
- [ ] Tests achieve >70% coverage
- [ ] Code follows KCP patterns
- [ ] Feature flags properly integrated
- [ ] Under 750 lines (excluding tests and generated code)

## Common Pitfalls to Avoid

1. **Don't block in event handlers** - enqueue work items instead
2. **Handle cache sync failures** - don't proceed if caches aren't ready
3. **Implement proper backoff** - avoid overwhelming the API server
4. **Clean up resources** - ensure proper shutdown handling
5. **Don't lose work items** - handle queue shutdown gracefully

## Integration Notes

This engine will be consumed by:
- Wave 2: Downstream synchronization components
- Wave 3: Upstream status synchronization
- Wave 4: WebSocket connection management

The engine should expose:
- Registration methods for transformers and filters
- Status query interface
- Metrics for monitoring
- Health check endpoints

## Success Criteria

The implementation is complete when:
1. Engine can start and connect to both KCP and downstream
2. Resources are discovered and syncers created
3. Work items are processed from the queue
4. Status is tracked and reportable
5. All tests pass
6. Can handle at least 100 resources without performance issues