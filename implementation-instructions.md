# Resource Aggregator Implementation Instructions

## Overview
This branch implements the resource aggregation system for TMC, collecting and aggregating resource metrics, capacity information, and usage data across multiple clusters to provide a unified view of resources.

**Branch**: `feature/tmc-completion/p1w3-aggregator`  
**Estimated Lines**: 500 lines  
**Wave**: 3  
**Dependencies**: p1w1-synctarget-controller must be complete  

## Dependencies

### Required Before Starting
- Phase 0 APIs complete
- p1w1-synctarget-controller merged (provides cluster data)
- Core TMC types available

### Blocks These Features
- None directly, but provides aggregated data for other components

## Files to Create/Modify

### Primary Implementation Files (500 lines total)

1. **pkg/quota/aggregator.go** (150 lines)
   - Main aggregator implementation
   - Data collection logic
   - Aggregation algorithms

2. **pkg/quota/collector.go** (120 lines)
   - Metrics collection from clusters
   - Data fetching logic
   - Collection scheduling

3. **pkg/quota/aggregation_store.go** (100 lines)
   - Storage for aggregated data
   - Query interface
   - Data persistence

4. **pkg/quota/metrics_processor.go** (80 lines)
   - Metrics processing logic
   - Data transformation
   - Statistical calculations

5. **pkg/quota/report_generator.go** (50 lines)
   - Report generation
   - Data formatting
   - Export utilities

### Test Files (not counted in line limit)

1. **pkg/quota/aggregator_test.go**
2. **pkg/quota/collector_test.go**
3. **pkg/quota/aggregation_store_test.go**

## Step-by-Step Implementation Guide

### Step 1: Setup Resource Aggregator (Hour 1-2)

