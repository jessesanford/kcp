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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
)

// MetricsCollectionPolicy defines how metrics are collected and processed for scaling decisions
// in the TMC system. It provides configuration for metric collection, aggregation, and evaluation
// for both HPA and VPA use cases.
//
// +crd
// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories=tmc
// +kubebuilder:printcolumn:name="Sources",type=integer,JSONPath=`.spec.metricSources[*]`,description="Number of metric sources"
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.phase`,description="Policy status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type MetricsCollectionPolicy struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec MetricsCollectionPolicySpec `json:"spec"`

	// +optional
	Status MetricsCollectionPolicyStatus `json:"status,omitempty"`
}

// MetricsCollectionPolicySpec defines the desired metrics collection configuration.
type MetricsCollectionPolicySpec struct {
	// collectionInterval specifies how frequently metrics should be collected.
	//
	// +optional
	// +kubebuilder:default="30s"
	CollectionInterval *metav1.Duration `json:"collectionInterval,omitempty"`

	// retentionPeriod specifies how long metrics should be stored for scaling decisions.
	//
	// +optional
	// +kubebuilder:default="24h"
	RetentionPeriod *metav1.Duration `json:"retentionPeriod,omitempty"`

	// metricSources defines the sources from which metrics should be collected.
	//
	// +required
	// +kubebuilder:validation:MinItems=1
	MetricSources []MetricSourceConfig `json:"metricSources"`

	// aggregationPolicy defines how collected metrics should be aggregated.
	//
	// +optional
	AggregationPolicy *MetricAggregationPolicy `json:"aggregationPolicy,omitempty"`

	// evaluationPolicy defines how metrics should be evaluated for scaling decisions.
	//
	// +optional
	EvaluationPolicy *MetricEvaluationPolicy `json:"evaluationPolicy,omitempty"`

	// placement defines where this metrics collection policy should be applied.
	//
	// +optional
	Placement *MetricsPlacement `json:"placement,omitempty"`
}

// MetricSourceConfig defines a source for metric collection.
type MetricSourceConfig struct {
	// name is a unique identifier for this metric source.
	//
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	Name string `json:"name"`

	// type specifies the type of metric source.
	//
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=Resource;Custom;External;ContainerResource
	Type MetricSourceType `json:"type"`

	// resourceMetric defines configuration for resource-based metrics (CPU, memory).
	//
	// +optional
	ResourceMetric *ResourceMetricConfig `json:"resourceMetric,omitempty"`

	// customMetric defines configuration for custom metrics from applications.
	//
	// +optional
	CustomMetric *CustomMetricConfig `json:"customMetric,omitempty"`

	// externalMetric defines configuration for external metrics from monitoring systems.
	//
	// +optional
	ExternalMetric *ExternalMetricConfig `json:"externalMetric,omitempty"`

	// containerResourceMetric defines configuration for per-container resource metrics.
	//
	// +optional
	ContainerResourceMetric *ContainerResourceMetricConfig `json:"containerResourceMetric,omitempty"`

	// enabled controls whether this metric source is active.
	//
	// +optional
	// +kubebuilder:default=true
	Enabled *bool `json:"enabled,omitempty"`

	// priority defines the priority of this metric source for scaling decisions.
	//
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:default=50
	Priority *int32 `json:"priority,omitempty"`
}

// MetricSourceType defines the type of metric source.
type MetricSourceType string

const (
	// ResourceMetricSourceType indicates resource-based metrics (CPU, memory).
	ResourceMetricSourceType MetricSourceType = "Resource"
	// CustomMetricSourceType indicates custom application metrics.
	CustomMetricSourceType MetricSourceType = "Custom"
	// ExternalMetricSourceType indicates external monitoring system metrics.
	ExternalMetricSourceType MetricSourceType = "External"
	// ContainerResourceMetricSourceType indicates per-container resource metrics.
	ContainerResourceMetricSourceType MetricSourceType = "ContainerResource"
)

// ResourceMetricConfig defines configuration for resource-based metrics.
type ResourceMetricConfig struct {
	// resourceName specifies the resource to monitor (cpu, memory, etc.).
	//
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=cpu;memory;storage;ephemeral-storage
	ResourceName string `json:"resourceName"`

	// targetType specifies how the target will be calculated.
	//
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=Utilization;AverageValue
	TargetType ResourceMetricTargetType `json:"targetType"`

	// averageUtilization is the target average utilization percentage.
	//
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=1000
	AverageUtilization *int32 `json:"averageUtilization,omitempty"`

	// averageValue is the target average absolute value.
	//
	// +optional
	AverageValue *intstr.IntOrString `json:"averageValue,omitempty"`

	// containerName specifies the container to monitor.
	//
	// +optional
	ContainerName string `json:"containerName,omitempty"`
}

// ResourceMetricTargetType specifies the type of resource metric target.
type ResourceMetricTargetType string

const (
	// UtilizationResourceMetricType targets resource utilization as a percentage.
	UtilizationResourceMetricType ResourceMetricTargetType = "Utilization"
	// AverageValueResourceMetricType targets an average absolute value.
	AverageValueResourceMetricType ResourceMetricTargetType = "AverageValue"
)

// CustomMetricConfig defines configuration for custom application metrics.
type CustomMetricConfig struct {
	// metricName is the name of the custom metric.
	//
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	MetricName string `json:"metricName"`

	// metricSelector is used to select which custom metric to use.
	//
	// +optional
	MetricSelector *metav1.LabelSelector `json:"metricSelector,omitempty"`

	// targetType specifies how the target will be calculated.
	//
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=Value;AverageValue
	TargetType CustomMetricTargetType `json:"targetType"`

	// targetValue is the target value of the metric.
	//
	// +optional
	TargetValue *intstr.IntOrString `json:"targetValue,omitempty"`

	// targetAverageValue is the target per-pod value of the metric.
	//
	// +optional
	TargetAverageValue *intstr.IntOrString `json:"targetAverageValue,omitempty"`

	// describedObject is the description of the kubernetes object associated with the metric.
	//
	// +optional
	DescribedObject *CrossVersionObjectReference `json:"describedObject,omitempty"`
}

// CustomMetricTargetType specifies the type of custom metric target.
type CustomMetricTargetType string

const (
	// ValueCustomMetricType targets an absolute value.
	ValueCustomMetricType CustomMetricTargetType = "Value"
	// AverageValueCustomMetricType targets an average value per pod.
	AverageValueCustomMetricType CustomMetricTargetType = "AverageValue"
)

// ExternalMetricConfig defines configuration for external metrics from monitoring systems.
type ExternalMetricConfig struct {
	// metricName is the name of the external metric.
	//
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	MetricName string `json:"metricName"`

	// metricSelector is used to select the specific external metric.
	//
	// +optional
	MetricSelector *metav1.LabelSelector `json:"metricSelector,omitempty"`

	// targetType specifies how the target will be calculated.
	//
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=Value;AverageValue
	TargetType ExternalMetricTargetType `json:"targetType"`

	// targetValue is the target value of the metric.
	//
	// +optional
	TargetValue *intstr.IntOrString `json:"targetValue,omitempty"`

	// targetAverageValue is the target per-pod value of the metric.
	//
	// +optional
	TargetAverageValue *intstr.IntOrString `json:"targetAverageValue,omitempty"`

	// sourceEndpoint defines the endpoint where the external metric can be retrieved.
	//
	// +optional
	SourceEndpoint *MetricSourceEndpoint `json:"sourceEndpoint,omitempty"`
}

// ExternalMetricTargetType specifies the type of external metric target.
type ExternalMetricTargetType string

const (
	// ValueExternalMetricType targets an absolute value.
	ValueExternalMetricType ExternalMetricTargetType = "Value"
	// AverageValueExternalMetricType targets an average value per pod.
	AverageValueExternalMetricType ExternalMetricTargetType = "AverageValue"
)

// ContainerResourceMetricConfig defines configuration for per-container resource metrics.
type ContainerResourceMetricConfig struct {
	// containerName specifies which container to monitor.
	//
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	ContainerName string `json:"containerName"`

	// resourceName specifies the resource to monitor.
	//
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=cpu;memory
	ResourceName string `json:"resourceName"`

	// targetType specifies how the target will be calculated.
	//
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=Utilization;AverageValue
	TargetType ResourceMetricTargetType `json:"targetType"`

	// averageUtilization is the target average utilization percentage.
	//
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=1000
	AverageUtilization *int32 `json:"averageUtilization,omitempty"`

	// averageValue is the target average absolute value.
	//
	// +optional
	AverageValue *intstr.IntOrString `json:"averageValue,omitempty"`
}

// MetricSourceEndpoint defines the endpoint configuration for external metrics.
type MetricSourceEndpoint struct {
	// url is the URL of the metrics endpoint.
	//
	// +required
	// +kubebuilder:validation:Required
	URL string `json:"url"`

	// secretRef references a secret containing authentication credentials.
	//
	// +optional
	SecretRef *SecretReference `json:"secretRef,omitempty"`

	// timeout specifies the timeout for metric collection requests.
	//
	// +optional
	// +kubebuilder:default="30s"
	Timeout *metav1.Duration `json:"timeout,omitempty"`
}

// SecretReference references a Kubernetes secret.
type SecretReference struct {
	// name is the name of the secret.
	//
	// +required
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// namespace is the namespace of the secret.
	//
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// key is the key within the secret containing the credential.
	//
	// +optional
	// +kubebuilder:default="token"
	Key string `json:"key,omitempty"`
}

// MetricAggregationPolicy defines how collected metrics should be aggregated.
type MetricAggregationPolicy struct {
	// aggregationWindow specifies the time window for metric aggregation.
	//
	// +optional
	// +kubebuilder:default="5m"
	AggregationWindow *metav1.Duration `json:"aggregationWindow,omitempty"`

	// aggregationMethod specifies how metrics should be aggregated within the window.
	//
	// +optional
	// +kubebuilder:validation:Enum=Average;Max;Min;Sum
	// +kubebuilder:default=Average
	AggregationMethod *MetricAggregationMethod `json:"aggregationMethod,omitempty"`

	// smoothingFactor applies smoothing to reduce metric volatility.
	//
	// +optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=1
	// +kubebuilder:default=0.1
	SmoothingFactor *float64 `json:"smoothingFactor,omitempty"`

	// enableOutlierDetection controls basic outlier filtering using IQR method.
	//
	// +optional
	// +kubebuilder:default=true
	EnableOutlierDetection *bool `json:"enableOutlierDetection,omitempty"`
}

// MetricAggregationMethod defines how metrics should be aggregated.
type MetricAggregationMethod string

const (
	// AverageAggregation calculates the average value.
	AverageAggregation MetricAggregationMethod = "Average"
	// MaxAggregation uses the maximum value.
	MaxAggregation MetricAggregationMethod = "Max"
	// MinAggregation uses the minimum value.
	MinAggregation MetricAggregationMethod = "Min"
	// SumAggregation calculates the sum of values.
	SumAggregation MetricAggregationMethod = "Sum"
)

// MetricEvaluationPolicy defines how metrics should be evaluated for scaling decisions.
type MetricEvaluationPolicy struct {
	// evaluationInterval specifies how often metrics should be evaluated for scaling.
	//
	// +optional
	// +kubebuilder:default="15s"
	EvaluationInterval *metav1.Duration `json:"evaluationInterval,omitempty"`

	// scaleUpCooldown specifies the minimum time between scale up operations.
	//
	// +optional
	// +kubebuilder:default="3m"
	ScaleUpCooldown *metav1.Duration `json:"scaleUpCooldown,omitempty"`

	// scaleDownCooldown specifies the minimum time between scale down operations.
	//
	// +optional
	// +kubebuilder:default="5m"
	ScaleDownCooldown *metav1.Duration `json:"scaleDownCooldown,omitempty"`

	// tolerance specifies the tolerance for target values.
	//
	// +optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=1
	// +kubebuilder:default=0.1
	Tolerance *float64 `json:"tolerance,omitempty"`

	// minConsecutiveReadings specifies the minimum number of consecutive readings above/below threshold.
	//
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=3
	MinConsecutiveReadings *int32 `json:"minConsecutiveReadings,omitempty"`

	// consensusMode defines how multiple metrics should be considered together.
	//
	// +optional
	// +kubebuilder:validation:Enum=Max;Average
	// +kubebuilder:default=Max
	ConsensusMode *MetricConsensusMode `json:"consensusMode,omitempty"`
}

// MetricConsensusMode defines how multiple metrics should be evaluated together.
type MetricConsensusMode string

const (
	// MaxConsensusMode uses the maximum value from all metrics.
	MaxConsensusMode MetricConsensusMode = "Max"
	// AverageConsensusMode uses the average value from all metrics.
	AverageConsensusMode MetricConsensusMode = "Average"
)

// MetricsPlacement defines where metrics collection should be applied.
type MetricsPlacement struct {
	// clusters specifies the clusters where metrics should be collected.
	//
	// +optional
	Clusters []string `json:"clusters,omitempty"`

	// clusterSelector selects clusters based on labels.
	//
	// +optional
	ClusterSelector *metav1.LabelSelector `json:"clusterSelector,omitempty"`

	// namespaces specifies the namespaces where metrics should be collected.
	//
	// +optional
	Namespaces []string `json:"namespaces,omitempty"`

	// namespaceSelector selects namespaces based on labels.
	//
	// +optional
	NamespaceSelector *metav1.LabelSelector `json:"namespaceSelector,omitempty"`
}

// MetricsCollectionPolicyStatus represents the observed state of the MetricsCollectionPolicy.
type MetricsCollectionPolicyStatus struct {
	// conditions contains the different condition statuses for the policy.
	//
	// +optional
	Conditions conditionsv1alpha1.Conditions `json:"conditions,omitempty"`

	// phase represents the current phase of the metrics collection policy.
	//
	// +optional
	Phase MetricsCollectionPhase `json:"phase,omitempty"`

	// lastCollectionTime is the last time metrics were successfully collected.
	//
	// +optional
	LastCollectionTime *metav1.Time `json:"lastCollectionTime,omitempty"`

	// collectedMetrics contains information about currently collected metrics.
	//
	// +optional
	CollectedMetrics []CollectedMetricInfo `json:"collectedMetrics,omitempty"`

	// activeCollectors tracks the number of active metric collectors.
	//
	// +optional
	ActiveCollectors int32 `json:"activeCollectors,omitempty"`

	// failureCount tracks the number of consecutive collection failures.
	//
	// +optional
	FailureCount int32 `json:"failureCount,omitempty"`

	// observedGeneration reflects the generation of the most recently observed policy specification.
	//
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// MetricsCollectionPhase represents the lifecycle phase of a metrics collection policy.
type MetricsCollectionPhase string

const (
	// MetricsCollectionPending indicates the policy is pending activation.
	MetricsCollectionPending MetricsCollectionPhase = "Pending"
	// MetricsCollectionActive indicates the policy is actively collecting metrics.
	MetricsCollectionActive MetricsCollectionPhase = "Active"
	// MetricsCollectionError indicates the policy encountered errors.
	MetricsCollectionError MetricsCollectionPhase = "Error"
	// MetricsCollectionSuspended indicates the policy is temporarily suspended.
	MetricsCollectionSuspended MetricsCollectionPhase = "Suspended"
)

// CollectedMetricInfo provides information about a collected metric.
type CollectedMetricInfo struct {
	// name is the name of the collected metric.
	//
	// +required
	Name string `json:"name"`

	// type is the type of the collected metric.
	//
	// +required
	Type MetricSourceType `json:"type"`

	// lastValue is the last collected value for this metric.
	//
	// +optional
	LastValue *intstr.IntOrString `json:"lastValue,omitempty"`

	// lastCollectionTime is when this metric was last successfully collected.
	//
	// +optional
	LastCollectionTime *metav1.Time `json:"lastCollectionTime,omitempty"`

	// collectionSuccess indicates whether the last collection attempt was successful.
	//
	// +optional
	CollectionSuccess bool `json:"collectionSuccess,omitempty"`

	// errorMessage contains any error from the last collection attempt.
	//
	// +optional
	ErrorMessage string `json:"errorMessage,omitempty"`
}

// CrossVersionObjectReference contains enough information to let you identify the referred resource.
type CrossVersionObjectReference struct {
	// kind is the kind of the referent.
	//
	// +required
	Kind string `json:"kind"`

	// name is the name of the referent.
	//
	// +required
	Name string `json:"name"`

	// apiVersion is the API version of the referent.
	//
	// +optional
	APIVersion string `json:"apiVersion,omitempty"`
}

// These are valid condition types for MetricsCollectionPolicy.
const (
	// MetricsCollectionReady indicates that the metrics collection policy is ready.
	MetricsCollectionReady conditionsv1alpha1.ConditionType = "Ready"

	// MetricsCollectionActive indicates that metrics are actively being collected.
	MetricsCollectionActive conditionsv1alpha1.ConditionType = "Active"

	// MetricsCollectionProgressing indicates that the policy is being processed.
	MetricsCollectionProgressing conditionsv1alpha1.ConditionType = "Progressing"

	// MetricsCollectionDegraded indicates that metrics collection is experiencing issues.
	MetricsCollectionDegraded conditionsv1alpha1.ConditionType = "Degraded"
)

// MetricsCollectionPolicyList contains a list of MetricsCollectionPolicy.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type MetricsCollectionPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MetricsCollectionPolicy `json:"items"`
}

func (in *MetricsCollectionPolicy) SetConditions(c conditionsv1alpha1.Conditions) {
	in.Status.Conditions = c
}

func (in *MetricsCollectionPolicy) GetConditions() conditionsv1alpha1.Conditions {
	return in.Status.Conditions
}