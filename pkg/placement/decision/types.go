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
	"fmt"
	"time"

	"github.com/kcp-dev/logicalcluster/v3"

	schedulerapi "github.com/kcp-dev/kcp/pkg/placement/scheduler"
)

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

// ==============================================================================
// Decision Recording and Audit Types
// ==============================================================================

// DecisionRecord represents a recorded placement decision with audit information.
type DecisionRecord struct {
	// DecisionID is the unique identifier for the decision
	DecisionID string
	
	// RequestID is the ID of the original placement request
	RequestID string
	
	// Decision contains the full decision information
	Decision *PlacementDecision
	
	// RecordedAt is when this record was created
	RecordedAt time.Time
	
	// RecorderVersion is the version of the recorder that created this record
	RecorderVersion string
	
	// TTL is the time-to-live for this record
	TTL time.Duration
	
	// ExpiresAt is when this record expires (derived from RecordedAt + TTL)
	ExpiresAt time.Time
	
	// Metadata contains additional metadata about the record
	Metadata map[string]interface{}
}

// DecisionAttempt represents a single attempt at making a placement decision.
type DecisionAttempt struct {
	// ID is the unique identifier for this attempt
	ID string
	
	// RequestID is the ID of the placement request
	RequestID string
	
	// StartTime is when the attempt started
	StartTime time.Time
	
	// EndTime is when the attempt completed (success or failure)
	EndTime time.Time
	
	// Duration is how long the attempt took
	Duration time.Duration
	
	// Success indicates if the attempt was successful
	Success bool
	
	// Error contains any error that occurred during the attempt
	Error error
	
	// Phase indicates which phase the attempt was in when it completed
	Phase DecisionPhase
	
	// IntermediateResults contains partial results from the attempt
	IntermediateResults *IntermediateDecisionResults
	
	// Workspace is the logical cluster where the attempt was made
	Workspace logicalcluster.Name
}

// DecisionPhase represents the phase of decision making.
type DecisionPhase string

const (
	// PhaseInitialization is the initialization phase
	PhaseInitialization DecisionPhase = "Initialization"
	
	// PhaseScheduling is the scheduling phase
	PhaseScheduling DecisionPhase = "Scheduling"
	
	// PhaseCELEvaluation is the CEL expression evaluation phase
	PhaseCELEvaluation DecisionPhase = "CELEvaluation"
	
	// PhaseValidation is the validation phase
	PhaseValidation DecisionPhase = "Validation"
	
	// PhaseFinalization is the finalization phase
	PhaseFinalization DecisionPhase = "Finalization"
)

// IntermediateDecisionResults contains partial results from a decision attempt.
type IntermediateDecisionResults struct {
	// CandidateWorkspaces are the workspaces that were considered
	CandidateWorkspaces []logicalcluster.Name
	
	// SchedulerResults contains partial results from scheduler evaluation
	SchedulerResults []SchedulerResult
	
	// CELResults contains partial results from CEL evaluation
	CELResults []CELEvaluationResult
	
	// ValidationResults contains validation results
	ValidationResults []ValidationResult
}

// SchedulerResult represents a result from scheduler evaluation.
type SchedulerResult struct {
	// Workspace is the evaluated workspace
	Workspace logicalcluster.Name
	
	// Score is the scheduler score (0-100)
	Score float64
	
	// Feasible indicates if the workspace is feasible for placement
	Feasible bool
	
	// Reasons contains the reasons for the score/feasibility
	Reasons []string
}

// ValidationResult represents a result from decision validation.
type ValidationResult struct {
	// ValidationType is the type of validation performed
	ValidationType ValidationType
	
	// Success indicates if validation passed
	Success bool
	
	// Message contains a human-readable validation message
	Message string
	
	// Details contains detailed validation information
	Details map[string]interface{}
}

// ValidationType represents different types of validation.
type ValidationType string

