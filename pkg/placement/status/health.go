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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HealthCalculator defines the interface for calculating overall health
// from individual target statuses.
type HealthCalculator interface {
	// CalculateOverallHealth determines overall health from target statuses
	CalculateOverallHealth(statuses []TargetStatus) HealthStatus
}

// HealthThresholds defines thresholds for health calculations
type HealthThresholds struct {
	// HealthyThreshold is the minimum percentage of healthy targets for overall healthy status
	HealthyThreshold float64
	
	// UnhealthyThreshold is the maximum percentage of unhealthy targets before overall unhealthy
	UnhealthyThreshold float64
	
	// ResourceReadyThreshold is the minimum percentage of ready resources for healthy status
	ResourceReadyThreshold float64
	
	// MaxStaleTime is the maximum time since last update before considering target stale
	MaxStaleTime time.Duration
}

// DefaultHealthThresholds returns default health calculation thresholds
func DefaultHealthThresholds() HealthThresholds {
	return HealthThresholds{
		HealthyThreshold:       0.80, // 80% of targets must be healthy
		UnhealthyThreshold:     0.30, // If 30% or more targets are unhealthy, overall is unhealthy
		ResourceReadyThreshold: 0.90, // 90% of resources should be ready
		MaxStaleTime:          5 * time.Minute, // 5 minutes before considering stale
	}
}

// calculator implements HealthCalculator with configurable thresholds
type calculator struct {
	thresholds HealthThresholds
}

// NewHealthCalculator creates a new health calculator with the given thresholds
func NewHealthCalculator(thresholds HealthThresholds) HealthCalculator {
	return &calculator{
		thresholds: thresholds,
	}
}

// NewDefaultHealthCalculator creates a health calculator with default thresholds
func NewDefaultHealthCalculator() HealthCalculator {
	return NewHealthCalculator(DefaultHealthThresholds())
}

// CalculateOverallHealth implements HealthCalculator.CalculateOverallHealth
func (c *calculator) CalculateOverallHealth(statuses []TargetStatus) HealthStatus {
	if len(statuses) == 0 {
		return HealthStatusUnknown
	}
	
	// Count target health states
	healthCounts := c.countHealthStates(statuses)
	
	// Calculate resource health
	resourceHealth := c.calculateResourceHealth(statuses)
	
	// Check for stale targets
	staleTargets := c.countStaleTargets(statuses)
	
	// Apply health calculation logic
	return c.determineOverallHealth(healthCounts, resourceHealth, staleTargets, len(statuses))
}

// healthCounts tracks the count of each health state
type healthCounts struct {
	healthy   int
	degraded  int
	unhealthy int
	unknown   int
}

// countHealthStates counts targets in each health state
func (c *calculator) countHealthStates(statuses []TargetStatus) healthCounts {
	counts := healthCounts{}
	
	for _, status := range statuses {
		switch status.Health {
		case HealthStatusHealthy:
			counts.healthy++
		case HealthStatusDegraded:
			counts.degraded++
		case HealthStatusUnhealthy:
			counts.unhealthy++
		case HealthStatusUnknown:
			counts.unknown++
		}
	}
	
	return counts
}

// resourceHealth tracks resource-level health metrics
type resourceHealth struct {
	totalResources int
	readyResources int
	readyPercentage float64
}

// calculateResourceHealth calculates resource-level health metrics
func (c *calculator) calculateResourceHealth(statuses []TargetStatus) resourceHealth {
	rh := resourceHealth{}
	
	for _, status := range statuses {
		rh.totalResources += status.ResourceCount
		rh.readyResources += status.ReadyResources
	}
	
	if rh.totalResources > 0 {
		rh.readyPercentage = float64(rh.readyResources) / float64(rh.totalResources)
	}
	
	return rh
}

// countStaleTargets counts targets that haven't been updated recently
func (c *calculator) countStaleTargets(statuses []TargetStatus) int {
	now := metav1.Now()
	staleCount := 0
	
	for _, status := range statuses {
		if now.Time.Sub(status.LastUpdated.Time) > c.thresholds.MaxStaleTime {
			staleCount++
		}
	}
	
	return staleCount
}

// determineOverallHealth applies the health calculation logic
func (c *calculator) determineOverallHealth(
	counts healthCounts,
	resourceHealth resourceHealth,
	staleTargets int,
	totalTargets int,
) HealthStatus {
	// Calculate percentages
	healthyPercentage := float64(counts.healthy) / float64(totalTargets)
	unhealthyPercentage := float64(counts.unhealthy) / float64(totalTargets)
	unknownPercentage := float64(counts.unknown) / float64(totalTargets)
	stalePercentage := float64(staleTargets) / float64(totalTargets)
	
	// Priority 1: If too many targets are unhealthy, overall is unhealthy
	if unhealthyPercentage >= c.thresholds.UnhealthyThreshold {
		return HealthStatusUnhealthy
	}
	
	// Priority 2: If too many targets are stale, overall is degraded or unhealthy
	if stalePercentage >= 0.5 { // If 50% or more are stale
		if unhealthyPercentage > 0.1 { // And some are explicitly unhealthy
			return HealthStatusUnhealthy
		}
		return HealthStatusDegraded
	}
	
	// Priority 3: If too many targets are unknown, overall health is uncertain
	if unknownPercentage >= 0.5 {
		return HealthStatusUnknown
	}
	
	// Priority 4: Check resource health
	if resourceHealth.totalResources > 0 {
		if resourceHealth.readyPercentage < 0.5 { // Less than 50% resources ready
			return HealthStatusUnhealthy
		}
		if resourceHealth.readyPercentage < c.thresholds.ResourceReadyThreshold {
			return HealthStatusDegraded
		}
	}
	
	// Priority 5: Check target health percentage
	if healthyPercentage >= c.thresholds.HealthyThreshold {
		// Enough healthy targets, but check for any degraded ones
		if counts.degraded > 0 || resourceHealth.readyPercentage < 1.0 {
			return HealthStatusDegraded
		}
		return HealthStatusHealthy
	}
	
	// Priority 6: If we have some healthy targets but not enough
	if counts.healthy > 0 {
		return HealthStatusDegraded
	}
	
	// Priority 7: If we have any unhealthy targets
	if counts.unhealthy > 0 {
		return HealthStatusUnhealthy
	}
	
	// Default: If we can't determine health, it's unknown
	return HealthStatusUnknown
}