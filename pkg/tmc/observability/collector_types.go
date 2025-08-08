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
	"time"

	"github.com/kcp-dev/logicalcluster/v3"
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

// ConsolidatedTimeSeries represents a consolidated time series with metadata.
type ConsolidatedTimeSeries struct {
	*TimeSeries
	Config          ConsolidationConfig `json:"config"`
	OriginalPoints  int                 `json:"original_points"`
	ConsolidatedBy  float64             `json:"consolidated_by"`
	SourceWorkspace logicalcluster.Name `json:"source_workspace"`
}

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

// TimeRange defines a time range for metrics queries.
type TimeRange struct {
	Start time.Time     `json:"start"`
	End   time.Time     `json:"end"`
	Step  time.Duration `json:"step,omitempty"`
}

// MetricsSource represents a source of metrics data from a cluster.
type MetricsSource interface {
	// GetMetricValue retrieves a specific metric value from a cluster
	GetMetricValue(ctx context.Context, clusterName string, workspace logicalcluster.Name, metricName string) (float64, map[string]string, error)

	// ListClusters returns available clusters in a workspace
	ListClusters(ctx context.Context, workspace logicalcluster.Name) ([]string, error)
}