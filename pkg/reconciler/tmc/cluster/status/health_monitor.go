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

package status

import (
	"context"
	"fmt"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	
	conditionsapi "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
	tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
)

// HealthMonitor provides health monitoring capabilities for TMC cluster resources.
// It performs health checks and generates appropriate status conditions based on
// cluster connectivity, resource availability, and performance metrics.
type HealthMonitor struct {
	// Configuration
	heartbeatTimeout time.Duration
	connectionTimeout time.Duration
	healthCheckInterval time.Duration
	
	// State tracking
	lastHeartbeats map[string]time.Time
	connectionLatency map[string]time.Duration
}

// NewHealthMonitor creates a new health monitor with the specified configuration.
func NewHealthMonitor(heartbeatTimeout, connectionTimeout, healthCheckInterval time.Duration) *HealthMonitor {
	return &HealthMonitor{
		heartbeatTimeout:     heartbeatTimeout,
		connectionTimeout:    connectionTimeout,
		healthCheckInterval:  healthCheckInterval,
		lastHeartbeats:      make(map[string]time.Time),
		connectionLatency:   make(map[string]time.Duration),
	}
}

// DefaultHealthMonitor creates a health monitor with sensible defaults.
func DefaultHealthMonitor() *HealthMonitor {
	return NewHealthMonitor(
		5*time.Minute,  // heartbeat timeout
		30*time.Second, // connection timeout
		1*time.Minute,  // health check interval
	)
}

// HealthCheckResult represents the result of a health check operation.
type HealthCheckResult struct {
	ClusterName string
	Healthy     bool
	Error       error
	Latency     time.Duration
	NodeCount   int32
	Version     string
	Conditions  []conditionsapi.Condition
	Metrics     *tmcv1alpha1.ClusterHealthMetrics
}

// PerformHealthCheck executes a comprehensive health check for the specified cluster.
// This method tests connectivity, gathers metrics, and generates appropriate conditions.
func (hm *HealthMonitor) PerformHealthCheck(ctx context.Context, clusterName string, client kubernetes.Interface) *HealthCheckResult {
	logger := klog.FromContext(ctx).WithValues("cluster", clusterName)
	startTime := time.Now()
	
	result := &HealthCheckResult{
		ClusterName: clusterName,
		Healthy:     false,
		Conditions:  []conditionsapi.Condition{},
	}
	
	// Test 1: Basic connectivity check
	connectivityResult := hm.checkConnectivity(ctx, clusterName, client)
	result.Conditions = append(result.Conditions, connectivityResult.conditions...)
	result.Latency = time.Since(startTime)
	
	if connectivityResult.err != nil {
		result.Error = connectivityResult.err
		logger.V(2).Info("Connectivity check failed", "error", connectivityResult.err)
		return result
	}
	
	// Test 2: Gather cluster metrics
	metricsResult := hm.gatherClusterMetrics(ctx, clusterName, client)
	result.Conditions = append(result.Conditions, metricsResult.conditions...)
	result.NodeCount = metricsResult.nodeCount
	result.Version = metricsResult.version
	result.Metrics = metricsResult.metrics
	
	if metricsResult.err != nil {
		logger.V(2).Info("Metrics gathering had issues", "error", metricsResult.err)
		// Don't fail the entire health check for metrics issues
	}
	
	// Test 3: Check resource availability
	resourceResult := hm.checkResourceAvailability(ctx, clusterName, client, metricsResult.metrics)
	result.Conditions = append(result.Conditions, resourceResult.conditions...)
	
	// Test 4: Update heartbeat tracking
	hm.updateHeartbeat(clusterName)
	heartbeatConditions := hm.checkHeartbeatStatus(clusterName)
	result.Conditions = append(result.Conditions, heartbeatConditions...)
	
	// Store latency for tracking
	hm.connectionLatency[clusterName] = result.Latency
	
	// Determine overall health based on critical conditions
	result.Healthy = !HasCriticalConditionError(result.Conditions)
	
	logger.V(3).Info("Health check completed",
		"healthy", result.Healthy,
		"latency", result.Latency,
		"nodeCount", result.NodeCount,
		"conditions", len(result.Conditions))
		
	return result
}

