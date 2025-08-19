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
	"crypto/x509"
	"fmt"
	"net/http"
	"strings"

	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"

	"github.com/kcp-dev/kcp/pkg/features"
)

// TMCVirtualWorkspaceAuthenticator provides authentication for TMC virtual workspaces.
// It validates that users have appropriate access to TMC resources and handles
// workspace-scoped authentication for multi-tenant TMC operations.
type TMCVirtualWorkspaceAuthenticator struct {
	delegate       authenticator.Request
	workspaceAuth  WorkspaceAuthenticator
	tmcAuthEnabled bool
}

// WorkspaceAuthenticator provides workspace-specific authentication logic.
type WorkspaceAuthenticator interface {
	// AuthenticateWorkspace validates user access to a specific logical cluster
	AuthenticateWorkspace(ctx context.Context, user user.Info, workspace logicalcluster.Name) error
	
	// GetUserWorkspaces returns the list of workspaces a user has access to
	GetUserWorkspaces(ctx context.Context, user user.Info) ([]logicalcluster.Name, error)
}

// TMCAuthenticationRequest represents a TMC-specific authentication request.
type TMCAuthenticationRequest struct {
	// User is the authenticated user information
	User user.Info
	
	// TargetWorkspace is the workspace being accessed
	TargetWorkspace logicalcluster.Name
	
	// RequestPath is the request path being accessed
	RequestPath string
	
	// Method is the HTTP method
	Method string
}

// NewTMCVirtualWorkspaceAuthenticator creates a new TMC virtual workspace authenticator.
func NewTMCVirtualWorkspaceAuthenticator(
	delegate authenticator.Request,
	workspaceAuth WorkspaceAuthenticator,
) *TMCVirtualWorkspaceAuthenticator {
	return &TMCVirtualWorkspaceAuthenticator{
		delegate:       delegate,
		workspaceAuth:  workspaceAuth,
		tmcAuthEnabled: features.DefaultMutableFeatureGate.Enabled(features.TMCEnabled),
	}
}

// AuthenticateRequest authenticates the incoming request for TMC virtual workspace access.
func (a *TMCVirtualWorkspaceAuthenticator) AuthenticateRequest(req *http.Request) (*authenticator.Response, bool, error) {
	if !a.tmcAuthEnabled {
		// Fall back to standard authentication when TMC is disabled
		return a.delegate.AuthenticateRequest(req)
	}

	// First, perform standard authentication
	resp, ok, err := a.delegate.AuthenticateRequest(req)
	if !ok || err != nil {
		return resp, ok, err
	}

	// Extract TMC-specific authentication information from the request
	tmcRequest, err := a.extractTMCRequest(req, resp.User)
	if err != nil {
		klog.V(4).Infof("Failed to extract TMC request: %v", err)
		return resp, false, fmt.Errorf("invalid TMC virtual workspace request: %w", err)
	}

	// Validate TMC workspace access
	if err := a.validateTMCAccess(req.Context(), tmcRequest); err != nil {
		klog.V(2).Infof("TMC workspace access denied for user %s: %v", resp.User.GetName(), err)
		return resp, false, fmt.Errorf("TMC workspace access denied: %w", err)
	}

	klog.V(4).Infof("TMC virtual workspace access granted for user %s to workspace %s", 
		resp.User.GetName(), tmcRequest.TargetWorkspace)

	return resp, true, nil
}

// extractTMCRequest extracts TMC-specific information from the HTTP request.
func (a *TMCVirtualWorkspaceAuthenticator) extractTMCRequest(req *http.Request, userInfo user.Info) (*TMCAuthenticationRequest, error) {
	// Extract workspace from the URL path
	// TMC virtual workspace URLs follow the pattern: /services/tmc/workspaces/{workspace}/...
	pathParts := strings.Split(strings.Trim(req.URL.Path, "/"), "/")
	
	if len(pathParts) < 4 || pathParts[0] != "services" || pathParts[1] != "tmc" || pathParts[2] != "workspaces" {
		return nil, fmt.Errorf("invalid TMC virtual workspace URL format: %s", req.URL.Path)
	}

	workspace := logicalcluster.Name(pathParts[3])
	if workspace == "" {
		return nil, fmt.Errorf("empty workspace name in URL: %s", req.URL.Path)
	}

	return &TMCAuthenticationRequest{
		User:            userInfo,
		TargetWorkspace: workspace,
		RequestPath:     req.URL.Path,
		Method:          req.Method,
	}, nil
}

// validateTMCAccess validates that the user has appropriate access to the TMC workspace.
func (a *TMCVirtualWorkspaceAuthenticator) validateTMCAccess(ctx context.Context, tmcReq *TMCAuthenticationRequest) error {
	// Check if the user has access to the target workspace
	if err := a.workspaceAuth.AuthenticateWorkspace(ctx, tmcReq.User, tmcReq.TargetWorkspace); err != nil {
		return fmt.Errorf("workspace authentication failed: %w", err)
	}

	// Additional TMC-specific access checks can be added here
	// For example, checking for specific TMC roles or permissions

	return nil
}

// defaultWorkspaceAuthenticator provides a simple workspace authentication implementation.
type defaultWorkspaceAuthenticator struct{}

// NewDefaultWorkspaceAuthenticator creates a default workspace authenticator.
func NewDefaultWorkspaceAuthenticator() WorkspaceAuthenticator {
	return &defaultWorkspaceAuthenticator{}
}

// AuthenticateWorkspace validates user access to a workspace.
func (a *defaultWorkspaceAuthenticator) AuthenticateWorkspace(ctx context.Context, user user.Info, workspace logicalcluster.Name) error {
	// Simple implementation - allow access if user is authenticated
	// In production, this would check RBAC permissions or workspace membership
	
	if user == nil || user.GetName() == "" {
		return fmt.Errorf("user not authenticated")
	}

	// Check for system users that should have access to all workspaces
	if strings.HasPrefix(user.GetName(), "system:") {
		return nil
	}

	// For now, allow all authenticated users
	// TODO: Implement proper workspace-based authorization
	klog.V(4).Infof("Allowing user %s access to workspace %s", user.GetName(), workspace)
	return nil
}

// GetUserWorkspaces returns the workspaces accessible to the user.
func (a *defaultWorkspaceAuthenticator) GetUserWorkspaces(ctx context.Context, user user.Info) ([]logicalcluster.Name, error) {
	// Simple implementation - return empty list, meaning all workspaces are accessible
	// In production, this would query the user's workspace memberships
	return nil, nil
}