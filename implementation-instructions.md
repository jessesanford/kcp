# Implementation Instructions: Discovery Contracts & Abstraction Layer

## Overview
- **Branch**: feature/tmc-phase4-vw-04-discovery-contracts
- **Purpose**: Implement resource discovery abstraction with provider registry, mock provider for testing, and schema management
- **Target Lines**: 350
- **Dependencies**: Branch vw-03 (workspace abstractions)
- **Estimated Time**: 2 days

## Files to Create

### 1. pkg/virtual/discovery/interface.go (50 lines)
**Purpose**: Define the core discovery provider interface

**Interfaces/Types to Define**:
```go
package discovery

import (
    "context"
    
    "k8s.io/apimachinery/pkg/runtime/schema"
    "k8s.io/client-go/discovery"
    "github.com/kcp-dev/kcp/pkg/virtual/workspace"
)

// Provider handles resource discovery for virtual workspaces
type Provider interface {
    // Name returns the provider name
    Name() string
    
    // Initialize sets up the provider
    Initialize(ctx context.Context, config ProviderConfig) error
    
    // Discover returns available resources for a workspace
    Discover(ctx context.Context, workspaceName string) (*DiscoveryResult, error)
    
    // GetOpenAPISchema returns OpenAPI schema for resources
    GetOpenAPISchema(ctx context.Context, workspaceName string, gvr schema.GroupVersionResource) ([]byte, error)
    
    // Watch monitors for discovery changes
    Watch(ctx context.Context, workspaceName string) (<-chan DiscoveryEvent, error)
    
    // Refresh forces a refresh of discovery data
    Refresh(ctx context.Context, workspaceName string) error
    
    // Close cleans up provider resources
    Close(ctx context.Context) error
}

// ProviderConfig configures a discovery provider
type ProviderConfig struct {
    // WorkspaceManager for workspace operations
    WorkspaceManager workspace.Manager
    
    // CacheEnabled enables discovery caching
    CacheEnabled bool
    
    // CacheTTL is cache time-to-live
    CacheTTL int64
    
    // RefreshInterval for automatic refresh
    RefreshInterval int64
}

// DiscoveryResult contains discovery information
type DiscoveryResult struct {
    // Groups lists API groups
    Groups []discovery.APIGroup
    
    // Resources maps GVR to resource info
    Resources map[schema.GroupVersionResource]ResourceInfo
    
    // PreferredVersions maps group to preferred version
    PreferredVersions map[string]string
}

// ResourceInfo contains detailed resource information
type ResourceInfo struct {
    discovery.APIResource
    
    // Schema contains OpenAPI schema
    Schema []byte
    
    // WorkspaceScoped indicates workspace scoping
    WorkspaceScoped bool
}

// DiscoveryEvent represents a discovery change
type DiscoveryEvent struct {
    Type      EventType
    Workspace string
    Resource  *ResourceInfo
    Error     error
}

type EventType string

const (
    EventTypeResourceAdded   EventType = "ResourceAdded"
    EventTypeResourceRemoved EventType = "ResourceRemoved"
    EventTypeResourceUpdated EventType = "ResourceUpdated"
)
```

### 2. pkg/virtual/discovery/mock_provider.go (100 lines)
**Purpose**: Implement a mock discovery provider for testing

