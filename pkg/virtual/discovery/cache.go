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
	"sync"
	"time"

	"k8s.io/klog/v2"

	"github.com/kcp-dev/kcp/pkg/virtual/interfaces"
	"github.com/kcp-dev/kcp/pkg/virtual/contracts"
)

// MemoryDiscoveryCache provides in-memory caching for discovered resources
type MemoryDiscoveryCache struct {
	// entries stores cached discovery data per workspace
	entries map[string]*cacheEntry

	// defaultTTL is the default cache expiration time
	defaultTTL time.Duration

	// mutex protects concurrent access
	mutex sync.RWMutex

	// cleanupInterval determines how often to run cache cleanup
	cleanupInterval time.Duration

	// stopCh signals shutdown for cleanup goroutine
	stopCh chan struct{}
}

// cacheEntry represents a cached discovery result
type cacheEntry struct {
	// resources are the cached resource information
	resources []interfaces.ResourceInfo

	// timestamp when this entry was created
	timestamp time.Time

	// ttl is the time-to-live for this entry
	ttl time.Duration
}

// NewMemoryDiscoveryCache creates a new memory-based discovery cache
func NewMemoryDiscoveryCache(defaultTTL, cleanupInterval time.Duration) *MemoryDiscoveryCache {
	return &MemoryDiscoveryCache{
		entries:         make(map[string]*cacheEntry),
		defaultTTL:      defaultTTL,
		cleanupInterval: cleanupInterval,
		stopCh:          make(chan struct{}),
	}
}

// Start begins cache cleanup operations
func (c *MemoryDiscoveryCache) Start() {
	go c.cleanupLoop()
	klog.V(4).InfoS("Discovery cache cleanup started", "interval", c.cleanupInterval)
}

// Stop terminates cache cleanup operations
func (c *MemoryDiscoveryCache) Stop() {
	close(c.stopCh)
	klog.V(4).InfoS("Discovery cache cleanup stopped")
}

// GetResources retrieves cached resources for a workspace
func (c *MemoryDiscoveryCache) GetResources(workspace string) ([]interfaces.ResourceInfo, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	entry, exists := c.entries[workspace]
	if !exists {
		return nil, false
	}

	// Check if entry has expired
	if time.Since(entry.timestamp) > entry.ttl {
		// Entry expired, remove it
		c.mutex.RUnlock()
		c.mutex.Lock()
		delete(c.entries, workspace)
		c.mutex.Unlock()
		c.mutex.RLock()
		return nil, false
	}

	// Return a copy to prevent external modifications
	resources := make([]interfaces.ResourceInfo, len(entry.resources))
	copy(resources, entry.resources)

	return resources, true
}

// SetResources caches resources for a workspace
func (c *MemoryDiscoveryCache) SetResources(workspace string, resources []interfaces.ResourceInfo, ttl int64) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Respect maximum cached workspaces
	if len(c.entries) >= contracts.MaxCachedWorkspaces && c.entries[workspace] == nil {
		// Remove oldest entry if we're at capacity
		c.removeOldestEntry()
	}

	cacheTTL := c.defaultTTL
	if ttl > 0 {
		cacheTTL = time.Duration(ttl) * time.Second
	}

	// Store a copy to prevent external modifications
	cachedResources := make([]interfaces.ResourceInfo, len(resources))
	copy(cachedResources, resources)

	c.entries[workspace] = &cacheEntry{
		resources: cachedResources,
		timestamp: time.Now(),
		ttl:       cacheTTL,
	}

	klog.V(5).InfoS("Cached discovery resources", "workspace", workspace, "count", len(resources), "ttl", cacheTTL)
}

// InvalidateWorkspace removes cached data for a workspace
func (c *MemoryDiscoveryCache) InvalidateWorkspace(workspace string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if _, exists := c.entries[workspace]; exists {
		delete(c.entries, workspace)
		klog.V(4).InfoS("Invalidated discovery cache for workspace", "workspace", workspace)
	}
}

// Clear removes all cached data
func (c *MemoryDiscoveryCache) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.entries = make(map[string]*cacheEntry)
	klog.V(4).InfoS("Cleared all discovery cache entries")
}

// cleanupLoop runs periodic cache cleanup
func (c *MemoryDiscoveryCache) cleanupLoop() {
	ticker := time.NewTicker(c.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.cleanupExpiredEntries()
		case <-c.stopCh:
			return
		}
	}
}

// cleanupExpiredEntries removes expired cache entries
func (c *MemoryDiscoveryCache) cleanupExpiredEntries() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	now := time.Now()
	expiredWorkspaces := []string{}

	for workspace, entry := range c.entries {
		if now.Sub(entry.timestamp) > entry.ttl {
			expiredWorkspaces = append(expiredWorkspaces, workspace)
		}
	}

	for _, workspace := range expiredWorkspaces {
		delete(c.entries, workspace)
	}

	if len(expiredWorkspaces) > 0 {
		klog.V(4).InfoS("Cleaned up expired cache entries", "count", len(expiredWorkspaces))
	}
}

// removeOldestEntry removes the oldest cache entry to make room for new ones
func (c *MemoryDiscoveryCache) removeOldestEntry() {
	if len(c.entries) == 0 {
		return
	}

	var oldestWorkspace string
	var oldestTime time.Time

	for workspace, entry := range c.entries {
		if oldestWorkspace == "" || entry.timestamp.Before(oldestTime) {
			oldestWorkspace = workspace
			oldestTime = entry.timestamp
		}
	}

	if oldestWorkspace != "" {
		delete(c.entries, oldestWorkspace)
		klog.V(4).InfoS("Removed oldest cache entry to make room", "workspace", oldestWorkspace)
	}
}