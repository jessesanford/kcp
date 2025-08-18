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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

// StatusCollector defines the interface for collecting status from sync targets
type StatusCollector interface {
	// CollectStatus collects status from a single sync target
	CollectStatus(ctx context.Context, target SyncTarget) (TargetStatus, error)
}

// collector implements StatusCollector interface
type collector struct {
	// collectionTimeout defines the timeout for individual status collection
	collectionTimeout time.Duration
}

// NewStatusCollector creates a new status collector
func NewStatusCollector() StatusCollector {
	return &collector{
		collectionTimeout: 10 * time.Second, // Default timeout per target
	}
}

// CollectStatus implements StatusCollector.CollectStatus
func (c *collector) CollectStatus(ctx context.Context, target SyncTarget) (TargetStatus, error) {
	logger := klog.FromContext(ctx).WithValues(
		"syncTarget", target.Name,
		"workspace", target.Workspace,
	)
	
	logger.V(4).Info("Collecting status from sync target")
	
	// Create timeout context for this collection
	collectCtx, cancel := context.WithTimeout(ctx, c.collectionTimeout)
	defer cancel()
	
	status := TargetStatus{
		Target:      target,
		Health:      HealthStatusUnknown,
		LastUpdated: metav1.Now(),
		Conditions:  []metav1.Condition{},
	}
	
	// Simulate sync target availability check
	if err := c.checkSyncTargetAvailability(collectCtx, target, &status); err != nil {
		logger.Error(err, "Failed to check sync target availability")
		status.Error = err
		status.Health = HealthStatusUnhealthy
		status.Conditions = append(status.Conditions, metav1.Condition{
			Type:    "SyncTargetAvailable",
			Status:  metav1.ConditionFalse,
			Reason:  "CheckFailed",
			Message: fmt.Sprintf("Failed to check sync target: %v", err),
			LastTransitionTime: metav1.Now(),
		})
		return status, nil // Return status with error rather than failing
	}
	
	// Collect resource status from sync target
	if err := c.collectResourceStatus(collectCtx, target, &status); err != nil {
		logger.Error(err, "Failed to collect resource status")
		// Don't fail the entire status collection, just log the error
		status.Conditions = append(status.Conditions, metav1.Condition{
			Type:    "ResourceStatusCollected",
			Status:  metav1.ConditionFalse,
			Reason:  "CollectionFailed",
			Message: fmt.Sprintf("Failed to collect resource status: %v", err),
			LastTransitionTime: metav1.Now(),
		})
	}
	
	logger.V(4).Info("Status collection completed",
		"health", status.Health,
		"resourceCount", status.ResourceCount,
		"readyResources", status.ReadyResources,
	)
	
	return status, nil
}

// checkSyncTargetAvailability simulates checking sync target availability
func (c *collector) checkSyncTargetAvailability(ctx context.Context, target SyncTarget, status *TargetStatus) error {
	// Simulate sync target health check
	// In a real implementation, this would check the actual sync target endpoint
	
	// Default to healthy for simulation
	status.Health = HealthStatusHealthy
	
	// Simulate various health conditions based on target name
	switch {
	case target.Name == "unavailable-target":
		status.Health = HealthStatusUnhealthy
		status.Conditions = append(status.Conditions, metav1.Condition{
			Type:    "SyncTargetReady",
			Status:  metav1.ConditionFalse,
			Reason:  "Unavailable",
			Message: "Sync target is unavailable",
			LastTransitionTime: metav1.Now(),
		})
		return fmt.Errorf("sync target %s is unavailable", target.Name)
		
	case target.Name == "degraded-target":
		status.Health = HealthStatusDegraded
		status.Conditions = append(status.Conditions, metav1.Condition{
			Type:    "SyncTargetReady",
			Status:  metav1.ConditionTrue,
			Reason:  "Ready",
			Message: "Sync target is ready but degraded",
			LastTransitionTime: metav1.Now(),
		})
		status.Conditions = append(status.Conditions, metav1.Condition{
			Type:    "HeartbeatHealthy",
			Status:  metav1.ConditionFalse,
			Reason:  "Intermittent",
			Message: "Heartbeat is intermittent",
			LastTransitionTime: metav1.Now(),
		})
		
	default:
		// Healthy target
		status.Conditions = append(status.Conditions, metav1.Condition{
			Type:    "SyncTargetReady",
			Status:  metav1.ConditionTrue,
			Reason:  "Ready",
			Message: "Sync target is ready and healthy",
			LastTransitionTime: metav1.Now(),
		})
		status.Conditions = append(status.Conditions, metav1.Condition{
			Type:    "HeartbeatHealthy",
			Status:  metav1.ConditionTrue,
			Reason:  "Healthy",
			Message: "Heartbeat is healthy",
			LastTransitionTime: metav1.Now(),
		})
	}
	
	return nil
}

// collectResourceStatus collects resource status information from the sync target
func (c *collector) collectResourceStatus(ctx context.Context, target SyncTarget, status *TargetStatus) error {
	// This would typically query the sync target's API to get resource counts
	// For now, we'll implement a basic simulation based on the sync target state
	
	// In a real implementation, this would:
	// 1. Query the sync target's API endpoint
	// 2. Count resources managed by this target
	// 3. Determine how many resources are in ready state
	// 4. Collect any resource-level errors or warnings
	
	// For this implementation, we'll simulate resource collection
	status.ResourceCount = c.simulateResourceCount(target)
	status.ReadyResources = c.simulateReadyResourceCount(target, status.ResourceCount)
	
	// Add resource status condition
	if status.ReadyResources == status.ResourceCount && status.ResourceCount > 0 {
		status.Conditions = append(status.Conditions, metav1.Condition{
			Type:    "ResourcesReady",
			Status:  metav1.ConditionTrue,
			Reason:  "AllResourcesReady",
			Message: fmt.Sprintf("All %d resources are ready", status.ResourceCount),
			LastTransitionTime: metav1.Now(),
		})
	} else if status.ResourceCount > 0 {
		status.Conditions = append(status.Conditions, metav1.Condition{
			Type:    "ResourcesReady",
			Status:  metav1.ConditionFalse,
			Reason:  "SomeResourcesNotReady",
			Message: fmt.Sprintf("%d/%d resources are ready", status.ReadyResources, status.ResourceCount),
			LastTransitionTime: metav1.Now(),
		})
		
		// If some resources are not ready, degrade health status
		if status.Health == HealthStatusHealthy && status.ResourceCount > status.ReadyResources {
			status.Health = HealthStatusDegraded
		}
	}
	
	return nil
}

// simulateResourceCount simulates counting resources for the target
// In a real implementation, this would query the actual sync target
func (c *collector) simulateResourceCount(target SyncTarget) int {
	// Simulate different resource counts based on target characteristics
	// This is just for demonstration - real implementation would query actual resources
	
	hash := 0
	for _, b := range target.Name {
		hash = hash*31 + int(b)
	}
	
	// Return a consistent but varied resource count
	return (hash % 20) + 5 // Between 5 and 24 resources
}

// simulateReadyResourceCount simulates counting ready resources
func (c *collector) simulateReadyResourceCount(target SyncTarget, total int) int {
	// Simulate different ready ratios based on target health
	// In reality, this would query the actual resource states
	
	if total == 0 {
		return 0
	}
	
	// Simulate mostly ready resources with some variation
	readyRatio := 0.85 + float64((len(target.Name) % 10))/100.0 // 0.85 to 0.94
	ready := int(float64(total) * readyRatio)
	
	if ready > total {
		ready = total
	}
	
	return ready
}