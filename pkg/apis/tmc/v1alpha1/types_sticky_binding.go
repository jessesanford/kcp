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

// StickyBinding represents a persistent binding between a session identifier and a target cluster.
// This API maintains state for session affinity by tracking which sessions are bound to which clusters,
// enabling consistent workload placement across the multi-cluster environment.
//
// +crd
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="Session ID",type=string,JSONPath=`.spec.sessionIdentifier`
// +kubebuilder:printcolumn:name="Target Cluster",type=string,JSONPath=`.spec.targetCluster`
// +kubebuilder:printcolumn:name="Binding Type",type=string,JSONPath=`.spec.bindingType`
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Expires",type=string,JSONPath=`.spec.expiresAt`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type StickyBinding struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec StickyBindingSpec `json:"spec,omitempty"`

	// +optional
	Status StickyBindingStatus `json:"status,omitempty"`
}

// StickyBindingSpec defines the desired state of StickyBinding.
type StickyBindingSpec struct {
	// SessionIdentifier uniquely identifies the session bound to a cluster
	// +kubebuilder:validation:Required
	SessionIdentifier string `json:"sessionIdentifier"`

	// TargetCluster is the cluster this session is bound to
	// +kubebuilder:validation:Required
	TargetCluster string `json:"targetCluster"`

	// BindingType defines how this binding was established
	// +kubebuilder:validation:Enum=ClientIP;Cookie;Header;WorkloadUID;PersistentSession;Manual
	// +kubebuilder:validation:Required
	BindingType SessionAffinityType `json:"bindingType"`

	// WorkloadReference references the workload associated with this binding
	// +optional
	WorkloadReference *ObjectReference `json:"workloadReference,omitempty"`

	// AffinityPolicyReference references the SessionAffinityPolicy that created this binding
	// +optional
	AffinityPolicyReference *ObjectReference `json:"affinityPolicyReference,omitempty"`

	// ExpiresAt defines when this binding should expire
	// +optional
	ExpiresAt *metav1.Time `json:"expiresAt,omitempty"`

	// BindingMetadata contains additional metadata about the binding
	// +optional
	BindingMetadata map[string]string `json:"bindingMetadata,omitempty"`

	// Weight defines the binding strength (higher values = stronger binding)
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:default=50
	// +optional
	Weight int32 `json:"weight,omitempty"`

	// AutoRenewal defines automatic renewal configuration for the binding
	// +optional
	AutoRenewal *BindingAutoRenewal `json:"autoRenewal,omitempty"`

	// ConflictResolution defines how to resolve conflicts with other bindings
	// +optional
	ConflictResolution *BindingConflictResolution `json:"conflictResolution,omitempty"`
}

// BindingAutoRenewal defines automatic renewal configuration for sticky bindings
type BindingAutoRenewal struct {
	// Enabled indicates whether auto-renewal is enabled
	// +kubebuilder:default=true
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// RenewalInterval defines how often to extend the binding
	// +kubebuilder:default="1800s"
	// +optional
	RenewalInterval metav1.Duration `json:"renewalInterval,omitempty"`

	// MaxRenewals defines maximum number of automatic renewals
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:default=24
	// +optional
	MaxRenewals int32 `json:"maxRenewals,omitempty"`

	// RenewalThreshold defines when to start renewal (seconds before expiration)
	// +kubebuilder:default="300s"
	// +optional
	RenewalThreshold metav1.Duration `json:"renewalThreshold,omitempty"`

	// StopOnInactivity stops renewal if session is inactive
	// +kubebuilder:default=true
	// +optional
	StopOnInactivity bool `json:"stopOnInactivity,omitempty"`

	// InactivityThreshold defines inactivity threshold for stopping renewal
	// +kubebuilder:default="3600s"
	// +optional
	InactivityThreshold metav1.Duration `json:"inactivityThreshold,omitempty"`
}

// BindingConflictResolution defines how to resolve conflicts with other bindings
type BindingConflictResolution struct {
	// Strategy defines the conflict resolution strategy
	// +kubebuilder:validation:Enum=HighestWeight;OldestBinding;NewestBinding;Manual
	// +kubebuilder:default="HighestWeight"
	// +optional
	Strategy ConflictResolutionStrategy `json:"strategy,omitempty"`

	// AllowMultipleBindings allows multiple bindings for the same session
	// +kubebuilder:default=false
	// +optional
	AllowMultipleBindings bool `json:"allowMultipleBindings,omitempty"`

	// MaxBindingsPerSession defines maximum bindings allowed per session when multiple bindings are allowed
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=10
	// +kubebuilder:default=1
	// +optional
	MaxBindingsPerSession int32 `json:"maxBindingsPerSession,omitempty"`

	// NotificationPolicy defines how to notify about conflict resolution
	// +optional
	NotificationPolicy *ConflictNotificationPolicy `json:"notificationPolicy,omitempty"`
}

