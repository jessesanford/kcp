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
	"testing"
	"time"

	"github.com/google/cel-go/cel"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/kcp-dev/logicalcluster/v3"

	celapi "github.com/kcp-dev/kcp/pkg/placement/cel"
	schedulerapi "github.com/kcp-dev/kcp/pkg/placement/scheduler"
)

// mockCELEvaluator is a mock implementation of CELEvaluator for testing.
type mockCELEvaluator struct {
	compileResult *celapi.CompiledExpression
	compileError  error
	evaluateResult bool
	evaluateError  error
}

func (m *mockCELEvaluator) CompileExpression(expr string) (*celapi.CompiledExpression, error) {
	if m.compileError != nil {
		return nil, m.compileError
	}
	if m.compileResult != nil {
		return m.compileResult, nil
	}
	return &celapi.CompiledExpression{
		Expression: expr,
		Hash:       "mock-hash",
		CompiledAt: time.Now(),
	}, nil
}

func (m *mockCELEvaluator) EvaluatePlacement(ctx context.Context, expr *celapi.CompiledExpression, placement *celapi.PlacementContext) (bool, error) {
	if m.evaluateError != nil {
		return false, m.evaluateError
	}
	return m.evaluateResult, nil
}

func (m *mockCELEvaluator) EvaluateWithVariables(ctx context.Context, expr *celapi.CompiledExpression, vars map[string]interface{}) (interface{}, error) {
	return m.evaluateResult, m.evaluateError
}

func (m *mockCELEvaluator) RegisterCustomFunction(name string, fn celapi.CustomFunction) error {
	return nil
}

func (m *mockCELEvaluator) GetEnvironment() *cel.Env {
	return nil
}

// mockValidator is a mock implementation of DecisionValidator for testing.
type mockValidator struct {
	validateError error
	conflicts     []ConflictDescription
}

func (m *mockValidator) ValidateDecision(ctx context.Context, decision *PlacementDecision) error {
	return m.validateError
}

func (m *mockValidator) ValidateResourceConstraints(ctx context.Context, placements []*WorkspacePlacement) error {
	return m.validateError
}

func (m *mockValidator) ValidatePolicyCompliance(ctx context.Context, decision *PlacementDecision) error {
	return m.validateError
}

func (m *mockValidator) CheckConflicts(ctx context.Context, decision *PlacementDecision) ([]ConflictDescription, error) {
	return m.conflicts, m.validateError
}

func TestNewDecisionMaker(t *testing.T) {
	celEvaluator := &mockCELEvaluator{}
	validator := &mockValidator{}
	recorder := NewInMemoryDecisionRecorder()
	config := DecisionConfig{
		Algorithm:       AlgorithmWeightedScore,
		SchedulerWeight: 70.0,
		CELWeight:      30.0,
		MinimumScore:   50.0,
	}

	dm := NewDecisionMaker(celEvaluator, validator, recorder, config)
	if dm == nil {
		t.Error("NewDecisionMaker returned nil")
	}
}

