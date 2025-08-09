package observability

import (
	"context"
	"fmt"
	"sync"
	"k8s.io/klog/v2"
	"github.com/kcp-dev/logicalcluster/v3"
)

type CollectorType string
const (
	PrometheusCollector CollectorType = "prometheus"
	ClusterDirectCollector CollectorType = "cluster-direct"
)

type MetricsCollectorRegistry struct {
	mu sync.RWMutex
	collectors map[CollectorType]MetricsSource
	priority []CollectorType
}

func NewMetricsCollectorRegistry() *MetricsCollectorRegistry {
	return &MetricsCollectorRegistry{
		collectors: make(map[CollectorType]MetricsSource),
		priority: make([]CollectorType, 0),
	}
}

func (mcr *MetricsCollectorRegistry) RegisterCollector(t CollectorType, c MetricsSource) {
	mcr.mu.Lock()
	defer mcr.mu.Unlock()
	mcr.collectors[t] = c
	mcr.priority = append(mcr.priority, t)
	klog.V(4).InfoS("Registered collector", "type", t)
}

func (mcr *MetricsCollectorRegistry) GetMetricValue(
	ctx context.Context, clusterName string, workspace logicalcluster.Name, metricName string,
) (float64, map[string]string, error) {
	mcr.mu.RLock()
	defer mcr.mu.RUnlock()

	var lastError error
	for _, collectorType := range mcr.priority {
		collector := mcr.collectors[collectorType]
		value, labels, err := collector.GetMetricValue(ctx, clusterName, workspace, metricName)
		if err \!= nil {
			lastError = err
			continue
		}
		if labels == nil {
			labels = make(map[string]string)
		}
		labels["collector_type"] = string(collectorType)
		return value, labels, nil
	}
	return 0, nil, fmt.Errorf("all collectors failed: %w", lastError)
}

func (mcr *MetricsCollectorRegistry) ListClusters(
	ctx context.Context, workspace logicalcluster.Name,
) ([]string, error) {
	mcr.mu.RLock()
	defer mcr.mu.RUnlock()

	for _, collectorType := range mcr.priority {
		collector := mcr.collectors[collectorType]
		clusters, err := collector.ListClusters(ctx, workspace)
		if err == nil {
			return clusters, nil
		}
	}
	return nil, fmt.Errorf("no collectors available")
}
