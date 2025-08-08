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
	"time"
)

// MetricPoint represents a single metric data point with timestamp and value.
type MetricPoint struct {
	// Timestamp when the metric was recorded
	Timestamp time.Time `json:"timestamp"`
	// Value of the metric at the timestamp
	Value float64 `json:"value"`
	// Labels associated with this metric point
	Labels map[string]string `json:"labels,omitempty"`
}

// MetricSeries represents a time series of metric points for a specific metric.
type MetricSeries struct {
	// Name of the metric
	Name string `json:"name"`
	// Description of what the metric measures
	Description string `json:"description,omitempty"`
	// Unit of measurement for the metric values
	Unit string `json:"unit,omitempty"`
	// Points contains the time series data points
	Points []MetricPoint `json:"points"`
	// CommonLabels are labels that apply to all points in this series
	CommonLabels map[string]string `json:"commonLabels,omitempty"`
}

// QueryOptions defines options for querying metrics from storage.
type QueryOptions struct {
	// StartTime is the earliest timestamp to include in results
	StartTime *time.Time `json:"startTime,omitempty"`
	// EndTime is the latest timestamp to include in results  
	EndTime *time.Time `json:"endTime,omitempty"`
	// LabelSelectors filter results by label key-value pairs
	LabelSelectors map[string]string `json:"labelSelectors,omitempty"`
	// Limit restricts the maximum number of points returned
	Limit int `json:"limit,omitempty"`
	// Resolution controls the granularity of returned data points
	Resolution time.Duration `json:"resolution,omitempty"`
}

// RetentionPolicy defines how long metrics should be stored and at what granularities.
type RetentionPolicy struct {
	// MaxAge is the maximum age of metrics to retain
	MaxAge time.Duration `json:"maxAge"`
	// MaxPoints is the maximum number of points to retain per metric series
	MaxPoints int `json:"maxPoints,omitempty"`
	// AggregationRules define how to downsample data over time
	AggregationRules []AggregationRule `json:"aggregationRules,omitempty"`
}

// AggregationRule defines how to aggregate data points over time windows.
// For initial implementation, only basic rules are supported.
type AggregationRule struct {
	// Window is the time window size for aggregation
	Window time.Duration `json:"window"`
	// Function is the aggregation function (avg, max, min, sum)
	Function string `json:"function"`
}

// StorageStats provides statistics about the storage backend.
type StorageStats struct {
	// TotalMetrics is the number of unique metric series stored
	TotalMetrics int64 `json:"totalMetrics"`
	// TotalPoints is the total number of data points stored
	TotalPoints int64 `json:"totalPoints"`
	// StorageSize is the approximate storage size in bytes
	StorageSize int64 `json:"storageSize,omitempty"`
	// OldestPoint is the timestamp of the oldest data point
	OldestPoint *time.Time `json:"oldestPoint,omitempty"`
	// NewestPoint is the timestamp of the newest data point
	NewestPoint *time.Time `json:"newestPoint,omitempty"`
}

// MetricsStorage defines the interface for storing and querying TMC metrics.
// Implementations must be thread-safe and support concurrent operations.
type MetricsStorage interface {
	// WriteMetricPoint stores a single metric data point.
	// The point will be associated with the given metric name.
	WriteMetricPoint(ctx context.Context, metricName string, point MetricPoint) error

	// WriteMetricSeries stores multiple data points for a metric series.
	// This is more efficient than multiple WriteMetricPoint calls.
	WriteMetricSeries(ctx context.Context, series MetricSeries) error

	// QueryMetrics retrieves metric data based on the provided options.
	// Returns a slice of metric series matching the query criteria.
	QueryMetrics(ctx context.Context, metricNames []string, options QueryOptions) ([]MetricSeries, error)

	// ListMetricNames returns all available metric names, optionally filtered by labels.
	// This is useful for discovering what metrics are available.
	ListMetricNames(ctx context.Context, labelSelectors map[string]string) ([]string, error)

	// DeleteMetrics removes metrics matching the specified criteria.
	// Use with caution as this operation is typically irreversible.
	DeleteMetrics(ctx context.Context, metricNames []string, options QueryOptions) error

	// ApplyRetentionPolicy applies retention rules to remove old data.
	// This should be called periodically to manage storage size.
	ApplyRetentionPolicy(ctx context.Context, policy RetentionPolicy) error

	// GetStats returns statistics about the storage backend.
	// This is useful for monitoring storage health and usage.
	GetStats(ctx context.Context) (StorageStats, error)

	// Close cleanly shuts down the storage backend and releases resources.
	// After calling Close, the storage backend should not be used.
	Close() error
}

// StorageConfig provides configuration for storage backends.
type StorageConfig struct {
	// RetentionPolicy defines retention behavior
	RetentionPolicy RetentionPolicy `json:"retentionPolicy,omitempty"`
}