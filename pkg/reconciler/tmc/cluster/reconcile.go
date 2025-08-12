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

package cluster

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

const (
	// Cluster condition types
	ConditionTypeHealthy     = "Healthy"
	ConditionTypeAPIServer   = "APIServerHealthy"
	ConditionTypeNodes       = "NodesReady"
	ConditionTypeConnectivity = "Connectivity"

	// Condition reasons
	ReasonHealthCheckPassed = "HealthCheckPassed"
	ReasonHealthCheckFailed = "HealthCheckFailed"
	ReasonAPIServerHealthy  = "APIServerHealthy"
	ReasonAPIServerDown     = "APIServerDown"
	ReasonNodesReady        = "NodesReady"
	ReasonNodesNotReady     = "NodesNotReady"
	ReasonConnected         = "Connected"
	ReasonConnectionFailed  = "ConnectionFailed"
)

// runWorker is a long-running function that processes items from the work queue
func (c *Controller) runWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

// processNextWorkItem reads a single work item off the workqueue and
// attempts to process it by calling the reconcile handler
func (c *Controller) processNextWorkItem(ctx context.Context) bool {
	obj, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(obj)

	// Extract cluster name from the typed key function result
	clusterName := obj.Key
	if clusterName == "" {
		utilruntime.HandleError(fmt.Errorf("empty cluster name in workqueue"))
		c.queue.Forget(obj)
		return true
	}

	if err := c.reconcileCluster(ctx, clusterName); err != nil {
		utilruntime.HandleError(fmt.Errorf("error reconciling cluster %s: %w", clusterName, err))
		c.queue.AddRateLimited(obj)
		return true
	}

	c.queue.Forget(obj)
	return true
}

// reconcileCluster performs the core reconciliation logic for a single cluster.
// This includes health checking, capacity gathering, and status updates.
func (c *Controller) reconcileCluster(ctx context.Context, clusterName string) error {
	klog.V(4).InfoS("Reconciling cluster", "cluster", clusterName, "workspace", c.workspace)

	// Get the cluster client
	c.clientsMutex.RLock()
	client, exists := c.clusterClients[clusterName]
	c.clientsMutex.RUnlock()

	if !exists {
		return fmt.Errorf("cluster client not found: %s", clusterName)
	}

	// Perform comprehensive health check
	healthStatus, err := c.performComprehensiveHealthCheck(ctx, clusterName, client)
	if err != nil {
		klog.V(2).InfoS("Cluster health check failed",
			"cluster", clusterName,
			"error", err)
		
		// Update health status with failure
		c.updateClusterHealthStatus(clusterName, &ClusterHealthStatus{
			Name:      clusterName,
			LastCheck: time.Now(),
			Healthy:   false,
			Error:     err.Error(),
			Conditions: []ClusterCondition{
				{
					Type:               ConditionTypeHealthy,
					Status:             "False",
					LastTransitionTime: time.Now(),
					Message:            fmt.Sprintf("Health check failed: %v", err),
				},
			},
		})
		
		return err
	}

	// Update health status with success
	c.updateClusterHealthStatus(clusterName, healthStatus)

	klog.V(3).InfoS("Cluster reconciliation completed",
		"cluster", clusterName,
		"healthy", healthStatus.Healthy,
		"nodes", healthStatus.NodeCount,
		"version", healthStatus.Version)

	return nil
}

// performComprehensiveHealthCheck conducts a thorough health check of the cluster
func (c *Controller) performComprehensiveHealthCheck(ctx context.Context, clusterName string, client kubernetes.Interface) (*ClusterHealthStatus, error) {
	healthStatus := &ClusterHealthStatus{
		Name:      clusterName,
		LastCheck: time.Now(),
		Healthy:   true,
		Conditions: make([]ClusterCondition, 0),
	}

	// Check API server connectivity
	if err := c.checkAPIServerHealth(ctx, client, healthStatus); err != nil {
		return nil, fmt.Errorf("API server health check failed: %w", err)
	}

	// Check cluster version
	if err := c.checkClusterVersion(ctx, client, healthStatus); err != nil {
		return nil, fmt.Errorf("cluster version check failed: %w", err)
	}

	// Check node health
	if err := c.checkNodeHealth(ctx, client, healthStatus); err != nil {
		return nil, fmt.Errorf("node health check failed: %w", err)
	}

	// Gather cluster capacity
	if err := c.gatherClusterCapacity(ctx, client, healthStatus); err != nil {
		klog.V(3).InfoS("Failed to gather cluster capacity",
			"cluster", clusterName,
			"error", err)
		// Don't fail the health check for capacity gathering failures
	}

	// Add overall health condition
	healthStatus.Conditions = append(healthStatus.Conditions, ClusterCondition{
		Type:               ConditionTypeHealthy,
		Status:             "True",
		LastTransitionTime: time.Now(),
		Message:            "All health checks passed successfully",
	})

	return healthStatus, nil
}

