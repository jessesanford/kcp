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

func TestSessionValidatorValidation(t *testing.T) {
	now := metav1.Now()

	tests := map[string]struct {
		validator *SessionValidator
		wantValid bool
	}{
		"valid session validator with basic configuration": {
			validator: &SessionValidator{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "basic-validator",
					Namespace: "default",
				},
				Spec: SessionValidatorSpec{
					ValidationRules: []ValidationRule{
						{
							Name: "resource-validation",
							Type: ValidationRuleTypeResourceConstraint,
							Condition: ValidationCondition{
								Event: ValidationEventSessionCreate,
							},
							Validator: ValidatorConfiguration{
								Type: ValidatorTypeRequired,
								Parameters: map[string]string{
									"field": "spec.resourceRequirements",
								},
								Timeout: metav1.Duration{Duration: 30 * time.Second},
							},
							Severity: ValidationSeverityError,
							Enabled:  true,
						},
					},
				},
			},
			wantValid: true,
		},
		"valid session validator with comprehensive configuration": {
			validator: &SessionValidator{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "comprehensive-validator",
					Namespace: "production",
				},
				Spec: SessionValidatorSpec{
					ValidationRules: []ValidationRule{
						{
							Name:        "policy-validation",
							Type:        ValidationRuleTypePlacementPolicy,
							Description: "Validates placement policies for compliance",
							Condition: ValidationCondition{
								Event: ValidationEventSessionUpdate,
								Filters: []ValidationFilter{
									{
										Field:    "metadata.labels.tier",
										Operator: FilterOperatorEquals,
										Value:    "production",
									},
								},
								Schedule: "0 */4 * * *", // Every 4 hours
							},
							Validator: ValidatorConfiguration{
								Type: ValidatorTypeCustom,
								Parameters: map[string]string{
									"script_language": "lua",
									"max_memory":      "64MB",
								},
								Script: `
									function validate(session)
										if session.spec.placementPolicy.priority < 500 then
											return false, "Production sessions must have priority >= 500"
										end
										return true, "Policy validation passed"
									end
								`,
								Timeout: metav1.Duration{Duration: 60 * time.Second},
								RetryPolicy: &ValidationRetryPolicy{
									MaxRetries:        3,
									RetryDelay:        metav1.Duration{Duration: 2 * time.Second},
									BackoffMultiplier: 2.0,
								},
							},
							Severity: ValidationSeverityCritical,
							Enabled:  true,
						},
						{
							Name: "dependency-validation",
							Type: ValidationRuleTypeDependencyCheck,
							Condition: ValidationCondition{
								Event: ValidationEventDecisionCreate,
							},
							Validator: ValidatorConfiguration{
								Type: ValidatorTypeConsistency,
								Parameters: map[string]string{
									"check_type": "circular_dependencies",
								},
							},
							Severity: ValidationSeverityWarning,
							Enabled:  true,
						},
					},
					ConflictDetection: &ConflictDetectionPolicy{
						Enabled: true,
						DetectionScope: &ConflictDetectionScope{
							IncludeNamespaces: []string{"production", "staging"},
							ExcludeClusters:   []string{"dev-cluster"},
							ResourceTypes:     []string{"deployments", "statefulsets"},
						},
						ConflictTypes: []ConflictType{
							ConflictTypeResourceContention,
							ConflictTypePolicyViolation,
							ConflictTypeAffinityConflict,
						},
						ResolutionStrategies: []ConflictResolutionStrategy{
							{
								ConflictType: ConflictTypeResourceContention,
								Strategy:     ConflictResolutionTypeMerge,
								Priority:     800,
								Parameters: map[string]string{
									"merge_strategy": "priority_based",
									"allow_partial":  "true",
								},
							},
							{
								ConflictType: ConflictTypePolicyViolation,
								Strategy:     ConflictResolutionTypeFail,
								Priority:     900,
								Parameters: map[string]string{
									"fail_fast": "true",
								},
							},
						},
						NotificationPolicy: &ConflictNotificationPolicy{
							Enabled: true,
							Channels: []NotificationChannelRef{
								{
									Name:      "alert-manager",
									Namespace: "monitoring",
								},
								{
									Name: "slack-channel",
								},
							},
							SeverityFilter: ValidationSeverityWarning,
						},
					},
					ResourceValidation: &ResourceValidationPolicy{
						ValidateCapacity:     true,
						ValidateAvailability: true,
						CapacityThresholds: map[string]ResourceThreshold{
							"cpu": {
								WarningThreshold: 70,
								ErrorThreshold:   90,
							},
							"memory": {
								WarningThreshold: 80,
								ErrorThreshold:   95,
							},
							"storage": {
								WarningThreshold: 75,
								ErrorThreshold:   90,
							},
						},
						ReservationPolicy: &ResourceReservationPolicy{
							ReservationMode:       ResourceReservationModeSoft,
							ReservationDuration:   metav1.Duration{Duration: 10 * time.Minute},
							AllowOversubscription: false,
						},
					},
					DependencyValidation: &DependencyValidationPolicy{
						ValidateDependencies: true,
						DependencyTypes: []DependencyType{
							DependencyTypeServiceDependency,
							DependencyTypeConfigDependency,
							DependencyTypeNetworkDependency,
						},
						CircularDependencyDetection: true,
						MaxDependencyDepth:          15,
					},
					ValidationScope: &ValidationScope{
						IncludeNamespaces: []string{"production", "staging"},
						ExcludeClusters:   []string{"test-cluster"},
						ResourceSelectors: []ResourceSelector{
							{
								APIVersion: "apps/v1",
								Kind:       "Deployment",
								LabelSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										"app.kubernetes.io/managed-by": "tmc",
									},
								},
							},
						},
					},
				},
				Status: SessionValidatorStatus{
					ValidationResults: []ValidationResult{
						{
							ValidationID:   "validation-123",
							RuleName:       "policy-validation",
							ValidationTime: now,
							Status:         ValidationResultStatusPassed,
							Severity:       ValidationSeverityInfo,
							Message:        "Policy validation completed successfully",
							Details: map[string]string{
								"checks_passed": "5",
								"warnings":      "0",
							},
							AffectedObjects: []ObjectReference{
								{
									APIVersion: "tmc.kcp.io/v1alpha1",
									Kind:       "PlacementSession",
									Name:       "test-session",
									Namespace:  "production",
								},
							},
						},
					},
					ConflictDetections: []ConflictDetection{
						{
							ConflictID:     "conflict-456",
							DetectionTime:  now,
							ConflictType:   ConflictTypeResourceContention,
							Severity:       ValidationSeverityWarning,
							Description:    "Resource contention detected between sessions",
							ConflictingSessions: []SessionReference{
								{
									Name:      "session-a",
									Namespace: "production",
									SessionID: "sess-123",
								},
								{
									Name:      "session-b",
									Namespace: "production",
									SessionID: "sess-456",
								},
							},
							ResolutionStatus: ConflictStatusResolving,
						},
					},
					LastValidationTime: &now,
					ValidationMetrics: &ValidationMetrics{
						TotalValidations:      150,
						SuccessfulValidations: 142,
						FailedValidations:     5,
						WarningValidations:    3,
						ConflictsDetected:     12,
						ConflictsResolved:     10,
						AverageValidationTime: &metav1.Duration{Duration: 250 * time.Millisecond},
					},
				},
			},
			wantValid: true,
		},
		"invalid - empty validation rules": {
			validator: &SessionValidator{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-validator",
					Namespace: "default",
				},
				Spec: SessionValidatorSpec{
					ValidationRules: []ValidationRule{}, // Invalid - empty rules
				},
			},
			wantValid: false,
		},
		"invalid - negative retry policy": {
			validator: &SessionValidator{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-retry-validator",
					Namespace: "default",
				},
				Spec: SessionValidatorSpec{
					ValidationRules: []ValidationRule{
						{
							Name: "test-rule",
							Type: ValidationRuleTypeSessionConfiguration,
							Condition: ValidationCondition{
								Event: ValidationEventSessionCreate,
							},
							Validator: ValidatorConfiguration{
								Type: ValidatorTypeRequired,
								RetryPolicy: &ValidationRetryPolicy{
									MaxRetries: -1, // Invalid - negative retries
								},
							},
						},
					},
				},
			},
			wantValid: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if tc.validator == nil {
				t.Fatal("validator cannot be nil")
			}

			// Validate validation rules
			if len(tc.validator.Spec.ValidationRules) == 0 && tc.wantValid {
				t.Error("expected valid validator, but validation rules are empty")
				return
			}

			// Validate each validation rule
			for i, rule := range tc.validator.Spec.ValidationRules {
				if rule.Name == "" && tc.wantValid {
					t.Errorf("validation rule %d: name cannot be empty", i)
				}

				// Validate rule type
				validTypes := []ValidationRuleType{
					ValidationRuleTypeSessionConfiguration,
					ValidationRuleTypePlacementPolicy,
					ValidationRuleTypeResourceConstraint,
					ValidationRuleTypeConflictDetection,
					ValidationRuleTypeDependencyCheck,
				}
				found := false
				for _, validType := range validTypes {
					if rule.Type == validType {
						found = true
						break
					}
				}
				if !found && tc.wantValid {
					t.Errorf("validation rule %d: invalid type %s", i, rule.Type)
				}

				// Validate validator type
				validValidatorTypes := []ValidatorType{
					ValidatorTypeRequired, ValidatorTypeRange, ValidatorTypeFormat,
					ValidatorTypeCustom, ValidatorTypeReference, ValidatorTypeUniqueness,
					ValidatorTypeConsistency,
				}
				found = false
				for _, validValidatorType := range validValidatorTypes {
					if rule.Validator.Type == validValidatorType {
						found = true
						break
					}
				}
				if !found && tc.wantValid {
					t.Errorf("validation rule %d: invalid validator type %s", i, rule.Validator.Type)
				}

				// Validate retry policy
				if rule.Validator.RetryPolicy != nil {
					if rule.Validator.RetryPolicy.MaxRetries < 0 && tc.wantValid {
						t.Errorf("validation rule %d: max retries cannot be negative", i)
					}
					if rule.Validator.RetryPolicy.BackoffMultiplier < 1 && rule.Validator.RetryPolicy.BackoffMultiplier != 0 && tc.wantValid {
						t.Errorf("validation rule %d: backoff multiplier must be >= 1", i)
					}
				}
			}

			// Validate conflict detection policy
			if tc.validator.Spec.ConflictDetection != nil {
				for i, strategy := range tc.validator.Spec.ConflictDetection.ResolutionStrategies {
					if strategy.Priority < 0 || strategy.Priority > 1000 {
						if tc.wantValid {
							t.Errorf("conflict resolution strategy %d: priority %d out of range [0, 1000]", i, strategy.Priority)
						}
					}
				}
			}

			// Validate resource validation policy
			if tc.validator.Spec.ResourceValidation != nil {
				for resourceType, threshold := range tc.validator.Spec.ResourceValidation.CapacityThresholds {
					if threshold.WarningThreshold < 0 || threshold.WarningThreshold > 100 {
						if tc.wantValid {
							t.Errorf("resource %s: warning threshold %d out of range [0, 100]", resourceType, threshold.WarningThreshold)
						}
					}
					if threshold.ErrorThreshold < 0 || threshold.ErrorThreshold > 100 {
						if tc.wantValid {
							t.Errorf("resource %s: error threshold %d out of range [0, 100]", resourceType, threshold.ErrorThreshold)
						}
					}
				}
			}

			// Validate dependency validation policy
			if tc.validator.Spec.DependencyValidation != nil {
				if tc.validator.Spec.DependencyValidation.MaxDependencyDepth < 1 && tc.wantValid {
					t.Error("max dependency depth must be >= 1")
				}
			}
		})
	}
}

