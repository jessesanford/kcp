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

package collectors

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/kcp/pkg/metrics"
)

// SyncerCollector collects metrics related to the TMC syncer operations.
// It tracks sync latency, resource sync counts, errors, queue depth, and transformation time.
// This integrates with Phase 7 syncer implementation.
type SyncerCollector struct {
	mu sync.RWMutex

	// Prometheus metrics
	syncLatency       *prometheus.HistogramVec
	resourcesSynced   *prometheus.CounterVec
	syncErrors        *prometheus.CounterVec
	queueDepth        *prometheus.GaugeVec
	transformTime     *prometheus.HistogramVec
	syncerState       *prometheus.GaugeVec
	heartbeatInterval *prometheus.GaugeVec
	batchSize         *prometheus.HistogramVec

	// Internal state for metrics collection
	registry *metrics.MetricsRegistry
	enabled  bool
}

// NewSyncerCollector creates a new syncer metrics collector.
func NewSyncerCollector() *SyncerCollector {
	return &SyncerCollector{
		enabled: true, // TODO: integrate with feature flags
	}
}

// Name returns the collector name for registration.
func (c *SyncerCollector) Name() string {
	return "syncer"
}

// Init initializes the collector with the provided registry.
func (c *SyncerCollector) Init(registry *metrics.MetricsRegistry) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.registry = registry
	prom := metrics.NewPrometheusMetrics(registry)

	// Sync latency histogram - tracks how long each sync operation takes
	c.syncLatency = prom.NewHistogramVec(
		metrics.SyncerSubsystem,
		"sync_duration_seconds",
		"Time taken to sync resources to downstream clusters",
		[]string{metrics.LabelWorkspace, metrics.LabelCluster, metrics.LabelResource, metrics.LabelOperation},
		metrics.LatencyBuckets,
	)

	// Resources synced counter - tracks number of resources synchronized
	c.resourcesSynced = prom.NewCounterVec(
		metrics.SyncerSubsystem,
		"resources_synced_total",
		"Total number of resources synchronized to downstream clusters",
		[]string{metrics.LabelWorkspace, metrics.LabelCluster, metrics.LabelResource, metrics.LabelStatus},
	)

	// Sync errors counter - tracks sync failures and their types
	c.syncErrors = prom.NewCounterVec(
		metrics.SyncerSubsystem,
		"errors_total",
		"Total number of sync errors by type",
		[]string{metrics.LabelWorkspace, metrics.LabelCluster, metrics.LabelError, metrics.LabelOperation},
	)

	// Queue depth gauge - tracks current number of items waiting to be synced
	c.queueDepth = prom.NewGaugeVec(
		metrics.SyncerSubsystem,
		"queue_depth",
		"Current number of items in the sync queue",
		[]string{metrics.LabelWorkspace, metrics.LabelCluster},
	)

	// Transform time histogram - tracks resource transformation latency
	c.transformTime = prom.NewHistogramVec(
		metrics.SyncerSubsystem,
		"transform_duration_seconds",
		"Time taken to transform resources for downstream clusters",
		[]string{metrics.LabelWorkspace, metrics.LabelResource, metrics.LabelOperation},
		metrics.LatencyBuckets,
	)

	// Syncer state gauge - tracks current syncer state (0=disconnected, 1=connected, 2=syncing)
	c.syncerState = prom.NewGaugeVec(
		metrics.SyncerSubsystem,
		"state",
		"Current state of the syncer (0=disconnected, 1=connected, 2=syncing)",
		[]string{metrics.LabelWorkspace, metrics.LabelCluster},
	)

	// Heartbeat interval gauge - tracks current heartbeat interval
	c.heartbeatInterval = prom.NewGaugeVec(
		metrics.SyncerSubsystem,
		"heartbeat_interval_seconds",
		"Current heartbeat interval in seconds",
		[]string{metrics.LabelWorkspace, metrics.LabelCluster},
	)

	// Batch size histogram - tracks size of sync batches
	c.batchSize = prom.NewHistogramVec(
		metrics.SyncerSubsystem,
		"batch_size",
		"Number of resources processed in each sync batch",
		[]string{metrics.LabelWorkspace, metrics.LabelCluster},
		metrics.SizeBuckets,
	)

	// Register all metrics with Prometheus
	prom.MustRegister(
		c.syncLatency,
		c.resourcesSynced,
		c.syncErrors,
		c.queueDepth,
		c.transformTime,
		c.syncerState,
		c.heartbeatInterval,
		c.batchSize,
	)

	klog.V(2).Info("Initialized TMC syncer metrics collector")
	return nil
}

