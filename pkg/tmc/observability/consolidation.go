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
	"math"
	"sort"
	"time"

	"k8s.io/klog/v2"
	utilfeature "k8s.io/apiserver/pkg/util/feature"

	"github.com/kcp-dev/logicalcluster/v3"
	"github.com/kcp-dev/kcp/pkg/features"
)

// ConsolidationConfig defines configuration for time-series consolidation.
type ConsolidationConfig struct {
	// MaxDataPoints is the maximum number of data points to return
	MaxDataPoints int `json:"max_data_points,omitempty"`
	// ConsolidationFunction defines how to consolidate data points
	ConsolidationFunction ConsolidationFunction `json:"consolidation_function,omitempty"`
	// Tolerance defines the time tolerance for aligning data points
	Tolerance time.Duration `json:"tolerance,omitempty"`
}

// ConsolidationFunction defines how to consolidate multiple data points.
type ConsolidationFunction string

const (
	// ConsolidationAverage averages data points within a time window
	ConsolidationAverage ConsolidationFunction = "average"
	// ConsolidationMax takes the maximum data point within a time window
	ConsolidationMax ConsolidationFunction = "max"
	// ConsolidationMin takes the minimum data point within a time window
	ConsolidationMin ConsolidationFunction = "min"
)

// DefaultConsolidationConfig returns the default consolidation configuration.
func DefaultConsolidationConfig() ConsolidationConfig {
	return ConsolidationConfig{
		MaxDataPoints:         1000,
		ConsolidationFunction: ConsolidationAverage,
		Tolerance:             time.Minute,
	}
}

// TimeSeriesConsolidator handles consolidation of time-series data.
type TimeSeriesConsolidator struct {
	metricsSource MetricsSource
	config        ConsolidationConfig
}

// NewTimeSeriesConsolidator creates a new time series consolidator.
func NewTimeSeriesConsolidator(metricsSource MetricsSource, config ConsolidationConfig) *TimeSeriesConsolidator {
	return &TimeSeriesConsolidator{
		metricsSource: metricsSource,
		config:        config,
	}
}

// ConsolidateTimeSeries consolidates time series data from multiple clusters.
// 
// This method collects time series data and applies consolidation to reduce 
// data points while preserving trends.
func (tsc *TimeSeriesConsolidator) ConsolidateTimeSeries(
	ctx context.Context,
	workspace logicalcluster.Name,
	metricName string,
	strategy AggregationStrategy,
	timeRange TimeRange,
) (*ConsolidatedTimeSeries, error) {
	// Check if time series consolidation is enabled
	if !utilfeature.DefaultFeatureGate.Enabled(features.TMCTimeSeriesConsolidation) {
		return nil, fmt.Errorf("TMC time series consolidation is disabled")
	}

	klog.V(4).InfoS("Consolidating time series", "workspace", workspace, "metric", metricName)

	// Get clusters in workspace
	clusters, err := tsc.metricsSource.ListClusters(ctx, workspace)
	if err != nil {
		return nil, fmt.Errorf("failed to list clusters: %w", err)
	}

	if len(clusters) == 0 {
		return nil, fmt.Errorf("no clusters found in workspace %s", workspace)
	}

	// Collect time series data from all clusters
	var allPoints []MetricPoint
	var aggregatedLabels map[string]string

	for _, clusterName := range clusters {
		points, labels, err := tsc.simulateTimeSeriesFromCluster(ctx, clusterName, workspace, metricName, timeRange)
		if err != nil {
			klog.V(2).InfoS("Failed to get time series from cluster", "cluster", clusterName, "error", err)
			continue
		}

		allPoints = append(allPoints, points...)
		if aggregatedLabels == nil {
			aggregatedLabels = make(map[string]string)
			for k, v := range labels {
				aggregatedLabels[k] = v
			}
		}
	}

	if len(allPoints) == 0 {
		return nil, fmt.Errorf("no time series data found for %s in workspace %s", metricName, workspace)
	}

	// Sort points by timestamp
	sort.Slice(allPoints, func(i, j int) bool {
		return allPoints[i].Timestamp.Before(allPoints[j].Timestamp)
	})

	originalCount := len(allPoints)

	// Apply consolidation based on configuration
	consolidatedPoints, err := tsc.applyConsolidation(allPoints, timeRange)
	if err != nil {
		return nil, fmt.Errorf("failed to apply consolidation: %w", err)
	}

	consolidationRatio := float64(originalCount) / float64(len(consolidatedPoints))

	return &ConsolidatedTimeSeries{
		TimeSeries: &TimeSeries{
			MetricName: metricName,
			Labels:     aggregatedLabels,
			Points:     consolidatedPoints,
		},
		Config:          tsc.config,
		OriginalPoints:  originalCount,
		ConsolidatedBy:  consolidationRatio,
		SourceWorkspace: workspace,
	}, nil
}

