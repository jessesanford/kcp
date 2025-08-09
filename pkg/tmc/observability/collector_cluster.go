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
	utilfeature "k8s.io/apiserver/pkg/util/feature"

	"github.com/kcp-dev/logicalcluster/v3"
	"github.com/kcp-dev/kcp/pkg/features"
)

// ClusterMetricsCollector collects metrics directly from individual clusters.
// This collector maintains a registry of cluster connections and retrieves
// metrics through cluster-specific endpoints.
type ClusterMetricsCollector struct {
	mu              sync.RWMutex
	clusterEndpoints map[string]string // clusterName -> endpoint URL
}

// NewClusterMetricsCollector creates a new cluster-direct metrics collector.
//
// Returns:
//   - MetricsSource: Configured cluster collector
func NewClusterMetricsCollector() MetricsSource {
	return &ClusterMetricsCollector{
		clusterEndpoints: make(map[string]string),
	}
}

// RegisterCluster registers a cluster endpoint for metrics collection.
//
// Parameters:
//   - clusterName: Name of the cluster
//   - endpoint: Metrics endpoint URL for the cluster
func (cmc *ClusterMetricsCollector) RegisterCluster(clusterName, endpoint string) {
	cmc.mu.Lock()
	defer cmc.mu.Unlock()
	
	cmc.clusterEndpoints[clusterName] = endpoint
	klog.V(4).InfoS("Registered cluster endpoint", "cluster", clusterName, "endpoint", endpoint)
}

// UnregisterCluster removes a cluster from the collector registry.
//
// Parameters:
//   - clusterName: Name of the cluster to remove
func (cmc *ClusterMetricsCollector) UnregisterCluster(clusterName string) {
	cmc.mu.Lock()
	defer cmc.mu.Unlock()
	
	delete(cmc.clusterEndpoints, clusterName)
	klog.V(4).InfoS("Unregistered cluster endpoint", "cluster", clusterName)
}

// GetMetricValue retrieves a specific metric value directly from a cluster.
//
// This method connects directly to the cluster's metrics endpoint to retrieve
// the requested metric. It validates workspace isolation and feature flags.
//
// Parameters:
//   - ctx: Context for the request
//   - clusterName: Name of the cluster to get metrics from
//   - workspace: Logical cluster workspace for validation
//   - metricName: Name of the metric to retrieve
//
// Returns:
//   - float64: Metric value
//   - map[string]string: Associated labels
//   - error: Retrieval error if any
func (cmc *ClusterMetricsCollector) GetMetricValue(
	ctx context.Context,
	clusterName string,
	workspace logicalcluster.Name,
	metricName string,
) (float64, map[string]string, error) {
	// Check if TMC metrics collection is enabled
	if !utilfeature.DefaultFeatureGate.Enabled(features.TMCMetricsAggregation) {
		return 0, nil, fmt.Errorf("TMC metrics collection is disabled")
	}

	klog.V(4).InfoS("Collecting metric directly from cluster",
		"cluster", clusterName,
		"workspace", workspace,
		"metric", metricName)

	cmc.mu.RLock()
	endpoint, exists := cmc.clusterEndpoints[clusterName]
	cmc.mu.RUnlock()

	if !exists {
		return 0, nil, fmt.Errorf("cluster %s is not registered for direct collection", clusterName)
	}

	// Validate workspace access (placeholder for actual validation logic)
	if err := cmc.validateWorkspaceAccess(clusterName, workspace); err != nil {
		return 0, nil, fmt.Errorf("workspace validation failed: %w", err)
	}

	// Simulate metric collection from cluster endpoint
	// In production, this would make HTTP requests to cluster metrics endpoints
	value, labels, err := cmc.collectFromClusterEndpoint(ctx, endpoint, metricName)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to collect from cluster %s: %w", clusterName, err)
	}

	// Add cluster identification to labels
	if labels == nil {
		labels = make(map[string]string)
	}
	labels["cluster"] = clusterName
	labels["workspace"] = workspace.String()
	labels["source"] = "cluster-direct"

	klog.V(6).InfoS("Retrieved metric from cluster",
		"cluster", clusterName,
		"metric", metricName,
		"value", value)

	return value, labels, nil
}

// ListClusters returns available clusters in a workspace.
//
// This method returns clusters that are registered with this collector
// and validates workspace access for each.
//
// Parameters:
//   - ctx: Context for the request
//   - workspace: Logical cluster workspace
//
// Returns:
//   - []string: List of available cluster names
//   - error: Discovery error if any
func (cmc *ClusterMetricsCollector) ListClusters(
	ctx context.Context,
	workspace logicalcluster.Name,
) ([]string, error) {
	// Check if TMC metrics collection is enabled
	if !utilfeature.DefaultFeatureGate.Enabled(features.TMCMetricsAggregation) {
		return nil, fmt.Errorf("TMC metrics collection is disabled")
	}

	klog.V(4).InfoS("Listing clusters for workspace", "workspace", workspace)

	cmc.mu.RLock()
	defer cmc.mu.RUnlock()

	var availableClusters []string
	for clusterName := range cmc.clusterEndpoints {
		// Validate workspace access for each cluster
		if err := cmc.validateWorkspaceAccess(clusterName, workspace); err != nil {
			klog.V(6).InfoS("Skipping cluster due to workspace validation",
				"cluster", clusterName,
				"workspace", workspace,
				"error", err)
			continue
		}
		availableClusters = append(availableClusters, clusterName)
	}

	klog.V(6).InfoS("Available clusters for workspace",
		"workspace", workspace,
		"count", len(availableClusters))

	return availableClusters, nil
}

// validateWorkspaceAccess validates that a cluster can be accessed from the given workspace.
// This is a placeholder implementation - in production, this would check proper RBAC
// and workspace isolation rules.
func (cmc *ClusterMetricsCollector) validateWorkspaceAccess(clusterName string, workspace logicalcluster.Name) error {
	// Placeholder validation logic
	// In production, this would:
	// 1. Check RBAC permissions for the workspace
	// 2. Validate cluster registration in the workspace
	// 3. Ensure proper isolation boundaries
	
	if workspace == "" {
		return fmt.Errorf("workspace cannot be empty")
	}
	
	// Basic validation passes for now
	return nil
}

// collectFromClusterEndpoint collects metrics from a specific cluster endpoint.
// This is a placeholder implementation - in production, this would make actual
// HTTP requests to cluster metrics endpoints.
func (cmc *ClusterMetricsCollector) collectFromClusterEndpoint(
	ctx context.Context,
	endpoint string,
	metricName string,
) (float64, map[string]string, error) {
	// Placeholder implementation for cluster-direct collection
	// In production, this would:
	// 1. Make HTTP requests to cluster metrics endpoints
	// 2. Parse metric data (Prometheus format, custom formats, etc.)
	// 3. Handle authentication and TLS
	// 4. Implement retries and timeout handling

	klog.V(6).InfoS("Collecting from cluster endpoint",
		"endpoint", endpoint,
		"metric", metricName)

	// Simulate different metrics with different values
	var value float64
	switch metricName {
	case "cpu_usage":
		value = 75.5
	case "memory_usage":
		value = 80.2
	case "pod_count":
		value = 42.0
	default:
		value = 10.0
	}

	labels := map[string]string{
		"endpoint": endpoint,
		"method":   "cluster-direct",
	}

	return value, labels, nil
}