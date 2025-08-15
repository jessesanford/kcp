# Implementation Instructions: KCP Authorization Integration

## Overview
- **Branch**: feature/tmc-phase4-vw-08-auth-integration
- **Purpose**: Implement KCP authorization integration for virtual workspaces with workspace-aware access control and RBAC integration
- **Target Lines**: 400
- **Dependencies**: 
  - vw-01-api-contracts (interfaces and contracts)
  - vw-05-auth-contracts (authorization interfaces)
- **Estimated Time**: 2.5 days

## Files to Create

### 1. pkg/virtual/authorization/kcp_authorizer.go (120 lines)
**Purpose**: Main KCP authorization provider that integrates with KCP's RBAC system and workspace isolation

**Key Structs/Interfaces**:
```go
package authorization

import (
    "context"
    "fmt"
    "sync"
    
    authzv1 "k8s.io/api/authorization/v1"
    rbacv1 "k8s.io/api/rbac/v1"
    "k8s.io/apimachinery/pkg/runtime/schema"
    "k8s.io/apiserver/pkg/authorization/authorizer"
    "k8s.io/client-go/tools/cache"
    "k8s.io/klog/v2"
    
    "github.com/kcp-dev/logicalcluster/v3"
    kcpclient "github.com/kcp-dev/kcp/sdk/client/clientset/versioned"
    kcpinformers "github.com/kcp-dev/kcp/sdk/client/informers/externalversions"
    
    "github.com/kcp-dev/kcp/pkg/virtual/interfaces"
    "github.com/kcp-dev/kcp/pkg/virtual/contracts"
)

// KCPAuthorizationProvider implements AuthorizationProvider for KCP environments
type KCPAuthorizationProvider struct {
    // delegate is the underlying KCP authorizer
    delegate authorizer.Authorizer
    
    // kcpClient provides access to KCP APIs
    kcpClient kcpclient.ClusterInterface
    
    // informerFactory provides shared informers for RBAC resources
    informerFactory kcpinformers.SharedInformerFactory
    
    // workspace is the logical cluster this provider serves
    workspace logicalcluster.Name
    
    // cache provides caching for authorization decisions
    cache AuthorizationCache
    
    // rbacInformers track RBAC resource changes
    clusterRoleInformer cache.SharedIndexInformer
    clusterRoleBindingInformer cache.SharedIndexInformer
    roleInformer cache.SharedIndexInformer
    roleBindingInformer cache.SharedIndexInformer
    
    // mutex protects concurrent access
    mutex sync.RWMutex
    
    // started indicates if the provider has been started
    started bool
    
    // stopCh signals shutdown
    stopCh <-chan struct{}
}

// NewKCPAuthorizationProvider creates a new KCP authorization provider
func NewKCPAuthorizationProvider(
    delegate authorizer.Authorizer,
    kcpClient kcpclient.ClusterInterface,
    informerFactory kcpinformers.SharedInformerFactory,
    workspace logicalcluster.Name,
) (*KCPAuthorizationProvider, error) {
    // Implementation details
}

// Start initializes the authorization provider and begins monitoring RBAC changes
func (p *KCPAuthorizationProvider) Start(ctx context.Context) error {
    // Implementation details
}

// Authorize determines if a request is allowed within the workspace context
func (p *KCPAuthorizationProvider) Authorize(ctx context.Context, req *interfaces.AuthorizationRequest) (*interfaces.AuthorizationDecision, error) {
    // Implementation details
}

// GetPermissions returns permissions for a user in the workspace
func (p *KCPAuthorizationProvider) GetPermissions(ctx context.Context, workspace, user string) ([]interfaces.Permission, error) {
    // Implementation details
}

// RefreshCache updates cached authorization data for the workspace
func (p *KCPAuthorizationProvider) RefreshCache(ctx context.Context, workspace string) error {
    // Implementation details
}

// buildAuthorizerAttributes converts request to authorizer attributes
func (p *KCPAuthorizationProvider) buildAuthorizerAttributes(req *interfaces.AuthorizationRequest) authorizer.Attributes {
    // Implementation details
}

// validateWorkspaceAccess ensures the user can access the specified workspace
func (p *KCPAuthorizationProvider) validateWorkspaceAccess(ctx context.Context, workspace, user string, groups []string) error {
    // Implementation details
}

// extractUserPermissions extracts permissions from RBAC rules for a user
func (p *KCPAuthorizationProvider) extractUserPermissions(ctx context.Context, user string, groups []string) ([]interfaces.Permission, error) {
    // Implementation details
}
```

### 2. pkg/virtual/authorization/cache.go (80 lines)
**Purpose**: Caching layer for authorization decisions with workspace awareness

