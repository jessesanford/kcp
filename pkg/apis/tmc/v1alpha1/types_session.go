/*
Copyright 2025 The KCP Authors.

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

// SessionAffinityPolicy defines policies for managing session affinity in TMC.
// It provides mechanisms for maintaining workload placement consistency
// and session stickiness across the cluster federation.
// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories=kcp,shortName=sap
type SessionAffinityPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired session affinity configuration
	Spec SessionAffinityPolicySpec `json:"spec,omitempty"`
	
	// Status contains the observed state of session affinity policy
	Status SessionAffinityPolicyStatus `json:"status,omitempty"`
}

// SessionAffinityPolicySpec defines the configuration for session affinity behavior.
type SessionAffinityPolicySpec struct {
	// SessionSelector defines which workloads this policy applies to
	// +optional
	SessionSelector *SessionSelector `json:"sessionSelector,omitempty"`
	
	// AffinityType defines the type of affinity to maintain
	// +kubebuilder:validation:Enum=ClusterAffinity;NodeAffinity;WorkspaceAffinity
	// +kubebuilder:default=ClusterAffinity
	AffinityType AffinityType `json:"affinityType,omitempty"`
	
	// SessionTTL defines how long sessions should be maintained
	// +kubebuilder:default="1h"
	// +optional
	SessionTTL *metav1.Duration `json:"sessionTTL,omitempty"`
	
	// StickinessFactor determines how strongly workloads stick to their current placement
	// Range: 0.0 (no stickiness) to 1.0 (maximum stickiness)
	// +kubebuilder:validation:Minimum=0.0
	// +kubebuilder:validation:Maximum=1.0
	// +kubebuilder:default=0.5
	// +optional
	StickinessFactor *float64 `json:"stickinessFactor,omitempty"`
	
	// MaxSessionsPerTarget limits concurrent sessions per target
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=100
	// +optional
	MaxSessionsPerTarget *int32 `json:"maxSessionsPerTarget,omitempty"`
}

// SessionSelector defines criteria for selecting workloads for session affinity.
type SessionSelector struct {
	// MatchLabels selects workloads with matching labels
	// +optional
	MatchLabels map[string]string `json:"matchLabels,omitempty"`
	
	// MatchExpressions provides label selector expressions
	// +optional
	MatchExpressions []metav1.LabelSelectorRequirement `json:"matchExpressions,omitempty"`
	
	// WorkloadTypes defines which workload types to include
	// +optional
	WorkloadTypes []string `json:"workloadTypes,omitempty"`
	
	// Namespaces defines namespace scope for selection
	// +optional
	Namespaces []string `json:"namespaces,omitempty"`
}

// AffinityType defines the scope of session affinity.
// +kubebuilder:validation:Enum=ClusterAffinity;NodeAffinity;WorkspaceAffinity
type AffinityType string

const (
	// ClusterAffinity maintains affinity at the cluster level
	ClusterAffinity AffinityType = "ClusterAffinity"
	
	// NodeAffinity maintains affinity at the node level within clusters
	NodeAffinity AffinityType = "NodeAffinity"
	
	// WorkspaceAffinity maintains affinity at the workspace level
	WorkspaceAffinity AffinityType = "WorkspaceAffinity"
)

// SessionAffinityPolicyStatus contains the observed state of session affinity policy.
type SessionAffinityPolicyStatus struct {
	// Conditions represent the current state of the session affinity policy
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	
	// ActiveSessions shows the number of currently active sessions
	// +optional
	ActiveSessions *int32 `json:"activeSessions,omitempty"`
	
	// TotalSessions shows the total number of sessions managed
	// +optional
	TotalSessions *int64 `json:"totalSessions,omitempty"`
	
	// LastUpdated is the timestamp of the last status update
	// +optional
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`
	
	// ObservedGeneration is the most recent generation observed by the controller
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// SessionState represents the current state of a workload session.
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories=kcp,shortName=ss
type SessionState struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired session state configuration
	Spec SessionStateSpec `json:"spec,omitempty"`
	
	// Status contains the observed state of the session
	Status SessionStateStatus `json:"status,omitempty"`
}

// SessionStateSpec defines the configuration for a workload session.
type SessionStateSpec struct {
	// WorkloadReference identifies the workload this session manages
	WorkloadReference WorkloadReference `json:"workloadReference"`
	
	// PlacementTargets defines where this session should be active
	// +optional
	PlacementTargets []PlacementTarget `json:"placementTargets,omitempty"`
	
	// SessionID provides a unique identifier for this session
	SessionID string `json:"sessionId"`
	
	// CreatedAt is the timestamp when this session was created
	CreatedAt metav1.Time `json:"createdAt"`
	
	// ExpiresAt defines when this session expires if not refreshed
	// +optional
	ExpiresAt *metav1.Time `json:"expiresAt,omitempty"`
}

// WorkloadReference identifies a workload managed by a session.
type WorkloadReference struct {
	// APIVersion is the API version of the workload
	APIVersion string `json:"apiVersion"`
	
	// Kind is the kind of the workload
	Kind string `json:"kind"`
	
	// Name is the name of the workload
	Name string `json:"name"`
	
	// Namespace is the namespace of the workload (if applicable)
	// +optional
	Namespace string `json:"namespace,omitempty"`
	
	// UID is the unique identifier of the workload
	// +optional
	UID string `json:"uid,omitempty"`
}

// PlacementTarget defines a target for workload placement.
type PlacementTarget struct {
	// ClusterName is the name of the target cluster
	ClusterName string `json:"clusterName"`
	
	// NodeSelector provides node selection criteria
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	
	// Priority defines the priority of this placement target
	// Higher values indicate higher priority
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:default=50
	// +optional
	Priority *int32 `json:"priority,omitempty"`
	
	// Weight defines the weight for load balancing
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1
	// +optional
	Weight *int32 `json:"weight,omitempty"`
}

// SessionStateStatus contains the observed state of a session.
type SessionStateStatus struct {
	// Phase represents the current phase of the session lifecycle
	// +kubebuilder:validation:Enum=Pending;Active;Expiring;Expired
	// +optional
	Phase SessionPhase `json:"phase,omitempty"`
	
	// CurrentPlacement shows where the workload is currently placed
	// +optional
	CurrentPlacement *PlacementTarget `json:"currentPlacement,omitempty"`
	
	// LastRefresh is the timestamp of the last session refresh
	// +optional
	LastRefresh *metav1.Time `json:"lastRefresh,omitempty"`
	
	// Conditions represent the current state of the session
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	
	// ObservedGeneration is the most recent generation observed by the controller
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// SessionPhase defines the lifecycle phase of a session.
// +kubebuilder:validation:Enum=Pending;Active;Expiring;Expired
type SessionPhase string

const (
	// SessionPending indicates the session is being established
	SessionPending SessionPhase = "Pending"
	
	// SessionActive indicates the session is active and managing workload placement
	SessionActive SessionPhase = "Active"
	
	// SessionExpiring indicates the session is approaching expiration
	SessionExpiring SessionPhase = "Expiring"
	
	// SessionExpired indicates the session has expired
	SessionExpired SessionPhase = "Expired"
)

// SessionAffinityPolicyList contains a list of SessionAffinityPolicy resources.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type SessionAffinityPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SessionAffinityPolicy `json:"items"`
}

// SessionStateList contains a list of SessionState resources.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type SessionStateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SessionState `json:"items"`
}