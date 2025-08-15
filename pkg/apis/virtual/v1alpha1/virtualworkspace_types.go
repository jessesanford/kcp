package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories=kcp
// +kubebuilder:printcolumn:name="URL",type=string,JSONPath=`.status.url`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// VirtualWorkspace defines a virtual workspace configuration
type VirtualWorkspace struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VirtualWorkspaceSpec   `json:"spec,omitempty"`
	Status VirtualWorkspaceStatus `json:"status,omitempty"`
}

// VirtualWorkspaceSpec defines the desired state of VirtualWorkspace
type VirtualWorkspaceSpec struct {
	// URL is the base URL for this virtual workspace
	URL string `json:"url"`

	// APIExportReference references the APIExport to serve
	// +optional
	APIExportReference *corev1.LocalObjectReference `json:"apiExportReference,omitempty"`

	// Authentication configures authentication for the workspace
	// +optional
	Authentication *AuthenticationConfig `json:"authentication,omitempty"`

	// RateLimiting configures rate limiting
	// +optional
	RateLimiting *RateLimitConfig `json:"rateLimiting,omitempty"`

	// Caching configures caching behavior
	// +optional
	Caching *CacheConfig `json:"caching,omitempty"`
}

// AuthenticationConfig defines authentication settings
type AuthenticationConfig struct {
	// Type specifies the authentication type
	Type AuthenticationType `json:"type"`

	// CertificateAuthorityData contains PEM-encoded CA certificates
	// +optional
	CertificateAuthorityData []byte `json:"certificateAuthorityData,omitempty"`

	// ClientCertificateData contains PEM-encoded client certificate
	// +optional
	ClientCertificateData []byte `json:"clientCertificateData,omitempty"`
}

// AuthenticationType specifies the type of authentication
type AuthenticationType string

const (
	AuthenticationTypeNone        AuthenticationType = "None"
	AuthenticationTypeCertificate AuthenticationType = "Certificate"
	AuthenticationTypeToken       AuthenticationType = "Token"
	AuthenticationTypeOIDC        AuthenticationType = "OIDC"
)

// RateLimitConfig defines rate limiting settings
type RateLimitConfig struct {
	// QPS is queries per second allowed
	QPS int32 `json:"qps"`

	// Burst is the burst size
	Burst int32 `json:"burst"`

	// PerUserLimits enables per-user rate limiting
	// +optional
	PerUserLimits bool `json:"perUserLimits,omitempty"`
}

// CacheConfig defines caching settings
type CacheConfig struct {
	// TTLSeconds is the cache TTL in seconds
	TTLSeconds int32 `json:"ttlSeconds"`

	// MaxSize is the maximum cache size in MB
	// +optional
	MaxSize int32 `json:"maxSize,omitempty"`
}

// VirtualWorkspaceStatus defines the observed state of VirtualWorkspace
type VirtualWorkspaceStatus struct {
	// URL is the actual URL for accessing the workspace
	// +optional
	URL string `json:"url,omitempty"`

	// Phase indicates the current state
	// +optional
	Phase VirtualWorkspacePhase `json:"phase,omitempty"`

	// Conditions represent the latest available observations
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ConnectedClients is the current number of connected clients
	// +optional
	ConnectedClients int32 `json:"connectedClients,omitempty"`
}

// VirtualWorkspacePhase represents the phase of a VirtualWorkspace
type VirtualWorkspacePhase string

const (
	VirtualWorkspacePhasePending      VirtualWorkspacePhase = "Pending"
	VirtualWorkspacePhaseInitializing VirtualWorkspacePhase = "Initializing"
	VirtualWorkspacePhaseReady        VirtualWorkspacePhase = "Ready"
	VirtualWorkspacePhaseTerminating  VirtualWorkspacePhase = "Terminating"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VirtualWorkspaceList contains a list of VirtualWorkspace
type VirtualWorkspaceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VirtualWorkspace `json:"items"`
}