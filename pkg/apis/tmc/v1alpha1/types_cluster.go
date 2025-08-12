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

// ClusterRegistration represents a physical cluster registered with the TMC for workload management.
//
// +crd
// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
type ClusterRegistration struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec ClusterRegistrationSpec `json:"spec,omitempty"`

	// +optional
	Status ClusterRegistrationStatus `json:"status,omitempty"`
}

// ClusterRegistrationSpec holds the desired state of the ClusterRegistration.
type ClusterRegistrationSpec struct {
	// Location specifies the geographical location of the cluster
	// +kubebuilder:validation:Required
	Location string `json:"location"`

	// ClusterEndpoint defines how to connect to the cluster
	// +kubebuilder:validation:Required
	ClusterEndpoint ClusterEndpoint `json:"clusterEndpoint"`

	// Capacity defines the advertised capacity of the cluster
	// +optional
	Capacity ClusterCapacity `json:"capacity,omitempty"`
}

// ClusterEndpoint defines connection information for a cluster
type ClusterEndpoint struct {
	// ServerURL is the URL of the Kubernetes API server
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Format=uri
	ServerURL string `json:"serverURL"`

	// CABundle contains the certificate authority bundle for the cluster
	// +optional
	CABundle []byte `json:"caBundle,omitempty"`

	// TLSConfig contains additional TLS configuration
	// +optional
	TLSConfig *TLSConfig `json:"tlsConfig,omitempty"`
}

// TLSConfig contains TLS configuration for cluster connection
type TLSConfig struct {
	// InsecureSkipVerify controls whether to skip certificate verification
	// +kubebuilder:default=false
	// +optional
	InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty"`
}

// ClusterCapacity defines the capacity information for a cluster
type ClusterCapacity struct {
	// CPU is the total CPU capacity of the cluster in milliCPU
	// +optional
	CPU *int64 `json:"cpu,omitempty"`

	// Memory is the total memory capacity of the cluster in bytes
	// +optional
	Memory *int64 `json:"memory,omitempty"`

	// MaxPods is the maximum number of pods that can be scheduled on this cluster
	// +optional
	MaxPods *int32 `json:"maxPods,omitempty"`
}

// ClusterRegistrationStatus communicates the observed state of the ClusterRegistration.
type ClusterRegistrationStatus struct {
	// ObservedGeneration represents the .metadata.generation that the status was set based upon.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions represent the latest available observations of the cluster's current state.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []conditionsv1alpha1.Condition `json:"conditions,omitempty"`
}

// ClusterRegistrationList contains a list of ClusterRegistration
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ClusterRegistrationList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterRegistration `json:"items"`
}