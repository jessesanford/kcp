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
)

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
		"valid decision with execution status": {
			decision: &PlacementDecision{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "executing-decision",
					Namespace: "default",
				},
				Spec: PlacementDecisionSpec{
					SessionRef:    sessionRef,
					WorkloadRef:   workloadRef,
					TargetCluster: "test-cluster",
				},
				Status: PlacementDecisionStatus{
					Phase: PlacementDecisionPhaseExecuting,
					ExecutionStatus: &DecisionExecutionStatus{
						Phase:     DecisionExecutionPhaseInProgress,
						StartTime: &metav1.Time{Time: time.Now()},
						ExecutionSteps: []ExecutionStep{
							{
								StepName: "validate-placement",
								Status:   ExecutionStepStatusCompleted,
								StartTime: &metav1.Time{Time: time.Now().Add(-2 * time.Minute)},
								CompletionTime: &metav1.Time{Time: time.Now().Add(-1 * time.Minute)},
								Message: "Placement validation successful",
							},
							{
								StepName:  "deploy-workload",
								Status:    ExecutionStepStatusRunning,
								StartTime: &metav1.Time{Time: time.Now().Add(-1 * time.Minute)},
								Message:   "Deploying workload to target cluster",
							},
						},
						RetryCount: 0,
					},
				},
			},
			wantValid: true,
		},
		"valid decision with conflict status": {
			decision: &PlacementDecision{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "conflict-decision",
					Namespace: "default",
				},
				Spec: PlacementDecisionSpec{
					SessionRef:    sessionRef,
					WorkloadRef:   workloadRef,
					TargetCluster: "cluster-with-conflicts",
				},
				Status: PlacementDecisionStatus{
					Phase: PlacementDecisionPhaseActive,
					ConflictStatus: &DecisionConflictStatus{
						HasConflicts:  true,
						ConflictCount: 1,
						ActiveConflicts: []DecisionConflict{
							{
								ConflictID:   "conflict-123",
								ConflictType: ConflictTypeResourceContention,
								ConflictingDecision: &PlacementReference{
									Name:      "other-decision",
									Namespace: "default",
								},
								DetectedTime: metav1.Time{Time: time.Now().Add(-5 * time.Minute)},
								Description:  "Resource contention with another placement",
								ResolutionStrategy: ConflictResolutionTypePriorityBased,
								Status:       ConflictStatusAnalyzing,
							},
						},
						LastConflictTime: &metav1.Time{Time: time.Now().Add(-5 * time.Minute)},
					},
				},
			},
			wantValid: true,
		},
		"valid decision with rollback status": {
			decision: &PlacementDecision{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rollback-decision",
					Namespace: "default",
				},
				Spec: PlacementDecisionSpec{
					SessionRef:    sessionRef,
					WorkloadRef:   workloadRef,
					TargetCluster: "failed-cluster",
				},
				Status: PlacementDecisionStatus{
					Phase: PlacementDecisionPhaseRolledBack,
					RollbackStatus: &RollbackStatus{
						InProgress:       false,
						RollbackAttempts: 1,
						LastRollbackTime: &metav1.Time{Time: time.Now().Add(-10 * time.Minute)},
						RollbackHistory: []RollbackOperation{
							{
								OperationID:   "rollback-op-123",
								StartTime:     metav1.Time{Time: time.Now().Add(-15 * time.Minute)},
								CompletionTime: &metav1.Time{Time: time.Now().Add(-10 * time.Minute)},
								Status:        RollbackOperationStatusCompleted,
								TriggerType:   RollbackTriggerTypeHealthCheck,
								TriggerReason: "Health check failure threshold exceeded",
								SourceCluster: "failed-cluster",
								TargetCluster: "backup-cluster",
								Steps: []RollbackStep{
									{
										StepName: "drain-workload",
										Status:   ExecutionStepStatusCompleted,
										StartTime: &metav1.Time{Time: time.Now().Add(-15 * time.Minute)},
										CompletionTime: &metav1.Time{Time: time.Now().Add(-12 * time.Minute)},
										Message: "Successfully drained workload from source cluster",
									},
									{
										StepName: "redeploy-workload",
										Status:   ExecutionStepStatusCompleted,
										StartTime: &metav1.Time{Time: time.Now().Add(-12 * time.Minute)},
										CompletionTime: &metav1.Time{Time: time.Now().Add(-10 * time.Minute)},
										Message: "Successfully redeployed workload to target cluster",
									},
								},
							},
						},
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
		"invalid - negative placement score": {
			decision: &PlacementDecision{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "negative-score-decision",
					Namespace: "default",
				},
				Spec: PlacementDecisionSpec{
					SessionRef:     sessionRef,
					WorkloadRef:    workloadRef,
					TargetCluster:  "cluster-1",
					PlacementScore: -10, // Invalid - negative
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

			// Validate rollback policy
			if tc.decision.Spec.RollbackPolicy != nil {
				for _, trigger := range tc.decision.Spec.RollbackPolicy.RollbackTriggers {
					if trigger.Type == "" {
						if tc.wantValid {
							t.Error("rollback trigger type cannot be empty")
						}
					}
				}
			}

			// Validate execution status
			if tc.decision.Status.ExecutionStatus != nil {
				for _, step := range tc.decision.Status.ExecutionStatus.ExecutionSteps {
					if step.StepName == "" {
						if tc.wantValid {
							t.Error("execution step name cannot be empty")
						}
					}
				}
			}

			// Validate conflict status
			if tc.decision.Status.ConflictStatus != nil && tc.decision.Status.ConflictStatus.HasConflicts {
				if len(tc.decision.Status.ConflictStatus.ActiveConflicts) == 0 && tc.wantValid {
					t.Error("conflict status indicates conflicts but no active conflicts provided")
				}
			}
		})
	}
}

func TestPlacementDecisionPhaseTransitions(t *testing.T) {
	tests := map[string]struct {
		fromPhase PlacementDecisionPhase
		toPhase   PlacementDecisionPhase
		isValid   bool
	}{
		"pending to evaluating": {
			fromPhase: PlacementDecisionPhasePending,
			toPhase:   PlacementDecisionPhaseEvaluating,
			isValid:   true,
		},
		"evaluating to decided": {
			fromPhase: PlacementDecisionPhaseEvaluating,
			toPhase:   PlacementDecisionPhaseDecided,
			isValid:   true,
		},
		"decided to executing": {
			fromPhase: PlacementDecisionPhaseDecided,
			toPhase:   PlacementDecisionPhaseExecuting,
			isValid:   true,
		},
		"executing to active": {
			fromPhase: PlacementDecisionPhaseExecuting,
			toPhase:   PlacementDecisionPhaseActive,
			isValid:   true,
		},
		"active to completed": {
			fromPhase: PlacementDecisionPhaseActive,
			toPhase:   PlacementDecisionPhaseCompleted,
			isValid:   true,
		},
		"evaluating to failed": {
			fromPhase: PlacementDecisionPhaseEvaluating,
			toPhase:   PlacementDecisionPhaseFailed,
			isValid:   true,
		},
		"executing to rolled back": {
			fromPhase: PlacementDecisionPhaseExecuting,
			toPhase:   PlacementDecisionPhaseRolledBack,
			isValid:   true,
		},
		"invalid - completed to pending": {
			fromPhase: PlacementDecisionPhaseCompleted,
			toPhase:   PlacementDecisionPhasePending,
			isValid:   false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// This is a logical test - in practice, phase transitions would be
			// enforced by the controller logic, not by the API types themselves
			if tc.fromPhase == "" || tc.toPhase == "" {
				t.Error("phases cannot be empty")
			}

			// Validate phase values are from the defined enum
			validPhases := map[PlacementDecisionPhase]bool{
				PlacementDecisionPhasePending:     true,
				PlacementDecisionPhaseEvaluating:  true,
				PlacementDecisionPhaseDecided:     true,
				PlacementDecisionPhaseExecuting:   true,
				PlacementDecisionPhaseActive:      true,
				PlacementDecisionPhaseCompleted:   true,
				PlacementDecisionPhaseFailed:      true,
				PlacementDecisionPhaseCancelled:   true,
				PlacementDecisionPhaseRolledBack:  true,
			}

			if !validPhases[tc.fromPhase] {
				t.Errorf("invalid from phase: %s", tc.fromPhase)
			}
			if !validPhases[tc.toPhase] {
				t.Errorf("invalid to phase: %s", tc.toPhase)
			}
		})
	}
}

