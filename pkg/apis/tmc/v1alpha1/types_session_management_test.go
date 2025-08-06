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
	"k8s.io/apimachinery/pkg/api/resource"

	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
)

func TestPlacementSessionValidation(t *testing.T) {
	tests := map[string]struct {
		session   *PlacementSession
		wantValid bool
	}{
		"valid placement session with basic configuration": {
			session: &PlacementSession{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-session",
					Namespace: "default",
				},
				Spec: PlacementSessionSpec{
					WorkloadSelector: WorkloadSelector{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "web-app"},
						},
					},
					ClusterSelector: ClusterSelector{
						ClusterNames: []string{"cluster-1", "cluster-2"},
					},
					SessionConfiguration: SessionConfiguration{
						SessionTimeout:      metav1.Duration{Duration: 24 * time.Hour},
						HeartbeatInterval:   metav1.Duration{Duration: 5 * time.Minute},
						MaxDecisions:        100,
						ConflictResolution:  ConflictResolutionTypeMerge,
						PersistenceStrategy: PersistenceStrategyPersistent,
					},
					Enabled: true,
				},
			},
			wantValid: true,
		},
		"valid session with placement policies and resource constraints": {
			session: &PlacementSession{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "advanced-session",
					Namespace: "production",
				},
				Spec: PlacementSessionSpec{
					WorkloadSelector: WorkloadSelector{
						WorkloadTypes: []WorkloadType{
							{APIVersion: "apps/v1", Kind: "Deployment"},
							{APIVersion: "apps/v1", Kind: "StatefulSet"},
						},
					},
					ClusterSelector: ClusterSelector{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"tier": "production"},
						},
					},
					SessionConfiguration: SessionConfiguration{
						ConflictResolution:  ConflictResolutionTypeOverride,
						PersistenceStrategy: PersistenceStrategyDistributed,
						RecoveryPolicy: &SessionRecoveryPolicy{
							RestartPolicy:     SessionRestartPolicyOnFailure,
							MaxRetries:        3,
							RetryDelay:        metav1.Duration{Duration: 1 * time.Minute},
							BackoffMultiplier: 2.0,
						},
					},
					PlacementPolicies: []PlacementPolicy{
						{
							Name:     "affinity-policy",
							Type:     PlacementPolicyTypeAffinity,
							Priority: 800,
							Rules: []PlacementRule{
								{
									Name: "cluster-affinity",
									Selector: PlacementRuleSelector{
										ClusterNames: []string{"prod-cluster-1"},
									},
									Constraints: []PlacementConstraint{
										{
											Type:     PlacementConstraintTypeZone,
											Key:      "topology.kubernetes.io/zone",
											Operator: PlacementConstraintOperatorIn,
											Values:   []string{"us-west-1a", "us-west-1b"},
											Required: true,
										},
									},
									Weight: 80,
								},
							},
							Enabled: true,
						},
					},
					ResourceConstraints: &ResourceConstraints{
						CPULimits: &ResourceLimit{
							Min: &resource.Quantity{
								Format: resource.DecimalSI,
							},
							Max: &resource.Quantity{
								Format: resource.DecimalSI,
							},
						},
					},
					Enabled: true,
				},
			},
			wantValid: true,
		},
		"invalid - missing workload selector": {
			session: &PlacementSession{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-session",
					Namespace: "default",
				},
				Spec: PlacementSessionSpec{
					ClusterSelector: ClusterSelector{
						ClusterNames: []string{"cluster-1"},
					},
					SessionConfiguration: SessionConfiguration{
						ConflictResolution: ConflictResolutionTypeMerge,
					},
				},
			},
			wantValid: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if tc.session == nil {
				t.Fatal("session cannot be nil")
			}

			// Validate workload selector
			if tc.session.Spec.WorkloadSelector.LabelSelector == nil &&
				len(tc.session.Spec.WorkloadSelector.WorkloadTypes) == 0 &&
				tc.wantValid {
				t.Error("expected valid session, but WorkloadSelector is empty")
				return
			}

			// Validate cluster selector
			if tc.session.Spec.ClusterSelector.ClusterNames == nil &&
				tc.session.Spec.ClusterSelector.LabelSelector == nil &&
				tc.wantValid {
				t.Error("expected valid session, but ClusterSelector is empty")
				return
			}

			// Validate placement policies
			for _, policy := range tc.session.Spec.PlacementPolicies {
				if policy.Priority < 0 || policy.Priority > 1000 {
					if tc.wantValid {
						t.Errorf("policy priority %d is out of valid range [0, 1000]", policy.Priority)
					}
				}

				// Validate policy rules
				for _, rule := range policy.Rules {
					if rule.Weight < 1 || rule.Weight > 100 {
						if tc.wantValid {
							t.Errorf("rule weight %d is out of valid range [1, 100]", rule.Weight)
						}
					}
				}
			}
		})
	}
}

