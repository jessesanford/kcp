package discovery

import (
	"context"
	"fmt"
	"sync"
	"time"
	
	authv1 "k8s.io/api/authorization/v1"
	kcpclient "github.com/kcp-dev/kcp/pkg/client/clientset/versioned"
	"github.com/kcp-dev/logicalcluster/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PermissionChecker checks access permissions for workspaces
type PermissionChecker struct {
	client kcpclient.Interface
	cache  *permissionCache
}

// permissionCache caches permission check results
type permissionCache struct {
	mu      sync.RWMutex
	entries map[string]*permissionEntry
}

type permissionEntry struct {
	allowed   bool
	timestamp time.Time
}

// NewPermissionChecker creates a new permission checker
func NewPermissionChecker(client kcpclient.Interface) *PermissionChecker {
	return &PermissionChecker{
		client: client,
		cache: &permissionCache{
			entries: make(map[string]*permissionEntry),
		},
	}
}

// CheckAccess checks if the current user can access a workspace
func (c *PermissionChecker) CheckAccess(ctx context.Context, workspace string, verb string) (bool, error) {
	cacheKey := fmt.Sprintf("%s:%s", workspace, verb)
	
	// Check cache
	if allowed, ok := c.cache.get(cacheKey); ok {
		return allowed, nil
	}
	
	// Perform SubjectAccessReview
	allowed, err := c.performAccessCheck(ctx, workspace, verb)
	if err != nil {
		return false, err
	}
	
	// Cache the result
	c.cache.put(cacheKey, allowed)
	
	return allowed, nil
}

// performAccessCheck performs the actual permission check
func (c *PermissionChecker) performAccessCheck(ctx context.Context, workspace string, verb string) (bool, error) {
	cluster := logicalcluster.NewPath(workspace)
	
	sar := &authv1.SubjectAccessReview{
		Spec: authv1.SubjectAccessReviewSpec{
			ResourceAttributes: &authv1.ResourceAttributes{
				Verb:     verb,
				Group:    "workload.kcp.io",
				Resource: "synctargets",
			},
		},
	}
	
	result, err := c.client.Cluster(cluster).
		AuthorizationV1().
		SubjectAccessReviews().
		Create(ctx, sar, metav1.CreateOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to check access: %w", err)
	}
	
	return result.Status.Allowed, nil
}

// CheckWorkspaceAccess checks access for workspace operations
func (c *PermissionChecker) CheckWorkspaceAccess(ctx context.Context, workspace string, verb string, resource string) (bool, error) {
	cacheKey := fmt.Sprintf("%s:%s:%s", workspace, verb, resource)
	
	// Check cache
	if allowed, ok := c.cache.get(cacheKey); ok {
		return allowed, nil
	}
	
	// Perform access check
	allowed, err := c.performWorkspaceAccessCheck(ctx, workspace, verb, resource)
	if err != nil {
		return false, err
	}
	
	// Cache the result
	c.cache.put(cacheKey, allowed)
	
	return allowed, nil
}

// performWorkspaceAccessCheck checks access for workspace-specific resources
func (c *PermissionChecker) performWorkspaceAccessCheck(ctx context.Context, workspace string, verb string, resource string) (bool, error) {
	cluster := logicalcluster.NewPath(workspace)
	
	sar := &authv1.SubjectAccessReview{
		Spec: authv1.SubjectAccessReviewSpec{
			ResourceAttributes: &authv1.ResourceAttributes{
				Verb:     verb,
				Group:    "tenancy.kcp.io",
				Resource: resource,
			},
		},
	}
	
	result, err := c.client.Cluster(cluster).
		AuthorizationV1().
		SubjectAccessReviews().
		Create(ctx, sar, metav1.CreateOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to check workspace access: %w", err)
	}
	
	return result.Status.Allowed, nil
}

// cache methods
func (c *permissionCache) get(key string) (bool, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	entry, ok := c.entries[key]
	if !ok {
		return false, false
	}
	
	// Check if cache entry is still valid (5 minutes TTL)
	if time.Since(entry.timestamp) > 5*time.Minute {
		return false, false
	}
	
	return entry.allowed, true
}

func (c *permissionCache) put(key string, allowed bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.entries[key] = &permissionEntry{
		allowed:   allowed,
		timestamp: time.Now(),
	}
}

// ClearCache clears all cached permissions
func (c *PermissionChecker) ClearCache() {
	c.cache.mu.Lock()
	defer c.cache.mu.Unlock()
	
	c.cache.entries = make(map[string]*permissionEntry)
}