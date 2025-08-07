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

// PlacementStrategy defines the placement strategy
// +kubebuilder:validation:Enum=RoundRobin;LeastLoaded;Random;LocationAware;Spread;Affinity
type PlacementStrategy string

const (
	// PlacementStrategyRoundRobin distributes workloads evenly across clusters
	PlacementStrategyRoundRobin PlacementStrategy = "RoundRobin"
	
	// PlacementStrategyLeastLoaded places workloads on the least loaded cluster
	PlacementStrategyLeastLoaded PlacementStrategy = "LeastLoaded"
	
	// PlacementStrategyRandom randomly selects target clusters
	PlacementStrategyRandom PlacementStrategy = "Random"
	
	// PlacementStrategyLocationAware considers cluster location for placement
	PlacementStrategyLocationAware PlacementStrategy = "LocationAware"
	
	// PlacementStrategySpread spreads workloads across multiple clusters
	PlacementStrategySpread PlacementStrategy = "Spread"
	
	// PlacementStrategyAffinity uses affinity rules for placement decisions
	PlacementStrategyAffinity PlacementStrategy = "Affinity"
)

// WorkloadReference references a Kubernetes workload
type WorkloadReference struct {
	// APIVersion is the API version of the workload
	// +kubebuilder:validation:Required
	APIVersion string `json:"apiVersion"`

	// Kind is the kind of the workload
	// +kubebuilder:validation:Required
	Kind string `json:"kind"`

	// Name is the name of the workload
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Namespace is the namespace of the workload
	// +optional
	Namespace string `json:"namespace,omitempty"`
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

// CapabilityRequirement specifies a required cluster capability
type CapabilityRequirement struct {
	// Name is the name of the required capability
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Value is the required value for this capability (optional)
	// +optional
	Value string `json:"value,omitempty"`
}