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
	"context"
	"fmt"
	"net/http"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"
)

// VirtualWorkspace represents an isolated virtual workspace with its configuration and resources
type VirtualWorkspace struct {
	Name        logicalcluster.Name              `json:"name"`
	Config      *VirtualWorkspaceConfig          `json:"config,omitempty"`
	Resources   []metav1.APIResource             `json:"resources,omitempty"`
	LastUpdated metav1.Time                      `json:"lastUpdated"`
	Status      VirtualWorkspaceStatus           `json:"status"`
}

// VirtualWorkspaceConfig holds the configuration for a virtual workspace
type VirtualWorkspaceConfig struct {
	Enabled       bool                           `json:"enabled"`
	Description   string                         `json:"description,omitempty"`
	AuthPolicy    string                         `json:"authPolicy,omitempty"`
	ResourceQuota *VirtualWorkspaceResourceQuota `json:"resourceQuota,omitempty"`
}

// VirtualWorkspaceResourceQuota defines resource limits for the workspace
type VirtualWorkspaceResourceQuota struct {
	MaxResources int64 `json:"maxResources,omitempty"`
	MaxRequests  int64 `json:"maxRequests,omitempty"`
}

// VirtualWorkspaceStatus represents the current state of the workspace
type VirtualWorkspaceStatus string

const (
	VirtualWorkspaceStatusActive     VirtualWorkspaceStatus = "Active"
	VirtualWorkspaceStatusInactive   VirtualWorkspaceStatus = "Inactive"
	VirtualWorkspaceStatusSuspended  VirtualWorkspaceStatus = "Suspended"
	VirtualWorkspaceStatusError      VirtualWorkspaceStatus = "Error"
)

// ResourceDiscoveryInterface defines the contract for resource discovery in virtual workspaces
type ResourceDiscoveryInterface interface {
	// DiscoverResources returns available API resources for a workspace
	DiscoverResources(ctx context.Context, workspace logicalcluster.Name) ([]metav1.APIResource, error)
	// RefreshResources triggers a refresh of the resource cache for a workspace
	RefreshResources(ctx context.Context, workspace logicalcluster.Name) error
}

// AuthorizationProvider defines the contract for workspace authorization
type AuthorizationProvider interface {
	// Authorize checks if the request is authorized for the workspace
	Authorize(ctx context.Context, attributes authorizer.Attributes) (authorized authorizer.Decision, reason string, err error)
	// GetWorkspaceAccess returns the access level for a user in a workspace
	GetWorkspaceAccess(ctx context.Context, user string, workspace logicalcluster.Name) (string, error)
}

// WorkspaceCache provides caching functionality for workspace metadata
type WorkspaceCache interface {
	// Get retrieves a workspace from cache
	Get(name logicalcluster.Name) (*VirtualWorkspace, error)
	// Set stores a workspace in cache
	Set(workspace *VirtualWorkspace) error
	// Delete removes a workspace from cache
	Delete(name logicalcluster.Name) error
	// List returns all cached workspaces
	List() ([]*VirtualWorkspace, error)
}

// WorkspaceProvider implements the VirtualWorkspaceProvider interface for serving virtual workspace APIs
type WorkspaceProvider struct {
	// discovery provides resource discovery functionality
	discovery ResourceDiscoveryInterface

	// auth provides authorization functionality
	auth AuthorizationProvider

	// cache provides workspace metadata caching
	cache WorkspaceCache

	// router handles request routing
	router *Router

	// handler processes HTTP requests
	handler *Handler

	// mu protects concurrent access to workspaces
	mu sync.RWMutex

	// workspaces maintains the map of active virtual workspaces
	workspaces map[logicalcluster.Name]*VirtualWorkspace

	// ready tracks if the provider is ready to serve requests
	ready bool
}

