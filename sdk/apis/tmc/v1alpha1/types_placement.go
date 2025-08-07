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
// It defines how workloads should be selected and which clusters they should be placed on.
//
// +crd
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="Policy",type=string,JSONPath=`.spec.placementPolicy`
// +kubebuilder:printcolumn:name="Clusters",type=integer,JSONPath=`.status.selectedClusters[*]`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type WorkloadPlacement struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec WorkloadPlacementSpec `json:"spec,omitempty"`

	// +optional
	Status WorkloadPlacementStatus `json:"status,omitempty"`
}

// WorkloadPlacementSpec defines the desired state of workload placement.
type WorkloadPlacementSpec struct {
	// WorkloadSelector selects the workloads this placement applies to.
	WorkloadSelector WorkloadSelector `json:"workloadSelector"`

	// ClusterSelector defines how to select target clusters for placement.
	ClusterSelector ClusterSelector `json:"clusterSelector"`

	// PlacementPolicy defines the strategy for placing workloads across selected clusters.
	// +kubebuilder:default="RoundRobin"
	// +optional
	PlacementPolicy PlacementPolicy `json:"placementPolicy,omitempty"`

	// Tolerations allow workloads to be placed on clusters with matching taints.
	// +optional
	Tolerations []WorkloadToleration `json:"tolerations,omitempty"`

	// Affinity defines workload placement preferences and requirements.
	// +optional
	Affinity *WorkloadAffinity `json:"affinity,omitempty"`
}

// WorkloadSelector defines criteria for selecting workloads to be managed by this placement policy.
type WorkloadSelector struct {
	// LabelSelector selects workloads based on their labels.
	// +optional
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`

	// WorkloadTypes specifies the types of workloads to select.
	// If empty, all workload types are selected.
	// +optional
	WorkloadTypes []WorkloadType `json:"workloadTypes,omitempty"`

	// NamespaceSelector selects workloads from specific namespaces.
	// If empty, workloads from all namespaces are selected.
	// +optional
	NamespaceSelector *metav1.LabelSelector `json:"namespaceSelector,omitempty"`
}

// WorkloadType represents a Kubernetes workload type.
type WorkloadType struct {
	// APIVersion is the API version of the workload (e.g., "apps/v1").
	// +kubebuilder:validation:Required
	APIVersion string `json:"apiVersion"`

	// Kind is the kind of the workload (e.g., "Deployment", "StatefulSet").
	// +kubebuilder:validation:Required
	Kind string `json:"kind"`
}

// ClusterSelector defines criteria for selecting target clusters.
type ClusterSelector struct {
	// LabelSelector selects clusters based on their labels.
	// +optional
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`

	// LocationSelector selects clusters from specific locations.
	// +optional
	LocationSelector []string `json:"locationSelector,omitempty"`

	// ClusterNames explicitly lists cluster names to target.
	// If specified, only these clusters will be considered for placement.
	// +optional
	ClusterNames []string `json:"clusterNames,omitempty"`

	// CapabilityRequirements specifies required cluster capabilities.
	// +optional
	CapabilityRequirements *CapabilityRequirements `json:"capabilityRequirements,omitempty"`
}

// CapabilityRequirements defines requirements for cluster capabilities.
type CapabilityRequirements struct {
	// Compute specifies computational resource requirements.
	// +optional
	Compute *ComputeRequirements `json:"compute,omitempty"`

	// Storage specifies storage capability requirements.
	// +optional
	Storage *StorageRequirements `json:"storage,omitempty"`

	// Network specifies networking capability requirements.
	// +optional
	Network *NetworkRequirements `json:"network,omitempty"`
}

// ComputeRequirements defines computational resource requirements.
type ComputeRequirements struct {
	// Architecture specifies required CPU architecture.
	// +optional
	Architecture string `json:"architecture,omitempty"`

	// MinCPU is the minimum CPU requirement.
	// +optional
	MinCPU string `json:"minCPU,omitempty"`

	// MinMemory is the minimum memory requirement.
	// +optional
	MinMemory string `json:"minMemory,omitempty"`
}

