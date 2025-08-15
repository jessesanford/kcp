# Implementation Instructions: KCP Discovery Provider Implementation

## Overview
- **Branch**: feature/tmc-phase4-vw-07-discovery-impl
- **Purpose**: Implement KCP discovery provider that integrates with virtual workspace framework and provides resource discovery for workspace-aware APIs
- **Target Lines**: 450
- **Dependencies**: 
  - vw-01-api-contracts (interfaces and contracts)
  - vw-04-discovery-contracts (discovery interfaces)
- **Estimated Time**: 3 days

## Files to Create

### 1. pkg/virtual/discovery/provider.go (100 lines)
**Purpose**: Main discovery provider implementation that integrates with KCP APIExport system

**Key Structs/Interfaces**:
```go
package discovery

import (
    "context"
    "sync"
    "time"
    
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/runtime/schema"
    "k8s.io/client-go/discovery"
    "k8s.io/client-go/tools/cache"
    
    kcpclient "github.com/kcp-dev/kcp/sdk/client/clientset/versioned"
    kcpinformers "github.com/kcp-dev/kcp/sdk/client/informers/externalversions"
    apisv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/apis/v1alpha1"
    
    "github.com/kcp-dev/kcp/pkg/virtual/interfaces"
    "github.com/kcp-dev/kcp/pkg/virtual/contracts"
)

// KCPDiscoveryProvider implements ResourceDiscoveryInterface for KCP environments
type KCPDiscoveryProvider struct {
    // kcpClient provides access to KCP APIs
    kcpClient kcpclient.ClusterInterface
    
    // informerFactory provides shared informers for KCP resources
    informerFactory kcpinformers.SharedInformerFactory
    
    // apiExportInformer monitors APIExport changes
    apiExportInformer cache.SharedIndexInformer
    
    // cache stores discovered resources per workspace
    cache interfaces.DiscoveryCache
    
    // workspace is the logical cluster this provider serves
    workspace string
    
    // mutex protects concurrent access
    mutex sync.RWMutex
    
    // started indicates if the provider has been started
    started bool
    
    // stopCh signals shutdown
    stopCh <-chan struct{}
}

// NewKCPDiscoveryProvider creates a new KCP discovery provider
func NewKCPDiscoveryProvider(
    kcpClient kcpclient.ClusterInterface,
    informerFactory kcpinformers.SharedInformerFactory,
    workspace string,
) (*KCPDiscoveryProvider, error) {
    // Implementation details
}

// Start initializes the discovery provider and begins monitoring
func (p *KCPDiscoveryProvider) Start(ctx context.Context) error {
    // Implementation details
}

// Discover returns available resources in the specified workspace
func (p *KCPDiscoveryProvider) Discover(ctx context.Context, workspace string) ([]interfaces.ResourceInfo, error) {
    // Implementation details
}

// GetOpenAPISchema returns the OpenAPI schema for workspace resources
func (p *KCPDiscoveryProvider) GetOpenAPISchema(ctx context.Context, workspace string) ([]byte, error) {
    // Implementation details
}

// Watch monitors for resource changes in the workspace
func (p *KCPDiscoveryProvider) Watch(ctx context.Context, workspace string) (<-chan interfaces.DiscoveryEvent, error) {
    // Implementation details
}

// IsResourceAvailable checks if a specific resource is available
func (p *KCPDiscoveryProvider) IsResourceAvailable(ctx context.Context, workspace string, gvr schema.GroupVersionResource) (bool, error) {
    // Implementation details
}
```

### 2. pkg/virtual/discovery/cache.go (80 lines)
**Purpose**: Implements discovery cache for performance optimization with workspace-aware caching