func TestMakePlacementDecision_WeightedScore(t *testing.T) {
	tests := map[string]struct {
		candidates      []*schedulerapi.ScoredCandidate
		celExpressions  []CELExpression
		celEvaluateResult bool
		config          DecisionConfig
		expectedSelected int
		expectedRejected int
		expectError     bool
	}{
		"successful decision with single candidate": {
			candidates: []*schedulerapi.ScoredCandidate{
				{
					Candidate: &schedulerapi.WorkspaceCandidate{
						Workspace: logicalcluster.Name("test-workspace-1"),
						Ready:     true,
						Labels:    labels.Set{"region": "us-west"},
					},
					Score: 80.0,
				},
			},
			celExpressions: []CELExpression{
				{Name: "test-expr", Expression: "workspace.ready", Weight: 100.0, Required: false},
			},
			celEvaluateResult: true,
			config: DecisionConfig{
				Algorithm:       AlgorithmWeightedScore,
				SchedulerWeight: 70.0,
				CELWeight:      30.0,
				MinimumScore:   50.0,
			},
			expectedSelected: 1,
			expectedRejected: 0,
			expectError:     false,
		},
		"decision with multiple candidates": {
			candidates: []*schedulerapi.ScoredCandidate{
				{
					Candidate: &schedulerapi.WorkspaceCandidate{
						Workspace: logicalcluster.Name("test-workspace-1"),
						Ready:     true,
						Labels:    labels.Set{"region": "us-west"},
					},
					Score: 90.0,
				},
				{
					Candidate: &schedulerapi.WorkspaceCandidate{
						Workspace: logicalcluster.Name("test-workspace-2"),
						Ready:     true,
						Labels:    labels.Set{"region": "us-east"},
					},
					Score: 70.0,
				},
				{
					Candidate: &schedulerapi.WorkspaceCandidate{
						Workspace: logicalcluster.Name("test-workspace-3"),
						Ready:     true,
						Labels:    labels.Set{"region": "eu-west"},
					},
					Score: 40.0, // Below minimum score
				},
			},
			celExpressions: []CELExpression{
				{Name: "test-expr", Expression: "workspace.ready", Weight: 100.0, Required: false},
			},
			celEvaluateResult: true,
			config: DecisionConfig{
				Algorithm:       AlgorithmWeightedScore,
				SchedulerWeight: 70.0,
				CELWeight:      30.0,
				MinimumScore:   50.0,
			},
			expectedSelected: 2,
			expectedRejected: 1,
			expectError:     false,
		},
		"decision with no candidates meeting minimum score": {
			candidates: []*schedulerapi.ScoredCandidate{
				{
					Candidate: &schedulerapi.WorkspaceCandidate{
						Workspace: logicalcluster.Name("test-workspace-1"),
						Ready:     true,
					},
					Score: 30.0, // Below minimum
				},
			},
			celExpressions: []CELExpression{},
			config: DecisionConfig{
				Algorithm:       AlgorithmWeightedScore,
				SchedulerWeight: 100.0,
				CELWeight:      0.0,
				MinimumScore:   50.0,
			},
			expectedSelected: 0,
			expectedRejected: 1,
			expectError:     false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			celEvaluator := &mockCELEvaluator{
				evaluateResult: tc.celEvaluateResult,
			}
			validator := &mockValidator{}
			recorder := NewInMemoryDecisionRecorder()

			dm := NewDecisionMaker(celEvaluator, validator, recorder, tc.config)

			request := &PlacementRequest{
				ID:             "test-request-1",
				Name:           "test-placement",
				Namespace:      "default",
				SourceWorkspace: logicalcluster.Name("source-workspace"),
				CELExpressions: tc.celExpressions,
				SchedulerRequest: &schedulerapi.PlacementRequest{
					Name:        "test-placement",
					Namespace:   "default",
					Workspace:   logicalcluster.Name("source-workspace"),
					Priority:    schedulerapi.PriorityNormal,
					MaxPlacements: 0, // No limit
				},
				CreatedAt: time.Now(),
			}

			ctx := context.Background()
			decision, err := dm.MakePlacementDecision(ctx, request, tc.candidates)

			if tc.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if len(decision.SelectedWorkspaces) != tc.expectedSelected {
				t.Errorf("Expected %d selected workspaces, got %d",
					tc.expectedSelected, len(decision.SelectedWorkspaces))
			}

			if len(decision.RejectedCandidates) != tc.expectedRejected {
				t.Errorf("Expected %d rejected candidates, got %d",
					tc.expectedRejected, len(decision.RejectedCandidates))
			}

			if decision.Status != DecisionStatusComplete {
				t.Errorf("Expected status %s, got %s", DecisionStatusComplete, decision.Status)
			}

			// Verify decision has proper rationale
			if decision.DecisionRationale.Summary == "" {
				t.Error("Expected non-empty decision rationale summary")
			}
		})
	}
}

