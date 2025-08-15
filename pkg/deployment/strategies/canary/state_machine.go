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

package canary

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
)

// CanaryState represents the current state of a canary deployment.
type CanaryState string

const (
	// StateInitializing indicates the canary deployment is being set up.
	StateInitializing CanaryState = "Initializing"
	// StateProgressing indicates the canary is actively rolling out.
	StateProgressing CanaryState = "Progressing"
	// StateAnalyzing indicates the canary is being analyzed for metrics.
	StateAnalyzing CanaryState = "Analyzing"
	// StatePromoting indicates the canary is being promoted to stable.
	StatePromoting CanaryState = "Promoting"
	// StateCompleted indicates the canary deployment completed successfully.
	StateCompleted CanaryState = "Completed"
	// StateRollingBack indicates the canary is being rolled back due to failure.
	StateRollingBack CanaryState = "RollingBack"
	// StateFailed indicates the canary deployment failed.
	StateFailed CanaryState = "Failed"
)

// CanaryEvent represents an event that can trigger state transitions.
type CanaryEvent string

const (
	// EventInitialize starts the canary deployment process.
	EventInitialize CanaryEvent = "Initialize"
	// EventProgress moves the canary to the next traffic percentage.
	EventProgress CanaryEvent = "Progress"
	// EventAnalyze triggers analysis of canary metrics.
	EventAnalyze CanaryEvent = "Analyze"
	// EventPromote promotes the canary to production.
	EventPromote CanaryEvent = "Promote"
	// EventRollback triggers rollback of the canary deployment.
	EventRollback CanaryEvent = "Rollback"
	// EventComplete marks the canary as successfully completed.
	EventComplete CanaryEvent = "Complete"
	// EventFail marks the canary as failed.
	EventFail CanaryEvent = "Fail"
)

// StateMachine manages canary deployment state transitions.
type StateMachine struct {
	currentState   CanaryState
	validTransitions map[CanaryState]sets.Set[CanaryState]
}

// NewStateMachine creates a new canary state machine.
func NewStateMachine() *StateMachine {
	sm := &StateMachine{
		currentState:   StateInitializing,
		validTransitions: make(map[CanaryState]sets.Set[CanaryState]),
	}
	sm.initializeTransitions()
	return sm
}

// initializeTransitions sets up valid state transitions for the canary deployment.
func (sm *StateMachine) initializeTransitions() {
	sm.validTransitions[StateInitializing] = sets.New(StateProgressing, StateFailed)
	sm.validTransitions[StateProgressing] = sets.New(StateAnalyzing, StateRollingBack, StateFailed)
	sm.validTransitions[StateAnalyzing] = sets.New(StateProgressing, StatePromoting, StateRollingBack, StateFailed)
	sm.validTransitions[StatePromoting] = sets.New(StateCompleted, StateFailed)
	sm.validTransitions[StateRollingBack] = sets.New(StateFailed, StateCompleted)
	sm.validTransitions[StateCompleted] = sets.New[CanaryState]() // Terminal state
	sm.validTransitions[StateFailed] = sets.New[CanaryState]() // Terminal state
}

// GetCurrentState returns the current state of the canary.
func (sm *StateMachine) GetCurrentState() CanaryState {
	return sm.currentState
}

// CanTransitionTo checks if a transition to the target state is valid.
func (sm *StateMachine) CanTransitionTo(targetState CanaryState) bool {
	validStates, exists := sm.validTransitions[sm.currentState]
	if !exists {
		return false
	}
	return validStates.Has(targetState)
}

// TransitionTo attempts to transition to a new state.
func (sm *StateMachine) TransitionTo(ctx context.Context, targetState CanaryState, reason string) error {
	if !sm.CanTransitionTo(targetState) {
		return fmt.Errorf("invalid state transition from %s to %s", sm.currentState, targetState)
	}

	klog.FromContext(ctx).V(2).Info("Canary state transition",
		"from", sm.currentState,
		"to", targetState,
		"reason", reason,
	)

	sm.currentState = targetState
	return nil
}

// IsTerminal returns true if the current state is terminal.
func (sm *StateMachine) IsTerminal() bool {
	return sm.currentState == StateCompleted || sm.currentState == StateFailed
}

// GetValidNextStates returns the set of valid next states from current state.
func (sm *StateMachine) GetValidNextStates() sets.Set[CanaryState] {
	if validStates, exists := sm.validTransitions[sm.currentState]; exists {
		return validStates.Clone()
	}
	return sets.New[CanaryState]()
}