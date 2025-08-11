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

package controller

import (
	"context"
	"fmt"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// performComprehensiveHealthCheck conducts detailed cluster health assessment
func (c *AdvancedClusterController) performComprehensiveHealthCheck(ctx context.Context, clusterName string, client kubernetes.Interface) (*AdvancedClusterHealthStatus, error) {
	startTime := time.Now()
	
	healthStatus := &AdvancedClusterHealthStatus{
		Name:      clusterName,
		LastCheck: startTime,
	}
	
	// Test 1: API Server connectivity and response time
	version, err := client.Discovery().ServerVersion()
	responseTime := time.Since(startTime).Milliseconds()
	
	if err != nil {
		healthStatus.Healthy = false
		healthStatus.Error = fmt.Sprintf("API server unreachable: %v", err)
		healthStatus.APIServerStatus = "Unreachable"
		healthStatus.Conditions = []ClusterHealthCondition{
			{
				Type:               "APIServerHealthy",
				Status:             metav1.ConditionFalse,
				Reason:             "Unreachable",
				Message:            fmt.Sprintf("API server unreachable: %v", err),
				LastTransitionTime: metav1.Now(),
			},
		}
		return healthStatus, nil
	}
	
	healthStatus.Version = version.String()
	healthStatus.APIServerStatus = "Healthy"
	
	// Test 2: Node availability and health
	nodeList, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		healthStatus.Healthy = false
		healthStatus.Error = fmt.Sprintf("Failed to list nodes: %v", err)
		healthStatus.Conditions = []ClusterHealthCondition{
			{
				Type:               "NodesAccessible",
				Status:             metav1.ConditionFalse,
				Reason:             "ListFailed",
				Message:            fmt.Sprintf("Failed to list nodes: %v", err),
				LastTransitionTime: metav1.Now(),
			},
		}
		return healthStatus, nil
	}
	
	healthStatus.NodeCount = len(nodeList.Items)
	
	// Count ready nodes
	readyNodes := 0
	for _, node := range nodeList.Items {
		for _, condition := range node.Status.Conditions {
			if condition.Type == "Ready" && condition.Status == "True" {
				readyNodes++
				break
			}
		}
	}
	
	// Test 3: Namespace access (basic RBAC test)
	namespaceList, err := client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{Limit: 5})
	if err != nil {
		klog.V(4).InfoS("Failed to list namespaces", "cluster", clusterName, "error", err)
		// This is not a critical failure, just log it
	}
	
	// Test 4: Pod access test
	podList, err := client.CoreV1().Pods("").List(ctx, metav1.ListOptions{Limit: 10})
	if err != nil {
		klog.V(4).InfoS("Failed to list pods", "cluster", clusterName, "error", err)
		// This is not a critical failure, just log it
	}
	
	// Determine overall health
	if readyNodes > 0 {
		healthStatus.Healthy = true
		healthStatus.ConsecutiveFailures = 0
		healthStatus.LastHealthyTime = startTime
		healthStatus.Error = ""
	} else {
		healthStatus.Healthy = false
		healthStatus.Error = fmt.Sprintf("No ready nodes found (total: %d)", healthStatus.NodeCount)
	}
	
	// Build conditions
	conditions := []ClusterHealthCondition{
		{
			Type:               "APIServerHealthy",
			Status:             metav1.ConditionTrue,
			Reason:             "Responding",
			Message:            fmt.Sprintf("API server responding in %dms", responseTime),
			LastTransitionTime: metav1.Now(),
		},
		{
			Type:   "NodesReady",
			Status: func() metav1.ConditionStatus {
				if readyNodes > 0 {
					return metav1.ConditionTrue
				}
				return metav1.ConditionFalse
			}(),
			Reason: func() string {
				if readyNodes > 0 {
					return "NodesAvailable"
				}
				return "NoReadyNodes"
			}(),
			Message:            fmt.Sprintf("%d of %d nodes ready", readyNodes, healthStatus.NodeCount),
			LastTransitionTime: metav1.Now(),
		},
	}
	
	// Add namespace condition if we got data
	if namespaceList != nil {
		conditions = append(conditions, ClusterHealthCondition{
			Type:               "NamespaceAccess",
			Status:             metav1.ConditionTrue,
			Reason:             "AccessGranted",
			Message:            fmt.Sprintf("Successfully listed %d namespaces", len(namespaceList.Items)),
			LastTransitionTime: metav1.Now(),
		})
	}
	
	// Add pod condition if we got data
	if podList != nil {
		conditions = append(conditions, ClusterHealthCondition{
			Type:               "PodAccess",
			Status:             metav1.ConditionTrue,
			Reason:             "AccessGranted",
			Message:            fmt.Sprintf("Successfully listed pods (%d found)", len(podList.Items)),
			LastTransitionTime: metav1.Now(),
		})
	}
	
	healthStatus.Conditions = conditions
	
	klog.V(4).InfoS("Comprehensive health check completed",
		"cluster", clusterName,
		"healthy", healthStatus.Healthy,
		"responseTime", responseTime,
		"nodes", healthStatus.NodeCount,
		"readyNodes", readyNodes,
		"version", healthStatus.Version)
	
	return healthStatus, nil
}

