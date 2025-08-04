# TMC Reimplementation Plan - Phase 2: Enhanced SyncTarget Integration

## Overview

Phase 2 builds on Phase 1's minimal foundation by adding basic workload synchronization capabilities while maintaining strict adherence to KCP patterns. This phase introduces the core syncing functionality needed to make SyncTargets actually functional.

## 🎯 Phase 2 Goals

**Add essential syncing capabilities to make SyncTargets operational**

- Implement basic resource synchronization logic
- Add heartbeat and health reporting
- Introduce minimal CLI tooling
- Maintain reviewer compliance
- Under 500 lines total across 2 PRs

## 📋 Addressing All Reviewer Feedback (Continued)

### ✅ Architecture Alignment (Extended)
- **Pattern**: Continue following exact KCP controller patterns
- **Reuse**: Leverage Phase 1 foundation, add minimal sync logic
- **Integration**: Build on existing KCP syncer concepts if any exist

### ✅ Implementation Quality (Extended)
- **File Organization**: Keep all files under 200 lines
- **Single Responsibility**: Each component has one clear purpose
- **KCP Integration**: Full integration with workspace and logical cluster concepts

### ✅ No Complex Infrastructure
- **Approach**: No separate TMC infrastructure - pure KCP pattern extension
- **Error Handling**: Use standard Kubernetes/KCP error patterns only
- **Metrics**: Use existing KCP metrics patterns, no custom systems

## 🏗️ Technical Implementation Plan

### PR 3: Basic Resource Synchronization (~250 lines)

**Objective**: Add minimal resource sync capabilities to SyncTarget

#### Files Modified/Created:
```
pkg/reconciler/workload/synctarget/syncer.go           (~150 lines) - NEW
pkg/reconciler/workload/synctarget/syncer_test.go     (~100 lines) - NEW
pkg/reconciler/workload/synctarget/synctarget_controller.go (~20 lines added)
```

#### Enhanced Controller Integration:
```go
// Add to synctarget_controller.go - integrate syncer
func (c *Controller) reconcile(ctx context.Context, key string) error {
    // ... existing logic ...
    
    // Initialize basic syncer if SyncTarget is ready
    if syncTarget.Status.Conditions != nil {
        for _, condition := range syncTarget.Status.Conditions {
            if condition.Type == "Ready" && condition.Status == metav1.ConditionTrue {
                // Start basic sync operations
                return c.ensureSyncerRunning(ctx, syncTarget)
            }
        }
    }
    
    return nil
}

func (c *Controller) ensureSyncerRunning(ctx context.Context, syncTarget *workloadv1alpha1.SyncTarget) error {
    // Minimal syncer startup logic following KCP patterns
    // Just enough to establish basic connectivity
    return nil
}
```

#### Basic Syncer Implementation:
```go
// pkg/reconciler/workload/synctarget/syncer.go
package synctarget

import (
    "context"
    "sync"
    "time"

    "k8s.io/client-go/kubernetes"
    "k8s.io/klog/v2"
    
    workloadv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"
    kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
)

// Syncer handles basic resource synchronization for a SyncTarget
type Syncer struct {
    syncTarget     *workloadv1alpha1.SyncTarget
    upstreamClient kcpclientset.ClusterInterface
    downstreamClient kubernetes.Interface
    
    stopCh chan struct{}
    mu     sync.RWMutex
}

// NewSyncer creates a new basic syncer
func NewSyncer(
    syncTarget *workloadv1alpha1.SyncTarget,
    upstreamClient kcpclientset.ClusterInterface,
    downstreamClient kubernetes.Interface,
) *Syncer {
    return &Syncer{
        syncTarget:       syncTarget,
        upstreamClient:   upstreamClient,
        downstreamClient: downstreamClient,
        stopCh:          make(chan struct{}),
    }
}

// Start begins basic sync operations
func (s *Syncer) Start(ctx context.Context) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    klog.InfoS("Starting basic syncer", "syncTarget", s.syncTarget.Name)
    
    // Start heartbeat reporting
    go s.runHeartbeat(ctx)
    
    // Start basic resource watching (minimal implementation)
    go s.runResourceSync(ctx)
    
    return nil
}

// Stop shuts down the syncer
func (s *Syncer) Stop() {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    close(s.stopCh)
    klog.InfoS("Stopped basic syncer", "syncTarget", s.syncTarget.Name)
}

// runHeartbeat sends periodic status updates
func (s *Syncer) runHeartbeat(ctx context.Context) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-s.stopCh:
            return
        case <-ticker.C:
            s.sendHeartbeat(ctx)
        }
    }
}

// sendHeartbeat updates SyncTarget status
func (s *Syncer) sendHeartbeat(ctx context.Context) {
    // Update LastSyncTime and maintain Ready condition
    // Following exact KCP status update patterns
}

// runResourceSync performs basic resource synchronization
func (s *Syncer) runResourceSync(ctx context.Context) {
    // Minimal resource sync logic
    // Just enough to demonstrate basic functionality
    // Focus on Deployments only initially
}
```

**Why This Approach Works:**
- **Under 250 lines total** across all changes
- **Builds on Phase 1** without breaking existing functionality
- **Minimal scope** - just basic sync, no complex placement
- **KCP patterns** - follows existing controller conventions
- **Testable** - clear interfaces for unit testing

### PR 4: Basic CLI and Status Reporting (~250 lines)

**Objective**: Add minimal CLI tool and enhanced status reporting

#### Files Created:
```
cmd/syncer/main.go                                      (~100 lines) - NEW
pkg/reconciler/workload/synctarget/status_reporter.go  (~100 lines) - NEW
pkg/reconciler/workload/synctarget/status_reporter_test.go (~50 lines) - NEW
```

