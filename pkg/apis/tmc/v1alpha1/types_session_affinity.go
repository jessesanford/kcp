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

// SessionAffinityPolicy defines session affinity and sticky placement policies for TMC workloads.
// This API provides fine-grained control over how workloads maintain affinity to specific clusters
// to ensure consistent placement and session continuity across the multi-cluster environment.
//
// +crd
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="Affinity Type",type=string,JSONPath=`.spec.affinityType`
// +kubebuilder:printcolumn:name="Stickiness",type=string,JSONPath=`.spec.stickinessPolicy.type`
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Active Bindings",type=integer,JSONPath=`.status.activeBindings`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type SessionAffinityPolicy struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec SessionAffinityPolicySpec `json:"spec,omitempty"`

	// +optional
	Status SessionAffinityPolicyStatus `json:"status,omitempty"`
}

// SessionAffinityPolicySpec defines the desired state of SessionAffinityPolicy.
type SessionAffinityPolicySpec struct {
	// WorkloadSelector selects the workloads this session affinity policy applies to
	// +kubebuilder:validation:Required
	WorkloadSelector WorkloadSelector `json:"workloadSelector"`

	// ClusterSelector defines which clusters are eligible for affinity binding
	// +kubebuilder:validation:Required
	ClusterSelector ClusterSelector `json:"clusterSelector"`

	// AffinityType defines the type of session affinity to maintain
	// +kubebuilder:validation:Enum=ClientIP;Cookie;Header;WorkloadUID;PersistentSession;None
	// +kubebuilder:validation:Required
	AffinityType SessionAffinityType `json:"affinityType"`

	// StickinessPolicy defines how workloads should maintain cluster affinity
	// +kubebuilder:validation:Required
	StickinessPolicy StickinessPolicy `json:"stickinessPolicy"`

	// AffinityRules defines custom affinity evaluation rules
	// +optional
	AffinityRules []AffinityRule `json:"affinityRules,omitempty"`

	// BindingPersistence controls how session-to-cluster bindings are persisted
	// +optional
	BindingPersistence *BindingPersistenceConfig `json:"bindingPersistence,omitempty"`

	// FailoverPolicy defines behavior when affinity targets become unavailable
	// +optional
	FailoverPolicy *AffinityFailoverPolicy `json:"failoverPolicy,omitempty"`

	// Weight defines the priority of this affinity policy when multiple policies apply
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:default=50
	// +optional
	Weight int32 `json:"weight,omitempty"`
}

// SessionAffinityType defines the mechanism used to determine session affinity
type SessionAffinityType string

const (
	// SessionAffinityTypeClientIP uses client IP address for affinity determination
	SessionAffinityTypeClientIP SessionAffinityType = "ClientIP"

	// SessionAffinityTypeCookie uses HTTP cookie for affinity determination
	SessionAffinityTypeCookie SessionAffinityType = "Cookie"

	// SessionAffinityTypeHeader uses specific HTTP header for affinity determination
	SessionAffinityTypeHeader SessionAffinityType = "Header"

	// SessionAffinityTypeWorkloadUID uses workload UID for affinity determination
	SessionAffinityTypeWorkloadUID SessionAffinityType = "WorkloadUID"

	// SessionAffinityTypePersistentSession uses persistent session tracking
	SessionAffinityTypePersistentSession SessionAffinityType = "PersistentSession"

	// SessionAffinityTypeNone disables session affinity
	SessionAffinityTypeNone SessionAffinityType = "None"
)

// StickinessPolicy defines how workloads should maintain cluster affinity
type StickinessPolicy struct {
	// Type defines the stickiness enforcement type
	// +kubebuilder:validation:Enum=Hard;Soft;Adaptive;None
	// +kubebuilder:default="Soft"
	Type StickinessType `json:"type"`

	// Duration defines how long affinity should be maintained
	// +kubebuilder:default="3600s"
	// +optional
	Duration metav1.Duration `json:"duration,omitempty"`

	// MaxBindings defines the maximum number of concurrent bindings per session
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=10
	// +kubebuilder:default=1
	// +optional
	MaxBindings int32 `json:"maxBindings,omitempty"`

	// BreakOnClusterFailure indicates whether to break affinity on cluster failure
	// +kubebuilder:default=true
	// +optional
	BreakOnClusterFailure bool `json:"breakOnClusterFailure,omitempty"`

	// RebalancingPolicy defines when and how to rebalance sticky sessions
	// +optional
	RebalancingPolicy *RebalancingPolicy `json:"rebalancingPolicy,omitempty"`
}

