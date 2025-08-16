# Wave2C PR3: Status Aggregation Implementation Instructions

## PR Overview
- **Branch**: `feature/tmc2-impl2/wave2c-03-status-aggregation`
- **Target Size**: 400 lines (excluding generated code)
- **Base Branch**: `main` (will use interfaces from PR1 and sync loop from PR2)
- **Dependencies**: Requires PR1 interfaces, can work with PR2 sync loop
- **Purpose**: Implement status aggregation, conflict resolution, and update application to KCP

## Files to Create

### 1. Status Aggregator Implementation (150 lines)

#### File: `pkg/reconciler/workload/syncer/upstream/aggregator.go` (150 lines)
```go
package upstream

import (
    "context"
    "fmt"
    "sort"
    "sync"
    "time"
    
    "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/klog/v2"
    
    workloadv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"
    kcpfeatures "github.com/kcp-dev/kcp/pkg/features"
    utilfeature "k8s.io/apiserver/pkg/util/feature"
)

// aggregatorImpl implements StatusAggregator
type aggregatorImpl struct {
    strategy        workloadv1alpha1.ConflictStrategy
    resolver        ConflictResolver
    
    mu              sync.RWMutex
    lastAggregation *AggregatedStatus
    aggregationCache map[string]*AggregatedStatus // keyed by resource key
    cacheTimeout     time.Duration
}

// NewAggregator creates a new status aggregator
func NewAggregator(strategy workloadv1alpha1.ConflictStrategy, resolver ConflictResolver) StatusAggregator {
    if !utilfeature.DefaultMutableFeatureGate.Enabled(kcpfeatures.UpstreamSyncAggregation) {
        klog.V(2).Info("UpstreamSyncAggregation feature gate is disabled")
        return &noopAggregator{}
    }
    
    return &aggregatorImpl{
        strategy:         strategy,
        resolver:         resolver,
        aggregationCache: make(map[string]*AggregatedStatus),
        cacheTimeout:     30 * time.Second,
    }
}

// AggregateStatus combines status from multiple clusters
func (a *aggregatorImpl) AggregateStatus(ctx context.Context, resources []ResourceStatus) (*AggregatedStatus, error) {
    if !utilfeature.DefaultMutableFeatureGate.Enabled(kcpfeatures.UpstreamSyncAggregation) {
        return nil, nil
    }
    
    if len(resources) == 0 {
        return nil, fmt.Errorf("no resources to aggregate")
    }
    
    klog.V(4).Infof("Aggregating status from %d resources", len(resources))
    
    // Group resources by type and name
    resourceKey := a.getResourceKey(resources[0].Resource)
    
    // Check cache
    if cached := a.getCached(resourceKey); cached != nil {
        klog.V(5).Infof("Using cached aggregation for %s", resourceKey)
        return cached, nil
    }
    
    // Detect conflicts
    conflicts := a.detectConflicts(resources)
    
    // Resolve conflicts if any
    var resolvedResources []ResourceStatus
    if len(conflicts) > 0 {
        klog.V(3).Infof("Detected %d conflicts for %s", len(conflicts), resourceKey)
        
        for _, conflict := range conflicts {
            resolution, err := a.resolver.Resolve(ctx, conflict)
            if err != nil {
                klog.Errorf("Failed to resolve conflict: %v", err)
                continue
            }
            
            if resolution.ResolvedStatus != nil {
                resolvedResources = append(resolvedResources, *resolution.ResolvedStatus)
            }
        }
    } else {
        resolvedResources = resources
    }
    
    // Perform aggregation based on strategy
    aggregated := a.performAggregation(resolvedResources)
    
    // Cache the result
    a.cache(resourceKey, aggregated)
    
    a.mu.Lock()
    a.lastAggregation = aggregated
    a.mu.Unlock()
    
    klog.V(4).Infof("Aggregation complete for %s", resourceKey)
    return aggregated, nil
}

// ResolveConflicts handles conflicting status
func (a *aggregatorImpl) ResolveConflicts(ctx context.Context, conflicts []Conflict) (*Resolution, error) {
    if !utilfeature.DefaultMutableFeatureGate.Enabled(kcpfeatures.UpstreamSyncConflictResolution) {
        return nil, fmt.Errorf("conflict resolution feature is disabled")
    }
    
    klog.V(3).Infof("Resolving %d conflicts", len(conflicts))
    
    // Use resolver for each conflict
    var lastResolution *Resolution
    for _, conflict := range conflicts {
        resolution, err := a.resolver.Resolve(ctx, conflict)
        if err != nil {
            klog.Errorf("Failed to resolve conflict for %s: %v", conflict.ResourceKey, err)
            continue
        }
        lastResolution = resolution
    }
    
    return lastResolution, nil
}

// SetStrategy updates the conflict resolution strategy
func (a *aggregatorImpl) SetStrategy(strategy workloadv1alpha1.ConflictStrategy) {
    a.mu.Lock()
    defer a.mu.Unlock()
    
    a.strategy = strategy
    klog.V(3).Infof("Updated aggregation strategy to %s", strategy)
}

// GetLastAggregation returns the last aggregation result
func (a *aggregatorImpl) GetLastAggregation() *AggregatedStatus {
    a.mu.RLock()
    defer a.mu.RUnlock()
    
    return a.lastAggregation
}

// detectConflicts identifies conflicts in resource statuses
func (a *aggregatorImpl) detectConflicts(resources []ResourceStatus) []Conflict {
    var conflicts []Conflict
    
    // Group by resource generation
    generationMap := make(map[int64][]ResourceStatus)
    for _, resource := range resources {
        gen := resource.Resource.GetGeneration()
        generationMap[gen] = append(generationMap[gen], resource)
    }
    
    // If multiple generations exist, we have a conflict
    if len(generationMap) > 1 {
        conflict := Conflict{
            ResourceKey: a.getResourceKey(resources[0].Resource),
            Statuses:    resources,
            Type:        ConflictTypeGeneration,
            Severity:    ConflictSeverityMedium,
        }
        conflicts = append(conflicts, conflict)
    }
    
    // Check for status conflicts
    statusMap := make(map[string][]ResourceStatus)
    for _, resource := range resources {
        statusKey := a.getStatusKey(resource)
        statusMap[statusKey] = append(statusMap[statusKey], resource)
    }
    
    if len(statusMap) > 1 {
        conflict := Conflict{
            ResourceKey: a.getResourceKey(resources[0].Resource),
            Statuses:    resources,
            Type:        ConflictTypeStatus,
            Severity:    ConflictSeverityLow,
        }
        conflicts = append(conflicts, conflict)
    }
    
    return conflicts
}

// performAggregation aggregates based on strategy
func (a *aggregatorImpl) performAggregation(resources []ResourceStatus) *AggregatedStatus {
    if len(resources) == 0 {
        return nil
    }
    
    var chosen *ResourceStatus
    
    switch a.strategy {
    case workloadv1alpha1.ConflictStrategyUseNewest:
        chosen = a.findNewest(resources)
    case workloadv1alpha1.ConflictStrategyUseOldest:
        chosen = a.findOldest(resources)
    case workloadv1alpha1.ConflictStrategyPriority:
        chosen = a.findByPriority(resources)
    default:
        chosen = &resources[0]
    }
    
    return &AggregatedStatus{
        ResourceKey:       a.getResourceKey(chosen.Resource),
        CombinedStatus:    chosen.Resource.DeepCopy(),
        SourceStatuses:    resources,
        AggregationTime:   time.Now(),
        ConflictsResolved: len(resources) - 1,
    }
}

// findNewest returns the newest resource status
func (a *aggregatorImpl) findNewest(resources []ResourceStatus) *ResourceStatus {
    if len(resources) == 0 {
        return nil
    }
    
    newest := &resources[0]
    for i := range resources {
        if resources[i].LastUpdated.After(newest.LastUpdated) {
            newest = &resources[i]
        }
    }
    
    return newest
}

// findOldest returns the oldest resource status
func (a *aggregatorImpl) findOldest(resources []ResourceStatus) *ResourceStatus {
    if len(resources) == 0 {
        return nil
    }
    
    oldest := &resources[0]
    for i := range resources {
        if resources[i].LastUpdated.Before(oldest.LastUpdated) {
            oldest = &resources[i]
        }
    }
    
    return oldest
}

// findByPriority returns highest priority resource
func (a *aggregatorImpl) findByPriority(resources []ResourceStatus) *ResourceStatus {
    if len(resources) == 0 {
        return nil
    }
    
    // Sort by cluster name as a simple priority
    sort.Slice(resources, func(i, j int) bool {
        return resources[i].ClusterName < resources[j].ClusterName
    })
    
    return &resources[0]
}

// Helper methods
func (a *aggregatorImpl) getResourceKey(resource *unstructured.Unstructured) string {
    return fmt.Sprintf("%s/%s/%s/%s",
        resource.GetAPIVersion(),
        resource.GetKind(),
        resource.GetNamespace(),
        resource.GetName())
}

func (a *aggregatorImpl) getStatusKey(status ResourceStatus) string {
    statusObj, _, _ := unstructured.NestedMap(status.Resource.Object, "status")
    return fmt.Sprintf("%v", statusObj)
}

func (a *aggregatorImpl) cache(key string, aggregated *AggregatedStatus) {
    a.mu.Lock()
    defer a.mu.Unlock()
    a.aggregationCache[key] = aggregated
}

func (a *aggregatorImpl) getCached(key string) *AggregatedStatus {
    a.mu.RLock()
    defer a.mu.RUnlock()
    
    cached, exists := a.aggregationCache[key]
    if !exists {
        return nil
    }
    
    if time.Since(cached.AggregationTime) > a.cacheTimeout {
        return nil
    }
    
    return cached
}

// noopAggregator when feature is disabled
type noopAggregator struct{}

func (n *noopAggregator) AggregateStatus(ctx context.Context, resources []ResourceStatus) (*AggregatedStatus, error) {
    return nil, nil
}
func (n *noopAggregator) ResolveConflicts(ctx context.Context, conflicts []Conflict) (*Resolution, error) {
    return nil, nil
}
func (n *noopAggregator) SetStrategy(strategy workloadv1alpha1.ConflictStrategy) {}
func (n *noopAggregator) GetLastAggregation() *AggregatedStatus { return nil }
```

