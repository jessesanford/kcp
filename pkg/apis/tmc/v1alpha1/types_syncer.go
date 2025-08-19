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

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories=tmc

// SyncerConfig represents the configuration for a TMC syncer instance.
type SyncerConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SyncerConfigSpec   `json:"spec,omitempty"`
	Status SyncerConfigStatus `json:"status,omitempty"`
}

// SyncerConfigSpec defines the desired state of SyncerConfig.
type SyncerConfigSpec struct {
	// ClusterName is the name of the physical cluster this syncer manages
	ClusterName string `json:"clusterName"`

	// ServerURL is the KCP server URL this syncer connects to
	ServerURL string `json:"serverURL"`

	// SyncMode defines how the syncer operates
	// +kubebuilder:validation:Enum=push;pull;bidirectional
	SyncMode SyncMode `json:"syncMode,omitempty"`

	// Resources defines which resources this syncer should handle
	Resources []SyncerResource `json:"resources,omitempty"`

	// Config contains syncer-specific configuration
	Config *SyncerConfiguration `json:"config,omitempty"`
}

// SyncMode defines the synchronization mode for a syncer.
type SyncMode string

const (
	// SyncModePush means syncer pushes from KCP to physical cluster
	SyncModePush SyncMode = "push"
	// SyncModePull means syncer pulls from physical cluster to KCP
	SyncModePull SyncMode = "pull"
	// SyncModeBidirectional means syncer synchronizes in both directions
	SyncModeBidirectional SyncMode = "bidirectional"
)

// SyncerResource defines a resource that should be synchronized.
type SyncerResource struct {
	// Group is the API group
	Group string `json:"group"`
	// Version is the API version
	Version string `json:"version"`
	// Resource is the resource name
	Resource string `json:"resource"`
}

// SyncerConfiguration contains syncer-specific configuration options.
type SyncerConfiguration struct {
	// QPS is the queries per second limit for API calls
	QPS *float32 `json:"qps,omitempty"`
	
	// Burst is the burst limit for API calls
	Burst *int32 `json:"burst,omitempty"`
	
	// ResyncPeriod defines how often to resync resources
	ResyncPeriod *metav1.Duration `json:"resyncPeriod,omitempty"`
	
	// MaxConcurrentSyncs is the maximum number of concurrent sync operations
	MaxConcurrentSyncs *int32 `json:"maxConcurrentSyncs,omitempty"`
}

// SyncerConfigStatus defines the observed state of SyncerConfig.
type SyncerConfigStatus struct {
	// Phase indicates the current phase of the syncer config
	// +kubebuilder:validation:Enum=Pending;Ready;Failed
	Phase SyncerConfigPhase `json:"phase,omitempty"`

	// Conditions represent the latest available observations
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ActiveSyncers tracks the number of active syncer instances
	ActiveSyncers int32 `json:"activeSyncers,omitempty"`

	// LastSync is the timestamp of the last successful sync
	LastSync *metav1.Time `json:"lastSync,omitempty"`
}

// SyncerConfigPhase represents the phase of a SyncerConfig.
type SyncerConfigPhase string

const (
	// SyncerConfigPhasePending means the config is being processed
	SyncerConfigPhasePending SyncerConfigPhase = "Pending"
	// SyncerConfigPhaseReady means the config is ready and active
	SyncerConfigPhaseReady SyncerConfigPhase = "Ready"
	// SyncerConfigPhaseFailed means the config failed
	SyncerConfigPhaseFailed SyncerConfigPhase = "Failed"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories=tmc

// SyncerTunnel represents a secure tunnel for syncer communication.
type SyncerTunnel struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SyncerTunnelSpec   `json:"spec,omitempty"`
	Status SyncerTunnelStatus `json:"status,omitempty"`
}

// SyncerTunnelSpec defines the desired state of SyncerTunnel.
type SyncerTunnelSpec struct {
	// ServerEndpoint is the KCP server endpoint for the tunnel
	ServerEndpoint string `json:"serverEndpoint"`

	// ClusterEndpoint is the physical cluster endpoint
	ClusterEndpoint string `json:"clusterEndpoint"`

	// TLSConfig contains TLS configuration for the tunnel
	TLSConfig *TunnelTLSConfig `json:"tlsConfig,omitempty"`

	// Authentication contains authentication configuration
	Authentication *TunnelAuthentication `json:"authentication,omitempty"`
}

