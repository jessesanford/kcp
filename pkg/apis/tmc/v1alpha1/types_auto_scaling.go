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
	"k8s.io/apimachinery/pkg/api/resource"
)

// +crd
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories=tmc
// +kubebuilder:printcolumn:name="Strategy",type=string,JSONPath=`.spec.strategy`
// +kubebuilder:printcolumn:name="Min Replicas",type=integer,JSONPath=`.spec.minReplicas`
// +kubebuilder:printcolumn:name="Max Replicas",type=integer,JSONPath=`.spec.maxReplicas`
// +kubebuilder:printcolumn:name="Current Replicas",type=integer,JSONPath=`.status.currentReplicas`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// HorizontalPodAutoscalerPolicy defines auto-scaling policies for workloads across multiple clusters.
// It integrates with TMC's placement engine to provide cluster-aware horizontal pod autoscaling.
type HorizontalPodAutoscalerPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired auto-scaling behavior.
	Spec HorizontalPodAutoscalerPolicySpec `json:"spec,omitempty"`

	// Status represents the current state of the auto-scaling policy.
	// +optional
	Status HorizontalPodAutoscalerPolicyStatus `json:"status,omitempty"`
}

// HorizontalPodAutoscalerPolicySpec defines the desired auto-scaling behavior.
type HorizontalPodAutoscalerPolicySpec struct {
	// Strategy defines the auto-scaling strategy across clusters.
	// +kubebuilder:validation:Enum=Distributed;Centralized;Hybrid
	// +kubebuilder:default=Distributed
	Strategy AutoScalingStrategy `json:"strategy"`

	// MinReplicas is the lower limit for the number of replicas across all clusters.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1
	MinReplicas *int32 `json:"minReplicas,omitempty"`

	// MaxReplicas is the upper limit for the number of replicas across all clusters.
	// +kubebuilder:validation:Minimum=1
	MaxReplicas int32 `json:"maxReplicas"`

	// TargetRef identifies the workload to auto-scale.
	TargetRef CrossClusterObjectReference `json:"targetRef"`

	// Metrics contains the specifications for which to use to calculate the desired replica count.
	// +optional
	// +kubebuilder:validation:MinItems=1
	Metrics []MetricSpec `json:"metrics,omitempty"`

	// Behavior configures the scaling behavior of the target in both Up and Down directions.
	// +optional
	Behavior *HorizontalPodAutoscalerBehavior `json:"behavior,omitempty"`

	// ClusterSelector determines which clusters this policy applies to.
	// +optional
	ClusterSelector *metav1.LabelSelector `json:"clusterSelector,omitempty"`

	// ScaleDownPolicy defines how replicas should be removed when scaling down.
	// +optional
	// +kubebuilder:validation:Enum=Balanced;PreferLocal;PreferRemote
	// +kubebuilder:default=Balanced
	ScaleDownPolicy *ScaleDownPolicy `json:"scaleDownPolicy,omitempty"`

	// ScaleUpPolicy defines how replicas should be added when scaling up.
	// +optional
	// +kubebuilder:validation:Enum=Balanced;PreferLocal;PreferRemote;LoadAware
	// +kubebuilder:default=LoadAware
	ScaleUpPolicy *ScaleUpPolicy `json:"scaleUpPolicy,omitempty"`
}

// HorizontalPodAutoscalerPolicyStatus represents the current state of auto-scaling.
type HorizontalPodAutoscalerPolicyStatus struct {
	// ObservedGeneration is the most recent generation observed by this controller.
	// +optional
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// LastScaleTime is the last time the HorizontalPodAutoscaler scaled the number of pods.
	// +optional
	LastScaleTime *metav1.Time `json:"lastScaleTime,omitempty"`

	// CurrentReplicas is the current number of replicas across all clusters.
	// +optional
	CurrentReplicas *int32 `json:"currentReplicas,omitempty"`

	// DesiredReplicas is the desired number of replicas across all clusters.
	// +optional
	DesiredReplicas *int32 `json:"desiredReplicas,omitempty"`

	// CurrentMetrics is the last read state of the metrics used by this controller.
	// +optional
	CurrentMetrics []MetricStatus `json:"currentMetrics,omitempty"`

	// Conditions represents the latest available observations of the policy's current state.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ClusterStatus contains per-cluster auto-scaling information.
	// +optional
	ClusterStatus []ClusterAutoScalingStatus `json:"clusterStatus,omitempty"`
}

