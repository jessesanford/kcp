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

package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/component-base/metrics"
	"k8s.io/klog/v2"

	kcpmetrics "github.com/kcp-dev/kcp/pkg/metrics"
	"github.com/kcp-dev/kcp/pkg/metrics/collectors"
	"github.com/kcp-dev/kcp/pkg/metrics/exporters"
)

// MetricsServer manages TMC metrics server lifecycle and configuration.
// It integrates with the main TMC controller to provide observability.
type MetricsServer struct {
	server   *http.Server
	registry *kcpmetrics.MetricsRegistry
	enabled  bool
}

// NewMetricsServer creates a new metrics server for TMC controller.
// It configures Prometheus metrics endpoint and collector registration.
func NewMetricsServer(port int, enabled bool) *MetricsServer {
	registry := kcpmetrics.GetRegistry()
	
	mux := http.NewServeMux()
	
	// Register Prometheus handler
	mux.Handle("/metrics", promhttp.HandlerFor(
		registry.GetPrometheusRegistry(),
		promhttp.HandlerOpts{
			EnableOpenMetrics: true,
		},
	))
	
	// Health endpoint for metrics server
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})
	
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}
	
	return &MetricsServer{
		server:   server,
		registry: registry,
		enabled:  enabled,
	}
}

// Start begins the metrics server and registers TMC collectors.
func (m *MetricsServer) Start(ctx context.Context) error {
	if !m.enabled {
		klog.V(2).Info("TMC metrics disabled, skipping metrics server startup")
		return nil
	}
	
	// Register TMC-specific collectors
	if err := m.registerCollectors(); err != nil {
		return fmt.Errorf("failed to register TMC collectors: %w", err)
	}
	
	// Register Prometheus exporter
	prometheusExporter, err := exporters.NewPrometheusExporter(exporters.PrometheusConfig{
		Registry: m.registry.GetPrometheusRegistry(),
		Interval: 30 * time.Second,
	})
	if err != nil {
		return fmt.Errorf("failed to create Prometheus exporter: %w", err)
	}
	
	if err := m.registry.RegisterExporter(prometheusExporter); err != nil {
		return fmt.Errorf("failed to register Prometheus exporter: %w", err)
	}
	
	// Start metric collection
	if err := m.registry.Start(ctx); err != nil {
		return fmt.Errorf("failed to start metrics registry: %w", err)
	}
	
	// Start HTTP server
	go func() {
		klog.Infof("Starting TMC metrics server on %s", m.server.Addr)
		if err := m.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			klog.Errorf("TMC metrics server error: %v", err)
		}
	}()
	
	// Handle graceful shutdown
	go func() {
		<-ctx.Done()
		klog.Info("Shutting down TMC metrics server")
		
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		
		if err := m.server.Shutdown(shutdownCtx); err != nil {
			klog.Errorf("TMC metrics server shutdown error: %v", err)
		}
		
		if err := m.registry.Stop(shutdownCtx); err != nil {
			klog.Errorf("TMC metrics registry shutdown error: %v", err)
		}
	}()
	
	return nil
}

// Stop gracefully shuts down the metrics server.
func (m *MetricsServer) Stop(ctx context.Context) error {
	if !m.enabled {
		return nil
	}
	
	if err := m.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown metrics server: %w", err)
	}
	
	if err := m.registry.Stop(ctx); err != nil {
		return fmt.Errorf("failed to stop metrics registry: %w", err)
	}
	
	return nil
}

// registerCollectors registers all TMC-specific metric collectors.
func (m *MetricsServer) registerCollectors() error {
	// Register syncer collector for connection monitoring
	syncerCollector, err := collectors.NewSyncerCollector()
	if err != nil {
		return fmt.Errorf("failed to create syncer collector: %w", err)
	}
	
	if err := m.registry.RegisterCollector(syncerCollector); err != nil {
		return fmt.Errorf("failed to register syncer collector: %w", err)
	}
	
	// Register cluster collector for cluster health monitoring
	clusterCollector, err := collectors.NewClusterCollector()
	if err != nil {
		return fmt.Errorf("failed to create cluster collector: %w", err)
	}
	
	if err := m.registry.RegisterCollector(clusterCollector); err != nil {
		return fmt.Errorf("failed to register cluster collector: %w", err)
	}
	
	// Register connection collector for network monitoring
	connectionCollector, err := collectors.NewConnectionCollector()
	if err != nil {
		return fmt.Errorf("failed to create connection collector: %w", err)
	}
	
	if err := m.registry.RegisterCollector(connectionCollector); err != nil {
		return fmt.Errorf("failed to register connection collector: %w", err)
	}
	
	// Register placement collector for placement decision monitoring
	placementCollector, err := collectors.NewPlacementCollector()
	if err != nil {
		return fmt.Errorf("failed to create placement collector: %w", err)
	}
	
	if err := m.registry.RegisterCollector(placementCollector); err != nil {
		return fmt.Errorf("failed to register placement collector: %w", err)
	}
	
	klog.V(2).Info("Registered all TMC metric collectors")
	return nil
}

// GetMetricsRegistry returns the metrics registry for external integration.
func (m *MetricsServer) GetMetricsRegistry() *kcpmetrics.MetricsRegistry {
	return m.registry
}

// IsEnabled returns whether metrics collection is enabled.
func (m *MetricsServer) IsEnabled() bool {
	return m.enabled
}

// RecordCustomMetric provides a convenience method for recording custom metrics
// from other parts of the TMC controller.
func (m *MetricsServer) RecordCustomMetric(name string, value float64, labels prometheus.Labels) {
	if !m.enabled {
		return
	}
	
	// This would be expanded with actual custom metric recording logic
	klog.V(4).Infof("Recording custom metric %s: %f", name, value)
}

// GetPrometheusRegistry exposes the underlying Prometheus registry for
// direct metric registration if needed by other controller components.
func (m *MetricsServer) GetPrometheusRegistry() *prometheus.Registry {
	return m.registry.GetPrometheusRegistry()
}