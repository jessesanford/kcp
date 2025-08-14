# Split Implementation: Wave2a-01 - Controller Foundation & Structure

## Overview
**Branch:** `feature/tmc-syncer-02a-controller-base`  
**Target Size:** ~400 lines  
**Dependencies:** Wave1 API Types must be available  
**Can Run In Parallel:** No - this is the controller foundation

## Implementation Tasks

### Step 1: Ensure API Types Are Available
Since this controller depends on the SyncTarget API types from Wave1:

```bash
# Option A: If Wave1 is merged
git fetch origin
git merge origin/main

# Option B: Copy API types from Wave1 branch
cp -r /workspaces/kcp-worktrees/phase2/wave1-01-split-from-api-foundation/pkg/apis/workload pkg/apis/
```

### Step 2: Create Controller Package Structure

```bash
# Create controller directories
mkdir -p pkg/reconciler/workload/synctarget
```

### Step 3: Create Controller Files

#### 1. **pkg/reconciler/workload/synctarget/doc.go** (~20 lines)
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

// Package synctarget implements a controller for managing SyncTarget resources
// in a workspace-aware manner following KCP patterns.
package synctarget
```

#### 2. **pkg/reconciler/workload/synctarget/controller.go** (~250 lines)
```go
package synctarget

import (
    "context"
    "fmt"
    "time"

    kcpcache "github.com/kcp-dev/kcp/pkg/cache"
    workloadv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"
    kcpclientset "github.com/kcp-dev/kcp/pkg/client/clientset/versioned/cluster"
    workloadinformers "github.com/kcp-dev/kcp/pkg/client/informers/externalversions/workload/v1alpha1"
    workloadlisters "github.com/kcp-dev/kcp/pkg/client/listers/workload/v1alpha1"
    
    "github.com/kcp-dev/logicalcluster/v3"
    
    "k8s.io/apimachinery/pkg/api/errors"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/util/runtime"
    "k8s.io/apimachinery/pkg/util/wait"
    "k8s.io/client-go/tools/cache"
    "k8s.io/client-go/util/workqueue"
    "k8s.io/klog/v2"
)

const (
    // ControllerName is the name of this controller
    ControllerName = "kcp-synctarget"
    
    // WorkerCount is the number of sync workers
    WorkerCount = 10
)

// Controller manages SyncTarget resources in a workspace-aware manner
type Controller struct {
    queue workqueue.RateLimitingInterface
    
    kcpClusterClient kcpclientset.ClusterInterface
    
    syncTargetLister  workloadlisters.SyncTargetClusterLister
    syncTargetIndexer cache.Indexer
    
    syncTargetInformer workloadinformers.SyncTargetClusterInformer
}

// NewController creates a new SyncTarget controller
func NewController(
    kcpClusterClient kcpclientset.ClusterInterface,
    syncTargetInformer workloadinformers.SyncTargetClusterInformer,
) (*Controller, error) {
    
    c := &Controller{
        queue: workqueue.NewNamedRateLimitingQueue(
            workqueue.DefaultControllerRateLimiter(),
            ControllerName,
        ),
        kcpClusterClient:    kcpClusterClient,
        syncTargetLister:    syncTargetInformer.Lister(),
        syncTargetIndexer:   syncTargetInformer.Informer().GetIndexer(),
        syncTargetInformer:  syncTargetInformer,
    }
    
    // Set up event handlers
    syncTargetInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
        AddFunc: c.enqueue,
        UpdateFunc: func(old, new interface{}) {
            c.enqueue(new)
        },
        DeleteFunc: c.enqueue,
    })
    
    return c, nil
}

// enqueue adds a SyncTarget to the work queue
func (c *Controller) enqueue(obj interface{}) {
    key, err := kcpcache.DeletionHandlingMetaClusterNamespaceKeyFunc(obj)
    if err != nil {
        runtime.HandleError(err)
        return
    }
    
    klog.V(4).Infof("Enqueuing SyncTarget %s", key)
    c.queue.Add(key)
}

// Start begins the controller loops
func (c *Controller) Start(ctx context.Context, workers int) {
    defer runtime.HandleCrash()
    defer c.queue.ShutDown()
    
    klog.Info("Starting SyncTarget controller")
    defer klog.Info("Shutting down SyncTarget controller")
    
    if !cache.WaitForCacheSync(ctx.Done(), c.syncTargetInformer.Informer().HasSynced) {
        runtime.HandleError(fmt.Errorf("failed to sync caches"))
        return
    }
    
    for i := 0; i < workers; i++ {
        go wait.UntilWithContext(ctx, c.worker, time.Second)
    }
    
    <-ctx.Done()
}

