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

package auth

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"

	"github.com/kcp-dev/kcp/pkg/features"
)

// TMCVirtualWorkspaceAuthorizer provides authorization for TMC virtual workspace operations.
// It enforces workspace-scoped access controls and TMC-specific permissions.
type TMCVirtualWorkspaceAuthorizer struct {
	delegate        authorizer.Authorizer
	tmcAuthEnabled  bool
	allowedActions  map[string][]string // Resource -> allowed verbs
}

// TMCAuthorizationContext contains TMC-specific authorization information.
type TMCAuthorizationContext struct {
	// Workspace is the target workspace
	Workspace logicalcluster.Name
	
	// TMCResource is the TMC resource being accessed
	TMCResource string
	
	// IsVirtualWorkspaceRequest indicates if this is a virtual workspace request
	IsVirtualWorkspaceRequest bool
}

// NewTMCVirtualWorkspaceAuthorizer creates a new TMC virtual workspace authorizer.
func NewTMCVirtualWorkspaceAuthorizer(delegate authorizer.Authorizer) *TMCVirtualWorkspaceAuthorizer {
	return &TMCVirtualWorkspaceAuthorizer{
		delegate:       delegate,
		tmcAuthEnabled: features.DefaultMutableFeatureGate.Enabled(features.TMCEnabled),
		allowedActions: defaultTMCAllowedActions(),
	}
}

// Authorize performs authorization for TMC virtual workspace requests.
func (a *TMCVirtualWorkspaceAuthorizer) Authorize(ctx context.Context, attr authorizer.Attributes) (authorizer.Decision, string, error) {
	if !a.tmcAuthEnabled {
		// Fall back to standard authorization when TMC is disabled
		return a.delegate.Authorize(ctx, attr)
	}

	// Extract TMC context from the request
	tmcCtx, err := a.extractTMCContext(ctx, attr)
	if err != nil {
		klog.V(4).Infof("Failed to extract TMC context: %v", err)
		// Not a TMC virtual workspace request, use standard authorization
		return a.delegate.Authorize(ctx, attr)
	}

	// If this is not a TMC virtual workspace request, use standard authorization
	if !tmcCtx.IsVirtualWorkspaceRequest {
		return a.delegate.Authorize(ctx, attr)
	}

	// Perform TMC-specific authorization
	decision, reason, err := a.authorizeTMCRequest(ctx, attr, tmcCtx)
	if err != nil {
		return authorizer.DecisionDeny, fmt.Sprintf("TMC authorization error: %s", err.Error()), err
	}

	if decision != authorizer.DecisionAllow {
		klog.V(2).Infof("TMC authorization denied for user %s on resource %s: %s", 
			attr.GetUser().GetName(), attr.GetResource(), reason)
		return decision, reason, nil
	}

	// Also check with the delegate authorizer for standard RBAC
	delegateDecision, delegateReason, err := a.delegate.Authorize(ctx, attr)
	if err != nil {
		return authorizer.DecisionDeny, fmt.Sprintf("delegate authorization error: %s", err.Error()), err
	}

	if delegateDecision != authorizer.DecisionAllow {
		return delegateDecision, fmt.Sprintf("standard authorization denied: %s", delegateReason), nil
	}

	return authorizer.DecisionAllow, "TMC virtual workspace access granted", nil
}

// extractTMCContext extracts TMC-specific context from the authorization attributes.
func (a *TMCVirtualWorkspaceAuthorizer) extractTMCContext(ctx context.Context, attr authorizer.Attributes) (*TMCAuthorizationContext, error) {
	// Check if this is a virtual workspace request
	requestInfo, ok := request.RequestInfoFrom(ctx)
	if !ok {
		return nil, fmt.Errorf("no request info in context")
	}

	// Check if the request path indicates TMC virtual workspace access
	if !strings.Contains(requestInfo.Path, "/services/tmc/") {
		return &TMCAuthorizationContext{
			IsVirtualWorkspaceRequest: false,
		}, nil
	}

	// Extract workspace from the request path
	pathParts := strings.Split(strings.Trim(requestInfo.Path, "/"), "/")
	if len(pathParts) < 4 || pathParts[0] != "services" || pathParts[1] != "tmc" || pathParts[2] != "workspaces" {
		return nil, fmt.Errorf("invalid TMC virtual workspace path format")
	}

	workspace := logicalcluster.Name(pathParts[3])

	return &TMCAuthorizationContext{
		Workspace:                 workspace,
		TMCResource:              attr.GetResource(),
		IsVirtualWorkspaceRequest: true,
	}, nil
}