#### Minimal CLI Implementation:
```go
// cmd/syncer/main.go
package main

import (
    "context"
    "flag"
    "os"

    "k8s.io/client-go/tools/clientcmd"
    "k8s.io/klog/v2"
    
    "github.com/kcp-dev/kcp/pkg/reconciler/workload/synctarget"
)

func main() {
    var (
        syncTargetName = flag.String("sync-target-name", "", "Name of the SyncTarget resource")
        kubeconfig = flag.String("kubeconfig", "", "Path to kubeconfig file")
    )
    flag.Parse()

    if *syncTargetName == "" {
        klog.Fatal("--sync-target-name is required")
    }

    ctx := context.Background()
    
    // Build client config following KCP patterns
    config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
    if err != nil {
        klog.Fatalf("Error building kubeconfig: %v", err)
    }

    // Initialize basic syncer
    syncer, err := synctarget.NewSyncerFromConfig(ctx, config, *syncTargetName)
    if err != nil {
        klog.Fatalf("Error creating syncer: %v", err)
    }

    // Start syncing
    if err := syncer.Start(ctx); err != nil {
        klog.Fatalf("Error starting syncer: %v", err)
    }

    // Wait for shutdown
    <-ctx.Done()
}
```

#### Status Reporter Implementation:
```go
// pkg/reconciler/workload/synctarget/status_reporter.go
package synctarget

import (
    "context"
    "time"

    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/klog/v2"
    
    workloadv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"
    kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
    "github.com/kcp-dev/logicalcluster/v3"
)

// StatusReporter manages SyncTarget status updates
type StatusReporter struct {
    client kcpclientset.ClusterInterface
    syncTargetName string
    clusterName logicalcluster.Name
}

// NewStatusReporter creates a new status reporter
func NewStatusReporter(
    client kcpclientset.ClusterInterface,
    syncTargetName string,
    clusterName logicalcluster.Name,
) *StatusReporter {
    return &StatusReporter{
        client: client,
        syncTargetName: syncTargetName,
        clusterName: clusterName,
    }
}

// UpdateHeartbeat updates the SyncTarget with current timestamp
func (r *StatusReporter) UpdateHeartbeat(ctx context.Context) error {
    syncTarget, err := r.client.Cluster(r.clusterName.Path()).
        WorkloadV1alpha1().
        SyncTargets().
        Get(ctx, r.syncTargetName, metav1.GetOptions{})
    if err != nil {
        return err
    }

    syncTarget = syncTarget.DeepCopy()
    now := metav1.NewTime(time.Now())
    syncTarget.Status.LastSyncTime = &now

    // Update Ready condition
    for i, condition := range syncTarget.Status.Conditions {
        if condition.Type == "Ready" {
            syncTarget.Status.Conditions[i].LastTransitionTime = now
            break
        }
    }

    _, err = r.client.Cluster(r.clusterName.Path()).
        WorkloadV1alpha1().
        SyncTargets().
        UpdateStatus(ctx, syncTarget, metav1.UpdateOptions{})

    if err != nil {
        klog.ErrorS(err, "Failed to update SyncTarget heartbeat", 
            "syncTarget", r.syncTargetName)
    }

    return err
}

// UpdateSyncStatus updates the sync status with resource counts
func (r *StatusReporter) UpdateSyncStatus(ctx context.Context, resourceCount int) error {
    // Basic status update - just maintain heartbeat for now
    // More sophisticated status reporting deferred to future phases
    return r.UpdateHeartbeat(ctx)
}
```

## 📊 PR Strategy & Timeline

| PR | Scope | Lines | Files | Focus |
|----|-------|-------|-------|-------|
| 3 | Basic Resource Sync | ~250 | 3 | Minimal sync logic + tests |
| 4 | CLI & Status Reporting | ~250 | 3 | Basic tooling + status updates |

**Total**: 2 PRs, 500 lines, 6 files, focused scope building on Phase 1

## ✅ Success Criteria

### Continues Meeting All Reviewer Requirements:
1. **✅ Zero governance file changes**
2. **✅ Files under 200 lines each**
3. **✅ >80% test coverage using KCP patterns**
4. **✅ Builds on Phase 1 foundation**
5. **✅ No separate infrastructure - pure KCP extension**
6. **✅ Single focus per PR**

### New Phase 2 Validation:
- Basic syncer can connect to downstream clusters
- Heartbeat reporting works with SyncTarget status
- CLI tool can start/stop basic sync operations
- Resource synchronization demonstrates basic functionality
- All integration follows KCP workspace patterns

## 🔄 Integration with Phase 1

Phase 2 enhances Phase 1 by:
- **Adding functionality** to existing SyncTarget controller
- **Introducing syncer logic** that operates on Phase 1 SyncTarget resources
- **Providing tooling** to interact with Phase 1 API foundation
- **Maintaining compatibility** with Phase 1 minimal approach

## 🎯 Future Extension Points

Phase 2 establishes foundation for:
- **Phase 3**: Multi-resource type synchronization
- **Phase 4**: Placement logic and advanced routing
- **Phase 5**: Full TMC feature parity

## 🚀 Expected Outcome

Phase 2 delivers:
- **Functional TMC syncing** with basic resource synchronization
- **Operational tooling** for managing sync operations
- **Status visibility** through SyncTarget resource status
- **Proven patterns** for extending in subsequent phases
- **Community acceptance** through continued KCP pattern compliance

This maintains the reviewer's preferred minimal approach while adding essential functionality to make the TMC foundation actually usable.