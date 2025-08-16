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

// Package interfaces provides core interfaces for the TMC status management system.
package interfaces

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/unstructured"
)

// StatusCollector defines the interface for collecting status information
// from multiple sources (workload clusters, syncer instances, etc.)
type StatusCollector interface {
	// CollectStatus gathers status from a specific source for the given resource
	CollectStatus(ctx context.Context, source string, gvr schema.GroupVersionResource, key types.NamespacedName) (*StatusUpdate, error)

	// CollectAllStatus gathers status from all configured sources for the given resource
	CollectAllStatus(ctx context.Context, gvr schema.GroupVersionResource, key types.NamespacedName) ([]*StatusUpdate, error)

	// RegisterSource adds a new source to collect status from
	RegisterSource(source string, config SourceConfig) error

	// UnregisterSource removes a source from collection
	UnregisterSource(source string) error

	// Sources returns all registered sources
	Sources() []string
}

// StatusAggregator defines the interface for aggregating status updates
// from multiple sources into a consolidated status
type StatusAggregator interface {
	// AggregateStatus combines multiple status updates using the specified strategy
	AggregateStatus(ctx context.Context, updates []*StatusUpdate, strategy AggregationStrategy) (*AggregatedStatus, error)

	// SetDefaultStrategy sets the default aggregation strategy for a resource type
	SetDefaultStrategy(gvr schema.GroupVersionResource, strategy AggregationStrategy)

	// GetDefaultStrategy returns the default aggregation strategy for a resource type
	GetDefaultStrategy(gvr schema.GroupVersionResource) AggregationStrategy
}

// StatusMerger defines the interface for merging status fields
// at the field level with configurable strategies
type StatusMerger interface {
	// MergeFields merges status fields from multiple sources
	MergeFields(ctx context.Context, statuses []*StatusUpdate, config MergeConfig) (*unstructured.Unstructured, error)

	// RegisterFieldMerger registers a custom merger for specific fields
	RegisterFieldMerger(fieldPath string, merger FieldMerger)

	// UnregisterFieldMerger removes a custom field merger
	UnregisterFieldMerger(fieldPath string)
}

// StatusCache defines the interface for caching aggregated status
// to improve performance and reduce redundant calculations
type StatusCache interface {
	// Get retrieves cached status if available and not expired
	Get(ctx context.Context, key CacheKey) (*AggregatedStatus, bool)

	// Set stores aggregated status with TTL
	Set(ctx context.Context, key CacheKey, status *AggregatedStatus, ttl time.Duration)

	// Delete removes cached status
	Delete(ctx context.Context, key CacheKey)

	// Clear removes all cached entries
	Clear(ctx context.Context)

	// Stats returns cache statistics
	Stats() CacheStats
}

// StatusUpdate represents a single status update from a source
type StatusUpdate struct {
	// Source identifies where this status came from (e.g., cluster name, syncer ID)
	Source string

	// Timestamp when this status was collected
	Timestamp time.Time

	// ResourceVersion of the source resource
	ResourceVersion string

	// Status contains the actual status data
	Status *unstructured.Unstructured

	// Metadata contains additional source-specific metadata
	Metadata map[string]interface{}
}

// AggregatedStatus represents the final aggregated status
type AggregatedStatus struct {
	// Status is the aggregated status object
	Status *unstructured.Unstructured

	// Sources lists all sources that contributed to this aggregation
	Sources []string

	// AggregatedAt is when this aggregation was performed
	AggregatedAt time.Time

	// Strategy used for aggregation
	Strategy AggregationStrategy

	// Conflicts indicates if there were any conflicts during aggregation
	Conflicts []StatusConflict
}

// StatusConflict represents a conflict detected during status aggregation
type StatusConflict struct {
	// FieldPath where the conflict occurred
	FieldPath string

	// ConflictingSources lists sources with conflicting values
	ConflictingSources []string

	// Values maps source names to their conflicting values
	Values map[string]interface{}

	// Resolution describes how the conflict was resolved
	Resolution string
}

// AggregationStrategy defines how to aggregate status from multiple sources
type AggregationStrategy string