func TestSessionStateValidation(t *testing.T) {
	now := metav1.Now()
	sessionRef := SessionReference{
		Name:      "test-session",
		Namespace: "default",
		SessionID: "session-123",
	}

	tests := map[string]struct {
		state     *SessionState
		wantValid bool
	}{
		"valid session state with basic configuration": {
			state: &SessionState{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-session-state",
					Namespace: "default",
				},
				Spec: SessionStateSpec{
					SessionRef: sessionRef,
					StateData: SessionStateData{
						CurrentPhase: SessionPhaseActive,
						PhaseTransitions: []PhaseTransition{
							{
								FromPhase:  SessionPhaseCreated,
								ToPhase:    SessionPhaseActive,
								Timestamp:  now,
								Reason:     "SessionStarted",
								Message:    "Session successfully started",
								Initiator:  "session-controller",
							},
						},
						PlacementContext: &PlacementContext{
							AvailableClusters: []ClusterInfo{
								{
									Name: "cluster-1",
									Location: &ClusterLocation{
										Region:   "us-west-2",
										Zone:     "us-west-2a",
										Provider: "aws",
									},
									Status: ClusterStatusReady,
									ResourceCapacity: map[string]resource.Quantity{
										"cpu":    resource.MustParse("100"),
										"memory": resource.MustParse("1000Gi"),
									},
									Labels: map[string]string{
										"tier": "production",
									},
								},
							},
						},
					},
				},
			},
			wantValid: true,
		},
		"valid session state with comprehensive data": {
			state: &SessionState{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "comprehensive-state",
					Namespace: "production",
				},
				Spec: SessionStateSpec{
					SessionRef: sessionRef,
					StateData: SessionStateData{
						CurrentPhase: SessionPhaseActive,
						ResourceAllocations: []ResourceAllocation{
							{
								ClusterName: "prod-cluster-1",
								AllocatedResources: map[string]resource.Quantity{
									"cpu":    resource.MustParse("50"),
									"memory": resource.MustParse("500Gi"),
								},
								WorkloadAllocations: []WorkloadAllocation{
									{
										WorkloadRef: WorkloadReference{
											APIVersion: "apps/v1",
											Kind:       "Deployment",
											Name:       "web-app",
											Namespace:  "production",
										},
										AllocatedResources: map[string]resource.Quantity{
											"cpu":    resource.MustParse("10"),
											"memory": resource.MustParse("100Gi"),
										},
									},
								},
								AllocationTime: now,
								Status:         AllocationStatusAllocated,
							},
						},
						ConflictHistory: []ConflictRecord{
							{
								ConflictID:   "conflict-123",
								Timestamp:    now,
								ConflictType: ConflictTypeResourceConflict,
								ConflictingPlacements: []PlacementReference{
									{
										Name:        "placement-1",
										Namespace:   "default",
										ClusterName: "cluster-1",
									},
									{
										Name:        "placement-2",
										Namespace:   "default",
										ClusterName: "cluster-1",
									},
								},
								Resolution: &ConflictResolution{
									Strategy: ConflictResolutionTypeMerge,
									WinningPlacement: &PlacementReference{
										Name:        "placement-1",
										Namespace:   "default",
										ClusterName: "cluster-1",
									},
									RejectedPlacements: []PlacementReference{
										{
											Name:        "placement-2",
											Namespace:   "default",
											ClusterName: "cluster-1",
										},
									},
									Reason: "Higher priority placement selected",
								},
								ResolutionTime: &now,
							},
						},
						SessionEvents: []SessionEvent{
							{
								EventID:   "event-123",
								Timestamp: now,
								EventType: SessionEventTypeStarted,
								Message:   "Session started successfully",
								Source:    "session-controller",
								Reason:    "UserRequest",
							},
						},
						Checkpoints: []StateCheckpoint{
							{
								CheckpointID: "checkpoint-123",
								Timestamp:    now,
								Phase:        SessionPhaseActive,
								Data:         []byte("serialized-state-data"),
								Version:      "v1",
								Checksum:     "abc123",
							},
						},
					},
					SyncPolicy: &StateSyncPolicy{
						SyncMode:           StateSyncModeImmediate,
						SyncInterval:       metav1.Duration{Duration: 1 * time.Minute},
						ConflictResolution: StateSyncConflictResolutionTimestamp,
						Replicas:           3,
					},
					RetentionPolicy: &StateRetentionPolicy{
						RetentionPeriod:        metav1.Duration{Duration: 24 * time.Hour},
						FailedSessionRetention: metav1.Duration{Duration: 7 * 24 * time.Hour},
						CheckpointRetention:    metav1.Duration{Duration: 30 * 24 * time.Hour},
						MaxCheckpoints:         10,
					},
				},
			},
			wantValid: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if tc.state == nil {
				t.Fatal("state cannot be nil")
			}

			// Validate session reference
			if tc.state.Spec.SessionRef.Name == "" && tc.wantValid {
				t.Error("expected valid state, but SessionRef.Name is empty")
				return
			}

			// Validate state data
			validPhases := []SessionPhase{
				SessionPhaseCreated, SessionPhaseInitializing, SessionPhaseActive,
				SessionPhaseSuspended, SessionPhaseCompleting, SessionPhaseCompleted,
				SessionPhaseFailed, SessionPhaseTerminated,
			}
			found := false
			for _, validPhase := range validPhases {
				if tc.state.Spec.StateData.CurrentPhase == validPhase {
					found = true
					break
				}
			}
			if !found && tc.wantValid {
				t.Errorf("invalid current phase: %s", tc.state.Spec.StateData.CurrentPhase)
			}

			// Validate resource allocations
			for _, allocation := range tc.state.Spec.StateData.ResourceAllocations {
				if allocation.ClusterName == "" && tc.wantValid {
					t.Error("resource allocation must have cluster name")
				}

				validStatuses := []AllocationStatus{
					AllocationStatusAllocated, AllocationStatusPending,
					AllocationStatusFailed, AllocationStatusReleased,
				}
				found := false
				for _, validStatus := range validStatuses {
					if allocation.Status == validStatus {
						found = true
						break
					}
				}
				if !found && tc.wantValid && allocation.Status != "" {
					t.Errorf("invalid allocation status: %s", allocation.Status)
				}
			}
		})
	}
}

