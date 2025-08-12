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

// ClusterRegistrationStatus communicates the observed state of the ClusterRegistration.
// This status focuses on the health, connectivity, and resource tracking aspects
// of registered physical clusters in the TMC system.
type ClusterRegistrationStatus struct {
	// Conditions represent the latest available observations of the cluster's state
	// Key conditions include: Ready, Connected, Heartbeat, Registration, Resources
	// +optional
	Conditions conditionsv1alpha1.Conditions `json:"conditions,omitempty"`

	// LastHeartbeat is the timestamp of the last successful cluster heartbeat
	// Used for cluster health monitoring and availability tracking
	// +optional
	LastHeartbeat *metav1.Time `json:"lastHeartbeat,omitempty"`

	// AllocatedResources tracks the resources currently allocated on this cluster
	// Updated through resource monitoring and workload placement tracking
	// +optional
	AllocatedResources *ClusterResourceUsage `json:"allocatedResources,omitempty"`

	// Capabilities contains the detected capabilities of the cluster
	// Populated through cluster discovery and feature detection
	// +optional
	Capabilities *ClusterCapabilities `json:"capabilities,omitempty"`

	// ConnectionStatus provides detailed information about cluster connectivity
	// +optional
	ConnectionStatus *ClusterConnectionStatus `json:"connectionStatus,omitempty"`

	// HealthMetrics contains aggregated health metrics for the cluster
	// +optional
	HealthMetrics *ClusterHealthMetrics `json:"healthMetrics,omitempty"`
}

// ClusterConnectionStatus provides detailed connectivity information
type ClusterConnectionStatus struct {
	// LastConnected is the timestamp when the cluster was last successfully contacted
	// +optional
	LastConnected *metav1.Time `json:"lastConnected,omitempty"`

	// ConnectionLatency tracks the network latency to the cluster
	// +optional
	ConnectionLatency *metav1.Duration `json:"connectionLatency,omitempty"`

	// APIServerURL is the currently active API server endpoint
	// +optional
	APIServerURL string `json:"apiServerURL,omitempty"`

	// TLSVerified indicates whether TLS verification succeeded
	// +optional
	TLSVerified *bool `json:"tlsVerified,omitempty"`

	// ErrorMessage contains the last connection error if any
	// +optional
	ErrorMessage string `json:"errorMessage,omitempty"`
}

// ClusterHealthMetrics contains aggregated health information
type ClusterHealthMetrics struct {
	// HealthScore is a computed health score (0-100) based on various metrics
	// +optional
	HealthScore *int32 `json:"healthScore,omitempty"`

	// NodeReadyCount tracks how many nodes are in Ready state
	// +optional
	NodeReadyCount *int32 `json:"nodeReadyCount,omitempty"`

	// NodeTotalCount is the total number of nodes
	// +optional
	NodeTotalCount *int32 `json:"nodeTotalCount,omitempty"`

	// PodReadyPercent is the percentage of pods in Ready state
	// +optional
	PodReadyPercent *int32 `json:"podReadyPercent,omitempty"`

	// ResourceUtilization tracks resource usage percentages
	// +optional
	ResourceUtilization *ResourceUtilizationMetrics `json:"resourceUtilization,omitempty"`

	// LastMetricsUpdate is when these metrics were last updated
	// +optional
	LastMetricsUpdate *metav1.Time `json:"lastMetricsUpdate,omitempty"`
}

// ResourceUtilizationMetrics tracks resource usage percentages
type ResourceUtilizationMetrics struct {
	// CPUUtilization is the CPU usage percentage (0-100)
	// +optional
	CPUUtilization *int32 `json:"cpuUtilization,omitempty"`

	// MemoryUtilization is the memory usage percentage (0-100)
	// +optional
	MemoryUtilization *int32 `json:"memoryUtilization,omitempty"`

	// StorageUtilization is the storage usage percentage (0-100)
	// +optional
	StorageUtilization *int32 `json:"storageUtilization,omitempty"`
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

	// Storage usage in bytes
	// +optional
	Storage *int64 `json:"storage,omitempty"`
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

	// PlatformInfo contains information about the underlying platform
	// +optional
	PlatformInfo *PlatformInfo `json:"platformInfo,omitempty"`
}

