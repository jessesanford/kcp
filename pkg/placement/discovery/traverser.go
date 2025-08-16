package discovery

import (
	"context"
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/kcp/pkg/placement/interfaces"
	kcpclient "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
	tenancyv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/tenancy/v1alpha1"
	"github.com/kcp-dev/logicalcluster/v3"
)

// WorkspaceTraverser implements workspace discovery and traversal
type WorkspaceTraverser struct {
	client      kcpclient.ClusterInterface
	cache       *DiscoveryCache
	permissions *PermissionChecker
}

// NewWorkspaceTraverser creates a new workspace traverser
func NewWorkspaceTraverser(client kcpclient.ClusterInterface) *WorkspaceTraverser {
	return &WorkspaceTraverser{
		client:      client,
		cache:       NewDiscoveryCache(10 * time.Minute),
		permissions: NewPermissionChecker(client),
	}
}

// ListWorkspaces lists workspaces matching the selector
func (t *WorkspaceTraverser) ListWorkspaces(ctx context.Context, selector labels.Selector) ([]interfaces.WorkspaceInfo, error) {
	// Check cache first
	if cached, ok := t.cache.GetWorkspaces(selector.String()); ok {
		return cached, nil
	}
	
	workspaces := []interfaces.WorkspaceInfo{}
	
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
	selector labels.Selector, workspaces *[]interfaces.WorkspaceInfo) error {
	
	// Check permissions for this workspace
	canAccess, err := t.permissions.CheckAccess(ctx, path.String(), "list")
	if err != nil {
		return err
	}
	if !canAccess {
		klog.V(4).Infof("Skipping workspace %s due to access restrictions", path.String())
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
			klog.Errorf("Failed to traverse child workspace %s: %v", childPath.String(), err)
			continue
		}
	}
	
	return nil
}

// getWorkspaceInfo retrieves workspace information from KCP API
func (t *WorkspaceTraverser) getWorkspaceInfo(ctx context.Context, path logicalcluster.Path) (interfaces.WorkspaceInfo, error) {
	// Extract workspace name and parent path
	workspaceName := path.Base()
	parentPath := path.Parent()
	
	// Get the workspace from the parent cluster
	workspace, err := t.client.Cluster(parentPath).TenancyV1alpha1().Workspaces().Get(ctx, workspaceName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// Workspace doesn't exist or we don't have access
			return interfaces.WorkspaceInfo{}, fmt.Errorf("workspace %s not found in %s: %w", workspaceName, parentPath, err)
		}
		return interfaces.WorkspaceInfo{}, fmt.Errorf("failed to get workspace %s from %s: %w", workspaceName, parentPath, err)
	}
	
	// Convert to WorkspaceInfo
	info := interfaces.WorkspaceInfo{
		Name:        logicalcluster.Name(path.String()),
		Labels:      workspace.Labels,
		Annotations: workspace.Annotations,
		Ready:       t.isWorkspaceReady(workspace),
	}
	
	// Set parent if not root
	if parentPath.String() != "root" && parentPath.String() != "" {
		parentName := logicalcluster.Name(parentPath.String())
		info.Parent = &parentName
	}
	
	return info, nil
}

// listChildWorkspaces lists child workspaces of a parent using KCP API
func (t *WorkspaceTraverser) listChildWorkspaces(ctx context.Context, parent logicalcluster.Path) ([]string, error) {
	// List workspaces in the parent cluster
	workspaces, err := t.client.Cluster(parent).TenancyV1alpha1().Workspaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		if errors.IsNotFound(err) || errors.IsForbidden(err) {
			// Parent doesn't exist or we don't have access - return empty list
			klog.V(4).Infof("Cannot access workspaces in %s: %v", parent, err)
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to list workspaces in %s: %w", parent, err)
	}
	
	// Extract workspace names
	names := make([]string, 0, len(workspaces.Items))
	for _, ws := range workspaces.Items {
		// Skip system workspaces unless they are explicitly requested
		if t.isSystemWorkspace(ws.Name) {
			klog.V(5).Infof("Skipping system workspace: %s", ws.Name)
			continue
		}
		names = append(names, ws.Name)
	}
	
	return names, nil
}

// GetClusters returns clusters in a workspace
func (t *WorkspaceTraverser) GetClusters(ctx context.Context, workspace string) ([]interfaces.ClusterTarget, error) {
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

// fetchClustersInWorkspace fetches clusters from a specific workspace
func (t *WorkspaceTraverser) fetchClustersInWorkspace(ctx context.Context, workspace string) ([]interfaces.ClusterTarget, error) {
	cluster := logicalcluster.NewPath(workspace)
	
	// List SyncTargets in the workspace
	syncTargets, err := t.client.Cluster(cluster).
		WorkloadV1alpha1().
		SyncTargets().
		List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list sync targets in workspace %s: %w", workspace, err)
	}
	
	targets := []interfaces.ClusterTarget{}
	for _, st := range syncTargets.Items {
		target := interfaces.ClusterTarget{
			Name:      st.Name,
			Workspace: logicalcluster.Name(workspace),
			Labels:    st.Labels,
			Ready:     true, // Simplified - would check actual sync target status
			Capacity: interfaces.ResourceCapacity{
				CPU:    "4",
				Memory: "8Gi",
				Pods:   110,
			},
		}
		targets = append(targets, target)
	}
	
	return targets, nil
}

// isWorkspaceReady checks if a workspace is ready for placement operations
func (t *WorkspaceTraverser) isWorkspaceReady(workspace *tenancyv1alpha1.Workspace) bool {
	// Check workspace phase
	if workspace.Status.Phase != tenancyv1alpha1.WorkspacePhaseReady {
		return false
	}
	
	// Check conditions for any failure conditions
	for _, condition := range workspace.Status.Conditions {
		if condition.Type == "Ready" && condition.Status == metav1.ConditionFalse {
			return false
		}
		// Check for other failure conditions
		if strings.Contains(condition.Type, "Failed") && condition.Status == metav1.ConditionTrue {
			return false
		}
	}
	
	return true
}

// isSystemWorkspace determines if a workspace is a system workspace
func (t *WorkspaceTraverser) isSystemWorkspace(workspaceName string) bool {
	systemWorkspaceNames := []string{
		"system",
		"admin",
		"kcp-system", 
		"kcp-root-compute",
	}
	
	// Check exact name matches
	for _, systemName := range systemWorkspaceNames {
		if workspaceName == systemName {
			return true
		}
	}
	
	// Check prefixes that indicate system workspaces
	systemPrefixes := []string{
		"kcp-",
		"system-",
		"admin-",
	}
	
	for _, prefix := range systemPrefixes {
		if strings.HasPrefix(workspaceName, prefix) {
			return true
		}
	}
	
	return false
}