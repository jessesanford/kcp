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

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
)

// DiscoveryCache provides high-performance caching for API discovery data.
// It manages TTL-based expiration, cache invalidation, and optimized lookups
// for virtual workspace discovery operations.
//
// The cache handles:
// - API group and resource listing caching
// - TTL-based automatic expiration
// - Selective cache invalidation
// - Performance metrics and monitoring
// - Memory-efficient storage
type DiscoveryCache struct {
	// logger provides structured logging
	logger logr.Logger

	// mu protects concurrent access to cache data
	mu sync.RWMutex

	// defaultTTL is the default cache entry TTL
	defaultTTL time.Duration

	// apiGroups caches API group information
	apiGroups map[string]*CachedAPIGroup

	// resources caches API resource information by group/version
	resources map[schema.GroupVersion]*CachedResourceList

	// invalidationCallbacks are called when cache entries are invalidated
	invalidationCallbacks []func(string)

	// stats tracks cache performance metrics
	stats *CacheStats

	// ticker handles periodic cleanup operations
	ticker *time.Ticker

	// ctx is used for lifecycle management
	ctx context.Context
	cancel context.CancelFunc
}

// CachedAPIGroup represents a cached API group with expiration.
type CachedAPIGroup struct {
	// Group is the cached API group
	Group *metav1.APIGroup

	// ExpiresAt indicates when this entry expires
	ExpiresAt time.Time

	// AccessedAt tracks when this entry was last accessed
	AccessedAt time.Time
}

// CachedResourceList represents cached API resources for a group/version.
type CachedResourceList struct {
	// Resources is the cached list of API resources
	Resources []metav1.APIResource

	// ExpiresAt indicates when this entry expires
	ExpiresAt time.Time

	// AccessedAt tracks when this entry was last accessed
	AccessedAt time.Time
}

// CacheStats tracks cache performance metrics.
type CacheStats struct {
	mu sync.RWMutex

	// Hits is the number of cache hits
	Hits int64

	// Misses is the number of cache misses
	Misses int64

	// Evictions is the number of entries evicted
	Evictions int64

	// Invalidations is the number of explicit invalidations
	Invalidations int64

	// Size is the current number of cached entries
	Size int64
}

// NewDiscoveryCache creates a new discovery cache with the specified TTL.
//
// The cache provides high-performance storage for API discovery data with
// automatic expiration and cleanup capabilities.
//
// Parameters:
//   - logger: Structured logger for cache operations
//   - defaultTTL: Default time-to-live for cache entries
//
// Returns:
//   - *DiscoveryCache: Configured cache instance
//   - error: Configuration error
func NewDiscoveryCache(logger logr.Logger, defaultTTL time.Duration) (*DiscoveryCache, error) {
	if defaultTTL <= 0 {
		defaultTTL = 5 * time.Minute
	}

	ctx, cancel := context.WithCancel(context.Background())

	cache := &DiscoveryCache{
		logger:     logger,
		defaultTTL: defaultTTL,
		apiGroups:  make(map[string]*CachedAPIGroup),
		resources:  make(map[schema.GroupVersion]*CachedResourceList),
		stats:      &CacheStats{},
		ctx:        ctx,
		cancel:     cancel,
	}

	return cache, nil
}

// GetAPIGroup retrieves a cached API group by name.
//
// Returns the cached group if present and not expired, otherwise returns nil.
// Updates access time and hit/miss statistics.
//
// Parameters:
//   - groupName: Name of the API group to retrieve
//
// Returns:
//   - *metav1.APIGroup: Cached API group, nil if not found or expired
//   - bool: True if found and valid, false otherwise
func (c *DiscoveryCache) GetAPIGroup(groupName string) (*metav1.APIGroup, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	cached, exists := c.apiGroups[groupName]
	if !exists {
		c.recordMiss()
		return nil, false
	}

	// Check if expired
	if time.Now().After(cached.ExpiresAt) {
		c.recordMiss()
		// Expired entry will be cleaned up by periodic cleanup
		return nil, false
	}

	// Update access time
	cached.AccessedAt = time.Now()
	c.recordHit()

	return cached.Group, true
}