// collectClusterMetrics gathers performance and utilization metrics
func (c *AdvancedClusterController) collectClusterMetrics(ctx context.Context, clusterName string, client kubernetes.Interface) (*ClusterMetrics, error) {
	metrics := &ClusterMetrics{}
	
	// Collect pod count across all namespaces
	podList, err := client.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err == nil {
		metrics.PodCount = len(podList.Items)
	}
	
	// Collect service count
	serviceList, err := client.CoreV1().Services("").List(ctx, metav1.ListOptions{})
	if err == nil {
		metrics.ServiceCount = len(serviceList.Items)
	}
	
	// Collect namespace count
	namespaceList, err := client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err == nil {
		metrics.NamespaceCount = len(namespaceList.Items)
	}
	
	// Basic response time test
	start := time.Now()
	_, err = client.Discovery().ServerVersion()
	if err == nil {
		metrics.ResponseTime = time.Since(start).Milliseconds()
	}
	
	klog.V(4).InfoS("Collected cluster metrics",
		"cluster", clusterName,
		"pods", metrics.PodCount,
		"services", metrics.ServiceCount,
		"namespaces", metrics.NamespaceCount,
		"responseTime", metrics.ResponseTime)
	
	return metrics, nil
}

// detectClusterCapabilities determines what the cluster supports
func (c *AdvancedClusterController) detectClusterCapabilities(ctx context.Context, clusterName string, client kubernetes.Interface) ([]ClusterCapability, error) {
	var capabilities []ClusterCapability
	
	// Check for metrics server
	_, err := client.AppsV1().Deployments("kube-system").Get(ctx, "metrics-server", metav1.GetOptions{})
	capabilities = append(capabilities, ClusterCapability{
		Name:      "MetricsServer",
		Supported: err == nil,
	})
	
	// Check for ingress support
	ingressList, err := client.NetworkingV1().Ingresses("").List(ctx, metav1.ListOptions{Limit: 1})
	capabilities = append(capabilities, ClusterCapability{
		Name:      "Ingress",
		Supported: err == nil && ingressList != nil,
	})
	
	// Check for storage classes
	storageList, err := client.StorageV1().StorageClasses().List(ctx, metav1.ListOptions{Limit: 1})
	capabilities = append(capabilities, ClusterCapability{
		Name:      "StorageClasses",
		Supported: err == nil && len(storageList.Items) > 0,
	})
	
	// Check Kubernetes version capabilities
	version, err := client.Discovery().ServerVersion()
	if err == nil {
		capabilities = append(capabilities, ClusterCapability{
			Name:      "KubernetesAPI",
			Supported: true,
			Version:   version.String(),
		})
	}
	
	klog.V(4).InfoS("Detected cluster capabilities",
		"cluster", clusterName,
		"capabilities", len(capabilities))
	
	return capabilities, nil
}

