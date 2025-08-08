/*
Copyright 2025 The KCP Authors.

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

package storage

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/kcp-dev/kcp/pkg/features"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/klog/v2"
)

// memoryStorage implements MetricsStorage using in-memory data structures.
// It's designed for development, testing, and small deployments where
// persistence is not required.
type memoryStorage struct {
	// mu protects all access to the data structures below
	mu sync.RWMutex
	
	// metrics maps metric names to their time series data
	metrics map[string]*metricSeries
	
	// config holds the storage configuration
	config StorageConfig
	
	// closed indicates if the storage has been closed
	closed bool
}

// metricSeries holds the time series data for a single metric.
type metricSeries struct {
	name         string
	description  string
	unit         string
	points       []MetricPoint
	commonLabels map[string]string
}

// NewMemoryStorage creates a new in-memory metrics storage backend.
func NewMemoryStorage(config StorageConfig) (MetricsStorage, error) {
	if !utilfeature.DefaultFeatureGate.Enabled(features.TMCMetricsStorage) {
		return nil, fmt.Errorf("TMC metrics storage feature is not enabled")
	}

	storage := &memoryStorage{
		metrics: make(map[string]*metricSeries),
		config:  config,
	}

	klog.V(2).InfoS("Created in-memory metrics storage", "config", config)
	return storage, nil
}

// WriteMetricPoint stores a single metric data point.
func (s *memoryStorage) WriteMetricPoint(ctx context.Context, metricName string, point MetricPoint) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return fmt.Errorf("storage is closed")
	}

	series, exists := s.metrics[metricName]
	if !exists {
		series = &metricSeries{
			name:   metricName,
			points: make([]MetricPoint, 0),
		}
		s.metrics[metricName] = series
	}

	// Insert point maintaining chronological order
	series.points = append(series.points, point)
	sort.Slice(series.points, func(i, j int) bool {
		return series.points[i].Timestamp.Before(series.points[j].Timestamp)
	})
	
	klog.V(4).InfoS("Wrote metric point", "metric", metricName, "timestamp", point.Timestamp, "value", point.Value)
	return nil
}

// WriteMetricSeries stores multiple data points for a metric series.
func (s *memoryStorage) WriteMetricSeries(ctx context.Context, series MetricSeries) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return fmt.Errorf("storage is closed")
	}

	existing, exists := s.metrics[series.Name]
	if !exists {
		existing = &metricSeries{
			name:         series.Name,
			description:  series.Description,
			unit:         series.Unit,
			commonLabels: series.CommonLabels,
			points:       make([]MetricPoint, 0),
		}
		s.metrics[series.Name] = existing
	}

	// Update metadata if provided
	if series.Description != "" {
		existing.description = series.Description
	}
	if series.Unit != "" {
		existing.unit = series.Unit
	}
	if len(series.CommonLabels) > 0 {
		existing.commonLabels = series.CommonLabels
	}

	// Add all points and sort
	existing.points = append(existing.points, series.Points...)
	sort.Slice(existing.points, func(i, j int) bool {
		return existing.points[i].Timestamp.Before(existing.points[j].Timestamp)
	})

	klog.V(4).InfoS("Wrote metric series", "metric", series.Name, "points", len(series.Points))
	return nil
}

// QueryMetrics retrieves metric data based on the provided options.
func (s *memoryStorage) QueryMetrics(ctx context.Context, metricNames []string, options QueryOptions) ([]MetricSeries, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, fmt.Errorf("storage is closed")
	}

	var result []MetricSeries

	for _, name := range metricNames {
		series, exists := s.metrics[name]
		if !exists {
			continue
		}

		filteredSeries := s.filterSeries(*series, options)
		if len(filteredSeries.Points) > 0 {
			result = append(result, filteredSeries)
		}
	}

	klog.V(4).InfoS("Queried metrics", "requested", len(metricNames), "returned", len(result))
	return result, nil
}

// ListMetricNames returns all available metric names, optionally filtered by labels.
func (s *memoryStorage) ListMetricNames(ctx context.Context, labelSelectors map[string]string) ([]string, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, fmt.Errorf("storage is closed")
	}

	var names []string
	for name, series := range s.metrics {
		if s.matchesLabelSelectors(series.commonLabels, labelSelectors) {
			names = append(names, name)
		}
	}

	sort.Strings(names)
	klog.V(4).InfoS("Listed metric names", "total", len(names), "selectors", labelSelectors)
	return names, nil
}

// DeleteMetrics removes metrics matching the specified criteria.
func (s *memoryStorage) DeleteMetrics(ctx context.Context, metricNames []string, options QueryOptions) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return fmt.Errorf("storage is closed")
	}

	for _, name := range metricNames {
		if _, exists := s.metrics[name]; exists {
			delete(s.metrics, name)
		}
	}

	klog.V(4).InfoS("Deleted metrics", "metrics", metricNames)
	return nil
}

// ApplyRetentionPolicy applies retention rules to remove old data.
func (s *memoryStorage) ApplyRetentionPolicy(ctx context.Context, policy RetentionPolicy) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return fmt.Errorf("storage is closed")
	}

	now := time.Now()
	cutoff := now.Add(-policy.MaxAge)
	
	var removedPoints int64

	for name, series := range s.metrics {
		var retainedPoints []MetricPoint
		
		for _, point := range series.points {
			if point.Timestamp.After(cutoff) {
				retainedPoints = append(retainedPoints, point)
			} else {
				removedPoints++
			}
		}

		// Apply max points limit
		if policy.MaxPoints > 0 && len(retainedPoints) > policy.MaxPoints {
			start := len(retainedPoints) - policy.MaxPoints
			removedPoints += int64(start)
			retainedPoints = retainedPoints[start:]
		}

		if len(retainedPoints) == 0 {
			delete(s.metrics, name)
		} else {
			series.points = retainedPoints
		}
	}

	klog.V(2).InfoS("Applied retention policy", "removedPoints", removedPoints, "cutoff", cutoff)
	return nil
}

// GetStats returns statistics about the storage backend.
func (s *memoryStorage) GetStats(ctx context.Context) (StorageStats, error) {
	if ctx.Err() != nil {
		return StorageStats{}, ctx.Err()
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return StorageStats{}, fmt.Errorf("storage is closed")
	}

	var stats StorageStats
	stats.TotalMetrics = int64(len(s.metrics))

	for _, series := range s.metrics {
		stats.TotalPoints += int64(len(series.points))
	}
	
	klog.V(4).InfoS("Generated storage stats", "metrics", stats.TotalMetrics, "points", stats.TotalPoints)
	return stats, nil
}

// Close cleanly shuts down the storage backend and releases resources.
func (s *memoryStorage) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	s.closed = true
	s.metrics = nil

	klog.V(2).InfoS("Closed memory storage")
	return nil
}

// Helper methods

// filterSeries applies query options to filter a metric series.
func (s *memoryStorage) filterSeries(series metricSeries, options QueryOptions) MetricSeries {
	var filteredPoints []MetricPoint
	
	for _, point := range series.points {
		// Apply time range filter
		if options.StartTime != nil && point.Timestamp.Before(*options.StartTime) {
			continue
		}
		if options.EndTime != nil && point.Timestamp.After(*options.EndTime) {
			continue
		}
		
		// Apply label selectors
		if !s.matchesLabelSelectors(point.Labels, options.LabelSelectors) {
			continue
		}
		
		filteredPoints = append(filteredPoints, point)
		
		// Apply limit
		if options.Limit > 0 && len(filteredPoints) >= options.Limit {
			break
		}
	}

	return MetricSeries{
		Name:         series.name,
		Description:  series.description,
		Unit:         series.unit,
		Points:       filteredPoints,
		CommonLabels: series.commonLabels,
	}
}

// matchesLabelSelectors checks if labels match the provided selectors.
func (s *memoryStorage) matchesLabelSelectors(labels, selectors map[string]string) bool {
	for key, expectedValue := range selectors {
		actualValue, exists := labels[key]
		if !exists || actualValue != expectedValue {
			return false
		}
	}
	return true
}