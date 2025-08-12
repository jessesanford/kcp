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

package collector

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"k8s.io/klog/v2"
)

// MetricsCollector handles Prometheus metrics collection for placement analysis
type MetricsCollector struct {
	// namespace is the metrics namespace prefix
	namespace string

	// placementsTotal tracks total number of placements
	placementsTotal *prometheus.GaugeVec

	// resourceCount tracks resource counts per placement
	resourceCount *prometheus.GaugeVec

	// targetClustersCount tracks number of target clusters per placement
	targetClustersCount *prometheus.GaugeVec

	// placementHealthStatus tracks health status of placements
	placementHealthStatus *prometheus.GaugeVec

	// collectionDuration tracks how long collection takes
	collectionDuration *prometheus.HistogramVec

	// collectionErrors tracks collection errors
	collectionErrors *prometheus.CounterVec

	// dataStoreSize tracks the size of the data store
	dataStoreSize prometheus.Gauge
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(namespace string) (*MetricsCollector, error) {
	if namespace == "" {
		return nil, fmt.Errorf("metrics namespace cannot be empty")
	}

	mc := &MetricsCollector{
		namespace: namespace,
	}

	// Initialize Prometheus metrics
	mc.placementsTotal = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "placements_total",
			Help:      "Total number of placement analysis data points collected",
		},
		[]string{"cluster", "workspace"},
	)

	mc.resourceCount = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "resource_count",
			Help:      "Number of resources managed by placement",
		},
		[]string{"cluster", "workspace", "placement", "namespace"},
	)

	mc.targetClustersCount = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "target_clusters_count",
			Help:      "Number of target clusters for placement",
		},
		[]string{"cluster", "workspace", "placement", "namespace"},
	)

	mc.placementHealthStatus = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "placement_health_status",
			Help:      "Health status of placement (1=healthy, 0=unhealthy, -1=unknown)",
		},
		[]string{"cluster", "workspace", "placement", "namespace", "status"},
	)

	mc.collectionDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "collection_duration_seconds",
			Help:      "Duration of placement data collection operations",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"operation"},
	)

	mc.collectionErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "collection_errors_total",
			Help:      "Total number of collection errors",
		},
		[]string{"operation", "error_type"},
	)

	mc.dataStoreSize = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "data_store_size",
			Help:      "Current size of the placement data store",
		},
	)

	klog.InfoS("Initialized placement analysis metrics collector", "namespace", namespace)

	return mc, nil
}

// RecordPlacementData records metrics for a placement data point
func (mc *MetricsCollector) RecordPlacementData(data PlacementData) {
	labels := prometheus.Labels{
		"cluster":   string(data.ClusterName),
		"workspace": data.WorkspaceName,
		"placement": data.PlacementName,
		"namespace": data.PlacementNamespace,
	}

	// Record resource count
	mc.resourceCount.With(labels).Set(float64(data.ResourceCount))

	// Record target clusters count
	mc.targetClustersCount.With(labels).Set(float64(len(data.TargetClusters)))

	// Record health status
	healthValue := mc.healthStatusToFloat(data.HealthStatus)
	healthLabels := prometheus.Labels{
		"cluster":   string(data.ClusterName),
		"workspace": data.WorkspaceName,
		"placement": data.PlacementName,
		"namespace": data.PlacementNamespace,
		"status":    data.HealthStatus,
	}
	mc.placementHealthStatus.With(healthLabels).Set(healthValue)

	// Update placement totals
	placementLabels := prometheus.Labels{
		"cluster":   string(data.ClusterName),
		"workspace": data.WorkspaceName,
	}
	mc.placementsTotal.With(placementLabels).Inc()
}

// RecordCollectionDuration records the duration of a collection operation
func (mc *MetricsCollector) RecordCollectionDuration(operation string, duration float64) {
	mc.collectionDuration.WithLabelValues(operation).Observe(duration)
}

// RecordCollectionError records a collection error
func (mc *MetricsCollector) RecordCollectionError(operation, errorType string) {
	mc.collectionErrors.WithLabelValues(operation, errorType).Inc()
}

// UpdateDataStoreSize updates the data store size metric
func (mc *MetricsCollector) UpdateDataStoreSize(size float64) {
	mc.dataStoreSize.Set(size)
}

// healthStatusToFloat converts health status strings to float values for metrics
func (mc *MetricsCollector) healthStatusToFloat(status string) float64 {
	switch status {
	case "Healthy":
		return 1.0
	case "Unhealthy":
		return 0.0
	case "Degraded":
		return 0.5
	case "Progressing":
		return 0.75
	case "Unknown":
		fallthrough
	default:
		return -1.0
	}
}

// Close cleanups the metrics collector
func (mc *MetricsCollector) Close() error {
	klog.InfoS("Closing placement analysis metrics collector", "namespace", mc.namespace)
	return nil
}