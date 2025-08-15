# Implementation Instructions: Virtual Workspace Abstractions

## Overview
- **Branch**: feature/tmc-phase4-vw-03-workspace-abstractions
- **Purpose**: Define virtual workspace specific interfaces, lifecycle management, caching abstractions, and metrics interfaces
- **Target Lines**: 300
- **Dependencies**: Branch vw-02 (core types)
- **Estimated Time**: 2 days

## Files to Create

### 1. pkg/virtual/workspace/interfaces.go (100 lines)
**Purpose**: Define core workspace management interfaces

**Interfaces/Types to Define**:
```go
package workspace

import (
    "context"
    "time"
    
    "k8s.io/apimachinery/pkg/runtime"
    "k8s.io/apimachinery/pkg/runtime/schema"
    virtualv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/virtual/v1alpha1"
)

// Manager manages virtual workspace lifecycle
type Manager interface {
    // Create creates a new virtual workspace
    Create(ctx context.Context, config *virtualv1alpha1.VirtualWorkspace) (*Workspace, error)
    
    // Update updates an existing virtual workspace
    Update(ctx context.Context, config *virtualv1alpha1.VirtualWorkspace) (*Workspace, error)
    
    // Delete removes a virtual workspace
    Delete(ctx context.Context, name string) error
    
    // Get retrieves a specific workspace
    Get(ctx context.Context, name string) (*Workspace, error)
    
    // List returns all managed workspaces
    List(ctx context.Context) ([]*Workspace, error)
    
    // Watch monitors workspace changes
    Watch(ctx context.Context) (<-chan WorkspaceEvent, error)
}

// Workspace represents a managed virtual workspace
type Workspace struct {
    // Name is the unique identifier
    Name string
    
    // Config is the workspace configuration
    Config *virtualv1alpha1.VirtualWorkspace
    
    // State represents current state
    State WorkspaceState
    
    // Resources available in this workspace
    Resources []ResourceInfo
    
    // Metadata contains additional information
    Metadata WorkspaceMetadata
}

// WorkspaceState represents the current state of a workspace
type WorkspaceState struct {
    // Phase indicates the current phase
    Phase virtualv1alpha1.VirtualWorkspacePhase
    
    // Ready indicates if the workspace is ready
    Ready bool
    
    // Message provides human-readable status
    Message string
    
    // LastTransition is when the state last changed
    LastTransition time.Time
}

// ResourceInfo describes an available resource
type ResourceInfo struct {
    // GroupVersionResource identifies the resource
    GroupVersionResource schema.GroupVersionResource
    
    // Namespaced indicates if resource is namespaced
    Namespaced bool
    
    // Verbs lists supported operations
    Verbs []string
    
    // ShortNames are aliases for the resource
    ShortNames []string
}

// WorkspaceMetadata contains workspace metadata
type WorkspaceMetadata struct {
    // CreatedAt is workspace creation time
    CreatedAt time.Time
    
    // UpdatedAt is last update time
    UpdatedAt time.Time
    
    // Labels are workspace labels
    Labels map[string]string
    
    // Annotations are workspace annotations
    Annotations map[string]string
}

// WorkspaceEvent represents a workspace change
type WorkspaceEvent struct {
    // Type of event
    Type EventType
    
    // Workspace affected
    Workspace *Workspace
    
    // OldWorkspace for update events
    OldWorkspace *Workspace
    
    // Error if event represents an error
    Error error
}

// EventType represents types of workspace events
type EventType string

const (
    EventTypeCreated EventType = "Created"
    EventTypeUpdated EventType = "Updated"
    EventTypeDeleted EventType = "Deleted"
    EventTypeError   EventType = "Error"
)
```

### 2. pkg/virtual/workspace/lifecycle.go (80 lines)
**Purpose**: Define lifecycle management interfaces and hooks

