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

// PlacementDecision represents a placement decision within a session-based placement system.
//
// +crd
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
type PlacementDecision struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec PlacementDecisionSpec `json:"spec,omitempty"`
	// +optional
	Status PlacementDecisionStatus `json:"status,omitempty"`
}

// PlacementDecisionList contains a list of PlacementDecision
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type PlacementDecisionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PlacementDecision `json:"items"`
}

// PlacementDecisionSpec defines the desired state of PlacementDecision
type PlacementDecisionSpec struct {
	// SessionRef references the placement session this decision belongs to
	SessionRef SessionReference `json:"sessionRef"`

	// WorkloadRef references the workload being placed
	WorkloadRef WorkloadReference `json:"workloadRef"`

	// TargetCluster specifies the target cluster for placement
	TargetCluster string `json:"targetCluster"`

	// PlacementReason describes why this placement was chosen
	// +optional
	PlacementReason string `json:"placementReason,omitempty"`

	// PlacementScore is the score assigned to this placement decision
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	// +optional
	PlacementScore int32 `json:"placementScore,omitempty"`

	// DecisionContext contains context information about the decision
	// +optional
	DecisionContext *DecisionContext `json:"decisionContext,omitempty"`

	// RollbackPolicy defines the rollback policy for this decision
	// +optional
	RollbackPolicy *RollbackPolicy `json:"rollbackPolicy,omitempty"`
}

// PlacementDecisionStatus defines the observed state of PlacementDecision
type PlacementDecisionStatus struct {
	// Conditions represent the current conditions of the PlacementDecision
	// +optional
	Conditions []conditionsv1alpha1.Condition `json:"conditions,omitempty"`

	// Phase represents the current phase of the placement decision
	// +optional
	Phase PlacementDecisionPhase `json:"phase,omitempty"`

	// ExecutionStatus contains the status of decision execution
	// +optional
	ExecutionStatus *DecisionExecutionStatus `json:"executionStatus,omitempty"`

	// ConflictStatus contains information about any conflicts
	// +optional
	ConflictStatus *DecisionConflictStatus `json:"conflictStatus,omitempty"`

	// RollbackStatus contains the status of any rollback operations
	// +optional
	RollbackStatus *RollbackStatus `json:"rollbackStatus,omitempty"`

	// ObservedGeneration reflects the generation of the most recently observed spec
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// DecisionTime is when the placement decision was made
	// +optional
	DecisionTime *metav1.Time `json:"decisionTime,omitempty"`
}

// PlacementDecisionPhase defines the phases of a placement decision
// +kubebuilder:validation:Enum=Pending;Evaluating;Decided;Executing;Active;Completed;Failed;Cancelled;RolledBack
type PlacementDecisionPhase string

const (
	// PlacementDecisionPhasePending indicates the decision is pending evaluation
	PlacementDecisionPhasePending PlacementDecisionPhase = "Pending"
	// PlacementDecisionPhaseEvaluating indicates the decision is being evaluated
	PlacementDecisionPhaseEvaluating PlacementDecisionPhase = "Evaluating"
	// PlacementDecisionPhaseDecided indicates the decision has been made
	PlacementDecisionPhaseDecided PlacementDecisionPhase = "Decided"
	// PlacementDecisionPhaseExecuting indicates the decision is being executed
	PlacementDecisionPhaseExecuting PlacementDecisionPhase = "Executing"
	// PlacementDecisionPhaseActive indicates the decision is active
	PlacementDecisionPhaseActive PlacementDecisionPhase = "Active"
	// PlacementDecisionPhaseCompleted indicates the decision has completed
	PlacementDecisionPhaseCompleted PlacementDecisionPhase = "Completed"
	// PlacementDecisionPhaseFailed indicates the decision has failed
	PlacementDecisionPhaseFailed PlacementDecisionPhase = "Failed"
	// PlacementDecisionPhaseCancelled indicates the decision was cancelled
	PlacementDecisionPhaseCancelled PlacementDecisionPhase = "Cancelled"
	// PlacementDecisionPhaseRolledBack indicates the decision was rolled back
	PlacementDecisionPhaseRolledBack PlacementDecisionPhase = "RolledBack"
)

