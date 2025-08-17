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
	"sort"
	"time"

	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"

	celapi "github.com/kcp-dev/kcp/pkg/placement/cel"
	schedulerapi "github.com/kcp-dev/kcp/pkg/placement/scheduler"
)

// defaultDecisionMaker implements the DecisionMaker interface.
type defaultDecisionMaker struct {
	celEvaluator celapi.CELEvaluator
	validator    DecisionValidator
	recorder     DecisionRecorder
	config       DecisionConfig
}

// NewDecisionMaker creates a new placement decision maker with the specified components.
func NewDecisionMaker(
	celEvaluator celapi.CELEvaluator,
	validator DecisionValidator,
	recorder DecisionRecorder,
	config DecisionConfig,
) DecisionMaker {
	return &defaultDecisionMaker{
		celEvaluator: celEvaluator,
		validator:    validator,
		recorder:     recorder,
		config:       config,
	}
}

// MakePlacementDecision makes a final placement decision based on scheduler results and CEL rules.
func (dm *defaultDecisionMaker) MakePlacementDecision(
	ctx context.Context,
	request *PlacementRequest,
	candidates []*schedulerapi.ScoredCandidate,
) (*PlacementDecision, error) {
	startTime := time.Now()
	decisionID := string(uuid.NewUUID())

	klog.V(2).InfoS("Starting placement decision", 
		"decisionID", decisionID,
		"requestID", request.ID,
		"candidates", len(candidates))

	// Create the initial decision structure
	decision := &PlacementDecision{
		ID:                   decisionID,
		RequestID:           request.ID,
		DecisionTime:        startTime,
		Status:              DecisionStatusPending,
		SchedulerDecision:   nil, // Will be filled if available
		CELEvaluationResults: []CELEvaluationResult{},
		DecisionRationale: DecisionRationale{
			DecisionAlgorithm:  string(dm.config.Algorithm),
			WeightingStrategy:  fmt.Sprintf("Scheduler: %.1f%%, CEL: %.1f%%", dm.config.SchedulerWeight, dm.config.CELWeight),
		},
	}

	// Apply timeout if configured
	if dm.config.MaxDecisionTime > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, dm.config.MaxDecisionTime)
		defer cancel()
	}

	// Record decision start event
	if dm.config.EnableAuditLogging && dm.recorder != nil {
		dm.recordEvent(ctx, decision, DecisionEventTypeStarted, "Decision making process started", nil)
	}

	// Step 1: Evaluate candidates with CEL expressions
	celResults, err := dm.evaluateCandidatesWithCEL(ctx, request, candidates)
	if err != nil {
		decision.Error = fmt.Errorf("CEL evaluation failed: %w", err)
		decision.Status = DecisionStatusError
		dm.recordEvent(ctx, decision, DecisionEventTypeError, "CEL evaluation failed", map[string]interface{}{"error": err.Error()})
		return decision, err
	}

	decision.CELEvaluationResults = celResults
	dm.recordEvent(ctx, decision, DecisionEventTypeCELEvaluated, fmt.Sprintf("CEL evaluation completed for %d candidates", len(candidates)), nil)

	// Step 2: Apply decision algorithm to combine scheduler and CEL results
	selectedWorkspaces, rejectedCandidates, err := dm.applyDecisionAlgorithm(ctx, request, candidates, celResults)
	if err != nil {
		decision.Error = fmt.Errorf("decision algorithm failed: %w", err)
		decision.Status = DecisionStatusError
		dm.recordEvent(ctx, decision, DecisionEventTypeError, "Decision algorithm failed", map[string]interface{}{"error": err.Error()})
		return decision, err
	}

	decision.SelectedWorkspaces = selectedWorkspaces
	decision.RejectedCandidates = rejectedCandidates

	// Step 3: Generate decision rationale
	dm.generateDecisionRationale(decision, request, candidates, celResults)

	// Step 4: Validate the decision
	if dm.validator != nil {
		if err := dm.validator.ValidateDecision(ctx, decision); err != nil {
			decision.Error = fmt.Errorf("decision validation failed: %w", err)
			decision.Status = DecisionStatusError
			dm.recordEvent(ctx, decision, DecisionEventTypeError, "Decision validation failed", map[string]interface{}{"error": err.Error()})
			return decision, err
		}
	}

	// Step 5: Complete the decision
	decision.DecisionDuration = time.Since(startTime)
	decision.Status = DecisionStatusComplete
	dm.recordEvent(ctx, decision, DecisionEventTypeCompleted, fmt.Sprintf("Decision completed in %v", decision.DecisionDuration), nil)

	klog.V(2).InfoS("Placement decision completed",
		"decisionID", decisionID,
		"selectedWorkspaces", len(selectedWorkspaces),
		"rejectedCandidates", len(rejectedCandidates),
		"duration", decision.DecisionDuration)

	return decision, nil
}

