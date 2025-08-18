/*
Copyright 2023 The KCP Authors.

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

package aggregators

import (
	"sync"
	"time"
)

// UtilizationAggregator provides utilities for aggregating and analyzing resource utilization.
// It tracks resource usage patterns, calculates averages, peaks, and provides utilization insights.
type UtilizationAggregator struct {
	mu sync.RWMutex
	
	// Configuration
	windowSize    time.Duration
	maxDataPoints int
	
	// Resource tracking
	resources map[string]*ResourceUtilization
	
	// Time series data
	dataPoints []UtilizationDataPoint
}

// ResourceUtilization tracks utilization for a specific resource type.
type ResourceUtilization struct {
	ResourceType   string
	Capacity       float64
	CurrentUsage   float64
	PeakUsage      float64
	AverageUsage   float64
	LastUpdated    time.Time
	UsageHistory   []UsagePoint
	
	// Statistics
	UtilizationPercent float64
	PeakPercent        float64
	AveragePercent     float64
}

// UsagePoint represents a single usage measurement.
type UsagePoint struct {
	Timestamp time.Time
	Usage     float64
	Capacity  float64
}

// UtilizationDataPoint represents utilization across all resources at a point in time.
type UtilizationDataPoint struct {
	Timestamp           time.Time
	OverallUtilization  float64
	ResourceBreakdown   map[string]float64
	ClusterBreakdown    map[string]float64
	WorkspaceBreakdown  map[string]float64
}

// UtilizationStats provides aggregated utilization statistics.
type UtilizationStats struct {
	OverallUtilization   float64
	PeakUtilization      float64
	AverageUtilization   float64
	ResourceUtilizations map[string]*ResourceUtilization
	Trend               string // "increasing", "decreasing", "stable"
	LastUpdated         time.Time
}

// UtilizationAggregatorOptions configures the utilization aggregator.
type UtilizationAggregatorOptions struct {
	WindowSize    time.Duration
	MaxDataPoints int
}

// NewUtilizationAggregator creates a new utilization aggregator.
func NewUtilizationAggregator(opts *UtilizationAggregatorOptions) *UtilizationAggregator {
	if opts == nil {
		opts = &UtilizationAggregatorOptions{
			WindowSize:    1 * time.Hour,
			MaxDataPoints: 3600, // One per second for an hour
		}
	}
	
	return &UtilizationAggregator{
		windowSize:    opts.WindowSize,
		maxDataPoints: opts.MaxDataPoints,
		resources:     make(map[string]*ResourceUtilization),
		dataPoints:    make([]UtilizationDataPoint, 0, opts.MaxDataPoints),
	}
}

// UpdateResourceUtilization updates the utilization for a specific resource.
func (ua *UtilizationAggregator) UpdateResourceUtilization(resourceType string, usage, capacity float64) {
	ua.mu.Lock()
	defer ua.mu.Unlock()
	
	now := time.Now()
	
	// Get or create resource utilization
	resource, exists := ua.resources[resourceType]
	if !exists {
		resource = &ResourceUtilization{
			ResourceType: resourceType,
			UsageHistory: make([]UsagePoint, 0, 1000),
		}
		ua.resources[resourceType] = resource
	}
	
	// Update current values
	resource.Capacity = capacity
	resource.CurrentUsage = usage
	resource.LastUpdated = now
	
	// Calculate percentages
	if capacity > 0 {
		resource.UtilizationPercent = (usage / capacity) * 100
	}
	
	// Update peak usage
	if usage > resource.PeakUsage {
		resource.PeakUsage = usage
		if capacity > 0 {
			resource.PeakPercent = (resource.PeakUsage / capacity) * 100
		}
	}
	
	// Add to history
	usagePoint := UsagePoint{
		Timestamp: now,
		Usage:     usage,
		Capacity:  capacity,
	}
	resource.UsageHistory = append(resource.UsageHistory, usagePoint)
	
	// Prune old history
	ua.pruneResourceHistory(resource, now)
	
	// Calculate average usage
	resource.AverageUsage = ua.calculateAverageUsage(resource)
	if capacity > 0 {
		resource.AveragePercent = (resource.AverageUsage / capacity) * 100
	}
}

// UpdateClusterUtilization updates utilization data for a specific cluster.
func (ua *UtilizationAggregator) UpdateClusterUtilization(cluster string, utilization float64, resourceBreakdown map[string]float64) {
	ua.mu.Lock()
	defer ua.mu.Unlock()
	
	now := time.Now()
	
	// Create or update data point
	dataPoint := UtilizationDataPoint{
		Timestamp:          now,
		OverallUtilization: utilization,
		ResourceBreakdown:  make(map[string]float64),
		ClusterBreakdown:   make(map[string]float64),
		WorkspaceBreakdown: make(map[string]float64),
	}
	
	// Copy resource breakdown
	for resource, value := range resourceBreakdown {
		dataPoint.ResourceBreakdown[resource] = value
	}
	
	// Set cluster utilization
	dataPoint.ClusterBreakdown[cluster] = utilization
	
	// Add data point
	ua.dataPoints = append(ua.dataPoints, dataPoint)
	
	// Prune old data points
	ua.pruneOldDataPoints(now)
}

// GetUtilizationStats returns current utilization statistics.
func (ua *UtilizationAggregator) GetUtilizationStats() *UtilizationStats {
	ua.mu.RLock()
	defer ua.mu.RUnlock()
	
	stats := &UtilizationStats{
		ResourceUtilizations: make(map[string]*ResourceUtilization),
		LastUpdated:         time.Now(),
	}
	
	// Copy resource utilizations
	for name, resource := range ua.resources {
		resourceCopy := *resource
		stats.ResourceUtilizations[name] = &resourceCopy
	}
	
	// Calculate overall statistics
	stats.OverallUtilization = ua.calculateOverallUtilization()
	stats.PeakUtilization = ua.calculatePeakUtilization()
	stats.AverageUtilization = ua.calculateAverageOverallUtilization()
	stats.Trend = ua.calculateTrend()
	
	return stats
}

// GetResourceUtilization returns utilization for a specific resource type.
func (ua *UtilizationAggregator) GetResourceUtilization(resourceType string) *ResourceUtilization {
	ua.mu.RLock()
	defer ua.mu.RUnlock()
	
	resource, exists := ua.resources[resourceType]
	if !exists {
		return nil
	}
	
	// Return a copy
	resourceCopy := *resource
	return &resourceCopy
}

// GetUtilizationTrend returns the utilization trend over the time window.
func (ua *UtilizationAggregator) GetUtilizationTrend(resourceType string) []UsagePoint {
	ua.mu.RLock()
	defer ua.mu.RUnlock()
	
	resource, exists := ua.resources[resourceType]
	if !exists {
		return nil
	}
	
	// Return a copy of the history
	history := make([]UsagePoint, len(resource.UsageHistory))
	copy(history, resource.UsageHistory)
	return history
}

// Clear removes all utilization data.
func (ua *UtilizationAggregator) Clear() {
	ua.mu.Lock()
	defer ua.mu.Unlock()
	
	ua.resources = make(map[string]*ResourceUtilization)
	ua.dataPoints = ua.dataPoints[:0]
}

// GetHighestUtilizedResources returns the N most highly utilized resources.
func (ua *UtilizationAggregator) GetHighestUtilizedResources(n int) []*ResourceUtilization {
	ua.mu.RLock()
	defer ua.mu.RUnlock()
	
	// Collect all resources
	resources := make([]*ResourceUtilization, 0, len(ua.resources))
	for _, resource := range ua.resources {
		resourceCopy := *resource
		resources = append(resources, &resourceCopy)
	}
	
	// Sort by utilization percentage (descending)
	for i := 0; i < len(resources)-1; i++ {
		for j := i + 1; j < len(resources); j++ {
			if resources[i].UtilizationPercent < resources[j].UtilizationPercent {
				resources[i], resources[j] = resources[j], resources[i]
			}
		}
	}
	
	// Return top N
	if n > len(resources) {
		n = len(resources)
	}
	return resources[:n]
}

// pruneResourceHistory removes old usage history points.
func (ua *UtilizationAggregator) pruneResourceHistory(resource *ResourceUtilization, now time.Time) {
	cutoff := now.Add(-ua.windowSize)
	
	writeIdx := 0
	for readIdx, point := range resource.UsageHistory {
		if point.Timestamp.After(cutoff) {
			if writeIdx != readIdx {
				resource.UsageHistory[writeIdx] = point
			}
			writeIdx++
		}
	}
	
	resource.UsageHistory = resource.UsageHistory[:writeIdx]
}

// pruneOldDataPoints removes old utilization data points.
func (ua *UtilizationAggregator) pruneOldDataPoints(now time.Time) {
	cutoff := now.Add(-ua.windowSize)
	
	writeIdx := 0
	for readIdx, point := range ua.dataPoints {
		if point.Timestamp.After(cutoff) {
			if writeIdx != readIdx {
				ua.dataPoints[writeIdx] = point
			}
			writeIdx++
		}
	}
	
	ua.dataPoints = ua.dataPoints[:writeIdx]
	
	// Limit total data points
	if len(ua.dataPoints) > ua.maxDataPoints {
		excess := len(ua.dataPoints) - ua.maxDataPoints
		copy(ua.dataPoints, ua.dataPoints[excess:])
		ua.dataPoints = ua.dataPoints[:ua.maxDataPoints]
	}
}

// calculateAverageUsage calculates the average usage for a resource.
func (ua *UtilizationAggregator) calculateAverageUsage(resource *ResourceUtilization) float64 {
	if len(resource.UsageHistory) == 0 {
		return 0
	}
	
	var sum float64
	for _, point := range resource.UsageHistory {
		sum += point.Usage
	}
	
	return sum / float64(len(resource.UsageHistory))
}

// calculateOverallUtilization calculates the current overall utilization.
func (ua *UtilizationAggregator) calculateOverallUtilization() float64 {
	if len(ua.resources) == 0 {
		return 0
	}
	
	var totalUsage, totalCapacity float64
	for _, resource := range ua.resources {
		totalUsage += resource.CurrentUsage
		totalCapacity += resource.Capacity
	}
	
	if totalCapacity == 0 {
		return 0
	}
	
	return (totalUsage / totalCapacity) * 100
}

// calculatePeakUtilization calculates the peak utilization across all resources.
func (ua *UtilizationAggregator) calculatePeakUtilization() float64 {
	var peak float64
	for _, resource := range ua.resources {
		if resource.PeakPercent > peak {
			peak = resource.PeakPercent
		}
	}
	return peak
}

// calculateAverageOverallUtilization calculates the average overall utilization.
func (ua *UtilizationAggregator) calculateAverageOverallUtilization() float64 {
	if len(ua.dataPoints) == 0 {
		return 0
	}
	
	var sum float64
	for _, point := range ua.dataPoints {
		sum += point.OverallUtilization
	}
	
	return sum / float64(len(ua.dataPoints))
}

// calculateTrend analyzes the utilization trend.
func (ua *UtilizationAggregator) calculateTrend() string {
	if len(ua.dataPoints) < 10 {
		return "unknown"
	}
	
	// Compare recent points with older points
	recentCount := len(ua.dataPoints) / 4
	if recentCount < 5 {
		recentCount = 5
	}
	
	var recentSum, olderSum float64
	
	// Calculate recent average
	for i := len(ua.dataPoints) - recentCount; i < len(ua.dataPoints); i++ {
		recentSum += ua.dataPoints[i].OverallUtilization
	}
	recentAvg := recentSum / float64(recentCount)
	
	// Calculate older average
	for i := 0; i < recentCount && i < len(ua.dataPoints); i++ {
		olderSum += ua.dataPoints[i].OverallUtilization
	}
	olderAvg := olderSum / float64(recentCount)
	
	// Determine trend
	diff := recentAvg - olderAvg
	threshold := 5.0 // 5% threshold
	
	if diff > threshold {
		return "increasing"
	} else if diff < -threshold {
		return "decreasing"
	}
	return "stable"
}