### 2. Conflict Resolver Implementation (100 lines)

#### File: `pkg/reconciler/workload/syncer/upstream/resolver.go` (100 lines)
```go
package upstream

import (
    "context"
    "fmt"
    "time"
    
    "k8s.io/klog/v2"
    
    workloadv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"
    kcpfeatures "github.com/kcp-dev/kcp/pkg/features"
    utilfeature "k8s.io/apiserver/pkg/util/feature"
)

// resolverImpl implements ConflictResolver
type resolverImpl struct {
    strategy workloadv1alpha1.ConflictStrategy
}

// NewResolver creates a new conflict resolver
func NewResolver(strategy workloadv1alpha1.ConflictStrategy) ConflictResolver {
    if !utilfeature.DefaultMutableFeatureGate.Enabled(kcpfeatures.UpstreamSyncConflictResolution) {
        klog.V(2).Info("UpstreamSyncConflictResolution feature gate is disabled")
        return &noopResolver{}
    }
    
    return &resolverImpl{
        strategy: strategy,
    }
}

// Resolve attempts to resolve a conflict
func (r *resolverImpl) Resolve(ctx context.Context, conflict Conflict) (*Resolution, error) {
    if !utilfeature.DefaultMutableFeatureGate.Enabled(kcpfeatures.UpstreamSyncConflictResolution) {
        return nil, fmt.Errorf("conflict resolution is disabled")
    }
    
    klog.V(4).Infof("Resolving conflict for %s with strategy %s", conflict.ResourceKey, r.strategy)
    
    // Check if auto-resolvable
    if !r.CanAutoResolve(conflict) {
        return nil, fmt.Errorf("conflict cannot be auto-resolved: %s", conflict.ResourceKey)
    }
    
    var resolvedStatus *ResourceStatus
    
    switch r.strategy {
    case workloadv1alpha1.ConflictStrategyUseNewest:
        resolvedStatus = r.resolveByNewest(conflict)
    case workloadv1alpha1.ConflictStrategyUseOldest:
        resolvedStatus = r.resolveByOldest(conflict)
    case workloadv1alpha1.ConflictStrategyPriority:
        resolvedStatus = r.resolveByPriority(conflict)
    case workloadv1alpha1.ConflictStrategyManual:
        return nil, fmt.Errorf("manual resolution required for %s", conflict.ResourceKey)
    default:
        return nil, fmt.Errorf("unknown strategy: %s", r.strategy)
    }
    
    if resolvedStatus == nil {
        return nil, fmt.Errorf("failed to resolve conflict for %s", conflict.ResourceKey)
    }
    
    resolution := &Resolution{
        Conflict:       conflict,
        ResolvedStatus: resolvedStatus,
        Strategy:       string(r.strategy),
        Timestamp:      time.Now(),
    }
    
    klog.V(3).Infof("Resolved conflict for %s using %s strategy", conflict.ResourceKey, r.strategy)
    return resolution, nil
}

// SetStrategy updates the resolution strategy
func (r *resolverImpl) SetStrategy(strategy workloadv1alpha1.ConflictStrategy) {
    r.strategy = strategy
    klog.V(3).Infof("Updated conflict resolution strategy to %s", strategy)
}

// CanAutoResolve checks if conflict can be automatically resolved
func (r *resolverImpl) CanAutoResolve(conflict Conflict) bool {
    // Cannot auto-resolve high severity conflicts
    if conflict.Severity == ConflictSeverityHigh {
        return false
    }
    
    // Cannot auto-resolve if manual strategy
    if r.strategy == workloadv1alpha1.ConflictStrategyManual {
        return false
    }
    
    // Cannot auto-resolve if no statuses
    if len(conflict.Statuses) == 0 {
        return false
    }
    
    // Can auto-resolve generation conflicts with newest/oldest
    if conflict.Type == ConflictTypeGeneration {
        return r.strategy == workloadv1alpha1.ConflictStrategyUseNewest ||
               r.strategy == workloadv1alpha1.ConflictStrategyUseOldest
    }
    
    // Can auto-resolve status conflicts with any strategy
    if conflict.Type == ConflictTypeStatus {
        return true
    }
    
    return false
}

// resolveByNewest picks the newest status
func (r *resolverImpl) resolveByNewest(conflict Conflict) *ResourceStatus {
    if len(conflict.Statuses) == 0 {
        return nil
    }
    
    newest := conflict.Statuses[0]
    for _, status := range conflict.Statuses[1:] {
        if status.LastUpdated.After(newest.LastUpdated) {
            newest = status
        }
    }
    
    return &newest
}

// resolveByOldest picks the oldest status
func (r *resolverImpl) resolveByOldest(conflict Conflict) *ResourceStatus {
    if len(conflict.Statuses) == 0 {
        return nil
    }
    
    oldest := conflict.Statuses[0]
    for _, status := range conflict.Statuses[1:] {
        if status.LastUpdated.Before(oldest.LastUpdated) {
            oldest = status
        }
    }
    
    return &oldest
}

// resolveByPriority resolves by cluster priority
func (r *resolverImpl) resolveByPriority(conflict Conflict) *ResourceStatus {
    if len(conflict.Statuses) == 0 {
        return nil
    }
    
    // Simple priority: prefer production clusters (containing "prod" in name)
    for i := range conflict.Statuses {
        if contains(conflict.Statuses[i].ClusterName, "prod") {
            return &conflict.Statuses[i]
        }
    }
    
    // Fallback to first
    return &conflict.Statuses[0]
}

// Helper function
func contains(s, substr string) bool {
    return len(s) >= len(substr) && s[0:len(substr)] == substr
}

// noopResolver when feature is disabled
type noopResolver struct{}

func (n *noopResolver) Resolve(ctx context.Context, conflict Conflict) (*Resolution, error) {
    return nil, fmt.Errorf("resolver disabled")
}
func (n *noopResolver) SetStrategy(strategy workloadv1alpha1.ConflictStrategy) {}
func (n *noopResolver) CanAutoResolve(conflict Conflict) bool { return false }
```

