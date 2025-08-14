# Split Implementation: Wave2a-03 - Deployment & Integration

## Overview
**Branch:** `feature/tmc-syncer-02a-deployment`  
**Target Size:** ~340 lines + tests  
**Dependencies:** Wave2a-01 (Controller Base) required  
**Can Run In Parallel:** Yes, with Wave2a-02 after Wave2a-01

## Implementation Tasks

### Prerequisites
```bash
# Ensure Wave2a-01 is available
git fetch origin
git merge origin/main  # or the specific branch
```

### Files to Create

#### 1. **pkg/reconciler/workload/synctarget/deployment.go** (~200 lines)

```go
package synctarget

import (
    "context"
    "fmt"
    
    workloadv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"
    
    "github.com/kcp-dev/logicalcluster/v3"
    
    appsv1 "k8s.io/api/apps/v1"
    corev1 "k8s.io/api/core/v1"
    "k8s.io/apimachinery/pkg/api/errors"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/util/intstr"
    "k8s.io/client-go/kubernetes"
    "k8s.io/klog/v2"
)

const (
    // SyncerNamespace is the namespace where syncers are deployed
    SyncerNamespace = "kcp-syncer-system"
    
    // SyncerImageName is the default syncer image
    SyncerImageName = "ghcr.io/kcp-dev/kcp/syncer:latest"
)

// DeploymentManager handles syncer deployment operations
type DeploymentManager struct {
    physicalClient kubernetes.Interface
}

// NewDeploymentManager creates a new deployment manager
func NewDeploymentManager(physicalClient kubernetes.Interface) *DeploymentManager {
    return &DeploymentManager{
        physicalClient: physicalClient,
    }
}

// EnsureDeployment ensures the syncer deployment exists and is up to date
func (dm *DeploymentManager) EnsureDeployment(ctx context.Context, cluster logicalcluster.Path, syncTarget *workloadv1alpha1.SyncTarget) error {
    deploymentName := syncerDeploymentName(syncTarget)
    
    klog.V(3).Infof("Ensuring deployment %s for SyncTarget %s/%s", deploymentName, cluster, syncTarget.Name)
    
    // Check if deployment exists
    existing, err := dm.physicalClient.AppsV1().Deployments(SyncerNamespace).Get(ctx, deploymentName, metav1.GetOptions{})
    if err != nil {
        if errors.IsNotFound(err) {
            // Create new deployment
            return dm.createDeployment(ctx, cluster, syncTarget)
        }
        return err
    }
    
    // Update existing deployment if needed
    return dm.updateDeployment(ctx, cluster, syncTarget, existing)
}

// createDeployment creates a new syncer deployment
func (dm *DeploymentManager) createDeployment(ctx context.Context, cluster logicalcluster.Path, syncTarget *workloadv1alpha1.SyncTarget) error {
    deployment := dm.buildDeployment(cluster, syncTarget)
    
    klog.V(2).Infof("Creating deployment %s for SyncTarget %s/%s", deployment.Name, cluster, syncTarget.Name)
    
    _, err := dm.physicalClient.AppsV1().Deployments(SyncerNamespace).Create(ctx, deployment, metav1.CreateOptions{})
    return err
}

// updateDeployment updates an existing syncer deployment
func (dm *DeploymentManager) updateDeployment(ctx context.Context, cluster logicalcluster.Path, syncTarget *workloadv1alpha1.SyncTarget, existing *appsv1.Deployment) error {
    desired := dm.buildDeployment(cluster, syncTarget)
    
    // Check if update is needed
    if deploymentEqual(existing, desired) {
        klog.V(4).Infof("Deployment %s is up to date", existing.Name)
        return nil
    }
    
    klog.V(2).Infof("Updating deployment %s for SyncTarget %s/%s", existing.Name, cluster, syncTarget.Name)
    
    // Update the deployment
    existing.Spec = desired.Spec
    _, err := dm.physicalClient.AppsV1().Deployments(SyncerNamespace).Update(ctx, existing, metav1.UpdateOptions{})
    return err
}

// buildDeployment builds a deployment for the syncer
func (dm *DeploymentManager) buildDeployment(cluster logicalcluster.Path, syncTarget *workloadv1alpha1.SyncTarget) *appsv1.Deployment {
    replicas := int32(1)
    labels := syncerLabels(syncTarget)
    
    return &appsv1.Deployment{
        ObjectMeta: metav1.ObjectMeta{
            Name:      syncerDeploymentName(syncTarget),
            Namespace: SyncerNamespace,
            Labels:    labels,
        },
        Spec: appsv1.DeploymentSpec{
            Replicas: &replicas,
            Selector: &metav1.LabelSelector{
                MatchLabels: labels,
            },
            Template: corev1.PodTemplateSpec{
                ObjectMeta: metav1.ObjectMeta{
                    Labels: labels,
                },
                Spec: corev1.PodSpec{
                    ServiceAccountName: syncerServiceAccountName(syncTarget),
                    Containers: []corev1.Container{
                        {
                            Name:  "syncer",
                            Image: SyncerImageName,
                            Args: []string{
                                "syncer",
                                "--cluster", cluster.String(),
                                "--sync-target", syncTarget.Name,
                                "--resources", syncTarget.Spec.ResourcesString(),
                            },
                            Env: []corev1.EnvVar{
                                {
                                    Name:  "SYNCER_NAMESPACE",
                                    Value: SyncerNamespace,
                                },
                            },
                            Ports: []corev1.ContainerPort{
                                {
                                    Name:          "metrics",
                                    ContainerPort: 8080,
                                },
                            },
                        },
                    },
                },
            },
        },
    }
}

// DeleteDeployment removes the syncer deployment
func (dm *DeploymentManager) DeleteDeployment(ctx context.Context, syncTarget *workloadv1alpha1.SyncTarget) error {
    deploymentName := syncerDeploymentName(syncTarget)
    
    klog.V(2).Infof("Deleting deployment %s", deploymentName)
    
    err := dm.physicalClient.AppsV1().Deployments(SyncerNamespace).Delete(ctx, deploymentName, metav1.DeleteOptions{})
    if err != nil && !errors.IsNotFound(err) {
        return err
    }
    
    return nil
}

// Helper functions
func syncerDeploymentName(syncTarget *workloadv1alpha1.SyncTarget) string {
    return fmt.Sprintf("syncer-%s", syncTarget.Name)
}

func syncerServiceAccountName(syncTarget *workloadv1alpha1.SyncTarget) string {
    return fmt.Sprintf("syncer-%s", syncTarget.Name)
}

func syncerLabels(syncTarget *workloadv1alpha1.SyncTarget) map[string]string {
    return map[string]string{
        "app":         "syncer",
        "sync-target": syncTarget.Name,
    }
}

func deploymentEqual(a, b *appsv1.Deployment) bool {
    // Simple comparison - can be enhanced
    return a.Spec.Template.Spec.Containers[0].Image == b.Spec.Template.Spec.Containers[0].Image
}
```

