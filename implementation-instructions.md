# Implementation Instructions: Authorization Framework

## Overview
- **Branch**: feature/tmc-phase4-vw-05-auth-contracts
- **Purpose**: Implement pluggable authorization framework with basic RBAC provider and context propagation
- **Target Lines**: 400
- **Dependencies**: Branch vw-04 (discovery contracts)
- **Estimated Time**: 2 days

## Files to Create

### 1. pkg/virtual/auth/interface.go (60 lines)
**Purpose**: Define core authorization interfaces

**Interfaces/Types to Define**:
```go
package auth

import (
    "context"
    
    authv1 "k8s.io/api/authorization/v1"
    "k8s.io/apimachinery/pkg/runtime/schema"
)

// Provider handles authorization for virtual workspaces
type Provider interface {
    // Name returns the provider name
    Name() string
    
    // Initialize sets up the provider
    Initialize(ctx context.Context, config ProviderConfig) error
    
    // Authorize performs an authorization check
    Authorize(ctx context.Context, req *Request) (*Decision, error)
    
    // GetPermissions returns user permissions in a workspace
    GetPermissions(ctx context.Context, workspace, user string) ([]Permission, error)
    
    // RefreshCache updates cached authorization data
    RefreshCache(ctx context.Context, workspace string) error
    
    // Close cleans up provider resources
    Close(ctx context.Context) error
}

// ProviderConfig configures an authorization provider
type ProviderConfig struct {
    // KubeConfig for accessing Kubernetes APIs
    KubeConfig string
    
    // CacheEnabled enables permission caching
    CacheEnabled bool
    
    // CacheTTL is cache time-to-live in seconds
    CacheTTL int64
    
    // AuditEnabled enables audit logging
    AuditEnabled bool
}

// Request represents an authorization request
type Request struct {
    // User making the request
    User string
    
    // Groups the user belongs to
    Groups []string
    
    // Workspace being accessed
    Workspace string
    
    // Resource being accessed
    Resource schema.GroupVersionResource
    
    // ResourceName for specific resource access
    ResourceName string
    
    // Verb being performed
    Verb string
    
    // Extra contains additional attributes
    Extra map[string][]string
}

// Decision represents an authorization decision
type Decision struct {
    // Allowed indicates if the request is authorized
    Allowed bool
    
    // Reason provides explanation for the decision
    Reason string
    
    // EvaluationError contains any error during evaluation
    EvaluationError error
    
    // AuditAnnotations for audit logging
    AuditAnnotations map[string]string
}

// Permission represents a granted permission
type Permission struct {
    // Resource this permission applies to
    Resource schema.GroupVersionResource
    
    // Verbs allowed on the resource
    Verbs []string
    
    // ResourceNames for specific resource access
    ResourceNames []string
    
    // NonResourceURLs for non-resource access
    NonResourceURLs []string
}
```

### 2. pkg/virtual/auth/basic_provider.go (120 lines)
**Purpose**: Implement a basic RBAC authorization provider