// StickinessType defines how strictly affinity should be enforced
type StickinessType string

const (
	// StickinessTypeHard enforces strict affinity - placement fails if affinity cannot be maintained
	StickinessTypeHard StickinessType = "Hard"

	// StickinessTypeSoft prefers affinity but allows alternative placement if necessary
	StickinessTypeSoft StickinessType = "Soft"

	// StickinessTypeAdaptive adjusts affinity strength based on cluster conditions
	StickinessTypeAdaptive StickinessType = "Adaptive"

	// StickinessTypeNone disables sticky placement
	StickinessTypeNone StickinessType = "None"
)

// AffinityRule defines custom rules for affinity evaluation and enforcement
type AffinityRule struct {
	// Name is a unique name for this affinity rule
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Type defines the rule evaluation type
	// +kubebuilder:validation:Enum=Constraint;Preference;Requirement
	// +kubebuilder:validation:Required
	Type AffinityRuleType `json:"type"`

	// Constraint defines constraint-based affinity requirements
	// +optional
	Constraint *AffinityConstraint `json:"constraint,omitempty"`

	// Preference defines preference-based affinity scoring
	// +optional
	Preference *AffinityPreference `json:"preference,omitempty"`

	// Weight defines the importance of this rule (higher values = more important)
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:default=50
	// +optional
	Weight int32 `json:"weight,omitempty"`

	// Disabled indicates whether this rule is temporarily disabled
	// +kubebuilder:default=false
	// +optional
	Disabled bool `json:"disabled,omitempty"`
}

// AffinityRuleType defines how affinity rules should be applied
type AffinityRuleType string

const (
	// AffinityRuleTypeConstraint requires the rule to be satisfied (hard requirement)
	AffinityRuleTypeConstraint AffinityRuleType = "Constraint"

	// AffinityRuleTypePreference prefers the rule but allows violations (soft requirement)
	AffinityRuleTypePreference AffinityRuleType = "Preference"

	// AffinityRuleTypeRequirement combines constraint and preference behavior
	AffinityRuleTypeRequirement AffinityRuleType = "Requirement"
)

// AffinityConstraint defines hard constraints for affinity placement
type AffinityConstraint struct {
	// RequiredClusterLabels defines labels that target clusters must have
	// +optional
	RequiredClusterLabels map[string]string `json:"requiredClusterLabels,omitempty"`

	// ProhibitedClusterLabels defines labels that target clusters must not have
	// +optional
	ProhibitedClusterLabels map[string]string `json:"prohibitedClusterLabels,omitempty"`

	// MinClusterResources defines minimum resource requirements for target clusters
	// +optional
	MinClusterResources map[string]string `json:"minClusterResources,omitempty"`

	// MaxLatency defines maximum acceptable latency to target clusters
	// +optional
	MaxLatency *metav1.Duration `json:"maxLatency,omitempty"`

	// RequiredZones defines zones where clusters must be located
	// +optional
	RequiredZones []string `json:"requiredZones,omitempty"`

	// ProhibitedZones defines zones where clusters must not be located
	// +optional
	ProhibitedZones []string `json:"prohibitedZones,omitempty"`
}

