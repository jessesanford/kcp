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

package status

import (
	"github.com/prometheus/client_golang/prometheus"
)

// MetricsRecorder defines the interface for recording aggregated status metrics
type MetricsRecorder interface {
	// RecordAggregatedStatus records metrics from aggregated status
	RecordAggregatedStatus(status *AggregatedStatus) error
}

// metricsRecorder implements MetricsRecorder using Prometheus metrics
type metricsRecorder struct {
	// Gauge metrics for current state
	totalTargetsGauge    prometheus.Gauge
	healthyTargetsGauge  prometheus.Gauge
	totalResourcesGauge  prometheus.Gauge
	readyResourcesGauge  prometheus.Gauge
	successPercentGauge  prometheus.Gauge
	
	// Histogram for aggregation latency
	aggregationLatencyHist prometheus.Histogram
	
	// Counter for aggregations
	aggregationCounter *prometheus.CounterVec
	
	// Gauge vector for health status breakdown
	healthStatusGauge *prometheus.GaugeVec
}

// NewMetricsRecorder creates a new metrics recorder with Prometheus metrics
func NewMetricsRecorder(registerer prometheus.Registerer) MetricsRecorder {
	recorder := &metricsRecorder{
		totalTargetsGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "tmc_placement_total_targets",
			Help: "Total number of sync targets for placement",
		}),
		healthyTargetsGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "tmc_placement_healthy_targets",
			Help: "Number of healthy sync targets for placement",
		}),
		totalResourcesGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "tmc_placement_total_resources",
			Help: "Total number of resources across all sync targets",
		}),
		readyResourcesGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "tmc_placement_ready_resources",
			Help: "Number of ready resources across all sync targets",
		}),
		successPercentGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "tmc_placement_success_percentage",
			Help: "Percentage of successful placements across sync targets",
		}),
		aggregationLatencyHist: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "tmc_placement_aggregation_duration_seconds",
			Help:    "Time taken to aggregate placement status",
			Buckets: prometheus.DefBuckets,
		}),
		aggregationCounter: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "tmc_placement_aggregations_total",
				Help: "Total number of placement status aggregations",
			},
			[]string{"overall_health"},
		),
		healthStatusGauge: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "tmc_placement_targets_by_health",
				Help: "Number of sync targets by health status",
			},
			[]string{"health_status"},
		),
	}
	
	// Register all metrics
	if registerer != nil {
		registerer.MustRegister(
			recorder.totalTargetsGauge,
			recorder.healthyTargetsGauge,
			recorder.totalResourcesGauge,
			recorder.readyResourcesGauge,
			recorder.successPercentGauge,
			recorder.aggregationLatencyHist,
			recorder.aggregationCounter,
			recorder.healthStatusGauge,
		)
	}
	
	return recorder
}

// RecordAggregatedStatus implements MetricsRecorder.RecordAggregatedStatus
func (m *metricsRecorder) RecordAggregatedStatus(status *AggregatedStatus) error {
	// Record basic counts
	m.totalTargetsGauge.Set(float64(status.TotalTargets))
	m.healthyTargetsGauge.Set(float64(status.HealthyTargets))
	m.totalResourcesGauge.Set(float64(status.TotalResources))
	m.readyResourcesGauge.Set(float64(status.ReadyResources))
	m.successPercentGauge.Set(status.SuccessPercentage)
	
	// Record aggregation latency
	m.aggregationLatencyHist.Observe(status.AggregationLatency.Seconds())
	
	// Increment aggregation counter with overall health label
	m.aggregationCounter.WithLabelValues(status.OverallHealth.String()).Inc()
	
	// Record health status breakdown
	m.recordHealthStatusBreakdown(status.TargetStatuses)
	
	return nil
}

// recordHealthStatusBreakdown records the count of targets by health status
func (m *metricsRecorder) recordHealthStatusBreakdown(statuses []TargetStatus) {
	// Count targets by health status
	healthCounts := make(map[HealthStatus]int)
	for _, status := range statuses {
		healthCounts[status.Health]++
	}
	
	// Reset all gauges first (to handle cases where a status type has 0 targets)
	for _, healthStatus := range []HealthStatus{
		HealthStatusHealthy,
		HealthStatusDegraded,
		HealthStatusUnhealthy,
		HealthStatusUnknown,
	} {
		m.healthStatusGauge.WithLabelValues(healthStatus.String()).Set(0)
	}
	
	// Set actual counts
	for healthStatus, count := range healthCounts {
		m.healthStatusGauge.WithLabelValues(healthStatus.String()).Set(float64(count))
	}
}

// noopMetricsRecorder is a no-op implementation for when metrics are disabled
type noopMetricsRecorder struct{}

// NewNoopMetricsRecorder creates a metrics recorder that doesn't record anything
func NewNoopMetricsRecorder() MetricsRecorder {
	return &noopMetricsRecorder{}
}

// RecordAggregatedStatus implements MetricsRecorder.RecordAggregatedStatus as a no-op
func (n *noopMetricsRecorder) RecordAggregatedStatus(status *AggregatedStatus) error {
	return nil // No-op
}