# Resource Quota Manager Implementation Instructions

## Overview
This branch implements the resource quota management system for TMC, providing quota enforcement, resource tracking, and capacity management across clusters. It ensures fair resource allocation and prevents resource exhaustion.

**Branch**: `feature/tmc-completion/p1w3-quota-manager`  
**Estimated Lines**: 550 lines  
**Wave**: 2  
**Dependencies**: p1w1-synctarget-controller must be complete  

## Dependencies

### Required Before Starting
- Phase 0 APIs complete
- p1w1-synctarget-controller merged (provides cluster capacity information)
- Core TMC types available

### Blocks These Features
- p1w3-aggregator may depend on quota information

## Files to Create/Modify

### Primary Implementation Files (550 lines total)

1. **pkg/quota/manager.go** (180 lines)
   - Main quota manager implementation
   - Quota enforcement logic
   - Resource tracking

2. **pkg/quota/evaluator.go** (120 lines)
   - Quota evaluation logic
   - Resource calculation
   - Limit checking

3. **pkg/quota/tracker.go** (100 lines)
   - Resource usage tracking
   - Real-time monitoring
   - Usage aggregation

4. **pkg/quota/enforcer.go** (80 lines)
   - Quota enforcement
   - Admission control
   - Violation handling

5. **pkg/quota/calculator.go** (70 lines)
   - Resource calculation utilities
   - Unit conversions
   - Aggregation helpers

### Test Files (not counted in line limit)

1. **pkg/quota/manager_test.go**
2. **pkg/quota/evaluator_test.go**
3. **pkg/quota/tracker_test.go**

## Step-by-Step Implementation Guide

### Step 1: Setup Quota Manager (Hour 1-2)

