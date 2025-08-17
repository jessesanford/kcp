# Virtual Workspace Discovery Implementation Instructions

## Overview
This branch implements the discovery mechanisms for the Virtual Workspace, providing API discovery, OpenAPI documentation, and resource schema information. It enables clients to discover available TMC APIs and understand their capabilities.

**Branch**: `feature/tmc-completion/p1w2-vw-discovery`  
**Estimated Lines**: 500 lines  
**Wave**: 3  
**Dependencies**: p1w2-vw-core must be complete  

## Dependencies

### Required Before Starting
- Phase 0 APIs complete
- p1w2-vw-core merged (provides VW infrastructure)
- Virtual workspace framework available

### Blocks These Features
- None directly, but enhances API discoverability

## Files to Create/Modify

### Primary Implementation Files (500 lines total)

1. **pkg/virtual/syncer/discovery/discovery_manager.go** (150 lines)
   - Main discovery management
   - API registration
   - Schema management

2. **pkg/virtual/syncer/discovery/openapi_provider.go** (120 lines)
   - OpenAPI spec generation
   - Schema definitions
   - Documentation generation

3. **pkg/virtual/syncer/discovery/resource_discovery.go** (100 lines)
   - Resource discovery endpoints
   - Group/version information
   - Available resources listing

4. **pkg/virtual/syncer/discovery/schema_store.go** (80 lines)
   - Schema storage and retrieval
   - Version management
   - Schema validation

5. **pkg/virtual/syncer/discovery/aggregator.go** (50 lines)
   - Discovery information aggregation
   - Cross-cluster discovery
   - Cache management

### Test Files (not counted in line limit)

1. **pkg/virtual/syncer/discovery/discovery_manager_test.go**
2. **pkg/virtual/syncer/discovery/openapi_provider_test.go**
3. **pkg/virtual/syncer/discovery/resource_discovery_test.go**

## Step-by-Step Implementation Guide

### Step 1: Setup Discovery Manager (Hour 1-2)

