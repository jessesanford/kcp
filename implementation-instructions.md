# Virtual Workspace Core Implementation Instructions

## Overview
This branch implements the core Virtual Workspace infrastructure that provides the foundation for TMC's multi-cluster workload management. It creates the virtual workspace framework that exposes unified APIs across multiple physical clusters.

**Branch**: `feature/tmc-completion/p6w2-vw-core`  
**Estimated Lines**: 700 lines  
**Wave**: 2 (Critical Path)  
**Dependencies**: p6w1-synctarget-controller must be complete  

## Dependencies

### Required Before Starting
- Phase 5 APIs complete (TMC types and interfaces)
- p6w1-synctarget-controller merged (provides SyncTarget access)
- KCP virtual workspace framework available

### Blocks These Features
- p6w2-vw-endpoints (needs core VW infrastructure)
- p6w2-vw-discovery (needs core VW infrastructure)

## Files to Create/Modify

### Primary Implementation Files (700 lines total)

1. **pkg/virtual/syncer/virtualworkspace.go** (250 lines)
   - Main virtual workspace implementation
   - REST API provider setup
   - Request routing logic

2. **pkg/virtual/syncer/builder.go** (150 lines)
   - Virtual workspace builder pattern
   - Configuration management
   - Middleware setup

3. **pkg/virtual/syncer/provider.go** (120 lines)
   - REST storage provider
   - Resource handling
   - API registration

4. **pkg/virtual/syncer/cache.go** (100 lines)
   - Caching layer for virtual resources
   - Indexing and lookup optimization
   - Cache invalidation logic

5. **pkg/virtual/syncer/transform.go** (80 lines)
   - Resource transformation utilities
   - Cross-cluster resource mapping
   - Label and annotation management

### Test Files (not counted in line limit)

1. **pkg/virtual/syncer/virtualworkspace_test.go**
2. **pkg/virtual/syncer/builder_test.go**
3. **pkg/virtual/syncer/provider_test.go**
4. **pkg/virtual/syncer/cache_test.go**

## Step-by-Step Implementation Guide

### Step 1: Setup Virtual Workspace Core (Hour 1-2)

