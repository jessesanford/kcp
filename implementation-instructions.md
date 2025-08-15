# Implementation Instructions: Workspace Discovery (Branch 16)

## Overview
This branch implements workspace traversal and cluster discovery mechanisms. It provides efficient methods to discover workspaces, find clusters within them, check permissions, and maintain a hierarchical view of the workspace structure with caching for performance.

## Dependencies
- **Base**: feature/tmc-phase4-13-placement-interfaces
- **Uses interfaces from**: Branch 13
- **Required for**: Branches 18, 19

## Files to Create

### 1. `pkg/placement/discovery/traverser.go` (120 lines)
Workspace traversal implementation.

```go
package discovery

import (
    "context"
    "fmt"
    "github.com/kcp-dev/kcp/pkg/placement/interfaces"
    kcpclient "github.com/kcp-dev/kcp/pkg/client/clientset/versioned"
    "github.com/kcp-dev/logicalcluster/v3"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/labels"
)

// WorkspaceTraverser implements workspace discovery and traversal
type WorkspaceTraverser struct {
    client      kcpclient.Interface
    cache       *HierarchyCache
    permissions *PermissionChecker
}

// NewWorkspaceTraverser creates a new workspace traverser
func NewWorkspaceTraverser(client kcpclient.Interface) *WorkspaceTraverser {
    return &WorkspaceTraverser{
        client:      client,
        cache:       NewHierarchyCache(),
        permissions: NewPermissionChecker(client),
    }
}

// ListWorkspaces lists workspaces matching the selector
func (t *WorkspaceTraverser) ListWorkspaces(ctx context.Context, selector labels.Selector) ([]interfaces.Workspace, error) {
    // Check cache first
    if cached, ok := t.cache.GetWorkspaces(selector.String()); ok {
        return cached, nil
    }
    
    workspaces := []interfaces.Workspace{}
    
    // Start from root workspace
    root := logicalcluster.NewPath("root")
    if err := t.traverseWorkspace(ctx, root, selector, &workspaces); err != nil {
        return nil, fmt.Errorf("failed to traverse workspaces: %w", err)
    }
    
    // Cache the results
    t.cache.PutWorkspaces(selector.String(), workspaces)
    
    return workspaces, nil
}

// traverseWorkspace recursively traverses workspace hierarchy
func (t *WorkspaceTraverser) traverseWorkspace(ctx context.Context, path logicalcluster.Path, 
    selector labels.Selector, workspaces *[]interfaces.Workspace) error {
    
    // Check permissions for this workspace
    canAccess, err := t.permissions.CheckAccess(ctx, path.String(), "list")
    if err != nil {
        return err
    }
    if !canAccess {
        return nil // Skip inaccessible workspaces
    }
    
    // Get workspace metadata
    ws, err := t.getWorkspaceInfo(ctx, path)
    if err != nil {
        return err
    }
    
    // Check if workspace matches selector
    if selector.Matches(labels.Set(ws.Labels)) {
        *workspaces = append(*workspaces, ws)
    }
    
    // Traverse child workspaces
    children, err := t.listChildWorkspaces(ctx, path)
    if err != nil {
        return err
    }
    
    for _, child := range children {
        childPath := path.Join(child)
        if err := t.traverseWorkspace(ctx, childPath, selector, workspaces); err != nil {
            // Log error but continue traversal
            continue
        }
    }
    
    return nil
}

// getWorkspaceInfo retrieves workspace information
func (t *WorkspaceTraverser) getWorkspaceInfo(ctx context.Context, path logicalcluster.Path) (interfaces.Workspace, error) {
    // Implementation would fetch workspace details from KCP API
    return interfaces.Workspace{
        Name:   path.String(),
        Path:   path.String(),
        Labels: map[string]string{},
        Status: interfaces.WorkspaceStatus{
            Phase: "Ready",
        },
    }, nil
}

// listChildWorkspaces lists child workspaces of a parent
func (t *WorkspaceTraverser) listChildWorkspaces(ctx context.Context, parent logicalcluster.Path) ([]string, error) {
    // Implementation would list child workspaces from KCP API
    return []string{}, nil
}

// GetClusters returns clusters in a workspace
func (t *WorkspaceTraverser) GetClusters(ctx context.Context, workspace string) ([]interfaces.Cluster, error) {
    // Check cache
    if cached, ok := t.cache.GetClusters(workspace); ok {
        return cached, nil
    }
    
    // Fetch clusters from workspace
    clusters, err := t.fetchClustersInWorkspace(ctx, workspace)
    if err != nil {
        return nil, err
    }
    
    // Cache results
    t.cache.PutClusters(workspace, clusters)
    
    return clusters, nil
}
```

