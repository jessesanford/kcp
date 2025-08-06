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

// StickyBinding represents a session-to-cluster binding that provides persistence
// and management for session affinity. It tracks the binding of sessions to specific
// clusters to ensure workload placement consistency across the multi-cluster environment.
//
// This resource is workspace-aware and supports KCP logical cluster isolation to ensure
// proper multi-tenancy in KCP environments.
//
// +crd
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={tmc,all}
// +kubebuilder:metadata:annotations="kcp.io/cluster-aware=true"
// +kubebuilder:validation:XValidation:rule="self.metadata.annotations['kcp.io/cluster'] != ''"
// +kubebuilder:printcolumn:name="Session ID",type=string,JSONPath=`.spec.sessionID`
// +kubebuilder:printcolumn:name="Target Cluster",type=string,JSONPath=`.spec.targetCluster`
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Expires",type=date,JSONPath=`.spec.expiresAt`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:rbac:groups=tmc.kcp.io,resources=stickybindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=tmc.kcp.io,resources=stickybindings/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=tmc.kcp.io,resources=stickybindings/finalizers,verbs=update
type StickyBinding struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec StickyBindingSpec `json:"spec,omitempty"`

	// +optional
	Status StickyBindingStatus `json:"status,omitempty"`
}

// StickyBindingSpec defines the desired state of StickyBinding
type StickyBindingSpec struct {
	// SessionID uniquely identifies the session this binding belongs to
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=128
	SessionID string `json:"sessionID"`

	// TargetCluster is the cluster this session is bound to
	// +kubebuilder:validation:Required
	TargetCluster string `json:"targetCluster"`

	// WorkloadReference refers to the workload associated with this binding
	// +kubebuilder:validation:Required
	WorkloadReference ObjectReference `json:"workloadReference"`

	// AffinityPolicy reference to the SessionAffinityPolicy that created this binding
	// +kubebuilder:validation:Required
	AffinityPolicyRef ObjectReference `json:"affinityPolicyRef"`

	// ExpiresAt defines when this binding should expire
	// +kubebuilder:validation:Required
	ExpiresAt metav1.Time `json:"expiresAt"`

	// AutoRenewal configuration for automatic binding renewal
	// +optional
	AutoRenewal *BindingAutoRenewal `json:"autoRenewal,omitempty"`

	// StorageBackend specifies where binding data should be persisted
	// +kubebuilder:validation:Required
	StorageBackend BindingStorageBackend `json:"storageBackend"`

	// ConflictResolution defines how to handle binding conflicts
	// +optional
	ConflictResolution *BindingConflictResolution `json:"conflictResolution,omitempty"`

	// Weight defines the priority of this binding for conflict resolution
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=1000
	// +kubebuilder:default=100
	// +optional
	Weight int32 `json:"weight,omitempty"`

	// Tags provide additional metadata for binding organization and management
	// +optional
	Tags map[string]string `json:"tags,omitempty"`
}

// BindingAutoRenewal configures automatic renewal of sticky bindings
type BindingAutoRenewal struct {
	// Enabled indicates whether auto-renewal is active
	// +kubebuilder:default=true
	Enabled bool `json:"enabled"`

	// RenewalInterval defines how often to check for renewal
	// +kubebuilder:default="300s"
	// +optional
	RenewalInterval metav1.Duration `json:"renewalInterval,omitempty"`

	// RenewalThreshold defines when to trigger renewal before expiration
	// +kubebuilder:default="600s"  
	// +optional
	RenewalThreshold metav1.Duration `json:"renewalThreshold,omitempty"`

	// MaxRenewalAttempts defines maximum renewal attempts before giving up
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=10
	// +kubebuilder:default=3
	// +optional
	MaxRenewalAttempts int32 `json:"maxRenewalAttempts,omitempty"`

	// ExtensionDuration defines how much to extend the binding on each renewal
	// +kubebuilder:default="3600s"
	// +optional
	ExtensionDuration metav1.Duration `json:"extensionDuration,omitempty"`
}

