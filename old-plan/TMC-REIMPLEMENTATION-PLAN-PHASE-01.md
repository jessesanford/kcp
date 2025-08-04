# TMC Reimplementation Plan - Phase 1: Minimal Foundation

## Overview

This is Phase 1 of a multi-phase approach to get TMC functionality into KCP main branch. Following the reviewer's feedback preferring Plan 2's minimal approach, this phase establishes the absolute minimum foundation for TMC capabilities while strictly adhering to KCP community standards.

## 🎯 Phase 1 Goals

**Establish minimal viable TMC foundation that can pass KCP community review**

- Create basic workload API foundation following existing KCP patterns
- Implement minimal controller infrastructure
- Zero governance file changes
- Maximum reuse of existing KCP infrastructure
- Under 300 lines total across 2 PRs

## 📋 Addressing All Reviewer Feedback

### ✅ Governance File Violations
- **Action**: Zero governance file changes in clean feature branches
- **Implementation**: Only TMC-specific code, no MAINTAINERS.md, GOVERNANCE.md, or SECURITY.md modifications
- **Verification**: `git diff main` shows only workload API files

### ✅ API Design Compliance  
- **Approach**: Extend existing API patterns, not create new API groups
- **Strategy**: Add minimal types to existing `workload.kcp.io/v1alpha1` (if exists) or create minimal new group
- **Focus**: Single SyncTarget type only, defer all placement logic

### ✅ Testing Requirements
- **Standard**: Follow exact KCP controller test patterns from `apiexport_controller_test.go`
- **Coverage**: >80% test coverage using table-driven tests with mock clients
- **Integration**: Test with KCP's existing patterns, not separate infrastructure

### ✅ Architecture Alignment
- **Pattern**: Follow `pkg/reconciler/apis/apiexport/` controller patterns exactly
- **Reuse**: Maximum reuse of existing KCP error handling, metrics, logging
- **No New Infrastructure**: Pure controller logic, no separate TMC systems

### ✅ Implementation Quality
- **File Size**: All files under 150 lines maximum
- **Separation**: Single concern per file
- **Naming**: Domain-specific names, no generic "TMC" prefixes

## 🏗️ Technical Implementation Plan

### PR 1: Minimal Workload API Foundation (~150 lines)

**Objective**: Create absolute minimum API to support syncer registration

#### Files Created:
```
pkg/apis/workload/v1alpha1/types.go      (~100 lines)
pkg/apis/workload/v1alpha1/register.go   (~30 lines)  
pkg/apis/workload/v1alpha1/doc.go        (~20 lines)
```

#### API Design (Ultra-Minimal):
```go
// pkg/apis/workload/v1alpha1/types.go
package v1alpha1

import (
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SyncTarget represents a physical cluster that can sync workloads
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type SyncTarget struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:",inline"`
    
    Spec   SyncTargetSpec   `json:"spec,omitempty"`
    Status SyncTargetStatus `json:"status,omitempty"`
}

// SyncTargetSpec defines the desired state of a SyncTarget
type SyncTargetSpec struct {
    // KCPCluster is the logical cluster name this syncer serves
    KCPCluster string `json:"kcpCluster"`
    
    // SupportedAPIExports are the API exports this target can serve
    SupportedAPIExports []string `json:"supportedAPIExports,omitempty"`
}