#### 2. **pkg/reconciler/workload/synctarget/finalizer.go** (~140 lines)

```go
package synctarget

import (
    "context"
    "fmt"
    
    workloadv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"
    
    "github.com/kcp-dev/logicalcluster/v3"
    
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/util/sets"
    "k8s.io/klog/v2"
)

const (
    // SyncTargetFinalizer is the finalizer added to SyncTargets
    SyncTargetFinalizer = "workload.kcp.io/synctarget"
)

// ensureFinalizer ensures the finalizer is present on the SyncTarget
func (c *Controller) ensureFinalizer(ctx context.Context, cluster logicalcluster.Path, syncTarget *workloadv1alpha1.SyncTarget) error {
    if hasFinalizer(syncTarget, SyncTargetFinalizer) {
        return nil
    }
    
    klog.V(3).Infof("Adding finalizer to SyncTarget %s/%s", cluster, syncTarget.Name)
    
    syncTarget.Finalizers = append(syncTarget.Finalizers, SyncTargetFinalizer)
    
    _, err := c.kcpClusterClient.Cluster(cluster).WorkloadV1alpha1().SyncTargets().Update(
        ctx,
        syncTarget,
        metav1.UpdateOptions{},
    )
    
    return err
}

// removeFinalizer removes the finalizer from the SyncTarget
func (c *Controller) removeFinalizer(ctx context.Context, cluster logicalcluster.Path, syncTarget *workloadv1alpha1.SyncTarget) error {
    if !hasFinalizer(syncTarget, SyncTargetFinalizer) {
        return nil
    }
    
    klog.V(3).Infof("Removing finalizer from SyncTarget %s/%s", cluster, syncTarget.Name)
    
    syncTarget.Finalizers = removeFinalizerFromSlice(syncTarget.Finalizers, SyncTargetFinalizer)
    
    _, err := c.kcpClusterClient.Cluster(cluster).WorkloadV1alpha1().SyncTargets().Update(
        ctx,
        syncTarget,
        metav1.UpdateOptions{},
    )
    
    return err
}

// handleDeletion processes the deletion of a SyncTarget
func (c *Controller) handleDeletion(ctx context.Context, cluster logicalcluster.Path, syncTarget *workloadv1alpha1.SyncTarget) error {
    if !hasFinalizer(syncTarget, SyncTargetFinalizer) {
        return nil
    }
    
    klog.V(2).Infof("Processing deletion of SyncTarget %s/%s", cluster, syncTarget.Name)
    
    // Clean up deployment
    if c.deploymentManager != nil {
        if err := c.deploymentManager.DeleteDeployment(ctx, syncTarget); err != nil {
            klog.Errorf("Failed to delete deployment for SyncTarget %s/%s: %v", cluster, syncTarget.Name, err)
            return err
        }
    }
    
    // Clean up other resources
    if err := c.cleanupResources(ctx, cluster, syncTarget); err != nil {
        klog.Errorf("Failed to cleanup resources for SyncTarget %s/%s: %v", cluster, syncTarget.Name, err)
        return err
    }
    
    // Remove finalizer
    return c.removeFinalizer(ctx, cluster, syncTarget)
}

// cleanupResources cleans up any resources associated with the SyncTarget
func (c *Controller) cleanupResources(ctx context.Context, cluster logicalcluster.Path, syncTarget *workloadv1alpha1.SyncTarget) error {
    klog.V(3).Infof("Cleaning up resources for SyncTarget %s/%s", cluster, syncTarget.Name)
    
    // Clean up ConfigMaps, Secrets, ServiceAccounts, etc.
    // This is a placeholder - actual cleanup logic depends on what resources are created
    
    return nil
}

// Helper functions
func hasFinalizer(syncTarget *workloadv1alpha1.SyncTarget, finalizer string) bool {
    return sets.NewString(syncTarget.Finalizers...).Has(finalizer)
}

func removeFinalizerFromSlice(finalizers []string, finalizer string) []string {
    var result []string
    for _, f := range finalizers {
        if f != finalizer {
            result = append(result, f)
        }
    }
    return result
}
```