// StorageRequirements defines storage capability requirements.
type StorageRequirements struct {
	// RequiredStorageClasses lists storage classes that must be available.
	// +optional
	RequiredStorageClasses []string `json:"requiredStorageClasses,omitempty"`

	// MinStorage is the minimum storage capacity requirement.
	// +optional
	MinStorage string `json:"minStorage,omitempty"`
}

// NetworkRequirements defines networking capability requirements.
type NetworkRequirements struct {
	// RequireLoadBalancer indicates that LoadBalancer support is required.
	// +optional
	RequireLoadBalancer bool `json:"requireLoadBalancer,omitempty"`

	// RequireIngress indicates that Ingress support is required.
	// +optional
	RequireIngress bool `json:"requireIngress,omitempty"`
}

// PlacementPolicy defines the strategy for workload placement.
// +kubebuilder:validation:Enum=RoundRobin;LeastLoaded;Random;LocationAware;Affinity
type PlacementPolicy string

const (
	// PlacementPolicyRoundRobin distributes workloads evenly across clusters in round-robin fashion.
	PlacementPolicyRoundRobin PlacementPolicy = "RoundRobin"

	// PlacementPolicyLeastLoaded places workloads on the cluster with the least current load.
	PlacementPolicyLeastLoaded PlacementPolicy = "LeastLoaded"

	// PlacementPolicyRandom randomly selects target clusters for workload placement.
	PlacementPolicyRandom PlacementPolicy = "Random"

	// PlacementPolicyLocationAware considers cluster location for placement decisions.
	PlacementPolicyLocationAware PlacementPolicy = "LocationAware"

	// PlacementPolicyAffinity uses affinity and anti-affinity rules for placement decisions.
	PlacementPolicyAffinity PlacementPolicy = "Affinity"
)

// WorkloadToleration allows workloads to be placed on clusters with matching taints.
type WorkloadToleration struct {
	// Key is the taint key that the toleration applies to.
	// +optional
	Key string `json:"key,omitempty"`

	// Operator represents the relationship between the key and value.
	// Valid operators are Equal and Exists.
	// +kubebuilder:default="Equal"
	// +kubebuilder:validation:Enum=Equal;Exists
	// +optional
	Operator TolerationOperator `json:"operator,omitempty"`

	// Value is the taint value the toleration matches.
	// +optional
	Value string `json:"value,omitempty"`

	// Effect indicates which taint effect to match.
	// +optional
	Effect TaintEffect `json:"effect,omitempty"`

	// TolerationSeconds represents the period of time the toleration tolerates the taint.
	// +optional
	TolerationSeconds *int64 `json:"tolerationSeconds,omitempty"`
}

// TolerationOperator is the operator for a toleration.
type TolerationOperator string

const (
	// TolerationOpEqual means the toleration value must equal the taint value.
	TolerationOpEqual TolerationOperator = "Equal"
	// TolerationOpExists means the toleration matches any taint with the specified key.
	TolerationOpExists TolerationOperator = "Exists"
)

// WorkloadAffinity defines affinity rules for workload placement.
type WorkloadAffinity struct {
	// ClusterAffinity defines rules for cluster selection based on cluster properties.
	// +optional
	ClusterAffinity *ClusterAffinity `json:"clusterAffinity,omitempty"`

	// ClusterAntiAffinity defines rules to avoid certain clusters.
	// +optional
	ClusterAntiAffinity *ClusterAffinity `json:"clusterAntiAffinity,omitempty"`
}

// ClusterAffinity defines affinity rules for cluster selection.
type ClusterAffinity struct {
	// RequiredDuringSchedulingIgnoredDuringExecution specifies hard requirements.
	// +optional
	RequiredDuringSchedulingIgnoredDuringExecution *ClusterSelector `json:"requiredDuringSchedulingIgnoredDuringExecution,omitempty"`

	// PreferredDuringSchedulingIgnoredDuringExecution specifies preferences.
	// +optional
	PreferredDuringSchedulingIgnoredDuringExecution []PreferredClusterSelector `json:"preferredDuringSchedulingIgnoredDuringExecution,omitempty"`
}

