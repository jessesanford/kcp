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

package canary

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	deploymentv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/deployment/v1alpha1"
	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
)

// canaryStateManager implements StateManager for managing canary deployment states.
type canaryStateManager struct {
	config        *CanaryConfiguration
	analyzer      MetricsAnalyzer
	trafficMgr    TrafficManager
}

// NewStateManager creates a new state manager for canary deployments.
func NewStateManager(config *CanaryConfiguration, analyzer MetricsAnalyzer, trafficMgr TrafficManager) StateManager {
	return &canaryStateManager{
		config:     config,
		analyzer:   analyzer,
		trafficMgr: trafficMgr,
	}
}

// GetCurrentState returns the current state of the canary deployment.
func (sm *canaryStateManager) GetCurrentState(canary *deploymentv1alpha1.CanaryDeployment) CanaryState {
	state := CanaryState{
		Phase:              canary.Status.Phase,
		Step:               canary.Status.CurrentStep,
		Message:            canary.Status.Message,
		LastTransitionTime: metav1.Now(),
	}

	if canary.Status.StepStartTime != nil {
		state.StepStartTime = *canary.Status.StepStartTime
	}

	// If phase is empty, default to Pending
	if state.Phase == "" {
		state.Phase = deploymentv1alpha1.CanaryPhasePending
	}

	return state
}

// TransitionTo attempts to transition the canary to a new state.
func (sm *canaryStateManager) TransitionTo(ctx context.Context, canary *deploymentv1alpha1.CanaryDeployment, newState CanaryState) error {
	currentState := sm.GetCurrentState(canary)
	
	// Validate the transition
	if err := sm.validateTransition(currentState, newState); err != nil {
		return fmt.Errorf("invalid state transition from %s to %s: %w", currentState.Phase, newState.Phase, err)
	}

	klog.V(2).Infof("Transitioning canary %s/%s from %s to %s", 
		canary.Namespace, canary.Name, currentState.Phase, newState.Phase)

	// Update the canary status
	canary.Status.Phase = newState.Phase
	canary.Status.CurrentStep = newState.Step
	canary.Status.Message = newState.Message

	// Update step start time for new steps
	if newState.Step != currentState.Step || newState.Phase != currentState.Phase {
		now := metav1.Now()
		canary.Status.StepStartTime = &now
	}

	// Update conditions based on the new state
	if err := sm.updateConditions(canary, currentState, newState); err != nil {
		return fmt.Errorf("failed to update conditions: %w", err)
	}

	// Perform state-specific actions
	if err := sm.performStateActions(ctx, canary, newState); err != nil {
		return fmt.Errorf("failed to perform actions for state %s: %w", newState.Phase, err)
	}

	return nil
}

// ShouldTransition checks if the canary should transition to the next state.
func (sm *canaryStateManager) ShouldTransition(ctx context.Context, canary *deploymentv1alpha1.CanaryDeployment) (bool, CanaryState, error) {
	currentState := sm.GetCurrentState(canary)
	
	switch currentState.Phase {
	case deploymentv1alpha1.CanaryPhasePending:
		return sm.shouldTransitionFromPending(ctx, canary, currentState)
		
	case deploymentv1alpha1.CanaryPhaseProgressing:
		return sm.shouldTransitionFromProgressing(ctx, canary, currentState)
		
	case deploymentv1alpha1.CanaryPhaseAnalyzing:
		return sm.shouldTransitionFromAnalyzing(ctx, canary, currentState)
		
	case deploymentv1alpha1.CanaryPhasePromoting:
		return sm.shouldTransitionFromPromoting(ctx, canary, currentState)
		
	case deploymentv1alpha1.CanaryPhaseSucceeded, deploymentv1alpha1.CanaryPhaseFailed:
		// Terminal states - no transitions
		return false, currentState, nil
		
	default:
		return false, currentState, fmt.Errorf("unknown canary phase: %s", currentState.Phase)
	}
}

// shouldTransitionFromPending determines if the canary should move from Pending state.
func (sm *canaryStateManager) shouldTransitionFromPending(ctx context.Context, canary *deploymentv1alpha1.CanaryDeployment, currentState CanaryState) (bool, CanaryState, error) {
	// Check if we have valid configuration
	if len(canary.Spec.Strategy.Steps) == 0 || len(canary.Spec.Analysis.MetricQueries) == 0 {
		return true, CanaryState{
			Phase:   deploymentv1alpha1.CanaryPhaseFailed,
			Message: "Invalid canary configuration: missing steps or metric queries",
		}, nil
	}

	// Transition to progressing to start the first step
	nextState := CanaryState{
		Phase:   deploymentv1alpha1.CanaryPhaseProgressing,
		Step:    0,
		Message: fmt.Sprintf("Starting canary rollout, step 1/%d", len(canary.Spec.Strategy.Steps)),
	}

	return true, nextState, nil
}