```go
// pkg/quota/manager.go
package quota

import (
    "context"
    "fmt"
    "sync"
    "time"
    
    tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
    kcpclientset "github.com/kcp-dev/kcp/pkg/client/clientset/versioned/cluster"
    tmcinformers "github.com/kcp-dev/kcp/pkg/client/informers/externalversions/tmc/v1alpha1"
    tmclisters "github.com/kcp-dev/kcp/pkg/client/listers/tmc/v1alpha1"
    
    "github.com/kcp-dev/logicalcluster/v3"
    
    corev1 "k8s.io/api/core/v1"
    "k8s.io/apimachinery/pkg/api/resource"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/util/runtime"
    "k8s.io/apimachinery/pkg/util/wait"
    "k8s.io/client-go/tools/cache"
    "k8s.io/client-go/util/workqueue"
    "k8s.io/klog/v2"
)

// QuotaManager manages resource quotas across clusters
type QuotaManager struct {
    // Client
    kcpClusterClient kcpclientset.ClusterInterface
    
    // Listers
    syncTargetLister  tmclisters.SyncTargetClusterLister
    placementLister   tmclisters.WorkloadPlacementClusterLister
    
    // Components
    evaluator        *QuotaEvaluator
    tracker          *ResourceTracker
    enforcer         *QuotaEnforcer
    calculator       *ResourceCalculator
    
    // Quota data
    quotas           map[string]*ResourceQuota
    usage            map[string]*ResourceUsage
    quotaMutex       sync.RWMutex
    
    // Configuration
    config           *QuotaConfig
    
    // Workspace
    logicalCluster   logicalcluster.Name
    
    // Queue
    queue            workqueue.RateLimitingInterface
}

// QuotaConfig holds quota manager configuration
type QuotaConfig struct {
    // Default quotas
    DefaultCPUQuota    resource.Quantity
    DefaultMemoryQuota resource.Quantity
    DefaultStorageQuota resource.Quantity
    
    // Enforcement
    EnforcementEnabled bool
    HardLimitEnabled   bool
    
    // Monitoring
    UsageCheckInterval time.Duration
    
    // Overcommit ratios
    CPUOvercommitRatio    float64
    MemoryOvercommitRatio float64
}

// ResourceQuota represents quota for a workspace or cluster
type ResourceQuota struct {
    Name      string
    Namespace string
    
    // Limits
    Hard ResourceList
    
    // Current usage
    Used ResourceList
    
    // Status
    Status QuotaStatus
}

// ResourceUsage represents current resource usage
type ResourceUsage struct {
    Cluster   string
    Timestamp time.Time
    
    // Resources
    CPU     resource.Quantity
    Memory  resource.Quantity
    Storage resource.Quantity
    Pods    int32
    
    // Capacity
    Capacity ResourceList
}

// ResourceList represents a list of resources
type ResourceList map[corev1.ResourceName]resource.Quantity

// QuotaStatus represents quota status
type QuotaStatus struct {
    Phase      string
    Conditions []metav1.Condition
}

// NewQuotaManager creates a new quota manager
func NewQuotaManager(
    kcpClusterClient kcpclientset.ClusterInterface,
    syncTargetInformer tmcinformers.SyncTargetClusterInformer,
    placementInformer tmcinformers.WorkloadPlacementClusterInformer,
    logicalCluster logicalcluster.Name,
    config *QuotaConfig,
) (*QuotaManager, error) {
    if config == nil {
        config = &QuotaConfig{
            DefaultCPUQuota:       resource.MustParse("1000"),
            DefaultMemoryQuota:    resource.MustParse("1000Gi"),
            DefaultStorageQuota:   resource.MustParse("10Ti"),
            EnforcementEnabled:    true,
            HardLimitEnabled:      false,
            UsageCheckInterval:    30 * time.Second,
            CPUOvercommitRatio:    1.5,
            MemoryOvercommitRatio: 1.2,
        }
    }
    
    qm := &QuotaManager{
        kcpClusterClient: kcpClusterClient,
        syncTargetLister: syncTargetInformer.Lister(),
        placementLister:  placementInformer.Lister(),
        config:          config,
        logicalCluster:  logicalCluster,
        quotas:          make(map[string]*ResourceQuota),
        usage:           make(map[string]*ResourceUsage),
        queue:           workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "quota"),
    }
    
    // Initialize components
    qm.evaluator = NewQuotaEvaluator(qm)
    qm.tracker = NewResourceTracker(qm)
    qm.enforcer = NewQuotaEnforcer(qm)
    qm.calculator = NewResourceCalculator()
    
    // Setup informer handlers
    syncTargetInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
        AddFunc:    qm.handleSyncTargetAdd,
        UpdateFunc: qm.handleSyncTargetUpdate,
        DeleteFunc: qm.handleSyncTargetDelete,
    })
    
    placementInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
        AddFunc:    qm.handlePlacementAdd,
        UpdateFunc: qm.handlePlacementUpdate,
        DeleteFunc: qm.handlePlacementDelete,
    })
    
    return qm, nil
}

// Start starts the quota manager
func (qm *QuotaManager) Start(ctx context.Context) error {
    defer runtime.HandleCrash()
    defer qm.queue.ShutDown()
    
    klog.Info("Starting quota manager")
    
    // Start components
    go qm.tracker.Start(ctx)
    go qm.enforcer.Start(ctx)
    
    // Start workers
    for i := 0; i < 2; i++ {
        go wait.UntilWithContext(ctx, qm.runWorker, time.Second)
    }
    
    // Start periodic tasks
    go qm.runPeriodicTasks(ctx)
    
    <-ctx.Done()
    klog.Info("Stopping quota manager")
    
    return nil
}

// runWorker runs a single worker
func (qm *QuotaManager) runWorker(ctx context.Context) {
    for qm.processNextItem(ctx) {
    }
}

// processNextItem processes the next item from the queue
func (qm *QuotaManager) processNextItem(ctx context.Context) bool {
    key, quit := qm.queue.Get()
    if quit {
        return false
    }
    defer qm.queue.Done(key)
    
    err := qm.reconcile(ctx, key.(string))
    if err != nil {
        runtime.HandleError(fmt.Errorf("error reconciling quota for %s: %v", key, err))
        qm.queue.AddRateLimited(key)
        return true
    }
    
    qm.queue.Forget(key)
    return true
}

// reconcile reconciles quota for a resource
func (qm *QuotaManager) reconcile(ctx context.Context, key string) error {
    klog.V(4).Infof("Reconciling quota for %s", key)
    
    // Parse key to get resource type and name
    resourceType, name, err := qm.parseKey(key)
    if err != nil {
        return err
    }
    
    switch resourceType {
    case "synctarget":
        return qm.reconcileSyncTargetQuota(ctx, name)
    case "placement":
        return qm.reconcilePlacementQuota(ctx, name)
    default:
        return fmt.Errorf("unknown resource type: %s", resourceType)
    }
}

// reconcileSyncTargetQuota reconciles quota for a SyncTarget
func (qm *QuotaManager) reconcileSyncTargetQuota(ctx context.Context, name string) error {
    // Get SyncTarget
    syncTarget, err := qm.syncTargetLister.Cluster(qm.logicalCluster).Get(name)
    if err != nil {
        return err
    }
    
    // Calculate available resources
    available := qm.calculator.CalculateAvailable(syncTarget)
    
    // Update usage tracking
    usage := &ResourceUsage{
        Cluster:   name,
        Timestamp: time.Now(),
        Capacity:  available,
    }
    
    qm.updateUsage(name, usage)
    
    // Check quotas
    if qm.config.EnforcementEnabled {
        if err := qm.evaluator.EvaluateClusterQuota(ctx, syncTarget, usage); err != nil {
            return fmt.Errorf("quota evaluation failed: %w", err)
        }
    }
    
    return nil
}

// reconcilePlacementQuota reconciles quota for a WorkloadPlacement
func (qm *QuotaManager) reconcilePlacementQuota(ctx context.Context, name string) error {
    // Get WorkloadPlacement
    placement, err := qm.placementLister.Cluster(qm.logicalCluster).Get(name)
    if err != nil {
        return err
    }
    
    // Evaluate placement against quotas
    if qm.config.EnforcementEnabled {
        allowed, reason, err := qm.evaluator.EvaluatePlacement(ctx, placement)
        if err != nil {
            return fmt.Errorf("placement evaluation failed: %w", err)
        }
        
        if !allowed {
            // Update placement status with quota violation
            if err := qm.updatePlacementStatus(ctx, placement, reason); err != nil {
                return err
            }
        }
    }
    
    return nil
}

// GetQuota returns quota for a workspace
func (qm *QuotaManager) GetQuota(workspace string) (*ResourceQuota, error) {
    qm.quotaMutex.RLock()
    defer qm.quotaMutex.RUnlock()
    
    quota, exists := qm.quotas[workspace]
    if !exists {
        // Return default quota
        return qm.getDefaultQuota(workspace), nil
    }
    
    return quota, nil
}

// SetQuota sets quota for a workspace
func (qm *QuotaManager) SetQuota(workspace string, quota *ResourceQuota) error {
    qm.quotaMutex.Lock()
    defer qm.quotaMutex.Unlock()
    
    qm.quotas[workspace] = quota
    
    // Trigger re-evaluation
    qm.queue.Add(fmt.Sprintf("quota:%s", workspace))
    
    return nil
}

// GetUsage returns current usage for a cluster
func (qm *QuotaManager) GetUsage(cluster string) (*ResourceUsage, error) {
    qm.quotaMutex.RLock()
    defer qm.quotaMutex.RUnlock()
    
    usage, exists := qm.usage[cluster]
    if !exists {
        return nil, fmt.Errorf("usage not found for cluster %s", cluster)
    }
    
    return usage, nil
}

// updateUsage updates usage for a cluster
func (qm *QuotaManager) updateUsage(cluster string, usage *ResourceUsage) {
    qm.quotaMutex.Lock()
    defer qm.quotaMutex.Unlock()
    
    qm.usage[cluster] = usage
}

// getDefaultQuota returns default quota
func (qm *QuotaManager) getDefaultQuota(workspace string) *ResourceQuota {
    return &ResourceQuota{
        Name:      workspace,
        Namespace: workspace,
        Hard: ResourceList{
            corev1.ResourceCPU:     qm.config.DefaultCPUQuota,
            corev1.ResourceMemory:  qm.config.DefaultMemoryQuota,
            corev1.ResourceStorage: qm.config.DefaultStorageQuota,
        },
        Used: ResourceList{},
        Status: QuotaStatus{
            Phase: "Active",
        },
    }
}
```