```go
// pkg/quota/aggregator.go
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
    "k8s.io/apimachinery/pkg/labels"
    "k8s.io/apimachinery/pkg/util/runtime"
    "k8s.io/apimachinery/pkg/util/wait"
    "k8s.io/client-go/tools/cache"
    "k8s.io/client-go/util/workqueue"
    "k8s.io/klog/v2"
)

// ResourceAggregator aggregates resource data across clusters
type ResourceAggregator struct {
    // Client
    kcpClusterClient kcpclientset.ClusterInterface
    
    // Listers
    syncTargetLister   tmclisters.SyncTargetClusterLister
    placementLister    tmclisters.WorkloadPlacementClusterLister
    
    // Components
    collector          *MetricsCollector
    store             *AggregationStore
    processor         *MetricsProcessor
    reportGenerator   *ReportGenerator
    
    // Configuration
    config            *AggregatorConfig
    
    // Workspace
    logicalCluster    logicalcluster.Name
    
    // Queue
    queue             workqueue.RateLimitingInterface
    
    // State
    mutex             sync.RWMutex
    lastAggregation   time.Time
}

// AggregatorConfig holds aggregator configuration
type AggregatorConfig struct {
    // Collection interval
    CollectionInterval time.Duration
    
    // Aggregation window
    AggregationWindow time.Duration
    
    // Retention period
    RetentionPeriod time.Duration
    
    // Batch size for collection
    BatchSize int
    
    // Enable detailed metrics
    DetailedMetrics bool
}

// AggregatedResources represents aggregated resource data
type AggregatedResources struct {
    Timestamp time.Time
    Window    time.Duration
    
    // Cluster data
    Clusters []ClusterResources
    
    // Totals
    TotalCapacity    ResourceList
    TotalAllocated   ResourceList
    TotalUsed        ResourceList
    TotalAvailable   ResourceList
    
    // Statistics
    UtilizationRate  map[corev1.ResourceName]float64
    AllocationRate   map[corev1.ResourceName]float64
    
    // By workspace
    WorkspaceMetrics map[string]*WorkspaceResources
}

// ClusterResources represents resources for a single cluster
type ClusterResources struct {
    ClusterName string
    Location    string
    
    // Resources
    Capacity   ResourceList
    Allocated  ResourceList
    Used       ResourceList
    Available  ResourceList
    
    // Health
    Healthy    bool
    LastSeen   time.Time
}

// WorkspaceResources represents resources for a workspace
type WorkspaceResources struct {
    Workspace string
    
    // Resources
    Quota     ResourceList
    Allocated ResourceList
    Used      ResourceList
    
    // Placements
    PlacementCount int
    ClusterCount   int
}

// NewResourceAggregator creates a new resource aggregator
func NewResourceAggregator(
    kcpClusterClient kcpclientset.ClusterInterface,
    syncTargetInformer tmcinformers.SyncTargetClusterInformer,
    placementInformer tmcinformers.WorkloadPlacementClusterInformer,
    logicalCluster logicalcluster.Name,
    config *AggregatorConfig,
) (*ResourceAggregator, error) {
    if config == nil {
        config = &AggregatorConfig{
            CollectionInterval: 30 * time.Second,
            AggregationWindow:  5 * time.Minute,
            RetentionPeriod:    24 * time.Hour,
            BatchSize:          10,
            DetailedMetrics:    true,
        }
    }
    
    ra := &ResourceAggregator{
        kcpClusterClient: kcpClusterClient,
        syncTargetLister: syncTargetInformer.Lister(),
        placementLister:  placementInformer.Lister(),
        config:          config,
        logicalCluster:  logicalCluster,
        queue:           workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "aggregator"),
    }
    
    // Initialize components
    ra.collector = NewMetricsCollector(ra)
    ra.store = NewAggregationStore(config.RetentionPeriod)
    ra.processor = NewMetricsProcessor()
    ra.reportGenerator = NewReportGenerator()
    
    // Setup informer handlers
    syncTargetInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
        AddFunc:    ra.handleSyncTargetChange,
        UpdateFunc: func(old, new interface{}) { ra.handleSyncTargetChange(new) },
        DeleteFunc: ra.handleSyncTargetChange,
    })
    
    placementInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
        AddFunc:    ra.handlePlacementChange,
        UpdateFunc: func(old, new interface{}) { ra.handlePlacementChange(new) },
        DeleteFunc: ra.handlePlacementChange,
    })
    
    return ra, nil
}

// Start starts the resource aggregator
func (ra *ResourceAggregator) Start(ctx context.Context) error {
    defer runtime.HandleCrash()
    defer ra.queue.ShutDown()
    
    klog.Info("Starting resource aggregator")
    
    // Start components
    go ra.collector.Start(ctx)
    go ra.store.Start(ctx)
    
    // Start workers
    for i := 0; i < 2; i++ {
        go wait.UntilWithContext(ctx, ra.runWorker, time.Second)
    }
    
    // Start aggregation loop
    go ra.runAggregationLoop(ctx)
    
    <-ctx.Done()
    klog.Info("Stopping resource aggregator")
    
    return nil
}

// runWorker runs a single worker
func (ra *ResourceAggregator) runWorker(ctx context.Context) {
    for ra.processNextItem(ctx) {
    }
}

// processNextItem processes the next item from the queue
func (ra *ResourceAggregator) processNextItem(ctx context.Context) bool {
    key, quit := ra.queue.Get()
    if quit {
        return false
    }
    defer ra.queue.Done(key)
    
    err := ra.reconcile(ctx, key.(string))
    if err != nil {
        runtime.HandleError(fmt.Errorf("error reconciling %s: %v", key, err))
        ra.queue.AddRateLimited(key)
        return true
    }
    
    ra.queue.Forget(key)
    return true
}

// reconcile handles reconciliation for a resource
func (ra *ResourceAggregator) reconcile(ctx context.Context, key string) error {
    klog.V(4).Infof("Reconciling aggregation for %s", key)
    
    // Trigger aggregation
    return ra.aggregate(ctx)
}

// runAggregationLoop runs periodic aggregation
func (ra *ResourceAggregator) runAggregationLoop(ctx context.Context) {
    ticker := time.NewTicker(ra.config.CollectionInterval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            if err := ra.aggregate(ctx); err != nil {
                klog.Errorf("Aggregation failed: %v", err)
            }
        }
    }
}

// aggregate performs resource aggregation
func (ra *ResourceAggregator) aggregate(ctx context.Context) error {
    klog.V(4).Info("Starting resource aggregation")
    
    // Collect metrics from all clusters
    clusterMetrics, err := ra.collector.CollectAll(ctx)
    if err != nil {
        return fmt.Errorf("failed to collect metrics: %w", err)
    }
    
    // Process metrics
    aggregated := ra.processor.Process(clusterMetrics, ra.config.AggregationWindow)
    
    // Calculate workspace metrics
    workspaceMetrics, err := ra.calculateWorkspaceMetrics(ctx)
    if err != nil {
        klog.Errorf("Failed to calculate workspace metrics: %v", err)
    }
    aggregated.WorkspaceMetrics = workspaceMetrics
    
    // Store aggregated data
    if err := ra.store.Store(aggregated); err != nil {
        return fmt.Errorf("failed to store aggregation: %w", err)
    }
    
    // Update state
    ra.mutex.Lock()
    ra.lastAggregation = time.Now()
    ra.mutex.Unlock()
    
    klog.V(2).Infof("Aggregation complete: %d clusters, CPU: %s/%s, Memory: %s/%s",
        len(clusterMetrics),
        aggregated.TotalUsed[corev1.ResourceCPU].String(),
        aggregated.TotalCapacity[corev1.ResourceCPU].String(),
        aggregated.TotalUsed[corev1.ResourceMemory].String(),
        aggregated.TotalCapacity[corev1.ResourceMemory].String())
    
    return nil
}

// calculateWorkspaceMetrics calculates metrics per workspace
func (ra *ResourceAggregator) calculateWorkspaceMetrics(ctx context.Context) (map[string]*WorkspaceResources, error) {
    metrics := make(map[string]*WorkspaceResources)
    
    // Get all placements
    placements, err := ra.placementLister.Cluster(ra.logicalCluster).List(labels.Everything())
    if err != nil {
        return metrics, err
    }
    
    // Aggregate by workspace
    for _, placement := range placements {
        workspace := placement.Namespace
        if workspace == "" {
            workspace = "default"
        }
        
        if _, exists := metrics[workspace]; !exists {
            metrics[workspace] = &WorkspaceResources{
                Workspace: workspace,
                Allocated: make(ResourceList),
                Used:      make(ResourceList),
            }
        }
        
        // Add placement resources
        if placement.Spec.ResourceRequirements != nil {
            if cpu := placement.Spec.ResourceRequirements.CPU; cpu != nil {
                current := metrics[workspace].Allocated[corev1.ResourceCPU]
                current.Add(*cpu)
                metrics[workspace].Allocated[corev1.ResourceCPU] = current
            }
            if memory := placement.Spec.ResourceRequirements.Memory; memory != nil {
                current := metrics[workspace].Allocated[corev1.ResourceMemory]
                current.Add(*memory)
                metrics[workspace].Allocated[corev1.ResourceMemory] = current
            }
        }
        
        metrics[workspace].PlacementCount++
        
        // Count unique clusters
        clusters := make(map[string]bool)
        for _, target := range placement.Status.SelectedClusters {
            clusters[target] = true
        }
        metrics[workspace].ClusterCount = len(clusters)
    }
    
    return metrics, nil
}

// GetLatestAggregation returns the latest aggregation
func (ra *ResourceAggregator) GetLatestAggregation() (*AggregatedResources, error) {
    return ra.store.GetLatest()
}

// GetAggregationHistory returns aggregation history
func (ra *ResourceAggregator) GetAggregationHistory(duration time.Duration) ([]*AggregatedResources, error) {
    return ra.store.GetHistory(duration)
}

// GetClusterResources returns resources for a specific cluster
func (ra *ResourceAggregator) GetClusterResources(clusterName string) (*ClusterResources, error) {
    latest, err := ra.GetLatestAggregation()
    if err != nil {
        return nil, err
    }
    
    for _, cluster := range latest.Clusters {
        if cluster.ClusterName == clusterName {
            return &cluster, nil
        }
    }
    
    return nil, fmt.Errorf("cluster %s not found", clusterName)
}

// GetWorkspaceResources returns resources for a specific workspace
func (ra *ResourceAggregator) GetWorkspaceResources(workspace string) (*WorkspaceResources, error) {
    latest, err := ra.GetLatestAggregation()
    if err != nil {
        return nil, err
    }
    
    if ws, exists := latest.WorkspaceMetrics[workspace]; exists {
        return ws, nil
    }
    
    return nil, fmt.Errorf("workspace %s not found", workspace)
}

// handleSyncTargetChange handles SyncTarget changes
func (ra *ResourceAggregator) handleSyncTargetChange(obj interface{}) {
    ra.queue.Add("aggregate")
}

// handlePlacementChange handles WorkloadPlacement changes
func (ra *ResourceAggregator) handlePlacementChange(obj interface{}) {
    ra.queue.Add("aggregate")
}
```