### 2. `pkg/placement/discovery/cluster_finder.go` (100 lines)
Cluster discovery within workspaces.

```go
package discovery

import (
    "context"
    "fmt"
    "github.com/kcp-dev/kcp/pkg/placement/interfaces"
    workloadv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"
    kcpclient "github.com/kcp-dev/kcp/pkg/client/clientset/versioned"
    "github.com/kcp-dev/logicalcluster/v3"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClusterFinder discovers clusters across workspaces
type ClusterFinder struct {
    client    kcpclient.Interface
    traverser *WorkspaceTraverser
}

// NewClusterFinder creates a new cluster finder
func NewClusterFinder(client kcpclient.Interface) *ClusterFinder {
    return &ClusterFinder{
        client:    client,
        traverser: NewWorkspaceTraverser(client),
    }
}

// FindClusters finds all clusters matching criteria
func (f *ClusterFinder) FindClusters(ctx context.Context, criteria interfaces.ClusterCriteria) ([]interfaces.ClusterTarget, error) {
    targets := []interfaces.ClusterTarget{}
    
    // Get workspaces to search
    workspaces, err := f.traverser.ListWorkspaces(ctx, criteria.WorkspaceSelector)
    if err != nil {
        return nil, fmt.Errorf("failed to list workspaces: %w", err)
    }
    
    // Search for clusters in each workspace
    for _, ws := range workspaces {
        clusters, err := f.findClustersInWorkspace(ctx, ws, criteria)
        if err != nil {
            // Log error but continue
            continue
        }
        targets = append(targets, clusters...)
    }
    
    return targets, nil
}

// findClustersInWorkspace finds clusters in a specific workspace
func (f *ClusterFinder) findClustersInWorkspace(ctx context.Context, 
    workspace interfaces.Workspace, criteria interfaces.ClusterCriteria) ([]interfaces.ClusterTarget, error) {
    
    cluster := logicalcluster.NewPath(workspace.Path)
    
    // List SyncTargets in the workspace
    syncTargets, err := f.client.Cluster(cluster).
        WorkloadV1alpha1().
        SyncTargets().
        List(ctx, metav1.ListOptions{
            LabelSelector: criteria.LabelSelector.String(),
        })
    if err != nil {
        return nil, err
    }
    
    targets := []interfaces.ClusterTarget{}
    for _, st := range syncTargets.Items {
        if f.matchesCriteria(&st, criteria) {
            target := f.syncTargetToClusterTarget(&st, workspace)
            targets = append(targets, target)
        }
    }
    
    return targets, nil
}

// matchesCriteria checks if a SyncTarget matches the criteria
func (f *ClusterFinder) matchesCriteria(st *workloadv1alpha1.SyncTarget, 
    criteria interfaces.ClusterCriteria) bool {
    
    // Check labels
    if !criteria.LabelSelector.Matches(labels.Set(st.Labels)) {
        return false
    }
    
    // Check regions if specified
    if len(criteria.Regions) > 0 {
        region := st.Labels["region"]
        found := false
        for _, r := range criteria.Regions {
            if r == region {
                found = true
                break
            }
        }
        if !found {
            return false
        }
    }
    
    // Check capabilities
    for _, required := range criteria.RequiredCapabilities {
        if !f.hasCapability(st, required) {
            return false
        }
    }
    
    return true
}

// syncTargetToClusterTarget converts SyncTarget to ClusterTarget
func (f *ClusterFinder) syncTargetToClusterTarget(st *workloadv1alpha1.SyncTarget, 
    workspace interfaces.Workspace) interfaces.ClusterTarget {
    
    return interfaces.ClusterTarget{
        Name:      st.Name,
        Workspace: workspace.Path,
        Labels:    st.Labels,
        Capacity:  f.extractCapacity(st),
        Available: f.extractAvailable(st),
        Region:    st.Labels["region"],
        Zone:      st.Labels["zone"],
    }
}
```