// evaluateCandidatesWithCEL evaluates all workspace candidates using CEL expressions.
func (dm *defaultDecisionMaker) evaluateCandidatesWithCEL(
	ctx context.Context,
	request *PlacementRequest,
	candidates []*schedulerapi.ScoredCandidate,
) ([]CELEvaluationResult, error) {
	var allResults []CELEvaluationResult

	// Combine request-specific expressions with default expressions
	expressions := append(dm.config.DefaultCELExpressions, request.CELExpressions...)
	if len(expressions) == 0 {
		klog.V(3).InfoS("No CEL expressions to evaluate")
		return allResults, nil
	}

	for _, candidate := range candidates {
		for _, expr := range expressions {
			result := dm.evaluateSingleCELExpression(ctx, expr, candidate, request)
			allResults = append(allResults, result)
		}
	}

	return allResults, nil
}

// evaluateSingleCELExpression evaluates a single CEL expression for a workspace candidate.
func (dm *defaultDecisionMaker) evaluateSingleCELExpression(
	ctx context.Context,
	expr CELExpression,
	candidate *schedulerapi.ScoredCandidate,
	request *PlacementRequest,
) CELEvaluationResult {
	startTime := time.Now()

	result := CELEvaluationResult{
		ExpressionName:  expr.Name,
		Expression:      expr.Expression,
		Workspace:       candidate.Candidate.Workspace,
		EvaluationTime:  0,
		Success:         false,
	}

	// Compile the expression
	compiledExpr, err := dm.celEvaluator.CompileExpression(expr.Expression)
	if err != nil {
		result.Error = fmt.Errorf("failed to compile CEL expression: %w", err)
		result.EvaluationTime = time.Since(startTime)
		return result
	}

	// Create placement context for evaluation
	placementContext := dm.createPlacementContext(candidate, request)

	// Evaluate the expression
	evalResult, err := dm.celEvaluator.EvaluatePlacement(ctx, compiledExpr, placementContext)
	if err != nil {
		result.Error = fmt.Errorf("failed to evaluate CEL expression: %w", err)
		result.EvaluationTime = time.Since(startTime)
		return result
	}

	result.Result = evalResult
	result.Success = true
	result.EvaluationTime = time.Since(startTime)

	// Convert boolean result to score
	if evalResult {
		result.Score = expr.Weight
	} else {
		result.Score = 0.0
	}

	return result
}

// createPlacementContext creates a placement context for CEL evaluation.
func (dm *defaultDecisionMaker) createPlacementContext(
	candidate *schedulerapi.ScoredCandidate,
	request *PlacementRequest,
) *celapi.PlacementContext {
	return &celapi.PlacementContext{
		Workspace: &celapi.WorkspaceContext{
			Name:          candidate.Candidate.Workspace,
			Labels:        candidate.Candidate.Labels,
			Ready:         candidate.Candidate.Ready,
			LastHeartbeat: candidate.Candidate.LastHeartbeat,
		},
		Request: &celapi.RequestContext{
			Name:            request.Name,
			Namespace:       request.Namespace,
			SourceWorkspace: request.SourceWorkspace,
			Labels:          map[string]string{}, // TODO: Convert from User.GetExtra() if needed
			Priority:        int32(request.SchedulerRequest.Priority),
			CreatedAt:       request.CreatedAt,
			Requirements: &celapi.ResourceRequirements{
				CPU:    request.SchedulerRequest.ResourceRequirements.CPU,
				Memory: request.SchedulerRequest.ResourceRequirements.Memory,
				Storage: request.SchedulerRequest.ResourceRequirements.Storage,
			},
		},
		Resources: &celapi.ResourceContext{
			AvailableCapacity: &celapi.ResourceCapacity{
				CPU:     candidate.Candidate.AvailableCapacity.CPU,
				Memory:  candidate.Candidate.AvailableCapacity.Memory,
				Storage: candidate.Candidate.AvailableCapacity.Storage,
			},
			CurrentUtilization: &celapi.ResourceUtilization{
				CPU:     candidate.Candidate.CurrentLoad.CPU,
				Memory:  candidate.Candidate.CurrentLoad.Memory,
				Storage: candidate.Candidate.CurrentLoad.Storage,
			},
		},
	}
}