**Key Structs/Interfaces**:
```go
package discovery

import (
    "sync"
    "time"
    
    "github.com/kcp-dev/kcp/pkg/virtual/interfaces"
)

// MemoryDiscoveryCache provides in-memory caching for discovered resources
type MemoryDiscoveryCache struct {
    // entries stores cached discovery data per workspace
    entries map[string]*cacheEntry
    
    // defaultTTL is the default cache expiration time
    defaultTTL time.Duration
    
    // mutex protects concurrent access
    mutex sync.RWMutex
    
    // cleanupInterval determines how often to run cache cleanup
    cleanupInterval time.Duration
    
    // stopCh signals shutdown for cleanup goroutine
    stopCh chan struct{}
}

// cacheEntry represents a cached discovery result
type cacheEntry struct {
    // resources are the cached resource information
    resources []interfaces.ResourceInfo
    
    // timestamp when this entry was created
    timestamp time.Time
    
    // ttl is the time-to-live for this entry
    ttl time.Duration
}

// NewMemoryDiscoveryCache creates a new memory-based discovery cache
func NewMemoryDiscoveryCache(defaultTTL, cleanupInterval time.Duration) *MemoryDiscoveryCache {
    // Implementation details
}

// Start begins cache cleanup operations
func (c *MemoryDiscoveryCache) Start() {
    // Implementation details
}

// Stop terminates cache cleanup operations
func (c *MemoryDiscoveryCache) Stop() {
    // Implementation details
}

// GetResources retrieves cached resources for a workspace
func (c *MemoryDiscoveryCache) GetResources(workspace string) ([]interfaces.ResourceInfo, bool) {
    // Implementation details
}

// SetResources caches resources for a workspace
func (c *MemoryDiscoveryCache) SetResources(workspace string, resources []interfaces.ResourceInfo, ttl int64) {
    // Implementation details
}

// InvalidateWorkspace removes cached data for a workspace
func (c *MemoryDiscoveryCache) InvalidateWorkspace(workspace string) {
    // Implementation details
}

// Clear removes all cached data
func (c *MemoryDiscoveryCache) Clear() {
    // Implementation details
}
```

### 3. pkg/virtual/discovery/watcher.go (90 lines)
**Purpose**: Implements resource change watching with KCP APIExport integration

**Key Structs/Interfaces**:
```go
package discovery

import (
    "context"
    "sync"
    
    "k8s.io/client-go/tools/cache"
    "k8s.io/klog/v2"
    
    apisv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/apis/v1alpha1"
    
    "github.com/kcp-dev/kcp/pkg/virtual/interfaces"
)

// ResourceWatcher monitors APIExport changes and generates discovery events
type ResourceWatcher struct {
    // provider is the parent discovery provider
    provider *KCPDiscoveryProvider
    
    // eventCh broadcasts discovery events
    eventCh chan interfaces.DiscoveryEvent
    
    // subscribers tracks active event subscribers
    subscribers map[string]chan interfaces.DiscoveryEvent
    
    // mutex protects concurrent access to subscribers
    mutex sync.RWMutex
    
    // stopCh signals shutdown
    stopCh <-chan struct{}
}

// NewResourceWatcher creates a new resource watcher
func NewResourceWatcher(provider *KCPDiscoveryProvider, stopCh <-chan struct{}) *ResourceWatcher {
    // Implementation details
}

// Start begins watching for resource changes
func (w *ResourceWatcher) Start(ctx context.Context) error {
    // Implementation details
}

// Subscribe creates a new event subscription for a workspace
func (w *ResourceWatcher) Subscribe(workspace string) <-chan interfaces.DiscoveryEvent {
    // Implementation details
}

// Unsubscribe removes an event subscription
func (w *ResourceWatcher) Unsubscribe(workspace string) {
    // Implementation details
}

// handleAPIExportAdd processes new APIExport additions
func (w *ResourceWatcher) handleAPIExportAdd(obj interface{}) {
    // Implementation details
}

// handleAPIExportUpdate processes APIExport updates
func (w *ResourceWatcher) handleAPIExportUpdate(oldObj, newObj interface{}) {
    // Implementation details
}

// handleAPIExportDelete processes APIExport deletions
func (w *ResourceWatcher) handleAPIExportDelete(obj interface{}) {
    // Implementation details
}

// broadcastEvent sends an event to all subscribers
func (w *ResourceWatcher) broadcastEvent(event interfaces.DiscoveryEvent) {
    // Implementation details
}

// extractWorkspaceFromAPIExport determines the workspace for an APIExport
func (w *ResourceWatcher) extractWorkspaceFromAPIExport(apiExport *apisv1alpha1.APIExport) string {
    // Implementation details
}
```

### 4. pkg/virtual/discovery/converter.go (70 lines)
**Purpose**: Converts KCP APIExport data to discovery resource information

