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
	"sync"
	"time"

	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"

	"github.com/kcp-dev/kcp/pkg/features"
)

// AggregationStrategy defines how metrics should be aggregated across clusters.
type AggregationStrategy string

const (
	// AggregationSum aggregates metrics by summing values across clusters
	AggregationSum AggregationStrategy = "sum"
	// AggregationAvg aggregates metrics by averaging values across clusters
	AggregationAvg AggregationStrategy = "avg"
	// AggregationMax aggregates metrics by taking maximum value across clusters
	AggregationMax AggregationStrategy = "max"
	// AggregationMin aggregates metrics by taking minimum value across clusters
	AggregationMin AggregationStrategy = "min"
)

// MetricPoint represents a single metric data point with timestamp and value.
type MetricPoint struct {
	Timestamp time.Time         `json:"timestamp"`
	Value     float64           `json:"value"`
	Labels    map[string]string `json:"labels,omitempty"`
}

// TimeSeries represents a time series of metric points.
type TimeSeries struct {
	MetricName string            `json:"metric_name"`
	Labels     map[string]string `json:"labels"`
	Points     []MetricPoint     `json:"points"`
}

// AggregatedMetric represents a metric aggregated across clusters.
type AggregatedMetric struct {
	MetricName     string              `json:"metric_name"`
	Strategy       AggregationStrategy `json:"strategy"`
	Workspace      logicalcluster.Name `json:"workspace"`
	Value          float64             `json:"value"`
	ClusterCount   int                 `json:"cluster_count"`
	Timestamp      time.Time           `json:"timestamp"`
	Labels         map[string]string   `json:"labels,omitempty"`
	SourceClusters []string            `json:"source_clusters"`
}

// TimeRange defines a time range for metrics queries.
type TimeRange struct {
	Start time.Time     `json:"start"`
	End   time.Time     `json:"end"`
	Step  time.Duration `json:"step,omitempty"`
}

// WorkspaceAwareMetricsCollector defines the interface for collecting metrics from clusters within workspaces.
type WorkspaceAwareMetricsCollector interface {
	// ListClusters returns the list of cluster names in a workspace
	ListClusters(ctx context.Context, workspace logicalcluster.Name) ([]string, error)

	// CollectClusterMetrics collects metrics from a specific cluster in a workspace
	CollectClusterMetrics(ctx context.Context, clusterName string, workspace logicalcluster.Name) (*ClusterMetrics, error)
}

// ClusterMetrics represents metrics collected from a single cluster.
type ClusterMetrics struct {
	Metrics   map[string]float64 `json:"metrics"`
	Labels    map[string]string  `json:"labels"`
	Timestamp time.Time          `json:"timestamp"`
}

// MetricsAggregator defines the interface for metrics aggregation.
type MetricsAggregator interface {
	// AggregateMetrics aggregates metrics from multiple clusters using the specified strategy
	AggregateMetrics(ctx context.Context, workspace logicalcluster.Name, metricName string, strategy AggregationStrategy, timeRange TimeRange) (*AggregatedMetric, error)

	// AggregateTimeSeries aggregates time series data from multiple clusters
	AggregateTimeSeries(ctx context.Context, workspace logicalcluster.Name, metricName string, strategy AggregationStrategy, timeRange TimeRange) (*TimeSeries, error)

	// ConsolidateTimeSeries consolidates time series data by removing duplicates and filling gaps
	ConsolidateTimeSeries(timeSeries []*TimeSeries, interval time.Duration) (*TimeSeries, error)
}

// MetricsAggregatorImpl implements the MetricsAggregator interface.
type MetricsAggregatorImpl struct {
	mu               sync.RWMutex
	metricsCollector WorkspaceAwareMetricsCollector
}

// NewMetricsAggregator creates a new metrics aggregator.
func NewMetricsAggregator(metricsCollector WorkspaceAwareMetricsCollector) MetricsAggregator {
	return &MetricsAggregatorImpl{
		metricsCollector: metricsCollector,
	}
}