```go
// pkg/virtual/syncer/discovery/discovery_manager.go
package discovery

import (
    "context"
    "fmt"
    "sync"
    
    "github.com/kcp-dev/kcp/pkg/virtual/syncer"
    tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
    
    "github.com/kcp-dev/logicalcluster/v3"
    
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/runtime/schema"
    "k8s.io/apiserver/pkg/endpoints/discovery"
    "k8s.io/klog/v2"
)

// DiscoveryManager manages API discovery for virtual workspace
type DiscoveryManager struct {
    // Core components
    virtualWorkspace *syncer.VirtualWorkspace
    openAPIProvider  *OpenAPIProvider
    schemaStore      *SchemaStore
    aggregator       *DiscoveryAggregator
    
    // Discovery data
    apiGroups        map[string]*metav1.APIGroup
    apiResources     map[schema.GroupVersion]*metav1.APIResourceList
    
    // Configuration
    config           *DiscoveryConfig
    
    // Synchronization
    mutex            sync.RWMutex
}

// DiscoveryConfig holds discovery configuration
type DiscoveryConfig struct {
    // Virtual workspace name
    WorkspaceName string
    
    // Logical cluster
    LogicalCluster logicalcluster.Name
    
    // API groups to expose
    ExposedGroups []string
    
    // OpenAPI configuration
    OpenAPIConfig *OpenAPIConfig
    
    // Cache TTL
    CacheTTL time.Duration
}

// NewDiscoveryManager creates a new discovery manager
func NewDiscoveryManager(vw *syncer.VirtualWorkspace, config *DiscoveryConfig) (*DiscoveryManager, error) {
    if vw == nil {
        return nil, fmt.Errorf("virtual workspace is required")
    }
    
    if config == nil {
        config = &DiscoveryConfig{
            WorkspaceName: "tmc-virtual",
            ExposedGroups: []string{"tmc.kcp.dev"},
            CacheTTL:      5 * time.Minute,
        }
    }
    
    dm := &DiscoveryManager{
        virtualWorkspace: vw,
        config:          config,
        apiGroups:       make(map[string]*metav1.APIGroup),
        apiResources:    make(map[schema.GroupVersion]*metav1.APIResourceList),
    }
    
    // Initialize components
    dm.openAPIProvider = NewOpenAPIProvider(config.OpenAPIConfig)
    dm.schemaStore = NewSchemaStore()
    dm.aggregator = NewDiscoveryAggregator(config.CacheTTL)
    
    // Register TMC API group
    if err := dm.registerTMCAPIGroup(); err != nil {
        return nil, fmt.Errorf("failed to register TMC API group: %w", err)
    }
    
    return dm, nil
}

// registerTMCAPIGroup registers the TMC API group
func (dm *DiscoveryManager) registerTMCAPIGroup() error {
    // Define TMC API group
    tmcGroup := &metav1.APIGroup{
        Name: tmcv1alpha1.GroupName,
        Versions: []metav1.GroupVersionForDiscovery{
            {
                GroupVersion: tmcv1alpha1.SchemeGroupVersion.String(),
                Version:      tmcv1alpha1.SchemeGroupVersion.Version,
            },
        },
        PreferredVersion: metav1.GroupVersionForDiscovery{
            GroupVersion: tmcv1alpha1.SchemeGroupVersion.String(),
            Version:      tmcv1alpha1.SchemeGroupVersion.Version,
        },
    }
    
    dm.apiGroups[tmcGroup.Name] = tmcGroup
    
    // Define TMC resources
    tmcResources := &metav1.APIResourceList{
        GroupVersion: tmcv1alpha1.SchemeGroupVersion.String(),
        APIResources: []metav1.APIResource{
            {
                Name:         "workloadplacements",
                SingularName: "workloadplacement",
                Namespaced:   true,
                Kind:         "WorkloadPlacement",
                Verbs: []string{
                    "create", "delete", "get", "list", "patch", "update", "watch",
                },
                ShortNames: []string{"wp"},
                Categories: []string{"tmc"},
            },
            {
                Name:         "synctargets",
                SingularName: "synctarget",
                Namespaced:   false,
                Kind:         "SyncTarget",
                Verbs: []string{
                    "get", "list", "watch",
                },
                ShortNames: []string{"st"},
                Categories: []string{"tmc"},
            },
            {
                Name:         "clusterregistrations",
                SingularName: "clusterregistration",
                Namespaced:   false,
                Kind:         "ClusterRegistration",
                Verbs: []string{
                    "get", "list", "watch",
                },
                ShortNames: []string{"cr"},
                Categories: []string{"tmc"},
            },
        },
    }
    
    dm.apiResources[tmcv1alpha1.SchemeGroupVersion] = tmcResources
    
    // Register schemas
    if err := dm.registerSchemas(); err != nil {
        return fmt.Errorf("failed to register schemas: %w", err)
    }
    
    return nil
}

// GetAPIGroups returns available API groups
func (dm *DiscoveryManager) GetAPIGroups() *metav1.APIGroupList {
    dm.mutex.RLock()
    defer dm.mutex.RUnlock()
    
    groups := &metav1.APIGroupList{
        Groups: make([]metav1.APIGroup, 0, len(dm.apiGroups)),
    }
    
    for _, group := range dm.apiGroups {
        if dm.shouldExposeGroup(group.Name) {
            groups.Groups = append(groups.Groups, *group)
        }
    }
    
    return groups
}

// GetAPIResourceList returns resources for a group version
func (dm *DiscoveryManager) GetAPIResourceList(groupVersion string) (*metav1.APIResourceList, error) {
    dm.mutex.RLock()
    defer dm.mutex.RUnlock()
    
    gv, err := schema.ParseGroupVersion(groupVersion)
    if err != nil {
        return nil, fmt.Errorf("invalid group version: %w", err)
    }
    
    resources, exists := dm.apiResources[gv]
    if !exists {
        return nil, fmt.Errorf("group version not found: %s", groupVersion)
    }
    
    return resources, nil
}

// GetOpenAPISpec returns the OpenAPI specification
func (dm *DiscoveryManager) GetOpenAPISpec() (*spec.Swagger, error) {
    return dm.openAPIProvider.GetSpec()
}

// shouldExposeGroup checks if a group should be exposed
func (dm *DiscoveryManager) shouldExposeGroup(group string) bool {
    for _, exposed := range dm.config.ExposedGroups {
        if exposed == group {
            return true
        }
    }
    return false
}

// registerSchemas registers resource schemas
func (dm *DiscoveryManager) registerSchemas() error {
    // Register WorkloadPlacement schema
    if err := dm.schemaStore.RegisterSchema(
        tmcv1alpha1.SchemeGroupVersion.WithKind("WorkloadPlacement"),
        getWorkloadPlacementSchema(),
    ); err != nil {
        return err
    }
    
    // Register SyncTarget schema
    if err := dm.schemaStore.RegisterSchema(
        tmcv1alpha1.SchemeGroupVersion.WithKind("SyncTarget"),
        getSyncTargetSchema(),
    ); err != nil {
        return err
    }
    
    // Register ClusterRegistration schema
    if err := dm.schemaStore.RegisterSchema(
        tmcv1alpha1.SchemeGroupVersion.WithKind("ClusterRegistration"),
        getClusterRegistrationSchema(),
    ); err != nil {
        return err
    }
    
    return nil
}

// RefreshDiscovery refreshes discovery information
func (dm *DiscoveryManager) RefreshDiscovery(ctx context.Context) error {
    dm.mutex.Lock()
    defer dm.mutex.Unlock()
    
    klog.V(4).Info("Refreshing discovery information")
    
    // Aggregate discovery from virtual workspace
    aggregated, err := dm.aggregator.Aggregate(ctx, dm.virtualWorkspace)
    if err != nil {
        return fmt.Errorf("failed to aggregate discovery: %w", err)
    }
    
    // Update local cache
    dm.apiGroups = aggregated.Groups
    dm.apiResources = aggregated.Resources
    
    // Update OpenAPI spec
    if err := dm.openAPIProvider.UpdateSpec(aggregated); err != nil {
        return fmt.Errorf("failed to update OpenAPI spec: %w", err)
    }
    
    klog.V(2).Info("Discovery information refreshed")
    return nil
}

// Start starts the discovery manager
func (dm *DiscoveryManager) Start(ctx context.Context) error {
    // Start periodic refresh
    go dm.runRefreshLoop(ctx)
    
    klog.Info("Discovery manager started")
    return nil
}

// runRefreshLoop runs the periodic refresh loop
func (dm *DiscoveryManager) runRefreshLoop(ctx context.Context) {
    ticker := time.NewTicker(dm.config.CacheTTL)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            if err := dm.RefreshDiscovery(ctx); err != nil {
                klog.Errorf("Failed to refresh discovery: %v", err)
            }
        }
    }
}
```

