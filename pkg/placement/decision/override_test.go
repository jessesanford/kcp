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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kcp-dev/logicalcluster/v3"
)

// MockOverrideStorage implements OverrideStorage for testing.
type MockOverrideStorage struct {
	overrides    map[string]*PlacementOverride
	history      map[string][]*OverrideHistoryEntry
	storeError   error
	updateError  error
	deleteError  error
	queryError   error
	historyError error
	cleanupError error
}

func NewMockOverrideStorage() *MockOverrideStorage {
	return &MockOverrideStorage{
		overrides: make(map[string]*PlacementOverride),
		history:   make(map[string][]*OverrideHistoryEntry),
	}
}

func (m *MockOverrideStorage) StoreOverride(ctx context.Context, override *PlacementOverride) error {
	if m.storeError != nil {
		return m.storeError
	}
	m.overrides[override.ID] = override
	return nil
}

func (m *MockOverrideStorage) UpdateOverride(ctx context.Context, override *PlacementOverride) error {
	if m.updateError != nil {
		return m.updateError
	}
	m.overrides[override.ID] = override
	return nil
}

func (m *MockOverrideStorage) DeleteOverride(ctx context.Context, overrideID string) error {
	if m.deleteError != nil {
		return m.deleteError
	}
	delete(m.overrides, overrideID)
	return nil
}

func (m *MockOverrideStorage) QueryOverrides(ctx context.Context, filter *OverrideFilter) ([]*PlacementOverride, error) {
	if m.queryError != nil {
		return nil, m.queryError
	}
	
	var results []*PlacementOverride
	for _, override := range m.overrides {
		if m.matchesFilter(override, filter) {
			results = append(results, override)
		}
	}
	return results, nil
}

func (m *MockOverrideStorage) GetOverrideHistory(ctx context.Context, placementID string) ([]*OverrideHistoryEntry, error) {
	if m.historyError != nil {
		return nil, m.historyError
	}
	return m.history[placementID], nil
}

func (m *MockOverrideStorage) CleanupExpiredOverrides(ctx context.Context) error {
	if m.cleanupError != nil {
		return m.cleanupError
	}
	
	// Remove expired overrides
	for id, override := range m.overrides {
		if override.ExpiresAt != nil && time.Now().After(*override.ExpiresAt) {
			delete(m.overrides, id)
		}
	}
	return nil
}

func (m *MockOverrideStorage) matchesFilter(override *PlacementOverride, filter *OverrideFilter) bool {
	if filter == nil {
		return true
	}
	if filter.PlacementID != "" && override.PlacementID != filter.PlacementID {
		return false
	}
	if filter.OverrideType != "" && override.OverrideType != filter.OverrideType {
		return false
	}
	if filter.AppliedBy != "" && override.AppliedBy != filter.AppliedBy {
		return false
	}
	return true
}

// MockOverrideValidator implements OverrideValidator for testing.
type MockOverrideValidator struct {
	validateError error
}

func NewMockOverrideValidator() *MockOverrideValidator {
	return &MockOverrideValidator{}
}

func (m *MockOverrideValidator) ValidateOverride(ctx context.Context, override *PlacementOverride) error {
	return m.validateError
}

func TestNewOverrideManager(t *testing.T) {
	tests := map[string]struct {
		storage       OverrideStorage
		eventRecorder *MockEventRecorder
		validator     OverrideValidator
		config        *OverrideManagerConfig
		wantError     bool
		errorContains string
	}{
		"valid configuration": {
			storage:       NewMockOverrideStorage(),
			eventRecorder: NewMockEventRecorder(),
			validator:     NewMockOverrideValidator(),
			config:        DefaultOverrideManagerConfig(),
			wantError:     false,
		},
		"nil storage": {
			storage:       nil,
			eventRecorder: NewMockEventRecorder(),
			validator:     NewMockOverrideValidator(),
			config:        DefaultOverrideManagerConfig(),
			wantError:     true,
			errorContains: "override storage cannot be nil",
		},
		"nil event recorder": {
			storage:       NewMockOverrideStorage(),
			eventRecorder: nil,
			validator:     NewMockOverrideValidator(),
			config:        DefaultOverrideManagerConfig(),
			wantError:     true,
			errorContains: "event recorder cannot be nil",
		},
		"nil validator": {
			storage:       NewMockOverrideStorage(),
			eventRecorder: NewMockEventRecorder(),
			validator:     nil,
			config:        DefaultOverrideManagerConfig(),
			wantError:     true,
			errorContains: "override validator cannot be nil",
		},
		"nil config uses default": {
			storage:       NewMockOverrideStorage(),
			eventRecorder: NewMockEventRecorder(),
			validator:     NewMockOverrideValidator(),
			config:        nil,
			wantError:     false,
		},
	}
	
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			manager, err := NewOverrideManager(tc.storage, tc.eventRecorder, tc.validator, tc.config)
			
			if tc.wantError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorContains)
				assert.Nil(t, manager)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, manager)
			}
		})
	}
}