### Step 2: Implement Quota Evaluator (Hour 3-4)

```go
// pkg/quota/evaluator.go
package quota

import (
    "context"
    "fmt"
    
    tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
    
    corev1 "k8s.io/api/core/v1"
    "k8s.io/apimachinery/pkg/api/resource"
    "k8s.io/klog/v2"
)

// QuotaEvaluator evaluates resources against quotas
type QuotaEvaluator struct {
    manager *QuotaManager
}

// NewQuotaEvaluator creates a new quota evaluator
func NewQuotaEvaluator(manager *QuotaManager) *QuotaEvaluator {
    return &QuotaEvaluator{
        manager: manager,
    }
}

// EvaluatePlacement evaluates if a placement fits within quotas
func (e *QuotaEvaluator) EvaluatePlacement(ctx context.Context, placement *tmcv1alpha1.WorkloadPlacement) (bool, string, error) {
    // Get workspace quota
    workspace := placement.Namespace
    quota, err := e.manager.GetQuota(workspace)
    if err != nil {
        return false, "", err
    }
    
    // Calculate required resources
    required := e.calculateRequiredResources(placement)
    
    // Check if resources fit within quota
    fits, resource := e.fitsInQuota(required, quota)
    if !fits {
        reason := fmt.Sprintf("Exceeds %s quota", resource)
        klog.V(2).Infof("Placement %s rejected: %s", placement.Name, reason)
        return false, reason, nil
    }
    
    // Check cluster capacity
    for _, target := range placement.Spec.TargetClusters {
        usage, err := e.manager.GetUsage(target.Name)
        if err != nil {
            continue // Skip if usage not available
        }
        
        if !e.hasCapacity(required, usage) {
            reason := fmt.Sprintf("Insufficient capacity in cluster %s", target.Name)
            klog.V(2).Infof("Placement %s rejected: %s", placement.Name, reason)
            return false, reason, nil
        }
    }
    
    return true, "", nil
}

// EvaluateClusterQuota evaluates cluster quota compliance
func (e *QuotaEvaluator) EvaluateClusterQuota(ctx context.Context, syncTarget *tmcv1alpha1.SyncTarget, usage *ResourceUsage) error {
    // Get cluster quota if defined
    quota, err := e.manager.GetQuota(syncTarget.Name)
    if err != nil {
        return err
    }
    
    // Check if usage exceeds quota
    if e.exceedsQuota(usage, quota) {
        klog.Warningf("Cluster %s exceeds quota", syncTarget.Name)
        
        if e.manager.config.HardLimitEnabled {
            // Trigger enforcement action
            return e.manager.enforcer.EnforceQuota(ctx, syncTarget, quota, usage)
        }
    }
    
    return nil
}

// calculateRequiredResources calculates resources required by a placement
func (e *QuotaEvaluator) calculateRequiredResources(placement *tmcv1alpha1.WorkloadPlacement) ResourceList {
    resources := make(ResourceList)
    
    // Extract resource requirements from placement
    if placement.Spec.ResourceRequirements != nil {
        if cpu := placement.Spec.ResourceRequirements.CPU; cpu != nil {
            resources[corev1.ResourceCPU] = *cpu
        }
        if memory := placement.Spec.ResourceRequirements.Memory; memory != nil {
            resources[corev1.ResourceMemory] = *memory
        }
        if storage := placement.Spec.ResourceRequirements.Storage; storage != nil {
            resources[corev1.ResourceStorage] = *storage
        }
    }
    
    // Apply default minimums if not specified
    if _, exists := resources[corev1.ResourceCPU]; !exists {
        resources[corev1.ResourceCPU] = resource.MustParse("100m")
    }
    if _, exists := resources[corev1.ResourceMemory]; !exists {
        resources[corev1.ResourceMemory] = resource.MustParse("128Mi")
    }
    
    return resources
}

// fitsInQuota checks if resources fit within quota
func (e *QuotaEvaluator) fitsInQuota(required ResourceList, quota *ResourceQuota) (bool, string) {
    for resourceName, requiredQuantity := range required {
        hardLimit, hasLimit := quota.Hard[resourceName]
        if !hasLimit {
            continue // No limit set for this resource
        }
        
        used, hasUsed := quota.Used[resourceName]
        if !hasUsed {
            used = resource.Quantity{}
        }
        
        available := hardLimit.DeepCopy()
        available.Sub(used)
        
        if requiredQuantity.Cmp(available) > 0 {
            return false, string(resourceName)
        }
    }
    
    return true, ""
}

// hasCapacity checks if cluster has capacity for resources
func (e *QuotaEvaluator) hasCapacity(required ResourceList, usage *ResourceUsage) bool {
    // Check CPU capacity
    if cpu, exists := required[corev1.ResourceCPU]; exists {
        availableCPU := usage.Capacity[corev1.ResourceCPU].DeepCopy()
        availableCPU.Sub(usage.CPU)
        
        // Apply overcommit ratio
        overcommitCPU := e.applyOvercommitRatio(availableCPU, e.manager.config.CPUOvercommitRatio)
        
        if cpu.Cmp(overcommitCPU) > 0 {
            return false
        }
    }
    
    // Check memory capacity
    if memory, exists := required[corev1.ResourceMemory]; exists {
        availableMemory := usage.Capacity[corev1.ResourceMemory].DeepCopy()
        availableMemory.Sub(usage.Memory)
        
        // Apply overcommit ratio
        overcommitMemory := e.applyOvercommitRatio(availableMemory, e.manager.config.MemoryOvercommitRatio)
        
        if memory.Cmp(overcommitMemory) > 0 {
            return false
        }
    }
    
    return true
}

// exceedsQuota checks if usage exceeds quota
func (e *QuotaEvaluator) exceedsQuota(usage *ResourceUsage, quota *ResourceQuota) bool {
    // Check CPU
    if cpuLimit, exists := quota.Hard[corev1.ResourceCPU]; exists {
        if usage.CPU.Cmp(cpuLimit) > 0 {
            return true
        }
    }
    
    // Check memory
    if memoryLimit, exists := quota.Hard[corev1.ResourceMemory]; exists {
        if usage.Memory.Cmp(memoryLimit) > 0 {
            return true
        }
    }
    
    // Check storage
    if storageLimit, exists := quota.Hard[corev1.ResourceStorage]; exists {
        if usage.Storage.Cmp(storageLimit) > 0 {
            return true
        }
    }
    
    return false
}

// applyOvercommitRatio applies overcommit ratio to a quantity
func (e *QuotaEvaluator) applyOvercommitRatio(quantity resource.Quantity, ratio float64) resource.Quantity {
    milliValue := quantity.MilliValue()
    overcommitMilliValue := int64(float64(milliValue) * ratio)
    return *resource.NewMilliQuantity(overcommitMilliValue, quantity.Format)
}
```

