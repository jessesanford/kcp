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
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

// TestStateMachine tests the canary state machine transitions.
func TestStateMachine(t *testing.T) {
	tests := map[string]struct {
		startState    CanaryState
		targetState   CanaryState
		shouldSucceed bool
	}{
		"valid initialization": {
			startState:    StateInitializing,
			targetState:   StateProgressing,
			shouldSucceed: true,
		},
		"valid analysis transition": {
			startState:    StateAnalyzing,
			targetState:   StateProgressing,
			shouldSucceed: true,
		},
		"invalid terminal transition": {
			startState:    StateCompleted,
			targetState:   StateProgressing,
			shouldSucceed: false,
		},
		"valid rollback": {
			startState:    StateAnalyzing,
			targetState:   StateRollingBack,
			shouldSucceed: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			sm := NewStateMachine()
			sm.currentState = tc.startState

			ctx := context.Background()
			err := sm.TransitionTo(ctx, tc.targetState, "test transition")

			if tc.shouldSucceed && err != nil {
				t.Errorf("Expected transition to succeed, got error: %v", err)
			}
			if !tc.shouldSucceed && err == nil {
				t.Errorf("Expected transition to fail, but it succeeded")
			}
			if tc.shouldSucceed && sm.GetCurrentState() != tc.targetState {
				t.Errorf("Expected state %s, got %s", tc.targetState, sm.GetCurrentState())
			}
		})
	}
}

// TestStateMachineValidTransitions tests valid next states.
func TestStateMachineValidTransitions(t *testing.T) {
	tests := map[string]struct {
		state                CanaryState
		expectedValidStates  sets.Set[CanaryState]
	}{
		"initializing state": {
			state:               StateInitializing,
			expectedValidStates: sets.New(StateProgressing, StateFailed),
		},
		"progressing state": {
			state:               StateProgressing,
			expectedValidStates: sets.New(StateAnalyzing, StateRollingBack, StateFailed),
		},
		"completed state": {
			state:               StateCompleted,
			expectedValidStates: sets.New[CanaryState](),
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			sm := NewStateMachine()
			sm.currentState = tc.state

			validStates := sm.GetValidNextStates()
			if !validStates.Equal(tc.expectedValidStates) {
				t.Errorf("Expected valid states %v, got %v", tc.expectedValidStates, validStates)
			}
		})
	}
}

// TestAnalysisEngine tests the metrics analysis functionality.
func TestAnalysisEngine(t *testing.T) {
	engine := NewDefaultAnalysisEngine()

	tests := map[string]struct {
		metrics          *Metrics
		canary           *CanaryDeployment
		expectedDecision AnalysisDecision
	}{
		"healthy metrics continue": {
			metrics: &Metrics{
				SuccessRate: TimeSeries{
					DataPoints: []DataPoint{
						{Timestamp: time.Now(), Value: 0.98},
						{Timestamp: time.Now().Add(-time.Minute), Value: 0.97},
					},
				},
				ErrorRate: TimeSeries{
					DataPoints: []DataPoint{
						{Timestamp: time.Now(), Value: 0.01},
					},
				},
				RequestCount: TimeSeries{
					DataPoints: generateTestDataPoints(15),
				},
			},
			canary: &CanaryDeployment{
				Spec: CanaryDeploymentSpec{
					SuccessThreshold:  0.95,
					RollbackThreshold: 0.05,
				},
			},
			expectedDecision: AnalysisDecisionContinue,
		},
		"poor metrics rollback": {
			metrics: &Metrics{
				SuccessRate: TimeSeries{
					DataPoints: []DataPoint{
						{Timestamp: time.Now(), Value: 0.80},
					},
				},
				ErrorRate: TimeSeries{
					DataPoints: []DataPoint{
						{Timestamp: time.Now(), Value: 0.10},
					},
				},
				RequestCount: TimeSeries{
					DataPoints: generateTestDataPoints(15),
				},
			},
			canary: &CanaryDeployment{
				Spec: CanaryDeploymentSpec{
					SuccessThreshold:  0.95,
					RollbackThreshold: 0.05,
				},
			},
			expectedDecision: AnalysisDecisionRollback,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			result, err := engine.AnalyzeMetrics(ctx, tc.metrics, tc.canary)
			if err != nil {
				t.Fatalf("Analysis failed: %v", err)
			}

			if result.Decision != tc.expectedDecision {
				t.Errorf("Expected decision %s, got %s", tc.expectedDecision, result.Decision)
			}
		})
	}
}

// TestTrafficManager tests traffic management functionality.
func TestTrafficManager(t *testing.T) {
	// Mock implementation for testing
	tm := &mockTrafficManager{}
	
	tests := map[string]struct {
		weight       int32
		shouldFail   bool
	}{
		"valid weight": {
			weight:     50,
			shouldFail: false,
		},
		"invalid weight high": {
			weight:     150,
			shouldFail: true,
		},
		"invalid weight low": {
			weight:     -10,
			shouldFail: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			canary := &CanaryDeployment{
				ObjectMeta: metav1.ObjectMeta{Name: "test-canary"},
			}

			err := tm.SetTrafficWeight(ctx, canary, tc.weight)
			if tc.shouldFail && err == nil {
				t.Error("Expected SetTrafficWeight to fail, but it succeeded")
			}
			if !tc.shouldFail && err != nil {
				t.Errorf("Expected SetTrafficWeight to succeed, got error: %v", err)
			}
		})
	}
}

// mockTrafficManager is a mock implementation for testing.
type mockTrafficManager struct{}

func (m *mockTrafficManager) SetTrafficWeight(ctx context.Context, canary *CanaryDeployment, weight int32) error {
	if weight < 0 || weight > 100 {
		return fmt.Errorf("invalid weight: %d", weight)
	}
	return nil
}

func (m *mockTrafficManager) GetCurrentTrafficWeight(ctx context.Context, canary *CanaryDeployment) (int32, error) {
	return canary.Status.CurrentTrafficPercentage, nil
}

func (m *mockTrafficManager) ValidateTrafficConfiguration(ctx context.Context, canary *CanaryDeployment) error {
	return nil
}

// generateTestDataPoints creates test data points for metrics testing.
func generateTestDataPoints(count int) []DataPoint {
	points := make([]DataPoint, count)
	base := time.Now()
	for i := 0; i < count; i++ {
		points[i] = DataPoint{
			Timestamp: base.Add(-time.Duration(i) * time.Minute),
			Value:     float64(100 + i),
		}
	}
	return points
}