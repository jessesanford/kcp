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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories=kcp
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status"

// Placement represents a workload placement policy within KCP's TMC system.
// It defines how workloads should be placed across available clusters based on
// specified constraints and requirements.
type Placement struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PlacementSpec   `json:"spec,omitempty"`
	Status PlacementStatus `json:"status,omitempty"`
}

// PlacementSpec defines the desired state of Placement.
type PlacementSpec struct {
	// WorkloadReference specifies which workload this placement applies to.
	// +required
	WorkloadReference WorkloadReference `json:"workloadReference"`

	// LocationSelector defines the criteria for selecting target locations.
	// +optional
	LocationSelector *LocationSelector `json:"locationSelector,omitempty"`

	// NumberOfClusters specifies how many clusters the workload should be placed on.
	// If not specified, the scheduler will determine the optimal number.
	// +optional
	// +kubebuilder:validation:Minimum=1
	NumberOfClusters *int32 `json:"numberOfClusters,omitempty"`

	// Constraints define placement constraints that must be satisfied.
	// +optional
	Constraints *PlacementConstraints `json:"constraints,omitempty"`
}

// WorkloadReference identifies a workload to be placed.
type WorkloadReference struct {
	// APIVersion of the referenced workload.
	// +required
	APIVersion string `json:"apiVersion"`

	// Kind of the referenced workload.
	// +required  
	Kind string `json:"kind"`

	// Name of the referenced workload.
	// +required
	Name string `json:"name"`

	// Namespace of the referenced workload (if namespaced).
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// LocationSelector defines criteria for selecting target locations.
type LocationSelector struct {
	// MatchLabels is a map of {key,value} pairs matching location labels.
	// +optional
	MatchLabels map[string]string `json:"matchLabels,omitempty"`

	// MatchExpressions is a list of label selector requirements for locations.
	// +optional
	MatchExpressions []metav1.LabelSelectorRequirement `json:"matchExpressions,omitempty"`
}

// PlacementConstraints define constraints that must be satisfied for placement.
type PlacementConstraints struct {
	// Affinity specifies affinity constraints for cluster placement.
	// +optional
	Affinity *PlacementAffinity `json:"affinity,omitempty"`

	// Tolerations allow placement on clusters with matching taints.
	// +optional
	Tolerations []PlacementToleration `json:"tolerations,omitempty"`
}

// PlacementAffinity defines affinity constraints for cluster placement.
type PlacementAffinity struct {
	// ClusterAffinity specifies cluster-level affinity constraints.
	// +optional
	ClusterAffinity *ClusterAffinity `json:"clusterAffinity,omitempty"`

	// ClusterAntiAffinity specifies cluster-level anti-affinity constraints.
	// +optional
	ClusterAntiAffinity *ClusterAntiAffinity `json:"clusterAntiAffinity,omitempty"`
}

// ClusterAffinity specifies cluster affinity scheduling rules.
type ClusterAffinity struct {
	// RequiredDuringSchedulingIgnoredDuringExecution specifies hard constraints.
	// +optional
	RequiredDuringSchedulingIgnoredDuringExecution *ClusterAffinityTerm `json:"requiredDuringSchedulingIgnoredDuringExecution,omitempty"`

	// PreferredDuringSchedulingIgnoredDuringExecution specifies soft constraints.
	// +optional
	PreferredDuringSchedulingIgnoredDuringExecution []WeightedClusterAffinityTerm `json:"preferredDuringSchedulingIgnoredDuringExecution,omitempty"`
}

// ClusterAntiAffinity specifies cluster anti-affinity scheduling rules.
type ClusterAntiAffinity struct {
	// RequiredDuringSchedulingIgnoredDuringExecution specifies hard constraints.
	// +optional
	RequiredDuringSchedulingIgnoredDuringExecution []ClusterAffinityTerm `json:"requiredDuringSchedulingIgnoredDuringExecution,omitempty"`

	// PreferredDuringSchedulingIgnoredDuringExecution specifies soft constraints.
	// +optional
	PreferredDuringSchedulingIgnoredDuringExecution []WeightedClusterAffinityTerm `json:"preferredDuringSchedulingIgnoredDuringExecution,omitempty"`
}

// ClusterAffinityTerm defines a cluster affinity term.
type ClusterAffinityTerm struct {
	// LabelSelector selects clusters by labels.
	// +optional
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`

	// LocationNames specifies specific location names.
	// +optional
	LocationNames []string `json:"locationNames,omitempty"`
}

// WeightedClusterAffinityTerm defines a weighted cluster affinity term.
type WeightedClusterAffinityTerm struct {
	// Weight associated with matching the corresponding clusterAffinityTerm.
	// +required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	Weight int32 `json:"weight"`

	// ClusterAffinityTerm is the affinity term.
	// +required
	ClusterAffinityTerm ClusterAffinityTerm `json:"clusterAffinityTerm"`
}

// PlacementToleration represents a toleration for cluster taints.
type PlacementToleration struct {
	// Key is the taint key that the toleration applies to.
	// +optional
	Key string `json:"key,omitempty"`

	// Operator represents a key's relationship to the value.
	// +optional
	Operator corev1.TolerationOperator `json:"operator,omitempty"`

	// Value is the taint value the toleration matches to.
	// +optional
	Value string `json:"value,omitempty"`

	// Effect indicates the taint effect to match.
	// +optional
	Effect corev1.TaintEffect `json:"effect,omitempty"`

	// TolerationSeconds represents the period of time the toleration tolerates the taint.
	// +optional
	TolerationSeconds *int64 `json:"tolerationSeconds,omitempty"`
}

// PlacementStatus represents the observed state of Placement.
type PlacementStatus struct {
	// Conditions contain details about the placement state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// PlacementDecisions contains the results of placement decisions.
	// +optional
	PlacementDecisions []PlacementDecision `json:"placementDecisions,omitempty"`

	// ObservedGeneration reflects the generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// PlacementDecision represents a placement decision for a specific cluster.
type PlacementDecision struct {
	// ClusterName is the name of the cluster where the workload is placed.
	// +required
	ClusterName string `json:"clusterName"`

	// Location is the location information of the cluster.
	// +optional
	Location string `json:"location,omitempty"`

	// Reason explains why this cluster was selected.
	// +optional
	Reason string `json:"reason,omitempty"`

	// Score represents the placement score for this cluster (0-100).
	// +optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	Score *int32 `json:"score,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PlacementList contains a list of Placement resources.
type PlacementList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Placement `json:"items"`
}

// Common placement condition types
const (
	// PlacementReadyCondition indicates whether the placement is ready.
	PlacementReadyCondition = "Ready"

	// PlacementValidCondition indicates whether the placement specification is valid.
	PlacementValidCondition = "Valid"

	// PlacementScheduledCondition indicates whether placement decisions have been made.
	PlacementScheduledCondition = "Scheduled"
)

// Common placement condition reasons
const (
	// PlacementReadyReason indicates successful placement processing.
	PlacementReadyReason = "Ready"

	// PlacementValidReason indicates valid placement specification.
	PlacementValidReason = "Valid" 

	// PlacementScheduledReason indicates successful scheduling decisions.
	PlacementScheduledReason = "Scheduled"

	// PlacementInvalidReason indicates invalid placement specification.
	PlacementInvalidReason = "Invalid"

	// PlacementSchedulingFailedReason indicates scheduling failures.
	PlacementSchedulingFailedReason = "SchedulingFailed"
)

// Stub types for existing workload APIs - these will be enhanced in later PRs
// These minimal definitions allow the placement controller to compile and function.

// +genclient
// +genclient:nonNamespaced  
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Cluster,categories=kcp

// Location represents a physical or logical location where clusters can be deployed.
type Location struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	
	Spec   LocationSpec   `json:"spec,omitempty"`
	Status LocationStatus `json:"status,omitempty"`
}

