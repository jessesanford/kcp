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

package upstream

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"

	workloadv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"
	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
)

// ResourceHealth represents the health status of a resource
type ResourceHealth string

const (
	HealthHealthy   ResourceHealth = "Healthy"
	HealthUnhealthy ResourceHealth = "Unhealthy"
	HealthUnknown   ResourceHealth = "Unknown"
	HealthPending   ResourceHealth = "Pending"
)

// statusAggregator handles status aggregation from physical cluster resources
type statusAggregator struct {
	// Resource health tracking
	resourceHealth map[string]ResourceHealth
	
	// Last update times
	lastStatusUpdate map[string]time.Time
}

// newStatusAggregator creates a new status aggregator
func newStatusAggregator() *statusAggregator {
	return &statusAggregator{
		resourceHealth:   make(map[string]ResourceHealth),
		lastStatusUpdate: make(map[string]time.Time),
	}
}

// aggregateResourceStatus aggregates status from physical cluster resources
func (sa *statusAggregator) aggregateResourceStatus(ctx context.Context, resources []unstructured.Unstructured, gvr schema.GroupVersionResource) ResourceHealth {
	logger := klog.FromContext(ctx)
	
	if len(resources) == 0 {
		return HealthUnknown
	}
	
	var healthyCount, unhealthyCount, pendingCount int
	
	for _, resource := range resources {
		health := sa.determineResourceHealth(resource, gvr)
		
		switch health {
		case HealthHealthy:
			healthyCount++
		case HealthUnhealthy:
			unhealthyCount++
		case HealthPending:
			pendingCount++
		}
	}
	
	// Aggregate health logic
	totalCount := len(resources)
	if unhealthyCount > 0 {
		logger.V(4).Info("Resources with unhealthy status detected",
			"resource", gvr.String(),
			"unhealthy", unhealthyCount,
			"total", totalCount)
		return HealthUnhealthy
	}
	
	if pendingCount == totalCount {
		return HealthPending
	}
	
	if healthyCount == totalCount {
		return HealthHealthy
	}
	
	// Mixed states - consider as pending
	return HealthPending
}

// determineResourceHealth determines the health of an individual resource
func (sa *statusAggregator) determineResourceHealth(resource unstructured.Unstructured, gvr schema.GroupVersionResource) ResourceHealth {
	switch gvr.Resource {
	case "pods":
		return sa.determinePodHealth(resource)
	case "deployments":
		return sa.determineDeploymentHealth(resource)
	case "statefulsets":
		return sa.determineStatefulSetHealth(resource)
	case "services":
		return sa.determineServiceHealth(resource)
	default:
		return sa.determineGenericHealth(resource)
	}
}

// determinePodHealth determines Pod health based on phase and conditions
func (sa *statusAggregator) determinePodHealth(resource unstructured.Unstructured) ResourceHealth {
	phase, found, err := unstructured.NestedString(resource.Object, "status", "phase")
	if !found || err != nil {
		return HealthUnknown
	}
	
	switch phase {
	case "Running":
		// Check if all containers are ready
		if sa.areContainersReady(resource) {
			return HealthHealthy
		}
		return HealthPending
	case "Succeeded":
		return HealthHealthy
	case "Failed":
		return HealthUnhealthy
	case "Pending":
		return HealthPending
	default:
		return HealthUnknown
	}
}

// determineDeploymentHealth determines Deployment health based on status
func (sa *statusAggregator) determineDeploymentHealth(resource unstructured.Unstructured) ResourceHealth {
	replicas, found, err := unstructured.NestedInt64(resource.Object, "spec", "replicas")
	if !found || err != nil {
		return HealthUnknown
	}
	
	readyReplicas, found, err := unstructured.NestedInt64(resource.Object, "status", "readyReplicas")
	if !found || err != nil {
		return HealthPending
	}
	
	if readyReplicas == replicas {
		return HealthHealthy
	} else if readyReplicas == 0 {
		return HealthUnhealthy
	}
	
	return HealthPending
}

