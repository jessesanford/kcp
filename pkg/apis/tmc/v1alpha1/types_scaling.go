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

// AutoScalingPolicy defines the scaling configuration for workloads in the TMC system.
// It provides comprehensive autoscaling capabilities including horizontal pod autoscaling,
// vertical pod autoscaling, and custom metric-based scaling.
//
// +crd
// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories=tmc
// +kubebuilder:printcolumn:name="Target",type=string,JSONPath=`.spec.targetRef.name`,description="Target workload for scaling"
// +kubebuilder:printcolumn:name="Min Replicas",type=integer,JSONPath=`.spec.horizontalPodAutoScaler.minReplicas`,description="Minimum replicas"
// +kubebuilder:printcolumn:name="Max Replicas",type=integer,JSONPath=`.spec.horizontalPodAutoScaler.maxReplicas`,description="Maximum replicas"
// +kubebuilder:printcolumn:name="Target CPU",type=string,JSONPath=`.spec.horizontalPodAutoScaler.targetCPUUtilizationPercentage`,description="Target CPU utilization"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type AutoScalingPolicy struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec AutoScalingPolicySpec `json:"spec"`

	// +optional
	Status AutoScalingPolicyStatus `json:"status,omitempty"`
}

// AutoScalingPolicySpec defines the desired scaling behavior for a workload.
type AutoScalingPolicySpec struct {
	// targetRef points to the target resource to scale, which can be a Deployment,
	// StatefulSet, or any other scalable resource across logical clusters in TMC.
	//
	// +required
	// +kubebuilder:validation:Required
	TargetRef ScaleTargetRef `json:"targetRef"`

	// horizontalPodAutoScaler defines the horizontal scaling configuration.
	// This includes replica scaling based on CPU, memory, and custom metrics.
	//
	// +optional
	HorizontalPodAutoScaler *HorizontalPodAutoScalerSpec `json:"horizontalPodAutoScaler,omitempty"`

	// verticalPodAutoScaler defines the vertical scaling configuration.
	// This includes resource request and limit adjustments.
	//
	// +optional
	VerticalPodAutoScaler *VerticalPodAutoScalerSpec `json:"verticalPodAutoScaler,omitempty"`

	// scalingPolicy defines advanced scaling behaviors and policies.
	//
	// +optional
	ScalingPolicy *ScalingPolicy `json:"scalingPolicy,omitempty"`

	// placement defines where this scaling policy should be applied across
	// the TMC clusters and regions.
	//
	// +optional
	Placement *ScalingPlacement `json:"placement,omitempty"`
}

// ScaleTargetRef identifies the target resource to scale in TMC multi-cluster environment.
type ScaleTargetRef struct {
	// apiVersion is the API group and version of the target resource.
	//
	// +required
	// +kubebuilder:validation:Required
	APIVersion string `json:"apiVersion"`

	// kind is the kind of the target resource.
	//
	// +required
	// +kubebuilder:validation:Required
	Kind string `json:"kind"`

	// name is the name of the target resource.
	//
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// namespace is the namespace of the target resource. For cluster-scoped
	// resources, this field should be empty.
	//
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// logicalCluster specifies the logical cluster where the target resource
	// is located in the TMC environment.
	//
	// +optional
	LogicalCluster string `json:"logicalCluster,omitempty"`
}

// HorizontalPodAutoScalerSpec defines the horizontal scaling configuration.
type HorizontalPodAutoScalerSpec struct {
	// minReplicas is the lower limit for the number of replicas to which
	// the autoscaler can scale down.
	//
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1
	MinReplicas *int32 `json:"minReplicas,omitempty"`

	// maxReplicas is the upper limit for the number of replicas to which
	// the autoscaler can scale up. It cannot be smaller than MinReplicas.
	//
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	MaxReplicas int32 `json:"maxReplicas"`

	// targetCPUUtilizationPercentage is the target average CPU utilization
	// (represented as a percentage of requested CPU) over all the pods.
	//
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	TargetCPUUtilizationPercentage *int32 `json:"targetCPUUtilizationPercentage,omitempty"`

	// targetMemoryUtilizationPercentage is the target average memory utilization
	// (represented as a percentage of requested memory) over all the pods.
	//
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	TargetMemoryUtilizationPercentage *int32 `json:"targetMemoryUtilizationPercentage,omitempty"`

	// metrics contains the specifications for which to use to calculate the
	// desired replica count (the maximum replica count across all metrics will
	// be used).
	//
	// +optional
	Metrics []MetricSpec `json:"metrics,omitempty"`
}