// worker processes items from the queue
func (c *Controller) worker(ctx context.Context) {
    for c.processNextItem(ctx) {
    }
}

// processNextItem handles one item from the queue
func (c *Controller) processNextItem(ctx context.Context) bool {
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
    
    runtime.HandleError(fmt.Errorf("error reconciling %v: %v", key, err))
    c.queue.AddRateLimited(key)
    
    return true
}

// Key represents a logical cluster and resource
type Key struct {
    Cluster   logicalcluster.Path
    Namespace string
    Name      string
}
```

#### 3. **pkg/reconciler/workload/synctarget/indexes.go** (~100 lines)
```go
package synctarget

import (
    workloadv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"
    "k8s.io/apimachinery/pkg/api/meta"
    "k8s.io/client-go/tools/cache"
)

const (
    // IndexByCluster is the index name for cluster-based lookups
    IndexByCluster = "spec.cluster"
    
    // IndexByWorkspace is the index name for workspace-based lookups
    IndexByWorkspace = "metadata.workspace"
)

// IndexByClusterFunc returns the cluster name for indexing
func IndexByClusterFunc(obj interface{}) ([]string, error) {
    target, ok := obj.(*workloadv1alpha1.SyncTarget)
    if !ok {
        return nil, nil
    }
    
    if target.Spec.Cluster == "" {
        return nil, nil
    }
    
    return []string{target.Spec.Cluster}, nil
}

// IndexByWorkspaceFunc returns the workspace for indexing
func IndexByWorkspaceFunc(obj interface{}) ([]string, error) {
    accessor, err := meta.Accessor(obj)
    if err != nil {
        return nil, err
    }
    
    workspace := accessor.GetAnnotations()["kcp.io/workspace"]
    if workspace == "" {
        return nil, nil
    }
    
    return []string{workspace}, nil
}

// AddIndexes adds custom indexes to the informer
func AddIndexes(informer cache.SharedIndexInformer) error {
    if err := informer.AddIndexers(cache.Indexers{
        IndexByCluster: IndexByClusterFunc,
    }); err != nil {
        return err
    }
    
    if err := informer.AddIndexers(cache.Indexers{
        IndexByWorkspace: IndexByWorkspaceFunc,
    }); err != nil {
        return err
    }
    
    return nil
}
```

#### 4. **cmd/syncer/main.go** updates (~30 lines to add)
Add to the existing main.go or create integration point:

```go
// Add to imports
import (
    synctargetcontroller "github.com/kcp-dev/kcp/pkg/reconciler/workload/synctarget"
)

// Add to controller setup
syncTargetController, err := synctargetcontroller.NewController(
    kcpClusterClient,
    syncTargetInformer,
)
if err != nil {
    return err
}

// Add to Start section
go syncTargetController.Start(ctx, 10)
```

## Implementation Checklist

### Pre-Implementation
- [ ] Create branch from main
- [ ] Ensure API types are available
- [ ] Set up workspace for development

### Implementation
- [ ] Create package structure
- [ ] Implement doc.go
- [ ] Implement controller.go with:
  - [ ] Controller struct
  - [ ] NewController function
  - [ ] Event handlers
  - [ ] Queue management
  - [ ] Worker loops
- [ ] Implement indexes.go with:
  - [ ] Cluster index
  - [ ] Workspace index
  - [ ] Index functions
- [ ] Update cmd integration

### Validation
- [ ] Code compiles
- [ ] Imports are correct
- [ ] Line count under 400
- [ ] No cyclic dependencies

### Testing
- [ ] Create basic unit tests
- [ ] Test controller creation
- [ ] Test indexing functions

## Commit Strategy

```bash
# Commit controller foundation
git add pkg/reconciler/workload/synctarget/
git commit -s -S -m "feat(controller): add SyncTarget controller foundation

- Create controller structure with workspace awareness
- Implement queue and worker management
- Add indexing for efficient lookups
- Follow KCP controller patterns"

# If cmd updates are separate
git add cmd/syncer/main.go
git commit -s -S -m "feat(cmd): wire SyncTarget controller into syncer"
```

## Success Criteria

1. ✅ Controller structure follows KCP patterns
2. ✅ Workspace isolation maintained
3. ✅ Proper event handling setup
4. ✅ Indexing for performance
5. ✅ Under 400 lines
6. ✅ Compiles successfully

## Notes for Next Splits

- Split 2 will add reconcile() implementation
- Split 3 will add deployment logic
- This foundation must be solid for others to build on