### Step 2: Implement Metrics Collector (Hour 3-4)

```go
// pkg/quota/collector.go
package quota

import (
    "context"
    "fmt"
    "sync"
    "time"
    
    tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
    
    corev1 "k8s.io/api/core/v1"
    "k8s.io/apimachinery/pkg/api/resource"
    "k8s.io/apimachinery/pkg/labels"
    "k8s.io/klog/v2"
)

// MetricsCollector collects metrics from clusters
type MetricsCollector struct {
    aggregator *ResourceAggregator
    
    // Collection state
    collecting bool
    mutex      sync.Mutex
}

// CollectedMetrics represents collected metrics from a cluster
type CollectedMetrics struct {
    ClusterName string
    Timestamp   time.Time
    
    // Resource metrics
    Capacity   ResourceList
    Allocated  ResourceList
    Used       ResourceList
    Available  ResourceList
    
    // Node metrics
    NodeCount        int
    ReadyNodeCount   int
    
    // Pod metrics
    PodCount         int
    RunningPodCount  int
    
    // Health status
    Healthy          bool
    HealthMessage    string
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(aggregator *ResourceAggregator) *MetricsCollector {
    return &MetricsCollector{
        aggregator: aggregator,
    }
}

// Start starts the metrics collector
func (c *MetricsCollector) Start(ctx context.Context) {
    // Periodic collection is handled by aggregator
    // This could start additional collection workers if needed
    klog.Info("Metrics collector started")
}

// CollectAll collects metrics from all clusters
func (c *MetricsCollector) CollectAll(ctx context.Context) ([]*CollectedMetrics, error) {
    c.mutex.Lock()
    if c.collecting {
        c.mutex.Unlock()
        return nil, fmt.Errorf("collection already in progress")
    }
    c.collecting = true
    c.mutex.Unlock()
    
    defer func() {
        c.mutex.Lock()
        c.collecting = false
        c.mutex.Unlock()
    }()
    
    // Get all SyncTargets
    syncTargets, err := c.aggregator.syncTargetLister.
        Cluster(c.aggregator.logicalCluster).
        List(labels.Everything())
    if err != nil {
        return nil, fmt.Errorf("failed to list SyncTargets: %w", err)
    }
    
    // Collect metrics from each cluster
    var allMetrics []*CollectedMetrics
    var wg sync.WaitGroup
    var mu sync.Mutex
    
    // Use semaphore for concurrency control
    sem := make(chan struct{}, c.aggregator.config.BatchSize)
    
    for _, syncTarget := range syncTargets {
        wg.Add(1)
        go func(st *tmcv1alpha1.SyncTarget) {
            defer wg.Done()
            
            sem <- struct{}{}
            defer func() { <-sem }()
            
            metrics, err := c.collectFromCluster(ctx, st)
            if err != nil {
                klog.Errorf("Failed to collect metrics from cluster %s: %v", st.Name, err)
                // Still add partial metrics
                metrics = c.createEmptyMetrics(st)
            }
            
            mu.Lock()
            allMetrics = append(allMetrics, metrics)
            mu.Unlock()
        }(syncTarget)
    }
    
    wg.Wait()
    
    return allMetrics, nil
}

// collectFromCluster collects metrics from a single cluster
func (c *MetricsCollector) collectFromCluster(ctx context.Context, syncTarget *tmcv1alpha1.SyncTarget) (*CollectedMetrics, error) {
    metrics := &CollectedMetrics{
        ClusterName: syncTarget.Name,
        Timestamp:   time.Now(),
        Capacity:    make(ResourceList),
        Allocated:   make(ResourceList),
        Used:        make(ResourceList),
        Available:   make(ResourceList),
    }
    
    // Get capacity from SyncTarget status
    if syncTarget.Status.Capacity != nil {
        metrics.Capacity[corev1.ResourceCPU] = syncTarget.Status.Capacity.CPU
        metrics.Capacity[corev1.ResourceMemory] = syncTarget.Status.Capacity.Memory
        metrics.Capacity[corev1.ResourceStorage] = syncTarget.Status.Capacity.Storage
        metrics.Capacity[corev1.ResourcePods] = resource.MustParse(fmt.Sprintf("%d", syncTarget.Status.Capacity.Pods))
    }
    
    // Get allocated resources
    if syncTarget.Status.Allocated != nil {
        metrics.Allocated[corev1.ResourceCPU] = syncTarget.Status.Allocated.CPU
        metrics.Allocated[corev1.ResourceMemory] = syncTarget.Status.Allocated.Memory
        metrics.Allocated[corev1.ResourceStorage] = syncTarget.Status.Allocated.Storage
    }
    
    // Calculate used resources (would come from actual monitoring)
    // For now, use allocated as used
    metrics.Used = metrics.Allocated
    
    // Calculate available
    for resourceName, capacity := range metrics.Capacity {
        available := capacity.DeepCopy()
        if allocated, exists := metrics.Allocated[resourceName]; exists {
            available.Sub(allocated)
        }
        metrics.Available[resourceName] = available
    }
    
    // Get node metrics from status
    if syncTarget.Status.NodeSummary != nil {
        metrics.NodeCount = int(syncTarget.Status.NodeSummary.Total)
        metrics.ReadyNodeCount = int(syncTarget.Status.NodeSummary.Ready)
    }
    
    // Check health
    metrics.Healthy = c.isHealthy(syncTarget)
    if !metrics.Healthy {
        metrics.HealthMessage = c.getHealthMessage(syncTarget)
    }
    
    klog.V(4).Infof("Collected metrics from cluster %s: CPU=%s/%s, Memory=%s/%s",
        syncTarget.Name,
        metrics.Used[corev1.ResourceCPU].String(),
        metrics.Capacity[corev1.ResourceCPU].String(),
        metrics.Used[corev1.ResourceMemory].String(),
        metrics.Capacity[corev1.ResourceMemory].String())
    
    return metrics, nil
}

// CollectWorkloadMetrics collects metrics for workloads
func (c *MetricsCollector) CollectWorkloadMetrics(ctx context.Context) (map[string]*WorkloadMetrics, error) {
    workloadMetrics := make(map[string]*WorkloadMetrics)
    
    // Get all placements
    placements, err := c.aggregator.placementLister.
        Cluster(c.aggregator.logicalCluster).
        List(labels.Everything())
    if err != nil {
        return workloadMetrics, err
    }
    
    for _, placement := range placements {
        key := fmt.Sprintf("%s/%s", placement.Namespace, placement.Name)
        
        metrics := &WorkloadMetrics{
            Name:      placement.Name,
            Namespace: placement.Namespace,
            Phase:     placement.Status.Phase,
        }
        
        // Get resource requirements
        if placement.Spec.ResourceRequirements != nil {
            metrics.RequestedCPU = placement.Spec.ResourceRequirements.CPU
            metrics.RequestedMemory = placement.Spec.ResourceRequirements.Memory
        }
        
        // Get selected clusters
        metrics.TargetClusters = len(placement.Status.SelectedClusters)
        
        workloadMetrics[key] = metrics
    }
    
    return workloadMetrics, nil
}

// WorkloadMetrics represents metrics for a workload
type WorkloadMetrics struct {
    Name      string
    Namespace string
    Phase     string
    
    RequestedCPU    *resource.Quantity
    RequestedMemory *resource.Quantity
    
    TargetClusters int
}

// createEmptyMetrics creates empty metrics for a failed collection
func (c *MetricsCollector) createEmptyMetrics(syncTarget *tmcv1alpha1.SyncTarget) *CollectedMetrics {
    return &CollectedMetrics{
        ClusterName:   syncTarget.Name,
        Timestamp:     time.Now(),
        Capacity:      make(ResourceList),
        Allocated:     make(ResourceList),
        Used:          make(ResourceList),
        Available:     make(ResourceList),
        Healthy:       false,
        HealthMessage: "Metrics collection failed",
    }
}

// isHealthy checks if a SyncTarget is healthy
func (c *MetricsCollector) isHealthy(syncTarget *tmcv1alpha1.SyncTarget) bool {
    // Check Ready condition
    for _, condition := range syncTarget.Status.Conditions {
        if condition.Type == "Ready" {
            return condition.Status == metav1.ConditionTrue
        }
    }
    return false
}

// getHealthMessage gets health message for unhealthy cluster
func (c *MetricsCollector) getHealthMessage(syncTarget *tmcv1alpha1.SyncTarget) string {
    for _, condition := range syncTarget.Status.Conditions {
        if condition.Type == "Ready" && condition.Status != metav1.ConditionTrue {
            return condition.Message
        }
    }
    return "Unknown health issue"
}
```

