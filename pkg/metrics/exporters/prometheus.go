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

package exporters

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/kcp/pkg/metrics"
)

// PrometheusExporter implements the MetricExporter interface for Prometheus metrics.
// It provides HTTP endpoints for metrics scraping and JSON export.
type PrometheusExporter struct {
	mu sync.RWMutex

	// Configuration
	address      string
	port         int
	metricsPath  string
	jsonPath     string
	
	// HTTP server
	server   *http.Server
	mux      *http.ServeMux
	
	// Registry reference
	registry *metrics.MetricsRegistry
	
	// Collection interval for push-based metrics
	collectInterval time.Duration
	collectTicker   *time.Ticker
	stopChan        chan struct{}
}

// PrometheusExporterOptions configures the Prometheus exporter.
type PrometheusExporterOptions struct {
	Address         string
	Port            int
	MetricsPath     string
	JSONPath        string
	CollectInterval time.Duration
}

// DefaultPrometheusExporterOptions returns default configuration for the Prometheus exporter.
func DefaultPrometheusExporterOptions() *PrometheusExporterOptions {
	return &PrometheusExporterOptions{
		Address:         "0.0.0.0",
		Port:            8080,
		MetricsPath:     "/metrics",
		JSONPath:        "/metrics/json",
		CollectInterval: 30 * time.Second,
	}
}

// NewPrometheusExporter creates a new Prometheus metrics exporter.
func NewPrometheusExporter(registry *metrics.MetricsRegistry, opts *PrometheusExporterOptions) *PrometheusExporter {
	if opts == nil {
		opts = DefaultPrometheusExporterOptions()
	}

	exporter := &PrometheusExporter{
		address:         opts.Address,
		port:            opts.Port,
		metricsPath:     opts.MetricsPath,
		jsonPath:        opts.JSONPath,
		registry:        registry,
		collectInterval: opts.CollectInterval,
		stopChan:        make(chan struct{}),
	}

	exporter.setupHTTPServer()
	return exporter
}

// Name returns the exporter name.
func (e *PrometheusExporter) Name() string {
	return "prometheus"
}

// setupHTTPServer configures the HTTP server with metrics endpoints.
func (e *PrometheusExporter) setupHTTPServer() {
	e.mux = http.NewServeMux()

	// Standard Prometheus metrics endpoint
	e.mux.Handle(e.metricsPath, promhttp.HandlerFor(
		e.registry.GetPrometheusRegistry(),
		promhttp.HandlerOpts{
			EnableOpenMetrics: true,
			Registry:          e.registry.GetPrometheusRegistry(),
		},
	))

	// JSON metrics endpoint for programmatic access
	e.mux.HandleFunc(e.jsonPath, e.handleJSONMetrics)

	// Health check endpoint
	e.mux.HandleFunc("/health", e.handleHealth)

	// Metrics metadata endpoint
	e.mux.HandleFunc("/metrics/metadata", e.handleMetricsMetadata)

	e.server = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", e.address, e.port),
		Handler: e.mux,
		// Security and performance settings
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}

// Start begins the Prometheus metrics export.
func (e *PrometheusExporter) Start(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Start periodic collection
	e.collectTicker = time.NewTicker(e.collectInterval)
	go e.collectLoop(ctx)

	// Start HTTP server
	go func() {
		klog.Infof("Starting Prometheus metrics server on %s:%d", e.address, e.port)
		klog.Infof("Metrics available at: http://%s:%d%s", e.address, e.port, e.metricsPath)
		klog.Infof("JSON metrics available at: http://%s:%d%s", e.address, e.port, e.jsonPath)
		
		if err := e.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			klog.Errorf("Prometheus metrics server failed: %v", err)
		}
	}()

	return nil
}

// Stop gracefully shuts down the Prometheus exporter.
func (e *PrometheusExporter) Stop(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Stop collection loop
	if e.collectTicker != nil {
		e.collectTicker.Stop()
	}
	close(e.stopChan)

	// Shutdown HTTP server
	if e.server != nil {
		return e.server.Shutdown(ctx)
	}

	klog.Info("Prometheus exporter stopped")
	return nil
}

// collectLoop periodically triggers metric collection from all collectors.
func (e *PrometheusExporter) collectLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-e.stopChan:
			return
		case <-e.collectTicker.C:
			if err := e.registry.CollectAll(); err != nil {
				klog.Errorf("Failed to collect metrics: %v", err)
			}
		}
	}
}