// applyDecisionAlgorithm applies the configured decision algorithm to select workspaces.
func (dm *defaultDecisionMaker) applyDecisionAlgorithm(
	ctx context.Context,
	request *PlacementRequest,
	candidates []*schedulerapi.ScoredCandidate,
	celResults []CELEvaluationResult,
) ([]*WorkspacePlacement, []*RejectedCandidate, error) {

	// Create workspace evaluation map
	workspaceEvals := make(map[logicalcluster.Name]*workspaceEvaluation)

	for _, candidate := range candidates {
		workspaceEvals[candidate.Candidate.Workspace] = &workspaceEvaluation{
			candidate:    candidate,
			celResults:   []CELEvaluationResult{},
			finalScore:   0.0,
			schedulerScore: candidate.Score,
			celScore:     0.0,
		}
	}

	// Aggregate CEL results by workspace
	for _, celResult := range celResults {
		if eval, exists := workspaceEvals[celResult.Workspace]; exists {
			eval.celResults = append(eval.celResults, celResult)
			eval.celScore += celResult.Score
		}
	}

	// Apply decision algorithm
	switch dm.config.Algorithm {
	case AlgorithmWeightedScore:
		return dm.applyWeightedScoreAlgorithm(workspaceEvals, request)
	case AlgorithmCELPrimary:
		return dm.applyCELPrimaryAlgorithm(workspaceEvals, request)
	case AlgorithmSchedulerPrimary:
		return dm.applySchedulerPrimaryAlgorithm(workspaceEvals, request)
	case AlgorithmConsensus:
		return dm.applyConsensusAlgorithm(workspaceEvals, request)
	default:
		return dm.applyWeightedScoreAlgorithm(workspaceEvals, request)
	}
}

type workspaceEvaluation struct {
	candidate      *schedulerapi.ScoredCandidate
	celResults     []CELEvaluationResult
	finalScore     float64
	schedulerScore float64
	celScore       float64
}

// applyWeightedScoreAlgorithm applies weighted scoring of scheduler and CEL results.
func (dm *defaultDecisionMaker) applyWeightedScoreAlgorithm(
	workspaceEvals map[logicalcluster.Name]*workspaceEvaluation,
	request *PlacementRequest,
) ([]*WorkspacePlacement, []*RejectedCandidate, error) {

	// Calculate final scores
	evaluations := make([]*workspaceEvaluation, 0, len(workspaceEvals))
	for _, eval := range workspaceEvals {
		// Normalize CEL score to 0-100 range
		if eval.celScore > 100 {
			eval.celScore = 100
		}
		
		// Calculate weighted final score
		eval.finalScore = (eval.schedulerScore*dm.config.SchedulerWeight + eval.celScore*dm.config.CELWeight) / 100.0
		evaluations = append(evaluations, eval)
	}

	// Sort by final score (descending)
	sort.Slice(evaluations, func(i, j int) bool {
		return evaluations[i].finalScore > evaluations[j].finalScore
	})

	return dm.selectWorkspaces(evaluations, request)
}

// applyCELPrimaryAlgorithm prioritizes CEL evaluation results over scheduler scores.
func (dm *defaultDecisionMaker) applyCELPrimaryAlgorithm(
	workspaceEvals map[logicalcluster.Name]*workspaceEvaluation,
	request *PlacementRequest,
) ([]*WorkspacePlacement, []*RejectedCandidate, error) {

	evaluations := make([]*workspaceEvaluation, 0, len(workspaceEvals))
	for _, eval := range workspaceEvals {
		// Use CEL score as primary, scheduler score as tiebreaker
		eval.finalScore = eval.celScore*10 + eval.schedulerScore/10
		evaluations = append(evaluations, eval)
	}

	// Sort by final score (descending)
	sort.Slice(evaluations, func(i, j int) bool {
		return evaluations[i].finalScore > evaluations[j].finalScore
	})

	return dm.selectWorkspaces(evaluations, request)
}