**Implementation**:
```go
package auth

import (
    "context"
    "fmt"
    "sync"
    
    authv1 "k8s.io/api/authorization/v1"
    "k8s.io/apimachinery/pkg/runtime/schema"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/rest"
)

// BasicProvider implements basic RBAC authorization
type BasicProvider struct {
    mu          sync.RWMutex
    name        string
    config      ProviderConfig
    kubeClient  kubernetes.Interface
    permissions map[string][]Permission // workspace:user -> permissions
    cache       *PermissionCache
}

// NewBasicProvider creates a new basic authorization provider
func NewBasicProvider(name string) *BasicProvider {
    return &BasicProvider{
        name:        name,
        permissions: make(map[string][]Permission),
    }
}

// Name returns the provider name
func (p *BasicProvider) Name() string {
    return p.name
}

// Initialize sets up the basic provider
func (p *BasicProvider) Initialize(ctx context.Context, config ProviderConfig) error {
    p.mu.Lock()
    defer p.mu.Unlock()
    
    p.config = config
    
    // Initialize Kubernetes client if needed
    if config.KubeConfig != "" {
        restConfig, err := rest.InClusterConfig()
        if err != nil {
            return fmt.Errorf("failed to get in-cluster config: %w", err)
        }
        
        client, err := kubernetes.NewForConfig(restConfig)
        if err != nil {
            return fmt.Errorf("failed to create kubernetes client: %w", err)
        }
        
        p.kubeClient = client
    }
    
    // Initialize cache if enabled
    if config.CacheEnabled {
        p.cache = NewPermissionCache(config.CacheTTL)
    }
    
    // Load initial permissions
    p.loadDefaultPermissions()
    
    return nil
}

// Authorize performs an authorization check
func (p *BasicProvider) Authorize(ctx context.Context, req *Request) (*Decision, error) {
    p.mu.RLock()
    defer p.mu.RUnlock()
    
    // Check cache first
    if p.cache != nil {
        if decision, ok := p.cache.Get(req); ok {
            return decision, nil
        }
    }
    
    // Perform authorization check
    decision := p.performAuthorization(req)
    
    // Cache the decision
    if p.cache != nil {
        p.cache.Set(req, decision)
    }
    
    // Audit log if enabled
    if p.config.AuditEnabled {
        p.auditLog(req, decision)
    }
    
    return decision, nil
}

// GetPermissions returns user permissions in a workspace
func (p *BasicProvider) GetPermissions(ctx context.Context, workspace, user string) ([]Permission, error) {
    p.mu.RLock()
    defer p.mu.RUnlock()
    
    key := fmt.Sprintf("%s:%s", workspace, user)
    permissions, ok := p.permissions[key]
    if !ok {
        return []Permission{}, nil
    }
    
    return permissions, nil
}

// RefreshCache updates cached authorization data
func (p *BasicProvider) RefreshCache(ctx context.Context, workspace string) error {
    p.mu.Lock()
    defer p.mu.Unlock()
    
    if p.cache != nil {
        p.cache.InvalidateWorkspace(workspace)
    }
    
    // Reload permissions for workspace
    // In a real implementation, this would fetch from Kubernetes RBAC
    
    return nil
}

// Close cleans up provider resources
func (p *BasicProvider) Close(ctx context.Context) error {
    p.mu.Lock()
    defer p.mu.Unlock()
    
    if p.cache != nil {
        p.cache.Clear()
    }
    
    return nil
}

// performAuthorization performs the actual authorization check
func (p *BasicProvider) performAuthorization(req *Request) *Decision {
    key := fmt.Sprintf("%s:%s", req.Workspace, req.User)
    permissions, ok := p.permissions[key]
    if !ok {
        return &Decision{
            Allowed: false,
            Reason:  "No permissions found for user in workspace",
        }
    }
    
    // Check if any permission matches the request
    for _, perm := range permissions {
        if p.matchesPermission(req, perm) {
            return &Decision{
                Allowed: true,
                Reason:  "Permission granted by RBAC",
                AuditAnnotations: map[string]string{
                    "authorization.k8s.io/decision": "allow",
                    "authorization.k8s.io/reason":   "RBAC",
                },
            }
        }
    }
    
    return &Decision{
        Allowed: false,
        Reason:  "No matching permission found",
        AuditAnnotations: map[string]string{
            "authorization.k8s.io/decision": "deny",
            "authorization.k8s.io/reason":   "RBAC: no matching rule",
        },
    }
}

// matchesPermission checks if a permission matches a request
func (p *BasicProvider) matchesPermission(req *Request, perm Permission) bool {
    // Check resource match
    if perm.Resource != req.Resource {
        return false
    }
    
    // Check verb match
    verbMatches := false
    for _, verb := range perm.Verbs {
        if verb == req.Verb || verb == "*" {
            verbMatches = true
            break
        }
    }
    
    if !verbMatches {
        return false
    }
    
    // Check resource name if specified
    if req.ResourceName != "" && len(perm.ResourceNames) > 0 {
        nameMatches := false
        for _, name := range perm.ResourceNames {
            if name == req.ResourceName || name == "*" {
                nameMatches = true
                break
            }
        }
        return nameMatches
    }
    
    return true
}

// loadDefaultPermissions loads default permissions for testing
func (p *BasicProvider) loadDefaultPermissions() {
    // Add default admin permissions
    p.permissions["default:admin"] = []Permission{
        {
            Resource: schema.GroupVersionResource{Group: "*", Version: "*", Resource: "*"},
            Verbs:    []string{"*"},
        },
    }
    
    // Add default viewer permissions
    p.permissions["default:viewer"] = []Permission{
        {
            Resource: schema.GroupVersionResource{Group: "*", Version: "*", Resource: "*"},
            Verbs:    []string{"get", "list", "watch"},
        },
    }
}

// auditLog logs authorization decisions
func (p *BasicProvider) auditLog(req *Request, decision *Decision) {
    // In a real implementation, this would write to audit log
    // For now, we just print to stdout for debugging
}
```

### 3. pkg/virtual/auth/rbac_evaluator.go (100 lines)
**Purpose**: Implement RBAC evaluation logic