### Step 3: Implement Resource Tracker (Hour 5)

```go
// pkg/quota/tracker.go
package quota

import (
    "context"
    "fmt"
    "sync"
    "time"
    
    tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
    
    corev1 "k8s.io/api/core/v1"
    "k8s.io/apimachinery/pkg/api/resource"
    "k8s.io/klog/v2"
)

// ResourceTracker tracks resource usage across clusters
type ResourceTracker struct {
    manager *QuotaManager
    
    // Tracking data
    clusterUsage map[string]*ClusterUsage
    mutex        sync.RWMutex
}

// ClusterUsage represents usage for a cluster
type ClusterUsage struct {
    ClusterName   string
    LastUpdated   time.Time
    
    // Allocated resources (from placements)
    AllocatedCPU     resource.Quantity
    AllocatedMemory  resource.Quantity
    AllocatedStorage resource.Quantity
    
    // Actual usage (from monitoring)
    ActualCPU     resource.Quantity
    ActualMemory  resource.Quantity
    ActualStorage resource.Quantity
    
    // Capacity
    TotalCPU     resource.Quantity
    TotalMemory  resource.Quantity
    TotalStorage resource.Quantity
}

// NewResourceTracker creates a new resource tracker
func NewResourceTracker(manager *QuotaManager) *ResourceTracker {
    return &ResourceTracker{
        manager:      manager,
        clusterUsage: make(map[string]*ClusterUsage),
    }
}

// Start starts the resource tracker
func (t *ResourceTracker) Start(ctx context.Context) {
    ticker := time.NewTicker(t.manager.config.UsageCheckInterval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            t.updateAllUsage(ctx)
        }
    }
}

// updateAllUsage updates usage for all clusters
func (t *ResourceTracker) updateAllUsage(ctx context.Context) {
    syncTargets, err := t.manager.syncTargetLister.Cluster(t.manager.logicalCluster).List(labels.Everything())
    if err != nil {
        klog.Errorf("Failed to list SyncTargets: %v", err)
        return
    }
    
    for _, syncTarget := range syncTargets {
        if err := t.updateClusterUsage(ctx, syncTarget); err != nil {
            klog.Errorf("Failed to update usage for cluster %s: %v", syncTarget.Name, err)
        }
    }
}

// updateClusterUsage updates usage for a single cluster
func (t *ResourceTracker) updateClusterUsage(ctx context.Context, syncTarget *tmcv1alpha1.SyncTarget) error {
    usage := &ClusterUsage{
        ClusterName: syncTarget.Name,
        LastUpdated: time.Now(),
    }
    
    // Get capacity from SyncTarget status
    if syncTarget.Status.Capacity != nil {
        usage.TotalCPU = syncTarget.Status.Capacity.CPU
        usage.TotalMemory = syncTarget.Status.Capacity.Memory
        usage.TotalStorage = syncTarget.Status.Capacity.Storage
    }
    
    // Calculate allocated resources from placements
    allocated, err := t.calculateAllocatedResources(ctx, syncTarget.Name)
    if err != nil {
        return err
    }
    usage.AllocatedCPU = allocated[corev1.ResourceCPU]
    usage.AllocatedMemory = allocated[corev1.ResourceMemory]
    usage.AllocatedStorage = allocated[corev1.ResourceStorage]
    
    // Get actual usage (would come from monitoring in production)
    actual := t.getActualUsage(ctx, syncTarget)
    usage.ActualCPU = actual[corev1.ResourceCPU]
    usage.ActualMemory = actual[corev1.ResourceMemory]
    usage.ActualStorage = actual[corev1.ResourceStorage]
    
    // Store usage
    t.mutex.Lock()
    t.clusterUsage[syncTarget.Name] = usage
    t.mutex.Unlock()
    
    // Update manager's usage tracking
    t.manager.updateUsage(syncTarget.Name, &ResourceUsage{
        Cluster:   syncTarget.Name,
        Timestamp: usage.LastUpdated,
        CPU:       usage.ActualCPU,
        Memory:    usage.ActualMemory,
        Storage:   usage.ActualStorage,
        Capacity: ResourceList{
            corev1.ResourceCPU:     usage.TotalCPU,
            corev1.ResourceMemory:  usage.TotalMemory,
            corev1.ResourceStorage: usage.TotalStorage,
        },
    })
    
    klog.V(4).Infof("Updated usage for cluster %s: CPU=%s/%s, Memory=%s/%s",
        syncTarget.Name,
        usage.ActualCPU.String(), usage.TotalCPU.String(),
        usage.ActualMemory.String(), usage.TotalMemory.String())
    
    return nil
}

// calculateAllocatedResources calculates allocated resources for a cluster
func (t *ResourceTracker) calculateAllocatedResources(ctx context.Context, clusterName string) (ResourceList, error) {
    resources := make(ResourceList)
    
    // Get all placements targeting this cluster
    placements, err := t.manager.placementLister.Cluster(t.manager.logicalCluster).List(labels.Everything())
    if err != nil {
        return resources, err
    }
    
    for _, placement := range placements {
        // Check if placement targets this cluster
        if !t.targetsCluster(placement, clusterName) {
            continue
        }
        
        // Add placement's resource requirements
        if placement.Spec.ResourceRequirements != nil {
            if cpu := placement.Spec.ResourceRequirements.CPU; cpu != nil {
                current := resources[corev1.ResourceCPU]
                current.Add(*cpu)
                resources[corev1.ResourceCPU] = current
            }
            if memory := placement.Spec.ResourceRequirements.Memory; memory != nil {
                current := resources[corev1.ResourceMemory]
                current.Add(*memory)
                resources[corev1.ResourceMemory] = current
            }
            if storage := placement.Spec.ResourceRequirements.Storage; storage != nil {
                current := resources[corev1.ResourceStorage]
                current.Add(*storage)
                resources[corev1.ResourceStorage] = current
            }
        }
    }
    
    return resources, nil
}

// targetsCluster checks if a placement targets a specific cluster
func (t *ResourceTracker) targetsCluster(placement *tmcv1alpha1.WorkloadPlacement, clusterName string) bool {
    for _, target := range placement.Spec.TargetClusters {
        if target.Name == clusterName {
            return true
        }
    }
    
    // Check if placement was scheduled to this cluster
    for _, selected := range placement.Status.SelectedClusters {
        if selected == clusterName {
            return true
        }
    }
    
    return false
}

// getActualUsage gets actual resource usage for a cluster
func (t *ResourceTracker) getActualUsage(ctx context.Context, syncTarget *tmcv1alpha1.SyncTarget) ResourceList {
    // In production, this would query metrics from the cluster
    // For now, return allocated resources as actual usage
    allocated, _ := t.calculateAllocatedResources(ctx, syncTarget.Name)
    return allocated
}

// GetClusterUsage returns usage for a cluster
func (t *ResourceTracker) GetClusterUsage(clusterName string) (*ClusterUsage, error) {
    t.mutex.RLock()
    defer t.mutex.RUnlock()
    
    usage, exists := t.clusterUsage[clusterName]
    if !exists {
        return nil, fmt.Errorf("usage not found for cluster %s", clusterName)
    }
    
    return usage, nil
}

// GetAllUsage returns usage for all clusters
func (t *ResourceTracker) GetAllUsage() map[string]*ClusterUsage {
    t.mutex.RLock()
    defer t.mutex.RUnlock()
    
    result := make(map[string]*ClusterUsage)
    for k, v := range t.clusterUsage {
        result[k] = v
    }
    
    return result
}
```

