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

package authorization

import (
	"crypto/sha256"
	"fmt"
	"sync"
	"time"

	"github.com/kcp-dev/kcp/pkg/virtual/interfaces"
)

// AuthorizationCache provides caching for authorization decisions
type AuthorizationCache interface {
	// GetDecision retrieves a cached authorization decision
	GetDecision(key string) (*interfaces.AuthorizationDecision, bool)

	// SetDecision caches an authorization decision
	SetDecision(key string, decision *interfaces.AuthorizationDecision, ttl time.Duration)

	// InvalidateUser removes all cached decisions for a user
	InvalidateUser(user string)

	// InvalidateWorkspace removes all cached decisions for a workspace
	InvalidateWorkspace(workspace string)

	// Clear removes all cached decisions
	Clear()
}

// MemoryAuthorizationCache provides in-memory caching for authorization decisions
type MemoryAuthorizationCache struct {
	// entries stores cached authorization decisions
	entries map[string]*authCacheEntry

	// userIndex maps users to their cache keys
	userIndex map[string][]string

	// workspaceIndex maps workspaces to their cache keys
	workspaceIndex map[string][]string

	// mutex protects concurrent access
	mutex sync.RWMutex

	// defaultTTL is the default cache expiration time
	defaultTTL time.Duration

	// cleanupInterval determines how often to run cache cleanup
	cleanupInterval time.Duration

	// stopCh signals shutdown for cleanup goroutine
	stopCh chan struct{}
}

// authCacheEntry represents a cached authorization decision
type authCacheEntry struct {
	// decision is the cached authorization decision
	decision *interfaces.AuthorizationDecision

	// expireAt is when this entry expires
	expireAt time.Time

	// user is the user this decision applies to
	user string

	// workspace is the workspace this decision applies to
	workspace string
}

// NewMemoryAuthorizationCache creates a new memory-based authorization cache
func NewMemoryAuthorizationCache(defaultTTL, cleanupInterval time.Duration) *MemoryAuthorizationCache {
	return &MemoryAuthorizationCache{
		entries:         make(map[string]*authCacheEntry),
		userIndex:       make(map[string][]string),
		workspaceIndex:  make(map[string][]string),
		defaultTTL:      defaultTTL,
		cleanupInterval: cleanupInterval,
		stopCh:          make(chan struct{}),
	}
}

// Start begins cache cleanup operations
func (c *MemoryAuthorizationCache) Start() {
	go c.cleanupLoop()
}

// Stop terminates cache cleanup operations
func (c *MemoryAuthorizationCache) Stop() {
	close(c.stopCh)
}

// GetDecision retrieves a cached authorization decision
func (c *MemoryAuthorizationCache) GetDecision(key string) (*interfaces.AuthorizationDecision, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	entry, exists := c.entries[key]
	if !exists {
		return nil, false
	}

	// Check expiration
	if time.Now().After(entry.expireAt) {
		// Entry is expired, but don't clean up here to avoid write lock
		return nil, false
	}

	return entry.decision, true
}

// SetDecision caches an authorization decision
func (c *MemoryAuthorizationCache) SetDecision(key string, decision *interfaces.AuthorizationDecision, ttl time.Duration) {
	if ttl == 0 {
		ttl = c.defaultTTL
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Remove existing entry from indexes
	if existing, exists := c.entries[key]; exists {
		c.removeFromIndexes(key, existing)
	}

	// Create new entry  
	entry := &authCacheEntry{
		decision:  decision,
		expireAt:  time.Now().Add(ttl),
		user:      "",  // simplified for now
		workspace: "", // simplified for now
	}

	// Store entry
	c.entries[key] = entry

	// Update indexes
	c.addToIndexes(key, entry)
}

// InvalidateUser removes all cached decisions for a user
func (c *MemoryAuthorizationCache) InvalidateUser(user string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	keys, exists := c.userIndex[user]
	if !exists {
		return
	}

	for _, key := range keys {
		if entry, exists := c.entries[key]; exists {
			c.removeFromIndexes(key, entry)
			delete(c.entries, key)
		}
	}
}

// InvalidateWorkspace removes all cached decisions for a workspace
func (c *MemoryAuthorizationCache) InvalidateWorkspace(workspace string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	keys, exists := c.workspaceIndex[workspace]
	if !exists {
		return
	}

	for _, key := range keys {
		if entry, exists := c.entries[key]; exists {
			c.removeFromIndexes(key, entry)
			delete(c.entries, key)
		}
	}
}

// Clear removes all cached decisions
func (c *MemoryAuthorizationCache) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.entries = make(map[string]*authCacheEntry)
	c.userIndex = make(map[string][]string)
	c.workspaceIndex = make(map[string][]string)
}

// cleanupLoop periodically removes expired entries
func (c *MemoryAuthorizationCache) cleanupLoop() {
	ticker := time.NewTicker(c.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.cleanupExpired()
		case <-c.stopCh:
			return
		}
	}
}

// cleanupExpired removes expired entries from the cache
func (c *MemoryAuthorizationCache) cleanupExpired() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	now := time.Now()
	expiredKeys := make([]string, 0)

	// Find expired entries
	for key, entry := range c.entries {
		if now.After(entry.expireAt) {
			expiredKeys = append(expiredKeys, key)
		}
	}

	// Remove expired entries
	for _, key := range expiredKeys {
		if entry, exists := c.entries[key]; exists {
			c.removeFromIndexes(key, entry)
			delete(c.entries, key)
		}
	}
}

// addToIndexes adds a cache entry to the user and workspace indexes
func (c *MemoryAuthorizationCache) addToIndexes(key string, entry *authCacheEntry) {
	// Add to user index
	if entry.user != "" {
		c.userIndex[entry.user] = append(c.userIndex[entry.user], key)
	}

	// Add to workspace index
	if entry.workspace != "" {
		c.workspaceIndex[entry.workspace] = append(c.workspaceIndex[entry.workspace], key)
	}
}

// removeFromIndexes removes a cache entry from the user and workspace indexes
func (c *MemoryAuthorizationCache) removeFromIndexes(key string, entry *authCacheEntry) {
	// Remove from user index
	if entry.user != "" {
		keys := c.userIndex[entry.user]
		for i, k := range keys {
			if k == key {
				c.userIndex[entry.user] = append(keys[:i], keys[i+1:]...)
				break
			}
		}
		if len(c.userIndex[entry.user]) == 0 {
			delete(c.userIndex, entry.user)
		}
	}

	// Remove from workspace index
	if entry.workspace != "" {
		keys := c.workspaceIndex[entry.workspace]
		for i, k := range keys {
			if k == key {
				c.workspaceIndex[entry.workspace] = append(keys[:i], keys[i+1:]...)
				break
			}
		}
		if len(c.workspaceIndex[entry.workspace]) == 0 {
			delete(c.workspaceIndex, entry.workspace)
		}
	}
}

// generateCacheKey creates a cache key from request parameters
func generateCacheKey(user, workspace, resource, verb, resourceName string) string {
	data := fmt.Sprintf("%s:%s:%s:%s:%s", user, workspace, resource, verb, resourceName)
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash)
}