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
	"math"
	"sort"
	"sync"
	"time"
)

// LatencyAggregator provides utilities for aggregating and analyzing latency measurements.
// It calculates percentiles, averages, and other statistical measures for operation latencies.
type LatencyAggregator struct {
	mu sync.RWMutex
	
	// Configuration
	windowSize    time.Duration
	maxSamples    int
	
	// Sample storage
	samples       []LatencySample
	sortedSamples []time.Duration
	dirty         bool // Flag to indicate if sortedSamples needs rebuilding
	
	// Cached statistics
	lastUpdated   time.Time
	cachedStats   *LatencyStats
}

// LatencySample represents a single latency measurement with metadata.
type LatencySample struct {
	Timestamp time.Time
	Latency   time.Duration
	Labels    map[string]string
}

// LatencyStats holds aggregated latency statistics.
type LatencyStats struct {
	Count        int64
	Min          time.Duration
	Max          time.Duration
	Mean         time.Duration
	Median       time.Duration
	P50          time.Duration
	P90          time.Duration
	P95          time.Duration
	P99          time.Duration
	P999         time.Duration
	StdDev       time.Duration
	LastUpdated  time.Time
}

// LatencyAggregatorOptions configures the latency aggregator.
type LatencyAggregatorOptions struct {
	WindowSize time.Duration
	MaxSamples int
}

// NewLatencyAggregator creates a new latency aggregator with default settings.
func NewLatencyAggregator(opts *LatencyAggregatorOptions) *LatencyAggregator {
	if opts == nil {
		opts = &LatencyAggregatorOptions{
			WindowSize: 5 * time.Minute,
			MaxSamples: 10000,
		}
	}
	
	return &LatencyAggregator{
		windowSize:    opts.WindowSize,
		maxSamples:    opts.MaxSamples,
		samples:       make([]LatencySample, 0, opts.MaxSamples),
		sortedSamples: make([]time.Duration, 0, opts.MaxSamples),
		dirty:         false,
	}
}

// AddSample adds a new latency measurement to the aggregator.
func (la *LatencyAggregator) AddSample(latency time.Duration, labels map[string]string) {
	la.mu.Lock()
	defer la.mu.Unlock()
	
	now := time.Now()
	sample := LatencySample{
		Timestamp: now,
		Latency:   latency,
		Labels:    labels,
	}
	
	// Add sample
	la.samples = append(la.samples, sample)
	la.dirty = true
	
	// Prune old samples if needed
	la.pruneOldSamples(now)
	
	// Limit total samples
	if len(la.samples) > la.maxSamples {
		// Remove oldest samples
		excess := len(la.samples) - la.maxSamples
		copy(la.samples, la.samples[excess:])
		la.samples = la.samples[:la.maxSamples]
	}
	
	// Invalidate cached stats
	la.cachedStats = nil
}

// GetStats returns aggregated latency statistics.
// Results are cached and only recalculated when new samples are added.
func (la *LatencyAggregator) GetStats() *LatencyStats {
	la.mu.RLock()
	
	// Return cached stats if available and not dirty
	if la.cachedStats != nil && !la.dirty {
		defer la.mu.RUnlock()
		return la.cachedStats
	}
	
	// Need to calculate, upgrade to write lock
	la.mu.RUnlock()
	la.mu.Lock()
	defer la.mu.Unlock()
	
	// Double-check after acquiring write lock
	if la.cachedStats != nil && !la.dirty {
		return la.cachedStats
	}
	
	la.cachedStats = la.calculateStats()
	la.dirty = false
	
	return la.cachedStats
}

// GetPercentile returns the specified percentile from current samples.
func (la *LatencyAggregator) GetPercentile(p float64) time.Duration {
	stats := la.GetStats()
	
	// For common percentiles, use pre-calculated values
	switch p {
	case 50, 0.5:
		return stats.P50
	case 90, 0.9:
		return stats.P90
	case 95, 0.95:
		return stats.P95
	case 99, 0.99:
		return stats.P99
	case 99.9, 0.999:
		return stats.P999
	}
	
	// Calculate custom percentile
	la.mu.RLock()
	defer la.mu.RUnlock()
	
	la.ensureSorted()
	
	if len(la.sortedSamples) == 0 {
		return 0
	}
	
	return calculatePercentile(la.sortedSamples, p)
}

