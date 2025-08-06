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

// ClusterRegistration represents a physical cluster that can execute workloads
// This integrates with KCP's APIExport/APIBinding system for API distribution
//
// +crd
// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories=kcp,shortName=cr
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Location",type=string,JSONPath=`.spec.location`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type ClusterRegistration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:",inline"`

	// Spec defines the desired state of the ClusterRegistration.
	// +optional
	Spec ClusterRegistrationSpec `json:"spec,omitempty"`

	// Status communicates the observed state of the ClusterRegistration.
	// +optional
	Status ClusterRegistrationStatus `json:"status,omitempty"`
}

// ClusterRegistrationSpec defines the desired state of a ClusterRegistration
type ClusterRegistrationSpec struct {
	// location provides logical location information for placement decisions.
	// This could be a region, zone, or any logical identifier for placement.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	// +required
	Location string `json:"location"`

	// capabilities define what this cluster can execute.
	// Each capability describes a type of workload or feature the cluster supports.
	// +optional
	// +listType=map
	// +listMapKey=type
	Capabilities []ClusterCapability `json:"capabilities,omitempty"`

	// apiBindings specify which KCP APIs this cluster should receive.
	// This integrates with KCP's existing APIBinding system for multi-tenant API access.
	// +optional
	// +listType=map
	// +listMapKey=name
	APIBindings []APIBindingReference `json:"apiBindings,omitempty"`
}

// ClusterCapability defines a capability of the physical cluster
type ClusterCapability struct {
	// type of capability (e.g., "compute", "storage", "networking")
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	// +required
	Type string `json:"type"`

	// available indicates if the capability is currently available
	// +required
	Available bool `json:"available"`

	// properties provide additional capability metadata as key-value pairs
	// +optional
	Properties map[string]string `json:"properties,omitempty"`
}

// APIBindingReference references an APIBinding for this cluster
type APIBindingReference struct {
	// name of the APIBinding in the workspace
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`
	// +required
	Name string `json:"name"`

	// workspace containing the APIBinding (defaults to current workspace)
	// +optional
	Workspace string `json:"workspace,omitempty"`
}

// ClusterRegistrationStatus represents the observed state of a ClusterRegistration
type ClusterRegistrationStatus struct {
	// conditions represent the latest available observations of the ClusterRegistration's current state.
	// +optional
	Conditions conditionsv1alpha1.Conditions `json:"conditions,omitempty"`

	// connectedAPIs tracks which APIs are successfully bound to this cluster
	// +optional
	// +listType=map
	// +listMapKey=apiBinding
	ConnectedAPIs []ConnectedAPI `json:"connectedAPIs,omitempty"`

	// lastHeartbeat tracks when the cluster last reported status
	// +optional
	LastHeartbeat *metav1.Time `json:"lastHeartbeat,omitempty"`
}

// ConnectedAPI tracks API binding status for a specific APIBinding
type ConnectedAPI struct {
	// apiBinding name that this status refers to
	// +kubebuilder:validation:MinLength=1
	// +required
	APIBinding string `json:"apiBinding"`

	// connected indicates successful binding and availability
	// +required
	Connected bool `json:"connected"`

	// error message if connection failed
	// +optional
	Error string `json:"error,omitempty"`
}

// WorkloadPlacement defines where workloads should be placed
// This works with KCP's workspace system for multi-tenant placement
//
// +crd
// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories=kcp,shortName=wp
// +kubebuilder:printcolumn:name="Strategy",type=string,JSONPath=`.spec.strategy`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type WorkloadPlacement struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:",inline"`

	// Spec defines the desired state of the WorkloadPlacement.
	// +optional
	Spec WorkloadPlacementSpec `json:"spec,omitempty"`

	// Status communicates the observed state of the WorkloadPlacement.
	// +optional
	Status WorkloadPlacementStatus `json:"status,omitempty"`
}

// WorkloadPlacementSpec defines the placement policy for workloads
type WorkloadPlacementSpec struct {
	// strategy defines how workloads are placed across available clusters
	// +kubebuilder:validation:Enum=RoundRobin;Affinity;Spread
	// +kubebuilder:default=RoundRobin
	// +required
	Strategy PlacementStrategy `json:"strategy"`

	// locationSelector selects clusters by location labels.
	// If not specified, all clusters are eligible for placement.
	// +optional
	LocationSelector *metav1.LabelSelector `json:"locationSelector,omitempty"`

	// capabilityRequirements specify required cluster capabilities.
	// All requirements must be met for a cluster to be selected.
	// +optional
	// +listType=map
	// +listMapKey=type
	CapabilityRequirements []CapabilityRequirement `json:"capabilityRequirements,omitempty"`
}

// PlacementStrategy defines the available placement strategies
// +kubebuilder:validation:Enum=RoundRobin;Affinity;Spread
type PlacementStrategy string

const (
	// PlacementStrategyRoundRobin distributes workloads evenly across clusters
	PlacementStrategyRoundRobin PlacementStrategy = "RoundRobin"
	// PlacementStrategyAffinity places workloads based on affinity rules
	PlacementStrategyAffinity PlacementStrategy = "Affinity"
	// PlacementStrategySpread spreads workloads for high availability
	PlacementStrategySpread PlacementStrategy = "Spread"
)

// CapabilityRequirement specifies a required cluster capability for placement
type CapabilityRequirement struct {
	// type of capability required (must match ClusterCapability.Type)
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	// +required
	Type string `json:"type"`

	// required indicates if this capability is mandatory
	// +kubebuilder:default=true
	// +required
	Required bool `json:"required"`
}

// WorkloadPlacementStatus represents the observed placement state
type WorkloadPlacementStatus struct {
	// conditions represent the latest available observations of the WorkloadPlacement's current state.
	// +optional
	Conditions conditionsv1alpha1.Conditions `json:"conditions,omitempty"`

	// selectedClusters are the clusters chosen for placement based on the current policy
	// +optional
	// +listType=set
	SelectedClusters []string `json:"selectedClusters,omitempty"`
}

// ClusterRegistrationList contains a list of ClusterRegistration resources
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ClusterRegistrationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:",inline"`

	// Items is the list of ClusterRegistration resources.
	Items []ClusterRegistration `json:"items"`
}

// WorkloadPlacementList contains a list of WorkloadPlacement resources
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type WorkloadPlacementList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:",inline"`

	// Items is the list of WorkloadPlacement resources.
	Items []WorkloadPlacement `json:"items"`
}

// GetConditions returns the conditions for ClusterRegistration
func (cr *ClusterRegistration) GetConditions() conditionsv1alpha1.Conditions {
	return cr.Status.Conditions
}

// SetConditions sets the conditions for ClusterRegistration
func (cr *ClusterRegistration) SetConditions(conditions conditionsv1alpha1.Conditions) {
	cr.Status.Conditions = conditions
}

// GetConditions returns the conditions for WorkloadPlacement
func (wp *WorkloadPlacement) GetConditions() conditionsv1alpha1.Conditions {
	return wp.Status.Conditions
}

// SetConditions sets the conditions for WorkloadPlacement
func (wp *WorkloadPlacement) SetConditions(conditions conditionsv1alpha1.Conditions) {
	wp.Status.Conditions = conditions
}

// Condition types following KCP conventions
const (
	// ClusterRegistrationReady indicates that the cluster registration is ready and healthy
	ClusterRegistrationReady conditionsv1alpha1.ConditionType = "Ready"
	// WorkloadPlacementReady indicates that the workload placement is ready and has selected clusters
	WorkloadPlacementReady conditionsv1alpha1.ConditionType = "Ready"
)