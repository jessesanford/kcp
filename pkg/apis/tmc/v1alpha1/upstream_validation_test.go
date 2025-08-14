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
)

func TestValidateUpstreamSyncConfig(t *testing.T) {
	tests := map[string]struct {
		config      *UpstreamSyncConfig
		wantErrors  int
		errorFields []string
	}{
		"valid config": {
			config: &UpstreamSyncConfig{
				Spec: UpstreamSyncSpec{
					SyncTargets: []SyncTargetReference{
						{Name: "cluster1"},
					},
					ResourceSelectors: []ResourceSelector{
						{APIGroup: "apps", Resource: "deployments"},
					},
					SyncInterval:     metav1.Duration{Duration: 30 * time.Second},
					ConflictStrategy: ConflictStrategyUseNewest,
				},
			},
			wantErrors: 0,
		},
		"missing sync targets": {
			config: &UpstreamSyncConfig{
				Spec: UpstreamSyncSpec{
					SyncTargets: []SyncTargetReference{},
					ResourceSelectors: []ResourceSelector{
						{APIGroup: "apps", Resource: "deployments"},
					},
					SyncInterval:     metav1.Duration{Duration: 30 * time.Second},
					ConflictStrategy: ConflictStrategyUseNewest,
				},
			},
			wantErrors:  1,
			errorFields: []string{"spec.syncTargets"},
		},
		"missing resource selectors": {
			config: &UpstreamSyncConfig{
				Spec: UpstreamSyncSpec{
					SyncTargets: []SyncTargetReference{
						{Name: "cluster1"},
					},
					ResourceSelectors: []ResourceSelector{},
					SyncInterval:      metav1.Duration{Duration: 30 * time.Second},
					ConflictStrategy:  ConflictStrategyUseNewest,
				},
			},
			wantErrors:  1,
			errorFields: []string{"spec.resourceSelectors"},
		},
		"sync interval too short": {
			config: &UpstreamSyncConfig{
				Spec: UpstreamSyncSpec{
					SyncTargets: []SyncTargetReference{
						{Name: "cluster1"},
					},
					ResourceSelectors: []ResourceSelector{
						{APIGroup: "apps", Resource: "deployments"},
					},
					SyncInterval:     metav1.Duration{Duration: 5 * time.Second},
					ConflictStrategy: ConflictStrategyUseNewest,
				},
			},
			wantErrors:  1,
			errorFields: []string{"spec.syncInterval"},
		},
		"invalid conflict strategy": {
			config: &UpstreamSyncConfig{
				Spec: UpstreamSyncSpec{
					SyncTargets: []SyncTargetReference{
						{Name: "cluster1"},
					},
					ResourceSelectors: []ResourceSelector{
						{APIGroup: "apps", Resource: "deployments"},
					},
					SyncInterval:     metav1.Duration{Duration: 30 * time.Second},
					ConflictStrategy: "InvalidStrategy",
				},
			},
			wantErrors:  1,
			errorFields: []string{"spec.conflictStrategy"},
		},
		"multiple validation errors": {
			config: &UpstreamSyncConfig{
				Spec: UpstreamSyncSpec{
					SyncTargets:       []SyncTargetReference{},
					ResourceSelectors: []ResourceSelector{},
					SyncInterval:      metav1.Duration{Duration: 1 * time.Second},
					ConflictStrategy:  "BadStrategy",
				},
			},
			wantErrors:  4,
			errorFields: []string{"spec.syncTargets", "spec.resourceSelectors", "spec.syncInterval", "spec.conflictStrategy"},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			errs := ValidateUpstreamSyncConfig(tc.config)
			
			if len(errs) != tc.wantErrors {
				t.Errorf("expected %d errors, got %d: %v", tc.wantErrors, len(errs), errs)
			}

			if tc.wantErrors > 0 {
				errorFieldMap := make(map[string]bool)
				for _, err := range errs {
					errorFieldMap[err.Field] = true
				}

				for _, expectedField := range tc.errorFields {
					if !errorFieldMap[expectedField] {
						t.Errorf("expected error for field %s, but not found in: %v", expectedField, errs)
					}
				}
			}
		})
	}
}

func TestSetDefaults_UpstreamSyncConfig(t *testing.T) {
	tests := map[string]struct {
		config   *UpstreamSyncConfig
		validate func(*testing.T, *UpstreamSyncConfig)
	}{
		"sets default sync interval": {
			config: &UpstreamSyncConfig{
				Spec: UpstreamSyncSpec{},
			},
			validate: func(t *testing.T, config *UpstreamSyncConfig) {
				expectedInterval := 30 * time.Second
				if config.Spec.SyncInterval.Duration != expectedInterval {
					t.Errorf("expected sync interval %v, got %v", expectedInterval, config.Spec.SyncInterval.Duration)
				}
			},
		},
		"sets default conflict strategy": {
			config: &UpstreamSyncConfig{
				Spec: UpstreamSyncSpec{},
			},
			validate: func(t *testing.T, config *UpstreamSyncConfig) {
				if config.Spec.ConflictStrategy != ConflictStrategyUseNewest {
					t.Errorf("expected conflict strategy %s, got %s", ConflictStrategyUseNewest, config.Spec.ConflictStrategy)
				}
			},
		},
		"initializes conditions": {
			config: &UpstreamSyncConfig{
				Status: UpstreamSyncStatus{},
			},
			validate: func(t *testing.T, config *UpstreamSyncConfig) {
				if config.Status.Conditions == nil {
					t.Error("expected conditions to be initialized")
				}
				if len(config.Status.Conditions) != 0 {
					t.Errorf("expected empty conditions slice, got %d conditions", len(config.Status.Conditions))
				}
			},
		},
		"preserves existing values": {
			config: &UpstreamSyncConfig{
				Spec: UpstreamSyncSpec{
					SyncInterval:     metav1.Duration{Duration: 60 * time.Second},
					ConflictStrategy: ConflictStrategyManual,
				},
				Status: UpstreamSyncStatus{
					Conditions: []metav1.Condition{
						{Type: "Ready", Status: metav1.ConditionTrue},
					},
				},
			},
			validate: func(t *testing.T, config *UpstreamSyncConfig) {
				if config.Spec.SyncInterval.Duration != 60*time.Second {
					t.Errorf("expected existing sync interval to be preserved")
				}
				if config.Spec.ConflictStrategy != ConflictStrategyManual {
					t.Errorf("expected existing conflict strategy to be preserved")
				}
				if len(config.Status.Conditions) != 1 {
					t.Errorf("expected existing conditions to be preserved")
				}
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			SetDefaults_UpstreamSyncConfig(tc.config)
			tc.validate(t, tc.config)
		})
	}
}