type connectivityResult struct {
	conditions []conditionsapi.Condition
	err        error
}

func (hm *HealthMonitor) checkConnectivity(ctx context.Context, clusterName string, client kubernetes.Interface) *connectivityResult {
	ctx, cancel := context.WithTimeout(ctx, hm.connectionTimeout)
	defer cancel()
	
	// Test basic API server connectivity
	_, err := client.Discovery().ServerVersion()
	if err != nil {
		return &connectivityResult{
			conditions: []conditionsapi.Condition{
				*ClusterDisconnectedCondition(fmt.Sprintf("Failed to connect to cluster: %v", err)),
			},
			err: fmt.Errorf("connectivity check failed: %w", err),
		}
	}
	
	return &connectivityResult{
		conditions: []conditionsapi.Condition{
			*ClusterConnectedCondition("Cluster is reachable and responsive"),
		},
	}
}

type metricsResult struct {
	conditions []conditionsapi.Condition
	nodeCount  int32
	version    string
	metrics    *tmcv1alpha1.ClusterHealthMetrics
	err        error
}

func (hm *HealthMonitor) gatherClusterMetrics(ctx context.Context, clusterName string, client kubernetes.Interface) *metricsResult {
	result := &metricsResult{
		conditions: []conditionsapi.Condition{},
		metrics:    &tmcv1alpha1.ClusterHealthMetrics{},
	}
	
	// Get cluster version
	version, err := client.Discovery().ServerVersion()
	if err != nil {
		result.err = fmt.Errorf("failed to get server version: %w", err)
	} else {
		result.version = version.String()
	}
	
	// Get node information
	nodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		result.conditions = append(result.conditions, 
			*NewConditionBuilder(tmcv1alpha1.CapabilitiesDetectedCondition).
				WithStatus("False").
				WithReason(tmcv1alpha1.CapabilitiesDetectionFailedReason).
				WithMessage(fmt.Sprintf("Failed to list nodes: %v", err)).
				WithSeverity(conditionsapi.ConditionSeverityWarning).
				Build())
		result.err = fmt.Errorf("failed to list nodes: %w", err)
		return result
	}
	
	result.nodeCount = int32(len(nodes.Items))
	
	// Count ready nodes
	readyNodes := int32(0)
	for _, node := range nodes.Items {
		for _, condition := range node.Status.Conditions {
			if condition.Type == "Ready" && condition.Status == "True" {
				readyNodes++
				break
			}
		}
	}
	
	// Calculate health score (simple algorithm)
	healthScore := int32(0)
	if result.nodeCount > 0 {
		healthScore = (readyNodes * 100) / result.nodeCount
	}
	
	// Update metrics
	result.metrics.NodeTotalCount = &result.nodeCount
	result.metrics.NodeReadyCount = &readyNodes
	result.metrics.HealthScore = &healthScore
	now := metav1.NewTime(time.Now())
	result.metrics.LastMetricsUpdate = &now
	
	result.conditions = append(result.conditions,
		*NewConditionBuilder(tmcv1alpha1.CapabilitiesDetectedCondition).
			WithStatus("True").
			WithReason(tmcv1alpha1.CapabilitiesDetectedReason).
			WithMessage(fmt.Sprintf("Detected %d nodes, %d ready", result.nodeCount, readyNodes)).
			WithSeverity(conditionsapi.ConditionSeverityInfo).
			Build())
	
	return result
}

type resourceResult struct {
	conditions []conditionsapi.Condition
}