// TunnelTLSConfig contains TLS configuration for syncer tunnels.
type TunnelTLSConfig struct {
	// Insecure allows insecure connections (for development only)
	Insecure bool `json:"insecure,omitempty"`

	// CABundle contains the CA certificate bundle
	CABundle []byte `json:"caBundle,omitempty"`

	// ServerName is the expected server name for TLS verification
	ServerName string `json:"serverName,omitempty"`
}

// TunnelAuthentication contains authentication configuration for syncer tunnels.
type TunnelAuthentication struct {
	// Type is the authentication type
	// +kubebuilder:validation:Enum=serviceAccount;certificate;token
	Type AuthenticationType `json:"type"`

	// ServiceAccount contains service account authentication details
	ServiceAccount *ServiceAccountAuth `json:"serviceAccount,omitempty"`

	// Certificate contains certificate authentication details
	Certificate *CertificateAuth `json:"certificate,omitempty"`

	// Token contains token authentication details
	Token *TokenAuth `json:"token,omitempty"`
}

// AuthenticationType defines the type of authentication.
type AuthenticationType string

const (
	// AuthenticationTypeServiceAccount uses Kubernetes service account tokens
	AuthenticationTypeServiceAccount AuthenticationType = "serviceAccount"
	// AuthenticationTypeCertificate uses client certificates
	AuthenticationTypeCertificate AuthenticationType = "certificate"
	// AuthenticationTypeToken uses bearer tokens
	AuthenticationTypeToken AuthenticationType = "token"
)

// ServiceAccountAuth contains service account authentication configuration.
type ServiceAccountAuth struct {
	// Name is the service account name
	Name string `json:"name"`
	// Namespace is the service account namespace
	Namespace string `json:"namespace"`
}

// CertificateAuth contains certificate authentication configuration.
type CertificateAuth struct {
	// CertificateData contains the client certificate
	CertificateData []byte `json:"certificateData,omitempty"`
	// KeyData contains the client private key
	KeyData []byte `json:"keyData,omitempty"`
}

// TokenAuth contains token authentication configuration.
type TokenAuth struct {
	// Token is the authentication token
	Token string `json:"token"`
}

// SyncerTunnelStatus defines the observed state of SyncerTunnel.
type SyncerTunnelStatus struct {
	// Phase indicates the current phase of the syncer tunnel
	// +kubebuilder:validation:Enum=Pending;Connected;Disconnected;Failed
	Phase SyncerTunnelPhase `json:"phase,omitempty"`

	// Conditions represent the latest available observations
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ConnectedSince is the timestamp when the tunnel was established
	ConnectedSince *metav1.Time `json:"connectedSince,omitempty"`

	// LastHeartbeat is the timestamp of the last heartbeat
	LastHeartbeat *metav1.Time `json:"lastHeartbeat,omitempty"`
}

// SyncerTunnelPhase represents the phase of a SyncerTunnel.
type SyncerTunnelPhase string

const (
	// SyncerTunnelPhasePending means the tunnel is being established
	SyncerTunnelPhasePending SyncerTunnelPhase = "Pending"
	// SyncerTunnelPhaseConnected means the tunnel is active
	SyncerTunnelPhaseConnected SyncerTunnelPhase = "Connected"
	// SyncerTunnelPhaseDisconnected means the tunnel is disconnected
	SyncerTunnelPhaseDisconnected SyncerTunnelPhase = "Disconnected"
	// SyncerTunnelPhaseFailed means the tunnel failed
	SyncerTunnelPhaseFailed SyncerTunnelPhase = "Failed"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SyncerConfigList contains a list of SyncerConfig.
type SyncerConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SyncerConfig `json:"items"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SyncerTunnelList contains a list of SyncerTunnel.
type SyncerTunnelList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SyncerTunnel `json:"items"`
}