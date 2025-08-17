/*
Copyright 2023 The KCP Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package discovery

import (
	"context"
	"fmt"
	"strings"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	tenancyv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/tenancy/v1alpha1"
	tenancyclient "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/typed/tenancy/v1alpha1"
	workloadv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/workload/v1alpha1"
	workloadclient "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/typed/workload/v1alpha1"
	"github.com/kcp-dev/logicalcluster/v3"
)

// WorkspaceDiscovererImpl is the main implementation of WorkspaceDiscoverer.
type WorkspaceDiscovererImpl struct {
	tenancyClient        tenancyclient.ClusterInterface
	workloadClient       workloadclient.ClusterInterface
	workspaceCache       cache.Store
	syncTargetCache      cache.Store
	authChecker          AuthorizationChecker
	workspaceIndex       WorkspaceIndex
	syncTargetIndex      SyncTargetIndex
	
	// indexLock protects the workspace indexing operations
	indexLock sync.RWMutex
	
	// workspaceLabelIndex maps label key-value pairs to workspace names
	workspaceLabelIndex map[string][]*tenancyv1alpha1.Workspace
	
	// workspaceHierarchyIndex maps workspace to its parent and children
	workspaceHierarchyIndex map[logicalcluster.Name]*hierarchyNode
}

type hierarchyNode struct {
	workspace *tenancyv1alpha1.Workspace
	parent    logicalcluster.Name
	children  []logicalcluster.Name
}

// NewWorkspaceDiscoverer creates a new workspace discoverer with the required dependencies.
func NewWorkspaceDiscoverer(
	tenancyClient tenancyclient.ClusterInterface,
	workloadClient workloadclient.ClusterInterface,
	workspaceInformer cache.SharedIndexInformer,
	syncTargetInformer cache.SharedIndexInformer,
	authChecker AuthorizationChecker,
) (*WorkspaceDiscovererImpl, error) {
	if tenancyClient == nil {
		return nil, fmt.Errorf("tenancyClient cannot be nil")
	}
	if workloadClient == nil {
		return nil, fmt.Errorf("workloadClient cannot be nil")
	}
	if authChecker == nil {
		return nil, fmt.Errorf("authChecker cannot be nil")
	}

	d := &WorkspaceDiscovererImpl{
		tenancyClient:           tenancyClient,
		workloadClient:          workloadClient,
		workspaceCache:          workspaceInformer.GetStore(),
		syncTargetCache:         syncTargetInformer.GetStore(),
		authChecker:             authChecker,
		workspaceLabelIndex:     make(map[string][]*tenancyv1alpha1.Workspace),
		workspaceHierarchyIndex: make(map[logicalcluster.Name]*hierarchyNode),
	}

	// Set up workspace indexing
	if _, err := workspaceInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    d.onWorkspaceAdd,
		UpdateFunc: d.onWorkspaceUpdate,
		DeleteFunc: d.onWorkspaceDelete,
	}); err != nil {
		return nil, fmt.Errorf("failed to add workspace event handler: %w", err)
	}

	// Set up sync target indexing  
	if _, err := syncTargetInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    d.onSyncTargetAdd,
		UpdateFunc: d.onSyncTargetUpdate,
		DeleteFunc: d.onSyncTargetDelete,
	}); err != nil {
		return nil, fmt.Errorf("failed to add sync target event handler: %w", err)
	}

	return d, nil
}

// DiscoverWorkspaces discovers workspaces based on the provided options.
func (d *WorkspaceDiscovererImpl) DiscoverWorkspaces(ctx context.Context, opts DiscoveryOptions) ([]*WorkspaceDiscoveryResult, error) {
	klog.V(4).InfoS("Starting workspace discovery", "labelSelector", opts.LabelSelector, "maxDepth", opts.MaxDepth)

	var workspaces []*tenancyv1alpha1.Workspace
	var err error

	// Get workspaces based on label selector
	if opts.LabelSelector != nil && !opts.LabelSelector.Empty() {
		workspaces, err = d.getWorkspacesByLabel(opts.LabelSelector)
	} else {
		workspaces = d.getAllWorkspaces()
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get workspaces: %w", err)
	}

	// Filter workspaces based on readiness requirements
	if !opts.IncludeNotReady {
		workspaces = d.filterReadyWorkspaces(workspaces)
	}

	results := make([]*WorkspaceDiscoveryResult, 0, len(workspaces))
	
	for _, workspace := range workspaces {
		result := &WorkspaceDiscoveryResult{
			Workspace: workspace,
		}

		// Check authorization if user context is provided
		if opts.User != nil {
			wsName := logicalcluster.From(workspace)
			authorized, authErr := d.authChecker.CanAccessWorkspace(ctx, opts.User, wsName)
			result.Authorized = authorized
			if authErr != nil {
				result.Error = authErr
				klog.V(2).InfoS("Authorization check failed", "workspace", wsName, "error", authErr)
			}
		} else {
			result.Authorized = true // No auth check requested
		}

		// Discover sync targets for this workspace
		wsName := logicalcluster.From(workspace)
		syncTargets, syncErr := d.DiscoverSyncTargets(ctx, wsName, opts)
		if syncErr != nil {
			result.Error = syncErr
			klog.V(2).InfoS("Sync target discovery failed", "workspace", wsName, "error", syncErr)
		} else {
			result.SyncTargets = syncTargets
		}

		results = append(results, result)
	}

	klog.V(4).InfoS("Completed workspace discovery", "resultCount", len(results))
	return results, nil
}

// DiscoverSyncTargets discovers sync targets within a specific workspace.
func (d *WorkspaceDiscovererImpl) DiscoverSyncTargets(ctx context.Context, workspace logicalcluster.Name, opts DiscoveryOptions) ([]*workloadv1alpha1.SyncTarget, error) {
	klog.V(4).InfoS("Discovering sync targets", "workspace", workspace, "syncTargetSelector", opts.SyncTargetSelector)

	syncTargets := d.getSyncTargetsByWorkspace(workspace)

	// Apply sync target label selector if provided
	if opts.SyncTargetSelector != nil && !opts.SyncTargetSelector.Empty() {
		filtered := make([]*workloadv1alpha1.SyncTarget, 0, len(syncTargets))
		for _, st := range syncTargets {
			if opts.SyncTargetSelector.Matches(labels.Set(st.Labels)) {
				filtered = append(filtered, st)
			}
		}
		syncTargets = filtered
	}

	klog.V(4).InfoS("Found sync targets", "workspace", workspace, "count", len(syncTargets))
	return syncTargets, nil
}

// GetWorkspaceHierarchy returns the full hierarchy for a given workspace.
func (d *WorkspaceDiscovererImpl) GetWorkspaceHierarchy(ctx context.Context, workspace logicalcluster.Name) ([]*tenancyv1alpha1.Workspace, error) {
	d.indexLock.RLock()
	defer d.indexLock.RUnlock()

	node := d.workspaceHierarchyIndex[workspace]
	if node == nil {
		return nil, fmt.Errorf("workspace %s not found", workspace)
	}

	hierarchy := []*tenancyv1alpha1.Workspace{node.workspace}

	// Add children recursively
	children, err := d.getChildrenRecursive(workspace, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get children: %w", err)
	}
	hierarchy = append(hierarchy, children...)

	return hierarchy, nil
}

// Event handlers for workspace indexing
func (d *WorkspaceDiscovererImpl) onWorkspaceAdd(obj interface{}) {
	workspace, ok := obj.(*tenancyv1alpha1.Workspace)
	if !ok {
		klog.ErrorS(nil, "Failed to cast object to workspace", "object", obj)
		return
	}
	d.indexWorkspace(workspace)
}

func (d *WorkspaceDiscovererImpl) onWorkspaceUpdate(oldObj, newObj interface{}) {
	d.onWorkspaceAdd(newObj)
}

func (d *WorkspaceDiscovererImpl) onWorkspaceDelete(obj interface{}) {
	workspace, ok := obj.(*tenancyv1alpha1.Workspace)
	if !ok {
		klog.ErrorS(nil, "Failed to cast object to workspace", "object", obj)
		return
	}
	d.removeWorkspaceFromIndex(workspace)
}

// Event handlers for sync target indexing
func (d *WorkspaceDiscovererImpl) onSyncTargetAdd(obj interface{}) {
	// Sync targets don't need special indexing beyond cache for this implementation
}

func (d *WorkspaceDiscovererImpl) onSyncTargetUpdate(oldObj, newObj interface{}) {
	// No special handling needed
}

func (d *WorkspaceDiscovererImpl) onSyncTargetDelete(obj interface{}) {
	// No special handling needed
}

// indexWorkspace updates all indices when a workspace is added or updated.
func (d *WorkspaceDiscovererImpl) indexWorkspace(workspace *tenancyv1alpha1.Workspace) {
	d.indexLock.Lock()
	defer d.indexLock.Unlock()

	wsName := logicalcluster.From(workspace)

	// Update hierarchy index
	node := &hierarchyNode{
		workspace: workspace,
		children:  []logicalcluster.Name{},
	}

	// Determine parent from workspace annotations or spec
	if parent := d.getWorkspaceParent(workspace); parent != "" {
		node.parent = parent
		// Add this workspace as child to parent
		if parentNode := d.workspaceHierarchyIndex[parent]; parentNode != nil {
			parentNode.children = append(parentNode.children, wsName)
		}
	}

	d.workspaceHierarchyIndex[wsName] = node

	// Update label index
	d.updateWorkspaceLabelIndex(workspace)
}

// Helper functions
func (d *WorkspaceDiscovererImpl) getWorkspacesByLabel(selector labels.Selector) ([]*tenancyv1alpha1.Workspace, error) {
	d.indexLock.RLock()
	defer d.indexLock.RUnlock()

	var result []*tenancyv1alpha1.Workspace
	for _, workspaces := range d.workspaceLabelIndex {
		for _, ws := range workspaces {
			if selector.Matches(labels.Set(ws.Labels)) {
				result = append(result, ws)
			}
		}
	}
	return result, nil
}

func (d *WorkspaceDiscovererImpl) getAllWorkspaces() []*tenancyv1alpha1.Workspace {
	objects := d.workspaceCache.List()
	workspaces := make([]*tenancyv1alpha1.Workspace, 0, len(objects))
	
	for _, obj := range objects {
		if ws, ok := obj.(*tenancyv1alpha1.Workspace); ok {
			workspaces = append(workspaces, ws)
		}
	}
	return workspaces
}

func (d *WorkspaceDiscovererImpl) filterReadyWorkspaces(workspaces []*tenancyv1alpha1.Workspace) []*tenancyv1alpha1.Workspace {
	var ready []*tenancyv1alpha1.Workspace
	for _, ws := range workspaces {
		if ws.Status.Phase == tenancyv1alpha1.LogicalClusterPhaseReady {
			ready = append(ready, ws)
		}
	}
	return ready
}

func (d *WorkspaceDiscovererImpl) getSyncTargetsByWorkspace(workspace logicalcluster.Name) []*workloadv1alpha1.SyncTarget {
	objects := d.syncTargetCache.List()
	var syncTargets []*workloadv1alpha1.SyncTarget
	
	for _, obj := range objects {
		if st, ok := obj.(*workloadv1alpha1.SyncTarget); ok {
			stWorkspace := logicalcluster.From(st)
			if stWorkspace == workspace {
				syncTargets = append(syncTargets, st)
			}
		}
	}
	return syncTargets
}

func (d *WorkspaceDiscovererImpl) getWorkspaceParent(workspace *tenancyv1alpha1.Workspace) logicalcluster.Name {
	// In KCP, parent workspace is determined by the logical cluster path
	wsName := logicalcluster.From(workspace)
	path := wsName.String()
	
	// Find the last colon to determine parent
	if lastColon := strings.LastIndex(path, ":"); lastColon > 0 {
		parentPath := path[:lastColon]
		return logicalcluster.Name(parentPath)
	}
	
	return ""
}

func (d *WorkspaceDiscovererImpl) updateWorkspaceLabelIndex(workspace *tenancyv1alpha1.Workspace) {
	// Remove old entries first
	d.removeWorkspaceFromLabelIndex(workspace)
	
	// Add new entries
	for key, value := range workspace.Labels {
		indexKey := fmt.Sprintf("%s=%s", key, value)
		d.workspaceLabelIndex[indexKey] = append(d.workspaceLabelIndex[indexKey], workspace)
	}
}

func (d *WorkspaceDiscovererImpl) removeWorkspaceFromLabelIndex(workspace *tenancyv1alpha1.Workspace) {
	wsName := logicalcluster.From(workspace)
	for key, workspaces := range d.workspaceLabelIndex {
		for i, ws := range workspaces {
			if logicalcluster.From(ws) == wsName {
				// Remove this workspace from the slice
				d.workspaceLabelIndex[key] = append(workspaces[:i], workspaces[i+1:]...)
				break
			}
		}
	}
}

func (d *WorkspaceDiscovererImpl) removeWorkspaceFromIndex(workspace *tenancyv1alpha1.Workspace) {
	d.indexLock.Lock()
	defer d.indexLock.Unlock()

	wsName := logicalcluster.From(workspace)
	
	// Remove from hierarchy index
	delete(d.workspaceHierarchyIndex, wsName)
	
	// Remove from label index
	d.removeWorkspaceFromLabelIndex(workspace)
}

func (d *WorkspaceDiscovererImpl) getChildrenRecursive(workspace logicalcluster.Name, depth int) ([]*tenancyv1alpha1.Workspace, error) {
	node := d.workspaceHierarchyIndex[workspace]
	if node == nil {
		return nil, nil
	}

	var result []*tenancyv1alpha1.Workspace
	for _, childName := range node.children {
		if childNode := d.workspaceHierarchyIndex[childName]; childNode != nil {
			result = append(result, childNode.workspace)
			
			// Recursively get children if depth allows
			if depth < 10 { // Prevent infinite recursion
				grandChildren, err := d.getChildrenRecursive(childName, depth+1)
				if err != nil {
					return nil, err
				}
				result = append(result, grandChildren...)
			}
		}
	}
	
	return result, nil
}