/*
Copyright 2023 The KCP Authors.

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

package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/component-base/metrics"
)

// TMC metric names follow Prometheus best practices with tmc_ prefix
const (
	MetricNamespace = "tmc"
	
	// Subsystem names for different components
	SyncerSubsystem     = "syncer"
	PlacementSubsystem  = "placement"
	ClusterSubsystem    = "cluster"
	ConnectionSubsystem = "connection"
)

// Common label names used across TMC metrics
const (
	// Workspace labels
	LabelWorkspace = "workspace"
	LabelShard     = "shard"
	
	// Resource labels
	LabelResource    = "resource"
	LabelGroup       = "group"
	LabelVersion     = "version"
	LabelKind        = "kind"
	
	// Cluster labels
	LabelCluster     = "cluster"
	LabelLocation    = "location"
	LabelProvider    = "provider"
	
	// Operation labels
	LabelOperation   = "operation"
	LabelStatus      = "status"
	LabelError       = "error"
	
	// Connection labels
	LabelEndpoint    = "endpoint"
	LabelState       = "state"
)

// PrometheusMetrics provides convenience methods for creating standardized
// TMC Prometheus metrics with consistent naming and labeling.
type PrometheusMetrics struct {
	registry *MetricsRegistry
}

// NewPrometheusMetrics creates a new PrometheusMetrics helper.
func NewPrometheusMetrics(registry *MetricsRegistry) *PrometheusMetrics {
	return &PrometheusMetrics{
		registry: registry,
	}
}

// NewCounterVec creates a new Prometheus CounterVec with TMC naming conventions.
func (p *PrometheusMetrics) NewCounterVec(subsystem, name, help string, labelNames []string) *prometheus.CounterVec {
	opts := prometheus.CounterOpts{
		Namespace: MetricNamespace,
		Subsystem: subsystem,
		Name:      name,
		Help:      help,
	}
	
	return prometheus.NewCounterVec(opts, labelNames)
}

// NewGaugeVec creates a new Prometheus GaugeVec with TMC naming conventions.
func (p *PrometheusMetrics) NewGaugeVec(subsystem, name, help string, labelNames []string) *prometheus.GaugeVec {
	opts := prometheus.GaugeOpts{
		Namespace: MetricNamespace,
		Subsystem: subsystem,
		Name:      name,
		Help:      help,
	}
	
	return prometheus.NewGaugeVec(opts, labelNames)
}

// NewHistogramVec creates a new Prometheus HistogramVec with TMC naming conventions
// and appropriate buckets for different types of operations.
func (p *PrometheusMetrics) NewHistogramVec(subsystem, name, help string, labelNames []string, buckets []float64) *prometheus.HistogramVec {
	opts := prometheus.HistogramOpts{
		Namespace: MetricNamespace,
		Subsystem: subsystem,
		Name:      name,
		Help:      help,
		Buckets:   buckets,
	}
	
	return prometheus.NewHistogramVec(opts, labelNames)
}

// NewSummaryVec creates a new Prometheus SummaryVec with TMC naming conventions.
func (p *PrometheusMetrics) NewSummaryVec(subsystem, name, help string, labelNames []string, objectives map[float64]float64) *prometheus.SummaryVec {
	opts := prometheus.SummaryOpts{
		Namespace:  MetricNamespace,
		Subsystem:  subsystem,
		Name:       name,
		Help:       help,
		Objectives: objectives,
	}
	
	return prometheus.NewSummaryVec(opts, labelNames)
}

// Standard bucket definitions for different types of operations
var (
	// LatencyBuckets for measuring operation latencies (in seconds)
	LatencyBuckets = []float64{
		0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0, 30.0, 60.0,
	}
	
	// SizeBuckets for measuring resource counts, queue depths, etc.
	SizeBuckets = []float64{
		1, 5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000,
	}
	
	// PercentageBuckets for measuring utilization percentages
	PercentageBuckets = []float64{
		10, 20, 30, 40, 50, 60, 70, 80, 90, 95, 99, 100,
	}
	
	// ThroughputBuckets for measuring items per second
	ThroughputBuckets = []float64{
		0.1, 0.5, 1, 2, 5, 10, 20, 50, 100, 200, 500, 1000,
	}
)

// Standard objectives for summary metrics
var (
	// LatencyObjectives for measuring operation latencies
	LatencyObjectives = map[float64]float64{
		0.5:  0.01,  // 50th percentile with 1% error
		0.9:  0.01,  // 90th percentile with 1% error
		0.95: 0.005, // 95th percentile with 0.5% error
		0.99: 0.001, // 99th percentile with 0.1% error
	}
)

// MustRegister registers the provided collectors with the internal Prometheus registry.
// It panics if any collector fails to register.
func (p *PrometheusMetrics) MustRegister(collectors ...prometheus.Collector) {
	p.registry.MustRegister(collectors...)
}

// Register safely registers the provided collector with the internal Prometheus registry.
func (p *PrometheusMetrics) Register(collector prometheus.Collector) error {
	return p.registry.Register(collector)
}

// NewKubernetesMetrics creates component-base metrics compatible with legacy registries.
// This is useful for integrating with existing Kubernetes metrics infrastructure.
func (p *PrometheusMetrics) NewKubernetesMetrics() *KubernetesMetrics {
	return &KubernetesMetrics{
		prom: p,
	}
}

// KubernetesMetrics provides methods for creating Kubernetes component-base metrics
// that integrate with the existing KCP metrics infrastructure.
type KubernetesMetrics struct {
	prom *PrometheusMetrics
}

// NewCounterVec creates a component-base CounterVec.
func (k *KubernetesMetrics) NewCounterVec(opts *metrics.CounterOpts, labelNames []string) *metrics.CounterVec {
	return metrics.NewCounterVec(opts, labelNames)
}

// NewGaugeVec creates a component-base GaugeVec.
func (k *KubernetesMetrics) NewGaugeVec(opts *metrics.GaugeOpts, labelNames []string) *metrics.GaugeVec {
	return metrics.NewGaugeVec(opts, labelNames)
}

// NewHistogramVec creates a component-base HistogramVec.
func (k *KubernetesMetrics) NewHistogramVec(opts *metrics.HistogramOpts, labelNames []string) *metrics.HistogramVec {
	return metrics.NewHistogramVec(opts, labelNames)
}

// Common helper functions for working with labels

// WorkspaceLabels creates a standard set of labels for workspace-scoped metrics.
func WorkspaceLabels(workspace, shard string) prometheus.Labels {
	return prometheus.Labels{
		LabelWorkspace: workspace,
		LabelShard:     shard,
	}
}

// ResourceLabels creates a standard set of labels for resource-scoped metrics.
func ResourceLabels(group, version, resource, kind string) prometheus.Labels {
	return prometheus.Labels{
		LabelGroup:    group,
		LabelVersion:  version,
		LabelResource: resource,
		LabelKind:     kind,
	}
}

// ClusterLabels creates a standard set of labels for cluster-scoped metrics.
func ClusterLabels(cluster, location, provider string) prometheus.Labels {
	return prometheus.Labels{
		LabelCluster:  cluster,
		LabelLocation: location,
		LabelProvider: provider,
	}
}

// OperationLabels creates a standard set of labels for operation-scoped metrics.
func OperationLabels(operation, status string) prometheus.Labels {
	return prometheus.Labels{
		LabelOperation: operation,
		LabelStatus:    status,
	}
}