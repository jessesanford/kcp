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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Cluster,categories=kcp
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Location",type="string",JSONPath=`.spec.cells[0].name`
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Syncer",type="string",JSONPath=`.status.syncerIdentity`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=`.metadata.creationTimestamp`

// SyncTarget defines a physical cluster target for workload synchronization.
// It represents a physical cluster that can host workloads in the TMC system,
// providing the foundation for multi-cluster workload placement and management.
type SyncTarget struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the SyncTarget
	Spec SyncTargetSpec `json:"spec"`

	// Status defines the observed state of the SyncTarget
	// +optional
	Status SyncTargetStatus `json:"status,omitempty"`
}

// SyncTargetSpec defines the desired state of a SyncTarget
type SyncTargetSpec struct {
	// Cells defines the cells this SyncTarget supports. At least one cell is required.
	// Cells represent failure domains or locations within the target cluster.
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:Required
	Cells []Cell `json:"cells"`

	// Connection defines how to connect to the target cluster
	// +optional
	Connection *SyncTargetConnection `json:"connection,omitempty"`

	// Credentials defines authentication credentials for the target cluster
	// +optional
	Credentials *SyncTargetCredentials `json:"credentials,omitempty"`

	// Capabilities defines what the target cluster supports
	// +optional
	Capabilities *SyncTargetCapabilities `json:"capabilities,omitempty"`

	// SupportedAPIExports defines which APIs this target can sync.
	// This allows the target to advertise which APIs it supports for workload placement.
	// +optional
	SupportedAPIExports []APIExportReference `json:"supportedAPIExports,omitempty"`

	// Unschedulable marks this SyncTarget as unavailable for new workloads.
	// When true, new workloads will not be scheduled to this target, but existing
	// workloads will continue to run.
	// +optional
	Unschedulable bool `json:"unschedulable,omitempty"`

	// EvictAfter defines when to evict workloads after target becomes unhealthy.
	// This provides a grace period for the target to recover before workloads are moved.
	// +optional
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Format=duration
	EvictAfter *metav1.Duration `json:"evictAfter,omitempty"`
}

// Cell represents a failure domain or location within a SyncTarget.
// Cells provide a way to organize and constrain workload placement within
// a physical cluster based on topology or other characteristics.
type Cell struct {
	// Name is the unique identifier for this cell within the SyncTarget
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Labels provide additional metadata for the cell that can be used
	// for workload placement decisions and constraints
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Taints applied to this cell that affect workload placement.
	// Workloads must tolerate these taints to be scheduled to this cell.
	// +optional
	Taints []Taint `json:"taints,omitempty"`
}

// Taint represents a taint on a cell that affects workload placement
type Taint struct {
	// Key is the taint key to be applied to the cell
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Key string `json:"key"`

	// Value is the taint value corresponding to the taint key
	// +optional
	Value string `json:"value,omitempty"`

	// Effect indicates the taint effect to apply to workloads that do not tolerate the taint.
	// Valid effects are NoSchedule, PreferNoSchedule and NoExecute.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=NoSchedule;PreferNoSchedule;NoExecute
	Effect TaintEffect `json:"effect"`
}

// TaintEffect defines the effect of a taint on workload placement
// +kubebuilder:validation:Enum=NoSchedule;PreferNoSchedule;NoExecute
type TaintEffect string

const (
	// TaintEffectNoSchedule means workloads will not be scheduled to the cell unless they tolerate the taint
	TaintEffectNoSchedule TaintEffect = "NoSchedule"
	// TaintEffectPreferNoSchedule means the scheduler will try to avoid scheduling workloads to the cell
	TaintEffectPreferNoSchedule TaintEffect = "PreferNoSchedule"
	// TaintEffectNoExecute means workloads will be evicted from the cell if they do not tolerate the taint
	TaintEffectNoExecute TaintEffect = "NoExecute"
)

// SyncTargetConnection defines how to connect to the target cluster
type SyncTargetConnection struct {
	// URL is the API server endpoint URL for the target cluster
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Format=uri
	URL string `json:"url"`

	// ServerName is the server name to use for TLS verification
	// +optional
	ServerName string `json:"serverName,omitempty"`

	// CABundle is the base64-encoded CA certificate bundle for TLS verification
	// +optional
	CABundle []byte `json:"caBundle,omitempty"`

	// InsecureSkipTLSVerify skips TLS certificate verification
	// This should only be used for testing
	// +optional
	InsecureSkipTLSVerify bool `json:"insecureSkipTLSVerify,omitempty"`
}

