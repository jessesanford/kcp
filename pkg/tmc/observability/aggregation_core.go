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

	"k8s.io/klog/v2"
	utilfeature "k8s.io/apiserver/pkg/util/feature"

	"github.com/kcp-dev/logicalcluster/v3"
	"github.com/kcp-dev/kcp/pkg/features"
)

// MetricsAggregatorImpl implements the MetricsAggregator interface.
type MetricsAggregatorImpl struct {
	mu            sync.RWMutex
	metricsSource MetricsSource
}

// NewMetricsAggregator creates a new metrics aggregator.
//
// Parameters:
//   - metricsSource: Source for collecting metrics from clusters
//
// Returns:
//   - MetricsAggregator: Configured aggregator ready for use
func NewMetricsAggregator(metricsSource MetricsSource) MetricsAggregator {
	return &MetricsAggregatorImpl{
		metricsSource: metricsSource,
	}
}

// AggregateMetrics aggregates metrics from multiple clusters using the specified strategy.
//
// This method collects metrics from all available clusters in a workspace and applies
// the specified aggregation strategy to produce a single aggregated value.
//
// Parameters:
//   - ctx: Context for the aggregation operation
//   - workspace: Logical cluster workspace to aggregate metrics from
//   - metricName: Name of the metric to aggregate
//   - strategy: Aggregation strategy to apply (sum, avg, max, min)
//   - timeRange: Time range for the aggregation (currently used for timestamp)
//
// Returns:
//   - *AggregatedMetric: Aggregated metric result
//   - error: Aggregation error if any
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
	clusters, err := ma.metricsSource.ListClusters(ctx, workspace)
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
		value, labels, err := ma.metricsSource.GetMetricValue(ctx, clusterName, workspace, metricName)
		if err != nil {
			klog.V(2).InfoS("Failed to get metric from cluster", "cluster", clusterName, "error", err)
			continue
		}

		clusterValues = append(clusterValues, value)
		sourceClusters = append(sourceClusters, clusterName)

		// Merge labels from first cluster
		if aggregatedLabels == nil {
			aggregatedLabels = make(map[string]string)
			for k, v := range labels {
				aggregatedLabels[k] = v
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
//
// This is a placeholder implementation that returns an error indicating
// time series consolidation is not implemented in this core module.
//
// Parameters:
//   - ctx: Context for the aggregation operation
//   - workspace: Logical cluster workspace
//   - metricName: Name of the metric to aggregate
//   - strategy: Aggregation strategy to apply
//   - timeRange: Time range for the aggregation
//
// Returns:
//   - *TimeSeries: Always nil in this implementation
//   - error: Error indicating consolidation module is required
func (ma *MetricsAggregatorImpl) AggregateTimeSeries(
	ctx context.Context,
	workspace logicalcluster.Name,
	metricName string,
	strategy AggregationStrategy,
	timeRange TimeRange,
) (*TimeSeries, error) {
	return nil, fmt.Errorf("time series aggregation requires TMC consolidation module")
}

// applyAggregationStrategy applies the specified aggregation strategy to a set of values.
//
// Supported strategies:
//   - sum: Sum all values
//   - avg: Average all values
//   - max: Maximum value
//   - min: Minimum value
//
// Parameters:
//   - strategy: Aggregation strategy to apply
//   - values: Values to aggregate
//
// Returns:
//   - float64: Aggregated result
//   - error: Strategy application error if any
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

// ValidateAggregationStrategy validates that the given aggregation strategy is supported.
//
// Parameters:
//   - strategy: Strategy to validate
//
// Returns:
//   - error: Validation error if strategy is unsupported
func ValidateAggregationStrategy(strategy AggregationStrategy) error {
	switch strategy {
	case AggregationSum, AggregationAvg, AggregationMax, AggregationMin:
		return nil
	default:
		return fmt.Errorf("unsupported aggregation strategy: %s", strategy)
	}
}