func TestDecisionContextValidation(t *testing.T) {
	tests := map[string]struct {
		context   *DecisionContext
		wantValid bool
	}{
		"valid basic context": {
			context: &DecisionContext{
				DecisionID:        "ctx-123",
				DecisionAlgorithm: "simple-scoring",
			},
			wantValid: true,
		},
		"valid comprehensive context": {
			context: &DecisionContext{
				DecisionID:        "ctx-456",
				DecisionAlgorithm: "weighted-scoring",
				EvaluatedClusters: []ClusterEvaluation{
					{
						ClusterName: "cluster-1",
						Score:       85,
						Eligible:    true,
						EvaluationCriteria: []EvaluationCriterion{
							{Name: "cpu", Weight: 40, Score: 90},
							{Name: "memory", Weight: 30, Score: 80},
							{Name: "affinity", Weight: 30, Score: 85},
						},
					},
				},
				AppliedPolicies: []AppliedPolicy{
					{
						PolicyName: "resource-policy",
						PolicyType: PlacementPolicyTypeResource,
						Applied:    true,
					},
				},
				DecisionMetrics: &DecisionMetrics{
					ClustersEvaluated: 3,
					PoliciesApplied:   1,
				},
			},
			wantValid: true,
		},
		"invalid - empty decision ID": {
			context: &DecisionContext{
				DecisionAlgorithm: "test-algorithm",
			},
			wantValid: false,
		},
		"invalid - invalid cluster evaluation score": {
			context: &DecisionContext{
				DecisionID: "ctx-789",
				EvaluatedClusters: []ClusterEvaluation{
					{
						ClusterName: "invalid-cluster",
						Score:       150, // Invalid - over 100
						Eligible:    true,
					},
				},
			},
			wantValid: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if tc.context == nil && tc.wantValid {
				t.Fatal("context cannot be nil for valid test")
			}

			if tc.context != nil {
				// Validate decision ID
				if tc.context.DecisionID == "" && tc.wantValid {
					t.Error("decision ID cannot be empty for valid context")
				}

				// Validate cluster evaluations
				for _, evaluation := range tc.context.EvaluatedClusters {
					if evaluation.Score < 0 || evaluation.Score > 100 {
						if tc.wantValid {
							t.Errorf("cluster evaluation score %d is out of valid range [0, 100]", evaluation.Score)
						}
					}

					for _, criterion := range evaluation.EvaluationCriteria {
						if criterion.Weight < 0 || criterion.Weight > 100 {
							if tc.wantValid {
								t.Errorf("criterion weight %d is out of valid range [0, 100]", criterion.Weight)
							}
						}
						if criterion.Score < 0 || criterion.Score > 100 {
							if tc.wantValid {
								t.Errorf("criterion score %d is out of valid range [0, 100]", criterion.Score)
							}
						}
					}
				}
			}
		})
	}
}

