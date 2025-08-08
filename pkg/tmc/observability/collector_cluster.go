package observability

import (
	"context"
	"fmt"
	"sync"

	"k8s.io/klog/v2"
	utilfeature "k8s.io/apiserver/pkg/util/feature"

	"github.com/kcp-dev/logicalcluster/v3"
	"github.com/kcp-dev/kcp/pkg/features"
)

// ClusterMetricsCollector collects metrics directly from clusters.
type ClusterMetricsCollector struct {
	mu               sync.RWMutex
	clusterEndpoints map[string]string
}

// NewClusterMetricsCollector creates a new cluster collector.
func NewClusterMetricsCollector() MetricsSource {
	return &ClusterMetricsCollector{
		clusterEndpoints: make(map[string]string),
	}
}

// RegisterCluster registers a cluster endpoint.
func (cmc *ClusterMetricsCollector) RegisterCluster(clusterName, endpoint string) {
	cmc.mu.Lock()
	defer cmc.mu.Unlock()
	cmc.clusterEndpoints[clusterName] = endpoint
	klog.V(4).InfoS("Registered cluster", "cluster", clusterName)
}

// GetMetricValue retrieves metric from cluster.
func (cmc *ClusterMetricsCollector) GetMetricValue(
	ctx context.Context, clusterName string, workspace logicalcluster.Name, metricName string,
) (float64, map[string]string, error) {
	if \!utilfeature.DefaultFeatureGate.Enabled(features.TMCMetricsAggregation) {
		return 0, nil, fmt.Errorf("TMC metrics disabled")
	}

	cmc.mu.RLock()
	endpoint, exists := cmc.clusterEndpoints[clusterName]
	cmc.mu.RUnlock()

	if \!exists {
		return 0, nil, fmt.Errorf("cluster not registered: %s", clusterName)
	}

	// Simulate collection
	value := 75.5
	labels := map[string]string{
		"cluster": clusterName, "workspace": workspace.String(),
		"source": "cluster-direct", "endpoint": endpoint,
	}
	return value, labels, nil
}

// ListClusters returns available clusters.
func (cmc *ClusterMetricsCollector) ListClusters(
	ctx context.Context, workspace logicalcluster.Name,
) ([]string, error) {
	if \!utilfeature.DefaultFeatureGate.Enabled(features.TMCMetricsAggregation) {
		return nil, fmt.Errorf("TMC metrics disabled")
	}

	cmc.mu.RLock()
	defer cmc.mu.RUnlock()

	var clusters []string
	for clusterName := range cmc.clusterEndpoints {
		clusters = append(clusters, clusterName)
	}
	return clusters, nil
}
