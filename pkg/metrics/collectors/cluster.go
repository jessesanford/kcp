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

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/kcp/pkg/metrics"
)

// ClusterCollector collects metrics related to cluster capacity, utilization, and health.
// It tracks cluster capacity, resource utilization, node counts, and available resources.
type ClusterCollector struct {
	mu sync.RWMutex

	// Prometheus metrics
	clusterCapacity     *prometheus.GaugeVec
	clusterUtilization  *prometheus.GaugeVec
	availableResources  *prometheus.GaugeVec
	nodeCount           *prometheus.GaugeVec
	clusterHealth       *prometheus.GaugeVec
	resourceAllocations *prometheus.GaugeVec
	networkLatency      *prometheus.HistogramVec
	clusterLoad         *prometheus.GaugeVec

	// Internal state for metrics collection
	registry *metrics.MetricsRegistry
	enabled  bool
}

// NewClusterCollector creates a new cluster metrics collector.
func NewClusterCollector() *ClusterCollector {
	return &ClusterCollector{
		enabled: true, // TODO: integrate with feature flags
	}
}

// Name returns the collector name for registration.
func (c *ClusterCollector) Name() string {
	return "cluster"
}

// Init initializes the collector with the provided registry.
func (c *ClusterCollector) Init(registry *metrics.MetricsRegistry) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.registry = registry
	prom := metrics.NewPrometheusMetrics(registry)

	// Cluster capacity gauge - tracks total capacity by resource type
	c.clusterCapacity = prom.NewGaugeVec(
		metrics.ClusterSubsystem,
		"capacity",
		"Total capacity of clusters by resource type",
		[]string{metrics.LabelCluster, metrics.LabelLocation, metrics.LabelProvider, "resource_type"},
	)

	// Cluster utilization gauge - tracks current utilization percentage by resource type
	c.clusterUtilization = prom.NewGaugeVec(
		metrics.ClusterSubsystem,
		"utilization_percent",
		"Current utilization percentage of clusters by resource type",
		[]string{metrics.LabelCluster, metrics.LabelLocation, metrics.LabelProvider, "resource_type"},
	)

	// Available resources gauge - tracks currently available resources
	c.availableResources = prom.NewGaugeVec(
		metrics.ClusterSubsystem,
		"available_resources",
		"Currently available resources in clusters",
		[]string{metrics.LabelCluster, metrics.LabelLocation, metrics.LabelProvider, "resource_type"},
	)

	// Node count gauge - tracks number of nodes per cluster
	c.nodeCount = prom.NewGaugeVec(
		metrics.ClusterSubsystem,
		"nodes",
		"Total number of nodes in each cluster",
		[]string{metrics.LabelCluster, metrics.LabelLocation, metrics.LabelProvider, "node_status"},
	)

	// Cluster health gauge - tracks cluster health status (0=unhealthy, 1=healthy)
	c.clusterHealth = prom.NewGaugeVec(
		metrics.ClusterSubsystem,
		"health_status",
		"Current health status of clusters (0=unhealthy, 1=healthy)",
		[]string{metrics.LabelCluster, metrics.LabelLocation, metrics.LabelProvider},
	)

	// Resource allocations gauge - tracks allocated resources by workload type
	c.resourceAllocations = prom.NewGaugeVec(
		metrics.ClusterSubsystem,
		"allocated_resources",
		"Resources currently allocated to workloads",
		[]string{metrics.LabelCluster, metrics.LabelWorkspace, "resource_type", "workload_type"},
	)

	// Network latency histogram - tracks network latency to clusters
	c.networkLatency = prom.NewHistogramVec(
		metrics.ClusterSubsystem,
		"network_latency_seconds",
		"Network latency to clusters from control plane",
		[]string{metrics.LabelCluster, metrics.LabelLocation, metrics.LabelProvider},
		metrics.LatencyBuckets,
	)

	// Cluster load gauge - tracks current load score of clusters
	c.clusterLoad = prom.NewGaugeVec(
		metrics.ClusterSubsystem,
		"load_score",
		"Current load score of clusters (0-100)",
		[]string{metrics.LabelCluster, metrics.LabelLocation, metrics.LabelProvider},
	)

	// Register all metrics with Prometheus
	prom.MustRegister(
		c.clusterCapacity,
		c.clusterUtilization,
		c.availableResources,
		c.nodeCount,
		c.clusterHealth,
		c.resourceAllocations,
		c.networkLatency,
		c.clusterLoad,
	)

	klog.V(2).Info("Initialized TMC cluster metrics collector")
	return nil
}