// DecisionContext contains context information about a placement decision
type DecisionContext struct {
	// DecisionID is the unique identifier for this decision
	DecisionID string `json:"decisionID"`

	// DecisionAlgorithm describes the algorithm used to make the decision
	// +optional
	DecisionAlgorithm string `json:"decisionAlgorithm,omitempty"`

	// EvaluatedClusters contains information about clusters evaluated for placement
	// +optional
	EvaluatedClusters []ClusterEvaluation `json:"evaluatedClusters,omitempty"`

	// AppliedPolicies contains the policies that were applied during decision making
	// +optional
	AppliedPolicies []AppliedPolicy `json:"appliedPolicies,omitempty"`

	// ConstraintEvaluations contains evaluations of placement constraints
	// +optional
	ConstraintEvaluations []ConstraintEvaluation `json:"constraintEvaluations,omitempty"`

	// AlternativePlacements contains alternative placements that were considered
	// +optional
	AlternativePlacements []AlternativePlacement `json:"alternativePlacements,omitempty"`

	// DecisionMetrics contains metrics about the decision process
	// +optional
	DecisionMetrics *DecisionMetrics `json:"decisionMetrics,omitempty"`
}

// ClusterEvaluation contains evaluation information for a cluster
type ClusterEvaluation struct {
	// ClusterName is the name of the evaluated cluster
	ClusterName string `json:"clusterName"`

	// Score is the score assigned to this cluster (0-100)
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	Score int32 `json:"score"`

	// Eligible indicates whether this cluster is eligible for placement
	Eligible bool `json:"eligible"`

	// EvaluationCriteria contains the criteria used for evaluation
	// +optional
	EvaluationCriteria []EvaluationCriterion `json:"evaluationCriteria,omitempty"`

	// ResourceAvailability contains information about resource availability
	// +optional
	ResourceAvailability map[string]resource.Quantity `json:"resourceAvailability,omitempty"`

	// RejectionReasons contains reasons why this cluster was rejected
	// +optional
	RejectionReasons []string `json:"rejectionReasons,omitempty"`
}

// EvaluationCriterion represents a criterion used in cluster evaluation
type EvaluationCriterion struct {
	// Name is the name of the evaluation criterion
	Name string `json:"name"`

	// Weight is the weight assigned to this criterion
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	Weight int32 `json:"weight"`

	// Score is the score for this criterion (0-100)
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	Score int32 `json:"score"`

	// Description provides a description of this criterion
	// +optional
	Description string `json:"description,omitempty"`
}

// AppliedPolicy represents a policy that was applied during decision making
type AppliedPolicy struct {
	// PolicyName is the name of the policy
	PolicyName string `json:"policyName"`

	// PolicyType is the type of the policy
	PolicyType PlacementPolicyType `json:"policyType"`

	// Impact describes how this policy affected the decision
	// +optional
	Impact string `json:"impact,omitempty"`

	// Applied indicates whether this policy was successfully applied
	Applied bool `json:"applied"`

	// AppliedRules contains the rules from this policy that were applied
	// +optional
	AppliedRules []string `json:"appliedRules,omitempty"`
}

// AlternativePlacement represents an alternative placement that was considered
type AlternativePlacement struct {
	// ClusterName is the name of the alternative cluster
	ClusterName string `json:"clusterName"`

	// Score is the score assigned to this alternative (0-100)
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	Score int32 `json:"score"`

	// Reason describes why this alternative was not chosen
	// +optional
	Reason string `json:"reason,omitempty"`

	// RankPosition is the rank position of this alternative
	// +optional
	RankPosition int32 `json:"rankPosition,omitempty"`
}