### 3. `pkg/placement/discovery/permission_checker.go` (80 lines)
Permission checking for workspace access.

```go
package discovery

import (
    "context"
    "fmt"
    "sync"
    "time"
    
    authv1 "k8s.io/api/authorization/v1"
    kcpclient "github.com/kcp-dev/kcp/pkg/client/clientset/versioned"
    "github.com/kcp-dev/logicalcluster/v3"
)

// PermissionChecker checks access permissions for workspaces
type PermissionChecker struct {
    client kcpclient.Interface
    cache  *permissionCache
}

// permissionCache caches permission check results
type permissionCache struct {
    mu      sync.RWMutex
    entries map[string]*permissionEntry
}

type permissionEntry struct {
    allowed   bool
    timestamp time.Time
}

// NewPermissionChecker creates a new permission checker
func NewPermissionChecker(client kcpclient.Interface) *PermissionChecker {
    return &PermissionChecker{
        client: client,
        cache: &permissionCache{
            entries: make(map[string]*permissionEntry),
        },
    }
}

// CheckAccess checks if the current user can access a workspace
func (c *PermissionChecker) CheckAccess(ctx context.Context, workspace string, verb string) (bool, error) {
    cacheKey := fmt.Sprintf("%s:%s", workspace, verb)
    
    // Check cache
    if allowed, ok := c.cache.get(cacheKey); ok {
        return allowed, nil
    }
    
    // Perform SubjectAccessReview
    allowed, err := c.performAccessCheck(ctx, workspace, verb)
    if err != nil {
        return false, err
    }
    
    // Cache the result
    c.cache.put(cacheKey, allowed)
    
    return allowed, nil
}

// performAccessCheck performs the actual permission check
func (c *PermissionChecker) performAccessCheck(ctx context.Context, workspace string, verb string) (bool, error) {
    cluster := logicalcluster.NewPath(workspace)
    
    sar := &authv1.SubjectAccessReview{
        Spec: authv1.SubjectAccessReviewSpec{
            ResourceAttributes: &authv1.ResourceAttributes{
                Verb:     verb,
                Group:    "workload.kcp.io",
                Resource: "synctargets",
            },
        },
    }
    
    result, err := c.client.Cluster(cluster).
        AuthorizationV1().
        SubjectAccessReviews().
        Create(ctx, sar, metav1.CreateOptions{})
    if err != nil {
        return false, fmt.Errorf("failed to check access: %w", err)
    }
    
    return result.Status.Allowed, nil
}

// cache methods
func (c *permissionCache) get(key string) (bool, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    
    entry, ok := c.entries[key]
    if !ok {
        return false, false
    }
    
    // Check if cache entry is still valid (5 minutes TTL)
    if time.Since(entry.timestamp) > 5*time.Minute {
        return false, false
    }
    
    return entry.allowed, true
}

func (c *permissionCache) put(key string, allowed bool) {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    c.entries[key] = &permissionEntry{
        allowed:   allowed,
        timestamp: time.Now(),
    }
}
```

### 4. `pkg/placement/discovery/cache.go` (70 lines)
Caching layer for discovery operations.

