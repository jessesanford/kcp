package interfaces

import (
	"context"

	"k8s.io/apimachinery/pkg/labels"

	"github.com/kcp-dev/logicalcluster/v3"
)

// WorkspaceDiscovery provides workspace traversal and cluster discovery
// capabilities for cross-workspace placement operations.
// It abstracts the complexities of navigating KCP workspace hierarchies
// and discovering available clusters across multiple workspaces.
type WorkspaceDiscovery interface {
	// ListWorkspaces returns workspaces matching the selector.
	// It traverses the workspace hierarchy starting from the current workspace
	// and returns all accessible workspaces that match the given selector.
	ListWorkspaces(ctx context.Context, selector labels.Selector) ([]WorkspaceInfo, error)

	// GetClusters returns available clusters in the specified workspace.
	// It discovers all registered clusters that are ready for workload placement
	// within the given logical cluster workspace.
	GetClusters(ctx context.Context, workspace logicalcluster.Name) ([]ClusterTarget, error)

	// CheckAccess verifies permission to place workloads in a workspace.
	// It performs RBAC checks to ensure the current user has the necessary
	// permissions to perform the specified verb on the given resource.
	CheckAccess(ctx context.Context, workspace logicalcluster.Name,
		verb string, resource string) (bool, error)

	// GetWorkspaceHierarchy returns the complete hierarchy tree starting from root.
	// This is useful for understanding workspace relationships and implementing
	// hierarchical placement policies.
	GetWorkspaceHierarchy(ctx context.Context,
		root logicalcluster.Name) (*WorkspaceTree, error)

	// FindWorkspacesByLabels discovers workspaces with specific labels.
	// This enables label-based workspace selection for placement policies.
	FindWorkspacesByLabels(ctx context.Context, labelSelector labels.Selector,
		options *DiscoveryOptions) ([]WorkspaceInfo, error)
}

// WorkspaceTree represents workspace hierarchy as a tree structure.
// Each node contains workspace information and references to child workspaces,
// enabling navigation and policy evaluation across the hierarchy.
type WorkspaceTree struct {
	// Root workspace information
	Root WorkspaceInfo

	// Children maps child workspace names to their tree nodes
	Children map[string]*WorkspaceTree

	// Depth indicates the depth of this node in the tree (0 for root)
	Depth int
}

// DiscoveryOptions configures workspace discovery behavior.
// These options control the scope and performance of workspace traversal operations.
type DiscoveryOptions struct {
	// MaxDepth limits how deep to traverse in the workspace hierarchy.
	// A value of 0 means no limit, 1 means only direct children, etc.
	MaxDepth int

	// IncludeSystemWorkspaces determines whether to include system workspaces
	// in discovery results. System workspaces are typically excluded from
	// user workload placement.
	IncludeSystemWorkspaces bool

	// FollowReferences determines whether to follow workspace references
	// and include referenced workspaces in the discovery results.
	FollowReferences bool

	// ConcurrentDiscovery enables parallel workspace discovery for better performance.
	// When enabled, multiple workspaces are discovered concurrently.
	ConcurrentDiscovery bool

	// TimeoutPerWorkspace sets the maximum time to spend discovering each workspace.
	// This prevents slow workspaces from blocking the entire discovery process.
	TimeoutPerWorkspace int

	// CacheResults enables caching of discovery results to improve performance
	// for repeated workspace queries.
	CacheResults bool
}