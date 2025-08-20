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
	"sync"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	tmcv1alpha1 "github.com/kcp-dev/kcp/apis/tmc/v1alpha1"
)

func TestTMCConfig_Validation(t *testing.T) {
	tests := []struct {
		name    string
		config  *tmcv1alpha1.TMCConfig
		wantErr bool
	}{
		{
			name: "valid TMCConfig",
			config: &tmcv1alpha1.TMCConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-config",
				},
				Spec: tmcv1alpha1.TMCConfigSpec{
					FeatureFlags: map[string]bool{
						"workloadSyncing": true,
						"statusTracking":  false,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "empty name",
			config: &tmcv1alpha1.TMCConfig{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: tmcv1alpha1.TMCConfigSpec{
					FeatureFlags: map[string]bool{
						"workloadSyncing": true,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid name format",
			config: &tmcv1alpha1.TMCConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: "Invalid_Name",
				},
				Spec: tmcv1alpha1.TMCConfigSpec{
					FeatureFlags: map[string]bool{
						"workloadSyncing": true,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "empty feature flag name",
			config: &tmcv1alpha1.TMCConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-config",
				},
				Spec: tmcv1alpha1.TMCConfigSpec{
					FeatureFlags: map[string]bool{
						"":                true,
						"workloadSyncing": false,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "no feature flags",
			config: &tmcv1alpha1.TMCConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-config",
				},
				Spec: tmcv1alpha1.TMCConfigSpec{},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := tmcv1alpha1.ValidateTMCConfig(tt.config)
			hasErr := len(errs) > 0

			if hasErr != tt.wantErr {
				t.Errorf("ValidateTMCConfig() error = %v, wantErr %v", errs, tt.wantErr)
			}
		})
	}
}

func TestTMCConfigSpec_Validation(t *testing.T) {
	tests := []struct {
		name    string
		spec    *tmcv1alpha1.TMCConfigSpec
		wantErr bool
	}{
		{
			name: "valid spec with feature flags",
			spec: &tmcv1alpha1.TMCConfigSpec{
				FeatureFlags: map[string]bool{
					"workloadSyncing": true,
					"statusTracking":  false,
					"multiCluster":    true,
				},
			},
			wantErr: false,
		},
		{
			name: "valid spec without feature flags",
			spec: &tmcv1alpha1.TMCConfigSpec{
				FeatureFlags: nil,
			},
			wantErr: false,
		},
		{
			name: "valid spec with empty feature flags",
			spec: &tmcv1alpha1.TMCConfigSpec{
				FeatureFlags: map[string]bool{},
			},
			wantErr: false,
		},
		{
			name: "invalid spec with empty feature flag name",
			spec: &tmcv1alpha1.TMCConfigSpec{
				FeatureFlags: map[string]bool{
					"":         true,
					"validFlag": false,
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fldPath := field.NewPath("spec")
			errs := tmcv1alpha1.ValidateTMCConfigSpec(tt.spec, fldPath)
			hasErr := len(errs) > 0

			if hasErr != tt.wantErr {
				t.Errorf("ValidateTMCConfigSpec() error = %v, wantErr %v", errs, tt.wantErr)
			}
		})
	}
}

func TestTMCConfig_DefaultValues(t *testing.T) {
	config := &tmcv1alpha1.TMCConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-config",
		},
	}

	// Test that a TMCConfig can be created without explicit spec
	if config.Spec.FeatureFlags == nil {
		// This is expected - feature flags are optional
		t.Logf("Feature flags are nil as expected for default TMCConfig")
	}

	// Validate the default config
	errs := tmcv1alpha1.ValidateTMCConfig(config)
	if len(errs) > 0 {
		t.Errorf("Default TMCConfig should be valid, got errors: %v", errs)
	}
}

func TestTMCConfig_DeepCopy(t *testing.T) {
	original := &tmcv1alpha1.TMCConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-config",
			Labels: map[string]string{
				"test": "label",
			},
		},
		Spec: tmcv1alpha1.TMCConfigSpec{
			FeatureFlags: map[string]bool{
				"workloadSyncing": true,
				"statusTracking":  false,
			},
		},
		Status: tmcv1alpha1.TMCConfigStatus{
			Conditions: []metav1.Condition{
				{
					Type:   "Ready",
					Status: metav1.ConditionTrue,
					Reason: "ConfigApplied",
				},
			},
		},
	}

	// Test DeepCopy
	copy := original.DeepCopy()
	
	// Modify the copy
	copy.Name = "modified-config"
	copy.Spec.FeatureFlags["newFlag"] = true
	copy.Status.Conditions[0].Message = "Modified"

	// Original should remain unchanged
	if original.Name != "test-config" {
		t.Errorf("Original name should be unchanged, got: %s", original.Name)
	}
	
	if original.Spec.FeatureFlags["newFlag"] {
		t.Errorf("Original should not have newFlag")
	}
	
	if original.Status.Conditions[0].Message == "Modified" {
		t.Errorf("Original condition message should be unchanged")
	}

	// Copy should have the modifications
	if copy.Name != "modified-config" {
		t.Errorf("Copy name should be modified, got: %s", copy.Name)
	}
	
	if !copy.Spec.FeatureFlags["newFlag"] {
		t.Errorf("Copy should have newFlag set to true")
	}
}

func TestTMCConfigList_DeepCopy(t *testing.T) {
	original := &tmcv1alpha1.TMCConfigList{
		Items: []tmcv1alpha1.TMCConfig{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "config1",
				},
				Spec: tmcv1alpha1.TMCConfigSpec{
					FeatureFlags: map[string]bool{
						"flag1": true,
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "config2",
				},
				Spec: tmcv1alpha1.TMCConfigSpec{
					FeatureFlags: map[string]bool{
						"flag2": false,
					},
				},
			},
		},
	}

	// Test DeepCopy
	copy := original.DeepCopy()
	
	// Modify the copy
	copy.Items[0].Name = "modified-config1"
	copy.Items[0].Spec.FeatureFlags["newFlag"] = true

	// Original should remain unchanged
	if original.Items[0].Name != "config1" {
		t.Errorf("Original item name should be unchanged, got: %s", original.Items[0].Name)
	}
	
	if original.Items[0].Spec.FeatureFlags["newFlag"] {
		t.Errorf("Original should not have newFlag")
	}

	// Copy should have the modifications
	if copy.Items[0].Name != "modified-config1" {
		t.Errorf("Copy item name should be modified, got: %s", copy.Items[0].Name)
	}
	
	if !copy.Items[0].Spec.FeatureFlags["newFlag"] {
		t.Errorf("Copy should have newFlag set to true")
	}
}