**Key Structs/Interfaces**:
```go
package authorization

import (
    "crypto/sha256"
    "fmt"
    "sync"
    "time"
    
    "github.com/kcp-dev/kcp/pkg/virtual/interfaces"
)

// AuthorizationCache provides caching for authorization decisions
type AuthorizationCache interface {
    // GetDecision retrieves a cached authorization decision
    GetDecision(key string) (*interfaces.AuthorizationDecision, bool)
    
    // SetDecision caches an authorization decision
    SetDecision(key string, decision *interfaces.AuthorizationDecision, ttl time.Duration)
    
    // InvalidateUser removes all cached decisions for a user
    InvalidateUser(user string)
    
    // InvalidateWorkspace removes all cached decisions for a workspace
    InvalidateWorkspace(workspace string)
    
    // Clear removes all cached decisions
    Clear()
}

// MemoryAuthorizationCache provides in-memory caching for authorization decisions
type MemoryAuthorizationCache struct {
    // entries stores cached authorization decisions
    entries map[string]*authCacheEntry
    
    // userIndex maps users to their cache keys
    userIndex map[string][]string
    
    // workspaceIndex maps workspaces to their cache keys
    workspaceIndex map[string][]string
    
    // mutex protects concurrent access
    mutex sync.RWMutex
    
    // defaultTTL is the default cache expiration time
    defaultTTL time.Duration
    
    // cleanupInterval determines how often to run cache cleanup
    cleanupInterval time.Duration
    
    // stopCh signals shutdown for cleanup goroutine
    stopCh chan struct{}
}

// authCacheEntry represents a cached authorization decision
type authCacheEntry struct {
    // decision is the cached authorization decision
    decision *interfaces.AuthorizationDecision
    
    // expireAt is when this entry expires
    expireAt time.Time
    
    // user is the user this decision applies to
    user string
    
    // workspace is the workspace this decision applies to
    workspace string
}

// NewMemoryAuthorizationCache creates a new memory-based authorization cache
func NewMemoryAuthorizationCache(defaultTTL, cleanupInterval time.Duration) *MemoryAuthorizationCache {
    // Implementation details
}

// Start begins cache cleanup operations
func (c *MemoryAuthorizationCache) Start() {
    // Implementation details
}

// Stop terminates cache cleanup operations
func (c *MemoryAuthorizationCache) Stop() {
    // Implementation details
}

// GetDecision retrieves a cached authorization decision
func (c *MemoryAuthorizationCache) GetDecision(key string) (*interfaces.AuthorizationDecision, bool) {
    // Implementation details
}

// SetDecision caches an authorization decision
func (c *MemoryAuthorizationCache) SetDecision(key string, decision *interfaces.AuthorizationDecision, ttl time.Duration) {
    // Implementation details
}

// InvalidateUser removes all cached decisions for a user
func (c *MemoryAuthorizationCache) InvalidateUser(user string) {
    // Implementation details
}

// InvalidateWorkspace removes all cached decisions for a workspace
func (c *MemoryAuthorizationCache) InvalidateWorkspace(workspace string) {
    // Implementation details
}

// Clear removes all cached decisions
func (c *MemoryAuthorizationCache) Clear() {
    // Implementation details
}

// generateCacheKey creates a cache key from request parameters
func generateCacheKey(user, workspace, resource, verb, resourceName string) string {
    // Implementation details
}
```

### 3. pkg/virtual/authorization/rbac_analyzer.go (90 lines)
**Purpose**: Analyzes RBAC rules to determine permissions within workspace context

