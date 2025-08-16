# PR3a: Core Controller & Abstractions - Implementation Instructions

## PR Overview
**Purpose**: Establish the controller foundation with clean abstractions for deployment management  
**Target Size**: 420 lines  
**Dependencies**: None (this is the foundation)  
**Base Branch**: main

## Files to Create

### 1. `pkg/reconciler/workload/synctarget/doc.go` (19 lines)
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

### 2. `pkg/reconciler/workload/synctarget/interfaces.go` (60 lines)
Create new file defining the abstractions:

```go
/*
Copyright 2024 The KCP Authors.
[License header - 14 lines]
*/

package synctarget

import (
	"context"

	"github.com/kcp-dev/logicalcluster/v3"
	
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	
	workloadv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/workload/v1alpha1"
)

// DeploymentManager abstracts syncer deployment operations
type DeploymentManager interface {
	// EnsureDeployment ensures a syncer deployment exists and is up-to-date
	EnsureDeployment(ctx context.Context, cluster logicalcluster.Path, target *workloadv1alpha1.SyncTarget) error
	
	// DeleteDeployment removes the syncer deployment
	DeleteDeployment(ctx context.Context, target *workloadv1alpha1.SyncTarget) error
	
	// GetDeploymentStatus retrieves the current deployment status
	GetDeploymentStatus(ctx context.Context, target *workloadv1alpha1.SyncTarget) (*DeploymentStatus, error)
}

// StatusUpdater abstracts status update operations
type StatusUpdater interface {
	// UpdateStatus updates the SyncTarget status based on deployment state
	UpdateStatus(ctx context.Context, target *workloadv1alpha1.SyncTarget, status *DeploymentStatus) error
}

// DeploymentStatus represents the state of a syncer deployment
type DeploymentStatus struct {
	Ready              bool
	Replicas           int32
	ReadyReplicas      int32
	AvailableReplicas  int32
	Condition          metav1.Condition
}

// ReconcileResult represents the outcome of a reconciliation
type ReconcileResult struct {
	Requeue      bool
	RequeueAfter time.Duration
	Error        error
}
```

### 3. `pkg/reconciler/workload/synctarget/controller.go` (180 lines)
Simplified controller focusing on structure and abstractions:

```go
/*
Copyright 2024 The KCP Authors.
[License header - 14 lines]
*/

package synctarget

import (
	"context"
	"fmt"
	"time"

	"github.com/kcp-dev/logicalcluster/v3"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
	kcpinformers "github.com/kcp-dev/kcp/sdk/client/informers/externalversions/cluster"
	workloadv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/workload/v1alpha1"
	workloadlisters "github.com/kcp-dev/kcp/sdk/client/listers/workload/v1alpha1/cluster"
)

const (
	// ControllerName is the name of this controller
	ControllerName = "synctarget-deployment"
	
	// WorkerCount is the default number of workers
	WorkerCount = 2
	
	// SyncTargetFinalizer is the finalizer we add to SyncTargets
	SyncTargetFinalizer = "workload.kcp.io/synctarget-deployment"
)

// Controller manages SyncTarget resources and their associated syncer deployments
type Controller struct {
	queue workqueue.RateLimitingInterface

	kcpClusterClient kcpclientset.ClusterInterface
	kubeClient       kubernetes.Interface

	syncTargetLister workloadlisters.SyncTargetClusterLister
	syncTargetSynced cache.InformerSynced

	// Abstractions for deployment and status management
	deploymentManager DeploymentManager
	statusUpdater     StatusUpdater
}

// NewController creates a new SyncTarget controller with deployment management
func NewController(
	kcpClusterClient kcpclientset.ClusterInterface,
	kubeClient kubernetes.Interface,
	syncTargetInformer kcpinformers.SyncTargetClusterInformer,
	deploymentManager DeploymentManager,
	statusUpdater StatusUpdater,
) (*Controller, error) {
	c := &Controller{
		queue: workqueue.NewNamedRateLimitingQueue(
			workqueue.DefaultControllerRateLimiter(),
			ControllerName,
		),

		kcpClusterClient: kcpClusterClient,
		kubeClient:       kubeClient,

		syncTargetLister: syncTargetInformer.Lister(),
		syncTargetSynced: syncTargetInformer.Informer().HasSynced,

		deploymentManager: deploymentManager,
		statusUpdater:     statusUpdater,
	}

	// Set up event handlers
	syncTargetInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.enqueue,
		UpdateFunc: func(old, new interface{}) {
			c.enqueue(new)
		},
		DeleteFunc: c.enqueue,
	})

	// Add indexes for efficient lookups
	if err := AddIndexes(syncTargetInformer.Informer()); err != nil {
		return nil, fmt.Errorf("failed to add indexes: %w", err)
	}

	return c, nil
}

// enqueue adds a SyncTarget to the work queue
func (c *Controller) enqueue(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}

	klog.V(4).Infof("Enqueuing SyncTarget %s", key)
	c.queue.Add(key)
}

// Start begins processing items from the work queue
func (c *Controller) Start(ctx context.Context) {
	defer runtime.HandleCrash()
	defer c.queue.ShutDown()

	klog.Info("Starting SyncTarget deployment controller")
	defer klog.Info("Shutting down SyncTarget deployment controller")

	if !cache.WaitForCacheSync(ctx.Done(), c.syncTargetSynced) {
		runtime.HandleError(fmt.Errorf("failed to sync caches"))
		return
	}

	klog.Info("Caches synced, starting workers")

	for i := 0; i < WorkerCount; i++ {
		go wait.Until(c.runWorker, time.Second, ctx.Done())
	}

	<-ctx.Done()
}

// runWorker processes items from the queue
func (c *Controller) runWorker() {
	for c.processNextItem() {
	}
}

// processNextItem handles a single item from the queue
func (c *Controller) processNextItem() bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)

	err := c.reconcile(key.(string))
	if err == nil {
		c.queue.Forget(key)
		return true
	}

	runtime.HandleError(fmt.Errorf("error reconciling %v: %v", key, err))
	c.queue.AddRateLimited(key)

	return true
}

// reconcile processes a single SyncTarget - delegating to abstractions
func (c *Controller) reconcile(key string) error {
	klog.V(4).Infof("Reconciling SyncTarget %s", key)
	
	// This will be implemented in PR3c with full reconciliation logic
	// For now, just return nil to allow compilation
	return nil
}
```

### 4. `pkg/reconciler/workload/synctarget/status.go` (64 lines)
New file for status management abstraction:

```go
/*
Copyright 2024 The KCP Authors.
[License header - 14 lines]
*/

package synctarget

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	workloadv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/workload/v1alpha1"
)

// statusUpdater implements the StatusUpdater interface
type statusUpdater struct {
	// Add fields as needed for status updates
}

// NewStatusUpdater creates a new StatusUpdater
func NewStatusUpdater() StatusUpdater {
	return &statusUpdater{}
}

// UpdateStatus updates the SyncTarget status based on deployment state
func (s *statusUpdater) UpdateStatus(ctx context.Context, target *workloadv1alpha1.SyncTarget, status *DeploymentStatus) error {
	klog.V(4).Infof("Updating status for SyncTarget %s", target.Name)

	// Update replicas status
	target.Status.Replicas = status.Replicas
	target.Status.ReadyReplicas = status.ReadyReplicas
	target.Status.AvailableReplicas = status.AvailableReplicas

	// Set condition based on deployment status
	setCondition(&target.Status.Conditions, status.Condition)

	return nil
}

// setCondition adds or updates a condition in the slice
func setCondition(conditions *[]metav1.Condition, newCondition metav1.Condition) {
	for i, condition := range *conditions {
		if condition.Type == newCondition.Type {
			(*conditions)[i] = newCondition
			return
		}
	}
	*conditions = append(*conditions, newCondition)
}
```

### 5. `pkg/reconciler/workload/synctarget/indexes_foundation.go` (97 lines)
Copy exactly from source:

```go
/*
Copyright 2024 The KCP Authors.
[License header - 14 lines]
*/

package synctarget

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

const (
	// IndexByCluster is the index name for cluster-based lookups
	IndexByCluster = "synctarget.cluster"

	// IndexByWorkspace is the index name for workspace-based lookups
	IndexByWorkspace = "synctarget.workspace"
)

// IndexByClusterFunc returns the cluster name for indexing
func IndexByClusterFunc(obj interface{}) ([]string, error) {
	runtimeObj, ok := obj.(runtime.Object)
	if !ok {
		return nil, nil
	}

	accessor, err := meta.Accessor(runtimeObj)
	if err != nil {
		return nil, err
	}

	// Look for cluster annotation or label
	cluster := accessor.GetAnnotations()["kcp.io/cluster"]
	if cluster == "" {
		cluster = accessor.GetLabels()["kcp.io/cluster"]
	}

	if cluster == "" {
		return nil, nil
	}

	return []string{cluster}, nil
}

// IndexByWorkspaceFunc returns the workspace for indexing
func IndexByWorkspaceFunc(obj interface{}) ([]string, error) {
	runtimeObj, ok := obj.(runtime.Object)
	if !ok {
		return nil, nil
	}

	accessor, err := meta.Accessor(runtimeObj)
	if err != nil {
		return nil, err
	}

	workspace := accessor.GetAnnotations()["kcp.io/workspace"]
	if workspace == "" {
		workspace = accessor.GetLabels()["kcp.io/workspace"]
	}

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

## Implementation Details

### Key Design Decisions

1. **Interface-First Approach**: Define clear contracts through interfaces that subsequent PRs will implement
2. **Minimal Controller Logic**: The controller in this PR only sets up the structure, actual reconciliation comes in PR3c
3. **Status Abstraction**: Separate status management into its own interface for cleaner separation
4. **Index Registration**: Include indexing setup for efficient lookups from the start

### Import Management

Ensure these imports are used correctly:
- `github.com/kcp-dev/logicalcluster/v3` for workspace awareness
- `github.com/kcp-dev/kcp/sdk/apis/workload/v1alpha1` for SyncTarget types
- `github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster` for KCP client
- `github.com/kcp-dev/kcp/sdk/client/informers/externalversions/cluster` for informers
- `github.com/kcp-dev/kcp/sdk/client/listers/workload/v1alpha1/cluster` for listers

### Compilation Requirements

This PR must compile independently. The abstractions allow for:
- Mock implementations for testing
- Stub implementations if needed
- The `reconcile` method returns nil for now (will be implemented in PR3c)

## Testing Requirements

While comprehensive tests come in PR3d, include basic compilation tests:

1. **Package Test**: Ensure the package compiles
2. **Interface Satisfaction**: Verify interfaces can be implemented
3. **Index Function Tests**: Test the indexing functions work correctly

## Integration Points

### For PR3b (Deployment Implementation):
- Will implement the `DeploymentManager` interface
- Must import this package's interfaces

### For PR3c (Integration):
- Will implement the full `reconcile` method
- Will wire together all components
- Will add helper functions for reconciliation

## Line Count Budget

| File | Estimated Lines | Purpose |
|------|-----------------|---------|
| `doc.go` | 19 | Package documentation |
| `interfaces.go` | 60 | Abstraction definitions |
| `controller.go` | 180 | Core controller structure |
| `status.go` | 64 | Status management |
| `indexes_foundation.go` | 97 | Index definitions |
| **Total** | **420** | ✅ On target |

## Verification Steps

1. **Compilation Check**:
   ```bash
   cd pkg/reconciler/workload/synctarget
   go build .
   ```

2. **Interface Verification**:
   ```bash
   # Ensure interfaces are properly defined
   go doc -all . | grep -E "type.*interface"
   ```

3. **Import Check**:
   ```bash
   go list -f '{{.Imports}}' .
   ```

4. **Line Count Verification**:
   ```bash
   find . -name "*.go" -not -name "*_test.go" | xargs wc -l
   ```

## Commit Message

```
feat(synctarget): add core controller foundation with abstractions

- Define DeploymentManager and StatusUpdater interfaces
- Implement basic controller structure with queue and informers
- Add index registration for efficient lookups
- Create status management abstraction
- Follow KCP patterns for workspace isolation

This establishes the foundation for syncer deployment management
with clean abstractions that allow for progressive enhancement.

Part of TMC Phase 2 Wave 2A implementation
```

## Notes for Implementation

1. **Keep It Minimal**: This PR focuses on structure and abstractions, not full functionality
2. **Document Interfaces**: Each interface method should have clear documentation
3. **Error Handling**: Include proper error handling even in the foundation
4. **Logging**: Use appropriate log levels (V(2) for info, V(4) for debug)
5. **KCP Patterns**: Ensure workspace awareness is built in from the start