### Step 4: Implement Quota Enforcer (Hour 6)

```go
// pkg/quota/enforcer.go
package quota

import (
    "context"
    "fmt"
    
    tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
    
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/klog/v2"
)

// QuotaEnforcer enforces quota limits
type QuotaEnforcer struct {
    manager *QuotaManager
}

// NewQuotaEnforcer creates a new quota enforcer
func NewQuotaEnforcer(manager *QuotaManager) *QuotaEnforcer {
    return &QuotaEnforcer{
        manager: manager,
    }
}

// Start starts the quota enforcer
func (e *QuotaEnforcer) Start(ctx context.Context) {
    // Periodic enforcement checks
    ticker := time.NewTicker(1 * time.Minute)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            e.enforceAllQuotas(ctx)
        }
    }
}

// enforceAllQuotas enforces quotas across all resources
func (e *QuotaEnforcer) enforceAllQuotas(ctx context.Context) {
    // Get all quotas
    e.manager.quotaMutex.RLock()
    quotas := make(map[string]*ResourceQuota)
    for k, v := range e.manager.quotas {
        quotas[k] = v
    }
    e.manager.quotaMutex.RUnlock()
    
    // Enforce each quota
    for workspace, quota := range quotas {
        if err := e.enforceWorkspaceQuota(ctx, workspace, quota); err != nil {
            klog.Errorf("Failed to enforce quota for workspace %s: %v", workspace, err)
        }
    }
}

// enforceWorkspaceQuota enforces quota for a workspace
func (e *QuotaEnforcer) enforceWorkspaceQuota(ctx context.Context, workspace string, quota *ResourceQuota) error {
    // Get current usage
    usage := e.calculateWorkspaceUsage(ctx, workspace)
    
    // Check for violations
    violations := e.checkViolations(usage, quota)
    if len(violations) == 0 {
        return nil
    }
    
    klog.Warningf("Quota violations in workspace %s: %v", workspace, violations)
    
    // Take enforcement action
    if e.manager.config.HardLimitEnabled {
        return e.enforceHardLimit(ctx, workspace, violations)
    }
    
    // Soft limit - just log warning
    return nil
}

// EnforceQuota enforces quota for a specific cluster
func (e *QuotaEnforcer) EnforceQuota(ctx context.Context, syncTarget *tmcv1alpha1.SyncTarget, quota *ResourceQuota, usage *ResourceUsage) error {
    klog.Infof("Enforcing quota for cluster %s", syncTarget.Name)
    
    // Identify placements to evict
    toEvict := e.selectPlacementsForEviction(ctx, syncTarget, quota, usage)
    
    // Evict placements
    for _, placement := range toEvict {
        if err := e.evictPlacement(ctx, placement); err != nil {
            klog.Errorf("Failed to evict placement %s: %v", placement.Name, err)
        }
    }
    
    return nil
}

// checkViolations checks for quota violations
func (e *QuotaEnforcer) checkViolations(usage ResourceList, quota *ResourceQuota) []string {
    var violations []string
    
    for resourceName, limit := range quota.Hard {
        used, exists := usage[resourceName]
        if !exists {
            continue
        }
        
        if used.Cmp(limit) > 0 {
            violations = append(violations, fmt.Sprintf("%s: %s > %s",
                resourceName, used.String(), limit.String()))
        }
    }
    
    return violations
}

// enforceHardLimit enforces hard quota limits
func (e *QuotaEnforcer) enforceHardLimit(ctx context.Context, workspace string, violations []string) error {
    // Block new placements
    e.manager.quotaMutex.Lock()
    if quota, exists := e.manager.quotas[workspace]; exists {
        quota.Status.Phase = "Exceeded"
        quota.Status.Conditions = append(quota.Status.Conditions, metav1.Condition{
            Type:               "QuotaExceeded",
            Status:             metav1.ConditionTrue,
            LastTransitionTime: metav1.Now(),
            Reason:             "LimitExceeded",
            Message:            fmt.Sprintf("Quota exceeded: %v", violations),
        })
    }
    e.manager.quotaMutex.Unlock()
    
    return nil
}

// selectPlacementsForEviction selects placements to evict
func (e *QuotaEnforcer) selectPlacementsForEviction(ctx context.Context, syncTarget *tmcv1alpha1.SyncTarget, quota *ResourceQuota, usage *ResourceUsage) []*tmcv1alpha1.WorkloadPlacement {
    // Simple strategy: evict lowest priority placements first
    // In production, this would be more sophisticated
    
    var toEvict []*tmcv1alpha1.WorkloadPlacement
    
    // This is a placeholder - actual implementation would:
    // 1. List all placements on the cluster
    // 2. Sort by priority/importance
    // 3. Select enough to bring usage under quota
    
    return toEvict
}

// evictPlacement evicts a placement
func (e *QuotaEnforcer) evictPlacement(ctx context.Context, placement *tmcv1alpha1.WorkloadPlacement) error {
    klog.Infof("Evicting placement %s due to quota violation", placement.Name)
    
    // Update placement status
    placement.Status.Phase = "Evicted"
    placement.Status.Conditions = append(placement.Status.Conditions, metav1.Condition{
        Type:               "Evicted",
        Status:             metav1.ConditionTrue,
        LastTransitionTime: metav1.Now(),
        Reason:             "QuotaExceeded",
        Message:            "Evicted due to quota violation",
    })
    
    // Update placement in API
    _, err := e.manager.kcpClusterClient.
        Cluster(logicalcluster.From(placement).Path()).
        TmcV1alpha1().
        WorkloadPlacements(placement.Namespace).
        UpdateStatus(ctx, placement, metav1.UpdateOptions{})
    
    return err
}

// calculateWorkspaceUsage calculates total usage for a workspace
func (e *QuotaEnforcer) calculateWorkspaceUsage(ctx context.Context, workspace string) ResourceList {
    usage := make(ResourceList)
    
    // Aggregate usage across all clusters in workspace
    // This is simplified - actual implementation would be more comprehensive
    
    return usage
}
```

