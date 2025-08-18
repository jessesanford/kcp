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
	"sync"
	"time"

	"k8s.io/klog/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// aggregator implements the StatusAggregator interface
type aggregator struct {
	collector StatusCollector
	health    HealthCalculator
	metrics   MetricsRecorder
	
	// Configuration
	timeout time.Duration
	
	mu sync.RWMutex
}

// NewStatusAggregator creates a new status aggregator with the given dependencies
func NewStatusAggregator(collector StatusCollector, health HealthCalculator, metrics MetricsRecorder) StatusAggregator {
	return &aggregator{
		collector: collector,
		health:    health,
		metrics:   metrics,
		timeout:   30 * time.Second, // Default timeout for status collection
	}
}

// AggregateStatus implements StatusAggregator.AggregateStatus
func (a *aggregator) AggregateStatus(ctx context.Context, placement *WorkloadPlacement) (*AggregatedStatus, error) {
	startTime := time.Now()
	
	logger := klog.FromContext(ctx).WithValues(
		"placement", placement.Name,
		"namespace", placement.Namespace,
	)
	
	logger.V(4).Info("Starting status aggregation")
	
	// Extract sync targets from placement
	targets, err := a.extractSyncTargets(placement)
	if err != nil {
		return nil, fmt.Errorf("failed to extract sync targets: %w", err)
	}
	
	if len(targets) == 0 {
		logger.V(2).Info("No sync targets found for placement")
		aggregated := &AggregatedStatus{
			OverallHealth:      HealthStatusUnknown,
			TargetStatuses:     []TargetStatus{},
			TotalTargets:       0,
			HealthyTargets:     0,
			SuccessPercentage:  0,
			LastAggregated:     metav1.Now(),
			AggregationLatency: time.Since(startTime),
		}
		
		// Record metrics even for empty target lists
		if err := a.RecordMetrics(aggregated); err != nil {
			logger.Error(err, "Failed to record aggregated metrics")
			// Don't fail the aggregation due to metrics recording issues
		}
		
		return aggregated, nil
	}
	
	// Collect status from all targets
	targetStatuses, err := a.CollectSyncTargetStatus(ctx, targets)
	if err != nil {
		logger.Error(err, "Failed to collect sync target status")
		// Continue with partial results rather than failing completely
	}
	
	// Calculate aggregated metrics
	aggregated := a.calculateAggregatedStatus(targetStatuses, time.Since(startTime))
	
	logger.V(4).Info("Status aggregation completed",
		"totalTargets", aggregated.TotalTargets,
		"healthyTargets", aggregated.HealthyTargets,
		"overallHealth", aggregated.OverallHealth,
		"successPercentage", aggregated.SuccessPercentage,
	)
	
	// Record metrics
	if err := a.RecordMetrics(aggregated); err != nil {
		logger.Error(err, "Failed to record aggregated metrics")
		// Don't fail the aggregation due to metrics recording issues
	}
	
	return aggregated, nil
}

// CollectSyncTargetStatus implements StatusAggregator.CollectSyncTargetStatus
func (a *aggregator) CollectSyncTargetStatus(ctx context.Context, targets []SyncTarget) ([]TargetStatus, error) {
	if len(targets) == 0 {
		return []TargetStatus{}, nil
	}
	
	// Create context with timeout
	collectCtx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()
	
	// Collect status from all targets concurrently
	type result struct {
		status TargetStatus
		err    error
	}
	
	results := make(chan result, len(targets))
	var wg sync.WaitGroup
	
	for _, target := range targets {
		wg.Add(1)
		go func(t SyncTarget) {
			defer wg.Done()
			
			status, err := a.collector.CollectStatus(collectCtx, t)
			results <- result{status: status, err: err}
		}(target)
	}
	
	// Wait for all collections to complete
	go func() {
		wg.Wait()
		close(results)
	}()
	
	// Collect results
	var statuses []TargetStatus
	var collectErrors []error
	
	for result := range results {
		if result.err != nil {
			collectErrors = append(collectErrors, result.err)
			// Create a status entry with error for failed collections
			statuses = append(statuses, TargetStatus{
				Target:        result.status.Target,
				Health:        HealthStatusUnknown,
				ResourceCount: 0,
				ReadyResources: 0,
				LastUpdated:   metav1.Now(),
				Error:         result.err,
			})
		} else {
			statuses = append(statuses, result.status)
		}
	}
	
	// Return partial results with error if some collections failed
	var aggregatedErr error
	if len(collectErrors) > 0 {
		aggregatedErr = fmt.Errorf("failed to collect status from %d/%d targets: %v", 
			len(collectErrors), len(targets), collectErrors[0])
	}
	
	return statuses, aggregatedErr
}