// ConflictResolutionStrategy defines how binding conflicts should be resolved
type ConflictResolutionStrategy string

const (
	// ConflictResolutionStrategyHighestWeight keeps binding with highest weight
	ConflictResolutionStrategyHighestWeight ConflictResolutionStrategy = "HighestWeight"

	// ConflictResolutionStrategyOldestBinding keeps the oldest binding
	ConflictResolutionStrategyOldestBinding ConflictResolutionStrategy = "OldestBinding"

	// ConflictResolutionStrategyNewestBinding keeps the newest binding
	ConflictResolutionStrategyNewestBinding ConflictResolutionStrategy = "NewestBinding"

	// ConflictResolutionStrategyManual requires manual intervention for conflicts
	ConflictResolutionStrategyManual ConflictResolutionStrategy = "Manual"
)

// ConflictNotificationPolicy defines how to notify about conflict resolution
type ConflictNotificationPolicy struct {
	// Enabled indicates whether conflict notifications are enabled
	// +kubebuilder:default=true
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// NotificationMethods defines how to send conflict notifications
	// +optional
	NotificationMethods []ConflictNotificationMethod `json:"notificationMethods,omitempty"`

	// IncludeDetails indicates whether to include detailed conflict information
	// +kubebuilder:default=true
	// +optional
	IncludeDetails bool `json:"includeDetails,omitempty"`
}

// ConflictNotificationMethod defines a method for sending conflict notifications
type ConflictNotificationMethod struct {
	// Type defines the notification method type
	// +kubebuilder:validation:Enum=Event;Log;Webhook;Email
	// +kubebuilder:validation:Required
	Type NotificationMethodType `json:"type"`

	// Configuration defines method-specific configuration
	// +optional
	Configuration map[string]string `json:"configuration,omitempty"`

	// Enabled indicates whether this notification method is enabled
	// +kubebuilder:default=true
	// +optional
	Enabled bool `json:"enabled,omitempty"`
}

// NotificationMethodType defines types of conflict notification methods
type NotificationMethodType string

const (
	// NotificationMethodTypeEvent sends Kubernetes events
	NotificationMethodTypeEvent NotificationMethodType = "Event"

	// NotificationMethodTypeLog writes to controller logs
	NotificationMethodTypeLog NotificationMethodType = "Log"

	// NotificationMethodTypeWebhook sends webhook notifications
	NotificationMethodTypeWebhook NotificationMethodType = "Webhook"

	// NotificationMethodTypeEmail sends email notifications
	NotificationMethodTypeEmail NotificationMethodType = "Email"
)

// StickyBindingStatus represents the observed state of StickyBinding
type StickyBindingStatus struct {
	// Conditions represent the latest available observations of the binding state
	// +optional
	Conditions conditionsv1alpha1.Conditions `json:"conditions,omitempty"`

	// Phase indicates the current binding phase
	// +kubebuilder:default="Active"
	// +optional
	Phase StickyBindingPhase `json:"phase,omitempty"`

	// LastUpdateTime is when the status was last updated
	// +optional
	LastUpdateTime *metav1.Time `json:"lastUpdateTime,omitempty"`

	// LastAccessTime is when the binding was last accessed
	// +optional
	LastAccessTime *metav1.Time `json:"lastAccessTime,omitempty"`

	// AccessCount represents the total number of times this binding was accessed
	AccessCount int64 `json:"accessCount"`

	// RenewalCount represents the number of times this binding was renewed
	RenewalCount int32 `json:"renewalCount"`

	// TargetClusterHealth represents the health of the target cluster
	// +kubebuilder:validation:Enum=Healthy;Degraded;Unhealthy;Unknown;NotChecked
	// +optional
	TargetClusterHealth ClusterHealthStatus `json:"targetClusterHealth,omitempty"`

	// ConflictHistory tracks conflicts involving this binding
	// +optional
	ConflictHistory []BindingConflictEvent `json:"conflictHistory,omitempty"`

	// PerformanceMetrics contains performance-related metrics for this binding
	// +optional
	PerformanceMetrics *BindingPerformanceMetrics `json:"performanceMetrics,omitempty"`

	// Message provides additional information about the binding state
	// +optional
	Message string `json:"message,omitempty"`
}

