# TMC Reimplementation Plan - Phase 3: Multi-Resource Synchronization

## Overview

Phase 3 expands on Phase 2's basic sync capabilities by adding support for multiple resource types and introducing bidirectional synchronization. This phase moves toward production-ready syncing while maintaining strict KCP pattern compliance.

## 🎯 Phase 3 Goals

**Enable comprehensive workload synchronization across multiple resource types**

- Add support for core Kubernetes resources (Deployments, Services, ConfigMaps, Secrets)
- Implement bidirectional sync (KCP ↔ Physical Cluster)
- Add resource transformation capabilities
- Introduce basic conflict resolution
- Maintain under 600 lines total across 3 PRs

## 📋 Addressing All Reviewer Feedback (Continued)

### ✅ Overly Complex Design (Addressed)
- **Approach**: Use standard Kubernetes error types, no custom TMC errors
- **Metrics**: Standard Kubernetes metrics patterns only
- **Health**: Use existing KCP health monitoring patterns

### ✅ Missing KCP Integration (Enhanced)
- **LogicalCluster**: Full integration with workspace-aware resource handling
- **Existing Patterns**: Build on KCP API export and syncer concepts
- **Controller Patterns**: Continue following exact KCP reconciliation loops

## 🏗️ Technical Implementation Plan

### PR 5: Multi-Resource Type Support (~200 lines)

**Objective**: Extend syncer to handle multiple Kubernetes resource types

#### Files Modified/Created:
```
pkg/reconciler/workload/synctarget/resource_sync.go      (~120 lines) - NEW
pkg/reconciler/workload/synctarget/resource_sync_test.go (~80 lines) - NEW
pkg/reconciler/workload/synctarget/syncer.go            (~20 lines modified)
```

#### Enhanced Resource Synchronization:
```go
// pkg/reconciler/workload/synctarget/resource_sync.go
package synctarget

import (
    "context"
    "fmt"

    appsv1 "k8s.io/api/apps/v1"
    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/runtime/schema"
    "k8s.io/client-go/kubernetes"
    "k8s.io/klog/v2"
    
    kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
    "github.com/kcp-dev/logicalcluster/v3"
)

// ResourceSynchronizer handles sync operations for different resource types
type ResourceSynchronizer struct {
    upstreamClient   kcpclientset.ClusterInterface
    downstreamClient kubernetes.Interface
    clusterName      logicalcluster.Name
}

// SupportedResources returns the resource types this syncer handles
func (r *ResourceSynchronizer) SupportedResources() []schema.GroupVersionResource {
    return []schema.GroupVersionResource{
        {Group: "apps", Version: "v1", Resource: "deployments"},
        {Group: "", Version: "v1", Resource: "services"},
        {Group: "", Version: "v1", Resource: "configmaps"},
        {Group: "", Version: "v1", Resource: "secrets"},
    }
}

// SyncDeployment synchronizes a Deployment resource
func (r *ResourceSynchronizer) SyncDeployment(ctx context.Context, namespace, name string) error {
    // Get deployment from upstream (KCP)
    upstream, err := r.upstreamClient.Cluster(r.clusterName.Path()).
        AppsV1().
        Deployments(namespace).
        Get(ctx, name, metav1.GetOptions{})
    if err != nil {
        return fmt.Errorf("failed to get upstream deployment: %w", err)
    }

    // Transform for downstream cluster
    downstream := r.transformDeployment(upstream.DeepCopy())

    // Apply to downstream cluster
    existing, err := r.downstreamClient.AppsV1().
        Deployments(namespace).
        Get(ctx, name, metav1.GetOptions{})
    
    if err != nil {
        // Create new deployment
        _, err = r.downstreamClient.AppsV1().
            Deployments(namespace).
            Create(ctx, downstream, metav1.CreateOptions{})
        return err
    }

    // Update existing deployment
    downstream.ResourceVersion = existing.ResourceVersion
    _, err = r.downstreamClient.AppsV1().
        Deployments(namespace).
        Update(ctx, downstream, metav1.UpdateOptions{})
    
    return err
}

// transformDeployment prepares a deployment for downstream cluster
func (r *ResourceSynchronizer) transformDeployment(deployment *appsv1.Deployment) *appsv1.Deployment {
    // Remove KCP-specific annotations and labels
    if deployment.Annotations != nil {
        delete(deployment.Annotations, "kcp.io/cluster")
        delete(deployment.Annotations, "kcp.io/workspace")
    }
    
    // Reset resource version for downstream
    deployment.ResourceVersion = ""
    deployment.UID = ""
    
    return deployment
}

// SyncService synchronizes a Service resource
func (r *ResourceSynchronizer) SyncService(ctx context.Context, namespace, name string) error {
    // Similar pattern to SyncDeployment but for Services
    // Following the same transformation and sync logic
    return nil
}

// SyncConfigMap synchronizes a ConfigMap resource
func (r *ResourceSynchronizer) SyncConfigMap(ctx context.Context, namespace, name string) error {
    // ConfigMap sync implementation
    return nil
}

// SyncSecret synchronizes a Secret resource
func (r *ResourceSynchronizer) SyncSecret(ctx context.Context, namespace, name string) error {
    // Secret sync implementation with special handling for sensitive data
    return nil
}
```

