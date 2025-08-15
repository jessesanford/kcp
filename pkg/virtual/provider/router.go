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

package provider

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"
)

// Router handles request routing for virtual workspaces, parsing workspace information
// from request paths and routing to appropriate handlers
type Router struct {
	// provider is the parent workspace provider
	provider *WorkspaceProvider

	// workspacePathRegex matches workspace paths in URLs
	workspacePathRegex *regexp.Regexp
}

// NewRouter creates a new request router with the specified workspace provider
func NewRouter(provider *WorkspaceProvider) *Router {
	if provider == nil {
		panic("provider cannot be nil")
	}

	// Compile regex for matching workspace paths
	// Matches patterns like: /clusters/<workspace>/api/v1/... or /clusters/<workspace>/apis/...
	workspaceRegex := regexp.MustCompile(`^/clusters/([^/]+)(/.*)?$`)

	return &Router{
		provider:           provider,
		workspacePathRegex: workspaceRegex,
	}
}

// Route handles routing requests to the appropriate handlers based on the request path
func (r *Router) Route(w http.ResponseWriter, req *http.Request) {
	// Extract workspace name from path
	workspace, err := r.extractWorkspace(req.URL.Path)
	if err != nil {
		klog.V(2).InfoS("Failed to extract workspace from request", "path", req.URL.Path, "error", err)
		http.Error(w, fmt.Sprintf("Invalid workspace path: %v", err), http.StatusBadRequest)
		return
	}

	// Log the routing decision
	klog.V(4).InfoS("Routing request", "workspace", workspace, "path", req.URL.Path, "method", req.Method)

	// Determine the type of request and route accordingly
	if strings.Contains(req.URL.Path, "/api/v1") || strings.Contains(req.URL.Path, "/apis/") {
		// This is an API request, route to the API handler
		r.routeAPIRequest(w, req, workspace)
	} else if req.URL.Path == fmt.Sprintf("/clusters/%s", workspace) {
		// This is a workspace metadata request
		r.routeWorkspaceRequest(w, req, workspace)
	} else {
		// Unknown path pattern
		klog.V(2).InfoS("Unknown path pattern", "path", req.URL.Path, "workspace", workspace)
		http.Error(w, fmt.Sprintf("Unknown path pattern: %s", req.URL.Path), http.StatusNotFound)
	}
}

// routeAPIRequest handles API requests for resources within a workspace
func (r *Router) routeAPIRequest(w http.ResponseWriter, req *http.Request, workspace logicalcluster.Name) {
	// Parse the API path to extract resource information
	apiPath := strings.TrimPrefix(req.URL.Path, fmt.Sprintf("/clusters/%s", workspace))
	
	klog.V(4).InfoS("Routing API request", "workspace", workspace, "apiPath", apiPath, "method", req.Method)

	// Route based on HTTP method
	switch req.Method {
	case http.MethodGet:
		if strings.Contains(apiPath, "/watch") {
			r.provider.handler.HandleWatch(w, req)
		} else if strings.HasSuffix(apiPath, "/") || !strings.Contains(apiPath[strings.LastIndex(apiPath, "/"):], ".") {
			// List request (ends with / or no specific resource name)
			r.provider.handler.HandleList(w, req)
		} else {
			// Get specific resource
			r.provider.handler.HandleGet(w, req)
		}
	case http.MethodPost:
		r.provider.handler.HandleCreate(w, req)
	case http.MethodPut:
		r.provider.handler.HandleUpdate(w, req)
	case http.MethodPatch:
		r.provider.handler.HandlePatch(w, req)
	case http.MethodDelete:
		r.provider.handler.HandleDelete(w, req)
	default:
		klog.V(2).InfoS("Unsupported HTTP method", "method", req.Method, "workspace", workspace, "path", req.URL.Path)
		http.Error(w, fmt.Sprintf("Method %s not supported", req.Method), http.StatusMethodNotAllowed)
	}
}

// routeWorkspaceRequest handles requests for workspace metadata
func (r *Router) routeWorkspaceRequest(w http.ResponseWriter, req *http.Request, workspace logicalcluster.Name) {
	klog.V(4).InfoS("Routing workspace metadata request", "workspace", workspace, "method", req.Method)

	switch req.Method {
	case http.MethodGet:
		r.provider.handler.HandleWorkspaceGet(w, req)
	case http.MethodPost:
		r.provider.handler.HandleWorkspaceCreate(w, req)
	case http.MethodPut:
		r.provider.handler.HandleWorkspaceUpdate(w, req)
	case http.MethodDelete:
		r.provider.handler.HandleWorkspaceDelete(w, req)
	default:
		klog.V(2).InfoS("Unsupported HTTP method for workspace", "method", req.Method, "workspace", workspace)
		http.Error(w, fmt.Sprintf("Method %s not supported for workspace operations", req.Method), http.StatusMethodNotAllowed)
	}
}

// extractWorkspace extracts the workspace name from a request path
// Expects paths in the format: /clusters/<workspace>/...
func (r *Router) extractWorkspace(path string) (logicalcluster.Name, error) {
	if path == "" {
		return "", fmt.Errorf("path cannot be empty")
	}

	// Use regex to extract workspace name
	matches := r.workspacePathRegex.FindStringSubmatch(path)
	if len(matches) < 2 {
		return "", fmt.Errorf("path does not contain valid workspace: %s (expected format: /clusters/<workspace>/...)", path)
	}

	workspaceName := matches[1]
	if workspaceName == "" {
		return "", fmt.Errorf("workspace name cannot be empty")
	}

	// Validate workspace name format
	if err := r.validateWorkspaceName(workspaceName); err != nil {
		return "", fmt.Errorf("invalid workspace name %q: %v", workspaceName, err)
	}

	return logicalcluster.Name(workspaceName), nil
}

// validateWorkspaceName validates that a workspace name follows KCP naming conventions
func (r *Router) validateWorkspaceName(name string) error {
	if name == "" {
		return fmt.Errorf("workspace name cannot be empty")
	}

	// Basic validation - workspace names should follow DNS naming conventions
	if len(name) > 253 {
		return fmt.Errorf("workspace name too long (max 253 characters)")
	}

	// Check for invalid characters
	validNameRegex := regexp.MustCompile(`^[a-z0-9][a-z0-9\-]*[a-z0-9]$|^[a-z0-9]$`)
	if !validNameRegex.MatchString(name) {
		return fmt.Errorf("workspace name must consist of lowercase alphanumeric characters or '-', and must start and end with an alphanumeric character")
	}

	// Additional KCP-specific validation
	if strings.Contains(name, "..") {
		return fmt.Errorf("workspace name cannot contain consecutive dots")
	}

	return nil
}

// GetWorkspaceName is a utility method to extract workspace name from a request
func (r *Router) GetWorkspaceName(req *http.Request) (logicalcluster.Name, error) {
	return r.extractWorkspace(req.URL.Path)
}

// IsWorkspacePath checks if a given path represents a workspace-scoped request
func (r *Router) IsWorkspacePath(path string) bool {
	return r.workspacePathRegex.MatchString(path)
}

// StripWorkspacePrefix removes the workspace prefix from a path, returning the API path
func (r *Router) StripWorkspacePrefix(path string) (string, error) {
	workspace, err := r.extractWorkspace(path)
	if err != nil {
		return "", err
	}

	prefix := fmt.Sprintf("/clusters/%s", workspace)
	if !strings.HasPrefix(path, prefix) {
		return "", fmt.Errorf("path does not start with expected workspace prefix: %s", prefix)
	}

	return strings.TrimPrefix(path, prefix), nil
}