### Step 2: Implement OpenAPI Provider (Hour 3-4)

```go
// pkg/virtual/syncer/discovery/openapi_provider.go
package discovery

import (
    "fmt"
    
    "github.com/go-openapi/spec"
    tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
    
    "k8s.io/apimachinery/pkg/runtime/schema"
    "k8s.io/apiserver/pkg/endpoints/openapi"
    "k8s.io/klog/v2"
)

// OpenAPIProvider provides OpenAPI specifications
type OpenAPIProvider struct {
    config *OpenAPIConfig
    spec   *spec.Swagger
    mutex  sync.RWMutex
}

// OpenAPIConfig holds OpenAPI configuration
type OpenAPIConfig struct {
    Title       string
    Version     string
    Description string
    Contact     *spec.ContactInfo
    License     *spec.License
}

// NewOpenAPIProvider creates a new OpenAPI provider
func NewOpenAPIProvider(config *OpenAPIConfig) *OpenAPIProvider {
    if config == nil {
        config = &OpenAPIConfig{
            Title:       "TMC Virtual Workspace API",
            Version:     "v1alpha1",
            Description: "API for managing multi-cluster workload placement",
        }
    }
    
    provider := &OpenAPIProvider{
        config: config,
    }
    
    // Initialize base spec
    provider.initializeSpec()
    
    return provider
}

// initializeSpec initializes the base OpenAPI spec
func (p *OpenAPIProvider) initializeSpec() {
    p.spec = &spec.Swagger{
        SwaggerProps: spec.SwaggerProps{
            Swagger: "2.0",
            Info: &spec.Info{
                InfoProps: spec.InfoProps{
                    Title:       p.config.Title,
                    Version:     p.config.Version,
                    Description: p.config.Description,
                    Contact:     p.config.Contact,
                    License:     p.config.License,
                },
            },
            Host:     "kcp.io",
            BasePath: "/apis",
            Schemes:  []string{"https"},
            Consumes: []string{"application/json"},
            Produces: []string{"application/json"},
            Paths:    &spec.Paths{Paths: make(map[string]spec.PathItem)},
            Definitions: spec.Definitions{
                "io.k8s.apimachinery.pkg.apis.meta.v1.ObjectMeta": getObjectMetaSchema(),
                "io.k8s.apimachinery.pkg.apis.meta.v1.ListMeta":   getListMetaSchema(),
            },
        },
    }
    
    // Add TMC definitions
    p.addTMCDefinitions()
    
    // Add TMC paths
    p.addTMCPaths()
}

// addTMCDefinitions adds TMC type definitions
func (p *OpenAPIProvider) addTMCDefinitions() {
    p.spec.Definitions["io.kcp.tmc.v1alpha1.WorkloadPlacement"] = spec.Schema{
        SchemaProps: spec.SchemaProps{
            Type:        []string{"object"},
            Description: "WorkloadPlacement defines placement policies for workloads",
            Properties: map[string]spec.Schema{
                "apiVersion": {
                    SchemaProps: spec.SchemaProps{
                        Type: []string{"string"},
                    },
                },
                "kind": {
                    SchemaProps: spec.SchemaProps{
                        Type: []string{"string"},
                    },
                },
                "metadata": {
                    SchemaProps: spec.SchemaProps{
                        Ref: spec.MustCreateRef("#/definitions/io.k8s.apimachinery.pkg.apis.meta.v1.ObjectMeta"),
                    },
                },
                "spec": {
                    SchemaProps: spec.SchemaProps{
                        Ref: spec.MustCreateRef("#/definitions/io.kcp.tmc.v1alpha1.WorkloadPlacementSpec"),
                    },
                },
                "status": {
                    SchemaProps: spec.SchemaProps{
                        Ref: spec.MustCreateRef("#/definitions/io.kcp.tmc.v1alpha1.WorkloadPlacementStatus"),
                    },
                },
            },
            Required: []string{"spec"},
        },
    }
    
    p.spec.Definitions["io.kcp.tmc.v1alpha1.WorkloadPlacementSpec"] = spec.Schema{
        SchemaProps: spec.SchemaProps{
            Type:        []string{"object"},
            Description: "WorkloadPlacementSpec defines the desired placement state",
            Properties: map[string]spec.Schema{
                "workload": {
                    SchemaProps: spec.SchemaProps{
                        Type:        []string{"object"},
                        Description: "Reference to the workload to place",
                    },
                },
                "targetClusters": {
                    SchemaProps: spec.SchemaProps{
                        Type:        []string{"array"},
                        Description: "Target clusters for placement",
                        Items: &spec.SchemaOrArray{
                            Schema: &spec.Schema{
                                SchemaProps: spec.SchemaProps{
                                    Type: []string{"string"},
                                },
                            },
                        },
                    },
                },
                "placement": {
                    SchemaProps: spec.SchemaProps{
                        Type:        []string{"object"},
                        Description: "Placement policy configuration",
                    },
                },
            },
        },
    }
    
    p.spec.Definitions["io.kcp.tmc.v1alpha1.WorkloadPlacementStatus"] = spec.Schema{
        SchemaProps: spec.SchemaProps{
            Type:        []string{"object"},
            Description: "WorkloadPlacementStatus defines the observed placement state",
            Properties: map[string]spec.Schema{
                "phase": {
                    SchemaProps: spec.SchemaProps{
                        Type:        []string{"string"},
                        Description: "Current phase of the placement",
                    },
                },
                "conditions": {
                    SchemaProps: spec.SchemaProps{
                        Type:        []string{"array"},
                        Description: "Conditions of the placement",
                        Items: &spec.SchemaOrArray{
                            Schema: &spec.Schema{
                                SchemaProps: spec.SchemaProps{
                                    Type: []string{"object"},
                                },
                            },
                        },
                    },
                },
                "selectedClusters": {
                    SchemaProps: spec.SchemaProps{
                        Type:        []string{"array"},
                        Description: "Actually selected clusters",
                        Items: &spec.SchemaOrArray{
                            Schema: &spec.Schema{
                                SchemaProps: spec.SchemaProps{
                                    Type: []string{"string"},
                                },
                            },
                        },
                    },
                },
            },
        },
    }
}

// addTMCPaths adds TMC API paths
func (p *OpenAPIProvider) addTMCPaths() {
    // WorkloadPlacement paths
    p.spec.Paths.Paths["/apis/tmc.kcp.dev/v1alpha1/workloadplacements"] = spec.PathItem{
        PathItemProps: spec.PathItemProps{
            Get: &spec.Operation{
                OperationProps: spec.OperationProps{
                    Description: "List WorkloadPlacements",
                    Tags:        []string{"WorkloadPlacement"},
                    Responses: &spec.Responses{
                        ResponsesProps: spec.ResponsesProps{
                            StatusCodeResponses: map[int]spec.Response{
                                200: {
                                    ResponseProps: spec.ResponseProps{
                                        Description: "OK",
                                        Schema: &spec.Schema{
                                            SchemaProps: spec.SchemaProps{
                                                Ref: spec.MustCreateRef("#/definitions/io.kcp.tmc.v1alpha1.WorkloadPlacementList"),
                                            },
                                        },
                                    },
                                },
                            },
                        },
                    },
                },
            },
            Post: &spec.Operation{
                OperationProps: spec.OperationProps{
                    Description: "Create a WorkloadPlacement",
                    Tags:        []string{"WorkloadPlacement"},
                    Parameters: []spec.Parameter{
                        {
                            ParamProps: spec.ParamProps{
                                Name:     "body",
                                In:       "body",
                                Required: true,
                                Schema: &spec.Schema{
                                    SchemaProps: spec.SchemaProps{
                                        Ref: spec.MustCreateRef("#/definitions/io.kcp.tmc.v1alpha1.WorkloadPlacement"),
                                    },
                                },
                            },
                        },
                    },
                    Responses: &spec.Responses{
                        ResponsesProps: spec.ResponsesProps{
                            StatusCodeResponses: map[int]spec.Response{
                                201: {
                                    ResponseProps: spec.ResponseProps{
                                        Description: "Created",
                                        Schema: &spec.Schema{
                                            SchemaProps: spec.SchemaProps{
                                                Ref: spec.MustCreateRef("#/definitions/io.kcp.tmc.v1alpha1.WorkloadPlacement"),
                                            },
                                        },
                                    },
                                },
                            },
                        },
                    },
                },
            },
        },
    }
}

// GetSpec returns the OpenAPI specification
func (p *OpenAPIProvider) GetSpec() (*spec.Swagger, error) {
    p.mutex.RLock()
    defer p.mutex.RUnlock()
    
    if p.spec == nil {
        return nil, fmt.Errorf("OpenAPI spec not initialized")
    }
    
    return p.spec, nil
}

// UpdateSpec updates the OpenAPI specification
func (p *OpenAPIProvider) UpdateSpec(aggregated *AggregatedDiscovery) error {
    p.mutex.Lock()
    defer p.mutex.Unlock()
    
    // Update based on aggregated discovery
    // This would merge discovered APIs into the spec
    
    klog.V(4).Info("OpenAPI spec updated")
    return nil
}
```