```go
// pkg/virtual/syncer/virtualworkspace.go
package syncer

import (
    "context"
    "fmt"
    "net/http"
    "sync"
    
    tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
    "github.com/kcp-dev/kcp/pkg/virtual/framework"
    virtualcontext "github.com/kcp-dev/kcp/pkg/virtual/framework/context"
    "github.com/kcp-dev/kcp/pkg/virtual/framework/dynamic"
    
    "github.com/kcp-dev/logicalcluster/v3"
    
    "k8s.io/apiserver/pkg/endpoints/request"
    "k8s.io/apiserver/pkg/registry/rest"
    "k8s.io/klog/v2"
)

// VirtualWorkspace provides a virtual view of resources across multiple clusters
type VirtualWorkspace struct {
    framework.VirtualWorkspace
    
    // Core components
    restProviders map[string]rest.Storage
    cache         *ResourceCache
    transformer   *ResourceTransformer
    
    // Configuration
    config        *VirtualWorkspaceConfig
    
    // Synchronization
    lock          sync.RWMutex
    ready         bool
}

// VirtualWorkspaceConfig holds configuration for the virtual workspace
type VirtualWorkspaceConfig struct {
    // Name of the virtual workspace
    Name string
    
    // Logical cluster for workspace isolation
    LogicalCluster logicalcluster.Name
    
    // SyncTarget selector
    SyncTargetSelector labels.Selector
    
    // Resource filters
    ResourceFilters []ResourceFilter
    
    // Caching configuration
    CacheSize int
    CacheTTL  time.Duration
}

// NewVirtualWorkspace creates a new virtual workspace instance
func NewVirtualWorkspace(config *VirtualWorkspaceConfig) (*VirtualWorkspace, error) {
    if config == nil {
        return nil, fmt.Errorf("config is required")
    }
    
    vw := &VirtualWorkspace{
        config:        config,
        restProviders: make(map[string]rest.Storage),
        cache:         NewResourceCache(config.CacheSize, config.CacheTTL),
        transformer:   NewResourceTransformer(config.LogicalCluster),
    }
    
    // Initialize base virtual workspace
    baseVW, err := framework.NewVirtualWorkspace(config.Name)
    if err != nil {
        return nil, fmt.Errorf("failed to create base virtual workspace: %w", err)
    }
    vw.VirtualWorkspace = baseVW
    
    // Setup REST providers
    if err := vw.setupProviders(); err != nil {
        return nil, fmt.Errorf("failed to setup providers: %w", err)
    }
    
    return vw, nil
}

// ServeHTTP handles incoming requests to the virtual workspace
func (vw *VirtualWorkspace) ServeHTTP(w http.ResponseWriter, req *http.Request) {
    ctx := req.Context()
    
    // Extract request info
    requestInfo, ok := request.RequestInfoFrom(ctx)
    if !ok {
        http.Error(w, "no RequestInfo", http.StatusInternalServerError)
        return
    }
    
    // Check if ready
    if !vw.IsReady() {
        http.Error(w, "virtual workspace not ready", http.StatusServiceUnavailable)
        return
    }
    
    // Log request
    klog.V(4).Infof("VirtualWorkspace handling request: %s %s", req.Method, req.URL.Path)
    
    // Route based on resource
    if provider, exists := vw.getProvider(requestInfo.Resource); exists {
        vw.handleWithProvider(w, req, provider, requestInfo)
    } else {
        // Delegate to base implementation
        vw.VirtualWorkspace.ServeHTTP(w, req)
    }
}

// setupProviders initializes REST storage providers
func (vw *VirtualWorkspace) setupProviders() error {
    // Setup provider for WorkloadPlacement
    vw.restProviders["workloadplacements"] = &WorkloadPlacementProvider{
        vw:    vw,
        cache: vw.cache,
    }
    
    // Setup provider for SyncTarget (virtual view)
    vw.restProviders["synctargets"] = &SyncTargetProvider{
        vw:    vw,
        cache: vw.cache,
    }
    
    // Setup provider for ClusterRegistration (virtual view)
    vw.restProviders["clusterregistrations"] = &ClusterRegistrationProvider{
        vw:    vw,
        cache: vw.cache,
    }
    
    return nil
}

// handleWithProvider processes request using the appropriate provider
func (vw *VirtualWorkspace) handleWithProvider(w http.ResponseWriter, req *http.Request, 
    provider rest.Storage, requestInfo *request.RequestInfo) {
    
    ctx := req.Context()
    
    // Add virtual workspace context
    ctx = virtualcontext.WithVirtualWorkspace(ctx, vw.config.Name)
    
    // Handle based on verb
    switch requestInfo.Verb {
    case "get":
        vw.handleGet(ctx, w, req, provider, requestInfo)
    case "list":
        vw.handleList(ctx, w, req, provider, requestInfo)
    case "create":
        vw.handleCreate(ctx, w, req, provider, requestInfo)
    case "update":
        vw.handleUpdate(ctx, w, req, provider, requestInfo)
    case "delete":
        vw.handleDelete(ctx, w, req, provider, requestInfo)
    case "watch":
        vw.handleWatch(ctx, w, req, provider, requestInfo)
    default:
        http.Error(w, fmt.Sprintf("verb %s not supported", requestInfo.Verb), http.StatusMethodNotAllowed)
    }
}
```

### Step 2: Implement Builder Pattern (Hour 3-4)

