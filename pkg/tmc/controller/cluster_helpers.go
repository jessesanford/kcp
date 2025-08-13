// Copyright The KCP Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package controller contains helper utilities for TMC cluster management.
// This package provides utility functions and helper methods that support
// the main cluster registration controller functionality.
package controller

import (
	"time"
)

// ClusterHealthStatus tracks the health of a physical cluster.
// This status information is used by TMC controllers to make
// placement and management decisions for workloads.
type ClusterHealthStatus struct {
	// Name of the cluster
	Name string

	// LastCheck time of last health check
	LastCheck time.Time

	// Healthy indicates if the cluster is healthy
	Healthy bool

	// Error message if unhealthy
	Error string

	// NodeCount from the latest health check
	NodeCount int

	// Version of the Kubernetes cluster
	Version string
}

// ClusterHealthHelper provides utility methods for managing cluster health status.
// This helper encapsulates common operations for cluster health management
// that can be shared across multiple controller implementations.
type ClusterHealthHelper struct {
	// clusterHealth maps cluster names to their health status
	clusterHealth map[string]*ClusterHealthStatus
}

// NewClusterHealthHelper creates a new cluster health helper.
// This helper provides utility methods for managing cluster health
// status across multiple clusters in a TMC deployment.
//
// Returns:
//   - *ClusterHealthHelper: Initialized helper instance
func NewClusterHealthHelper() *ClusterHealthHelper {
	return &ClusterHealthHelper{
		clusterHealth: make(map[string]*ClusterHealthStatus),
	}
}

// SetClusterHealth updates the health status for a specific cluster.
// This method is used by controllers to update cluster health information
// after performing health checks or receiving status updates.
//
// Parameters:
//   - clusterName: Name of the cluster to update
//   - status: New health status information
func (h *ClusterHealthHelper) SetClusterHealth(clusterName string, status *ClusterHealthStatus) {
	if status == nil {
		return
	}

	// Create a copy to avoid external modification
	h.clusterHealth[clusterName] = &ClusterHealthStatus{
		Name:      status.Name,
		LastCheck: status.LastCheck,
		Healthy:   status.Healthy,
		Error:     status.Error,
		NodeCount: status.NodeCount,
		Version:   status.Version,
	}
}

// GetClusterHealth returns the current health status of a cluster.
// This method provides read-only access to cluster health information
// and returns a copy to prevent external modification of the internal state.
//
// Parameters:
//   - clusterName: Name of the cluster to query
//
// Returns:
//   - *ClusterHealthStatus: Health status information for the cluster
//   - bool: true if the cluster health status exists, false otherwise
func (h *ClusterHealthHelper) GetClusterHealth(clusterName string) (*ClusterHealthStatus, bool) {
	health, exists := h.clusterHealth[clusterName]
	if !exists {
		return nil, false
	}

	// Return a copy to avoid race conditions and external modification
	return &ClusterHealthStatus{
		Name:      health.Name,
		LastCheck: health.LastCheck,
		Healthy:   health.Healthy,
		Error:     health.Error,
		NodeCount: health.NodeCount,
		Version:   health.Version,
	}, true
}

// GetAllClusterHealth returns health status for all clusters.
// This method provides a snapshot of all cluster health information
// and returns copies to prevent external modification of the internal state.
//
// Returns:
//   - map[string]*ClusterHealthStatus: Map of cluster names to health status
func (h *ClusterHealthHelper) GetAllClusterHealth() map[string]*ClusterHealthStatus {
	result := make(map[string]*ClusterHealthStatus)

	for name, health := range h.clusterHealth {
		result[name] = &ClusterHealthStatus{
			Name:      health.Name,
			LastCheck: health.LastCheck,
			Healthy:   health.Healthy,
			Error:     health.Error,
			NodeCount: health.NodeCount,
			Version:   health.Version,
		}
	}

	return result
}

// IsHealthy returns true if all clusters are healthy.
// This method provides a quick way to check if the entire cluster
// fleet is in a healthy state, which is useful for overall system
// health monitoring and readiness checks.
//
// Returns:
//   - bool: true if all clusters are healthy, false if any cluster is unhealthy
func (h *ClusterHealthHelper) IsHealthy() bool {
	for _, health := range h.clusterHealth {
		if !health.Healthy {
			return false
		}
	}
	return true
}

// GetHealthyClusterCount returns the number of healthy clusters.
// This method provides metrics about cluster health for monitoring
// and alerting purposes.
//
// Returns:
//   - int: Number of clusters currently marked as healthy
func (h *ClusterHealthHelper) GetHealthyClusterCount() int {
	count := 0
	for _, health := range h.clusterHealth {
		if health.Healthy {
			count++
		}
	}
	return count
}

// GetUnhealthyClusterCount returns the number of unhealthy clusters.
// This method provides metrics about cluster health for monitoring
// and alerting purposes.
//
// Returns:
//   - int: Number of clusters currently marked as unhealthy
func (h *ClusterHealthHelper) GetUnhealthyClusterCount() int {
	count := 0
	for _, health := range h.clusterHealth {
		if !health.Healthy {
			count++
		}
	}
	return count
}

// GetTotalClusterCount returns the total number of tracked clusters.
// This method provides the total count of all clusters being monitored
// by this helper instance.
//
// Returns:
//   - int: Total number of clusters being tracked
func (h *ClusterHealthHelper) GetTotalClusterCount() int {
	return len(h.clusterHealth)
}

// RemoveCluster removes a cluster from health tracking.
// This method is used when a cluster is deregistered or no longer
// needs to be monitored for health status.
//
// Parameters:
//   - clusterName: Name of the cluster to remove from tracking
func (h *ClusterHealthHelper) RemoveCluster(clusterName string) {
	delete(h.clusterHealth, clusterName)
}

// ClearAll removes all cluster health information.
// This method is used for cleanup or reset operations.
func (h *ClusterHealthHelper) ClearAll() {
	h.clusterHealth = make(map[string]*ClusterHealthStatus)
}
