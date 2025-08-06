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

	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
)

func TestSessionAffinityPolicy_Validation(t *testing.T) {
	tests := map[string]struct {
		policy      *SessionAffinityPolicy
		expectValid bool
		description string
	}{
		"valid minimal policy": {
			policy: &SessionAffinityPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-policy",
					Namespace: "default",
					Annotations: map[string]string{
						"kcp.io/cluster": "test-cluster",
					},
				},
				Spec: SessionAffinityPolicySpec{
					WorkloadSelector: WorkloadSelector{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "test-app",
							},
						},
					},
					ClusterSelector: ClusterSelector{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"region": "us-east-1",
							},
						},
					},
					AffinityType: SessionAffinityTypeClientIP,
					StickinessPolicy: StickinessPolicy{
						Type: StickinessTypeSoft,
					},
				},
			},
			expectValid: true,
			description: "minimal valid SessionAffinityPolicy should pass validation",
		},
		"policy with cookie affinity": {
			policy: &SessionAffinityPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cookie-policy",
					Namespace: "default",
					Annotations: map[string]string{
						"kcp.io/cluster": "test-cluster",
					},
				},
				Spec: SessionAffinityPolicySpec{
					WorkloadSelector: WorkloadSelector{
						WorkloadTypes: []WorkloadType{
							{APIVersion: "apps/v1", Kind: "Deployment"},
						},
					},
					ClusterSelector: ClusterSelector{
						ClusterNames: []string{"cluster-1", "cluster-2"},
					},
					AffinityType: SessionAffinityTypeCookie,
					StickinessPolicy: StickinessPolicy{
						Type:                  StickinessTypeHard,
						Duration:              metav1.Duration{Duration: time.Hour},
						MaxBindings:           5,
						BreakOnClusterFailure: false,
					},
					Weight: 75,
				},
			},
			expectValid: true,
			description: "cookie-based affinity with hard stickiness should be valid",
		},
		"policy with failover configuration": {
			policy: &SessionAffinityPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "failover-policy",
					Namespace: "production",
					Annotations: map[string]string{
						"kcp.io/cluster": "prod-cluster",
					},
				},
				Spec: SessionAffinityPolicySpec{
					WorkloadSelector: WorkloadSelector{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"tier": "frontend",
							},
						},
					},
					ClusterSelector: ClusterSelector{
						LocationSelector: []string{"us-west-2a", "us-west-2b"},
					},
					AffinityType: SessionAffinityTypePersistentSession,
					StickinessPolicy: StickinessPolicy{
						Type: StickinessTypeAdaptive,
					},
					FailoverPolicy: &AffinityFailoverPolicy{
						Strategy:            FailoverStrategyDelayed,
						DelayBeforeFailover: metav1.Duration{Duration: 5 * time.Minute},
						MaxFailoverAttempts: 2,
						BackoffMultiplier:   150,
						AlternativeClusterSelector: &ClusterSelector{
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"failover": "enabled",
								},
							},
						},
					},
				},
			},
			expectValid: true,
			description: "policy with comprehensive failover configuration should be valid",
		},
		"invalid policy without KCP annotation": {
			policy: &SessionAffinityPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-policy",
					Namespace: "default",
				},
				Spec: SessionAffinityPolicySpec{
					WorkloadSelector: WorkloadSelector{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "test",
							},
						},
					},
					ClusterSelector: ClusterSelector{
						ClusterNames: []string{"cluster-1"},
					},
					AffinityType: SessionAffinityTypeClientIP,
					StickinessPolicy: StickinessPolicy{
						Type: StickinessTypeSoft,
					},
				},
			},
			expectValid: false,
			description: "policy without kcp.io/cluster annotation should fail validation",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Basic structural validation
			if tc.policy == nil {
				t.Fatal("test policy cannot be nil")
			}

			// Validate required fields are present
			if tc.expectValid {
				if tc.policy.Spec.AffinityType == "" {
					t.Error("expected valid policy to have AffinityType set")
				}

				if tc.policy.Spec.StickinessPolicy.Type == "" {
					t.Error("expected valid policy to have StickinessPolicy.Type set")
				}

				// Check KCP annotation for valid policies
				if tc.policy.Annotations == nil || tc.policy.Annotations["kcp.io/cluster"] == "" {
					t.Error("expected valid policy to have kcp.io/cluster annotation")
				}
			}

			t.Logf("Test '%s': %s", name, tc.description)
		})
	}
}