func TestOverrideManager_CreateOverride(t *testing.T) {
	tests := map[string]struct {
		request       *CreateOverrideRequest
		validateError error
		storageError  error
		wantError     bool
		errorContains string
	}{
		"successful override creation": {
			request: &CreateOverrideRequest{
				PlacementID:      "placement-1",
				OverrideType:     OverrideTypeForce,
				TargetWorkspaces: []logicalcluster.Name{logicalcluster.Name("root:target")},
				Reason:           "Emergency placement required",
				AppliedBy:        "admin@example.com",
				Priority:         100,
			},
			wantError: false,
		},
		"nil request": {
			request:       nil,
			wantError:     true,
			errorContains: "create override request cannot be nil",
		},
		"validation error": {
			request: &CreateOverrideRequest{
				PlacementID:  "placement-1",
				OverrideType: OverrideTypeForce,
				Reason:       "Test override",
				AppliedBy:    "admin@example.com",
			},
			validateError: errors.New("validation failed"),
			wantError:     true,
			errorContains: "override validation failed",
		},
		"storage error": {
			request: &CreateOverrideRequest{
				PlacementID:      "placement-1",
				OverrideType:     OverrideTypeForce,
				TargetWorkspaces: []logicalcluster.Name{logicalcluster.Name("root:target")},
				Reason:           "Test override",
				AppliedBy:        "admin@example.com",
			},
			storageError:  errors.New("storage failure"),
			wantError:     true,
			errorContains: "failed to store override",
		},
	}
	
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			storage := NewMockOverrideStorage()
			storage.storeError = tc.storageError
			eventRecorder := NewMockEventRecorder()
			validator := NewMockOverrideValidator()
			validator.validateError = tc.validateError
			
			manager, err := NewOverrideManager(storage, eventRecorder, validator, DefaultOverrideManagerConfig())
			require.NoError(t, err)
			
			ctx := context.Background()
			override, err := manager.CreateOverride(ctx, tc.request)
			
			if tc.wantError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorContains)
				assert.Nil(t, override)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, override)
				assert.Equal(t, tc.request.PlacementID, override.PlacementID)
				assert.Equal(t, tc.request.OverrideType, override.OverrideType)
				assert.Equal(t, tc.request.Reason, override.Reason)
				assert.NotEmpty(t, override.ID)
				assert.False(t, override.CreatedAt.IsZero())
			}
		})
	}
}

