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

package reporters

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/kcp-dev/kcp/pkg/health"
)

// JSONHealthReporter provides JSON-formatted health status reporting for TMC components.
type JSONHealthReporter struct {
	aggregator health.HealthAggregator
	timeout    time.Duration
}

// NewJSONHealthReporter creates a new JSON health status reporter.
func NewJSONHealthReporter(aggregator health.HealthAggregator, timeout time.Duration) *JSONHealthReporter {
	return &JSONHealthReporter{
		aggregator: aggregator,
		timeout:    timeout,
	}
}

// ServeHTTP implements the http.Handler interface for JSON health status endpoint.
func (j *JSONHealthReporter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), j.timeout)
	defer cancel()
	
	// Check if a specific component is requested
	component := r.URL.Query().Get("component")
	if component != "" {
		j.serveComponentHealthJSON(w, r, ctx, component)
		return
	}
	
	// Return overall system health
	j.serveSystemHealthJSON(w, r, ctx)
}

// serveComponentHealthJSON serves health status for a specific component in JSON format.
func (j *JSONHealthReporter) serveComponentHealthJSON(w http.ResponseWriter, r *http.Request, ctx context.Context, componentName string) {
	status, err := j.aggregator.CheckComponent(ctx, componentName)
	if err != nil {
		j.writeErrorJSON(w, http.StatusNotFound, fmt.Sprintf("Component not found: %s", componentName))
		return
	}
	
	response := ComponentHealthResponse{
		Component: componentName,
		Status:    status,
	}
	
	j.writeHealthJSON(w, status.Healthy, response)
}

// serveSystemHealthJSON serves overall system health status in JSON format.
func (j *JSONHealthReporter) serveSystemHealthJSON(w http.ResponseWriter, r *http.Request, ctx context.Context) {
	systemStatus := j.aggregator.CheckAll(ctx)
	
	response := SystemHealthResponse{
		Overall: systemStatus,
	}
	
	j.writeHealthJSON(w, systemStatus.Healthy, response)
}

// writeHealthJSON writes a health response as JSON.
func (j *JSONHealthReporter) writeHealthJSON(w http.ResponseWriter, healthy bool, response interface{}) {
	w.Header().Set("Content-Type", "application/json")
	
	if healthy {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(response); err != nil {
		j.writeErrorJSON(w, http.StatusInternalServerError, "Failed to encode JSON response")
	}
}

// writeErrorJSON writes an error response as JSON.
func (j *JSONHealthReporter) writeErrorJSON(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	
	errorResponse := ErrorResponse{
		Error:     message,
		Timestamp: time.Now(),
	}
	
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	encoder.Encode(errorResponse)
}

// ComponentHealthResponse represents the JSON response for a single component's health.
type ComponentHealthResponse struct {
	Component string              `json:"component"`
	Status    health.HealthStatus `json:"status"`
}

// SystemHealthResponse represents the JSON response for overall system health.
type SystemHealthResponse struct {
	Overall health.SystemHealthStatus `json:"overall"`
}

// ErrorResponse represents an error response in JSON format.
type ErrorResponse struct {
	Error     string    `json:"error"`
	Timestamp time.Time `json:"timestamp"`
}

// PrometheusHealthReporter provides Prometheus-compatible metrics for health status.
type PrometheusHealthReporter struct {
	aggregator health.HealthAggregator
	timeout    time.Duration
}

// NewPrometheusHealthReporter creates a new Prometheus health metrics reporter.
func NewPrometheusHealthReporter(aggregator health.HealthAggregator, timeout time.Duration) *PrometheusHealthReporter {
	return &PrometheusHealthReporter{
		aggregator: aggregator,
		timeout:    timeout,
	}
}

// ServeHTTP implements the http.Handler interface for Prometheus metrics endpoint.
func (p *PrometheusHealthReporter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), p.timeout)
	defer cancel()
	
	systemStatus := p.aggregator.CheckAll(ctx)
	
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	
	// Overall system health metric
	fmt.Fprintf(w, "# HELP kcp_health_status Overall health status of KCP system (1=healthy, 0=unhealthy)\n")
	fmt.Fprintf(w, "# TYPE kcp_health_status gauge\n")
	healthValue := 0
	if systemStatus.Healthy {
		healthValue = 1
	}
	fmt.Fprintf(w, "kcp_health_status %d\n", healthValue)
	
	// Component count metrics
	fmt.Fprintf(w, "# HELP kcp_health_components_total Total number of health-checked components\n")
	fmt.Fprintf(w, "# TYPE kcp_health_components_total gauge\n")
	fmt.Fprintf(w, "kcp_health_components_total %d\n", systemStatus.TotalCount)
	
	fmt.Fprintf(w, "# HELP kcp_health_components_healthy Number of healthy components\n")
	fmt.Fprintf(w, "# TYPE kcp_health_components_healthy gauge\n")
	fmt.Fprintf(w, "kcp_health_components_healthy %d\n", systemStatus.HealthyCount)
	
	// Individual component health metrics
	fmt.Fprintf(w, "# HELP kcp_component_health_status Health status of individual components (1=healthy, 0=unhealthy)\n")
	fmt.Fprintf(w, "# TYPE kcp_component_health_status gauge\n")
	
	for componentName, status := range systemStatus.Components {
		componentHealthValue := 0
		if status.Healthy {
			componentHealthValue = 1
		}
		fmt.Fprintf(w, "kcp_component_health_status{component=\"%s\"} %d\n", componentName, componentHealthValue)
	}
	
	// Health check timestamp
	fmt.Fprintf(w, "# HELP kcp_health_check_timestamp_seconds Unix timestamp of last health check\n")
	fmt.Fprintf(w, "# TYPE kcp_health_check_timestamp_seconds gauge\n")
	fmt.Fprintf(w, "kcp_health_check_timestamp_seconds %.3f\n", float64(systemStatus.Timestamp.Unix()))
}

// CompactJSONHealthReporter provides compact JSON health reporting without details.
type CompactJSONHealthReporter struct {
	*JSONHealthReporter
}

// NewCompactJSONHealthReporter creates a new compact JSON health reporter.
func NewCompactJSONHealthReporter(aggregator health.HealthAggregator, timeout time.Duration) *CompactJSONHealthReporter {
	return &CompactJSONHealthReporter{
		JSONHealthReporter: NewJSONHealthReporter(aggregator, timeout),
	}
}

// ServeHTTP implements compact JSON health reporting.
func (c *CompactJSONHealthReporter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), c.timeout)
	defer cancel()
	
	systemStatus := c.aggregator.CheckAll(ctx)
	
	// Create compact response with minimal information
	compactResponse := CompactHealthResponse{
		Healthy:      systemStatus.Healthy,
		Message:      systemStatus.Message,
		Timestamp:    systemStatus.Timestamp,
		HealthyCount: systemStatus.HealthyCount,
		TotalCount:   systemStatus.TotalCount,
	}
	
	// Add only the status of each component, not full details
	compactResponse.Components = make(map[string]bool)
	for name, status := range systemStatus.Components {
		compactResponse.Components[name] = status.Healthy
	}
	
	c.writeHealthJSON(w, systemStatus.Healthy, compactResponse)
}

// CompactHealthResponse represents a compact JSON health response.
type CompactHealthResponse struct {
	Healthy      bool              `json:"healthy"`
	Message      string            `json:"message"`
	Timestamp    time.Time         `json:"timestamp"`
	HealthyCount int               `json:"healthy_count"`
	TotalCount   int               `json:"total_count"`
	Components   map[string]bool   `json:"components"`
}