**Interfaces/Types to Define**:
```go
package workspace

import (
    "context"
    "time"
)

// LifecycleManager handles workspace lifecycle operations
type LifecycleManager interface {
    // Initialize prepares a workspace for use
    Initialize(ctx context.Context, workspace *Workspace) error
    
    // Start activates a workspace
    Start(ctx context.Context, workspace *Workspace) error
    
    // Stop deactivates a workspace
    Stop(ctx context.Context, workspace *Workspace) error
    
    // Destroy cleans up workspace resources
    Destroy(ctx context.Context, workspace *Workspace) error
    
    // Health checks workspace health
    Health(ctx context.Context, workspace *Workspace) (*HealthStatus, error)
}

// LifecycleHook allows custom logic at lifecycle points
type LifecycleHook interface {
    // PreCreate runs before workspace creation
    PreCreate(ctx context.Context, workspace *Workspace) error
    
    // PostCreate runs after workspace creation
    PostCreate(ctx context.Context, workspace *Workspace) error
    
    // PreDelete runs before workspace deletion
    PreDelete(ctx context.Context, workspace *Workspace) error
    
    // PostDelete runs after workspace deletion
    PostDelete(ctx context.Context, workspace *Workspace) error
    
    // OnError handles lifecycle errors
    OnError(ctx context.Context, workspace *Workspace, err error) error
}

// HealthStatus represents workspace health
type HealthStatus struct {
    // Healthy indicates overall health
    Healthy bool
    
    // Components lists component health
    Components []ComponentHealth
    
    // LastCheck is when health was last checked
    LastCheck time.Time
    
    // Message provides health details
    Message string
}

// ComponentHealth represents health of a workspace component
type ComponentHealth struct {
    // Name of the component
    Name string
    
    // Healthy indicates component health
    Healthy bool
    
    // Message provides component status
    Message string
    
    // LastCheck is when component was last checked
    LastCheck time.Time
}

// LifecyclePolicy defines lifecycle behavior
type LifecyclePolicy struct {
    // AutoStart indicates if workspace should auto-start
    AutoStart bool
    
    // RestartPolicy defines restart behavior
    RestartPolicy RestartPolicy
    
    // HealthCheckInterval is how often to check health
    HealthCheckInterval time.Duration
    
    // MaxRetries for operations
    MaxRetries int
    
    // RetryBackoff for failed operations
    RetryBackoff time.Duration
}

// RestartPolicy defines restart behavior
type RestartPolicy string

const (
    RestartPolicyAlways    RestartPolicy = "Always"
    RestartPolicyOnFailure RestartPolicy = "OnFailure"
    RestartPolicyNever     RestartPolicy = "Never"
)
```

### 3. pkg/virtual/workspace/cache.go (60 lines)
**Purpose**: Define caching interfaces for workspace data

**Interfaces/Types to Define**:
```go
package workspace

import (
    "context"
    "time"
    
    "k8s.io/apimachinery/pkg/runtime"
)

// Cache provides caching for workspace data
type Cache interface {
    // Get retrieves an item from cache
    Get(ctx context.Context, key string) (interface{}, bool)
    
    // Set stores an item in cache
    Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
    
    // Delete removes an item from cache
    Delete(ctx context.Context, key string) error
    
    // Clear removes all items from cache
    Clear(ctx context.Context) error
    
    // Keys returns all cache keys
    Keys(ctx context.Context) ([]string, error)
    
    // Stats returns cache statistics
    Stats(ctx context.Context) (*CacheStats, error)
}

// ObjectCache provides type-safe object caching
type ObjectCache interface {
    // GetObject retrieves a typed object from cache
    GetObject(ctx context.Context, key string, obj runtime.Object) (bool, error)
    
    // SetObject stores a typed object in cache
    SetObject(ctx context.Context, key string, obj runtime.Object, ttl time.Duration) error
    
    // InvalidatePattern removes objects matching pattern
    InvalidatePattern(ctx context.Context, pattern string) error
}

// CacheStats provides cache statistics
type CacheStats struct {
    // Hits is the number of cache hits
    Hits int64
    
    // Misses is the number of cache misses
    Misses int64
    
    // Evictions is the number of evictions
    Evictions int64
    
    // Size is current cache size in bytes
    Size int64
    
    // Items is the number of items in cache
    Items int64
    
    // HitRate is the cache hit rate
    HitRate float64
}

// CacheConfig configures cache behavior
type CacheConfig struct {
    // MaxSize is maximum cache size in MB
    MaxSize int
    
    // DefaultTTL is default TTL for items
    DefaultTTL time.Duration
    
    // EvictionPolicy defines eviction behavior
    EvictionPolicy EvictionPolicy
    
    // EnableMetrics enables cache metrics
    EnableMetrics bool
}

// EvictionPolicy defines cache eviction strategy
type EvictionPolicy string

const (
    EvictionPolicyLRU  EvictionPolicy = "LRU"  // Least Recently Used
    EvictionPolicyLFU  EvictionPolicy = "LFU"  // Least Frequently Used
    EvictionPolicyFIFO EvictionPolicy = "FIFO" // First In First Out
    EvictionPolicyTTL  EvictionPolicy = "TTL"  // Time To Live based
)
```

