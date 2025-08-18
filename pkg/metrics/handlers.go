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
	"encoding/json"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/kcp/pkg/metrics/exporters"
)

// HTTPHandlers provides HTTP endpoints for TMC metrics.
// It serves both Prometheus-formatted metrics and JSON endpoints
// for programmatic access and monitoring dashboards.
type HTTPHandlers struct {
	registry *MetricsRegistry
	
	// Exporters
	promExporter *exporters.PrometheusExporter
	otelExporter *exporters.OpenTelemetryExporter
}

// NewHTTPHandlers creates a new HTTP handlers instance.
func NewHTTPHandlers(registry *MetricsRegistry) *HTTPHandlers {
	handlers := &HTTPHandlers{
		registry: registry,
	}
	
	// Create exporters
	handlers.promExporter = exporters.NewPrometheusExporter(registry, nil)
	handlers.otelExporter = exporters.NewOpenTelemetryExporter(registry, nil)
	
	return handlers
}

// RegisterHandlers registers all metrics HTTP handlers with the provided ServeMux.
func (h *HTTPHandlers) RegisterHandlers(mux *http.ServeMux) {
	// Standard Prometheus metrics endpoint
	mux.Handle("/metrics", h.metricsHandler())
	
	// JSON metrics endpoint for programmatic access
	mux.HandleFunc("/metrics/json", h.jsonMetricsHandler)
	
	// Health check endpoint
	mux.HandleFunc("/health", h.healthHandler)
	
	// Metrics metadata endpoint
	mux.HandleFunc("/metrics/metadata", h.metadataHandler)
	
	// Collector-specific endpoints
	mux.HandleFunc("/metrics/syncer", h.syncerMetricsHandler)
	mux.HandleFunc("/metrics/placement", h.placementMetricsHandler)
	mux.HandleFunc("/metrics/cluster", h.clusterMetricsHandler)
	mux.HandleFunc("/metrics/connection", h.connectionMetricsHandler)
	
	klog.V(2).Info("Registered TMC metrics HTTP handlers")
}

// metricsHandler returns the Prometheus metrics handler.
func (h *HTTPHandlers) metricsHandler() http.Handler {
	return promhttp.HandlerFor(
		h.registry.GetPrometheusRegistry(),
		promhttp.HandlerOpts{
			EnableOpenMetrics: true,
			Registry:          h.registry.GetPrometheusRegistry(),
		},
	)
}

// jsonMetricsHandler serves metrics in JSON format.
func (h *HTTPHandlers) jsonMetricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	// Collect metrics from all collectors
	if err := h.registry.CollectAll(); err != nil {
		klog.Errorf("Failed to collect metrics for JSON export: %v", err)
		http.Error(w, "Failed to collect metrics", http.StatusInternalServerError)
		return
	}
	
	// Gather metrics from Prometheus registry
	metricFamilies, err := h.registry.GetPrometheusRegistry().Gather()
	if err != nil {
		klog.Errorf("Failed to gather Prometheus metrics: %v", err)
		http.Error(w, "Failed to gather metrics", http.StatusInternalServerError)
		return
	}
	
	// Convert to JSON format
	response := map[string]interface{}{
		"timestamp": time.Now().Unix(),
		"service":   "kcp-tmc",
		"version":   "v0.1.0",
		"metrics": map[string]interface{}{
			"count": len(metricFamilies),
		},
		"endpoints": map[string]string{
			"prometheus": "/metrics",
			"json":       "/metrics/json",
			"health":     "/health",
			"metadata":   "/metrics/metadata",
			"syncer":     "/metrics/syncer",
			"placement":  "/metrics/placement",
			"cluster":    "/metrics/cluster",
			"connection": "/metrics/connection",
		},
	}
	
	if err := json.NewEncoder(w).Encode(response); err != nil {
		klog.Errorf("Failed to encode JSON response: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// healthHandler serves a health check endpoint.
func (h *HTTPHandlers) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	status := "healthy"
	if !h.registry.IsEnabled() {
		status = "disabled"
	}
	
	health := map[string]interface{}{
		"status":      status,
		"timestamp":   time.Now().Unix(),
		"service":     "kcp-tmc-metrics",
		"collectors":  len(h.registry.collectors),
		"exporters":   len(h.registry.exporters),
	}
	
	json.NewEncoder(w).Encode(health)
}

// metadataHandler serves metadata about available metrics.
func (h *HTTPHandlers) metadataHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	metadata := map[string]interface{}{
		"service":    "kcp-tmc-metrics",
		"version":    "v0.1.0",
		"timestamp":  time.Now().Unix(),
		"collectors": []map[string]string{
			{"name": "syncer", "description": "TMC syncer metrics including sync latency and resource counts"},
			{"name": "placement", "description": "TMC placement metrics including decision latency and cluster selection"},
			{"name": "cluster", "description": "TMC cluster metrics including capacity and utilization"},
			{"name": "connection", "description": "TMC connection metrics including state and throughput"},
		},
		"exporters": []map[string]string{
			{"name": "prometheus", "description": "Prometheus metrics exporter"},
			{"name": "opentelemetry", "description": "OpenTelemetry traces and metrics exporter"},
		},
		"endpoints": map[string]interface{}{
			"prometheus": map[string]string{
				"path":        "/metrics",
				"format":      "prometheus",
				"description": "Standard Prometheus metrics endpoint",
			},
			"json": map[string]string{
				"path":        "/metrics/json",
				"format":      "json",
				"description": "JSON formatted metrics for programmatic access",
			},
			"health": map[string]string{
				"path":        "/health",
				"format":      "json",
				"description": "Health check endpoint",
			},
		},
	}
	
	json.NewEncoder(w).Encode(metadata)
}

