/*
Copyright 2024 The KCP Authors.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
)

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster

// ClusterRegistration represents a registered physical cluster in TMC
type ClusterRegistration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterRegistrationSpec   `json:"spec,omitempty"`
	Status ClusterRegistrationStatus `json:"status,omitempty"`
}

// ClusterRegistrationSpec defines the desired state of ClusterRegistration
type ClusterRegistrationSpec struct {
	Location        string           `json:"location"`
	ClusterEndpoint ClusterEndpoint  `json:"clusterEndpoint"`
	Capacity        ClusterCapacity  `json:"capacity,omitempty"`
}

// ClusterEndpoint contains the endpoint information for accessing the cluster
type ClusterEndpoint struct {
	ServerURL string     `json:"serverURL"`
	TLSConfig *TLSConfig `json:"tlsConfig,omitempty"`
}

// TLSConfig contains TLS configuration
type TLSConfig struct {
	InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty"`
}

// ClusterCapacity defines the capacity of the cluster
type ClusterCapacity struct {
	CPU     *int64 `json:"cpu,omitempty"`
	Memory  *int64 `json:"memory,omitempty"`
	MaxPods *int32 `json:"maxPods,omitempty"`
}

// ClusterRegistrationStatus defines the observed state of ClusterRegistration
type ClusterRegistrationStatus struct {
	ObservedGeneration int64                            `json:"observedGeneration,omitempty"`
	Conditions         []conditionsv1alpha1.Condition  `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterRegistrationList contains a list of ClusterRegistration
type ClusterRegistrationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterRegistration `json:"items"`
}