### Step 3: Implement Aggregation Store (Hour 5)

```go
// pkg/quota/aggregation_store.go
package quota

import (
    "fmt"
    "sync"
    "time"
    
    "k8s.io/klog/v2"
)

// AggregationStore stores aggregated resource data
type AggregationStore struct {
    // Storage
    aggregations []*AggregatedResources
    mutex        sync.RWMutex
    
    // Configuration
    retentionPeriod time.Duration
    maxEntries      int
}

// NewAggregationStore creates a new aggregation store
func NewAggregationStore(retentionPeriod time.Duration) *AggregationStore {
    return &AggregationStore{
        aggregations:    make([]*AggregatedResources, 0),
        retentionPeriod: retentionPeriod,
        maxEntries:      10000, // Prevent unbounded growth
    }
}

// Start starts the aggregation store
func (s *AggregationStore) Start(ctx context.Context) {
    // Periodic cleanup
    ticker := time.NewTicker(1 * time.Hour)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            s.cleanup()
        }
    }
}

// Store stores an aggregation
func (s *AggregationStore) Store(aggregation *AggregatedResources) error {
    s.mutex.Lock()
    defer s.mutex.Unlock()
    
    // Add to store
    s.aggregations = append(s.aggregations, aggregation)
    
    // Enforce max entries
    if len(s.aggregations) > s.maxEntries {
        // Remove oldest entries
        removeCount := len(s.aggregations) - s.maxEntries
        s.aggregations = s.aggregations[removeCount:]
    }
    
    klog.V(4).Infof("Stored aggregation at %s, total entries: %d",
        aggregation.Timestamp.Format(time.RFC3339), len(s.aggregations))
    
    return nil
}

// GetLatest returns the latest aggregation
func (s *AggregationStore) GetLatest() (*AggregatedResources, error) {
    s.mutex.RLock()
    defer s.mutex.RUnlock()
    
    if len(s.aggregations) == 0 {
        return nil, fmt.Errorf("no aggregations available")
    }
    
    return s.aggregations[len(s.aggregations)-1], nil
}

// GetHistory returns aggregation history for a duration
func (s *AggregationStore) GetHistory(duration time.Duration) ([]*AggregatedResources, error) {
    s.mutex.RLock()
    defer s.mutex.RUnlock()
    
    cutoff := time.Now().Add(-duration)
    var history []*AggregatedResources
    
    for i := len(s.aggregations) - 1; i >= 0; i-- {
        if s.aggregations[i].Timestamp.Before(cutoff) {
            break
        }
        history = append([]*AggregatedResources{s.aggregations[i]}, history...)
    }
    
    return history, nil
}

// GetByTimestamp returns aggregation closest to a timestamp
func (s *AggregationStore) GetByTimestamp(timestamp time.Time) (*AggregatedResources, error) {
    s.mutex.RLock()
    defer s.mutex.RUnlock()
    
    if len(s.aggregations) == 0 {
        return nil, fmt.Errorf("no aggregations available")
    }
    
    // Find closest aggregation
    var closest *AggregatedResources
    minDiff := time.Duration(1<<63 - 1) // Max duration
    
    for _, agg := range s.aggregations {
        diff := timestamp.Sub(agg.Timestamp).Abs()
        if diff < minDiff {
            minDiff = diff
            closest = agg
        }
    }
    
    return closest, nil
}

// Query performs a query on stored aggregations
func (s *AggregationStore) Query(filter AggregationFilter) ([]*AggregatedResources, error) {
    s.mutex.RLock()
    defer s.mutex.RUnlock()
    
    var results []*AggregatedResources
    
    for _, agg := range s.aggregations {
        if filter.Matches(agg) {
            results = append(results, agg)
        }
    }
    
    return results, nil
}

// AggregationFilter filters aggregations
type AggregationFilter struct {
    StartTime    *time.Time
    EndTime      *time.Time
    ClusterName  string
    WorkspaceName string
}

// Matches checks if an aggregation matches the filter
func (f AggregationFilter) Matches(agg *AggregatedResources) bool {
    // Check time range
    if f.StartTime != nil && agg.Timestamp.Before(*f.StartTime) {
        return false
    }
    if f.EndTime != nil && agg.Timestamp.After(*f.EndTime) {
        return false
    }
    
    // Check cluster name
    if f.ClusterName != "" {
        found := false
        for _, cluster := range agg.Clusters {
            if cluster.ClusterName == f.ClusterName {
                found = true
                break
            }
        }
        if !found {
            return false
        }
    }
    
    // Check workspace
    if f.WorkspaceName != "" {
        if _, exists := agg.WorkspaceMetrics[f.WorkspaceName]; !exists {
            return false
        }
    }
    
    return true
}

// cleanup removes old aggregations
func (s *AggregationStore) cleanup() {
    s.mutex.Lock()
    defer s.mutex.Unlock()
    
    cutoff := time.Now().Add(-s.retentionPeriod)
    var kept []*AggregatedResources
    
    for _, agg := range s.aggregations {
        if agg.Timestamp.After(cutoff) {
            kept = append(kept, agg)
        }
    }
    
    removed := len(s.aggregations) - len(kept)
    if removed > 0 {
        klog.V(2).Infof("Cleaned up %d old aggregations", removed)
        s.aggregations = kept
    }
}

// GetStatistics returns statistics about stored data
func (s *AggregationStore) GetStatistics() *StoreStatistics {
    s.mutex.RLock()
    defer s.mutex.RUnlock()
    
    stats := &StoreStatistics{
        TotalEntries: len(s.aggregations),
    }
    
    if len(s.aggregations) > 0 {
        stats.OldestEntry = s.aggregations[0].Timestamp
        stats.NewestEntry = s.aggregations[len(s.aggregations)-1].Timestamp
        
        // Calculate average interval
        if len(s.aggregations) > 1 {
            totalDuration := stats.NewestEntry.Sub(stats.OldestEntry)
            stats.AverageInterval = totalDuration / time.Duration(len(s.aggregations)-1)
        }
    }
    
    return stats
}

// StoreStatistics represents statistics about the store
type StoreStatistics struct {
    TotalEntries    int
    OldestEntry     time.Time
    NewestEntry     time.Time
    AverageInterval time.Duration
}
```

