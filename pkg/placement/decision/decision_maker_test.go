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
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/cel-go/cel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/resource"

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
	score         float64
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
	if m.evaluateError != nil {
		return nil, m.evaluateError
	}
	return m.evaluateResult, m.evaluateError
}

func (m *mockCELEvaluator) RegisterCustomFunction(name string, fn celapi.CustomFunction) error {
	return nil
}

func (m *mockCELEvaluator) GetEnvironment() *cel.Env {
	return nil
}

func TestNewDecisionMaker(t *testing.T) {
	celEvaluator := &mockCELEvaluator{}
	
	config := DecisionConfig{
		Algorithm:       AlgorithmWeightedScore,
		SchedulerWeight: 60.0,
		CELWeight:      40.0,
		MinimumScore:   50.0,
		MaxDecisionTime: 30 * time.Second,
	}

	dm := NewDecisionMaker(celEvaluator, config)
	
	require.NotNil(t, dm)
	assert.Equal(t, config.Algorithm, dm.config.Algorithm)
	assert.Equal(t, config.SchedulerWeight, dm.config.SchedulerWeight)
	assert.Equal(t, config.CELWeight, dm.config.CELWeight)
}

func TestDecisionMaker_MakePlacementDecision_BasicScenario(t *testing.T) {
	celEvaluator := &mockCELEvaluator{
		evaluateResult: true,
	}
	
	config := DecisionConfig{
		Algorithm:       AlgorithmWeightedScore,
		SchedulerWeight: 60.0,
		CELWeight:      40.0,
		MinimumScore:   50.0,
	}

	dm := NewDecisionMaker(celEvaluator, config)
	
	ctx := context.Background()
	
	request := &PlacementRequest{
		ID:        "test-request-1",
		Name:      "test-placement",
		Namespace: "default",
		SourceWorkspace: logicalcluster.Name("root:test"),
		CELExpressions: []CELExpression{
			{
				Name:        "availability-check",
				Expression:  "cluster.availability > 0.9",
				Weight:      80.0,
				Required:    true,
				Description: "Check cluster availability",
			},
		},
		DecisionDeadline: time.Now().Add(5 * time.Minute),
		MaxRetries:       3,
		CreatedAt:        time.Now(),
	}
	
	candidates := []*schedulerapi.ScoredCandidate{
		{
			Workspace: logicalcluster.Name("root:production"),
			Score:     85.0,
			ResourceAllocation: schedulerapi.ResourceAllocation{
				CPU:    resource.MustParse("4"),
				Memory: resource.MustParse("8Gi"),
			},
		},
		{
			Workspace: logicalcluster.Name("root:staging"),
			Score:     75.0,
			ResourceAllocation: schedulerapi.ResourceAllocation{
				CPU:    resource.MustParse("2"),
				Memory: resource.MustParse("4Gi"),
			},
		},
	}

	decision, err := dm.MakePlacementDecision(ctx, request, candidates)
	
	require.NoError(t, err)
	require.NotNil(t, decision)
	assert.Equal(t, request.ID, decision.RequestID)
	assert.NotEmpty(t, decision.ID)
	assert.Equal(t, DecisionStatusComplete, decision.Status)
	assert.NotEmpty(t, decision.SelectedWorkspaces)
	assert.NotZero(t, decision.DecisionDuration)
}

func TestDecisionMaker_MakePlacementDecision_NoSuitableCandidates(t *testing.T) {
	celEvaluator := &mockCELEvaluator{
		evaluateResult: false, // CEL evaluation fails for all candidates
	}
	
	config := DecisionConfig{
		Algorithm:       AlgorithmWeightedScore,
		SchedulerWeight: 60.0,
		CELWeight:      40.0,
		MinimumScore:   80.0, // High minimum score
	}

	dm := NewDecisionMaker(celEvaluator, config)
	
	ctx := context.Background()
	
	request := &PlacementRequest{
		ID:        "test-request-2",
		Name:      "test-placement",
		Namespace: "default",
		SourceWorkspace: logicalcluster.Name("root:test"),
		CELExpressions: []CELExpression{
			{
				Name:        "strict-check",
				Expression:  "cluster.reliability > 0.99",
				Weight:      100.0,
				Required:    true,
				Description: "Strict reliability check",
			},
		},
	}
	
	candidates := []*schedulerapi.ScoredCandidate{
		{
			Workspace: logicalcluster.Name("root:development"),
			Score:     60.0, // Below minimum score
		},
	}

	decision, err := dm.MakePlacementDecision(ctx, request, candidates)
	
	require.NoError(t, err)
	require.NotNil(t, decision)
	assert.Empty(t, decision.SelectedWorkspaces)
	assert.NotEmpty(t, decision.RejectedCandidates)
	assert.Equal(t, DecisionStatusComplete, decision.Status)
}

