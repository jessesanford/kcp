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
	Location string `json:"location"`

	// ClusterEndpoint defines how to connect to the cluster
	ClusterEndpoint ClusterEndpoint `json:"clusterEndpoint"`
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
}

// ClusterRegistrationStatus communicates the observed state of the ClusterRegistration.
type ClusterRegistrationStatus struct {
	// Conditions represent the latest available observations of the cluster's state
	// +optional
	Conditions conditionsv1alpha1.Conditions `json:"conditions,omitempty"`
}

// ClusterRegistrationList is a list of ClusterRegistration resources
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ClusterRegistrationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ClusterRegistration `json:"items"`
}