### Step 5: Implement Resource Calculator (Hour 7)

```go
// pkg/quota/calculator.go
package quota

import (
    tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
    
    corev1 "k8s.io/api/core/v1"
    "k8s.io/apimachinery/pkg/api/resource"
)

// ResourceCalculator calculates resource metrics
type ResourceCalculator struct{}

// NewResourceCalculator creates a new resource calculator
func NewResourceCalculator() *ResourceCalculator {
    return &ResourceCalculator{}
}

// CalculateAvailable calculates available resources for a SyncTarget
func (c *ResourceCalculator) CalculateAvailable(syncTarget *tmcv1alpha1.SyncTarget) ResourceList {
    available := make(ResourceList)
    
    if syncTarget.Status.Capacity != nil {
        // Start with total capacity
        available[corev1.ResourceCPU] = syncTarget.Status.Capacity.CPU
        available[corev1.ResourceMemory] = syncTarget.Status.Capacity.Memory
        available[corev1.ResourceStorage] = syncTarget.Status.Capacity.Storage
        
        // Subtract allocated resources
        if syncTarget.Status.Allocated != nil {
            if cpu := available[corev1.ResourceCPU]; !cpu.IsZero() {
                cpu.Sub(syncTarget.Status.Allocated.CPU)
                available[corev1.ResourceCPU] = cpu
            }
            if memory := available[corev1.ResourceMemory]; !memory.IsZero() {
                memory.Sub(syncTarget.Status.Allocated.Memory)
                available[corev1.ResourceMemory] = memory
            }
            if storage := available[corev1.ResourceStorage]; !storage.IsZero() {
                storage.Sub(syncTarget.Status.Allocated.Storage)
                available[corev1.ResourceStorage] = storage
            }
        }
    }
    
    return available
}

// AddResources adds two resource lists
func (c *ResourceCalculator) AddResources(a, b ResourceList) ResourceList {
    result := make(ResourceList)
    
    // Copy all from a
    for k, v := range a {
        result[k] = v.DeepCopy()
    }
    
    // Add all from b
    for k, v := range b {
        if existing, exists := result[k]; exists {
            existing.Add(v)
            result[k] = existing
        } else {
            result[k] = v.DeepCopy()
        }
    }
    
    return result
}

// SubtractResources subtracts b from a
func (c *ResourceCalculator) SubtractResources(a, b ResourceList) ResourceList {
    result := make(ResourceList)
    
    // Copy all from a
    for k, v := range a {
        result[k] = v.DeepCopy()
    }
    
    // Subtract b
    for k, v := range b {
        if existing, exists := result[k]; exists {
            existing.Sub(v)
            result[k] = existing
        }
    }
    
    return result
}

// ConvertToMillis converts resources to milli-units
func (c *ResourceCalculator) ConvertToMillis(resources ResourceList) map[corev1.ResourceName]int64 {
    millis := make(map[corev1.ResourceName]int64)
    
    for name, quantity := range resources {
        millis[name] = quantity.MilliValue()
    }
    
    return millis
}

// CalculateUtilization calculates resource utilization percentage
func (c *ResourceCalculator) CalculateUtilization(used, total ResourceList) map[corev1.ResourceName]float64 {
    utilization := make(map[corev1.ResourceName]float64)
    
    for name, totalQuantity := range total {
        if totalQuantity.IsZero() {
            continue
        }
        
        usedQuantity, exists := used[name]
        if !exists {
            utilization[name] = 0
            continue
        }
        
        utilization[name] = float64(usedQuantity.MilliValue()) / float64(totalQuantity.MilliValue()) * 100
    }
    
    return utilization
}
```

