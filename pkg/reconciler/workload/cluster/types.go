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

package cluster

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClusterRegistration represents a cluster registration in the TMC system.
// For Phase 6 Wave 2, we'll work with a simplified structure that can be
// extended as the full TMC APIs are developed.
type ClusterRegistration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterRegistrationSpec   `json:"spec,omitempty"`
	Status ClusterRegistrationStatus `json:"status,omitempty"`
}

// ClusterRegistrationSpec defines the desired state of ClusterRegistration.
type ClusterRegistrationSpec struct {
	// Location specifies the location of the cluster.
	Location string `json:"location,omitempty"`
	
	// Labels contains metadata labels for the cluster.
	Labels map[string]string `json:"labels,omitempty"`
	
	// Capabilities describes the capabilities of the cluster.
	Capabilities map[string]string `json:"capabilities,omitempty"`
}

// ClusterRegistrationStatus defines the observed state of ClusterRegistration.
type ClusterRegistrationStatus struct {
	// Conditions represents the latest available observations of the cluster registration's state.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	
	// Phase represents the current phase of cluster registration.
	Phase ClusterRegistrationPhase `json:"phase,omitempty"`
	
	// SyncTargetRef references the associated SyncTarget.
	SyncTargetRef *ClusterReference `json:"syncTargetRef,omitempty"`
}

// ClusterRegistrationPhase defines the phase of cluster registration.
type ClusterRegistrationPhase string

const (
	// ClusterRegistrationPhasePending indicates the cluster registration is being processed.
	ClusterRegistrationPhasePending ClusterRegistrationPhase = "Pending"
	
	// ClusterRegistrationPhaseRegistered indicates the cluster is registered.
	ClusterRegistrationPhaseRegistered ClusterRegistrationPhase = "Registered"
	
	// ClusterRegistrationPhaseReady indicates the cluster is ready for workloads.
	ClusterRegistrationPhaseReady ClusterRegistrationPhase = "Ready"
	
	// ClusterRegistrationPhaseFailed indicates the cluster registration failed.
	ClusterRegistrationPhaseFailed ClusterRegistrationPhase = "Failed"
)

// ClusterReference represents a reference to a cluster resource.
type ClusterReference struct {
	// Name is the name of the referenced resource.
	Name string `json:"name"`
	
	// Namespace is the namespace of the referenced resource.
	Namespace string `json:"namespace,omitempty"`
	
	// Cluster is the logical cluster of the referenced resource.
	Cluster string `json:"cluster,omitempty"`
}