```go
// pkg/virtual/syncer/builder.go
package syncer

import (
    "context"
    "fmt"
    
    "github.com/kcp-dev/kcp/pkg/virtual/framework"
    tmcinformers "github.com/kcp-dev/kcp/pkg/client/informers/externalversions/tmc/v1alpha1"
    
    "k8s.io/client-go/tools/cache"
)

// VirtualWorkspaceBuilder builds virtual workspace instances
type VirtualWorkspaceBuilder struct {
    config              *VirtualWorkspaceConfig
    informers          map[string]cache.SharedIndexInformer
    middlewares        []Middleware
    transformers       []ResourceTransformer
    authorizationFunc  AuthorizationFunc
}

// Middleware represents a middleware function
type Middleware func(http.Handler) http.Handler

// AuthorizationFunc checks if a request is authorized
type AuthorizationFunc func(ctx context.Context, attrs authorizer.Attributes) (authorized bool, reason string, err error)

// NewVirtualWorkspaceBuilder creates a new builder
func NewVirtualWorkspaceBuilder(name string) *VirtualWorkspaceBuilder {
    return &VirtualWorkspaceBuilder{
        config: &VirtualWorkspaceConfig{
            Name:      name,
            CacheSize: 1000,
            CacheTTL:  5 * time.Minute,
        },
        informers:    make(map[string]cache.SharedIndexInformer),
        middlewares:  []Middleware{},
        transformers: []ResourceTransformer{},
    }
}

// WithLogicalCluster sets the logical cluster
func (b *VirtualWorkspaceBuilder) WithLogicalCluster(cluster logicalcluster.Name) *VirtualWorkspaceBuilder {
    b.config.LogicalCluster = cluster
    return b
}

// WithSyncTargetSelector sets the SyncTarget selector
func (b *VirtualWorkspaceBuilder) WithSyncTargetSelector(selector labels.Selector) *VirtualWorkspaceBuilder {
    b.config.SyncTargetSelector = selector
    return b
}

// WithInformer adds an informer for a resource type
func (b *VirtualWorkspaceBuilder) WithInformer(resource string, informer cache.SharedIndexInformer) *VirtualWorkspaceBuilder {
    b.informers[resource] = informer
    return b
}

// WithMiddleware adds a middleware
func (b *VirtualWorkspaceBuilder) WithMiddleware(mw Middleware) *VirtualWorkspaceBuilder {
    b.middlewares = append(b.middlewares, mw)
    return b
}

// WithTransformer adds a resource transformer
func (b *VirtualWorkspaceBuilder) WithTransformer(t ResourceTransformer) *VirtualWorkspaceBuilder {
    b.transformers = append(b.transformers, t)
    return b
}

// WithAuthorization sets the authorization function
func (b *VirtualWorkspaceBuilder) WithAuthorization(authz AuthorizationFunc) *VirtualWorkspaceBuilder {
    b.authorizationFunc = authz
    return b
}

// WithCaching configures caching
func (b *VirtualWorkspaceBuilder) WithCaching(size int, ttl time.Duration) *VirtualWorkspaceBuilder {
    b.config.CacheSize = size
    b.config.CacheTTL = ttl
    return b
}

// Build creates the virtual workspace
func (b *VirtualWorkspaceBuilder) Build() (*VirtualWorkspace, error) {
    // Validate configuration
    if err := b.validate(); err != nil {
        return nil, fmt.Errorf("invalid configuration: %w", err)
    }
    
    // Create virtual workspace
    vw, err := NewVirtualWorkspace(b.config)
    if err != nil {
        return nil, err
    }
    
    // Apply informers
    for resource, informer := range b.informers {
        if err := vw.RegisterInformer(resource, informer); err != nil {
            return nil, fmt.Errorf("failed to register informer for %s: %w", resource, err)
        }
    }
    
    // Apply middlewares
    handler := http.Handler(vw)
    for i := len(b.middlewares) - 1; i >= 0; i-- {
        handler = b.middlewares[i](handler)
    }
    vw.handler = handler
    
    // Apply transformers
    for _, t := range b.transformers {
        vw.transformer.AddTransformer(t)
    }
    
    // Set authorization
    if b.authorizationFunc != nil {
        vw.authorizationFunc = b.authorizationFunc
    }
    
    return vw, nil
}

// validate checks if the configuration is valid
func (b *VirtualWorkspaceBuilder) validate() error {
    if b.config.Name == "" {
        return fmt.Errorf("name is required")
    }
    
    if b.config.LogicalCluster.Empty() {
        return fmt.Errorf("logical cluster is required")
    }
    
    if b.config.CacheSize <= 0 {
        return fmt.Errorf("cache size must be positive")
    }
    
    if b.config.CacheTTL <= 0 {
        return fmt.Errorf("cache TTL must be positive")
    }
    
    return nil
}

// RegisterInformer registers an informer for a resource type
func (vw *VirtualWorkspace) RegisterInformer(resource string, informer cache.SharedIndexInformer) error {
    vw.lock.Lock()
    defer vw.lock.Unlock()
    
    // Add event handlers
    informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
        AddFunc: func(obj interface{}) {
            vw.handleInformerAdd(resource, obj)
        },
        UpdateFunc: func(old, new interface{}) {
            vw.handleInformerUpdate(resource, old, new)
        },
        DeleteFunc: func(obj interface{}) {
            vw.handleInformerDelete(resource, obj)
        },
    })
    
    return nil
}
```