```go
package discovery

import (
    "sync"
    "time"
    "github.com/kcp-dev/kcp/pkg/placement/interfaces"
)

// DiscoveryCache caches discovery results
type DiscoveryCache struct {
    mu         sync.RWMutex
    workspaces map[string]*cacheEntry[[]interfaces.Workspace]
    clusters   map[string]*cacheEntry[[]interfaces.Cluster]
    ttl        time.Duration
}

// cacheEntry wraps cached data with timestamp
type cacheEntry[T any] struct {
    data      T
    timestamp time.Time
}

// NewDiscoveryCache creates a new discovery cache
func NewDiscoveryCache(ttl time.Duration) *DiscoveryCache {
    return &DiscoveryCache{
        workspaces: make(map[string]*cacheEntry[[]interfaces.Workspace]),
        clusters:   make(map[string]*cacheEntry[[]interfaces.Cluster]),
        ttl:        ttl,
    }
}

// GetWorkspaces retrieves cached workspaces
func (c *DiscoveryCache) GetWorkspaces(key string) ([]interfaces.Workspace, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    
    entry, ok := c.workspaces[key]
    if !ok {
        return nil, false
    }
    
    if time.Since(entry.timestamp) > c.ttl {
        return nil, false
    }
    
    return entry.data, true
}

// PutWorkspaces caches workspace list
func (c *DiscoveryCache) PutWorkspaces(key string, workspaces []interfaces.Workspace) {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    c.workspaces[key] = &cacheEntry[[]interfaces.Workspace]{
        data:      workspaces,
        timestamp: time.Now(),
    }
}

// GetClusters retrieves cached clusters
func (c *DiscoveryCache) GetClusters(workspace string) ([]interfaces.Cluster, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    
    entry, ok := c.clusters[workspace]
    if !ok {
        return nil, false
    }
    
    if time.Since(entry.timestamp) > c.ttl {
        return nil, false
    }
    
    return entry.data, true
}

// PutClusters caches cluster list
func (c *DiscoveryCache) PutClusters(workspace string, clusters []interfaces.Cluster) {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    c.clusters[workspace] = &cacheEntry[[]interfaces.Cluster]{
        data:      clusters,
        timestamp: time.Now(),
    }
}

// Clear removes all cached entries
func (c *DiscoveryCache) Clear() {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    c.workspaces = make(map[string]*cacheEntry[[]interfaces.Workspace])
    c.clusters = make(map[string]*cacheEntry[[]interfaces.Cluster])
}
```

### 5. `pkg/placement/discovery/hierarchy.go` (90 lines)
Workspace hierarchy management.

```go
package discovery

import (
    "context"
    "fmt"
    "strings"
    "github.com/kcp-dev/kcp/pkg/placement/interfaces"
    "github.com/kcp-dev/logicalcluster/v3"
)

// HierarchyManager manages workspace hierarchy relationships
type HierarchyManager struct {
    traverser *WorkspaceTraverser
    cache     *HierarchyCache
}

// HierarchyCache caches hierarchy information
type HierarchyCache struct {
    parents  map[string]string
    children map[string][]string
    depths   map[string]int
}

// NewHierarchyManager creates a new hierarchy manager
func NewHierarchyManager(traverser *WorkspaceTraverser) *HierarchyManager {
    return &HierarchyManager{
        traverser: traverser,
        cache:     NewHierarchyCache(),
    }
}

// NewHierarchyCache creates a new hierarchy cache
func NewHierarchyCache() *HierarchyCache {
    return &HierarchyCache{
        parents:  make(map[string]string),
        children: make(map[string][]string),
        depths:   make(map[string]int),
    }
}

// GetParent returns the parent workspace
func (m *HierarchyManager) GetParent(workspace string) (string, error) {
    if parent, ok := m.cache.parents[workspace]; ok {
        return parent, nil
    }
    
    path := logicalcluster.NewPath(workspace)
    if path.IsRoot() {
        return "", nil
    }
    
    parent := path.Parent().String()
    m.cache.parents[workspace] = parent
    
    return parent, nil
}

// GetChildren returns child workspaces
func (m *HierarchyManager) GetChildren(ctx context.Context, workspace string) ([]string, error) {
    if children, ok := m.cache.children[workspace]; ok {
        return children, nil
    }
    
    children, err := m.traverser.listChildWorkspaces(ctx, logicalcluster.NewPath(workspace))
    if err != nil {
        return nil, err
    }
    
    m.cache.children[workspace] = children
    return children, nil
}

// GetDepth returns the depth of a workspace in the hierarchy
func (m *HierarchyManager) GetDepth(workspace string) int {
    if depth, ok := m.cache.depths[workspace]; ok {
        return depth
    }
    
    path := logicalcluster.NewPath(workspace)
    depth := len(strings.Split(path.String(), ":")) - 1
    
    m.cache.depths[workspace] = depth
    return depth
}

// IsAncestor checks if workspace A is an ancestor of workspace B
func (m *HierarchyManager) IsAncestor(ancestorPath, descendantPath string) bool {
    return strings.HasPrefix(descendantPath, ancestorPath+":")
}

// GetCommonAncestor finds the common ancestor of two workspaces
func (m *HierarchyManager) GetCommonAncestor(ws1, ws2 string) string {
    parts1 := strings.Split(ws1, ":")
    parts2 := strings.Split(ws2, ":")
    
    common := []string{}
    for i := 0; i < len(parts1) && i < len(parts2); i++ {
        if parts1[i] == parts2[i] {
            common = append(common, parts1[i])
        } else {
            break
        }
    }
    
    if len(common) == 0 {
        return "root"
    }
    
    return strings.Join(common, ":")
}
```

