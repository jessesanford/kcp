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

// Package controller implements TMC cluster health monitoring and registration.
// This controller focuses on health checking physical clusters, monitoring their
// status, and providing detailed health metrics for TMC workload placement decisions.
package controller

import (
	"context"
	"fmt"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/util/workqueue"

	"github.com/kcp-dev/logicalcluster/v3"
	kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
)

// ClusterHealthController manages health monitoring for physical clusters.
// This controller focuses specifically on health checking, status monitoring,
// and providing detailed cluster health information for TMC operations.
type ClusterHealthController struct {
	// Core components
	queue workqueue.RateLimitingInterface
	
	// KCP client for future TMC API integration
	kcpClusterClient kcpclientset.ClusterInterface
	
	// Physical cluster clients for health monitoring
	clusterClients map[string]kubernetes.Interface
	
	// Configuration
	workspace    logicalcluster.Name
	resyncPeriod time.Duration
	workerCount  int
	
	// Health monitoring state
	clusterHealth map[string]*ClusterHealthStatus
}

// ClusterHealthStatus tracks comprehensive health information for a physical cluster
type ClusterHealthStatus struct {
	Name                   string
	LastCheck              time.Time
	Healthy                bool
	Error                  string
	NodeCount              int
	Version                string
	CPUCapacity            string
	MemoryCapacity         string
	SuccessiveHealthChecks int
	FailedHealthChecks     int
	AverageResponseTime    time.Duration
	APIServerEndpoint      string
	LastSuccessfulAPI      time.Time
}

// NewClusterHealthController creates a new cluster health monitoring controller.
// This controller provides comprehensive health monitoring for physical clusters
// and will integrate with TMC APIs for workload placement decisions.
func NewClusterHealthController(
	kcpClusterClient kcpclientset.ClusterInterface,
	clusterConfigs map[string]*rest.Config,
	workspace logicalcluster.Name,
	resyncPeriod time.Duration,
	workerCount int,
) (*ClusterHealthController, error) {
	
	if len(clusterConfigs) == 0 {
		return nil, fmt.Errorf("at least one cluster configuration is required")
	}
	
	// Build cluster clients
	clusterClients := make(map[string]kubernetes.Interface)
	clusterHealth := make(map[string]*ClusterHealthStatus)
	
	for name, config := range clusterConfigs {
		client, err := kubernetes.NewForConfig(config)
		if err != nil {
			return nil, fmt.Errorf("failed to create client for cluster %s: %w", name, err)
		}
		clusterClients[name] = client
		clusterHealth[name] = &ClusterHealthStatus{
			Name:    name,
			Healthy: false, // Start as unhealthy until first successful check
			APIServerEndpoint: config.Host,
		}
		
		klog.V(2).InfoS("Configured cluster health monitoring", "cluster", name, "endpoint", config.Host)
	}
	
	c := &ClusterHealthController{
		queue: workqueue.NewNamedRateLimitingQueue(
			workqueue.DefaultControllerRateLimiter(),
			"cluster-health-controller"),
		kcpClusterClient: kcpClusterClient,
		clusterClients:   clusterClients,
		workspace:        workspace,
		resyncPeriod:     resyncPeriod,
		workerCount:      workerCount,
		clusterHealth:    clusterHealth,
	}
	
	klog.InfoS("Created cluster health controller", "workspace", workspace, "clusters", len(clusterConfigs))
	
	return c, nil
}

// Start runs the cluster health monitoring controller.
// This starts comprehensive health monitoring for all configured clusters
// with detailed health metrics collection and status tracking.
func (c *ClusterHealthController) Start(ctx context.Context) error {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()
	
	klog.InfoS("Starting cluster health controller", "workspace", c.workspace, "clusters", len(c.clusterClients))
	defer klog.InfoS("Shutting down cluster health controller")
	
	// Start periodic health checks
	go c.startHealthMonitoring(ctx)
	
	// Start workers for processing health checks
	for i := 0; i < c.workerCount; i++ {
		go wait.UntilWithContext(ctx, c.runWorker, time.Second)
	}
	
	// Queue initial health checks for all clusters
	for clusterName := range c.clusterClients {
		c.queue.Add(clusterName)
	}
	
	<-ctx.Done()
	return nil
}