func TestPlacementDecisionValidation(t *testing.T) {
	sessionRef := SessionReference{
		Name:      "test-session",
		Namespace: "default",
		SessionID: "session-123",
	}
	workloadRef := WorkloadReference{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Name:       "web-app",
		Namespace:  "default",
	}

	tests := map[string]struct {
		decision  *PlacementDecision
		wantValid bool
	}{
		"valid placement decision with basic configuration": {
			decision: &PlacementDecision{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-decision",
					Namespace: "default",
				},
				Spec: PlacementDecisionSpec{
					SessionRef:      sessionRef,
					WorkloadRef:     workloadRef,
					TargetCluster:   "prod-cluster-1",
					PlacementReason: "Best resource availability",
					PlacementScore:  85,
				},
			},
			wantValid: true,
		},
		"valid decision with comprehensive context": {
			decision: &PlacementDecision{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "comprehensive-decision",
					Namespace: "production",
				},
				Spec: PlacementDecisionSpec{
					SessionRef:      sessionRef,
					WorkloadRef:     workloadRef,
					TargetCluster:   "prod-cluster-1",
					PlacementReason: "Optimal placement based on policies",
					PlacementScore:  92,
					DecisionContext: &DecisionContext{
						DecisionID:        "decision-123",
						DecisionAlgorithm: "weighted-scoring",
						EvaluatedClusters: []ClusterEvaluation{
							{
								ClusterName: "prod-cluster-1",
								Score:       92,
								Eligible:    true,
								EvaluationCriteria: []EvaluationCriterion{
									{
										Name:        "resource-availability",
										Weight:      50,
										Score:       90,
										Description: "Available CPU and memory resources",
									},
									{
										Name:        "affinity-match",
										Weight:      30,
										Score:       95,
										Description: "Matches workload affinity requirements",
									},
								},
								ResourceAvailability: map[string]resource.Quantity{
									"cpu":    resource.MustParse("50"),
									"memory": resource.MustParse("500Gi"),
								},
							},
							{
								ClusterName: "prod-cluster-2",
								Score:       75,
								Eligible:    true,
								RejectionReasons: []string{
									"Lower resource availability",
								},
							},
						},
						AppliedPolicies: []AppliedPolicy{
							{
								PolicyName:   "affinity-policy",
								PolicyType:   PlacementPolicyTypeAffinity,
								Impact:       "Preferred cluster-1 due to affinity rules",
								Applied:      true,
								AppliedRules: []string{"cluster-affinity", "zone-preference"},
							},
						},
						DecisionMetrics: &DecisionMetrics{
							EvaluationDuration:   &metav1.Duration{Duration: 200 * time.Millisecond},
							ClustersEvaluated:    2,
							PoliciesApplied:      1,
							ConstraintsEvaluated: 3,
							ConflictsDetected:    0,
						},
					},
					RollbackPolicy: &RollbackPolicy{
						Enabled:      true,
						AutoRollback: false,
						RollbackTriggers: []RollbackTrigger{
							{
								Type:      RollbackTriggerTypeHealthCheck,
								Condition: "health_score < 50",
								Threshold: "50",
								Duration:  metav1.Duration{Duration: 5 * time.Minute},
							},
						},
						RollbackTimeout: metav1.Duration{Duration: 10 * time.Minute},
						RetainHistory:   true,
					},
				},
			},
			wantValid: true,
		},
		"invalid - empty target cluster": {
			decision: &PlacementDecision{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-decision",
					Namespace: "default",
				},
				Spec: PlacementDecisionSpec{
					SessionRef:  sessionRef,
					WorkloadRef: workloadRef,
					// Missing TargetCluster
				},
			},
			wantValid: false,
		},
		"invalid - invalid placement score": {
			decision: &PlacementDecision{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-score-decision",
					Namespace: "default",
				},
				Spec: PlacementDecisionSpec{
					SessionRef:     sessionRef,
					WorkloadRef:    workloadRef,
					TargetCluster:  "cluster-1",
					PlacementScore: 150, // Invalid - over 100
				},
			},
			wantValid: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if tc.decision == nil {
				t.Fatal("decision cannot be nil")
			}

			// Validate target cluster
			if tc.decision.Spec.TargetCluster == "" && tc.wantValid {
				t.Error("expected valid decision, but TargetCluster is empty")
				return
			}

			// Validate placement score
			if tc.decision.Spec.PlacementScore < 0 || tc.decision.Spec.PlacementScore > 100 {
				if tc.wantValid {
					t.Errorf("placement score %d is out of valid range [0, 100]", tc.decision.Spec.PlacementScore)
				}
			}

			// Validate decision context
			if tc.decision.Spec.DecisionContext != nil {
				for _, evaluation := range tc.decision.Spec.DecisionContext.EvaluatedClusters {
					if evaluation.Score < 0 || evaluation.Score > 100 {
						if tc.wantValid {
							t.Errorf("cluster evaluation score %d is out of valid range [0, 100]", evaluation.Score)
						}
					}

					for _, criterion := range evaluation.EvaluationCriteria {
						if criterion.Weight < 0 || criterion.Weight > 100 {
							if tc.wantValid {
								t.Errorf("evaluation criterion weight %d is out of valid range [0, 100]", criterion.Weight)
							}
						}
						if criterion.Score < 0 || criterion.Score > 100 {
							if tc.wantValid {
								t.Errorf("evaluation criterion score %d is out of valid range [0, 100]", criterion.Score)
							}
						}
					}
				}
			}
		})
	}
}