// shouldTransitionFromProgressing determines if the canary should move from Progressing state.
func (sm *canaryStateManager) shouldTransitionFromProgressing(ctx context.Context, canary *deploymentv1alpha1.CanaryDeployment, currentState CanaryState) (bool, CanaryState, error) {
	// Check if we've waited long enough for the current step
	stepDuration := sm.config.DefaultStepDuration
	if canary.Spec.Strategy.StepDuration != nil {
		stepDuration = canary.Spec.Strategy.StepDuration.Duration
	}

	if time.Since(currentState.StepStartTime.Time) < stepDuration {
		return false, currentState, nil
	}

	// Transition to analyzing phase to check metrics
	nextState := CanaryState{
		Phase:   deploymentv1alpha1.CanaryPhaseAnalyzing,
		Step:    currentState.Step,
		Message: fmt.Sprintf("Analyzing metrics for step %d", currentState.Step+1),
	}

	return true, nextState, nil
}

// shouldTransitionFromAnalyzing determines if the canary should move from Analyzing state.
func (sm *canaryStateManager) shouldTransitionFromAnalyzing(ctx context.Context, canary *deploymentv1alpha1.CanaryDeployment, currentState CanaryState) (bool, CanaryState, error) {
	// Perform metrics analysis
	analysisResults, err := sm.analyzer.AnalyzeMetrics(ctx, canary)
	if err != nil {
		return true, CanaryState{
			Phase:   deploymentv1alpha1.CanaryPhaseFailed,
			Message: fmt.Errorf("metrics analysis failed: %w", err).Error(),
		}, nil
	}

	// Update the canary status with analysis results
	canary.Status.AnalysisResults = analysisResults
	canary.Status.LastAnalysisTime = &metav1.Time{Time: time.Now()}

	// Calculate overall success
	totalWeight := 0
	passedWeight := 0
	for _, result := range analysisResults {
		totalWeight += result.Weight
		if result.Passed {
			passedWeight += result.Weight
		}
	}

	threshold := sm.config.DefaultSuccessThreshold
	if canary.Spec.Analysis.Threshold != nil {
		threshold = *canary.Spec.Analysis.Threshold
	}

	successPercentage := 0
	if totalWeight > 0 {
		successPercentage = (passedWeight * 100) / totalWeight
	}

	if successPercentage >= threshold {
		// Analysis passed - move to promoting
		nextState := CanaryState{
			Phase:   deploymentv1alpha1.CanaryPhasePromoting,
			Step:    currentState.Step,
			Message: fmt.Sprintf("Analysis passed (%d%% >= %d%%), promoting to next step", successPercentage, threshold),
		}
		return true, nextState, nil
	} else {
		// Analysis failed - rollback
		nextState := CanaryState{
			Phase:   deploymentv1alpha1.CanaryPhaseFailed,
			Message: fmt.Sprintf("Analysis failed (%d%% < %d%%), initiating rollback", successPercentage, threshold),
		}
		return true, nextState, nil
	}
}

// shouldTransitionFromPromoting determines if the canary should move from Promoting state.
func (sm *canaryStateManager) shouldTransitionFromPromoting(ctx context.Context, canary *deploymentv1alpha1.CanaryDeployment, currentState CanaryState) (bool, CanaryState, error) {
	// Check if we're at the final step
	if currentState.Step >= len(canary.Spec.Strategy.Steps)-1 {
		// Final step completed successfully
		nextState := CanaryState{
			Phase:   deploymentv1alpha1.CanaryPhaseSucceeded,
			Step:    currentState.Step,
			Message: "Canary rollout completed successfully",
		}
		return true, nextState, nil
	}

	// Move to the next step
	nextStep := currentState.Step + 1
	nextState := CanaryState{
		Phase:   deploymentv1alpha1.CanaryPhaseProgressing,
		Step:    nextStep,
		Message: fmt.Sprintf("Progressing to step %d/%d (%d%% traffic)", nextStep+1, len(canary.Spec.Strategy.Steps), canary.Spec.Strategy.Steps[nextStep]),
	}

	return true, nextState, nil
}

