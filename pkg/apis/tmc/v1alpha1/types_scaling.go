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

	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
)

// AutoScalingPolicy defines the scaling configuration for workloads in the TMC system.
// It provides basic horizontal pod autoscaling capabilities for multi-cluster deployments.
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
	// This includes replica scaling based on CPU utilization.
	//
	// +optional
	HorizontalPodAutoScaler *HorizontalPodAutoScalerSpec `json:"horizontalPodAutoScaler,omitempty"`

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
	// +kubebuilder:default=70
	TargetCPUUtilizationPercentage *int32 `json:"targetCPUUtilizationPercentage,omitempty"`
}

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

	// currentCPUUtilizationPercentage is the current average CPU utilization
	// over all the pods, represented as a percentage of requested CPU.
	//
	// +optional
	CurrentCPUUtilizationPercentage *int32 `json:"currentCPUUtilizationPercentage,omitempty"`
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