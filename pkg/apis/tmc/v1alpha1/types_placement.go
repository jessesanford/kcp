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

// WorkloadPlacement represents a policy for placing workloads across clusters.
//
// +crd
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
type WorkloadPlacement struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec WorkloadPlacementSpec `json:"spec,omitempty"`

	// +optional
	Status WorkloadPlacementStatus `json:"status,omitempty"`
}

// WorkloadPlacementSpec holds the desired state of the WorkloadPlacement.
type WorkloadPlacementSpec struct {
	// WorkloadSelector selects the workloads this placement applies to
	// +kubebuilder:validation:Required
	WorkloadSelector WorkloadSelector `json:"workloadSelector"`

	// ClusterSelector defines how to select target clusters
	// +kubebuilder:validation:Required
	ClusterSelector ClusterSelector `json:"clusterSelector"`

	// PlacementPolicy defines the placement strategy
	// +kubebuilder:default="RoundRobin"
	// +optional
	PlacementPolicy PlacementPolicy `json:"placementPolicy,omitempty"`

	// NumberOfClusters specifies how many clusters to place workloads on
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	// +optional
	NumberOfClusters *int32 `json:"numberOfClusters,omitempty"`
}

// WorkloadPlacementStatus communicates the observed state of the WorkloadPlacement.
type WorkloadPlacementStatus struct {
	// Conditions represent the latest available observations of the placement's state
	// +optional
	Conditions conditionsv1alpha1.Conditions `json:"conditions,omitempty"`

	// SelectedClusters lists the clusters selected for workload placement
	// +optional
	SelectedClusters []string `json:"selectedClusters,omitempty"`

	// PlacedWorkloads tracks the workloads that have been placed
	// +optional
	PlacedWorkloads []PlacedWorkload `json:"placedWorkloads,omitempty"`

	// LastPlacementTime is the timestamp of the last placement decision
	// +optional
	LastPlacementTime *metav1.Time `json:"lastPlacementTime,omitempty"`

	// PlacementDecisions tracks the current placement decisions
	// +optional
	PlacementDecisions []PlacementDecision `json:"placementDecisions,omitempty"`
}

// PlacedWorkload represents a workload that has been placed on a cluster
type PlacedWorkload struct {
	// WorkloadRef references the placed workload
	// +kubebuilder:validation:Required
	WorkloadRef WorkloadReference `json:"workloadRef"`

	// ClusterName is the name of the cluster where the workload was placed
	// +kubebuilder:validation:Required
	ClusterName string `json:"clusterName"`

	// PlacementTime is when the workload was placed
	// +kubebuilder:validation:Required
	PlacementTime metav1.Time `json:"placementTime"`

	// Status indicates the current status of the placed workload
	// +kubebuilder:default="Pending"
	// +optional
	Status PlacedWorkloadStatus `json:"status,omitempty"`
}

// PlacementDecision represents a placement decision made by the controller
type PlacementDecision struct {
	// ClusterName is the name of the selected cluster
	// +kubebuilder:validation:Required
	ClusterName string `json:"clusterName"`

	// Reason provides the rationale for this placement decision
	// +optional
	Reason string `json:"reason,omitempty"`

	// Score represents the placement score for this cluster (higher is better)
	// +optional
	Score *int32 `json:"score,omitempty"`

	// DecisionTime is when this placement decision was made
	// +kubebuilder:validation:Required
	DecisionTime metav1.Time `json:"decisionTime"`
}

// WorkloadPlacementList is a list of WorkloadPlacement resources
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type WorkloadPlacementList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []WorkloadPlacement `json:"items"`
}