// Collect gathers current metrics from the syncer.
// This is typically called by the metrics registry on a regular interval.
func (c *SyncerCollector) Collect() error {
	if !c.enabled {
		return nil
	}

	// In a real implementation, this would collect metrics from the actual syncer
	// For now, we just ensure the metrics are properly initialized
	klog.V(4).Info("Collecting syncer metrics")
	return nil
}

// Close cleans up collector resources.
func (c *SyncerCollector) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.enabled = false
	klog.V(2).Info("Closed TMC syncer metrics collector")
	return nil
}

// Public API methods for recording metrics
// These would be called by the actual syncer implementation

// RecordSyncLatency records the latency of a sync operation.
func (c *SyncerCollector) RecordSyncLatency(workspace, cluster, resource, operation string, duration time.Duration) {
	if !c.enabled {
		return
	}

	c.syncLatency.WithLabelValues(workspace, cluster, resource, operation).Observe(duration.Seconds())
}

// RecordResourceSynced increments the counter for successfully synced resources.
func (c *SyncerCollector) RecordResourceSynced(workspace, cluster, resource, status string) {
	if !c.enabled {
		return
	}

	c.resourcesSynced.WithLabelValues(workspace, cluster, resource, status).Inc()
}

// RecordSyncError increments the counter for sync errors.
func (c *SyncerCollector) RecordSyncError(workspace, cluster, errorType, operation string) {
	if !c.enabled {
		return
	}

	c.syncErrors.WithLabelValues(workspace, cluster, errorType, operation).Inc()
}

// SetQueueDepth sets the current queue depth for a workspace/cluster combination.
func (c *SyncerCollector) SetQueueDepth(workspace, cluster string, depth float64) {
	if !c.enabled {
		return
	}

	c.queueDepth.WithLabelValues(workspace, cluster).Set(depth)
}

// RecordTransformTime records the time taken to transform a resource.
func (c *SyncerCollector) RecordTransformTime(workspace, resource, operation string, duration time.Duration) {
	if !c.enabled {
		return
	}

	c.transformTime.WithLabelValues(workspace, resource, operation).Observe(duration.Seconds())
}

// SetSyncerState sets the current state of a syncer.
func (c *SyncerCollector) SetSyncerState(workspace, cluster string, state float64) {
	if !c.enabled {
		return
	}

	c.syncerState.WithLabelValues(workspace, cluster).Set(state)
}

// SetHeartbeatInterval sets the current heartbeat interval.
func (c *SyncerCollector) SetHeartbeatInterval(workspace, cluster string, interval time.Duration) {
	if !c.enabled {
		return
	}

	c.heartbeatInterval.WithLabelValues(workspace, cluster).Set(interval.Seconds())
}

// RecordBatchSize records the size of a sync batch.
func (c *SyncerCollector) RecordBatchSize(workspace, cluster string, size float64) {
	if !c.enabled {
		return
	}

	c.batchSize.WithLabelValues(workspace, cluster).Observe(size)
}

// GetSyncerCollector returns a shared instance of the syncer collector.
// This allows the syncer implementation to access the collector for recording metrics.
var (
	syncerCollectorInstance *SyncerCollector
	syncerCollectorOnce     sync.Once
)

// GetSyncerCollector returns the global syncer collector instance.
func GetSyncerCollector() *SyncerCollector {
	syncerCollectorOnce.Do(func() {
		syncerCollectorInstance = NewSyncerCollector()
		// Register with global registry
		if err := metrics.GetRegistry().RegisterCollector(syncerCollectorInstance); err != nil {
			klog.Errorf("Failed to register syncer collector: %v", err)
		}
	})
	return syncerCollectorInstance
}