**Key Structs/Interfaces**:
```go
package authorization

import (
    "context"
    "strings"
    
    rbacv1 "k8s.io/api/rbac/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/runtime/schema"
    "k8s.io/apimachinery/pkg/labels"
    rbacv1listers "k8s.io/client-go/listers/rbac/v1"
    
    "github.com/kcp-dev/logicalcluster/v3"
    
    "github.com/kcp-dev/kcp/pkg/virtual/interfaces"
)

// RBACAnalyzer analyzes RBAC rules to extract permissions
type RBACAnalyzer struct {
    // clusterRoleLister lists cluster roles
    clusterRoleLister rbacv1listers.ClusterRoleLister
    
    // clusterRoleBindingLister lists cluster role bindings
    clusterRoleBindingLister rbacv1listers.ClusterRoleBindingLister
    
    // roleLister lists roles
    roleLister rbacv1listers.RoleLister
    
    // roleBindingLister lists role bindings
    roleBindingLister rbacv1listers.RoleBindingLister
    
    // workspace is the logical cluster for analysis
    workspace logicalcluster.Name
}

// NewRBACAnalyzer creates a new RBAC analyzer
func NewRBACAnalyzer(
    clusterRoleLister rbacv1listers.ClusterRoleLister,
    clusterRoleBindingLister rbacv1listers.ClusterRoleBindingLister,
    roleLister rbacv1listers.RoleLister,
    roleBindingLister rbacv1listers.RoleBindingLister,
    workspace logicalcluster.Name,
) *RBACAnalyzer {
    // Implementation details
}

// AnalyzePermissions extracts permissions for a user from RBAC rules
func (a *RBACAnalyzer) AnalyzePermissions(ctx context.Context, user string, groups []string) ([]interfaces.Permission, error) {
    // Implementation details
}

// CheckAccess verifies if a user has specific access based on RBAC rules
func (a *RBACAnalyzer) CheckAccess(ctx context.Context, user string, groups []string, gvr schema.GroupVersionResource, verb, resourceName string) (bool, string, error) {
    // Implementation details
}

// extractClusterPermissions extracts permissions from cluster roles and bindings
func (a *RBACAnalyzer) extractClusterPermissions(ctx context.Context, user string, groups []string) ([]interfaces.Permission, error) {
    // Implementation details
}

// extractNamespacePermissions extracts permissions from roles and bindings
func (a *RBACAnalyzer) extractNamespacePermissions(ctx context.Context, user string, groups []string) ([]interfaces.Permission, error) {
    // Implementation details
}

// isUserBoundToRole checks if a user is bound to a specific role
func (a *RBACAnalyzer) isUserBoundToRole(subject rbacv1.Subject, user string, groups []string) bool {
    // Implementation details
}

// convertPolicyRulesToPermissions converts RBAC policy rules to permissions
func (a *RBACAnalyzer) convertPolicyRulesToPermissions(rules []rbacv1.PolicyRule) []interfaces.Permission {
    // Implementation details
}

// matchesResource checks if a policy rule matches a specific resource
func (a *RBACAnalyzer) matchesResource(rule rbacv1.PolicyRule, gvr schema.GroupVersionResource) bool {
    // Implementation details
}

// matchesVerb checks if a policy rule allows a specific verb
func (a *RBACAnalyzer) matchesVerb(rule rbacv1.PolicyRule, verb string) bool {
    // Implementation details
}

// matchesResourceName checks if a policy rule matches a specific resource name
func (a *RBACAnalyzer) matchesResourceName(rule rbacv1.PolicyRule, resourceName string) bool {
    // Implementation details
}
```

### 4. pkg/virtual/authorization/workspace_filter.go (60 lines)
**Purpose**: Workspace-aware filtering for authorization decisions

**Key Structs/Interfaces**:
```go
package authorization

import (
    "context"
    "fmt"
    "strings"
    
    "github.com/kcp-dev/logicalcluster/v3"
    
    "github.com/kcp-dev/kcp/pkg/virtual/interfaces"
)

// WorkspaceFilter provides workspace-aware authorization filtering
type WorkspaceFilter struct {
    // workspace is the logical cluster for filtering
    workspace logicalcluster.Name
    
    // allowCrossWorkspaceAccess determines if cross-workspace access is allowed
    allowCrossWorkspaceAccess bool
}

// NewWorkspaceFilter creates a new workspace filter
func NewWorkspaceFilter(workspace logicalcluster.Name, allowCrossWorkspaceAccess bool) *WorkspaceFilter {
    // Implementation details
}

// FilterRequest validates and filters authorization requests for workspace boundaries
func (f *WorkspaceFilter) FilterRequest(ctx context.Context, req *interfaces.AuthorizationRequest) (*interfaces.AuthorizationRequest, error) {
    // Implementation details
}

// ValidateWorkspaceAccess checks if access to a workspace is allowed
func (f *WorkspaceFilter) ValidateWorkspaceAccess(ctx context.Context, targetWorkspace, user string, groups []string) error {
    // Implementation details
}

// IsWorkspaceResource determines if a resource is workspace-scoped
func (f *WorkspaceFilter) IsWorkspaceResource(gvr schema.GroupVersionResource) bool {
    // Implementation details
}

// ExtractWorkspaceFromPath extracts workspace information from request path
func (f *WorkspaceFilter) ExtractWorkspaceFromPath(path string) (string, error) {
    // Implementation details
}

// ResolveLogicalCluster resolves a workspace name to a logical cluster
func (f *WorkspaceFilter) ResolveLogicalCluster(workspace string) (logicalcluster.Name, error) {
    // Implementation details
}

// EnforceWorkspaceIsolation ensures requests don't cross workspace boundaries
func (f *WorkspaceFilter) EnforceWorkspaceIsolation(ctx context.Context, req *interfaces.AuthorizationRequest) error {
    // Implementation details
}
```

### 5. pkg/virtual/authorization/metrics.go (50 lines)
**Purpose**: Metrics collection for authorization operations