func TestOverrideManager_ApplyOverride(t *testing.T) {
	tests := map[string]struct {
		decision        *PlacementDecision
		override        *PlacementOverride
		wantError       bool
		errorContains   string
		validateChanges func(*testing.T, *PlacementDecision)
	}{
		"successful force override": {
			decision: &PlacementDecision{
				ID:        "decision-1",
				RequestID: "request-1",
				Status:    DecisionStatusComplete,
				SelectedWorkspaces: []*WorkspacePlacement{
					{Workspace: logicalcluster.Name("root:original"), FinalScore: 80.0},
				},
				DecisionRationale: DecisionRationale{Summary: "Original decision"},
			},
			override: &PlacementOverride{
				ID:               "override-1",
				OverrideType:     OverrideTypeForce,
				TargetWorkspaces: []logicalcluster.Name{logicalcluster.Name("root:forced")},
				Reason:           "Emergency placement",
				AppliedBy:        "admin",
				CreatedAt:        time.Now(),
			},
			wantError: false,
			validateChanges: func(t *testing.T, result *PlacementDecision) {
				assert.Equal(t, DecisionStatusOverridden, result.Status)
				assert.Len(t, result.SelectedWorkspaces, 1)
				assert.Equal(t, logicalcluster.Name("root:forced"), result.SelectedWorkspaces[0].Workspace)
				assert.Equal(t, 100.0, result.SelectedWorkspaces[0].FinalScore)
				assert.Contains(t, result.DecisionRationale.OverrideFactors, "Applied Force override by admin: Emergency placement")
			},
		},
		"successful exclude override": {
			decision: &PlacementDecision{
				ID:        "decision-1",
				RequestID: "request-1",
				Status:    DecisionStatusComplete,
				SelectedWorkspaces: []*WorkspacePlacement{
					{Workspace: logicalcluster.Name("root:keep"), FinalScore: 90.0},
					{Workspace: logicalcluster.Name("root:exclude"), FinalScore: 85.0},
				},
				DecisionRationale: DecisionRationale{Summary: "Original decision"},
			},
			override: &PlacementOverride{
				ID:                 "override-1",
				OverrideType:       OverrideTypeExclude,
				ExcludedWorkspaces: []logicalcluster.Name{logicalcluster.Name("root:exclude")},
				Reason:             "Maintenance window",
				AppliedBy:          "admin",
				CreatedAt:          time.Now(),
			},
			wantError: false,
			validateChanges: func(t *testing.T, result *PlacementDecision) {
				assert.Equal(t, DecisionStatusOverridden, result.Status)
				assert.Len(t, result.SelectedWorkspaces, 1)
				assert.Equal(t, logicalcluster.Name("root:keep"), result.SelectedWorkspaces[0].Workspace)
				assert.Len(t, result.RejectedCandidates, 1)
				assert.Equal(t, logicalcluster.Name("root:exclude"), result.RejectedCandidates[0].Workspace)
			},
		},
		"nil decision": {
			decision:      nil,
			override:      &PlacementOverride{ID: "override-1"},
			wantError:     true,
			errorContains: "placement decision cannot be nil",
		},
		"nil override": {
			decision: &PlacementDecision{ID: "decision-1"},
			override: nil,
			wantError: true,
			errorContains: "placement override cannot be nil",
		},
		"expired override": {
			decision: &PlacementDecision{
				ID:        "decision-1",
				RequestID: "request-1",
				Status:    DecisionStatusComplete,
			},
			override: &PlacementOverride{
				ID:           "override-1",
				OverrideType: OverrideTypeForce,
				ExpiresAt:    &[]time.Time{time.Now().Add(-time.Hour)}[0], // Expired
				CreatedAt:    time.Now().Add(-2 * time.Hour),
			},
			wantError:     true,
			errorContains: "override override-1 has expired",
		},
	}
	
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			storage := NewMockOverrideStorage()
			eventRecorder := NewMockEventRecorder()
			validator := NewMockOverrideValidator()
			
			manager, err := NewOverrideManager(storage, eventRecorder, validator, DefaultOverrideManagerConfig())
			require.NoError(t, err)
			
			ctx := context.Background()
			result, err := manager.ApplyOverride(ctx, tc.decision, tc.override)
			
			if tc.wantError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorContains)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, result)
				if tc.validateChanges != nil {
					tc.validateChanges(t, result)
				}
			}
		})
	}
}

func TestOverrideManager_GetActiveOverrides(t *testing.T) {
	storage := NewMockOverrideStorage()
	eventRecorder := NewMockEventRecorder()
	validator := NewMockOverrideValidator()
	
	manager, err := NewOverrideManager(storage, eventRecorder, validator, DefaultOverrideManagerConfig())
	require.NoError(t, err)
	
	ctx := context.Background()
	
	// Create an active override
	request := &CreateOverrideRequest{
		PlacementID:      "placement-1",
		OverrideType:     OverrideTypeForce,
		TargetWorkspaces: []logicalcluster.Name{logicalcluster.Name("root:target")},
		Reason:           "Test override",
		AppliedBy:        "admin",
	}
	
	override, err := manager.CreateOverride(ctx, request)
	require.NoError(t, err)
	
	// Test getting active overrides
	activeOverrides, err := manager.GetActiveOverrides(ctx, "placement-1")
	require.NoError(t, err)
	assert.Len(t, activeOverrides, 1)
	assert.Equal(t, override.ID, activeOverrides[0].ID)
	
	// Test with non-existent placement
	activeOverrides, err = manager.GetActiveOverrides(ctx, "non-existent")
	require.NoError(t, err)
	assert.Len(t, activeOverrides, 0)
	
	// Test with empty placement ID
	_, err = manager.GetActiveOverrides(ctx, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "placement ID cannot be empty")
}

