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

package v1alpha1

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	tmcv1alpha1 "github.com/kcp-dev/kcp/apis/tmc/v1alpha1"
)

func TestTMCStatus_Validation(t *testing.T) {
	now := metav1.Now()
	
	tests := []struct {
		name    string
		status  *tmcv1alpha1.TMCStatus
		wantErr bool
	}{
		{
			name: "valid status with all fields",
			status: &tmcv1alpha1.TMCStatus{
				Conditions: []metav1.Condition{
					{
						Type:               "Ready",
						Status:             metav1.ConditionTrue,
						Reason:             "ConfigApplied",
						Message:            "TMC configuration applied successfully",
						LastTransitionTime: now,
					},
				},
				Phase:              "Running",
				ObservedGeneration: 5,
			},
			wantErr: false,
		},
		{
			name: "valid status with minimal fields",
			status: &tmcv1alpha1.TMCStatus{
				Conditions:         []metav1.Condition{},
				Phase:              "",
				ObservedGeneration: 0,
			},
			wantErr: false,
		},
		{
			name: "valid status with multiple conditions",
			status: &tmcv1alpha1.TMCStatus{
				Conditions: []metav1.Condition{
					{
						Type:               "Ready",
						Status:             metav1.ConditionTrue,
						Reason:             "ConfigApplied",
						LastTransitionTime: now,
					},
					{
						Type:               "Synced",
						Status:             metav1.ConditionFalse,
						Reason:             "SyncInProgress",
						LastTransitionTime: now,
					},
				},
				Phase:              "Running",
				ObservedGeneration: 3,
			},
			wantErr: false,
		},
		{
			name: "invalid phase",
			status: &tmcv1alpha1.TMCStatus{
				Phase: "InvalidPhase",
			},
			wantErr: true,
		},
		{
			name: "duplicate condition types",
			status: &tmcv1alpha1.TMCStatus{
				Conditions: []metav1.Condition{
					{
						Type:               "Ready",
						Status:             metav1.ConditionTrue,
						Reason:             "ConfigApplied",
						LastTransitionTime: now,
					},
					{
						Type:               "Ready",
						Status:             metav1.ConditionFalse,
						Reason:             "ConfigFailed",
						LastTransitionTime: now,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "condition with empty type",
			status: &tmcv1alpha1.TMCStatus{
				Conditions: []metav1.Condition{
					{
						Type:               "",
						Status:             metav1.ConditionTrue,
						Reason:             "SomeReason",
						LastTransitionTime: now,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "condition with empty status",
			status: &tmcv1alpha1.TMCStatus{
				Conditions: []metav1.Condition{
					{
						Type:               "Ready",
						Status:             "",
						Reason:             "SomeReason",
						LastTransitionTime: now,
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fldPath := field.NewPath("status")
			errs := tmcv1alpha1.ValidateTMCStatus(tt.status, fldPath)
			hasErr := len(errs) > 0

			if hasErr != tt.wantErr {
				t.Errorf("ValidateTMCStatus() error = %v, wantErr %v", errs, tt.wantErr)
			}
		})
	}
}

func TestTMCStatus_ValidPhases(t *testing.T) {
	validPhases := []string{
		"Pending",
		"Running", 
		"Succeeded",
		"Failed",
		"Unknown",
		"Terminating",
	}

	for _, phase := range validPhases {
		t.Run(phase, func(t *testing.T) {
			status := &tmcv1alpha1.TMCStatus{
				Phase: phase,
			}
			
			fldPath := field.NewPath("status")
			errs := tmcv1alpha1.ValidateTMCStatus(status, fldPath)
			
			if len(errs) > 0 {
				t.Errorf("Phase %s should be valid, got errors: %v", phase, errs)
			}
		})
	}
}

func TestTMCStatus_InvalidPhases(t *testing.T) {
	invalidPhases := []string{
		"invalid",
		"PENDING",
		"running",
		"Complete",
		"Error",
	}

	for _, phase := range invalidPhases {
		t.Run(phase, func(t *testing.T) {
			status := &tmcv1alpha1.TMCStatus{
				Phase: phase,
			}
			
			fldPath := field.NewPath("status")
			errs := tmcv1alpha1.ValidateTMCStatus(status, fldPath)
			
			if len(errs) == 0 {
				t.Errorf("Phase %s should be invalid", phase)
			}
		})
	}
}

func TestTMCStatus_ConditionManagement(t *testing.T) {
	now := metav1.Now()
	later := metav1.NewTime(now.Add(1 * time.Hour))
	
	status := &tmcv1alpha1.TMCStatus{
		Conditions: []metav1.Condition{
			{
				Type:               "Ready",
				Status:             metav1.ConditionFalse,
				Reason:             "Initializing",
				Message:            "System is initializing",
				LastTransitionTime: now,
			},
		},
		Phase:              "Pending",
		ObservedGeneration: 1,
	}

	// Validate initial state
	fldPath := field.NewPath("status")
	errs := tmcv1alpha1.ValidateTMCStatus(status, fldPath)
	if len(errs) > 0 {
		t.Errorf("Initial status should be valid, got errors: %v", errs)
	}

	// Update condition
	status.Conditions[0].Status = metav1.ConditionTrue
	status.Conditions[0].Reason = "SystemReady"
	status.Conditions[0].Message = "System is ready"
	status.Conditions[0].LastTransitionTime = later
	status.Phase = "Running"
	status.ObservedGeneration = 2

	// Validate updated state
	errs = tmcv1alpha1.ValidateTMCStatus(status, fldPath)
	if len(errs) > 0 {
		t.Errorf("Updated status should be valid, got errors: %v", errs)
	}

	// Verify the values
	if status.Conditions[0].Status != metav1.ConditionTrue {
		t.Errorf("Expected condition status to be True, got: %s", status.Conditions[0].Status)
	}
	
	if status.Phase != "Running" {
		t.Errorf("Expected phase to be Running, got: %s", status.Phase)
	}
	
	if status.ObservedGeneration != 2 {
		t.Errorf("Expected observed generation to be 2, got: %d", status.ObservedGeneration)
	}
}

func TestTMCStatus_DeepCopy(t *testing.T) {
	now := metav1.Now()
	
	original := &tmcv1alpha1.TMCStatus{
		Conditions: []metav1.Condition{
			{
				Type:               "Ready",
				Status:             metav1.ConditionTrue,
				Reason:             "ConfigApplied",
				Message:            "Configuration applied successfully",
				LastTransitionTime: now,
			},
		},
		Phase:              "Running",
		ObservedGeneration: 5,
	}

	// Test DeepCopy
	copy := original.DeepCopy()
	
	// Modify the copy
	copy.Phase = "Failed"
	copy.ObservedGeneration = 6
	copy.Conditions[0].Message = "Modified message"

	// Original should remain unchanged
	if original.Phase != "Running" {
		t.Errorf("Original phase should be unchanged, got: %s", original.Phase)
	}
	
	if original.ObservedGeneration != 5 {
		t.Errorf("Original observed generation should be unchanged, got: %d", original.ObservedGeneration)
	}
	
	if original.Conditions[0].Message != "Configuration applied successfully" {
		t.Errorf("Original condition message should be unchanged, got: %s", original.Conditions[0].Message)
	}

	// Copy should have the modifications
	if copy.Phase != "Failed" {
		t.Errorf("Copy phase should be modified, got: %s", copy.Phase)
	}
	
	if copy.ObservedGeneration != 6 {
		t.Errorf("Copy observed generation should be modified, got: %d", copy.ObservedGeneration)
	}
	
	if copy.Conditions[0].Message != "Modified message" {
		t.Errorf("Copy condition message should be modified, got: %s", copy.Conditions[0].Message)
	}
}

// Test TMCStatus lifecycle transitions
func TestTMCStatus_LifecycleTransitions(t *testing.T) {
	now := metav1.Now()
	
	// Test key transitions
	status := &tmcv1alpha1.TMCStatus{
		Phase: "Pending",
		Conditions: []metav1.Condition{
			{
				Type:               "Ready",
				Status:             metav1.ConditionFalse,
				Reason:             "Initializing",
				LastTransitionTime: now,
			},
		},
	}

	// Test Pending -> Running transition
	status.Phase = "Running"
	fldPath := field.NewPath("status")
	errs := tmcv1alpha1.ValidateTMCStatus(status, fldPath)
	if len(errs) > 0 {
		t.Errorf("Pending to Running transition should be valid, got errors: %v", errs)
	}

	// Test Running -> Succeeded transition
	status.Phase = "Succeeded"
	errs = tmcv1alpha1.ValidateTMCStatus(status, fldPath)
	if len(errs) > 0 {
		t.Errorf("Running to Succeeded transition should be valid, got errors: %v", errs)
	}
}