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
	"fmt"
	"testing"
	"time"
)

// MockMetricCollector is a test implementation of MetricCollector.
type MockMetricCollector struct {
	name        string
	initialized bool
	collected   bool
	closed      bool
}

func NewMockMetricCollector(name string) *MockMetricCollector {
	return &MockMetricCollector{
		name: name,
	}
}

func (m *MockMetricCollector) Name() string {
	return m.name
}

func (m *MockMetricCollector) Init(registry *MetricsRegistry) error {
	m.initialized = true
	return nil
}

func (m *MockMetricCollector) Collect() error {
	m.collected = true
	return nil
}

func (m *MockMetricCollector) Close() error {
	m.closed = true
	return nil
}

// MockMetricExporter is a test implementation of MetricExporter.
type MockMetricExporter struct {
	name    string
	started bool
	stopped bool
}

func NewMockMetricExporter(name string) *MockMetricExporter {
	return &MockMetricExporter{
		name: name,
	}
}

func (m *MockMetricExporter) Name() string {
	return m.name
}

func (m *MockMetricExporter) Start(ctx context.Context) error {
	m.started = true
	return nil
}

func (m *MockMetricExporter) Stop(ctx context.Context) error {
	m.stopped = true
	return nil
}

// TestNewMetricsRegistry tests the creation of a new metrics registry.
func TestNewMetricsRegistry(t *testing.T) {
	tests := map[string]struct {
		enabled bool
	}{
		"enabled registry": {
			enabled: true,
		},
		"disabled registry": {
			enabled: false,
		},
	}
	
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			registry := NewMetricsRegistry(tc.enabled)
			
			if registry == nil {
				t.Fatal("Expected non-nil registry")
			}
			
			if registry.IsEnabled() != tc.enabled {
				t.Errorf("Expected enabled=%v, got %v", tc.enabled, registry.IsEnabled())
			}
			
			if registry.GetPrometheusRegistry() == nil {
				t.Error("Expected non-nil Prometheus registry")
			}
			
			if registry.GetMeter() == nil {
				t.Error("Expected non-nil OpenTelemetry meter")
			}
		})
	}
}

// TestMetricsRegistryCollectorOperations tests collector registration and operations.
func TestMetricsRegistryCollectorOperations(t *testing.T) {
	registry := NewMetricsRegistry(true)
	
	// Test collector registration
	collector := NewMockMetricCollector("test-collector")
	err := registry.RegisterCollector(collector)
	if err != nil {
		t.Fatalf("Failed to register collector: %v", err)
	}
	
	if !collector.initialized {
		t.Error("Expected collector to be initialized")
	}
	
	// Test duplicate registration
	err = registry.RegisterCollector(collector)
	if err != nil {
		t.Errorf("Unexpected error on duplicate registration: %v", err)
	}
	
	// Test collection
	err = registry.CollectAll()
	if err != nil {
		t.Fatalf("Failed to collect metrics: %v", err)
	}
	
	if !collector.collected {
		t.Error("Expected collector to be called")
	}
	
	// Test stop
	ctx := context.Background()
	err = registry.Stop(ctx)
	if err != nil {
		t.Fatalf("Failed to stop registry: %v", err)
	}
	
	if !collector.closed {
		t.Error("Expected collector to be closed")
	}
}

// TestMetricsRegistryExporterOperations tests exporter registration and operations.
func TestMetricsRegistryExporterOperations(t *testing.T) {
	registry := NewMetricsRegistry(true)
	
	// Test exporter registration
	exporter := NewMockMetricExporter("test-exporter")
	err := registry.RegisterExporter(exporter)
	if err != nil {
		t.Fatalf("Failed to register exporter: %v", err)
	}
	
	// Test start
	ctx := context.Background()
	err = registry.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start registry: %v", err)
	}
	
	if !exporter.started {
		t.Error("Expected exporter to be started")
	}
	
	// Test stop
	err = registry.Stop(ctx)
	if err != nil {
		t.Fatalf("Failed to stop registry: %v", err)
	}
	
	if !exporter.stopped {
		t.Error("Expected exporter to be stopped")
	}
}

// TestMetricsRegistryDisabled tests behavior when metrics are disabled.
func TestMetricsRegistryDisabled(t *testing.T) {
	registry := NewMetricsRegistry(false)
	
	// Test collector registration with disabled registry
	collector := NewMockMetricCollector("test-collector")
	err := registry.RegisterCollector(collector)
	if err != nil {
		t.Errorf("Unexpected error registering collector: %v", err)
	}
	
	// Collector should not be initialized when registry is disabled
	if collector.initialized {
		t.Error("Expected collector not to be initialized when registry is disabled")
	}
	
	// Test exporter registration with disabled registry
	exporter := NewMockMetricExporter("test-exporter")
	err = registry.RegisterExporter(exporter)
	if err != nil {
		t.Errorf("Unexpected error registering exporter: %v", err)
	}
	
	// Test start with disabled registry
	ctx := context.Background()
	err = registry.Start(ctx)
	if err != nil {
		t.Errorf("Unexpected error starting disabled registry: %v", err)
	}
	
	// Exporter should not be started when registry is disabled
	if exporter.started {
		t.Error("Expected exporter not to be started when registry is disabled")
	}
}

// TestGetRegistry tests the global registry singleton.
func TestGetRegistry(t *testing.T) {
	// Get registry twice and ensure they're the same instance
	registry1 := GetRegistry()
	registry2 := GetRegistry()
	
	if registry1 != registry2 {
		t.Error("Expected GetRegistry to return the same instance")
	}
	
	if registry1 == nil {
		t.Error("Expected non-nil registry from GetRegistry")
	}
}

// BenchmarkMetricsCollection benchmarks the metrics collection process.
func BenchmarkMetricsCollection(b *testing.B) {
	registry := NewMetricsRegistry(true)
	
	// Register multiple collectors
	for i := 0; i < 10; i++ {
		collector := NewMockMetricCollector(fmt.Sprintf("collector-%d", i))
		registry.RegisterCollector(collector)
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		registry.CollectAll()
	}
}

// TestMetricsLifecycle tests the complete lifecycle of the metrics system.
func TestMetricsLifecycle(t *testing.T) {
	registry := NewMetricsRegistry(true)
	
	// Register collector and exporter
	collector := NewMockMetricCollector("lifecycle-collector")
	exporter := NewMockMetricExporter("lifecycle-exporter")
	
	err := registry.RegisterCollector(collector)
	if err != nil {
		t.Fatalf("Failed to register collector: %v", err)
	}
	
	err = registry.RegisterExporter(exporter)
	if err != nil {
		t.Fatalf("Failed to register exporter: %v", err)
	}
	
	// Start the registry
	ctx := context.Background()
	err = registry.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start registry: %v", err)
	}
	
	// Verify components are started
	if !exporter.started {
		t.Error("Expected exporter to be started")
	}
	
	// Collect metrics
	err = registry.CollectAll()
	if err != nil {
		t.Fatalf("Failed to collect metrics: %v", err)
	}
	
	if !collector.collected {
		t.Error("Expected collector to be called")
	}
	
	// Stop the registry
	err = registry.Stop(ctx)
	if err != nil {
		t.Fatalf("Failed to stop registry: %v", err)
	}
	
	// Verify components are stopped
	if !exporter.stopped {
		t.Error("Expected exporter to be stopped")
	}
	
	if !collector.closed {
		t.Error("Expected collector to be closed")
	}
}