// AffinityPreference defines scoring preferences for affinity placement
type AffinityPreference struct {
	// PreferredClusterLabels defines labels that preferred clusters should have
	// +optional
	PreferredClusterLabels map[string]string `json:"preferredClusterLabels,omitempty"`

	// LatencyWeight defines how much to weight cluster latency in scoring (0-100)
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:default=20
	// +optional
	LatencyWeight int32 `json:"latencyWeight,omitempty"`

	// ResourceWeight defines how much to weight cluster resources in scoring (0-100)
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:default=30
	// +optional
	ResourceWeight int32 `json:"resourceWeight,omitempty"`

	// LoadWeight defines how much to weight cluster load in scoring (0-100)
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:default=25
	// +optional
	LoadWeight int32 `json:"loadWeight,omitempty"`

	// AffinityHistoryWeight defines how much to weight previous affinity in scoring (0-100)
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:default=25
	// +optional
	AffinityHistoryWeight int32 `json:"affinityHistoryWeight,omitempty"`
}

// BindingPersistenceConfig controls how session-to-cluster bindings are persisted
type BindingPersistenceConfig struct {
	// Enabled indicates whether binding persistence is enabled
	// +kubebuilder:default=true
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// StorageType defines where binding data should be stored
	// +kubebuilder:validation:Enum=Memory;ConfigMap;Secret;CustomResource;External
	// +kubebuilder:default="ConfigMap"
	// +optional
	StorageType BindingStorageType `json:"storageType,omitempty"`

	// TTL defines how long binding data should be retained
	// +kubebuilder:default="86400s"
	// +optional
	TTL metav1.Duration `json:"ttl,omitempty"`

	// MaxStoredBindings defines maximum number of bindings to retain
	// +kubebuilder:validation:Minimum=10
	// +kubebuilder:validation:Maximum=10000
	// +kubebuilder:default=1000
	// +optional
	MaxStoredBindings int32 `json:"maxStoredBindings,omitempty"`

	// CleanupInterval defines how often to clean expired bindings
	// +kubebuilder:default="3600s"
	// +optional
	CleanupInterval metav1.Duration `json:"cleanupInterval,omitempty"`

	// ExternalConfig defines configuration for external storage backends
	// +optional
	ExternalConfig *ExternalStorageConfig `json:"externalConfig,omitempty"`
}

// BindingStorageType defines where session binding data should be stored
type BindingStorageType string

const (
	// BindingStorageTypeMemory stores bindings in memory (not persistent)
	BindingStorageTypeMemory BindingStorageType = "Memory"

	// BindingStorageTypeConfigMap stores bindings in Kubernetes ConfigMaps
	BindingStorageTypeConfigMap BindingStorageType = "ConfigMap"

	// BindingStorageTypeSecret stores bindings in Kubernetes Secrets
	BindingStorageTypeSecret BindingStorageType = "Secret"

	// BindingStorageTypeCustomResource stores bindings in custom resources
	BindingStorageTypeCustomResource BindingStorageType = "CustomResource"

	// BindingStorageTypeExternal stores bindings in external systems
	BindingStorageTypeExternal BindingStorageType = "External"
)

// ExternalStorageConfig defines configuration for external storage backends
type ExternalStorageConfig struct {
	// Type defines the external storage type
	// +kubebuilder:validation:Enum=Redis;Consul;etcd;Database
	// +kubebuilder:validation:Required
	Type ExternalStorageType `json:"type"`

	// ConnectionString defines how to connect to the external storage
	// +kubebuilder:validation:Required
	ConnectionString string `json:"connectionString"`

	// Credentials defines credentials for external storage access
	// +optional
	Credentials *ObjectReference `json:"credentials,omitempty"`

	// TLS defines TLS configuration for external storage connections
	// +optional
	TLS *TLSConfig `json:"tls,omitempty"`
}

// ExternalStorageType defines supported external storage backends
type ExternalStorageType string

const (
	// ExternalStorageTypeRedis uses Redis for external storage
	ExternalStorageTypeRedis ExternalStorageType = "Redis"

	// ExternalStorageTypeConsul uses Consul for external storage
	ExternalStorageTypeConsul ExternalStorageType = "Consul"

	// ExternalStorageTypeEtcd uses etcd for external storage
	ExternalStorageTypeEtcd ExternalStorageType = "etcd"

	// ExternalStorageTypeDatabase uses relational database for external storage
	ExternalStorageTypeDatabase ExternalStorageType = "Database"
)

