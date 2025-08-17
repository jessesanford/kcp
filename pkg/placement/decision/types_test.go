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
	"testing"
	"time"

	"github.com/kcp-dev/logicalcluster/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecisionStatusConstants(t *testing.T) {
	tests := map[string]struct {
		status   DecisionStatus
		expected string
	}{
		"pending status": {
			status:   DecisionStatusPending,
			expected: "Pending",
		},
		"complete status": {
			status:   DecisionStatusComplete,
			expected: "Complete",
		},
		"error status": {
			status:   DecisionStatusError,
			expected: "Error",
		},
		"overridden status": {
			status:   DecisionStatusOverridden,
			expected: "Overridden",
		},
		"rolled back status": {
			status:   DecisionStatusRolledBack,
			expected: "RolledBack",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.expected, string(tc.status))
		})
	}
}

func TestOverrideTypeConstants(t *testing.T) {
	tests := map[string]struct {
		overrideType OverrideType
		expected     string
	}{
		"force override": {
			overrideType: OverrideTypeForce,
			expected:     "Force",
		},
		"exclude override": {
			overrideType: OverrideTypeExclude,
			expected:     "Exclude",
		},
		"prefer override": {
			overrideType: OverrideTypePrefer,
			expected:     "Prefer",
		},
		"avoid override": {
			overrideType: OverrideTypeAvoid,
			expected:     "Avoid",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.expected, string(tc.overrideType))
		})
	}
}

func TestConflictTypeConstants(t *testing.T) {
	tests := map[string]struct {
		conflictType ConflictType
		expected     string
	}{
		"resource overcommit": {
			conflictType: ConflictTypeResourceOvercommit,
			expected:     "ResourceOvercommit",
		},
		"affinity violation": {
			conflictType: ConflictTypeAffinityViolation,
			expected:     "AffinityViolation",
		},
		"anti-affinity violation": {
			conflictType: ConflictTypeAntiAffinityViolation,
			expected:     "AntiAffinityViolation",
		},
		"policy violation": {
			conflictType: ConflictTypePolicyViolation,
			expected:     "PolicyViolation",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.expected, string(tc.conflictType))
		})
	}
}

func TestConflictSeverityConstants(t *testing.T) {
	tests := map[string]struct {
		severity ConflictSeverity
		expected string
	}{
		"low severity": {
			severity: SeverityLow,
			expected: "Low",
		},
		"medium severity": {
			severity: SeverityMedium,
			expected: "Medium",
		},
		"high severity": {
			severity: SeverityHigh,
			expected: "High",
		},
		"critical severity": {
			severity: SeverityCritical,
			expected: "Critical",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.expected, string(tc.severity))
		})
	}
}

func TestDecisionAlgorithmConstants(t *testing.T) {
	tests := map[string]struct {
		algorithm DecisionAlgorithm
		expected  string
	}{
		"weighted score": {
			algorithm: AlgorithmWeightedScore,
			expected:  "WeightedScore",
		},
		"cel primary": {
			algorithm: AlgorithmCELPrimary,
			expected:  "CELPrimary",
		},
		"scheduler primary": {
			algorithm: AlgorithmSchedulerPrimary,
			expected:  "SchedulerPrimary",
		},
		"consensus": {
			algorithm: AlgorithmConsensus,
			expected:  "Consensus",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.expected, string(tc.algorithm))
		})
	}
}

func TestDecisionEventTypeConstants(t *testing.T) {
	tests := map[string]struct {
		eventType DecisionEventType
		expected  string
	}{
		"started event": {
			eventType: DecisionEventTypeStarted,
			expected:  "Started",
		},
		"scheduler evaluated": {
			eventType: DecisionEventTypeSchedulerEvaluated,
			expected:  "SchedulerEvaluated",
		},
		"cel evaluated": {
			eventType: DecisionEventTypeCELEvaluated,
			expected:  "CELEvaluated",
		},
		"override applied": {
			eventType: DecisionEventTypeOverrideApplied,
			expected:  "OverrideApplied",
		},
		"completed event": {
			eventType: DecisionEventTypeCompleted,
			expected:  "Completed",
		},
		"error event": {
			eventType: DecisionEventTypeError,
			expected:  "Error",
		},
		"rolled back event": {
			eventType: DecisionEventTypeRolledBack,
			expected:  "RolledBack",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.expected, string(tc.eventType))
		})
	}
}