### Step 3: Implement Storage Provider (Hour 5)

```go
// pkg/virtual/syncer/provider.go
package syncer

import (
    "context"
    "fmt"
    
    tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
    
    metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/runtime"
    "k8s.io/apiserver/pkg/registry/rest"
)

// WorkloadPlacementProvider provides REST storage for WorkloadPlacement
type WorkloadPlacementProvider struct {
    rest.Storage
    
    vw    *VirtualWorkspace
    cache *ResourceCache
}

// NewWorkloadPlacementProvider creates a new provider
func NewWorkloadPlacementProvider(vw *VirtualWorkspace) *WorkloadPlacementProvider {
    return &WorkloadPlacementProvider{
        vw:    vw,
        cache: vw.cache,
    }
}

// New creates a new WorkloadPlacement
func (p *WorkloadPlacementProvider) New() runtime.Object {
    return &tmcv1alpha1.WorkloadPlacement{}
}

// NewList creates a new WorkloadPlacement list
func (p *WorkloadPlacementProvider) NewList() runtime.Object {
    return &tmcv1alpha1.WorkloadPlacementList{}
}

// Get retrieves a WorkloadPlacement
func (p *WorkloadPlacementProvider) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
    // Check cache first
    if cached := p.cache.Get("workloadplacement", name); cached != nil {
        return cached, nil
    }
    
    // Retrieve from backend
    placement, err := p.vw.getWorkloadPlacement(ctx, name)
    if err != nil {
        return nil, err
    }
    
    // Transform for virtual view
    transformed := p.vw.transformer.TransformWorkloadPlacement(placement)
    
    // Update cache
    p.cache.Set("workloadplacement", name, transformed)
    
    return transformed, nil
}

// List retrieves a list of WorkloadPlacements
func (p *WorkloadPlacementProvider) List(ctx context.Context, options *metainternalversion.ListOptions) (runtime.Object, error) {
    // Parse options
    selector := labels.Everything()
    if options.LabelSelector != nil {
        selector = options.LabelSelector
    }
    
    // Get all placements
    placements, err := p.vw.listWorkloadPlacements(ctx, selector)
    if err != nil {
        return nil, err
    }
    
    // Transform for virtual view
    list := &tmcv1alpha1.WorkloadPlacementList{
        Items: make([]tmcv1alpha1.WorkloadPlacement, 0, len(placements)),
    }
    
    for _, placement := range placements {
        transformed := p.vw.transformer.TransformWorkloadPlacement(placement)
        list.Items = append(list.Items, *transformed)
    }
    
    return list, nil
}

// Create creates a new WorkloadPlacement
func (p *WorkloadPlacementProvider) Create(ctx context.Context, obj runtime.Object, 
    createValidation rest.ValidateObjectFunc, options *metav1.CreateOptions) (runtime.Object, error) {
    
    placement, ok := obj.(*tmcv1alpha1.WorkloadPlacement)
    if !ok {
        return nil, fmt.Errorf("expected WorkloadPlacement, got %T", obj)
    }
    
    // Validate
    if createValidation != nil {
        if err := createValidation(ctx, obj); err != nil {
            return nil, err
        }
    }
    
    // Transform from virtual view
    backend := p.vw.transformer.TransformToBackend(placement)
    
    // Create in backend
    created, err := p.vw.createWorkloadPlacement(ctx, backend)
    if err != nil {
        return nil, err
    }
    
    // Transform back to virtual view
    result := p.vw.transformer.TransformWorkloadPlacement(created)
    
    // Update cache
    p.cache.Set("workloadplacement", result.Name, result)
    
    return result, nil
}

// Update updates a WorkloadPlacement
func (p *WorkloadPlacementProvider) Update(ctx context.Context, name string, objInfo rest.UpdatedObjectInfo,
    createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc,
    forceAllowCreate bool, options *metav1.UpdateOptions) (runtime.Object, bool, error) {
    
    // Get existing object
    existing, err := p.Get(ctx, name, &metav1.GetOptions{})
    if err != nil {
        return nil, false, err
    }
    
    // Get updated object
    updated, err := objInfo.UpdatedObject(ctx, existing)
    if err != nil {
        return nil, false, err
    }
    
    placement, ok := updated.(*tmcv1alpha1.WorkloadPlacement)
    if !ok {
        return nil, false, fmt.Errorf("expected WorkloadPlacement, got %T", updated)
    }
    
    // Validate
    if updateValidation != nil {
        if err := updateValidation(ctx, updated, existing); err != nil {
            return nil, false, err
        }
    }
    
    // Transform from virtual view
    backend := p.vw.transformer.TransformToBackend(placement)
    
    // Update in backend
    result, err := p.vw.updateWorkloadPlacement(ctx, backend)
    if err != nil {
        return nil, false, err
    }
    
    // Transform back to virtual view
    transformed := p.vw.transformer.TransformWorkloadPlacement(result)
    
    // Update cache
    p.cache.Set("workloadplacement", transformed.Name, transformed)
    
    return transformed, false, nil
}

// Delete deletes a WorkloadPlacement
func (p *WorkloadPlacementProvider) Delete(ctx context.Context, name string, 
    deleteValidation rest.ValidateObjectFunc, options *metav1.DeleteOptions) (runtime.Object, bool, error) {
    
    // Get existing object
    existing, err := p.Get(ctx, name, &metav1.GetOptions{})
    if err != nil {
        return nil, false, err
    }
    
    // Validate
    if deleteValidation != nil {
        if err := deleteValidation(ctx, existing); err != nil {
            return nil, false, err
        }
    }
    
    // Delete from backend
    if err := p.vw.deleteWorkloadPlacement(ctx, name); err != nil {
        return nil, false, err
    }
    
    // Remove from cache
    p.cache.Delete("workloadplacement", name)
    
    return existing, true, nil
}
```

