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

package validation

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func TestNewConfigurationValidator(t *testing.T) {
	tests := map[string]struct {
		options         []ValidatorOption
		expectedMaxReplicas int32
		expectedMaxClusters int32
	}{
		"default configuration": {
			options:             []ValidatorOption{},
			expectedMaxReplicas: 10000,
			expectedMaxClusters: 100,
		},
		"custom max replicas": {
			options:             []ValidatorOption{WithMaxReplicas(5000)},
			expectedMaxReplicas: 5000,
			expectedMaxClusters: 100,
		},
		"multiple options": {
			options:             []ValidatorOption{WithMaxReplicas(8000), WithMaxClusters(75)},
			expectedMaxReplicas: 8000,
			expectedMaxClusters: 75,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			validator := NewConfigurationValidator(tc.options...)

			if validator.maxReplicas != tc.expectedMaxReplicas {
				t.Errorf("expected maxReplicas %d, got %d", tc.expectedMaxReplicas, validator.maxReplicas)
			}

			if validator.maxClusters != tc.expectedMaxClusters {
				t.Errorf("expected maxClusters %d, got %d", tc.expectedMaxClusters, validator.maxClusters)
			}
		})
	}
}

func TestValidateWorkloadScalingPolicy(t *testing.T) {
	validator := NewConfigurationValidator()
	ctx := &ValidationContext{
		FieldPath:           field.NewPath("spec"),
		WorkloadNamespace:   "test-namespace",
		AllowCrossNamespace: false,
	}

	tests := map[string]struct {
		spec           *WorkloadScalingPolicySpec
		expectedValid  bool
		expectedErrors int
	}{
		"valid complete policy": {
			spec: &WorkloadScalingPolicySpec{
				MinReplicas: 1,
				MaxReplicas: 10,
				WorkloadSelector: WorkloadSelector{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "test"},
					},
				},
				ClusterSelector: ClusterSelector{
					ClusterNames: []string{"cluster-1", "cluster-2"},
				},
				ScalingMetrics: []ScalingMetric{
					{
						Type:        CPUUtilizationMetric,
						TargetValue: intstr.FromString("80%"),
					},
				},
			},
			expectedValid:  true,
			expectedErrors: 0,
		},
		"nil spec": {
			spec:            nil,
			expectedValid:   false,
			expectedErrors:  1,
		},
		"invalid replica constraints": {
			spec: &WorkloadScalingPolicySpec{
				MinReplicas: -1,
				MaxReplicas: 0,
				WorkloadSelector: WorkloadSelector{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "test"},
					},
				},
				ClusterSelector: ClusterSelector{
					ClusterNames: []string{"cluster-1"},
				},
				ScalingMetrics: []ScalingMetric{
					{
						Type:        CPUUtilizationMetric,
						TargetValue: intstr.FromString("80%"),
					},
				},
			},
			expectedValid:   false,
			expectedErrors:  2,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result := validator.ValidateWorkloadScalingPolicy(tc.spec, ctx)

			if result.Valid != tc.expectedValid {
				t.Errorf("expected valid %v, got %v", tc.expectedValid, result.Valid)
			}

			if len(result.Errors) != tc.expectedErrors {
				t.Errorf("expected %d errors, got %d: %v", tc.expectedErrors, len(result.Errors), result.Errors)
			}
		})
	}
}

func TestUtilityValidationFunctions(t *testing.T) {
	validator := NewConfigurationValidator()

	t.Run("isValidAPIVersion", func(t *testing.T) {
		tests := map[string]struct {
			apiVersion string
			expected   bool
		}{
			"valid core API":         {"v1", true},
			"valid group API":        {"apps/v1", true},
			"empty API version":      {"", false},
		}

		for name, tc := range tests {
			t.Run(name, func(t *testing.T) {
				result := validator.isValidAPIVersion(tc.apiVersion)
				if result != tc.expected {
					t.Errorf("isValidAPIVersion(%q) = %v, expected %v", tc.apiVersion, result, tc.expected)
				}
			})
		}
	})

	t.Run("isValidKind", func(t *testing.T) {
		tests := map[string]struct {
			kind     string
			expected bool
		}{
			"valid kind":           {"Deployment", true},
			"valid single letter":  {"A", true},
			"empty kind":           {"", false},
			"lowercase start":      {"deployment", false},
		}

		for name, tc := range tests {
			t.Run(name, func(t *testing.T) {
				result := validator.isValidKind(tc.kind)
				if result != tc.expected {
					t.Errorf("isValidKind(%q) = %v, expected %v", tc.kind, result, tc.expected)
				}
			})
		}
	})
}

func TestValidationResult(t *testing.T) {
	t.Run("ExtractErrorMessages", func(t *testing.T) {
		result := &ScalingPolicyValidationResult{
			Errors: field.ErrorList{
				field.Required(field.NewPath("test"), "test error 1"),
				field.Invalid(field.NewPath("test2"), "value", "test error 2"),
			},
		}

		messages := result.ExtractErrorMessages()
		if len(messages) != 2 {
			t.Errorf("expected 2 messages, got %d", len(messages))
		}
	})

	t.Run("HasCriticalErrors", func(t *testing.T) {
		tests := map[string]struct {
			result   *ScalingPolicyValidationResult
			expected bool
		}{
			"no errors": {
				result:   &ScalingPolicyValidationResult{Valid: true, Errors: field.ErrorList{}},
				expected: false,
			},
			"has errors": {
				result: &ScalingPolicyValidationResult{
					Valid:  false,
					Errors: field.ErrorList{field.Required(field.NewPath("test"), "error")},
				},
				expected: true,
			},
		}

		for name, tc := range tests {
			t.Run(name, func(t *testing.T) {
				result := tc.result.HasCriticalErrors()
				if result != tc.expected {
					t.Errorf("HasCriticalErrors() = %v, expected %v", result, tc.expected)
				}
			})
		}
	})
}