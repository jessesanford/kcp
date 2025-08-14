# Implementation Plan: PR3 - SyncTarget Deployment Management

## Overview
**PR Title**: feat(controller): add deployment and lifecycle management for SyncTargets
**Target Size**: 476 lines (excluding generated code)
**Dependencies**: PR2 (extends SyncTarget controller) - base this branch on PR2 branch

## Setup Requirements

### Base Branch Setup
```bash
# This branch should be based on PR2
git checkout -b feature/tmc2-impl2/phase2/wave2a-03c-deployment-mgmt \
  origin/feature/tmc2-impl2/phase2/wave2a-03b-synctarget-core
```

## Files to Copy from Source

### Step 1: Copy Deployment Management Files

#### File 1: `pkg/reconciler/workload/synctarget/deployment.go`
Copy directly from source (263 lines)
- This file contains the deployment creation and management logic
- Handles Deployment resource lifecycle
- Manages deployment updates and rollouts

#### File 2: `pkg/reconciler/workload/synctarget/finalizer.go`
Copy directly from source (213 lines)
- Handles finalizer logic for cleanup
- Ensures proper resource deletion ordering
- Manages deployment cleanup on SyncTarget deletion

## Files to Modify from PR2

### Step 2: Update Core Controller

Modify `pkg/reconciler/workload/synctarget/controller.go` (from PR2):

```go
// Add to imports:
import (
    appsv1 "k8s.io/api/apps/v1"
    corev1 "k8s.io/api/core/v1"
    "k8s.io/client-go/kubernetes"
    appsv1informers "k8s.io/client-go/informers/apps/v1"
)

// Add to Controller struct:
type Controller struct {
    // ... existing fields ...
    
    kubeClient        kubernetes.Interface
    deploymentLister  appsv1informers.DeploymentLister
    deploymentIndexer cache.Indexer
}

// Update NewController to accept kubernetes client:
func NewController(
    kcpClusterClient kcpclientset.ClusterInterface,
    kubeClient kubernetes.Interface,  // Add this
    syncTargetInformer workloadinformers.SyncTargetClusterInformer,
    deploymentInformer appsv1informers.DeploymentInformer,  // Add this
) (*Controller, error) {
    c := &Controller{
        // ... existing initialization ...
        kubeClient:        kubeClient,
        deploymentLister:  deploymentInformer.Lister(),
        deploymentIndexer: deploymentInformer.Informer().GetIndexer(),
    }
    // ... rest of initialization ...
}

// Update reconcile function to call deployment manager:
func (c *Controller) reconcile(ctx context.Context, key string) error {
    // ... existing code ...
    
    // Add finalizer handling
    if !syncTarget.DeletionTimestamp.IsZero() {
        return c.handleFinalizer(ctx, syncTarget)
    }
    
    // Ensure finalizer is present
    if err := c.ensureFinalizer(ctx, syncTarget); err != nil {
        return err
    }
    
    // Manage deployment
    deployment, err := c.ensureDeployment(ctx, syncTarget)
    if err != nil {
        return err
    }
    
    // Update status based on deployment
    c.updateStatusFromDeployment(syncTargetCopy, deployment)
    
    // ... rest of existing code ...
}
```

## New Code to Add

### Step 3: Add Deployment Manager Tests
Create `pkg/reconciler/workload/synctarget/deployment_test.go`:

