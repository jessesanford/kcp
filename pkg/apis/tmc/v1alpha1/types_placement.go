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
)

// WorkloadPlacement defines how and where workloads should be placed across clusters.
// It provides policies and strategies for transparent multi-cluster placement.
//
// +crd
// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories=kcp
// +kubebuilder:printcolumn:name="Strategy",type=string,JSONPath=`.spec.strategy`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type WorkloadPlacement struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the WorkloadPlacement.
	Spec WorkloadPlacementSpec `json:"spec,omitempty"`

	// Status defines the observed state of the WorkloadPlacement.
	// +optional
	Status WorkloadPlacementStatus `json:"status,omitempty"`
}

// WorkloadPlacementSpec defines the desired state of WorkloadPlacement
type WorkloadPlacementSpec struct {
	// WorkloadSelector defines which workloads this placement applies to.
	WorkloadSelector WorkloadSelector `json:"workloadSelector"`

	// Strategy defines the placement strategy to use.
	Strategy PlacementStrategy `json:"strategy"`

	// LocationSelector selects clusters based on location labels.
	// If specified, only clusters matching this selector will be considered.
	// +optional
	LocationSelector *metav1.LabelSelector `json:"locationSelector,omitempty"`

	// CapabilityRequirements defines the minimum capabilities required
	// from clusters for this placement.
	// +optional
	CapabilityRequirements []CapabilityRequirement `json:"capabilityRequirements,omitempty"`

	// MaxClusters defines the maximum number of clusters to place workloads on.
	// If 0 or unset, no limit is applied.
	// +optional
	MaxClusters int32 `json:"maxClusters,omitempty"`

	// PreferredClusters is a list of cluster names that are preferred for placement.
	// These clusters will be given higher priority during placement decisions.
	// +optional
	PreferredClusters []string `json:"preferredClusters,omitempty"`
}

// WorkloadPlacementStatus defines the observed state of WorkloadPlacement
type WorkloadPlacementStatus struct {
	// Phase represents the current phase of the placement.
	// +optional
	Phase PlacementPhase `json:"phase,omitempty"`

	// Conditions represent the latest available observations of the
	// placement's current state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// SelectedClusters contains the list of clusters selected for placement.
	// +optional
	SelectedClusters []SelectedCluster `json:"selectedClusters,omitempty"`

	// PlacedWorkloads tracks workloads that have been placed by this placement.
	// +optional
	PlacedWorkloads []PlacedWorkload `json:"placedWorkloads,omitempty"`

	// LastPlacementTime is the last time a placement decision was made.
	// +optional
	LastPlacementTime *metav1.Time `json:"lastPlacementTime,omitempty"`
}