// VerticalPodAutoScalerSpec defines the vertical scaling configuration.
type VerticalPodAutoScalerSpec struct {
	// updateMode controls whether VPA applies resource recommendations.
	//
	// +optional
	// +kubebuilder:validation:Enum=Off;Initial;Recreate;Auto
	// +kubebuilder:default=Auto
	UpdateMode *VPAUpdateMode `json:"updateMode,omitempty"`

	// resourcePolicy controls how the autoscaler computes recommended resources.
	//
	// +optional
	ResourcePolicy *VPAResourcePolicy `json:"resourcePolicy,omitempty"`
}

// VPAUpdateMode represents the update mode for VPA.
type VPAUpdateMode string

const (
	// VPAUpdateModeOff means VPA will not take any action for the workload.
	VPAUpdateModeOff VPAUpdateMode = "Off"
	// VPAUpdateModeInitial means VPA will only apply recommendations when pods are created.
	VPAUpdateModeInitial VPAUpdateMode = "Initial"
	// VPAUpdateModeRecreate means VPA will recreate pods when necessary to apply recommendations.
	VPAUpdateModeRecreate VPAUpdateMode = "Recreate"
	// VPAUpdateModeAuto means VPA will automatically apply recommendations.
	VPAUpdateModeAuto VPAUpdateMode = "Auto"
)

// VPAResourcePolicy controls resource recommendation computation.
type VPAResourcePolicy struct {
	// containerPolicies controls how the autoscaler computes resource recommendations
	// for containers belonging to the target pod.
	//
	// +optional
	ContainerPolicies []VPAContainerResourcePolicy `json:"containerPolicies,omitempty"`
}

// VPAContainerResourcePolicy controls resource recommendation for a container.
type VPAContainerResourcePolicy struct {
	// containerName is the name of the container or DefaultContainerResourcePolicy.
	//
	// +optional
	ContainerName *string `json:"containerName,omitempty"`

	// mode controls whether the VPA will apply the recommendation for this container.
	//
	// +optional
	// +kubebuilder:validation:Enum=Off;Auto
	Mode *VPAContainerScalingMode `json:"mode,omitempty"`

	// minAllowed specifies the minimal amount of resources that will be recommended.
	//
	// +optional
	MinAllowed map[string]intstr.IntOrString `json:"minAllowed,omitempty"`

	// maxAllowed specifies the maximum amount of resources that will be recommended.
	//
	// +optional
	MaxAllowed map[string]intstr.IntOrString `json:"maxAllowed,omitempty"`
}

// VPAContainerScalingMode represents the scaling mode for a container in VPA.
type VPAContainerScalingMode string

const (
	// VPAContainerScalingModeOff disables scaling for this container.
	VPAContainerScalingModeOff VPAContainerScalingMode = "Off"
	// VPAContainerScalingModeAuto enables automatic scaling for this container.
	VPAContainerScalingModeAuto VPAContainerScalingMode = "Auto"
)

// MetricSpec specifies how to scale based on a single metric.
type MetricSpec struct {
	// type is the type of metric source. It should be one of "Object", "Pods", "Resource", or "External".
	//
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=Object;Pods;Resource;External
	Type MetricSourceType `json:"type"`

	// object refers to a metric describing a single kubernetes object.
	//
	// +optional
	Object *ObjectMetricSource `json:"object,omitempty"`

	// pods refers to a metric describing each pod in the current scale target.
	//
	// +optional
	Pods *PodsMetricSource `json:"pods,omitempty"`

	// resource refers to a resource metric (such as those specified in requests and limits).
	//
	// +optional
	Resource *ResourceMetricSource `json:"resource,omitempty"`

	// external refers to a global metric that is not associated with any Kubernetes object.
	//
	// +optional
	External *ExternalMetricSource `json:"external,omitempty"`
}

// MetricSourceType indicates the type of metric.
type MetricSourceType string