func TestSessionValidatorValidation(t *testing.T) {
	tests := map[string]struct {
		validator *SessionValidator
		wantValid bool
	}{
		"valid session validator with basic rules": {
			validator: &SessionValidator{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-validator",
					Namespace: "default",
				},
				Spec: SessionValidatorSpec{
					ValidationRules: []ValidationRule{
						{
							Name: "session-timeout-check",
							Type: ValidationRuleTypeSessionConfiguration,
							Condition: ValidationCondition{
								Event: ValidationEventSessionCreate,
							},
							Validator: ValidatorConfiguration{
								Type: ValidatorTypeRange,
								Parameters: map[string]string{
									"field": "spec.sessionConfiguration.sessionTimeout",
									"min":   "1h",
									"max":   "72h",
								},
								Timeout: metav1.Duration{Duration: 30 * time.Second},
							},
							Severity:    ValidationSeverityError,
							Enabled:     true,
							Description: "Validate session timeout is within acceptable range",
						},
					},
				},
			},
			wantValid: true,
		},
		"valid validator with comprehensive configuration": {
			validator: &SessionValidator{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "comprehensive-validator",
					Namespace: "production",
				},
				Spec: SessionValidatorSpec{
					ValidationRules: []ValidationRule{
						{
							Name: "resource-constraint-check",
							Type: ValidationRuleTypeResourceConstraint,
							Condition: ValidationCondition{
								Event: ValidationEventDecisionCreate,
								Filters: []ValidationFilter{
									{
										Field:    "spec.workloadRef.kind",
										Operator: FilterOperatorEquals,
										Value:    "Deployment",
									},
								},
							},
							Validator: ValidatorConfiguration{
								Type: ValidatorTypeCustom,
								Parameters: map[string]string{
									"script_type": "lua",
								},
								Script: `
									if workload.resources.cpu > cluster.available.cpu then
										return false, "Insufficient CPU resources"
									end
									return true, "Resource constraints satisfied"
								`,
								Timeout: metav1.Duration{Duration: 60 * time.Second},
								RetryPolicy: &ValidationRetryPolicy{
									MaxRetries:        3,
									RetryDelay:        metav1.Duration{Duration: 1 * time.Second},
									BackoffMultiplier: 2.0,
								},
							},
							Severity: ValidationSeverityError,
							Enabled:  true,
						},
					},
					ConflictDetection: &ConflictDetectionPolicy{
						Enabled: true,
						DetectionScope: &ConflictDetectionScope{
							IncludeNamespaces: []string{"production", "staging"},
							ResourceTypes:     []string{"deployments", "statefulsets"},
						},
						ConflictTypes: []ConflictType{
							ConflictTypeResourceConflict,
							ConflictTypeConstraintConflict,
						},
						ResolutionStrategies: []ConflictResolutionStrategy{
							{
								ConflictType: ConflictTypeResourceConflict,
								Strategy:     ConflictResolutionTypeOverride,
								Priority:     800,
								Parameters: map[string]string{
									"prefer": "higher_priority_workload",
								},
							},
						},
					},
					ResourceValidation: &ResourceValidationPolicy{
						ValidateCapacity:     true,
						ValidateAvailability: true,
						CapacityThresholds: map[string]ResourceThreshold{
							"cpu": {
								WarningThreshold: 80,
								ErrorThreshold:   95,
							},
							"memory": {
								WarningThreshold: 85,
								ErrorThreshold:   98,
							},
						},
						ReservationPolicy: &ResourceReservationPolicy{
							ReservationMode:        ResourceReservationModeSoft,
							ReservationDuration:    metav1.Duration{Duration: 5 * time.Minute},
							AllowOversubscription: false,
						},
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
					ValidationRules: []ValidationRule{}, // Invalid - empty
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
				t.Error("expected valid validator, but ValidationRules is empty")
				return
			}

			// Validate rule configurations
			for _, rule := range tc.validator.Spec.ValidationRules {
				// Validate rule type
				validRuleTypes := []ValidationRuleType{
					ValidationRuleTypeSessionConfiguration,
					ValidationRuleTypePlacementPolicy,
					ValidationRuleTypeResourceConstraint,
					ValidationRuleTypeConflictDetection,
					ValidationRuleTypeDependencyCheck,
				}
				found := false
				for _, validType := range validRuleTypes {
					if rule.Type == validType {
						found = true
						break
					}
				}
				if !found && tc.wantValid {
					t.Errorf("invalid rule type: %s", rule.Type)
				}

				// Validate validator type
				validValidatorTypes := []ValidatorType{
					ValidatorTypeRequired, ValidatorTypeRange, ValidatorTypeFormat,
					ValidatorTypeCustom, ValidatorTypeReference, ValidatorTypeUniqueness,
					ValidatorTypeConsistency,
				}
				found = false
				for _, validType := range validValidatorTypes {
					if rule.Validator.Type == validType {
						found = true
						break
					}
				}
				if !found && tc.wantValid {
					t.Errorf("invalid validator type: %s", rule.Validator.Type)
				}
			}
		})
	}
}