const (
	// ValidationTypeResourceConstraints validates resource constraints
	ValidationTypeResourceConstraints ValidationType = "ResourceConstraints"
	
	// ValidationTypePolicyCompliance validates policy compliance
	ValidationTypePolicyCompliance ValidationType = "PolicyCompliance"
	
	// ValidationTypeConflictCheck validates for conflicts
	ValidationTypeConflictCheck ValidationType = "ConflictCheck"
)

// HistoryQuery represents a query for decision history.
type HistoryQuery struct {
	// RequestID filters by placement request ID
	RequestID string
	
	// DecisionIDs filters by specific decision IDs
	DecisionIDs []string
	
	// WorkspaceFilter filters by workspace involvement
	WorkspaceFilter logicalcluster.Name
	
	// TimeRange filters by time range
	TimeRange *TimeRange
	
	// StatusFilter filters by decision status
	StatusFilter []DecisionStatus
	
	// Limit limits the number of results
	Limit int
	
	// Offset specifies the result offset for pagination
	Offset int
	
	// SortBy specifies how to sort results
	SortBy HistorySortBy
	
	// SortOrder specifies sort order (ascending/descending)
	SortOrder SortOrder
}

// Validate validates the history query.
func (q *HistoryQuery) Validate() error {
	if q.Limit < 0 {
		return fmt.Errorf("limit cannot be negative")
	}
	if q.Offset < 0 {
		return fmt.Errorf("offset cannot be negative")
	}
	if q.TimeRange != nil {
		if err := q.TimeRange.Validate(); err != nil {
			return fmt.Errorf("invalid time range: %w", err)
		}
	}
	return nil
}

// HistorySortBy specifies how to sort history query results.
type HistorySortBy string

const (
	// SortByDecisionTime sorts by decision time
	SortByDecisionTime HistorySortBy = "DecisionTime"
	
	// SortByRecordedTime sorts by recorded time
	SortByRecordedTime HistorySortBy = "RecordedTime"
	
	// SortByDuration sorts by decision duration
	SortByDuration HistorySortBy = "Duration"
	
	// SortByRequestID sorts by request ID
	SortByRequestID HistorySortBy = "RequestID"
)

// SortOrder specifies sort order.
type SortOrder string

const (
	// SortOrderAscending sorts in ascending order
	SortOrderAscending SortOrder = "Ascending"
	
	// SortOrderDescending sorts in descending order
	SortOrderDescending SortOrder = "Descending"
)

// TimeRange represents a time range for queries.
type TimeRange struct {
	// Start is the start time (inclusive)
	Start time.Time
	
	// End is the end time (exclusive)
	End time.Time
}

// Validate validates the time range.
func (tr *TimeRange) Validate() error {
	if tr.Start.After(tr.End) {
		return fmt.Errorf("start time cannot be after end time")
	}
	return nil
}

// DecisionMetrics contains aggregated metrics about placement decisions.
type DecisionMetrics struct {
	// TotalDecisions is the total number of decisions in the time range
	TotalDecisions int64
	
	// SuccessfulDecisions is the number of successful decisions
	SuccessfulDecisions int64
	
	// FailedDecisions is the number of failed decisions
	FailedDecisions int64
	
	// AverageDecisionDuration is the average decision duration
	AverageDecisionDuration time.Duration
	
	// MedianDecisionDuration is the median decision duration
	MedianDecisionDuration time.Duration
	
	// P95DecisionDuration is the 95th percentile decision duration
	P95DecisionDuration time.Duration
	
	// WorkspaceUtilization contains per-workspace utilization metrics
	WorkspaceUtilization map[logicalcluster.Name]*WorkspaceUtilizationMetrics
	
	// DecisionsByStatus contains counts by decision status
	DecisionsByStatus map[DecisionStatus]int64
	
	// TimeRange is the time range these metrics cover
	TimeRange TimeRange
}