#### Integration with Existing Syncer:
```go
// Modification to pkg/reconciler/workload/synctarget/syncer.go
func (s *Syncer) runResourceSync(ctx context.Context) {
    resourceSync := &ResourceSynchronizer{
        upstreamClient:   s.upstreamClient,
        downstreamClient: s.downstreamClient,
        clusterName:      s.clusterName,
    }
    
    // Start sync loops for each supported resource type
    for _, gvr := range resourceSync.SupportedResources() {
        go s.runResourceSyncLoop(ctx, resourceSync, gvr)
    }
}

func (s *Syncer) runResourceSyncLoop(ctx context.Context, resourceSync *ResourceSynchronizer, gvr schema.GroupVersionResource) {
    // Resource-specific sync loop implementation
    // Use informers to watch for changes and trigger sync operations
}
```

### PR 6: Bidirectional Sync & Status Propagation (~200 lines)

**Objective**: Implement downstream-to-upstream status synchronization

#### Files Modified/Created:
```
pkg/reconciler/workload/synctarget/status_sync.go      (~120 lines) - NEW
pkg/reconciler/workload/synctarget/status_sync_test.go (~80 lines) - NEW
pkg/reconciler/workload/synctarget/resource_sync.go    (~20 lines modified)
```

#### Status Synchronization Implementation:
```go
// pkg/reconciler/workload/synctarget/status_sync.go
package synctarget

import (
    "context"
    "fmt"
    "reflect"

    appsv1 "k8s.io/api/apps/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/client-go/kubernetes"
    "k8s.io/klog/v2"
    
    kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
    "github.com/kcp-dev/logicalcluster/v3"
)

// StatusSynchronizer handles upstream status updates from downstream clusters
type StatusSynchronizer struct {
    upstreamClient   kcpclientset.ClusterInterface
    downstreamClient kubernetes.Interface
    clusterName      logicalcluster.Name
}

// SyncDeploymentStatus propagates deployment status from downstream to upstream
func (s *StatusSynchronizer) SyncDeploymentStatus(ctx context.Context, namespace, name string) error {
    // Get current status from downstream cluster
    downstream, err := s.downstreamClient.AppsV1().
        Deployments(namespace).
        Get(ctx, name, metav1.GetOptions{})
    if err != nil {
        return fmt.Errorf("failed to get downstream deployment status: %w", err)
    }

    // Get upstream deployment
    upstream, err := s.upstreamClient.Cluster(s.clusterName.Path()).
        AppsV1().
        Deployments(namespace).
        Get(ctx, name, metav1.GetOptions{})
    if err != nil {
        return fmt.Errorf("failed to get upstream deployment: %w", err)
    }

    // Check if status needs updating
    if reflect.DeepEqual(upstream.Status, downstream.Status) {
        return nil // No update needed
    }

    // Update upstream status
    upstream = upstream.DeepCopy()
    upstream.Status = downstream.Status
    
    _, err = s.upstreamClient.Cluster(s.clusterName.Path()).
        AppsV1().
        Deployments(namespace).
        UpdateStatus(ctx, upstream, metav1.UpdateOptions{})
    
    if err != nil {
        klog.ErrorS(err, "Failed to update upstream deployment status", 
            "deployment", name, "namespace", namespace)
    }

    return err
}

// SyncServiceStatus propagates service status from downstream to upstream  
func (s *StatusSynchronizer) SyncServiceStatus(ctx context.Context, namespace, name string) error {
    // Service status sync implementation
    // Handle LoadBalancer IP assignments, endpoint updates, etc.
    return nil
}

// StartStatusSync begins periodic status synchronization
func (s *StatusSynchronizer) StartStatusSync(ctx context.Context) {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            s.syncAllResourceStatuses(ctx)
        }
    }
}

func (s *StatusSynchronizer) syncAllResourceStatuses(ctx context.Context) {
    // Iterate through all resources and sync their statuses
    // Focus on resources that have meaningful status fields
}
```

