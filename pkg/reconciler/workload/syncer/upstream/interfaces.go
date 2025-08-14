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

package upstream

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"

	tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
)

// Syncer is the main interface for upstream synchronization
type Syncer interface {
	// Start begins the sync process with the given context
	Start(ctx context.Context) error

	// Stop gracefully stops syncing
	Stop()

	// ReconcileSyncTarget handles synchronization for a specific target
	ReconcileSyncTarget(ctx context.Context, target interface{}) error

	// GetMetrics returns current synchronization metrics
	GetMetrics() Metrics

	// IsReady returns true if the syncer is ready to process
	IsReady() bool
}

// ResourceWatcher watches resources in physical clusters
type ResourceWatcher interface {
	// Watch starts watching resources and returns event channel
	Watch(ctx context.Context, gvr schema.GroupVersionResource, namespace string) (<-chan Event, error)

	// List returns current state of resources
	List(ctx context.Context, gvr schema.GroupVersionResource, namespace string) ([]*unstructured.Unstructured, error)

	// Stop stops all watches
	Stop()

	// IsWatching returns true if actively watching the given GVR
	IsWatching(gvr schema.GroupVersionResource) bool
}

// EventProcessor processes events from physical clusters
type EventProcessor interface {
	// ProcessEvent handles a single event
	ProcessEvent(ctx context.Context, event Event) error

	// BatchProcess handles multiple events efficiently
	BatchProcess(ctx context.Context, events []Event) error

	// SetRateLimiter configures rate limiting for event processing
	SetRateLimiter(limiter RateLimiter)

	// GetQueueLength returns current event queue length
	GetQueueLength() int
}

// StatusAggregator aggregates status from multiple clusters
type StatusAggregator interface {
	// AggregateStatus combines status from multiple sources
	AggregateStatus(ctx context.Context, resources []ResourceStatus) (*AggregatedStatus, error)

	// ResolveConflicts handles conflicting status information
	ResolveConflicts(ctx context.Context, conflicts []Conflict) (*Resolution, error)

	// SetStrategy configures the conflict resolution strategy
	SetStrategy(strategy tmcv1alpha1.ConflictStrategy)

	// GetLastAggregation returns the last aggregation result
	GetLastAggregation() *AggregatedStatus
}

// CacheManager manages local cache of remote cluster state
type CacheManager interface {
	// Store stores resource state with TTL
	Store(key string, resource *unstructured.Unstructured, ttl time.Duration) error

	// Get retrieves resource from cache
	Get(key string) (*unstructured.Unstructured, bool, error)

	// Delete removes resource from cache
	Delete(key string) error

	// List lists cached resources matching selector
	List(selector labels.Selector) ([]*unstructured.Unstructured, error)

	// Flush clears all cached data
	Flush() error

	// GetMetrics returns cache metrics
	GetMetrics() CacheMetrics
}

// UpdateApplier applies updates to KCP workspace
type UpdateApplier interface {
	// Apply applies a single update to KCP
	Apply(ctx context.Context, update *Update) error

	// ApplyBatch applies multiple updates in a batch
	ApplyBatch(ctx context.Context, updates []*Update) error

	// SetDryRun enables dry-run mode
	SetDryRun(enabled bool)

	// GetAppliedCount returns number of successful applies
	GetAppliedCount() int64
}

// PhysicalClusterClient manages connections to physical clusters
type PhysicalClusterClient interface {
	// Connect establishes connection to physical cluster
	Connect(ctx context.Context, config *rest.Config) error

	// Dynamic returns dynamic client for the cluster
	Dynamic() dynamic.Interface

	// Discovery returns discovery client for the cluster
	Discovery() discovery.DiscoveryInterface

	// IsHealthy checks if connection is healthy
	IsHealthy(ctx context.Context) bool

	// GetClusterID returns unique identifier for this cluster
	GetClusterID() string

	// Close closes the connection
	Close() error
}

// ConflictResolver handles resource conflicts between clusters
type ConflictResolver interface {
	// Resolve attempts to resolve a conflict
	Resolve(ctx context.Context, conflict Conflict) (*Resolution, error)

	// SetStrategy sets the resolution strategy
	SetStrategy(strategy tmcv1alpha1.ConflictStrategy)

	// CanAutoResolve checks if conflict can be auto-resolved
	CanAutoResolve(conflict Conflict) bool
}

// RateLimiter provides rate limiting for operations
type RateLimiter interface {
	// Allow returns true if operation is allowed
	Allow() bool

	// Wait blocks until operation is allowed
	Wait(ctx context.Context) error

	// Reset resets the rate limiter
	Reset()
}