// LocationSpec defines the desired state of Location.
type LocationSpec struct {
	// DisplayName is the human-readable name for this location.
	// +optional
	DisplayName string `json:"displayName,omitempty"`
}

// LocationStatus represents the observed state of Location.
type LocationStatus struct {
	// Conditions contain details about the location state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// LocationList contains a list of Location resources.
type LocationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Location `json:"items"`
}

// Minimal stub types for other workload resources to satisfy compilation
// These will be enhanced in subsequent PRs focused on their specific functionality.

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ResourceExport represents a resource export in the workload API.
type ResourceExport struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ResourceExportList contains a list of ResourceExport resources.
type ResourceExportList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ResourceExport `json:"items"`
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ResourceImport represents a resource import in the workload API.
type ResourceImport struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ResourceImportList contains a list of ResourceImport resources.
type ResourceImportList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ResourceImport `json:"items"`
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SyncTarget represents a sync target in the workload API.
type SyncTarget struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SyncTargetList contains a list of SyncTarget resources.
type SyncTargetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SyncTarget `json:"items"`
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SyncTargetHeartbeat represents a sync target heartbeat in the workload API.
type SyncTargetHeartbeat struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SyncTargetHeartbeatList contains a list of SyncTargetHeartbeat resources.
type SyncTargetHeartbeatList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SyncTargetHeartbeat `json:"items"`
}