**Implementation**:
```go
package discovery

import (
    "context"
    "fmt"
    "sync"
    
    "k8s.io/apimachinery/pkg/runtime/schema"
    "k8s.io/client-go/discovery"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MockProvider provides mock discovery for testing
type MockProvider struct {
    mu        sync.RWMutex
    name      string
    resources map[string]*DiscoveryResult
    schemas   map[string][]byte
    watchers  map[string][]chan DiscoveryEvent
    config    ProviderConfig
}

// NewMockProvider creates a new mock discovery provider
func NewMockProvider(name string) *MockProvider {
    return &MockProvider{
        name:      name,
        resources: make(map[string]*DiscoveryResult),
        schemas:   make(map[string][]byte),
        watchers:  make(map[string][]chan DiscoveryEvent),
    }
}

// Name returns the provider name
func (m *MockProvider) Name() string {
    return m.name
}

// Initialize sets up the mock provider
func (m *MockProvider) Initialize(ctx context.Context, config ProviderConfig) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    m.config = config
    
    // Initialize with default resources
    m.setupDefaultResources()
    
    return nil
}

// Discover returns mock resources
func (m *MockProvider) Discover(ctx context.Context, workspaceName string) (*DiscoveryResult, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()
    
    result, ok := m.resources[workspaceName]
    if !ok {
        // Return default resources for unknown workspaces
        return m.getDefaultDiscoveryResult(), nil
    }
    
    return result, nil
}

// GetOpenAPISchema returns mock schema
func (m *MockProvider) GetOpenAPISchema(ctx context.Context, workspaceName string, gvr schema.GroupVersionResource) ([]byte, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()
    
    key := fmt.Sprintf("%s/%s", workspaceName, gvr.String())
    schema, ok := m.schemas[key]
    if !ok {
        return []byte("{}"), nil // Return empty schema
    }
    
    return schema, nil
}

// Watch monitors for mock discovery changes
func (m *MockProvider) Watch(ctx context.Context, workspaceName string) (<-chan DiscoveryEvent, error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    ch := make(chan DiscoveryEvent, 10)
    m.watchers[workspaceName] = append(m.watchers[workspaceName], ch)
    
    return ch, nil
}

// Refresh triggers a mock refresh
func (m *MockProvider) Refresh(ctx context.Context, workspaceName string) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    // Simulate refresh by sending event to watchers
    event := DiscoveryEvent{
        Type:      EventTypeResourceUpdated,
        Workspace: workspaceName,
    }
    
    for _, ch := range m.watchers[workspaceName] {
        select {
        case ch <- event:
        default:
            // Channel full, skip
        }
    }
    
    return nil
}

// Close cleans up mock provider
func (m *MockProvider) Close(ctx context.Context) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    // Close all watcher channels
    for _, watchers := range m.watchers {
        for _, ch := range watchers {
            close(ch)
        }
    }
    
    m.watchers = make(map[string][]chan DiscoveryEvent)
    return nil
}

// setupDefaultResources initializes default mock resources
func (m *MockProvider) setupDefaultResources() {
    defaultResult := m.getDefaultDiscoveryResult()
    m.resources["default"] = defaultResult
}

// getDefaultDiscoveryResult returns default discovery result
func (m *MockProvider) getDefaultDiscoveryResult() *DiscoveryResult {
    return &DiscoveryResult{
        Groups: []discovery.APIGroup{
            {
                Name: "apps",
                Versions: []discovery.GroupVersionForDiscovery{
                    {GroupVersion: "apps/v1", Version: "v1"},
                },
                PreferredVersion: discovery.GroupVersionForDiscovery{
                    GroupVersion: "apps/v1",
                    Version:      "v1",
                },
            },
        },
        Resources: map[schema.GroupVersionResource]ResourceInfo{
            {Group: "apps", Version: "v1", Resource: "deployments"}: {
                APIResource: discovery.APIResource{
                    Name:       "deployments",
                    Namespaced: true,
                    Kind:       "Deployment",
                    Verbs:      []string{"get", "list", "watch", "create", "update", "patch", "delete"},
                },
                WorkspaceScoped: true,
            },
        },
        PreferredVersions: map[string]string{
            "apps": "v1",
        },
    }
}

// AddResource adds a mock resource for testing
func (m *MockProvider) AddResource(workspaceName string, gvr schema.GroupVersionResource, resource ResourceInfo) {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    if _, ok := m.resources[workspaceName]; !ok {
        m.resources[workspaceName] = &DiscoveryResult{
            Resources: make(map[schema.GroupVersionResource]ResourceInfo),
        }
    }
    
    m.resources[workspaceName].Resources[gvr] = resource
}
```

### 3. pkg/virtual/discovery/registry.go (80 lines)
**Purpose**: Implement a registry for discovery providers

