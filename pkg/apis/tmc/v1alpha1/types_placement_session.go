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
	"k8s.io/apimachinery/pkg/api/resource"

	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
)

// PlacementSession represents a session-based approach to workload placement management.
//
// +crd
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
type PlacementSession struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec PlacementSessionSpec `json:"spec,omitempty"`
	// +optional
	Status PlacementSessionStatus `json:"status,omitempty"`
}

// PlacementSessionList contains a list of PlacementSession
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type PlacementSessionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PlacementSession `json:"items"`
}

// PlacementSessionSpec defines the desired state of PlacementSession
type PlacementSessionSpec struct {
	// WorkloadSelector selects the workloads to include in this placement session
	WorkloadSelector WorkloadSelector `json:"workloadSelector"`

	// ClusterSelector selects the clusters available for this placement session
	ClusterSelector ClusterSelector `json:"clusterSelector"`

	// SessionConfiguration defines the configuration for this placement session
	SessionConfiguration SessionConfiguration `json:"sessionConfiguration"`

	// PlacementPolicies defines the placement policies for this session
	// +optional
	PlacementPolicies []PlacementPolicy `json:"placementPolicies,omitempty"`

	// ResourceConstraints defines resource constraints for workloads in this session
	// +optional
	ResourceConstraints *ResourceConstraints `json:"resourceConstraints,omitempty"`

	// Enabled indicates whether this placement session is enabled
	// +kubebuilder:default=true
	// +optional
	Enabled bool `json:"enabled,omitempty"`
}

// PlacementSessionStatus defines the observed state of PlacementSession
type PlacementSessionStatus struct {
	// Conditions represent the current conditions of the PlacementSession
	// +optional
	Conditions []conditionsv1alpha1.Condition `json:"conditions,omitempty"`

	// Phase represents the current phase of the placement session
	// +optional
	Phase SessionPhase `json:"phase,omitempty"`

	// ActiveDecisions contains the currently active placement decisions
	// +optional
	ActiveDecisions []PlacementDecisionRef `json:"activeDecisions,omitempty"`

	// DecisionHistory contains the history of placement decisions
	// +optional
	DecisionHistory []PlacementDecisionRef `json:"decisionHistory,omitempty"`

	// SessionMetrics contains metrics about the session
	// +optional
	SessionMetrics *SessionMetrics `json:"sessionMetrics,omitempty"`

	// LastUpdateTime is the last time the session was updated
	// +optional
	LastUpdateTime *metav1.Time `json:"lastUpdateTime,omitempty"`

	// ObservedGeneration reflects the generation of the most recently observed spec
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// SessionID is the unique identifier for this session across clusters
	// +optional
	SessionID string `json:"sessionID,omitempty"`
}

// SessionConfiguration defines configuration for placement sessions
type SessionConfiguration struct {
	// SessionTimeout defines how long a session can remain active
	// +kubebuilder:default="24h"
	// +optional
	SessionTimeout metav1.Duration `json:"sessionTimeout,omitempty"`

	// HeartbeatInterval defines the interval for session heartbeats
	// +kubebuilder:default="5m"
	// +optional
	HeartbeatInterval metav1.Duration `json:"heartbeatInterval,omitempty"`

	// MaxDecisions defines the maximum number of placement decisions per session
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=100
	// +optional
	MaxDecisions int32 `json:"maxDecisions,omitempty"`

	// ConflictResolution defines how to handle placement conflicts
	// +kubebuilder:validation:Enum=Override;Merge;Fail
	// +kubebuilder:default="Merge"
	// +optional
	ConflictResolution ConflictResolutionType `json:"conflictResolution,omitempty"`

	// PersistenceStrategy defines how session state should be persisted
	// +kubebuilder:validation:Enum=InMemory;Persistent;Distributed
	// +kubebuilder:default="Persistent"
	// +optional
	PersistenceStrategy PersistenceStrategy `json:"persistenceStrategy,omitempty"`

	// RecoveryPolicy defines how sessions should recover from failures
	// +optional
	RecoveryPolicy *SessionRecoveryPolicy `json:"recoveryPolicy,omitempty"`
}

