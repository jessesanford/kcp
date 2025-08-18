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

package metrics

import (
	"context"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"k8s.io/component-base/metrics"
	"k8s.io/klog/v2"
)

// MetricsRegistry manages all TMC metrics and their lifecycle.
// It provides centralized registration, initialization, and cleanup
// for all metrics collectors, exporters, and aggregators.
type MetricsRegistry struct {
	mu sync.RWMutex

	// Prometheus registry for custom metrics
	promRegistry *prometheus.Registry

	// OpenTelemetry meter for OTel metrics
	meter metric.Meter

	// Feature flag for metrics enablement
	enabled bool

	// Collectors store all registered metric collectors
	collectors map[string]MetricCollector

	// Exporters manage metric export to different backends
	exporters []MetricExporter
}

// MetricCollector defines the interface for all metric collectors
type MetricCollector interface {
	// Name returns the unique name of the collector
	Name() string
	
	// Init initializes the collector with the provided registry
	Init(registry *MetricsRegistry) error
	
	// Collect gathers metrics from the collector
	Collect() error
	
	// Close cleans up collector resources
	Close() error
}

// MetricExporter defines the interface for metric exporters
type MetricExporter interface {
	// Name returns the unique name of the exporter
	Name() string
	
	// Start begins metric export
	Start(ctx context.Context) error
	
	// Stop gracefully shuts down the exporter
	Stop(ctx context.Context) error
}

// Global registry instance
var (
	globalRegistry *MetricsRegistry
	registryOnce   sync.Once
)

// NewMetricsRegistry creates a new metrics registry with proper initialization.
// It sets up both Prometheus and OpenTelemetry metric collection infrastructure.
func NewMetricsRegistry(enabled bool) *MetricsRegistry {
	registry := &MetricsRegistry{
		promRegistry: prometheus.NewRegistry(),
		meter:        otel.Meter("github.com/kcp-dev/kcp/metrics"),
		enabled:      enabled,
		collectors:   make(map[string]MetricCollector),
		exporters:    make([]MetricExporter, 0),
	}
	
	klog.V(2).Infof("Created new TMC metrics registry (enabled: %v)", enabled)
	return registry
}

// GetRegistry returns the global metrics registry, creating it if necessary.
func GetRegistry() *MetricsRegistry {
	registryOnce.Do(func() {
		// Check if metrics are enabled via feature flag
		enabled := true // TODO: integrate with actual feature flag
		globalRegistry = NewMetricsRegistry(enabled)
	})
	return globalRegistry
}

// RegisterCollector adds a new metric collector to the registry.
// It initializes the collector if the registry is enabled.
func (r *MetricsRegistry) RegisterCollector(collector MetricCollector) error {
	if !r.enabled {
		klog.V(4).Infof("Metrics disabled, skipping collector registration: %s", collector.Name())
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	name := collector.Name()
	if _, exists := r.collectors[name]; exists {
		klog.Warningf("Collector %s already registered, skipping", name)
		return nil
	}

	if err := collector.Init(r); err != nil {
		return err
	}

	r.collectors[name] = collector
	klog.V(2).Infof("Registered TMC metric collector: %s", name)
	return nil
}

// RegisterExporter adds a new metric exporter to the registry.
func (r *MetricsRegistry) RegisterExporter(exporter MetricExporter) error {
	if !r.enabled {
		klog.V(4).Infof("Metrics disabled, skipping exporter registration: %s", exporter.Name())
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.exporters = append(r.exporters, exporter)
	klog.V(2).Infof("Registered TMC metric exporter: %s", exporter.Name())
	return nil
}

// GetPrometheusRegistry returns the internal Prometheus registry for custom metrics.
func (r *MetricsRegistry) GetPrometheusRegistry() *prometheus.Registry {
	return r.promRegistry
}

// GetMeter returns the OpenTelemetry meter for creating instruments.
func (r *MetricsRegistry) GetMeter() metric.Meter {
	return r.meter
}

// IsEnabled returns whether metrics collection is enabled.
func (r *MetricsRegistry) IsEnabled() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.enabled
}

// CollectAll triggers collection from all registered collectors.
// This is typically called by exporters on their collection intervals.
func (r *MetricsRegistry) CollectAll() error {
	if !r.enabled {
		return nil
	}

	r.mu.RLock()
	collectors := make([]MetricCollector, 0, len(r.collectors))
	for _, collector := range r.collectors {
		collectors = append(collectors, collector)
	}
	r.mu.RUnlock()

	for _, collector := range collectors {
		if err := collector.Collect(); err != nil {
			klog.Errorf("Failed to collect metrics from %s: %v", collector.Name(), err)
		}
	}

	return nil
}

// Start begins metric collection and export for all registered exporters.
func (r *MetricsRegistry) Start(ctx context.Context) error {
	if !r.enabled {
		klog.V(2).Info("TMC metrics disabled, skipping startup")
		return nil
	}

	r.mu.RLock()
	exporters := make([]MetricExporter, len(r.exporters))
	copy(exporters, r.exporters)
	r.mu.RUnlock()

	for _, exporter := range exporters {
		if err := exporter.Start(ctx); err != nil {
			klog.Errorf("Failed to start metric exporter %s: %v", exporter.Name(), err)
			return err
		}
		klog.V(2).Infof("Started TMC metric exporter: %s", exporter.Name())
	}

	klog.Info("TMC metrics system started successfully")
	return nil
}

// Stop gracefully shuts down all metric exporters and collectors.
func (r *MetricsRegistry) Stop(ctx context.Context) error {
	if !r.enabled {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Stop exporters first
	for _, exporter := range r.exporters {
		if err := exporter.Stop(ctx); err != nil {
			klog.Errorf("Failed to stop metric exporter %s: %v", exporter.Name(), err)
		}
	}

	// Stop collectors
	for name, collector := range r.collectors {
		if err := collector.Close(); err != nil {
			klog.Errorf("Failed to close metric collector %s: %v", name, err)
		}
	}

	klog.Info("TMC metrics system stopped successfully")
	return nil
}

// MustRegister is a convenience function for registering metrics that should never fail.
// It panics if registration fails, similar to Prometheus MustRegister.
func (r *MetricsRegistry) MustRegister(collectors ...prometheus.Collector) {
	r.promRegistry.MustRegister(collectors...)
}

// Register provides safe registration of Prometheus collectors.
func (r *MetricsRegistry) Register(collector prometheus.Collector) error {
	return r.promRegistry.Register(collector)
}