// BindingStorageBackend configures where binding persistence is stored
type BindingStorageBackend struct {
	// Type defines the storage backend type
	// +kubebuilder:validation:Enum=Memory;ConfigMap;Secret;CustomResource;External
	// +kubebuilder:validation:Required
	Type StorageBackendType `json:"type"`

	// ConfigMapRef references a ConfigMap for storage (when type is ConfigMap)
	// +optional
	ConfigMapRef *corev1.LocalObjectReference `json:"configMapRef,omitempty"`

	// SecretRef references a Secret for storage (when type is Secret)
	// +optional
	SecretRef *corev1.LocalObjectReference `json:"secretRef,omitempty"`

	// ExternalConfig configures external storage systems
	// +optional
	ExternalConfig *ExternalStorageConfig `json:"externalConfig,omitempty"`

	// Encryption settings for persisted binding data
	// +optional
	Encryption *StorageEncryption `json:"encryption,omitempty"`
}

// StorageBackendType defines the supported storage backend types
type StorageBackendType string

const (
	// StorageBackendTypeMemory stores bindings in memory only (ephemeral)
	StorageBackendTypeMemory StorageBackendType = "Memory"

	// StorageBackendTypeConfigMap stores bindings in Kubernetes ConfigMaps
	StorageBackendTypeConfigMap StorageBackendType = "ConfigMap"

	// StorageBackendTypeSecret stores bindings in Kubernetes Secrets
	StorageBackendTypeSecret StorageBackendType = "Secret"

	// StorageBackendTypeCustomResource stores bindings as custom Kubernetes resources
	StorageBackendTypeCustomResource StorageBackendType = "CustomResource"

	// StorageBackendTypeExternal stores bindings in external systems (Redis, etcd, etc.)
	StorageBackendTypeExternal StorageBackendType = "External"
)

// ExternalStorageConfig configures external storage systems
type ExternalStorageConfig struct {
	// URL is the endpoint URL for the external storage system
	// +kubebuilder:validation:Required
	URL string `json:"url"`

	// AuthSecretRef references a secret containing authentication credentials
	// +optional
	AuthSecretRef *corev1.LocalObjectReference `json:"authSecretRef,omitempty"`

	// ConnectionTimeout defines timeout for storage operations
	// +kubebuilder:default="30s"
	// +optional
	ConnectionTimeout metav1.Duration `json:"connectionTimeout,omitempty"`

	// TLS configuration for secure connections
	// +optional
	TLS *ExternalStorageTLS `json:"tls,omitempty"`
}

// ExternalStorageTLS configures TLS for external storage connections
type ExternalStorageTLS struct {
	// Enabled indicates whether TLS should be used
	// +kubebuilder:default=true
	Enabled bool `json:"enabled"`

	// CASecretRef references a secret containing the CA certificate
	// +optional
	CASecretRef *corev1.LocalObjectReference `json:"caSecretRef,omitempty"`

	// ClientCertSecretRef references a secret containing client certificates
	// +optional
	ClientCertSecretRef *corev1.LocalObjectReference `json:"clientCertSecretRef,omitempty"`

	// InsecureSkipVerify skips certificate verification (not recommended for production)
	// +kubebuilder:default=false
	// +optional
	InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty"`
}

// StorageEncryption configures encryption for stored binding data
type StorageEncryption struct {
	// Enabled indicates whether encryption should be applied
	// +kubebuilder:default=false
	Enabled bool `json:"enabled"`

	// Algorithm defines the encryption algorithm to use
	// +kubebuilder:validation:Enum=AES256;ChaCha20Poly1305
	// +kubebuilder:default="AES256"
	// +optional
	Algorithm EncryptionAlgorithm `json:"algorithm,omitempty"`

	// KeySecretRef references a secret containing the encryption key
	// +optional
	KeySecretRef *corev1.LocalObjectReference `json:"keySecretRef,omitempty"`
}

// EncryptionAlgorithm defines supported encryption algorithms
type EncryptionAlgorithm string

const (
	// EncryptionAlgorithmAES256 uses AES-256 encryption
	EncryptionAlgorithmAES256 EncryptionAlgorithm = "AES256"

	// EncryptionAlgorithmChaCha20Poly1305 uses ChaCha20-Poly1305 encryption
	EncryptionAlgorithmChaCha20Poly1305 EncryptionAlgorithm = "ChaCha20Poly1305"
)