func TestMakePlacementDecision_CELPrimary(t *testing.T) {
	celEvaluator := &mockCELEvaluator{
		evaluateResult: true,
	}
	validator := &mockValidator{}
	recorder := NewInMemoryDecisionRecorder()

	config := DecisionConfig{
		Algorithm:    AlgorithmCELPrimary,
		MinimumScore: 0.0,
	}

	dm := NewDecisionMaker(celEvaluator, validator, recorder, config)

	candidates := []*schedulerapi.ScoredCandidate{
		{
			Candidate: &schedulerapi.WorkspaceCandidate{
				Workspace: logicalcluster.Name("test-workspace-1"),
				Ready:     true,
			},
			Score: 50.0,
		},
		{
			Candidate: &schedulerapi.WorkspaceCandidate{
				Workspace: logicalcluster.Name("test-workspace-2"),
				Ready:     true,
			},
			Score: 80.0, // Higher scheduler score, but CEL should dominate
		},
	}

	request := &PlacementRequest{
		ID:             "test-request-1",
		CELExpressions: []CELExpression{
			{Name: "prefer-ws1", Expression: "workspace.name == 'test-workspace-1'", Weight: 100.0},
		},
		SchedulerRequest: &schedulerapi.PlacementRequest{
			MaxPlacements: 1,
		},
		CreatedAt: time.Now(),
	}

	ctx := context.Background()
	decision, err := dm.MakePlacementDecision(ctx, request, candidates)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(decision.SelectedWorkspaces) != 1 {
		t.Errorf("Expected 1 selected workspace, got %d", len(decision.SelectedWorkspaces))
	}

	if decision.DecisionRationale.DecisionAlgorithm != string(AlgorithmCELPrimary) {
		t.Errorf("Expected algorithm %s, got %s", 
			AlgorithmCELPrimary, decision.DecisionRationale.DecisionAlgorithm)
	}
}

func TestMakePlacementDecision_Consensus(t *testing.T) {
	celEvaluator := &mockCELEvaluator{
		evaluateResult: true,
	}
	validator := &mockValidator{}
	recorder := NewInMemoryDecisionRecorder()

	config := DecisionConfig{
		Algorithm:    AlgorithmConsensus,
		MinimumScore: 60.0, // Both scheduler and CEL must score >= 60
	}

	dm := NewDecisionMaker(celEvaluator, validator, recorder, config)

	candidates := []*schedulerapi.ScoredCandidate{
		{
			Candidate: &schedulerapi.WorkspaceCandidate{
				Workspace: logicalcluster.Name("test-workspace-1"),
				Ready:     true,
			},
			Score: 70.0, // Above threshold
		},
		{
			Candidate: &schedulerapi.WorkspaceCandidate{
				Workspace: logicalcluster.Name("test-workspace-2"),
				Ready:     true,
			},
			Score: 50.0, // Below threshold
		},
	}

	request := &PlacementRequest{
		ID: "test-request-1",
		CELExpressions: []CELExpression{
			{Name: "test-expr", Expression: "workspace.ready", Weight: 80.0}, // Above threshold
		},
		SchedulerRequest: &schedulerapi.PlacementRequest{},
		CreatedAt:       time.Now(),
	}

	ctx := context.Background()
	decision, err := dm.MakePlacementDecision(ctx, request, candidates)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Only workspace-1 should be selected (both scheduler and CEL scores are above threshold)
	if len(decision.SelectedWorkspaces) != 1 {
		t.Errorf("Expected 1 selected workspace, got %d", len(decision.SelectedWorkspaces))
	}

	if decision.SelectedWorkspaces[0].Workspace != "test-workspace-1" {
		t.Errorf("Expected test-workspace-1 to be selected, got %s",
			decision.SelectedWorkspaces[0].Workspace)
	}

	if len(decision.RejectedCandidates) != 1 {
		t.Errorf("Expected 1 rejected candidate, got %d", len(decision.RejectedCandidates))
	}
}