// PlatformInfo contains platform-specific information
type PlatformInfo struct {
	// Type indicates the platform type (e.g., "aws", "gcp", "azure", "on-premises")
	// +optional
	Type string `json:"type,omitempty"`

	// Region is the platform region/zone information
	// +optional
	Region string `json:"region,omitempty"`

	// Version is the platform-specific version information
	// +optional
	Version string `json:"version,omitempty"`
}

// WorkloadPlacementStatus communicates the observed state of the WorkloadPlacement.
// This status focuses on placement decisions, workload tracking, and placement health.
type WorkloadPlacementStatus struct {
	// Conditions represent the latest available observations of the placement's state
	// Key conditions include: Ready, PlacementAvailable, Scheduling, Synced
	// +optional
	Conditions conditionsv1alpha1.Conditions `json:"conditions,omitempty"`

	// SelectedClusters lists the clusters selected for workload placement
	// Updated when placement decisions are made
	// +optional
	SelectedClusters []string `json:"selectedClusters,omitempty"`

	// PlacedWorkloads tracks the workloads that have been placed
	// Contains status tracking for each placed workload
	// +optional
	PlacedWorkloads []PlacedWorkload `json:"placedWorkloads,omitempty"`

	// LastPlacementTime is the timestamp of the last placement decision
	// +optional
	LastPlacementTime *metav1.Time `json:"lastPlacementTime,omitempty"`

	// PlacementDecisions tracks the current placement decisions
	// Includes reasoning and scoring for placement choices
	// +optional
	PlacementDecisions []PlacementDecision `json:"placementDecisions,omitempty"`

	// PlacementStats provides aggregated statistics about placement operations
	// +optional
	PlacementStats *PlacementStatistics `json:"placementStats,omitempty"`

	// ConflictingPlacements tracks any placement conflicts that need resolution
	// +optional
	ConflictingPlacements []PlacementConflict `json:"conflictingPlacements,omitempty"`
}

// PlacedWorkload represents a workload that has been placed on a cluster with status tracking
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

	// StatusMessage provides additional context about the workload status
	// +optional
	StatusMessage string `json:"statusMessage,omitempty"`

	// LastStatusUpdate tracks when the status was last updated
	// +optional
	LastStatusUpdate *metav1.Time `json:"lastStatusUpdate,omitempty"`

	// ResourcesAllocated tracks the resources allocated to this workload
	// +optional
	ResourcesAllocated *WorkloadResourceUsage `json:"resourcesAllocated,omitempty"`
}

// PlacementDecision represents a placement decision made by the controller with status information
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

	// DecisionStatus indicates the current status of this placement decision
	// +optional
	DecisionStatus PlacementDecisionStatus `json:"decisionStatus,omitempty"`
}

// PlacementStatistics provides aggregated statistics about placement operations
type PlacementStatistics struct {
	// TotalPlacements is the total number of placement attempts
	// +optional
	TotalPlacements *int32 `json:"totalPlacements,omitempty"`

	// SuccessfulPlacements is the number of successful placements
	// +optional
	SuccessfulPlacements *int32 `json:"successfulPlacements,omitempty"`

	// FailedPlacements is the number of failed placements
	// +optional
	FailedPlacements *int32 `json:"failedPlacements,omitempty"`

	// AverageDecisionTime is the average time to make placement decisions
	// +optional
	AverageDecisionTime *metav1.Duration `json:"averageDecisionTime,omitempty"`

	// LastStatisticsUpdate is when these statistics were last updated
	// +optional
	LastStatisticsUpdate *metav1.Time `json:"lastStatisticsUpdate,omitempty"`
}

// PlacementConflict represents a conflict in placement decisions that needs resolution
type PlacementConflict struct {
	// ConflictType describes the type of conflict
	// +kubebuilder:validation:Required
	ConflictType PlacementConflictType `json:"conflictType"`

	// WorkloadRef references the workload involved in the conflict
	// +kubebuilder:validation:Required
	WorkloadRef WorkloadReference `json:"workloadRef"`

	// ConflictingClusters lists the clusters involved in the conflict
	// +optional
	ConflictingClusters []string `json:"conflictingClusters,omitempty"`

	// ConflictMessage describes the conflict in detail
	// +optional
	ConflictMessage string `json:"conflictMessage,omitempty"`

	// DetectedTime is when the conflict was first detected
	// +kubebuilder:validation:Required
	DetectedTime metav1.Time `json:"detectedTime"`

	// ResolutionStatus indicates the status of conflict resolution
	// +optional
	ResolutionStatus ConflictResolutionStatus `json:"resolutionStatus,omitempty"`
}