// applySchedulerPrimaryAlgorithm prioritizes scheduler scores over CEL evaluation.
func (dm *defaultDecisionMaker) applySchedulerPrimaryAlgorithm(
	workspaceEvals map[logicalcluster.Name]*workspaceEvaluation,
	request *PlacementRequest,
) ([]*WorkspacePlacement, []*RejectedCandidate, error) {

	evaluations := make([]*workspaceEvaluation, 0, len(workspaceEvals))
	for _, eval := range workspaceEvals {
		// Use scheduler score as primary, CEL score as tiebreaker
		eval.finalScore = eval.schedulerScore*10 + eval.celScore/10
		evaluations = append(evaluations, eval)
	}

	// Sort by final score (descending)
	sort.Slice(evaluations, func(i, j int) bool {
		return evaluations[i].finalScore > evaluations[j].finalScore
	})

	return dm.selectWorkspaces(evaluations, request)
}

// applyConsensusAlgorithm requires both scheduler and CEL to agree on selections.
func (dm *defaultDecisionMaker) applyConsensusAlgorithm(
	workspaceEvals map[logicalcluster.Name]*workspaceEvaluation,
	request *PlacementRequest,
) ([]*WorkspacePlacement, []*RejectedCandidate, error) {

	evaluations := make([]*workspaceEvaluation, 0, len(workspaceEvals))
	for _, eval := range workspaceEvals {
		// Both scheduler and CEL must score above minimum threshold
		minThreshold := dm.config.MinimumScore
		if eval.schedulerScore >= minThreshold && eval.celScore >= minThreshold {
			eval.finalScore = (eval.schedulerScore + eval.celScore) / 2.0
		} else {
			eval.finalScore = 0.0 // Reject if either fails
		}
		evaluations = append(evaluations, eval)
	}

	// Sort by final score (descending)
	sort.Slice(evaluations, func(i, j int) bool {
		return evaluations[i].finalScore > evaluations[j].finalScore
	})

	return dm.selectWorkspaces(evaluations, request)
}

// selectWorkspaces selects the appropriate workspaces based on sorted evaluations.
func (dm *defaultDecisionMaker) selectWorkspaces(
	evaluations []*workspaceEvaluation,
	request *PlacementRequest,
) ([]*WorkspacePlacement, []*RejectedCandidate, error) {

	var selectedWorkspaces []*WorkspacePlacement
	var rejectedCandidates []*RejectedCandidate

	maxPlacements := request.SchedulerRequest.MaxPlacements
	if maxPlacements == 0 {
		maxPlacements = len(evaluations) // No limit
	}

	selectedCount := 0
	for _, eval := range evaluations {
		workspace := eval.candidate.Candidate.Workspace
		
		if selectedCount < maxPlacements && eval.finalScore >= dm.config.MinimumScore {
			// Select this workspace
			placement := &WorkspacePlacement{
				Workspace:      workspace,
				SchedulerScore: eval.schedulerScore,
				CELScore:      eval.celScore,
				FinalScore:    eval.finalScore,
				AllocatedResources: schedulerapi.ResourceAllocation{
					CPU:           request.SchedulerRequest.ResourceRequirements.CPU,
					Memory:        request.SchedulerRequest.ResourceRequirements.Memory,
					Storage:       request.SchedulerRequest.ResourceRequirements.Storage,
					ReservationID: string(uuid.NewUUID()),
					ExpiresAt:     time.Now().Add(30 * time.Minute),
				},
				SelectionReason: fmt.Sprintf("Selected with final score %.2f (scheduler: %.2f, CEL: %.2f)", 
					eval.finalScore, eval.schedulerScore, eval.celScore),
				CELResults: eval.celResults,
			}
			selectedWorkspaces = append(selectedWorkspaces, placement)
			selectedCount++
		} else {
			// Reject this workspace
			reason := "Score below minimum threshold"
			if selectedCount >= maxPlacements {
				reason = "Maximum placements reached"
			}
			
			rejected := &RejectedCandidate{
				Workspace:       workspace,
				SchedulerScore:  eval.schedulerScore,
				CELScore:       eval.celScore,
				FinalScore:     eval.finalScore,
				RejectionReason: reason,
				CELResults:     eval.celResults,
			}
			rejectedCandidates = append(rejectedCandidates, rejected)
		}
	}

	return selectedWorkspaces, rejectedCandidates, nil
}