// BindingConflictResolution defines how to handle conflicts between bindings
type BindingConflictResolution struct {
	// Strategy defines the conflict resolution strategy
	// +kubebuilder:validation:Enum=HighestWeight;NewestBinding;OldestBinding;Manual
	// +kubebuilder:default="HighestWeight"
	Strategy ConflictResolutionStrategy `json:"strategy"`

	// ManualApprovalRequired indicates if manual approval is needed for conflict resolution
	// +kubebuilder:default=false
	// +optional
	ManualApprovalRequired bool `json:"manualApprovalRequired,omitempty"`

	// ConflictTimeout defines how long to wait for conflict resolution
	// +kubebuilder:default="300s"
	// +optional
	ConflictTimeout metav1.Duration `json:"conflictTimeout,omitempty"`
}

// ConflictResolutionStrategy defines how to resolve binding conflicts
type ConflictResolutionStrategy string

const (
	// ConflictResolutionStrategyHighestWeight resolves conflicts by preferring the highest weight binding
	ConflictResolutionStrategyHighestWeight ConflictResolutionStrategy = "HighestWeight"

	// ConflictResolutionStrategyNewestBinding resolves conflicts by preferring the newest binding
	ConflictResolutionStrategyNewestBinding ConflictResolutionStrategy = "NewestBinding"

	// ConflictResolutionStrategyOldestBinding resolves conflicts by preferring the oldest binding
	ConflictResolutionStrategyOldestBinding ConflictResolutionStrategy = "OldestBinding"

	// ConflictResolutionStrategyManual requires manual intervention to resolve conflicts
	ConflictResolutionStrategyManual ConflictResolutionStrategy = "Manual"
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

	// LastRenewalTime is when the binding was last renewed
	// +optional
	LastRenewalTime *metav1.Time `json:"lastRenewalTime,omitempty"`

	// NextRenewalTime is when the next renewal attempt will occur
	// +optional
	NextRenewalTime *metav1.Time `json:"nextRenewalTime,omitempty"`

	// RenewalAttempts tracks the number of renewal attempts made
	RenewalAttempts int32 `json:"renewalAttempts"`

	// ConflictStatus tracks any conflicts with this binding
	// +optional
	ConflictStatus *BindingConflictStatus `json:"conflictStatus,omitempty"`

	// StorageStatus tracks the persistence status of this binding
	// +optional
	StorageStatus *StorageStatus `json:"storageStatus,omitempty"`

	// Performance metrics for this binding
	// +optional
	Performance *BindingPerformanceMetrics `json:"performance,omitempty"`

	// Message provides additional information about the binding state
	// +optional
	Message string `json:"message,omitempty"`
}

// StickyBindingPhase represents the current phase of the sticky binding
type StickyBindingPhase string

const (
	// StickyBindingPhaseActive indicates the binding is active and in use
	StickyBindingPhaseActive StickyBindingPhase = "Active"

	// StickyBindingPhasePending indicates the binding is being created
	StickyBindingPhasePending StickyBindingPhase = "Pending"

	// StickyBindingPhaseRenewing indicates the binding is being renewed
	StickyBindingPhaseRenewing StickyBindingPhase = "Renewing"

	// StickyBindingPhaseExpired indicates the binding has expired
	StickyBindingPhaseExpired StickyBindingPhase = "Expired"

	// StickyBindingPhaseConflicted indicates the binding has conflicts
	StickyBindingPhaseConflicted StickyBindingPhase = "Conflicted"

	// StickyBindingPhaseFailed indicates the binding has failed
	StickyBindingPhaseFailed StickyBindingPhase = "Failed"
)

// BindingConflictStatus tracks conflicts between bindings
type BindingConflictStatus struct {
	// HasConflicts indicates whether there are any conflicts
	HasConflicts bool `json:"hasConflicts"`

	// ConflictingBindings lists other bindings that conflict with this one
	// +optional
	ConflictingBindings []ObjectReference `json:"conflictingBindings,omitempty"`

	// ResolutionStatus indicates the status of conflict resolution
	// +optional
	ResolutionStatus ConflictResolutionStatus `json:"resolutionStatus,omitempty"`

	// LastConflictTime is when the last conflict was detected
	// +optional
	LastConflictTime *metav1.Time `json:"lastConflictTime,omitempty"`
}