// TLSConfig defines TLS configuration for external connections
type TLSConfig struct {
	// Enabled indicates whether TLS is enabled
	// +kubebuilder:default=true
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// InsecureSkipVerify skips TLS certificate verification (for testing only)
	// +kubebuilder:default=false
	// +optional
	InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty"`

	// CertificateAuthority defines the CA certificate for verification
	// +optional
	CertificateAuthority *ObjectReference `json:"certificateAuthority,omitempty"`

	// ClientCertificate defines client certificate for mutual TLS
	// +optional
	ClientCertificate *ObjectReference `json:"clientCertificate,omitempty"`

	// ClientKey defines client private key for mutual TLS
	// +optional
	ClientKey *ObjectReference `json:"clientKey,omitempty"`
}

// AffinityFailoverPolicy defines behavior when affinity targets become unavailable
type AffinityFailoverPolicy struct {
	// Strategy defines the failover strategy
	// +kubebuilder:validation:Enum=Immediate;Delayed;Manual;Disabled
	// +kubebuilder:default="Delayed"
	// +optional
	Strategy FailoverStrategy `json:"strategy,omitempty"`

	// DelayBeforeFailover defines delay before triggering failover
	// +kubebuilder:default="300s"
	// +optional
	DelayBeforeFailover metav1.Duration `json:"delayBeforeFailover,omitempty"`

	// MaxFailoverAttempts defines maximum number of failover attempts
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=10
	// +kubebuilder:default=3
	// +optional
	MaxFailoverAttempts int32 `json:"maxFailoverAttempts,omitempty"`

	// BackoffMultiplier defines backoff multiplier for retry attempts
	// +kubebuilder:validation:Minimum=1.0
	// +kubebuilder:validation:Maximum=5.0
	// +kubebuilder:default=2.0
	// +optional
	BackoffMultiplier float64 `json:"backoffMultiplier,omitempty"`

	// AlternativeClusterSelector defines fallback cluster selection criteria
	// +optional
	AlternativeClusterSelector *ClusterSelector `json:"alternativeClusterSelector,omitempty"`
}

// FailoverStrategy defines how failover should be handled
type FailoverStrategy string

const (
	// FailoverStrategyImmediate triggers immediate failover on target unavailability
	FailoverStrategyImmediate FailoverStrategy = "Immediate"

	// FailoverStrategyDelayed waits for a delay before triggering failover
	FailoverStrategyDelayed FailoverStrategy = "Delayed"

	// FailoverStrategyManual requires manual intervention to trigger failover
	FailoverStrategyManual FailoverStrategy = "Manual"

	// FailoverStrategyDisabled disables failover entirely
	FailoverStrategyDisabled FailoverStrategy = "Disabled"
)

// RebalancingPolicy defines when and how to rebalance sticky sessions
type RebalancingPolicy struct {
	// Enabled indicates whether rebalancing is enabled
	// +kubebuilder:default=false
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// Trigger defines what triggers rebalancing
	// +kubebuilder:validation:Enum=Schedule;LoadImbalance;ClusterChange;Manual
	// +kubebuilder:default="LoadImbalance"
	// +optional
	Trigger RebalancingTrigger `json:"trigger,omitempty"`

	// Schedule defines schedule-based rebalancing configuration
	// +optional
	Schedule *RebalancingSchedule `json:"schedule,omitempty"`

	// LoadImbalanceThreshold defines threshold for load-based rebalancing (percentage)
	// +kubebuilder:validation:Minimum=10
	// +kubebuilder:validation:Maximum=90
	// +kubebuilder:default=80
	// +optional
	LoadImbalanceThreshold int32 `json:"loadImbalanceThreshold,omitempty"`

	// MaxSessionsToMove defines maximum sessions to move in one rebalancing operation
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=1000
	// +kubebuilder:default=100
	// +optional
	MaxSessionsToMove int32 `json:"maxSessionsToMove,omitempty"`

	// DrainTimeout defines timeout for draining sessions during rebalancing
	// +kubebuilder:default="600s"
	// +optional
	DrainTimeout metav1.Duration `json:"drainTimeout,omitempty"`
}

