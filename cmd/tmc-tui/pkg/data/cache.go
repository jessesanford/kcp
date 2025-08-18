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

package data

import (
	"sync"
	"time"

	"k8s.io/klog/v2"
)

// CacheEntry represents a cached data item with timestamp and expiry.
type CacheEntry struct {
	Data      interface{}
	Timestamp time.Time
	TTL       time.Duration
}

// IsExpired checks if the cache entry has expired.
func (c *CacheEntry) IsExpired() bool {
	if c.TTL == 0 {
		return false // Never expires if TTL is 0
	}
	return time.Since(c.Timestamp) > c.TTL
}

// Cache provides thread-safe caching for TUI data.
type Cache struct {
	mu      sync.RWMutex
	entries map[string]*CacheEntry
	defaultTTL time.Duration
}

// NewCache creates a new cache with the specified default TTL.
func NewCache(defaultTTL time.Duration) *Cache {
	return &Cache{
		entries:    make(map[string]*CacheEntry),
		defaultTTL: defaultTTL,
	}
}

// Get retrieves a value from the cache if it exists and hasn't expired.
// Returns the data and a boolean indicating if the data was found and valid.
func (c *Cache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.entries[key]
	if !exists {
		return nil, false
	}

	if entry.IsExpired() {
		klog.V(4).Infof("Cache entry expired for key: %s", key)
		return nil, false
	}

	klog.V(4).Infof("Cache hit for key: %s", key)
	return entry.Data, true
}

// Set stores a value in the cache with the default TTL.
func (c *Cache) Set(key string, data interface{}) {
	c.SetWithTTL(key, data, c.defaultTTL)
}

// SetWithTTL stores a value in the cache with a custom TTL.
func (c *Cache) SetWithTTL(key string, data interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[key] = &CacheEntry{
		Data:      data,
		Timestamp: time.Now(),
		TTL:       ttl,
	}

	klog.V(4).Infof("Cached data for key: %s (TTL: %v)", key, ttl)
}

// Delete removes a specific key from the cache.
func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.entries[key]; exists {
		delete(c.entries, key)
		klog.V(4).Infof("Deleted cache entry for key: %s", key)
	}
}

// Clear removes all entries from the cache.
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	count := len(c.entries)
	c.entries = make(map[string]*CacheEntry)
	klog.V(3).Infof("Cleared cache (%d entries removed)", count)
}

// CleanupExpired removes all expired entries from the cache.
// Returns the number of entries that were removed.
func (c *Cache) CleanupExpired() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	var expiredKeys []string
	for key, entry := range c.entries {
		if entry.IsExpired() {
			expiredKeys = append(expiredKeys, key)
		}
	}

	for _, key := range expiredKeys {
		delete(c.entries, key)
	}

	if len(expiredKeys) > 0 {
		klog.V(3).Infof("Cleaned up %d expired cache entries", len(expiredKeys))
	}

	return len(expiredKeys)
}

// Size returns the current number of entries in the cache.
func (c *Cache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// Keys returns all keys currently in the cache.
func (c *Cache) Keys() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	keys := make([]string, 0, len(c.entries))
	for key := range c.entries {
		keys = append(keys, key)
	}
	return keys
}

// Stats returns cache statistics.
type CacheStats struct {
	Size         int
	ExpiredCount int
}

// GetStats returns current cache statistics.
func (c *Cache) GetStats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var expiredCount int
	for _, entry := range c.entries {
		if entry.IsExpired() {
			expiredCount++
		}
	}

	return CacheStats{
		Size:         len(c.entries),
		ExpiredCount: expiredCount,
	}
}