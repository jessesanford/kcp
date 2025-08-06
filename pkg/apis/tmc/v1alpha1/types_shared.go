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

// WorkloadSelector selects workloads based on various criteria
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
	APIVersion string `json:"apiVersion"`

	// Kind is the kind of the workload
	Kind string `json:"kind"`
}

// ClusterSelector selects clusters based on various criteria
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

// WorkloadHealthStatus defines the health status values for workloads
// +kubebuilder:validation:Enum=Healthy;Unhealthy;Degraded;Unknown;Checking
type WorkloadHealthStatus string

const (
	// WorkloadHealthStatusHealthy indicates the workload is healthy
	WorkloadHealthStatusHealthy WorkloadHealthStatus = "Healthy"
	// WorkloadHealthStatusUnhealthy indicates the workload is unhealthy
	WorkloadHealthStatusUnhealthy WorkloadHealthStatus = "Unhealthy"
	// WorkloadHealthStatusDegraded indicates the workload is degraded
	WorkloadHealthStatusDegraded WorkloadHealthStatus = "Degraded"
	// WorkloadHealthStatusUnknown indicates the workload health is unknown
	WorkloadHealthStatusUnknown WorkloadHealthStatus = "Unknown"
	// WorkloadHealthStatusChecking indicates the workload is being checked
	WorkloadHealthStatusChecking WorkloadHealthStatus = "Checking"
)

// ObjectReference is a reference to a Kubernetes object
type ObjectReference struct {
	// APIVersion is the API version of the referenced object
	APIVersion string `json:"apiVersion"`

	// Kind is the kind of the referenced object
	Kind string `json:"kind"`

	// Name is the name of the referenced object
	Name string `json:"name"`

	// Namespace is the namespace of the referenced object
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// UID is the UID of the referenced object
	// +optional
	UID string `json:"uid,omitempty"`
}

// SessionReference is a reference to a placement session
type SessionReference struct {
	// Name is the name of the session
	Name string `json:"name"`

	// Namespace is the namespace of the session
	Namespace string `json:"namespace"`

	// SessionID is the unique identifier of the session
	// +optional
	SessionID string `json:"sessionID,omitempty"`
}

// WorkloadReference is a reference to a workload
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

// PlacementReference is a reference to a placement decision
type PlacementReference struct {
	// Name is the name of the placement decision
	Name string `json:"name"`

	// Namespace is the namespace of the placement decision
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// UID is the UID of the placement decision
	// +optional
	UID string `json:"uid,omitempty"`
}

// ConflictType defines the types of conflicts that can occur
// +kubebuilder:validation:Enum=ResourceContention;PolicyViolation;AffinityConflict;ConstraintViolation;ClusterUnavailable
type ConflictType string

const (
	// ConflictTypeResourceContention indicates resource contention conflict
	ConflictTypeResourceContention ConflictType = "ResourceContention"
	// ConflictTypePolicyViolation indicates policy violation conflict
	ConflictTypePolicyViolation ConflictType = "PolicyViolation"
	// ConflictTypeAffinityConflict indicates affinity rule conflict
	ConflictTypeAffinityConflict ConflictType = "AffinityConflict"
	// ConflictTypeConstraintViolation indicates constraint violation conflict
	ConflictTypeConstraintViolation ConflictType = "ConstraintViolation"
	// ConflictTypeClusterUnavailable indicates cluster unavailability conflict
	ConflictTypeClusterUnavailable ConflictType = "ClusterUnavailable"
)

// ConflictResolutionType defines the types of conflict resolution strategies
// +kubebuilder:validation:Enum=Override;Merge;Fail
type ConflictResolutionType string

const (
	// ConflictResolutionTypeOverride overrides existing placements with new ones
	ConflictResolutionTypeOverride ConflictResolutionType = "Override"
	// ConflictResolutionTypeMerge merges conflicting placements when possible
	ConflictResolutionTypeMerge ConflictResolutionType = "Merge"
	// ConflictResolutionTypeFail fails the session when conflicts occur
	ConflictResolutionTypeFail ConflictResolutionType = "Fail"
)

// ConflictStatus defines the status of a conflict
// +kubebuilder:validation:Enum=Detected;Analyzing;Resolving;Resolved;Failed
type ConflictStatus string

const (
	// ConflictStatusDetected indicates the conflict has been detected
	ConflictStatusDetected ConflictStatus = "Detected"
	// ConflictStatusAnalyzing indicates the conflict is being analyzed
	ConflictStatusAnalyzing ConflictStatus = "Analyzing"
	// ConflictStatusResolving indicates the conflict is being resolved
	ConflictStatusResolving ConflictStatus = "Resolving"
	// ConflictStatusResolved indicates the conflict has been resolved
	ConflictStatusResolved ConflictStatus = "Resolved"
	// ConflictStatusFailed indicates conflict resolution has failed
	ConflictStatusFailed ConflictStatus = "Failed"
)