// ConflictResolutionStatus defines the status of conflict resolution
type ConflictResolutionStatus string

const (
	// ConflictResolutionStatusPending indicates conflict resolution is pending
	ConflictResolutionStatusPending ConflictResolutionStatus = "Pending"

	// ConflictResolutionStatusInProgress indicates conflict resolution is in progress
	ConflictResolutionStatusInProgress ConflictResolutionStatus = "InProgress"

	// ConflictResolutionStatusResolved indicates the conflict has been resolved
	ConflictResolutionStatusResolved ConflictResolutionStatus = "Resolved"

	// ConflictResolutionStatusFailed indicates conflict resolution failed
	ConflictResolutionStatusFailed ConflictResolutionStatus = "Failed"

	// ConflictResolutionStatusManualRequired indicates manual intervention is required
	ConflictResolutionStatusManualRequired ConflictResolutionStatus = "ManualRequired"
)

// StorageStatus tracks the persistence status of bindings
type StorageStatus struct {
	// BackendType indicates which storage backend is actively being used
	BackendType StorageBackendType `json:"backendType"`

	// LastSyncTime is when the binding was last synchronized to storage
	// +optional
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	// SyncFailures tracks the number of consecutive sync failures
	SyncFailures int32 `json:"syncFailures"`

	// StorageHealth indicates the health of the storage backend
	StorageHealth StorageHealthStatus `json:"storageHealth"`

	// Error message if storage operations are failing
	// +optional
	Error string `json:"error,omitempty"`
}

// StorageHealthStatus defines the health status of storage backends
type StorageHealthStatus string

const (
	// StorageHealthStatusHealthy indicates storage is functioning normally
	StorageHealthStatusHealthy StorageHealthStatus = "Healthy"

	// StorageHealthStatusDegraded indicates storage is experiencing issues
	StorageHealthStatusDegraded StorageHealthStatus = "Degraded"

	// StorageHealthStatusUnhealthy indicates storage is not functioning
	StorageHealthStatusUnhealthy StorageHealthStatus = "Unhealthy"

	// StorageHealthStatusUnknown indicates storage health is unknown
	StorageHealthStatusUnknown StorageHealthStatus = "Unknown"
)

// BindingPerformanceMetrics tracks performance statistics for bindings
type BindingPerformanceMetrics struct {
	// CreationLatency is the time it took to create this binding
	// +optional
	CreationLatency metav1.Duration `json:"creationLatency,omitempty"`

	// AverageRenewalLatency is the average time for binding renewals
	// +optional
	AverageRenewalLatency metav1.Duration `json:"averageRenewalLatency,omitempty"`

	// StorageLatency is the average time for storage operations
	// +optional
	StorageLatency metav1.Duration `json:"storageLatency,omitempty"`

	// ConflictResolutionTime is the time spent resolving conflicts
	// +optional
	ConflictResolutionTime metav1.Duration `json:"conflictResolutionTime,omitempty"`

	// RequestCount tracks the number of requests served by this binding
	RequestCount int64 `json:"requestCount"`

	// ErrorCount tracks the number of errors encountered
	ErrorCount int64 `json:"errorCount"`

	// LastRequestTime is when this binding last served a request
	// +optional
	LastRequestTime *metav1.Time `json:"lastRequestTime,omitempty"`
}

// StickyBindingList is a list of StickyBinding resources
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type StickyBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []StickyBinding `json:"items"`
}

// GetConditions returns the conditions for the StickyBinding.
func (sb *StickyBinding) GetConditions() conditionsv1alpha1.Conditions {
	return sb.Status.Conditions
}

// SetConditions sets the conditions for the StickyBinding.
func (sb *StickyBinding) SetConditions(conditions conditionsv1alpha1.Conditions) {
	sb.Status.Conditions = conditions
}