// validateTransition validates whether a state transition is allowed.
func (sm *canaryStateManager) validateTransition(from, to CanaryState) error {
	// Define allowed transitions
	allowedTransitions := map[deploymentv1alpha1.CanaryPhase][]deploymentv1alpha1.CanaryPhase{
		deploymentv1alpha1.CanaryPhasePending:      {deploymentv1alpha1.CanaryPhaseProgressing, deploymentv1alpha1.CanaryPhaseFailed},
		deploymentv1alpha1.CanaryPhaseProgressing:  {deploymentv1alpha1.CanaryPhaseAnalyzing, deploymentv1alpha1.CanaryPhaseFailed, deploymentv1alpha1.CanaryPhaseRollingBack},
		deploymentv1alpha1.CanaryPhaseAnalyzing:    {deploymentv1alpha1.CanaryPhasePromoting, deploymentv1alpha1.CanaryPhaseFailed, deploymentv1alpha1.CanaryPhaseRollingBack},
		deploymentv1alpha1.CanaryPhasePromoting:    {deploymentv1alpha1.CanaryPhaseProgressing, deploymentv1alpha1.CanaryPhaseSucceeded, deploymentv1alpha1.CanaryPhaseFailed},
		deploymentv1alpha1.CanaryPhaseRollingBack:  {deploymentv1alpha1.CanaryPhaseFailed},
		deploymentv1alpha1.CanaryPhaseSucceeded:    {}, // Terminal state
		deploymentv1alpha1.CanaryPhaseFailed:       {}, // Terminal state
	}

	allowedNextStates, exists := allowedTransitions[from.Phase]
	if !exists {
		return fmt.Errorf("unknown source phase: %s", from.Phase)
	}

	for _, allowed := range allowedNextStates {
		if to.Phase == allowed {
			return nil
		}
	}

	return fmt.Errorf("transition from %s to %s is not allowed", from.Phase, to.Phase)
}

// updateConditions updates the conditions on the canary based on state transitions.
func (sm *canaryStateManager) updateConditions(canary *deploymentv1alpha1.CanaryDeployment, from, to CanaryState) error {
	now := metav1.Now()
	
	switch to.Phase {
	case deploymentv1alpha1.CanaryPhaseProgressing:
		conditionsv1alpha1.Set(&canary.Status.Conditions, conditionsv1alpha1.Condition{
			Type:               "Progressing",
			Status:             metav1.ConditionTrue,
			LastTransitionTime: now,
			Reason:             "CanaryProgressing",
			Message:            to.Message,
		})
		
	case deploymentv1alpha1.CanaryPhaseAnalyzing:
		conditionsv1alpha1.Set(&canary.Status.Conditions, conditionsv1alpha1.Condition{
			Type:               "Analyzing",
			Status:             metav1.ConditionTrue,
			LastTransitionTime: now,
			Reason:             "AnalyzingMetrics",
			Message:            to.Message,
		})
		
	case deploymentv1alpha1.CanaryPhaseSucceeded:
		conditionsv1alpha1.Set(&canary.Status.Conditions, conditionsv1alpha1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionTrue,
			LastTransitionTime: now,
			Reason:             "CanarySucceeded",
			Message:            to.Message,
		})
		
	case deploymentv1alpha1.CanaryPhaseFailed:
		conditionsv1alpha1.Set(&canary.Status.Conditions, conditionsv1alpha1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			LastTransitionTime: now,
			Reason:             "CanaryFailed",
			Message:            to.Message,
		})
	}
	
	return nil
}

// performStateActions performs any actions required when entering a new state.
func (sm *canaryStateManager) performStateActions(ctx context.Context, canary *deploymentv1alpha1.CanaryDeployment, state CanaryState) error {
	switch state.Phase {
	case deploymentv1alpha1.CanaryPhaseProgressing:
		// Update traffic weights for the current step
		if state.Step < len(canary.Spec.Strategy.Steps) {
			trafficPercentage := canary.Spec.Strategy.Steps[state.Step]
			if err := sm.trafficMgr.SetTrafficWeight(ctx, canary, trafficPercentage); err != nil {
				return fmt.Errorf("failed to set traffic weight to %d%%: %w", trafficPercentage, err)
			}
			canary.Spec.TrafficPercentage = trafficPercentage
		}
		
	case deploymentv1alpha1.CanaryPhaseSucceeded:
		// Set traffic to 100% canary
		if err := sm.trafficMgr.SetTrafficWeight(ctx, canary, 100); err != nil {
			klog.Errorf("Failed to set final traffic weight for successful canary %s/%s: %v", canary.Namespace, canary.Name, err)
		}
		canary.Spec.TrafficPercentage = 100
		
	case deploymentv1alpha1.CanaryPhaseFailed, deploymentv1alpha1.CanaryPhaseRollingBack:
		// Rollback traffic to stable version
		if err := sm.trafficMgr.SetTrafficWeight(ctx, canary, 0); err != nil {
			klog.Errorf("Failed to rollback traffic for failed canary %s/%s: %v", canary.Namespace, canary.Name, err)
		}
		canary.Spec.TrafficPercentage = 0
	}
	
	return nil
}