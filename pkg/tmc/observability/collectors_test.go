package observability

import (
	"context"
	"testing"
	"github.com/kcp-dev/logicalcluster/v3"
)

func TestPrometheusMetricsCollector(t *testing.T) {
	collector := NewPrometheusMetricsCollector("http://prometheus:9090")
	_, _, err := collector.GetMetricValue(context.Background(), "test", "root:test", "cpu_usage")
	if err == nil {
		t.Error("Expected error when feature disabled")
	}
}

func TestClusterMetricsCollector(t *testing.T) {
	collector := NewClusterMetricsCollector().(*ClusterMetricsCollector)
	collector.RegisterCluster("test", "http://test:8080")
	
	_, _, err := collector.GetMetricValue(context.Background(), "test", "root:test", "cpu_usage")
	if err == nil {
		t.Error("Expected error when feature disabled")
	}
}

func TestMetricsCollectorRegistry(t *testing.T) {
	registry := NewMetricsCollectorRegistry()
	prometheusCollector := NewPrometheusMetricsCollector("http://prometheus:9090")
	
	registry.RegisterCollector(PrometheusCollector, prometheusCollector)
	
	_, _, err := registry.GetMetricValue(context.Background(), "test", "root:test", "cpu_usage")
	if err == nil {
		t.Error("Expected error when feature disabled")
	}
}