// PreferredClusterSelector defines a preference for cluster selection with a weight.
type PreferredClusterSelector struct {
	// Weight is the preference weight, in the range 1-100.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	Weight int32 `json:"weight"`

	// Preference defines the cluster selection preference.
	Preference ClusterSelector `json:"preference"`
}

// WorkloadPlacementStatus communicates the observed state of the WorkloadPlacement.
type WorkloadPlacementStatus struct {
	// Conditions represent the latest available observations of the placement's state.
	// +optional
	Conditions conditionsv1alpha1.Conditions `json:"conditions,omitempty"`

	// SelectedClusters lists the clusters selected for workload placement.
	// +optional
	SelectedClusters []string `json:"selectedClusters,omitempty"`

	// PlacedWorkloads tracks the workloads that have been placed according to this policy.
	// +optional
	PlacedWorkloads []PlacedWorkload `json:"placedWorkloads,omitempty"`

	// LastPlacementTime is the timestamp of the last placement decision.
	// +optional
	LastPlacementTime *metav1.Time `json:"lastPlacementTime,omitempty"`

	// PlacementHistory maintains a record of recent placement decisions.
	// +optional
	PlacementHistory []PlacementHistoryEntry `json:"placementHistory,omitempty"`
}

// PlacedWorkload represents a workload that has been placed on a cluster.
type PlacedWorkload struct {
	// WorkloadRef references the placed workload.
	WorkloadRef WorkloadReference `json:"workloadRef"`

	// ClusterName is the name of the cluster where the workload was placed.
	ClusterName string `json:"clusterName"`

	// PlacementTime is when the workload was placed on the cluster.
	PlacementTime metav1.Time `json:"placementTime"`

	// Status indicates the current status of the placed workload.
	// +kubebuilder:default="Pending"
	// +optional
	Status PlacedWorkloadStatus `json:"status,omitempty"`

	// LastUpdateTime is the last time the workload status was updated.
	// +optional
	LastUpdateTime *metav1.Time `json:"lastUpdateTime,omitempty"`
}

// WorkloadReference references a Kubernetes workload.
type WorkloadReference struct {
	// APIVersion is the API version of the workload.
	APIVersion string `json:"apiVersion"`

	// Kind is the kind of the workload.
	Kind string `json:"kind"`

	// Name is the name of the workload.
	Name string `json:"name"`

	// Namespace is the namespace of the workload.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// UID is the unique identifier of the workload.
	// +optional
	UID string `json:"uid,omitempty"`
}

// PlacedWorkloadStatus represents the status of a placed workload.
// +kubebuilder:validation:Enum=Pending;Placed;Failed;Removed
type PlacedWorkloadStatus string

const (
	// PlacedWorkloadStatusPending indicates the workload is waiting to be placed.
	PlacedWorkloadStatusPending PlacedWorkloadStatus = "Pending"

	// PlacedWorkloadStatusPlaced indicates the workload has been successfully placed.
	PlacedWorkloadStatusPlaced PlacedWorkloadStatus = "Placed"

	// PlacedWorkloadStatusFailed indicates the workload placement failed.
	PlacedWorkloadStatusFailed PlacedWorkloadStatus = "Failed"

	// PlacedWorkloadStatusRemoved indicates the workload has been removed from the cluster.
	PlacedWorkloadStatusRemoved PlacedWorkloadStatus = "Removed"
)

// PlacementHistoryEntry records a placement decision for historical tracking.
type PlacementHistoryEntry struct {
	// Timestamp is when the placement decision was made.
	Timestamp metav1.Time `json:"timestamp"`

	// Policy is the placement policy that was used.
	Policy PlacementPolicy `json:"policy"`

	// SelectedClusters are the clusters that were selected.
	SelectedClusters []string `json:"selectedClusters"`

	// Reason explains why this placement decision was made.
	// +optional
	Reason string `json:"reason,omitempty"`
}

// WorkloadPlacementList is a list of WorkloadPlacement resources.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type WorkloadPlacementList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []WorkloadPlacement `json:"items"`
}