### Step 3: Implement Resource Discovery (Hour 5)

```go
// pkg/virtual/syncer/discovery/resource_discovery.go
package discovery

import (
    "context"
    "fmt"
    "net/http"
    
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/runtime/schema"
    "k8s.io/klog/v2"
)

// ResourceDiscovery provides resource discovery endpoints
type ResourceDiscovery struct {
    manager *DiscoveryManager
}

// NewResourceDiscovery creates a new resource discovery handler
func NewResourceDiscovery(manager *DiscoveryManager) *ResourceDiscovery {
    return &ResourceDiscovery{
        manager: manager,
    }
}

// ServeHTTP handles discovery requests
func (rd *ResourceDiscovery) ServeHTTP(w http.ResponseWriter, req *http.Request) {
    path := req.URL.Path
    
    switch {
    case path == "/apis":
        rd.handleAPIGroups(w, req)
    case strings.HasPrefix(path, "/apis/") && strings.Count(path, "/") == 2:
        rd.handleGroupVersion(w, req)
    case path == "/openapi/v2":
        rd.handleOpenAPI(w, req)
    default:
        http.Error(w, "Not Found", http.StatusNotFound)
    }
}

// handleAPIGroups handles /apis requests
func (rd *ResourceDiscovery) handleAPIGroups(w http.ResponseWriter, req *http.Request) {
    if req.Method != http.MethodGet {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }
    
    groups := rd.manager.GetAPIGroups()
    rd.writeJSON(w, groups)
}

// handleGroupVersion handles /apis/{group}/{version} requests
func (rd *ResourceDiscovery) handleGroupVersion(w http.ResponseWriter, req *http.Request) {
    if req.Method != http.MethodGet {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }
    
    // Extract group/version from path
    parts := strings.Split(strings.TrimPrefix(req.URL.Path, "/apis/"), "/")
    if len(parts) != 2 {
        http.Error(w, "Invalid path", http.StatusBadRequest)
        return
    }
    
    groupVersion := parts[0] + "/" + parts[1]
    resources, err := rd.manager.GetAPIResourceList(groupVersion)
    if err != nil {
        http.Error(w, err.Error(), http.StatusNotFound)
        return
    }
    
    rd.writeJSON(w, resources)
}

// handleOpenAPI handles /openapi/v2 requests
func (rd *ResourceDiscovery) handleOpenAPI(w http.ResponseWriter, req *http.Request) {
    if req.Method != http.MethodGet {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }
    
    spec, err := rd.manager.GetOpenAPISpec()
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    rd.writeJSON(w, spec)
}

// writeJSON writes a JSON response
func (rd *ResourceDiscovery) writeJSON(w http.ResponseWriter, obj interface{}) {
    w.Header().Set("Content-Type", "application/json")
    encoder := json.NewEncoder(w)
    encoder.SetIndent("", "  ")
    
    if err := encoder.Encode(obj); err != nil {
        klog.Errorf("Failed to encode response: %v", err)
    }
}

// DiscoverResources discovers available resources in the virtual workspace
func DiscoverResources(ctx context.Context, vw VirtualWorkspace) (*DiscoveredResources, error) {
    discovered := &DiscoveredResources{
        Groups:    make(map[string]*metav1.APIGroup),
        Resources: make(map[schema.GroupVersion]*metav1.APIResourceList),
    }
    
    // Discover from virtual workspace providers
    providers := vw.GetProviders()
    for resourceType, provider := range providers {
        gvr := getGVRForResource(resourceType)
        
        // Add to group if not exists
        if _, exists := discovered.Groups[gvr.Group]; !exists {
            discovered.Groups[gvr.Group] = &metav1.APIGroup{
                Name: gvr.Group,
                Versions: []metav1.GroupVersionForDiscovery{
                    {
                        GroupVersion: gvr.GroupVersion().String(),
                        Version:      gvr.Version,
                    },
                },
            }
        }
        
        // Add resource
        gv := gvr.GroupVersion()
        if _, exists := discovered.Resources[gv]; !exists {
            discovered.Resources[gv] = &metav1.APIResourceList{
                GroupVersion: gv.String(),
                APIResources: []metav1.APIResource{},
            }
        }
        
        apiResource := metav1.APIResource{
            Name:         gvr.Resource,
            Kind:         getKindForResource(resourceType),
            Verbs:        provider.GetVerbs(),
            Namespaced:   provider.IsNamespaced(),
        }
        
        discovered.Resources[gv].APIResources = append(
            discovered.Resources[gv].APIResources,
            apiResource,
        )
    }
    
    return discovered, nil
}

// DiscoveredResources holds discovered resources
type DiscoveredResources struct {
    Groups    map[string]*metav1.APIGroup
    Resources map[schema.GroupVersion]*metav1.APIResourceList
}

// getGVRForResource returns the GVR for a resource type
func getGVRForResource(resourceType string) schema.GroupVersionResource {
    switch resourceType {
    case "workloadplacements":
        return tmcv1alpha1.SchemeGroupVersion.WithResource("workloadplacements")
    case "synctargets":
        return tmcv1alpha1.SchemeGroupVersion.WithResource("synctargets")
    case "clusterregistrations":
        return tmcv1alpha1.SchemeGroupVersion.WithResource("clusterregistrations")
    default:
        return schema.GroupVersionResource{}
    }
}

// getKindForResource returns the kind for a resource type
func getKindForResource(resourceType string) string {
    switch resourceType {
    case "workloadplacements":
        return "WorkloadPlacement"
    case "synctargets":
        return "SyncTarget"
    case "clusterregistrations":
        return "ClusterRegistration"
    default:
        return ""
    }
}
```