// DecisionMetrics contains metrics about the decision-making process
type DecisionMetrics struct {
	// EvaluationDuration is the time taken to evaluate placement options
	// +optional
	EvaluationDuration *metav1.Duration `json:"evaluationDuration,omitempty"`

	// ClustersEvaluated is the number of clusters evaluated
	// +optional
	ClustersEvaluated int32 `json:"clustersEvaluated,omitempty"`

	// PoliciesApplied is the number of policies applied
	// +optional
	PoliciesApplied int32 `json:"policiesApplied,omitempty"`

	// ConstraintsEvaluated is the number of constraints evaluated
	// +optional
	ConstraintsEvaluated int32 `json:"constraintsEvaluated,omitempty"`

	// ConflictsDetected is the number of conflicts detected during evaluation
	// +optional
	ConflictsDetected int32 `json:"conflictsDetected,omitempty"`
}

// DecisionExecutionStatus contains the status of decision execution
type DecisionExecutionStatus struct {
	// Phase represents the current execution phase
	Phase DecisionExecutionPhase `json:"phase"`

	// StartTime is when execution started
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// CompletionTime is when execution completed
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// ExecutionSteps contains the steps taken during execution
	// +optional
	ExecutionSteps []ExecutionStep `json:"executionSteps,omitempty"`

	// LastError contains the last error encountered during execution
	// +optional
	LastError string `json:"lastError,omitempty"`

	// RetryCount is the number of execution retry attempts
	// +optional
	RetryCount int32 `json:"retryCount,omitempty"`
}

// DecisionExecutionPhase defines the phases of decision execution
// +kubebuilder:validation:Enum=Pending;InProgress;Completed;Failed;Retrying
type DecisionExecutionPhase string

const (
	// DecisionExecutionPhasePending indicates execution is pending
	DecisionExecutionPhasePending DecisionExecutionPhase = "Pending"
	// DecisionExecutionPhaseInProgress indicates execution is in progress
	DecisionExecutionPhaseInProgress DecisionExecutionPhase = "InProgress"
	// DecisionExecutionPhaseCompleted indicates execution has completed
	DecisionExecutionPhaseCompleted DecisionExecutionPhase = "Completed"
	// DecisionExecutionPhaseFailed indicates execution has failed
	DecisionExecutionPhaseFailed DecisionExecutionPhase = "Failed"
	// DecisionExecutionPhaseRetrying indicates execution is being retried
	DecisionExecutionPhaseRetrying DecisionExecutionPhase = "Retrying"
)

// ExecutionStep represents a step in the execution of a placement decision
type ExecutionStep struct {
	// StepName is the name of the execution step
	StepName string `json:"stepName"`

	// Status is the status of this execution step
	Status ExecutionStepStatus `json:"status"`

	// StartTime is when this step started
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// CompletionTime is when this step completed
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// Message contains a message about this step
	// +optional
	Message string `json:"message,omitempty"`

	// Error contains any error that occurred during this step
	// +optional
	Error string `json:"error,omitempty"`
}

// ExecutionStepStatus defines the status of an execution step
// +kubebuilder:validation:Enum=Pending;Running;Completed;Failed;Skipped
type ExecutionStepStatus string

const (
	// ExecutionStepStatusPending indicates the step is pending
	ExecutionStepStatusPending ExecutionStepStatus = "Pending"
	// ExecutionStepStatusRunning indicates the step is running
	ExecutionStepStatusRunning ExecutionStepStatus = "Running"
	// ExecutionStepStatusCompleted indicates the step has completed
	ExecutionStepStatusCompleted ExecutionStepStatus = "Completed"
	// ExecutionStepStatusFailed indicates the step has failed
	ExecutionStepStatusFailed ExecutionStepStatus = "Failed"
	// ExecutionStepStatusSkipped indicates the step was skipped
	ExecutionStepStatusSkipped ExecutionStepStatus = "Skipped"
)

