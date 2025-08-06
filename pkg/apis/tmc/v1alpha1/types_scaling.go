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
	"k8s.io/apimachinery/pkg/util/intstr"
)

// WorkloadScalingPolicy defines scaling policies for TMC workload placement.
// This enables TMC to make intelligent scaling decisions across clusters
// based on workload demand and cluster capacity.
//
// +crd
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories=tmc
// +kubebuilder:printcolumn:name="Min Replicas",type="integer",JSONPath=".spec.minReplicas"
// +kubebuilder:printcolumn:name="Max Replicas",type="integer",JSONPath=".spec.maxReplicas"
// +kubebuilder:printcolumn:name="Current Replicas",type="integer",JSONPath=".status.currentReplicas"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type WorkloadScalingPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WorkloadScalingPolicySpec   `json:"spec,omitempty"`
	Status WorkloadScalingPolicyStatus `json:"status,omitempty"`
}

// WorkloadScalingPolicySpec defines the desired scaling behavior
type WorkloadScalingPolicySpec struct {
	// WorkloadSelector specifies which workloads this scaling policy applies to
	WorkloadSelector WorkloadSelector `json:"workloadSelector"`

	// ClusterSelector specifies which clusters can be used for scaling
	ClusterSelector ClusterSelector `json:"clusterSelector"`

	// MinReplicas is the minimum number of replicas across all clusters
	// +kubebuilder:validation:Minimum=1
	MinReplicas int32 `json:"minReplicas"`

	// MaxReplicas is the maximum number of replicas across all clusters
	// +kubebuilder:validation:Minimum=1
	MaxReplicas int32 `json:"maxReplicas"`

	// ScalingMetrics defines metrics used for scaling decisions
	// +kubebuilder:validation:MinItems=1
	ScalingMetrics []ScalingMetric `json:"scalingMetrics"`
}

// ScalingMetric defines a metric used for scaling decisions
type ScalingMetric struct {
	// Type specifies the type of metric
	// +kubebuilder:validation:Enum=CPUUtilization;MemoryUtilization;RequestsPerSecond;QueueLength;Custom
	Type ScalingMetricType `json:"type"`

	// TargetValue is the target value for this metric
	TargetValue intstr.IntOrString `json:"targetValue"`

	// MetricSelector specifies how to query the metric (required for Custom metrics)
	// +optional
	MetricSelector *MetricSelector `json:"metricSelector,omitempty"`
}

// ScalingMetricType defines the types of metrics for scaling
type ScalingMetricType string

const (
	// CPUUtilizationMetric scales based on CPU utilization percentage
	CPUUtilizationMetric ScalingMetricType = "CPUUtilization"
	// MemoryUtilizationMetric scales based on memory utilization percentage
	MemoryUtilizationMetric ScalingMetricType = "MemoryUtilization"
	// RequestsPerSecondMetric scales based on requests per second
	RequestsPerSecondMetric ScalingMetricType = "RequestsPerSecond"
	// QueueLengthMetric scales based on queue length
	QueueLengthMetric ScalingMetricType = "QueueLength"
	// CustomMetric scales based on a custom metric
	CustomMetric ScalingMetricType = "Custom"
)

// MetricSelector defines how to select and query a scaling metric
type MetricSelector struct {
	// MetricName is the name of the metric
	// +kubebuilder:validation:MinLength=1
	MetricName string `json:"metricName"`

	// Selector defines label selectors for the metric
	// +optional
	Selector *metav1.LabelSelector `json:"selector,omitempty"`

	// AggregationType defines how to aggregate the metric across instances
	// +kubebuilder:validation:Enum=Average;Maximum;Minimum;Sum
	// +optional
	AggregationType *MetricAggregationType `json:"aggregationType,omitempty"`
}

// MetricAggregationType defines how to aggregate metrics
type MetricAggregationType string

const (
	// AverageAggregation uses average values
	AverageAggregation MetricAggregationType = "Average"
	// MaximumAggregation uses maximum values
	MaximumAggregation MetricAggregationType = "Maximum"
	// MinimumAggregation uses minimum values
	MinimumAggregation MetricAggregationType = "Minimum"
	// SumAggregation uses sum of values
	SumAggregation MetricAggregationType = "Sum"
)

// WorkloadScalingPolicyStatus defines the observed scaling state
type WorkloadScalingPolicyStatus struct {
	// Conditions represent the latest available observations
	// +optional
	Conditions conditionsv1alpha1.Conditions `json:"conditions,omitempty"`

	// CurrentReplicas is the current total number of replicas across clusters
	// +optional
	CurrentReplicas *int32 `json:"currentReplicas,omitempty"`

	// DesiredReplicas is the desired total number of replicas
	// +optional
	DesiredReplicas *int32 `json:"desiredReplicas,omitempty"`

	// ClusterReplicas shows current replica distribution across clusters
	// +optional
	ClusterReplicas map[string]int32 `json:"clusterReplicas,omitempty"`

	// LastScaleTime indicates when the last scaling operation occurred
	// +optional
	LastScaleTime *metav1.Time `json:"lastScaleTime,omitempty"`

	// CurrentMetrics shows current values of scaling metrics
	// +optional
	CurrentMetrics []CurrentMetricStatus `json:"currentMetrics,omitempty"`

	// ObservedWorkloads lists workloads currently managed by this policy
	// +optional
	ObservedWorkloads []WorkloadReference `json:"observedWorkloads,omitempty"`
}

// CurrentMetricStatus shows the current status of a scaling metric
type CurrentMetricStatus struct {
	// Type identifies the metric type
	Type ScalingMetricType `json:"type"`

	// CurrentValue is the current value of the metric
	CurrentValue intstr.IntOrString `json:"currentValue"`

	// TargetValue is the target value for this metric
	TargetValue intstr.IntOrString `json:"targetValue"`

	// MetricName is the name of the metric (for custom metrics)
	// +optional
	MetricName string `json:"metricName,omitempty"`
}

// WorkloadScalingPolicyList contains a list of WorkloadScalingPolicy
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type WorkloadScalingPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WorkloadScalingPolicy `json:"items"`
}