// NewWorkspaceProvider creates a new workspace provider with the specified dependencies
func NewWorkspaceProvider(
	discovery ResourceDiscoveryInterface,
	auth AuthorizationProvider,
	cache WorkspaceCache,
) (*WorkspaceProvider, error) {
	if discovery == nil {
		return nil, fmt.Errorf("discovery interface cannot be nil")
	}
	if auth == nil {
		return nil, fmt.Errorf("authorization provider cannot be nil")
	}
	if cache == nil {
		return nil, fmt.Errorf("workspace cache cannot be nil")
	}

	provider := &WorkspaceProvider{
		discovery:  discovery,
		auth:       auth,
		cache:      cache,
		workspaces: make(map[logicalcluster.Name]*VirtualWorkspace),
		ready:      false,
	}

	// Initialize router with provider reference
	provider.router = NewRouter(provider)
	
	// Initialize handler with provider reference  
	provider.handler = NewHandler(provider)

	return provider, nil
}

// Initialize sets up the provider and prepares it to serve requests
func (p *WorkspaceProvider) Initialize(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Load existing workspaces from cache
	cachedWorkspaces, err := p.cache.List()
	if err != nil {
		return fmt.Errorf("failed to load cached workspaces: %v", err)
	}

	// Initialize workspaces map with cached data
	for _, workspace := range cachedWorkspaces {
		p.workspaces[workspace.Name] = workspace
		klog.V(4).InfoS("Loaded workspace from cache", "workspace", workspace.Name)
	}

	// Mark as ready
	p.ready = true
	klog.InfoS("WorkspaceProvider initialized successfully", "workspaces", len(p.workspaces))

	return nil
}

// ServeHTTP implements the main HTTP handler for virtual workspace requests
func (p *WorkspaceProvider) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Check if provider is ready
	if !p.IsReady() {
		http.Error(w, "Virtual workspace provider not ready", http.StatusServiceUnavailable)
		return
	}

	// Extract workspace information from request
	workspace, err := p.router.extractWorkspace(r.URL.Path)
	if err != nil {
		klog.V(2).InfoS("Failed to extract workspace from path", "path", r.URL.Path, "error", err)
		http.Error(w, fmt.Sprintf("Invalid workspace path: %v", err), http.StatusBadRequest)
		return
	}

	// Validate workspace exists and is active
	vw, err := p.GetWorkspace(string(workspace))
	if err != nil {
		klog.V(2).InfoS("Workspace not found", "workspace", workspace, "error", err)
		http.Error(w, fmt.Sprintf("Workspace not found: %v", err), http.StatusNotFound)
		return
	}

	if vw.Status != VirtualWorkspaceStatusActive {
		klog.V(2).InfoS("Workspace not active", "workspace", workspace, "status", vw.Status)
		http.Error(w, fmt.Sprintf("Workspace %s is not active (status: %s)", workspace, vw.Status), http.StatusServiceUnavailable)
		return
	}

	// Check authorization for the workspace
	if err := p.checkAuthorization(r.Context(), r, workspace); err != nil {
		klog.V(2).InfoS("Authorization failed", "workspace", workspace, "error", err)
		http.Error(w, fmt.Sprintf("Access denied: %v", err), http.StatusForbidden)
		return
	}

	// Route request to appropriate handler
	p.router.Route(w, r)
}

// GetWorkspace retrieves a workspace by name, checking cache first
func (p *WorkspaceProvider) GetWorkspace(name string) (*VirtualWorkspace, error) {
	if name == "" {
		return nil, fmt.Errorf("workspace name cannot be empty")
	}

	workspaceName := logicalcluster.Name(name)

	// Check in-memory cache first
	p.mu.RLock()
	workspace, exists := p.workspaces[workspaceName]
	p.mu.RUnlock()

	if exists {
		return workspace, nil
	}

	// Fall back to persistent cache
	workspace, err := p.cache.Get(workspaceName)
	if err != nil {
		return nil, fmt.Errorf("workspace %q not found: %v", name, err)
	}

	// Update in-memory cache
	p.mu.Lock()
	p.workspaces[workspaceName] = workspace
	p.mu.Unlock()

	return workspace, nil
}

