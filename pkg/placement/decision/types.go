/*
Copyright 2023 The KCP Authors.

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

package decision

import (
	"context"
	"time"

	"github.com/kcp-dev/logicalcluster/v3"

	schedulerapi "github.com/kcp-dev/kcp/pkg/placement/scheduler"
)

// DecisionMaker provides the main interface for making placement decisions.
// It coordinates scheduler results with CEL evaluation to produce final placement decisions.
type DecisionMaker interface {
	// MakePlacementDecision makes a final placement decision based on scheduler results and CEL rules
	MakePlacementDecision(ctx context.Context, request *PlacementRequest, candidates []*schedulerapi.ScoredCandidate) (*PlacementDecision, error)
	
	// ValidateDecision validates a placement decision against constraints and policies
	ValidateDecision(ctx context.Context, decision *PlacementDecision) error
	
	// RecordDecision records a placement decision for audit and debugging purposes
	RecordDecision(ctx context.Context, decision *PlacementDecision) error
	
	// GetDecisionHistory returns the decision history for a specific placement
	GetDecisionHistory(ctx context.Context, placementID string) ([]*DecisionRecord, error)
	
	// ApplyOverride applies manual placement overrides to a decision
	ApplyOverride(ctx context.Context, decision *PlacementDecision, override *PlacementOverride) (*PlacementDecision, error)
}

// PlacementRequest represents a request for placement decision making.
type PlacementRequest struct {
	// ID is the unique identifier for this placement request
	ID string
	
	// Name is the name of the placement request
	Name string
	
	// Namespace is the namespace of the placement request
	Namespace string
	
	// SourceWorkspace is the workspace where the request originated
	SourceWorkspace logicalcluster.Name
	
	// SchedulerRequest is the original scheduler request
	SchedulerRequest *schedulerapi.PlacementRequest
	
	// CELExpressions are custom CEL expressions to evaluate for this placement
	CELExpressions []CELExpression
	
	// DecisionDeadline is when this decision must be made by
	DecisionDeadline time.Time
	
	// MaxRetries is the maximum number of decision retries allowed
	MaxRetries int
	
	// CurrentRetry tracks the current retry count
	CurrentRetry int
	
	// CreatedAt is when this decision request was created
	CreatedAt time.Time
}

// CELExpression defines a CEL expression for placement evaluation.
type CELExpression struct {
	// Name is the identifier for this expression
	Name string
	
	// Expression is the CEL expression string
	Expression string
	
	// Weight is the weight of this expression in the final decision (0-100)
	Weight float64
	
	// Required indicates if this expression must evaluate to true
	Required bool
	
	// Description explains what this expression evaluates
	Description string
}

// PlacementDecision represents a final placement decision.
type PlacementDecision struct {
	// ID is the unique identifier for this decision
	ID string
	
	// RequestID is the ID of the original placement request
	RequestID string
	
	// SelectedWorkspaces are the workspaces chosen for placement
	SelectedWorkspaces []*WorkspacePlacement
	
	// RejectedCandidates are candidates that were considered but not selected
	RejectedCandidates []*RejectedCandidate
	
	// DecisionTime is when this decision was made
	DecisionTime time.Time
	
	// DecisionDuration is how long the decision process took
	DecisionDuration time.Duration
	
	// DecisionRationale explains the reasoning behind this decision
	DecisionRationale DecisionRationale
	
	// SchedulerDecision is the underlying scheduler decision
	SchedulerDecision *schedulerapi.SchedulingDecision
	
	// CELEvaluationResults contains the results of CEL expression evaluations
	CELEvaluationResults []CELEvaluationResult
	
	// Override contains any manual override that was applied
	Override *PlacementOverride
	
	// Error contains any error that occurred during decision making
	Error error
	
	// Status indicates the current status of this decision
	Status DecisionStatus
}

// WorkspacePlacement represents a workspace selected in a placement decision.
type WorkspacePlacement struct {
	// Workspace is the selected workspace
	Workspace logicalcluster.Name
	
	// SchedulerScore is the score from the scheduler (0-100)
	SchedulerScore float64
	
	// CELScore is the combined CEL evaluation score (0-100)
	CELScore float64
	
	// FinalScore is the weighted final score (0-100)
	FinalScore float64
	
	// AllocatedResources are the resources reserved in this workspace
	AllocatedResources schedulerapi.ResourceAllocation
	
	// SelectionReason explains why this workspace was selected
	SelectionReason string
	
	// CELResults contains detailed CEL evaluation results for this workspace
	CELResults []CELEvaluationResult
}

// RejectedCandidate represents a workspace candidate that was not selected.
type RejectedCandidate struct {
	// Workspace is the rejected workspace
	Workspace logicalcluster.Name
	
	// SchedulerScore is the score from the scheduler
	SchedulerScore float64
	
	// CELScore is the combined CEL evaluation score
	CELScore float64
	
	// FinalScore is the weighted final score
	FinalScore float64
	
	// RejectionReason explains why this workspace was rejected
	RejectionReason string
	
	// CELResults contains detailed CEL evaluation results for this workspace
	CELResults []CELEvaluationResult
}

// CELEvaluationResult represents the result of evaluating a CEL expression.
type CELEvaluationResult struct {
	// ExpressionName is the name of the evaluated expression
	ExpressionName string
	
	// Expression is the CEL expression that was evaluated
	Expression string
	
	// Result is the evaluation result (typically boolean)
	Result interface{}
	
	// Score is the numeric score derived from this evaluation (0-100)
	Score float64
	
	// Success indicates if the evaluation succeeded
	Success bool
	
	// Error contains any evaluation error
	Error error
	
	// EvaluationTime is how long this evaluation took
	EvaluationTime time.Duration
	
	// Workspace is the workspace this evaluation was performed for
	Workspace logicalcluster.Name
}

// DecisionRationale provides detailed reasoning for a placement decision.
type DecisionRationale struct {
	// Summary is a brief summary of the decision reasoning
	Summary string
	
	// SchedulerFactors describes the scheduler-based factors
	SchedulerFactors []string
	
	// CELFactors describes the CEL evaluation factors
	CELFactors []string
	
	// OverrideFactors describes any override factors that were applied
	OverrideFactors []string
	
	// ConstraintViolations describes any constraint violations that occurred
	ConstraintViolations []string
	
	// DecisionAlgorithm describes which algorithm was used for the final decision
	DecisionAlgorithm string
	
	// WeightingStrategy describes how different factors were weighted
	WeightingStrategy string
}

// DecisionStatus represents the status of a placement decision.
type DecisionStatus string

const (
	// DecisionStatusPending indicates the decision is still being made
	DecisionStatusPending DecisionStatus = "Pending"
	
	// DecisionStatusComplete indicates the decision has been successfully made
	DecisionStatusComplete DecisionStatus = "Complete"
	
	// DecisionStatusError indicates an error occurred during decision making
	DecisionStatusError DecisionStatus = "Error"
	
	// DecisionStatusOverridden indicates the decision was overridden manually
	DecisionStatusOverridden DecisionStatus = "Overridden"
	
	// DecisionStatusRolledBack indicates the decision was rolled back
	DecisionStatusRolledBack DecisionStatus = "RolledBack"
)

// PlacementOverride represents a manual override for placement decisions.
type PlacementOverride struct {
	// ID is the unique identifier for this override
	ID string
	
	// PlacementID is the ID of the placement this override applies to
	PlacementID string
	
	// OverrideType specifies the type of override
	OverrideType OverrideType
	
	// TargetWorkspaces specifies the workspaces to use for placement
	TargetWorkspaces []logicalcluster.Name
	
	// ExcludedWorkspaces specifies workspaces to exclude from placement
	ExcludedWorkspaces []logicalcluster.Name
	
	// Reason explains why this override was applied
	Reason string
	
	// AppliedBy indicates who applied this override
	AppliedBy string
	
	// CreatedAt is when this override was created
	CreatedAt time.Time
	
	// ExpiresAt is when this override expires (optional)
	ExpiresAt *time.Time
	
	// Priority is the priority of this override (higher values take precedence)
	Priority int32
}

// OverrideType specifies the type of placement override.
type OverrideType string

const (
	// OverrideTypeForce forces placement to specific workspaces
	OverrideTypeForce OverrideType = "Force"
	
	// OverrideTypeExclude excludes specific workspaces from placement
	OverrideTypeExclude OverrideType = "Exclude"
	
	// OverrideTypePrefer adds preference for specific workspaces
	OverrideTypePrefer OverrideType = "Prefer"
	
	// OverrideTypeAvoid adds avoidance for specific workspaces
	OverrideTypeAvoid OverrideType = "Avoid"
)

// DecisionRecord represents a historical record of a placement decision.
type DecisionRecord struct {
	// Decision is the placement decision
	Decision *PlacementDecision
	
	// Timestamp is when this record was created
	Timestamp time.Time
	
	// Version is the version of this record (for updates)
	Version int64
	
	// Events are events related to this decision
	Events []DecisionEvent
}

// DecisionEvent represents an event in the decision-making process.
type DecisionEvent struct {
	// Type is the type of event
	Type DecisionEventType
	
	// Timestamp is when this event occurred
	Timestamp time.Time
	
	// Message is a human-readable description of the event
	Message string
	
	// Details contains additional structured information about the event
	Details map[string]interface{}
}

// DecisionEventType represents the type of decision event.
type DecisionEventType string

const (
	// DecisionEventTypeStarted indicates decision making started
	DecisionEventTypeStarted DecisionEventType = "Started"
	
	// DecisionEventTypeSchedulerEvaluated indicates scheduler evaluation completed
	DecisionEventTypeSchedulerEvaluated DecisionEventType = "SchedulerEvaluated"
	
	// DecisionEventTypeCELEvaluated indicates CEL evaluation completed
	DecisionEventTypeCELEvaluated DecisionEventType = "CELEvaluated"
	
	// DecisionEventTypeOverrideApplied indicates an override was applied
	DecisionEventTypeOverrideApplied DecisionEventType = "OverrideApplied"
	
	// DecisionEventTypeCompleted indicates decision making completed
	DecisionEventTypeCompleted DecisionEventType = "Completed"
	
	// DecisionEventTypeError indicates an error occurred
	DecisionEventTypeError DecisionEventType = "Error"
	
	// DecisionEventTypeRolledBack indicates the decision was rolled back
	DecisionEventTypeRolledBack DecisionEventType = "RolledBack"
)

// DecisionAlgorithm defines different algorithms for making placement decisions.
type DecisionAlgorithm string

const (
	// AlgorithmWeightedScore uses weighted scoring of scheduler and CEL results
	AlgorithmWeightedScore DecisionAlgorithm = "WeightedScore"
	
	// AlgorithmCELPrimary prioritizes CEL evaluation results over scheduler scores
	AlgorithmCELPrimary DecisionAlgorithm = "CELPrimary"
	
	// AlgorithmSchedulerPrimary prioritizes scheduler scores over CEL evaluation
	AlgorithmSchedulerPrimary DecisionAlgorithm = "SchedulerPrimary"
	
	// AlgorithmConsensus requires both scheduler and CEL to agree on selections
	AlgorithmConsensus DecisionAlgorithm = "Consensus"
)

// DecisionConfig provides configuration for the decision-making process.
type DecisionConfig struct {
	// Algorithm specifies which decision algorithm to use
	Algorithm DecisionAlgorithm
	
	// SchedulerWeight is the weight given to scheduler scores (0-100)
	SchedulerWeight float64
	
	// CELWeight is the weight given to CEL evaluation scores (0-100)
	CELWeight float64
	
	// MinimumScore is the minimum score required for workspace selection
	MinimumScore float64
	
	// MaxDecisionTime is the maximum time allowed for decision making
	MaxDecisionTime time.Duration
	
	// EnableAuditLogging indicates if audit logging should be enabled
	EnableAuditLogging bool
	
	// DefaultCELExpressions are CEL expressions applied to all placement requests
	DefaultCELExpressions []CELExpression
}

// DecisionValidator provides validation for placement decisions.
type DecisionValidator interface {
	// ValidateDecision validates a placement decision against constraints
	ValidateDecision(ctx context.Context, decision *PlacementDecision) error
	
	// ValidateResourceConstraints validates resource allocation constraints
	ValidateResourceConstraints(ctx context.Context, placements []*WorkspacePlacement) error
	
	// ValidatePolicyCompliance validates policy compliance for the decision
	ValidatePolicyCompliance(ctx context.Context, decision *PlacementDecision) error
	
	// CheckConflicts checks for conflicts with existing placements
	CheckConflicts(ctx context.Context, decision *PlacementDecision) ([]ConflictDescription, error)
}

// ConflictDescription describes a conflict with existing placements.
type ConflictDescription struct {
	// Type is the type of conflict
	Type ConflictType
	
	// Description is a human-readable description of the conflict
	Description string
	
	// AffectedWorkspaces are the workspaces affected by this conflict
	AffectedWorkspaces []logicalcluster.Name
	
	// Severity indicates the severity of the conflict
	Severity ConflictSeverity
	
	// ResolutionSuggestion suggests how to resolve the conflict
	ResolutionSuggestion string
}

// ConflictType represents different types of placement conflicts.
type ConflictType string

const (
	// ConflictTypeResourceOvercommit indicates resource overcommitment
	ConflictTypeResourceOvercommit ConflictType = "ResourceOvercommit"
	
	// ConflictTypeAffinityViolation indicates affinity rule violation
	ConflictTypeAffinityViolation ConflictType = "AffinityViolation"
	
	// ConflictTypeAntiAffinityViolation indicates anti-affinity rule violation
	ConflictTypeAntiAffinityViolation ConflictType = "AntiAffinityViolation"
	
	// ConflictTypePolicyViolation indicates policy violation
	ConflictTypePolicyViolation ConflictType = "PolicyViolation"
)

// ConflictSeverity represents the severity of a placement conflict.
type ConflictSeverity string

const (
	// SeverityLow indicates a low-severity conflict that can be ignored
	SeverityLow ConflictSeverity = "Low"
	
	// SeverityMedium indicates a medium-severity conflict that should be addressed
	SeverityMedium ConflictSeverity = "Medium"
	
	// SeverityHigh indicates a high-severity conflict that must be resolved
	SeverityHigh ConflictSeverity = "High"
	
	// SeverityCritical indicates a critical conflict that blocks placement
	SeverityCritical ConflictSeverity = "Critical"
)