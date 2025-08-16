# Wave2C PR1: API Abstractions Implementation Instructions

## PR Overview
- **Branch**: `feature/tmc2-impl2/wave2c-01-api-abstractions`
- **Target Size**: 400 lines (excluding generated code)
- **Base Branch**: `main`
- **Dependencies**: None (this is the foundation PR)
- **Purpose**: Define all interfaces, types, and abstractions for upstream sync without any implementation

## Files to Create

### 1. API Types (120 lines total)

#### File: `pkg/apis/workload/v1alpha1/upstream_types.go` (50 lines)
```go
package v1alpha1

import (
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories=kcp
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"

// UpstreamSyncConfig defines configuration for syncing resources from physical clusters to KCP
type UpstreamSyncConfig struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`
    
    Spec   UpstreamSyncSpec   `json:"spec,omitempty"`
    Status UpstreamSyncStatus `json:"status,omitempty"`
}

// UpstreamSyncSpec defines the desired state of upstream synchronization
type UpstreamSyncSpec struct {
    // SyncTargets specifies which physical clusters to sync from
    SyncTargets []SyncTargetReference `json:"syncTargets"`
    
    // ResourceSelectors defines which resources to sync
    ResourceSelectors []ResourceSelector `json:"resourceSelectors"`
    
    // SyncInterval defines how often to sync (default: 30s)
    // +kubebuilder:default="30s"
    SyncInterval metav1.Duration `json:"syncInterval,omitempty"`
    
    // ConflictStrategy defines how to handle conflicts between clusters
    // +kubebuilder:default=UseNewest
    // +kubebuilder:validation:Enum=UseNewest;UseOldest;Manual;Priority
    ConflictStrategy ConflictStrategy `json:"conflictStrategy,omitempty"`
}

// SyncTargetReference identifies a SyncTarget to monitor
type SyncTargetReference struct {
    // Name of the SyncTarget
    Name string `json:"name"`
    
    // Workspace containing the SyncTarget (optional, defaults to current)
    Workspace string `json:"workspace,omitempty"`
}

// ResourceSelector identifies resources to sync
type ResourceSelector struct {
    // APIGroup to sync (e.g., "apps")
    APIGroup string `json:"apiGroup"`
    
    // Resource type (e.g., "deployments")
    Resource string `json:"resource"`
    
    // Namespace to sync from (optional, empty means all)
    Namespace string `json:"namespace,omitempty"`
    
    // LabelSelector for filtering resources
    LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`
}

// ConflictStrategy defines how conflicts are resolved
type ConflictStrategy string

const (
    ConflictStrategyUseNewest ConflictStrategy = "UseNewest"
    ConflictStrategyUseOldest ConflictStrategy = "UseOldest"
    ConflictStrategyManual    ConflictStrategy = "Manual"
    ConflictStrategyPriority  ConflictStrategy = "Priority"
)

// UpstreamSyncStatus defines the observed state
type UpstreamSyncStatus struct {
    // ObservedGeneration tracks spec generation
    ObservedGeneration int64 `json:"observedGeneration,omitempty"`
    
    // Conditions represent the latest available observations
    Conditions []metav1.Condition `json:"conditions,omitempty"`
    
    // LastSyncTime records when sync last occurred
    LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`
    
    // SyncedResources count of resources synced
    SyncedResources int32 `json:"syncedResources,omitempty"`
}

// +kubebuilder:object:root=true

// UpstreamSyncConfigList contains a list of UpstreamSyncConfig
type UpstreamSyncConfigList struct {
    metav1.TypeMeta `json:",inline"`
    metav1.ListMeta `json:"metadata,omitempty"`
    Items           []UpstreamSyncConfig `json:"items"`
}
```

#### File: `pkg/apis/workload/v1alpha1/upstream_defaults.go` (30 lines)
```go
package v1alpha1

import (
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "time"
)

// SetDefaults_UpstreamSyncConfig sets defaults for UpstreamSyncConfig
func SetDefaults_UpstreamSyncConfig(obj *UpstreamSyncConfig) {
    if obj.Spec.SyncInterval.Duration == 0 {
        obj.Spec.SyncInterval = metav1.Duration{Duration: 30 * time.Second}
    }
    
    if obj.Spec.ConflictStrategy == "" {
        obj.Spec.ConflictStrategy = ConflictStrategyUseNewest
    }
    
    // Ensure status conditions are initialized
    if obj.Status.Conditions == nil {
        obj.Status.Conditions = []metav1.Condition{}
    }
}
```

#### File: `pkg/apis/workload/v1alpha1/upstream_validation.go` (40 lines)
```go
package v1alpha1