// WorkloadResourceUsage tracks resources used by a specific workload
type WorkloadResourceUsage struct {
	// CPU usage in milliCPU
	// +optional
	CPU *int64 `json:"cpu,omitempty"`

	// Memory usage in bytes
	// +optional
	Memory *int64 `json:"memory,omitempty"`

	// Storage usage in bytes
	// +optional
	Storage *int64 `json:"storage,omitempty"`
}

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
// +kubebuilder:validation:Enum=Pending;Placed;Running;Failed;Removed
type PlacedWorkloadStatus string

const (
	// PlacedWorkloadStatusPending indicates the workload is waiting to be placed
	PlacedWorkloadStatusPending PlacedWorkloadStatus = "Pending"
	
	// PlacedWorkloadStatusPlaced indicates the workload has been successfully placed
	PlacedWorkloadStatusPlaced PlacedWorkloadStatus = "Placed"
	
	// PlacedWorkloadStatusRunning indicates the workload is running successfully
	PlacedWorkloadStatusRunning PlacedWorkloadStatus = "Running"
	
	// PlacedWorkloadStatusFailed indicates the workload placement failed
	PlacedWorkloadStatusFailed PlacedWorkloadStatus = "Failed"
	
	// PlacedWorkloadStatusRemoved indicates the workload has been removed
	PlacedWorkloadStatusRemoved PlacedWorkloadStatus = "Removed"
)

// PlacementDecisionStatus represents the status of a placement decision
// +kubebuilder:validation:Enum=Pending;Approved;Rejected;Executed;Failed
type PlacementDecisionStatus string

const (
	// PlacementDecisionStatusPending indicates the decision is pending review
	PlacementDecisionStatusPending PlacementDecisionStatus = "Pending"
	
	// PlacementDecisionStatusApproved indicates the decision has been approved
	PlacementDecisionStatusApproved PlacementDecisionStatus = "Approved"
	
	// PlacementDecisionStatusRejected indicates the decision was rejected
	PlacementDecisionStatusRejected PlacementDecisionStatus = "Rejected"
	
	// PlacementDecisionStatusExecuted indicates the decision has been executed
	PlacementDecisionStatusExecuted PlacementDecisionStatus = "Executed"
	
	// PlacementDecisionStatusFailed indicates the decision execution failed
	PlacementDecisionStatusFailed PlacementDecisionStatus = "Failed"
)

// PlacementConflictType represents the type of placement conflict
// +kubebuilder:validation:Enum=ResourceConflict;PolicyConflict;DependencyConflict;AffinityConflict
type PlacementConflictType string

const (
	// PlacementConflictTypeResource indicates a resource availability conflict
	PlacementConflictTypeResource PlacementConflictType = "ResourceConflict"
	
	// PlacementConflictTypePolicy indicates a policy violation conflict
	PlacementConflictTypePolicy PlacementConflictType = "PolicyConflict"
	
	// PlacementConflictTypeDependency indicates a dependency constraint conflict
	PlacementConflictTypeDependency PlacementConflictType = "DependencyConflict"
	
	// PlacementConflictTypeAffinity indicates an affinity/anti-affinity conflict
	PlacementConflictTypeAffinity PlacementConflictType = "AffinityConflict"
)

// ConflictResolutionStatus represents the status of conflict resolution
// +kubebuilder:validation:Enum=Detected;InProgress;Resolved;Failed
type ConflictResolutionStatus string

const (
	// ConflictResolutionStatusDetected indicates the conflict has been detected
	ConflictResolutionStatusDetected ConflictResolutionStatus = "Detected"
	
	// ConflictResolutionStatusInProgress indicates resolution is in progress
	ConflictResolutionStatusInProgress ConflictResolutionStatus = "InProgress"
	
	// ConflictResolutionStatusResolved indicates the conflict has been resolved
	ConflictResolutionStatusResolved ConflictResolutionStatus = "Resolved"
	
	// ConflictResolutionStatusFailed indicates resolution failed
	ConflictResolutionStatusFailed ConflictResolutionStatus = "Failed"
)