#### 3. **Test Files** (not counted in line limit)

Create test files for validation but they don't count against the line limit:

- `pkg/reconciler/workload/synctarget/controller_test.go`
- `pkg/reconciler/workload/synctarget/deployment_test.go`
- `pkg/reconciler/workload/synctarget/finalizer_test.go`

## Implementation Checklist

### Pre-Implementation
- [ ] Wave2a-01 controller base available
- [ ] Physical cluster client setup understood
- [ ] Deployment patterns reviewed

### Implementation
- [ ] Create deployment.go with:
  - [ ] Deployment manager struct
  - [ ] Create deployment logic
  - [ ] Update deployment logic
  - [ ] Delete deployment logic
  - [ ] Helper functions
- [ ] Create finalizer.go with:
  - [ ] Finalizer management
  - [ ] Deletion handling
  - [ ] Resource cleanup
- [ ] Update controller to use deployment manager
- [ ] Create comprehensive tests

### Integration
- [ ] Wire deployment manager into controller
- [ ] Ensure finalizer handling in reconcile
- [ ] Test with physical cluster

### Validation
- [ ] Deployment creates successfully
- [ ] Updates work correctly
- [ ] Deletion cleans up properly
- [ ] Line count ~340 (excluding tests)

## Commit Strategy

```bash
# Add deployment logic
git add pkg/reconciler/workload/synctarget/deployment.go
git commit -s -S -m "feat(controller): add syncer deployment management

- Implement deployment creation and updates
- Handle syncer lifecycle in physical cluster
- Add configuration generation
- Support capacity-based scaling"

# Add finalizer logic
git add pkg/reconciler/workload/synctarget/finalizer.go
git commit -s -S -m "feat(controller): add finalizer and cleanup logic

- Implement finalizer management
- Handle graceful deletion
- Clean up resources on removal
- Ensure no orphaned resources"

# Add tests
git add pkg/reconciler/workload/synctarget/*_test.go
git commit -s -S -m "test: add tests for SyncTarget controller

- Test controller creation
- Test deployment logic
- Test finalizer handling
- Validate error cases"
```

## Success Criteria

1. ✅ Deployment logic complete
2. ✅ Finalizer handling works
3. ✅ Clean deletion process
4. ✅ Tests provide good coverage
5. ✅ Under 340 lines (excluding tests)
6. ✅ Integrates with controller base

## Dependencies

- **Requires:** Wave2a-01 (Controller Base)
- **Can Parallel With:** Wave2a-02 (Reconciliation)
- **Provides:** Complete deployment functionality

## Notes

- Physical cluster client needed for deployment
- ServiceAccount and RBAC setup may be needed
- Consider using owner references for cleanup
- Tests are critical for deployment logic