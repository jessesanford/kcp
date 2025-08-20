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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClusterSchedulingProfile defines scheduling characteristics and constraints for a cluster
// +k8s:genclient
// +k8s:genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
type ClusterSchedulingProfile struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired scheduling profile for the cluster
	Spec ClusterSchedulingProfileSpec `json:"spec,omitempty"`

	// Status represents the current observed state of the cluster
	Status ClusterSchedulingProfileStatus `json:"status,omitempty"`
}

// ClusterSchedulingProfileSpec defines the desired scheduling characteristics
type ClusterSchedulingProfileSpec struct {
	// ClusterName is the name of the physical cluster this profile represents
	ClusterName string `json:"clusterName"`

	// ResourceCapacity defines the total resource capacity of the cluster
	ResourceCapacity corev1.ResourceList `json:"resourceCapacity,omitempty"`

	// AvailableResources defines currently available resources
	// +optional
	AvailableResources corev1.ResourceList `json:"availableResources,omitempty"`

	// NodeSelector defines constraints for workload placement
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Tolerations defines tolerations the cluster can support
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// SupportedAPIVersions lists the API versions this cluster supports
	// +optional
	SupportedAPIVersions []string `json:"supportedAPIVersions,omitempty"`

	// Location provides geographical or topology information
	// +optional
	Location *ClusterLocation `json:"location,omitempty"`

	// Weight defines the preference weight for scheduling to this cluster
	// Higher values indicate higher preference
	// +optional
	Weight *int32 `json:"weight,omitempty"`
}

// ClusterLocation represents geographical or topology information
type ClusterLocation struct {
	// Region represents the geographical region
	// +optional
	Region string `json:"region,omitempty"`

	// Zone represents the availability zone within a region  
	// +optional
	Zone string `json:"zone,omitempty"`

	// Labels provide additional location metadata
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
}

// ClusterSchedulingProfileStatus defines the observed state
type ClusterSchedulingProfileStatus struct {
	// Conditions represents the latest available observations
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// LastUpdateTime when the profile was last updated
	// +optional
	LastUpdateTime *metav1.Time `json:"lastUpdateTime,omitempty"`

	// Allocatable represents the resources available for scheduling
	// +optional
	Allocatable corev1.ResourceList `json:"allocatable,omitempty"`

	// NodeCount represents the current number of nodes
	// +optional
	NodeCount *int32 `json:"nodeCount,omitempty"`

	// ReadyNodes represents the number of ready nodes
	// +optional
	ReadyNodes *int32 `json:"readyNodes,omitempty"`
}

// ClusterSchedulingProfileList contains a list of ClusterSchedulingProfile
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ClusterSchedulingProfileList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterSchedulingProfile `json:"items"`
}

// SchedulingDecision records placement decisions made by the scheduler
// +k8s:genclient
// +k8s:genclient:nonNamespaced  
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
type SchedulingDecision struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the scheduling decision details
	Spec SchedulingDecisionSpec `json:"spec,omitempty"`

	// Status represents the current state of the scheduling decision
	Status SchedulingDecisionStatus `json:"status,omitempty"`
}

// SchedulingDecisionSpec defines the details of a scheduling decision
type SchedulingDecisionSpec struct {
	// WorkloadReference identifies the workload being scheduled
	WorkloadReference WorkloadReference `json:"workloadReference"`

	// SelectedClusters lists the clusters selected for placement
	SelectedClusters []ClusterSelection `json:"selectedClusters"`

	// SchedulingPolicy defines the policy used for this decision
	SchedulingPolicy SchedulingPolicy `json:"schedulingPolicy,omitempty"`

	// Timestamp when the decision was made
	Timestamp metav1.Time `json:"timestamp"`
}

// WorkloadReference identifies a workload resource
type WorkloadReference struct {
	// APIVersion of the workload
	APIVersion string `json:"apiVersion"`

	// Kind of the workload  
	Kind string `json:"kind"`

	// Namespace of the workload (if namespaced)
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Name of the workload
	Name string `json:"name"`

	// UID of the workload resource
	// +optional
	UID string `json:"uid,omitempty"`
}

// ClusterSelection represents a cluster selected for workload placement
type ClusterSelection struct {
	// ClusterName is the name of the selected cluster
	ClusterName string `json:"clusterName"`

	// Reason explains why this cluster was selected
	// +optional
	Reason string `json:"reason,omitempty"`

	// Score indicates the scheduling score for this cluster
	// +optional
	Score *float64 `json:"score,omitempty"`

	// ResourceRequirements defines the resources allocated to this cluster
	// +optional
	ResourceRequirements *ResourceRequirements `json:"resourceRequirements,omitempty"`
}

