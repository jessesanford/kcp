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
	WorkloadSelector WorkloadSelector `json:"workloadSelector"`

	// ClusterSelector defines how to select target clusters
	ClusterSelector ClusterSelector `json:"clusterSelector"`

	// PlacementPolicy defines the placement strategy
	// +kubebuilder:default="RoundRobin"
	// +optional
	PlacementPolicy PlacementPolicy `json:"placementPolicy,omitempty"`
}

// WorkloadSelector defines how to select workloads for placement
type WorkloadSelector struct {
	// LabelSelector selects workloads based on labels
	// +optional
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`

	// WorkloadTypes specifies the types of workloads to select
	// +optional
	WorkloadTypes []WorkloadType `json:"workloadTypes,omitempty"`

	// NamespaceSelector selects workloads from specific namespaces
	// +optional
	NamespaceSelector *metav1.LabelSelector `json:"namespaceSelector,omitempty"`
}

// WorkloadType represents a Kubernetes workload type
type WorkloadType struct {
	// APIVersion is the API version of the workload
	// +kubebuilder:validation:Required
	APIVersion string `json:"apiVersion"`

	// Kind is the kind of the workload
	// +kubebuilder:validation:Required
	Kind string `json:"kind"`
}

// ClusterSelector defines how to select target clusters
type ClusterSelector struct {
	// LabelSelector selects clusters based on labels
	// +optional
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`

	// LocationSelector selects clusters from specific locations
	// +optional
	LocationSelector []string `json:"locationSelector,omitempty"`

	// ClusterNames explicitly lists cluster names to target
	// +optional
	ClusterNames []string `json:"clusterNames,omitempty"`
}

// PlacementPolicy defines the placement strategy
// +kubebuilder:validation:Enum=RoundRobin;LeastLoaded;Random;LocationAware
type PlacementPolicy string

const (
	// PlacementPolicyRoundRobin distributes workloads evenly across clusters
	PlacementPolicyRoundRobin PlacementPolicy = "RoundRobin"
	
	// PlacementPolicyLeastLoaded places workloads on the least loaded cluster
	PlacementPolicyLeastLoaded PlacementPolicy = "LeastLoaded"
	
	// PlacementPolicyRandom randomly selects target clusters
	PlacementPolicyRandom PlacementPolicy = "Random"
	
	// PlacementPolicyLocationAware considers cluster location for placement
	PlacementPolicyLocationAware PlacementPolicy = "LocationAware"
)

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
}

// PlacedWorkload represents a workload that has been placed on a cluster
type PlacedWorkload struct {
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
}

// WorkloadReference references a Kubernetes workload
type WorkloadReference struct {
	// APIVersion is the API version of the workload
	APIVersion string `json:"apiVersion"`

	// Kind is the kind of the workload
	Kind string `json:"kind"`

	// Name is the name of the workload
	Name string `json:"name"`

	// Namespace is the namespace of the workload
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// PlacedWorkloadStatus represents the status of a placed workload
// +kubebuilder:validation:Enum=Pending;Placed;Failed;Removed
type PlacedWorkloadStatus string

const (
	// PlacedWorkloadStatusPending indicates the workload is waiting to be placed
	PlacedWorkloadStatusPending PlacedWorkloadStatus = "Pending"
	
	// PlacedWorkloadStatusPlaced indicates the workload has been successfully placed
	PlacedWorkloadStatusPlaced PlacedWorkloadStatus = "Placed"
	
	// PlacedWorkloadStatusFailed indicates the workload placement failed
	PlacedWorkloadStatusFailed PlacedWorkloadStatus = "Failed"
	
	// PlacedWorkloadStatusRemoved indicates the workload has been removed
	PlacedWorkloadStatusRemoved PlacedWorkloadStatus = "Removed"
)

// WorkloadPlacementList is a list of WorkloadPlacement resources
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type WorkloadPlacementList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []WorkloadPlacement `json:"items"`
}