**Key Structs/Interfaces**:
```go
package authorization

import (
    "time"
    
    "github.com/prometheus/client_golang/prometheus"
    "k8s.io/component-base/metrics"
)

var (
    // authorizationRequestsTotal counts total authorization requests
    authorizationRequestsTotal = metrics.NewCounterVec(
        &metrics.CounterOpts{
            Name: "kcp_virtual_authorization_requests_total",
            Help: "Total number of authorization requests handled",
        },
        []string{"workspace", "user", "result"},
    )
    
    // authorizationRequestDuration measures authorization request duration
    authorizationRequestDuration = metrics.NewHistogramVec(
        &metrics.HistogramOpts{
            Name: "kcp_virtual_authorization_request_duration_seconds",
            Help: "Duration of authorization requests in seconds",
            Buckets: prometheus.ExponentialBuckets(0.001, 2, 15),
        },
        []string{"workspace", "operation"},
    )
    
    // authorizationCacheHits counts cache hits/misses
    authorizationCacheHits = metrics.NewCounterVec(
        &metrics.CounterOpts{
            Name: "kcp_virtual_authorization_cache_hits_total",
            Help: "Total number of authorization cache hits",
        },
        []string{"workspace", "hit_type"},
    )
    
    // authorizationDenials tracks authorization denials
    authorizationDenials = metrics.NewCounterVec(
        &metrics.CounterOpts{
            Name: "kcp_virtual_authorization_denials_total",
            Help: "Total number of authorization denials",
        },
        []string{"workspace", "user", "resource", "verb"},
    )
)

// init registers metrics
func init() {
    metrics.MustRegister(
        authorizationRequestsTotal,
        authorizationRequestDuration,
        authorizationCacheHits,
        authorizationDenials,
    )
}

// RecordAuthorizationRequest records metrics for an authorization request
func RecordAuthorizationRequest(workspace, user string, duration time.Duration, allowed bool, err error) {
    // Implementation details
}

// RecordCacheHit records an authorization cache hit or miss
func RecordCacheHit(workspace string, hit bool) {
    // Implementation details
}

// RecordDenial records an authorization denial
func RecordDenial(workspace, user, resource, verb string) {
    // Implementation details
}
```

## Implementation Steps

1. **Create authorization package structure**:
   - Create `pkg/virtual/authorization/` directory
   - Implement core KCP authorization provider

2. **Implement caching layer**:
   - Create memory-based authorization cache
   - Add cache invalidation strategies
   - Implement TTL management with user/workspace indexing

3. **Add RBAC analysis**:
   - Implement RBAC rule analyzer
   - Extract permissions from roles and bindings
   - Handle both cluster and namespace-scoped permissions

4. **Build workspace filtering**:
   - Implement workspace isolation enforcement
   - Add cross-workspace access controls
   - Validate workspace boundaries

5. **Add metrics collection**:
   - Implement Prometheus metrics
   - Track authorization performance
   - Monitor cache effectiveness and denials

## Testing Requirements
- Unit test coverage: >90%
- Test scenarios:
  - RBAC rule analysis and permission extraction
  - Authorization decision caching
  - Workspace isolation enforcement
  - Cross-workspace access controls
  - Error handling and recovery
  - Concurrent access patterns

## Integration Points
- Uses: `pkg/virtual/interfaces` for authorization contracts
- Uses: KCP RBAC informers and listers
- Uses: KCP logical cluster utilities
- Uses: Kubernetes authorizer framework
- Provides: AuthorizationProvider implementation

## Acceptance Criteria
- [ ] Implements AuthorizationProvider interface completely
- [ ] Integrates with KCP RBAC system
- [ ] Provides efficient authorization caching
- [ ] Enforces workspace isolation boundaries
- [ ] Handles both cluster and namespace-scoped permissions
- [ ] Includes comprehensive error handling
- [ ] Provides Prometheus metrics
- [ ] Passes all unit tests
- [ ] Follows KCP architectural patterns
- [ ] Maintains security boundaries

## Common Pitfalls
- **RBAC complexity**: Handle both cluster and namespace-scoped RBAC correctly
- **Cache consistency**: Ensure cache invalidation matches RBAC changes
- **Workspace isolation**: Never allow unauthorized cross-workspace access
- **Permission evaluation**: Correctly evaluate complex RBAC rule combinations
- **Security boundaries**: Maintain strict security isolation between workspaces
- **Performance**: Avoid expensive RBAC evaluations on every request
- **Logical clusters**: Properly resolve and validate logical cluster references

## Code Review Focus
- RBAC rule evaluation correctness
- Workspace isolation enforcement
- Authorization cache consistency
- Security boundary maintenance
- Error handling completeness
- Performance optimization
- Metrics coverage and usefulness