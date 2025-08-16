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
	"sync"
	"time"

	"github.com/kcp-dev/kcp/pkg/virtual/interfaces"
	"github.com/kcp-dev/logicalcluster/v3"
)

// DiscoveryCache is a TTL-based cache implementation for discovered resources.
// It provides workspace-isolated caching with automatic expiration and cleanup.
type DiscoveryCache struct {
	// entries stores cached data with expiration times
	entries map[string]*cacheEntry
	
	// mutex protects concurrent access to the cache
	mutex sync.RWMutex
	
	// defaultTTL is the default cache entry TTL in seconds
	defaultTTL time.Duration
	
	// cleanupInterval controls how often expired entries are cleaned up
	cleanupInterval time.Duration
	
	// stopCh signals the cleanup goroutine to stop
	stopCh chan struct{}
	
	// cleanupOnce ensures cleanup goroutine starts only once
	cleanupOnce sync.Once
}

// cacheEntry represents a single cache entry with data and expiration time
type cacheEntry struct {
	// resources contains the cached resource information
	resources []interfaces.ResourceInfo
	
	// expiresAt indicates when this entry expires
	expiresAt time.Time
	
	// lastAccessed tracks when this entry was last accessed
	lastAccessed time.Time
}

// NewDiscoveryCache creates a new discovery cache with the specified default TTL.
// If defaultTTLSeconds is 0, a default of 5 minutes is used.
func NewDiscoveryCache(defaultTTLSeconds int64) interfaces.DiscoveryCache {
	defaultTTL := 5 * time.Minute
	if defaultTTLSeconds > 0 {
		defaultTTL = time.Duration(defaultTTLSeconds) * time.Second
	}
	
	cache := &DiscoveryCache{
		entries:         make(map[string]*cacheEntry),
		defaultTTL:      defaultTTL,
		cleanupInterval: time.Minute, // Run cleanup every minute
		stopCh:          make(chan struct{}),
	}
	
	// Start cleanup goroutine
	cache.startCleanup()
	
	return cache
}

// GetResources retrieves cached resources for a workspace.
// Returns the cached resources and true if found and not expired, otherwise nil and false.
func (c *DiscoveryCache) GetResources(workspace logicalcluster.Name) ([]interfaces.ResourceInfo, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	
	key := workspace.String()
	entry, exists := c.entries[key]
	if !exists {
		RecordCacheHit(key, false)
		return nil, false
	}
	
	// Check if entry has expired
	now := time.Now()
	if now.After(entry.expiresAt) {
		RecordCacheHit(key, false)
		// Don't clean up here to avoid lock upgrade, let cleanup goroutine handle it
		return nil, false
	}
	
	// Update access time and return cached data
	entry.lastAccessed = now
	RecordCacheHit(key, true)
	
	// Return a copy to prevent external mutation
	result := make([]interfaces.ResourceInfo, len(entry.resources))
	copy(result, entry.resources)
	return result, true
}

// SetResources caches resources for a workspace with the specified TTL.
// If ttl is 0, the default TTL is used.
func (c *DiscoveryCache) SetResources(workspace logicalcluster.Name, resources []interfaces.ResourceInfo, ttl int64) {
	if len(resources) == 0 {
		// Don't cache empty results
		return
	}
	
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	cacheTTL := c.defaultTTL
	if ttl > 0 {
		cacheTTL = time.Duration(ttl) * time.Second
	}
	
	now := time.Now()
	key := workspace.String()
	
	// Create a copy to prevent external mutation
	cachedResources := make([]interfaces.ResourceInfo, len(resources))
	copy(cachedResources, resources)
	
	c.entries[key] = &cacheEntry{
		resources:    cachedResources,
		expiresAt:    now.Add(cacheTTL),
		lastAccessed: now,
	}
}

// InvalidateWorkspace removes cached data for a workspace.
func (c *DiscoveryCache) InvalidateWorkspace(workspace logicalcluster.Name) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	delete(c.entries, workspace.String())
}

// Clear removes all cached data.
func (c *DiscoveryCache) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	c.entries = make(map[string]*cacheEntry)
}

// Stop stops the cleanup goroutine and releases resources.
func (c *DiscoveryCache) Stop() {
	close(c.stopCh)
}

// startCleanup starts the background cleanup goroutine that removes expired entries.
func (c *DiscoveryCache) startCleanup() {
	c.cleanupOnce.Do(func() {
		go c.cleanupExpiredEntries()
	})
}

// cleanupExpiredEntries runs periodically to remove expired cache entries.
func (c *DiscoveryCache) cleanupExpiredEntries() {
	ticker := time.NewTicker(c.cleanupInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			c.performCleanup()
		case <-c.stopCh:
			return
		}
	}
}

// performCleanup removes expired entries from the cache.
func (c *DiscoveryCache) performCleanup() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	now := time.Now()
	for key, entry := range c.entries {
		if now.After(entry.expiresAt) {
			delete(c.entries, key)
		}
	}
}