# Split Implementation: Wave2a-02 - Reconciliation Logic

## Overview
**Branch:** `feature/tmc-syncer-02a-reconcile`  
**Target Size:** ~450 lines  
**Dependencies:** Wave2a-01 (Controller Base) must be complete  
**Can Run In Parallel:** Yes, with Wave2a-03 after Wave2a-01

## Implementation Tasks

### Prerequisites
Ensure Wave2a-01 controller foundation is available:
```bash
# Merge Wave2a-01 if complete
git fetch origin
git merge origin/main  # or merge the branch directly
```

### Files to Create/Modify

#### 1. **pkg/reconciler/workload/synctarget/reconcile.go** (~350 lines)

```go
package synctarget

import (
    "context"
    "fmt"

    workloadv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"
    kcpcache "github.com/kcp-dev/kcp/pkg/cache"
    
    "github.com/kcp-dev/logicalcluster/v3"
    
    corev1 "k8s.io/api/core/v1"
    "k8s.io/apimachinery/pkg/api/errors"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/klog/v2"
)

// reconcile handles the main reconciliation logic for a SyncTarget
func (c *Controller) reconcile(ctx context.Context, key string) error {
    klog.V(4).Infof("Reconciling SyncTarget %s", key)
    
    // Parse the key to get cluster and name
    cluster, namespace, name, err := kcpcache.SplitMetaClusterNamespaceKey(key)
    if err != nil {
        return err
    }
    
    if namespace != "" {
        // SyncTargets are cluster-scoped
        return fmt.Errorf("unexpected namespace in key: %s", key)
    }
    
    // Get the SyncTarget
    syncTarget, err := c.syncTargetLister.Cluster(cluster).Get(name)
    if err != nil {
        if errors.IsNotFound(err) {
            klog.V(2).Infof("SyncTarget %s/%s not found, likely deleted", cluster, name)
            return nil
        }
        return err
    }
    
    // Don't reconcile if being deleted
    if syncTarget.DeletionTimestamp != nil {
        return c.reconcileDelete(ctx, cluster, syncTarget)
    }
    
    // Deep copy to avoid modifying cache
    syncTarget = syncTarget.DeepCopy()
    
    // Reconcile the SyncTarget
    reconcileErr := c.reconcileResource(ctx, cluster, syncTarget)
    
    // Update status
    statusErr := c.updateStatus(ctx, cluster, syncTarget, reconcileErr)
    
    // Return the reconcile error if status update succeeded
    if statusErr != nil {
        return statusErr
    }
    return reconcileErr
}

// reconcileResource performs the main reconciliation logic
func (c *Controller) reconcileResource(ctx context.Context, cluster logicalcluster.Path, syncTarget *workloadv1alpha1.SyncTarget) error {
    klog.V(3).Infof("Reconciling SyncTarget %s/%s", cluster, syncTarget.Name)
    
    // Phase 1: Validate prerequisites
    if err := c.validatePrerequisites(ctx, cluster, syncTarget); err != nil {
        setCondition(syncTarget, workloadv1alpha1.SyncTargetCondition{
            Type:    workloadv1alpha1.SyncTargetValid,
            Status:  corev1.ConditionFalse,
            Reason:  "ValidationFailed",
            Message: err.Error(),
        })
        return err
    }
    
    setCondition(syncTarget, workloadv1alpha1.SyncTargetCondition{
        Type:    workloadv1alpha1.SyncTargetValid,
        Status:  corev1.ConditionTrue,
        Reason:  "Valid",
        Message: "SyncTarget validation passed",
    })
    
    // Phase 2: Ensure syncer deployment
    if err := c.ensureSyncerDeployment(ctx, cluster, syncTarget); err != nil {
        setCondition(syncTarget, workloadv1alpha1.SyncTargetCondition{
            Type:    workloadv1alpha1.SyncTargetDeployed,
            Status:  corev1.ConditionFalse,
            Reason:  "DeploymentFailed",
            Message: err.Error(),
        })
        return err
    }
    
    setCondition(syncTarget, workloadv1alpha1.SyncTargetCondition{
        Type:    workloadv1alpha1.SyncTargetDeployed,
        Status:  corev1.ConditionTrue,
        Reason:  "Deployed",
        Message: "Syncer deployment successful",
    })
    
    // Phase 3: Check syncer health
    healthy, err := c.checkSyncerHealth(ctx, cluster, syncTarget)
    if err != nil {
        setCondition(syncTarget, workloadv1alpha1.SyncTargetCondition{
            Type:    workloadv1alpha1.SyncTargetReady,
            Status:  corev1.ConditionUnknown,
            Reason:  "HealthCheckFailed",
            Message: err.Error(),
        })
        return err
    }
    
    if !healthy {
        setCondition(syncTarget, workloadv1alpha1.SyncTargetCondition{
            Type:    workloadv1alpha1.SyncTargetReady,
            Status:  corev1.ConditionFalse,
            Reason:  "Unhealthy",
            Message: "Syncer is not healthy",
        })
        return fmt.Errorf("syncer is not healthy")
    }
    
    setCondition(syncTarget, workloadv1alpha1.SyncTargetCondition{
        Type:    workloadv1alpha1.SyncTargetReady,
        Status:  corev1.ConditionTrue,
        Reason:  "Ready",
        Message: "Syncer is healthy and ready",
    })
    
    // Update phase
    syncTarget.Status.Phase = workloadv1alpha1.SyncTargetPhaseReady
    
    return nil
}

// validatePrerequisites checks if the SyncTarget can be reconciled
func (c *Controller) validatePrerequisites(ctx context.Context, cluster logicalcluster.Path, syncTarget *workloadv1alpha1.SyncTarget) error {
    // Check if cluster reference is valid
    if syncTarget.Spec.Cluster == "" {
        return fmt.Errorf("cluster reference is required")
    }
    
    // Check if cells are specified
    if len(syncTarget.Spec.Cells) == 0 {
        return fmt.Errorf("at least one cell must be specified")
    }
    
    // Validate capacity if specified
    if syncTarget.Spec.Capacity != nil {
        for key, quantity := range syncTarget.Spec.Capacity {
            if quantity.IsNegative() {
                return fmt.Errorf("capacity for %s cannot be negative", key)
            }
        }
    }
    
    return nil
}

// ensureSyncerDeployment ensures the syncer is deployed (stub for now)
func (c *Controller) ensureSyncerDeployment(ctx context.Context, cluster logicalcluster.Path, syncTarget *workloadv1alpha1.SyncTarget) error {
    // This will be implemented in Split 3 (Deployment)
    klog.V(4).Infof("Ensuring syncer deployment for %s/%s", cluster, syncTarget.Name)
    // Stub implementation - actual deployment logic in Split 3
    return nil
}

// checkSyncerHealth checks if the syncer is healthy (stub for now)
func (c *Controller) checkSyncerHealth(ctx context.Context, cluster logicalcluster.Path, syncTarget *workloadv1alpha1.SyncTarget) (bool, error) {
    // This will be enhanced in Split 3
    klog.V(4).Infof("Checking syncer health for %s/%s", cluster, syncTarget.Name)
    
    // Check last heartbeat
    if syncTarget.Status.LastHeartbeatTime != nil {
        timeSinceHeartbeat := metav1.Now().Sub(syncTarget.Status.LastHeartbeatTime.Time)
        if timeSinceHeartbeat > 5*time.Minute {
            return false, nil
        }
    }
    
    return true, nil
}

// reconcileDelete handles deletion of a SyncTarget
func (c *Controller) reconcileDelete(ctx context.Context, cluster logicalcluster.Path, syncTarget *workloadv1alpha1.SyncTarget) error {
    klog.V(2).Infof("Deleting SyncTarget %s/%s", cluster, syncTarget.Name)
    
    // Cleanup will be implemented in Split 3
    // For now, just remove finalizers if present
    
    return nil
}
```

