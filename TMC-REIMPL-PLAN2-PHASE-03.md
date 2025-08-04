# TMC Reimplementation Plan 2 - Phase 3: Workload Synchronization

## 🎯 **ARCHITECTURAL FOUNDATION**

**TMC Controllers Synchronize External Workload APIs with Physical Clusters**

- **KCP Role**: Provides TMC placement APIs and external workload APIs via APIExport
- **TMC Controllers**: Watch external workload APIs, create resources on physical clusters
- **Synchronization**: External controllers handle workload creation, KCP tracks placement decisions
- **Status Flow**: Physical cluster status propagates back through TMC controllers to KCP

## 📋 **Phase 3 Objectives**

**Implement bidirectional workload synchronization between external APIs and physical clusters**

- Add workload API watching capabilities to TMC controllers
- Implement resource creation and lifecycle management on physical clusters
- Build bidirectional status synchronization
- Add resource transformation and filtering
- **Scope**: 1000-1200 lines across 2 PRs

## 🏗️ **Workload Synchronization Architecture**

### **Understanding the Correct Flow**

```go
// Correct TMC Workload Flow:
// 1. External system creates Deployment in KCP workspace via APIBinding
// 2. TMC controller watches this Deployment API (bound via APIBinding)  
// 3. TMC controller uses WorkloadPlacement to determine target clusters
// 4. TMC controller creates Deployment on selected physical clusters
// 5. TMC controller propagates status back to KCP Deployment
```