// SessionPhase defines the phases of a placement session
// +kubebuilder:validation:Enum=Created;Initializing;Active;Suspended;Completing;Completed;Failed;Terminated
type SessionPhase string

const (
	// SessionPhaseCreated indicates the session has been created but not started
	SessionPhaseCreated SessionPhase = "Created"
	// SessionPhaseInitializing indicates the session is being initialized
	SessionPhaseInitializing SessionPhase = "Initializing"
	// SessionPhaseActive indicates the session is actively managing placements
	SessionPhaseActive SessionPhase = "Active"
	// SessionPhaseSuspended indicates the session is temporarily suspended
	SessionPhaseSuspended SessionPhase = "Suspended"
	// SessionPhaseCompleting indicates the session is being shut down gracefully
	SessionPhaseCompleting SessionPhase = "Completing"
	// SessionPhaseCompleted indicates the session has completed successfully
	SessionPhaseCompleted SessionPhase = "Completed"
	// SessionPhaseFailed indicates the session has failed
	SessionPhaseFailed SessionPhase = "Failed"
	// SessionPhaseTerminated indicates the session was terminated
	SessionPhaseTerminated SessionPhase = "Terminated"
)

// ConflictResolutionType defines how to handle placement conflicts
// +kubebuilder:validation:Enum=Override;Merge;Fail
type ConflictResolutionType string

const (
	// ConflictResolutionTypeOverride overrides existing placements with new ones
	ConflictResolutionTypeOverride ConflictResolutionType = "Override"
	// ConflictResolutionTypeMerge merges conflicting placements when possible
	ConflictResolutionTypeMerge ConflictResolutionType = "Merge"
	// ConflictResolutionTypeFail fails the session when conflicts occur
	ConflictResolutionTypeFail ConflictResolutionType = "Fail"
)

// PersistenceStrategy defines how session state should be persisted
// +kubebuilder:validation:Enum=InMemory;Persistent;Distributed
type PersistenceStrategy string

const (
	// PersistenceStrategyInMemory stores session state in memory only
	PersistenceStrategyInMemory PersistenceStrategy = "InMemory"
	// PersistenceStrategyPersistent stores session state persistently
	PersistenceStrategyPersistent PersistenceStrategy = "Persistent"
	// PersistenceStrategyDistributed distributes session state across clusters
	PersistenceStrategyDistributed PersistenceStrategy = "Distributed"
)

// PlacementPolicy defines a placement policy within a session
type PlacementPolicy struct {
	// Name is the name of the placement policy
	Name string `json:"name"`

	// Type specifies the type of placement policy
	Type PlacementPolicyType `json:"type"`

	// Rules defines the placement rules for this policy
	Rules []PlacementRule `json:"rules"`

	// Priority defines the priority of this policy (higher numbers = higher priority)
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=1000
	// +kubebuilder:default=500
	// +optional
	Priority int32 `json:"priority,omitempty"`

	// Enabled indicates whether this placement policy is enabled
	// +kubebuilder:default=true
	// +optional
	Enabled bool `json:"enabled,omitempty"`
}

// PlacementPolicyType defines the types of placement policies
// +kubebuilder:validation:Enum=Affinity;AntiAffinity;Spread;Consolidate;Custom
type PlacementPolicyType string

const (
	// PlacementPolicyTypeAffinity represents affinity-based placement policies
	PlacementPolicyTypeAffinity PlacementPolicyType = "Affinity"
	// PlacementPolicyTypeAntiAffinity represents anti-affinity placement policies
	PlacementPolicyTypeAntiAffinity PlacementPolicyType = "AntiAffinity"
	// PlacementPolicyTypeSpread represents spread placement policies
	PlacementPolicyTypeSpread PlacementPolicyType = "Spread"
	// PlacementPolicyTypeConsolidate represents consolidation placement policies
	PlacementPolicyTypeConsolidate PlacementPolicyType = "Consolidate"
	// PlacementPolicyTypeCustom represents custom placement policies
	PlacementPolicyTypeCustom PlacementPolicyType = "Custom"
)