// updateFailureStatus updates failure tracking for a cluster
func (c *AdvancedClusterController) updateFailureStatus(clusterName string, err error) {
	c.healthMutex.Lock()
	defer c.healthMutex.Unlock()
	
	if health, exists := c.clusterHealth[clusterName]; exists {
		health.Healthy = false
		health.Error = err.Error()
		health.LastCheck = time.Now()
		health.ConsecutiveFailures++
		
		// Add failure condition
		health.Conditions = append(health.Conditions, ClusterHealthCondition{
			Type:               "HealthCheckFailed",
			Status:             metav1.ConditionTrue,
			Reason:             "CheckFailed",
			Message:            fmt.Sprintf("Health check failed: %v", err),
			LastTransitionTime: metav1.Now(),
		})
		
		klog.V(4).InfoS("Updated failure status",
			"cluster", clusterName,
			"consecutiveFailures", health.ConsecutiveFailures,
			"error", err)
	}
}


// GetAdvancedClusterHealth returns comprehensive health status for a cluster
func (c *AdvancedClusterController) GetAdvancedClusterHealth(clusterName string) (*AdvancedClusterHealthStatus, bool) {
	c.healthMutex.RLock()
	defer c.healthMutex.RUnlock()
	
	health, exists := c.clusterHealth[clusterName]
	if !exists {
		return nil, false
	}
	
	// Return a deep copy to avoid race conditions
	return &AdvancedClusterHealthStatus{
		Name:                health.Name,
		Healthy:             health.Healthy,
		LastCheck:           health.LastCheck,
		Error:               health.Error,
		NodeCount:           health.NodeCount,
		Version:             health.Version,
		APIServerStatus:     health.APIServerStatus,
		Conditions:          append([]ClusterHealthCondition{}, health.Conditions...),
		Metrics:             health.Metrics,
		Capabilities:        append([]ClusterCapability{}, health.Capabilities...),
		ConsecutiveFailures: health.ConsecutiveFailures,
		LastHealthyTime:     health.LastHealthyTime,
	}, true
}

// GetAllAdvancedClusterHealth returns comprehensive health status for all clusters
func (c *AdvancedClusterController) GetAllAdvancedClusterHealth() map[string]*AdvancedClusterHealthStatus {
	c.healthMutex.RLock()
	defer c.healthMutex.RUnlock()
	
	result := make(map[string]*AdvancedClusterHealthStatus)
	
	for name, health := range c.clusterHealth {
		result[name] = &AdvancedClusterHealthStatus{
			Name:                health.Name,
			Healthy:             health.Healthy,
			LastCheck:           health.LastCheck,
			Error:               health.Error,
			NodeCount:           health.NodeCount,
			Version:             health.Version,
			APIServerStatus:     health.APIServerStatus,
			Conditions:          append([]ClusterHealthCondition{}, health.Conditions...),
			Metrics:             health.Metrics,
			Capabilities:        append([]ClusterCapability{}, health.Capabilities...),
			ConsecutiveFailures: health.ConsecutiveFailures,
			LastHealthyTime:     health.LastHealthyTime,
		}
	}
	
	return result
}

// IsClusterHealthy returns true if the cluster is healthy
func (c *AdvancedClusterController) IsClusterHealthy(clusterName string) bool {
	c.healthMutex.RLock()
	defer c.healthMutex.RUnlock()
	
	if health, exists := c.clusterHealth[clusterName]; exists {
		return health.Healthy && health.ConsecutiveFailures == 0
	}
	return false
}

// GetClusterMetrics returns the latest metrics for a cluster
func (c *AdvancedClusterController) GetClusterMetrics(clusterName string) (*ClusterMetrics, bool) {
	c.healthMutex.RLock()
	defer c.healthMutex.RUnlock()
	
	if health, exists := c.clusterHealth[clusterName]; exists && health.Metrics != nil {
		// Return copy of metrics
		return &ClusterMetrics{
			CPUUtilization:    health.Metrics.CPUUtilization,
			MemoryUtilization: health.Metrics.MemoryUtilization,
			PodCount:          health.Metrics.PodCount,
			ServiceCount:      health.Metrics.ServiceCount,
			NamespaceCount:    health.Metrics.NamespaceCount,
			ResponseTime:      health.Metrics.ResponseTime,
		}, true
	}
	return nil, false
}