// WorkspaceUtilizationMetrics contains utilization metrics for a workspace.
type WorkspaceUtilizationMetrics struct {
	// Workspace is the workspace these metrics apply to
	Workspace logicalcluster.Name
	
	// DecisionsCount is the number of decisions involving this workspace
	DecisionsCount int64
	
	// SuccessfulPlacements is the number of successful placements
	SuccessfulPlacements int64
	
	// AverageScore is the average placement score for this workspace
	AverageScore float64
	
	// ResourceUtilization contains resource utilization information
	ResourceUtilization map[string]interface{}
}

// RetentionPolicy defines how long decision records should be retained.
type RetentionPolicy struct {
	// DefaultTTL is the default TTL for decision records
	DefaultTTL time.Duration
	
	// SuccessfulDecisionTTL is the TTL for successful decisions
	SuccessfulDecisionTTL time.Duration
	
	// FailedDecisionTTL is the TTL for failed decisions
	FailedDecisionTTL time.Duration
	
	// AttemptTTL is the TTL for decision attempts
	AttemptTTL time.Duration
	
	// MaxRecords is the maximum number of records to keep
	MaxRecords int64
	
	// PurgeInterval is how often to run purge operations
	PurgeInterval time.Duration
}

// Validate validates the retention policy.
func (rp *RetentionPolicy) Validate() error {
	if rp.DefaultTTL <= 0 {
		return fmt.Errorf("default TTL must be positive")
	}
	if rp.SuccessfulDecisionTTL <= 0 {
		return fmt.Errorf("successful decision TTL must be positive")
	}
	if rp.FailedDecisionTTL <= 0 {
		return fmt.Errorf("failed decision TTL must be positive")
	}
	if rp.AttemptTTL <= 0 {
		return fmt.Errorf("attempt TTL must be positive")
	}
	if rp.MaxRecords <= 0 {
		return fmt.Errorf("max records must be positive")
	}
	if rp.PurgeInterval <= 0 {
		return fmt.Errorf("purge interval must be positive")
	}
	return nil
}

// RecorderConfig configures the decision recorder.
type RecorderConfig struct {
	// Version is the recorder version
	Version string
	
	// DefaultTTL is the default TTL for records
	DefaultTTL time.Duration
	
	// DefaultRetentionPolicy is the default retention policy
	DefaultRetentionPolicy *RetentionPolicy
	
	// CleanupInterval is how often to run cleanup operations
	CleanupInterval time.Duration
	
	// CleanupTimeout is the timeout for cleanup operations
	CleanupTimeout time.Duration
	
	// EnableMetrics enables Prometheus metrics collection
	EnableMetrics bool
	
	// EnableAuditLogging enables detailed audit logging
	EnableAuditLogging bool
}

// Validate validates the recorder configuration.
func (rc *RecorderConfig) Validate() error {
	if rc.Version == "" {
		return fmt.Errorf("version cannot be empty")
	}
	if rc.DefaultTTL <= 0 {
		return fmt.Errorf("default TTL must be positive")
	}
	if rc.DefaultRetentionPolicy != nil {
		if err := rc.DefaultRetentionPolicy.Validate(); err != nil {
			return fmt.Errorf("invalid default retention policy: %w", err)
		}
	}
	if rc.CleanupInterval <= 0 {
		return fmt.Errorf("cleanup interval must be positive")
	}
	if rc.CleanupTimeout <= 0 {
		return fmt.Errorf("cleanup timeout must be positive")
	}
	return nil
}

// DefaultRecorderConfig returns a default recorder configuration.
func DefaultRecorderConfig() *RecorderConfig {
	return &RecorderConfig{
		Version:         "1.0.0",
		DefaultTTL:      24 * time.Hour,
		CleanupInterval: time.Hour,
		CleanupTimeout:  5 * time.Minute,
		EnableMetrics:   true,
		EnableAuditLogging: true,
		DefaultRetentionPolicy: &RetentionPolicy{
			DefaultTTL:            24 * time.Hour,
			SuccessfulDecisionTTL: 7 * 24 * time.Hour,  // 7 days
			FailedDecisionTTL:     30 * 24 * time.Hour, // 30 days
			AttemptTTL:            24 * time.Hour,      // 1 day
			MaxRecords:            10000,
			PurgeInterval:         time.Hour,
		},
	}
}