// AggregateMetrics aggregates metrics from multiple clusters using the specified strategy.
func (ma *MetricsAggregatorImpl) AggregateMetrics(
	ctx context.Context,
	workspace logicalcluster.Name,
	metricName string,
	strategy AggregationStrategy,
	timeRange TimeRange,
) (*AggregatedMetric, error) {
	// Check if metrics aggregation is enabled
	if !utilfeature.DefaultFeatureGate.Enabled(features.TMCMetricsAggregation) {
		return nil, fmt.Errorf("TMC metrics aggregation is disabled")
	}

	klog.V(4).InfoS("Aggregating metrics",
		"workspace", workspace,
		"metric", metricName,
		"strategy", strategy)

	// Check if advanced aggregation is required and enabled
	if strategy != AggregationSum && !utilfeature.DefaultFeatureGate.Enabled(features.TMCAdvancedAggregation) {
		return nil, fmt.Errorf("advanced aggregation strategies require TMCAdvancedAggregation feature flag")
	}

	// Get list of clusters in workspace
	clusters, err := ma.metricsCollector.ListClusters(ctx, workspace)
	if err != nil {
		return nil, fmt.Errorf("failed to list clusters for workspace %s: %w", workspace, err)
	}

	if len(clusters) == 0 {
		return nil, fmt.Errorf("no clusters found in workspace %s", workspace)
	}

	// Collect metrics from all clusters
	var clusterValues []float64
	var sourceClusters []string
	var aggregatedLabels map[string]string

	for _, clusterName := range clusters {
		metrics, err := ma.metricsCollector.CollectClusterMetrics(ctx, clusterName, workspace)
		if err != nil {
			klog.V(2).InfoS("Failed to get metrics from cluster", "cluster", clusterName, "error", err)
			continue
		}

		// Find the specific metric value
		if value, exists := metrics.Metrics[metricName]; exists {
			clusterValues = append(clusterValues, value)
			sourceClusters = append(sourceClusters, clusterName)

			// Merge labels from first cluster
			if aggregatedLabels == nil {
				aggregatedLabels = make(map[string]string)
				for k, v := range metrics.Labels {
					aggregatedLabels[k] = v
				}
			}
		}
	}

	if len(clusterValues) == 0 {
		return nil, fmt.Errorf("no metric values found for %s in workspace %s", metricName, workspace)
	}

	// Apply aggregation strategy
	aggregatedValue, err := ma.applyAggregationStrategy(strategy, clusterValues)
	if err != nil {
		return nil, fmt.Errorf("failed to apply aggregation strategy %s: %w", strategy, err)
	}

	return &AggregatedMetric{
		MetricName:     metricName,
		Strategy:       strategy,
		Workspace:      workspace,
		Value:          aggregatedValue,
		ClusterCount:   len(clusterValues),
		Timestamp:      time.Now(),
		Labels:         aggregatedLabels,
		SourceClusters: sourceClusters,
	}, nil
}

// AggregateTimeSeries aggregates time series data from multiple clusters.
func (ma *MetricsAggregatorImpl) AggregateTimeSeries(
	ctx context.Context,
	workspace logicalcluster.Name,
	metricName string,
	strategy AggregationStrategy,
	timeRange TimeRange,
) (*TimeSeries, error) {
	// Check if time series consolidation is enabled
	if !utilfeature.DefaultFeatureGate.Enabled(features.TMCTimeSeriesConsolidation) {
		return nil, fmt.Errorf("TMC time series consolidation is disabled")
	}

	klog.V(4).InfoS("Aggregating time series",
		"workspace", workspace,
		"metric", metricName,
		"strategy", strategy)

	// Get list of clusters in workspace
	clusters, err := ma.metricsCollector.ListClusters(ctx, workspace)
	if err != nil {
		return nil, fmt.Errorf("failed to list clusters for workspace %s: %w", workspace, err)
	}

	// Collect time series from all clusters
	var allTimeSeries []*TimeSeries

	for _, clusterName := range clusters {
		ts, err := ma.getTimeSeriesFromCluster(ctx, clusterName, workspace, metricName, timeRange)
		if err != nil {
			klog.V(2).InfoS("Failed to get time series from cluster", "cluster", clusterName, "error", err)
			continue
		}
		if ts != nil {
			allTimeSeries = append(allTimeSeries, ts)
		}
	}

	if len(allTimeSeries) == 0 {
		return nil, fmt.Errorf("no time series data found for %s in workspace %s", metricName, workspace)
	}

	// Consolidate and aggregate time series
	step := timeRange.Step
	if step == 0 {
		step = time.Minute // default step
	}

	return ma.ConsolidateTimeSeries(allTimeSeries, step)
}

