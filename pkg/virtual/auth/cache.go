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
	"fmt"
	"strings"
	"sync"
	"time"
)

// PermissionCache provides caching for authorization decisions to improve performance.
// It implements TTL-based expiration and workspace-specific invalidation
// to ensure consistency with RBAC changes.
type PermissionCache struct {
	mu    sync.RWMutex
	cache map[string]*CacheEntry
	ttl   time.Duration
}

// CacheEntry represents a cached authorization decision with expiration.
type CacheEntry struct {
	Decision  *Decision
	ExpiresAt time.Time
}

// NewPermissionCache creates a new permission cache with the specified TTL.
// The TTL determines how long cached decisions remain valid.
func NewPermissionCache(ttlSeconds int64) *PermissionCache {
	return &PermissionCache{
		cache: make(map[string]*CacheEntry),
		ttl:   time.Duration(ttlSeconds) * time.Second,
	}
}

// Get retrieves a cached authorization decision for the given request.
// Returns the decision and true if found and not expired, nil and false otherwise.
func (c *PermissionCache) Get(req *Request) (*Decision, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := c.requestKey(req)
	entry, ok := c.cache[key]
	if !ok {
		return nil, false
	}

	// Check if the entry has expired
	if time.Now().After(entry.ExpiresAt) {
		return nil, false
	}

	// Return a copy of the decision to avoid mutation
	decision := &Decision{
		Allowed:          entry.Decision.Allowed,
		Reason:           entry.Decision.Reason,
		EvaluationError:  entry.Decision.EvaluationError,
		AuditAnnotations: make(map[string]string),
	}

	// Copy audit annotations
	for k, v := range entry.Decision.AuditAnnotations {
		decision.AuditAnnotations[k] = v
	}
	decision.AuditAnnotations["cache.virtual.io/hit"] = "true"

	return decision, true
}

// Set caches an authorization decision for the given request.
// The decision will expire after the configured TTL.
func (c *PermissionCache) Set(req *Request, decision *Decision) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := c.requestKey(req)

	// Create a copy of the decision to avoid external mutation
	cachedDecision := &Decision{
		Allowed:          decision.Allowed,
		Reason:           decision.Reason,
		EvaluationError:  decision.EvaluationError,
		AuditAnnotations: make(map[string]string),
	}

	// Copy audit annotations
	for k, v := range decision.AuditAnnotations {
		cachedDecision.AuditAnnotations[k] = v
	}
	cachedDecision.AuditAnnotations["cache.virtual.io/cached"] = time.Now().Format(time.RFC3339)

	c.cache[key] = &CacheEntry{
		Decision:  cachedDecision,
		ExpiresAt: time.Now().Add(c.ttl),
	}
}

// InvalidateWorkspace removes all cached decisions for the specified workspace.
// This should be called when RBAC rules change in a workspace to ensure consistency.
func (c *PermissionCache) InvalidateWorkspace(workspace string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Remove all entries that belong to the specified workspace
	for key := range c.cache {
		if c.keyBelongsToWorkspace(key, workspace) {
			delete(c.cache, key)
		}
	}
}

// InvalidateUser removes all cached decisions for the specified user.
// This should be called when a user's permissions change.
func (c *PermissionCache) InvalidateUser(user string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Remove all entries that belong to the specified user
	for key := range c.cache {
		if c.keyBelongsToUser(key, user) {
			delete(c.cache, key)
		}
	}
}

// Clear removes all cached decisions.
// This is useful during shutdown or when a complete cache invalidation is needed.
func (c *PermissionCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache = make(map[string]*CacheEntry)
}

// CleanupExpired removes all expired entries from the cache.
// This should be called periodically to prevent memory leaks.
func (c *PermissionCache) CleanupExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, entry := range c.cache {
		if now.After(entry.ExpiresAt) {
			delete(c.cache, key)
		}
	}
}

// Size returns the current number of cached entries.
func (c *PermissionCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.cache)
}

// Stats returns cache statistics for monitoring and debugging.
func (c *PermissionCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := CacheStats{
		Size: len(c.cache),
		TTL:  c.ttl,
	}

	now := time.Now()
	for _, entry := range c.cache {
		if now.After(entry.ExpiresAt) {
			stats.ExpiredEntries++
		}
	}

	return stats
}

// CacheStats contains statistics about the cache state.
type CacheStats struct {
	Size           int
	ExpiredEntries int
	TTL            time.Duration
}

// requestKey generates a unique cache key for an authorization request.
// The key includes all relevant request parameters to ensure cache correctness.
func (c *PermissionCache) requestKey(req *Request) string {
	// Format: workspace:user:groups:resource:resourceName:verb
	groupsStr := strings.Join(req.Groups, ",")
	return fmt.Sprintf("%s:%s:%s:%s:%s:%s",
		req.Workspace,
		req.User,
		groupsStr,
		req.Resource.String(),
		req.ResourceName,
		req.Verb,
	)
}

// keyBelongsToWorkspace checks if a cache key belongs to the specified workspace.
func (c *PermissionCache) keyBelongsToWorkspace(key, workspace string) bool {
	// Key format: workspace:user:groups:resource:resourceName:verb
	parts := strings.SplitN(key, ":", 2)
	return len(parts) >= 1 && parts[0] == workspace
}

// keyBelongsToUser checks if a cache key belongs to the specified user.
func (c *PermissionCache) keyBelongsToUser(key, user string) bool {
	// Key format: workspace:user:groups:resource:resourceName:verb
	parts := strings.SplitN(key, ":", 3)
	return len(parts) >= 2 && parts[1] == user
}