func TestApplyOverride_Force(t *testing.T) {
	celEvaluator := &mockCELEvaluator{}
	validator := &mockValidator{}
	recorder := NewInMemoryDecisionRecorder()
	config := DecisionConfig{Algorithm: AlgorithmWeightedScore}

	dm := NewDecisionMaker(celEvaluator, validator, recorder, config)

	// Create an initial decision with different workspaces
	decision := &PlacementDecision{
		ID:        "test-decision-1",
		RequestID: "test-request-1",
		SelectedWorkspaces: []*WorkspacePlacement{
			{Workspace: logicalcluster.Name("original-workspace")},
		},
		Status: DecisionStatusComplete,
	}

	// Create a force override
	override := &PlacementOverride{
		ID:               "test-override-1",
		PlacementID:     "test-request-1",
		OverrideType:    OverrideTypeForce,
		TargetWorkspaces: []logicalcluster.Name{"forced-workspace-1", "forced-workspace-2"},
		Reason:          "Emergency override for testing",
		AppliedBy:       "test-user",
		CreatedAt:       time.Now(),
	}

	ctx := context.Background()
	modifiedDecision, err := dm.ApplyOverride(ctx, decision, override)

	if err != nil {
		t.Fatalf("Unexpected error applying override: %v", err)
	}

	if modifiedDecision.Status != DecisionStatusOverridden {
		t.Errorf("Expected status %s, got %s", DecisionStatusOverridden, modifiedDecision.Status)
	}

	if len(modifiedDecision.SelectedWorkspaces) != 2 {
		t.Errorf("Expected 2 selected workspaces after force override, got %d",
			len(modifiedDecision.SelectedWorkspaces))
	}

	expectedWorkspaces := map[logicalcluster.Name]bool{
		"forced-workspace-1": false,
		"forced-workspace-2": false,
	}

	for _, placement := range modifiedDecision.SelectedWorkspaces {
		if _, exists := expectedWorkspaces[placement.Workspace]; exists {
			expectedWorkspaces[placement.Workspace] = true
		} else {
			t.Errorf("Unexpected workspace in selection: %s", placement.Workspace)
		}
	}

	for workspace, found := range expectedWorkspaces {
		if !found {
			t.Errorf("Expected workspace %s not found in selection", workspace)
		}
	}

	if modifiedDecision.Override == nil {
		t.Error("Expected override to be recorded in decision")
	} else if modifiedDecision.Override.ID != override.ID {
		t.Errorf("Expected override ID %s, got %s", override.ID, modifiedDecision.Override.ID)
	}
}

func TestApplyOverride_Exclude(t *testing.T) {
	celEvaluator := &mockCELEvaluator{}
	validator := &mockValidator{}
	recorder := NewInMemoryDecisionRecorder()
	config := DecisionConfig{Algorithm: AlgorithmWeightedScore}

	dm := NewDecisionMaker(celEvaluator, validator, recorder, config)

	// Create an initial decision with multiple workspaces
	decision := &PlacementDecision{
		ID:        "test-decision-1",
		RequestID: "test-request-1",
		SelectedWorkspaces: []*WorkspacePlacement{
			{Workspace: logicalcluster.Name("workspace-1")},
			{Workspace: logicalcluster.Name("workspace-2")},
			{Workspace: logicalcluster.Name("workspace-3")},
		},
		RejectedCandidates: []*RejectedCandidate{},
		Status:             DecisionStatusComplete,
	}

	// Create an exclude override
	override := &PlacementOverride{
		ID:                 "test-override-1",
		PlacementID:       "test-request-1",
		OverrideType:      OverrideTypeExclude,
		ExcludedWorkspaces: []logicalcluster.Name{"workspace-2"},
		Reason:            "Workspace maintenance",
		AppliedBy:         "test-user",
		CreatedAt:         time.Now(),
	}

	ctx := context.Background()
	modifiedDecision, err := dm.ApplyOverride(ctx, decision, override)

	if err != nil {
		t.Fatalf("Unexpected error applying override: %v", err)
	}

	if len(modifiedDecision.SelectedWorkspaces) != 2 {
		t.Errorf("Expected 2 selected workspaces after exclude override, got %d",
			len(modifiedDecision.SelectedWorkspaces))
	}

	// Verify workspace-2 is excluded
	for _, placement := range modifiedDecision.SelectedWorkspaces {
		if placement.Workspace == "workspace-2" {
			t.Error("Expected workspace-2 to be excluded but it's still selected")
		}
	}

	// Verify workspace-2 is in rejected candidates
	found := false
	for _, rejected := range modifiedDecision.RejectedCandidates {
		if rejected.Workspace == "workspace-2" {
			found = true
			if rejected.RejectionReason == "" {
				t.Error("Expected rejection reason for excluded workspace")
			}
			break
		}
	}
	if !found {
		t.Error("Expected workspace-2 to be in rejected candidates")
	}
}

