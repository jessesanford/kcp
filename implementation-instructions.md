# Implementation Instructions: p6w1-synctarget-controller

## Overview
This branch implements the SyncTarget controller, which is the foundational controller for TMC Phase 6. It manages the lifecycle and synchronization of SyncTarget resources, coordinating with downstream clusters and maintaining workspace isolation.

## Branch Information
- **Branch Name**: `feature/tmc-completion/p6w1-synctarget-controller`
- **Estimated Lines**: 750
- **Wave**: 1 (Critical Path)
- **Parallel Agents**: Can run alongside p6w3-webhooks

## Dependencies

### Upstream Dependencies
- **Phase 5 APIs**: Requires TMC API types from Phase 5
- **KCP Core**: Utilizes KCP's workspace and APIExport systems

### Downstream Blocks
- **p6w2-vw-core**: VW Core depends on SyncTarget controller interfaces
- **p6w3-quota-manager**: Quota manager integrates with SyncTarget
- **p6w3-aggregator**: Resource aggregator requires SyncTarget data

## Implementation Breakdown

### Files to Create

1. **pkg/reconciler/workload/synctarget/controller.go** (250 lines)
   - Main controller structure
   - Reconciliation loop
   - Event handlers
   - Workspace-aware processing

2. **pkg/reconciler/workload/synctarget/synctarget_reconciler.go** (200 lines)
   - Core reconciliation logic
   - Status management
   - Condition updates
   - Health checking

3. **pkg/reconciler/workload/synctarget/cluster_manager.go** (150 lines)
   - Cluster connection management
   - Authentication handling
   - Connection pooling
   - Heartbeat monitoring

4. **pkg/reconciler/workload/synctarget/workspace_handler.go** (100 lines)
   - Workspace isolation logic
   - Cross-workspace references
   - Permission validation

5. **pkg/reconciler/workload/synctarget/controller_test.go** (50 lines)
   - Unit tests for controller
   - Mock implementations
   - Test scenarios

## Step-by-Step Implementation Guide

### Step 1: Controller Foundation
```go
// pkg/reconciler/workload/synctarget/controller.go
package synctarget

import (
    "context"
    "time"
    
    kcpcache "github.com/kcp-dev/kcp/pkg/cache"
    "github.com/kcp-dev/kcp/pkg/logging"
    "github.com/kcp-dev/kcp/pkg/reconciler/committer"
    tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
)

type Controller struct {
    queue workqueue.RateLimitingInterface
    kcpClusterClient kcpclientset.ClusterInterface
    tmcInformer cache.SharedIndexInformer
    clusterManager *ClusterManager
}
```

### Step 2: Reconciliation Logic
Implement the main reconciliation loop with proper error handling and status updates.

### Step 3: Cluster Management
Create cluster connection management with health checking and automatic reconnection.

### Step 4: Workspace Integration
Ensure proper workspace isolation and handle cross-workspace references correctly.

### Step 5: Testing
Write comprehensive unit tests covering all scenarios including error cases.

## Critical KCP Patterns to Follow

1. **Workspace Isolation**
   - Always use workspace-aware clients
   - Validate workspace boundaries
   - Handle logical cluster names properly

2. **APIExport Integration**
   - Register with APIExport system
   - Handle permission claims
   - Manage API bindings

3. **Status Management**
   - Use conditions consistently
   - Follow KCP condition conventions
   - Update generation/observedGeneration

4. **Event Handling**
   - Use KCP's event recorder
   - Include workspace context in events
   - Follow event naming conventions

## Testing Requirements

### Unit Tests
- Controller initialization
- Reconciliation scenarios
- Error handling
- Workspace isolation

### Integration Tests
- End-to-end SyncTarget lifecycle
- Multi-workspace scenarios
- Cluster connection handling
- Status propagation

## Integration Points

1. **With VW Core (p6w2-vw-core)**
   - Exports SyncTarget interfaces
   - Provides cluster data
   - Shares connection pool

2. **With Quota Manager (p6w3-quota-manager)**
   - Reports resource usage
   - Enforces quotas
   - Updates capacity data

3. **With Resource Aggregator (p6w3-aggregator)**
   - Provides cluster metrics
   - Shares status information
   - Coordinates aggregation

## Validation Checklist

### Before Starting
- [ ] Phase 5 APIs are available
- [ ] KCP development environment is set up
- [ ] Understand workspace isolation requirements
- [ ] Review KCP controller patterns

### During Implementation
- [ ] Follow KCP coding standards
- [ ] Implement proper error handling
- [ ] Add comprehensive logging
- [ ] Write tests alongside code
- [ ] Document complex logic

### Before PR
- [ ] All tests pass
- [ ] Code is under 750 lines (excluding generated code)
- [ ] No linting errors
- [ ] Documentation is complete
- [ ] Integration with Phase 5 APIs verified
- [ ] Workspace isolation tested
- [ ] Status conditions work correctly
- [ ] No uncommitted files

## Common Pitfalls to Avoid

1. **Workspace Violations**
   - Don't access resources across workspace boundaries without permission
   - Always validate workspace context
   - Use proper cluster-aware clients

2. **Status Updates**
   - Don't update status in the same reconciliation as spec
   - Always use status subresource
   - Handle conflicts with retry

3. **Resource Leaks**
   - Properly close connections
   - Clean up goroutines
   - Use context cancellation

4. **Testing Gaps**
   - Don't skip error scenarios
   - Test workspace boundaries
   - Verify status updates

## Success Criteria

- [ ] Controller successfully reconciles SyncTarget resources
- [ ] Proper workspace isolation maintained
- [ ] Health checking works reliably
- [ ] Status accurately reflects cluster state
- [ ] All tests pass including integration tests
- [ ] Code review feedback addressed
- [ ] PR ready for merge