const (
	// LatestWins uses the most recent status update
	AggregationStrategyLatestWins AggregationStrategy = "latest-wins"

	// MergeAll attempts to merge all status updates
	AggregationStrategyMergeAll AggregationStrategy = "merge-all"

	// ConflictDetection detects and reports conflicts without resolution
	AggregationStrategyConflictDetection AggregationStrategy = "conflict-detection"

	// SourcePriority uses predefined source priorities
	AggregationStrategySourcePriority AggregationStrategy = "source-priority"
)

// SourceConfig contains configuration for a status collection source
type SourceConfig struct {
	// Endpoint is the source endpoint for status collection
	Endpoint string

	// Priority defines source priority for conflict resolution
	Priority int

	// Timeout for status collection operations
	Timeout time.Duration

	// RetryPolicy defines retry behavior
	RetryPolicy RetryPolicy

	// Metadata contains source-specific metadata
	Metadata map[string]interface{}
}

// RetryPolicy defines retry behavior for status collection
type RetryPolicy struct {
	// MaxRetries is the maximum number of retry attempts
	MaxRetries int

	// InitialDelay is the initial delay between retries
	InitialDelay time.Duration

	// MaxDelay is the maximum delay between retries
	MaxDelay time.Duration

	// BackoffFactor multiplies the delay after each retry
	BackoffFactor float64
}

// MergeConfig defines how to merge status fields
type MergeConfig struct {
	// DefaultStrategy is used when no specific field merger is registered
	DefaultStrategy FieldMergeStrategy

	// FieldStrategies maps field paths to specific merge strategies
	FieldStrategies map[string]FieldMergeStrategy

	// ConflictBehavior defines what to do when conflicts are detected
	ConflictBehavior ConflictBehavior
}

// FieldMergeStrategy defines how to merge individual fields
type FieldMergeStrategy string

const (
	// FieldMergeLatest uses the latest value
	FieldMergeLatest FieldMergeStrategy = "latest"

	// FieldMergeConcat concatenates string values
	FieldMergeConcat FieldMergeStrategy = "concat"

	// FieldMergeSum sums numeric values
	FieldMergeSum FieldMergeStrategy = "sum"

	// FieldMergeMax uses the maximum value
	FieldMergeMax FieldMergeStrategy = "max"

	// FieldMergeMin uses the minimum value
	FieldMergeMin FieldMergeStrategy = "min"

	// FieldMergeArray merges arrays
	FieldMergeArray FieldMergeStrategy = "array"
)

// ConflictBehavior defines how to handle merge conflicts
type ConflictBehavior string

const (
	// ConflictBehaviorIgnore ignores conflicts and uses first value
	ConflictBehaviorIgnore ConflictBehavior = "ignore"

	// ConflictBehaviorError returns an error on conflicts
	ConflictBehaviorError ConflictBehavior = "error"

	// ConflictBehaviorLog logs conflicts but continues
	ConflictBehaviorLog ConflictBehavior = "log"
)

// FieldMerger defines a custom merger for specific fields
type FieldMerger interface {
	// MergeField merges values for a specific field
	MergeField(ctx context.Context, values []interface{}) (interface{}, error)
}

// CacheKey uniquely identifies a cached status entry
type CacheKey struct {
	// GVR is the GroupVersionResource
	GVR schema.GroupVersionResource

	// NamespacedName uniquely identifies the resource
	NamespacedName types.NamespacedName

	// AggregationHash is a hash of the aggregation parameters
	AggregationHash string
}

// CacheStats provides cache performance statistics
type CacheStats struct {
	// Hits is the number of cache hits
	Hits int64

	// Misses is the number of cache misses
	Misses int64

	// Evictions is the number of cache evictions
	Evictions int64

	// Size is the current number of cached entries
	Size int64

	// HitRatio is the cache hit ratio (hits / (hits + misses))
	HitRatio float64
}

// ResourceStatusProvider defines an interface for resources that can provide
// their status for aggregation
type ResourceStatusProvider interface {
	runtime.Object

	// GetStatus returns the current status of the resource
	GetStatus() *unstructured.Unstructured

	// SetStatus updates the status of the resource
	SetStatus(status *unstructured.Unstructured)
}