// WorkloadSelector defines criteria for selecting workloads
type WorkloadSelector struct {
	// LabelSelector selects workloads based on their labels.
	// +optional
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`

	// WorkloadTypes restricts the selector to specific workload types.
	// If empty, all workload types are selected.
	// +optional
	WorkloadTypes []WorkloadType `json:"workloadTypes,omitempty"`

	// NamespaceSelector selects workloads from namespaces matching this selector.
	// +optional
	NamespaceSelector *metav1.LabelSelector `json:"namespaceSelector,omitempty"`
}

// PlacementStrategy defines how workloads should be placed
type PlacementStrategy string

const (
	// PlacementStrategyRoundRobin places workloads using round-robin across available clusters.
	PlacementStrategyRoundRobin PlacementStrategy = "RoundRobin"
	// PlacementStrategySpread spreads workloads evenly across all selected clusters.
	PlacementStrategySpread PlacementStrategy = "Spread"
	// PlacementStrategyAffinity uses cluster affinity rules for placement.
	PlacementStrategyAffinity PlacementStrategy = "Affinity"
	// PlacementStrategyCapacity places workloads based on cluster capacity.
	PlacementStrategyCapacity PlacementStrategy = "Capacity"
)

// CapabilityRequirement defines a required capability for placement
type CapabilityRequirement struct {
	// Type is the type of capability required.
	Type CapabilityType `json:"type"`

	// Required indicates if this capability is mandatory.
	Required bool `json:"required"`

	// Values contains specific values required for this capability.
	// +optional
	Values []string `json:"values,omitempty"`
}

// CapabilityType represents different types of cluster capabilities
type CapabilityType string

const (
	// CapabilityTypeLoadBalancer requires LoadBalancer support.
	CapabilityTypeLoadBalancer CapabilityType = "LoadBalancer"
	// CapabilityTypePersistentStorage requires persistent storage support.
	CapabilityTypePersistentStorage CapabilityType = "PersistentStorage"
	// CapabilityTypeStorageClass requires specific storage classes.
	CapabilityTypeStorageClass CapabilityType = "StorageClass"
	// CapabilityTypeNetworkPolicies requires NetworkPolicy support.
	CapabilityTypeNetworkPolicies CapabilityType = "NetworkPolicies"
)

// SelectedCluster represents a cluster selected for placement
type SelectedCluster struct {
	// Name is the name of the selected cluster.
	Name string `json:"name"`

	// Weight represents the relative weight of this cluster for placement.
	// Higher weights indicate higher preference.
	// +optional
	Weight int32 `json:"weight,omitempty"`

	// Reason provides a human-readable reason why this cluster was selected.
	// +optional
	Reason string `json:"reason,omitempty"`
}

// PlacedWorkload tracks a workload that has been placed
type PlacedWorkload struct {
	// WorkloadRef references the placed workload.
	WorkloadRef ObjectReference `json:"workloadRef"`

	// ClusterName is the name of the cluster where the workload was placed.
	ClusterName string `json:"clusterName"`

	// PlacementTime is when the workload was placed.
	PlacementTime metav1.Time `json:"placementTime"`

	// Status represents the current status of the placed workload.
	// +optional
	Status PlacedWorkloadStatus `json:"status,omitempty"`
}

// ObjectReference contains enough information to let you inspect or modify the referred object.
type ObjectReference struct {
	// API version of the referent.
	APIVersion string `json:"apiVersion"`
	// Kind of the referent.
	Kind string `json:"kind"`
	// Name of the referent.
	Name string `json:"name"`
	// Namespace of the referent.
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// PlacedWorkloadStatus represents the status of a placed workload
type PlacedWorkloadStatus string

const (
	// PlacedWorkloadStatusPending indicates the workload placement is pending.
	PlacedWorkloadStatusPending PlacedWorkloadStatus = "Pending"
	// PlacedWorkloadStatusActive indicates the workload is actively placed and running.
	PlacedWorkloadStatusActive PlacedWorkloadStatus = "Active"
	// PlacedWorkloadStatusFailed indicates the workload placement has failed.
	PlacedWorkloadStatusFailed PlacedWorkloadStatus = "Failed"
)

// PlacementPhase represents the phase of workload placement
type PlacementPhase string

const (
	// PlacementPhasePending indicates the placement is being evaluated.
	PlacementPhasePending PlacementPhase = "Pending"
	// PlacementPhaseReady indicates the placement is ready and active.
	PlacementPhaseReady PlacementPhase = "Ready"
	// PlacementPhaseFailed indicates the placement has failed.
	PlacementPhaseFailed PlacementPhase = "Failed"
)

// Condition types for WorkloadPlacement
const (
	// WorkloadPlacementReady indicates that the placement is ready.
	WorkloadPlacementReady = "Ready"
	// WorkloadPlacementClustersSelected indicates that clusters have been selected.
	WorkloadPlacementClustersSelected = "ClustersSelected"
	// WorkloadPlacementWorkloadsPlaced indicates that workloads have been successfully placed.
	WorkloadPlacementWorkloadsPlaced = "WorkloadsPlaced"
)

// WorkloadPlacementList contains a list of WorkloadPlacement
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type WorkloadPlacementList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WorkloadPlacement `json:"items"`
}