### Step 4: Implement Caching Layer (Hour 6)

```go
// pkg/virtual/syncer/cache.go
package syncer

import (
    "fmt"
    "sync"
    "time"
    
    "k8s.io/apimachinery/pkg/runtime"
    "k8s.io/apimachinery/pkg/util/cache"
)

// ResourceCache provides caching for virtual resources
type ResourceCache struct {
    cache     *cache.LRUExpireCache
    indexers  map[string]Indexer
    lock      sync.RWMutex
}

// Indexer provides indexing for cached resources
type Indexer interface {
    Index(obj runtime.Object) []string
    ByIndex(indexValue string) []runtime.Object
}

// NewResourceCache creates a new resource cache
func NewResourceCache(size int, ttl time.Duration) *ResourceCache {
    return &ResourceCache{
        cache:    cache.NewLRUExpireCache(size),
        indexers: make(map[string]Indexer),
    }
}

// Get retrieves an object from cache
func (c *ResourceCache) Get(resourceType, name string) runtime.Object {
    c.lock.RLock()
    defer c.lock.RUnlock()
    
    key := c.makeKey(resourceType, name)
    if obj, exists := c.cache.Get(key); exists {
        return obj.(runtime.Object)
    }
    
    return nil
}

// Set stores an object in cache
func (c *ResourceCache) Set(resourceType, name string, obj runtime.Object) {
    c.lock.Lock()
    defer c.lock.Unlock()
    
    key := c.makeKey(resourceType, name)
    c.cache.Add(key, obj, 5*time.Minute)
    
    // Update indexes
    if indexer, exists := c.indexers[resourceType]; exists {
        for _, indexValue := range indexer.Index(obj) {
            c.updateIndex(resourceType, indexValue, obj)
        }
    }
}

// Delete removes an object from cache
func (c *ResourceCache) Delete(resourceType, name string) {
    c.lock.Lock()
    defer c.lock.Unlock()
    
    key := c.makeKey(resourceType, name)
    c.cache.Remove(key)
}

// List returns all cached objects of a type
func (c *ResourceCache) List(resourceType string) []runtime.Object {
    c.lock.RLock()
    defer c.lock.RUnlock()
    
    var result []runtime.Object
    for _, key := range c.cache.Keys() {
        if c.getResourceType(key.(string)) == resourceType {
            if obj, exists := c.cache.Get(key); exists {
                result = append(result, obj.(runtime.Object))
            }
        }
    }
    
    return result
}

// ByIndex returns objects matching an index value
func (c *ResourceCache) ByIndex(resourceType, indexName, indexValue string) []runtime.Object {
    c.lock.RLock()
    defer c.lock.RUnlock()
    
    if indexer, exists := c.indexers[resourceType]; exists {
        return indexer.ByIndex(indexValue)
    }
    
    return nil
}

// AddIndexer adds an indexer for a resource type
func (c *ResourceCache) AddIndexer(resourceType string, indexer Indexer) {
    c.lock.Lock()
    defer c.lock.Unlock()
    
    c.indexers[resourceType] = indexer
}

// makeKey creates a cache key
func (c *ResourceCache) makeKey(resourceType, name string) string {
    return fmt.Sprintf("%s/%s", resourceType, name)
}

// getResourceType extracts resource type from key
func (c *ResourceCache) getResourceType(key string) string {
    parts := strings.Split(key, "/")
    if len(parts) > 0 {
        return parts[0]
    }
    return ""
}

// updateIndex updates index entries
func (c *ResourceCache) updateIndex(resourceType, indexValue string, obj runtime.Object) {
    // Implementation depends on specific indexing requirements
    // This is a placeholder for index update logic
}

// Invalidate removes expired entries
func (c *ResourceCache) Invalidate() {
    c.lock.Lock()
    defer c.lock.Unlock()
    
    // LRUExpireCache handles expiration automatically
    // This method can be used for forced invalidation if needed
}
```