### Step 4: Implement Schema Store (Hour 6)

```go
// pkg/virtual/syncer/discovery/schema_store.go
package discovery

import (
    "fmt"
    "sync"
    
    "k8s.io/apimachinery/pkg/runtime/schema"
    "k8s.io/apiserver/pkg/endpoints/openapi"
    "k8s.io/klog/v2"
)

// SchemaStore stores and manages resource schemas
type SchemaStore struct {
    schemas map[schema.GroupVersionKind]interface{}
    mutex   sync.RWMutex
}

// NewSchemaStore creates a new schema store
func NewSchemaStore() *SchemaStore {
    return &SchemaStore{
        schemas: make(map[schema.GroupVersionKind]interface{}),
    }
}

// RegisterSchema registers a schema for a GVK
func (s *SchemaStore) RegisterSchema(gvk schema.GroupVersionKind, schema interface{}) error {
    s.mutex.Lock()
    defer s.mutex.Unlock()
    
    if _, exists := s.schemas[gvk]; exists {
        return fmt.Errorf("schema already registered for %s", gvk)
    }
    
    s.schemas[gvk] = schema
    klog.V(4).Infof("Registered schema for %s", gvk)
    
    return nil
}

// GetSchema retrieves a schema for a GVK
func (s *SchemaStore) GetSchema(gvk schema.GroupVersionKind) (interface{}, error) {
    s.mutex.RLock()
    defer s.mutex.RUnlock()
    
    schema, exists := s.schemas[gvk]
    if !exists {
        return nil, fmt.Errorf("schema not found for %s", gvk)
    }
    
    return schema, nil
}

// UpdateSchema updates a schema for a GVK
func (s *SchemaStore) UpdateSchema(gvk schema.GroupVersionKind, schema interface{}) error {
    s.mutex.Lock()
    defer s.mutex.Unlock()
    
    s.schemas[gvk] = schema
    klog.V(4).Infof("Updated schema for %s", gvk)
    
    return nil
}

// DeleteSchema removes a schema for a GVK
func (s *SchemaStore) DeleteSchema(gvk schema.GroupVersionKind) {
    s.mutex.Lock()
    defer s.mutex.Unlock()
    
    delete(s.schemas, gvk)
    klog.V(4).Infof("Deleted schema for %s", gvk)
}

// ListSchemas returns all registered GVKs
func (s *SchemaStore) ListSchemas() []schema.GroupVersionKind {
    s.mutex.RLock()
    defer s.mutex.RUnlock()
    
    gvks := make([]schema.GroupVersionKind, 0, len(s.schemas))
    for gvk := range s.schemas {
        gvks = append(gvks, gvk)
    }
    
    return gvks
}

// ValidateAgainstSchema validates an object against its schema
func (s *SchemaStore) ValidateAgainstSchema(gvk schema.GroupVersionKind, obj interface{}) error {
    schema, err := s.GetSchema(gvk)
    if err != nil {
        return err
    }
    
    // Perform validation (simplified)
    // In production, this would use proper JSON schema validation
    if schema == nil {
        return fmt.Errorf("schema is nil for %s", gvk)
    }
    
    return nil
}

// getWorkloadPlacementSchema returns the schema for WorkloadPlacement
func getWorkloadPlacementSchema() interface{} {
    // In production, this would return a proper JSON schema
    return map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "spec": map[string]interface{}{
                "type": "object",
                "required": []string{"workload"},
            },
            "status": map[string]interface{}{
                "type": "object",
            },
        },
    }
}

// getSyncTargetSchema returns the schema for SyncTarget
func getSyncTargetSchema() interface{} {
    return map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "spec": map[string]interface{}{
                "type": "object",
                "required": []string{"kubeConfig"},
            },
            "status": map[string]interface{}{
                "type": "object",
            },
        },
    }
}

// getClusterRegistrationSchema returns the schema for ClusterRegistration
func getClusterRegistrationSchema() interface{} {
    return map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "spec": map[string]interface{}{
                "type": "object",
                "required": []string{"location"},
            },
            "status": map[string]interface{}{
                "type": "object",
            },
        },
    }
}
```