// CalculateOverallHealth implements StatusAggregator.CalculateOverallHealth
func (a *aggregator) CalculateOverallHealth(statuses []TargetStatus) HealthStatus {
	if a.health == nil {
		// Fallback basic calculation if no health calculator provided
		return a.basicHealthCalculation(statuses)
	}
	return a.health.CalculateOverallHealth(statuses)
}

// RecordMetrics implements StatusAggregator.RecordMetrics
func (a *aggregator) RecordMetrics(status *AggregatedStatus) error {
	if a.metrics == nil {
		// No-op if no metrics recorder provided
		return nil
	}
	return a.metrics.RecordAggregatedStatus(status)
}

// extractSyncTargets extracts sync targets from the workload placement
func (a *aggregator) extractSyncTargets(placement *WorkloadPlacement) ([]SyncTarget, error) {
	var targets []SyncTarget
	
	// Extract targets from placement spec
	if placement.Spec.LocationResource != nil {
		// Handle location-based placement
		target := SyncTarget{
			Name:      placement.Spec.LocationResource.Name,
			Workspace: placement.Spec.LocationResource.Workspace,
			LastSeen:  metav1.Now(),
		}
		targets = append(targets, target)
	}
	
	// Handle additional targets from status if available
	for _, condition := range placement.Status.Conditions {
		if condition.Type == "Placed" && condition.Status == metav1.ConditionTrue {
			// Extract placement information from condition message or reason
			// This is a simplified extraction - in practice, this would parse
			// more detailed placement information
			targets = append(targets, SyncTarget{
				Name:      condition.Reason,
				Workspace: placement.Namespace,
				LastSeen:  condition.LastTransitionTime,
			})
		}
	}
	
	return targets, nil
}

// calculateAggregatedStatus computes the aggregated status from individual target statuses
func (a *aggregator) calculateAggregatedStatus(statuses []TargetStatus, latency time.Duration) *AggregatedStatus {
	aggregated := &AggregatedStatus{
		TargetStatuses:     statuses,
		TotalTargets:       len(statuses),
		LastAggregated:     metav1.Now(),
		AggregationLatency: latency,
	}
	
	// Calculate totals
	var totalResources, readyResources, healthyTargets int
	
	for _, status := range statuses {
		totalResources += status.ResourceCount
		readyResources += status.ReadyResources
		
		if status.Health.IsHealthy() {
			healthyTargets++
		}
	}
	
	aggregated.TotalResources = totalResources
	aggregated.ReadyResources = readyResources
	aggregated.HealthyTargets = healthyTargets
	
	// Calculate success percentage
	if aggregated.TotalTargets > 0 {
		aggregated.SuccessPercentage = float64(healthyTargets) / float64(aggregated.TotalTargets) * 100
	}
	
	// Calculate overall health
	aggregated.OverallHealth = a.CalculateOverallHealth(statuses)
	
	return aggregated
}

// basicHealthCalculation provides a fallback health calculation
func (a *aggregator) basicHealthCalculation(statuses []TargetStatus) HealthStatus {
	if len(statuses) == 0 {
		return HealthStatusUnknown
	}
	
	var healthy, degraded, unhealthy, unknown int
	
	for _, status := range statuses {
		switch status.Health {
		case HealthStatusHealthy:
			healthy++
		case HealthStatusDegraded:
			degraded++
		case HealthStatusUnhealthy:
			unhealthy++
		case HealthStatusUnknown:
			unknown++
		}
	}
	
	total := len(statuses)
	
	// If majority are healthy, overall is healthy
	if float64(healthy)/float64(total) >= 0.8 {
		return HealthStatusHealthy
	}
	
	// If any are unhealthy, overall is degraded or unhealthy
	if unhealthy > 0 {
		if float64(unhealthy)/float64(total) >= 0.5 {
			return HealthStatusUnhealthy
		}
		return HealthStatusDegraded
	}
	
	// If we have degraded targets, overall is degraded
	if degraded > 0 {
		return HealthStatusDegraded
	}
	
	// If too many unknown, overall is unknown
	if float64(unknown)/float64(total) >= 0.5 {
		return HealthStatusUnknown
	}
	
	return HealthStatusDegraded
}