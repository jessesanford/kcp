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

// ClusterRegistration represents a physical Kubernetes cluster registered with TMC.
// It tracks cluster health, capabilities, and connectivity for workload placement.
//
// +crd
// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories=kcp
// +kubebuilder:printcolumn:name="Location",type=string,JSONPath=`.spec.location`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type ClusterRegistration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the ClusterRegistration.
	Spec ClusterRegistrationSpec `json:"spec,omitempty"`

	// Status defines the observed state of the ClusterRegistration.
	// +optional
	Status ClusterRegistrationStatus `json:"status,omitempty"`
}

// ClusterRegistrationSpec defines the desired state of ClusterRegistration
type ClusterRegistrationSpec struct {
	// Location is the geographical or logical location of this cluster.
	// This is used for placement decisions and policy enforcement.
	// Examples: "us-west-1", "europe", "edge", "datacenter-1"
	Location string `json:"location"`

	// KubeConfigSecretRef references a Secret containing the kubeconfig
	// needed to connect to this cluster. The Secret must be in the same
	// workspace as the ClusterRegistration.
	// +optional
	KubeConfigSecretRef *SecretReference `json:"kubeConfigSecretRef,omitempty"`

	// Capabilities describes the capabilities and resources available
	// on this cluster for placement decisions.
	// +optional
	Capabilities ClusterCapabilities `json:"capabilities,omitempty"`

	// AcceptedWorkloadTypes specifies what types of workloads this cluster
	// can accept. If empty, all workload types are accepted.
	// +optional
	AcceptedWorkloadTypes []WorkloadType `json:"acceptedWorkloadTypes,omitempty"`
}

// ClusterRegistrationStatus defines the observed state of ClusterRegistration
type ClusterRegistrationStatus struct {
	// Phase represents the current phase of cluster registration.
	// +optional
	Phase ClusterPhase `json:"phase,omitempty"`

	// Conditions represent the latest available observations of the
	// cluster's current state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// LastHeartbeat is the last time the cluster reported its health status.
	// +optional
	LastHeartbeat *metav1.Time `json:"lastHeartbeat,omitempty"`

	// ClusterInfo contains information about the cluster's Kubernetes version
	// and other relevant details.
	// +optional
	ClusterInfo *ClusterInfo `json:"clusterInfo,omitempty"`

	// ResourceSummary provides a summary of available resources on this cluster.
	// +optional
	ResourceSummary *ResourceSummary `json:"resourceSummary,omitempty"`
}

// ClusterCapabilities describes what a cluster can provide
type ClusterCapabilities struct {
	// HasLoadBalancer indicates if the cluster supports LoadBalancer services.
	// +optional
	HasLoadBalancer bool `json:"hasLoadBalancer,omitempty"`

	// HasPersistentStorage indicates if the cluster supports persistent volumes.
	// +optional
	HasPersistentStorage bool `json:"hasPersistentStorage,omitempty"`

	// SupportedStorageClasses lists the available storage classes.
	// +optional
	SupportedStorageClasses []string `json:"supportedStorageClasses,omitempty"`

	// NetworkPolicies indicates if the cluster supports NetworkPolicies.
	// +optional
	NetworkPolicies bool `json:"networkPolicies,omitempty"`
}

// ClusterInfo contains information about the cluster
type ClusterInfo struct {
	// KubernetesVersion is the version of Kubernetes running on the cluster.
	// +optional
	KubernetesVersion string `json:"kubernetesVersion,omitempty"`

	// Platform describes the platform/provider of the cluster.
	// Examples: "eks", "gke", "aks", "kind", "k3s"
	// +optional
	Platform string `json:"platform,omitempty"`

	// NodeCount is the number of nodes in the cluster.
	// +optional
	NodeCount int32 `json:"nodeCount,omitempty"`
}

// ResourceSummary provides a summary of cluster resources
type ResourceSummary struct {
	// AvailableCPU is the total available CPU in the cluster.
	// +optional
	AvailableCPU string `json:"availableCPU,omitempty"`

	// AvailableMemory is the total available memory in the cluster.
	// +optional
	AvailableMemory string `json:"availableMemory,omitempty"`

	// AvailableStorage is the total available storage in the cluster.
	// +optional
	AvailableStorage string `json:"availableStorage,omitempty"`
}

// ClusterPhase represents the phase of cluster registration
type ClusterPhase string

const (
	// ClusterPhasePending indicates the cluster registration is pending.
	ClusterPhasePending ClusterPhase = "Pending"
	// ClusterPhaseReady indicates the cluster is ready to accept workloads.
	ClusterPhaseReady ClusterPhase = "Ready"
	// ClusterPhaseNotReady indicates the cluster is not ready.
	ClusterPhaseNotReady ClusterPhase = "NotReady"
	// ClusterPhaseOffline indicates the cluster is offline or unreachable.
	ClusterPhaseOffline ClusterPhase = "Offline"
)

// Condition types for ClusterRegistration
const (
	// ClusterRegistrationReady indicates that the cluster is ready to accept workloads.
	ClusterRegistrationReady = "Ready"
	// ClusterRegistrationConnectable indicates that the cluster can be connected to.
	ClusterRegistrationConnectable = "Connectable"
	// ClusterRegistrationHealthy indicates that the cluster is healthy.
	ClusterRegistrationHealthy = "Healthy"
)

// SecretReference references a secret in the same namespace
type SecretReference struct {
	// Name is the name of the secret.
	Name string `json:"name"`

	// Key is the key in the secret that contains the kubeconfig.
	// Defaults to "kubeconfig" if not specified.
	// +optional
	Key string `json:"key,omitempty"`
}

// WorkloadType represents a Kubernetes workload type
type WorkloadType struct {
	// APIVersion is the API version of the workload type.
	APIVersion string `json:"apiVersion"`

	// Kind is the kind of the workload type.
	Kind string `json:"kind"`
}

// ClusterRegistrationList contains a list of ClusterRegistration
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ClusterRegistrationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterRegistration `json:"items"`
}