### Step 5: Implement Discovery Aggregator (Hour 7)

```go
// pkg/virtual/syncer/discovery/aggregator.go
package discovery

import (
    "context"
    "fmt"
    "time"
    
    "github.com/kcp-dev/kcp/pkg/virtual/syncer"
    
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/runtime/schema"
    "k8s.io/apimachinery/pkg/util/cache"
)

// DiscoveryAggregator aggregates discovery information
type DiscoveryAggregator struct {
    cache *cache.LRUExpireCache
    ttl   time.Duration
}

// AggregatedDiscovery holds aggregated discovery data
type AggregatedDiscovery struct {
    Groups    map[string]*metav1.APIGroup
    Resources map[schema.GroupVersion]*metav1.APIResourceList
    Timestamp time.Time
}

// NewDiscoveryAggregator creates a new aggregator
func NewDiscoveryAggregator(ttl time.Duration) *DiscoveryAggregator {
    return &DiscoveryAggregator{
        cache: cache.NewLRUExpireCache(100),
        ttl:   ttl,
    }
}

// Aggregate aggregates discovery from virtual workspace
func (a *DiscoveryAggregator) Aggregate(ctx context.Context, vw *syncer.VirtualWorkspace) (*AggregatedDiscovery, error) {
    // Check cache
    if cached := a.getFromCache("main"); cached != nil {
        return cached, nil
    }
    
    // Aggregate from virtual workspace
    aggregated := &AggregatedDiscovery{
        Groups:    make(map[string]*metav1.APIGroup),
        Resources: make(map[schema.GroupVersion]*metav1.APIResourceList),
        Timestamp: time.Now(),
    }
    
    // Discover resources
    discovered, err := DiscoverResources(ctx, vw)
    if err != nil {
        return nil, fmt.Errorf("failed to discover resources: %w", err)
    }
    
    aggregated.Groups = discovered.Groups
    aggregated.Resources = discovered.Resources
    
    // Cache result
    a.cache.Add("main", aggregated, a.ttl)
    
    return aggregated, nil
}

// getFromCache retrieves from cache
func (a *DiscoveryAggregator) getFromCache(key string) *AggregatedDiscovery {
    if obj, exists := a.cache.Get(key); exists {
        return obj.(*AggregatedDiscovery)
    }
    return nil
}

// ClearCache clears the cache
func (a *DiscoveryAggregator) ClearCache() {
    a.cache = cache.NewLRUExpireCache(100)
}
```