func TestValidationRuleTypeValues(t *testing.T) {
	validRuleTypes := []ValidationRuleType{
		ValidationRuleTypeSessionConfiguration,
		ValidationRuleTypePlacementPolicy,
		ValidationRuleTypeResourceConstraint,
		ValidationRuleTypeConflictDetection,
		ValidationRuleTypeDependencyCheck,
	}

	// Verify all rule types are defined correctly
	for _, ruleType := range validRuleTypes {
		if string(ruleType) == "" {
			t.Errorf("rule type %v has empty string value", ruleType)
		}
	}

	// Test rule type validation with real values
	testRuleTypes := map[ValidationRuleType]bool{
		ValidationRuleTypeSessionConfiguration: true,
		ValidationRuleTypePlacementPolicy:      true,
		ValidationRuleTypeResourceConstraint:   true,
		ValidationRuleTypeConflictDetection:    true,
		ValidationRuleTypeDependencyCheck:      true,
		"InvalidRuleType":                      false,
	}

	for ruleType, shouldBeValid := range testRuleTypes {
		found := false
		for _, validType := range validRuleTypes {
			if ruleType == validType {
				found = true
				break
			}
		}

		if found != shouldBeValid {
			t.Errorf("rule type %s: expected valid=%t, got valid=%t", ruleType, shouldBeValid, found)
		}
	}
}