// runWorker processes health check requests from the workqueue
func (c *ClusterHealthController) runWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

// processNextWorkItem processes a single health check work item
func (c *ClusterHealthController) processNextWorkItem(ctx context.Context) bool {
	obj, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(obj)
	
	clusterName := obj.(string)
	if err := c.performDetailedHealthCheck(ctx, clusterName); err != nil {
		utilruntime.HandleError(fmt.Errorf("error performing health check for cluster %s: %w", clusterName, err))
		c.queue.AddRateLimited(obj)
		return true
	}
	
	c.queue.Forget(obj)
	return true
}

// performDetailedHealthCheck conducts comprehensive health monitoring for a cluster
func (c *ClusterHealthController) performDetailedHealthCheck(ctx context.Context, clusterName string) error {
	klog.V(4).InfoS("Performing detailed health check", "cluster", clusterName)
	
	client, exists := c.clusterClients[clusterName]
	if !exists {
		return fmt.Errorf("cluster client not found: %s", clusterName)
	}
	
	startTime := time.Now()
	health := c.clusterHealth[clusterName]
	
	// Initialize health check
	health.LastCheck = startTime
	
	// Test 1: API server connectivity and version
	version, err := client.Discovery().ServerVersion()
	if err != nil {
		c.recordFailedHealthCheck(health, fmt.Errorf("API server unreachable: %w", err))
		return err
	}
	
	// Test 2: Node inventory and capacity
	nodeList, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		c.recordFailedHealthCheck(health, fmt.Errorf("failed to list nodes: %w", err))
		return err
	}
	
	// Test 3: Namespace access (basic RBAC check)
	_, err = client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil {
		c.recordFailedHealthCheck(health, fmt.Errorf("namespace access denied: %w", err))
		return err
	}
	
	// Calculate response time
	responseTime := time.Since(startTime)
	
	// Update health status with detailed information
	c.updateHealthStatus(health, version.String(), nodeList, responseTime)
	
	klog.V(2).InfoS("Health check completed", "cluster", clusterName, "healthy", health.Healthy, "responseTime", responseTime)
	return nil
}

// updateHealthStatus updates the health status with detailed cluster information
func (c *ClusterHealthController) updateHealthStatus(health *ClusterHealthStatus, version string, nodeList *metav1.NodeList, responseTime time.Duration) {
	// Mark as healthy and update metrics
	health.Healthy = true
	health.Error = ""
	health.Version = version
	health.NodeCount = len(nodeList.Items)
	health.SuccessiveHealthChecks++
	health.LastSuccessfulAPI = time.Now()
	
	// Calculate average response time
	if health.AverageResponseTime == 0 {
		health.AverageResponseTime = responseTime
	} else {
		health.AverageResponseTime = (health.AverageResponseTime + responseTime) / 2
	}
	
	// Extract capacity information from nodes
	if len(nodeList.Items) > 0 {
		totalCPU := int64(0)
		totalMemory := int64(0)
		
		for _, node := range nodeList.Items {
			if cpu := node.Status.Capacity["cpu"]; !cpu.IsZero() {
				totalCPU += cpu.MilliValue()
			}
			if memory := node.Status.Capacity["memory"]; !memory.IsZero() {
				totalMemory += memory.Value()
			}
		}
		
		health.CPUCapacity = fmt.Sprintf("%dm", totalCPU)
		health.MemoryCapacity = fmt.Sprintf("%dGi", totalMemory/(1024*1024*1024))
	}
	
	klog.V(4).InfoS("Updated cluster health status", "cluster", health.Name, "nodes", health.NodeCount, "cpu", health.CPUCapacity, "memory", health.MemoryCapacity, "avgResponse", health.AverageResponseTime)
}