// RebalancingTrigger defines what triggers rebalancing operations
type RebalancingTrigger string

const (
	// RebalancingTriggerSchedule triggers rebalancing on a schedule
	RebalancingTriggerSchedule RebalancingTrigger = "Schedule"

	// RebalancingTriggerLoadImbalance triggers rebalancing on load imbalance
	RebalancingTriggerLoadImbalance RebalancingTrigger = "LoadImbalance"

	// RebalancingTriggerClusterChange triggers rebalancing on cluster changes
	RebalancingTriggerClusterChange RebalancingTrigger = "ClusterChange"

	// RebalancingTriggerManual requires manual triggering
	RebalancingTriggerManual RebalancingTrigger = "Manual"
)

// RebalancingSchedule defines schedule-based rebalancing configuration
type RebalancingSchedule struct {
	// CronExpression defines when rebalancing should occur (cron format)
	// +kubebuilder:validation:Required
	CronExpression string `json:"cronExpression"`

	// TimeZone defines the timezone for the cron expression
	// +kubebuilder:default="UTC"
	// +optional
	TimeZone string `json:"timeZone,omitempty"`

	// Suspended indicates whether scheduled rebalancing is suspended
	// +kubebuilder:default=false
	// +optional
	Suspended bool `json:"suspended,omitempty"`
}

// SessionAffinityPolicyStatus represents the observed state of SessionAffinityPolicy
type SessionAffinityPolicyStatus struct {
	// Conditions represent the latest available observations of the policy state
	// +optional
	Conditions conditionsv1alpha1.Conditions `json:"conditions,omitempty"`

	// Phase indicates the current session affinity policy phase
	// +kubebuilder:default="Active"
	// +optional
	Phase SessionAffinityPolicyPhase `json:"phase,omitempty"`

	// LastUpdateTime is when the status was last updated
	// +optional
	LastUpdateTime *metav1.Time `json:"lastUpdateTime,omitempty"`

	// ActiveBindings represents the current number of active session bindings
	ActiveBindings int32 `json:"activeBindings"`

	// TotalBindings represents the total number of session bindings managed
	TotalBindings int32 `json:"totalBindings"`

	// BindingStatistics provides statistics about session bindings
	// +optional
	BindingStatistics *BindingStatistics `json:"bindingStatistics,omitempty"`

	// ClusterAffinityState contains per-cluster affinity state information
	// +optional
	ClusterAffinityState []ClusterAffinityStatus `json:"clusterAffinityState,omitempty"`

	// AffectedWorkloads contains workloads affected by this affinity policy
	// +optional
	AffectedWorkloads []ObjectReference `json:"affectedWorkloads,omitempty"`

	// FailoverEvents tracks recent failover events
	// +optional
	FailoverEvents []FailoverEvent `json:"failoverEvents,omitempty"`

	// Message provides additional information about the policy state
	// +optional
	Message string `json:"message,omitempty"`
}

// SessionAffinityPolicyPhase represents the current phase of the session affinity policy
type SessionAffinityPolicyPhase string

const (
	// SessionAffinityPolicyPhaseActive indicates the policy is active and processing
	SessionAffinityPolicyPhaseActive SessionAffinityPolicyPhase = "Active"

	// SessionAffinityPolicyPhaseDraining indicates the policy is draining sessions
	SessionAffinityPolicyPhaseDraining SessionAffinityPolicyPhase = "Draining"

	// SessionAffinityPolicyPhaseSuspended indicates the policy is suspended
	SessionAffinityPolicyPhaseSuspended SessionAffinityPolicyPhase = "Suspended"

	// SessionAffinityPolicyPhaseFailed indicates the policy has failed
	SessionAffinityPolicyPhaseFailed SessionAffinityPolicyPhase = "Failed"

	// SessionAffinityPolicyPhaseUnknown indicates the policy state is unknown
	SessionAffinityPolicyPhaseUnknown SessionAffinityPolicyPhase = "Unknown"
)