const (
	// ObjectMetricSourceType is a metric describing a single kubernetes object.
	ObjectMetricSourceType MetricSourceType = "Object"
	// PodsMetricSourceType is a metric describing each pod in the current scale target.
	PodsMetricSourceType MetricSourceType = "Pods"
	// ResourceMetricSourceType is a resource metric known to Kubernetes.
	ResourceMetricSourceType MetricSourceType = "Resource"
	// ExternalMetricSourceType is a global metric that is not associated with any Kubernetes object.
	ExternalMetricSourceType MetricSourceType = "External"
)

// ObjectMetricSource indicates how to scale based on a metric describing a single kubernetes object.
type ObjectMetricSource struct {
	// describedObject specifies the descriptions of a object.
	//
	// +required
	DescribedObject CrossVersionObjectReference `json:"describedObject"`

	// target specifies the target value for the given metric.
	//
	// +required
	Target MetricTarget `json:"target"`

	// metric identifies the target metric by name and selector.
	//
	// +required
	Metric MetricIdentifier `json:"metric"`
}

// PodsMetricSource indicates how to scale based on a metric describing each pod in the current scale target.
type PodsMetricSource struct {
	// metric identifies the target metric by name and selector.
	//
	// +required
	Metric MetricIdentifier `json:"metric"`

	// target specifies the target value for the given metric.
	//
	// +required
	Target MetricTarget `json:"target"`
}

// ResourceMetricSource indicates how to scale based on a resource metric known to Kubernetes.
type ResourceMetricSource struct {
	// name is the name of the resource in question.
	//
	// +required
	Name string `json:"name"`

	// target specifies the target value for the given metric.
	//
	// +required
	Target MetricTarget `json:"target"`
}

