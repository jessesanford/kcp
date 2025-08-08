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
	"testing"

	"github.com/kcp-dev/logicalcluster/v3"
)

func TestPrometheusMetricsCollector(t *testing.T) {
	tests := map[string]struct {
		baseURL     string
		clusterName string
		workspace   logicalcluster.Name
		metricName  string
		wantError   bool
	}{
		"valid prometheus collector": {
			baseURL:     "http://prometheus:9090",
			clusterName: "test-cluster",
			workspace:   "root:test",
			metricName:  "cpu_usage",
			wantError:   false,
		},
		"empty cluster name": {
			baseURL:     "http://prometheus:9090",
			clusterName: "",
			workspace:   "root:test",
			metricName:  "cpu_usage",
			wantError:   true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			collector := NewPrometheusMetricsCollector(tc.baseURL)

			// Test GetMetricValue
			_, _, err := collector.GetMetricValue(context.Background(), tc.clusterName, tc.workspace, tc.metricName)
			if (err != nil) != tc.wantError {
				t.Errorf("GetMetricValue() error = %v, wantError %v", err, tc.wantError)
			}

			// Test ListClusters
			_, err = collector.ListClusters(context.Background(), tc.workspace)
			if (err != nil) != tc.wantError {
				t.Errorf("ListClusters() error = %v, wantError %v", err, tc.wantError)
			}
		})
	}
}

func TestClusterMetricsCollector(t *testing.T) {
	collector := NewClusterMetricsCollector().(*ClusterMetricsCollector)
	
	// Register test cluster
	collector.RegisterCluster("test-cluster", "http://test:8080/metrics")
	
	tests := map[string]struct {
		clusterName string
		workspace   logicalcluster.Name
		metricName  string
		wantError   bool
	}{
		"registered cluster": {
			clusterName: "test-cluster",
			workspace:   "root:test",
			metricName:  "cpu_usage",
			wantError:   false,
		},
		"unregistered cluster": {
			clusterName: "unknown-cluster",
			workspace:   "root:test",
			metricName:  "cpu_usage",
			wantError:   true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Test GetMetricValue
			_, _, err := collector.GetMetricValue(context.Background(), tc.clusterName, tc.workspace, tc.metricName)
			if (err != nil) != tc.wantError {
				t.Errorf("GetMetricValue() error = %v, wantError %v", err, tc.wantError)
			}
		})
	}

	// Test cluster registration/unregistration
	t.Run("cluster registration", func(t *testing.T) {
		collector.RegisterCluster("new-cluster", "http://new:8080")
		
		clusters, err := collector.ListClusters(context.Background(), "root:test")
		if err != nil {
			t.Errorf("ListClusters() failed: %v", err)
		}
		
		if len(clusters) < 1 {
			t.Error("Expected at least one cluster after registration")
		}
		
		collector.UnregisterCluster("new-cluster")
	})
}

func TestMetricsCollectorRegistry(t *testing.T) {
	registry := NewMetricsCollectorRegistry()
	
	// Create test collectors
	prometheusCollector := NewPrometheusMetricsCollector("http://prometheus:9090")
	clusterCollector := NewClusterMetricsCollector()
	
	// Test registration
	registry.RegisterCollector(PrometheusCollector, prometheusCollector)
	registry.RegisterCollector(ClusterDirectCollector, clusterCollector)
	
	// Test getting registered collectors
	collectors := registry.GetRegisteredCollectors()
	if len(collectors) != 2 {
		t.Errorf("Expected 2 collectors, got %d", len(collectors))
	}
	
	// Test primary collector
	primary := registry.GetPrimaryCollector()
	if primary == nil {
		t.Error("Expected primary collector, got nil")
	}
	
	// Test unregistration
	registry.UnregisterCollector(PrometheusCollector)
	collectors = registry.GetRegisteredCollectors()
	if len(collectors) != 1 {
		t.Errorf("Expected 1 collector after unregistration, got %d", len(collectors))
	}
}