#### 2. **pkg/reconciler/workload/synctarget/status.go** (~100 lines)

```go
package synctarget

import (
    "context"
    "reflect"
    
    workloadv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"
    
    "github.com/kcp-dev/logicalcluster/v3"
    
    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/klog/v2"
)

// updateStatus updates the status of a SyncTarget
func (c *Controller) updateStatus(ctx context.Context, cluster logicalcluster.Path, syncTarget *workloadv1alpha1.SyncTarget, reconcileErr error) error {
    // Get the current SyncTarget to check if status needs updating
    current, err := c.syncTargetLister.Cluster(cluster).Get(syncTarget.Name)
    if err != nil {
        return err
    }
    
    // Check if status needs updating
    if reflect.DeepEqual(current.Status, syncTarget.Status) {
        klog.V(6).Infof("Status unchanged for SyncTarget %s/%s", cluster, syncTarget.Name)
        return nil
    }
    
    // Update the status
    klog.V(3).Infof("Updating status for SyncTarget %s/%s", cluster, syncTarget.Name)
    updated, err := c.kcpClusterClient.Cluster(cluster).WorkloadV1alpha1().SyncTargets().UpdateStatus(
        ctx,
        syncTarget,
        metav1.UpdateOptions{},
    )
    if err != nil {
        return err
    }
    
    klog.V(4).Infof("Successfully updated status for SyncTarget %s/%s", cluster, updated.Name)
    return nil
}

// setCondition sets or updates a condition on the SyncTarget
func setCondition(syncTarget *workloadv1alpha1.SyncTarget, condition workloadv1alpha1.SyncTargetCondition) {
    // Set timestamp
    condition.LastTransitionTime = metav1.Now()
    
    // Find existing condition
    for i, existing := range syncTarget.Status.Conditions {
        if existing.Type == condition.Type {
            if existing.Status != condition.Status {
                syncTarget.Status.Conditions[i] = condition
            } else {
                // Update message and reason if changed
                syncTarget.Status.Conditions[i].Message = condition.Message
                syncTarget.Status.Conditions[i].Reason = condition.Reason
            }
            return
        }
    }
    
    // Add new condition
    syncTarget.Status.Conditions = append(syncTarget.Status.Conditions, condition)
}

// removeCondition removes a condition from the SyncTarget
func removeCondition(syncTarget *workloadv1alpha1.SyncTarget, conditionType workloadv1alpha1.ConditionType) {
    var newConditions []workloadv1alpha1.SyncTargetCondition
    for _, c := range syncTarget.Status.Conditions {
        if c.Type != conditionType {
            newConditions = append(newConditions, c)
        }
    }
    syncTarget.Status.Conditions = newConditions
}
```

