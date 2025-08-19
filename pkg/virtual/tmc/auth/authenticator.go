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
	"net/http"
	"strings"

	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/kcp/sdk/apis/core/v1alpha1"
)

// TMCAuthenticator provides authentication for TMC virtual workspaces
// with multi-tenant workspace scoping and feature gate integration.
type TMCAuthenticator struct {
	delegate   authenticator.Request
	workspaces WorkspaceProvider
}

// WorkspaceProvider provides access to workspace information for authentication decisions
type WorkspaceProvider interface {
	GetWorkspace(ctx context.Context, name string) (*v1alpha1.Workspace, error)
	ListWorkspacesForUser(ctx context.Context, user user.Info) ([]*v1alpha1.Workspace, error)
}

// NewTMCAuthenticator creates a new TMC virtual workspace authenticator
func NewTMCAuthenticator(delegate authenticator.Request, workspaces WorkspaceProvider) *TMCAuthenticator {
	return &TMCAuthenticator{
		delegate:   delegate,
		workspaces: workspaces,
	}
}

// AuthenticateRequest authenticates a request in the context of TMC virtual workspaces
func (a *TMCAuthenticator) AuthenticateRequest(req *http.Request) (*authenticator.Response, bool, error) {
	// First delegate to the underlying authenticator
	resp, ok, err := a.delegate.AuthenticateRequest(req)
	if err != nil || !ok {
		return resp, ok, err
	}

	// Extract workspace from the request path
	workspace := extractWorkspaceFromPath(req.URL.Path)
	if workspace == "" {
		klog.V(4).Infof("No workspace found in path %s, using standard authentication", req.URL.Path)
		return resp, ok, nil
	}

	// Validate workspace access for the authenticated user
	ctx := req.Context()
	if err := a.validateWorkspaceAccess(ctx, resp.User, workspace); err != nil {
		klog.V(2).Infof("TMC workspace access denied for user %s to workspace %s: %v", 
			resp.User.GetName(), workspace, err)
		return nil, false, nil
	}

	klog.V(4).Infof("TMC virtual workspace access granted for user %s to workspace %s", 
		resp.User.GetName(), workspace)

	// Enhance user info with workspace context
	enhancedUser := &user.DefaultInfo{
		Name:   resp.User.GetName(),
		UID:    resp.User.GetUID(),
		Groups: append(resp.User.GetGroups(), fmt.Sprintf("workspace:%s", workspace)),
		Extra:  resp.User.GetExtra(),
	}

	return &authenticator.Response{
		User:      enhancedUser,
		Audiences: resp.Audiences,
	}, true, nil
}

// validateWorkspaceAccess checks if the user has access to the specified workspace
func (a *TMCAuthenticator) validateWorkspaceAccess(ctx context.Context, user user.Info, workspaceName string) error {
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

// extractWorkspaceFromPath extracts the workspace name from a virtual workspace path
func extractWorkspaceFromPath(path string) string {
	// Expected path format: /services/apiexport/<workspace>/...
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