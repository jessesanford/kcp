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

	"github.com/kcp-dev/logicalcluster/v3"

	apisv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/apis/v1alpha1"
)

// WorkspaceIntegrator provides KCP workspace integration utilities
type WorkspaceIntegrator struct {
	// workspace is the logical cluster for this integrator
	workspace logicalcluster.Name
}

// NewWorkspaceIntegrator creates a new workspace integrator
func NewWorkspaceIntegrator(workspace logicalcluster.Name) *WorkspaceIntegrator {
	return &WorkspaceIntegrator{
		workspace: workspace,
	}
}

// FilterAPIExportsForWorkspace filters APIExports relevant to the workspace
func (i *WorkspaceIntegrator) FilterAPIExportsForWorkspace(
	ctx context.Context,
	apiExports []*apisv1alpha1.APIExport,
) ([]*apisv1alpha1.APIExport, error) {
	if len(apiExports) == 0 {
		return nil, nil
	}

	var filtered []*apisv1alpha1.APIExport
	
	for _, apiExport := range apiExports {
		if apiExport == nil {
			continue
		}

		// Check if this APIExport is available in our workspace
		available, err := i.IsAPIExportAvailable(ctx, apiExport)
		if err != nil {
			return nil, fmt.Errorf("failed to check APIExport availability for %s: %w", apiExport.Name, err)
		}

		if available {
			filtered = append(filtered, apiExport)
		}
	}

	return filtered, nil
}

// IsAPIExportAvailable checks if an APIExport is available in the workspace
func (i *WorkspaceIntegrator) IsAPIExportAvailable(
	ctx context.Context,
	apiExport *apisv1alpha1.APIExport,
) (bool, error) {
	if apiExport == nil {
		return false, fmt.Errorf("apiExport cannot be nil")
	}

	// Check if the APIExport belongs to our workspace or is globally accessible
	// Extract workspace from annotations
	if workspace, ok := apiExport.Annotations["cluster.kcp.io/workspace"]; ok {
		if workspace == i.workspace.String() {
			return true, nil
		}
	}

	// Check if the APIExport is marked as globally available
	if apiExport.Annotations != nil {
		if globalStr, exists := apiExport.Annotations["apis.kcp.io/global"]; exists && globalStr == "true" {
			return true, nil
		}
	}

	// Check if we have access through workspace hierarchy
	// This is a simplified check - in practice, you'd validate the workspace tree
	return i.checkWorkspaceHierarchyAccess(ctx, apiExport)
}

// checkWorkspaceHierarchyAccess checks access through workspace hierarchy
func (i *WorkspaceIntegrator) checkWorkspaceHierarchyAccess(
	ctx context.Context,
	apiExport *apisv1alpha1.APIExport,
) (bool, error) {
	// Simplified workspace hierarchy check
	// In practice, this would traverse the workspace tree and check permissions
	
	// For now, allow access if the APIExport is in a parent workspace
	// This is a simplified check - in practice, you'd validate the workspace tree
	if workspace, ok := apiExport.Annotations["cluster.kcp.io/workspace"]; ok {
		// Simple prefix check for hierarchy
		if len(workspace) < len(i.workspace.String()) && 
			i.workspace.String()[:len(workspace)] == workspace {
			return true, nil
		}
	}

	return false, nil
}

// ResolveLogicalCluster resolves workspace references to logical clusters
func (i *WorkspaceIntegrator) ResolveLogicalCluster(workspace string) (logicalcluster.Name, error) {
	if workspace == "" {
		return logicalcluster.Name{}, fmt.Errorf("workspace cannot be empty")
	}

	// Parse the workspace string into a logical cluster name
	logicalCluster := logicalcluster.NewPath(workspace)
	return logicalcluster.Name(logicalCluster), nil
}

// ValidateWorkspaceAccess validates that discovery is allowed for the workspace
func (i *WorkspaceIntegrator) ValidateWorkspaceAccess(ctx context.Context, workspace string) error {
	// Resolve the target workspace
	targetWorkspace, err := i.ResolveLogicalCluster(workspace)
	if err != nil {
		return fmt.Errorf("failed to resolve target workspace: %w", err)
	}

	// Check if we can access this workspace
	// This is a simplified check - in practice, you'd validate permissions
	if targetWorkspace == i.workspace {
		return nil // Always allow access to our own workspace
	}

	// Check if it's a child workspace
	if i.isChildWorkspace(targetWorkspace) {
		return nil
	}

	// Check if it's a parent workspace (with proper permissions)
	if i.isParentWorkspace(targetWorkspace) {
		return nil
	}

	return fmt.Errorf("access denied to workspace %q from %q", workspace, i.workspace.String())
}

// isChildWorkspace checks if the target is a child of our workspace
func (i *WorkspaceIntegrator) isChildWorkspace(target logicalcluster.Name) bool {
	ourWorkspace := i.workspace.String()
	targetWorkspace := target.String()

	// If target workspace is longer and starts with our workspace, it's a child
	if len(targetWorkspace) > len(ourWorkspace) {
		return targetWorkspace[:len(ourWorkspace)] == ourWorkspace
	}
	return false
}

// isParentWorkspace checks if the target is a parent of our workspace
func (i *WorkspaceIntegrator) isParentWorkspace(target logicalcluster.Name) bool {
	ourWorkspace := i.workspace.String()
	targetWorkspace := target.String()

	// If our workspace is longer and starts with target workspace, it's a parent
	if len(ourWorkspace) > len(targetWorkspace) {
		return ourWorkspace[:len(targetWorkspace)] == targetWorkspace
	}
	return false
}

// ExtractFeatureGates extracts relevant feature gates for discovery
func (i *WorkspaceIntegrator) ExtractFeatureGates() map[string]bool {
	// Return a map of feature gates relevant to discovery
	// In practice, this would query the actual feature gate configuration
	return map[string]bool{
		"VirtualWorkspaces":        true,
		"APIExportDiscovery":       true,
		"WorkspaceAwareDiscovery":  true,
		"DiscoveryCache":          true,
		"CrossWorkspaceDiscovery": false, // Disabled by default for security
	}
}