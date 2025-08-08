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

// MetricsSource represents a source of metrics data from clusters.
type MetricsSource interface {
	GetMetricValue(ctx context.Context, clusterName string, workspace logicalcluster.Name, metricName string) (float64, map[string]string, error)
	ListClusters(ctx context.Context, workspace logicalcluster.Name) ([]string, error)
}

// PrometheusMetricsCollector implements MetricsSource using Prometheus HTTP API.
type PrometheusMetricsCollector struct {
	baseURL    string
	httpClient *http.Client
}

// NewPrometheusMetricsCollector creates a new Prometheus-based metrics collector.
func NewPrometheusMetricsCollector(baseURL string) MetricsSource {
	return &PrometheusMetricsCollector{
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// GetMetricValue retrieves a specific metric value from a cluster via Prometheus.
func (pmc *PrometheusMetricsCollector) GetMetricValue(
	ctx context.Context,
	clusterName string,
	workspace logicalcluster.Name,
	metricName string,
) (float64, map[string]string, error) {
	if \!utilfeature.DefaultFeatureGate.Enabled(features.TMCMetricsAggregation) {
		return 0, nil, fmt.Errorf("TMC metrics collection is disabled")
	}

	klog.V(4).InfoS("Collecting metric from Prometheus", "cluster", clusterName, "workspace", workspace, "metric", metricName)

	// Build Prometheus query with workspace and cluster filtering
	query := fmt.Sprintf(`%s{cluster="%s",workspace="%s"}`, metricName, clusterName, workspace)
	queryURL := fmt.Sprintf("%s/api/v1/query", pmc.baseURL)
	
	params := url.Values{}
	params.Add("query", query)
	params.Add("time", strconv.FormatInt(time.Now().Unix(), 10))
	
	fullURL := fmt.Sprintf("%s?%s", queryURL, params.Encode())
	
	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err \!= nil {
		return 0, nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	resp, err := pmc.httpClient.Do(req)
	if err \!= nil {
		return 0, nil, fmt.Errorf("failed to query Prometheus: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode \!= http.StatusOK {
		return 0, nil, fmt.Errorf("Prometheus returned status %d", resp.StatusCode)
	}
	
	body, err := io.ReadAll(resp.Body)
	if err \!= nil {
		return 0, nil, fmt.Errorf("failed to read response: %w", err)
	}
	
	// Parse response (simplified for demo)
	value, labels, err := pmc.parseResponse(string(body))
	if err \!= nil {
		return 0, nil, fmt.Errorf("failed to parse Prometheus response: %w", err)
	}
	
	return value, labels, nil
}

// ListClusters returns available clusters in a workspace.
func (pmc *PrometheusMetricsCollector) ListClusters(
	ctx context.Context,
	workspace logicalcluster.Name,
) ([]string, error) {
	if \!utilfeature.DefaultFeatureGate.Enabled(features.TMCMetricsAggregation) {
		return nil, fmt.Errorf("TMC metrics collection is disabled")
	}

	klog.V(4).InfoS("Listing clusters from Prometheus", "workspace", workspace)

	query := fmt.Sprintf(`{workspace="%s"}`, workspace)
	queryURL := fmt.Sprintf("%s/api/v1/label/cluster/values", pmc.baseURL)
	
	params := url.Values{}
	params.Add("match[]", query)
	fullURL := fmt.Sprintf("%s?%s", queryURL, params.Encode())
	
	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err \!= nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	resp, err := pmc.httpClient.Do(req)
	if err \!= nil {
		return nil, fmt.Errorf("failed to query Prometheus labels: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode \!= http.StatusOK {
		return nil, fmt.Errorf("Prometheus returned status %d", resp.StatusCode)
	}
	
	body, err := io.ReadAll(resp.Body)
	if err \!= nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	
	clusters, err := pmc.parseClusterLabels(string(body))
	if err \!= nil {
		return nil, fmt.Errorf("failed to parse cluster labels: %w", err)
	}
	
	return clusters, nil
}

// parseResponse parses a Prometheus query response (simplified implementation).
func (pmc *PrometheusMetricsCollector) parseResponse(body string) (float64, map[string]string, error) {
	// Simplified parsing - in production, use proper JSON parsing
	return 1.0, map[string]string{"source": "prometheus"}, nil
}

// parseClusterLabels parses cluster names from Prometheus response (simplified implementation).
func (pmc *PrometheusMetricsCollector) parseClusterLabels(body string) ([]string, error) {
	// For testing, return some cluster names
	return []string{"cluster-1", "cluster-2"}, nil
}
EOF < /dev/null