### 3. Update Applier Implementation (100 lines)

#### File: `pkg/reconciler/workload/syncer/upstream/applier/applier.go` (100 lines)
```go
package applier

import (
    "context"
    "fmt"
    "sync/atomic"
    
    "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/types"
    "k8s.io/client-go/dynamic"
    "k8s.io/klog/v2"
    
    "github.com/kcp-dev/kcp/pkg/reconciler/workload/syncer/upstream"
    kcpfeatures "github.com/kcp-dev/kcp/pkg/features"
    utilfeature "k8s.io/apiserver/pkg/util/feature"
)

// applierImpl implements UpdateApplier
type applierImpl struct {
    client       dynamic.Interface
    dryRun       bool
    appliedCount int64
}

// NewApplier creates a new update applier
func NewApplier(client dynamic.Interface) upstream.UpdateApplier {
    if !utilfeature.DefaultMutableFeatureGate.Enabled(kcpfeatures.UpstreamSyncAggregation) {
        klog.V(2).Info("UpstreamSyncAggregation feature gate is disabled")
        return &noopApplier{}
    }
    
    return &applierImpl{
        client: client,
    }
}

// Apply applies a single update to KCP
func (a *applierImpl) Apply(ctx context.Context, update *upstream.Update) error {
    if !utilfeature.DefaultMutableFeatureGate.Enabled(kcpfeatures.UpstreamSync) {
        return nil
    }
    
    klog.V(4).Infof("Applying %s update for %s/%s", 
        update.Type, update.Resource.GetNamespace(), update.Resource.GetName())
    
    // Get resource interface
    gvr := update.Resource.GroupVersionKind().GroupVersion().WithResource(
        update.Resource.GetKind() + "s") // Simple pluralization
    
    var resourceClient dynamic.ResourceInterface
    if update.Resource.GetNamespace() != "" {
        resourceClient = a.client.Resource(gvr).Namespace(update.Resource.GetNamespace())
    } else {
        resourceClient = a.client.Resource(gvr)
    }
    
    // Apply based on type
    var err error
    switch update.Type {
    case upstream.UpdateTypeCreate:
        err = a.applyCreate(ctx, resourceClient, update)
    case upstream.UpdateTypeUpdate:
        err = a.applyUpdate(ctx, resourceClient, update)
    case upstream.UpdateTypeDelete:
        err = a.applyDelete(ctx, resourceClient, update)
    case upstream.UpdateTypeStatus:
        err = a.applyStatus(ctx, resourceClient, update)
    default:
        err = fmt.Errorf("unknown update type: %s", update.Type)
    }
    
    if err != nil {
        klog.Errorf("Failed to apply update: %v", err)
        return err
    }
    
    atomic.AddInt64(&a.appliedCount, 1)
    klog.V(4).Info("Update applied successfully")
    return nil
}

// ApplyBatch applies multiple updates
func (a *applierImpl) ApplyBatch(ctx context.Context, updates []*upstream.Update) error {
    if !utilfeature.DefaultMutableFeatureGate.Enabled(kcpfeatures.UpstreamSync) {
        return nil
    }
    
    klog.V(3).Infof("Applying batch of %d updates", len(updates))
    
    var errors []error
    for _, update := range updates {
        if err := a.Apply(ctx, update); err != nil {
            errors = append(errors, err)
        }
    }
    
    if len(errors) > 0 {
        return fmt.Errorf("batch apply had %d errors", len(errors))
    }
    
    return nil
}

// SetDryRun enables or disables dry-run mode
func (a *applierImpl) SetDryRun(enabled bool) {
    a.dryRun = enabled
    klog.V(3).Infof("Dry-run mode set to %v", enabled)
}

// GetAppliedCount returns the number of successful applies
func (a *applierImpl) GetAppliedCount() int64 {
    return atomic.LoadInt64(&a.appliedCount)
}

// applyCreate creates a new resource
func (a *applierImpl) applyCreate(ctx context.Context, client dynamic.ResourceInterface, update *upstream.Update) error {
    options := metav1.CreateOptions{}
    if a.dryRun {
        options.DryRun = []string{metav1.DryRunAll}
    }
    
    _, err := client.Create(ctx, update.Resource, options)
    return err
}

// applyUpdate updates an existing resource
func (a *applierImpl) applyUpdate(ctx context.Context, client dynamic.ResourceInterface, update *upstream.Update) error {
    options := metav1.UpdateOptions{}
    if a.dryRun {
        options.DryRun = []string{metav1.DryRunAll}
    }
    
    if update.Strategy == upstream.ApplyStrategyServerSide {
        // Use server-side apply
        data, err := update.Resource.MarshalJSON()
        if err != nil {
            return err
        }
        
        patchOptions := metav1.PatchOptions{
            FieldManager: "upstream-sync",
            Force:        &[]bool{true}[0],
        }
        if a.dryRun {
            patchOptions.DryRun = []string{metav1.DryRunAll}
        }
        
        _, err = client.Patch(ctx, update.Resource.GetName(), 
            types.ApplyPatchType, data, patchOptions)
        return err
    }
    
    _, err := client.Update(ctx, update.Resource, options)
    return err
}

// applyDelete deletes a resource
func (a *applierImpl) applyDelete(ctx context.Context, client dynamic.ResourceInterface, update *upstream.Update) error {
    options := metav1.DeleteOptions{}
    if a.dryRun {
        options.DryRun = []string{metav1.DryRunAll}
    }
    
    return client.Delete(ctx, update.Resource.GetName(), options)
}

// applyStatus updates only the status subresource
func (a *applierImpl) applyStatus(ctx context.Context, client dynamic.ResourceInterface, update *upstream.Update) error {
    options := metav1.UpdateOptions{}
    if a.dryRun {
        options.DryRun = []string{metav1.DryRunAll}
    }
    
    _, err := client.UpdateStatus(ctx, update.Resource, options)
    return err
}

// noopApplier when feature is disabled
type noopApplier struct{}

func (n *noopApplier) Apply(ctx context.Context, update *upstream.Update) error { return nil }
func (n *noopApplier) ApplyBatch(ctx context.Context, updates []*upstream.Update) error { return nil }
func (n *noopApplier) SetDryRun(enabled bool) {}
func (n *noopApplier) GetAppliedCount() int64 { return 0 }
```