**Implementation**:
```go
package auth

import (
    "context"
    "fmt"
    
    rbacv1 "k8s.io/api/rbac/v1"
    "k8s.io/apimachinery/pkg/runtime/schema"
    "k8s.io/client-go/kubernetes"
)

// RBACEvaluator evaluates RBAC rules
type RBACEvaluator struct {
    client kubernetes.Interface
}

// NewRBACEvaluator creates a new RBAC evaluator
func NewRBACEvaluator(client kubernetes.Interface) *RBACEvaluator {
    return &RBACEvaluator{
        client: client,
    }
}

// Evaluate evaluates RBAC rules for a request
func (e *RBACEvaluator) Evaluate(ctx context.Context, req *Request) (*Decision, error) {
    // Get roles for user
    roles, err := e.getRolesForUser(ctx, req.User, req.Groups, req.Workspace)
    if err != nil {
        return nil, fmt.Errorf("failed to get roles: %w", err)
    }
    
    // Evaluate each role
    for _, role := range roles {
        if e.roleAllows(role, req) {
            return &Decision{
                Allowed: true,
                Reason:  fmt.Sprintf("Allowed by role: %s", role.Name),
            }, nil
        }
    }
    
    return &Decision{
        Allowed: false,
        Reason:  "No role grants permission",
    }, nil
}

// getRolesForUser gets all roles bound to a user
func (e *RBACEvaluator) getRolesForUser(ctx context.Context, user string, groups []string, namespace string) ([]*rbacv1.Role, error) {
    // Get role bindings
    roleBindings, err := e.client.RbacV1().RoleBindings(namespace).List(ctx, metav1.ListOptions{})
    if err != nil {
        return nil, err
    }
    
    var roles []*rbacv1.Role
    
    for _, binding := range roleBindings.Items {
        // Check if user is in subjects
        if e.subjectMatchesUser(binding.Subjects, user, groups) {
            // Get the role
            role, err := e.client.RbacV1().Roles(namespace).Get(ctx, binding.RoleRef.Name, metav1.GetOptions{})
            if err != nil {
                continue // Skip if role not found
            }
            roles = append(roles, role)
        }
    }
    
    // Also check cluster role bindings
    clusterRoleBindings, err := e.client.RbacV1().ClusterRoleBindings().List(ctx, metav1.ListOptions{})
    if err != nil {
        return roles, nil // Return what we have
    }
    
    for _, binding := range clusterRoleBindings.Items {
        if e.subjectMatchesUser(binding.Subjects, user, groups) {
            // Get the cluster role
            clusterRole, err := e.client.RbacV1().ClusterRoles().Get(ctx, binding.RoleRef.Name, metav1.GetOptions{})
            if err != nil {
                continue
            }
            // Convert to role for uniform handling
            roles = append(roles, &rbacv1.Role{
                Rules: clusterRole.Rules,
                ObjectMeta: metav1.ObjectMeta{
                    Name: clusterRole.Name,
                },
            })
        }
    }
    
    return roles, nil
}

// subjectMatchesUser checks if subjects include the user
func (e *RBACEvaluator) subjectMatchesUser(subjects []rbacv1.Subject, user string, groups []string) bool {
    for _, subject := range subjects {
        switch subject.Kind {
        case "User":
            if subject.Name == user {
                return true
            }
        case "Group":
            for _, group := range groups {
                if subject.Name == group {
                    return true
                }
            }
        }
    }
    return false
}

// roleAllows checks if a role allows the request
func (e *RBACEvaluator) roleAllows(role *rbacv1.Role, req *Request) bool {
    for _, rule := range role.Rules {
        if e.ruleMatches(rule, req) {
            return true
        }
    }
    return false
}

// ruleMatches checks if a policy rule matches the request
func (e *RBACEvaluator) ruleMatches(rule rbacv1.PolicyRule, req *Request) bool {
    // Check API groups
    groupMatches := false
    for _, group := range rule.APIGroups {
        if group == "*" || group == req.Resource.Group {
            groupMatches = true
            break
        }
    }
    
    if !groupMatches && len(rule.APIGroups) > 0 {
        return false
    }
    
    // Check resources
    resourceMatches := false
    for _, resource := range rule.Resources {
        if resource == "*" || resource == req.Resource.Resource {
            resourceMatches = true
            break
        }
    }
    
    if !resourceMatches && len(rule.Resources) > 0 {
        return false
    }
    
    // Check verbs
    for _, verb := range rule.Verbs {
        if verb == "*" || verb == req.Verb {
            return true
        }
    }
    
    return false
}
```

### 4. pkg/virtual/auth/context.go (60 lines)
**Purpose**: Manage authorization context propagation

