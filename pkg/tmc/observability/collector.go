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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"

	tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
	kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
)

// ClusterMetrics represents metrics from a specific cluster.
type ClusterMetrics struct {
	ClusterName string                 `json:"cluster_name"`
	Workspace   logicalcluster.Name    `json:"workspace"`
	Location    string                 `json:"location"`
	Timestamp   time.Time              `json:"timestamp"`
	Metrics     map[string]float64     `json:"metrics"`
	Labels      map[string]string      `json:"labels,omitempty"`
}

// WorkspaceAwareMetricsCollector implements cross-cluster metrics collection with KCP workspace awareness.
type WorkspaceAwareMetricsCollector struct {
	kcpClusterClient kcpclientset.ClusterInterface
	metricsManager   *MetricsManager
	mu               sync.RWMutex
	clusterCache     map[string]*clusterInfo
	cacheExpiry      time.Duration
}

// clusterInfo holds cached cluster information with expiry.
type clusterInfo struct {
	cluster    *tmcv1alpha1.ClusterRegistration
	metrics    *ClusterMetrics
	lastUpdate time.Time
}

// NewWorkspaceAwareMetricsCollector creates a new workspace-aware metrics collector.
//
// Parameters:
//   - kcpClusterClient: KCP cluster client for accessing cluster registrations
//   - metricsManager: Metrics manager for collecting prometheus metrics
//   - cacheExpiry: Duration to cache cluster information (default: 1 minute)
//
// Returns:
//   - *WorkspaceAwareMetricsCollector: Configured collector ready for use
func NewWorkspaceAwareMetricsCollector(
	kcpClusterClient kcpclientset.ClusterInterface,
	metricsManager *MetricsManager,
	cacheExpiry time.Duration,
) *WorkspaceAwareMetricsCollector {
	if cacheExpiry == 0 {
		cacheExpiry = time.Minute
	}

	return &WorkspaceAwareMetricsCollector{
		kcpClusterClient: kcpClusterClient,
		metricsManager:   metricsManager,
		clusterCache:     make(map[string]*clusterInfo),
		cacheExpiry:      cacheExpiry,
	}
}

// CollectClusterMetrics collects current metrics from a cluster in a workspace-aware manner.
//
// Parameters:
//   - ctx: Context for the collection operation
//   - clusterName: Name of the cluster to collect metrics from
//   - workspace: Logical cluster workspace
//
// Returns:
//   - *ClusterMetrics: Collected metrics from the cluster
//   - error: Collection error if any
func (wmc *WorkspaceAwareMetricsCollector) CollectClusterMetrics(
	ctx context.Context,
	clusterName string,
	workspace logicalcluster.Name,
) (*ClusterMetrics, error) {
	klog.V(4).InfoS("Collecting cluster metrics",
		"cluster", clusterName,
		"workspace", workspace)

	// Check cache first
	cacheKey := fmt.Sprintf("%s/%s", workspace, clusterName)
	if cachedInfo := wmc.getCachedClusterInfo(cacheKey); cachedInfo != nil {
		klog.V(6).InfoS("Using cached cluster metrics", "cluster", clusterName)
		return cachedInfo.metrics, nil
	}

	// Get cluster registration from KCP
	clusterReg, err := wmc.kcpClusterClient.Cluster(workspace.Path()).
		TmcV1alpha1().
		ClusterRegistrations().
		Get(ctx, clusterName, metav1.GetOptions{})

	if err != nil {
		return nil, fmt.Errorf("failed to get cluster registration %s in workspace %s: %w",
			clusterName, workspace, err)
	}

	// Collect metrics based on cluster status and capabilities
	metrics, err := wmc.collectMetricsFromCluster(ctx, clusterReg, workspace)
	if err != nil {
		return nil, fmt.Errorf("failed to collect metrics from cluster %s: %w", clusterName, err)
	}

	// Cache the results
	wmc.cacheClusterInfo(cacheKey, clusterReg, metrics)

	return metrics, nil
}

// ListClusters returns a list of clusters in the specified workspace.
//
// Parameters:
//   - ctx: Context for the list operation
//   - workspace: Logical cluster workspace
//
// Returns:
//   - []string: List of cluster names in the workspace
//   - error: List operation error if any
func (wmc *WorkspaceAwareMetricsCollector) ListClusters(
	ctx context.Context,
	workspace logicalcluster.Name,
) ([]string, error) {
	klog.V(4).InfoS("Listing clusters", "workspace", workspace)

	clusterList, err := wmc.kcpClusterClient.Cluster(workspace.Path()).
		TmcV1alpha1().
		ClusterRegistrations().
		List(ctx, metav1.ListOptions{})

	if err != nil {
		return nil, fmt.Errorf("failed to list cluster registrations in workspace %s: %w", workspace, err)
	}

	clusterNames := make([]string, 0, len(clusterList.Items))
	for _, cluster := range clusterList.Items {
		// Only include ready clusters
		if wmc.isClusterReady(&cluster) {
			clusterNames = append(clusterNames, cluster.Name)
		}
	}

	klog.V(4).InfoS("Found clusters", "count", len(clusterNames), "workspace", workspace)
	return clusterNames, nil
}