### 4. Metrics Collector (50 lines)

#### File: `pkg/reconciler/workload/syncer/upstream/metrics/collector.go` (50 lines)
```go
package metrics

import (
    "sync"
    
    "github.com/prometheus/client_golang/prometheus"
    "k8s.io/component-base/metrics"
    "k8s.io/component-base/metrics/legacyregistry"
)

var (
    upstreamSyncTargets = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "kcp_upstream_sync_targets_total",
            Help: "Number of active upstream sync targets",
        },
        []string{"workspace"},
    )
    
    upstreamResourcesSynced = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "kcp_upstream_resources_synced_total",
            Help: "Total number of resources synced from upstream",
        },
        []string{"workspace", "cluster", "resource"},
    )
    
    upstreamConflictsResolved = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "kcp_upstream_conflicts_resolved_total",
            Help: "Total number of conflicts resolved",
        },
        []string{"workspace", "strategy"},
    )
    
    upstreamSyncLatency = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "kcp_upstream_sync_latency_seconds",
            Help:    "Latency of upstream sync operations",
            Buckets: prometheus.ExponentialBuckets(0.001, 2, 10),
        },
        []string{"workspace", "operation"},
    )
    
    registerOnce sync.Once
)

// Register registers all upstream sync metrics
func Register() {
    registerOnce.Do(func() {
        legacyregistry.MustRegister(upstreamSyncTargets)
        legacyregistry.MustRegister(upstreamResourcesSynced)
        legacyregistry.MustRegister(upstreamConflictsResolved)
        legacyregistry.MustRegister(upstreamSyncLatency)
    })
}

// RecordSyncTargets records the number of active sync targets
func RecordSyncTargets(workspace string, count float64) {
    upstreamSyncTargets.WithLabelValues(workspace).Set(count)
}

// RecordResourcesSynced increments the resources synced counter
func RecordResourcesSynced(workspace, cluster, resource string) {
    upstreamResourcesSynced.WithLabelValues(workspace, cluster, resource).Inc()
}

// RecordConflictResolved increments the conflicts resolved counter
func RecordConflictResolved(workspace, strategy string) {
    upstreamConflictsResolved.WithLabelValues(workspace, strategy).Inc()
}

// RecordSyncLatency records sync operation latency
func RecordSyncLatency(workspace, operation string, seconds float64) {
    upstreamSyncLatency.WithLabelValues(workspace, operation).Observe(seconds)
}
```