// Test concurrent access to TMCConfig to ensure thread safety
func TestTMCConfig_ConcurrentAccess(t *testing.T) {
	config := &tmcv1alpha1.TMCConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "concurrent-test",
		},
		Spec: tmcv1alpha1.TMCConfigSpec{
			FeatureFlags: map[string]bool{
				"feature1": true,
			},
		},
	}

	const numGoroutines = 10
	const numOperations = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Test concurrent deep copy operations
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				copy := config.DeepCopy()
				if copy.Name != "concurrent-test" {
					t.Errorf("Goroutine %d: Expected name 'concurrent-test', got %s", id, copy.Name)
				}
				if !copy.Spec.FeatureFlags["feature1"] {
					t.Errorf("Goroutine %d: Expected feature1 to be true", id)
				}
			}
		}(i)
	}

	wg.Wait()
}

// Test TMCConfig validation with edge cases
func TestTMCConfig_ValidationEdgeCases(t *testing.T) {
	// Test many feature flags
	manyFlagsConfig := &tmcv1alpha1.TMCConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "test-config"},
		Spec: tmcv1alpha1.TMCConfigSpec{
			FeatureFlags: func() map[string]bool {
				flags := make(map[string]bool)
				for i := 0; i < 50; i++ {
					flags["flag"+string(rune('A'+i%26))] = i%2 == 0
				}
				return flags
			}(),
		},
	}
	
	errs := tmcv1alpha1.ValidateTMCConfig(manyFlagsConfig)
	if len(errs) > 0 {
		t.Errorf("Config with many flags should be valid, got errors: %v", errs)
	}
}