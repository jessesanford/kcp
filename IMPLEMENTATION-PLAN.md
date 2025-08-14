# Implementation Plan: PR2 - SyncTarget Controller Core

## Overview
**PR Title**: feat(controller): add core SyncTarget controller without deployment
**Target Size**: 484 lines (excluding generated code)
**Dependencies**: PR1 (needs API types) - base this branch on PR1 branch

## Setup Requirements

### Base Branch Setup
```bash
# This branch should be based on PR1
git checkout -b feature/tmc2-impl2/phase2/wave2a-03b-synctarget-core \
  origin/feature/tmc2-impl2/phase2/wave2a-03a-api-types
```

## Files to Extract from Source

### Step 1: Create Directory Structure
```bash
mkdir -p pkg/reconciler/workload/synctarget
```

### Step 2: Copy and Modify Core Controller Files

#### File 1: `pkg/reconciler/workload/synctarget/doc.go`
Copy directly from source (18 lines)

#### File 2: `pkg/reconciler/workload/synctarget/controller.go`
**Modifications Required**: Remove deployment and finalizer references
- Copy the file
- Remove imports for deployment manager
- Remove deployment reconciliation calls
- Keep only core reconciliation logic
- Expected size after modifications: ~150 lines

**Key modifications**:
```go
// Remove from reconcile function:
// - deployment.EnsureDeployment() calls
// - finalizer.HandleFinalizer() calls
// Keep:
// - Basic status updates
// - Condition management
// - Event recording
```

#### File 3: `pkg/reconciler/workload/synctarget/controller_clean.go`
Copy directly, but simplify:
- Remove deployment cleanup logic
- Keep basic cleanup patterns
- Expected size: ~100 lines

#### File 4: `pkg/reconciler/workload/synctarget/indexes_foundation.go`
Copy directly from source (96 lines)

## New Code to Add

### Step 3: Create Simplified Controller
Create `pkg/reconciler/workload/synctarget/controller_simple.go`:

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
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	workloadv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/workload/v1alpha1"
	kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
	workloadinformers "github.com/kcp-dev/kcp/sdk/client/informers/externalversions/workload/v1alpha1"
)

const (
	ControllerName = "kcp-synctarget"
)

// Controller watches SyncTargets and manages their lifecycle
type Controller struct {
	queue workqueue.RateLimitingInterface

	kcpClusterClient kcpclientset.ClusterInterface

	syncTargetLister  workloadinformers.SyncTargetClusterLister
	syncTargetIndexer cache.Indexer
}

// NewController creates a new SyncTarget controller
func NewController(
	kcpClusterClient kcpclientset.ClusterInterface,
	syncTargetInformer workloadinformers.SyncTargetClusterInformer,
) (*Controller, error) {
	c := &Controller{
		queue:             workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), ControllerName),
		kcpClusterClient:  kcpClusterClient,
		syncTargetLister:  syncTargetInformer.Lister(),
		syncTargetIndexer: syncTargetInformer.Informer().GetIndexer(),
	}

	syncTargetInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { c.enqueue(obj) },
		UpdateFunc: func(_, obj interface{}) { c.enqueue(obj) },
		DeleteFunc: func(obj interface{}) { c.enqueue(obj) },
	})

	return c, nil
}

func (c *Controller) enqueue(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	c.queue.Add(key)
}

// Start begins processing items from the work queue
func (c *Controller) Start(ctx context.Context, numThreads int) {
	defer runtime.HandleCrash()
	defer c.queue.ShutDown()

	klog.Infof("Starting %s controller", ControllerName)
	defer klog.Infof("Shutting down %s controller", ControllerName)

	for i := 0; i < numThreads; i++ {
		go wait.Until(func() { c.startWorker(ctx) }, time.Second, ctx.Done())
	}

	<-ctx.Done()
}

func (c *Controller) startWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