## Testing Requirements

### Unit Tests (200 lines)

#### File: `pkg/reconciler/workload/syncer/upstream/aggregator_test.go` (100 lines)
- Test status aggregation with different strategies
- Test conflict detection
- Test caching behavior
- Test feature flag integration

#### File: `pkg/reconciler/workload/syncer/upstream/resolver_test.go` (50 lines)
- Test conflict resolution strategies
- Test auto-resolve logic
- Test manual resolution handling

#### File: `pkg/reconciler/workload/syncer/upstream/applier/applier_test.go` (50 lines)
- Test different update types
- Test dry-run mode
- Test batch application
- Test error handling

## Integration Points

1. **With PR1 Interfaces**:
   - Implement StatusAggregator interface
   - Implement ConflictResolver interface
   - Implement UpdateApplier interface

2. **With PR2 Sync Loop**:
   - Processor calls aggregator with collected statuses
   - Syncer uses applier to update KCP

3. **With Metrics System**:
   - Register metrics on startup
   - Record metrics throughout operations

## Feature Flag Usage

Three feature flags control behavior:

```go
// Main upstream sync flag
UpstreamSync

// Aggregation specific flag
UpstreamSyncAggregation

// Conflict resolution flag
UpstreamSyncConflictResolution
```

All implementations check appropriate flags and return noop versions when disabled.