## Testing Requirements

### Unit Tests

1. **Discovery Manager Tests**
   - Test initialization
   - Test API group registration
   - Test resource discovery
   - Test refresh logic

2. **OpenAPI Provider Tests**
   - Test spec generation
   - Test definition creation
   - Test path generation
   - Test spec updates

3. **Resource Discovery Tests**
   - Test discovery endpoints
   - Test resource listing
   - Test filtering

4. **Schema Store Tests**
   - Test schema registration
   - Test validation
   - Test updates

5. **Aggregator Tests**
   - Test aggregation
   - Test caching
   - Test TTL expiration

### Integration Tests

1. **End-to-End Discovery**
   - Test complete discovery flow
   - Test OpenAPI generation
   - Test client discovery

2. **Cache Behavior**
   - Test cache invalidation
   - Test refresh cycles

## KCP Patterns to Follow

### Discovery Standards
- Follow Kubernetes discovery patterns
- Provide standard discovery endpoints
- Support client-go discovery

### OpenAPI Compliance
- Generate valid OpenAPI v2 specs
- Include all required fields
- Support schema validation

### Caching Strategy
- Cache discovery data appropriately
- Implement TTL-based expiration
- Handle cache invalidation

## Integration Points

### With VW Core (p1w2-vw-core)
- Discovers available providers
- Uses virtual workspace for data
- Shares resource information

