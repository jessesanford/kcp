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

package observability

import (
	"context"
	"fmt"
	"sync"

	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"
)

// CollectorType represents the type of metrics collector.
type CollectorType string

const (
	// PrometheusCollector represents a Prometheus-based collector
	PrometheusCollector CollectorType = "prometheus"
	// ClusterDirectCollector represents a cluster-direct collector
	ClusterDirectCollector CollectorType = "cluster-direct"
)

// MetricsCollectorRegistry manages multiple metrics collectors and provides
// failover and load balancing capabilities.
type MetricsCollectorRegistry struct {
	mu         sync.RWMutex
	collectors map[CollectorType]MetricsSource
	priority   []CollectorType // Ordered by priority for failover
}

// NewMetricsCollectorRegistry creates a new collector registry.
//
// Returns:
//   - *MetricsCollectorRegistry: Configured registry ready for use
func NewMetricsCollectorRegistry() *MetricsCollectorRegistry {
	return &MetricsCollectorRegistry{
		collectors: make(map[CollectorType]MetricsSource),
		priority:   make([]CollectorType, 0),
	}
}

// RegisterCollector registers a metrics collector with the registry.
//
// Parameters:
//   - collectorType: Type of collector being registered
//   - collector: The metrics collector implementation
func (mcr *MetricsCollectorRegistry) RegisterCollector(collectorType CollectorType, collector MetricsSource) {
	mcr.mu.Lock()
	defer mcr.mu.Unlock()

	mcr.collectors[collectorType] = collector
	
	// Add to priority list if not already present
	for _, existing := range mcr.priority {
		if existing == collectorType {
			return
		}
	}
	mcr.priority = append(mcr.priority, collectorType)

	klog.V(4).InfoS("Registered metrics collector", "type", collectorType)
}

// UnregisterCollector removes a collector from the registry.
//
// Parameters:
//   - collectorType: Type of collector to remove
func (mcr *MetricsCollectorRegistry) UnregisterCollector(collectorType CollectorType) {
	mcr.mu.Lock()
	defer mcr.mu.Unlock()

	delete(mcr.collectors, collectorType)
	
	// Remove from priority list
	for i, existing := range mcr.priority {
		if existing == collectorType {
			mcr.priority = append(mcr.priority[:i], mcr.priority[i+1:]...)
			break
		}
	}

	klog.V(4).InfoS("Unregistered metrics collector", "type", collectorType)
}

// GetPrimaryCollector returns the primary (highest priority) collector.
// This implements the MetricsSource interface for seamless integration.
//
// Returns:
//   - MetricsSource: Primary collector or nil if none registered
func (mcr *MetricsCollectorRegistry) GetPrimaryCollector() MetricsSource {
	mcr.mu.RLock()
	defer mcr.mu.RUnlock()

	if len(mcr.priority) == 0 {
		return nil
	}

	primaryType := mcr.priority[0]
	return mcr.collectors[primaryType]
}

// GetMetricValue retrieves a metric value using the registered collectors with failover.
//
// This method tries collectors in priority order until one succeeds or all fail.
//
// Parameters:
//   - ctx: Context for the request
//   - clusterName: Name of the cluster to get metrics from
//   - workspace: Logical cluster workspace
//   - metricName: Name of the metric to retrieve
//
// Returns:
//   - float64: Metric value
//   - map[string]string: Associated labels
//   - error: Retrieval error if all collectors fail
func (mcr *MetricsCollectorRegistry) GetMetricValue(
	ctx context.Context,
	clusterName string,
	workspace logicalcluster.Name,
	metricName string,
) (float64, map[string]string, error) {
	mcr.mu.RLock()
	defer mcr.mu.RUnlock()

	if len(mcr.collectors) == 0 {
		return 0, nil, fmt.Errorf("no metrics collectors registered")
	}

	var lastError error
	
	// Try collectors in priority order
	for _, collectorType := range mcr.priority {
		collector, exists := mcr.collectors[collectorType]
		if !exists {
			continue
		}

		klog.V(6).InfoS("Trying collector for metric",
			"collector", collectorType,
			"cluster", clusterName,
			"metric", metricName)

		value, labels, err := collector.GetMetricValue(ctx, clusterName, workspace, metricName)
		if err != nil {
			lastError = err
			klog.V(4).InfoS("Collector failed, trying next",
				"collector", collectorType,
				"error", err)
			continue
		}

		// Add collector type to labels for traceability
		if labels == nil {
			labels = make(map[string]string)
		}
		labels["collector_type"] = string(collectorType)

		return value, labels, nil
	}

	return 0, nil, fmt.Errorf("all collectors failed, last error: %w", lastError)
}

// ListClusters returns available clusters using the registered collectors with failover.
//
// Parameters:
//   - ctx: Context for the request
//   - workspace: Logical cluster workspace
//
// Returns:
//   - []string: List of available cluster names
//   - error: Discovery error if all collectors fail
func (mcr *MetricsCollectorRegistry) ListClusters(
	ctx context.Context,
	workspace logicalcluster.Name,
) ([]string, error) {
	mcr.mu.RLock()
	defer mcr.mu.RUnlock()

	if len(mcr.collectors) == 0 {
		return nil, fmt.Errorf("no metrics collectors registered")
	}

	var lastError error
	
	// Try collectors in priority order
	for _, collectorType := range mcr.priority {
		collector, exists := mcr.collectors[collectorType]
		if !exists {
			continue
		}

		klog.V(6).InfoS("Trying collector for cluster list",
			"collector", collectorType,
			"workspace", workspace)

		clusters, err := collector.ListClusters(ctx, workspace)
		if err != nil {
			lastError = err
			klog.V(4).InfoS("Collector failed, trying next",
				"collector", collectorType,
				"error", err)
			continue
		}

		return clusters, nil
	}

	return nil, fmt.Errorf("all collectors failed, last error: %w", lastError)
}

// GetRegisteredCollectors returns a list of currently registered collector types.
//
// Returns:
//   - []CollectorType: List of registered collector types in priority order
func (mcr *MetricsCollectorRegistry) GetRegisteredCollectors() []CollectorType {
	mcr.mu.RLock()
	defer mcr.mu.RUnlock()

	result := make([]CollectorType, len(mcr.priority))
	copy(result, mcr.priority)
	return result
}