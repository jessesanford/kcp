package discovery

import (
	"context"
	"fmt"
	"github.com/kcp-dev/kcp/pkg/placement/interfaces"
	workloadv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"
	kcpclient "github.com/kcp-dev/kcp/pkg/client/clientset/versioned"
	"github.com/kcp-dev/logicalcluster/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// ClusterCriteria defines criteria for finding clusters
type ClusterCriteria struct {
	WorkspaceSelector     labels.Selector
	LabelSelector         labels.Selector
	Regions               []string
	RequiredCapabilities  []string
}

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
func (f *ClusterFinder) FindClusters(ctx context.Context, criteria ClusterCriteria) ([]interfaces.ClusterTarget, error) {
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
	workspace interfaces.WorkspaceInfo, criteria ClusterCriteria) ([]interfaces.ClusterTarget, error) {
	
	cluster := logicalcluster.NewPath(string(workspace.Name))
	
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
	criteria ClusterCriteria) bool {
	
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

// hasCapability checks if sync target has a required capability
func (f *ClusterFinder) hasCapability(st *workloadv1alpha1.SyncTarget, capability string) bool {
	// Check labels for capability indicators
	if capabilities, ok := st.Labels["capabilities"]; ok {
		// Simple check - in reality would parse comma-separated list
		return capabilities == capability
	}
	return false
}

// syncTargetToClusterTarget converts SyncTarget to ClusterTarget
func (f *ClusterFinder) syncTargetToClusterTarget(st *workloadv1alpha1.SyncTarget, 
	workspace interfaces.WorkspaceInfo) interfaces.ClusterTarget {
	
	return interfaces.ClusterTarget{
		Name:      st.Name,
		Workspace: workspace.Name,
		Labels:    st.Labels,
		Capacity:  f.extractCapacity(st),
		Ready:     f.isReady(st),
		Location:  f.extractLocation(st),
	}
}

// extractCapacity extracts resource capacity from SyncTarget
func (f *ClusterFinder) extractCapacity(st *workloadv1alpha1.SyncTarget) interfaces.ResourceCapacity {
	// Extract from SyncTarget status or use defaults
	return interfaces.ResourceCapacity{
		CPU:    "4",
		Memory: "8Gi",
		Pods:   110,
	}
}

// extractLocation extracts location information from SyncTarget
func (f *ClusterFinder) extractLocation(st *workloadv1alpha1.SyncTarget) *interfaces.LocationInfo {
	if region, ok := st.Labels["region"]; ok {
		return &interfaces.LocationInfo{
			Name:   st.Labels["location"],
			Region: region,
			Zone:   st.Labels["zone"],
		}
	}
	return nil
}

// isReady checks if SyncTarget is ready for placement
func (f *ClusterFinder) isReady(st *workloadv1alpha1.SyncTarget) bool {
	// Check SyncTarget status conditions
	return true // Simplified
}