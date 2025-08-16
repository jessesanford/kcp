package discovery

import (
	"sync"
	"time"
	
	"github.com/kcp-dev/kcp/pkg/placement/interfaces"
)

// DiscoveryCache caches discovery results
type DiscoveryCache struct {
	mu         sync.RWMutex
	workspaces map[string]*cacheEntry[[]interfaces.WorkspaceInfo]
	clusters   map[string]*cacheEntry[[]interfaces.ClusterTarget]
	ttl        time.Duration
}

// cacheEntry wraps cached data with timestamp
type cacheEntry[T any] struct {
	data      T
	timestamp time.Time
}

// NewDiscoveryCache creates a new discovery cache
func NewDiscoveryCache(ttl time.Duration) *DiscoveryCache {
	if ttl == 0 {
		ttl = 10 * time.Minute // default TTL
	}
	
	return &DiscoveryCache{
		workspaces: make(map[string]*cacheEntry[[]interfaces.WorkspaceInfo]),
		clusters:   make(map[string]*cacheEntry[[]interfaces.ClusterTarget]),
		ttl:        ttl,
	}
}

// GetWorkspaces retrieves cached workspaces
func (c *DiscoveryCache) GetWorkspaces(key string) ([]interfaces.WorkspaceInfo, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	entry, ok := c.workspaces[key]
	if !ok {
		return nil, false
	}
	
	if time.Since(entry.timestamp) > c.ttl {
		return nil, false
	}
	
	return entry.data, true
}

// PutWorkspaces caches workspace list
func (c *DiscoveryCache) PutWorkspaces(key string, workspaces []interfaces.WorkspaceInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.workspaces[key] = &cacheEntry[[]interfaces.WorkspaceInfo]{
		data:      workspaces,
		timestamp: time.Now(),
	}
}

// GetClusters retrieves cached clusters
func (c *DiscoveryCache) GetClusters(workspace string) ([]interfaces.ClusterTarget, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	entry, ok := c.clusters[workspace]
	if !ok {
		return nil, false
	}
	
	if time.Since(entry.timestamp) > c.ttl {
		return nil, false
	}
	
	return entry.data, true
}

// PutClusters caches cluster list
func (c *DiscoveryCache) PutClusters(workspace string, clusters []interfaces.ClusterTarget) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.clusters[workspace] = &cacheEntry[[]interfaces.ClusterTarget]{
		data:      clusters,
		timestamp: time.Now(),
	}
}

// Clear removes all cached entries
func (c *DiscoveryCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.workspaces = make(map[string]*cacheEntry[[]interfaces.WorkspaceInfo])
	c.clusters = make(map[string]*cacheEntry[[]interfaces.ClusterTarget])
}

// GetStats returns cache statistics
func (c *DiscoveryCache) GetStats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	return CacheStats{
		WorkspaceEntries: len(c.workspaces),
		ClusterEntries:   len(c.clusters),
		TTL:              c.ttl,
	}
}

// CacheStats provides cache statistics
type CacheStats struct {
	WorkspaceEntries int
	ClusterEntries   int
	TTL              time.Duration
}