### 4. pkg/virtual/workspace/metrics.go (60 lines)
**Purpose**: Define metrics interfaces for workspace monitoring

**Interfaces/Types to Define**:
```go
package workspace

import (
    "context"
    "time"
)

// MetricsCollector collects workspace metrics
type MetricsCollector interface {
    // RecordRequest records an API request
    RecordRequest(ctx context.Context, metric RequestMetric) error
    
    // RecordLatency records operation latency
    RecordLatency(ctx context.Context, operation string, duration time.Duration) error
    
    // RecordError records an error occurrence
    RecordError(ctx context.Context, operation string, err error) error
    
    // GetMetrics retrieves current metrics
    GetMetrics(ctx context.Context) (*Metrics, error)
    
    // Reset clears all metrics
    Reset(ctx context.Context) error
}

// RequestMetric represents a single request metric
type RequestMetric struct {
    // Workspace that handled the request
    Workspace string
    
    // Method is the HTTP method
    Method string
    
    // Path is the request path
    Path string
    
    // StatusCode is the response status
    StatusCode int
    
    // Latency is request duration
    Latency time.Duration
    
    // Size is response size in bytes
    Size int64
}

// Metrics contains aggregated metrics
type Metrics struct {
    // RequestCount is total requests
    RequestCount int64
    
    // ErrorCount is total errors
    ErrorCount int64
    
    // LatencyP50 is 50th percentile latency
    LatencyP50 time.Duration
    
    // LatencyP95 is 95th percentile latency
    LatencyP95 time.Duration
    
    // LatencyP99 is 99th percentile latency
    LatencyP99 time.Duration
    
    // BytesIn is total bytes received
    BytesIn int64
    
    // BytesOut is total bytes sent
    BytesOut int64
    
    // ActiveConnections is current connections
    ActiveConnections int64
}

// MetricsExporter exports metrics to external systems
type MetricsExporter interface {
    // Export sends metrics to external system
    Export(ctx context.Context, metrics *Metrics) error
    
    // Configure sets exporter configuration
    Configure(config map[string]interface{}) error
}

// MetricsConfig configures metrics collection
type MetricsConfig struct {
    // Enabled enables metrics collection
    Enabled bool
    
    // SampleRate is the sampling rate (0.0-1.0)
    SampleRate float64
    
    // FlushInterval is how often to flush metrics
    FlushInterval time.Duration
    
    // Exporters lists enabled exporters
    Exporters []string
}
```

## Implementation Steps

1. **Create package structure**:
   - Create `pkg/virtual/workspace/` directory
   - Add package documentation

2. **Implement core interfaces**:
   - Start with `interfaces.go` for workspace management
   - Add `lifecycle.go` for lifecycle operations
   - Include `cache.go` for caching abstractions
   - Add `metrics.go` for monitoring

3. **Define data structures**:
   - Workspace representation
   - Event types and handling
   - Configuration structures

4. **Add constants and enums**:
   - Event types
   - Lifecycle phases
   - Policy definitions

## Testing Requirements
- Unit test coverage: Mock implementations for testing
- Test scenarios:
  - Interface compilation
  - Mock workspace operations
  - Cache behavior simulation
  - Metrics collection

## Integration Points
- Uses: Core types from branch vw-02
- Provides: Abstractions for workspace implementation in future branches

## Acceptance Criteria
- [ ] All interfaces clearly defined
- [ ] Comprehensive godoc comments
- [ ] Mock implementations for testing
- [ ] No circular dependencies
- [ ] Follows KCP workspace patterns
- [ ] Clear separation of concerns
- [ ] No linting errors

## Common Pitfalls
- **Keep interfaces focused**: Single responsibility per interface
- **Avoid implementation details**: These are abstractions only
- **Consider extensibility**: Design for future enhancements
- **Follow Go idioms**: Use standard Go patterns
- **Document behavior**: Clear contracts in comments
- **Think about testing**: Interfaces should be easily mockable

## Code Review Focus
- Interface design clarity
- Separation of concerns
- Extensibility for future features
- Consistency with KCP patterns
- Documentation completeness