// authorizeTMCRequest performs TMC-specific authorization logic.
func (a *TMCVirtualWorkspaceAuthorizer) authorizeTMCRequest(ctx context.Context, attr authorizer.Attributes, tmcCtx *TMCAuthorizationContext) (authorizer.Decision, string, error) {
	// Check if the resource is a TMC resource
	if !a.isTMCResource(attr.GetResource()) {
		return authorizer.DecisionDeny, "non-TMC resource access through TMC virtual workspace", nil
	}

	// Check if the verb is allowed for this TMC resource
	allowedVerbs, exists := a.allowedActions[attr.GetResource()]
	if !exists {
		return authorizer.DecisionDeny, fmt.Sprintf("TMC resource %s not recognized", attr.GetResource()), nil
	}

	verb := attr.GetVerb()
	for _, allowedVerb := range allowedVerbs {
		if verb == allowedVerb {
			// Verb is allowed, check workspace-specific permissions
			return a.authorizeWorkspaceAccess(ctx, attr, tmcCtx)
		}
	}

	return authorizer.DecisionDeny, fmt.Sprintf("verb %s not allowed for TMC resource %s", verb, attr.GetResource()), nil
}

// authorizeWorkspaceAccess checks workspace-specific access permissions.
func (a *TMCVirtualWorkspaceAuthorizer) authorizeWorkspaceAccess(ctx context.Context, attr authorizer.Attributes, tmcCtx *TMCAuthorizationContext) (authorizer.Decision, string, error) {
	// For now, allow access if user is authenticated and resource is TMC
	// In production, this would check workspace-specific RBAC or membership

	user := attr.GetUser()
	if user == nil || user.GetName() == "" {
		return authorizer.DecisionDeny, "user not authenticated", nil
	}

	// Allow system users
	if strings.HasPrefix(user.GetName(), "system:") {
		return authorizer.DecisionAllow, "system user allowed", nil
	}

	// TODO: Implement proper workspace-scoped authorization
	// This could involve checking:
	// - Workspace membership
	// - TMC-specific roles
	// - Resource-specific permissions

	klog.V(4).Infof("TMC workspace access granted for user %s on resource %s in workspace %s", 
		user.GetName(), attr.GetResource(), tmcCtx.Workspace)

	return authorizer.DecisionAllow, "TMC workspace access granted", nil
}

// isTMCResource checks if the given resource is a TMC resource.
func (a *TMCVirtualWorkspaceAuthorizer) isTMCResource(resource string) bool {
	tmcResources := []string{
		"clusterregistrations",
		"workloadplacements", 
		"syncerconfigs",
		"workloadsyncs",
		"syncertunnels",
	}

	for _, tmcResource := range tmcResources {
		if resource == tmcResource {
			return true
		}
	}

	return false
}

// defaultTMCAllowedActions returns the default allowed actions for TMC resources.
func defaultTMCAllowedActions() map[string][]string {
	return map[string][]string{
		"clusterregistrations": {"get", "list", "create", "update", "patch", "delete"},
		"workloadplacements":   {"get", "list", "create", "update", "patch", "delete"},
		"syncerconfigs":        {"get", "list", "create", "update", "patch", "delete"},
		"workloadsyncs":        {"get", "list", "create", "update", "patch"},
		"syncertunnels":        {"get", "list", "create", "update", "patch", "delete"},
	}
}