import (
    "fmt"
    "time"
    
    "k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateUpstreamSyncConfig validates an UpstreamSyncConfig
func ValidateUpstreamSyncConfig(config *UpstreamSyncConfig) field.ErrorList {
    allErrs := field.ErrorList{}
    specPath := field.NewPath("spec")
    
    // Validate SyncTargets
    if len(config.Spec.SyncTargets) == 0 {
        allErrs = append(allErrs, field.Required(specPath.Child("syncTargets"), 
            "at least one sync target must be specified"))
    }
    
    // Validate ResourceSelectors
    if len(config.Spec.ResourceSelectors) == 0 {
        allErrs = append(allErrs, field.Required(specPath.Child("resourceSelectors"), 
            "at least one resource selector must be specified"))
    }
    
    // Validate SyncInterval
    if config.Spec.SyncInterval.Duration < 10*time.Second {
        allErrs = append(allErrs, field.Invalid(specPath.Child("syncInterval"), 
            config.Spec.SyncInterval, "sync interval must be at least 10s"))
    }
    
    // Validate ConflictStrategy
    validStrategies := map[ConflictStrategy]bool{
        ConflictStrategyUseNewest: true,
        ConflictStrategyUseOldest: true,
        ConflictStrategyManual:    true,
        ConflictStrategyPriority:  true,
    }
    
    if !validStrategies[config.Spec.ConflictStrategy] {
        allErrs = append(allErrs, field.Invalid(specPath.Child("conflictStrategy"),
            config.Spec.ConflictStrategy, 
            fmt.Sprintf("must be one of: %v", validStrategies)))
    }
    
    return allErrs
}
```

### 2. Core Interfaces (150 lines)

#### File: `pkg/reconciler/workload/syncer/upstream/interfaces.go` (150 lines)
```go
package upstream

import (
    "context"
    "time"
    
    "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
    "k8s.io/apimachinery/pkg/labels"
    "k8s.io/apimachinery/pkg/runtime/schema"
    "k8s.io/client-go/discovery"
    "k8s.io/client-go/dynamic"
    "k8s.io/client-go/rest"
    
    workloadv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"
    "github.com/kcp-dev/kcp/pkg/apis/core"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Syncer is the main interface for upstream synchronization
type Syncer interface {
    // Start begins the sync process with the given context
    Start(ctx context.Context) error
    
    // Stop gracefully stops syncing
    Stop()
    
    // ReconcileSyncTarget handles synchronization for a specific target
    ReconcileSyncTarget(ctx context.Context, target *workloadv1alpha1.SyncTarget) error
    
    // GetMetrics returns current synchronization metrics
    GetMetrics() Metrics
    
    // IsReady returns true if the syncer is ready to process
    IsReady() bool
}

// ResourceWatcher watches resources in physical clusters
type ResourceWatcher interface {
    // Watch starts watching resources and returns event channel
    Watch(ctx context.Context, gvr schema.GroupVersionResource, namespace string) (<-chan Event, error)
    
    // List returns current state of resources
    List(ctx context.Context, gvr schema.GroupVersionResource, namespace string) ([]*unstructured.Unstructured, error)
    
    // Stop stops all watches
    Stop()
    
    // IsWatching returns true if actively watching the given GVR
    IsWatching(gvr schema.GroupVersionResource) bool
}

// EventProcessor processes events from physical clusters
type EventProcessor interface {
    // ProcessEvent handles a single event
    ProcessEvent(ctx context.Context, event Event) error
    
    // BatchProcess handles multiple events efficiently
    BatchProcess(ctx context.Context, events []Event) error
    
    // SetRateLimiter configures rate limiting for event processing
    SetRateLimiter(limiter RateLimiter)
    
    // GetQueueLength returns current event queue length
    GetQueueLength() int
}

// StatusAggregator aggregates status from multiple clusters
type StatusAggregator interface {
    // AggregateStatus combines status from multiple sources
    AggregateStatus(ctx context.Context, resources []ResourceStatus) (*AggregatedStatus, error)
    
    // ResolveConflicts handles conflicting status information
    ResolveConflicts(ctx context.Context, conflicts []Conflict) (*Resolution, error)
    
    // SetStrategy configures the conflict resolution strategy
    SetStrategy(strategy workloadv1alpha1.ConflictStrategy)
    
    // GetLastAggregation returns the last aggregation result
    GetLastAggregation() *AggregatedStatus
}

// CacheManager manages local cache of remote cluster state
type CacheManager interface {
    // Store stores resource state with TTL
    Store(key string, resource *unstructured.Unstructured, ttl time.Duration) error
    
    // Get retrieves resource from cache
    Get(key string) (*unstructured.Unstructured, bool, error)
    
    // Delete removes resource from cache
    Delete(key string) error
    
    // List lists cached resources matching selector
    List(selector labels.Selector) ([]*unstructured.Unstructured, error)
    
    // Flush clears all cached data
    Flush() error
    
    // GetMetrics returns cache metrics
    GetMetrics() CacheMetrics
}

// UpdateApplier applies updates to KCP workspace
type UpdateApplier interface {
    // Apply applies a single update to KCP
    Apply(ctx context.Context, update *Update) error
    
    // ApplyBatch applies multiple updates in a batch
    ApplyBatch(ctx context.Context, updates []*Update) error
    
    // SetDryRun enables dry-run mode
    SetDryRun(enabled bool)
    
    // GetAppliedCount returns number of successful applies
    GetAppliedCount() int64
}

// PhysicalClusterClient manages connections to physical clusters
type PhysicalClusterClient interface {
    // Connect establishes connection to physical cluster
    Connect(ctx context.Context, config *rest.Config) error
    
    // Dynamic returns dynamic client for the cluster
    Dynamic() dynamic.Interface
    
    // Discovery returns discovery client for the cluster
    Discovery() discovery.DiscoveryInterface
    
    // IsHealthy checks if connection is healthy
    IsHealthy(ctx context.Context) bool
    
    // GetClusterID returns unique identifier for this cluster
    GetClusterID() string
    
    // Close closes the connection
    Close() error
}

// ConflictResolver handles resource conflicts between clusters
type ConflictResolver interface {
    // Resolve attempts to resolve a conflict
    Resolve(ctx context.Context, conflict Conflict) (*Resolution, error)
    
    // SetStrategy sets the resolution strategy
    SetStrategy(strategy workloadv1alpha1.ConflictStrategy)
    
    // CanAutoResolve checks if conflict can be auto-resolved
    CanAutoResolve(conflict Conflict) bool
}

// RateLimiter provides rate limiting for operations
type RateLimiter interface {
    // Allow returns true if operation is allowed
    Allow() bool
    
    // Wait blocks until operation is allowed
    Wait(ctx context.Context) error
    
    // Reset resets the rate limiter
    Reset()
}
```

### 3. Type Definitions (50 lines)

#### File: `pkg/reconciler/workload/syncer/upstream/types.go` (50 lines)
```go
package upstream

import (
    "time"
    
    "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "github.com/kcp-dev/kcp/pkg/apis/core"
)

// EventType represents the type of resource event
type EventType string

const (
    EventTypeCreate EventType = "Create"
    EventTypeUpdate EventType = "Update"
    EventTypeDelete EventType = "Delete"
)

// Event represents a resource change event from a physical cluster
type Event struct {
    Type        EventType
    Resource    *unstructured.Unstructured
    OldResource *unstructured.Unstructured // For updates
    Timestamp   time.Time
    Source      ClusterSource
}

// ClusterSource identifies the source cluster
type ClusterSource struct {
    Name      string
    Workspace core.LogicalCluster
    Region    string
}

// ResourceStatus represents resource status from a single cluster
type ResourceStatus struct {
    ClusterName string
    Resource    *unstructured.Unstructured
    Conditions  []metav1.Condition
    LastUpdated time.Time
    Health      HealthStatus
}

// HealthStatus represents resource health
type HealthStatus string

const (
    HealthStatusHealthy   HealthStatus = "Healthy"
    HealthStatusDegraded  HealthStatus = "Degraded"
    HealthStatusUnhealthy HealthStatus = "Unhealthy"
    HealthStatusUnknown   HealthStatus = "Unknown"
)

// Conflict represents a status conflict between clusters
type Conflict struct {
    ResourceKey string
    Statuses    []ResourceStatus
    Type        ConflictType
    Severity    ConflictSeverity
}

// ConflictType categorizes the conflict
type ConflictType string

const (
    ConflictTypeStatus     ConflictType = "Status"
    ConflictTypeGeneration ConflictType = "Generation"
    ConflictTypeContent    ConflictType = "Content"
)

// ConflictSeverity indicates conflict severity
type ConflictSeverity string

const (
    ConflictSeverityLow    ConflictSeverity = "Low"
    ConflictSeverityMedium ConflictSeverity = "Medium"
    ConflictSeverityHigh   ConflictSeverity = "High"
)

// AggregatedStatus represents combined status from all clusters
type AggregatedStatus struct {
    ResourceKey      string
    CombinedStatus   *unstructured.Unstructured
    SourceStatuses   []ResourceStatus
    AggregationTime  time.Time
    ConflictsResolved int
}

// Resolution represents a conflict resolution
type Resolution struct {
    Conflict       Conflict
    ResolvedStatus *ResourceStatus
    Strategy       string
    Timestamp      time.Time
}

// Update represents an update to apply to KCP
type Update struct {
    Type      UpdateType
    Resource  *unstructured.Unstructured
    Workspace core.LogicalCluster
    Strategy  ApplyStrategy
}

// UpdateType categorizes the update
type UpdateType string

const (
    UpdateTypeCreate UpdateType = "Create"
    UpdateTypeUpdate UpdateType = "Update"
    UpdateTypeDelete UpdateType = "Delete"
    UpdateTypeStatus UpdateType = "Status"
)

// ApplyStrategy defines how updates are applied
type ApplyStrategy string

const (
    ApplyStrategyReplace     ApplyStrategy = "Replace"
    ApplyStrategyMerge       ApplyStrategy = "Merge"
    ApplyStrategyServerSide  ApplyStrategy = "ServerSide"
)

// Metrics tracks synchronization metrics
type Metrics struct {
    SyncTargetsActive   int
    ResourcesSynced     int64
    EventsProcessed     int64
    ConflictsResolved   int64
    ErrorCount          int64
    LastSyncTime        time.Time
    SyncLatency         time.Duration
    QueueDepth          int
}

// CacheMetrics tracks cache performance
type CacheMetrics struct {
    Hits       int64
    Misses     int64
    Evictions  int64
    Size       int
    ErrorCount int64
}
```

### 4. Factory Pattern (80 lines)

#### File: `pkg/reconciler/workload/syncer/upstream/factory.go` (80 lines)
```go
package upstream

import (
    "context"
    "fmt"
    
    "k8s.io/client-go/dynamic"
    "k8s.io/client-go/rest"
    "k8s.io/klog/v2"
    
    kcpfeatures "github.com/kcp-dev/kcp/pkg/features"
    "k8s.io/apimachinery/pkg/util/runtime"
    utilfeature "k8s.io/apiserver/pkg/util/feature"
    workloadv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"
)

// Factory creates upstream sync components
type Factory interface {
    // NewSyncer creates a new syncer instance
    NewSyncer(config *Config) (Syncer, error)
    
    // NewWatcher creates a resource watcher for a cluster
    NewWatcher(client dynamic.Interface, clusterName string) ResourceWatcher
    
    // NewProcessor creates an event processor
    NewProcessor(applier UpdateApplier) EventProcessor
    
    // NewAggregator creates a status aggregator
    NewAggregator(strategy workloadv1alpha1.ConflictStrategy) StatusAggregator
    
    // NewCacheManager creates a cache manager
    NewCacheManager(maxSize int) CacheManager
    
    // NewUpdateApplier creates an update applier
    NewUpdateApplier(client dynamic.Interface) UpdateApplier
    
    // NewPhysicalClient creates a physical cluster client
    NewPhysicalClient(config *rest.Config, clusterID string) PhysicalClusterClient
    
    // NewConflictResolver creates a conflict resolver
    NewConflictResolver(strategy workloadv1alpha1.ConflictStrategy) ConflictResolver
}

// Config holds configuration for creating sync components
type Config struct {
    // ClusterName identifies the KCP workspace
    ClusterName string
    
    // Namespace to operate in
    Namespace string
    
    // SyncInterval for periodic sync
    SyncInterval time.Duration
    
    // MaxRetries for failed operations
    MaxRetries int
    
    // CacheSize for local cache
    CacheSize int
    
    // EnableMetrics enables metrics collection
    EnableMetrics bool
}

// defaultFactory is the default implementation of Factory
type defaultFactory struct{}

// NewFactory creates a new factory instance
func NewFactory() Factory {
    if !utilfeature.DefaultMutableFeatureGate.Enabled(kcpfeatures.UpstreamSync) {
        klog.V(2).Info("UpstreamSync feature gate is disabled")
        return &noopFactory{}
    }
    return &defaultFactory{}
}

// Implementation stubs for defaultFactory - actual implementation in PR2/PR3
func (f *defaultFactory) NewSyncer(config *Config) (Syncer, error) {
    runtime.HandleError(fmt.Errorf("syncer implementation pending in wave2c-02"))
    return nil, fmt.Errorf("not implemented: awaiting wave2c-02-core-sync")
}

func (f *defaultFactory) NewWatcher(client dynamic.Interface, clusterName string) ResourceWatcher {
    runtime.HandleError(fmt.Errorf("watcher implementation pending in wave2c-02"))
    return nil
}

func (f *defaultFactory) NewProcessor(applier UpdateApplier) EventProcessor {
    runtime.HandleError(fmt.Errorf("processor implementation pending in wave2c-02"))
    return nil
}

func (f *defaultFactory) NewAggregator(strategy workloadv1alpha1.ConflictStrategy) StatusAggregator {
    runtime.HandleError(fmt.Errorf("aggregator implementation pending in wave2c-03"))
    return nil
}

func (f *defaultFactory) NewCacheManager(maxSize int) CacheManager {
    runtime.HandleError(fmt.Errorf("cache manager implementation pending in wave2c-02"))
    return nil
}

func (f *defaultFactory) NewUpdateApplier(client dynamic.Interface) UpdateApplier {
    runtime.HandleError(fmt.Errorf("update applier implementation pending in wave2c-03"))
    return nil
}

func (f *defaultFactory) NewPhysicalClient(config *rest.Config, clusterID string) PhysicalClusterClient {
    runtime.HandleError(fmt.Errorf("physical client implementation pending in wave2c-02"))
    return nil
}

func (f *defaultFactory) NewConflictResolver(strategy workloadv1alpha1.ConflictStrategy) ConflictResolver {
    runtime.HandleError(fmt.Errorf("conflict resolver implementation pending in wave2c-03"))
    return nil
}

// noopFactory returns when feature gate is disabled
type noopFactory struct{}

func (f *noopFactory) NewSyncer(config *Config) (Syncer, error) {
    return &noopSyncer{}, nil
}

// ... implement other noop methods similarly ...

// noopSyncer is a no-op implementation when feature is disabled
type noopSyncer struct{}

func (s *noopSyncer) Start(ctx context.Context) error { return nil }
func (s *noopSyncer) Stop() {}
func (s *noopSyncer) ReconcileSyncTarget(ctx context.Context, target *workloadv1alpha1.SyncTarget) error {
    return nil
}
func (s *noopSyncer) GetMetrics() Metrics { return Metrics{} }
func (s *noopSyncer) IsReady() bool { return false }
```

### 5. Feature Flags (50 lines)

#### File: `pkg/features/kcp_features.go` (additions - 50 lines)
```go
// Add to existing feature gates file

const (
    // UpstreamSync enables synchronization from physical clusters to KCP
    // owner: @jessesanford
    // alpha: v0.1
    UpstreamSync featuregate.Feature = "UpstreamSync"
    
    // UpstreamSyncLoop enables the core synchronization loop
    // owner: @jessesanford  
    // alpha: v0.1
    UpstreamSyncLoop featuregate.Feature = "UpstreamSyncLoop"
    
    // UpstreamSyncAggregation enables status aggregation across clusters
    // owner: @jessesanford
    // alpha: v0.1
    UpstreamSyncAggregation featuregate.Feature = "UpstreamSyncAggregation"
    
    // UpstreamSyncConflictResolution enables automatic conflict resolution
    // owner: @jessesanford
    // alpha: v0.1
    UpstreamSyncConflictResolution featuregate.Feature = "UpstreamSyncConflictResolution"
)

// Add to defaultKCPFeatureGates
func init() {
    runtime.Must(utilfeature.DefaultMutableFeatureGate.Add(defaultKCPFeatureGates))
}

var defaultKCPFeatureGates = map[featuregate.Feature]featuregate.FeatureSpec{
    // ... existing features ...
    
    UpstreamSync: {Default: false, PreRelease: featuregate.Alpha},
    UpstreamSyncLoop: {Default: false, PreRelease: featuregate.Alpha},
    UpstreamSyncAggregation: {Default: false, PreRelease: featuregate.Alpha},
    UpstreamSyncConflictResolution: {Default: false, PreRelease: featuregate.Alpha},
}

// Helper function to check if upstream sync is fully enabled
func IsUpstreamSyncFullyEnabled() bool {
    return utilfeature.DefaultMutableFeatureGate.Enabled(UpstreamSync) &&
           utilfeature.DefaultMutableFeatureGate.Enabled(UpstreamSyncLoop) &&
           utilfeature.DefaultMutableFeatureGate.Enabled(UpstreamSyncAggregation)
}
```

## Testing Requirements

### Unit Tests (200 lines)

#### File: `pkg/reconciler/workload/syncer/upstream/factory_test.go` (100 lines)
- Test factory creation with feature flags enabled/disabled
- Test component creation methods return appropriate errors
- Test noop implementations when feature disabled
- Verify factory interface compliance

#### File: `pkg/apis/workload/v1alpha1/upstream_validation_test.go` (100 lines)
- Test validation of UpstreamSyncConfig
- Test default values are applied correctly
- Test invalid configurations are rejected
- Test edge cases in validation logic

## Integration Points

1. **With existing SyncTarget controller**:
   - Import types in `pkg/reconciler/workload/synctarget/synctarget_controller.go`
   - Check feature flag before creating upstream syncer

2. **With APIExport/APIBinding**:
   - Ensure workspace isolation in all interfaces
   - Pass logical cluster information through interfaces

3. **With feature flag system**:
   - Import from `pkg/features/kcp_features.go`
   - Check flags before instantiating components

## Feature Flag Usage

All components must check feature flags:

```go
// Example usage in controller
if utilfeature.DefaultMutableFeatureGate.Enabled(kcpfeatures.UpstreamSync) {
    factory := upstream.NewFactory()
    syncer, err := factory.NewSyncer(config)
    if err != nil {
        return err
    }
    // Use syncer...
}
```

## Line Count Budget

| File | Lines | Purpose |
|------|-------|---------|
| upstream_types.go | 50 | API type definitions |
| upstream_defaults.go | 30 | Default values |
| upstream_validation.go | 40 | Validation logic |
| interfaces.go | 150 | Core interfaces |
| types.go | 50 | Supporting types |
| factory.go | 80 | Factory pattern |
| **Total Implementation** | **400** | |
| factory_test.go | 100 | Factory tests |
| upstream_validation_test.go | 100 | Validation tests |
| **Total Tests** | **200** | |
| **Grand Total** | **600** | Within 800 line limit |

## Implementation Notes

1. **No Implementation Code**: This PR contains ONLY interfaces, types, and stubs. No actual synchronization logic.

2. **Feature Flag Integration**: Every component checks feature flags and returns noop implementations when disabled.

3. **Workspace Awareness**: All interfaces include workspace/logical cluster parameters for proper KCP integration.

4. **Error Handling**: Factory methods return errors indicating implementation is pending in future PRs.

5. **Generated Code**: Remember to run:
   ```bash
   make generate
   make update-codegen
   ```
   This will generate deepcopy methods and CRD manifests (not counted in line limit).

6. **Compilation**: This PR must compile independently. Use interface{} or stub returns where needed.

7. **Documentation**: Every exported type and method must have godoc comments.

8. **Testing**: Focus tests on validation, defaults, and factory behavior. Cannot test implementations yet.

## Commit Structure

1. First commit: Add API types and validation
2. Second commit: Add core interfaces
3. Third commit: Add factory pattern and feature flags
4. Fourth commit: Add tests
5. Fifth commit: Run code generation

## Success Criteria

- [ ] All interfaces are well-documented
- [ ] Feature flags properly integrated
- [ ] Validation logic is comprehensive
- [ ] Factory pattern established
- [ ] Tests pass
- [ ] Code compiles independently
- [ ] Under 400 lines of implementation code
- [ ] Generated code is committed