// StickyBindingPhase represents the current phase of the sticky binding
type StickyBindingPhase string

const (
	// StickyBindingPhaseActive indicates the binding is active and ready for use
	StickyBindingPhaseActive StickyBindingPhase = "Active"

	// StickyBindingPhaseExpiring indicates the binding is about to expire
	StickyBindingPhaseExpiring StickyBindingPhase = "Expiring"

	// StickyBindingPhaseExpired indicates the binding has expired
	StickyBindingPhaseExpired StickyBindingPhase = "Expired"

	// StickyBindingPhaseConflicted indicates the binding has conflicts
	StickyBindingPhaseConflicted StickyBindingPhase = "Conflicted"

	// StickyBindingPhaseDraining indicates the binding is being drained
	StickyBindingPhaseDraining StickyBindingPhase = "Draining"

	// StickyBindingPhaseFailed indicates the binding has failed
	StickyBindingPhaseFailed StickyBindingPhase = "Failed"

	// StickyBindingPhaseUnknown indicates the binding state is unknown
	StickyBindingPhaseUnknown StickyBindingPhase = "Unknown"
)

// ClusterHealthStatus represents the health status of a cluster for binding purposes
type ClusterHealthStatus string

const (
	// ClusterHealthStatusHealthy indicates the cluster is healthy
	ClusterHealthStatusHealthy ClusterHealthStatus = "Healthy"

	// ClusterHealthStatusDegraded indicates the cluster is degraded but functional
	ClusterHealthStatusDegraded ClusterHealthStatus = "Degraded"

	// ClusterHealthStatusUnhealthy indicates the cluster is unhealthy
	ClusterHealthStatusUnhealthy ClusterHealthStatus = "Unhealthy"

	// ClusterHealthStatusUnknown indicates the cluster health is unknown
	ClusterHealthStatusUnknown ClusterHealthStatus = "Unknown"

	// ClusterHealthStatusNotChecked indicates the cluster health is not being checked
	ClusterHealthStatusNotChecked ClusterHealthStatus = "NotChecked"
)

// BindingConflictEvent represents a conflict event involving this binding
type BindingConflictEvent struct {
	// Timestamp is when the conflict occurred
	Timestamp metav1.Time `json:"timestamp"`

	// ConflictType describes the type of conflict
	// +kubebuilder:validation:Enum=DuplicateSession;WeightConflict;PolicyConflict;ResourceConflict
	ConflictType BindingConflictType `json:"conflictType"`

	// ConflictingBinding references the binding that caused the conflict
	// +optional
	ConflictingBinding *ObjectReference `json:"conflictingBinding,omitempty"`

	// Resolution describes how the conflict was resolved
	Resolution string `json:"resolution"`

	// ResolutionStrategy indicates which strategy was used to resolve the conflict
	ResolutionStrategy ConflictResolutionStrategy `json:"resolutionStrategy"`

	// Details provides additional details about the conflict
	// +optional
	Details string `json:"details,omitempty"`
}

// BindingConflictType defines types of binding conflicts
type BindingConflictType string

const (
	// BindingConflictTypeDuplicateSession indicates multiple bindings for same session
	BindingConflictTypeDuplicateSession BindingConflictType = "DuplicateSession"

	// BindingConflictTypeWeightConflict indicates conflicting binding weights
	BindingConflictTypeWeightConflict BindingConflictType = "WeightConflict"

	// BindingConflictTypePolicyConflict indicates conflicting affinity policies
	BindingConflictTypePolicyConflict BindingConflictType = "PolicyConflict"

	// BindingConflictTypeResourceConflict indicates resource availability conflicts
	BindingConflictTypeResourceConflict BindingConflictType = "ResourceConflict"
)

