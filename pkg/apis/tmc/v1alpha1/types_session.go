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

// +crd
// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories=tmc
// +kubebuilder:printcolumn:name="Workspace",type="string",JSONPath=".spec.workspace",description="Target workspace"
// +kubebuilder:printcolumn:name="Session Type",type="string",JSONPath=".spec.sessionType",description="Type of session"
// +kubebuilder:printcolumn:name="Active Sessions",type="integer",JSONPath=".status.activeSessions",description="Number of active sessions"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// WorkspaceSession manages session state and client interactions within a specific KCP workspace.
// It provides workspace-aware session management enabling sticky routing and client tracking
// for multi-tenant TMC operations with proper workspace isolation.
type WorkspaceSession struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the WorkspaceSession.
	Spec WorkspaceSessionSpec `json:"spec,omitempty"`

	// Status defines the observed state of the WorkspaceSession.
	Status WorkspaceSessionStatus `json:"status,omitempty"`
}

// WorkspaceSessionSpec defines the desired workspace session configuration.
type WorkspaceSessionSpec struct {
	// Workspace specifies the logical cluster workspace this session applies to.
	// +kubebuilder:validation:Required
	Workspace string `json:"workspace"`

	// SessionType defines the type of workspace session management.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=Interactive;Batch;Service
	SessionType WorkspaceSessionType `json:"sessionType"`

	// MaxSessions limits concurrent sessions for this workspace.
	// +kubebuilder:default=50
	// +optional
	MaxSessions *int32 `json:"maxSessions,omitempty"`

	// SessionTimeout defines session timeout duration.
	// +kubebuilder:default="30m"
	// +optional
	SessionTimeout *metav1.Duration `json:"sessionTimeout,omitempty"`

	// IdleTimeout defines idle session timeout.
	// +kubebuilder:default="15m"
	// +optional
	IdleTimeout *metav1.Duration `json:"idleTimeout,omitempty"`
}

// WorkspaceSessionType defines the type of workspace session.
type WorkspaceSessionType string

const (
	// WorkspaceSessionTypeInteractive is for interactive client sessions.
	WorkspaceSessionTypeInteractive WorkspaceSessionType = "Interactive"

	// WorkspaceSessionTypeBatch is for batch processing sessions.
	WorkspaceSessionTypeBatch WorkspaceSessionType = "Batch"

	// WorkspaceSessionTypeService is for service-to-service sessions.
	WorkspaceSessionTypeService WorkspaceSessionType = "Service"
)