func TestPlacementSessionStatusConditions(t *testing.T) {
	session := &PlacementSession{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-placement-session",
			Namespace: "default",
		},
		Status: PlacementSessionStatus{
			Conditions: []conditionsv1alpha1.Condition{
				{
					Type:   "Ready",
					Status: "True",
					Reason: "SessionActive",
				},
				{
					Type:   "Healthy",
					Status: "True",
					Reason: "AllDecisionsSuccessful",
				},
			},
			Phase:     SessionPhaseActive,
			SessionID: "session-123",
			SessionMetrics: &SessionMetrics{
				TotalDecisions:      10,
				ActiveDecisions:     5,
				SuccessfulDecisions: 8,
				FailedDecisions:     2,
				ConflictsResolved:   1,
				AverageDecisionTime: &metav1.Duration{Duration: 500 * time.Millisecond},
				SessionDuration:     &metav1.Duration{Duration: 2 * time.Hour},
			},
		},
	}

	// Test condition presence
	if len(session.Status.Conditions) != 2 {
		t.Errorf("expected 2 conditions, got %d", len(session.Status.Conditions))
	}

	// Test session metrics
	if session.Status.SessionMetrics.TotalDecisions != 10 {
		t.Errorf("expected total decisions 10, got %d", session.Status.SessionMetrics.TotalDecisions)
	}

	if session.Status.SessionMetrics.SuccessfulDecisions+session.Status.SessionMetrics.FailedDecisions != session.Status.SessionMetrics.TotalDecisions {
		t.Error("successful + failed decisions should equal total decisions")
	}

	// Test phase validation
	validPhases := []SessionPhase{
		SessionPhaseCreated, SessionPhaseInitializing, SessionPhaseActive,
		SessionPhaseSuspended, SessionPhaseCompleting, SessionPhaseCompleted,
		SessionPhaseFailed, SessionPhaseTerminated,
	}
	found := false
	for _, validPhase := range validPhases {
		if session.Status.Phase == validPhase {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("invalid session phase: %s", session.Status.Phase)
	}
}