**Key Structs/Interfaces**:
```go
package discovery

import (
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/runtime/schema"
    
    apisv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/apis/v1alpha1"
    
    "github.com/kcp-dev/kcp/pkg/virtual/interfaces"
)

// APIExportConverter converts APIExport data to ResourceInfo
type APIExportConverter struct {
    // workspace identifies the logical cluster for conversions
    workspace string
}

// NewAPIExportConverter creates a new APIExport converter
func NewAPIExportConverter(workspace string) *APIExportConverter {
    // Implementation details
}

// ConvertAPIExport converts an APIExport to ResourceInfo array
func (c *APIExportConverter) ConvertAPIExport(apiExport *apisv1alpha1.APIExport) ([]interfaces.ResourceInfo, error) {
    // Implementation details
}

// convertAPIResourceSchema converts an APIResourceSchema to ResourceInfo
func (c *APIExportConverter) convertAPIResourceSchema(
    schema *apisv1alpha1.APIResourceSchema,
    apiExport *apisv1alpha1.APIExport,
) (interfaces.ResourceInfo, error) {
    // Implementation details
}

// extractResourceInfo extracts resource information from schema
func (c *APIExportConverter) extractResourceInfo(schema *apisv1alpha1.APIResourceSchema) metav1.APIResource {
    // Implementation details
}

// isWorkspaceScoped determines if a resource is workspace-scoped
func (c *APIExportConverter) isWorkspaceScoped(schema *apisv1alpha1.APIResourceSchema) bool {
    // Implementation details
}

// extractOpenAPISchema extracts OpenAPI schema from resource schema
func (c *APIExportConverter) extractOpenAPISchema(schema *apisv1alpha1.APIResourceSchema) ([]byte, error) {
    // Implementation details
}

// buildGroupVersionResource constructs GVR from schema
func (c *APIExportConverter) buildGroupVersionResource(schema *apisv1alpha1.APIResourceSchema) schema.GroupVersionResource {
    // Implementation details
}
```

### 5. pkg/virtual/discovery/integration.go (60 lines)
**Purpose**: Integration utilities for KCP workspace patterns and logical clusters

**Key Structs/Interfaces**:
```go
package discovery

import (
    "context"
    "fmt"
    
    "github.com/kcp-dev/logicalcluster/v3"
    kcpfeatures "github.com/kcp-dev/kcp/pkg/features"
    
    apisv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/apis/v1alpha1"
)

// WorkspaceIntegrator provides KCP workspace integration utilities
type WorkspaceIntegrator struct {
    // workspace is the logical cluster for this integrator
    workspace logicalcluster.Name
}

// NewWorkspaceIntegrator creates a new workspace integrator
func NewWorkspaceIntegrator(workspace logicalcluster.Name) *WorkspaceIntegrator {
    // Implementation details
}

// FilterAPIExportsForWorkspace filters APIExports relevant to the workspace
func (i *WorkspaceIntegrator) FilterAPIExportsForWorkspace(
    ctx context.Context,
    apiExports []*apisv1alpha1.APIExport,
) ([]*apisv1alpha1.APIExport, error) {
    // Implementation details
}

// IsAPIExportAvailable checks if an APIExport is available in the workspace
func (i *WorkspaceIntegrator) IsAPIExportAvailable(
    ctx context.Context,
    apiExport *apisv1alpha1.APIExport,
) (bool, error) {
    // Implementation details
}

// ResolveLogicalCluster resolves workspace references to logical clusters
func (i *WorkspaceIntegrator) ResolveLogicalCluster(workspace string) (logicalcluster.Name, error) {
    // Implementation details
}

// ValidateWorkspaceAccess validates that discovery is allowed for the workspace
func (i *WorkspaceIntegrator) ValidateWorkspaceAccess(ctx context.Context, workspace string) error {
    // Implementation details
}

// ExtractFeatureGates extracts relevant feature gates for discovery
func (i *WorkspaceIntegrator) ExtractFeatureGates() map[string]bool {
    // Implementation details
}
```

### 6. pkg/virtual/discovery/metrics.go (50 lines)
**Purpose**: Metrics collection for discovery operations