// StickyBinding condition types
const (
	// StickyBindingConditionReady indicates the binding is ready and active
	StickyBindingConditionReady conditionsv1alpha1.ConditionType = "Ready"

	// StickyBindingConditionStorageReady indicates the storage backend is healthy
	StickyBindingConditionStorageReady conditionsv1alpha1.ConditionType = "StorageReady"

	// StickyBindingConditionConflictResolved indicates conflicts have been resolved
	StickyBindingConditionConflictResolved conditionsv1alpha1.ConditionType = "ConflictResolved"

	// StickyBindingConditionRenewalHealthy indicates renewal is functioning properly
	StickyBindingConditionRenewalHealthy conditionsv1alpha1.ConditionType = "RenewalHealthy"
)

// SessionBindingConstraint defines constraints on session bindings to enforce
// operational policies and resource limits across the multi-cluster environment.
//
// This resource is workspace-aware and supports KCP logical cluster isolation to ensure
// proper multi-tenancy in KCP environments.
//
// +crd
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={tmc,all}
// +kubebuilder:metadata:annotations="kcp.io/cluster-aware=true"
// +kubebuilder:validation:XValidation:rule="self.metadata.annotations['kcp.io/cluster'] != ''"
// +kubebuilder:printcolumn:name="Constraint Type",type=string,JSONPath=`.spec.constraintType`
// +kubebuilder:printcolumn:name="Enforcement",type=string,JSONPath=`.spec.enforcement`
// +kubebuilder:printcolumn:name="Target",type=string,JSONPath=`.spec.target.type`
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:rbac:groups=tmc.kcp.io,resources=sessionbindingconstraints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=tmc.kcp.io,resources=sessionbindingconstraints/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=tmc.kcp.io,resources=sessionbindingconstraints/finalizers,verbs=update
type SessionBindingConstraint struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec SessionBindingConstraintSpec `json:"spec,omitempty"`

	// +optional
	Status SessionBindingConstraintStatus `json:"status,omitempty"`
}

// SessionBindingConstraintSpec defines the desired constraint behavior
type SessionBindingConstraintSpec struct {
	// ConstraintType defines the type of constraint to enforce
	// +kubebuilder:validation:Enum=MaxBindingsPerCluster;MaxBindingsPerWorkload;ResourceUtilizationLimit;NetworkBandwidthLimit;StorageCapacityLimit
	// +kubebuilder:validation:Required
	ConstraintType BindingConstraintType `json:"constraintType"`

	// Target defines what the constraint applies to
	// +kubebuilder:validation:Required
	Target ConstraintTarget `json:"target"`

	// Enforcement defines how strictly the constraint should be enforced
	// +kubebuilder:validation:Enum=Hard;Soft;Warning
	// +kubebuilder:default="Hard"
	Enforcement ConstraintEnforcement `json:"enforcement"`

	// Limit defines the constraint limit (interpretation depends on constraint type)
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Required
	Limit int64 `json:"limit"`

	// ViolationAction defines what action to take when constraint is violated
	// +kubebuilder:validation:Enum=Block;Warn;Log;None
	// +kubebuilder:default="Block"
	// +optional
	ViolationAction ViolationAction `json:"violationAction,omitempty"`

	// Exemptions define conditions under which the constraint can be bypassed
	// +optional
	Exemptions []ConstraintExemption `json:"exemptions,omitempty"`

	// CheckInterval defines how often to evaluate the constraint
	// +kubebuilder:default="60s"
	// +optional
	CheckInterval metav1.Duration `json:"checkInterval,omitempty"`

	// Description provides human-readable explanation of the constraint
	// +optional
	Description string `json:"description,omitempty"`
}

// BindingConstraintType defines the types of constraints that can be enforced
type BindingConstraintType string

const (
	// BindingConstraintTypeMaxBindingsPerCluster limits bindings per cluster
	BindingConstraintTypeMaxBindingsPerCluster BindingConstraintType = "MaxBindingsPerCluster"

	// BindingConstraintTypeMaxBindingsPerWorkload limits bindings per workload
	BindingConstraintTypeMaxBindingsPerWorkload BindingConstraintType = "MaxBindingsPerWorkload"

	// BindingConstraintTypeResourceUtilizationLimit limits resource utilization
	BindingConstraintTypeResourceUtilizationLimit BindingConstraintType = "ResourceUtilizationLimit"

	// BindingConstraintTypeNetworkBandwidthLimit limits network bandwidth usage
	BindingConstraintTypeNetworkBandwidthLimit BindingConstraintType = "NetworkBandwidthLimit"

	// BindingConstraintTypeStorageCapacityLimit limits storage capacity usage
	BindingConstraintTypeStorageCapacityLimit BindingConstraintType = "StorageCapacityLimit"
)