// generateDecisionRationale generates detailed reasoning for the placement decision.
func (dm *defaultDecisionMaker) generateDecisionRationale(
	decision *PlacementDecision,
	request *PlacementRequest,
	candidates []*schedulerapi.ScoredCandidate,
	celResults []CELEvaluationResult,
) {
	rationale := &decision.DecisionRationale

	rationale.Summary = fmt.Sprintf("Selected %d of %d candidate workspaces using %s algorithm",
		len(decision.SelectedWorkspaces), len(candidates), rationale.DecisionAlgorithm)

	// Add scheduler factors
	rationale.SchedulerFactors = []string{
		fmt.Sprintf("Evaluated %d workspace candidates", len(candidates)),
		fmt.Sprintf("Scheduler weight: %.1f%%", dm.config.SchedulerWeight),
	}

	// Add CEL factors
	celExprCount := len(request.CELExpressions) + len(dm.config.DefaultCELExpressions)
	if celExprCount > 0 {
		rationale.CELFactors = []string{
			fmt.Sprintf("Evaluated %d CEL expressions", celExprCount),
			fmt.Sprintf("CEL weight: %.1f%%", dm.config.CELWeight),
			fmt.Sprintf("Total CEL evaluations: %d", len(celResults)),
		}
	}

	if decision.Override != nil {
		rationale.OverrideFactors = []string{
			fmt.Sprintf("Applied %s override: %s", decision.Override.OverrideType, decision.Override.Reason),
		}
	}
}

// recordEvent records a decision event for audit purposes.
func (dm *defaultDecisionMaker) recordEvent(
	ctx context.Context,
	decision *PlacementDecision,
	eventType DecisionEventType,
	message string,
	details map[string]interface{},
) {
	if dm.recorder != nil {
		event := DecisionEvent{
			Type:      eventType,
			Timestamp: time.Now(),
			Message:   message,
			Details:   details,
		}
		
		// This is a best-effort operation, don't fail the decision on record errors
		if err := dm.recorder.RecordEvent(ctx, decision.ID, event); err != nil {
			klog.V(3).InfoS("Failed to record decision event", "error", err)
		}
	}
}

// ValidateDecision validates a placement decision against constraints and policies.
func (dm *defaultDecisionMaker) ValidateDecision(ctx context.Context, decision *PlacementDecision) error {
	if dm.validator != nil {
		return dm.validator.ValidateDecision(ctx, decision)
	}
	return nil
}

// RecordDecision records a placement decision for audit and debugging purposes.
func (dm *defaultDecisionMaker) RecordDecision(ctx context.Context, decision *PlacementDecision) error {
	if dm.recorder != nil {
		return dm.recorder.RecordDecision(ctx, decision)
	}
	return nil
}

// GetDecisionHistory returns the decision history for a specific placement.
func (dm *defaultDecisionMaker) GetDecisionHistory(ctx context.Context, placementID string) ([]*DecisionRecord, error) {
	if dm.recorder != nil {
		return dm.recorder.GetDecisionHistory(ctx, placementID)
	}
	return []*DecisionRecord{}, nil
}

// ApplyOverride applies manual placement overrides to a decision.
func (dm *defaultDecisionMaker) ApplyOverride(
	ctx context.Context,
	decision *PlacementDecision,
	override *PlacementOverride,
) (*PlacementDecision, error) {
	klog.V(2).InfoS("Applying placement override",
		"decisionID", decision.ID,
		"overrideID", override.ID,
		"overrideType", override.OverrideType)

	// Clone the decision to avoid modifying the original
	modifiedDecision := *decision
	modifiedDecision.Override = override
	modifiedDecision.Status = DecisionStatusOverridden

	switch override.OverrideType {
	case OverrideTypeForce:
		err := dm.applyForceOverride(&modifiedDecision, override)
		if err != nil {
			return nil, fmt.Errorf("failed to apply force override: %w", err)
		}
	case OverrideTypeExclude:
		dm.applyExcludeOverride(&modifiedDecision, override)
	case OverrideTypePrefer:
		dm.applyPreferOverride(&modifiedDecision, override)
	case OverrideTypeAvoid:
		dm.applyAvoidOverride(&modifiedDecision, override)
	default:
		return nil, fmt.Errorf("unknown override type: %s", override.OverrideType)
	}

	// Add override factor to rationale
	modifiedDecision.DecisionRationale.OverrideFactors = append(
		modifiedDecision.DecisionRationale.OverrideFactors,
		fmt.Sprintf("Applied %s override by %s: %s", override.OverrideType, override.AppliedBy, override.Reason),
	)

	dm.recordEvent(ctx, &modifiedDecision, DecisionEventTypeOverrideApplied,
		fmt.Sprintf("Applied %s override", override.OverrideType),
		map[string]interface{}{"overrideType": string(override.OverrideType), "reason": override.Reason})

	return &modifiedDecision, nil
}

