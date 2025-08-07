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
// +kubebuilder:printcolumn:name="Strategy",type=string,JSONPath=`.spec.strategy`
// +kubebuilder:printcolumn:name="Clusters",type=integer,JSONPath=`.spec.numberOfClusters`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
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

	// Strategy defines the placement strategy
	// +kubebuilder:default="RoundRobin"
	// +optional
	Strategy PlacementStrategy `json:"strategy,omitempty"`

	// NumberOfClusters specifies how many clusters to place workloads on
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	// +optional
	NumberOfClusters *int32 `json:"numberOfClusters,omitempty"`

	// LocationSelector selects clusters from specific locations
	// +optional
	LocationSelector *metav1.LabelSelector `json:"locationSelector,omitempty"`

	// CapabilityRequirements specifies required cluster capabilities
	// +optional
	CapabilityRequirements []CapabilityRequirement `json:"capabilityRequirements,omitempty"`
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

// WorkloadPlacementList is a list of WorkloadPlacement resources
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type WorkloadPlacementList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []WorkloadPlacement `json:"items"`
}

const (
	// WorkloadPlacementReady indicates the placement policy is ready and active
	WorkloadPlacementReady = "Ready"
)