// DecisionEventType represents the type of decision event.
type DecisionEventType string

const (
	// DecisionEventTypeNormal represents a normal decision event
	DecisionEventTypeNormal DecisionEventType = "Normal"
	
	// DecisionEventTypeWarning represents a warning decision event
	DecisionEventTypeWarning DecisionEventType = "Warning"
)

// DecisionStorage provides an interface for storing and retrieving decision records.
type DecisionStorage interface {
	// Store stores a decision record
	Store(ctx context.Context, record *DecisionRecord) error
	
	// StoreAttempt stores a decision attempt
	StoreAttempt(ctx context.Context, attempt *DecisionAttempt) error
	
	// Query retrieves records based on query criteria
	Query(ctx context.Context, query *HistoryQuery) ([]*DecisionRecord, error)
	
	// GetMetrics retrieves aggregated metrics
	GetMetrics(ctx context.Context, timeRange TimeRange) (*DecisionMetrics, error)
	
	// Purge removes records based on retention policy
	Purge(ctx context.Context, policy *RetentionPolicy) (int, error)
}

// ==============================================================================
// Override Management Types
// ==============================================================================

// CreateOverrideRequest represents a request to create a placement override.
type CreateOverrideRequest struct {
	// PlacementID is the ID of the placement this override applies to
	PlacementID string
	
	// OverrideType specifies the type of override
	OverrideType OverrideType
	
	// TargetWorkspaces specifies workspaces to target (for Force/Prefer overrides)
	TargetWorkspaces []logicalcluster.Name
	
	// ExcludedWorkspaces specifies workspaces to exclude (for Exclude overrides)
	ExcludedWorkspaces []logicalcluster.Name
	
	// Reason explains why this override is being created
	Reason string
	
	// AppliedBy indicates who is creating this override
	AppliedBy string
	
	// ExpiresAt is when this override should expire (optional)
	ExpiresAt *time.Time
	
	// Priority is the priority of this override (higher values take precedence)
	Priority int32
}

// Validate validates the create override request.
func (r *CreateOverrideRequest) Validate() error {
	if r.PlacementID == "" {
		return fmt.Errorf("placement ID cannot be empty")
	}
	if r.Reason == "" {
		return fmt.Errorf("reason cannot be empty")
	}
	if r.AppliedBy == "" {
		return fmt.Errorf("applied by cannot be empty")
	}
	
	// Validate override-type specific requirements
	switch r.OverrideType {
	case OverrideTypeForce:
		if len(r.TargetWorkspaces) == 0 {
			return fmt.Errorf("force override requires target workspaces")
		}
	case OverrideTypeExclude:
		if len(r.ExcludedWorkspaces) == 0 {
			return fmt.Errorf("exclude override requires excluded workspaces")
		}
	case OverrideTypePrefer:
		if len(r.TargetWorkspaces) == 0 {
			return fmt.Errorf("prefer override requires target workspaces")
		}
	case OverrideTypeAvoid:
		if len(r.TargetWorkspaces) == 0 {
			return fmt.Errorf("avoid override requires target workspaces")
		}
	default:
		return fmt.Errorf("invalid override type: %s", r.OverrideType)
	}
	
	return nil
}

// OverrideFilter provides filtering criteria for override queries.
type OverrideFilter struct {
	// PlacementID filters by placement ID
	PlacementID string
	
	// OverrideType filters by override type
	OverrideType OverrideType
	
	// AppliedBy filters by who applied the override
	AppliedBy string
	
	// Active filters for active (not expired) overrides only
	Active bool
	
	// TimeRange filters by creation time
	TimeRange *TimeRange
	
	// Limit limits the number of results
	Limit int
	
	// Offset specifies the result offset for pagination
	Offset int
}