## Line Count Budget

| File | Lines | Purpose |
|------|-------|---------|
| aggregator.go | 150 | Status aggregation logic |
| resolver.go | 100 | Conflict resolution |
| applier/applier.go | 100 | Update application |
| metrics/collector.go | 50 | Metrics collection |
| **Total Implementation** | **400** | |
| aggregator_test.go | 100 | Aggregator tests |
| resolver_test.go | 50 | Resolver tests |
| applier_test.go | 50 | Applier tests |
| **Total Tests** | **200** | |
| **Grand Total** | **600** | Within 800 line limit |

## Implementation Notes

1. **Builds on PR1 & PR2**: Uses interfaces from PR1, works with sync loop from PR2.

2. **Aggregation Strategies**: Implements multiple strategies (newest, oldest, priority).

3. **Conflict Resolution**: Automatic resolution for low/medium severity conflicts.

4. **Server-Side Apply**: Uses SSA for efficient updates.

5. **Metrics Integration**: Prometheus metrics for monitoring.

6. **Caching**: Aggregation results are cached for efficiency.

7. **Feature Flags**: Three levels of feature control.

## Commit Structure

1. First commit: Add aggregator implementation
2. Second commit: Add conflict resolver
3. Third commit: Add update applier
4. Fourth commit: Add metrics collector
5. Fifth commit: Add all tests