**Implementation**:
```go
package auth

import (
    "context"
    
    "k8s.io/apiserver/pkg/authentication/user"
)

// contextKey is the type for context keys
type contextKey string

const (
    // UserContextKey is the context key for user info
    UserContextKey contextKey = "virtual-workspace-user"
    
    // WorkspaceContextKey is the context key for workspace
    WorkspaceContextKey contextKey = "virtual-workspace"
    
    // DecisionContextKey is the context key for auth decision
    DecisionContextKey contextKey = "virtual-workspace-auth-decision"
)

// WithUser adds user info to context
func WithUser(ctx context.Context, userInfo user.Info) context.Context {
    return context.WithValue(ctx, UserContextKey, userInfo)
}

// GetUser retrieves user info from context
func GetUser(ctx context.Context) (user.Info, bool) {
    userInfo, ok := ctx.Value(UserContextKey).(user.Info)
    return userInfo, ok
}

// WithWorkspace adds workspace to context
func WithWorkspace(ctx context.Context, workspace string) context.Context {
    return context.WithValue(ctx, WorkspaceContextKey, workspace)
}

// GetWorkspace retrieves workspace from context
func GetWorkspace(ctx context.Context) (string, bool) {
    workspace, ok := ctx.Value(WorkspaceContextKey).(string)
    return workspace, ok
}

// WithDecision adds authorization decision to context
func WithDecision(ctx context.Context, decision *Decision) context.Context {
    return context.WithValue(ctx, DecisionContextKey, decision)
}

// GetDecision retrieves authorization decision from context
func GetDecision(ctx context.Context) (*Decision, bool) {
    decision, ok := ctx.Value(DecisionContextKey).(*Decision)
    return decision, ok
}

// ExtractAuthInfo extracts all auth info from context
func ExtractAuthInfo(ctx context.Context) *AuthInfo {
    info := &AuthInfo{}
    
    if userInfo, ok := GetUser(ctx); ok {
        info.User = userInfo.GetName()
        info.Groups = userInfo.GetGroups()
        info.Extra = userInfo.GetExtra()
    }
    
    if workspace, ok := GetWorkspace(ctx); ok {
        info.Workspace = workspace
    }
    
    if decision, ok := GetDecision(ctx); ok {
        info.Decision = decision
    }
    
    return info
}

// AuthInfo contains all authentication/authorization info
type AuthInfo struct {
    User      string
    Groups    []string
    Extra     map[string][]string
    Workspace string
    Decision  *Decision
}
```

### 5. pkg/virtual/auth/cache.go (70 lines)
**Purpose**: Implement permission caching

**Implementation**:
```go
package auth

import (
    "fmt"
    "sync"
    "time"
)

// PermissionCache caches authorization decisions
type PermissionCache struct {
    mu       sync.RWMutex
    cache    map[string]*CacheEntry
    ttl      time.Duration
}

// CacheEntry represents a cached decision
type CacheEntry struct {
    Decision  *Decision
    ExpiresAt time.Time
}

// NewPermissionCache creates a new permission cache
func NewPermissionCache(ttlSeconds int64) *PermissionCache {
    return &PermissionCache{
        cache: make(map[string]*CacheEntry),
        ttl:   time.Duration(ttlSeconds) * time.Second,
    }
}

// Get retrieves a cached decision
func (c *PermissionCache) Get(req *Request) (*Decision, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    
    key := c.requestKey(req)
    entry, ok := c.cache[key]
    if !ok {
        return nil, false
    }
    
    // Check if expired
    if time.Now().After(entry.ExpiresAt) {
        return nil, false
    }
    
    return entry.Decision, true
}

// Set caches a decision
func (c *PermissionCache) Set(req *Request, decision *Decision) {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    key := c.requestKey(req)
    c.cache[key] = &CacheEntry{
        Decision:  decision,
        ExpiresAt: time.Now().Add(c.ttl),
    }
}

// InvalidateWorkspace removes all cached decisions for a workspace
func (c *PermissionCache) InvalidateWorkspace(workspace string) {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    // Remove entries matching workspace
    for key := range c.cache {
        if containsWorkspace(key, workspace) {
            delete(c.cache, key)
        }
    }
}

// Clear removes all cached decisions
func (c *PermissionCache) Clear() {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    c.cache = make(map[string]*CacheEntry)
}

// requestKey generates a cache key for a request
func (c *PermissionCache) requestKey(req *Request) string {
    return fmt.Sprintf("%s:%s:%s:%s:%s:%s",
        req.Workspace,
        req.User,
        req.Resource.String(),
        req.ResourceName,
        req.Verb,
        fmt.Sprint(req.Groups),
    )
}

// containsWorkspace checks if a key contains the workspace
func containsWorkspace(key, workspace string) bool {
    // Simple check - in production would parse the key properly
    return len(key) > len(workspace) && key[:len(workspace)] == workspace
}

// CleanupExpired removes expired entries
func (c *PermissionCache) CleanupExpired() {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    now := time.Now()
    for key, entry := range c.cache {
        if now.After(entry.ExpiresAt) {
            delete(c.cache, key)
        }
    }
}
```