// BindingStatistics provides statistics about session bindings
type BindingStatistics struct {
	// ByCluster provides binding counts per cluster
	// +optional
	ByCluster map[string]int32 `json:"byCluster,omitempty"`

	// ByAffinityType provides binding counts per affinity type
	// +optional
	ByAffinityType map[string]int32 `json:"byAffinityType,omitempty"`

	// AverageBindingDuration represents average duration of bindings
	// +optional
	AverageBindingDuration *metav1.Duration `json:"averageBindingDuration,omitempty"`

	// ExpiredBindings represents number of expired bindings cleaned up
	ExpiredBindings int32 `json:"expiredBindings"`

	// FailedBindings represents number of failed binding attempts
	FailedBindings int32 `json:"failedBindings"`

	// LastCleanupTime represents when expired bindings were last cleaned
	// +optional
	LastCleanupTime *metav1.Time `json:"lastCleanupTime,omitempty"`
}

// ClusterAffinityStatus contains affinity state for a specific cluster
type ClusterAffinityStatus struct {
	// ClusterName is the name of the cluster
	ClusterName string `json:"clusterName"`

	// ActiveBindings is the number of active bindings to this cluster
	ActiveBindings int32 `json:"activeBindings"`

	// Weight is the current affinity weight for this cluster (0-100)
	Weight int32 `json:"weight"`

	// Health represents the cluster's health for affinity purposes
	// +kubebuilder:validation:Enum=Healthy;Degraded;Unhealthy;Unknown
	Health ClusterAffinityHealth `json:"health"`

	// LastAffinityTime is when affinity was last established to this cluster
	// +optional
	LastAffinityTime *metav1.Time `json:"lastAffinityTime,omitempty"`

	// AverageLatency represents average latency to this cluster
	// +optional
	AverageLatency *metav1.Duration `json:"averageLatency,omitempty"`

	// ResourceUtilization represents current resource utilization (0-100)
	// +optional
	ResourceUtilization int32 `json:"resourceUtilization,omitempty"`

	// Message provides additional information about cluster affinity state
	// +optional
	Message string `json:"message,omitempty"`
}

// ClusterAffinityHealth represents the health of a cluster for affinity purposes
type ClusterAffinityHealth string

const (
	// ClusterAffinityHealthHealthy indicates the cluster is healthy for affinity
	ClusterAffinityHealthHealthy ClusterAffinityHealth = "Healthy"

	// ClusterAffinityHealthDegraded indicates the cluster is degraded
	ClusterAffinityHealthDegraded ClusterAffinityHealth = "Degraded"

	// ClusterAffinityHealthUnhealthy indicates the cluster is unhealthy
	ClusterAffinityHealthUnhealthy ClusterAffinityHealth = "Unhealthy"

	// ClusterAffinityHealthUnknown indicates the cluster health is unknown
	ClusterAffinityHealthUnknown ClusterAffinityHealth = "Unknown"
)

// FailoverEvent represents a failover event in the affinity policy
type FailoverEvent struct {
	// Timestamp is when the failover event occurred
	Timestamp metav1.Time `json:"timestamp"`

	// Reason describes why the failover occurred
	Reason string `json:"reason"`

	// SourceCluster is the cluster that was failed over from
	SourceCluster string `json:"sourceCluster"`

	// TargetCluster is the cluster that was failed over to
	// +optional
	TargetCluster string `json:"targetCluster,omitempty"`

	// AffectedSessions is the number of sessions affected by the failover
	AffectedSessions int32 `json:"affectedSessions"`

	// Success indicates whether the failover was successful
	Success bool `json:"success"`

	// Message provides additional details about the failover event
	// +optional
	Message string `json:"message,omitempty"`
}

// SessionAffinityPolicyList is a list of SessionAffinityPolicy resources
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type SessionAffinityPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []SessionAffinityPolicy `json:"items"`
}