// simulateTimeSeriesFromCluster generates synthetic time series data.
// In practice, this would query the cluster's metrics endpoint.
func (tsc *TimeSeriesConsolidator) simulateTimeSeriesFromCluster(
	ctx context.Context,
	clusterName string,
	workspace logicalcluster.Name,
	metricName string,
	timeRange TimeRange,
) ([]MetricPoint, map[string]string, error) {
	step := timeRange.Step
	if step == 0 {
		step = time.Minute
	}

	var points []MetricPoint
	labels := map[string]string{
		"cluster": clusterName,
		"metric":  metricName,
	}

	// Generate synthetic time series data
	for t := timeRange.Start; t.Before(timeRange.End) || t.Equal(timeRange.End); t = t.Add(step) {
		baseValue := float64(len(clusterName)) * 10
		variation := math.Sin(float64(t.Unix())/3600) * 5
		value := baseValue + variation

		points = append(points, MetricPoint{
			Timestamp: t,
			Value:     value,
			Labels:    labels,
		})
	}

	return points, labels, nil
}

// applyConsolidation applies consolidation to reduce data points.
func (tsc *TimeSeriesConsolidator) applyConsolidation(
	points []MetricPoint,
	timeRange TimeRange,
) ([]MetricPoint, error) {
	if len(points) <= tsc.config.MaxDataPoints {
		return points, nil
	}

	// Calculate consolidation window
	totalDuration := timeRange.End.Sub(timeRange.Start)
	windowSize := totalDuration / time.Duration(tsc.config.MaxDataPoints)

	if windowSize < tsc.config.Tolerance {
		windowSize = tsc.config.Tolerance
	}

	var consolidated []MetricPoint
	windowStart := timeRange.Start

	for windowStart.Before(timeRange.End) {
		windowEnd := windowStart.Add(windowSize)
		if windowEnd.After(timeRange.End) {
			windowEnd = timeRange.End
		}

		// Find points within window
		var windowPoints []MetricPoint
		for _, point := range points {
			if (point.Timestamp.After(windowStart) || point.Timestamp.Equal(windowStart)) &&
				point.Timestamp.Before(windowEnd) {
				windowPoints = append(windowPoints, point)
			}
		}

		if len(windowPoints) > 0 {
			consolidatedPoint, err := tsc.consolidateWindowPoints(windowPoints, windowStart.Add(windowSize/2))
			if err != nil {
				return nil, fmt.Errorf("failed to consolidate window points: %w", err)
			}
			consolidated = append(consolidated, consolidatedPoint)
		}

		windowStart = windowEnd
	}

	return consolidated, nil
}

// consolidateWindowPoints consolidates multiple points into a single point.
func (tsc *TimeSeriesConsolidator) consolidateWindowPoints(
	points []MetricPoint,
	timestamp time.Time,
) (MetricPoint, error) {
	if len(points) == 0 {
		return MetricPoint{}, fmt.Errorf("no points to consolidate")
	}

	if len(points) == 1 {
		result := points[0]
		result.Timestamp = timestamp
		return result, nil
	}

	var value float64
	labels := make(map[string]string)

	// Copy labels from first point
	for k, v := range points[0].Labels {
		labels[k] = v
	}

	switch tsc.config.ConsolidationFunction {
	case ConsolidationAverage:
		sum := 0.0
		for _, point := range points {
			sum += point.Value
		}
		value = sum / float64(len(points))

	case ConsolidationMax:
		value = points[0].Value
		for _, point := range points[1:] {
			if point.Value > value {
				value = point.Value
			}
		}

	case ConsolidationMin:
		value = points[0].Value
		for _, point := range points[1:] {
			if point.Value < value {
				value = point.Value
			}
		}

	default:
		return MetricPoint{}, fmt.Errorf("unsupported consolidation function: %s", tsc.config.ConsolidationFunction)
	}

	return MetricPoint{
		Timestamp: timestamp,
		Value:     value,
		Labels:    labels,
	}, nil
}

// ValidateConsolidationFunction validates the given consolidation function.
func ValidateConsolidationFunction(function ConsolidationFunction) error {
	switch function {
	case ConsolidationAverage, ConsolidationMax, ConsolidationMin:
		return nil
	default:
		return fmt.Errorf("unsupported consolidation function: %s", function)
	}
}