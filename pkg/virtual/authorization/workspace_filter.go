/*
Copyright 2024 The KCP Authors.

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

package authorization

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"

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
	return &WorkspaceFilter{
		workspace:                 workspace,
		allowCrossWorkspaceAccess: allowCrossWorkspaceAccess,
	}
}

// FilterRequest validates and filters authorization requests for workspace boundaries
func (f *WorkspaceFilter) FilterRequest(ctx context.Context, req *interfaces.AuthorizationRequest) (*interfaces.AuthorizationRequest, error) {
	if req == nil {
		return nil, fmt.Errorf("authorization request cannot be nil")
	}

	// Create a copy of the request to avoid modifying the original
	filtered := &interfaces.AuthorizationRequest{
		User:         req.User,
		Groups:       req.Groups,
		Workspace:    req.Workspace,
		Verb:         req.Verb,
		Resource:     req.Resource,
		ResourceName: req.ResourceName,
		Path:         req.Path,
		Namespace:    req.Namespace,
	}

	// Extract workspace from path if not explicitly set
	if filtered.Workspace == "" {
		extractedWorkspace, err := f.ExtractWorkspaceFromPath(req.Path)
		if err == nil && extractedWorkspace != "" {
			filtered.Workspace = extractedWorkspace
		}
	}

	// Enforce workspace isolation
	if err := f.EnforceWorkspaceIsolation(ctx, filtered); err != nil {
		return nil, err
	}

	return filtered, nil
}

// ValidateWorkspaceAccess checks if access to a workspace is allowed
func (f *WorkspaceFilter) ValidateWorkspaceAccess(ctx context.Context, targetWorkspace, user string, groups []string) error {
	if targetWorkspace == "" {
		return fmt.Errorf("target workspace cannot be empty")
	}

	// Always allow access to the current workspace
	if targetWorkspace == f.workspace.String() {
		return nil
	}

	// Check if cross-workspace access is allowed
	if !f.allowCrossWorkspaceAccess {
		return fmt.Errorf("cross-workspace access not allowed: user %q cannot access workspace %q from workspace %q", 
			user, targetWorkspace, f.workspace)
	}

	// Additional validation logic could be added here
	// For example, checking if the user has permission to access the target workspace
	return nil
}

// IsWorkspaceResource determines if a resource is workspace-scoped
func (f *WorkspaceFilter) IsWorkspaceResource(gvr schema.GroupVersionResource) bool {
	// Define workspace-scoped resources
	workspaceResources := map[string]bool{
		"workspaces.tenancy.kcp.io":      true,
		"workspacetypes.tenancy.kcp.io":  true,
		"apiexports.apis.kcp.io":         true,
		"apiresourceschemas.apis.kcp.io": true,
		"apibindings.apis.kcp.io":        true,
	}

	resourceKey := gvr.Resource + "." + gvr.Group
	return workspaceResources[resourceKey]
}

// ExtractWorkspaceFromPath extracts workspace information from request path
func (f *WorkspaceFilter) ExtractWorkspaceFromPath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path cannot be empty")
	}

	// KCP paths: /clusters/<workspace>/...
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	
	if len(parts) >= 2 && parts[0] == "clusters" {
		workspace := parts[1]
		if workspace != "" {
			return workspace, nil
		}
	}

	// If not in cluster path format, assume it's the current workspace
	return f.workspace.String(), nil
}

// ResolveLogicalCluster resolves a workspace name to a logical cluster
func (f *WorkspaceFilter) ResolveLogicalCluster(workspace string) (logicalcluster.Name, error) {
	if workspace == "" {
		return "", fmt.Errorf("workspace name cannot be empty")
	}
	cluster, err := logicalcluster.New(workspace)
	if err != nil {
		return "", fmt.Errorf("invalid workspace name %q: %w", workspace, err)
	}
	return cluster, nil
}

// EnforceWorkspaceIsolation ensures requests don't cross workspace boundaries
func (f *WorkspaceFilter) EnforceWorkspaceIsolation(ctx context.Context, req *interfaces.AuthorizationRequest) error {
	// If no workspace is specified in the request, use the current workspace
	if req.Workspace == "" {
		req.Workspace = f.workspace.String()
		return nil
	}

	// Validate access to the target workspace
	if err := f.ValidateWorkspaceAccess(ctx, req.Workspace, req.User, req.Groups); err != nil {
		return fmt.Errorf("workspace isolation violation: %w", err)
	}

	// For workspace-scoped resources, ensure they're accessed within the correct workspace
	if f.IsWorkspaceResource(req.Resource) {
		targetCluster, err := f.ResolveLogicalCluster(req.Workspace)
		if err != nil {
			return fmt.Errorf("failed to resolve target workspace %q: %w", req.Workspace, err)
		}

		// Additional validation could be added here to check if the resource
		// actually exists in the target workspace
		_ = targetCluster
	}

	return nil
}