func TestDecisionMaker_MakePlacementDecision_CELEvaluationError(t *testing.T) {
	celEvaluator := &mockCELEvaluator{
		evaluateError: errors.New("CEL evaluation failed"),
	}
	
	config := DecisionConfig{
		Algorithm:       AlgorithmWeightedScore,
		SchedulerWeight: 60.0,
		CELWeight:      40.0,
		MinimumScore:   50.0,
	}

	dm := NewDecisionMaker(celEvaluator, config)
	
	ctx := context.Background()
	
	request := &PlacementRequest{
		ID:        "test-request-3",
		Name:      "test-placement",
		Namespace: "default",
		SourceWorkspace: logicalcluster.Name("root:test"),
		CELExpressions: []CELExpression{
			{
				Name:        "faulty-expression",
				Expression:  "invalid.cel.expression",
				Weight:      80.0,
				Required:    true,
			},
		},
	}
	
	candidates := []*schedulerapi.ScoredCandidate{
		{
			Workspace: logicalcluster.Name("root:production"),
			Score:     85.0,
		},
	}

	decision, err := dm.MakePlacementDecision(ctx, request, candidates)
	
	require.NoError(t, err) // Should not fail entirely, but mark candidates as rejected
	require.NotNil(t, decision)
	assert.Empty(t, decision.SelectedWorkspaces)
	assert.NotEmpty(t, decision.RejectedCandidates)
	assert.Contains(t, decision.RejectedCandidates[0].RejectionReason, "CEL evaluation failed")
}

func TestDecisionMaker_AlgorithmVariants(t *testing.T) {
	testCases := map[string]struct {
		algorithm DecisionAlgorithm
		config    DecisionConfig
	}{
		"weighted score": {
			algorithm: AlgorithmWeightedScore,
			config: DecisionConfig{
				Algorithm:       AlgorithmWeightedScore,
				SchedulerWeight: 70.0,
				CELWeight:      30.0,
				MinimumScore:   50.0,
			},
		},
		"cel primary": {
			algorithm: AlgorithmCELPrimary,
			config: DecisionConfig{
				Algorithm:    AlgorithmCELPrimary,
				MinimumScore: 60.0,
			},
		},
		"scheduler primary": {
			algorithm: AlgorithmSchedulerPrimary,
			config: DecisionConfig{
				Algorithm:    AlgorithmSchedulerPrimary,
				MinimumScore: 50.0,
			},
		},
		"consensus": {
			algorithm: AlgorithmConsensus,
			config: DecisionConfig{
				Algorithm:    AlgorithmConsensus,
				MinimumScore: 70.0,
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			celEvaluator := &mockCELEvaluator{evaluateResult: true}
			dm := NewDecisionMaker(celEvaluator, tc.config)
			
			request := &PlacementRequest{
				ID:              fmt.Sprintf("test-%s", strings.ReplaceAll(name, " ", "-")),
				SourceWorkspace: logicalcluster.Name("root:test"),
			}
			
			candidates := []*schedulerapi.ScoredCandidate{
				{Workspace: logicalcluster.Name("root:test"), Score: 80.0},
			}

			decision, err := dm.MakePlacementDecision(context.Background(), request, candidates)
			
			require.NoError(t, err)
			require.NotNil(t, decision)
			assert.Equal(t, string(tc.algorithm), decision.DecisionRationale.DecisionAlgorithm)
		})
	}
}

func TestDecisionMaker_RequiredCELExpressions(t *testing.T) {
	celEvaluator := &mockCELEvaluator{
		evaluateResult: false, // Required expression fails
	}
	
	config := DecisionConfig{
		Algorithm:    AlgorithmWeightedScore,
		MinimumScore: 50.0,
	}

	dm := NewDecisionMaker(celEvaluator, config)
	
	ctx := context.Background()
	
	request := &PlacementRequest{
		ID:        "test-required-cel",
		Name:      "required-cel-test",
		Namespace: "default",
		SourceWorkspace: logicalcluster.Name("root:test"),
		CELExpressions: []CELExpression{
			{
				Name:        "security-check",
				Expression:  "cluster.security_compliant == true",
				Weight:      100.0,
				Required:    true, // This expression must pass
				Description: "Security compliance check",
			},
		},
	}
	
	candidates := []*schedulerapi.ScoredCandidate{
		{
			Workspace: logicalcluster.Name("root:non-compliant"),
			Score:     95.0, // High scheduler score but fails required CEL
		},
	}

	decision, err := dm.MakePlacementDecision(ctx, request, candidates)
	
	require.NoError(t, err)
	require.NotNil(t, decision)
	assert.Empty(t, decision.SelectedWorkspaces) // Should be rejected due to required CEL failure
	assert.NotEmpty(t, decision.RejectedCandidates)
	
	rejected := decision.RejectedCandidates[0]
	assert.Contains(t, rejected.RejectionReason, "required CEL expression failed")
}