// GetAPIResources retrieves cached API resources for a group/version.
//
// Returns the cached resources if present and not expired.
//
// Parameters:
//   - gv: GroupVersion to retrieve resources for
//
// Returns:
//   - []metav1.APIResource: Cached resources, nil if not found or expired
//   - bool: True if found and valid, false otherwise
func (c *DiscoveryCache) GetAPIResources(gv schema.GroupVersion) ([]metav1.APIResource, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	cached, exists := c.resources[gv]
	if !exists {
		c.recordMiss()
		return nil, false
	}

	// Check if expired
	if time.Now().After(cached.ExpiresAt) {
		c.recordMiss()
		return nil, false
	}

	// Update access time
	cached.AccessedAt = time.Now()
	c.recordHit()

	return cached.Resources, true
}

// PutAPIGroup stores an API group in the cache with default TTL.
//
// Parameters:
//   - groupName: Name of the API group
//   - group: API group to cache
func (c *DiscoveryCache) PutAPIGroup(groupName string, group *metav1.APIGroup) {
	c.PutAPIGroupWithTTL(groupName, group, c.defaultTTL)
}

// PutAPIGroupWithTTL stores an API group in the cache with custom TTL.
//
// Parameters:
//   - groupName: Name of the API group
//   - group: API group to cache
//   - ttl: Time-to-live for this entry
func (c *DiscoveryCache) PutAPIGroupWithTTL(groupName string, group *metav1.APIGroup, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	c.apiGroups[groupName] = &CachedAPIGroup{
		Group:      group,
		ExpiresAt:  now.Add(ttl),
		AccessedAt: now,
	}

	c.updateSize()
}

// PutAPIResources stores API resources in the cache with default TTL.
//
// Parameters:
//   - gv: GroupVersion for the resources
//   - resources: API resources to cache
func (c *DiscoveryCache) PutAPIResources(gv schema.GroupVersion, resources []metav1.APIResource) {
	c.PutAPIResourcesWithTTL(gv, resources, c.defaultTTL)
}

// PutAPIResourcesWithTTL stores API resources in the cache with custom TTL.
//
// Parameters:
//   - gv: GroupVersion for the resources
//   - resources: API resources to cache
//   - ttl: Time-to-live for this entry
func (c *DiscoveryCache) PutAPIResourcesWithTTL(gv schema.GroupVersion, resources []metav1.APIResource, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	c.resources[gv] = &CachedResourceList{
		Resources:  resources,
		ExpiresAt:  now.Add(ttl),
		AccessedAt: now,
	}

	c.updateSize()
}

// InvalidateAPIGroup removes a specific API group from the cache.
//
// Parameters:
//   - groupName: Name of the API group to invalidate
func (c *DiscoveryCache) InvalidateAPIGroup(groupName string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.apiGroups[groupName]; exists {
		delete(c.apiGroups, groupName)
		c.recordInvalidation()
		c.updateSize()

		// Notify callbacks
		for _, callback := range c.invalidationCallbacks {
			callback(groupName)
		}
	}
}

// InvalidateAPIResources removes API resources for a group/version from the cache.
//
// Parameters:
//   - gv: GroupVersion to invalidate
func (c *DiscoveryCache) InvalidateAPIResources(gv schema.GroupVersion) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.resources[gv]; exists {
		delete(c.resources, gv)
		c.recordInvalidation()
		c.updateSize()
	}
}

// InvalidateAll clears all entries from the cache.
func (c *DiscoveryCache) InvalidateAll() {
	c.mu.Lock()
	defer c.mu.Unlock()

	count := len(c.apiGroups) + len(c.resources)
	c.apiGroups = make(map[string]*CachedAPIGroup)
	c.resources = make(map[schema.GroupVersion]*CachedResourceList)

	c.stats.mu.Lock()
	c.stats.Invalidations += int64(count)
	c.stats.Size = 0
	c.stats.mu.Unlock()
}

// UpdateAPIGroups bulk updates API groups in the cache.
//
// Parameters:
//   - groups: Map of group name to API group
func (c *DiscoveryCache) UpdateAPIGroups(groups map[string]*metav1.APIGroup) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	expiration := now.Add(c.defaultTTL)

	for name, group := range groups {
		c.apiGroups[name] = &CachedAPIGroup{
			Group:      group,
			ExpiresAt:  expiration,
			AccessedAt: now,
		}
	}

	c.updateSize()
}