func TestPlacementRequestCreation(t *testing.T) {
	testCases := map[string]struct {
		id                string
		name              string
		namespace         string
		sourceWorkspace   logicalcluster.Name
		celExpressions    []CELExpression
		expectedID        string
		expectedName      string
		expectedNamespace string
	}{
		"basic placement request": {
			id:                "test-request-1",
			name:              "test-placement",
			namespace:         "default",
			sourceWorkspace:   logicalcluster.Name("root:test"),
			celExpressions:    []CELExpression{},
			expectedID:        "test-request-1",
			expectedName:      "test-placement",
			expectedNamespace: "default",
		},
		"placement with cel expressions": {
			id:              "test-request-2",
			name:            "cel-placement",
			namespace:       "test-ns",
			sourceWorkspace: logicalcluster.Name("root:production"),
			celExpressions: []CELExpression{
				{
					Name:        "availability-check",
					Expression:  "cluster.availability > 0.9",
					Weight:      80.0,
					Required:    true,
					Description: "Check cluster availability",
				},
			},
			expectedID:        "test-request-2",
			expectedName:      "cel-placement",
			expectedNamespace: "test-ns",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			request := &PlacementRequest{
				ID:               tc.id,
				Name:             tc.name,
				Namespace:        tc.namespace,
				SourceWorkspace:  tc.sourceWorkspace,
				CELExpressions:   tc.celExpressions,
				DecisionDeadline: time.Now().Add(5 * time.Minute),
				MaxRetries:       3,
				CreatedAt:        time.Now(),
			}

			assert.Equal(t, tc.expectedID, request.ID)
			assert.Equal(t, tc.expectedName, request.Name)
			assert.Equal(t, tc.expectedNamespace, request.Namespace)
			assert.Equal(t, tc.sourceWorkspace, request.SourceWorkspace)
			assert.Len(t, request.CELExpressions, len(tc.celExpressions))
		})
	}
}

func TestCELExpressionValidation(t *testing.T) {
	testCases := map[string]struct {
		expression      CELExpression
		expectedName    string
		expectedWeight  float64
		expectedRequired bool
	}{
		"required expression": {
			expression: CELExpression{
				Name:        "memory-check",
				Expression:  "cluster.memory > 4096",
				Weight:      100.0,
				Required:    true,
				Description: "Ensure minimum memory",
			},
			expectedName:    "memory-check",
			expectedWeight:  100.0,
			expectedRequired: true,
		},
		"optional expression": {
			expression: CELExpression{
				Name:        "performance-preference",
				Expression:  "cluster.cpu_speed > 2.0",
				Weight:      60.0,
				Required:    false,
				Description: "Prefer faster CPUs",
			},
			expectedName:    "performance-preference",
			expectedWeight:  60.0,
			expectedRequired: false,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.expectedName, tc.expression.Name)
			assert.Equal(t, tc.expectedWeight, tc.expression.Weight)
			assert.Equal(t, tc.expectedRequired, tc.expression.Required)
			assert.NotEmpty(t, tc.expression.Description)
		})
	}
}

func TestPlacementDecisionCreation(t *testing.T) {
	testCases := map[string]struct {
		decision         PlacementDecision
		expectedID       string
		expectedRequestID string
		expectedStatus   DecisionStatus
	}{
		"completed decision": {
			decision: PlacementDecision{
				ID:        "decision-1",
				RequestID: "request-1",
				Status:    DecisionStatusComplete,
				SelectedWorkspaces: []*WorkspacePlacement{
					{
						Workspace:      logicalcluster.Name("root:production"),
						SchedulerScore: 85.0,
						CELScore:       90.0,
						FinalScore:     87.5,
						SelectionReason: "Best overall score",
					},
				},
				DecisionTime:     time.Now(),
				DecisionDuration: 150 * time.Millisecond,
			},
			expectedID:       "decision-1",
			expectedRequestID: "request-1",
			expectedStatus:   DecisionStatusComplete,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.expectedID, tc.decision.ID)
			assert.Equal(t, tc.expectedRequestID, tc.decision.RequestID)
			assert.Equal(t, tc.expectedStatus, tc.decision.Status)
			
			if len(tc.decision.SelectedWorkspaces) > 0 {
				workspace := tc.decision.SelectedWorkspaces[0]
				assert.NotEmpty(t, workspace.Workspace)
				assert.GreaterOrEqual(t, workspace.FinalScore, 0.0)
				assert.LessOrEqual(t, workspace.FinalScore, 100.0)
			}
		})
	}
}

func TestPlacementOverrideCreation(t *testing.T) {
	testCases := map[string]struct {
		override         PlacementOverride
		expectedType     OverrideType
		expectedPriority int32
	}{
		"force override": {
			override: PlacementOverride{
				ID:          "override-1",
				PlacementID: "placement-1",
				OverrideType: OverrideTypeForce,
				TargetWorkspaces: []logicalcluster.Name{
					logicalcluster.Name("root:emergency"),
				},
				Reason:    "Emergency deployment needed",
				AppliedBy: "admin@example.com",
				CreatedAt: time.Now(),
				Priority:  100,
			},
			expectedType:     OverrideTypeForce,
			expectedPriority: 100,
		},
		"exclude override": {
			override: PlacementOverride{
				ID:          "override-2",
				PlacementID: "placement-2",
				OverrideType: OverrideTypeExclude,
				ExcludedWorkspaces: []logicalcluster.Name{
					logicalcluster.Name("root:maintenance"),
				},
				Reason:    "Workspace under maintenance",
				AppliedBy: "operator@example.com",
				CreatedAt: time.Now(),
				Priority:  50,
			},
			expectedType:     OverrideTypeExclude,
			expectedPriority: 50,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.expectedType, tc.override.OverrideType)
			assert.Equal(t, tc.expectedPriority, tc.override.Priority)
			assert.NotEmpty(t, tc.override.Reason)
			assert.NotEmpty(t, tc.override.AppliedBy)
		})
	}
}

