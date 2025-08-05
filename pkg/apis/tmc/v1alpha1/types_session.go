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

// WorkloadSessionPolicy represents session management and configuration for workload placement.
//
// +crd
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
type WorkloadSessionPolicy struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec WorkloadSessionPolicySpec `json:"spec,omitempty"`

	// +optional
	Status WorkloadSessionPolicyStatus `json:"status,omitempty"`
}

// WorkloadSessionPolicySpec defines the desired state of WorkloadSessionPolicy.
type WorkloadSessionPolicySpec struct {
	// WorkloadSelector selects the workloads this session policy applies to
	WorkloadSelector WorkloadSelector `json:"workloadSelector"`

	// ClusterSelector defines which clusters to apply session policies
	ClusterSelector ClusterSelector `json:"clusterSelector"`

	// SessionConfig defines session configuration settings
	SessionConfig SessionConfig `json:"sessionConfig"`

	// SessionAffinity defines session affinity settings
	// +optional
	SessionAffinity *SessionAffinity `json:"sessionAffinity,omitempty"`

	// SessionTimeout defines session timeout settings
	// +optional
	SessionTimeout *SessionTimeout `json:"sessionTimeout,omitempty"`
}

// SessionConfig defines session configuration settings
type SessionConfig struct {
	// SessionType defines the type of session management
	// +kubebuilder:validation:Enum=Sticky;RoundRobin;LeastConnections;IPHash;Cookie
	SessionType SessionType `json:"sessionType"`

	// CookieConfig defines cookie-based session configuration
	// +optional
	CookieConfig *SessionCookieConfig `json:"cookieConfig,omitempty"`

	// PersistenceConfig defines session persistence settings
	// +optional
	PersistenceConfig *SessionPersistenceConfig `json:"persistenceConfig,omitempty"`

	// DrainTimeout defines timeout for draining sessions during updates
	// +kubebuilder:default="300s"
	// +optional
	DrainTimeout metav1.Duration `json:"drainTimeout,omitempty"`
}

// SessionType represents session management types
// +kubebuilder:validation:Enum=Sticky;RoundRobin;LeastConnections;IPHash;Cookie
type SessionType string

const (
	// SessionTypeSticky maintains session affinity to specific instances
	SessionTypeSticky SessionType = "Sticky"

	// SessionTypeRoundRobin distributes sessions in round-robin fashion
	SessionTypeRoundRobin SessionType = "RoundRobin"

	// SessionTypeLeastConnections routes to instances with least connections
	SessionTypeLeastConnections SessionType = "LeastConnections"

	// SessionTypeIPHash uses IP hash for session distribution
	SessionTypeIPHash SessionType = "IPHash"

	// SessionTypeCookie uses cookie-based session management
	SessionTypeCookie SessionType = "Cookie"
)

// SessionCookieConfig defines cookie-based session configuration
type SessionCookieConfig struct {
	// Name is the cookie name
	// +kubebuilder:default="TMCSESSIONID"
	// +optional
	Name string `json:"name,omitempty"`

	// Domain defines the cookie domain
	// +optional
	Domain string `json:"domain,omitempty"`

	// Path defines the cookie path
	// +kubebuilder:default="/"
	// +optional
	Path string `json:"path,omitempty"`

	// MaxAge defines cookie max age in seconds
	// +kubebuilder:default=3600
	// +optional
	MaxAge int32 `json:"maxAge,omitempty"`

	// Secure indicates if cookie should be secure
	// +kubebuilder:default=true
	// +optional
	Secure bool `json:"secure,omitempty"`

	// HTTPOnly indicates if cookie should be HTTP only
	// +kubebuilder:default=true
	// +optional
	HTTPOnly bool `json:"httpOnly,omitempty"`

	// SameSite defines cookie SameSite attribute
	// +kubebuilder:validation:Enum=Strict;Lax;None
	// +kubebuilder:default="Lax"
	// +optional
	SameSite SameSitePolicy `json:"sameSite,omitempty"`
}