func (c *Controller) processNextWorkItem(ctx context.Context) bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)

	err := c.reconcile(ctx, key.(string))
	if err == nil {
		c.queue.Forget(key)
		return true
	}

	runtime.HandleError(fmt.Errorf("%s: failed to reconcile %q: %w", ControllerName, key, err))
	c.queue.AddRateLimited(key)

	return true
}

func (c *Controller) reconcile(ctx context.Context, key string) error {
	klog.V(2).Infof("Reconciling SyncTarget %q", key)

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	syncTarget, err := c.syncTargetLister.Cluster(logicalcluster.From(namespace)).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			klog.V(2).Infof("SyncTarget %q not found", key)
			return nil
		}
		return err
	}

	// Update status conditions
	syncTargetCopy := syncTarget.DeepCopy()
	
	// Set Ready condition based on basic validation
	readyCondition := metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		ObservedGeneration: syncTarget.Generation,
		Reason:             "SyncTargetReady",
		Message:            "SyncTarget is ready",
		LastTransitionTime: metav1.Now(),
	}

	// Simple validation - in PR3 this will check deployment status
	if syncTarget.Spec.Location == "" {
		readyCondition.Status = metav1.ConditionFalse
		readyCondition.Reason = "MissingLocation"
		readyCondition.Message = "SyncTarget location is required"
	}

	meta.SetStatusCondition(&syncTargetCopy.Status.Conditions, readyCondition)

	// Update status if changed
	if !equality.Semantic.DeepEqual(syncTarget.Status, syncTargetCopy.Status) {
		_, err = c.kcpClusterClient.Cluster(logicalcluster.From(namespace)).
			WorkloadV1alpha1().
			SyncTargets().
			UpdateStatus(ctx, syncTargetCopy, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}
```

### Step 4: Add Controller Tests
Create `pkg/reconciler/workload/synctarget/controller_test.go`:

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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	workloadv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/workload/v1alpha1"
	kcpfakeclient "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster/fake"
	"github.com/kcp-dev/kcp/sdk/client/informers/externalversions"
)

func TestControllerReconcile(t *testing.T) {
	tests := []struct {
		name           string
		syncTarget     *workloadv1alpha1.SyncTarget
		wantConditions []metav1.Condition
	}{
		{
			name: "valid sync target becomes ready",
			syncTarget: &workloadv1alpha1.SyncTarget{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-target",
				},
				Spec: workloadv1alpha1.SyncTargetSpec{
					Location: "us-west-2",
				},
			},
			wantConditions: []metav1.Condition{
				{
					Type:   "Ready",
					Status: metav1.ConditionTrue,
					Reason: "SyncTargetReady",
				},
			},
		},
		{
			name: "sync target without location not ready",
			syncTarget: &workloadv1alpha1.SyncTarget{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-target",
				},
				Spec: workloadv1alpha1.SyncTargetSpec{},
			},
			wantConditions: []metav1.Condition{
				{
					Type:   "Ready",
					Status: metav1.ConditionFalse,
					Reason: "MissingLocation",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			scheme := runtime.NewScheme()
			_ = workloadv1alpha1.AddToScheme(scheme)

			fakeClient := kcpfakeclient.NewSimpleClientset(tt.syncTarget)
			informerFactory := externalversions.NewSharedInformerFactory(fakeClient, 0)
			
			controller, err := NewController(
				fakeClient,
				informerFactory.Workload().V1alpha1().SyncTargets(),
			)
			if err != nil {
				t.Fatalf("Failed to create controller: %v", err)
			}

			informerFactory.Start(ctx.Done())
			informerFactory.WaitForCacheSync(ctx.Done())

			// Run one reconciliation
			key := "default/test-target"
			err = controller.reconcile(ctx, key)
			if err != nil {
				t.Fatalf("Reconcile failed: %v", err)
			}

			// Check conditions
			updated, err := fakeClient.WorkloadV1alpha1().SyncTargets("default").Get(ctx, tt.syncTarget.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("Failed to get updated SyncTarget: %v", err)
			}

			for _, wantCond := range tt.wantConditions {
				found := false
				for _, cond := range updated.Status.Conditions {
					if cond.Type == wantCond.Type {
						found = true
						if cond.Status != wantCond.Status {
							t.Errorf("Condition %s: got status %s, want %s", cond.Type, cond.Status, wantCond.Status)
						}
						if cond.Reason != wantCond.Reason {
							t.Errorf("Condition %s: got reason %s, want %s", cond.Type, cond.Reason, wantCond.Reason)
						}
						break
					}
				}
				if !found {
					t.Errorf("Expected condition %s not found", wantCond.Type)
				}
			}
		})
	}
}
```

## Testing Requirements

### Unit Tests
- [x] Controller creation and initialization
- [x] Basic reconciliation logic
- [x] Status condition updates
- [x] Error handling

### Integration Tests
- Will be expanded in PR3 with deployment management

## Commit Structure

### Commit 1: Add core controller structure
```bash
git add pkg/reconciler/workload/synctarget/doc.go
git add pkg/reconciler/workload/synctarget/controller_simple.go
git add pkg/reconciler/workload/synctarget/indexes_foundation.go
git commit -s -S -m "feat(controller): add core SyncTarget controller

- Implement basic controller structure with workqueue
- Add reconciliation loop for SyncTarget resources
- Include status condition management
- Set up indexers for efficient lookups

Part of TMC Phase 2 Wave 2A implementation"
```

### Commit 2: Add controller tests
```bash
git add pkg/reconciler/workload/synctarget/controller_test.go
git commit -s -S -m "test(controller): add SyncTarget controller unit tests

- Test basic reconciliation flow
- Verify status condition updates
- Ensure proper error handling"
```

### Commit 3: Add cleanup logic
```bash
git add pkg/reconciler/workload/synctarget/controller_clean.go
git commit -s -S -m "feat(controller): add cleanup logic for SyncTarget

- Implement resource cleanup patterns
- Prepare for finalizer support (PR3)
- Ensure proper resource lifecycle management"
```

## Line Count Verification

Before pushing:
```bash
/workspaces/kcp-shared-tools/tmc-pr-line-counter.sh -c feature/tmc2-impl2/phase2/wave2a-03b-synctarget-core
```

Expected count: ~450 lines (excluding generated code)

## PR Description Template

```markdown
## Summary
This PR implements the core SyncTarget controller that manages the lifecycle of SyncTarget resources. This is the foundation controller without deployment management, which will be added in PR3.

## What Type of PR Is This?
/kind feature

## Changes
- Added core SyncTarget controller with reconciliation loop
- Implemented status condition management
- Added indexers for efficient resource lookups
- Included comprehensive unit tests
- Set up cleanup patterns for resource management

## Testing
- ✅ Unit tests for controller reconciliation
- ✅ Status condition updates verified
- ✅ Error handling tested

## Documentation
- Controller includes comprehensive godoc comments
- Reconciliation logic is well-documented

## Dependencies
- Requires PR1 (API types) to be merged first

## Related Issue(s)
Part of TMC Phase 2 Wave 2A implementation

## Release Notes
```release-note
Add core SyncTarget controller for managing cluster registration lifecycle
```
```

## Success Criteria Checklist

- [ ] Based on PR1 branch (has API types)
- [ ] Core controller logic extracted and simplified
- [ ] Deployment/finalizer logic removed (for PR3)
- [ ] Tests added and passing
- [ ] Line count under 500 (excluding generated)
- [ ] Commits signed with DCO and GPG
- [ ] No binary files committed
- [ ] PR description complete
- [ ] Ready for review

## Notes for Implementation

1. This PR focuses on the core controller logic only
2. Deployment management is intentionally excluded (coming in PR3)
3. Keep the reconciliation simple - just status updates
4. The controller should compile and run but won't do deployment yet
5. Tests should verify basic controller behavior