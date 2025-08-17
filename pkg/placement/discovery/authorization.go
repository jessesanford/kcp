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

	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"
)

// authCheckerImpl implements the AuthorizationChecker interface using the Kubernetes
// authorization system to verify workspace access permissions.
type authCheckerImpl struct {
	authorizer authorizer.Authorizer
}

// NewAuthorizationChecker creates a new authorization checker using the provided authorizer.
func NewAuthorizationChecker(authz authorizer.Authorizer) AuthorizationChecker {
	if authz == nil {
		klog.Warning("No authorizer provided to NewAuthorizationChecker - authorization will always pass")
	}
	return &authCheckerImpl{
		authorizer: authz,
	}
}

// CanAccessWorkspace checks if a user can access a specific workspace by verifying
// they have 'get' permissions on the workspace resource.
func (a *authCheckerImpl) CanAccessWorkspace(ctx context.Context, user user.Info, workspace logicalcluster.Name) (bool, error) {
	if a.authorizer == nil {
		// If no authorizer is configured, allow access
		klog.V(4).InfoS("No authorizer configured, allowing workspace access", "workspace", workspace, "user", user.GetName())
		return true, nil
	}

	if user == nil {
		return false, fmt.Errorf("user cannot be nil")
	}

	// Create authorization attributes for workspace access
	attrs := &authorizer.AttributesRecord{
		User:            user,
		Verb:            "get",
		Namespace:       "", // Workspaces are cluster-scoped
		Resource:        "workspaces",
		ResourceRequest: true,
		APIGroup:        "tenancy.kcp.io",
		APIVersion:      "v1alpha1",
		Name:            workspace.String(),
	}

	// Check authorization
	decision, reason, err := a.authorizer.Authorize(ctx, attrs)
	if err != nil {
		klog.V(2).InfoS("Authorization check error", "workspace", workspace, "user", user.GetName(), "error", err)
		return false, fmt.Errorf("authorization check failed: %w", err)
	}

	authorized := decision == authorizer.DecisionAllow
	if !authorized {
		klog.V(4).InfoS("Workspace access denied", 
			"workspace", workspace, 
			"user", user.GetName(), 
			"reason", reason,
		)
	}

	return authorized, nil
}

// GetPermittedWorkspaces returns all workspaces the user has access to.
// Note: This is a simplified implementation that checks against known workspaces.
// In a production system, you might want to implement more sophisticated discovery.
func (a *authCheckerImpl) GetPermittedWorkspaces(ctx context.Context, user user.Info) ([]logicalcluster.Name, error) {
	if a.authorizer == nil {
		klog.V(4).InfoS("No authorizer configured, cannot determine permitted workspaces", "user", user.GetName())
		return nil, fmt.Errorf("no authorizer configured")
	}

	if user == nil {
		return nil, fmt.Errorf("user cannot be nil")
	}

	// This is a simplified approach. In a full implementation, you would:
	// 1. List all available workspaces (potentially from an index)
	// 2. Check authorization for each workspace
	// 3. Return only those the user can access
	//
	// For this discovery implementation, we'll return an empty list and let
	// the caller use CanAccessWorkspace for specific workspace checks.

	klog.V(4).InfoS("GetPermittedWorkspaces called - returning empty list for simplified implementation", "user", user.GetName())
	return []logicalcluster.Name{}, nil
}

// WorkspaceAccessVerifier provides a higher-level interface for checking workspace access
// with additional context and caching capabilities.
type WorkspaceAccessVerifier struct {
	authChecker AuthorizationChecker
	// Future: Add caching layer here
}

// NewWorkspaceAccessVerifier creates a new workspace access verifier.
func NewWorkspaceAccessVerifier(authChecker AuthorizationChecker) *WorkspaceAccessVerifier {
	return &WorkspaceAccessVerifier{
		authChecker: authChecker,
	}
}

// VerifyAccess checks if the user can perform the specified action on the workspace.
func (v *WorkspaceAccessVerifier) VerifyAccess(ctx context.Context, user user.Info, workspace logicalcluster.Name, action string) (bool, error) {
	if v.authChecker == nil {
		return false, fmt.Errorf("no authorization checker configured")
	}

	switch action {
	case "read", "get", "list":
		return v.authChecker.CanAccessWorkspace(ctx, user, workspace)
	default:
		// For actions other than read, we could extend the authorization check
		// but for workspace discovery, read access is the primary concern
		return false, fmt.Errorf("unsupported action: %s", action)
	}
}

// BatchVerifyAccess checks access to multiple workspaces efficiently.
func (v *WorkspaceAccessVerifier) BatchVerifyAccess(ctx context.Context, user user.Info, workspaces []logicalcluster.Name) (map[logicalcluster.Name]bool, error) {
	if v.authChecker == nil {
		return nil, fmt.Errorf("no authorization checker configured")
	}

	results := make(map[logicalcluster.Name]bool, len(workspaces))
	
	for _, workspace := range workspaces {
		authorized, err := v.authChecker.CanAccessWorkspace(ctx, user, workspace)
		if err != nil {
			klog.V(2).InfoS("Batch authorization check failed for workspace", 
				"workspace", workspace, 
				"user", user.GetName(), 
				"error", err,
			)
			// Continue checking other workspaces even if one fails
			results[workspace] = false
		} else {
			results[workspace] = authorized
		}
	}

	return results, nil
}

// createSubjectAccessReview creates a SubjectAccessReview for the given parameters.
// This is a helper function that could be used for more complex authorization scenarios.
func createSubjectAccessReview(user user.Info, workspace logicalcluster.Name, verb string) *authorizationv1.SubjectAccessReview {
	return &authorizationv1.SubjectAccessReview{
		Spec: authorizationv1.SubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Namespace: "",
				Verb:      verb,
				Group:     "tenancy.kcp.io",
				Version:   "v1alpha1",
				Resource:  "workspaces",
				Name:      workspace.String(),
			},
			User:   user.GetName(),
			Groups: user.GetGroups(),
			Extra:  convertUserExtra(user.GetExtra()),
		},
	}
}

// convertUserExtra converts user.Info extra data to the format expected by SubjectAccessReview.
func convertUserExtra(extra map[string][]string) map[string]authorizationv1.ExtraValue {
	if extra == nil {
		return nil
	}
	
	converted := make(map[string]authorizationv1.ExtraValue, len(extra))
	for key, values := range extra {
		converted[key] = authorizationv1.ExtraValue(values)
	}
	return converted
}