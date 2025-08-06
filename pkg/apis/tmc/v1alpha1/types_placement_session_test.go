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
		"invalid - empty cluster selector": {
			session: &PlacementSession{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-clusters",
					Namespace: "default",
				},
				Spec: PlacementSessionSpec{
					WorkloadSelector: WorkloadSelector{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "web"},
						},
					},
					ClusterSelector: ClusterSelector{}, // Empty selector
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
			if !isValidWorkloadSelector(tc.session.Spec.WorkloadSelector) && tc.wantValid {
				t.Error("expected valid session, but WorkloadSelector is invalid")
				return
			}

			// Validate cluster selector
			if !isValidClusterSelector(tc.session.Spec.ClusterSelector) && tc.wantValid {
				t.Error("expected valid session, but ClusterSelector is invalid")
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

func TestSessionPhaseTransitions(t *testing.T) {
	validTransitions := map[SessionPhase][]SessionPhase{
		SessionPhaseCreated:      {SessionPhaseInitializing, SessionPhaseTerminated},
		SessionPhaseInitializing: {SessionPhaseActive, SessionPhaseFailed, SessionPhaseTerminated},
		SessionPhaseActive:       {SessionPhaseSuspended, SessionPhaseCompleting, SessionPhaseFailed, SessionPhaseTerminated},
		SessionPhaseSuspended:    {SessionPhaseActive, SessionPhaseTerminated},
		SessionPhaseCompleting:   {SessionPhaseCompleted, SessionPhaseFailed},
		SessionPhaseCompleted:    {}, // Terminal state
		SessionPhaseFailed:       {SessionPhaseActive}, // Can be restarted
		SessionPhaseTerminated:   {}, // Terminal state
	}

	for fromPhase, allowedToPhases := range validTransitions {
		for _, toPhase := range allowedToPhases {
			t.Run(string(fromPhase)+"->"+string(toPhase), func(t *testing.T) {
				if !isValidPhaseTransition(fromPhase, toPhase) {
					t.Errorf("transition from %s to %s should be valid", fromPhase, toPhase)
				}
			})
		}
	}

	// Test invalid transitions
	invalidTransitions := []struct {
		from, to SessionPhase
	}{
		{SessionPhaseCompleted, SessionPhaseActive},
		{SessionPhaseTerminated, SessionPhaseActive},
		{SessionPhaseCreated, SessionPhaseActive}, // Must go through Initializing
	}

	for _, transition := range invalidTransitions {
		t.Run(string(transition.from)+"->"+string(transition.to)+"(invalid)", func(t *testing.T) {
			if isValidPhaseTransition(transition.from, transition.to) {
				t.Errorf("transition from %s to %s should be invalid", transition.from, transition.to)
			}
		})
	}
}

func TestSessionConfigurationValidation(t *testing.T) {
	tests := map[string]struct {
		config    SessionConfiguration
		wantValid bool
	}{
		"valid basic configuration": {
			config: SessionConfiguration{
				SessionTimeout:      metav1.Duration{Duration: 1 * time.Hour},
				HeartbeatInterval:   metav1.Duration{Duration: 30 * time.Second},
				MaxDecisions:        50,
				ConflictResolution:  ConflictResolutionTypeMerge,
				PersistenceStrategy: PersistenceStrategyPersistent,
			},
			wantValid: true,
		},
		"invalid - zero timeout": {
			config: SessionConfiguration{
				SessionTimeout:      metav1.Duration{Duration: 0},
				ConflictResolution:  ConflictResolutionTypeMerge,
				PersistenceStrategy: PersistenceStrategyPersistent,
			},
			wantValid: false,
		},
		"invalid - negative max decisions": {
			config: SessionConfiguration{
				SessionTimeout:      metav1.Duration{Duration: 1 * time.Hour},
				MaxDecisions:        -1,
				ConflictResolution:  ConflictResolutionTypeMerge,
				PersistenceStrategy: PersistenceStrategyPersistent,
			},
			wantValid: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			isValid := isValidSessionConfiguration(tc.config)
			if isValid != tc.wantValid {
				t.Errorf("expected validity %v, got %v", tc.wantValid, isValid)
			}
		})
	}
}

// Helper validation functions

func isValidPhaseTransition(from, to SessionPhase) bool {
	validTransitions := map[SessionPhase][]SessionPhase{
		SessionPhaseCreated:      {SessionPhaseInitializing, SessionPhaseTerminated},
		SessionPhaseInitializing: {SessionPhaseActive, SessionPhaseFailed, SessionPhaseTerminated},
		SessionPhaseActive:       {SessionPhaseSuspended, SessionPhaseCompleting, SessionPhaseFailed, SessionPhaseTerminated},
		SessionPhaseSuspended:    {SessionPhaseActive, SessionPhaseTerminated},
		SessionPhaseCompleting:   {SessionPhaseCompleted, SessionPhaseFailed},
		SessionPhaseCompleted:    {},
		SessionPhaseFailed:       {SessionPhaseActive},
		SessionPhaseTerminated:   {},
	}

	allowedTransitions, exists := validTransitions[from]
	if !exists {
		return false
	}

	for _, allowed := range allowedTransitions {
		if allowed == to {
			return true
		}
	}
	return false
}

func isValidSessionConfiguration(config SessionConfiguration) bool {
	// Session timeout must be positive
	if config.SessionTimeout.Duration <= 0 {
		return false
	}

	// Max decisions must be non-negative (0 means unlimited)
	if config.MaxDecisions < 0 {
		return false
	}

	// Heartbeat interval should be positive if specified
	if config.HeartbeatInterval.Duration < 0 {
		return false
	}

	return true
}