### Step 5: Implement Resource Transformation (Hour 7)

```go
// pkg/virtual/syncer/transform.go
package syncer

import (
    "fmt"
    
    tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
    "github.com/kcp-dev/logicalcluster/v3"
    
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ResourceTransformer transforms resources between virtual and backend views
type ResourceTransformer struct {
    logicalCluster logicalcluster.Name
    transformers   []TransformFunc
}

// TransformFunc is a function that transforms a resource
type TransformFunc func(obj runtime.Object) runtime.Object

// NewResourceTransformer creates a new transformer
func NewResourceTransformer(cluster logicalcluster.Name) *ResourceTransformer {
    return &ResourceTransformer{
        logicalCluster: cluster,
        transformers:   []TransformFunc{},
    }
}

// TransformWorkloadPlacement transforms a placement for virtual view
func (t *ResourceTransformer) TransformWorkloadPlacement(placement *tmcv1alpha1.WorkloadPlacement) *tmcv1alpha1.WorkloadPlacement {
    result := placement.DeepCopy()
    
    // Add virtual workspace annotations
    if result.Annotations == nil {
        result.Annotations = make(map[string]string)
    }
    result.Annotations["virtual.kcp.dev/workspace"] = string(t.logicalCluster)
    
    // Transform cluster references
    if result.Spec.TargetClusters != nil {
        for i, target := range result.Spec.TargetClusters {
            result.Spec.TargetClusters[i] = t.transformClusterReference(target)
        }
    }
    
    // Apply custom transformers
    for _, transformer := range t.transformers {
        result = transformer(result).(*tmcv1alpha1.WorkloadPlacement)
    }
    
    return result
}

// TransformToBackend transforms from virtual view to backend
func (t *ResourceTransformer) TransformToBackend(placement *tmcv1alpha1.WorkloadPlacement) *tmcv1alpha1.WorkloadPlacement {
    result := placement.DeepCopy()
    
    // Remove virtual workspace annotations
    delete(result.Annotations, "virtual.kcp.dev/workspace")
    
    // Add backend labels
    if result.Labels == nil {
        result.Labels = make(map[string]string)
    }
    result.Labels["tmc.kcp.dev/logical-cluster"] = string(t.logicalCluster)
    
    return result
}

// transformClusterReference transforms a cluster reference
func (t *ResourceTransformer) transformClusterReference(ref tmcv1alpha1.ClusterReference) tmcv1alpha1.ClusterReference {
    // Add workspace prefix to cluster name
    ref.Name = fmt.Sprintf("%s-%s", t.logicalCluster, ref.Name)
    return ref
}

// AddTransformer adds a custom transformer
func (t *ResourceTransformer) AddTransformer(transformer TransformFunc) {
    t.transformers = append(t.transformers, transformer)
}

// TransformSyncTarget transforms a SyncTarget for virtual view
func (t *ResourceTransformer) TransformSyncTarget(st *tmcv1alpha1.SyncTarget) *tmcv1alpha1.SyncTarget {
    result := st.DeepCopy()
    
    // Hide sensitive information
    result.Spec.KubeConfig = ""
    
    // Add virtual annotations
    if result.Annotations == nil {
        result.Annotations = make(map[string]string)
    }
    result.Annotations["virtual.kcp.dev/readonly"] = "true"
    
    return result
}

// TransformClusterRegistration transforms a registration for virtual view
func (t *ResourceTransformer) TransformClusterRegistration(cr *tmcv1alpha1.ClusterRegistration) *tmcv1alpha1.ClusterRegistration {
    result := cr.DeepCopy()
    
    // Hide credentials
    if result.Status.Credentials != nil {
        result.Status.Credentials.SecretRef = nil
    }
    
    // Add virtual metadata
    if result.Annotations == nil {
        result.Annotations = make(map[string]string)
    }
    result.Annotations["virtual.kcp.dev/view"] = "summary"
    
    return result
}
```