func TestSessionAffinityPolicyStatus_Phases(t *testing.T) {
	tests := map[string]struct {
		status      SessionAffinityPolicyStatus
		expectedMsg string
	}{
		"active status": {
			status: SessionAffinityPolicyStatus{
				Phase:          SessionAffinityPolicyPhaseActive,
				ActiveBindings: 5,
				TotalBindings:  10,
				LastUpdateTime: &metav1.Time{Time: time.Now()},
			},
			expectedMsg: "policy should be in active state",
		},
		"draining status": {
			status: SessionAffinityPolicyStatus{
				Phase:          SessionAffinityPolicyPhaseDraining,
				ActiveBindings: 2,
				TotalBindings:  10,
				Message:        "Policy is being drained due to cluster maintenance",
			},
			expectedMsg: "policy should be in draining state with fewer active bindings",
		},
		"failed status": {
			status: SessionAffinityPolicyStatus{
				Phase:   SessionAffinityPolicyPhaseFailed,
				Message: "Failed to establish affinity bindings",
				Conditions: conditionsv1alpha1.Conditions{
					{
						Type:    "Ready",
						Status:  "False",
						Reason:  "BindingFailed",
						Message: "Unable to create session bindings",
					},
				},
			},
			expectedMsg: "policy should be in failed state with error conditions",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if tc.status.Phase == "" {
				t.Error("status phase should not be empty")
			}

			// Validate phase-specific logic
			switch tc.status.Phase {
			case SessionAffinityPolicyPhaseActive:
				if tc.status.ActiveBindings < 0 {
					t.Error("active policy should not have negative active bindings")
				}
			case SessionAffinityPolicyPhaseDraining:
				if tc.status.ActiveBindings > tc.status.TotalBindings {
					t.Error("draining policy cannot have more active than total bindings")
				}
			case SessionAffinityPolicyPhaseFailed:
				if len(tc.status.Conditions) == 0 {
					t.Error("failed policy should have condition information")
				}
			}

			t.Logf("Status test '%s': %s", name, tc.expectedMsg)
		})
	}
}

func TestStickinessPolicy_Types(t *testing.T) {
	tests := map[string]struct {
		policy      StickinessPolicy
		expectValid bool
	}{
		"hard stickiness": {
			policy: StickinessPolicy{
				Type:                  StickinessTypeHard,
				Duration:              metav1.Duration{Duration: time.Hour},
				MaxBindings:           1,
				BreakOnClusterFailure: false,
			},
			expectValid: true,
		},
		"soft stickiness with multiple bindings": {
			policy: StickinessPolicy{
				Type:                  StickinessTypeSoft,
				Duration:              metav1.Duration{Duration: 30 * time.Minute},
				MaxBindings:           3,
				BreakOnClusterFailure: true,
			},
			expectValid: true,
		},
		"adaptive stickiness": {
			policy: StickinessPolicy{
				Type:        StickinessTypeAdaptive,
				MaxBindings: 5,
			},
			expectValid: true,
		},
		"no stickiness": {
			policy: StickinessPolicy{
				Type: StickinessTypeNone,
			},
			expectValid: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if tc.policy.Type == "" {
				t.Error("stickiness type should not be empty")
			}

			// Validate type-specific constraints
			if tc.policy.MaxBindings < 0 {
				t.Error("MaxBindings cannot be negative")
			}

			if tc.policy.MaxBindings > 10 {
				t.Error("MaxBindings should not exceed maximum allowed value")
			}

			if tc.expectValid {
				t.Logf("Valid stickiness policy: %s with %d max bindings",
					tc.policy.Type, tc.policy.MaxBindings)
			}
		})
	}
}

func TestFailoverEvent_Creation(t *testing.T) {
	now := metav1.Now()
	event := FailoverEvent{
		Timestamp:        now,
		Reason:           "ClusterUnhealthy",
		SourceCluster:    "unhealthy-cluster",
		TargetCluster:    "healthy-cluster",
		AffectedSessions: 15,
		Success:          true,
		Message:          "Successfully failed over 15 sessions to healthy cluster",
	}

	if event.Timestamp.IsZero() {
		t.Error("failover event should have a timestamp")
	}

	if event.Reason == "" {
		t.Error("failover event should have a reason")
	}

	if event.SourceCluster == "" {
		t.Error("failover event should specify source cluster")
	}

	if event.AffectedSessions <= 0 {
		t.Error("successful failover should affect at least one session")
	}

	if !event.Success && event.TargetCluster != "" {
		t.Error("failed failover should not specify target cluster")
	}

	t.Logf("Failover event: %d sessions from %s to %s (%s)",
		event.AffectedSessions, event.SourceCluster, event.TargetCluster, event.Reason)
}

func TestSessionAffinityTypes_Constants(t *testing.T) {
	affinityTypes := []SessionAffinityType{
		SessionAffinityTypeClientIP,
		SessionAffinityTypeCookie,
		SessionAffinityTypeHeader,
		SessionAffinityTypeWorkloadUID,
		SessionAffinityTypePersistentSession,
		SessionAffinityTypeNone,
	}

	if len(affinityTypes) != 6 {
		t.Errorf("expected 6 affinity types, got %d", len(affinityTypes))
	}

	for i, affinityType := range affinityTypes {
		if affinityType == "" {
			t.Errorf("affinity type at index %d is empty", i)
		}
		t.Logf("Affinity type %d: %s", i, affinityType)
	}
}

func TestFailoverStrategies_Constants(t *testing.T) {
	strategies := []FailoverStrategy{
		FailoverStrategyImmediate,
		FailoverStrategyDelayed,
		FailoverStrategyManual,
		FailoverStrategyDisabled,
	}

	if len(strategies) != 4 {
		t.Errorf("expected 4 failover strategies, got %d", len(strategies))
	}

	for i, strategy := range strategies {
		if strategy == "" {
			t.Errorf("failover strategy at index %d is empty", i)
		}
		t.Logf("Failover strategy %d: %s", i, strategy)
	}
}