// BindingPerformanceMetrics contains performance-related metrics for sticky bindings
type BindingPerformanceMetrics struct {
	// AverageResponseTime represents average response time for workloads using this binding
	// +optional
	AverageResponseTime *metav1.Duration `json:"averageResponseTime,omitempty"`

	// P95ResponseTime represents 95th percentile response time
	// +optional
	P95ResponseTime *metav1.Duration `json:"p95ResponseTime,omitempty"`

	// SuccessRate represents the success rate for workloads using this binding (0-100)
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	// +optional
	SuccessRate int32 `json:"successRate,omitempty"`

	// ErrorRate represents the error rate for workloads using this binding (0-100)
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	// +optional
	ErrorRate int32 `json:"errorRate,omitempty"`

	// ThroughputPerSecond represents average throughput per second
	// +optional
	ThroughputPerSecond float64 `json:"throughputPerSecond,omitempty"`

	// LastMetricsUpdate is when these metrics were last updated
	// +optional
	LastMetricsUpdate *metav1.Time `json:"lastMetricsUpdate,omitempty"`

	// MetricsCollectionEnabled indicates whether performance metrics collection is enabled
	// +kubebuilder:default=true
	// +optional
	MetricsCollectionEnabled bool `json:"metricsCollectionEnabled,omitempty"`
}

// StickyBindingList is a list of StickyBinding resources
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type StickyBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []StickyBinding `json:"items"`
}

// SessionBindingConstraint defines constraints for session binding operations.
// This API allows operators to define rules and limitations for how session bindings
// can be created, modified, and maintained across the multi-cluster environment.
//
// +crd
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="Constraint Type",type=string,JSONPath=`.spec.constraintType`
// +kubebuilder:printcolumn:name="Target",type=string,JSONPath=`.spec.target.type`
// +kubebuilder:printcolumn:name="Enforcement",type=string,JSONPath=`.spec.enforcement`
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Violations",type=integer,JSONPath=`.status.violationCount`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type SessionBindingConstraint struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec SessionBindingConstraintSpec `json:"spec,omitempty"`

	// +optional
	Status SessionBindingConstraintStatus `json:"status,omitempty"`
}

// SessionBindingConstraintSpec defines the desired state of SessionBindingConstraint.
type SessionBindingConstraintSpec struct {
	// ConstraintType defines the type of constraint to apply
	// +kubebuilder:validation:Enum=MaxBindingsPerCluster;MaxBindingsPerSession;BindingDurationLimit;ClusterAffinityLimit;ResourceUtilizationLimit;GeographicLimit
	// +kubebuilder:validation:Required
	ConstraintType BindingConstraintType `json:"constraintType"`

	// Target defines what the constraint applies to
	// +kubebuilder:validation:Required
	Target ConstraintTarget `json:"target"`

	// Enforcement defines how strictly the constraint should be enforced
	// +kubebuilder:validation:Enum=Strict;Warning;Advisory
	// +kubebuilder:default="Strict"
	// +optional
	Enforcement ConstraintEnforcement `json:"enforcement,omitempty"`

	// Parameters define constraint-specific parameters
	// +optional
	Parameters map[string]string `json:"parameters,omitempty"`

	// MaxViolations defines maximum allowed violations before enforcement action
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:default=0
	// +optional
	MaxViolations int32 `json:"maxViolations,omitempty"`

	// ViolationAction defines action to take when violations exceed threshold
	// +kubebuilder:validation:Enum=Block;Warn;Log;Ignore
	// +kubebuilder:default="Block"
	// +optional
	ViolationAction ViolationAction `json:"violationAction,omitempty"`

	// GracePeriod defines grace period for constraint violations
	// +kubebuilder:default="300s"
	// +optional
	GracePeriod metav1.Duration `json:"gracePeriod,omitempty"`

	// Exemptions define exceptions to the constraint
	// +optional
	Exemptions []ConstraintExemption `json:"exemptions,omitempty"`
}

// BindingConstraintType defines types of binding constraints
type BindingConstraintType string

const (
	// BindingConstraintTypeMaxBindingsPerCluster limits bindings per cluster
	BindingConstraintTypeMaxBindingsPerCluster BindingConstraintType = "MaxBindingsPerCluster"

	// BindingConstraintTypeMaxBindingsPerSession limits bindings per session
	BindingConstraintTypeMaxBindingsPerSession BindingConstraintType = "MaxBindingsPerSession"

	// BindingConstraintTypeBindingDurationLimit limits binding duration
	BindingConstraintTypeBindingDurationLimit BindingConstraintType = "BindingDurationLimit"

	// BindingConstraintTypeClusterAffinityLimit limits affinity to specific clusters
	BindingConstraintTypeClusterAffinityLimit BindingConstraintType = "ClusterAffinityLimit"

	// BindingConstraintTypeResourceUtilizationLimit limits based on resource utilization
	BindingConstraintTypeResourceUtilizationLimit BindingConstraintType = "ResourceUtilizationLimit"

	// BindingConstraintTypeGeographicLimit limits based on geographic location
	BindingConstraintTypeGeographicLimit BindingConstraintType = "GeographicLimit"
)