## Success Criteria

- [ ] StatusAggregator interface fully implemented
- [ ] ConflictResolver handles all strategies
- [ ] UpdateApplier supports all update types
- [ ] Metrics properly collected
- [ ] Feature flags control all behavior
- [ ] Tests cover main scenarios
- [ ] Under 400 lines of implementation code

## Full Integration Example

When all three PRs are merged:

```go
// In the syncer controller
factory := upstream.NewFactory()

// Create components (PR1 provides factory)
syncer := factory.NewSyncer(config)           // PR2 implements
watcher := factory.NewWatcher(client, "cluster1") // PR2 implements
aggregator := factory.NewAggregator(strategy)     // PR3 implements
resolver := factory.NewResolver(strategy)         // PR3 implements
applier := factory.NewUpdateApplier(kcpClient)   // PR3 implements

// Wire together
processor := factory.NewProcessor(applier)     // PR2 implements
syncer.Start(ctx)

// Process flow:
// 1. Watcher (PR2) observes changes in physical clusters
// 2. Processor (PR2) batches and processes events
// 3. Aggregator (PR3) combines status from multiple clusters
// 4. Resolver (PR3) handles any conflicts
// 5. Applier (PR3) updates KCP with aggregated status
```

This creates a complete upstream synchronization system with clean separation of concerns and progressive enhancement through feature flags.