// Collect gathers current metrics from cluster monitoring.
func (c *ClusterCollector) Collect() error {
	if !c.enabled {
		return nil
	}

	// In a real implementation, this would collect metrics from actual cluster monitoring
	klog.V(4).Info("Collecting cluster metrics")
	return nil
}

// Close cleans up collector resources.
func (c *ClusterCollector) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.enabled = false
	klog.V(2).Info("Closed TMC cluster metrics collector")
	return nil
}

// Public API methods for recording metrics
// These would be called by cluster monitoring components

// SetClusterCapacity sets the total capacity for a resource type in a cluster.
func (c *ClusterCollector) SetClusterCapacity(cluster, location, provider, resourceType string, capacity float64) {
	if !c.enabled {
		return
	}

	c.clusterCapacity.WithLabelValues(cluster, location, provider, resourceType).Set(capacity)
}

// SetClusterUtilization sets the current utilization percentage for a resource type.
func (c *ClusterCollector) SetClusterUtilization(cluster, location, provider, resourceType string, utilization float64) {
	if !c.enabled {
		return
	}

	c.clusterUtilization.WithLabelValues(cluster, location, provider, resourceType).Set(utilization)
}

// SetAvailableResources sets the currently available resources in a cluster.
func (c *ClusterCollector) SetAvailableResources(cluster, location, provider, resourceType string, available float64) {
	if !c.enabled {
		return
	}

	c.availableResources.WithLabelValues(cluster, location, provider, resourceType).Set(available)
}

// SetNodeCount sets the number of nodes in a cluster by status.
func (c *ClusterCollector) SetNodeCount(cluster, location, provider, nodeStatus string, count float64) {
	if !c.enabled {
		return
	}

	c.nodeCount.WithLabelValues(cluster, location, provider, nodeStatus).Set(count)
}

// SetClusterHealth sets the health status of a cluster.
func (c *ClusterCollector) SetClusterHealth(cluster, location, provider string, healthy bool) {
	if !c.enabled {
		return
	}

	healthValue := float64(0)
	if healthy {
		healthValue = 1
	}
	c.clusterHealth.WithLabelValues(cluster, location, provider).Set(healthValue)
}

// SetResourceAllocations sets the current resource allocations in a cluster.
func (c *ClusterCollector) SetResourceAllocations(cluster, workspace, resourceType, workloadType string, allocated float64) {
	if !c.enabled {
		return
	}

	c.resourceAllocations.WithLabelValues(cluster, workspace, resourceType, workloadType).Set(allocated)
}

// RecordNetworkLatency records network latency to a cluster.
func (c *ClusterCollector) RecordNetworkLatency(cluster, location, provider string, latencySeconds float64) {
	if !c.enabled {
		return
	}

	c.networkLatency.WithLabelValues(cluster, location, provider).Observe(latencySeconds)
}

// SetClusterLoad sets the current load score of a cluster.
func (c *ClusterCollector) SetClusterLoad(cluster, location, provider string, loadScore float64) {
	if !c.enabled {
		return
	}

	c.clusterLoad.WithLabelValues(cluster, location, provider).Set(loadScore)
}

// GetClusterCollector returns a shared instance of the cluster collector.
var (
	clusterCollectorInstance *ClusterCollector
	clusterCollectorOnce     sync.Once
)

// GetClusterCollector returns the global cluster collector instance.
func GetClusterCollector() *ClusterCollector {
	clusterCollectorOnce.Do(func() {
		clusterCollectorInstance = NewClusterCollector()
		// Register with global registry
		if err := metrics.GetRegistry().RegisterCollector(clusterCollectorInstance); err != nil {
			klog.Errorf("Failed to register cluster collector: %v", err)
		}
	})
	return clusterCollectorInstance
}