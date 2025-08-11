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

// This file contains core TMC status types for cluster registration and basic placement.
// Extended status types (scaling, traffic monitoring, session management) are planned for future releases.

// ==========================
// Core Cluster Status Types
// ==========================

// ClusterRegistrationStatus communicates the observed state of a registered cluster.
type ClusterRegistrationStatus struct {
	// Conditions represent the latest available observations of the cluster's state
	// +optional
	Conditions conditionsv1alpha1.Conditions `json:"conditions,omitempty"`

	// LastHeartbeat is the timestamp of the last successful cluster heartbeat
	// +optional
	LastHeartbeat *metav1.Time `json:"lastHeartbeat,omitempty"`

	// AllocatedResources tracks the resources currently allocated on this cluster
	// +optional
	AllocatedResources *ClusterResourceUsage `json:"allocatedResources,omitempty"`

	// Capabilities contains the detected capabilities of the cluster
	// +optional
	Capabilities *ClusterCapabilities `json:"capabilities,omitempty"`
}

// ClusterResourceUsage tracks resource usage on a cluster
type ClusterResourceUsage struct {
	// CPU usage in milliCPU
	// +optional
	CPU *int64 `json:"cpu,omitempty"`

	// Memory usage in bytes
	// +optional
	Memory *int64 `json:"memory,omitempty"`

	// Pod count
	// +optional
	Pods *int32 `json:"pods,omitempty"`
}

// ClusterCapabilities contains the detected capabilities of a cluster
type ClusterCapabilities struct {
	// KubernetesVersion is the detected Kubernetes version
	// +optional
	KubernetesVersion string `json:"kubernetesVersion,omitempty"`

	// SupportedAPIVersions lists the API versions supported by the cluster
	// +optional
	SupportedAPIVersions []string `json:"supportedAPIVersions,omitempty"`

	// AvailableResources lists the resource types available in the cluster
	// +optional
	AvailableResources []string `json:"availableResources,omitempty"`

	// NodeCount is the number of nodes in the cluster
	// +optional
	NodeCount *int32 `json:"nodeCount,omitempty"`

	// Features contains detected cluster features
	// +optional
	Features []string `json:"features,omitempty"`

	// LastDetected is the timestamp when capabilities were last detected
	// +optional
	LastDetected *metav1.Time `json:"lastDetected,omitempty"`
}

// ==========================
// Basic Placement Status Types
// ==========================

// WorkloadPlacementStatus communicates the observed state of workload placement.
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

// PlacementDecision represents a placement decision made by the controller
type PlacementDecision struct {
	// ClusterName is the name of the selected cluster
	ClusterName string `json:"clusterName"`

	// Reason provides the rationale for this placement decision
	// +optional
	Reason string `json:"reason,omitempty"`

	// Score represents the placement score for this cluster (higher is better)
	// +optional
	Score *int32 `json:"score,omitempty"`

	// DecisionTime is when this placement decision was made
	DecisionTime metav1.Time `json:"decisionTime"`
}

// ==========================
// Shared Types and Enums
// ==========================

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