// AutoScalingStrategy defines how auto-scaling decisions are distributed across clusters.
type AutoScalingStrategy string

const (
	// DistributedAutoScaling allows each cluster to make independent scaling decisions.
	DistributedAutoScaling AutoScalingStrategy = "Distributed"

	// CentralizedAutoScaling makes all scaling decisions centrally and distributes replicas.
	CentralizedAutoScaling AutoScalingStrategy = "Centralized"

	// HybridAutoScaling combines both approaches based on cluster capabilities.
	HybridAutoScaling AutoScalingStrategy = "Hybrid"
)

// CrossClusterObjectReference identifies a workload across potentially multiple clusters.
type CrossClusterObjectReference struct {
	// APIVersion of the referent.
	APIVersion string `json:"apiVersion"`

	// Kind of the referent.
	Kind string `json:"kind"`

	// Name of the referent.
	Name string `json:"name"`

	// Namespace of the referent, if applicable.
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// MetricSpec specifies how to scale based on a single metric.
type MetricSpec struct {
	// Type is the type of metric source.
	// +kubebuilder:validation:Enum=Object;Pods;Resource;External;ContainerResource
	Type MetricSourceType `json:"type"`

	// Object refers to a metric describing a single kubernetes object.
	// +optional
	Object *ObjectMetricSource `json:"object,omitempty"`

	// Pods refers to a metric describing each pod in the current scale target.
	// +optional
	Pods *PodsMetricSource `json:"pods,omitempty"`

	// Resource refers to a resource metric known to Kubernetes.
	// +optional
	Resource *ResourceMetricSource `json:"resource,omitempty"`

	// ContainerResource refers to a resource metric known to Kubernetes.
	// +optional
	ContainerResource *ContainerResourceMetricSource `json:"containerResource,omitempty"`

	// External refers to a global metric that is not associated with any Kubernetes object.
	// +optional
	External *ExternalMetricSource `json:"external,omitempty"`
}

// MetricSourceType indicates the type of metric.
type MetricSourceType string

const (
	// ObjectMetricSourceType is a metric describing a kubernetes object.
	ObjectMetricSourceType MetricSourceType = "Object"

	// PodsMetricSourceType is a metric describing each pod in the current scale target.
	PodsMetricSourceType MetricSourceType = "Pods"

	// ResourceMetricSourceType is a resource metric known to Kubernetes.
	ResourceMetricSourceType MetricSourceType = "Resource"

	// ContainerResourceMetricSourceType is a resource metric known to Kubernetes.
	ContainerResourceMetricSourceType MetricSourceType = "ContainerResource"

	// ExternalMetricSourceType is a global metric that is not associated with any Kubernetes object.
	ExternalMetricSourceType MetricSourceType = "External"
)

// ObjectMetricSource indicates how to scale on a metric describing a kubernetes object.
type ObjectMetricSource struct {
	DescribedObject CrossClusterObjectReference `json:"describedObject"`
	Target          MetricTarget                 `json:"target"`
	Metric          MetricIdentifier             `json:"metric"`
}

// PodsMetricSource indicates how to scale on a metric describing each pod.
type PodsMetricSource struct {
	Metric MetricIdentifier `json:"metric"`
	Target MetricTarget     `json:"target"`
}

// ResourceMetricSource indicates how to scale on a resource metric known to Kubernetes.
type ResourceMetricSource struct {
	Name   string       `json:"name"`
	Target MetricTarget `json:"target"`
}

// ContainerResourceMetricSource indicates how to scale on a resource metric known to Kubernetes.
type ContainerResourceMetricSource struct {
	Name      string       `json:"name"`
	Target    MetricTarget `json:"target"`
	Container string       `json:"container"`
}

// ExternalMetricSource indicates how to scale on a metric not associated with any Kubernetes object.
type ExternalMetricSource struct {
	Metric MetricIdentifier `json:"metric"`
	Target MetricTarget     `json:"target"`
}

// MetricIdentifier defines the name and optionally selector for a metric.
type MetricIdentifier struct {
	Name     string                `json:"name"`
	Selector *metav1.LabelSelector `json:"selector,omitempty"`
}

// MetricTarget defines the target value, average value, or average utilization of a specific metric.
type MetricTarget struct {
	Type               MetricTargetType    `json:"type"`
	Value              *resource.Quantity  `json:"value,omitempty"`
	AverageValue       *resource.Quantity  `json:"averageValue,omitempty"`
	AverageUtilization *int32              `json:"averageUtilization,omitempty"`
}

// MetricTargetType specifies the type of metric.
type MetricTargetType string

const (
	// UtilizationMetricType declares a MetricTarget is an AverageUtilization value.
	UtilizationMetricType MetricTargetType = "Utilization"

	// ValueMetricType declares a MetricTarget is a raw value.
	ValueMetricType MetricTargetType = "Value"

	// AverageValueMetricType declares a MetricTarget is an AverageValue.
	AverageValueMetricType MetricTargetType = "AverageValue"
)

// HorizontalPodAutoscalerBehavior configures scaling behavior.
type HorizontalPodAutoscalerBehavior struct {
	// ScaleUp is scaling policy for scaling Up.
	// +optional
	ScaleUp *HPAScalingRules `json:"scaleUp,omitempty"`

	// ScaleDown is scaling policy for scaling Down.
	// +optional
	ScaleDown *HPAScalingRules `json:"scaleDown,omitempty"`
}

// HPAScalingRules configures the scaling behavior for one direction.
type HPAScalingRules struct {
	// StabilizationWindowSeconds is the number of seconds for which past recommendations should be considered.
	// +optional
	StabilizationWindowSeconds *int32 `json:"stabilizationWindowSeconds,omitempty"`

	// SelectPolicy is used to specify which policy should be used.
	// +optional
	SelectPolicy *ScalingPolicySelect `json:"selectPolicy,omitempty"`

	// Policies is a list of potential scaling policies.
	// +optional
	Policies []HPAScalingPolicy `json:"policies,omitempty"`
}

// ScalingPolicySelect is used to specify which policy should be used while scaling.
type ScalingPolicySelect string

const (
	// MaxPolicySelect selects the policy with the highest change.
	MaxPolicySelect ScalingPolicySelect = "Max"

	// MinPolicySelect selects the policy with the lowest change.
	MinPolicySelect ScalingPolicySelect = "Min"

	// DisabledPolicySelect disables the scaling direction.
	DisabledPolicySelect ScalingPolicySelect = "Disabled"
)

// HPAScalingPolicy is a single policy for scaling.
type HPAScalingPolicy struct {
	// Type is used to specify the scaling policy.
	Type HPAScalingPolicyType `json:"type"`

	// Value contains the amount of change which is permitted by the policy.
	Value int32 `json:"value"`

	// PeriodSeconds specifies the window of time for which the policy should hold true.
	PeriodSeconds int32 `json:"periodSeconds"`
}

// HPAScalingPolicyType is the type of scaling policy.
type HPAScalingPolicyType string

const (
	// PodsScalingPolicy is a policy used to specify a change in absolute number of pods.
	PodsScalingPolicy HPAScalingPolicyType = "Pods"

	// PercentScalingPolicy is a policy used to specify a relative amount of change with respect to the current number of pods.
	PercentScalingPolicy HPAScalingPolicyType = "Percent"
)

// ScaleDownPolicy defines how replicas should be removed when scaling down.
type ScaleDownPolicy string

const (
	// BalancedScaleDown removes replicas proportionally from all clusters.
	BalancedScaleDown ScaleDownPolicy = "Balanced"

	// PreferLocalScaleDown removes replicas from local cluster first.
	PreferLocalScaleDown ScaleDownPolicy = "PreferLocal"

	// PreferRemoteScaleDown removes replicas from remote clusters first.
	PreferRemoteScaleDown ScaleDownPolicy = "PreferRemote"
)

// ScaleUpPolicy defines how replicas should be added when scaling up.
type ScaleUpPolicy string

const (
	// BalancedScaleUp adds replicas proportionally to all clusters.
	BalancedScaleUp ScaleUpPolicy = "Balanced"

	// PreferLocalScaleUp adds replicas to local cluster first.
	PreferLocalScaleUp ScaleUpPolicy = "PreferLocal"

	// PreferRemoteScaleUp adds replicas to remote clusters first.
	PreferRemoteScaleUp ScaleUpPolicy = "PreferRemote"

	// LoadAwareScaleUp adds replicas based on cluster load and capacity.
	LoadAwareScaleUp ScaleUpPolicy = "LoadAware"
)

// MetricStatus describes the last-read state of a single metric.
type MetricStatus struct {
	Type MetricSourceType `json:"type"`

	// Object refers to a metric describing a single kubernetes object.
	// +optional
	Object *ObjectMetricStatus `json:"object,omitempty"`

	// Pods refers to a metric describing each pod in the current scale target.
	// +optional
	Pods *PodsMetricStatus `json:"pods,omitempty"`

	// Resource refers to a resource metric known to Kubernetes.
	// +optional
	Resource *ResourceMetricStatus `json:"resource,omitempty"`

	// ContainerResource refers to a resource metric known to Kubernetes.
	// +optional
	ContainerResource *ContainerResourceMetricStatus `json:"containerResource,omitempty"`

	// External refers to a global metric that is not associated with any Kubernetes object.
	// +optional
	External *ExternalMetricStatus `json:"external,omitempty"`
}

// ObjectMetricStatus indicates the current value of a metric describing a kubernetes object.
type ObjectMetricStatus struct {
	Metric       MetricIdentifier        `json:"metric"`
	Current      MetricValueStatus       `json:"current"`
	DescribedObject CrossClusterObjectReference `json:"describedObject"`
}

// PodsMetricStatus indicates the current value of a metric describing each pod.
type PodsMetricStatus struct {
	Metric  MetricIdentifier  `json:"metric"`
	Current MetricValueStatus `json:"current"`
}

// ResourceMetricStatus indicates the current value of a resource metric known to Kubernetes.
type ResourceMetricStatus struct {
	Name    string            `json:"name"`
	Current MetricValueStatus `json:"current"`
}

// ContainerResourceMetricStatus indicates the current value of a resource metric known to Kubernetes.
type ContainerResourceMetricStatus struct {
	Name      string            `json:"name"`
	Current   MetricValueStatus `json:"current"`
	Container string            `json:"container"`
}

// ExternalMetricStatus indicates the current value of a global metric not associated with any Kubernetes object.
type ExternalMetricStatus struct {
	Metric  MetricIdentifier  `json:"metric"`
	Current MetricValueStatus `json:"current"`
}

// MetricValueStatus holds the current value for a metric.
type MetricValueStatus struct {
	Value              *resource.Quantity `json:"value,omitempty"`
	AverageValue       *resource.Quantity `json:"averageValue,omitempty"`
	AverageUtilization *int32             `json:"averageUtilization,omitempty"`
}

// ClusterAutoScalingStatus contains per-cluster auto-scaling information.
type ClusterAutoScalingStatus struct {
	// ClusterName is the name of the cluster.
	ClusterName string `json:"clusterName"`

	// CurrentReplicas is the current number of replicas in this cluster.
	// +optional
	CurrentReplicas *int32 `json:"currentReplicas,omitempty"`

	// DesiredReplicas is the desired number of replicas in this cluster.
	// +optional
	DesiredReplicas *int32 `json:"desiredReplicas,omitempty"`

	// LastScaleTime is the last time this cluster was scaled.
	// +optional
	LastScaleTime *metav1.Time `json:"lastScaleTime,omitempty"`

	// Conditions represents the latest available observations of this cluster's auto-scaling state.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Metrics contains the current metrics for this cluster.
	// +optional
	Metrics []MetricStatus `json:"metrics,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// HorizontalPodAutoscalerPolicyList contains a list of HorizontalPodAutoscalerPolicy.
type HorizontalPodAutoscalerPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HorizontalPodAutoscalerPolicy `json:"items"`
}

// Condition types for HorizontalPodAutoscalerPolicy
const (
	// HorizontalPodAutoscalerPolicyReady indicates the policy is ready and actively scaling.
	HorizontalPodAutoscalerPolicyReady string = "Ready"

	// HorizontalPodAutoscalerPolicyActive indicates the policy is actively making scaling decisions.
	HorizontalPodAutoscalerPolicyActive string = "Active"

	// HorizontalPodAutoscalerPolicyTargetFound indicates the target workload was found.
	HorizontalPodAutoscalerPolicyTargetFound string = "TargetFound"

	// HorizontalPodAutoscalerPolicyMetricsAvailable indicates metrics are available for scaling decisions.
	HorizontalPodAutoscalerPolicyMetricsAvailable string = "MetricsAvailable"

	// HorizontalPodAutoscalerPolicyScalingLimited indicates scaling is limited by constraints.
	HorizontalPodAutoscalerPolicyScalingLimited string = "ScalingLimited"
)