### Step 4: Implement Metrics Processor (Hour 6)

```go
// pkg/quota/metrics_processor.go
package quota

import (
    "time"
    
    corev1 "k8s.io/api/core/v1"
    "k8s.io/apimachinery/pkg/api/resource"
)

// MetricsProcessor processes collected metrics
type MetricsProcessor struct{}

// NewMetricsProcessor creates a new metrics processor
func NewMetricsProcessor() *MetricsProcessor {
    return &MetricsProcessor{}
}

// Process processes collected metrics into aggregated form
func (p *MetricsProcessor) Process(metrics []*CollectedMetrics, window time.Duration) *AggregatedResources {
    aggregated := &AggregatedResources{
        Timestamp:        time.Now(),
        Window:           window,
        Clusters:         make([]ClusterResources, 0, len(metrics)),
        TotalCapacity:    make(ResourceList),
        TotalAllocated:   make(ResourceList),
        TotalUsed:        make(ResourceList),
        TotalAvailable:   make(ResourceList),
        UtilizationRate:  make(map[corev1.ResourceName]float64),
        AllocationRate:   make(map[corev1.ResourceName]float64),
        WorkspaceMetrics: make(map[string]*WorkspaceResources),
    }
    
    // Process each cluster
    for _, m := range metrics {
        cluster := ClusterResources{
            ClusterName: m.ClusterName,
            Capacity:    m.Capacity,
            Allocated:   m.Allocated,
            Used:        m.Used,
            Available:   m.Available,
            Healthy:     m.Healthy,
            LastSeen:    m.Timestamp,
        }
        
        aggregated.Clusters = append(aggregated.Clusters, cluster)
        
        // Aggregate totals
        p.addToTotal(aggregated.TotalCapacity, m.Capacity)
        p.addToTotal(aggregated.TotalAllocated, m.Allocated)
        p.addToTotal(aggregated.TotalUsed, m.Used)
        p.addToTotal(aggregated.TotalAvailable, m.Available)
    }
    
    // Calculate rates
    aggregated.UtilizationRate = p.calculateUtilizationRate(
        aggregated.TotalUsed,
        aggregated.TotalCapacity,
    )
    
    aggregated.AllocationRate = p.calculateAllocationRate(
        aggregated.TotalAllocated,
        aggregated.TotalCapacity,
    )
    
    return aggregated
}

// addToTotal adds resources to total
func (p *MetricsProcessor) addToTotal(total, add ResourceList) {
    for resourceName, quantity := range add {
        if existing, exists := total[resourceName]; exists {
            existing.Add(quantity)
            total[resourceName] = existing
        } else {
            total[resourceName] = quantity.DeepCopy()
        }
    }
}

// calculateUtilizationRate calculates utilization rate
func (p *MetricsProcessor) calculateUtilizationRate(used, capacity ResourceList) map[corev1.ResourceName]float64 {
    rates := make(map[corev1.ResourceName]float64)
    
    for resourceName, capacityQuantity := range capacity {
        if capacityQuantity.IsZero() {
            continue
        }
        
        usedQuantity, exists := used[resourceName]
        if !exists {
            rates[resourceName] = 0
            continue
        }
        
        rates[resourceName] = float64(usedQuantity.MilliValue()) / float64(capacityQuantity.MilliValue()) * 100
    }
    
    return rates
}

// calculateAllocationRate calculates allocation rate
func (p *MetricsProcessor) calculateAllocationRate(allocated, capacity ResourceList) map[corev1.ResourceName]float64 {
    rates := make(map[corev1.ResourceName]float64)
    
    for resourceName, capacityQuantity := range capacity {
        if capacityQuantity.IsZero() {
            continue
        }
        
        allocatedQuantity, exists := allocated[resourceName]
        if !exists {
            rates[resourceName] = 0
            continue
        }
        
        rates[resourceName] = float64(allocatedQuantity.MilliValue()) / float64(capacityQuantity.MilliValue()) * 100
    }
    
    return rates
}

// CalculateTrends calculates resource trends
func (p *MetricsProcessor) CalculateTrends(history []*AggregatedResources) *ResourceTrends {
    if len(history) < 2 {
        return nil
    }
    
    trends := &ResourceTrends{
        Period: history[len(history)-1].Timestamp.Sub(history[0].Timestamp),
    }
    
    // Calculate CPU trend
    trends.CPUTrend = p.calculateTrend(history, corev1.ResourceCPU)
    
    // Calculate Memory trend
    trends.MemoryTrend = p.calculateTrend(history, corev1.ResourceMemory)
    
    // Calculate Storage trend
    trends.StorageTrend = p.calculateTrend(history, corev1.ResourceStorage)
    
    return trends
}

// ResourceTrends represents resource usage trends
type ResourceTrends struct {
    Period       time.Duration
    CPUTrend     TrendData
    MemoryTrend  TrendData
    StorageTrend TrendData
}

// TrendData represents trend data for a resource
type TrendData struct {
    Direction   string  // "increasing", "decreasing", "stable"
    ChangeRate  float64 // Percentage change per hour
    Prediction  resource.Quantity // Predicted value in 1 hour
}

// calculateTrend calculates trend for a specific resource
func (p *MetricsProcessor) calculateTrend(history []*AggregatedResources, resourceName corev1.ResourceName) TrendData {
    // Simple linear regression for trend
    // In production, use more sophisticated time series analysis
    
    var trend TrendData
    
    if len(history) < 2 {
        trend.Direction = "stable"
        return trend
    }
    
    first := history[0].TotalUsed[resourceName]
    last := history[len(history)-1].TotalUsed[resourceName]
    duration := history[len(history)-1].Timestamp.Sub(history[0].Timestamp)
    
    if duration == 0 {
        trend.Direction = "stable"
        return trend
    }
    
    changePerHour := float64(last.MilliValue()-first.MilliValue()) / duration.Hours()
    
    if changePerHour > 0 {
        trend.Direction = "increasing"
    } else if changePerHour < 0 {
        trend.Direction = "decreasing"
    } else {
        trend.Direction = "stable"
    }
    
    // Calculate change rate as percentage
    if !first.IsZero() {
        trend.ChangeRate = (changePerHour / float64(first.MilliValue())) * 100
    }
    
    // Simple prediction
    predictedMillis := last.MilliValue() + int64(changePerHour)
    trend.Prediction = *resource.NewMilliQuantity(predictedMillis, last.Format)
    
    return trend
}
```