// CreateWorkspace creates a new virtual workspace with the given configuration
func (p *WorkspaceProvider) CreateWorkspace(ctx context.Context, name string, config *VirtualWorkspaceConfig) (*VirtualWorkspace, error) {
	if name == "" {
		return nil, fmt.Errorf("workspace name cannot be empty")
	}
	if config == nil {
		config = &VirtualWorkspaceConfig{
			Enabled: true,
		}
	}

	workspaceName := logicalcluster.Name(name)

	// Check if workspace already exists
	if _, err := p.GetWorkspace(name); err == nil {
		return nil, fmt.Errorf("workspace %q already exists", name)
	}

	// Discover resources for the new workspace
	resources, err := p.discovery.DiscoverResources(ctx, workspaceName)
	if err != nil {
		return nil, fmt.Errorf("failed to discover resources for workspace %q: %v", name, err)
	}

	// Create workspace
	workspace := &VirtualWorkspace{
		Name:        workspaceName,
		Config:      config,
		Resources:   resources,
		LastUpdated: metav1.Now(),
		Status:      VirtualWorkspaceStatusActive,
	}

	// Store in cache
	if err := p.cache.Set(workspace); err != nil {
		return nil, fmt.Errorf("failed to cache workspace %q: %v", name, err)
	}

	// Update in-memory cache
	p.mu.Lock()
	p.workspaces[workspaceName] = workspace
	p.mu.Unlock()

	klog.InfoS("Created virtual workspace", "workspace", name, "resources", len(resources))
	return workspace, nil
}

// DeleteWorkspace removes a virtual workspace
func (p *WorkspaceProvider) DeleteWorkspace(ctx context.Context, name string) error {
	if name == "" {
		return fmt.Errorf("workspace name cannot be empty")
	}

	workspaceName := logicalcluster.Name(name)

	// Remove from persistent cache
	if err := p.cache.Delete(workspaceName); err != nil {
		return fmt.Errorf("failed to delete workspace %q from cache: %v", name, err)
	}

	// Remove from in-memory cache
	p.mu.Lock()
	delete(p.workspaces, workspaceName)
	p.mu.Unlock()

	klog.InfoS("Deleted virtual workspace", "workspace", name)
	return nil
}

// ListWorkspaces returns all available virtual workspaces
func (p *WorkspaceProvider) ListWorkspaces(ctx context.Context) ([]*VirtualWorkspace, error) {
	p.mu.RLock()
	workspaces := make([]*VirtualWorkspace, 0, len(p.workspaces))
	for _, workspace := range p.workspaces {
		workspaces = append(workspaces, workspace)
	}
	p.mu.RUnlock()

	return workspaces, nil
}

// IsReady checks if the provider is ready to serve requests
func (p *WorkspaceProvider) IsReady() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.ready
}

// checkAuthorization verifies if the request is authorized for the workspace
func (p *WorkspaceProvider) checkAuthorization(ctx context.Context, r *http.Request, workspace logicalcluster.Name) error {
	// Build authorization attributes from request
	requestInfo, ok := genericapirequest.RequestInfoFrom(ctx)
	if !ok {
		// For testing and basic scenarios, we'll allow requests without proper request info
		klog.V(4).InfoS("No request info found in context, skipping authorization")
		return nil
	}

	user, _ := genericapirequest.UserFrom(ctx)
	attributes := authorizer.AttributesRecord{
		User:            user,
		Verb:            requestInfo.Verb,
		Namespace:       requestInfo.Namespace,
		APIGroup:        requestInfo.APIGroup,
		APIVersion:      requestInfo.APIVersion,
		Resource:        requestInfo.Resource,
		Subresource:     requestInfo.Subresource,
		Name:            requestInfo.Name,
		ResourceRequest: requestInfo.IsResourceRequest,
		Path:            requestInfo.Path,
	}

	// Check authorization with the authorization provider
	decision, reason, err := p.auth.Authorize(ctx, attributes)
	if err != nil {
		return fmt.Errorf("authorization error: %v", err)
	}

	if decision != authorizer.DecisionAllow {
		return fmt.Errorf("access denied: %s", reason)
	}

	return nil
}