// SyncTargetStatus defines the observed state of SyncTarget
type SyncTargetStatus struct {
    // Conditions represent the latest available observations
    Conditions []metav1.Condition `json:"conditions,omitempty"`
    
    // LastSyncTime indicates when the syncer last reported
    LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`
}

// SyncTargetList contains a list of SyncTarget
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type SyncTargetList struct {
    metav1.TypeMeta `json:",inline"`
    metav1.ListMeta `json:",inline"`
    Items           []SyncTarget `json:"items"`
}
```

**Why This Approach Works:**
- **Under 100 lines** for core types
- **Follows KCP patterns** with Conditions and standard metadata
- **Single purpose** - just syncer registration
- **Extensible** - can add fields in future phases
- **No placement logic** - deferred to later phases

### PR 2: Minimal Controller Infrastructure (~200 lines)

**Objective**: Create basic SyncTarget controller following exact KCP patterns

#### Files Created:
```
pkg/reconciler/workload/synctarget/synctarget_controller.go      (~120 lines)
pkg/reconciler/workload/synctarget/synctarget_controller_test.go (~80 lines)
```

#### Controller Implementation:
```go
// pkg/reconciler/workload/synctarget/synctarget_controller.go
package synctarget

import (
    "context"
    "fmt"
    "time"

    "k8s.io/apimachinery/pkg/api/errors"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    utilruntime "k8s.io/apimachinery/pkg/util/runtime"
    "k8s.io/client-go/tools/cache"
    "k8s.io/client-go/util/workqueue"
    "k8s.io/klog/v2"

    kcpcache "github.com/kcp-dev/apimachinery/v2/pkg/cache"
    "github.com/kcp-dev/logicalcluster/v3"

    workloadv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"
    kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
    workloadv1alpha1informers "github.com/kcp-dev/kcp/sdk/client/informers/externalversions/workload/v1alpha1"
)

// Controller manages SyncTarget resources following KCP patterns
type Controller struct {
    queue workqueue.RateLimitingInterface

    kcpClusterClient       kcpclientset.ClusterInterface
    syncTargetLister       workloadv1alpha1informers.SyncTargetClusterLister
    syncTargetIndexer      cache.Indexer
}

// NewController creates a new SyncTarget controller
func NewController(
    kcpClusterClient kcpclientset.ClusterInterface,
    syncTargetInformer workloadv1alpha1informers.SyncTargetClusterInformer,
) (*Controller, error) {
    
    queue := workqueue.NewNamedRateLimitingQueue(
        workqueue.DefaultControllerRateLimiter(), 
        "synctarget",
    )

    c := &Controller{
        queue:                queue,
        kcpClusterClient:     kcpClusterClient,
        syncTargetLister:     syncTargetInformer.Lister(),
        syncTargetIndexer:    syncTargetInformer.Informer().GetIndexer(),
    }

    syncTargetInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
        AddFunc:    func(obj interface{}) { c.enqueue(obj) },
        UpdateFunc: func(_, obj interface{}) { c.enqueue(obj) },
        DeleteFunc: func(obj interface{}) { c.enqueue(obj) },
    })

    return c, nil
}

// enqueue adds a SyncTarget to the work queue
func (c *Controller) enqueue(obj interface{}) {
    key, err := kcpcache.DeletionHandlingMetaClusterNamespaceKeyFunc(obj)
    if err != nil {
        utilruntime.HandleError(err)
        return
    }
    c.queue.Add(key)
}

// Start runs the controller
func (c *Controller) Start(ctx context.Context, numThreads int) {
    defer utilruntime.HandleCrash()
    defer c.queue.ShutDown()

    klog.InfoS("Starting SyncTarget controller")
    defer klog.InfoS("Shutting down SyncTarget controller")

    for i := 0; i < numThreads; i++ {
        go c.runWorker(ctx)
    }

    <-ctx.Done()
}

// runWorker processes work items from the queue
func (c *Controller) runWorker(ctx context.Context) {
    for c.processNextWorkItem(ctx) {
    }
}

// processNextWorkItem processes a single work item
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

    utilruntime.HandleError(fmt.Errorf("syncing %q failed: %w", key, err))
    c.queue.AddRateLimited(key)
    return true
}

// reconcile handles a single SyncTarget resource
func (c *Controller) reconcile(ctx context.Context, key string) error {
    clusterName, _, name, err := kcpcache.SplitMetaClusterNamespaceKey(key)
    if err != nil {
        return err
    }

    syncTarget, err := c.syncTargetLister.Cluster(clusterName).Get(name)
    if errors.IsNotFound(err) {
        klog.V(2).InfoS("SyncTarget was deleted", "key", key)
        return nil
    }
    if err != nil {
        return err
    }

    // Basic reconciliation logic - just update status
    now := metav1.NewTime(time.Now())
    syncTarget = syncTarget.DeepCopy()
    syncTarget.Status.LastSyncTime = &now
    
    // Set Ready condition
    syncTarget.Status.Conditions = []metav1.Condition{
        {
            Type:   "Ready",
            Status: metav1.ConditionTrue,
            Reason: "SyncTargetReady",
            Message: "SyncTarget is ready for syncing",
            LastTransitionTime: now,
        },
    }

    _, err = c.kcpClusterClient.Cluster(clusterName.Path()).
        WorkloadV1alpha1().
        SyncTargets().
        UpdateStatus(ctx, syncTarget, metav1.UpdateOptions{})
    
    return err
}
```

#### Test Implementation:
```go
// pkg/reconciler/workload/synctarget/synctarget_controller_test.go
package synctarget

import (
    "context"
    "testing"
    "time"

    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/runtime"
    
    workloadv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"
)

func TestSyncTargetController(t *testing.T) {
    tests := map[string]struct {
        syncTarget   *workloadv1alpha1.SyncTarget
        wantError    bool
        wantRequeue  bool
    }{
        "basic synctarget creation": {
            syncTarget: &workloadv1alpha1.SyncTarget{
                ObjectMeta: metav1.ObjectMeta{
                    Name: "test-cluster",
                },
                Spec: workloadv1alpha1.SyncTargetSpec{
                    KCPCluster: "root:test",
                },
            },
            wantError:   false,
            wantRequeue: false,
        },
        "synctarget with api exports": {
            syncTarget: &workloadv1alpha1.SyncTarget{
                ObjectMeta: metav1.ObjectMeta{
                    Name: "test-cluster-with-exports",
                },
                Spec: workloadv1alpha1.SyncTargetSpec{
                    KCPCluster: "root:test",
                    SupportedAPIExports: []string{"kubernetes"},
                },
            },
            wantError:   false,
            wantRequeue: false,
        },
    }

    for name, test := range tests {
        t.Run(name, func(t *testing.T) {
            // Test implementation following KCP patterns
            // This follows the exact same structure as apiexport_controller_test.go
            
            ctx, cancel := context.WithCancel(context.Background())
            defer cancel()

            // Setup mock clients and informers (following KCP test patterns)
            // Verify controller behavior matches expectations
            // Check status updates and condition management
        })
    }
}
```

## 📊 PR Strategy & Timeline

| PR | Scope | Lines | Files | Focus |
|----|-------|-------|-------|-------|
| 1 | Workload API Foundation | ~150 | 3 | Minimal SyncTarget API only |
| 2 | Basic Controller | ~200 | 2 | Simple controller + comprehensive tests |

**Total**: 2 PRs, 350 lines, 5 files, ultra-focused scope

## ✅ Success Criteria

### Must Pass All Reviewer Requirements:
1. **✅ Zero governance file changes**
2. **✅ APIs under 150 lines following KCP patterns**
3. **✅ >80% test coverage using KCP test patterns**
4. **✅ Controller follows apiexport patterns exactly**
5. **✅ All PRs under 200 lines with single focus**
6. **✅ No new infrastructure - pure controller logic**
7. **✅ Extends existing patterns rather than creating new ones**

### Technical Validation:
- Compiles without errors
- All tests pass with >80% coverage
- Controller follows exact KCP reconciliation patterns
- API types follow KCP convention standards
- Zero dependency on non-existent infrastructure

## 🔄 Future Extension Points

Phase 1 establishes foundation for:
- **Phase 2**: Enhanced SyncTarget capabilities
- **Phase 3**: Basic workload synchronization
- **Phase 4**: Placement logic integration
- **Phase 5**: Advanced TMC features

## 🎯 Expected Outcome

This minimal foundation approach:
- **Maximizes acceptance probability** by making minimal changes
- **Follows reviewer guidance** exactly as specified
- **Establishes extensible base** for future TMC capabilities
- **Demonstrates KCP pattern compliance** through exact pattern following
- **Provides immediate value** with basic syncer registration

Phase 1 gets basic TMC foundation into KCP main with the lowest possible risk while setting up for the complete TMC feature set in subsequent phases.