**Implementation**:
```go
package discovery

import (
    "context"
    "fmt"
    "sync"
)

// Registry manages discovery providers
type Registry struct {
    mu        sync.RWMutex
    providers map[string]Provider
    default   string
}

// NewRegistry creates a new provider registry
func NewRegistry() *Registry {
    return &Registry{
        providers: make(map[string]Provider),
    }
}

// Register adds a provider to the registry
func (r *Registry) Register(name string, provider Provider) error {
    r.mu.Lock()
    defer r.mu.Unlock()
    
    if _, exists := r.providers[name]; exists {
        return fmt.Errorf("provider %s already registered", name)
    }
    
    r.providers[name] = provider
    
    // Set as default if first provider
    if len(r.providers) == 1 {
        r.default = name
    }
    
    return nil
}

// Unregister removes a provider from the registry
func (r *Registry) Unregister(name string) error {
    r.mu.Lock()
    defer r.mu.Unlock()
    
    if _, exists := r.providers[name]; !exists {
        return fmt.Errorf("provider %s not found", name)
    }
    
    delete(r.providers, name)
    
    // Clear default if it was removed
    if r.default == name {
        r.default = ""
        // Set new default if providers remain
        for name := range r.providers {
            r.default = name
            break
        }
    }
    
    return nil
}

// Get retrieves a provider by name
func (r *Registry) Get(name string) (Provider, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    
    provider, exists := r.providers[name]
    if !exists {
        return nil, fmt.Errorf("provider %s not found", name)
    }
    
    return provider, nil
}

// GetDefault retrieves the default provider
func (r *Registry) GetDefault() (Provider, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    
    if r.default == "" {
        return nil, fmt.Errorf("no default provider set")
    }
    
    return r.providers[r.default], nil
}

// SetDefault sets the default provider
func (r *Registry) SetDefault(name string) error {
    r.mu.Lock()
    defer r.mu.Unlock()
    
    if _, exists := r.providers[name]; !exists {
        return fmt.Errorf("provider %s not found", name)
    }
    
    r.default = name
    return nil
}

// List returns all registered provider names
func (r *Registry) List() []string {
    r.mu.RLock()
    defer r.mu.RUnlock()
    
    names := make([]string, 0, len(r.providers))
    for name := range r.providers {
        names = append(names, name)
    }
    
    return names
}

// Close closes all registered providers
func (r *Registry) Close(ctx context.Context) error {
    r.mu.Lock()
    defer r.mu.Unlock()
    
    var lastErr error
    for name, provider := range r.providers {
        if err := provider.Close(ctx); err != nil {
            lastErr = fmt.Errorf("failed to close provider %s: %w", name, err)
        }
    }
    
    r.providers = make(map[string]Provider)
    r.default = ""
    
    return lastErr
}
```

### 4. pkg/virtual/discovery/schema_manager.go (70 lines)
**Purpose**: Manage OpenAPI schemas for discovered resources

**Implementation**:
```go
package discovery

import (
    "context"
    "encoding/json"
    "fmt"
    "sync"
    
    "k8s.io/apimachinery/pkg/runtime/schema"
    "k8s.io/kube-openapi/pkg/validation/spec"
)

// SchemaManager manages OpenAPI schemas
type SchemaManager struct {
    mu      sync.RWMutex
    schemas map[string]*spec.Schema
    merged  *spec.Swagger
}

// NewSchemaManager creates a new schema manager
func NewSchemaManager() *SchemaManager {
    return &SchemaManager{
        schemas: make(map[string]*spec.Schema),
        merged: &spec.Swagger{
            SwaggerProps: spec.SwaggerProps{
                Swagger: "2.0",
                Info: &spec.Info{
                    InfoProps: spec.InfoProps{
                        Title:   "Virtual Workspace API",
                        Version: "v1alpha1",
                    },
                },
                Paths: &spec.Paths{
                    Paths: make(map[string]spec.PathItem),
                },
                Definitions: spec.Definitions{},
            },
        },
    }
}

// AddSchema adds a schema for a resource
func (sm *SchemaManager) AddSchema(gvr schema.GroupVersionResource, schemaBytes []byte) error {
    sm.mu.Lock()
    defer sm.mu.Unlock()
    
    var schema spec.Schema
    if err := json.Unmarshal(schemaBytes, &schema); err != nil {
        return fmt.Errorf("failed to unmarshal schema: %w", err)
    }
    
    key := gvr.String()
    sm.schemas[key] = &schema
    
    // Update merged schema
    sm.updateMergedSchema()
    
    return nil
}

// GetSchema retrieves schema for a resource
func (sm *SchemaManager) GetSchema(gvr schema.GroupVersionResource) (*spec.Schema, bool) {
    sm.mu.RLock()
    defer sm.mu.RUnlock()
    
    schema, ok := sm.schemas[gvr.String()]
    return schema, ok
}

// GetMergedSchema returns the merged OpenAPI document
func (sm *SchemaManager) GetMergedSchema() *spec.Swagger {
    sm.mu.RLock()
    defer sm.mu.RUnlock()
    
    return sm.merged
}

// RemoveSchema removes a schema
func (sm *SchemaManager) RemoveSchema(gvr schema.GroupVersionResource) {
    sm.mu.Lock()
    defer sm.mu.Unlock()
    
    delete(sm.schemas, gvr.String())
    sm.updateMergedSchema()
}

// Clear removes all schemas
func (sm *SchemaManager) Clear() {
    sm.mu.Lock()
    defer sm.mu.Unlock()
    
    sm.schemas = make(map[string]*spec.Schema)
    sm.updateMergedSchema()
}

// updateMergedSchema rebuilds the merged OpenAPI document
func (sm *SchemaManager) updateMergedSchema() {
    // In a real implementation, this would merge all schemas
    // For now, we just ensure definitions exist
    if sm.merged.Definitions == nil {
        sm.merged.Definitions = spec.Definitions{}
    }
    
    for key, schema := range sm.schemas {
        sm.merged.Definitions[key] = *schema
    }
}

// ValidateAgainstSchema validates an object against its schema
func (sm *SchemaManager) ValidateAgainstSchema(gvr schema.GroupVersionResource, obj interface{}) error {
    sm.mu.RLock()
    defer sm.mu.RUnlock()
    
    schema, ok := sm.schemas[gvr.String()]
    if !ok {
        return fmt.Errorf("no schema found for %s", gvr.String())
    }
    
    // In a real implementation, this would perform validation
    // For now, we just check that schema exists
    if schema == nil {
        return fmt.Errorf("schema is nil for %s", gvr.String())
    }
    
    return nil
}
```