## Testing Requirements

### Unit Tests

1. **Quota Manager Tests**
   - Test initialization
   - Test quota CRUD operations
   - Test reconciliation logic
   - Test event handlers

2. **Evaluator Tests**
   - Test placement evaluation
   - Test quota checking
   - Test capacity verification
   - Test overcommit calculations

3. **Tracker Tests**
   - Test usage tracking
   - Test allocation calculations
   - Test aggregation

4. **Enforcer Tests**
   - Test violation detection
   - Test enforcement actions
   - Test eviction logic

5. **Calculator Tests**
   - Test resource calculations
   - Test conversions
   - Test utilization metrics

### Integration Tests

1. **End-to-End Quota Management**
   - Create quotas
   - Place workloads
   - Verify enforcement
   - Check usage tracking

2. **Multi-Cluster Scenarios**
   - Test cross-cluster quotas
   - Test aggregated usage
   - Test capacity management

## KCP Patterns to Follow

### Controller Patterns
- Use informers and listers
- Implement work queues
- Handle retries properly

### Resource Management
- Follow Kubernetes resource model
- Use standard units and quantities
- Implement proper validation

### Multi-Tenancy
- Enforce workspace isolation
- Respect quota boundaries
- Implement fair sharing

## Integration Points

### With SyncTarget Controller (p1w1-synctarget-controller)
- Get cluster capacity information
- Monitor cluster health
- Track resource availability

