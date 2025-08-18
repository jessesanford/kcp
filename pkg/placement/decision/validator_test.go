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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/kcp-dev/logicalcluster/v3"

	schedulerapi "github.com/kcp-dev/kcp/pkg/placement/scheduler"
)

func TestNewDecisionValidator(t *testing.T) {
	config := ValidationConfig{
		EnableResourceValidation: true,
		EnablePolicyValidation:   true,
		MaxValidationTime:       10 * time.Second,
		MinimumWorkspaces:       1,
		MaximumWorkspaces:       5,
	}

	validator := NewDecisionValidator(config)
	require.NotNil(t, validator)

	// Verify it implements the interface
	var _ DecisionValidator = validator
}

func TestValidateDecision_StructureValidation(t *testing.T) {
	validator := NewDecisionValidator(ValidationConfig{})

	tests := map[string]struct {
		decision  *PlacementDecision
		wantError bool
		errorMsg  string
	}{
		"nil decision": {
			decision:  nil,
			wantError: true,
			errorMsg:  "decision cannot be nil",
		},
		"empty decision ID": {
			decision: &PlacementDecision{
				ID:        "",
				RequestID: "req-1",
				Status:    DecisionStatusComplete,
			},
			wantError: true,
			errorMsg:  "decision ID cannot be empty",
		},
		"empty request ID": {
			decision: &PlacementDecision{
				ID:        "dec-1",
				RequestID: "",
				Status:    DecisionStatusComplete,
			},
			wantError: true,
			errorMsg:  "request ID cannot be empty",
		},
		"empty status": {
			decision: &PlacementDecision{
				ID:        "dec-1",
				RequestID: "req-1",
				Status:    "",
			},
			wantError: true,
			errorMsg:  "decision status cannot be empty",
		},
		"future decision time": {
			decision: &PlacementDecision{
				ID:           "dec-1",
				RequestID:    "req-1",
				Status:       DecisionStatusComplete,
				DecisionTime: time.Now().Add(5 * time.Minute),
			},
			wantError: true,
			errorMsg:  "decision time cannot be in the future",
		},
		"duplicate workspace selection": {
			decision: &PlacementDecision{
				ID:           "dec-1",
				RequestID:    "req-1",
				Status:       DecisionStatusComplete,
				DecisionTime: time.Now().Add(-time.Minute),
				SelectedWorkspaces: []*WorkspacePlacement{
					{
						Workspace:      logicalcluster.Name("ws-1"),
						SchedulerScore: 80.0,
						CELScore:       70.0,
						FinalScore:     75.0,
					},
					{
						Workspace:      logicalcluster.Name("ws-1"), // Duplicate
						SchedulerScore: 85.0,
						CELScore:       75.0,
						FinalScore:     80.0,
					},
				},
			},
			wantError: true,
			errorMsg:  "duplicate workspace selection",
		},
		"invalid scheduler score": {
			decision: &PlacementDecision{
				ID:           "dec-1",
				RequestID:    "req-1",
				Status:       DecisionStatusComplete,
				DecisionTime: time.Now().Add(-time.Minute),
				SelectedWorkspaces: []*WorkspacePlacement{
					{
						Workspace:      logicalcluster.Name("ws-1"),
						SchedulerScore: 150.0, // Out of range
						CELScore:       70.0,
						FinalScore:     75.0,
					},
				},
			},
			wantError: true,
			errorMsg:  "scheduler score out of range",
		},
		"valid decision": {
			decision: &PlacementDecision{
				ID:           "dec-1",
				RequestID:    "req-1",
				Status:       DecisionStatusComplete,
				DecisionTime: time.Now().Add(-time.Minute),
				SelectedWorkspaces: []*WorkspacePlacement{
					{
						Workspace:      logicalcluster.Name("ws-1"),
						SchedulerScore: 80.0,
						CELScore:       70.0,
						FinalScore:     75.0,
					},
				},
			},
			wantError: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := validator.ValidateDecision(context.Background(), tc.decision)

			if tc.wantError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateDecision_WorkspaceCount(t *testing.T) {
	tests := map[string]struct {
		config    ValidationConfig
		decision  *PlacementDecision
		wantError bool
		errorMsg  string
	}{
		"below minimum": {
			config: ValidationConfig{
				MinimumWorkspaces: 2,
			},
			decision: &PlacementDecision{
				ID:           "dec-1",
				RequestID:    "req-1",
				Status:       DecisionStatusComplete,
				DecisionTime: time.Now().Add(-time.Minute),
				SelectedWorkspaces: []*WorkspacePlacement{
					{
						Workspace:      logicalcluster.Name("ws-1"),
						SchedulerScore: 80.0,
						CELScore:       70.0,
						FinalScore:     75.0,
					},
				},
			},
			wantError: true,
			errorMsg:  "insufficient workspaces selected",
		},
		"above maximum": {
			config: ValidationConfig{
				MaximumWorkspaces: 1,
			},
			decision: &PlacementDecision{
				ID:           "dec-1",
				RequestID:    "req-1",
				Status:       DecisionStatusComplete,
				DecisionTime: time.Now().Add(-time.Minute),
				SelectedWorkspaces: []*WorkspacePlacement{
					{
						Workspace:      logicalcluster.Name("ws-1"),
						SchedulerScore: 80.0,
						CELScore:       70.0,
						FinalScore:     75.0,
					},
					{
						Workspace:      logicalcluster.Name("ws-2"),
						SchedulerScore: 85.0,
						CELScore:       75.0,
						FinalScore:     80.0,
					},
				},
			},
			wantError: true,
			errorMsg:  "too many workspaces selected",
		},
		"within range": {
			config: ValidationConfig{
				MinimumWorkspaces: 1,
				MaximumWorkspaces: 3,
			},
			decision: &PlacementDecision{
				ID:           "dec-1",
				RequestID:    "req-1",
				Status:       DecisionStatusComplete,
				DecisionTime: time.Now().Add(-time.Minute),
				SelectedWorkspaces: []*WorkspacePlacement{
					{
						Workspace:      logicalcluster.Name("ws-1"),
						SchedulerScore: 80.0,
						CELScore:       70.0,
						FinalScore:     75.0,
					},
					{
						Workspace:      logicalcluster.Name("ws-2"),
						SchedulerScore: 85.0,
						CELScore:       75.0,
						FinalScore:     80.0,
					},
				},
			},
			wantError: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			validator := NewDecisionValidator(tc.config)
			err := validator.ValidateDecision(context.Background(), tc.decision)

			if tc.wantError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateResourceConstraints(t *testing.T) {
	validator := NewDecisionValidator(ValidationConfig{
		EnableResourceValidation: true,
		ResourceOvercommitThreshold: 0.8,
	})

	tests := map[string]struct {
		placements []*WorkspacePlacement
		wantError  bool
		errorMsg   string
	}{
		"valid resource allocation": {
			placements: []*WorkspacePlacement{
				{
					Workspace:      logicalcluster.Name("ws-1"),
					SchedulerScore: 80.0,
					CELScore:       70.0,
					FinalScore:     75.0,
					AllocatedResources: schedulerapi.ResourceAllocation{
						CPU:           resource.MustParse("2"),
						Memory:        resource.MustParse("4Gi"),
						Storage:       resource.MustParse("10Gi"),
						ReservationID: "res-1",
						ExpiresAt:     time.Now().Add(30 * time.Minute),
					},
				},
			},
			wantError: false,
		},
		"negative CPU allocation": {
			placements: []*WorkspacePlacement{
				{
					Workspace: logicalcluster.Name("ws-1"),
					AllocatedResources: schedulerapi.ResourceAllocation{
						CPU:           resource.MustParse("-1"),
						Memory:        resource.MustParse("4Gi"),
						Storage:       resource.MustParse("10Gi"),
						ReservationID: "res-1",
						ExpiresAt:     time.Now().Add(30 * time.Minute),
					},
				},
			},
			wantError: true,
			errorMsg:  "CPU allocation cannot be negative",
		},
		"empty reservation ID": {
			placements: []*WorkspacePlacement{
				{
					Workspace: logicalcluster.Name("ws-1"),
					AllocatedResources: schedulerapi.ResourceAllocation{
						CPU:           resource.MustParse("2"),
						Memory:        resource.MustParse("4Gi"),
						Storage:       resource.MustParse("10Gi"),
						ReservationID: "",
						ExpiresAt:     time.Now().Add(30 * time.Minute),
					},
				},
			},
			wantError: true,
			errorMsg:  "reservation ID cannot be empty",
		},
		"expired allocation": {
			placements: []*WorkspacePlacement{
				{
					Workspace: logicalcluster.Name("ws-1"),
					AllocatedResources: schedulerapi.ResourceAllocation{
						CPU:           resource.MustParse("2"),
						Memory:        resource.MustParse("4Gi"),
						Storage:       resource.MustParse("10Gi"),
						ReservationID: "res-1",
						ExpiresAt:     time.Now().Add(-time.Minute),
					},
				},
			},
			wantError: true,
			errorMsg:  "resource allocation has already expired",
		},
		"excessive CPU allocation": {
			placements: []*WorkspacePlacement{
				{
					Workspace: logicalcluster.Name("ws-1"),
					AllocatedResources: schedulerapi.ResourceAllocation{
						CPU:           resource.MustParse("200"), // 200 CPU cores
						Memory:        resource.MustParse("4Gi"),
						Storage:       resource.MustParse("10Gi"),
						ReservationID: "res-1",
						ExpiresAt:     time.Now().Add(30 * time.Minute),
					},
				},
			},
			wantError: true,
			errorMsg:  "CPU allocation exceeds overcommit threshold",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := validator.ValidateResourceConstraints(context.Background(), tc.placements)

			if tc.wantError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCheckConflicts(t *testing.T) {
	validator := NewDecisionValidator(ValidationConfig{
		EnableConflictChecking: true,
	})

	// Test with high resource allocation that should generate conflicts
	decision := &PlacementDecision{
		ID:           "dec-1",
		RequestID:    "req-1",
		Status:       DecisionStatusComplete,
		DecisionTime: time.Now().Add(-time.Minute),
		SelectedWorkspaces: []*WorkspacePlacement{
			{
				Workspace:      logicalcluster.Name("ws-1"),
				SchedulerScore: 80.0,
				CELScore:       70.0,
				FinalScore:     75.0,
				AllocatedResources: schedulerapi.ResourceAllocation{
					CPU:           resource.MustParse("128"), // High CPU allocation
					Memory:        resource.MustParse("4Gi"),
					Storage:       resource.MustParse("10Gi"),
					ReservationID: "res-1",
					ExpiresAt:     time.Now().Add(30 * time.Minute),
				},
			},
			{
				Workspace:      logicalcluster.Name("ws-2"),
				SchedulerScore: 85.0,
				CELScore:       75.0,
				FinalScore:     50.0, // Low final score
				AllocatedResources: schedulerapi.ResourceAllocation{
					CPU:           resource.MustParse("2"),
					Memory:        resource.MustParse("4Gi"),
					Storage:       resource.MustParse("10Gi"),
					ReservationID: "res-2",
					ExpiresAt:     time.Now().Add(30 * time.Minute),
				},
			},
		},
	}

	conflicts, err := validator.CheckConflicts(context.Background(), decision)
	require.NoError(t, err)
	require.NotEmpty(t, conflicts)

	// Should detect resource overcommit conflict
	var hasResourceConflict bool
	for _, conflict := range conflicts {
		if conflict.Type == ConflictTypeResourceOvercommit {
			hasResourceConflict = true
			assert.Equal(t, SeverityMedium, conflict.Severity)
			assert.Contains(t, conflict.Description, "High CPU allocation")
		}
	}
	assert.True(t, hasResourceConflict, "Should detect resource overcommit conflict")

	// Should detect policy violation for low score
	var hasPolicyConflict bool
	for _, conflict := range conflicts {
		if conflict.Type == ConflictTypePolicyViolation {
			hasPolicyConflict = true
			assert.Equal(t, SeverityLow, conflict.Severity)
			assert.Contains(t, conflict.Description, "low confidence score")
		}
	}
	assert.True(t, hasPolicyConflict, "Should detect policy violation for low score")

	// Should detect anti-affinity conflict
	var hasAffinityConflict bool
	for _, conflict := range conflicts {
		if conflict.Type == ConflictTypeAntiAffinityViolation {
			hasAffinityConflict = true
			assert.Equal(t, SeverityLow, conflict.Severity)
			assert.Contains(t, conflict.Description, "Multiple workspaces selected")
		}
	}
	assert.True(t, hasAffinityConflict, "Should detect anti-affinity conflict")
}

func TestValidateDecision_WithTimeout(t *testing.T) {
	config := ValidationConfig{
		EnableResourceValidation: true,
		EnablePolicyValidation:   true,
		EnableConflictChecking:   true,
		MaxValidationTime:       1 * time.Millisecond, // Very short timeout
	}
	validator := NewDecisionValidator(config)

	decision := &PlacementDecision{
		ID:           "dec-1",
		RequestID:    "req-1",
		Status:       DecisionStatusComplete,
		DecisionTime: time.Now().Add(-time.Minute),
		SelectedWorkspaces: []*WorkspacePlacement{
			{
				Workspace:      logicalcluster.Name("ws-1"),
				SchedulerScore: 80.0,
				CELScore:       70.0,
				FinalScore:     75.0,
				AllocatedResources: schedulerapi.ResourceAllocation{
					CPU:           resource.MustParse("2"),
					Memory:        resource.MustParse("4Gi"),
					Storage:       resource.MustParse("10Gi"),
					ReservationID: "res-1",
					ExpiresAt:     time.Now().Add(30 * time.Minute),
				},
			},
		},
	}

	// The validation should complete even with a short timeout for this simple case
	err := validator.ValidateDecision(context.Background(), decision)
	require.NoError(t, err)
}

func TestValidateDecision_WithCriticalConflict(t *testing.T) {
	// Create a validator that would generate a critical conflict
	validator := &defaultDecisionValidator{
		config: ValidationConfig{
			EnableConflictChecking: true,
		},
	}

	// Override the conflict checking to simulate a critical conflict
	originalCheckConflicts := validator.CheckConflicts
	validator.CheckConflicts = func(ctx context.Context, decision *PlacementDecision) ([]ConflictDescription, error) {
		return []ConflictDescription{
			{
				Type:        ConflictTypeResourceOvercommit,
				Description: "Critical resource overcommit detected",
				Severity:    SeverityCritical,
				AffectedWorkspaces: []logicalcluster.Name{
					logicalcluster.Name("ws-1"),
				},
				ResolutionSuggestion: "Reduce resource requirements",
			},
		}, nil
	}
	defer func() {
		validator.CheckConflicts = originalCheckConflicts
	}()

	decision := &PlacementDecision{
		ID:           "dec-1",
		RequestID:    "req-1",
		Status:       DecisionStatusComplete,
		DecisionTime: time.Now().Add(-time.Minute),
		SelectedWorkspaces: []*WorkspacePlacement{
			{
				Workspace:      logicalcluster.Name("ws-1"),
				SchedulerScore: 80.0,
				CELScore:       70.0,
				FinalScore:     75.0,
				AllocatedResources: schedulerapi.ResourceAllocation{
					CPU:           resource.MustParse("2"),
					Memory:        resource.MustParse("4Gi"),
					Storage:       resource.MustParse("10Gi"),
					ReservationID: "res-1",
					ExpiresAt:     time.Now().Add(30 * time.Minute),
				},
			},
		},
	}

	err := validator.ValidateDecision(context.Background(), decision)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "critical conflict detected")
}