### Step 5: Implement Report Generator (Hour 7)

```go
// pkg/quota/report_generator.go
package quota

import (
    "bytes"
    "encoding/json"
    "fmt"
    "text/template"
    "time"
    
    corev1 "k8s.io/api/core/v1"
)

// ReportGenerator generates reports from aggregated data
type ReportGenerator struct {
    templates map[string]*template.Template
}

// NewReportGenerator creates a new report generator
func NewReportGenerator() *ReportGenerator {
    rg := &ReportGenerator{
        templates: make(map[string]*template.Template),
    }
    
    // Initialize templates
    rg.initTemplates()
    
    return rg
}

// initTemplates initializes report templates
func (rg *ReportGenerator) initTemplates() {
    // Summary report template
    summaryTmpl := `Resource Aggregation Report
===========================
Generated: {{.Timestamp}}
Window: {{.Window}}

Cluster Summary:
----------------
Total Clusters: {{len .Clusters}}
Healthy Clusters: {{.HealthyCount}}

Resource Totals:
----------------
CPU:     {{.TotalUsed.cpu}}/{{.TotalCapacity.cpu}} ({{.UtilizationRate.cpu}}%)
Memory:  {{.TotalUsed.memory}}/{{.TotalCapacity.memory}} ({{.UtilizationRate.memory}}%)
Storage: {{.TotalUsed.storage}}/{{.TotalCapacity.storage}} ({{.UtilizationRate.storage}}%)