### 6. `pkg/placement/discovery/discovery_test.go` (140 lines)
Comprehensive tests for discovery components.

```go
package discovery_test

import (
    "context"
    "testing"
    "time"
    
    "github.com/kcp-dev/kcp/pkg/placement/discovery"
    "github.com/kcp-dev/kcp/pkg/placement/interfaces"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "k8s.io/apimachinery/pkg/labels"
)

func TestWorkspaceTraversal(t *testing.T) {
    ctx := context.Background()
    client := newMockKCPClient()
    traverser := discovery.NewWorkspaceTraverser(client)
    
    tests := []struct {
        name     string
        selector labels.Selector
        expected []string
    }{
        {
            name:     "list all workspaces",
            selector: labels.Everything(),
            expected: []string{"root", "root:org", "root:org:team1", "root:org:team2"},
        },
        {
            name:     "filter by label",
            selector: labels.SelectorFromSet(labels.Set{"env": "prod"}),
            expected: []string{"root:org:team1"},
        },
        {
            name:     "empty result",
            selector: labels.SelectorFromSet(labels.Set{"env": "staging"}),
            expected: []string{},
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            workspaces, err := traverser.ListWorkspaces(ctx, tt.selector)
            require.NoError(t, err)
            
            names := []string{}
            for _, ws := range workspaces {
                names = append(names, ws.Name)
            }
            
            assert.ElementsMatch(t, tt.expected, names)
        })
    }
}

func TestClusterDiscovery(t *testing.T) {
    ctx := context.Background()
    client := newMockKCPClient()
    finder := discovery.NewClusterFinder(client)
    
    criteria := interfaces.ClusterCriteria{
        WorkspaceSelector: labels.Everything(),
        LabelSelector:     labels.SelectorFromSet(labels.Set{"type": "compute"}),
        Regions:           []string{"us-west-2", "us-east-1"},
    }
    
    clusters, err := finder.FindClusters(ctx, criteria)
    require.NoError(t, err)
    
    assert.Len(t, clusters, 2)
    for _, cluster := range clusters {
        assert.Contains(t, criteria.Regions, cluster.Region)
        assert.Equal(t, "compute", cluster.Labels["type"])
    }
}

func TestPermissionChecking(t *testing.T) {
    ctx := context.Background()
    client := newMockKCPClient()
    checker := discovery.NewPermissionChecker(client)
    
    tests := []struct {
        name      string
        workspace string
        verb      string
        expected  bool
    }{
        {
            name:      "allowed access",
            workspace: "root:org:team1",
            verb:      "list",
            expected:  true,
        },
        {
            name:      "denied access",
            workspace: "root:org:team2",
            verb:      "delete",
            expected:  false,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            allowed, err := checker.CheckAccess(ctx, tt.workspace, tt.verb)
            require.NoError(t, err)
            assert.Equal(t, tt.expected, allowed)
        })
    }
}

func TestCaching(t *testing.T) {
    cache := discovery.NewDiscoveryCache(100 * time.Millisecond)
    
    // Test workspace caching
    workspaces := []interfaces.Workspace{
        {Name: "ws1", Path: "root:ws1"},
        {Name: "ws2", Path: "root:ws2"},
    }
    
    cache.PutWorkspaces("test-key", workspaces)
    
    // Should retrieve from cache
    cached, ok := cache.GetWorkspaces("test-key")
    assert.True(t, ok)
    assert.Equal(t, workspaces, cached)
    
    // Wait for TTL to expire
    time.Sleep(150 * time.Millisecond)
    
    // Should not retrieve expired entry
    _, ok = cache.GetWorkspaces("test-key")
    assert.False(t, ok)
}

func TestHierarchy(t *testing.T) {
    manager := discovery.NewHierarchyManager(nil)
    
    // Test ancestor checking
    assert.True(t, manager.IsAncestor("root", "root:org:team"))
    assert.True(t, manager.IsAncestor("root:org", "root:org:team"))
    assert.False(t, manager.IsAncestor("root:org:team", "root:org"))
    
    // Test common ancestor
    ancestor := manager.GetCommonAncestor("root:org:team1", "root:org:team2")
    assert.Equal(t, "root:org", ancestor)
    
    // Test depth calculation
    assert.Equal(t, 0, manager.GetDepth("root"))
    assert.Equal(t, 1, manager.GetDepth("root:org"))
    assert.Equal(t, 2, manager.GetDepth("root:org:team"))
}
```