// PlacementRule defines a specific placement rule within a policy
type PlacementRule struct {
	// Name is the name of the placement rule
	Name string `json:"name"`

	// Selector defines what this rule applies to
	Selector PlacementRuleSelector `json:"selector"`

	// Constraints defines the placement constraints for this rule
	Constraints []PlacementConstraint `json:"constraints"`

	// Weight defines the weight of this rule in placement decisions
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:default=50
	// +optional
	Weight int32 `json:"weight,omitempty"`
}

// PlacementRuleSelector defines what a placement rule applies to
type PlacementRuleSelector struct {
	// WorkloadTypes specifies the workload types this rule applies to
	// +optional
	WorkloadTypes []WorkloadType `json:"workloadTypes,omitempty"`

	// LabelSelector selects workloads based on labels
	// +optional
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`

	// ClusterNames specifies specific clusters this rule applies to
	// +optional
	ClusterNames []string `json:"clusterNames,omitempty"`
}

// PlacementConstraint defines a constraint for workload placement
type PlacementConstraint struct {
	// Type specifies the type of constraint
	Type PlacementConstraintType `json:"type"`

	// Key specifies the constraint key (e.g., node label key, zone name)
	Key string `json:"key"`

	// Operator specifies the constraint operator
	Operator PlacementConstraintOperator `json:"operator"`

	// Values specifies the constraint values
	// +optional
	Values []string `json:"values,omitempty"`

	// Required indicates whether this constraint is required or preferred
	// +kubebuilder:default=true
	// +optional
	Required bool `json:"required,omitempty"`
}

// PlacementConstraintType defines the types of placement constraints
// +kubebuilder:validation:Enum=NodeLabel;Zone;Region;ClusterLabel;Resource;Custom
type PlacementConstraintType string

const (
	// PlacementConstraintTypeNodeLabel represents node label constraints
	PlacementConstraintTypeNodeLabel PlacementConstraintType = "NodeLabel"
	// PlacementConstraintTypeZone represents availability zone constraints
	PlacementConstraintTypeZone PlacementConstraintType = "Zone"
	// PlacementConstraintTypeRegion represents region constraints
	PlacementConstraintTypeRegion PlacementConstraintType = "Region"
	// PlacementConstraintTypeClusterLabel represents cluster label constraints
	PlacementConstraintTypeClusterLabel PlacementConstraintType = "ClusterLabel"
	// PlacementConstraintTypeResource represents resource constraints
	PlacementConstraintTypeResource PlacementConstraintType = "Resource"
	// PlacementConstraintTypeCustom represents custom constraints
	PlacementConstraintTypeCustom PlacementConstraintType = "Custom"
)

// PlacementConstraintOperator defines constraint operators
// +kubebuilder:validation:Enum=In;NotIn;Exists;DoesNotExist;Gt;Lt;Equals;NotEquals
type PlacementConstraintOperator string

const (
	// PlacementConstraintOperatorIn represents the "In" operator
	PlacementConstraintOperatorIn PlacementConstraintOperator = "In"
	// PlacementConstraintOperatorNotIn represents the "NotIn" operator
	PlacementConstraintOperatorNotIn PlacementConstraintOperator = "NotIn"
	// PlacementConstraintOperatorExists represents the "Exists" operator
	PlacementConstraintOperatorExists PlacementConstraintOperator = "Exists"
	// PlacementConstraintOperatorDoesNotExist represents the "DoesNotExist" operator
	PlacementConstraintOperatorDoesNotExist PlacementConstraintOperator = "DoesNotExist"
	// PlacementConstraintOperatorGt represents the "Gt" (greater than) operator
	PlacementConstraintOperatorGt PlacementConstraintOperator = "Gt"
	// PlacementConstraintOperatorLt represents the "Lt" (less than) operator
	PlacementConstraintOperatorLt PlacementConstraintOperator = "Lt"
	// PlacementConstraintOperatorEquals represents the "Equals" operator
	PlacementConstraintOperatorEquals PlacementConstraintOperator = "Equals"
	// PlacementConstraintOperatorNotEquals represents the "NotEquals" operator
	PlacementConstraintOperatorNotEquals PlacementConstraintOperator = "NotEquals"
)

// ResourceConstraints defines resource constraints for workloads
type ResourceConstraints struct {
	// CPULimits defines CPU resource limits
	// +optional
	CPULimits *ResourceLimit `json:"cpuLimits,omitempty"`

	// MemoryLimits defines memory resource limits
	// +optional
	MemoryLimits *ResourceLimit `json:"memoryLimits,omitempty"`

	// StorageLimits defines storage resource limits
	// +optional
	StorageLimits *ResourceLimit `json:"storageLimits,omitempty"`

	// CustomLimits defines custom resource limits
	// +optional
	CustomLimits map[string]ResourceLimit `json:"customLimits,omitempty"`
}

// ResourceLimit defines a resource limit
type ResourceLimit struct {
	// Min specifies the minimum resource requirement
	// +optional
	Min *resource.Quantity `json:"min,omitempty"`

	// Max specifies the maximum resource limit
	// +optional
	Max *resource.Quantity `json:"max,omitempty"`

	// Default specifies the default resource allocation
	// +optional
	Default *resource.Quantity `json:"default,omitempty"`
}

// SessionRecoveryPolicy defines how sessions should recover from failures
type SessionRecoveryPolicy struct {
	// RestartPolicy defines whether to restart failed sessions
	// +kubebuilder:validation:Enum=Always;OnFailure;Never
	// +kubebuilder:default="OnFailure"
	// +optional
	RestartPolicy SessionRestartPolicy `json:"restartPolicy,omitempty"`

	// MaxRetries defines the maximum number of restart attempts
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=3
	// +optional
	MaxRetries int32 `json:"maxRetries,omitempty"`

	// RetryDelay defines the delay between restart attempts
	// +kubebuilder:default="1m"
	// +optional
	RetryDelay metav1.Duration `json:"retryDelay,omitempty"`

	// BackoffMultiplier defines the multiplier for exponential backoff
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=2
	// +optional
	BackoffMultiplier float64 `json:"backoffMultiplier,omitempty"`
}

// SessionRestartPolicy defines session restart policies
// +kubebuilder:validation:Enum=Always;OnFailure;Never
type SessionRestartPolicy string

const (
	// SessionRestartPolicyAlways always restarts failed sessions
	SessionRestartPolicyAlways SessionRestartPolicy = "Always"
	// SessionRestartPolicyOnFailure restarts sessions only on failure
	SessionRestartPolicyOnFailure SessionRestartPolicy = "OnFailure"
	// SessionRestartPolicyNever never restarts failed sessions
	SessionRestartPolicyNever SessionRestartPolicy = "Never"
)

// PlacementDecisionRef references a placement decision
type PlacementDecisionRef struct {
	// Name is the name of the placement decision
	Name string `json:"name"`

	// Namespace is the namespace of the placement decision
	Namespace string `json:"namespace"`

	// UID is the unique identifier of the placement decision
	// +optional
	UID string `json:"uid,omitempty"`

	// CreationTimestamp is when the decision was created
	// +optional
	CreationTimestamp metav1.Time `json:"creationTimestamp,omitempty"`

	// Status is the current status of the placement decision
	// +optional
	Status string `json:"status,omitempty"`
}

// SessionMetrics contains metrics about a placement session
type SessionMetrics struct {
	// TotalDecisions is the total number of placement decisions made
	// +optional
	TotalDecisions int32 `json:"totalDecisions,omitempty"`

	// ActiveDecisions is the number of currently active placement decisions
	// +optional
	ActiveDecisions int32 `json:"activeDecisions,omitempty"`

	// SuccessfulDecisions is the number of successful placement decisions
	// +optional
	SuccessfulDecisions int32 `json:"successfulDecisions,omitempty"`

	// FailedDecisions is the number of failed placement decisions
	// +optional
	FailedDecisions int32 `json:"failedDecisions,omitempty"`

	// ConflictsResolved is the number of conflicts resolved
	// +optional
	ConflictsResolved int32 `json:"conflictsResolved,omitempty"`

	// AverageDecisionTime is the average time to make placement decisions
	// +optional
	AverageDecisionTime *metav1.Duration `json:"averageDecisionTime,omitempty"`

	// SessionDuration is the duration the session has been active
	// +optional
	SessionDuration *metav1.Duration `json:"sessionDuration,omitempty"`
}