func TestConflictDescriptionCreation(t *testing.T) {
	testCases := map[string]struct {
		conflict         ConflictDescription
		expectedType     ConflictType
		expectedSeverity ConflictSeverity
	}{
		"resource conflict": {
			conflict: ConflictDescription{
				Type:        ConflictTypeResourceOvercommit,
				Description: "Insufficient CPU resources available",
				AffectedWorkspaces: []logicalcluster.Name{
					logicalcluster.Name("root:production"),
				},
				Severity:             SeverityHigh,
				ResolutionSuggestion: "Scale up cluster or reduce resource requests",
			},
			expectedType:     ConflictTypeResourceOvercommit,
			expectedSeverity: SeverityHigh,
		},
		"policy conflict": {
			conflict: ConflictDescription{
				Type:        ConflictTypePolicyViolation,
				Description: "Deployment violates security policy",
				AffectedWorkspaces: []logicalcluster.Name{
					logicalcluster.Name("root:secure"),
				},
				Severity:             SeverityCritical,
				ResolutionSuggestion: "Update deployment to comply with security policy",
			},
			expectedType:     ConflictTypePolicyViolation,
			expectedSeverity: SeverityCritical,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.expectedType, tc.conflict.Type)
			assert.Equal(t, tc.expectedSeverity, tc.conflict.Severity)
			assert.NotEmpty(t, tc.conflict.Description)
			assert.NotEmpty(t, tc.conflict.ResolutionSuggestion)
			assert.NotEmpty(t, tc.conflict.AffectedWorkspaces)
		})
	}
}

func TestDecisionConfigDefaults(t *testing.T) {
	config := DecisionConfig{
		Algorithm:             AlgorithmWeightedScore,
		SchedulerWeight:       60.0,
		CELWeight:            40.0,
		MinimumScore:         50.0,
		MaxDecisionTime:      30 * time.Second,
		EnableAuditLogging:   true,
		DefaultCELExpressions: []CELExpression{},
	}

	assert.Equal(t, AlgorithmWeightedScore, config.Algorithm)
	assert.Equal(t, 60.0, config.SchedulerWeight)
	assert.Equal(t, 40.0, config.CELWeight)
	assert.Equal(t, 50.0, config.MinimumScore)
	assert.Equal(t, 30*time.Second, config.MaxDecisionTime)
	assert.True(t, config.EnableAuditLogging)
}

func TestCELEvaluationResult(t *testing.T) {
	result := CELEvaluationResult{
		ExpressionName:  "availability-check",
		Expression:      "cluster.availability > 0.9",
		Result:          true,
		Score:          95.0,
		Success:        true,
		EvaluationTime: 5 * time.Millisecond,
		Workspace:      logicalcluster.Name("root:production"),
	}

	assert.Equal(t, "availability-check", result.ExpressionName)
	assert.True(t, result.Result.(bool))
	assert.Equal(t, 95.0, result.Score)
	assert.True(t, result.Success)
	assert.Nil(t, result.Error)
	assert.Equal(t, 5*time.Millisecond, result.EvaluationTime)
}

func TestDecisionEventCreation(t *testing.T) {
	event := DecisionEvent{
		Type:      DecisionEventTypeStarted,
		Timestamp: time.Now(),
		Message:   "Decision making process started",
		Details: map[string]interface{}{
			"request_id": "req-123",
			"workspace":  "root:production",
		},
	}

	assert.Equal(t, DecisionEventTypeStarted, event.Type)
	assert.NotEmpty(t, event.Message)
	assert.Contains(t, event.Details, "request_id")
	assert.Contains(t, event.Details, "workspace")
}

func TestDecisionRecord(t *testing.T) {
	decision := &PlacementDecision{
		ID:        "decision-1",
		RequestID: "request-1",
		Status:    DecisionStatusComplete,
	}

	record := DecisionRecord{
		Decision:  decision,
		Timestamp: time.Now(),
		Version:   1,
		Events: []DecisionEvent{
			{
				Type:      DecisionEventTypeStarted,
				Timestamp: time.Now(),
				Message:   "Process started",
			},
		},
	}

	require.NotNil(t, record.Decision)
	assert.Equal(t, "decision-1", record.Decision.ID)
	assert.Equal(t, int64(1), record.Version)
	assert.Len(t, record.Events, 1)
	assert.Equal(t, DecisionEventTypeStarted, record.Events[0].Type)
}