Top Utilized Clusters:
----------------------
{{range .TopUtilized}}
- {{.ClusterName}}: CPU {{.CPUUtil}}%, Memory {{.MemUtil}}%
{{end}}
`
    
    rg.templates["summary"] = template.Must(template.New("summary").Parse(summaryTmpl))
}

// GenerateSummaryReport generates a summary report
func (rg *ReportGenerator) GenerateSummaryReport(aggregated *AggregatedResources) (string, error) {
    data := rg.prepareSummaryData(aggregated)
    
    var buf bytes.Buffer
    if err := rg.templates["summary"].Execute(&buf, data); err != nil {
        return "", err
    }
    
    return buf.String(), nil
}

// GenerateJSONReport generates a JSON report
func (rg *ReportGenerator) GenerateJSONReport(aggregated *AggregatedResources) ([]byte, error) {
    return json.MarshalIndent(aggregated, "", "  ")
}

// GenerateCSVReport generates a CSV report
func (rg *ReportGenerator) GenerateCSVReport(aggregated *AggregatedResources) string {
    var buf bytes.Buffer
    
    // Header
    buf.WriteString("Cluster,CPU Capacity,CPU Used,Memory Capacity,Memory Used,Healthy\n")
    
    // Data rows
    for _, cluster := range aggregated.Clusters {
        buf.WriteString(fmt.Sprintf("%s,%s,%s,%s,%s,%v\n",
            cluster.ClusterName,
            cluster.Capacity[corev1.ResourceCPU].String(),
            cluster.Used[corev1.ResourceCPU].String(),
            cluster.Capacity[corev1.ResourceMemory].String(),
            cluster.Used[corev1.ResourceMemory].String(),
            cluster.Healthy,
        ))
    }
    
    return buf.String()
}

// prepareSummaryData prepares data for summary template
func (rg *ReportGenerator) prepareSummaryData(aggregated *AggregatedResources) map[string]interface{} {
    data := make(map[string]interface{})
    
    data["Timestamp"] = aggregated.Timestamp.Format(time.RFC3339)
    data["Window"] = aggregated.Window.String()
    data["Clusters"] = aggregated.Clusters
    
    // Count healthy clusters
    healthyCount := 0
    for _, cluster := range aggregated.Clusters {
        if cluster.Healthy {
            healthyCount++
        }
    }
    data["HealthyCount"] = healthyCount
    
    // Resource data
    data["TotalCapacity"] = aggregated.TotalCapacity
    data["TotalUsed"] = aggregated.TotalUsed
    data["UtilizationRate"] = aggregated.UtilizationRate
    
    // Find top utilized clusters
    data["TopUtilized"] = rg.findTopUtilized(aggregated, 5)
    
    return data
}

// findTopUtilized finds top utilized clusters
func (rg *ReportGenerator) findTopUtilized(aggregated *AggregatedResources, limit int) []map[string]interface{} {
    // Calculate utilization for each cluster
    type clusterUtil struct {
        name    string
        cpuUtil float64
        memUtil float64
    }
    
    var utils []clusterUtil
    for _, cluster := range aggregated.Clusters {
        cu := clusterUtil{name: cluster.ClusterName}
        
        if !cluster.Capacity[corev1.ResourceCPU].IsZero() {
            cu.cpuUtil = float64(cluster.Used[corev1.ResourceCPU].MilliValue()) /
                float64(cluster.Capacity[corev1.ResourceCPU].MilliValue()) * 100
        }
        
        if !cluster.Capacity[corev1.ResourceMemory].IsZero() {
            cu.memUtil = float64(cluster.Used[corev1.ResourceMemory].MilliValue()) /
                float64(cluster.Capacity[corev1.ResourceMemory].MilliValue()) * 100
        }
        
        utils = append(utils, cu)
    }
    
    // Sort by CPU utilization
    // Simple bubble sort for small datasets
    for i := 0; i < len(utils)-1; i++ {
        for j := 0; j < len(utils)-i-1; j++ {
            if utils[j].cpuUtil < utils[j+1].cpuUtil {
                utils[j], utils[j+1] = utils[j+1], utils[j]
            }
        }
    }
    
    // Convert to map format
    var result []map[string]interface{}
    for i := 0; i < limit && i < len(utils); i++ {
        result = append(result, map[string]interface{}{
            "ClusterName": utils[i].name,
            "CPUUtil":     fmt.Sprintf("%.1f", utils[i].cpuUtil),
            "MemUtil":     fmt.Sprintf("%.1f", utils[i].memUtil),
        })
    }
    
    return result
}
```

