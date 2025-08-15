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
	"sync"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// BasicProvider implements basic RBAC authorization for virtual workspaces.
// It provides a simple implementation that can be used for testing and
// development, with support for caching and audit logging.
type BasicProvider struct {
	mu          sync.RWMutex
	name        string
	config      ProviderConfig
	kubeClient  kubernetes.Interface
	permissions map[string][]Permission // workspace:user -> permissions
	cache       *PermissionCache
}

// NewBasicProvider creates a new basic authorization provider.
// The provider starts with default permissions for admin and viewer roles.
func NewBasicProvider(name string) *BasicProvider {
	return &BasicProvider{
		name:        name,
		permissions: make(map[string][]Permission),
	}
}

// Name returns the provider name for identification.
func (p *BasicProvider) Name() string {
	return p.name
}

// Initialize sets up the basic provider with the given configuration.
// It establishes Kubernetes client connection and initializes caching if enabled.
func (p *BasicProvider) Initialize(ctx context.Context, config ProviderConfig) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.config = config

	// Initialize Kubernetes client if needed
	if config.KubeConfig != "" {
		restConfig, err := rest.InClusterConfig()
		if err != nil {
			return fmt.Errorf("failed to get in-cluster config: %w", err)
		}

		client, err := kubernetes.NewForConfig(restConfig)
		if err != nil {
			return fmt.Errorf("failed to create kubernetes client: %w", err)
		}

		p.kubeClient = client
	}

	// Initialize cache if enabled
	if config.CacheEnabled {
		p.cache = NewPermissionCache(config.CacheTTL)
	}

	// Load initial permissions
	p.loadDefaultPermissions()

	return nil
}

// Authorize performs an authorization check for the given request.
// It first checks the cache if enabled, then performs the authorization
// logic and caches the result.
func (p *BasicProvider) Authorize(ctx context.Context, req *Request) (*Decision, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Check cache first
	if p.cache != nil {
		if decision, ok := p.cache.Get(req); ok {
			return decision, nil
		}
	}

	// Perform authorization check
	decision := p.performAuthorization(req)

	// Cache the decision
	if p.cache != nil {
		p.cache.Set(req, decision)
	}

	// Audit log if enabled
	if p.config.AuditEnabled {
		p.auditLog(req, decision)
	}

	return decision, nil
}

// GetPermissions returns all permissions for a user in the specified workspace.
// This is used for UI display and permission introspection.
func (p *BasicProvider) GetPermissions(ctx context.Context, workspace, user string) ([]Permission, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	key := fmt.Sprintf("%s:%s", workspace, user)
	permissions, ok := p.permissions[key]
	if !ok {
		return []Permission{}, nil
	}

	return permissions, nil
}

// RefreshCache updates cached authorization data for the specified workspace.
// This should be called when RBAC rules change or permissions are updated.
func (p *BasicProvider) RefreshCache(ctx context.Context, workspace string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cache != nil {
		p.cache.InvalidateWorkspace(workspace)
	}

	// In a real implementation, this would reload permissions from Kubernetes RBAC
	// For now, we just clear the cache to force re-evaluation

	return nil
}

// Close cleans up provider resources and closes connections.
// Should be called during graceful shutdown.
func (p *BasicProvider) Close(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cache != nil {
		p.cache.Clear()
	}

	return nil
}

// performAuthorization performs the actual authorization logic.
// It checks if the user has any permissions that match the request.
func (p *BasicProvider) performAuthorization(req *Request) *Decision {
	key := fmt.Sprintf("%s:%s", req.Workspace, req.User)
	permissions, ok := p.permissions[key]
	if !ok {
		return &Decision{
			Allowed: false,
			Reason:  "No permissions found for user in workspace",
			AuditAnnotations: map[string]string{
				"authorization.virtual.io/decision": "deny",
				"authorization.virtual.io/reason":   "no-permissions",
			},
		}
	}

	// Check if any permission matches the request
	for _, perm := range permissions {
		if p.matchesPermission(req, perm) {
			return &Decision{
				Allowed: true,
				Reason:  "Permission granted by RBAC",
				AuditAnnotations: map[string]string{
					"authorization.virtual.io/decision": "allow",
					"authorization.virtual.io/reason":   "rbac-match",
				},
			}
		}
	}

	return &Decision{
		Allowed: false,
		Reason:  "No matching permission found",
		AuditAnnotations: map[string]string{
			"authorization.virtual.io/decision": "deny",
			"authorization.virtual.io/reason":   "no-matching-rule",
		},
	}
}

// matchesPermission checks if a permission matches the authorization request.
// It compares resource, verb, and resource name if specified.
func (p *BasicProvider) matchesPermission(req *Request, perm Permission) bool {
	// Check resource match (support wildcards)
	if perm.Resource.Group != "*" && perm.Resource.Group != req.Resource.Group {
		return false
	}
	if perm.Resource.Version != "*" && perm.Resource.Version != req.Resource.Version {
		return false
	}
	if perm.Resource.Resource != "*" && perm.Resource.Resource != req.Resource.Resource {
		return false
	}

	// Check verb match
	verbMatches := false
	for _, verb := range perm.Verbs {
		if verb == req.Verb || verb == "*" {
			verbMatches = true
			break
		}
	}

	if !verbMatches {
		return false
	}

	// Check resource name if specified in the request
	if req.ResourceName != "" && len(perm.ResourceNames) > 0 {
		nameMatches := false
		for _, name := range perm.ResourceNames {
			if name == req.ResourceName || name == "*" {
				nameMatches = true
				break
			}
		}
		return nameMatches
	}

	return true
}

// loadDefaultPermissions loads default permissions for admin and viewer roles.
// This is used for testing and development scenarios.
func (p *BasicProvider) loadDefaultPermissions() {
	// Add default admin permissions (full access)
	p.permissions["default:admin"] = []Permission{
		{
			Resource: schema.GroupVersionResource{Group: "*", Version: "*", Resource: "*"},
			Verbs:    []string{"*"},
		},
	}

	// Add default viewer permissions (read-only)
	p.permissions["default:viewer"] = []Permission{
		{
			Resource: schema.GroupVersionResource{Group: "*", Version: "*", Resource: "*"},
			Verbs:    []string{"get", "list", "watch"},
		},
	}

	// Add default editor permissions (no delete)
	p.permissions["default:editor"] = []Permission{
		{
			Resource: schema.GroupVersionResource{Group: "*", Version: "*", Resource: "*"},
			Verbs:    []string{"get", "list", "watch", "create", "update", "patch"},
		},
	}
}

// auditLog logs authorization decisions for security monitoring.
// In production, this would integrate with a proper audit logging system.
func (p *BasicProvider) auditLog(req *Request, decision *Decision) {
	// In a real implementation, this would write to a structured audit log
	// For now, we keep this as a placeholder for the interface
}