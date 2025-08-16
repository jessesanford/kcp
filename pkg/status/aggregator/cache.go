/*
Copyright The KCP Authors.

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

// Package aggregator implements TTL-based caching for aggregated status.
package aggregator

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sync"
	"time"

	"k8s.io/klog/v2"

	"github.com/kcp-dev/kcp/pkg/status/interfaces"
)

// Cache implements the StatusCache interface with TTL-based expiration
type Cache struct {
	// mu protects access to cache entries
	mu sync.RWMutex

	// entries stores cached status entries
	entries map[interfaces.CacheKey]*cacheEntry

	// defaultTTL is used when no TTL is specified
	defaultTTL time.Duration

	// cleanupInterval defines how often to run cleanup
	cleanupInterval time.Duration

	// stats tracks cache performance metrics
	stats interfaces.CacheStats

	// stopCh signals cleanup goroutine to stop
	stopCh chan struct{}

	// stopped indicates if cache has been stopped
	stopped bool
}

// cacheEntry represents a single cached status entry
type cacheEntry struct {
	// status is the cached aggregated status
	status *interfaces.AggregatedStatus

	// expiresAt is when this entry expires
	expiresAt time.Time

	// lastAccessed tracks when this entry was last accessed
	lastAccessed time.Time
}

// CacheConfig contains configuration for the status cache
type CacheConfig struct {
	// DefaultTTL is used when no TTL is specified for cache entries
	DefaultTTL time.Duration

	// CleanupInterval defines how often to run cleanup of expired entries
	CleanupInterval time.Duration

	// MaxSize limits the number of cached entries (0 = unlimited)
	MaxSize int
}

// NewCache creates a new status cache instance
func NewCache(config CacheConfig) *Cache {
	if config.DefaultTTL == 0 {
		config.DefaultTTL = 5 * time.Minute
	}
	if config.CleanupInterval == 0 {
		config.CleanupInterval = time.Minute
	}

	cache := &Cache{
		entries:         make(map[interfaces.CacheKey]*cacheEntry),
		defaultTTL:      config.DefaultTTL,
		cleanupInterval: config.CleanupInterval,
		stopCh:          make(chan struct{}),
	}

	// Start cleanup goroutine
	go cache.cleanupLoop()

	return cache
}

// Get retrieves cached status if available and not expired
func (c *Cache) Get(ctx context.Context, key interfaces.CacheKey) (*interfaces.AggregatedStatus, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, exists := c.entries[key]
	if !exists {
		c.stats.Misses++
		c.updateHitRatio()
		return nil, false
	}

	// Check if entry has expired
	if time.Now().After(entry.expiresAt) {
		delete(c.entries, key)
		c.stats.Misses++
		c.stats.Evictions++
		c.updateHitRatio()
		return nil, false
	}

	// Update access time
	entry.lastAccessed = time.Now()
	c.stats.Hits++
	c.updateHitRatio()

	klog.V(5).InfoS("Cache hit", "key", c.keyString(key))
	return deepCopyAggregatedStatus(entry.status), true
}

// Set stores aggregated status with TTL
func (c *Cache) Set(ctx context.Context, key interfaces.CacheKey, status *interfaces.AggregatedStatus, ttl time.Duration) {
	if ttl == 0 {
		ttl = c.defaultTTL
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stopped {
		return
	}

	now := time.Now()
	entry := &cacheEntry{
		status:       deepCopyAggregatedStatus(status),
		expiresAt:    now.Add(ttl),
		lastAccessed: now,
	}

	// Check if this is a new entry
	if _, exists := c.entries[key]; !exists {
		c.stats.Size++
	}

	c.entries[key] = entry

	klog.V(5).InfoS("Cache set", 
		"key", c.keyString(key), 
		"ttl", ttl.String(),
		"expiresAt", entry.expiresAt.Format(time.RFC3339))
}

// Delete removes cached status
func (c *Cache) Delete(ctx context.Context, key interfaces.CacheKey) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.entries[key]; exists {
		delete(c.entries, key)
		c.stats.Size--
		klog.V(5).InfoS("Cache delete", "key", c.keyString(key))
	}
}

// Clear removes all cached entries
func (c *Cache) Clear(ctx context.Context) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entryCount := len(c.entries)
	c.entries = make(map[interfaces.CacheKey]*cacheEntry)
	c.stats.Size = 0
	c.stats.Evictions += int64(entryCount)

	klog.V(3).InfoS("Cache cleared", "evictedEntries", entryCount)
}

// Stats returns cache statistics
func (c *Cache) Stats() interfaces.CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.stats
}

// Stop stops the cache cleanup goroutine
func (c *Cache) Stop() {
	c.mu.Lock()
	if !c.stopped {
		c.stopped = true
		close(c.stopCh)
	}
	c.mu.Unlock()
}

// cleanupLoop runs periodic cleanup of expired entries
func (c *Cache) cleanupLoop() {
	ticker := time.NewTicker(c.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.cleanup()
		}
	}
}

// cleanup removes expired entries from the cache
func (c *Cache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	var expiredKeys []interfaces.CacheKey

	// Find expired entries
	for key, entry := range c.entries {
		if now.After(entry.expiresAt) {
			expiredKeys = append(expiredKeys, key)
		}
	}

	// Remove expired entries
	for _, key := range expiredKeys {
		delete(c.entries, key)
		c.stats.Size--
		c.stats.Evictions++
	}

	if len(expiredKeys) > 0 {
		klog.V(4).InfoS("Cache cleanup completed", "expiredEntries", len(expiredKeys))
	}
}

// updateHitRatio calculates the current cache hit ratio
func (c *Cache) updateHitRatio() {
	total := c.stats.Hits + c.stats.Misses
	if total == 0 {
		c.stats.HitRatio = 0
	} else {
		c.stats.HitRatio = float64(c.stats.Hits) / float64(total)
	}
}

// keyString creates a string representation of a cache key for logging
func (c *Cache) keyString(key interfaces.CacheKey) string {
	return fmt.Sprintf("%s/%s:%s", key.GVR.String(), key.NamespacedName.String(), key.AggregationHash[:8])
}

// deepCopyAggregatedStatus creates a deep copy of AggregatedStatus
func deepCopyAggregatedStatus(status *interfaces.AggregatedStatus) *interfaces.AggregatedStatus {
	if status == nil {
		return nil
	}

	copy := &interfaces.AggregatedStatus{
		AggregatedAt: status.AggregatedAt,
		Strategy:     status.Strategy,
		Sources:      make([]string, len(status.Sources)),
		Conflicts:    make([]interfaces.StatusConflict, len(status.Conflicts)),
	}

	// Copy sources slice
	copy.Sources = append(copy.Sources, status.Sources...)

	// Copy conflicts slice
	for i, conflict := range status.Conflicts {
		copy.Conflicts[i] = interfaces.StatusConflict{
			FieldPath:          conflict.FieldPath,
			ConflictingSources: append([]string{}, conflict.ConflictingSources...),
			Values:             make(map[string]interface{}),
			Resolution:         conflict.Resolution,
		}

		// Deep copy Values map
		for k, v := range conflict.Values {
			copy.Conflicts[i].Values[k] = v
		}
	}

	// Deep copy status if present
	if status.Status != nil {
		copy.Status = status.Status.DeepCopy()
	}

	return copy
}

// GenerateAggregationHash creates a hash for aggregation parameters
func GenerateAggregationHash(strategy interfaces.AggregationStrategy, sources []string) string {
	hasher := sha256.New()
	hasher.Write([]byte(string(strategy)))
	for _, source := range sources {
		hasher.Write([]byte(source))
	}
	return fmt.Sprintf("%x", hasher.Sum(nil))
}