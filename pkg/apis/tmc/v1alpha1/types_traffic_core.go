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

package v1alpha1

import (
	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TrafficMetrics provides basic traffic analysis for TMC workload placement decisions.
// This API enables TMC to make intelligent placement decisions based on actual
// traffic performance across clusters.
//
// +crd
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories=tmc
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Success Rate",type="string",JSONPath=".status.metrics.successRate"
// +kubebuilder:printcolumn:name="Avg Latency",type="string",JSONPath=".status.metrics.averageLatency"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type TrafficMetrics struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TrafficMetricsSpec   `json:"spec,omitempty"`
	Status TrafficMetricsStatus `json:"status,omitempty"`
}

// TrafficMetricsSpec defines the desired traffic metrics collection
type TrafficMetricsSpec struct {
	// WorkloadSelector specifies which workloads to analyze traffic for
	WorkloadSelector WorkloadSelector `json:"workloadSelector"`

	// ClusterSelector specifies which clusters to collect metrics from
	ClusterSelector ClusterSelector `json:"clusterSelector"`

	// MetricsSource defines where to collect traffic metrics
	MetricsSource TrafficSource `json:"metricsSource"`

	// CollectionInterval defines how often to collect metrics
	// +optional
	CollectionInterval *metav1.Duration `json:"collectionInterval,omitempty"`

	// RetentionPeriod defines how long to keep collected metrics
	// +optional
	RetentionPeriod *metav1.Duration `json:"retentionPeriod,omitempty"`
}

// TrafficSource defines where to collect traffic metrics from
type TrafficSource struct {
	// Type specifies the metrics source type
	Type TrafficSourceType `json:"type"`

	// Endpoint is the metrics collection endpoint URL
	// Required for Prometheus and Custom sources
	// +optional
	Endpoint string `json:"endpoint,omitempty"`

	// MetricsPath defines the path for metrics collection
	// +optional
	MetricsPath string `json:"metricsPath,omitempty"`

	// Labels defines additional labels to filter metrics
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
}

// TrafficSourceType defines supported traffic metrics sources
type TrafficSourceType string

const (
	// PrometheusSource collects metrics from Prometheus
	PrometheusSource TrafficSourceType = "Prometheus"
	// IstioSource collects metrics from Istio service mesh
	IstioSource TrafficSourceType = "Istio"
	// CustomSource collects metrics from a custom endpoint
	CustomSource TrafficSourceType = "Custom"
)

// TrafficMetricsStatus defines the observed traffic metrics
type TrafficMetricsStatus struct {
	// Conditions represent the latest available observations
	// +optional
	Conditions conditionsv1alpha1.Conditions `json:"conditions,omitempty"`

	// Phase indicates the current collection phase
	// +optional
	Phase TrafficMetricsPhase `json:"phase,omitempty"`

	// LastUpdateTime indicates when metrics were last collected
	// +optional
	LastUpdateTime *metav1.Time `json:"lastUpdateTime,omitempty"`

	// Metrics contains the current traffic metrics by cluster
	// +optional
	Metrics map[string]ClusterTrafficMetrics `json:"metrics,omitempty"`

	// ObservedWorkloads lists workloads currently being monitored
	// +optional
	ObservedWorkloads []WorkloadReference `json:"observedWorkloads,omitempty"`

	// TotalRequests is the sum of all requests across clusters
	// +optional
	TotalRequests *int64 `json:"totalRequests,omitempty"`

	// OverallSuccessRate is the weighted success rate across all clusters (0-100)
	// +optional
	OverallSuccessRate *float64 `json:"overallSuccessRate,omitempty"`
}

// TrafficMetricsPhase defines the phase of traffic metrics collection
type TrafficMetricsPhase string

const (
	// InitializingPhase indicates metrics collection is starting
	InitializingPhase TrafficMetricsPhase = "Initializing"
	// CollectingPhase indicates metrics are being actively collected
	CollectingPhase TrafficMetricsPhase = "Collecting"
	// AnalyzingPhase indicates collected metrics are being processed
	AnalyzingPhase TrafficMetricsPhase = "Analyzing"
	// ReadyPhase indicates metrics are ready for TMC placement decisions
	ReadyPhase TrafficMetricsPhase = "Ready"
	// FailedPhase indicates metrics collection has failed
	FailedPhase TrafficMetricsPhase = "Failed"
)

// ClusterTrafficMetrics contains traffic metrics for a specific cluster
type ClusterTrafficMetrics struct {
	// ClusterName identifies the cluster these metrics apply to
	ClusterName string `json:"clusterName"`

	// RequestCount is the number of requests processed
	RequestCount int64 `json:"requestCount"`

	// SuccessRate is the percentage of successful requests (0-100)
	SuccessRate float64 `json:"successRate"`

	// AverageLatency is the average request latency in milliseconds
	AverageLatency int64 `json:"averageLatency"`

	// P95Latency is the 95th percentile latency in milliseconds
	// +optional
	P95Latency *int64 `json:"p95Latency,omitempty"`

	// ErrorCount is the number of failed requests
	ErrorCount int64 `json:"errorCount"`

	// Throughput is requests per second
	Throughput float64 `json:"throughput"`

	// LastUpdated indicates when these metrics were last collected
	LastUpdated metav1.Time `json:"lastUpdated"`

	// HealthScore is a computed health score for TMC placement (0-100, higher is better)
	// Combines success rate, latency, and throughput into a single placement score
	// +optional
	HealthScore *float64 `json:"healthScore,omitempty"`
}

// TrafficMetricsList contains a list of TrafficMetrics
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type TrafficMetricsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TrafficMetrics `json:"items"`
}