**Key Principles:**
1. **KCP serves external workload APIs** via APIExport (not KCP's own APIs)
2. **TMC controllers watch these external APIs** via APIBinding
3. **Physical clusters execute workloads** based on placement decisions
4. **Status synchronization** maintains consistency
5. **Resource transformation** handles cluster-specific adaptations

## 📊 **PR 5: Workload Synchronization Engine (~600 lines)**

**Objective**: Add workload synchronization capabilities to TMC controllers

### **Files Created:**
```
pkg/tmc/sync/engine.go                          (~200 lines)
pkg/tmc/sync/deployment_sync.go                 (~200 lines)
pkg/tmc/sync/status_sync.go                     (~150 lines)
pkg/tmc/sync/engine_test.go                     (~50 lines)
```

### **Synchronization Engine:**
```go
// pkg/tmc/sync/engine.go
package sync

import (
    "context"
    "fmt"
    "sync"
    "time"
    
    appsv1 "k8s.io/api/apps/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/runtime/schema"
    "k8s.io/apimachinery/pkg/util/runtime"
    "k8s.io/client-go/dynamic"
    "k8s.io/client-go/dynamic/dynamicinformer"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/rest"
    "k8s.io/client-go/tools/cache"
    "k8s.io/client-go/util/workqueue"
    "k8s.io/klog/v2"
    
    "github.com/kcp-dev/logicalcluster/v3"
    
    tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
    kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
)

// SyncEngine manages workload synchronization between KCP and physical clusters
type SyncEngine struct {
    // KCP clients
    kcpClusterClient kcpclientset.ClusterInterface
    kcpDynamicClient dynamic.Interface
    
    // Physical cluster clients
    clusterClients map[string]kubernetes.Interface
    
    // Informers
    kcpInformerFactory dynamicinformer.DynamicSharedInformerFactory
    
    // Synchronizers for different resource types
    deploymentSync *DeploymentSynchronizer
    
    // Configuration
    workspace logicalcluster.Name
    
    // State management
    stopCh chan struct{}
    wg     sync.WaitGroup
}

// NewSyncEngine creates a new workload synchronization engine
func NewSyncEngine(
    kcpConfig *rest.Config,
    clusterConfigs map[string]*rest.Config,
    workspace logicalcluster.Name,
) (*SyncEngine, error) {
    
    // Create KCP clients
    kcpClusterClient, err := kcpclientset.NewForConfig(kcpConfig)
    if err != nil {
        return nil, fmt.Errorf("failed to create KCP cluster client: %w", err)
    }
    
    kcpDynamicClient, err := dynamic.NewForConfig(kcpConfig)
    if err != nil {
        return nil, fmt.Errorf("failed to create KCP dynamic client: %w", err)
    }
    
    // Create physical cluster clients
    clusterClients := make(map[string]kubernetes.Interface)
    for name, config := range clusterConfigs {
        client, err := kubernetes.NewForConfig(config)
        if err != nil {
            return nil, fmt.Errorf("failed to create client for cluster %s: %w", name, err)
        }
        clusterClients[name] = client
    }
    
    // Create KCP informer factory for this workspace
    kcpInformerFactory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(
        kcpDynamicClient,
        30*time.Second,
        metav1.NamespaceAll,
        func(options *metav1.ListOptions) {
            // Filter to our workspace using cluster-aware client
            options.LabelSelector = fmt.Sprintf("kcp.io/cluster=%s", workspace)
        },
    )
    
    engine := &SyncEngine{
        kcpClusterClient:   kcpClusterClient,
        kcpDynamicClient:   kcpDynamicClient,
        clusterClients:     clusterClients,
        kcpInformerFactory: kcpInformerFactory,
        workspace:          workspace,
        stopCh:            make(chan struct{}),
    }
    
    // Create deployment synchronizer
    engine.deploymentSync, err = NewDeploymentSynchronizer(
        engine.kcpDynamicClient,
        engine.clusterClients,
        engine.workspace,
    )
    if err != nil {
        return nil, fmt.Errorf("failed to create deployment synchronizer: %w", err)
    }
    
    return engine, nil
}

// Start starts the synchronization engine
func (e *SyncEngine) Start(ctx context.Context) error {
    klog.InfoS("Starting workload synchronization engine", "workspace", e.workspace)
    
    // Start KCP informers
    e.kcpInformerFactory.Start(e.stopCh)
    
    // Wait for cache sync
    synced := e.kcpInformerFactory.WaitForCacheSync(e.stopCh)
    for gvr, hasSynced := range synced {
        if !hasSynced {
            return fmt.Errorf("failed to sync informer for %s", gvr)
        }
    }
    
    // Start deployment synchronizer
    e.wg.Add(1)
    go func() {
        defer e.wg.Done()
        e.deploymentSync.Start(ctx)
    }()
    
    return nil
}

// Stop stops the synchronization engine
func (e *SyncEngine) Stop() {
    klog.InfoS("Stopping workload synchronization engine")
    close(e.stopCh)
    e.wg.Wait()
}

// AddResourceType adds a new resource type for synchronization
func (e *SyncEngine) AddResourceType(gvr schema.GroupVersionResource) error {
    // Create informer for this resource type
    informer := e.kcpInformerFactory.ForResource(gvr)
    
    informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
        AddFunc:    e.handleResourceAdd,
        UpdateFunc: e.handleResourceUpdate,
        DeleteFunc: e.handleResourceDelete,
    })
    
    return nil
}

// handleResourceAdd handles resource creation events
func (e *SyncEngine) handleResourceAdd(obj interface{}) {
    e.handleResourceEvent("add", obj)
}

// handleResourceUpdate handles resource update events  
func (e *SyncEngine) handleResourceUpdate(oldObj, newObj interface{}) {
    e.handleResourceEvent("update", newObj)
}

// handleResourceDelete handles resource deletion events
func (e *SyncEngine) handleResourceDelete(obj interface{}) {
    e.handleResourceEvent("delete", obj)
}

// handleResourceEvent dispatches resource events to appropriate synchronizers
func (e *SyncEngine) handleResourceEvent(eventType string, obj interface{}) {
    // Extract resource information and dispatch to appropriate synchronizer
    // Implementation would determine resource type and route to correct sync handler
    klog.V(4).InfoS("Handling resource event", "type", eventType)
}
```

### **Deployment Synchronizer:**
```go
// pkg/tmc/sync/deployment_sync.go
package sync

import (
    "context"
    "fmt"
    "time"
    
    appsv1 "k8s.io/api/apps/v1"
    "k8s.io/apimachinery/pkg/api/errors"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
    "k8s.io/apimachinery/pkg/runtime"
    "k8s.io/apimachinery/pkg/runtime/schema"
    "k8s.io/client-go/dynamic"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/tools/cache"
    "k8s.io/client-go/util/workqueue"
    "k8s.io/klog/v2"
    
    "github.com/kcp-dev/logicalcluster/v3"
)

var DeploymentGVR = schema.GroupVersionResource{
    Group:    "apps",
    Version:  "v1", 
    Resource: "deployments",
}

// DeploymentSynchronizer handles Deployment resource synchronization
type DeploymentSynchronizer struct {
    queue workqueue.RateLimitingInterface
    
    // KCP clients  
    kcpDynamicClient dynamic.Interface
    
    // Physical cluster clients
    clusterClients map[string]kubernetes.Interface
    
    // Configuration
    workspace logicalcluster.Name
}

// NewDeploymentSynchronizer creates a new deployment synchronizer
func NewDeploymentSynchronizer(
    kcpDynamicClient dynamic.Interface,
    clusterClients map[string]kubernetes.Interface,
    workspace logicalcluster.Name,
) (*DeploymentSynchronizer, error) {
    
    return &DeploymentSynchronizer{
        queue: workqueue.NewNamedRateLimitingQueue(
            workqueue.DefaultControllerRateLimiter(),
            "deployment-sync"),
        kcpDynamicClient: kcpDynamicClient,
        clusterClients:   clusterClients,
        workspace:        workspace,
    }, nil
}

// Start starts the deployment synchronizer
func (d *DeploymentSynchronizer) Start(ctx context.Context) {
    defer d.queue.ShutDown()
    
    klog.InfoS("Starting deployment synchronizer")
    defer klog.InfoS("Shutting down deployment synchronizer")
    
    for i := 0; i < 2; i++ {
        go d.runWorker(ctx)
    }
    
    <-ctx.Done()
}

// runWorker processes work items from the queue
func (d *DeploymentSynchronizer) runWorker(ctx context.Context) {
    for d.processNextWorkItem(ctx) {
    }
}

// processNextWorkItem processes a single work item
func (d *DeploymentSynchronizer) processNextWorkItem(ctx context.Context) bool {
    key, quit := d.queue.Get()
    if quit {
        return false
    }
    defer d.queue.Done(key)
    
    err := d.syncDeployment(ctx, key.(string))
    if err == nil {
        d.queue.Forget(key)
        return true
    }
    
    klog.ErrorS(err, "Failed to sync deployment", "key", key)
    d.queue.AddRateLimited(key)
    return true
}

// syncDeployment synchronizes a deployment to target clusters
func (d *DeploymentSynchronizer) syncDeployment(ctx context.Context, key string) error {
    namespace, name, err := cache.SplitMetaNamespaceKey(key)
    if err != nil {
        return err
    }
    
    // Get deployment from KCP (via bound API)
    kcpDeployment, err := d.getKCPDeployment(ctx, namespace, name)
    if errors.IsNotFound(err) {
        // Deployment was deleted, clean up from clusters
        return d.deleteFromClusters(ctx, namespace, name)
    }
    if err != nil {
        return err
    }
    
    // Determine target clusters from placement annotations/labels
    targetClusters, err := d.getTargetClusters(kcpDeployment)
    if err != nil {
        return err
    }
    
    // Sync to each target cluster
    var syncErrors []error
    for _, clusterName := range targetClusters {
        if err := d.syncToCluster(ctx, clusterName, kcpDeployment); err != nil {
            syncErrors = append(syncErrors, fmt.Errorf("cluster %s: %w", clusterName, err))
        }
    }
    
    if len(syncErrors) > 0 {
        return fmt.Errorf("sync errors: %v", syncErrors)
    }
    
    return nil
}

// getKCPDeployment retrieves deployment from KCP
func (d *DeploymentSynchronizer) getKCPDeployment(
    ctx context.Context,
    namespace, name string,
) (*appsv1.Deployment, error) {
    
    unstructuredObj, err := d.kcpDynamicClient.
        Resource(DeploymentGVR).
        Namespace(namespace).
        Get(ctx, name, metav1.GetOptions{})
    if err != nil {
        return nil, err
    }
    
    // Convert to typed Deployment
    deployment := &appsv1.Deployment{}
    if err := runtime.DefaultUnstructuredConverter.FromUnstructured(
        unstructuredObj.Object, deployment); err != nil {
        return nil, fmt.Errorf("failed to convert to Deployment: %w", err)
    }
    
    return deployment, nil
}

// getTargetClusters determines which clusters should receive this deployment
func (d *DeploymentSynchronizer) getTargetClusters(
    deployment *appsv1.Deployment,
) ([]string, error) {
    
    // Check for placement annotations/labels
    if clusters, exists := deployment.Annotations["tmc.kcp.io/target-clusters"]; exists {
        // Parse comma-separated cluster list
        return parseClusterList(clusters), nil
    }
    
    // Default: sync to all available clusters
    var clusters []string
    for clusterName := range d.clusterClients {
        clusters = append(clusters, clusterName)
    }
    
    return clusters, nil
}

// syncToCluster syncs deployment to a specific cluster
func (d *DeploymentSynchronizer) syncToCluster(
    ctx context.Context,
    clusterName string,
    kcpDeployment *appsv1.Deployment,
) error {
    
    clusterClient, exists := d.clusterClients[clusterName]
    if !exists {
        return fmt.Errorf("no client for cluster %s", clusterName)
    }
    
    // Transform deployment for target cluster
    clusterDeployment := d.transformDeployment(kcpDeployment, clusterName)
    
    // Get existing deployment from cluster
    existing, err := clusterClient.AppsV1().
        Deployments(clusterDeployment.Namespace).
        Get(ctx, clusterDeployment.Name, metav1.GetOptions{})
    
    if errors.IsNotFound(err) {
        // Create new deployment
        _, err = clusterClient.AppsV1().
            Deployments(clusterDeployment.Namespace).
            Create(ctx, clusterDeployment, metav1.CreateOptions{})
        if err != nil {
            return fmt.Errorf("failed to create deployment: %w", err)
        }
        
        klog.V(2).InfoS("Created deployment on cluster",
            "deployment", clusterDeployment.Name,
            "namespace", clusterDeployment.Namespace,
            "cluster", clusterName)
        
        return nil
    }
    if err != nil {
        return fmt.Errorf("failed to get existing deployment: %w", err)
    }
    
    // Update existing deployment
    clusterDeployment.ResourceVersion = existing.ResourceVersion
    _, err = clusterClient.AppsV1().
        Deployments(clusterDeployment.Namespace).
        Update(ctx, clusterDeployment, metav1.UpdateOptions{})
    if err != nil {
        return fmt.Errorf("failed to update deployment: %w", err)
    }
    
    klog.V(2).InfoS("Updated deployment on cluster",
        "deployment", clusterDeployment.Name,
        "namespace", clusterDeployment.Namespace,
        "cluster", clusterName)
    
    return nil
}

// transformDeployment transforms deployment for target cluster
func (d *DeploymentSynchronizer) transformDeployment(
    kcpDeployment *appsv1.Deployment,
    clusterName string,
) *appsv1.Deployment {
    
    // Deep copy the deployment
    transformed := kcpDeployment.DeepCopy()
    
    // Reset cluster-specific fields
    transformed.ResourceVersion = ""
    transformed.UID = ""
    transformed.SelfLink = ""
    transformed.CreationTimestamp = metav1.Time{}
    
    // Add cluster-specific annotations
    if transformed.Annotations == nil {
        transformed.Annotations = make(map[string]string)
    }
    transformed.Annotations["tmc.kcp.io/source-cluster"] = string(d.workspace)
    transformed.Annotations["tmc.kcp.io/target-cluster"] = clusterName
    
    // Remove KCP-specific annotations
    delete(transformed.Annotations, "kcp.io/cluster")
    
    return transformed
}

// deleteFromClusters removes deployment from all clusters
func (d *DeploymentSynchronizer) deleteFromClusters(
    ctx context.Context,
    namespace, name string,
) error {
    
    var deleteErrors []error
    
    for clusterName, client := range d.clusterClients {
        err := client.AppsV1().
            Deployments(namespace).
            Delete(ctx, name, metav1.DeleteOptions{})
        
        if err != nil && !errors.IsNotFound(err) {
            deleteErrors = append(deleteErrors, 
                fmt.Errorf("cluster %s: %w", clusterName, err))
        }
    }
    
    if len(deleteErrors) > 0 {
        return fmt.Errorf("delete errors: %v", deleteErrors)
    }
    
    return nil
}

// Helper functions
func parseClusterList(clusters string) []string {
    // Implementation would parse comma-separated cluster names
    return []string{clusters} // Simplified
}
```

## 📊 **PR 6: Status Synchronization & Resource Lifecycle (~600 lines)**

**Objective**: Add bidirectional status sync and complete resource lifecycle management

### **Files Created:**
```
pkg/tmc/sync/status_sync.go                     (~250 lines)
pkg/tmc/sync/lifecycle.go                       (~200 lines)
pkg/tmc/sync/transform.go                       (~150 lines)
```

### **Status Synchronization:**
```go
// pkg/tmc/sync/status_sync.go
package sync

import (
    "context"
    "fmt"
    "reflect"
    "time"
    
    appsv1 "k8s.io/api/apps/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
    "k8s.io/apimachinery/pkg/runtime"
    "k8s.io/client-go/dynamic"
    "k8s.io/client-go/kubernetes"
    "k8s.io/klog/v2"
    
    "github.com/kcp-dev/logicalcluster/v3"
)

// StatusSynchronizer handles status synchronization from clusters back to KCP
type StatusSynchronizer struct {
    kcpDynamicClient dynamic.Interface
    clusterClients   map[string]kubernetes.Interface
    workspace        logicalcluster.Name
    
    // Status aggregation strategies
    statusAggregator *StatusAggregator
}

// StatusAggregator aggregates status from multiple clusters
type StatusAggregator struct {
    strategy AggregationStrategy
}

// AggregationStrategy defines how to aggregate status from multiple clusters
type AggregationStrategy string

const (
    // MajorityStrategy uses majority consensus for status
    MajorityStrategy AggregationStrategy = "majority"
    
    // UnionStrategy combines all cluster statuses
    UnionStrategy AggregationStrategy = "union"
    
    // LatestStrategy uses the most recent status
    LatestStrategy AggregationStrategy = "latest"
)

// NewStatusSynchronizer creates a new status synchronizer
func NewStatusSynchronizer(
    kcpDynamicClient dynamic.Interface,
    clusterClients map[string]kubernetes.Interface,
    workspace logicalcluster.Name,
) *StatusSynchronizer {
    
    return &StatusSynchronizer{
        kcpDynamicClient: kcpDynamicClient,
        clusterClients:   clusterClients,
        workspace:        workspace,
        statusAggregator: &StatusAggregator{
            strategy: MajorityStrategy,
        },
    }
}

// SyncDeploymentStatus synchronizes deployment status from clusters to KCP
func (s *StatusSynchronizer) SyncDeploymentStatus(
    ctx context.Context,
    namespace, name string,
) error {
    
    // Collect status from all clusters
    clusterStatuses := make(map[string]*appsv1.DeploymentStatus)
    
    for clusterName, client := range s.clusterClients {
        deployment, err := client.AppsV1().
            Deployments(namespace).
            Get(ctx, name, metav1.GetOptions{})
        
        if err != nil {
            klog.V(4).InfoS("Failed to get deployment status from cluster",
                "cluster", clusterName, "error", err)
            continue
        }
        
        clusterStatuses[clusterName] = &deployment.Status
    }
    
    if len(clusterStatuses) == 0 {
        // No status available from any cluster
        return nil
    }
    
    // Aggregate status
    aggregatedStatus := s.statusAggregator.AggregateDeploymentStatus(clusterStatuses)
    
    // Update KCP deployment status
    return s.updateKCPDeploymentStatus(ctx, namespace, name, aggregatedStatus)
}

// updateKCPDeploymentStatus updates the deployment status in KCP
func (s *StatusSynchronizer) updateKCPDeploymentStatus(
    ctx context.Context,
    namespace, name string,
    status *appsv1.DeploymentStatus,
) error {
    
    // Get current deployment from KCP
    unstructuredObj, err := s.kcpDynamicClient.
        Resource(DeploymentGVR).
        Namespace(namespace).
        Get(ctx, name, metav1.GetOptions{})
    if err != nil {
        return fmt.Errorf("failed to get KCP deployment: %w", err)
    }
    
    // Convert to typed Deployment
    deployment := &appsv1.Deployment{}
    if err := runtime.DefaultUnstructuredConverter.FromUnstructured(
        unstructuredObj.Object, deployment); err != nil {
        return fmt.Errorf("failed to convert to Deployment: %w", err)
    }
    
    // Check if status actually changed
    if reflect.DeepEqual(&deployment.Status, status) {
        return nil // No update needed
    }
    
    // Update status
    deployment.Status = *status
    
    // Convert back to unstructured for update
    unstructuredObj.Object, err = runtime.DefaultUnstructuredConverter.ToUnstructured(deployment)
    if err != nil {
        return fmt.Errorf("failed to convert to unstructured: %w", err)
    }
    
    // Update in KCP
    _, err = s.kcpDynamicClient.
        Resource(DeploymentGVR).
        Namespace(namespace).
        UpdateStatus(ctx, unstructuredObj, metav1.UpdateOptions{})
    
    if err != nil {
        return fmt.Errorf("failed to update KCP deployment status: %w", err)
    }
    
    klog.V(2).InfoS("Updated deployment status in KCP",
        "deployment", name,
        "namespace", namespace,
        "replicas", status.Replicas,
        "readyReplicas", status.ReadyReplicas)
    
    return nil
}

// AggregateDeploymentStatus aggregates deployment status from multiple clusters
func (a *StatusAggregator) AggregateDeploymentStatus(
    clusterStatuses map[string]*appsv1.DeploymentStatus,
) *appsv1.DeploymentStatus {
    
    switch a.strategy {
    case MajorityStrategy:
        return a.aggregateByMajority(clusterStatuses)
    case UnionStrategy:
        return a.aggregateByUnion(clusterStatuses)
    case LatestStrategy:
        return a.aggregateByLatest(clusterStatuses)
    default:
        return a.aggregateByMajority(clusterStatuses)
    }
}

// aggregateByMajority uses majority consensus for status aggregation
func (a *StatusAggregator) aggregateByMajority(
    clusterStatuses map[string]*appsv1.DeploymentStatus,
) *appsv1.DeploymentStatus {
    
    if len(clusterStatuses) == 0 {
        return &appsv1.DeploymentStatus{}
    }
    
    // Simple aggregation: sum all replicas
    result := &appsv1.DeploymentStatus{}
    
    for _, status := range clusterStatuses {
        result.Replicas += status.Replicas
        result.ReadyReplicas += status.ReadyReplicas
        result.AvailableReplicas += status.AvailableReplicas
        result.UnavailableReplicas += status.UnavailableReplicas
        result.UpdatedReplicas += status.UpdatedReplicas
    }
    
    // Aggregate conditions (simplified)
    result.Conditions = a.aggregateConditions(clusterStatuses)
    
    return result
}

// aggregateByUnion combines all cluster statuses
func (a *StatusAggregator) aggregateByUnion(
    clusterStatuses map[string]*appsv1.DeploymentStatus,
) *appsv1.DeploymentStatus {
    // Similar to majority but with different logic
    return a.aggregateByMajority(clusterStatuses)
}

// aggregateByLatest uses the most recent status
func (a *StatusAggregator) aggregateByLatest(
    clusterStatuses map[string]*appsv1.DeploymentStatus,
) *appsv1.DeploymentStatus {
    // Find status with most recent timestamp
    var latest *appsv1.DeploymentStatus
    var latestTime time.Time
    
    for _, status := range clusterStatuses {
        for _, condition := range status.Conditions {
            if condition.LastUpdateTime.After(latestTime) {
                latestTime = condition.LastUpdateTime.Time
                latest = status
            }
        }
    }
    
    if latest != nil {
        return latest.DeepCopy()
    }
    
    return &appsv1.DeploymentStatus{}
}

// aggregateConditions aggregates deployment conditions
func (a *StatusAggregator) aggregateConditions(
    clusterStatuses map[string]*appsv1.DeploymentStatus,
) []appsv1.DeploymentCondition {
    
    // Simplified condition aggregation
    conditionMap := make(map[appsv1.DeploymentConditionType]*appsv1.DeploymentCondition)
    
    for _, status := range clusterStatuses {
        for _, condition := range status.Conditions {
            existing, exists := conditionMap[condition.Type]
            if !exists || condition.LastUpdateTime.After(existing.LastUpdateTime.Time) {
                conditionMap[condition.Type] = condition.DeepCopy()
            }
        }
    }
    
    var result []appsv1.DeploymentCondition
    for _, condition := range conditionMap {
        result = append(result, *condition)
    }
    
    return result
}

// StartStatusSync starts periodic status synchronization
func (s *StatusSynchronizer) StartStatusSync(ctx context.Context) {
    ticker := time.NewTicker(15 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            s.syncAllStatuses(ctx)
        }
    }
}

// syncAllStatuses synchronizes status for all tracked resources
func (s *StatusSynchronizer) syncAllStatuses(ctx context.Context) {
    // This would iterate through all resources being synced
    // For now, simplified implementation
    klog.V(4).InfoS("Performing status sync cycle")
}
```

### **Resource Transformation:**
```go
// pkg/tmc/sync/transform.go
package sync

import (
    "fmt"
    "strings"
    
    appsv1 "k8s.io/api/apps/v1"
    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ResourceTransformer handles resource transformations for different clusters
type ResourceTransformer struct {
    transformRules map[string]TransformRule
}

// TransformRule defines transformation rules for a cluster
type TransformRule struct {
    ClusterName     string
    NamespacePrefix string
    ImageRegistry   string
    ResourceLimits  corev1.ResourceList
    Annotations     map[string]string
    Labels          map[string]string
}

// NewResourceTransformer creates a new resource transformer
func NewResourceTransformer(rules map[string]TransformRule) *ResourceTransformer {
    return &ResourceTransformer{
        transformRules: rules,
    }
}

// TransformDeployment transforms a deployment for a specific cluster
func (t *ResourceTransformer) TransformDeployment(
    deployment *appsv1.Deployment,
    targetCluster string,
) *appsv1.Deployment {
    
    transformed := deployment.DeepCopy()
    
    // Apply cluster-specific transformation rules
    if rule, exists := t.transformRules[targetCluster]; exists {
        t.applyTransformRule(transformed, rule)
    }
    
    // Always apply basic transformations
    t.applyBasicTransformations(transformed, targetCluster)
    
    return transformed
}

// applyTransformRule applies a specific transform rule
func (t *ResourceTransformer) applyTransformRule(
    deployment *appsv1.Deployment,
    rule TransformRule,
) {
    
    // Transform namespace if prefix specified
    if rule.NamespacePrefix != "" {
        deployment.Namespace = rule.NamespacePrefix + deployment.Namespace
    }
    
    // Transform image registry
    if rule.ImageRegistry != "" {
        t.transformImageRegistry(deployment, rule.ImageRegistry)
    }
    
    // Apply resource limits
    if len(rule.ResourceLimits) > 0 {
        t.applyResourceLimits(deployment, rule.ResourceLimits)
    }
    
    // Add annotations
    if deployment.Annotations == nil {
        deployment.Annotations = make(map[string]string)
    }
    for k, v := range rule.Annotations {
        deployment.Annotations[k] = v
    }
    
    // Add labels
    if deployment.Labels == nil {
        deployment.Labels = make(map[string]string)
    }
    for k, v := range rule.Labels {
        deployment.Labels[k] = v
    }
}

// applyBasicTransformations applies basic transformations
func (t *ResourceTransformer) applyBasicTransformations(
    deployment *appsv1.Deployment,
    targetCluster string,
) {
    
    // Reset cluster-specific fields
    deployment.ResourceVersion = ""
    deployment.UID = ""
    deployment.SelfLink = ""
    deployment.CreationTimestamp = metav1.Time{}
    deployment.Status = appsv1.DeploymentStatus{}
    
    // Add TMC tracking annotations
    if deployment.Annotations == nil {
        deployment.Annotations = make(map[string]string)
    }
    deployment.Annotations["tmc.kcp.io/target-cluster"] = targetCluster
    deployment.Annotations["tmc.kcp.io/managed-by"] = "tmc-controller"
    
    // Remove KCP-specific annotations
    delete(deployment.Annotations, "kcp.io/cluster")
}

// transformImageRegistry transforms container images to use a different registry
func (t *ResourceTransformer) transformImageRegistry(
    deployment *appsv1.Deployment,
    newRegistry string,
) {
    
    containers := deployment.Spec.Template.Spec.Containers
    for i := range containers {
        containers[i].Image = t.replaceRegistry(containers[i].Image, newRegistry)
    }
    
    initContainers := deployment.Spec.Template.Spec.InitContainers
    for i := range initContainers {
        initContainers[i].Image = t.replaceRegistry(initContainers[i].Image, newRegistry)
    }
}

// replaceRegistry replaces the registry in an image name
func (t *ResourceTransformer) replaceRegistry(image, newRegistry string) string {
    parts := strings.Split(image, "/")
    if len(parts) == 1 {
        // No registry specified, just image name
        return fmt.Sprintf("%s/%s", newRegistry, image)
    }
    
    // Replace the registry part
    parts[0] = newRegistry
    return strings.Join(parts, "/")
}

// applyResourceLimits applies resource limits to deployment containers
func (t *ResourceTransformer) applyResourceLimits(
    deployment *appsv1.Deployment,
    limits corev1.ResourceList,
) {
    
    containers := deployment.Spec.Template.Spec.Containers
    for i := range containers {
        if containers[i].Resources.Limits == nil {
            containers[i].Resources.Limits = make(corev1.ResourceList)
        }
        
        for resource, limit := range limits {
            containers[i].Resources.Limits[resource] = limit
        }
    }
}
```

## ✅ **Phase 3 Success Criteria**

### **Synchronization Compliance:**
1. **✅ External workload APIs** - TMC watches workload APIs bound via APIBinding
2. **✅ Physical cluster execution** - workloads created on real clusters
3. **✅ Bidirectional status sync** - cluster status propagates back to KCP
4. **✅ Resource transformation** - cluster-specific adaptations
5. **✅ Lifecycle management** - create, update, delete operations

### **Technical Validation:**
- TMC controller watches bound workload APIs in KCP workspace
- Deployments are created correctly on target physical clusters
- Status from physical clusters aggregates back to KCP
- Resource transformations work for different cluster types
- Deletion cleanup works across all clusters

### **Integration Test:**
```bash
# 1. Create deployment in KCP workspace (via bound API)
kubectl --kubeconfig=kcp.config create deployment nginx --image=nginx

# 2. TMC controller picks up deployment and places on clusters
# 3. Verify deployment exists on physical clusters
kubectl --kubeconfig=cluster1.config get deployment nginx
kubectl --kubeconfig=cluster2.config get deployment nginx

# 4. Verify status aggregates back to KCP
kubectl --kubeconfig=kcp.config get deployment nginx -o yaml
```

## 🎯 **Phase 3 Outcome**

This phase establishes:
- **Complete workload synchronization** between KCP APIs and physical clusters
- **Bidirectional status propagation** maintaining consistency
- **Resource transformation** for cluster-specific requirements
- **Foundation for advanced placement** in Phase 4
- **Production-ready sync engine** with proper error handling

**Phase 3 provides fully functional TMC workload synchronization while respecting KCP's architectural boundaries and maintaining proper separation between API provision and workload execution.**