// ConsolidateTimeSeries consolidates time series data by removing duplicates and filling gaps.
func (ma *MetricsAggregatorImpl) ConsolidateTimeSeries(timeSeries []*TimeSeries, interval time.Duration) (*TimeSeries, error) {
	if len(timeSeries) == 0 {
		return nil, fmt.Errorf("no time series to consolidate")
	}

	// Find time bounds and collect points
	var earliestTime, latestTime time.Time
	allPoints := make(map[time.Time][]float64)
	metricName := timeSeries[0].MetricName
	consolidatedLabels := make(map[string]string)

	// Collect all points and determine time bounds
	for i, ts := range timeSeries {
		if ts.MetricName != metricName {
			return nil, fmt.Errorf("inconsistent metric names: %s vs %s", metricName, ts.MetricName)
		}

		// Merge labels from first series
		if i == 0 {
			for k, v := range ts.Labels {
				consolidatedLabels[k] = v
			}
		}

		for _, point := range ts.Points {
			// Normalize timestamp to interval boundaries
			normalizedTime := point.Timestamp.Truncate(interval)

			if earliestTime.IsZero() || normalizedTime.Before(earliestTime) {
				earliestTime = normalizedTime
			}
			if latestTime.IsZero() || normalizedTime.After(latestTime) {
				latestTime = normalizedTime
			}

			allPoints[normalizedTime] = append(allPoints[normalizedTime], point.Value)
		}
	}

	// Generate consolidated points with gap filling
	var consolidatedPoints []MetricPoint
	for t := earliestTime; !t.After(latestTime); t = t.Add(interval) {
		if values, exists := allPoints[t]; exists {
			// Average values at the same time point
			sum := 0.0
			for _, v := range values {
				sum += v
			}
			avg := sum / float64(len(values))

			consolidatedPoints = append(consolidatedPoints, MetricPoint{
				Timestamp: t,
				Value:     avg,
				Labels:    map[string]string{"consolidated": "true"},
			})
		} else {
			// Fill gap with interpolated value or zero
			var fillValue float64 = 0.0

			// Simple interpolation: use previous value if available
			if len(consolidatedPoints) > 0 {
				fillValue = consolidatedPoints[len(consolidatedPoints)-1].Value
			}

			consolidatedPoints = append(consolidatedPoints, MetricPoint{
				Timestamp: t,
				Value:     fillValue,
				Labels:    map[string]string{"consolidated": "true", "filled": "true"},
			})
		}
	}

	return &TimeSeries{
		MetricName: metricName,
		Labels:     consolidatedLabels,
		Points:     consolidatedPoints,
	}, nil
}

// applyAggregationStrategy applies the specified aggregation strategy to a set of values.
func (ma *MetricsAggregatorImpl) applyAggregationStrategy(strategy AggregationStrategy, values []float64) (float64, error) {
	if len(values) == 0 {
		return 0, fmt.Errorf("no values to aggregate")
	}

	switch strategy {
	case AggregationSum:
		sum := 0.0
		for _, v := range values {
			sum += v
		}
		return sum, nil

	case AggregationAvg:
		sum := 0.0
		for _, v := range values {
			sum += v
		}
		return sum / float64(len(values)), nil

	case AggregationMax:
		max := values[0]
		for _, v := range values[1:] {
			if v > max {
				max = v
			}
		}
		return max, nil

	case AggregationMin:
		min := values[0]
		for _, v := range values[1:] {
			if v < min {
				min = v
			}
		}
		return min, nil

	default:
		return 0, fmt.Errorf("unsupported aggregation strategy: %s", strategy)
	}
}

// Helper methods for internal operations

func (ma *MetricsAggregatorImpl) getTimeSeriesFromCluster(
	ctx context.Context,
	clusterName string,
	workspace logicalcluster.Name,
	metricName string,
	timeRange TimeRange,
) (*TimeSeries, error) {
	metrics, err := ma.metricsCollector.CollectClusterMetrics(ctx, clusterName, workspace)
	if err != nil {
		return nil, err
	}

	var points []MetricPoint
	labels := map[string]string{"cluster": clusterName}

	if value, exists := metrics.Metrics[metricName]; exists {
		points = append(points, MetricPoint{
			Timestamp: metrics.Timestamp,
			Value:     value,
			Labels:    map[string]string{"cluster": clusterName},
		})

		// Merge cluster labels
		for k, v := range metrics.Labels {
			labels[k] = v
		}
	}

	if len(points) == 0 {
		return nil, nil
	}

	return &TimeSeries{
		MetricName: metricName,
		Labels:     labels,
		Points:     points,
	}, nil
}