// GetSampleCount returns the current number of samples.
func (la *LatencyAggregator) GetSampleCount() int {
	la.mu.RLock()
	defer la.mu.RUnlock()
	return len(la.samples)
}

// Clear removes all samples and resets the aggregator.
func (la *LatencyAggregator) Clear() {
	la.mu.Lock()
	defer la.mu.Unlock()
	
	la.samples = la.samples[:0]
	la.sortedSamples = la.sortedSamples[:0]
	la.cachedStats = nil
	la.dirty = false
}

// pruneOldSamples removes samples outside the time window.
func (la *LatencyAggregator) pruneOldSamples(now time.Time) {
	cutoff := now.Add(-la.windowSize)
	
	// Find first sample within window
	writeIdx := 0
	for readIdx, sample := range la.samples {
		if sample.Timestamp.After(cutoff) {
			if writeIdx != readIdx {
				la.samples[writeIdx] = sample
			}
			writeIdx++
		}
	}
	
	// Truncate slice if we removed samples
	if writeIdx < len(la.samples) {
		la.samples = la.samples[:writeIdx]
		la.dirty = true
	}
}

// ensureSorted ensures sortedSamples is up to date.
func (la *LatencyAggregator) ensureSorted() {
	if !la.dirty {
		return
	}
	
	// Resize if needed
	if cap(la.sortedSamples) < len(la.samples) {
		la.sortedSamples = make([]time.Duration, len(la.samples))
	} else {
		la.sortedSamples = la.sortedSamples[:len(la.samples)]
	}
	
	// Copy latencies
	for i, sample := range la.samples {
		la.sortedSamples[i] = sample.Latency
	}
	
	// Sort
	sort.Slice(la.sortedSamples, func(i, j int) bool {
		return la.sortedSamples[i] < la.sortedSamples[j]
	})
}

// calculateStats computes all statistics from current samples.
func (la *LatencyAggregator) calculateStats() *LatencyStats {
	sampleCount := len(la.samples)
	if sampleCount == 0 {
		return &LatencyStats{LastUpdated: time.Now()}
	}
	
	la.ensureSorted()
	
	stats := &LatencyStats{
		Count:       int64(sampleCount),
		Min:         la.sortedSamples[0],
		Max:         la.sortedSamples[sampleCount-1],
		P50:         calculatePercentile(la.sortedSamples, 50),
		P90:         calculatePercentile(la.sortedSamples, 90),
		P95:         calculatePercentile(la.sortedSamples, 95),
		P99:         calculatePercentile(la.sortedSamples, 99),
		P999:        calculatePercentile(la.sortedSamples, 99.9),
		LastUpdated: time.Now(),
	}
	
	// Calculate mean
	var sum time.Duration
	for _, latency := range la.sortedSamples {
		sum += latency
	}
	stats.Mean = time.Duration(int64(sum) / int64(sampleCount))
	stats.Median = stats.P50
	
	// Calculate standard deviation
	var sumSquares float64
	meanFloat := float64(stats.Mean)
	for _, latency := range la.sortedSamples {
		diff := float64(latency) - meanFloat
		sumSquares += diff * diff
	}
	variance := sumSquares / float64(sampleCount)
	stats.StdDev = time.Duration(math.Sqrt(variance))
	
	return stats
}

// calculatePercentile calculates the specified percentile from sorted samples.
func calculatePercentile(sortedSamples []time.Duration, percentile float64) time.Duration {
	if len(sortedSamples) == 0 {
		return 0
	}
	
	if percentile <= 0 {
		return sortedSamples[0]
	}
	if percentile >= 100 {
		return sortedSamples[len(sortedSamples)-1]
	}
	
	// Convert percentile to 0-1 range if needed
	if percentile > 1 {
		percentile = percentile / 100
	}
	
	// Calculate index
	index := percentile * float64(len(sortedSamples)-1)
	lower := int(math.Floor(index))
	upper := int(math.Ceil(index))
	
	if lower == upper || upper >= len(sortedSamples) {
		return sortedSamples[lower]
	}
	
	// Linear interpolation
	weight := index - float64(lower)
	lowerVal := float64(sortedSamples[lower])
	upperVal := float64(sortedSamples[upper])
	
	interpolated := lowerVal + weight*(upperVal-lowerVal)
	return time.Duration(interpolated)
}