// SameSitePolicy represents cookie SameSite policies
// +kubebuilder:validation:Enum=Strict;Lax;None
type SameSitePolicy string

const (
	// SameSitePolicyStrict enforces strict same-site policy
	SameSitePolicyStrict SameSitePolicy = "Strict"

	// SameSitePolicyLax enforces lax same-site policy
	SameSitePolicyLax SameSitePolicy = "Lax"

	// SameSitePolicyNone allows cross-site requests
	SameSitePolicyNone SameSitePolicy = "None"
)

// SessionPersistenceConfig defines session persistence settings
type SessionPersistenceConfig struct {
	// Enabled indicates if session persistence is enabled
	// +kubebuilder:default=true
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// BackendType defines the persistence backend type
	// +kubebuilder:validation:Enum=Memory;Redis
	// +kubebuilder:default="Memory"
	// +optional
	BackendType PersistenceBackendType `json:"backendType,omitempty"`

	// ConnectionString defines the backend connection string
	// +optional
	ConnectionString string `json:"connectionString,omitempty"`

	// TTL defines time-to-live for session data
	// +kubebuilder:default="1800s"
	// +optional
	TTL metav1.Duration `json:"ttl,omitempty"`
}

// PersistenceBackendType represents persistence backend types
// +kubebuilder:validation:Enum=Memory;Redis
type PersistenceBackendType string

const (
	// PersistenceBackendTypeMemory uses in-memory persistence
	PersistenceBackendTypeMemory PersistenceBackendType = "Memory"

	// PersistenceBackendTypeRedis uses Redis for persistence
	PersistenceBackendTypeRedis PersistenceBackendType = "Redis"
)

// SessionAffinity defines session affinity settings
type SessionAffinity struct {
	// Type defines the affinity type
	// +kubebuilder:validation:Enum=ClientIP;Cookie;Header;None
	// +kubebuilder:default="ClientIP"
	// +optional
	Type SessionAffinityType `json:"type,omitempty"`

	// TimeoutSeconds defines affinity timeout
	// +kubebuilder:default=3600
	// +optional
	TimeoutSeconds int32 `json:"timeoutSeconds,omitempty"`

	// CookieName defines cookie name for cookie-based affinity
	// +optional
	CookieName string `json:"cookieName,omitempty"`

	// HeaderName defines header name for header-based affinity
	// +optional
	HeaderName string `json:"headerName,omitempty"`
}

// SessionAffinityType represents session affinity types
// +kubebuilder:validation:Enum=ClientIP;Cookie;Header;None
type SessionAffinityType string

const (
	// SessionAffinityTypeClientIP uses client IP for affinity
	SessionAffinityTypeClientIP SessionAffinityType = "ClientIP"

	// SessionAffinityTypeCookie uses cookie for affinity
	SessionAffinityTypeCookie SessionAffinityType = "Cookie"

	// SessionAffinityTypeHeader uses header for affinity
	SessionAffinityTypeHeader SessionAffinityType = "Header"

	// SessionAffinityTypeNone disables session affinity
	SessionAffinityTypeNone SessionAffinityType = "None"
)

// SessionTimeout defines session timeout settings
type SessionTimeout struct {
	// IdleTimeout defines idle timeout for sessions
	// +kubebuilder:default="1800s"
	// +optional
	IdleTimeout metav1.Duration `json:"idleTimeout,omitempty"`

	// MaxLifetime defines maximum session lifetime
	// +kubebuilder:default="86400s"
	// +optional
	MaxLifetime metav1.Duration `json:"maxLifetime,omitempty"`

	// WarningThreshold defines when to warn about expiring sessions
	// +kubebuilder:default="300s"
	// +optional
	WarningThreshold metav1.Duration `json:"warningThreshold,omitempty"`

	// CleanupInterval defines interval for cleaning expired sessions
	// +kubebuilder:default="900s"
	// +optional
	CleanupInterval metav1.Duration `json:"cleanupInterval,omitempty"`
}