// recordFailedHealthCheck records a failed health check with error details
func (c *ClusterHealthController) recordFailedHealthCheck(health *ClusterHealthStatus, err error) {
	health.Healthy = false
	health.Error = err.Error()
	health.FailedHealthChecks++
	health.SuccessiveHealthChecks = 0
	
	klog.V(2).InfoS("Health check failed", "cluster", health.Name, "error", err.Error())
}

// startHealthMonitoring starts the periodic health monitoring loop
func (c *ClusterHealthController) startHealthMonitoring(ctx context.Context) {
	ticker := time.NewTicker(c.resyncPeriod)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Queue health checks for all clusters
			for clusterName := range c.clusterClients {
				c.queue.Add(clusterName)
			}
		}
	}
}

// GetClusterHealth returns the current comprehensive health status of a cluster
func (c *ClusterHealthController) GetClusterHealth(clusterName string) (*ClusterHealthStatus, bool) {
	health, exists := c.clusterHealth[clusterName]
	if !exists {
		return nil, false
	}
	
	// Return a deep copy to prevent race conditions
	return &ClusterHealthStatus{
		Name:                   health.Name,
		LastCheck:              health.LastCheck,
		Healthy:                health.Healthy,
		Error:                  health.Error,
		NodeCount:              health.NodeCount,
		Version:                health.Version,
		CPUCapacity:            health.CPUCapacity,
		MemoryCapacity:         health.MemoryCapacity,
		SuccessiveHealthChecks: health.SuccessiveHealthChecks,
		FailedHealthChecks:     health.FailedHealthChecks,
		AverageResponseTime:    health.AverageResponseTime,
		APIServerEndpoint:      health.APIServerEndpoint,
		LastSuccessfulAPI:      health.LastSuccessfulAPI,
	}, true
}

// GetAllClusterHealth returns comprehensive health status for all clusters
func (c *ClusterHealthController) GetAllClusterHealth() map[string]*ClusterHealthStatus {
	result := make(map[string]*ClusterHealthStatus)
	
	for name, health := range c.clusterHealth {
		result[name] = &ClusterHealthStatus{
			Name:                   health.Name,
			LastCheck:              health.LastCheck,
			Healthy:                health.Healthy,
			Error:                  health.Error,
			NodeCount:              health.NodeCount,
			Version:                health.Version,
			CPUCapacity:            health.CPUCapacity,
			MemoryCapacity:         health.MemoryCapacity,
			SuccessiveHealthChecks: health.SuccessiveHealthChecks,
			FailedHealthChecks:     health.FailedHealthChecks,
			AverageResponseTime:    health.AverageResponseTime,
			APIServerEndpoint:      health.APIServerEndpoint,
			LastSuccessfulAPI:      health.LastSuccessfulAPI,
		}
	}
	
	return result
}

// IsHealthy returns true if all monitored clusters are healthy
func (c *ClusterHealthController) IsHealthy() bool {
	for _, health := range c.clusterHealth {
		if !health.Healthy {
			return false
		}
	}
	return true
}

// GetHealthySummary returns a summary of cluster health status
func (c *ClusterHealthController) GetHealthSummary() map[string]interface{} {
	totalClusters := len(c.clusterHealth)
	healthyClusters := 0
	totalNodes := 0
	
	for _, health := range c.clusterHealth {
		if health.Healthy {
			healthyClusters++
		}
		totalNodes += health.NodeCount
	}
	
	return map[string]interface{}{
		"total_clusters":   totalClusters,
		"healthy_clusters": healthyClusters,
		"unhealthy_clusters": totalClusters - healthyClusters,
		"total_nodes":      totalNodes,
		"overall_healthy":  healthyClusters == totalClusters,
	}
}