// SyncTargetCredentials defines authentication credentials for the target cluster
type SyncTargetCredentials struct {
	// Type specifies the authentication type (token, certificate, etc.)
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=token;certificate;serviceAccount
	Type SyncTargetAuthType `json:"type"`

	// Token contains bearer token authentication details
	// +optional
	Token *TokenCredentials `json:"token,omitempty"`

	// Certificate contains client certificate authentication details
	// +optional
	Certificate *CertificateCredentials `json:"certificate,omitempty"`

	// ServiceAccount references a service account for authentication
	// +optional
	ServiceAccount *ServiceAccountCredentials `json:"serviceAccount,omitempty"`
}

// SyncTargetAuthType defines the authentication type for SyncTarget
type SyncTargetAuthType string

const (
	// SyncTargetAuthTypeToken uses bearer token authentication
	SyncTargetAuthTypeToken SyncTargetAuthType = "token"
	// SyncTargetAuthTypeCertificate uses client certificate authentication
	SyncTargetAuthTypeCertificate SyncTargetAuthType = "certificate"
	// SyncTargetAuthTypeServiceAccount uses service account authentication
	SyncTargetAuthTypeServiceAccount SyncTargetAuthType = "serviceAccount"
)

// TokenCredentials defines token-based authentication
type TokenCredentials struct {
	// Value is the bearer token value
	// +kubebuilder:validation:Required
	Value string `json:"value"`
}

// CertificateCredentials defines certificate-based authentication
type CertificateCredentials struct {
	// ClientCert is the base64-encoded client certificate
	// +kubebuilder:validation:Required
	ClientCert []byte `json:"clientCert"`

	// ClientKey is the base64-encoded client private key
	// +kubebuilder:validation:Required
	ClientKey []byte `json:"clientKey"`
}

// ServiceAccountCredentials defines service account-based authentication
type ServiceAccountCredentials struct {
	// Namespace is the namespace of the service account
	// +kubebuilder:validation:Required
	Namespace string `json:"namespace"`

	// Name is the name of the service account
	// +kubebuilder:validation:Required
	Name string `json:"name"`
}

// SyncTargetCapabilities defines what the target cluster supports
type SyncTargetCapabilities struct {
	// SupportedResourceTypes lists the resource types this target supports
	// +optional
	SupportedResourceTypes []ResourceTypeSupport `json:"supportedResourceTypes,omitempty"`

	// MaxWorkloads defines the maximum number of workloads this target can handle
	// +optional
	MaxWorkloads *int32 `json:"maxWorkloads,omitempty"`

	// Features lists the features supported by this target
	// +optional
	Features []string `json:"features,omitempty"`
}

// ResourceTypeSupport defines support for a specific resource type
type ResourceTypeSupport struct {
	// Group is the API group of the resource
	// +optional
	Group string `json:"group,omitempty"`

	// Version is the API version of the resource
	// +kubebuilder:validation:Required
	Version string `json:"version"`

	// Kind is the resource kind
	// +kubebuilder:validation:Required
	Kind string `json:"kind"`

	// Supported indicates whether this resource type is supported
	// +optional
	Supported bool `json:"supported,omitempty"`
}

// APIExportReference references an APIExport that this SyncTarget supports
type APIExportReference struct {
	// Workspace is the logical cluster workspace containing the APIExport
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Workspace string `json:"workspace"`

	// Name is the name of the APIExport within the workspace
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}