### 5. pkg/virtual/discovery/mock_provider_test.go (50 lines)
**Purpose**: Test the mock discovery provider

**Test Implementation**:
```go
package discovery

import (
    "context"
    "testing"
    
    "k8s.io/apimachinery/pkg/runtime/schema"
)

func TestMockProvider(t *testing.T) {
    ctx := context.Background()
    provider := NewMockProvider("test-provider")
    
    // Test initialization
    config := ProviderConfig{
        CacheEnabled: true,
        CacheTTL:     60,
    }
    
    if err := provider.Initialize(ctx, config); err != nil {
        t.Fatalf("Failed to initialize provider: %v", err)
    }
    
    // Test discovery
    result, err := provider.Discover(ctx, "default")
    if err != nil {
        t.Fatalf("Failed to discover resources: %v", err)
    }
    
    if len(result.Groups) == 0 {
        t.Error("Expected at least one API group")
    }
    
    if len(result.Resources) == 0 {
        t.Error("Expected at least one resource")
    }
    
    // Test schema retrieval
    gvr := schema.GroupVersionResource{
        Group:    "apps",
        Version:  "v1",
        Resource: "deployments",
    }
    
    schema, err := provider.GetOpenAPISchema(ctx, "default", gvr)
    if err != nil {
        t.Fatalf("Failed to get schema: %v", err)
    }
    
    if len(schema) == 0 {
        t.Error("Expected non-empty schema")
    }
    
    // Test watch
    ch, err := provider.Watch(ctx, "test-workspace")
    if err != nil {
        t.Fatalf("Failed to watch: %v", err)
    }
    
    // Trigger refresh
    if err := provider.Refresh(ctx, "test-workspace"); err != nil {
        t.Fatalf("Failed to refresh: %v", err)
    }
    
    // Check for event
    select {
    case event := <-ch:
        if event.Type != EventTypeResourceUpdated {
            t.Errorf("Expected ResourceUpdated event, got %s", event.Type)
        }
    default:
        t.Error("Expected to receive an event")
    }
    
    // Test cleanup
    if err := provider.Close(ctx); err != nil {
        t.Fatalf("Failed to close provider: %v", err)
    }
}
```

## Implementation Steps

1. **Create package structure**:
   - Create `pkg/virtual/discovery/` directory
   - Add package documentation

2. **Implement core interfaces**:
   - Start with `interface.go` to define provider contract
   - Add `mock_provider.go` for testing
   - Create `registry.go` for provider management
   - Add `schema_manager.go` for OpenAPI handling

3. **Add test coverage**:
   - Test mock provider functionality
   - Test registry operations
   - Test schema management

4. **Ensure extensibility**:
   - Design for multiple provider implementations
   - Support dynamic provider registration
   - Enable schema composition

## Testing Requirements
- Unit test coverage: >80%
- Test scenarios:
  - Mock provider operations
  - Registry management
  - Schema validation
  - Resource discovery
  - Watch functionality

## Integration Points
- Uses: Workspace abstractions from branch vw-03
- Provides: Discovery abstraction for future provider implementations

## Acceptance Criteria
- [ ] Discovery provider interface defined
- [ ] Mock provider fully functional
- [ ] Registry supports multiple providers
- [ ] Schema manager handles OpenAPI
- [ ] Tests pass with good coverage
- [ ] Documentation complete
- [ ] Follows KCP patterns
- [ ] No linting errors

## Common Pitfalls
- **Don't hardcode provider types**: Use registry pattern
- **Handle concurrent access**: Use proper locking
- **Cache discovery results**: Avoid excessive API calls
- **Validate schemas**: Ensure OpenAPI compliance
- **Clean up resources**: Proper provider lifecycle
- **Test edge cases**: Empty results, errors, timeouts

## Code Review Focus
- Thread safety in registry
- Provider lifecycle management
- Schema validation completeness
- Error handling patterns
- Mock provider realism