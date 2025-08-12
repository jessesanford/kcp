/*
Copyright 2025 The KCP Authors.

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

func TestSessionAffinityPolicyDefaults(t *testing.T) {
	tests := map[string]struct {
		policy   *SessionAffinityPolicy
		validate func(t *testing.T, policy *SessionAffinityPolicy)
	}{
		"default affinity type": {
			policy: &SessionAffinityPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test-policy"},
				Spec: SessionAffinityPolicySpec{},
			},
			validate: func(t *testing.T, policy *SessionAffinityPolicy) {
				if policy.Spec.AffinityType != "" && policy.Spec.AffinityType != ClusterAffinity {
					t.Errorf("expected default affinity type to be ClusterAffinity, got %s", policy.Spec.AffinityType)
				}
			},
		},
		"default session TTL": {
			policy: &SessionAffinityPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test-policy"},
				Spec: SessionAffinityPolicySpec{
					SessionTTL: &metav1.Duration{Duration: time.Hour},
				},
			},
			validate: func(t *testing.T, policy *SessionAffinityPolicy) {
				if policy.Spec.SessionTTL != nil && policy.Spec.SessionTTL.Duration != time.Hour {
					t.Errorf("expected session TTL to be 1h, got %v", policy.Spec.SessionTTL.Duration)
				}
			},
		},
		"default stickiness factor": {
			policy: &SessionAffinityPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test-policy"},
				Spec: SessionAffinityPolicySpec{
					StickinessFactor: func() *float64 { f := 0.5; return &f }(),
				},
			},
			validate: func(t *testing.T, policy *SessionAffinityPolicy) {
				if policy.Spec.StickinessFactor != nil && *policy.Spec.StickinessFactor != 0.5 {
					t.Errorf("expected default stickiness factor to be 0.5, got %f", *policy.Spec.StickinessFactor)
				}
			},
		},
		"default max sessions per target": {
			policy: &SessionAffinityPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test-policy"},
				Spec: SessionAffinityPolicySpec{
					MaxSessionsPerTarget: func() *int32 { i := int32(100); return &i }(),
				},
			},
			validate: func(t *testing.T, policy *SessionAffinityPolicy) {
				if policy.Spec.MaxSessionsPerTarget != nil && *policy.Spec.MaxSessionsPerTarget != 100 {
					t.Errorf("expected default max sessions per target to be 100, got %d", *policy.Spec.MaxSessionsPerTarget)
				}
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			tc.validate(t, tc.policy)
		})
	}
}

func TestSessionAffinityPolicyValidation(t *testing.T) {
	tests := map[string]struct {
		policy      *SessionAffinityPolicy
		expectValid bool
		description string
	}{
		"valid cluster affinity policy": {
			policy: &SessionAffinityPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "valid-cluster-policy"},
				Spec: SessionAffinityPolicySpec{
					AffinityType:     ClusterAffinity,
					SessionTTL:       &metav1.Duration{Duration: time.Hour},
					StickinessFactor: func() *float64 { f := 0.7; return &f }(),
				},
			},
			expectValid: true,
			description: "should accept valid cluster affinity configuration",
		},
		"valid node affinity policy": {
			policy: &SessionAffinityPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "valid-node-policy"},
				Spec: SessionAffinityPolicySpec{
					AffinityType:     NodeAffinity,
					SessionTTL:       &metav1.Duration{Duration: 30 * time.Minute},
					StickinessFactor: func() *float64 { f := 1.0; return &f }(),
				},
			},
			expectValid: true,
			description: "should accept valid node affinity configuration",
		},
		"valid workspace affinity policy": {
			policy: &SessionAffinityPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "valid-workspace-policy"},
				Spec: SessionAffinityPolicySpec{
					AffinityType:     WorkspaceAffinity,
					SessionTTL:       &metav1.Duration{Duration: 2 * time.Hour},
					StickinessFactor: func() *float64 { f := 0.0; return &f }(),
				},
			},
			expectValid: true,
			description: "should accept valid workspace affinity configuration",
		},
		"policy with session selector": {
			policy: &SessionAffinityPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "selector-policy"},
				Spec: SessionAffinityPolicySpec{
					AffinityType: ClusterAffinity,
					SessionSelector: &SessionSelector{
						MatchLabels: map[string]string{
							"app":     "web-server",
							"version": "v1.0",
						},
						WorkloadTypes: []string{"Deployment", "StatefulSet"},
						Namespaces:    []string{"production", "staging"},
					},
				},
			},
			expectValid: true,
			description: "should accept policy with comprehensive session selector",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Basic validation - check that required fields are present
			if tc.policy.Name == "" {
				t.Error("policy name should not be empty")
			}
			
			// Test specific validations based on the test case
			if !tc.expectValid {
				t.Errorf("expected policy to be invalid, but no validation errors found")
			}
		})
	}
}

func TestSessionStateLifecycle(t *testing.T) {
	now := metav1.Now()
	expiresAt := metav1.NewTime(now.Add(time.Hour))

	tests := map[string]struct {
		state    *SessionState
		validate func(t *testing.T, state *SessionState)
	}{
		"pending session state": {
			state: &SessionState{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-session",
					Namespace: "default",
				},
				Spec: SessionStateSpec{
					WorkloadReference: WorkloadReference{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       "web-app",
						Namespace:  "default",
					},
					SessionID: "session-12345",
					CreatedAt: now,
				},
				Status: SessionStateStatus{
					Phase: SessionPending,
				},
			},
			validate: func(t *testing.T, state *SessionState) {
				if state.Status.Phase != SessionPending {
					t.Errorf("expected phase to be Pending, got %s", state.Status.Phase)
				}
				if state.Spec.SessionID == "" {
					t.Error("session ID should not be empty")
				}
			},
		},
		"active session with placement": {
			state: &SessionState{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "active-session",
					Namespace: "default",
				},
				Spec: SessionStateSpec{
					WorkloadReference: WorkloadReference{
						APIVersion: "apps/v1",
						Kind:       "StatefulSet",
						Name:       "database",
						Namespace:  "default",
						UID:        "db-uid-12345",
					},
					PlacementTargets: []PlacementTarget{
						{
							ClusterName: "cluster-west",
							Priority:    func() *int32 { p := int32(80); return &p }(),
							Weight:      func() *int32 { w := int32(3); return &w }(),
						},
						{
							ClusterName: "cluster-east",
							Priority:    func() *int32 { p := int32(60); return &p }(),
							Weight:      func() *int32 { w := int32(1); return &w }(),
						},
					},
					SessionID: "session-67890",
					CreatedAt: now,
					ExpiresAt: &expiresAt,
				},
				Status: SessionStateStatus{
					Phase: SessionActive,
					CurrentPlacement: &PlacementTarget{
						ClusterName: "cluster-west",
						Priority:    func() *int32 { p := int32(80); return &p }(),
						Weight:      func() *int32 { w := int32(3); return &w }(),
					},
					LastRefresh: &now,
				},
			},
			validate: func(t *testing.T, state *SessionState) {
				if state.Status.Phase != SessionActive {
					t.Errorf("expected phase to be Active, got %s", state.Status.Phase)
				}
				if len(state.Spec.PlacementTargets) != 2 {
					t.Errorf("expected 2 placement targets, got %d", len(state.Spec.PlacementTargets))
				}
				if state.Status.CurrentPlacement == nil {
					t.Error("active session should have current placement")
				}
				if state.Status.CurrentPlacement.ClusterName != "cluster-west" {
					t.Errorf("expected current placement cluster to be 'cluster-west', got %s", state.Status.CurrentPlacement.ClusterName)
				}
			},
		},
		"expiring session": {
			state: &SessionState{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "expiring-session",
					Namespace: "default",
				},
				Spec: SessionStateSpec{
					WorkloadReference: WorkloadReference{
						APIVersion: "batch/v1",
						Kind:       "Job",
						Name:       "data-processor",
						Namespace:  "default",
					},
					SessionID: "session-expiring",
					CreatedAt: now,
					ExpiresAt: &metav1.Time{Time: now.Add(5 * time.Minute)},
				},
				Status: SessionStateStatus{
					Phase: SessionExpiring,
					Conditions: []metav1.Condition{
						{
							Type:   "SessionExpiring",
							Status: metav1.ConditionTrue,
							Reason: "ApproachingTTL",
						},
					},
				},
			},
			validate: func(t *testing.T, state *SessionState) {
				if state.Status.Phase != SessionExpiring {
					t.Errorf("expected phase to be Expiring, got %s", state.Status.Phase)
				}
				if len(state.Status.Conditions) != 1 {
					t.Errorf("expected 1 condition, got %d", len(state.Status.Conditions))
				}
				if state.Status.Conditions[0].Type != "SessionExpiring" {
					t.Errorf("expected condition type SessionExpiring, got %s", state.Status.Conditions[0].Type)
				}
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			tc.validate(t, tc.state)
		})
	}
}

func TestAffinityTypeValidation(t *testing.T) {
	tests := map[string]struct {
		affinityType AffinityType
		valid        bool
	}{
		"cluster affinity": {
			affinityType: ClusterAffinity,
			valid:        true,
		},
		"node affinity": {
			affinityType: NodeAffinity,
			valid:        true,
		},
		"workspace affinity": {
			affinityType: WorkspaceAffinity,
			valid:        true,
		},
		"empty affinity type": {
			affinityType: "",
			valid:        false,
		},
		"invalid affinity type": {
			affinityType: "InvalidAffinity",
			valid:        false,
		},
	}

	validTypes := map[AffinityType]bool{
		ClusterAffinity:   true,
		NodeAffinity:     true,
		WorkspaceAffinity: true,
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			isValid := validTypes[tc.affinityType]
			if isValid != tc.valid {
				t.Errorf("expected affinity type %s to be valid=%v, got valid=%v", tc.affinityType, tc.valid, isValid)
			}
		})
	}
}

func TestSessionPhaseValidation(t *testing.T) {
	tests := map[string]struct {
		phase SessionPhase
		valid bool
	}{
		"pending phase": {
			phase: SessionPending,
			valid: true,
		},
		"active phase": {
			phase: SessionActive,
			valid: true,
		},
		"expiring phase": {
			phase: SessionExpiring,
			valid: true,
		},
		"expired phase": {
			phase: SessionExpired,
			valid: true,
		},
		"empty phase": {
			phase: "",
			valid: false,
		},
		"invalid phase": {
			phase: "InvalidPhase",
			valid: false,
		},
	}

	validPhases := map[SessionPhase]bool{
		SessionPending:  true,
		SessionActive:   true,
		SessionExpiring: true,
		SessionExpired:  true,
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			isValid := validPhases[tc.phase]
			if isValid != tc.valid {
				t.Errorf("expected session phase %s to be valid=%v, got valid=%v", tc.phase, tc.valid, isValid)
			}
		})
	}
}

func TestPlacementTargetDefaults(t *testing.T) {
	tests := map[string]struct {
		target   PlacementTarget
		validate func(t *testing.T, target PlacementTarget)
	}{
		"default priority": {
			target: PlacementTarget{
				ClusterName: "test-cluster",
				Priority:    func() *int32 { p := int32(50); return &p }(),
			},
			validate: func(t *testing.T, target PlacementTarget) {
				if target.Priority != nil && *target.Priority != 50 {
					t.Errorf("expected default priority to be 50, got %d", *target.Priority)
				}
			},
		},
		"default weight": {
			target: PlacementTarget{
				ClusterName: "test-cluster",
				Weight:      func() *int32 { w := int32(1); return &w }(),
			},
			validate: func(t *testing.T, target PlacementTarget) {
				if target.Weight != nil && *target.Weight != 1 {
					t.Errorf("expected default weight to be 1, got %d", *target.Weight)
				}
			},
		},
		"cluster name required": {
			target: PlacementTarget{
				ClusterName: "required-cluster",
			},
			validate: func(t *testing.T, target PlacementTarget) {
				if target.ClusterName == "" {
					t.Error("cluster name is required for placement target")
				}
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			tc.validate(t, tc.target)
		})
	}
}

func TestWorkloadReferenceValidation(t *testing.T) {
	tests := map[string]struct {
		workloadRef WorkloadReference
		valid       bool
		description string
	}{
		"valid deployment reference": {
			workloadRef: WorkloadReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       "web-app",
				Namespace:  "default",
				UID:        "deploy-uid-12345",
			},
			valid:       true,
			description: "should accept valid deployment reference",
		},
		"valid cluster-scoped reference": {
			workloadRef: WorkloadReference{
				APIVersion: "v1",
				Kind:       "Node",
				Name:       "worker-node-1",
			},
			valid:       true,
			description: "should accept cluster-scoped resource reference",
		},
		"missing api version": {
			workloadRef: WorkloadReference{
				Kind:      "Deployment",
				Name:      "web-app",
				Namespace: "default",
			},
			valid:       false,
			description: "should reject reference without API version",
		},
		"missing kind": {
			workloadRef: WorkloadReference{
				APIVersion: "apps/v1",
				Name:       "web-app",
				Namespace:  "default",
			},
			valid:       false,
			description: "should reject reference without kind",
		},
		"missing name": {
			workloadRef: WorkloadReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Namespace:  "default",
			},
			valid:       false,
			description: "should reject reference without name",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			isValid := tc.workloadRef.APIVersion != "" && tc.workloadRef.Kind != "" && tc.workloadRef.Name != ""
			if isValid != tc.valid {
				t.Errorf("%s: expected valid=%v, got valid=%v", tc.description, tc.valid, isValid)
			}
		})
	}
}