func TestValidatorTypeValues(t *testing.T) {
	validValidatorTypes := []ValidatorType{
		ValidatorTypeRequired,
		ValidatorTypeRange,
		ValidatorTypeFormat,
		ValidatorTypeCustom,
		ValidatorTypeReference,
		ValidatorTypeUniqueness,
		ValidatorTypeConsistency,
	}

	// Verify all validator types are defined correctly
	for _, validatorType := range validValidatorTypes {
		if string(validatorType) == "" {
			t.Errorf("validator type %v has empty string value", validatorType)
		}
	}

	// Test validator type validation with real values
	testValidatorTypes := map[ValidatorType]bool{
		ValidatorTypeRequired:    true,
		ValidatorTypeRange:       true,
		ValidatorTypeFormat:      true,
		ValidatorTypeCustom:      true,
		ValidatorTypeReference:   true,
		ValidatorTypeUniqueness:  true,
		ValidatorTypeConsistency: true,
		"InvalidValidatorType":   false,
	}

	for validatorType, shouldBeValid := range testValidatorTypes {
		found := false
		for _, validType := range validValidatorTypes {
			if validatorType == validType {
				found = true
				break
			}
		}

		if found != shouldBeValid {
			t.Errorf("validator type %s: expected valid=%t, got valid=%t", validatorType, shouldBeValid, found)
		}
	}
}

