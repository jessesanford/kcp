# Implementation Instructions: Wave2a-03 - Deployment Logic

## 🎯 Objective
Implement syncer deployment management and finalizer logic (~340 lines)

## 📋 Prerequisites
- Wave2a-01 controller foundation must be complete
- Physical cluster client understanding required

## ⚠️ CRITICAL: Implementation Approach
**YOU MUST CREATE NEW CODE** - The to-be-split branch only contains API types, NOT controller implementation.
- Cherry-pick Wave2a-01 first: `git cherry-pick <Wave2a-01-commit-hash>`
- CREATE all deployment logic from scratch
- DO NOT look for existing controller code in the to-be-split branch (it doesn't exist)

## 🔨 Implementation Tasks

### 1. Create `pkg/reconciler/workload/synctarget/deployment.go` (~200 lines)

**DeploymentManager Structure:**
```go
type DeploymentManager struct {
    physicalClient kubernetes.Interface
}

func NewDeploymentManager(physicalClient kubernetes.Interface) *DeploymentManager
```

**Core Functions to Implement:**
```go
// Main deployment ensure function
func (dm *DeploymentManager) EnsureDeployment(ctx context.Context, cluster logicalcluster.Path, syncTarget *workloadv1alpha1.SyncTarget) error

// Create new deployment
func (dm *DeploymentManager) createDeployment(ctx context.Context, cluster logicalcluster.Path, syncTarget *workloadv1alpha1.SyncTarget) error

// Update existing deployment
func (dm *DeploymentManager) updateDeployment(ctx context.Context, cluster logicalcluster.Path, syncTarget *workloadv1alpha1.SyncTarget, existing *appsv1.Deployment) error

// Build deployment spec
func (dm *DeploymentManager) buildDeployment(cluster logicalcluster.Path, syncTarget *workloadv1alpha1.SyncTarget) *appsv1.Deployment

// Delete deployment
func (dm *DeploymentManager) DeleteDeployment(ctx context.Context, syncTarget *workloadv1alpha1.SyncTarget) error
```

**Deployment Configuration:**
- Namespace: `kcp-syncer-system`
- Image: `ghcr.io/kcp-dev/kcp/syncer:latest`
- ServiceAccount: `syncer-{syncTargetName}`
- Labels: `app=syncer`, `sync-target={name}`

### 2. Create `pkg/reconciler/workload/synctarget/finalizer.go` (~140 lines)

**Finalizer Management Functions:**
```go
// Ensure finalizer is present
func (c *Controller) ensureFinalizer(ctx context.Context, cluster logicalcluster.Path, syncTarget *workloadv1alpha1.SyncTarget) error

// Remove finalizer
func (c *Controller) removeFinalizer(ctx context.Context, cluster logicalcluster.Path, syncTarget *workloadv1alpha1.SyncTarget) error

// Handle deletion with cleanup
func (c *Controller) handleDeletion(ctx context.Context, cluster logicalcluster.Path, syncTarget *workloadv1alpha1.SyncTarget) error

// Cleanup associated resources
func (c *Controller) cleanupResources(ctx context.Context, cluster logicalcluster.Path, syncTarget *workloadv1alpha1.SyncTarget) error
```

**Finalizer Constant:**
```go
const SyncTargetFinalizer = "workload.kcp.io/synctarget"
```

### 3. Integration with Controller

Update existing controller to:
- Initialize DeploymentManager in NewController
- Wire deployment logic into reconcile
- Add finalizer handling in reconciliation
- Replace deployment stubs from Wave2a-02

## 📝 Critical Implementation Notes

### Deployment Spec Requirements
```go
Spec: appsv1.DeploymentSpec{
    Replicas: &replicas, // Start with 1
    Selector: &metav1.LabelSelector{
        MatchLabels: labels,
    },
    Template: corev1.PodTemplateSpec{
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
                    },
                },
            },
        },
    },
}
```

### Deletion Flow
1. Check for finalizer presence
2. Clean up deployment
3. Clean up other resources (ConfigMaps, Secrets, ServiceAccounts)
4. Remove finalizer
5. Let Kubernetes delete the object

### Helper Functions
```go
func syncerDeploymentName(syncTarget *workloadv1alpha1.SyncTarget) string
func syncerServiceAccountName(syncTarget *workloadv1alpha1.SyncTarget) string
func syncerLabels(syncTarget *workloadv1alpha1.SyncTarget) map[string]string
func deploymentEqual(a, b *appsv1.Deployment) bool
```

## ✅ Validation Steps

1. **Compile Check**
   ```bash
   go build ./pkg/reconciler/workload/synctarget/...
   ```

2. **Test Deployment Creation**
   ```bash
   # In test environment
   kubectl get deployments -n kcp-syncer-system
   ```

3. **Test Finalizer**
   ```bash
   # Create and delete a SyncTarget
   kubectl delete synctarget test-target
   # Verify cleanup happens
   ```

4. **Line Count Verification**
   ```bash
   /workspaces/kcp-shared-tools/tmc-pr-line-counter.sh -c $(git branch --show-current)
   ```
   Target: ~340 lines

## 🔄 Commit Structure

```bash
# Commit 1: Deployment management
git add pkg/reconciler/workload/synctarget/deployment.go
git commit -s -S -m "feat(controller): add syncer deployment management"

# Commit 2: Finalizer logic
git add pkg/reconciler/workload/synctarget/finalizer.go
git commit -s -S -m "feat(controller): add finalizer and cleanup logic"

# Commit 3: Integration and tests
git add pkg/reconciler/workload/synctarget/controller.go
git add pkg/reconciler/workload/synctarget/*_test.go
git commit -s -S -m "feat(controller): integrate deployment manager with controller"
```

## ⚠️ Important Reminders

- **DO** handle NotFound errors gracefully
- **DO** implement idempotent operations
- **DO** clean up all resources on deletion
- **DO NOT** leave orphaned resources
- **DO NOT** exceed 340 lines (excluding tests)
- **DO** consider RBAC requirements for deployment

## 🎯 Success Metrics

- [ ] Deployment creates successfully
- [ ] Updates work correctly
- [ ] Deletion cleans up all resources
- [ ] Finalizer prevents premature deletion
- [ ] No resource leaks
- [ ] Under 340 lines of code
- [ ] Tests provide good coverage