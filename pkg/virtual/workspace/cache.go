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

	"k8s.io/apimachinery/pkg/runtime/schema"
	
	"github.com/kcp-dev/logicalcluster/v3"
)

// WorkspaceCache provides high-performance caching for virtual workspace operations.
// Essential for scalability in multi-tenant environments with frequent workspace access.
//
// Design Principles:
// - Thread-safe concurrent access
// - Configurable eviction policies
// - Metrics and observability hooks
// - Memory-efficient storage
//
// Cache Hierarchy:
// 1. Workspace metadata (WorkspaceInfo)
// 2. Client connections and configurations
// 3. Resource discovery and capabilities
// 4. Authorization decisions
//
// Performance Characteristics:
// - Sub-millisecond lookup times for cached data
// - Automatic background refresh for active workspaces
// - Least-recently-used (LRU) eviction under memory pressure
// - Configurable TTL per data type
type WorkspaceCache interface {
	// GetWorkspaceInfo retrieves cached workspace metadata.
	// Returns nil if the workspace is not in cache or has expired.
	//
	// Parameters:
	//   - ctx: Context for cancellation and tracing
	//   - ref: Workspace to look up
	//
	// Returns:
	//   - *WorkspaceInfo: Cached metadata, or nil if not found
	//   - bool: Whether the item was found in cache
	//   - error: Cache access errors (rare)
	GetWorkspaceInfo(ctx context.Context, ref WorkspaceReference) (*WorkspaceInfo, bool, error)

	// SetWorkspaceInfo stores workspace metadata in the cache.
	// Existing entries are replaced with the new data.
	//
	// Parameters:
	//   - ctx: Context for cancellation and tracing
	//   - info: Workspace metadata to cache
	//   - ttl: Time-to-live for this entry
	//
	// Returns:
	//   - error: Storage errors or capacity issues
	SetWorkspaceInfo(ctx context.Context, info *WorkspaceInfo, ttl time.Duration) error

	// InvalidateWorkspace removes all cached data for a specific workspace.
	// Used when workspace state changes externally.
	//
	// Parameters:
	//   - ctx: Context for cancellation and tracing
	//   - ref: Workspace to invalidate
	//
	// Returns:
	//   - error: Invalidation errors (rare)
	InvalidateWorkspace(ctx context.Context, ref WorkspaceReference) error

	// GetClient retrieves a cached workspace client connection.
	// Clients are expensive to create and benefit significantly from caching.
	//
	// Parameters:
	//   - ctx: Context for cancellation and tracing
	//   - ref: Workspace for the client
	//
	// Returns:
	//   - WorkspaceClient: Cached client, or nil if not found
	//   - bool: Whether the client was found in cache
	//   - error: Cache access errors
	GetClient(ctx context.Context, ref WorkspaceReference) (WorkspaceClient, bool, error)

	// SetClient stores a workspace client in the cache.
	// Clients are cached with longer TTL due to creation cost.
	//
	// Parameters:
	//   - ctx: Context for cancellation and tracing
	//   - ref: Workspace for the client
	//   - client: Client instance to cache
	//   - ttl: Time-to-live for this entry
	//
	// Returns:
	//   - error: Storage errors or capacity issues
	SetClient(ctx context.Context, ref WorkspaceReference, client WorkspaceClient, ttl time.Duration) error

	// GetCapabilities retrieves cached API resource information for a workspace.
	// Used for feature discovery and compatibility checking.
	//
	// Parameters:
	//   - ctx: Context for cancellation and tracing
	//   - ref: Workspace to look up
	//
	// Returns:
	//   - []WorkspaceCapability: Cached capabilities, or nil if not found
	//   - bool: Whether capabilities were found in cache
	//   - error: Cache access errors
	GetCapabilities(ctx context.Context, ref WorkspaceReference) ([]WorkspaceCapability, bool, error)

	// SetCapabilities stores API resource information in the cache.
	// Capabilities change infrequently and can be cached for longer periods.
	//
	// Parameters:
	//   - ctx: Context for cancellation and tracing
	//   - ref: Workspace for the capabilities
	//   - capabilities: Resource capabilities to cache
	//   - ttl: Time-to-live for this entry
	//
	// Returns:
	//   - error: Storage errors or capacity issues
	SetCapabilities(ctx context.Context, ref WorkspaceReference, capabilities []WorkspaceCapability, ttl time.Duration) error

	// ListCachedWorkspaces returns references to all workspaces currently in cache.
	// Useful for cache management and debugging.
	//
	// Parameters:
	//   - ctx: Context for cancellation and tracing
	//
	// Returns:
	//   - []WorkspaceReference: All cached workspace references
	//   - error: Enumeration errors
	ListCachedWorkspaces(ctx context.Context) ([]WorkspaceReference, error)

	// Prune removes expired entries from the cache.
	// Typically called periodically by a background goroutine.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout
	//
	// Returns:
	//   - int: Number of entries removed
	//   - error: Pruning errors
	Prune(ctx context.Context) (int, error)

	// Clear removes all entries from the cache.
	// Used for testing and emergency cache reset scenarios.
	//
	// Parameters:
	//   - ctx: Context for cancellation
	//
	// Returns:
	//   - error: Clear operation errors
	Clear(ctx context.Context) error

	// Stats returns cache performance metrics.
	// Essential for monitoring and capacity planning.
	//
	// Parameters:
	//   - ctx: Context for cancellation
	//
	// Returns:
	//   - CacheStats: Current cache metrics
	//   - error: Stats collection errors
	Stats(ctx context.Context) (CacheStats, error)
}

