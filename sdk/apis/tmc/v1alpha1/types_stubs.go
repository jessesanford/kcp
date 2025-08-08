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

// ClusterRegistration represents a minimal stub for cluster registration.
// This is a placeholder for the placement engine interface.
// Full implementation will be provided in a follow-up PR.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ClusterRegistration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterRegistrationSpec   `json:"spec,omitempty"`
	Status ClusterRegistrationStatus `json:"status,omitempty"`
}

// ClusterRegistrationSpec defines minimal cluster information for placement decisions.
type ClusterRegistrationSpec struct {
	// Location specifies the geographical location of the cluster
	Location string `json:"location,omitempty"`
}

// ClusterRegistrationStatus represents the minimal status needed by placement engine.
type ClusterRegistrationStatus struct {
	// Ready indicates if the cluster is available for placement
	Ready bool `json:"ready,omitempty"`
}

// WorkloadPlacement represents a minimal stub for workload placement requests.
// This is a placeholder for the placement engine interface.
// Full implementation will be provided in a follow-up PR.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type WorkloadPlacement struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WorkloadPlacementSpec   `json:"spec,omitempty"`
	Status WorkloadPlacementStatus `json:"status,omitempty"`
}

// WorkloadPlacementSpec defines minimal placement requirements.
type WorkloadPlacementSpec struct {
	// ClusterSelector specifies criteria for cluster selection
	ClusterSelector ClusterSelector `json:"clusterSelector,omitempty"`
	
	// NumberOfClusters specifies how many clusters to select
	NumberOfClusters *int32 `json:"numberOfClusters,omitempty"`
}

// ClusterSelector defines criteria for selecting target clusters.
type ClusterSelector struct {
	// LabelSelector selects clusters based on labels
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`
	
	// LocationSelector selects clusters in specific locations
	LocationSelector []string `json:"locationSelector,omitempty"`
	
	// ClusterNames explicitly lists cluster names to consider
	ClusterNames []string `json:"clusterNames,omitempty"`
}

// WorkloadPlacementStatus represents minimal placement status.
type WorkloadPlacementStatus struct {
	// Phase indicates the current phase of placement
	Phase string `json:"phase,omitempty"`
}

// ClusterRegistrationList contains a list of ClusterRegistration resources.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ClusterRegistrationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterRegistration `json:"items"`
}

// WorkloadPlacementList contains a list of WorkloadPlacement resources.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type WorkloadPlacementList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WorkloadPlacement `json:"items"`
}