// ConstraintTarget defines what a constraint applies to
type ConstraintTarget struct {
	// Type defines the target type
	// +kubebuilder:validation:Enum=Cluster;Namespace;WorkloadType;AffinityPolicy;Global
	// +kubebuilder:validation:Required
	Type ConstraintTargetType `json:"type"`

	// Selector defines how to select the targets
	// +optional
	Selector *metav1.LabelSelector `json:"selector,omitempty"`

	// Names explicitly lists target names (alternative to selector)
	// +optional
	Names []string `json:"names,omitempty"`
}

// ConstraintTargetType defines types of constraint targets
type ConstraintTargetType string

const (
	// ConstraintTargetTypeCluster targets specific clusters
	ConstraintTargetTypeCluster ConstraintTargetType = "Cluster"

	// ConstraintTargetTypeNamespace targets specific namespaces
	ConstraintTargetTypeNamespace ConstraintTargetType = "Namespace"

	// ConstraintTargetTypeWorkloadType targets specific workload types
	ConstraintTargetTypeWorkloadType ConstraintTargetType = "WorkloadType"

	// ConstraintTargetTypeAffinityPolicy targets specific affinity policies
	ConstraintTargetTypeAffinityPolicy ConstraintTargetType = "AffinityPolicy"

	// ConstraintTargetTypeGlobal applies globally
	ConstraintTargetTypeGlobal ConstraintTargetType = "Global"
)

// ConstraintEnforcement defines how strictly constraints should be enforced
type ConstraintEnforcement string

const (
	// ConstraintEnforcementStrict strictly enforces constraints
	ConstraintEnforcementStrict ConstraintEnforcement = "Strict"

	// ConstraintEnforcementWarning enforces with warnings
	ConstraintEnforcementWarning ConstraintEnforcement = "Warning"

	// ConstraintEnforcementAdvisory provides advisory guidance only
	ConstraintEnforcementAdvisory ConstraintEnforcement = "Advisory"
)

// ViolationAction defines actions to take on constraint violations
type ViolationAction string

const (
	// ViolationActionBlock blocks operations that violate constraints
	ViolationActionBlock ViolationAction = "Block"

	// ViolationActionWarn allows operations but generates warnings
	ViolationActionWarn ViolationAction = "Warn"

	// ViolationActionLog allows operations but logs violations
	ViolationActionLog ViolationAction = "Log"

	// ViolationActionIgnore ignores constraint violations
	ViolationActionIgnore ViolationAction = "Ignore"
)

// ConstraintExemption defines an exemption from a constraint
type ConstraintExemption struct {
	// Name is a unique name for this exemption
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Reason describes why this exemption exists
	// +kubebuilder:validation:Required
	Reason string `json:"reason"`

	// Target defines what this exemption applies to
	// +kubebuilder:validation:Required
	Target ConstraintTarget `json:"target"`

	// ExpiresAt defines when this exemption expires
	// +optional
	ExpiresAt *metav1.Time `json:"expiresAt,omitempty"`

	// Conditions defines conditions under which this exemption applies
	// +optional
	Conditions []ExemptionCondition `json:"conditions,omitempty"`
}

// ExemptionCondition defines a condition for constraint exemption
type ExemptionCondition struct {
	// Type defines the condition type
	// +kubebuilder:validation:Enum=ClusterHealth;ResourceAvailability;MaintenanceWindow;Emergency
	// +kubebuilder:validation:Required
	Type ExemptionConditionType `json:"type"`

	// Parameters define condition-specific parameters
	// +optional
	Parameters map[string]string `json:"parameters,omitempty"`
}

// ExemptionConditionType defines types of exemption conditions
type ExemptionConditionType string

const (
	// ExemptionConditionTypeClusterHealth exempts based on cluster health
	ExemptionConditionTypeClusterHealth ExemptionConditionType = "ClusterHealth"

	// ExemptionConditionTypeResourceAvailability exempts based on resource availability
	ExemptionConditionTypeResourceAvailability ExemptionConditionType = "ResourceAvailability"

	// ExemptionConditionTypeMaintenanceWindow exempts during maintenance windows
	ExemptionConditionTypeMaintenanceWindow ExemptionConditionType = "MaintenanceWindow"

	// ExemptionConditionTypeEmergency exempts during emergency situations
	ExemptionConditionTypeEmergency ExemptionConditionType = "Emergency"
)