// syncerMetricsHandler serves syncer-specific metrics.
func (h *HTTPHandlers) syncerMetricsHandler(w http.ResponseWriter, r *http.Request) {
	h.serveCollectorMetrics(w, r, "syncer", "TMC syncer metrics")
}

// placementMetricsHandler serves placement-specific metrics.
func (h *HTTPHandlers) placementMetricsHandler(w http.ResponseWriter, r *http.Request) {
	h.serveCollectorMetrics(w, r, "placement", "TMC placement metrics")
}

// clusterMetricsHandler serves cluster-specific metrics.
func (h *HTTPHandlers) clusterMetricsHandler(w http.ResponseWriter, r *http.Request) {
	h.serveCollectorMetrics(w, r, "cluster", "TMC cluster metrics")
}

// connectionMetricsHandler serves connection-specific metrics.
func (h *HTTPHandlers) connectionMetricsHandler(w http.ResponseWriter, r *http.Request) {
	h.serveCollectorMetrics(w, r, "connection", "TMC connection metrics")
}

// serveCollectorMetrics is a helper function to serve metrics for a specific collector.
func (h *HTTPHandlers) serveCollectorMetrics(w http.ResponseWriter, r *http.Request, collectorName, description string) {
	w.Header().Set("Content-Type", "application/json")
	
	// Check if collector exists
	h.registry.mu.RLock()
	collector, exists := h.registry.collectors[collectorName]
	h.registry.mu.RUnlock()
	
	if !exists {
		http.Error(w, "Collector not found", http.StatusNotFound)
		return
	}
	
	// Collect metrics from the specific collector
	if err := collector.Collect(); err != nil {
		klog.Errorf("Failed to collect metrics from %s: %v", collectorName, err)
		http.Error(w, "Failed to collect metrics", http.StatusInternalServerError)
		return
	}
	
	response := map[string]interface{}{
		"collector":   collectorName,
		"description": description,
		"timestamp":   time.Now().Unix(),
		"status":      "active",
		"message":     "Metrics collected successfully",
	}
	
	json.NewEncoder(w).Encode(response)
}

// StartMetricsServer starts an HTTP server serving metrics endpoints.
// This is a convenience function for quickly setting up a metrics server.
func StartMetricsServer(registry *MetricsRegistry, address string) error {
	handlers := NewHTTPHandlers(registry)
	
	mux := http.NewServeMux()
	handlers.RegisterHandlers(mux)
	
	server := &http.Server{
		Addr:    address,
		Handler: mux,
		// Security and performance settings
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	
	klog.Infof("Starting TMC metrics server on %s", address)
	klog.Infof("Metrics endpoints:")
	klog.Infof("  - Prometheus: http://%s/metrics", address)
	klog.Infof("  - JSON: http://%s/metrics/json", address)
	klog.Infof("  - Health: http://%s/health", address)
	klog.Infof("  - Metadata: http://%s/metrics/metadata", address)
	
	return server.ListenAndServe()
}