func TestDecisionValidator_ValidateDecision(t *testing.T) {
	tests := map[string]struct {
		decision    *PlacementDecision
		expectError bool
		errorContains string
	}{
		"valid decision": {
			decision: &PlacementDecision{
				ID:        "test-decision-1",
				RequestID: "test-request-1",
				SelectedWorkspaces: []*WorkspacePlacement{
					{
						Workspace:      logicalcluster.Name("test-workspace-1"),
						SchedulerScore: 80.0,
						CELScore:      70.0,
						FinalScore:    75.0,
						AllocatedResources: schedulerapi.ResourceAllocation{
							CPU:           resource.MustParse("100m"),
							Memory:        resource.MustParse("128Mi"),
							ReservationID: "test-reservation-1",
							ExpiresAt:     time.Now().Add(30 * time.Minute),
						},
					},
				},
				DecisionTime: time.Now(),
				Status:       DecisionStatusComplete,
			},
			expectError: false,
		},
		"nil decision": {
			decision:      nil,
			expectError:   true,
			errorContains: "cannot be nil",
		},
		"negative final score": {
			decision: &PlacementDecision{
				ID:        "test-decision-1",
				RequestID: "test-request-1",
				SelectedWorkspaces: []*WorkspacePlacement{
					{
						Workspace:      logicalcluster.Name("test-workspace-1"),
						SchedulerScore: 80.0,
						CELScore:      70.0,
						FinalScore:    -10.0, // Invalid negative score
						AllocatedResources: schedulerapi.ResourceAllocation{
							ReservationID: "test-reservation-1",
							ExpiresAt:     time.Now().Add(30 * time.Minute),
						},
					},
				},
				DecisionTime: time.Now(),
				Status:       DecisionStatusComplete,
			},
			expectError:   true,
			errorContains: "final score cannot be negative",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			validator := NewDecisionValidator(nil)
			ctx := context.Background()

			err := validator.ValidateDecision(ctx, tc.decision)

			if tc.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if tc.errorContains != "" && !containsString(err.Error(), tc.errorContains) {
					t.Errorf("Expected error containing '%s', got '%s'", tc.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestDecisionRecorder_RecordAndRetrieve(t *testing.T) {
	recorder := NewInMemoryDecisionRecorder()

	decision := &PlacementDecision{
		ID:        "test-decision-1",
		RequestID: "test-request-1",
		SelectedWorkspaces: []*WorkspacePlacement{
			{Workspace: logicalcluster.Name("test-workspace-1")},
		},
		DecisionTime: time.Now(),
		Status:       DecisionStatusComplete,
	}

	ctx := context.Background()

	// Record the decision
	err := recorder.RecordDecision(ctx, decision)
	if err != nil {
		t.Fatalf("Unexpected error recording decision: %v", err)
	}

	// Retrieve the decision
	retrievedRecord, err := recorder.GetDecision(ctx, decision.ID)
	if err != nil {
		t.Fatalf("Unexpected error retrieving decision: %v", err)
	}

	if retrievedRecord.Decision.ID != decision.ID {
		t.Errorf("Expected decision ID %s, got %s", decision.ID, retrievedRecord.Decision.ID)
	}

	if retrievedRecord.Version != 1 {
		t.Errorf("Expected version 1, got %d", retrievedRecord.Version)
	}

	// Get decision history
	history, err := recorder.GetDecisionHistory(ctx, decision.RequestID)
	if err != nil {
		t.Fatalf("Unexpected error getting decision history: %v", err)
	}

	if len(history) != 1 {
		t.Errorf("Expected 1 decision in history, got %d", len(history))
	}

	if history[0].Decision.ID != decision.ID {
		t.Errorf("Expected decision ID %s in history, got %s", decision.ID, history[0].Decision.ID)
	}
}

func TestOverrideManager_CreateAndRetrieve(t *testing.T) {
	validator := NewDefaultOverrideValidator()
	manager := NewInMemoryOverrideManager(validator)

	override := &PlacementOverride{
		PlacementID:      "test-placement-1",
		OverrideType:     OverrideTypeForce,
		TargetWorkspaces: []logicalcluster.Name{"forced-workspace-1"},
		Reason:           "Test override",
		AppliedBy:        "test-user",
		Priority:         100,
	}

	ctx := context.Background()

	// Create the override
	err := manager.CreateOverride(ctx, override)
	if err != nil {
		t.Fatalf("Unexpected error creating override: %v", err)
	}

	// Verify ID was generated
	if override.ID == "" {
		t.Error("Expected ID to be generated for override")
	}

	// Retrieve the override
	retrievedOverride, err := manager.GetOverride(ctx, override.ID)
	if err != nil {
		t.Fatalf("Unexpected error retrieving override: %v", err)
	}

	if retrievedOverride.PlacementID != override.PlacementID {
		t.Errorf("Expected placement ID %s, got %s", 
			override.PlacementID, retrievedOverride.PlacementID)
	}

	if retrievedOverride.OverrideType != override.OverrideType {
		t.Errorf("Expected override type %s, got %s", 
			override.OverrideType, retrievedOverride.OverrideType)
	}

	// List overrides for placement
	overrides, err := manager.ListOverrides(ctx, override.PlacementID)
	if err != nil {
		t.Fatalf("Unexpected error listing overrides: %v", err)
	}

	if len(overrides) != 1 {
		t.Errorf("Expected 1 override, got %d", len(overrides))
	}

	// Get active overrides
	activeOverrides, err := manager.GetActiveOverrides(ctx, override.PlacementID)
	if err != nil {
		t.Fatalf("Unexpected error getting active overrides: %v", err)
	}

	if len(activeOverrides) != 1 {
		t.Errorf("Expected 1 active override, got %d", len(activeOverrides))
	}
}

func TestOverrideValidator_ValidateConflicts(t *testing.T) {
	validator := NewDefaultOverrideValidator()

	overrides := []*PlacementOverride{
		{
			ID:               "override-1",
			PlacementID:     "test-placement-1",
			OverrideType:    OverrideTypeForce,
			TargetWorkspaces: []logicalcluster.Name{"workspace-1"},
			Reason:          "Force override",
			AppliedBy:       "user-1",
			CreatedAt:       time.Now(),
		},
		{
			ID:                 "override-2",
			PlacementID:       "test-placement-1",
			OverrideType:      OverrideTypeExclude,
			ExcludedWorkspaces: []logicalcluster.Name{"workspace-1"}, // Same workspace as force
			Reason:            "Exclude override",
			AppliedBy:         "user-2",
			CreatedAt:         time.Now(),
		},
	}

	ctx := context.Background()
	conflicts, err := validator.CheckConflicts(ctx, overrides)

	if err != nil {
		t.Fatalf("Unexpected error checking conflicts: %v", err)
	}

	if len(conflicts) == 0 {
		t.Error("Expected conflicts between force and exclude on same workspace")
	}

	foundContradictory := false
	for _, conflict := range conflicts {
		if conflict.ConflictType == ConflictTypeContradictory {
			foundContradictory = true
			if conflict.Severity != SeverityCritical {
				t.Errorf("Expected critical severity for contradictory conflict, got %s", conflict.Severity)
			}
			break
		}
	}

	if !foundContradictory {
		t.Error("Expected contradictory conflict type")
	}
}

// Helper function to check if a string contains a substring.
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || 
		(len(substr) > 0 && len(s) > len(substr) && 
		 (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
		  findSubstring(s, substr))))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}