### 6. pkg/virtual/auth/basic_provider_test.go (60 lines)
**Purpose**: Test the basic authorization provider

**Test Implementation**:
```go
package auth

import (
    "context"
    "testing"
    
    "k8s.io/apimachinery/pkg/runtime/schema"
)

func TestBasicProvider(t *testing.T) {
    ctx := context.Background()
    provider := NewBasicProvider("test-provider")
    
    // Test initialization
    config := ProviderConfig{
        CacheEnabled: true,
        CacheTTL:     60,
        AuditEnabled: false,
    }
    
    if err := provider.Initialize(ctx, config); err != nil {
        t.Fatalf("Failed to initialize provider: %v", err)
    }
    
    // Test admin authorization
    adminReq := &Request{
        User:      "admin",
        Workspace: "default",
        Resource: schema.GroupVersionResource{
            Group:    "apps",
            Version:  "v1",
            Resource: "deployments",
        },
        Verb: "create",
    }
    
    decision, err := provider.Authorize(ctx, adminReq)
    if err != nil {
        t.Fatalf("Failed to authorize admin: %v", err)
    }
    
    if !decision.Allowed {
        t.Error("Expected admin to be authorized")
    }
    
    // Test viewer authorization for write
    viewerReq := &Request{
        User:      "viewer",
        Workspace: "default",
        Resource: schema.GroupVersionResource{
            Group:    "apps",
            Version:  "v1",
            Resource: "deployments",
        },
        Verb: "create",
    }
    
    decision, err = provider.Authorize(ctx, viewerReq)
    if err != nil {
        t.Fatalf("Failed to authorize viewer: %v", err)
    }
    
    if decision.Allowed {
        t.Error("Expected viewer to be denied for create")
    }
    
    // Test viewer authorization for read
    viewerReq.Verb = "get"
    decision, err = provider.Authorize(ctx, viewerReq)
    if err != nil {
        t.Fatalf("Failed to authorize viewer for get: %v", err)
    }
    
    if !decision.Allowed {
        t.Error("Expected viewer to be allowed for get")
    }
    
    // Test get permissions
    permissions, err := provider.GetPermissions(ctx, "default", "admin")
    if err != nil {
        t.Fatalf("Failed to get permissions: %v", err)
    }
    
    if len(permissions) == 0 {
        t.Error("Expected admin to have permissions")
    }
}
```

## Implementation Steps

1. **Create package structure**:
   - Create `pkg/virtual/auth/` directory
   - Add package documentation

2. **Implement core interfaces**:
   - Start with `interface.go` for provider contract
   - Add `basic_provider.go` for basic RBAC
   - Create `rbac_evaluator.go` for RBAC logic
   - Add `context.go` for context propagation

3. **Add caching layer**:
   - Implement `cache.go` for permission caching
   - Include TTL-based expiration
   - Support workspace invalidation

4. **Add test coverage**:
   - Test basic provider functionality
   - Test RBAC evaluation
   - Test caching behavior

## Testing Requirements
- Unit test coverage: >80%
- Test scenarios:
  - Admin permissions
  - Viewer permissions
  - Permission denial
  - Cache hits/misses
  - Context propagation

## Integration Points
- Uses: Discovery contracts from branch vw-04
- Provides: Authorization framework for workspace providers

## Acceptance Criteria
- [ ] Authorization provider interface defined
- [ ] Basic RBAC provider implemented
- [ ] RBAC evaluator functional
- [ ] Context propagation working
- [ ] Permission caching implemented
- [ ] Tests pass with good coverage
- [ ] Documentation complete
- [ ] Follows KCP patterns
- [ ] No linting errors

## Common Pitfalls
- **Handle cache invalidation**: Ensure consistency
- **Secure context propagation**: Don't leak sensitive data
- **Audit all decisions**: Important for security
- **Test permission edge cases**: Empty permissions, wildcards
- **Handle concurrent access**: Thread-safe operations
- **Clean up resources**: Proper provider lifecycle

## Code Review Focus
- Security of authorization logic
- Cache invalidation correctness
- RBAC evaluation accuracy
- Context security
- Performance under load