// DecisionConflictStatus contains information about decision conflicts
type DecisionConflictStatus struct {
	// HasConflicts indicates whether there are active conflicts
	HasConflicts bool `json:"hasConflicts"`

	// ConflictCount is the number of active conflicts
	// +optional
	ConflictCount int32 `json:"conflictCount,omitempty"`

	// ActiveConflicts contains information about active conflicts
	// +optional
	ActiveConflicts []DecisionConflict `json:"activeConflicts,omitempty"`

	// ResolvedConflicts contains information about resolved conflicts
	// +optional
	ResolvedConflicts []DecisionConflict `json:"resolvedConflicts,omitempty"`

	// LastConflictTime is when the last conflict was detected
	// +optional
	LastConflictTime *metav1.Time `json:"lastConflictTime,omitempty"`
}

// DecisionConflict represents a conflict with a placement decision
type DecisionConflict struct {
	// ConflictID is the unique identifier for the conflict
	ConflictID string `json:"conflictID"`

	// ConflictType describes the type of conflict
	ConflictType ConflictType `json:"conflictType"`

	// ConflictingDecision references the conflicting placement decision
	// +optional
	ConflictingDecision *PlacementReference `json:"conflictingDecision,omitempty"`

	// DetectedTime is when the conflict was detected
	DetectedTime metav1.Time `json:"detectedTime"`

	// Description provides a description of the conflict
	// +optional
	Description string `json:"description,omitempty"`

	// ResolutionStrategy describes how the conflict should be resolved
	// +optional
	ResolutionStrategy ConflictResolutionType `json:"resolutionStrategy,omitempty"`

	// Status is the current status of the conflict
	Status ConflictStatus `json:"status"`
}

// ConflictStatus defines the status of a conflict
// +kubebuilder:validation:Enum=Detected;Analyzing;Resolving;Resolved;Failed
type ConflictStatus string

const (
	// ConflictStatusDetected indicates the conflict has been detected
	ConflictStatusDetected ConflictStatus = "Detected"
	// ConflictStatusAnalyzing indicates the conflict is being analyzed
	ConflictStatusAnalyzing ConflictStatus = "Analyzing"
	// ConflictStatusResolving indicates the conflict is being resolved
	ConflictStatusResolving ConflictStatus = "Resolving"
	// ConflictStatusResolved indicates the conflict has been resolved
	ConflictStatusResolved ConflictStatus = "Resolved"
	// ConflictStatusFailed indicates conflict resolution has failed
	ConflictStatusFailed ConflictStatus = "Failed"
)

// RollbackPolicy defines the rollback policy for a placement decision
type RollbackPolicy struct {
	// Enabled indicates whether rollback is enabled
	// +kubebuilder:default=true
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// AutoRollback indicates whether rollback should be automatic
	// +kubebuilder:default=false
	// +optional
	AutoRollback bool `json:"autoRollback,omitempty"`

	// RollbackTriggers defines what triggers a rollback
	// +optional
	RollbackTriggers []RollbackTrigger `json:"rollbackTriggers,omitempty"`

	// RollbackTimeout defines the timeout for rollback operations
	// +kubebuilder:default="10m"
	// +optional
	RollbackTimeout metav1.Duration `json:"rollbackTimeout,omitempty"`

	// RetainHistory indicates whether to retain rollback history
	// +kubebuilder:default=true
	// +optional
	RetainHistory bool `json:"retainHistory,omitempty"`
}

// RollbackTrigger defines a trigger for automatic rollback
type RollbackTrigger struct {
	// Type specifies the type of rollback trigger
	Type RollbackTriggerType `json:"type"`

	// Condition specifies the condition that triggers rollback
	// +optional
	Condition string `json:"condition,omitempty"`

	// Threshold specifies the threshold for the trigger
	// +optional
	Threshold string `json:"threshold,omitempty"`

	// Duration specifies how long the condition must persist
	// +kubebuilder:default="5m"
	// +optional
	Duration metav1.Duration `json:"duration,omitempty"`
}

// RollbackTriggerType defines the types of rollback triggers
// +kubebuilder:validation:Enum=HealthCheck;ResourceExhaustion;PerformanceDegradation;Manual
type RollbackTriggerType string