```go
/*
Copyright 2024 The KCP Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package synctarget

import (
    "context"
    "testing"

    appsv1 "k8s.io/api/apps/v1"
    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/client-go/kubernetes/fake"

    workloadv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/workload/v1alpha1"
)

func TestEnsureDeployment(t *testing.T) {
    tests := []struct {
        name              string
        syncTarget        *workloadv1alpha1.SyncTarget
        existingDeployment *appsv1.Deployment
        wantReplicas      int32
        wantError         bool
    }{
        {
            name: "create new deployment",
            syncTarget: &workloadv1alpha1.SyncTarget{
                ObjectMeta: metav1.ObjectMeta{
                    Name:      "test-target",
                    Namespace: "default",
                },
                Spec: workloadv1alpha1.SyncTargetSpec{
                    Location: "us-west-2",
                },
            },
            existingDeployment: nil,
            wantReplicas:      1,
            wantError:         false,
        },
        {
            name: "update existing deployment",
            syncTarget: &workloadv1alpha1.SyncTarget{
                ObjectMeta: metav1.ObjectMeta{
                    Name:      "test-target",
                    Namespace: "default",
                },
                Spec: workloadv1alpha1.SyncTargetSpec{
                    Location: "us-west-2",
                },
            },
            existingDeployment: &appsv1.Deployment{
                ObjectMeta: metav1.ObjectMeta{
                    Name:      "synctarget-test-target",
                    Namespace: "default",
                },
                Spec: appsv1.DeploymentSpec{
                    Replicas: ptr.To[int32](2),
                },
            },
            wantReplicas: 1,
            wantError:    false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            ctx := context.Background()
            
            kubeClient := fake.NewSimpleClientset()
            if tt.existingDeployment != nil {
                _, err := kubeClient.AppsV1().Deployments(tt.existingDeployment.Namespace).
                    Create(ctx, tt.existingDeployment, metav1.CreateOptions{})
                if err != nil {
                    t.Fatalf("Failed to create existing deployment: %v", err)
                }
            }

            c := &Controller{
                kubeClient: kubeClient,
            }

            deployment, err := c.ensureDeployment(ctx, tt.syncTarget)
            if (err != nil) != tt.wantError {
                t.Errorf("ensureDeployment() error = %v, wantError %v", err, tt.wantError)
                return
            }

            if !tt.wantError {
                if deployment == nil {
                    t.Fatal("Expected deployment to be created")
                }
                if *deployment.Spec.Replicas != tt.wantReplicas {
                    t.Errorf("Deployment replicas = %d, want %d", *deployment.Spec.Replicas, tt.wantReplicas)
                }
            }
        })
    }
}

func TestHandleFinalizer(t *testing.T) {
    tests := []struct {
        name              string
        syncTarget        *workloadv1alpha1.SyncTarget
        existingDeployment *appsv1.Deployment
        wantDeploymentDeleted bool
        wantFinalizerRemoved  bool
    }{
        {
            name: "cleanup deployment and remove finalizer",
            syncTarget: &workloadv1alpha1.SyncTarget{
                ObjectMeta: metav1.ObjectMeta{
                    Name:              "test-target",
                    Namespace:         "default",
                    DeletionTimestamp: &metav1.Time{},
                    Finalizers:        []string{SyncTargetFinalizer},
                },
            },
            existingDeployment: &appsv1.Deployment{
                ObjectMeta: metav1.ObjectMeta{
                    Name:      "synctarget-test-target",
                    Namespace: "default",
                },
            },
            wantDeploymentDeleted: true,
            wantFinalizerRemoved:  true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            ctx := context.Background()
            
            kubeClient := fake.NewSimpleClientset()
            if tt.existingDeployment != nil {
                _, err := kubeClient.AppsV1().Deployments(tt.existingDeployment.Namespace).
                    Create(ctx, tt.existingDeployment, metav1.CreateOptions{})
                if err != nil {
                    t.Fatalf("Failed to create existing deployment: %v", err)
                }
            }

            c := &Controller{
                kubeClient: kubeClient,
            }

            err := c.handleFinalizer(ctx, tt.syncTarget)
            if err != nil {
                t.Fatalf("handleFinalizer() error = %v", err)
            }

            // Check if deployment was deleted
            deployments, err := kubeClient.AppsV1().Deployments(tt.syncTarget.Namespace).List(ctx, metav1.ListOptions{})
            if err != nil {
                t.Fatalf("Failed to list deployments: %v", err)
            }

            if tt.wantDeploymentDeleted && len(deployments.Items) > 0 {
                t.Error("Expected deployment to be deleted but it still exists")
            }
            if !tt.wantDeploymentDeleted && len(deployments.Items) == 0 {
                t.Error("Expected deployment to exist but it was deleted")
            }
        })
    }
}
```

### Step 4: Add Integration Helpers
Create `pkg/reconciler/workload/synctarget/helpers.go`:

```go
/*
Copyright 2024 The KCP Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package synctarget

import (
    appsv1 "k8s.io/api/apps/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

    workloadv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/workload/v1alpha1"
)

// updateStatusFromDeployment updates the SyncTarget status based on deployment state
func (c *Controller) updateStatusFromDeployment(syncTarget *workloadv1alpha1.SyncTarget, deployment *appsv1.Deployment) {
    if deployment == nil {
        meta.SetStatusCondition(&syncTarget.Status.Conditions, metav1.Condition{
            Type:               "DeploymentReady",
            Status:             metav1.ConditionFalse,
            ObservedGeneration: syncTarget.Generation,
            Reason:             "DeploymentNotFound",
            Message:            "Deployment not found",
            LastTransitionTime: metav1.Now(),
        })
        return
    }

    // Check deployment conditions
    deploymentReady := false
    for _, cond := range deployment.Status.Conditions {
        if cond.Type == appsv1.DeploymentAvailable && cond.Status == corev1.ConditionTrue {
            deploymentReady = true
            break
        }
    }

    if deploymentReady {
        meta.SetStatusCondition(&syncTarget.Status.Conditions, metav1.Condition{
            Type:               "DeploymentReady",
            Status:             metav1.ConditionTrue,
            ObservedGeneration: syncTarget.Generation,
            Reason:             "DeploymentAvailable",
            Message:            "Deployment is available",
            LastTransitionTime: metav1.Now(),
        })
    } else {
        meta.SetStatusCondition(&syncTarget.Status.Conditions, metav1.Condition{
            Type:               "DeploymentReady",
            Status:             metav1.ConditionFalse,
            ObservedGeneration: syncTarget.Generation,
            Reason:             "DeploymentNotReady",
            Message:            "Deployment is not ready",
            LastTransitionTime: metav1.Now(),
        })
    }

    // Update replicas status
    syncTarget.Status.Replicas = deployment.Status.Replicas
    syncTarget.Status.ReadyReplicas = deployment.Status.ReadyReplicas
    syncTarget.Status.AvailableReplicas = deployment.Status.AvailableReplicas
}

// deploymentName returns the deployment name for a SyncTarget
func deploymentName(syncTarget *workloadv1alpha1.SyncTarget) string {
    return "synctarget-" + syncTarget.Name
}
```

