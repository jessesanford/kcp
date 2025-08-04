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

// WorkloadPlacementAdvanced represents advanced placement policies with affinity, rollouts, and traffic splitting.
//
// +crd
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
type WorkloadPlacementAdvanced struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec WorkloadPlacementAdvancedSpec `json:"spec,omitempty"`

	// +optional
	Status WorkloadPlacementAdvancedStatus `json:"status,omitempty"`
}

// WorkloadPlacementAdvancedSpec holds the desired state of the WorkloadPlacementAdvanced.
type WorkloadPlacementAdvancedSpec struct {
	// WorkloadSelector selects the workloads this placement applies to
	WorkloadSelector WorkloadSelector `json:"workloadSelector"`

	// ClusterSelector defines how to select target clusters
	ClusterSelector ClusterSelector `json:"clusterSelector"`

	// PlacementPolicy defines the basic placement strategy
	// +kubebuilder:default="RoundRobin"
	// +optional
	PlacementPolicy PlacementPolicy `json:"placementPolicy,omitempty"`

	// AffinityRules defines advanced affinity and anti-affinity rules
	// +optional
	AffinityRules *AffinityRules `json:"affinityRules,omitempty"`

	// RolloutStrategy defines how workload updates are rolled out across clusters
	// +optional
	RolloutStrategy *RolloutStrategy `json:"rolloutStrategy,omitempty"`

	// TrafficSplitting defines traffic distribution across clusters
	// +optional
	TrafficSplitting *TrafficSplitting `json:"trafficSplitting,omitempty"`
}

// AffinityRules defines placement affinity and anti-affinity rules
type AffinityRules struct {
	// ClusterAffinity defines which clusters workloads should prefer
	// +optional
	ClusterAffinity *ClusterAffinity `json:"clusterAffinity,omitempty"`

	// ClusterAntiAffinity defines which clusters workloads should avoid
	// +optional
	ClusterAntiAffinity *ClusterAntiAffinity `json:"clusterAntiAffinity,omitempty"`
}

// ClusterAffinity defines cluster affinity preferences
type ClusterAffinity struct {
	// RequiredDuringSchedulingIgnoredDuringExecution hard requirements
	// +optional
	RequiredDuringSchedulingIgnoredDuringExecution []ClusterAffinityTerm `json:"requiredDuringSchedulingIgnoredDuringExecution,omitempty"`

	// PreferredDuringSchedulingIgnoredDuringExecution soft preferences
	// +optional
	PreferredDuringSchedulingIgnoredDuringExecution []WeightedClusterAffinityTerm `json:"preferredDuringSchedulingIgnoredDuringExecution,omitempty"`
}

// ClusterAntiAffinity defines cluster anti-affinity requirements
type ClusterAntiAffinity struct {
	// RequiredDuringSchedulingIgnoredDuringExecution hard requirements
	// +optional
	RequiredDuringSchedulingIgnoredDuringExecution []ClusterAffinityTerm `json:"requiredDuringSchedulingIgnoredDuringExecution,omitempty"`

	// PreferredDuringSchedulingIgnoredDuringExecution soft preferences
	// +optional
	PreferredDuringSchedulingIgnoredDuringExecution []WeightedClusterAffinityTerm `json:"preferredDuringSchedulingIgnoredDuringExecution,omitempty"`
}