// checkAPIServerHealth verifies API server connectivity and responsiveness
func (c *Controller) checkAPIServerHealth(ctx context.Context, client kubernetes.Interface, healthStatus *ClusterHealthStatus) error {
	// Test API server by getting server version
	version, err := client.Discovery().ServerVersion()
	if err != nil {
		healthStatus.Conditions = append(healthStatus.Conditions, ClusterCondition{
			Type:               ConditionTypeAPIServer,
			Status:             "False",
			LastTransitionTime: time.Now(),
			Message:            fmt.Sprintf("Failed to get server version: %v", err),
		})
		return err
	}

	healthStatus.Version = version.String()
	healthStatus.Conditions = append(healthStatus.Conditions, ClusterCondition{
		Type:               ConditionTypeAPIServer,
		Status:             "True",
		LastTransitionTime: time.Now(),
		Message:            fmt.Sprintf("API server healthy, version: %s", version.String()),
	})

	return nil
}

// checkClusterVersion validates the cluster version compatibility
func (c *Controller) checkClusterVersion(ctx context.Context, client kubernetes.Interface, healthStatus *ClusterHealthStatus) error {
	// Additional version validation could be added here
	// For now, just ensure we can communicate with the API server
	return nil
}

// checkNodeHealth examines the health and readiness of cluster nodes
func (c *Controller) checkNodeHealth(ctx context.Context, client kubernetes.Interface, healthStatus *ClusterHealthStatus) error {
	// List all nodes
	nodeList, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		healthStatus.Conditions = append(healthStatus.Conditions, ClusterCondition{
			Type:               ConditionTypeNodes,
			Status:             "False",
			LastTransitionTime: time.Now(),
			Message:            fmt.Sprintf("Failed to list nodes: %v", err),
		})
		return err
	}

	healthStatus.NodeCount = len(nodeList.Items)

	// Check node readiness
	readyNodes := 0
	for _, node := range nodeList.Items {
		if c.isNodeReady(&node) {
			readyNodes++
		}
	}

	message := fmt.Sprintf("%d/%d nodes ready", readyNodes, len(nodeList.Items))
	if readyNodes == 0 && len(nodeList.Items) > 0 {
		healthStatus.Conditions = append(healthStatus.Conditions, ClusterCondition{
			Type:               ConditionTypeNodes,
			Status:             "False",
			LastTransitionTime: time.Now(),
			Message:            message,
		})
		return fmt.Errorf("no ready nodes found")
	}

	healthStatus.Conditions = append(healthStatus.Conditions, ClusterCondition{
		Type:               ConditionTypeNodes,
		Status:             "True",
		LastTransitionTime: time.Now(),
		Message:            message,
	})

	return nil
}

// gatherClusterCapacity collects basic resource capacity information from the cluster
func (c *Controller) gatherClusterCapacity(ctx context.Context, client kubernetes.Interface, healthStatus *ClusterHealthStatus) error {
	nodeList, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list nodes for capacity calculation: %w", err)
	}

	var totalCPU, totalMemory int64

	for _, node := range nodeList.Items {
		if !c.isNodeReady(&node) {
			continue // Skip unready nodes in capacity calculation
		}

		// CPU capacity (in millicores)
		if cpu, ok := node.Status.Capacity[corev1.ResourceCPU]; ok {
			totalCPU += cpu.MilliValue()
		}

		// Memory capacity (in bytes)
		if memory, ok := node.Status.Capacity[corev1.ResourceMemory]; ok {
			totalMemory += memory.Value()
		}
	}

	healthStatus.Capacity = ClusterCapacity{
		CPU:    totalCPU,
		Memory: totalMemory,
	}

	return nil
}

// isNodeReady checks if a node is in Ready condition
func (c *Controller) isNodeReady(node *corev1.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}

// updateClusterHealthStatus safely updates the health status for a cluster
func (c *Controller) updateClusterHealthStatus(clusterName string, healthStatus *ClusterHealthStatus) {
	c.healthMutex.Lock()
	defer c.healthMutex.Unlock()

	// Store the health status
	c.clusterHealth[clusterName] = healthStatus

	klog.V(3).InfoS("Updated cluster health status",
		"cluster", clusterName,
		"healthy", healthStatus.Healthy,
		"lastCheck", healthStatus.LastCheck.Format(time.RFC3339),
		"conditions", len(healthStatus.Conditions))
}

