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

package workspace

import (
	"context"
	"time"
)

// MetricsCollector collects and aggregates workspace operational metrics.
// It provides instrumentation for performance monitoring, troubleshooting,
// and capacity planning across virtual workspaces.
type MetricsCollector interface {
	// RecordRequest records metrics for an API request to a workspace.
	// Captures latency, status codes, and throughput information.
	RecordRequest(ctx context.Context, metric RequestMetric) error

	// RecordLatency records operation latency for workspace operations.
	// Used to track performance of lifecycle operations and health checks.
	RecordLatency(ctx context.Context, operation string, workspace string, duration time.Duration) error

	// RecordError records error occurrences for monitoring and alerting.
	// Includes error categorization for better operational insights.
	RecordError(ctx context.Context, operation string, workspace string, errorType string, err error) error

	// IncrementCounter increments a named counter metric.
	// Useful for tracking discrete events like workspace creations or failures.
	IncrementCounter(ctx context.Context, name string, labels map[string]string) error

	// SetGauge sets a gauge metric to a specific value.
	// Used for tracking current values like active connections or resource usage.
	SetGauge(ctx context.Context, name string, value float64, labels map[string]string) error

	// GetMetrics retrieves current aggregated metrics.
	// Returns a snapshot of all collected metrics data.
	GetMetrics(ctx context.Context) (*Metrics, error)

	// Reset clears all collected metrics data.
	// Useful for testing or periodic metric rotation.
	Reset(ctx context.Context) error

	// Flush ensures all buffered metrics are persisted or exported.
	// Called periodically or during shutdown to prevent data loss.
	Flush(ctx context.Context) error
}

// RequestMetric contains detailed information about a single API request
// processed by a virtual workspace.
type RequestMetric struct {
	// Workspace that processed the request
	Workspace string

	// Method is the HTTP method used (GET, POST, PUT, DELETE, etc.)
	Method string

	// Path is the API endpoint path that was accessed
	Path string

	// StatusCode is the HTTP response status code
	StatusCode int

	// Latency is the total request processing time
	Latency time.Duration

	// Size is the response payload size in bytes
	Size int64

	// UserAgent identifies the client making the request
	UserAgent string

	// Timestamp records when the request occurred
	Timestamp time.Time

	// Resource identifies the Kubernetes resource accessed
	Resource string

	// Verb is the Kubernetes API verb (get, list, create, update, delete, watch)
	Verb string
}

// Metrics contains aggregated metric data for workspace operations.
// Provides comprehensive operational visibility and performance insights.
type Metrics struct {
	// RequestCount is the total number of API requests processed
	RequestCount int64

	// ErrorCount is the total number of errors encountered
	ErrorCount int64

	// LatencyP50 is the 50th percentile request latency
	LatencyP50 time.Duration

	// LatencyP95 is the 95th percentile request latency
	LatencyP95 time.Duration

	// LatencyP99 is the 99th percentile request latency
	LatencyP99 time.Duration

	// BytesIn is the total bytes received in requests
	BytesIn int64

	// BytesOut is the total bytes sent in responses
	BytesOut int64

	// ActiveConnections is the current number of active connections
	ActiveConnections int64

	// WorkspaceCount is the current number of active workspaces
	WorkspaceCount int64

	// ErrorRate is the percentage of requests that resulted in errors
	ErrorRate float64

	// ThroughputRPS is the requests per second throughput
	ThroughputRPS float64

	// LastUpdate indicates when these metrics were last calculated
	LastUpdate time.Time
}

// MetricsExporter exports metrics to external monitoring systems.
// Supports integration with Prometheus, CloudWatch, and other platforms.
type MetricsExporter interface {
	// Export sends current metrics to the configured external system.
	// Called periodically based on the flush interval configuration.
	Export(ctx context.Context, metrics *Metrics) error

	// Configure updates the exporter configuration.
	// Allows runtime reconfiguration without restart.
	Configure(ctx context.Context, config map[string]interface{}) error

	// HealthCheck verifies the exporter can reach its target system.
	// Used for monitoring the health of the metrics pipeline.
	HealthCheck(ctx context.Context) error

	// Close cleanly shuts down the exporter and flushes any pending data.
	Close(ctx context.Context) error
}

// MetricsConfig configures metrics collection behavior and export settings.
type MetricsConfig struct {
	// Enabled controls whether metrics collection is active
	Enabled bool

	// SampleRate is the fraction of requests to sample (0.0-1.0)
	// 1.0 means sample all requests, 0.1 means sample 10%
	SampleRate float64

	// FlushInterval determines how often to export metrics
	FlushInterval time.Duration

	// BufferSize is the maximum number of metrics to buffer
	// before forcing a flush operation
	BufferSize int

	// Exporters lists the enabled export destinations
	Exporters []string

	// Labels are default labels applied to all metrics
	Labels map[string]string

	// HistogramBuckets define latency histogram bucket boundaries
	HistogramBuckets []float64

	// RetentionPeriod is how long to keep metrics in memory
	RetentionPeriod time.Duration

	// EnableDetailedMetrics includes additional detailed metrics
	// that may have higher memory overhead
	EnableDetailedMetrics bool
}

// MetricType represents different categories of metrics.
type MetricType string

const (
	// MetricTypeCounter represents cumulative metrics that only increase
	MetricTypeCounter MetricType = "counter"

	// MetricTypeGauge represents metrics that can increase or decrease
	MetricTypeGauge MetricType = "gauge"

	// MetricTypeHistogram represents metrics that track distributions
	MetricTypeHistogram MetricType = "histogram"

	// MetricTypeSummary represents metrics with quantile calculations
	MetricTypeSummary MetricType = "summary"
)

// MetricLabel represents a label attached to metrics for dimensionality.
type MetricLabel struct {
	// Name is the label key
	Name string

	// Value is the label value
	Value string
}

// MetricRegistry provides a central registry for workspace metrics.
// Manages metric definitions, collection, and aggregation.
type MetricRegistry interface {
	// RegisterMetric defines a new metric for collection.
	// Returns an error if a metric with the same name already exists.
	RegisterMetric(name string, metricType MetricType, description string) error

	// UnregisterMetric removes a metric from collection.
	// Existing metric data is preserved until the next reset.
	UnregisterMetric(name string) error

	// ListMetrics returns all registered metric definitions.
	ListMetrics() []MetricDefinition

	// GetCollector returns a metrics collector for the registry.
	GetCollector() MetricsCollector
}

// MetricDefinition describes a metric's properties and behavior.
type MetricDefinition struct {
	// Name is the unique metric identifier
	Name string

	// Type specifies the metric type (counter, gauge, histogram, summary)
	Type MetricType

	// Description provides human-readable metric documentation
	Description string

	// Labels lists the expected label names for this metric
	Labels []string

	// Unit specifies the metric unit (bytes, seconds, requests, etc.)
	Unit string

	// CreatedAt records when the metric was registered
	CreatedAt time.Time
}