// collectMetricsFromCluster collects metrics from a specific cluster registration.
func (wmc *WorkspaceAwareMetricsCollector) collectMetricsFromCluster(
	ctx context.Context,
	clusterReg *tmcv1alpha1.ClusterRegistration,
	workspace logicalcluster.Name,
) (*ClusterMetrics, error) {
	// Generate synthetic metrics based on cluster status
	// In a production system, this would connect to actual cluster metrics endpoints
	metrics := make(map[string]float64)
	labels := make(map[string]string)

	// Add cluster metadata as labels
	labels["cluster"] = clusterReg.Name
	labels["location"] = clusterReg.Spec.Location
	labels["workspace"] = string(workspace)

	// Generate health score based on cluster conditions
	healthScore := wmc.calculateClusterHealthScore(clusterReg)
	metrics["tmc_cluster_health_score"] = healthScore

	// Generate resource utilization metrics based on capabilities
	wmc.addResourceUtilizationMetrics(metrics, clusterReg)

	// Add placement decision metrics
	metrics["tmc_placement_decisions_total"] = float64(len(clusterReg.Name) * 5)
	metrics["tmc_placement_latency_seconds"] = 0.15 + (float64(len(clusterReg.Name)) * 0.01)

	// Add controller reconciliation metrics
	metrics["tmc_controller_reconciles_total"] = float64(len(clusterReg.Name) * 10)

	// Add workload sync status metrics
	if wmc.isClusterReady(clusterReg) {
		metrics["tmc_workload_sync_status"] = 1.0 // success
	} else {
		metrics["tmc_workload_sync_status"] = 0.0 // failure
	}

	return &ClusterMetrics{
		ClusterName: clusterReg.Name,
		Workspace:   workspace,
		Location:    clusterReg.Spec.Location,
		Timestamp:   time.Now(),
		Metrics:     metrics,
		Labels:      labels,
	}, nil
}

// calculateClusterHealthScore calculates a health score based on cluster conditions.
func (wmc *WorkspaceAwareMetricsCollector) calculateClusterHealthScore(
	clusterReg *tmcv1alpha1.ClusterRegistration,
) float64 {
	if !wmc.isClusterReady(clusterReg) {
		return 0.0
	}

	// Base health score
	healthScore := 70.0

	// Boost score based on capabilities
	availableCapabilities := 0
	totalCapabilities := len(clusterReg.Spec.Capabilities)

	for _, cap := range clusterReg.Spec.Capabilities {
		if cap.Available {
			availableCapabilities++
		}
	}

	if totalCapabilities > 0 {
		capabilityBonus := float64(availableCapabilities) / float64(totalCapabilities) * 30.0
		healthScore += capabilityBonus
	}

	// Check heartbeat freshness
	if clusterReg.Status.LastHeartbeat != nil {
		timeSinceHeartbeat := time.Since(clusterReg.Status.LastHeartbeat.Time)
		if timeSinceHeartbeat < 2*time.Minute {
			// Recent heartbeat - no penalty
		} else if timeSinceHeartbeat < 5*time.Minute {
			healthScore -= 10.0
		} else {
			healthScore -= 20.0
		}
	}

	// Ensure score is within bounds
	if healthScore < 0 {
		healthScore = 0
	}
	if healthScore > 100 {
		healthScore = 100
	}

	return healthScore
}

// addResourceUtilizationMetrics adds resource utilization metrics based on cluster capabilities.
func (wmc *WorkspaceAwareMetricsCollector) addResourceUtilizationMetrics(
	metrics map[string]float64,
	clusterReg *tmcv1alpha1.ClusterRegistration,
) {
	// Generate synthetic resource utilization metrics
	// In production, these would come from actual cluster monitoring

	for _, cap := range clusterReg.Spec.Capabilities {
		if cap.Available {
			switch cap.Type {
			case "compute":
				metrics["tmc_cluster_cpu_utilization_percent"] = 45.0 + (float64(len(clusterReg.Name)) * 2.5)
				metrics["tmc_cluster_memory_utilization_percent"] = 55.0 + (float64(len(clusterReg.Name)) * 1.8)
			case "storage":
				metrics["tmc_cluster_storage_utilization_percent"] = 35.0 + (float64(len(clusterReg.Name)) * 3.2)
			case "network":
				metrics["tmc_cluster_network_utilization_percent"] = 25.0 + (float64(len(clusterReg.Name)) * 1.5)
			}
		}
	}
}

// isClusterReady checks if a cluster is ready based on its conditions.
func (wmc *WorkspaceAwareMetricsCollector) isClusterReady(
	clusterReg *tmcv1alpha1.ClusterRegistration,
) bool {
	for _, condition := range clusterReg.Status.Conditions {
		if condition.Type == string(tmcv1alpha1.ClusterRegistrationReady) {
			return condition.Status == metav1.ConditionTrue
		}
	}
	return false
}

// getCachedClusterInfo retrieves cached cluster information if still valid.
func (wmc *WorkspaceAwareMetricsCollector) getCachedClusterInfo(cacheKey string) *clusterInfo {
	wmc.mu.RLock()
	defer wmc.mu.RUnlock()

	if info, exists := wmc.clusterCache[cacheKey]; exists {
		if time.Since(info.lastUpdate) < wmc.cacheExpiry {
			return info
		}
	}

	return nil
}

// cacheClusterInfo caches cluster information with timestamp.
func (wmc *WorkspaceAwareMetricsCollector) cacheClusterInfo(
	cacheKey string,
	clusterReg *tmcv1alpha1.ClusterRegistration,
	metrics *ClusterMetrics,
) {
	wmc.mu.Lock()
	defer wmc.mu.Unlock()

	wmc.clusterCache[cacheKey] = &clusterInfo{
		cluster:    clusterReg,
		metrics:    metrics,
		lastUpdate: time.Now(),
	}

	// Clean up expired entries
	wmc.cleanupExpiredCache()
}

// cleanupExpiredCache removes expired entries from the cache.
func (wmc *WorkspaceAwareMetricsCollector) cleanupExpiredCache() {
	now := time.Now()
	for key, info := range wmc.clusterCache {
		if now.Sub(info.lastUpdate) > wmc.cacheExpiry {
			delete(wmc.clusterCache, key)
		}
	}
}