### With VW Endpoints (p1w2-vw-endpoints)
- Provides discovery endpoints
- Shares API information
- Enables client discovery

## Validation Checklist

### Before Commit
- [ ] All files created as specified
- [ ] Line count under 500 (run `/workspaces/kcp-shared-tools/tmc-pr-line-counter.sh`)
- [ ] All tests passing (`make test`)
- [ ] Linting passes (`make lint`)
- [ ] OpenAPI spec valid

### Functionality Complete
- [ ] Discovery manager operational
- [ ] OpenAPI spec generated
- [ ] Resource discovery works
- [ ] Schema validation functional
- [ ] Aggregation working

### Integration Ready
- [ ] Integrates with VW core
- [ ] Discovery endpoints accessible
- [ ] OpenAPI available
- [ ] Client discovery works

### Documentation Complete
- [ ] Discovery endpoints documented
- [ ] OpenAPI spec documented
- [ ] Schema formats documented
- [ ] Client usage documented

## Commit Message Template
```
feat(discovery): implement Virtual Workspace discovery mechanisms

- Add discovery manager with API group registration
- Implement OpenAPI specification generation
- Add resource discovery endpoints
- Implement schema store and validation
- Add discovery aggregation with caching
- Ensure Kubernetes-compatible discovery

Part of TMC Phase 1 Wave 3 implementation
Depends on: p1w2-vw-core
```

## Next Steps
After this branch is complete:
1. Clients can discover TMC APIs
2. OpenAPI documentation available
3. Schema validation operational