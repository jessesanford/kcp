// Copyright The KCP Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClusterRegistration represents a physical Kubernetes cluster registered with TMC.
// This resource follows KCP patterns for workspace-aware cluster management.
//
// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Healthy",type="boolean",JSONPath=".status.healthy"
// +kubebuilder:printcolumn:name="Nodes",type="integer",JSONPath=".status.nodeCount"
// +kubebuilder:printcolumn:name="Version",type="string",JSONPath=".status.version"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type ClusterRegistration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the cluster registration
	Spec ClusterRegistrationSpec `json:"spec,omitempty"`

	// Status defines the observed state of the cluster registration
	Status ClusterRegistrationStatus `json:"status,omitempty"`
}

// ClusterRegistrationSpec defines the desired state of ClusterRegistration
type ClusterRegistrationSpec struct {
	// Location specifies the geographic location or region of the cluster
	Location string `json:"location,omitempty"`

	// KubeconfigSecret references a secret containing the kubeconfig for this cluster
	KubeconfigSecret *ClusterKubeconfigSecret `json:"kubeconfigSecret,omitempty"`

	// Labels are arbitrary key-value pairs for cluster classification
	Labels map[string]string `json:"labels,omitempty"`

	// HealthCheckInterval specifies how often to perform health checks (default 30s)
	// +kubebuilder:default="30s"
	HealthCheckInterval *metav1.Duration `json:"healthCheckInterval,omitempty"`
}

// ClusterKubeconfigSecret references a secret containing cluster credentials
type ClusterKubeconfigSecret struct {
	// Name of the secret containing the kubeconfig
	Name string `json:"name"`

	// Key within the secret containing the kubeconfig data (default "kubeconfig")
	// +kubebuilder:default="kubeconfig"
	Key string `json:"key,omitempty"`
}

// ClusterRegistrationStatus defines the observed state of ClusterRegistration
type ClusterRegistrationStatus struct {
	// Healthy indicates if the cluster passed its last health check
	Healthy bool `json:"healthy"`

	// LastCheck is the time of the last successful health check
	LastCheck *metav1.Time `json:"lastCheck,omitempty"`

	// NodeCount from the latest health check
	NodeCount int `json:"nodeCount,omitempty"`

	// Version of the Kubernetes cluster
	Version string `json:"version,omitempty"`

	// Capacity contains resource capacity information
	Capacity *ClusterCapacity `json:"capacity,omitempty"`

	// Conditions represent the current conditions of the cluster
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Error contains the error message if the cluster is unhealthy
	Error string `json:"error,omitempty"`
}

// ClusterCapacity represents basic resource capacity of a cluster
type ClusterCapacity struct {
	// CPU capacity in millicores
	CPU int64 `json:"cpu"`

	// Memory capacity in bytes
	Memory int64 `json:"memory"`
}

// ClusterRegistrationList contains a list of ClusterRegistration
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ClusterRegistrationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterRegistration `json:"items"`
}