func (hm *HealthMonitor) checkResourceAvailability(ctx context.Context, clusterName string, client kubernetes.Interface, metrics *tmcv1alpha1.ClusterHealthMetrics) *resourceResult {
	result := &resourceResult{
		conditions: []conditionsapi.Condition{},
	}
	
	// Simple resource availability check based on node readiness
	if metrics.NodeReadyCount != nil && metrics.NodeTotalCount != nil {
		readyRatio := float64(*metrics.NodeReadyCount) / float64(*metrics.NodeTotalCount)
		
		if readyRatio >= 0.8 { // 80% or more nodes ready
			result.conditions = append(result.conditions,
				*ResourcesAvailableCondition(fmt.Sprintf("Cluster has sufficient resources: %d/%d nodes ready", 
					*metrics.NodeReadyCount, *metrics.NodeTotalCount)))
		} else if readyRatio >= 0.5 { // 50-79% nodes ready
			result.conditions = append(result.conditions,
				*ResourcesInsufficientCondition(fmt.Sprintf("Cluster has limited resources: %d/%d nodes ready", 
					*metrics.NodeReadyCount, *metrics.NodeTotalCount)))
		} else { // Less than 50% nodes ready
			result.conditions = append(result.conditions,
				*NewConditionBuilder(tmcv1alpha1.ResourcesAvailableCondition).
					WithStatus("False").
					WithReason(tmcv1alpha1.ResourcesUnavailableReason).
					WithMessage(fmt.Sprintf("Cluster has insufficient resources: %d/%d nodes ready", 
						*metrics.NodeReadyCount, *metrics.NodeTotalCount)).
					WithSeverity(conditionsapi.ConditionSeverityError).
					Build())
		}
	}
	
	return result
}

// updateHeartbeat records a successful heartbeat for the specified cluster.
func (hm *HealthMonitor) updateHeartbeat(clusterName string) {
	hm.lastHeartbeats[clusterName] = time.Now()
}

// checkHeartbeatStatus evaluates heartbeat health for the specified cluster.
func (hm *HealthMonitor) checkHeartbeatStatus(clusterName string) []conditionsapi.Condition {
	lastHeartbeat, exists := hm.lastHeartbeats[clusterName]
	if !exists {
		return []conditionsapi.Condition{
			*HeartbeatUnhealthyCondition(tmcv1alpha1.HeartbeatMissedReason, "No heartbeat recorded yet"),
		}
	}
	
	timeSinceHeartbeat := time.Since(lastHeartbeat)
	
	if timeSinceHeartbeat > hm.heartbeatTimeout {
		return []conditionsapi.Condition{
			*HeartbeatUnhealthyCondition(tmcv1alpha1.HeartbeatMissedReason,
				fmt.Sprintf("Heartbeat missed for %v", timeSinceHeartbeat)),
		}
	} else if timeSinceHeartbeat > hm.heartbeatTimeout/2 {
		return []conditionsapi.Condition{
			*HeartbeatUnhealthyCondition(tmcv1alpha1.HeartbeatStaleReason,
				fmt.Sprintf("Heartbeat is getting stale: %v ago", timeSinceHeartbeat)),
		}
	}
	
	return []conditionsapi.Condition{
		*HeartbeatHealthyCondition(fmt.Sprintf("Heartbeat is healthy, last seen %v ago", timeSinceHeartbeat)),
	}
}

// GetConnectionLatency returns the last recorded connection latency for a cluster.
func (hm *HealthMonitor) GetConnectionLatency(clusterName string) (time.Duration, bool) {
	latency, exists := hm.connectionLatency[clusterName]
	return latency, exists
}

// GetLastHeartbeat returns the timestamp of the last heartbeat for a cluster.
func (hm *HealthMonitor) GetLastHeartbeat(clusterName string) (time.Time, bool) {
	heartbeat, exists := hm.lastHeartbeats[clusterName]
	return heartbeat, exists
}

// IsClusterHealthy performs a quick health check based on recent heartbeat and connection status.
func (hm *HealthMonitor) IsClusterHealthy(clusterName string) bool {
	// Check heartbeat recency
	heartbeatConditions := hm.checkHeartbeatStatus(clusterName)
	return !HasCriticalConditionError(heartbeatConditions)
}