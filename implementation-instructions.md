# Implementation Instructions: Wave2a-02 - Reconciliation Logic

## 🎯 Objective
Implement core reconciliation logic for SyncTarget controller (~450 lines)

## 📋 Prerequisites
- Wave2a-01 controller foundation must be complete
- Controller base should compile without errors

## ⚠️ CRITICAL: Implementation Approach
**YOU MUST CREATE NEW CODE** - The to-be-split branch only contains API types, NOT controller implementation.
- Cherry-pick Wave2a-01 first: `git cherry-pick <Wave2a-01-commit-hash>`
- CREATE all reconciliation logic from scratch
- DO NOT look for existing controller code in the to-be-split branch (it doesn't exist)

## 🔨 Implementation Tasks

### 1. Create `pkg/reconciler/workload/synctarget/reconcile.go` (~350 lines)

**Core Functions to Implement:**
```go
// Main reconciliation entry point
func (c *Controller) reconcile(ctx context.Context, key string) error

// Resource reconciliation logic with phases
func (c *Controller) reconcileResource(ctx context.Context, cluster logicalcluster.Path, syncTarget *workloadv1alpha1.SyncTarget) error

// Validation checks
func (c *Controller) validatePrerequisites(ctx context.Context, cluster logicalcluster.Path, syncTarget *workloadv1alpha1.SyncTarget) error

// Deployment stubs (actual implementation in Wave2a-03)
func (c *Controller) ensureSyncerDeployment(ctx context.Context, cluster logicalcluster.Path, syncTarget *workloadv1alpha1.SyncTarget) error

// Health checking
func (c *Controller) checkSyncerHealth(ctx context.Context, cluster logicalcluster.Path, syncTarget *workloadv1alpha1.SyncTarget) (bool, error)

// Deletion handling
func (c *Controller) reconcileDelete(ctx context.Context, cluster logicalcluster.Path, syncTarget *workloadv1alpha1.SyncTarget) error
```

**Reconciliation Phases:**
1. Parse key and retrieve SyncTarget
2. Check deletion timestamp
3. Validate prerequisites
4. Ensure syncer deployment (stub)
5. Check syncer health
6. Update status conditions

### 2. Create `pkg/reconciler/workload/synctarget/status.go` (~100 lines)

**Status Management Functions:**
```go
// Update SyncTarget status
func (c *Controller) updateStatus(ctx context.Context, cluster logicalcluster.Path, syncTarget *workloadv1alpha1.SyncTarget, reconcileErr error) error

// Set or update condition
func setCondition(syncTarget *workloadv1alpha1.SyncTarget, condition workloadv1alpha1.SyncTargetCondition)

// Remove condition
func removeCondition(syncTarget *workloadv1alpha1.SyncTarget, conditionType workloadv1alpha1.ConditionType)
```

**Conditions to Manage:**
- `SyncTargetValid` - Prerequisites validation
- `SyncTargetDeployed` - Deployment status
- `SyncTargetReady` - Overall health

### 3. Update Controller Integration

Modify existing `controller.go` to:
- Call `reconcile()` from the process function
- Use status update helpers
- Handle errors appropriately

## 📝 Critical Implementation Notes

### Error Handling Pattern
```go
reconcileErr := c.reconcileResource(ctx, cluster, syncTarget)
statusErr := c.updateStatus(ctx, cluster, syncTarget, reconcileErr)
if statusErr != nil {
    return statusErr
}
return reconcileErr
```

### Deep Copy Pattern
Always deep copy before modifications:
```go
syncTarget = syncTarget.DeepCopy()
```

### Logging Standards
- V(2): Important operations
- V(3): Normal operations
- V(4): Debug details
- V(6): Verbose debugging

## ✅ Validation Steps

1. **Compile Check**
   ```bash
   go build ./pkg/reconciler/workload/synctarget/...
   ```

2. **Unit Tests**
   ```bash
   go test ./pkg/reconciler/workload/synctarget/... -v
   ```

3. **Line Count Verification**
   ```bash
   /workspaces/kcp-shared-tools/tmc-pr-line-counter.sh -c $(git branch --show-current)
   ```
   Target: ~450 lines

## 🔄 Commit Structure

```bash
# Commit 1: Reconciliation logic
git add pkg/reconciler/workload/synctarget/reconcile.go
git commit -s -S -m "feat(controller): add SyncTarget reconciliation logic"

# Commit 2: Status management
git add pkg/reconciler/workload/synctarget/status.go
git commit -s -S -m "feat(controller): add status management for SyncTarget"

# Commit 3: Integration
git add pkg/reconciler/workload/synctarget/controller.go
git commit -s -S -m "feat(controller): integrate reconciliation with controller"
```

## ⚠️ Important Reminders

- **DO NOT** implement actual deployment logic (that's Wave2a-03)
- **DO** create stubs for deployment functions
- **DO** implement complete validation logic
- **DO** handle all error cases
- **DO NOT** exceed 450 lines total
- **DO** maintain workspace isolation throughout

## 🎯 Success Metrics

- [ ] All reconciliation phases implemented
- [ ] Status conditions properly managed
- [ ] Error handling comprehensive
- [ ] Code compiles without errors
- [ ] Under 450 lines of code
- [ ] Ready for Wave2a-03 to build upon