// handleJSONMetrics serves metrics in JSON format for programmatic access.
func (e *PrometheusExporter) handleJSONMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	// Collect metrics from all collectors
	if err := e.registry.CollectAll(); err != nil {
		klog.Errorf("Failed to collect metrics for JSON export: %v", err)
		http.Error(w, "Failed to collect metrics", http.StatusInternalServerError)
		return
	}

	// Gather metrics from Prometheus registry
	metricFamilies, err := e.registry.GetPrometheusRegistry().Gather()
	if err != nil {
		klog.Errorf("Failed to gather Prometheus metrics: %v", err)
		http.Error(w, "Failed to gather metrics", http.StatusInternalServerError)
		return
	}

	// Convert to JSON-friendly format
	jsonMetrics := make(map[string]interface{})
	
	for _, mf := range metricFamilies {
		if mf.GetName() == "" {
			continue
		}
		
		familyData := map[string]interface{}{
			"name": mf.GetName(),
			"help": mf.GetHelp(),
			"type": mf.GetType().String(),
			"metrics": convertMetricsToJSON(mf.GetMetric()),
		}
		
		jsonMetrics[mf.GetName()] = familyData
	}

	// Add metadata
	response := map[string]interface{}{
		"timestamp": time.Now().Unix(),
		"metrics":   jsonMetrics,
		"metadata": map[string]interface{}{
			"exporter": "tmc-prometheus",
			"version":  "v1",
		},
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		klog.Errorf("Failed to encode JSON metrics: %v", err)
		http.Error(w, "Failed to encode metrics", http.StatusInternalServerError)
	}
}

// handleHealth serves a simple health check endpoint.
func (e *PrometheusExporter) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	status := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
		"exporter":  e.Name(),
	}
	
	json.NewEncoder(w).Encode(status)
}

// handleMetricsMetadata serves metadata about available metrics.
func (e *PrometheusExporter) handleMetricsMetadata(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	metadata := map[string]interface{}{
		"collectors": []string{"syncer", "placement", "cluster", "connection"},
		"endpoints": map[string]string{
			"prometheus": e.metricsPath,
			"json":       e.jsonPath,
			"health":     "/health",
			"metadata":   "/metrics/metadata",
		},
		"collection_interval": e.collectInterval.String(),
		"timestamp":          time.Now().Unix(),
	}
	
	json.NewEncoder(w).Encode(metadata)
}

// convertMetricsToJSON converts Prometheus metric DTOs to JSON-friendly format.
func convertMetricsToJSON(metrics []*dto.Metric) []map[string]interface{} {
	var result []map[string]interface{}
	
	for _, metric := range metrics {
		m := make(map[string]interface{})
		
		// Add labels
		if len(metric.GetLabel()) > 0 {
			labels := make(map[string]string)
			for _, label := range metric.GetLabel() {
				labels[label.GetName()] = label.GetValue()
			}
			m["labels"] = labels
		}
		
		// Add timestamp if available
		if metric.GetTimestampMs() != 0 {
			m["timestamp"] = metric.GetTimestampMs()
		}
		
		// Add value based on metric type
		if counter := metric.GetCounter(); counter != nil {
			m["value"] = counter.GetValue()
		} else if gauge := metric.GetGauge(); gauge != nil {
			m["value"] = gauge.GetValue()
		} else if histogram := metric.GetHistogram(); histogram != nil {
			m["sample_count"] = histogram.GetSampleCount()
			m["sample_sum"] = histogram.GetSampleSum()
			
			buckets := make([]map[string]interface{}, len(histogram.GetBucket()))
			for i, bucket := range histogram.GetBucket() {
				buckets[i] = map[string]interface{}{
					"upper_bound":      bucket.GetUpperBound(),
					"cumulative_count": bucket.GetCumulativeCount(),
				}
			}
			m["buckets"] = buckets
		} else if summary := metric.GetSummary(); summary != nil {
			m["sample_count"] = summary.GetSampleCount()
			m["sample_sum"] = summary.GetSampleSum()
			
			quantiles := make([]map[string]interface{}, len(summary.GetQuantile()))
			for i, quantile := range summary.GetQuantile() {
				quantiles[i] = map[string]interface{}{
					"quantile": quantile.GetQuantile(),
					"value":    quantile.GetValue(),
				}
			}
			m["quantiles"] = quantiles
		}
		
		result = append(result, m)
	}
	
	return result
}