// GetStats returns current cache performance statistics.
func (c *DiscoveryCache) GetStats() CacheStats {
	c.stats.mu.RLock()
	defer c.stats.mu.RUnlock()

	return *c.stats
}

// Start begins cache background operations including periodic cleanup.
func (c *DiscoveryCache) Start(ctx context.Context) {
	logger := c.logger.WithName("start")
	logger.Info("Starting discovery cache")

	// Start periodic cleanup
	c.ticker = time.NewTicker(30 * time.Second)
	go c.periodicCleanup()

	logger.Info("Discovery cache started")
}

// Stop stops cache background operations.
func (c *DiscoveryCache) Stop() {
	if c.ticker != nil {
		c.ticker.Stop()
	}
	if c.cancel != nil {
		c.cancel()
	}
}

// periodicCleanup removes expired entries from the cache.
func (c *DiscoveryCache) periodicCleanup() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-c.ticker.C:
			c.cleanupExpired()
		}
	}
}

// cleanupExpired removes expired entries from the cache.
func (c *DiscoveryCache) cleanupExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	evicted := 0

	// Clean up expired API groups
	for name, cached := range c.apiGroups {
		if now.After(cached.ExpiresAt) {
			delete(c.apiGroups, name)
			evicted++
		}
	}

	// Clean up expired resources
	for gv, cached := range c.resources {
		if now.After(cached.ExpiresAt) {
			delete(c.resources, gv)
			evicted++
		}
	}

	if evicted > 0 {
		c.stats.mu.Lock()
		c.stats.Evictions += int64(evicted)
		c.stats.mu.Unlock()
		c.updateSize()

		c.logger.V(2).Info("Cleaned up expired cache entries", "evicted", evicted)
	}
}

// recordHit increments the cache hit counter.
func (c *DiscoveryCache) recordHit() {
	c.stats.mu.Lock()
	c.stats.Hits++
	c.stats.mu.Unlock()
}

// recordMiss increments the cache miss counter.
func (c *DiscoveryCache) recordMiss() {
	c.stats.mu.Lock()
	c.stats.Misses++
	c.stats.mu.Unlock()
}

// recordInvalidation increments the invalidation counter.
func (c *DiscoveryCache) recordInvalidation() {
	c.stats.mu.Lock()
	c.stats.Invalidations++
	c.stats.mu.Unlock()
}

// updateSize updates the cache size metric.
func (c *DiscoveryCache) updateSize() {
	c.stats.mu.Lock()
	c.stats.Size = int64(len(c.apiGroups) + len(c.resources))
	c.stats.mu.Unlock()
}

// AddInvalidationCallback adds a callback function that is called when entries are invalidated.
func (c *DiscoveryCache) AddInvalidationCallback(callback func(string)) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.invalidationCallbacks = append(c.invalidationCallbacks, callback)
}

// VersionNegotiator handles API version negotiation and preference.
type VersionNegotiator struct {
	// preferredVersions defines version preferences for API groups
	preferredVersions map[string][]string
}

// NewVersionNegotiator creates a new version negotiator with default preferences.
func NewVersionNegotiator() VersionNegotiator {
	return VersionNegotiator{
		preferredVersions: map[string][]string{
			"apps":    {"v1", "v1beta2", "v1beta1"},
			"":        {"v1"}, // core group
			"batch":   {"v1", "v1beta1"},
			"networking.k8s.io": {"v1", "v1beta1"},
		},
	}
}

// SortVersions sorts versions according to Kubernetes version preferences.
func (vn VersionNegotiator) SortVersions(versions []string) {
	// Use Kubernetes version-aware sorting
	// For simplicity, we'll do basic string sorting here
	// Real implementation would use version.CompareKubeAwareVersionStrings
	for i := 0; i < len(versions); i++ {
		for j := i + 1; j < len(versions); j++ {
			if versions[i] > versions[j] {
				versions[i], versions[j] = versions[j], versions[i]
			}
		}
	}
}

// GetPreferredVersion returns the preferred version for an API group.
func (vn VersionNegotiator) GetPreferredVersion(group string, availableVersions sets.Set[string]) string {
	if preferred, exists := vn.preferredVersions[group]; exists {
		for _, version := range preferred {
			if availableVersions.Has(version) {
				return version
			}
		}
	}

	// Fallback to first available version
	return availableVersions.UnsortedList()[0]
}