// ExternalMetricSource indicates how to scale based on a global metric not associated with any Kubernetes object.
type ExternalMetricSource struct {
	// metric identifies the target metric by name and selector.
	//
	// +required
	Metric MetricIdentifier `json:"metric"`

	// target specifies the target value for the given metric.
	//
	// +required
	Target MetricTarget `json:"target"`
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

// MetricIdentifier defines the name and optionally selector for a metric.
type MetricIdentifier struct {
	// name is the name of the given metric.
	//
	// +required
	Name string `json:"name"`

	// selector is the string-encoded form of a standard kubernetes label selector for the given metric.
	//
	// +optional
	Selector *metav1.LabelSelector `json:"selector,omitempty"`
}

// MetricTarget defines the target value, average value, or average utilization of a specific metric.
type MetricTarget struct {
	// type represents whether the metric type is Utilization, Value, or AverageValue.
	//
	// +required
	// +kubebuilder:validation:Enum=Utilization;Value;AverageValue
	Type MetricTargetType `json:"type"`

	// value is the target value of the metric (as a quantity).
	//
	// +optional
	Value *intstr.IntOrString `json:"value,omitempty"`

	// averageValue is the target value of the average of the metric across all relevant pods.
	//
	// +optional
	AverageValue *intstr.IntOrString `json:"averageValue,omitempty"`

	// averageUtilization is the target value of the average of the resource metric across all relevant pods,
	// represented as a percentage of the requested value of the resource for the pods.
	//
	// +optional
	// +kubebuilder:validation:Minimum=1
	AverageUtilization *int32 `json:"averageUtilization,omitempty"`
}

// MetricTargetType specifies the type of metric being targeted.
type MetricTargetType string

const (
	// UtilizationMetricType declares a MetricTarget is an AverageUtilization value.
	UtilizationMetricType MetricTargetType = "Utilization"
	// ValueMetricType declares a MetricTarget is a raw value.
	ValueMetricType MetricTargetType = "Value"
	// AverageValueMetricType declares a MetricTarget is an AverageValue.
	AverageValueMetricType MetricTargetType = "AverageValue"
)

// ScalingPolicy defines advanced scaling behaviors and policies.
type ScalingPolicy struct {
	// scaleUp defines the scaling up behavior.
	//
	// +optional
	ScaleUp *HPAScalingRules `json:"scaleUp,omitempty"`

	// scaleDown defines the scaling down behavior.
	//
	// +optional
	ScaleDown *HPAScalingRules `json:"scaleDown,omitempty"`

	// stabilizationWindowSeconds is the number of seconds for which past recommendations should be
	// considered while scaling up or scaling down.
	//
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=300
	StabilizationWindowSeconds *int32 `json:"stabilizationWindowSeconds,omitempty"`
}

// HPAScalingRules configures the scaling behavior for one direction.
type HPAScalingRules struct {
	// policies is a list of potential scaling polices which can be used during scaling.
	// At least one policy must be specified, otherwise the HPAScalingRules will be discarded as invalid.
	//
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=100
	// +listType=atomic
	Policies []HPAScalingPolicy `json:"policies"`

	// selectPolicy is used to specify which policy should be used while scaling in a specific direction.
	//
	// +optional
	// +kubebuilder:validation:Enum=Max;Min;Disabled
	SelectPolicy *ScalingPolicySelect `json:"selectPolicy,omitempty"`

	// stabilizationWindowSeconds is the number of seconds for which past recommendations should be
	// considered while scaling up or scaling down.
	//
	// +optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=3600
	StabilizationWindowSeconds *int32 `json:"stabilizationWindowSeconds,omitempty"`
}

// HPAScalingPolicy is a single policy which must hold true for a specified past interval.
type HPAScalingPolicy struct {
	// type is used to specify the scaling policy.
	//
	// +required
	// +kubebuilder:validation:Enum=Pods;Percent
	Type HPAScalingPolicyType `json:"type"`

	// value contains the amount of change which is permitted by the policy.
	//
	// +required
	// +kubebuilder:validation:Minimum=1
	Value int32 `json:"value"`

	// periodSeconds specifies the window of time for which the policy should hold true.
	//
	// +required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=1800
	PeriodSeconds int32 `json:"periodSeconds"`
}

// HPAScalingPolicyType is the type of scaling policy.
type HPAScalingPolicyType string

const (
	// PodsScalingPolicy is a policy used to specify a change in absolute number of pods.
	PodsScalingPolicy HPAScalingPolicyType = "Pods"
	// PercentScalingPolicy is a policy used to specify a relative amount of change with respect to
	// the current number of pods.
	PercentScalingPolicy HPAScalingPolicyType = "Percent"
)

// ScalingPolicySelect is used to specify which policy should be used while scaling in a specific direction.
type ScalingPolicySelect string

const (
	// MaxPolicySelect selects the policy with the highest Value.
	MaxPolicySelect ScalingPolicySelect = "Max"
	// MinPolicySelect selects the policy with the lowest Value.
	MinPolicySelect ScalingPolicySelect = "Min"
	// DisabledPolicySelect disables the scaling direction.
	DisabledPolicySelect ScalingPolicySelect = "Disabled"
)

// ScalingPlacement defines where this scaling policy should be applied across TMC clusters.
type ScalingPlacement struct {
	// clusters specifies the list of clusters where this scaling policy should be applied.
	//
	// +optional
	Clusters []string `json:"clusters,omitempty"`

	// clusterSelector is a label selector that selects clusters where this scaling policy should be applied.
	//
	// +optional
	ClusterSelector *metav1.LabelSelector `json:"clusterSelector,omitempty"`

	// regions specifies the list of regions where this scaling policy should be applied.
	//
	// +optional
	Regions []string `json:"regions,omitempty"`

	// regionSelector is a label selector that selects regions where this scaling policy should be applied.
	//
	// +optional
	RegionSelector *metav1.LabelSelector `json:"regionSelector,omitempty"`
}

// AutoScalingPolicyStatus communicates the observed state of the AutoScalingPolicy.
type AutoScalingPolicyStatus struct {
	// conditions contains the different condition statuses for the AutoScalingPolicy.
	//
	// +optional
	Conditions conditionsv1alpha1.Conditions `json:"conditions,omitempty"`

	// currentReplicas is the current number of replicas of pods managed by this autoscaler.
	//
	// +optional
	CurrentReplicas int32 `json:"currentReplicas,omitempty"`

	// desiredReplicas is the desired number of replicas of pods managed by this autoscaler.
	//
	// +optional
	DesiredReplicas int32 `json:"desiredReplicas,omitempty"`

	// lastScaleTime is the last time the AutoScalingPolicy scaled the number of pods.
	//
	// +optional
	LastScaleTime *metav1.Time `json:"lastScaleTime,omitempty"`

	// currentMetrics is the last read state of the metrics used by this autoscaler.
	//
	// +optional
	CurrentMetrics []MetricStatus `json:"currentMetrics,omitempty"`
}

// MetricStatus describes the last-read state of a single metric.
type MetricStatus struct {
	// type is the type of metric source. It will be one of "Object", "Pods", "Resource", or "External".
	//
	// +required
	Type MetricSourceType `json:"type"`

	// object refers to a metric describing a single kubernetes object.
	//
	// +optional
	Object *ObjectMetricStatus `json:"object,omitempty"`

	// pods refers to a metric describing each pod in the current scale target.
	//
	// +optional
	Pods *PodsMetricStatus `json:"pods,omitempty"`

	// resource refers to a resource metric (such as those specified in requests and limits).
	//
	// +optional
	Resource *ResourceMetricStatus `json:"resource,omitempty"`

	// external refers to a global metric that is not associated with any Kubernetes object.
	//
	// +optional
	External *ExternalMetricStatus `json:"external,omitempty"`
}

// ObjectMetricStatus indicates the current value of a metric describing a single kubernetes object.
type ObjectMetricStatus struct {
	// metric identifies the target metric by name and selector.
	//
	// +required
	Metric MetricIdentifier `json:"metric"`

	// current contains the current value for the given metric.
	//
	// +required
	Current MetricValueStatus `json:"current"`

	// describedObject specifies the descriptions of a object.
	//
	// +required
	DescribedObject CrossVersionObjectReference `json:"describedObject"`
}

// PodsMetricStatus indicates the current value of a metric describing each pod in the current scale target.
type PodsMetricStatus struct {
	// metric identifies the target metric by name and selector.
	//
	// +required
	Metric MetricIdentifier `json:"metric"`

	// current contains the current value for the given metric.
	//
	// +required
	Current MetricValueStatus `json:"current"`
}

// ResourceMetricStatus indicates the current value of a resource metric known to Kubernetes.
type ResourceMetricStatus struct {
	// name is the name of the resource in question.
	//
	// +required
	Name string `json:"name"`

	// current contains the current value for the given metric.
	//
	// +required
	Current MetricValueStatus `json:"current"`
}

// ExternalMetricStatus indicates the current value of a global metric not associated with any Kubernetes object.
type ExternalMetricStatus struct {
	// metric identifies the target metric by name and selector.
	//
	// +required
	Metric MetricIdentifier `json:"metric"`

	// current contains the current value for the given metric.
	//
	// +required
	Current MetricValueStatus `json:"current"`
}

// MetricValueStatus holds the current value for a metric.
type MetricValueStatus struct {
	// value is the current value of the metric (as a quantity).
	//
	// +optional
	Value *intstr.IntOrString `json:"value,omitempty"`

	// averageValue is the current value of the average of the metric across all relevant pods.
	//
	// +optional
	AverageValue *intstr.IntOrString `json:"averageValue,omitempty"`

	// averageUtilization is the current value of the average of the resource metric across all relevant pods,
	// represented as a percentage of the requested value of the resource for the pods.
	//
	// +optional
	AverageUtilization *int32 `json:"averageUtilization,omitempty"`
}

// These are valid condition types for AutoScalingPolicy.
const (
	// AutoScalingPolicyReady indicates that the scaling policy is ready to be applied.
	AutoScalingPolicyReady conditionsv1alpha1.ConditionType = "Ready"

	// AutoScalingPolicyActive indicates that the scaling policy is actively being applied.
	AutoScalingPolicyActive conditionsv1alpha1.ConditionType = "Active"

	// AutoScalingPolicyProgressing indicates that the scaling policy is in progress.
	AutoScalingPolicyProgressing conditionsv1alpha1.ConditionType = "Progressing"

	// AutoScalingPolicyScalingActive indicates that the policy is actively scaling resources.
	AutoScalingPolicyScalingActive conditionsv1alpha1.ConditionType = "ScalingActive"

	// AutoScalingPolicyScalingLimited indicates that the policy scaling is limited by constraints.
	AutoScalingPolicyScalingLimited conditionsv1alpha1.ConditionType = "ScalingLimited"
)

// AutoScalingPolicyList contains a list of AutoScalingPolicy.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type AutoScalingPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AutoScalingPolicy `json:"items"`
}

func (in *AutoScalingPolicy) SetConditions(c conditionsv1alpha1.Conditions) {
	in.Status.Conditions = c
}

func (in *AutoScalingPolicy) GetConditions() conditionsv1alpha1.Conditions {
	return in.Status.Conditions
}