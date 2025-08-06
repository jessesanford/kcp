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
// +kubebuilder:printcolumn:name="Affinity Type",type=string,JSONPath=`.spec.affinityType`
// +kubebuilder:printcolumn:name="Stickiness",type=string,JSONPath=`.spec.stickinessPolicy.type`
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Active Bindings",type=integer,JSONPath=`.status.activeBindings`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:rbac:groups=tmc.kcp.io,resources=sessionaffinitypolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=tmc.kcp.io,resources=sessionaffinitypolicies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=tmc.kcp.io,resources=sessionaffinitypolicies/finalizers,verbs=update
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

	// Weight defines the priority of this affinity policy when multiple policies apply
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:default=50
	// +optional
	Weight int32 `json:"weight,omitempty"`

	// FailoverPolicy defines behavior when affinity targets become unavailable
	// +optional
	FailoverPolicy *AffinityFailoverPolicy `json:"failoverPolicy,omitempty"`
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

	// BackoffMultiplier defines backoff multiplier for retry attempts (as integer, 100 = 1.0x)
	// +kubebuilder:validation:Minimum=100
	// +kubebuilder:validation:Maximum=500
	// +kubebuilder:default=200
	// +optional
	BackoffMultiplier int32 `json:"backoffMultiplier,omitempty"`

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