### PR 7: Conflict Resolution & Resource Transformation (~200 lines)

**Objective**: Add basic conflict resolution and resource transformation capabilities

#### Files Modified/Created:
```
pkg/reconciler/workload/synctarget/conflict_resolver.go      (~120 lines) - NEW
pkg/reconciler/workload/synctarget/conflict_resolver_test.go (~80 lines) - NEW
pkg/reconciler/workload/synctarget/resource_sync.go         (~20 lines modified)
```

#### Conflict Resolution Implementation:
```go
// pkg/reconciler/workload/synctarget/conflict_resolver.go
package synctarget

import (
    "context"
    "fmt"
    "time"

    appsv1 "k8s.io/api/apps/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/klog/v2"
)

// ConflictResolver handles resource version conflicts during synchronization
type ConflictResolver struct {
    maxRetries int
    retryDelay time.Duration
}

// ConflictResolutionStrategy defines how to handle sync conflicts
type ConflictResolutionStrategy string

const (
    // UpstreamWins means KCP version takes precedence
    UpstreamWins ConflictResolutionStrategy = "upstream-wins"
    // LastWriteWins uses timestamp-based resolution
    LastWriteWins ConflictResolutionStrategy = "last-write-wins"
    // MergeChanges attempts to merge non-conflicting changes
    MergeChanges ConflictResolutionStrategy = "merge-changes"
)

// ResolveDeploymentConflict handles conflicts during deployment sync
func (r *ConflictResolver) ResolveDeploymentConflict(
    ctx context.Context,
    upstream *appsv1.Deployment,
    downstream *appsv1.Deployment,
    strategy ConflictResolutionStrategy,
) (*appsv1.Deployment, error) {
    
    switch strategy {
    case UpstreamWins:
        return r.resolveUpstreamWins(upstream, downstream), nil
    case LastWriteWins:
        return r.resolveLastWriteWins(upstream, downstream), nil
    case MergeChanges:
        return r.resolveMergeChanges(upstream, downstream)
    default:
        return upstream, nil // Default to upstream wins
    }
}

func (r *ConflictResolver) resolveUpstreamWins(upstream, downstream *appsv1.Deployment) *appsv1.Deployment {
    // Upstream (KCP) version takes precedence
    result := upstream.DeepCopy()
    result.ResourceVersion = downstream.ResourceVersion
    return result
}

func (r *ConflictResolver) resolveLastWriteWins(upstream, downstream *appsv1.Deployment) *appsv1.Deployment {
    // Compare timestamps and choose the most recent
    upstreamTime := upstream.ObjectMeta.CreationTimestamp
    downstreamTime := downstream.ObjectMeta.CreationTimestamp
    
    if upstreamTime.After(downstreamTime.Time) {
        result := upstream.DeepCopy()
        result.ResourceVersion = downstream.ResourceVersion
        return result
    }
    
    return downstream.DeepCopy()
}

func (r *ConflictResolver) resolveMergeChanges(upstream, downstream *appsv1.Deployment) (*appsv1.Deployment, error) {
    // Attempt to merge non-conflicting changes
    result := upstream.DeepCopy()
    result.ResourceVersion = downstream.ResourceVersion
    
    // Merge annotations (prefer upstream for conflicts)
    if result.Annotations == nil {
        result.Annotations = make(map[string]string)
    }
    
    for k, v := range downstream.Annotations {
        if _, exists := result.Annotations[k]; !exists {
            result.Annotations[k] = v
        }
    }
    
    // Similar merge logic for labels and other non-spec fields
    return result, nil
}

// RetryWithConflictResolution handles retry logic for conflict resolution
func (r *ConflictResolver) RetryWithConflictResolution(
    ctx context.Context,
    operation func() error,
) error {
    var lastErr error
    
    for i := 0; i < r.maxRetries; i++ {
        err := operation()
        if err == nil {
            return nil
        }
        
        lastErr = err
        
        // Check if this is a conflict error we can resolve
        if !isConflictError(err) {
            return err
        }
        
        klog.V(2).InfoS("Retrying after conflict", "attempt", i+1, "error", err)
        
        // Wait before retry
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-time.After(r.retryDelay):
            // Continue with retry
        }
    }
    
    return fmt.Errorf("exhausted retries for conflict resolution: %w", lastErr)
}

func isConflictError(err error) bool {
    // Check if error indicates a resource version conflict
    // Implementation depends on specific error types
    return false
}
```

