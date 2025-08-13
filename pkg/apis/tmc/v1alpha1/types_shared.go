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