// WorkloadSessionPolicyStatus represents the observed state of WorkloadSessionPolicy
type WorkloadSessionPolicyStatus struct {
	// Conditions represent the latest available observations of the session policy state
	// +optional
	Conditions conditionsv1alpha1.Conditions `json:"conditions,omitempty"`

	// Phase indicates the current session policy phase
	// +kubebuilder:default="Active"
	// +optional
	Phase SessionPolicyPhase `json:"phase,omitempty"`

	// LastUpdateTime is when the status was last updated
	// +optional
	LastUpdateTime *metav1.Time `json:"lastUpdateTime,omitempty"`

	// ActiveSessions represents the current number of active sessions
	ActiveSessions int32 `json:"activeSessions"`

	// TotalSessions represents the total number of sessions managed
	TotalSessions int32 `json:"totalSessions"`

	// SessionBackends contains information about session backends
	// +optional
	SessionBackends []SessionBackendStatus `json:"sessionBackends,omitempty"`

	// AffectedWorkloads contains workloads affected by this session policy
	// +optional
	AffectedWorkloads []WorkloadReference `json:"affectedWorkloads,omitempty"`

	// Message provides additional information about the session policy state
	// +optional
	Message string `json:"message,omitempty"`
}

// SessionPolicyPhase represents the current phase of session policy
// +kubebuilder:validation:Enum=Active;Draining;Suspended;Failed;Unknown
type SessionPolicyPhase string

const (
	// SessionPolicyPhaseActive indicates policy is active
	SessionPolicyPhaseActive SessionPolicyPhase = "Active"

	// SessionPolicyPhaseDraining indicates policy is draining sessions
	SessionPolicyPhaseDraining SessionPolicyPhase = "Draining"

	// SessionPolicyPhaseSuspended indicates policy is suspended
	SessionPolicyPhaseSuspended SessionPolicyPhase = "Suspended"

	// SessionPolicyPhaseFailed indicates policy failed
	SessionPolicyPhaseFailed SessionPolicyPhase = "Failed"

	// SessionPolicyPhaseUnknown indicates unknown policy state
	SessionPolicyPhaseUnknown SessionPolicyPhase = "Unknown"
)

// SessionBackendStatus represents the status of a session backend
type SessionBackendStatus struct {
	// Name is the backend name
	Name string `json:"name"`

	// ClusterName is the cluster where the backend is located
	ClusterName string `json:"clusterName"`

	// Status is the backend status
	// +kubebuilder:validation:Enum=Healthy;Unhealthy;Draining;Unknown
	Status SessionBackendStatusType `json:"status"`

	// ActiveSessions is the number of active sessions on this backend
	ActiveSessions int32 `json:"activeSessions"`

	// Weight is the current load balancing weight
	Weight int32 `json:"weight"`

	// LastHealthCheck is when the backend was last health checked
	// +optional
	LastHealthCheck *metav1.Time `json:"lastHealthCheck,omitempty"`

	// HealthCheckFailures is the number of consecutive health check failures
	HealthCheckFailures int32 `json:"healthCheckFailures"`

	// Message provides additional information about the backend status
	// +optional
	Message string `json:"message,omitempty"`
}

// SessionBackendStatusType represents session backend status
// +kubebuilder:validation:Enum=Healthy;Unhealthy;Draining;Unknown
type SessionBackendStatusType string

const (
	// SessionBackendStatusTypeHealthy indicates healthy backend
	SessionBackendStatusTypeHealthy SessionBackendStatusType = "Healthy"

	// SessionBackendStatusTypeUnhealthy indicates unhealthy backend
	SessionBackendStatusTypeUnhealthy SessionBackendStatusType = "Unhealthy"

	// SessionBackendStatusTypeDraining indicates draining backend
	SessionBackendStatusTypeDraining SessionBackendStatusType = "Draining"

	// SessionBackendStatusTypeUnknown indicates unknown backend status
	SessionBackendStatusTypeUnknown SessionBackendStatusType = "Unknown"
)

// WorkloadSessionPolicyList is a list of WorkloadSessionPolicy resources
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type WorkloadSessionPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []WorkloadSessionPolicy `json:"items"`
}