## Testing Requirements

### Unit Tests

1. **Aggregator Tests**
   - Test initialization
   - Test aggregation logic
   - Test workspace metrics calculation
   - Test event handling

2. **Collector Tests**
   - Test metrics collection
   - Test batch processing
   - Test error handling
   - Test health checks

3. **Store Tests**
   - Test storage operations
   - Test retention
   - Test queries
   - Test cleanup

4. **Processor Tests**
   - Test metrics processing
   - Test rate calculations
   - Test trend analysis

5. **Report Generator Tests**
   - Test report generation
   - Test different formats
   - Test template rendering

### Integration Tests

1. **End-to-End Aggregation**
   - Collect from multiple clusters
   - Process and aggregate
   - Store and retrieve
   - Generate reports

2. **Performance Tests**
   - Test with large numbers of clusters
   - Test with high-frequency updates
   - Test query performance

## KCP Patterns to Follow

### Data Collection
- Use informers for efficiency
- Batch operations
- Handle failures gracefully

### Storage Patterns
- Implement retention policies
- Use efficient data structures
- Provide query capabilities

### Aggregation Logic
- Support multiple aggregation levels
- Calculate derived metrics
- Maintain accuracy

## Integration Points

### With SyncTarget Controller (p1w1-synctarget-controller)
- Collect cluster metrics
- Monitor health status
- Track capacity changes

### With Quota Manager (p1w3-quota-manager)
- Provide aggregated usage data
- Share capacity information
- Support quota decisions

## Validation Checklist

### Before Commit
- [ ] All files created as specified
- [ ] Line count under 500 (run `/workspaces/kcp-shared-tools/tmc-pr-line-counter.sh`)
- [ ] All tests passing (`make test`)
- [ ] Linting passes (`make lint`)
- [ ] Metrics calculations accurate

### Functionality Complete
- [ ] Aggregator operational
- [ ] Collection working
- [ ] Storage functional
- [ ] Processing accurate
- [ ] Reports generated

### Integration Ready
- [ ] Integrates with SyncTarget
- [ ] Data accessible via API
- [ ] Metrics exported
- [ ] History maintained

### Documentation Complete
- [ ] API documented
- [ ] Metrics explained
- [ ] Query patterns documented
- [ ] Report formats documented

## Commit Message Template
```
feat(aggregator): implement resource aggregation system

- Add resource aggregator with multi-cluster support
- Implement metrics collection from all clusters
- Add aggregation store with retention and queries
- Implement metrics processing and trend analysis
- Add report generation in multiple formats
- Ensure accurate resource accounting

Part of TMC Phase 1 Wave 3 implementation
Depends on: p1w1-synctarget-controller
```

## Next Steps
After this branch is complete:
1. Resource aggregation operational
2. Historical data available
3. Reports can be generated