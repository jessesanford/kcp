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
	corev1 "k8s.io/api/core/v1"
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

	// Capabilities describes the workload capabilities of this cluster
	// +optional
	Capabilities ClusterCapabilities `json:"capabilities,omitempty"`

	// Credentials reference for authenticating with the cluster
	// +optional
	Credentials *ClusterCredentials `json:"credentials,omitempty"`

	// ResourceQuotas define the resource limits for this cluster
	// +optional
	ResourceQuotas ClusterResourceQuotas `json:"resourceQuotas,omitempty"`
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

// ClusterCapabilities describes the workload capabilities of a cluster
type ClusterCapabilities struct {
	// SupportedWorkloads lists the types of workloads this cluster can run
	// +optional
	SupportedWorkloads []WorkloadCapability `json:"supportedWorkloads,omitempty"`

	// Architecture specifies the CPU architecture (amd64, arm64, etc.)
	// +kubebuilder:validation:Enum=amd64;arm64;s390x;ppc64le
	// +optional
	Architecture string `json:"architecture,omitempty"`

	// Features lists additional cluster features available
	// +optional
	Features []string `json:"features,omitempty"`

	// KubernetesVersion is the Kubernetes version running on the cluster
	// +optional
	KubernetesVersion string `json:"kubernetesVersion,omitempty"`
}

// WorkloadCapability represents a type of workload the cluster supports
type WorkloadCapability struct {
	// Type is the workload type (e.g., "deployment", "statefulset", "daemonset")
	// +kubebuilder:validation:Required
	Type string `json:"type"`

	// APIVersion is the API version for this workload type
	// +kubebuilder:validation:Required
	APIVersion string `json:"apiVersion"`

	// Supported indicates if this workload type is currently supported
	// +optional
	Supported bool `json:"supported,omitempty"`
}

// ClusterCredentials references the credentials needed to access the cluster
type ClusterCredentials struct {
	// SecretRef references a Secret containing cluster credentials
	// +optional
	SecretRef *corev1.LocalObjectReference `json:"secretRef,omitempty"`

	// ServiceAccountRef references a ServiceAccount for cluster access
	// +optional
	ServiceAccountRef *corev1.LocalObjectReference `json:"serviceAccountRef,omitempty"`

	// TokenRef references a token-based credential
	// +optional
	TokenRef *TokenReference `json:"tokenRef,omitempty"`
}

// TokenReference specifies a token-based credential reference
type TokenReference struct {
	// SecretRef references the Secret containing the token
	SecretRef corev1.LocalObjectReference `json:"secretRef"`

	// Key is the key within the Secret that contains the token
	// +kubebuilder:default="token"
	// +optional
	Key string `json:"key,omitempty"`
}

// ClusterResourceQuotas defines resource limits for the cluster
type ClusterResourceQuotas struct {
	// Hard represents the set of desired hard limits for the cluster
	// +optional
	Hard corev1.ResourceList `json:"hard,omitempty"`

	// MaxWorkloads is the maximum number of workloads that can be placed on this cluster
	// +kubebuilder:validation:Minimum=0
	// +optional
	MaxWorkloads *int32 `json:"maxWorkloads,omitempty"`

	// ReservedResources are resources reserved for system use
	// +optional
	ReservedResources corev1.ResourceList `json:"reservedResources,omitempty"`
}

// ClusterRegistrationStatus communicates the observed state of the ClusterRegistration.
type ClusterRegistrationStatus struct {
	// Conditions represent the latest available observations of the cluster's state
	// +optional
	Conditions conditionsv1alpha1.Conditions `json:"conditions,omitempty"`

	// AllocatedResources tracks the currently allocated resources on the cluster
	// +optional
	AllocatedResources corev1.ResourceList `json:"allocatedResources,omitempty"`

	// AvailableResources shows the currently available resources on the cluster
	// +optional
	AvailableResources corev1.ResourceList `json:"availableResources,omitempty"`

	// WorkloadCount is the current number of workloads on the cluster
	// +optional
	WorkloadCount int32 `json:"workloadCount,omitempty"`

	// LastHeartbeat is the timestamp of the last successful health check
	// +optional
	LastHeartbeat *metav1.Time `json:"lastHeartbeat,omitempty"`
}

// ClusterRegistrationList is a list of ClusterRegistration resources
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ClusterRegistrationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ClusterRegistration `json:"items"`
}