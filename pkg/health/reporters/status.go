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
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/kcp-dev/kcp/pkg/health"
)

// HealthStatusReporter provides detailed health status reporting for TMC components.
type HealthStatusReporter struct {
	aggregator health.HealthAggregator
	timeout    time.Duration
}

// NewHealthStatusReporter creates a new health status reporter.
func NewHealthStatusReporter(aggregator health.HealthAggregator, timeout time.Duration) *HealthStatusReporter {
	return &HealthStatusReporter{
		aggregator: aggregator,
		timeout:    timeout,
	}
}

// ServeHTTP implements the http.Handler interface for the health status endpoint.
func (h *HealthStatusReporter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()
	
	// Check if a specific component is requested
	component := r.URL.Query().Get("component")
	if component != "" {
		h.serveComponentHealth(w, r, ctx, component)
		return
	}
	
	// Return overall system health
	h.serveSystemHealth(w, r, ctx)
}

// serveComponentHealth serves health status for a specific component.
func (h *HealthStatusReporter) serveComponentHealth(w http.ResponseWriter, r *http.Request, ctx context.Context, componentName string) {
	status, err := h.aggregator.CheckComponent(ctx, componentName)
	if err != nil {
		http.Error(w, fmt.Sprintf("Component not found: %s", componentName), http.StatusNotFound)
		return
	}
	
	w.Header().Set("Content-Type", "text/plain")
	if status.Healthy {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	
	fmt.Fprintf(w, "Component: %s\n", componentName)
	fmt.Fprintf(w, "Status: %s\n", getStatusText(status.Healthy))
	fmt.Fprintf(w, "Message: %s\n", status.Message)
	fmt.Fprintf(w, "Checked: %s\n", status.Timestamp.Format(time.RFC3339))
	
	if len(status.Details) > 0 {
		fmt.Fprintf(w, "\nDetails:\n")
		h.writeDetailsText(w, status.Details, "  ")
	}
}

// serveSystemHealth serves overall system health status.
func (h *HealthStatusReporter) serveSystemHealth(w http.ResponseWriter, r *http.Request, ctx context.Context) {
	systemStatus := h.aggregator.CheckAll(ctx)
	
	w.Header().Set("Content-Type", "text/plain")
	if systemStatus.Healthy {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	
	fmt.Fprintf(w, "System Health: %s\n", getStatusText(systemStatus.Healthy))
	fmt.Fprintf(w, "Message: %s\n", systemStatus.Message)
	fmt.Fprintf(w, "Checked: %s\n", systemStatus.Timestamp.Format(time.RFC3339))
	fmt.Fprintf(w, "Components: %d healthy, %d total\n", systemStatus.HealthyCount, systemStatus.TotalCount)
	
	if len(systemStatus.Components) > 0 {
		fmt.Fprintf(w, "\nComponent Status:\n")
		
		// Sort components by name for consistent output
		var componentNames []string
		for name := range systemStatus.Components {
			componentNames = append(componentNames, name)
		}
		sort.Strings(componentNames)
		
		for _, name := range componentNames {
			status := systemStatus.Components[name]
			fmt.Fprintf(w, "  %s: %s - %s (checked %s)\n", 
				name, 
				getStatusText(status.Healthy),
				status.Message,
				status.Timestamp.Format("15:04:05"))
		}
	}
}

// writeDetailsText writes health details as formatted text.
func (h *HealthStatusReporter) writeDetailsText(w http.ResponseWriter, details map[string]interface{}, indent string) {
	// Sort keys for consistent output
	var keys []string
	for key := range details {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	
	for _, key := range keys {
		value := details[key]
		switch v := value.(type) {
		case map[string]interface{}:
			fmt.Fprintf(w, "%s%s:\n", indent, key)
			h.writeDetailsText(w, v, indent+"  ")
		case time.Time:
			fmt.Fprintf(w, "%s%s: %s\n", indent, key, v.Format(time.RFC3339))
		case time.Duration:
			fmt.Fprintf(w, "%s%s: %v\n", indent, key, v)
		case float64:
			fmt.Fprintf(w, "%s%s: %.2f\n", indent, key, v)
		default:
			fmt.Fprintf(w, "%s%s: %v\n", indent, key, v)
		}
	}
}

// getStatusText returns a human-readable status text.
func getStatusText(healthy bool) string {
	if healthy {
		return "HEALTHY"
	}
	return "UNHEALTHY"
}

// DetailedHealthStatusReporter provides more detailed health reporting with metrics.
type DetailedHealthStatusReporter struct {
	*HealthStatusReporter
	includeMetrics bool
}

// NewDetailedHealthStatusReporter creates a new detailed health status reporter.
func NewDetailedHealthStatusReporter(aggregator health.HealthAggregator, timeout time.Duration, includeMetrics bool) *DetailedHealthStatusReporter {
	return &DetailedHealthStatusReporter{
		HealthStatusReporter: NewHealthStatusReporter(aggregator, timeout),
		includeMetrics:       includeMetrics,
	}
}

// ServeHTTP implements the http.Handler interface with enhanced detail reporting.
func (d *DetailedHealthStatusReporter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Check for verbose mode
	verbose := r.URL.Query().Get("verbose") == "true"
	if verbose {
		d.serveVerboseHealth(w, r)
		return
	}
	
	// Fall back to standard reporting
	d.HealthStatusReporter.ServeHTTP(w, r)
}

// serveVerboseHealth serves verbose health status information.
func (d *DetailedHealthStatusReporter) serveVerboseHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), d.timeout)
	defer cancel()
	
	systemStatus := d.aggregator.CheckAll(ctx)
	
	w.Header().Set("Content-Type", "text/plain")
	if systemStatus.Healthy {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	
	fmt.Fprintf(w, "=== TMC System Health Report ===\n")
	fmt.Fprintf(w, "Overall Status: %s\n", getStatusText(systemStatus.Healthy))
	fmt.Fprintf(w, "Message: %s\n", systemStatus.Message)
	fmt.Fprintf(w, "Report Time: %s\n", systemStatus.Timestamp.Format(time.RFC3339))
	fmt.Fprintf(w, "Components Summary: %d/%d healthy (%.1f%%)\n\n", 
		systemStatus.HealthyCount, 
		systemStatus.TotalCount, 
		float64(systemStatus.HealthyCount)/float64(systemStatus.TotalCount)*100)
	
	// Group components by status
	var healthyComponents []string
	var unhealthyComponents []string
	
	for name, status := range systemStatus.Components {
		if status.Healthy {
			healthyComponents = append(healthyComponents, name)
		} else {
			unhealthyComponents = append(unhealthyComponents, name)
		}
	}
	
	// Show unhealthy components first
	if len(unhealthyComponents) > 0 {
		fmt.Fprintf(w, "UNHEALTHY COMPONENTS (%d):\n", len(unhealthyComponents))
		fmt.Fprintf(w, "%s\n", strings.Repeat("=", 40))
		sort.Strings(unhealthyComponents)
		
		for _, name := range unhealthyComponents {
			status := systemStatus.Components[name]
			fmt.Fprintf(w, "\n[UNHEALTHY] %s\n", name)
			fmt.Fprintf(w, "  Message: %s\n", status.Message)
			fmt.Fprintf(w, "  Last Check: %s\n", status.Timestamp.Format(time.RFC3339))
			
			if len(status.Details) > 0 {
				fmt.Fprintf(w, "  Details:\n")
				d.writeDetailsText(w, status.Details, "    ")
			}
		}
		fmt.Fprintf(w, "\n")
	}
	
	// Show healthy components
	if len(healthyComponents) > 0 {
		fmt.Fprintf(w, "HEALTHY COMPONENTS (%d):\n", len(healthyComponents))
		fmt.Fprintf(w, "%s\n", strings.Repeat("=", 40))
		sort.Strings(healthyComponents)
		
		for _, name := range healthyComponents {
			status := systemStatus.Components[name]
			fmt.Fprintf(w, "\n[HEALTHY] %s\n", name)
			fmt.Fprintf(w, "  Message: %s\n", status.Message)
			fmt.Fprintf(w, "  Last Check: %s\n", status.Timestamp.Format(time.RFC3339))
			
			// Only show details for healthy components if metrics are enabled
			if d.includeMetrics && len(status.Details) > 0 {
				fmt.Fprintf(w, "  Metrics:\n")
				d.writeDetailsText(w, status.Details, "    ")
			}
		}
	}
	
	fmt.Fprintf(w, "\n=== End of Health Report ===\n")
}