// ConstraintTarget defines what the constraint targets
type ConstraintTarget struct {
	// Type defines the target type
	// +kubebuilder:validation:Enum=Cluster;Namespace;Workload;Global
	// +kubebuilder:validation:Required
	Type ConstraintTargetType `json:"type"`

	// ClusterSelector selects target clusters (when type is Cluster)
	// +optional
	ClusterSelector *ClusterSelector `json:"clusterSelector,omitempty"`

	// NamespaceSelector selects target namespaces (when type is Namespace)
	// +optional
	NamespaceSelector *metav1.LabelSelector `json:"namespaceSelector,omitempty"`

	// WorkloadSelector selects target workloads (when type is Workload)
	// +optional
	WorkloadSelector *WorkloadSelector `json:"workloadSelector,omitempty"`
}

// ConstraintTargetType defines what a constraint can target
type ConstraintTargetType string

const (
	// ConstraintTargetTypeCluster targets specific clusters
	ConstraintTargetTypeCluster ConstraintTargetType = "Cluster"

	// ConstraintTargetTypeNamespace targets specific namespaces
	ConstraintTargetTypeNamespace ConstraintTargetType = "Namespace"

	// ConstraintTargetTypeWorkload targets specific workloads
	ConstraintTargetTypeWorkload ConstraintTargetType = "Workload"

	// ConstraintTargetTypeGlobal applies globally across the system
	ConstraintTargetTypeGlobal ConstraintTargetType = "Global"
)

// ConstraintEnforcement defines how strictly constraints are enforced
type ConstraintEnforcement string

const (
	// ConstraintEnforcementHard strictly enforces the constraint (blocks violations)
	ConstraintEnforcementHard ConstraintEnforcement = "Hard"

	// ConstraintEnforcementSoft prefers to enforce but allows violations when necessary
	ConstraintEnforcementSoft ConstraintEnforcement = "Soft"

	// ConstraintEnforcementWarning only warns about violations
	ConstraintEnforcementWarning ConstraintEnforcement = "Warning"
)

// ViolationAction defines actions to take when constraints are violated
type ViolationAction string

const (
	// ViolationActionBlock blocks the operation that would violate the constraint
	ViolationActionBlock ViolationAction = "Block"

	// ViolationActionWarn allows the operation but generates warnings
	ViolationActionWarn ViolationAction = "Warn"

	// ViolationActionLog logs the violation but takes no other action
	ViolationActionLog ViolationAction = "Log"

	// ViolationActionNone takes no action on violations
	ViolationActionNone ViolationAction = "None"
)

// ConstraintExemption defines conditions under which constraints can be bypassed
type ConstraintExemption struct {
	// Name is a descriptive name for the exemption
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Target defines what this exemption applies to
	// +kubebuilder:validation:Required
	Target ConstraintTarget `json:"target"`

	// Conditions define when this exemption is active
	// +optional
	Conditions []ExemptionCondition `json:"conditions,omitempty"`

	// ExpiresAt defines when this exemption expires
	// +optional
	ExpiresAt *metav1.Time `json:"expiresAt,omitempty"`

	// Reason provides justification for the exemption
	// +optional
	Reason string `json:"reason,omitempty"`
}

// ExemptionCondition defines a condition for constraint exemption
type ExemptionCondition struct {
	// Type defines the condition type
	// +kubebuilder:validation:Enum=Emergency;Maintenance;Testing;Development
	// +kubebuilder:validation:Required
	Type ExemptionConditionType `json:"type"`

	// Value provides additional context for the condition
	// +optional
	Value string `json:"value,omitempty"`
}

// ExemptionConditionType defines types of exemption conditions
type ExemptionConditionType string