### With Resource Aggregator (p1w3-aggregator)
- Share usage data
- Coordinate metrics collection
- Provide quota information

## Validation Checklist

### Before Commit
- [ ] All files created as specified
- [ ] Line count under 550 (run `/workspaces/kcp-shared-tools/tmc-pr-line-counter.sh`)
- [ ] All tests passing (`make test`)
- [ ] Linting passes (`make lint`)
- [ ] Resource calculations accurate

### Functionality Complete
- [ ] Quota manager operational
- [ ] Evaluation logic working
- [ ] Usage tracking functional
- [ ] Enforcement active
- [ ] Calculations correct

### Integration Ready
- [ ] Integrates with SyncTarget
- [ ] Quota API exposed
- [ ] Metrics available
- [ ] Event handling works

### Documentation Complete
- [ ] Quota API documented
- [ ] Usage patterns documented
- [ ] Configuration documented
- [ ] Enforcement policies documented

## Commit Message Template
```
feat(quota): implement resource quota management system

- Add quota manager with enforcement capabilities
- Implement quota evaluation and validation
- Add resource usage tracking across clusters
- Implement quota enforcement with eviction support
- Add resource calculation utilities
- Ensure workspace isolation throughout

Part of TMC Phase 1 Wave 2 implementation
Depends on: p1w1-synctarget-controller
```

## Next Steps
After this branch is complete:
1. Resource quotas will be enforced
2. Usage tracking operational
3. Capacity management active