// SyncTargetStatus defines the observed state of a SyncTarget
type SyncTargetStatus struct {
	// ConnectionState represents the current connection state to the target cluster
	// +optional
	ConnectionState ConnectionState `json:"connectionState,omitempty"`

	// SyncState represents the current synchronization state
	// +optional
	SyncState SyncState `json:"syncState,omitempty"`

	// SyncedResources lists the resources currently being synced to this target
	// +optional
	SyncedResources []SyncedResourceStatus `json:"syncedResources,omitempty"`

	// Health represents the overall health status of the target
	// +optional
	Health *HealthStatus `json:"health,omitempty"`

	// Allocatable resources on this target available for workload placement
	// +optional
	Allocatable ResourceList `json:"allocatable,omitempty"`

	// Capacity defines the total resources available on this target
	// +optional
	Capacity ResourceList `json:"capacity,omitempty"`

	// SyncerIdentity identifies the syncer component managing this target.
	// This is used to track which syncer instance is responsible for synchronization.
	// +optional
	SyncerIdentity string `json:"syncerIdentity,omitempty"`

	// LastHeartbeatTime is when the syncer last sent a heartbeat for this target.
	// This is used to determine if the target is still healthy and reachable.
	// +optional
	LastHeartbeatTime *metav1.Time `json:"lastHeartbeatTime,omitempty"`

	// VirtualWorkspaces contains the virtual workspace URLs through which this target is exposed.
	// These URLs allow clients to interact with resources on this target.
	// +optional
	VirtualWorkspaces []VirtualWorkspace `json:"virtualWorkspaces,omitempty"`

	// Conditions represent the current status conditions of the SyncTarget.
	// Known condition types include Ready and Heartbeat.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// ConnectionState represents the connection state to a target cluster
type ConnectionState string

const (
	// ConnectionStateConnected indicates the target is connected and reachable
	ConnectionStateConnected ConnectionState = "Connected"
	// ConnectionStateDisconnected indicates the target is not reachable
	ConnectionStateDisconnected ConnectionState = "Disconnected"
	// ConnectionStateConnecting indicates a connection is being established
	ConnectionStateConnecting ConnectionState = "Connecting"
	// ConnectionStateError indicates there's a connection error
	ConnectionStateError ConnectionState = "Error"
)

// SyncState represents the synchronization state
type SyncState string

const (
	// SyncStateReady indicates synchronization is active and healthy
	SyncStateReady SyncState = "Ready"
	// SyncStateNotReady indicates synchronization is not ready
	SyncStateNotReady SyncState = "NotReady"
	// SyncStateSyncing indicates synchronization is in progress
	SyncStateSyncing SyncState = "Syncing"
	// SyncStateError indicates synchronization errors
	SyncStateError SyncState = "Error"
)

// SyncedResourceStatus represents the status of a synced resource
type SyncedResourceStatus struct {
	// Group is the API group of the resource
	// +optional
	Group string `json:"group,omitempty"`

	// Version is the API version of the resource
	// +kubebuilder:validation:Required
	Version string `json:"version"`

	// Kind is the resource kind
	// +kubebuilder:validation:Required
	Kind string `json:"kind"`

	// Namespace is the resource namespace (if namespaced)
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Name is the resource name
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// SyncState is the current sync state of this resource
	// +optional
	SyncState SyncState `json:"syncState,omitempty"`

	// LastSyncTime is when this resource was last successfully synced
	// +optional
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	// Error contains any error encountered during sync
	// +optional
	Error string `json:"error,omitempty"`
}

// HealthStatus represents the health of a SyncTarget
type HealthStatus struct {
	// Status is the overall health status
	// +kubebuilder:validation:Enum=Healthy;Degraded;Unhealthy;Unknown
	Status HealthStatusType `json:"status"`

	// LastChecked is when the health was last checked
	// +optional
	LastChecked *metav1.Time `json:"lastChecked,omitempty"`

	// Message provides additional context about the health status
	// +optional
	Message string `json:"message,omitempty"`

	// Checks contains individual health check results
	// +optional
	Checks []HealthCheck `json:"checks,omitempty"`
}

// HealthStatusType represents the health status of a component
type HealthStatusType string

const (
	// HealthStatusHealthy indicates the component is healthy
	HealthStatusHealthy HealthStatusType = "Healthy"
	// HealthStatusDegraded indicates the component is degraded but functional
	HealthStatusDegraded HealthStatusType = "Degraded"
	// HealthStatusUnhealthy indicates the component is unhealthy
	HealthStatusUnhealthy HealthStatusType = "Unhealthy"
	// HealthStatusUnknown indicates the health status is unknown
	HealthStatusUnknown HealthStatusType = "Unknown"
)

// HealthCheck represents an individual health check result
type HealthCheck struct {
	// Name is the name of the health check
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Status is the result of this health check
	// +kubebuilder:validation:Enum=Passed;Failed;Unknown
	Status HealthCheckStatus `json:"status"`

	// Message provides details about the health check result
	// +optional
	Message string `json:"message,omitempty"`

	// LastChecked is when this check was last performed
	// +optional
	LastChecked *metav1.Time `json:"lastChecked,omitempty"`
}

// HealthCheckStatus represents the status of an individual health check
type HealthCheckStatus string

const (
	// HealthCheckStatusPassed indicates the health check passed
	HealthCheckStatusPassed HealthCheckStatus = "Passed"
	// HealthCheckStatusFailed indicates the health check failed
	HealthCheckStatusFailed HealthCheckStatus = "Failed"
	// HealthCheckStatusUnknown indicates the health check status is unknown
	HealthCheckStatusUnknown HealthCheckStatus = "Unknown"
)

// ResourceList is a map of resource name to quantity, representing available resources
type ResourceList map[string]resource.Quantity

// VirtualWorkspace represents a virtual workspace URL for accessing the SyncTarget
type VirtualWorkspace struct {
	// URL is the virtual workspace URL for accessing resources on this SyncTarget
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Format=uri
	URL string `json:"url"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SyncTargetList contains a list of SyncTargets
type SyncTargetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SyncTarget `json:"items"`
}