func TestOverrideManager_ExpireOverride(t *testing.T) {
	storage := NewMockOverrideStorage()
	eventRecorder := NewMockEventRecorder()
	validator := NewMockOverrideValidator()
	
	manager, err := NewOverrideManager(storage, eventRecorder, validator, DefaultOverrideManagerConfig())
	require.NoError(t, err)
	
	ctx := context.Background()
	
	// Create an override
	request := &CreateOverrideRequest{
		PlacementID:      "placement-1",
		OverrideType:     OverrideTypeForce,
		TargetWorkspaces: []logicalcluster.Name{logicalcluster.Name("root:target")},
		Reason:           "Test override",
		AppliedBy:        "admin",
	}
	
	override, err := manager.CreateOverride(ctx, request)
	require.NoError(t, err)
	
	// Expire the override
	err = manager.ExpireOverride(ctx, override.ID, "Test expiration")
	require.NoError(t, err)
	
	// Verify it's no longer active
	activeOverrides, err := manager.GetActiveOverrides(ctx, "placement-1")
	require.NoError(t, err)
	assert.Len(t, activeOverrides, 0)
	
	// Test with non-existent override
	err = manager.ExpireOverride(ctx, "non-existent", "Test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "override non-existent not found")
	
	// Test with empty override ID
	err = manager.ExpireOverride(ctx, "", "Test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "override ID cannot be empty")
}

func TestOverrideManager_DeleteOverride(t *testing.T) {
	storage := NewMockOverrideStorage()
	eventRecorder := NewMockEventRecorder()
	validator := NewMockOverrideValidator()
	
	manager, err := NewOverrideManager(storage, eventRecorder, validator, DefaultOverrideManagerConfig())
	require.NoError(t, err)
	
	ctx := context.Background()
	
	// Create an override
	request := &CreateOverrideRequest{
		PlacementID:      "placement-1",
		OverrideType:     OverrideTypeForce,
		TargetWorkspaces: []logicalcluster.Name{logicalcluster.Name("root:target")},
		Reason:           "Test override",
		AppliedBy:        "admin",
	}
	
	override, err := manager.CreateOverride(ctx, request)
	require.NoError(t, err)
	
	// Delete the override
	err = manager.DeleteOverride(ctx, override.ID, "Test deletion")
	require.NoError(t, err)
	
	// Verify it's no longer active
	activeOverrides, err := manager.GetActiveOverrides(ctx, "placement-1")
	require.NoError(t, err)
	assert.Len(t, activeOverrides, 0)
	
	// Verify it's deleted from storage
	assert.NotContains(t, storage.overrides, override.ID)
}

func TestOverrideManager_ListOverrides(t *testing.T) {
	storage := NewMockOverrideStorage()
	eventRecorder := NewMockEventRecorder()
	validator := NewMockOverrideValidator()
	
	manager, err := NewOverrideManager(storage, eventRecorder, validator, DefaultOverrideManagerConfig())
	require.NoError(t, err)
	
	ctx := context.Background()
	
	// Create multiple overrides
	requests := []*CreateOverrideRequest{
		{
			PlacementID:      "placement-1",
			OverrideType:     OverrideTypeForce,
			TargetWorkspaces: []logicalcluster.Name{logicalcluster.Name("root:target1")},
			Reason:           "Test override 1",
			AppliedBy:        "admin1",
		},
		{
			PlacementID:      "placement-2",
			OverrideType:     OverrideTypeExclude,
			ExcludedWorkspaces: []logicalcluster.Name{logicalcluster.Name("root:exclude")},
			Reason:           "Test override 2",
			AppliedBy:        "admin2",
		},
	}
	
	for _, request := range requests {
		_, err := manager.CreateOverride(ctx, request)
		require.NoError(t, err)
	}
	
	// List all overrides
	overrides, err := manager.ListOverrides(ctx, &OverrideFilter{})
	require.NoError(t, err)
	assert.Len(t, overrides, 2)
	
	// List with filter
	overrides, err = manager.ListOverrides(ctx, &OverrideFilter{OverrideType: OverrideTypeForce})
	require.NoError(t, err)
	assert.Len(t, overrides, 1)
	assert.Equal(t, OverrideTypeForce, overrides[0].OverrideType)
}

func TestCreateOverrideRequest_Validate(t *testing.T) {
	tests := map[string]struct {
		request       *CreateOverrideRequest
		wantError     bool
		errorContains string
	}{
		"valid force override": {
			request: &CreateOverrideRequest{
				PlacementID:      "placement-1",
				OverrideType:     OverrideTypeForce,
				TargetWorkspaces: []logicalcluster.Name{logicalcluster.Name("root:target")},
				Reason:           "Emergency placement",
				AppliedBy:        "admin@example.com",
			},
			wantError: false,
		},
		"valid exclude override": {
			request: &CreateOverrideRequest{
				PlacementID:        "placement-1",
				OverrideType:       OverrideTypeExclude,
				ExcludedWorkspaces: []logicalcluster.Name{logicalcluster.Name("root:exclude")},
				Reason:             "Maintenance window",
				AppliedBy:          "admin@example.com",
			},
			wantError: false,
		},
		"empty placement ID": {
			request: &CreateOverrideRequest{
				PlacementID:  "", // Invalid
				OverrideType: OverrideTypeForce,
				Reason:       "Test",
				AppliedBy:    "admin",
			},
			wantError:     true,
			errorContains: "placement ID cannot be empty",
		},
		"empty reason": {
			request: &CreateOverrideRequest{
				PlacementID:  "placement-1",
				OverrideType: OverrideTypeForce,
				Reason:       "", // Invalid
				AppliedBy:    "admin",
			},
			wantError:     true,
			errorContains: "reason cannot be empty",
		},
		"force override without target workspaces": {
			request: &CreateOverrideRequest{
				PlacementID:      "placement-1",
				OverrideType:     OverrideTypeForce,
				TargetWorkspaces: nil, // Invalid for force override
				Reason:           "Test",
				AppliedBy:        "admin",
			},
			wantError:     true,
			errorContains: "force override requires target workspaces",
		},
		"exclude override without excluded workspaces": {
			request: &CreateOverrideRequest{
				PlacementID:        "placement-1",
				OverrideType:       OverrideTypeExclude,
				ExcludedWorkspaces: nil, // Invalid for exclude override
				Reason:             "Test",
				AppliedBy:          "admin",
			},
			wantError:     true,
			errorContains: "exclude override requires excluded workspaces",
		},
	}
	
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := tc.request.Validate()
			
			if tc.wantError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestOverrideManagerConfig_Validate(t *testing.T) {
	tests := map[string]struct {
		config        *OverrideManagerConfig
		wantError     bool
		errorContains string
	}{
		"valid config": {
			config: &OverrideManagerConfig{
				MaxActiveOverridesPerPlacement: 10,
				DefaultOverridePriority:        50,
				CleanupInterval:                time.Hour,
				CleanupTimeout:                 5 * time.Minute,
				PreferenceScoreBoost:           20.0,
				AvoidanceScorePenalty:          15.0,
			},
			wantError: false,
		},
		"invalid max active overrides": {
			config: &OverrideManagerConfig{
				MaxActiveOverridesPerPlacement: 0, // Invalid
				DefaultOverridePriority:        50,
				CleanupInterval:                time.Hour,
				CleanupTimeout:                 5 * time.Minute,
				PreferenceScoreBoost:           20.0,
				AvoidanceScorePenalty:          15.0,
			},
			wantError:     true,
			errorContains: "max active overrides per placement must be positive",
		},
		"invalid cleanup interval": {
			config: &OverrideManagerConfig{
				MaxActiveOverridesPerPlacement: 10,
				DefaultOverridePriority:        50,
				CleanupInterval:                0, // Invalid
				CleanupTimeout:                 5 * time.Minute,
				PreferenceScoreBoost:           20.0,
				AvoidanceScorePenalty:          15.0,
			},
			wantError:     true,
			errorContains: "cleanup interval must be positive",
		},
	}
	
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := tc.config.Validate()
			
			if tc.wantError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDefaultOverrideManagerConfig(t *testing.T) {
	config := DefaultOverrideManagerConfig()
	
	assert.True(t, config.MaxActiveOverridesPerPlacement > 0)
	assert.True(t, config.DefaultOverridePriority >= 0)
	assert.True(t, config.CleanupInterval > 0)
	assert.True(t, config.CleanupTimeout > 0)
	assert.True(t, config.PreferenceScoreBoost >= 0)
	assert.True(t, config.AvoidanceScorePenalty >= 0)
	
	// Validate the default config
	err := config.Validate()
	assert.NoError(t, err)
}