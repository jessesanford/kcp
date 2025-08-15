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
	parents   map[string]string
	children  map[string][]string
	depths    map[string]int
	workspaces map[string]*cacheEntry[[]interfaces.WorkspaceInfo]
	clusters  map[string]*cacheEntry[[]interfaces.ClusterTarget]
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
		parents:    make(map[string]string),
		children:   make(map[string][]string),
		depths:     make(map[string]int),
		workspaces: make(map[string]*cacheEntry[[]interfaces.WorkspaceInfo]),
		clusters:   make(map[string]*cacheEntry[[]interfaces.ClusterTarget]),
	}
}

// GetWorkspaces retrieves cached workspaces (implements cache interface)
func (c *HierarchyCache) GetWorkspaces(key string) ([]interfaces.WorkspaceInfo, bool) {
	entry, ok := c.workspaces[key]
	if !ok {
		return nil, false
	}
	
	return entry.data, true
}

// PutWorkspaces caches workspace list (implements cache interface)
func (c *HierarchyCache) PutWorkspaces(key string, workspaces []interfaces.WorkspaceInfo) {
	c.workspaces[key] = &cacheEntry[[]interfaces.WorkspaceInfo]{
		data: workspaces,
	}
}

// GetClusters retrieves cached clusters (implements cache interface)
func (c *HierarchyCache) GetClusters(workspace string) ([]interfaces.ClusterTarget, bool) {
	entry, ok := c.clusters[workspace]
	if !ok {
		return nil, false
	}
	
	return entry.data, true
}

// PutClusters caches cluster list (implements cache interface)
func (c *HierarchyCache) PutClusters(workspace string, clusters []interfaces.ClusterTarget) {
	c.clusters[workspace] = &cacheEntry[[]interfaces.ClusterTarget]{
		data: clusters,
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

// BuildWorkspaceTree builds a complete workspace hierarchy tree
func (m *HierarchyManager) BuildWorkspaceTree(ctx context.Context, root string) (*interfaces.WorkspaceTree, error) {
	// Get workspace info for root
	rootInfo, err := m.traverser.getWorkspaceInfo(ctx, logicalcluster.NewPath(root))
	if err != nil {
		return nil, fmt.Errorf("failed to get root workspace info: %w", err)
	}
	
	tree := &interfaces.WorkspaceTree{
		Root:     rootInfo,
		Children: make(map[string]*interfaces.WorkspaceTree),
		Depth:    m.GetDepth(root),
	}
	
	// Get children and build subtrees
	children, err := m.GetChildren(ctx, root)
	if err != nil {
		return nil, fmt.Errorf("failed to get children for %s: %w", root, err)
	}
	
	for _, child := range children {
		childPath := root + ":" + child
		childTree, err := m.BuildWorkspaceTree(ctx, childPath)
		if err != nil {
			// Skip problematic children but continue
			continue
		}
		tree.Children[child] = childTree
	}
	
	return tree, nil
}

// GetWorkspaceAncestors returns all ancestors of a workspace
func (m *HierarchyManager) GetWorkspaceAncestors(workspace string) []string {
	ancestors := []string{}
	current := workspace
	
	for {
		parent, err := m.GetParent(current)
		if err != nil || parent == "" {
			break
		}
		ancestors = append(ancestors, parent)
		current = parent
	}
	
	return ancestors
}