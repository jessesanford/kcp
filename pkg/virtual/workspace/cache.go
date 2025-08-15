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

package workspace

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
)

// Cache provides a generic caching interface for workspace-related data.
// It supports TTL-based expiration, eviction policies, and statistics tracking.
type Cache interface {
	// Get retrieves an item from cache by key.
	// Returns the value and a boolean indicating if the key was found.
	Get(ctx context.Context, key string) (interface{}, bool)

	// Set stores an item in cache with the specified TTL.
	// If TTL is 0, the item will use the cache's default TTL.
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error

	// Delete removes an item from cache by key.
	// Returns true if the item was deleted, false if it didn't exist.
	Delete(ctx context.Context, key string) bool

	// Clear removes all items from the cache.
	// This operation may be expensive for large caches.
	Clear(ctx context.Context) error

	// Keys returns all cache keys.
	// For large caches, consider using KeysByPattern instead.
	Keys(ctx context.Context) ([]string, error)

	// KeysByPattern returns keys matching the specified pattern.
	// Supports wildcard patterns like "workspace:*" or "user:*:config".
	KeysByPattern(ctx context.Context, pattern string) ([]string, error)

	// Stats returns current cache statistics.
	// Includes hit rate, miss rate, eviction count, and memory usage.
	Stats(ctx context.Context) (*CacheStats, error)

	// Size returns the current number of items in the cache.
	Size(ctx context.Context) (int, error)

	// Flush forces a cache flush to persistent storage if applicable.
	// No-op for in-memory only caches.
	Flush(ctx context.Context) error
}

// ObjectCache provides type-safe caching for Kubernetes runtime objects.
// It handles serialization/deserialization automatically and supports
// object-specific operations.
type ObjectCache interface {
	// GetObject retrieves a typed object from cache.
	// The obj parameter should be a pointer to the target type.
	GetObject(ctx context.Context, key string, obj runtime.Object) (bool, error)

	// SetObject stores a typed object in cache with TTL.
	// The object is automatically serialized for storage.
	SetObject(ctx context.Context, key string, obj runtime.Object, ttl time.Duration) error

	// DeleteObject removes an object from cache by key.
	DeleteObject(ctx context.Context, key string) error

	// ListObjects returns all cached objects of a specific type.
	// The objList parameter should be a pointer to a list type.
	ListObjects(ctx context.Context, objList runtime.Object) error

	// InvalidatePattern removes all cached objects matching the pattern.
	// Useful for invalidating related objects (e.g., "workspace:foo:*").
	InvalidatePattern(ctx context.Context, pattern string) error

	// Watch returns a channel that receives cache events for object changes.
	// Events include additions, updates, and deletions.
	Watch(ctx context.Context, pattern string) (<-chan ObjectCacheEvent, error)
}

// CacheStats provides comprehensive cache performance metrics.
type CacheStats struct {
	// Hits is the total number of cache hits
	Hits int64

	// Misses is the total number of cache misses
	Misses int64

	// Evictions is the total number of items evicted
	Evictions int64

	// Size is the current cache size in bytes
	Size int64

	// Items is the current number of items in cache
	Items int64

	// HitRate is the cache hit rate percentage (0-100)
	HitRate float64

	// MemoryUsage tracks memory consumption in bytes
	MemoryUsage int64

	// AverageItemSize is the average size per cached item
	AverageItemSize int64

	// OldestItem tracks the age of the oldest cached item
	OldestItem time.Duration

	// ExpiredItems counts items that expired naturally
	ExpiredItems int64
}

// CacheConfig defines cache behavior and resource limits.
type CacheConfig struct {
	// MaxSize is the maximum cache size in bytes
	// Zero means no size limit
	MaxSize int64

	// MaxItems is the maximum number of items to cache
	// Zero means no item limit
	MaxItems int

	// DefaultTTL is the default time-to-live for cache items
	// Zero means items don't expire by default
	DefaultTTL time.Duration

	// EvictionPolicy defines how items are evicted when limits are reached
	EvictionPolicy EvictionPolicy

	// EnableMetrics controls whether to collect cache performance metrics
	EnableMetrics bool

	// Persistent indicates if the cache should survive restarts
	Persistent bool

	// SyncInterval is how often to sync to persistent storage
	// Only applicable when Persistent is true
	SyncInterval time.Duration

	// CleanupInterval is how often to run cleanup operations
	// Includes removing expired items and compacting storage
	CleanupInterval time.Duration
}

// EvictionPolicy defines the strategy for removing items when cache limits are reached.
type EvictionPolicy string

const (
	// EvictionPolicyLRU removes least recently used items first
	EvictionPolicyLRU EvictionPolicy = "LRU"

	// EvictionPolicyLFU removes least frequently used items first
	EvictionPolicyLFU EvictionPolicy = "LFU"

	// EvictionPolicyFIFO removes oldest items first (first in, first out)
	EvictionPolicyFIFO EvictionPolicy = "FIFO"

	// EvictionPolicyTTL removes items closest to expiration first
	EvictionPolicyTTL EvictionPolicy = "TTL"

	// EvictionPolicyRandom removes items randomly
	EvictionPolicyRandom EvictionPolicy = "Random"
)

// ObjectCacheEvent represents a change in the object cache.
type ObjectCacheEvent struct {
	// Type specifies the type of cache event
	Type ObjectCacheEventType

	// Key is the cache key that changed
	Key string

	// Object is the cached object (may be nil for delete events)
	Object runtime.Object

	// OldObject is the previous object for update events
	OldObject runtime.Object

	// Timestamp records when the event occurred
	Timestamp time.Time
}

// ObjectCacheEventType represents different types of object cache events.
type ObjectCacheEventType string

const (
	// ObjectCacheEventAdded indicates an object was added to cache
	ObjectCacheEventAdded ObjectCacheEventType = "Added"

	// ObjectCacheEventUpdated indicates an object was updated in cache
	ObjectCacheEventUpdated ObjectCacheEventType = "Updated"

	// ObjectCacheEventDeleted indicates an object was removed from cache
	ObjectCacheEventDeleted ObjectCacheEventType = "Deleted"

	// ObjectCacheEventExpired indicates an object expired from cache
	ObjectCacheEventExpired ObjectCacheEventType = "Expired"

	// ObjectCacheEventEvicted indicates an object was evicted from cache
	ObjectCacheEventEvicted ObjectCacheEventType = "Evicted"
)

// CacheProvider creates cache instances with specified configurations.
// Different implementations can provide in-memory, Redis, or other cache backends.
type CacheProvider interface {
	// NewCache creates a new cache instance with the given configuration
	NewCache(config *CacheConfig) (Cache, error)

	// NewObjectCache creates a new object cache with the given configuration
	NewObjectCache(config *CacheConfig) (ObjectCache, error)

	// HealthCheck verifies the cache provider is functioning correctly
	HealthCheck(ctx context.Context) error
}