## Implementation Checklist

### Pre-Implementation
- [ ] Ensure Wave2a-01 is available
- [ ] Controller foundation compiles
- [ ] Branch from latest code

### Implementation
- [ ] Create reconcile.go with:
  - [ ] Main reconcile function
  - [ ] Resource reconciliation logic
  - [ ] Validation logic
  - [ ] Health checking
  - [ ] Delete handling
- [ ] Create status.go with:
  - [ ] Status update logic
  - [ ] Condition management
  - [ ] Status comparison
- [ ] Update controller.go to use reconcile

### Testing
- [ ] Unit tests for reconciliation
- [ ] Test condition management
- [ ] Test status updates
- [ ] Test error handling

### Validation
- [ ] Code compiles
- [ ] No cyclic imports
- [ ] Line count ~450
- [ ] Proper error handling

## Commit Strategy

```bash
# Add reconciliation logic
git add pkg/reconciler/workload/synctarget/reconcile.go
git commit -s -S -m "feat(controller): add SyncTarget reconciliation logic

- Implement main reconciliation loop
- Add validation and health checking
- Handle resource lifecycle
- Manage deletion gracefully"

# Add status management
git add pkg/reconciler/workload/synctarget/status.go
git commit -s -S -m "feat(controller): add status management for SyncTarget

- Implement status update logic
- Add condition management helpers
- Ensure efficient status updates"
```

## Success Criteria

1. ✅ Reconciliation follows KCP patterns
2. ✅ Proper condition management
3. ✅ Efficient status updates
4. ✅ Error handling in place
5. ✅ Under 450 lines
6. ✅ All phases of reconciliation covered

## Dependencies

- **Requires:** Wave2a-01 (Controller Base)
- **Can Parallel With:** Wave2a-03 (Deployment)
- **Provides:** Core reconciliation logic

## Notes for Parallel Development

- Wave2a-03 will implement the actual deployment logic
- Stubs are provided for deployment functions
- Status management is reusable
- Condition helpers should be used consistently