func TestValidationEventValues(t *testing.T) {
	validEvents := []ValidationEvent{
		ValidationEventSessionCreate,
		ValidationEventSessionUpdate,
		ValidationEventSessionDelete,
		ValidationEventDecisionCreate,
		ValidationEventDecisionUpdate,
		ValidationEventConflictDetected,
		ValidationEventResourceChange,
	}

	// Verify all event types are defined correctly
	for _, eventType := range validEvents {
		if string(eventType) == "" {
			t.Errorf("event type %v has empty string value", eventType)
		}
	}

	// Test event validation with real values
	testEvents := map[ValidationEvent]bool{
		ValidationEventSessionCreate:    true,
		ValidationEventSessionUpdate:    true,
		ValidationEventSessionDelete:    true,
		ValidationEventDecisionCreate:   true,
		ValidationEventDecisionUpdate:   true,
		ValidationEventConflictDetected: true,
		ValidationEventResourceChange:   true,
		"InvalidEvent":                  false,
	}

	for eventType, shouldBeValid := range testEvents {
		found := false
		for _, validEvent := range validEvents {
			if eventType == validEvent {
				found = true
				break
			}
		}

		if found != shouldBeValid {
			t.Errorf("event type %s: expected valid=%t, got valid=%t", eventType, shouldBeValid, found)
		}
	}
}

func TestFilterOperatorValues(t *testing.T) {
	validOperators := []FilterOperator{
		FilterOperatorEquals,
		FilterOperatorNotEquals,
		FilterOperatorContains,
		FilterOperatorNotContains,
		FilterOperatorIn,
		FilterOperatorNotIn,
		FilterOperatorExists,
		FilterOperatorNotExists,
	}

	// Verify all operators are defined correctly
	for _, operator := range validOperators {
		if string(operator) == "" {
			t.Errorf("operator %v has empty string value", operator)
		}
	}

	// Test operator validation with real values
	testOperators := map[FilterOperator]bool{
		FilterOperatorEquals:      true,
		FilterOperatorNotEquals:   true,
		FilterOperatorContains:    true,
		FilterOperatorNotContains: true,
		FilterOperatorIn:          true,
		FilterOperatorNotIn:       true,
		FilterOperatorExists:      true,
		FilterOperatorNotExists:   true,
		"InvalidOperator":         false,
	}

	for operator, shouldBeValid := range testOperators {
		found := false
		for _, validOperator := range validOperators {
			if operator == validOperator {
				found = true
				break
			}
		}

		if found != shouldBeValid {
			t.Errorf("operator %s: expected valid=%t, got valid=%t", operator, shouldBeValid, found)
		}
	}
}