## Testing Requirements

### Unit Tests

1. **Virtual Workspace Tests**
   - Test creation and initialization
   - Test request routing
   - Test provider registration
   - Test ready state management

2. **Builder Tests**
   - Test builder configuration
   - Test validation logic
   - Test middleware application
   - Test build process

3. **Provider Tests**
   - Test CRUD operations
   - Test validation
   - Test error handling
   - Test caching integration

4. **Cache Tests**
   - Test cache operations
   - Test expiration
   - Test indexing
   - Test invalidation

5. **Transformer Tests**
   - Test resource transformation
   - Test bidirectional conversion
   - Test annotation/label management

### Integration Tests

1. **End-to-End Virtual Workspace**
   - Create virtual workspace
   - Handle requests
   - Verify transformations
   - Check caching

2. **Multi-Resource Management**
   - Test multiple resource types
   - Test cross-resource references
   - Test consistency

## KCP Patterns to Follow

### Virtual Workspace Framework
- Extend framework.VirtualWorkspace properly
- Implement all required interfaces
- Handle context propagation

### Workspace Isolation
- Maintain logical cluster boundaries
- Filter resources by workspace
- Enforce access control

### REST Storage Implementation
- Implement full REST semantics
- Handle watch operations
- Support field selectors

### Caching Strategy
- Use LRU with expiration
- Implement proper invalidation
- Support indexing for performance

## Integration Points

### With SyncTarget Controller (p6w1-synctarget-controller)
- Access SyncTarget information
- Transform for virtual view
- Hide sensitive data

### With VW Endpoints (p6w2-vw-endpoints)
- Provides core infrastructure
- Shares cache and transformers
- Handles base routing

### With VW Discovery (p6w2-vw-discovery)
- Provides resource information
- Shares virtual workspace context
- Coordinates API exposure

## Validation Checklist

### Before Commit
- [ ] All files created as specified
- [ ] Line count under 700 (run `/workspaces/kcp-shared-tools/tmc-pr-line-counter.sh`)
- [ ] All tests passing (`make test`)
- [ ] Linting passes (`make lint`)
- [ ] No sensitive data exposed

### Functionality Complete
- [ ] Virtual workspace creates and starts
- [ ] Request routing works
- [ ] Resource transformation functional
- [ ] Caching operational
- [ ] Builder pattern works

### Integration Ready
- [ ] Interfaces exported for extensions
- [ ] Cache shareable
- [ ] Transformers extensible
- [ ] Context propagation works

### Documentation Complete
- [ ] Code comments comprehensive
- [ ] API patterns documented
- [ ] Usage examples provided
- [ ] Architecture documented

## Commit Message Template
```
feat(virtual): implement Virtual Workspace core infrastructure

- Add virtual workspace with REST provider framework
- Implement builder pattern for configuration
- Add caching layer with indexing support
- Implement resource transformation pipeline
- Ensure workspace isolation throughout
- Hide sensitive data in virtual views

Part of TMC Phase 6 Wave 2 implementation
Depends on: p6w1-synctarget-controller
Critical path for: p6w2-vw-endpoints, p6w2-vw-discovery
```

## Next Steps
After this branch is complete:
1. p6w2-vw-endpoints can implement endpoint exposure
2. p6w2-vw-discovery can add discovery features
3. Virtual workspace will be fully operational