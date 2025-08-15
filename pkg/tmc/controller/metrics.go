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

package controller

import (
	"context"
	"time"
)

// MetricsCollector collects controller metrics for observability.
// Implementations MUST be thread-safe as metrics collection happens
// concurrently across multiple reconciliation loops.
type MetricsCollector interface {
	// RecordReconcile records the duration and result of a reconciliation attempt.
	// The result should be a standard string like "success", "error", "requeue".
	// Duration should be the total time spent in the reconcile function.
	RecordReconcile(controllerName string, duration time.Duration, result string)

	// RecordError increments the error counter for the given controller and error type.
	// The errorType should be a categorized error type like "api_error", "validation_error".
	// This helps in understanding the distribution of different error types.
	RecordError(controllerName string, errorType string)

	// RecordQueueDepth records the current depth of the work queue.
	// This should be called periodically to track queue backlog and processing capacity.
	RecordQueueDepth(controllerName string, depth int)

	// RecordLeaderElection records leader election events.
	// The event should be "acquired", "lost", or "renewed".
	RecordLeaderElection(controllerName string, event string)

	// GetMetrics returns a snapshot of current metrics.
	// This provides programmatic access to metrics for debugging and testing.
	// The returned map should be safe to modify (i.e., a copy).
	GetMetrics() map[string]interface{}

	// Reset clears all collected metrics.
	// This is primarily useful for testing scenarios.
	Reset()
}

// MetricsExporter handles the export of metrics to external systems.
// This abstraction allows for different metrics backends (Prometheus, etc.)
// without coupling controllers to specific metrics implementations.
type MetricsExporter interface {
	// Export exports metrics to the configured backend.
	// This may be called periodically or on-demand depending on implementation.
	// Returns an error if export fails.
	Export(ctx context.Context) error

	// RegisterCollector registers a metrics collector with this exporter.
	// The exporter will periodically collect metrics from registered collectors.
	RegisterCollector(collector MetricsCollector) error

	// UnregisterCollector removes a previously registered collector.
	UnregisterCollector(collector MetricsCollector) error

	// GetEndpoint returns the metrics endpoint URL if applicable.
	// This is useful for Prometheus-style pull-based metrics systems.
	// Returns empty string if not applicable (e.g., for push-based systems).
	GetEndpoint() string

	// Start begins the metrics export process.
	// This typically starts background processes for periodic export.
	// Should be non-blocking and return quickly.
	Start(ctx context.Context) error

	// Stop gracefully shuts down the metrics export process.
	// Should wait for in-flight exports to complete.
	Stop() error
}

// MetricsRegistry manages multiple metrics collectors and exporters.
// This provides a centralized way to coordinate metrics collection across
// multiple controllers and export to multiple backends.
type MetricsRegistry interface {
	// RegisterCollector registers a new metrics collector.
	// Returns an ID that can be used to unregister the collector later.
	RegisterCollector(name string, collector MetricsCollector) (string, error)

	// UnregisterCollector removes a collector by its ID.
	UnregisterCollector(id string) error

	// RegisterExporter registers a new metrics exporter.
	// Returns an ID that can be used to unregister the exporter later.
	RegisterExporter(name string, exporter MetricsExporter) (string, error)

	// UnregisterExporter removes an exporter by its ID.
	UnregisterExporter(id string) error

	// GetCollector returns a collector by name.
	GetCollector(name string) (MetricsCollector, error)

	// GetExporter returns an exporter by name.
	GetExporter(name string) (MetricsExporter, error)

	// Start starts all registered exporters.
	Start(ctx context.Context) error

	// Stop stops all registered exporters.
	Stop() error
}