## Testing Requirements

### Unit Tests
- [x] Deployment creation and updates
- [x] Finalizer handling and cleanup
- [x] Status updates from deployment state
- [x] Error handling during deployment operations

### Integration Tests
- Test full controller flow with deployment management
- Verify cleanup on deletion

## Commit Structure

### Commit 1: Add deployment management
```bash
git add pkg/reconciler/workload/synctarget/deployment.go
git commit -s -S -m "feat(controller): add deployment management for SyncTarget

- Implement deployment creation and updates
- Handle deployment lifecycle based on SyncTarget spec
- Manage deployment configuration and rollouts

Part of TMC Phase 2 Wave 2A implementation"
```

### Commit 2: Add finalizer handling
```bash
git add pkg/reconciler/workload/synctarget/finalizer.go
git commit -s -S -m "feat(controller): add finalizer for SyncTarget cleanup

- Implement finalizer to ensure proper cleanup
- Handle deployment deletion on SyncTarget removal
- Ensure ordered resource cleanup"
```

### Commit 3: Integrate deployment with controller
```bash
# Update controller.go with deployment integration
git add pkg/reconciler/workload/synctarget/controller.go
git add pkg/reconciler/workload/synctarget/helpers.go
git commit -s -S -m "feat(controller): integrate deployment management with SyncTarget controller

- Wire deployment manager into reconciliation loop
- Update status based on deployment state
- Add helper functions for deployment operations"
```

### Commit 4: Add tests
```bash
git add pkg/reconciler/workload/synctarget/deployment_test.go
git commit -s -S -m "test(controller): add tests for deployment management

- Test deployment creation and updates
- Verify finalizer handling
- Ensure proper cleanup on deletion"
```

## Line Count Verification

Before pushing:
```bash
/workspaces/kcp-shared-tools/tmc-pr-line-counter.sh -c feature/tmc2-impl2/phase2/wave2a-03c-deployment-mgmt
```

Expected count: ~600 lines (excluding generated code)

## PR Description Template

```markdown
## Summary
This PR extends the SyncTarget controller with deployment management capabilities. It adds the ability to create and manage Kubernetes Deployments for each SyncTarget, handle lifecycle operations, and ensure proper cleanup on deletion.

## What Type of PR Is This?
/kind feature

## Changes
- Added deployment creation and management for SyncTargets
- Implemented finalizer for proper resource cleanup
- Integrated deployment status into SyncTarget status conditions
- Added comprehensive tests for deployment operations

## Testing
- ✅ Unit tests for deployment creation and updates
- ✅ Finalizer handling tested
- ✅ Status update logic verified
- ✅ Cleanup on deletion tested

## Documentation
- Deployment logic is well-documented
- Helper functions include clear descriptions

## Dependencies
- Requires PR2 (SyncTarget controller core) to be merged first

## Related Issue(s)
Part of TMC Phase 2 Wave 2A implementation

## Release Notes
```release-note
Add deployment management to SyncTarget controller for automated syncer deployment
```
```

## Success Criteria Checklist

- [ ] Based on PR2 branch (has core controller)
- [ ] Deployment management files copied correctly
- [ ] Controller updated to integrate deployment
- [ ] Finalizer logic working properly
- [ ] Tests added and passing
- [ ] Line count under 700 (excluding generated)
- [ ] Commits signed with DCO and GPG
- [ ] No binary files committed
- [ ] PR description complete
- [ ] Ready for review

## Notes for Implementation

1. This PR adds the "meat" to the controller - actual deployment management
2. The deployment.go and finalizer.go files can be copied mostly as-is
3. Main work is integrating them with the simplified controller from PR2
4. Tests should verify the full lifecycle: create, update, delete
5. Make sure finalizer prevents orphaned deployments