## 📊 PR Strategy & Timeline

| PR | Scope | Lines | Files | Focus |
|----|-------|-------|-------|-------|
| 5 | Multi-Resource Support | ~200 | 3 | Deployments, Services, ConfigMaps, Secrets |
| 6 | Bidirectional Sync | ~200 | 3 | Status propagation from downstream |
| 7 | Conflict Resolution | ~200 | 3 | Basic conflict handling strategies |

**Total**: 3 PRs, 600 lines, 9 files, focused incremental enhancement

## ✅ Success Criteria

### Continues Meeting All Reviewer Requirements:
1. **✅ Zero governance file changes**
2. **✅ Files under 200 lines each**  
3. **✅ >80% test coverage using KCP patterns**
4. **✅ Builds incrementally on previous phases**
5. **✅ No separate infrastructure - pure KCP extension**
6. **✅ Standard Kubernetes error handling**

### New Phase 3 Validation:
- Multi-resource type synchronization works reliably
- Bidirectional sync maintains consistency between KCP and clusters
- Basic conflict resolution prevents sync failures
- Resource transformations work correctly for downstream clusters
- Status propagation reflects real cluster state in KCP

## 🔄 Integration with Previous Phases

Phase 3 enhances Phase 1-2 by:
- **Extending syncer** from Phase 2 with multi-resource capabilities
- **Building on SyncTarget** from Phase 1 with operational functionality
- **Maintaining compatibility** with existing controller patterns
- **Adding value** without breaking existing integrations

## 🎯 Future Extension Points

Phase 3 establishes foundation for:
- **Phase 4**: Placement logic and intelligent resource routing
- **Phase 5**: Advanced TMC features, virtual workspaces, complex transformations

## 🚀 Expected Outcome

Phase 3 delivers:
- **Production-ready syncing** for core Kubernetes resources
- **Bidirectional synchronization** maintaining state consistency
- **Conflict resolution** preventing sync failures
- **Resource transformation** supporting cluster-specific requirements
- **Comprehensive testing** proving reliability and correctness

This phase moves TMC from basic foundation to functionally complete core synchronization, maintaining the reviewer's preferred incremental approach while delivering significant operational value.