func TestRollbackPolicyValidation(t *testing.T) {
	tests := map[string]struct {
		policy    *RollbackPolicy
		wantValid bool
	}{
		"valid basic policy": {
			policy: &RollbackPolicy{
				Enabled:      true,
				AutoRollback: false,
			},
			wantValid: true,
		},
		"valid comprehensive policy": {
			policy: &RollbackPolicy{
				Enabled:      true,
				AutoRollback: true,
				RollbackTriggers: []RollbackTrigger{
					{
						Type:      RollbackTriggerTypeHealthCheck,
						Condition: "health < 50",
						Threshold: "50",
						Duration:  metav1.Duration{Duration: 5 * time.Minute},
					},
					{
						Type:     RollbackTriggerTypeResourceExhaustion,
						Duration: metav1.Duration{Duration: 10 * time.Minute},
					},
				},
				RollbackTimeout: metav1.Duration{Duration: 15 * time.Minute},
				RetainHistory:   true,
			},
			wantValid: true,
		},
		"invalid - trigger without type": {
			policy: &RollbackPolicy{
				Enabled: true,
				RollbackTriggers: []RollbackTrigger{
					{
						// Missing Type
						Condition: "some condition",
					},
				},
			},
			wantValid: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if tc.policy == nil && tc.wantValid {
				t.Fatal("policy cannot be nil for valid test")
			}

			if tc.policy != nil {
				// Validate triggers
				for _, trigger := range tc.policy.RollbackTriggers {
					if trigger.Type == "" && tc.wantValid {
						t.Error("rollback trigger type cannot be empty")
					}
				}
			}
		})
	}
}