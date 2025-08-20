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
	"k8s.io/apimachinery/pkg/runtime"
)

// WorkloadPlacement defines the desired placement strategy for a workload
// +k8s:genclient
// +k8s:genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
type WorkloadPlacement struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired placement configuration
	Spec WorkloadPlacementSpec `json:"spec,omitempty"`

	// Status represents the current state of the workload placement
	Status WorkloadPlacementStatus `json:"status,omitempty"`
}

// WorkloadPlacementSpec defines the desired state of WorkloadPlacement
type WorkloadPlacementSpec struct {
	// Selector specifies which workloads this placement rule applies to
	Selector *metav1.LabelSelector `json:"selector,omitempty"`

	// TargetClusters defines the clusters where workloads should be placed
	TargetClusters []TargetCluster `json:"targetClusters,omitempty"`

	// PlacementPolicy defines how workloads should be distributed
	PlacementPolicy PlacementPolicy `json:"placementPolicy,omitempty"`
}

// TargetCluster represents a cluster that can host workloads
type TargetCluster struct {
	// Name of the target cluster
	Name string `json:"name"`

	// Weight for workload distribution (higher means more preference)
	// +optional
	Weight *int32 `json:"weight,omitempty"`
}

// PlacementPolicy defines workload distribution strategies
type PlacementPolicy struct {
	// Type of placement policy
	Type PlacementPolicyType `json:"type"`

	// MaxReplicas defines the maximum number of replicas per cluster
	// +optional
	MaxReplicas *int32 `json:"maxReplicas,omitempty"`
}

// PlacementPolicyType defines the type of placement policy
// +kubebuilder:validation:Enum=Spread;Pack;Preference
type PlacementPolicyType string

const (
	// SpreadPlacementPolicy spreads workloads across clusters
	SpreadPlacementPolicy PlacementPolicyType = "Spread"
	// PackPlacementPolicy packs workloads into fewer clusters
	PackPlacementPolicy PlacementPolicyType = "Pack"
	// PreferencePlacementPolicy uses cluster weights for placement
	PreferencePlacementPolicy PlacementPolicyType = "Preference"
)

// WorkloadPlacementStatus defines the observed state of WorkloadPlacement
type WorkloadPlacementStatus struct {
	// Conditions represents the latest available observations
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// SelectedClusters shows which clusters are currently selected
	SelectedClusters []string `json:"selectedClusters,omitempty"`
}

// WorkloadPlacementList contains a list of WorkloadPlacement
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type WorkloadPlacementList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WorkloadPlacement `json:"items"`
}

// WorkloadSync manages the synchronization of workloads to target clusters
// +k8s:genclient
// +k8s:genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
type WorkloadSync struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired synchronization configuration
	Spec WorkloadSyncSpec `json:"spec,omitempty"`

	// Status represents the current state of the workload sync
	Status WorkloadSyncStatus `json:"status,omitempty"`
}

// WorkloadSyncSpec defines the desired state of WorkloadSync
type WorkloadSyncSpec struct {
	// SourceWorkload defines the workload to synchronize
	SourceWorkload WorkloadReference `json:"sourceWorkload"`

	// TargetClusters defines where to synchronize the workload
	TargetClusters []string `json:"targetClusters"`

	// SyncPolicy defines how synchronization should behave
	SyncPolicy SyncPolicy `json:"syncPolicy,omitempty"`
}

// WorkloadReference identifies a workload to synchronize
type WorkloadReference struct {
	// APIVersion of the workload
	APIVersion string `json:"apiVersion"`

	// Kind of the workload
	Kind string `json:"kind"`

	// Namespace of the workload (if namespaced)
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Name of the workload
	Name string `json:"name"`
}

// SyncPolicy defines synchronization behavior
type SyncPolicy struct {
	// ConflictResolution defines how to handle sync conflicts
	ConflictResolution ConflictResolutionType `json:"conflictResolution,omitempty"`

	// SyncFrequency defines how often to check for changes
	// +optional
	SyncFrequency *metav1.Duration `json:"syncFrequency,omitempty"`
}

// ConflictResolutionType defines conflict resolution strategies
// +kubebuilder:validation:Enum=SourceWins;TargetWins;Manual
type ConflictResolutionType string

const (
	// SourceWinsConflictResolution always uses the source workload
	SourceWinsConflictResolution ConflictResolutionType = "SourceWins"
	// TargetWinsConflictResolution preserves target modifications
	TargetWinsConflictResolution ConflictResolutionType = "TargetWins"
	// ManualConflictResolution requires manual intervention
	ManualConflictResolution ConflictResolutionType = "Manual"
)

// WorkloadSyncStatus defines the observed state of WorkloadSync
type WorkloadSyncStatus struct {
	// Conditions represents the latest available observations
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// SyncTargets shows the status of each target cluster
	SyncTargets []SyncTarget `json:"syncTargets,omitempty"`
}

// SyncTarget represents the sync status for a specific cluster
type SyncTarget struct {
	// Cluster name
	Cluster string `json:"cluster"`

	// State of synchronization
	State SyncState `json:"state"`

	// LastSyncTime when sync was last attempted
	// +optional
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	// Message provides additional details about the sync state
	// +optional
	Message string `json:"message,omitempty"`

	// SyncedObject contains the object reference that was synced
	// +optional
	SyncedObject *runtime.RawExtension `json:"syncedObject,omitempty"`
}

// SyncState represents the state of workload synchronization
// +kubebuilder:validation:Enum=Pending;Syncing;Synced;Failed
type SyncState string

const (
	// PendingSyncState indicates sync has not started
	PendingSyncState SyncState = "Pending"
	// SyncingSyncState indicates sync is in progress
	SyncingSyncState SyncState = "Syncing"
	// SyncedSyncState indicates sync completed successfully
	SyncedSyncState SyncState = "Synced"
	// FailedSyncState indicates sync failed
	FailedSyncState SyncState = "Failed"
)

// WorkloadSyncList contains a list of WorkloadSync
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type WorkloadSyncList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WorkloadSync `json:"items"`
}