## Implementation Steps

### Step 1: Setup Dependencies
```bash
# Ensure branch 13 is available
git fetch origin feature/tmc-phase4-13-placement-interfaces
```

### Step 2: Create Package Structure
```bash
mkdir -p pkg/placement/discovery
```

### Step 3: Implement Discovery Components
1. Start with `traverser.go` - workspace traversal
2. Add `cluster_finder.go` - cluster discovery
3. Create `permission_checker.go` - access control
4. Add `cache.go` - caching layer
5. Create `hierarchy.go` - hierarchy management
6. Add `discovery_test.go` - comprehensive tests

### Step 4: Add Benchmarks
Create performance benchmarks for traversal operations.

### Step 5: Integration Testing
Test with real KCP API if available.

## KCP Patterns to Follow

1. **Logical Clusters**: Use logicalcluster.Path for workspace paths
2. **Client-go Patterns**: Follow KCP client patterns
3. **RBAC Integration**: Use SubjectAccessReview for permissions
4. **Caching Strategy**: Cache with TTL for performance
5. **Error Handling**: Graceful degradation for inaccessible workspaces

## Testing Requirements

### Unit Tests Required
- [ ] Workspace traversal tests
- [ ] Cluster discovery tests
- [ ] Permission checking tests
- [ ] Cache functionality tests
- [ ] Hierarchy management tests

### Performance Tests
- [ ] Large hierarchy traversal
- [ ] Cache performance
- [ ] Concurrent access tests

## Integration Points

This discovery implementation will be:
- **Used by**: Branch 18 (Scheduler)
- **Used by**: Branch 19 (Controller)
- **Tested in**: Branch 23 (Integration)

## Validation Checklist

- [ ] Efficient workspace traversal
- [ ] Permission checks implemented
- [ ] Caching working correctly
- [ ] Thread-safe implementation
- [ ] Error handling robust
- [ ] Performance optimized
- [ ] KCP patterns followed
- [ ] Documentation complete
- [ ] Test coverage >80%
- [ ] Feature flag ready

## Line Count Validation
Run before committing:
```bash
/workspaces/kcp-shared-tools/tmc-pr-line-counter.sh -c feature/tmc-phase4-16-workspace-discovery
```

Target: ~600 lines