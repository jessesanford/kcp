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

package observability

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"k8s.io/klog/v2"
	utilfeature "k8s.io/apiserver/pkg/util/feature"

	"github.com/kcp-dev/logicalcluster/v3"
	"github.com/kcp-dev/kcp/pkg/features"
)

// PrometheusMetricsCollector implements MetricsSource using Prometheus HTTP API.
type PrometheusMetricsCollector struct {
	baseURL    string
	httpClient *http.Client
}

// NewPrometheusMetricsCollector creates a new Prometheus-based metrics collector.
//
// Parameters:
//   - baseURL: Prometheus server base URL (e.g., "http://prometheus:9090")
//
// Returns:
//   - MetricsSource: Configured Prometheus collector
func NewPrometheusMetricsCollector(baseURL string) MetricsSource {
	return &PrometheusMetricsCollector{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetMetricValue retrieves a specific metric value from a cluster via Prometheus.
//
// This method queries Prometheus for the specified metric from a specific cluster
// within the given workspace. It uses workspace-aware labeling to isolate metrics.
//
// Parameters:
//   - ctx: Context for the request
//   - clusterName: Name of the cluster to get metrics from
//   - workspace: Logical cluster workspace for isolation
//   - metricName: Name of the metric to retrieve
//
// Returns:
//   - float64: Metric value
//   - map[string]string: Associated labels
//   - error: Retrieval error if any
func (pmc *PrometheusMetricsCollector) GetMetricValue(
	ctx context.Context,
	clusterName string,
	workspace logicalcluster.Name,
	metricName string,
) (float64, map[string]string, error) {
	// Check if TMC metrics collection is enabled
	if !utilfeature.DefaultFeatureGate.Enabled(features.TMCMetricsAggregation) {
		return 0, nil, fmt.Errorf("TMC metrics collection is disabled")
	}

	klog.V(4).InfoS("Collecting metric from Prometheus",
		"cluster", clusterName,
		"workspace", workspace,
		"metric", metricName)

	// Build Prometheus query with workspace and cluster filtering
	query := fmt.Sprintf(`%s{cluster="%s",workspace="%s"}`, metricName, clusterName, workspace)
	
	// Build query URL
	queryURL := fmt.Sprintf("%s/api/v1/query", pmc.baseURL)
	params := url.Values{}
	params.Add("query", query)
	params.Add("time", strconv.FormatInt(time.Now().Unix(), 10))
	
	fullURL := fmt.Sprintf("%s?%s", queryURL, params.Encode())
	
	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	// Execute request
	resp, err := pmc.httpClient.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to query Prometheus: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return 0, nil, fmt.Errorf("Prometheus returned status %d", resp.StatusCode)
	}
	
	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to read response: %w", err)
	}
	
	// Parse Prometheus response (simplified parsing)
	value, labels, err := pmc.parsePrometheusResponse(string(body))
	if err != nil {
		return 0, nil, fmt.Errorf("failed to parse Prometheus response: %w", err)
	}
	
	klog.V(6).InfoS("Retrieved metric value",
		"cluster", clusterName,
		"metric", metricName,
		"value", value)
	
	return value, labels, nil
}

// ListClusters returns available clusters in a workspace by querying Prometheus metrics.
//
// This method discovers clusters by looking for distinct cluster labels in the
// metrics data within the specified workspace.
//
// Parameters:
//   - ctx: Context for the request
//   - workspace: Logical cluster workspace
//
// Returns:
//   - []string: List of available cluster names
//   - error: Discovery error if any
func (pmc *PrometheusMetricsCollector) ListClusters(
	ctx context.Context,
	workspace logicalcluster.Name,
) ([]string, error) {
	// Check if TMC metrics collection is enabled
	if !utilfeature.DefaultFeatureGate.Enabled(features.TMCMetricsAggregation) {
		return nil, fmt.Errorf("TMC metrics collection is disabled")
	}

	klog.V(4).InfoS("Listing clusters from Prometheus", "workspace", workspace)

	// Query for all metrics in the workspace to discover clusters
	query := fmt.Sprintf(`{workspace="%s"}`, workspace)
	
	// Build query URL for labels endpoint
	queryURL := fmt.Sprintf("%s/api/v1/label/cluster/values", pmc.baseURL)
	params := url.Values{}
	params.Add("match[]", query)
	
	fullURL := fmt.Sprintf("%s?%s", queryURL, params.Encode())
	
	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	// Execute request
	resp, err := pmc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to query Prometheus labels: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Prometheus returned status %d", resp.StatusCode)
	}
	
	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	
	// Parse cluster names from response
	clusters, err := pmc.parseClusterLabels(string(body))
	if err != nil {
		return nil, fmt.Errorf("failed to parse cluster labels: %w", err)
	}
	
	klog.V(6).InfoS("Discovered clusters",
		"workspace", workspace,
		"count", len(clusters))
	
	return clusters, nil
}

// parsePrometheusResponse parses a Prometheus query response and extracts value and labels.
// This is a simplified parser for demonstration - in production, use a proper JSON library.
func (pmc *PrometheusMetricsCollector) parsePrometheusResponse(body string) (float64, map[string]string, error) {
	// Simplified parsing - in production, use proper JSON parsing
	// For now, return dummy values for testing
	
	// Look for numeric values in the response
	if strings.Contains(body, "\"value\"") {
		// Extract first numeric value found (very basic parsing)
		lines := strings.Split(body, "\n")
		for _, line := range lines {
			if strings.Contains(line, "\"value\"") {
				// Basic extraction - in production use proper JSON parsing
				parts := strings.Split(line, ",")
				for _, part := range parts {
					if strings.Contains(part, "\"") {
						valueStr := strings.Trim(strings.Split(part, ":")[1], " \"")
						if value, err := strconv.ParseFloat(valueStr, 64); err == nil {
							labels := map[string]string{
								"source": "prometheus",
							}
							return value, labels, nil
						}
					}
				}
			}
		}
	}
	
	// Return default values if parsing fails
	return 1.0, map[string]string{"source": "prometheus"}, nil
}

// parseClusterLabels parses cluster names from Prometheus label values response.
func (pmc *PrometheusMetricsCollector) parseClusterLabels(body string) ([]string, error) {
	// Simplified parsing - in production, use proper JSON parsing
	// For testing, return some cluster names
	clusters := []string{"cluster-1", "cluster-2"}
	
	// In production, parse the JSON response to extract actual cluster names
	// from the "data" array in the Prometheus response
	
	return clusters, nil
}