// SessionBindingConstraintStatus represents the observed state of SessionBindingConstraint
type SessionBindingConstraintStatus struct {
	// Conditions represent the latest available observations of the constraint state
	// +optional
	Conditions conditionsv1alpha1.Conditions `json:"conditions,omitempty"`

	// Phase indicates the current constraint phase
	// +kubebuilder:default="Active"
	// +optional
	Phase ConstraintPhase `json:"phase,omitempty"`

	// LastUpdateTime is when the status was last updated
	// +optional
	LastUpdateTime *metav1.Time `json:"lastUpdateTime,omitempty"`

	// ViolationCount represents the current number of constraint violations
	ViolationCount int32 `json:"violationCount"`

	// TotalEvaluations represents the total number of constraint evaluations performed
	TotalEvaluations int64 `json:"totalEvaluations"`

	// RecentViolations contains information about recent violations
	// +optional
	RecentViolations []ConstraintViolation `json:"recentViolations,omitempty"`

	// ExemptionStatus contains status of active exemptions
	// +optional
	ExemptionStatus []ExemptionStatus `json:"exemptionStatus,omitempty"`

	// Message provides additional information about the constraint state
	// +optional
	Message string `json:"message,omitempty"`
}

// ConstraintPhase represents the current phase of the constraint
type ConstraintPhase string

const (
	// ConstraintPhaseActive indicates the constraint is active and enforced
	ConstraintPhaseActive ConstraintPhase = "Active"

	// ConstraintPhaseSuspended indicates the constraint is temporarily suspended
	ConstraintPhaseSuspended ConstraintPhase = "Suspended"

	// ConstraintPhaseFailed indicates the constraint evaluation has failed
	ConstraintPhaseFailed ConstraintPhase = "Failed"

	// ConstraintPhaseUnknown indicates the constraint state is unknown
	ConstraintPhaseUnknown ConstraintPhase = "Unknown"
)

// ConstraintViolation represents a constraint violation event
type ConstraintViolation struct {
	// Timestamp is when the violation occurred
	Timestamp metav1.Time `json:"timestamp"`

	// Target identifies what violated the constraint
	Target string `json:"target"`

	// Description describes the violation
	Description string `json:"description"`

	// Action describes what action was taken
	Action string `json:"action"`

	// Severity indicates the severity of the violation
	// +kubebuilder:validation:Enum=Critical;High;Medium;Low;Info
	Severity ViolationSeverity `json:"severity"`

	// Details provides additional details about the violation
	// +optional
	Details map[string]string `json:"details,omitempty"`
}

// ViolationSeverity defines the severity level of constraint violations
type ViolationSeverity string

const (
	// ViolationSeverityCritical indicates a critical violation
	ViolationSeverityCritical ViolationSeverity = "Critical"

	// ViolationSeverityHigh indicates a high severity violation
	ViolationSeverityHigh ViolationSeverity = "High"

	// ViolationSeverityMedium indicates a medium severity violation
	ViolationSeverityMedium ViolationSeverity = "Medium"

	// ViolationSeverityLow indicates a low severity violation
	ViolationSeverityLow ViolationSeverity = "Low"

	// ViolationSeverityInfo indicates an informational violation
	ViolationSeverityInfo ViolationSeverity = "Info"
)

// ExemptionStatus represents the status of a constraint exemption
type ExemptionStatus struct {
	// Name is the name of the exemption
	Name string `json:"name"`

	// Active indicates whether the exemption is currently active
	Active bool `json:"active"`

	// ActivatedAt is when the exemption was activated
	// +optional
	ActivatedAt *metav1.Time `json:"activatedAt,omitempty"`

	// ExpiresAt is when the exemption expires
	// +optional
	ExpiresAt *metav1.Time `json:"expiresAt,omitempty"`

	// UsageCount represents how many times this exemption has been applied
	UsageCount int32 `json:"usageCount"`

	// LastUsedAt is when this exemption was last used
	// +optional
	LastUsedAt *metav1.Time `json:"lastUsedAt,omitempty"`

	// Message provides additional information about the exemption status
	// +optional
	Message string `json:"message,omitempty"`
}

// SessionBindingConstraintList is a list of SessionBindingConstraint resources
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type SessionBindingConstraintList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []SessionBindingConstraint `json:"items"`
}