// ClusterAffinityTerm defines a single cluster affinity term
type ClusterAffinityTerm struct {
	// LabelSelector selects clusters based on labels
	// +optional
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`

	// LocationSelector selects clusters from specific locations
	// +optional
	LocationSelector []string `json:"locationSelector,omitempty"`
}

// WeightedClusterAffinityTerm adds weight to cluster affinity terms
type WeightedClusterAffinityTerm struct {
	// Weight associated with matching the corresponding clusterAffinityTerm
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	Weight int32 `json:"weight"`

	// ClusterAffinityTerm defines the cluster affinity term
	ClusterAffinityTerm ClusterAffinityTerm `json:"clusterAffinityTerm"`
}

// RolloutStrategy defines the rollout strategy for workload updates
type RolloutStrategy struct {
	// Type defines the rollout strategy type
	// +kubebuilder:default="RollingUpdate"
	// +optional
	Type RolloutStrategyType `json:"type,omitempty"`

	// RollingUpdate defines rolling update parameters
	// +optional
	RollingUpdate *RollingUpdateStrategy `json:"rollingUpdate,omitempty"`

	// BlueGreen defines blue-green deployment parameters
	// +optional
	BlueGreen *BlueGreenStrategy `json:"blueGreen,omitempty"`

	// Canary defines canary deployment parameters
	// +optional
	Canary *CanaryStrategy `json:"canary,omitempty"`
}

// RolloutStrategyType defines the type of rollout strategy
// +kubebuilder:validation:Enum=RollingUpdate;BlueGreen;Canary;Recreate
type RolloutStrategyType string

const (
	// RolloutStrategyTypeRollingUpdate performs rolling updates
	RolloutStrategyTypeRollingUpdate RolloutStrategyType = "RollingUpdate"

	// RolloutStrategyTypeBlueGreen performs blue-green deployments
	RolloutStrategyTypeBlueGreen RolloutStrategyType = "BlueGreen"

	// RolloutStrategyTypeCanary performs canary deployments
	RolloutStrategyTypeCanary RolloutStrategyType = "Canary"

	// RolloutStrategyTypeRecreate performs recreate deployments
	RolloutStrategyTypeRecreate RolloutStrategyType = "Recreate"
)

// RollingUpdateStrategy defines rolling update parameters
type RollingUpdateStrategy struct {
	// MaxUnavailable is the maximum number of clusters that can be unavailable during the update
	// +kubebuilder:default="25%"
	// +optional
	MaxUnavailable *intstr.IntOrString `json:"maxUnavailable,omitempty"`

	// MaxSurge is the maximum number of clusters that can be created above the desired number
	// +kubebuilder:default="25%"
	// +optional
	MaxSurge *intstr.IntOrString `json:"maxSurge,omitempty"`
}

// BlueGreenStrategy defines blue-green deployment parameters
type BlueGreenStrategy struct {
	// AutoPromotionEnabled enables automatic promotion
	// +kubebuilder:default=false
	// +optional
	AutoPromotionEnabled bool `json:"autoPromotionEnabled,omitempty"`
}

// CanaryStrategy defines canary deployment parameters
type CanaryStrategy struct {
	// Steps defines the canary deployment steps as weight percentages
	// +optional
	Steps []int32 `json:"steps,omitempty"`
}

// TrafficSplitting defines traffic distribution across clusters
type TrafficSplitting struct {
	// ClusterWeights defines traffic weight distribution across clusters
	ClusterWeights []ClusterWeight `json:"clusterWeights"`
}

// ClusterWeight defines traffic weight for a cluster
type ClusterWeight struct {
	// ClusterName is the name of the cluster
	ClusterName string `json:"clusterName"`

	// Weight is the traffic weight (0-100)
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	Weight int32 `json:"weight"`
}

// WorkloadPlacementAdvancedStatus communicates the observed state of the WorkloadPlacementAdvanced.
type WorkloadPlacementAdvancedStatus struct {
	// Conditions represent the latest available observations of the placement's state
	// +optional
	Conditions conditionsv1alpha1.Conditions `json:"conditions,omitempty"`

	// SelectedClusters lists the clusters selected for workload placement
	// +optional
	SelectedClusters []string `json:"selectedClusters,omitempty"`

	// PlacedWorkloads tracks the workloads that have been placed
	// +optional
	PlacedWorkloads []PlacedWorkloadAdvanced `json:"placedWorkloads,omitempty"`

	// RolloutState tracks the current rollout state
	// +optional
	RolloutState *RolloutState `json:"rolloutState,omitempty"`

	// TrafficState tracks the current traffic splitting state
	// +optional
	TrafficState *TrafficState `json:"trafficState,omitempty"`

	// LastPlacementTime is the timestamp of the last placement decision
	// +optional
	LastPlacementTime *metav1.Time `json:"lastPlacementTime,omitempty"`
}

// PlacedWorkloadAdvanced represents a workload placed with advanced features
type PlacedWorkloadAdvanced struct {
	// WorkloadRef references the placed workload
	WorkloadRef WorkloadReference `json:"workloadRef"`

	// ClusterName is the name of the cluster where the workload was placed
	ClusterName string `json:"clusterName"`

	// PlacementTime is when the workload was placed
	PlacementTime metav1.Time `json:"placementTime"`

	// Status indicates the current status of the placed workload
	// +kubebuilder:default="Pending"
	// +optional
	Status PlacedWorkloadStatus `json:"status,omitempty"`

	// TrafficWeight is the traffic weight for this placement
	// +optional
	TrafficWeight *int32 `json:"trafficWeight,omitempty"`
}

// RolloutState tracks the current rollout state
type RolloutState struct {
	// Phase indicates the current rollout phase
	// +kubebuilder:default="Pending"
	// +optional
	Phase RolloutPhase `json:"phase,omitempty"`

	// CurrentStep is the current step in the rollout
	// +optional
	CurrentStep *int32 `json:"currentStep,omitempty"`

	// TotalSteps is the total number of steps in the rollout
	// +optional
	TotalSteps *int32 `json:"totalSteps,omitempty"`

	// Message provides additional information about the rollout state
	// +optional
	Message string `json:"message,omitempty"`
}

// RolloutPhase represents the current phase of a rollout
// +kubebuilder:validation:Enum=Pending;InProgress;Paused;Completed;Failed;Aborted
type RolloutPhase string

const (
	// RolloutPhasePending indicates rollout is pending
	RolloutPhasePending RolloutPhase = "Pending"

	// RolloutPhaseInProgress indicates rollout is in progress
	RolloutPhaseInProgress RolloutPhase = "InProgress"

	// RolloutPhasePaused indicates rollout is paused
	RolloutPhasePaused RolloutPhase = "Paused"

	// RolloutPhaseCompleted indicates rollout is completed
	RolloutPhaseCompleted RolloutPhase = "Completed"

	// RolloutPhaseFailed indicates rollout failed
	RolloutPhaseFailed RolloutPhase = "Failed"

	// RolloutPhaseAborted indicates rollout was aborted
	RolloutPhaseAborted RolloutPhase = "Aborted"
)

// TrafficState tracks the current traffic splitting state
type TrafficState struct {
	// ActiveWeights shows the current traffic weight distribution
	ActiveWeights []ClusterWeight `json:"activeWeights"`

	// TargetWeights shows the target traffic weight distribution
	// +optional
	TargetWeights []ClusterWeight `json:"targetWeights,omitempty"`

	// LastUpdateTime is when traffic weights were last updated
	// +optional
	LastUpdateTime *metav1.Time `json:"lastUpdateTime,omitempty"`
}

// WorkloadPlacementAdvancedList is a list of WorkloadPlacementAdvanced resources
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type WorkloadPlacementAdvancedList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []WorkloadPlacementAdvanced `json:"items"`
}