// applyForceOverride forces placement to specific workspaces.
func (dm *defaultDecisionMaker) applyForceOverride(decision *PlacementDecision, override *PlacementOverride) error {
	if len(override.TargetWorkspaces) == 0 {
		return fmt.Errorf("force override requires target workspaces")
	}

	// Clear existing selections and create new ones for target workspaces
	decision.SelectedWorkspaces = []*WorkspacePlacement{}
	decision.RejectedCandidates = []*RejectedCandidate{}

	for _, workspace := range override.TargetWorkspaces {
		placement := &WorkspacePlacement{
			Workspace:         workspace,
			SchedulerScore:    100.0, // Max score for forced placements
			CELScore:         100.0,
			FinalScore:       100.0,
			SelectionReason:   fmt.Sprintf("Forced by override: %s", override.Reason),
			CELResults:       []CELEvaluationResult{},
		}
		decision.SelectedWorkspaces = append(decision.SelectedWorkspaces, placement)
	}

	return nil
}

// applyExcludeOverride excludes specific workspaces from placement.
func (dm *defaultDecisionMaker) applyExcludeOverride(decision *PlacementDecision, override *PlacementOverride) {
	excludeMap := make(map[logicalcluster.Name]bool)
	for _, workspace := range override.ExcludedWorkspaces {
		excludeMap[workspace] = true
	}

	// Move excluded workspaces from selected to rejected
	var newSelected []*WorkspacePlacement
	for _, placement := range decision.SelectedWorkspaces {
		if excludeMap[placement.Workspace] {
			rejected := &RejectedCandidate{
				Workspace:       placement.Workspace,
				SchedulerScore:  placement.SchedulerScore,
				CELScore:       placement.CELScore,
				FinalScore:     placement.FinalScore,
				RejectionReason: fmt.Sprintf("Excluded by override: %s", override.Reason),
				CELResults:     placement.CELResults,
			}
			decision.RejectedCandidates = append(decision.RejectedCandidates, rejected)
		} else {
			newSelected = append(newSelected, placement)
		}
	}
	decision.SelectedWorkspaces = newSelected
}

// applyPreferOverride adds preference for specific workspaces.
func (dm *defaultDecisionMaker) applyPreferOverride(decision *PlacementDecision, override *PlacementOverride) {
	preferMap := make(map[logicalcluster.Name]bool)
	for _, workspace := range override.TargetWorkspaces {
		preferMap[workspace] = true
	}

	// Boost scores for preferred workspaces
	for _, placement := range decision.SelectedWorkspaces {
		if preferMap[placement.Workspace] {
			placement.FinalScore = min(placement.FinalScore*1.2, 100.0) // 20% bonus, capped at 100
			placement.SelectionReason += fmt.Sprintf(" (preferred by override: %s)", override.Reason)
		}
	}

	// Re-sort by final score
	sort.Slice(decision.SelectedWorkspaces, func(i, j int) bool {
		return decision.SelectedWorkspaces[i].FinalScore > decision.SelectedWorkspaces[j].FinalScore
	})
}

// applyAvoidOverride reduces preference for specific workspaces.
func (dm *defaultDecisionMaker) applyAvoidOverride(decision *PlacementDecision, override *PlacementOverride) {
	avoidMap := make(map[logicalcluster.Name]bool)
	for _, workspace := range override.TargetWorkspaces {
		avoidMap[workspace] = true
	}

	// Reduce scores for avoided workspaces
	for _, placement := range decision.SelectedWorkspaces {
		if avoidMap[placement.Workspace] {
			placement.FinalScore *= 0.8 // 20% penalty
			placement.SelectionReason += fmt.Sprintf(" (avoided by override: %s)", override.Reason)
		}
	}

	// Re-sort by final score
	sort.Slice(decision.SelectedWorkspaces, func(i, j int) bool {
		return decision.SelectedWorkspaces[i].FinalScore > decision.SelectedWorkspaces[j].FinalScore
	})
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}