// OverrideHistoryEntry represents an entry in override history.
type OverrideHistoryEntry struct {
	// OverrideID is the ID of the override
	OverrideID string
	
	// PlacementID is the ID of the placement
	PlacementID string
	
	// Action is the action that was performed
	Action OverrideAction
	
	// Timestamp is when the action occurred
	Timestamp time.Time
	
	// Message provides additional context about the action
	Message string
	
	// AppliedBy indicates who performed the action
	AppliedBy string
}

// OverrideAction represents an action performed on an override.
type OverrideAction string

const (
	// OverrideActionCreated indicates an override was created
	OverrideActionCreated OverrideAction = "Created"
	
	// OverrideActionApplied indicates an override was applied to a decision
	OverrideActionApplied OverrideAction = "Applied"
	
	// OverrideActionExpired indicates an override expired
	OverrideActionExpired OverrideAction = "Expired"
	
	// OverrideActionDeleted indicates an override was deleted
	OverrideActionDeleted OverrideAction = "Deleted"
)

// OverrideValidator provides validation for placement overrides.
type OverrideValidator interface {
	// ValidateOverride validates an override against system policies
	ValidateOverride(ctx context.Context, override *PlacementOverride) error
}

// OverrideStorage provides storage operations for placement overrides.
type OverrideStorage interface {
	// StoreOverride stores a placement override
	StoreOverride(ctx context.Context, override *PlacementOverride) error
	
	// UpdateOverride updates an existing override
	UpdateOverride(ctx context.Context, override *PlacementOverride) error
	
	// DeleteOverride deletes an override
	DeleteOverride(ctx context.Context, overrideID string) error
	
	// QueryOverrides retrieves overrides based on filter criteria
	QueryOverrides(ctx context.Context, filter *OverrideFilter) ([]*PlacementOverride, error)
	
	// GetOverrideHistory retrieves override history for a placement
	GetOverrideHistory(ctx context.Context, placementID string) ([]*OverrideHistoryEntry, error)
	
	// CleanupExpiredOverrides removes expired overrides from storage
	CleanupExpiredOverrides(ctx context.Context) error
}

// OverrideManagerConfig configures the override manager.
type OverrideManagerConfig struct {
	// MaxActiveOverridesPerPlacement limits active overrides per placement
	MaxActiveOverridesPerPlacement int
	
	// DefaultOverridePriority is the default priority for new overrides
	DefaultOverridePriority int32
	
	// CleanupInterval is how often to run cleanup operations
	CleanupInterval time.Duration
	
	// CleanupTimeout is the timeout for cleanup operations
	CleanupTimeout time.Duration
	
	// PreferenceScoreBoost is the score boost for preference overrides
	PreferenceScoreBoost float64
	
	// AvoidanceScorePenalty is the score penalty for avoidance overrides
	AvoidanceScorePenalty float64
}

// Validate validates the override manager configuration.
func (c *OverrideManagerConfig) Validate() error {
	if c.MaxActiveOverridesPerPlacement <= 0 {
		return fmt.Errorf("max active overrides per placement must be positive")
	}
	if c.CleanupInterval <= 0 {
		return fmt.Errorf("cleanup interval must be positive")
	}
	if c.CleanupTimeout <= 0 {
		return fmt.Errorf("cleanup timeout must be positive")
	}
	if c.PreferenceScoreBoost < 0 {
		return fmt.Errorf("preference score boost cannot be negative")
	}
	if c.AvoidanceScorePenalty < 0 {
		return fmt.Errorf("avoidance score penalty cannot be negative")
	}
	return nil
}

// DefaultOverrideManagerConfig returns a default override manager configuration.
func DefaultOverrideManagerConfig() *OverrideManagerConfig {
	return &OverrideManagerConfig{
		MaxActiveOverridesPerPlacement: 10,
		DefaultOverridePriority:        50,
		CleanupInterval:                time.Hour,
		CleanupTimeout:                 5 * time.Minute,
		PreferenceScoreBoost:           20.0,
		AvoidanceScorePenalty:          15.0,
	}
}