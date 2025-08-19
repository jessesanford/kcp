/*
Copyright 2025 The KCP Authors.

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

package auth

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/authorization/authorizerfactory"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/kcp/sdk/apis/core/v1alpha1"
)

// TMCAuthorizer provides authorization for TMC virtual workspaces
// with resource-level permissions and workspace-scoped access control.
type TMCAuthorizer struct {
	delegate   authorizer.Authorizer
	workspaces WorkspaceProvider
}

// ResourceAccessPolicy defines access policies for TMC resources
type ResourceAccessPolicy struct {
	// AllowedResources defines which resources can be accessed in virtual workspaces
	AllowedResources map[string][]string // group -> []resources
	// WorkspaceAdmins defines users with full workspace access
	WorkspaceAdmins []string
	// DefaultPermissions defines default permissions for authenticated users
	DefaultPermissions []string // verbs like "get", "list", "watch"
}

// NewTMCAuthorizer creates a new TMC virtual workspace authorizer
func NewTMCAuthorizer(delegate authorizer.Authorizer, workspaces WorkspaceProvider) *TMCAuthorizer {
	return &TMCAuthorizer{
		delegate:   delegate,
		workspaces: workspaces,
	}
}

// Authorize makes an authorization decision based on TMC virtual workspace context
func (a *TMCAuthorizer) Authorize(ctx context.Context, attr authorizer.Attributes) (authorizer.Decision, string, error) {
	// Extract workspace from the request URL
	workspace := extractWorkspaceFromURL(attr.GetPath())
	if workspace == "" {
		// No workspace context, delegate to standard authorizer
		return a.delegate.Authorize(ctx, attr)
	}

	klog.V(4).Infof("TMC authorization request for user %s in workspace %s: %s %s/%s",
		attr.GetUser().GetName(), workspace, attr.GetVerb(), attr.GetAPIGroup(), attr.GetResource())

	// Check if workspace exists and user has access
	if err := a.validateWorkspaceAccess(ctx, attr.GetUser(), workspace); err != nil {
		klog.V(2).Infof("TMC workspace access denied for user %s to workspace %s: %v", 
			attr.GetUser().GetName(), workspace, err)
		return authorizer.DecisionDeny, fmt.Sprintf("workspace access denied: %v", err), nil
	}

	// Check TMC-specific resource permissions
	if allowed, reason := a.checkTMCResourceAccess(attr, workspace); !allowed {
		klog.V(2).Infof("TMC resource access denied for user %s in workspace %s: %s", 
			attr.GetUser().GetName(), workspace, reason)
		return authorizer.DecisionDeny, reason, nil
	}

	// Delegate to underlying authorizer for final decision
	decision, reason, err := a.delegate.Authorize(ctx, attr)
	
	klog.V(4).Infof("TMC authorization decision for user %s in workspace %s: %v (reason: %s)",
		attr.GetUser().GetName(), workspace, decision, reason)
	
	return decision, reason, err
}

// validateWorkspaceAccess validates that the user has access to the workspace
func (a *TMCAuthorizer) validateWorkspaceAccess(ctx context.Context, user authorizerfactory.UserInfo, workspaceName string) error {
	// Get the workspace
	ws, err := a.workspaces.GetWorkspace(ctx, workspaceName)
	if err != nil {
		return fmt.Errorf("failed to get workspace %s: %w", workspaceName, err)
	}

	if ws == nil {
		return fmt.Errorf("workspace %s not found", workspaceName)
	}

	// Check if user has access to this workspace
	userWorkspaces, err := a.workspaces.ListWorkspacesForUser(ctx, user)
	if err != nil {
		return fmt.Errorf("failed to list workspaces for user %s: %w", user.GetName(), err)
	}

	for _, userWs := range userWorkspaces {
		if userWs.Name == workspaceName {
			return nil
		}
	}

	return fmt.Errorf("user %s does not have access to workspace %s", user.GetName(), workspaceName)
}

// checkTMCResourceAccess checks if the user can access the requested TMC resource
func (a *TMCAuthorizer) checkTMCResourceAccess(attr authorizer.Attributes, workspace string) (bool, string) {
	// Define TMC-specific resource access policies
	policy := a.getTMCResourcePolicy()
	
	// Check if the resource is explicitly allowed
	group := attr.GetAPIGroup()
	resource := attr.GetResource()
	verb := attr.GetVerb()
	
	// Allow access to TMC API groups
	if strings.Contains(group, "tmc.kcp.io") {
		// Check if user is workspace admin
		if a.isWorkspaceAdmin(attr.GetUser(), workspace) {
			return true, ""
		}
		
		// Check resource-specific permissions
		if allowedResources, ok := policy.AllowedResources[group]; ok {
			for _, allowedResource := range allowedResources {
				if allowedResource == resource || allowedResource == "*" {
					// Check if verb is allowed
					return a.isVerbAllowed(verb, policy.DefaultPermissions), ""
				}
			}
		}
		
		return false, fmt.Sprintf("resource %s/%s not allowed in workspace %s", group, resource, workspace)
	}
	
	// Allow standard Kubernetes resources with basic permissions
	if group == "" || group == "v1" {
		return a.isVerbAllowed(verb, policy.DefaultPermissions), ""
	}
	
	// Allow other API groups by default for backward compatibility
	return true, ""
}

// getTMCResourcePolicy returns the TMC resource access policy
func (a *TMCAuthorizer) getTMCResourcePolicy() *ResourceAccessPolicy {
	return &ResourceAccessPolicy{
		AllowedResources: map[string][]string{
			"tmc.kcp.io/v1alpha1": {"clusters", "placements", "workloaddistributions"},
			"scheduling.tmc.kcp.io/v1alpha1": {"placements", "schedulingconstraints"},
			"workload.tmc.kcp.io/v1alpha1": {"workloaddistributions", "syncresources"},
		},
		WorkspaceAdmins: []string{
			"system:admin",
			"system:masters",
		},
		DefaultPermissions: []string{"get", "list", "watch"},
	}
}

// isWorkspaceAdmin checks if the user is a workspace administrator
func (a *TMCAuthorizer) isWorkspaceAdmin(user authorizerfactory.UserInfo, workspace string) bool {
	policy := a.getTMCResourcePolicy()
	
	username := user.GetName()
	for _, admin := range policy.WorkspaceAdmins {
		if admin == username {
			return true
		}
	}
	
	// Check if user belongs to admin groups
	for _, group := range user.GetGroups() {
		for _, admin := range policy.WorkspaceAdmins {
			if admin == group {
				return true
			}
		}
		if group == fmt.Sprintf("workspace:%s:admin", workspace) {
			return true
		}
	}
	
	return false
}

// isVerbAllowed checks if the verb is in the allowed list
func (a *TMCAuthorizer) isVerbAllowed(verb string, allowedVerbs []string) bool {
	for _, allowed := range allowedVerbs {
		if allowed == verb || allowed == "*" {
			return true
		}
	}
	return false
}

// extractWorkspaceFromURL extracts the workspace name from a virtual workspace URL
func extractWorkspaceFromURL(path string) string {
	// Expected URL format: /services/apiexport/<workspace>/...
	// or /clusters/<workspace>/...
	parts := strings.Split(strings.Trim(path, "/"), "/")
	
	if len(parts) >= 3 {
		if parts[0] == "services" && parts[1] == "apiexport" {
			return parts[2]
		}
		if parts[0] == "clusters" {
			return parts[1]
		}
	}
	
	return ""
}