// determineStatefulSetHealth determines StatefulSet health
func (sa *statusAggregator) determineStatefulSetHealth(resource unstructured.Unstructured) ResourceHealth {
	replicas, found, err := unstructured.NestedInt64(resource.Object, "spec", "replicas")
	if !found || err != nil {
		return HealthUnknown
	}
	
	readyReplicas, found, err := unstructured.NestedInt64(resource.Object, "status", "readyReplicas")
	if !found || err != nil {
		return HealthPending
	}
	
	if readyReplicas == replicas {
		return HealthHealthy
	} else if readyReplicas == 0 {
		return HealthUnhealthy
	}
	
	return HealthPending
}

// determineServiceHealth determines Service health
func (sa *statusAggregator) determineServiceHealth(resource unstructured.Unstructured) ResourceHealth {
	// Services are generally healthy if they exist
	// Could check for endpoint readiness in more sophisticated implementation
	return HealthHealthy
}

// determineGenericHealth provides generic health determination for other resources
func (sa *statusAggregator) determineGenericHealth(resource unstructured.Unstructured) ResourceHealth {
	// Check for common status conditions
	conditions, found, err := unstructured.NestedSlice(resource.Object, "status", "conditions")
	if !found || err != nil {
		return HealthUnknown
	}
	
	for _, conditionInterface := range conditions {
		condition, ok := conditionInterface.(map[string]interface{})
		if !ok {
			continue
		}
		
		conditionType, _, _ := unstructured.NestedString(condition, "type")
		conditionStatus, _, _ := unstructured.NestedString(condition, "status")
		
		// Look for Ready condition
		if conditionType == "Ready" {
			if conditionStatus == "True" {
				return HealthHealthy
			} else if conditionStatus == "False" {
				return HealthUnhealthy
			}
		}
	}
	
	return HealthUnknown
}

// areContainersReady checks if all containers in a pod are ready
func (sa *statusAggregator) areContainersReady(resource unstructured.Unstructured) bool {
	containerStatuses, found, err := unstructured.NestedSlice(resource.Object, "status", "containerStatuses")
	if !found || err != nil {
		return false
	}
	
	for _, statusInterface := range containerStatuses {
		status, ok := statusInterface.(map[string]interface{})
		if !ok {
			continue
		}
		
		ready, found, err := unstructured.NestedBool(status, "ready")
		if !found || err != nil || !ready {
			return false
		}
	}
	
	return true
}

// updateSyncTargetHealth updates the SyncTarget status with aggregated health information
func (sa *statusAggregator) updateSyncTargetHealth(syncTarget *workloadv1alpha1.SyncTarget, overallHealth ResourceHealth, resourceCounts map[string]int) {
	now := time.Now()
	
	// Update resource counts
	if resourceCounts != nil {
		syncTarget.Status.WorkloadCount = int32(resourceCounts["total"])
	}
	
	// Update health-based conditions
	conditions := syncTarget.Status.Conditions
	
	// Update or add SyncerReady condition based on health
	syncerReadyCondition := conditionsv1alpha1.Condition{
		Type:               workloadv1alpha1.SyncTargetSyncerReady,
		LastTransitionTime: metav1.NewTime(now),
	}
	
	switch overallHealth {
	case HealthHealthy:
		syncerReadyCondition.Status = metav1.ConditionTrue
		syncerReadyCondition.Reason = "SyncHealthy"
		syncerReadyCondition.Message = "All synced resources are healthy"
	case HealthUnhealthy:
		syncerReadyCondition.Status = metav1.ConditionFalse
		syncerReadyCondition.Reason = "SyncUnhealthy" 
		syncerReadyCondition.Message = "Some synced resources are unhealthy"
	case HealthPending:
		syncerReadyCondition.Status = metav1.ConditionUnknown
		syncerReadyCondition.Reason = "SyncPending"
		syncerReadyCondition.Message = "Synced resources are in pending state"
	default:
		syncerReadyCondition.Status = metav1.ConditionUnknown
		syncerReadyCondition.Reason = "SyncStatusUnknown"
		syncerReadyCondition.Message = "Unable to determine sync health"
	}
	
	// Update conditions
	conditions.SetCondition(syncerReadyCondition)
	syncTarget.Status.Conditions = conditions
	
	// Update last sync time
	syncTarget.Status.LastSyncTime = &metav1.NewTime(now)
}