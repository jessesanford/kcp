/*
Copyright 2025 The KCP Authors.

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
	autoscalingv2 "k8s.io/api/autoscaling/v2"
)

// ScalingPolicy defines automated scaling behavior for TMC workloads.
// It provides comprehensive scaling configuration including metrics, behavior,
// and integration with HorizontalPodAutoscaler patterns.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Target",type=string,JSONPath=`.spec.targetRef.name`
// +kubebuilder:printcolumn:name="Min Replicas",type=integer,JSONPath=`.spec.minReplicas`
// +kubebuilder:printcolumn:name="Max Replicas",type=integer,JSONPath=`.spec.maxReplicas`
// +kubebuilder:printcolumn:name="Current",type=integer,JSONPath=`.status.currentReplicas`
// +kubebuilder:printcolumn:name="Desired",type=integer,JSONPath=`.status.desiredReplicas`
type ScalingPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired scaling behavior
	Spec ScalingPolicySpec `json:"spec,omitempty"`

	// Status reflects the observed state of scaling
	// +optional
	Status ScalingPolicyStatus `json:"status,omitempty"`
}

// ScalingPolicySpec defines the configuration for scaling behavior
type ScalingPolicySpec struct {
	// TargetRef identifies the resource to be scaled
	// +kubebuilder:validation:Required
	TargetRef CrossNamespaceObjectReference `json:"targetRef"`

	// MinReplicas specifies the minimum number of replicas
	// If not set, defaults to 1 to avoid scaling to zero
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default:=1
	// +optional
	MinReplicas *int32 `json:"minReplicas,omitempty"`

	// MaxReplicas specifies the maximum number of replicas
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Required
	MaxReplicas int32 `json:"maxReplicas"`

	// Metrics contains the specifications for scaling metrics
	// At least one metric must be specified
	// +kubebuilder:validation:MinItems=1
	// +optional
	Metrics []MetricSpec `json:"metrics,omitempty"`

	// Behavior configures the scaling behavior of the target
	// If not set, the default scaling behavior applies
	// +optional
	Behavior *ScalingBehavior `json:"behavior,omitempty"`
}

// CrossNamespaceObjectReference identifies a resource across namespaces
type CrossNamespaceObjectReference struct {
	// APIVersion of the referenced object
	// +kubebuilder:validation:Required
	APIVersion string `json:"apiVersion"`

	// Kind of the referenced object
	// +kubebuilder:validation:Required
	Kind string `json:"kind"`

	// Name of the referenced object
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Namespace of the referenced object
	// If empty, assumes the same namespace as the ScalingPolicy
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// MetricSpec specifies how to scale on a particular metric type
type MetricSpec struct {
	// Type is the type of metric source
	// +kubebuilder:validation:Enum=Resource;Pods;Object;External
	// +kubebuilder:validation:Required
	Type MetricSourceType `json:"type"`

	// Resource refers to a resource metric known by Kubernetes
	// describing each pod in the current scale target
	// +optional
	Resource *ResourceMetricSource `json:"resource,omitempty"`

	// Pods refers to a metric describing each pod in the current scale target
	// +optional
	Pods *PodsMetricSource `json:"pods,omitempty"`

	// Object refers to a metric describing a single kubernetes object
	// +optional
	Object *ObjectMetricSource `json:"object,omitempty"`

	// External refers to a global metric not associated with any Kubernetes object
	// +optional
	External *ExternalMetricSource `json:"external,omitempty"`

	// ContainerResource refers to a resource metric known by Kubernetes
	// describing a single container in each pod of the current scale target
	// +optional
	ContainerResource *ContainerResourceMetricSource `json:"containerResource,omitempty"`
}

// MetricSourceType indicates the type of metric source
type MetricSourceType string

const (
	// ResourceMetricSourceType is a resource metric known by Kubernetes, as
	// specified in requests and limits, describing each pod in the current
	// scale target (e.g. CPU or memory).
	ResourceMetricSourceType MetricSourceType = "Resource"

	// PodsMetricSourceType is a metric describing each pod in the current scale
	// target (for example, transactions-processed-per-second).
	PodsMetricSourceType MetricSourceType = "Pods"

	// ObjectMetricSourceType is a metric describing a single kubernetes object
	// (for example, hits-per-second on an Ingress object).
	ObjectMetricSourceType MetricSourceType = "Object"

	// ExternalMetricSourceType is a global metric that is not associated
	// with any Kubernetes object.
	ExternalMetricSourceType MetricSourceType = "External"

	// ContainerResourceMetricSourceType is a resource metric known by Kubernetes, as
	// specified in requests and limits, describing a single container in each pod in the
	// current scale target (e.g. CPU or memory).
	ContainerResourceMetricSourceType MetricSourceType = "ContainerResource"
)

// ResourceMetricSource indicates how to scale on a resource metric known by Kubernetes
type ResourceMetricSource struct {
	// Name is the name of the resource in question
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Target specifies the target value for the given metric
	// +kubebuilder:validation:Required
	Target MetricTarget `json:"target"`
}

// PodsMetricSource indicates how to scale on a metric describing each pod
type PodsMetricSource struct {
	// Metric identifies the target metric by name and selector
	// +kubebuilder:validation:Required
	Metric MetricIdentifier `json:"metric"`

	// Target specifies the target value for the given metric
	// +kubebuilder:validation:Required
	Target MetricTarget `json:"target"`
}

// ObjectMetricSource indicates how to scale on a metric describing a kubernetes object
type ObjectMetricSource struct {
	// DescribedObject specifies the descriptions of a object
	// +kubebuilder:validation:Required
	DescribedObject CrossVersionObjectReference `json:"describedObject"`

	// Target specifies the target value for the given metric
	// +kubebuilder:validation:Required
	Target MetricTarget `json:"target"`

	// Metric identifies the target metric by name and selector
	// +kubebuilder:validation:Required
	Metric MetricIdentifier `json:"metric"`
}

// ExternalMetricSource indicates how to scale on a metric not associated with any Kubernetes object
type ExternalMetricSource struct {
	// Metric identifies the target metric by name and selector
	// +kubebuilder:validation:Required
	Metric MetricIdentifier `json:"metric"`

	// Target specifies the target value for the given metric
	// +kubebuilder:validation:Required
	Target MetricTarget `json:"target"`
}

// ContainerResourceMetricSource indicates how to scale on a resource metric known by Kubernetes
type ContainerResourceMetricSource struct {
	// Name is the name of the resource in question
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Target specifies the target value for the given metric
	// +kubebuilder:validation:Required
	Target MetricTarget `json:"target"`

	// Container is the name of the container in the pods of the scaling target
	// +kubebuilder:validation:Required
	Container string `json:"container"`
}

// MetricTarget defines the target value, average value, or average utilization of a specific metric
type MetricTarget struct {
	// Type represents whether the metric type is Utilization, Value, or AverageValue
	// +kubebuilder:validation:Enum=Utilization;Value;AverageValue
	// +kubebuilder:validation:Required
	Type autoscalingv2.MetricTargetType `json:"type"`

	// Value is the target value of the metric (as a quantity)
	// +optional
	Value *resource.Quantity `json:"value,omitempty"`

	// AverageValue is the target value of the average of the metric across all relevant pods
	// +optional
	AverageValue *resource.Quantity `json:"averageValue,omitempty"`

	// AverageUtilization is the target value of the average of the resource metric
	// across all relevant pods, represented as a percentage of the requested value
	// +kubebuilder:validation:Minimum=1
	// +optional
	AverageUtilization *int32 `json:"averageUtilization,omitempty"`
}

// MetricIdentifier defines the name and optional selector for a metric
type MetricIdentifier struct {
	// Name is the name of the given metric
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Selector is the string-encoded form of a standard kubernetes label selector
	// for the given metric
	// +optional
	Selector *metav1.LabelSelector `json:"selector,omitempty"`
}

// CrossVersionObjectReference contains enough information to let you identify the referred resource
type CrossVersionObjectReference struct {
	// APIVersion defines the versioned schema of this representation of an object
	// +kubebuilder:validation:Required
	APIVersion string `json:"apiVersion"`

	// Kind is a string value representing the REST resource this object represents
	// +kubebuilder:validation:Required
	Kind string `json:"kind"`

	// Name of the referent
	// +kubebuilder:validation:Required
	Name string `json:"name"`
}

// ScalingBehavior configures the scaling behavior for one direction
type ScalingBehavior struct {
	// ScaleUp is scaling policy for scaling Up
	// +optional
	ScaleUp *ScalingRules `json:"scaleUp,omitempty"`

	// ScaleDown is scaling policy for scaling Down
	// +optional
	ScaleDown *ScalingRules `json:"scaleDown,omitempty"`
}

// ScalingRules configures the scaling behavior for one direction
type ScalingRules struct {
	// StabilizationWindowSeconds is the number of seconds for which past recommendations should be
	// considered while scaling up or scaling down.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=3600
	// +optional
	StabilizationWindowSeconds *int32 `json:"stabilizationWindowSeconds,omitempty"`

	// SelectPolicy is used to specify which policy should be used
	// +kubebuilder:validation:Enum=Max;Min;Disabled
	// +optional
	SelectPolicy *autoscalingv2.ScalingPolicySelect `json:"selectPolicy,omitempty"`

	// Policies is a list of potential scaling polices which can be used during scaling
	// +kubebuilder:validation:MaxItems=20
	// +optional
	Policies []HPAScalingRule `json:"policies,omitempty"`
}

// HPAScalingRule represents a single policy which must hold true for a specified duration
type HPAScalingRule struct {
	// Type is used to specify the scaling policy
	// +kubebuilder:validation:Enum=Pods;Percent
	// +kubebuilder:validation:Required
	Type autoscalingv2.HPAScalingPolicyType `json:"type"`

	// Value contains the amount of change which is permitted by the policy
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Required
	Value int32 `json:"value"`

	// PeriodSeconds specifies the window of time for which the policy should hold true
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=1800
	// +kubebuilder:validation:Required
	PeriodSeconds int32 `json:"periodSeconds"`
}

// ScalingPolicyStatus describes the runtime state of the scaling policy
type ScalingPolicyStatus struct {
	// ObservedGeneration is the most recent generation observed by this controller
	// +optional
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// LastScaleTime is the last time the scaling policy successfully scaled the target
	// +optional
	LastScaleTime *metav1.Time `json:"lastScaleTime,omitempty"`

	// CurrentReplicas is the current number of replicas of the target resource
	// +optional
	CurrentReplicas int32 `json:"currentReplicas,omitempty"`

	// DesiredReplicas is the desired number of replicas of the target resource
	// +optional
	DesiredReplicas int32 `json:"desiredReplicas,omitempty"`

	// CurrentMetrics is the last read state of the metrics used by this controller
	// +optional
	CurrentMetrics []MetricStatus `json:"currentMetrics,omitempty"`

	// Conditions is the set of conditions required for this policy to scale its target
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// MetricStatus describes the last-read state of a single metric
type MetricStatus struct {
	// Type is the type of metric source
	// +kubebuilder:validation:Enum=Resource;Pods;Object;External
	// +kubebuilder:validation:Required
	Type MetricSourceType `json:"type"`

	// Resource refers to a resource metric
	// +optional
	Resource *ResourceMetricStatus `json:"resource,omitempty"`

	// Pods refers to a metric describing each pod in the current scale target
	// +optional
	Pods *PodsMetricStatus `json:"pods,omitempty"`

	// Object refers to a metric describing a single kubernetes object
	// +optional
	Object *ObjectMetricStatus `json:"object,omitempty"`

	// External refers to a global metric not associated with any Kubernetes object
	// +optional
	External *ExternalMetricStatus `json:"external,omitempty"`

	// ContainerResource refers to a resource metric for a container
	// +optional
	ContainerResource *ContainerResourceMetricStatus `json:"containerResource,omitempty"`
}

// ResourceMetricStatus indicates the current value of a resource metric known by Kubernetes
type ResourceMetricStatus struct {
	// Name is the name of the resource in question
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Current contains the current value for the given metric
	// +kubebuilder:validation:Required
	Current MetricValueStatus `json:"current"`
}

// PodsMetricStatus indicates the current value of a metric describing each pod
type PodsMetricStatus struct {
	// Metric identifies the target metric by name and selector
	// +kubebuilder:validation:Required
	Metric MetricIdentifier `json:"metric"`

	// Current contains the current value for the given metric
	// +kubebuilder:validation:Required
	Current MetricValueStatus `json:"current"`
}

// ObjectMetricStatus indicates the current value of a metric describing a kubernetes object
type ObjectMetricStatus struct {
	// Metric identifies the target metric by name and selector
	// +kubebuilder:validation:Required
	Metric MetricIdentifier `json:"metric"`

	// Current contains the current value for the given metric
	// +kubebuilder:validation:Required
	Current MetricValueStatus `json:"current"`

	// DescribedObject specifies the descriptions of a object
	// +kubebuilder:validation:Required
	DescribedObject CrossVersionObjectReference `json:"describedObject"`
}

// ExternalMetricStatus indicates the current value of a global metric not associated with any Kubernetes object
type ExternalMetricStatus struct {
	// Metric identifies the target metric by name and selector
	// +kubebuilder:validation:Required
	Metric MetricIdentifier `json:"metric"`

	// Current contains the current value for the given metric
	// +kubebuilder:validation:Required
	Current MetricValueStatus `json:"current"`
}

// ContainerResourceMetricStatus indicates the current value of a resource metric known by Kubernetes
type ContainerResourceMetricStatus struct {
	// Name is the name of the resource in question
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Current contains the current value for the given metric
	// +kubebuilder:validation:Required
	Current MetricValueStatus `json:"current"`

	// Container is the name of the container in the pods of the scaling target
	// +kubebuilder:validation:Required
	Container string `json:"container"`
}

// MetricValueStatus holds the current value for a metric
type MetricValueStatus struct {
	// Value is the current value of the metric (as a quantity)
	// +optional
	Value *resource.Quantity `json:"value,omitempty"`

	// AverageValue is the current value of the average of the metric across all relevant pods
	// +optional
	AverageValue *resource.Quantity `json:"averageValue,omitempty"`

	// AverageUtilization is the current value of the average of the resource metric
	// across all relevant pods, represented as a percentage of the requested value
	// +optional
	AverageUtilization *int32 `json:"averageUtilization,omitempty"`
}

// ScalingPolicyList contains a list of ScalingPolicy objects
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
type ScalingPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ScalingPolicy `json:"items"`
}