func TestDecisionMaker_EdgeCases(t *testing.T) {
	testCases := map[string]struct {
		setup    func() (*mockCELEvaluator, DecisionConfig, *PlacementRequest, []*schedulerapi.ScoredCandidate)
		validate func(t *testing.T, decision *PlacementDecision, err error)
	}{
		"multiple workspaces": {
			setup: func() (*mockCELEvaluator, DecisionConfig, *PlacementRequest, []*schedulerapi.ScoredCandidate) {
				return &mockCELEvaluator{evaluateResult: true},
					DecisionConfig{Algorithm: AlgorithmWeightedScore, MinimumScore: 60.0},
					&PlacementRequest{ID: "multi", SourceWorkspace: logicalcluster.Name("root:test")},
					[]*schedulerapi.ScoredCandidate{
						{Workspace: logicalcluster.Name("root:high"), Score: 90.0},
						{Workspace: logicalcluster.Name("root:low"), Score: 45.0},
					}
			},
			validate: func(t *testing.T, decision *PlacementDecision, err error) {
				require.NoError(t, err)
				assert.NotEmpty(t, decision.SelectedWorkspaces)
				assert.NotEmpty(t, decision.RejectedCandidates)
			},
		},
		"empty candidates": {
			setup: func() (*mockCELEvaluator, DecisionConfig, *PlacementRequest, []*schedulerapi.ScoredCandidate) {
				return &mockCELEvaluator{},
					DecisionConfig{Algorithm: AlgorithmWeightedScore},
					&PlacementRequest{ID: "empty", SourceWorkspace: logicalcluster.Name("root:test")},
					[]*schedulerapi.ScoredCandidate{}
			},
			validate: func(t *testing.T, decision *PlacementDecision, err error) {
				require.NoError(t, err)
				assert.Empty(t, decision.SelectedWorkspaces)
				assert.Equal(t, DecisionStatusComplete, decision.Status)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			celEval, config, request, candidates := tc.setup()
			dm := NewDecisionMaker(celEval, config)
			decision, err := dm.MakePlacementDecision(context.Background(), request, candidates)
			tc.validate(t, decision, err)
		})
	}
}

func TestDecisionMaker_DecisionRationale(t *testing.T) {
	celEvaluator := &mockCELEvaluator{
		evaluateResult: true,
	}
	
	config := DecisionConfig{
		Algorithm:       AlgorithmWeightedScore,
		SchedulerWeight: 60.0,
		CELWeight:      40.0,
	}

	dm := NewDecisionMaker(celEvaluator, config)
	
	ctx := context.Background()
	
	request := &PlacementRequest{
		ID:        "test-rationale",
		Name:      "rationale-test",
		Namespace: "default",
		SourceWorkspace: logicalcluster.Name("root:test"),
		CELExpressions: []CELExpression{
			{
				Name:        "rationale-check",
				Expression:  "cluster.test == true",
				Weight:      75.0,
				Description: "Test rationale generation",
			},
		},
	}
	
	candidates := []*schedulerapi.ScoredCandidate{
		{
			Workspace: logicalcluster.Name("root:selected"),
			Score:     85.0,
		},
	}

	decision, err := dm.MakePlacementDecision(ctx, request, candidates)
	
	require.NoError(t, err)
	require.NotNil(t, decision)
	require.NotNil(t, decision.DecisionRationale)
	
	rationale := decision.DecisionRationale
	assert.NotEmpty(t, rationale.Summary)
	assert.NotEmpty(t, rationale.DecisionAlgorithm)
	assert.Equal(t, string(config.Algorithm), rationale.DecisionAlgorithm)
	assert.Contains(t, rationale.WeightingStrategy, "scheduler")
	assert.Contains(t, rationale.WeightingStrategy, "CEL")
}