// CacheStats provides observability into cache performance and resource usage.
// Used for monitoring, alerting, and capacity planning decisions.
type CacheStats struct {
	// TotalEntries is the current number of cached items.
	TotalEntries int64 `json:"totalEntries"`

	// HitRate is the percentage of cache requests that were served from cache.
	// Higher values indicate better performance.
	HitRate float64 `json:"hitRate"`

	// MissRate is the percentage of cache requests that required backend lookups.
	// Lower values indicate better cache effectiveness.
	MissRate float64 `json:"missRate"`

	// EvictionCount is the number of entries removed due to capacity limits.
	// High values may indicate insufficient cache capacity.
	EvictionCount int64 `json:"evictionCount"`

	// MemoryUsageBytes is the approximate memory consumed by cached data.
	MemoryUsageBytes int64 `json:"memoryUsageBytes"`

	// OldestEntryAge is the time since the oldest cache entry was created.
	// Useful for understanding cache turnover patterns.
	OldestEntryAge time.Duration `json:"oldestEntryAge"`

	// AverageAccessTime is the mean time for cache operations.
	// Should remain consistently low for good performance.
	AverageAccessTime time.Duration `json:"averageAccessTime"`
}

// CacheEventType represents different types of cache lifecycle events.
// Used for monitoring, debugging, and integration with external systems.
type CacheEventType string

const (
	// CacheEventTypeHit indicates a successful cache lookup.
	CacheEventTypeHit CacheEventType = "hit"

	// CacheEventTypeMiss indicates a cache lookup that required backend access.
	CacheEventTypeMiss CacheEventType = "miss"

	// CacheEventTypeSet indicates data was stored in the cache.
	CacheEventTypeSet CacheEventType = "set"

	// CacheEventTypeEvict indicates an entry was removed due to capacity limits.
	CacheEventTypeEvict CacheEventType = "evict"

	// CacheEventTypeExpire indicates an entry was removed due to TTL expiration.
	CacheEventTypeExpire CacheEventType = "expire"

	// CacheEventTypeInvalidate indicates an entry was manually removed.
	CacheEventTypeInvalidate CacheEventType = "invalidate"
)

// CacheEvent represents a cache operation for monitoring and debugging.
// Implementations may emit these events for observability integration.
type CacheEvent struct {
	// Type categorizes the cache operation.
	Type CacheEventType `json:"type"`

	// Workspace identifies the workspace involved in the operation.
	Workspace WorkspaceReference `json:"workspace"`

	// Timestamp records when the event occurred.
	Timestamp time.Time `json:"timestamp"`

	// DataType indicates what kind of data was involved (e.g., "info", "client").
	DataType string `json:"dataType"`

	// Size indicates the approximate memory impact of the operation.
	Size int64 `json:"size"`
}