const (
	// ExemptionConditionTypeEmergency exempts during emergency situations
	ExemptionConditionTypeEmergency ExemptionConditionType = "Emergency"

	// ExemptionConditionTypeMaintenance exempts during maintenance windows
	ExemptionConditionTypeMaintenance ExemptionConditionType = "Maintenance"

	// ExemptionConditionTypeTesting exempts during testing activities
	ExemptionConditionTypeTesting ExemptionConditionType = "Testing"

	// ExemptionConditionTypeDevelopment exempts during development activities
	ExemptionConditionTypeDevelopment ExemptionConditionType = "Development"
)

// SessionBindingConstraintStatus represents the observed state of the constraint
type SessionBindingConstraintStatus struct {
	// Conditions represent the latest available observations of the constraint state
	// +optional
	Conditions conditionsv1alpha1.Conditions `json:"conditions,omitempty"`

	// Phase indicates the current constraint phase
	// +kubebuilder:default="Active"
	// +optional
	Phase SessionBindingConstraintPhase `json:"phase,omitempty"`

	// LastUpdateTime is when the status was last updated
	// +optional
	LastUpdateTime *metav1.Time `json:"lastUpdateTime,omitempty"`

	// LastCheckTime is when the constraint was last evaluated
	// +optional
	LastCheckTime *metav1.Time `json:"lastCheckTime,omitempty"`

	// ViolationCount tracks the number of constraint violations detected
	ViolationCount int64 `json:"violationCount"`

	// CurrentUsage tracks current usage relative to the constraint limit
	// +optional
	CurrentUsage *int64 `json:"currentUsage,omitempty"`

	// RecentViolations tracks recent constraint violations
	// +optional
	RecentViolations []ConstraintViolation `json:"recentViolations,omitempty"`

	// Message provides additional information about the constraint state
	// +optional
	Message string `json:"message,omitempty"`
}

// SessionBindingConstraintPhase represents the current phase of the constraint
type SessionBindingConstraintPhase string

const (
	// SessionBindingConstraintPhaseActive indicates the constraint is active
	SessionBindingConstraintPhaseActive SessionBindingConstraintPhase = "Active"

	// SessionBindingConstraintPhaseInactive indicates the constraint is inactive
	SessionBindingConstraintPhaseInactive SessionBindingConstraintPhase = "Inactive"

	// SessionBindingConstraintPhaseFailed indicates the constraint has failed
	SessionBindingConstraintPhaseFailed SessionBindingConstraintPhase = "Failed"

	// SessionBindingConstraintPhaseUnknown indicates the constraint state is unknown
	SessionBindingConstraintPhaseUnknown SessionBindingConstraintPhase = "Unknown"
)

// ConstraintViolation represents a violation of a binding constraint
type ConstraintViolation struct {
	// Timestamp is when the violation occurred
	Timestamp metav1.Time `json:"timestamp"`

	// ViolationType describes the type of violation
	ViolationType string `json:"violationType"`

	// Target describes what was affected by the violation
	Target ObjectReference `json:"target"`

	// CurrentValue is the current value that violated the constraint
	CurrentValue int64 `json:"currentValue"`

	// LimitValue is the constraint limit that was violated
	LimitValue int64 `json:"limitValue"`

	// Action describes what action was taken
	Action ViolationAction `json:"action"`

	// Message provides additional details about the violation
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

// GetConditions returns the conditions for the SessionBindingConstraint.
func (sbc *SessionBindingConstraint) GetConditions() conditionsv1alpha1.Conditions {
	return sbc.Status.Conditions
}

// SetConditions sets the conditions for the SessionBindingConstraint.
func (sbc *SessionBindingConstraint) SetConditions(conditions conditionsv1alpha1.Conditions) {
	sbc.Status.Conditions = conditions
}

// SessionBindingConstraint condition types
const (
	// SessionBindingConstraintConditionReady indicates the constraint is ready
	SessionBindingConstraintConditionReady conditionsv1alpha1.ConditionType = "Ready"

	// SessionBindingConstraintConditionEnforced indicates the constraint is being enforced
	SessionBindingConstraintConditionEnforced conditionsv1alpha1.ConditionType = "Enforced"
)