**Key Structs/Interfaces**:
```go
package discovery

import (
    "time"
    
    "github.com/prometheus/client_golang/prometheus"
    "k8s.io/component-base/metrics"
)

var (
    // discoveryRequestsTotal counts total discovery requests
    discoveryRequestsTotal = metrics.NewCounterVec(
        &metrics.CounterOpts{
            Name: "kcp_virtual_discovery_requests_total",
            Help: "Total number of discovery requests handled",
        },
        []string{"workspace", "result"},
    )
    
    // discoveryRequestDuration measures discovery request duration
    discoveryRequestDuration = metrics.NewHistogramVec(
        &metrics.HistogramOpts{
            Name: "kcp_virtual_discovery_request_duration_seconds",
            Help: "Duration of discovery requests in seconds",
            Buckets: prometheus.ExponentialBuckets(0.001, 2, 15),
        },
        []string{"workspace", "operation"},
    )
    
    // discoveryCacheHits counts cache hits/misses
    discoveryCacheHits = metrics.NewCounterVec(
        &metrics.CounterOpts{
            Name: "kcp_virtual_discovery_cache_hits_total",
            Help: "Total number of discovery cache hits",
        },
        []string{"workspace", "hit_type"},
    )
    
    // discoveryWatchersActive tracks active watchers
    discoveryWatchersActive = metrics.NewGaugeVec(
        &metrics.GaugeOpts{
            Name: "kcp_virtual_discovery_watchers_active",
            Help: "Number of active discovery watchers",
        },
        []string{"workspace"},
    )
)

// init registers metrics
func init() {
    metrics.MustRegister(
        discoveryRequestsTotal,
        discoveryRequestDuration,
        discoveryCacheHits,
        discoveryWatchersActive,
    )
}

// RecordDiscoveryRequest records metrics for a discovery request
func RecordDiscoveryRequest(workspace, operation string, duration time.Duration, err error) {
    // Implementation details
}

// RecordCacheHit records a cache hit or miss
func RecordCacheHit(workspace string, hit bool) {
    // Implementation details
}

// UpdateActiveWatchers updates the active watcher count
func UpdateActiveWatchers(workspace string, delta int) {
    // Implementation details
}
```

## Implementation Steps

1. **Create discovery package structure**:
   - Create `pkg/virtual/discovery/` directory
   - Implement core provider with KCP integration

2. **Implement caching layer**:
   - Create memory-based discovery cache
   - Add cache invalidation strategies
   - Implement TTL management

3. **Add resource watching**:
   - Implement APIExport change monitoring
   - Create event broadcasting system
   - Add subscription management

4. **Build conversion utilities**:
   - Convert APIExport to ResourceInfo
   - Extract OpenAPI schemas
   - Handle workspace scoping

5. **Add KCP integration**:
   - Implement logical cluster resolution
   - Add workspace filtering
   - Handle KCP feature gates

6. **Implement metrics collection**:
   - Add Prometheus metrics
   - Track discovery performance
   - Monitor cache effectiveness

## Testing Requirements
- Unit test coverage: >90%
- Test scenarios:
  - APIExport discovery and conversion
  - Cache hit/miss scenarios
  - Workspace filtering and access
  - Event watching and subscription
  - Error handling and recovery
  - Concurrent access patterns

## Integration Points
- Uses: `pkg/virtual/interfaces` for discovery contracts
- Uses: KCP APIExport informers and clients
- Uses: KCP logical cluster utilities
- Provides: ResourceDiscoveryInterface implementation

## Acceptance Criteria
- [ ] Implements ResourceDiscoveryInterface completely
- [ ] Integrates with KCP APIExport system
- [ ] Provides efficient caching with TTL
- [ ] Supports resource change watching
- [ ] Handles workspace-aware filtering
- [ ] Includes comprehensive error handling
- [ ] Provides Prometheus metrics
- [ ] Passes all unit tests
- [ ] Follows KCP architectural patterns
- [ ] Maintains workspace isolation

## Common Pitfalls
- **APIExport lifecycle**: Handle APIExport creation, updates, and deletions properly
- **Cache consistency**: Ensure cache invalidation matches APIExport changes
- **Workspace isolation**: Never leak resources between workspaces
- **Concurrent access**: Protect shared data structures with proper locking
- **Memory leaks**: Clean up watchers and subscriptions on shutdown
- **Feature gates**: Respect KCP feature gates for discovery behavior
- **Logical clusters**: Properly resolve and validate logical cluster references

## Code Review Focus
- KCP APIExport integration correctness
- Cache efficiency and consistency
- Workspace isolation enforcement
- Resource conversion accuracy
- Error handling completeness
- Concurrent access safety
- Metrics coverage and usefulness