// ResourceRequirements defines resource allocation details
type ResourceRequirements struct {
	// Requests defines the minimum required resources
	// +optional
	Requests corev1.ResourceList `json:"requests,omitempty"`

	// Limits defines the maximum allowed resources
	// +optional
	Limits corev1.ResourceList `json:"limits,omitempty"`
}

// SchedulingPolicy defines the policy used for scheduling
type SchedulingPolicy struct {
	// PolicyType defines the type of scheduling policy used
	PolicyType SchedulingPolicyType `json:"policyType"`

	// Parameters provides additional policy configuration
	// +optional
	Parameters map[string]string `json:"parameters,omitempty"`
}

// SchedulingPolicyType defines types of scheduling policies
// +kubebuilder:validation:Enum=BestFit;RoundRobin;WeightedRoundRobin;ResourceAware;AffinityBased
type SchedulingPolicyType string

const (
	// BestFitSchedulingPolicy selects the best fitting cluster
	BestFitSchedulingPolicy SchedulingPolicyType = "BestFit"
	// RoundRobinSchedulingPolicy distributes workloads evenly
	RoundRobinSchedulingPolicy SchedulingPolicyType = "RoundRobin"
	// WeightedRoundRobinSchedulingPolicy considers cluster weights
	WeightedRoundRobinSchedulingPolicy SchedulingPolicyType = "WeightedRoundRobin"
	// ResourceAwareSchedulingPolicy considers available resources
	ResourceAwareSchedulingPolicy SchedulingPolicyType = "ResourceAware"
	// AffinityBasedSchedulingPolicy uses affinity and anti-affinity rules
	AffinityBasedSchedulingPolicy SchedulingPolicyType = "AffinityBased"
)

// SchedulingDecisionStatus defines the observed state
type SchedulingDecisionStatus struct {
	// Conditions represents the latest available observations
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Phase represents the current phase of the scheduling decision
	Phase SchedulingDecisionPhase `json:"phase,omitempty"`

	// PlacementResults shows the results of workload placement
	PlacementResults []PlacementResult `json:"placementResults,omitempty"`

	// LastUpdateTime when the decision status was last updated
	// +optional
	LastUpdateTime *metav1.Time `json:"lastUpdateTime,omitempty"`
}

// SchedulingDecisionPhase represents the phase of a scheduling decision
// +kubebuilder:validation:Enum=Pending;Scheduled;Failed;Complete
type SchedulingDecisionPhase string

const (
	// PendingSchedulingDecisionPhase indicates the decision is being processed
	PendingSchedulingDecisionPhase SchedulingDecisionPhase = "Pending"
	// ScheduledSchedulingDecisionPhase indicates clusters have been selected
	ScheduledSchedulingDecisionPhase SchedulingDecisionPhase = "Scheduled"
	// FailedSchedulingDecisionPhase indicates scheduling failed
	FailedSchedulingDecisionPhase SchedulingDecisionPhase = "Failed"
	// CompleteSchedulingDecisionPhase indicates workloads are placed
	CompleteSchedulingDecisionPhase SchedulingDecisionPhase = "Complete"
)

// PlacementResult represents the result of placing workload on a cluster
type PlacementResult struct {
	// ClusterName where the workload was placed
	ClusterName string `json:"clusterName"`

	// Status of the placement
	Status PlacementStatus `json:"status"`

	// Message provides additional details about the placement
	// +optional
	Message string `json:"message,omitempty"`

	// ResourcesAllocated shows the actual resources allocated
	// +optional
	ResourcesAllocated corev1.ResourceList `json:"resourcesAllocated,omitempty"`

	// PlacementTime when the workload was placed
	// +optional
	PlacementTime *metav1.Time `json:"placementTime,omitempty"`
}

// PlacementStatus represents the status of workload placement
// +kubebuilder:validation:Enum=Pending;Placed;Failed;Removing
type PlacementStatus string

const (
	// PendingPlacementStatus indicates placement is pending
	PendingPlacementStatus PlacementStatus = "Pending"
	// PlacedPlacementStatus indicates workload is successfully placed
	PlacedPlacementStatus PlacementStatus = "Placed"
	// FailedPlacementStatus indicates placement failed
	FailedPlacementStatus PlacementStatus = "Failed"
	// RemovingPlacementStatus indicates workload is being removed
	RemovingPlacementStatus PlacementStatus = "Removing"
)

// SchedulingDecisionList contains a list of SchedulingDecision
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type SchedulingDecisionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SchedulingDecision `json:"items"`
}