const (
	// RollbackTriggerTypeHealthCheck represents health check based triggers
	RollbackTriggerTypeHealthCheck RollbackTriggerType = "HealthCheck"
	// RollbackTriggerTypeResourceExhaustion represents resource exhaustion triggers
	RollbackTriggerTypeResourceExhaustion RollbackTriggerType = "ResourceExhaustion"
	// RollbackTriggerTypePerformanceDegradation represents performance triggers
	RollbackTriggerTypePerformanceDegradation RollbackTriggerType = "PerformanceDegradation"
	// RollbackTriggerTypeManual represents manual triggers
	RollbackTriggerTypeManual RollbackTriggerType = "Manual"
)

// RollbackStatus contains the status of rollback operations
type RollbackStatus struct {
	// InProgress indicates whether a rollback is in progress
	InProgress bool `json:"inProgress"`

	// RollbackAttempts is the number of rollback attempts made
	// +optional
	RollbackAttempts int32 `json:"rollbackAttempts,omitempty"`

	// LastRollbackTime is when the last rollback was attempted
	// +optional
	LastRollbackTime *metav1.Time `json:"lastRollbackTime,omitempty"`

	// RollbackHistory contains the history of rollback operations
	// +optional
	RollbackHistory []RollbackOperation `json:"rollbackHistory,omitempty"`

	// CurrentRollback contains information about the current rollback
	// +optional
	CurrentRollback *RollbackOperation `json:"currentRollback,omitempty"`
}

// RollbackOperation represents a rollback operation
type RollbackOperation struct {
	// OperationID is the unique identifier for the rollback operation
	OperationID string `json:"operationID"`

	// StartTime is when the rollback started
	StartTime metav1.Time `json:"startTime"`

	// CompletionTime is when the rollback completed
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// Status is the status of the rollback operation
	Status RollbackOperationStatus `json:"status"`

	// TriggerType describes what triggered the rollback
	TriggerType RollbackTriggerType `json:"triggerType"`

	// TriggerReason provides the reason for the rollback
	// +optional
	TriggerReason string `json:"triggerReason,omitempty"`

	// SourceCluster is the cluster being rolled back from
	SourceCluster string `json:"sourceCluster"`

	// TargetCluster is the cluster being rolled back to
	// +optional
	TargetCluster string `json:"targetCluster,omitempty"`

	// Steps contains the steps taken during rollback
	// +optional
	Steps []RollbackStep `json:"steps,omitempty"`

	// Error contains any error that occurred during rollback
	// +optional
	Error string `json:"error,omitempty"`
}

// RollbackOperationStatus defines the status of a rollback operation
// +kubebuilder:validation:Enum=Pending;InProgress;Completed;Failed;Cancelled
type RollbackOperationStatus string

const (
	// RollbackOperationStatusPending indicates the operation is pending
	RollbackOperationStatusPending RollbackOperationStatus = "Pending"
	// RollbackOperationStatusInProgress indicates the operation is in progress
	RollbackOperationStatusInProgress RollbackOperationStatus = "InProgress"
	// RollbackOperationStatusCompleted indicates the operation has completed
	RollbackOperationStatusCompleted RollbackOperationStatus = "Completed"
	// RollbackOperationStatusFailed indicates the operation has failed
	RollbackOperationStatusFailed RollbackOperationStatus = "Failed"
	// RollbackOperationStatusCancelled indicates the operation was cancelled
	RollbackOperationStatusCancelled RollbackOperationStatus = "Cancelled"
)

// RollbackStep represents a step in a rollback operation
type RollbackStep struct {
	// StepName is the name of the rollback step
	StepName string `json:"stepName"`

	// Status is the status of this rollback step
	Status ExecutionStepStatus `json:"status"`

	// StartTime is when this step started
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// CompletionTime is when this step completed
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// Message contains a message about this step
	// +optional
	Message string `json:"message,omitempty"`

	// Error contains any error that occurred during this step
	// +optional
	Error string `json:"error,omitempty"`
}