// WorkspaceSessionStatus defines the observed state of the WorkspaceSession.
type WorkspaceSessionStatus struct {
	// Conditions represent the latest available observations of the session's current state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ActiveSessions shows the number of currently active sessions.
	// +optional
	ActiveSessions int32 `json:"activeSessions,omitempty"`

	// ActiveClients shows the number of currently connected clients.
	// +optional
	ActiveClients int32 `json:"activeClients,omitempty"`

	// LastActivity indicates the last activity timestamp.
	// +optional
	LastActivity *metav1.Time `json:"lastActivity,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// WorkspaceSessionList contains a list of WorkspaceSession objects.
type WorkspaceSessionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WorkspaceSession `json:"items"`
}

// +crd
// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories=tmc
// +kubebuilder:printcolumn:name="Affinity Method",type="string",JSONPath=".spec.affinityMethod",description="Method used for session affinity"
// +kubebuilder:printcolumn:name="Max Sessions",type="integer",JSONPath=".spec.maxSessions",description="Maximum concurrent sessions"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// SessionPolicy defines global session affinity and management policies for TMC workload operations.
// It provides centralized control over session behavior across workspace boundaries.
type SessionPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the SessionPolicy.
	Spec SessionPolicySpec `json:"spec,omitempty"`

	// Status defines the observed state of the SessionPolicy.
	Status SessionPolicyStatus `json:"status,omitempty"`
}

// SessionPolicySpec defines the desired session management policy configuration.
type SessionPolicySpec struct {
	// AffinityMethod defines how session affinity is implemented.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=ClientIP;Cookie;Header;WorkspaceAware
	AffinityMethod SessionAffinityMethod `json:"affinityMethod"`

	// MaxSessions limits concurrent sessions per policy scope.
	// +kubebuilder:default=1000
	// +optional
	MaxSessions *int32 `json:"maxSessions,omitempty"`

	// SessionTimeout defines session timeout duration.
	// +kubebuilder:default="30m"
	// +optional
	SessionTimeout *metav1.Duration `json:"sessionTimeout,omitempty"`

	// WorkspaceSelector selects workspaces this policy applies to.
	// +optional
	WorkspaceSelector *metav1.LabelSelector `json:"workspaceSelector,omitempty"`

	// CookieSettings provides cookie configuration when using Cookie affinity method.
	// +optional
	CookieSettings *SessionCookieSettings `json:"cookieSettings,omitempty"`

	// HeaderSettings provides header configuration when using Header affinity method.
	// +optional
	HeaderSettings *SessionHeaderSettings `json:"headerSettings,omitempty"`
}

// SessionAffinityMethod defines the method used for session affinity.
type SessionAffinityMethod string

const (
	// SessionAffinityMethodClientIP uses client IP address for affinity.
	SessionAffinityMethodClientIP SessionAffinityMethod = "ClientIP"

	// SessionAffinityMethodCookie uses HTTP cookies for session tracking.
	SessionAffinityMethodCookie SessionAffinityMethod = "Cookie"

	// SessionAffinityMethodHeader uses HTTP headers for session identification.
	SessionAffinityMethodHeader SessionAffinityMethod = "Header"

	// SessionAffinityMethodWorkspaceAware combines workspace context with client identification.
	SessionAffinityMethodWorkspaceAware SessionAffinityMethod = "WorkspaceAware"
)

// SessionCookieSettings defines cookie-based session settings.
type SessionCookieSettings struct {
	// CookieName is the name of the session cookie.
	// +kubebuilder:default="TMC-Session-ID"
	CookieName string `json:"cookieName,omitempty"`

	// CookiePath is the path for the session cookie.
	// +kubebuilder:default="/"
	CookiePath string `json:"cookiePath,omitempty"`

	// SecureCookie enables secure flag for cookies.
	// +kubebuilder:default=true
	SecureCookie *bool `json:"secureCookie,omitempty"`

	// HTTPOnlyCookie enables HttpOnly flag for cookies.
	// +kubebuilder:default=true
	HTTPOnlyCookie *bool `json:"httpOnlyCookie,omitempty"`
}

// SessionHeaderSettings defines header-based session settings.
type SessionHeaderSettings struct {
	// HeaderName is the name of the session header.
	// +kubebuilder:default="X-TMC-Session-ID"
	HeaderName string `json:"headerName,omitempty"`

	// HeaderPrefix is the prefix for session header values.
	// +optional
	HeaderPrefix string `json:"headerPrefix,omitempty"`
}

// SessionPolicyStatus defines the observed state of the SessionPolicy.
type SessionPolicyStatus struct {
	// Conditions represent the latest available observations of the policy's current state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ActiveSessions shows current active sessions under this policy.
	// +optional
	ActiveSessions int32 `json:"activeSessions,omitempty"`

	// AppliedWorkspaces shows which workspaces are currently using this policy.
	// +optional
	AppliedWorkspaces []string `json:"appliedWorkspaces,omitempty"`

	// LastUpdated indicates when the policy status was last updated.
	// +optional
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SessionPolicyList contains a list of SessionPolicy objects.
type SessionPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SessionPolicy `json:"items"`
}

// SessionConditionType represents a condition type for session resources.
type SessionConditionType string

const (
	// SessionConditionReady indicates whether the session resource is ready.
	SessionConditionReady SessionConditionType = "Ready"

	// SessionConditionActive indicates whether the session resource is actively handling requests.
	SessionConditionActive SessionConditionType = "Active"

	// SessionConditionPolicyApplied indicates whether session policies are applied.
	SessionConditionPolicyApplied SessionConditionType = "PolicyApplied"
)