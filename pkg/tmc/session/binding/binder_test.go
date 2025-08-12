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

package binding

import (
	"context"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
)

func TestNewBinder(t *testing.T) {
	tests := map[string]struct {
		resolver *Resolver
		wantPanic bool
	}{
		"valid resolver": {
			resolver: NewResolver(),
			wantPanic: false,
		},
		"nil resolver": {
			resolver: nil,
			wantPanic: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					if !tc.wantPanic {
						t.Errorf("NewBinder() unexpected panic: %v", r)
					}
				} else if tc.wantPanic {
					t.Error("NewBinder() expected panic but none occurred")
				}
			}()

			binder := NewBinder(tc.resolver)
			if !tc.wantPanic && binder == nil {
				t.Error("NewBinder() returned nil")
			}
		})
	}
}

func TestBinder_Bind(t *testing.T) {
	binder := NewBinder(NewResolver())
	ctx := context.Background()
	now := time.Now()

	tests := map[string]struct {
		request *BindingRequest
		wantErr bool
		wantConditions int
		wantTargets int
	}{
		"valid cluster affinity request": {
			request: &BindingRequest{
				WorkloadReference: tmcv1alpha1.WorkloadReference{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       "test-workload",
					Namespace:  "default",
				},
				Policy: &tmcv1alpha1.SessionAffinityPolicy{
					ObjectMeta: metav1.ObjectMeta{Name: "test-policy"},
					Spec: tmcv1alpha1.SessionAffinityPolicySpec{
						AffinityType: tmcv1alpha1.ClusterAffinity,
						SessionTTL:   &metav1.Duration{Duration: time.Hour},
					},
				},
				Candidates: []tmcv1alpha1.PlacementTarget{
					{
						ClusterName: "cluster-1",
						Priority:    int32Ptr(80),
						Weight:      int32Ptr(100),
					},
					{
						ClusterName: "cluster-2",
						Priority:    int32Ptr(60),
						Weight:      int32Ptr(50),
					},
				},
				Context: BindingContext{
					Namespace:   "test-ns",
					RequestTime: now,
				},
			},
			wantErr: false,
			wantConditions: 1,
			wantTargets: 1,
		},
		"valid node affinity request": {
			request: &BindingRequest{
				WorkloadReference: tmcv1alpha1.WorkloadReference{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       "test-workload",
				},
				Policy: &tmcv1alpha1.SessionAffinityPolicy{
					ObjectMeta: metav1.ObjectMeta{Name: "test-policy"},
					Spec: tmcv1alpha1.SessionAffinityPolicySpec{
						AffinityType: tmcv1alpha1.NodeAffinity,
						StickinessFactor: float64Ptr(0.7),
					},
				},
				Candidates: []tmcv1alpha1.PlacementTarget{
					{
						ClusterName: "cluster-1",
						NodeSelector: map[string]string{
							"zone": "us-west-1a",
						},
					},
				},
				Context: BindingContext{
					Namespace:   "test-ns",
					RequestTime: now,
				},
			},
			wantErr: false,
			wantConditions: 1,
			wantTargets: 1,
		},
		"workspace affinity request": {
			request: &BindingRequest{
				WorkloadReference: tmcv1alpha1.WorkloadReference{
					APIVersion: "apps/v1",
					Kind:       "StatefulSet",
					Name:       "test-workload",
				},
				Policy: &tmcv1alpha1.SessionAffinityPolicy{
					ObjectMeta: metav1.ObjectMeta{Name: "test-policy"},
					Spec: tmcv1alpha1.SessionAffinityPolicySpec{
						AffinityType: tmcv1alpha1.WorkspaceAffinity,
						SessionSelector: &tmcv1alpha1.SessionSelector{
							Namespaces: []string{"test-ns", "other-ns"},
						},
					},
				},
				Candidates: []tmcv1alpha1.PlacementTarget{
					{ClusterName: "cluster-1"},
					{ClusterName: "cluster-2"},
				},
				Context: BindingContext{
					Namespace:   "test-ns",
					RequestTime: now,
				},
			},
			wantErr: false,
			wantConditions: 1,
			wantTargets: 2, // workspace affinity can select multiple targets
		},
		"request with existing sessions": {
			request: &BindingRequest{
				WorkloadReference: tmcv1alpha1.WorkloadReference{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       "test-workload",
				},
				Policy: &tmcv1alpha1.SessionAffinityPolicy{
					ObjectMeta: metav1.ObjectMeta{Name: "test-policy"},
					Spec: tmcv1alpha1.SessionAffinityPolicySpec{
						AffinityType: tmcv1alpha1.ClusterAffinity,
						StickinessFactor: float64Ptr(0.8),
					},
				},
				Candidates: []tmcv1alpha1.PlacementTarget{
					{ClusterName: "cluster-1"},
					{ClusterName: "cluster-2"},
				},
				Context: BindingContext{
					Namespace:   "test-ns",
					RequestTime: now,
					ExistingSessions: []tmcv1alpha1.SessionState{
						{
							Spec: tmcv1alpha1.SessionStateSpec{
								SessionID: "existing-session",
							},
							Status: tmcv1alpha1.SessionStateStatus{
								Phase: tmcv1alpha1.SessionActive,
								CurrentPlacement: &tmcv1alpha1.PlacementTarget{
									ClusterName: "cluster-1",
								},
							},
						},
					},
				},
			},
			wantErr: false,
			wantConditions: 1,
			wantTargets: 1,
		},
		"nil request": {
			request: nil,
			wantErr: true,
		},
		"invalid workload name": {
			request: &BindingRequest{
				WorkloadReference: tmcv1alpha1.WorkloadReference{
					Kind: "Deployment",
					Name: "", // empty name
				},
				Context: BindingContext{
					Namespace:   "test-ns",
					RequestTime: now,
				},
			},
			wantErr: true,
		},
		"invalid workload kind": {
			request: &BindingRequest{
				WorkloadReference: tmcv1alpha1.WorkloadReference{
					Name: "test-workload",
					Kind: "", // empty kind
				},
				Context: BindingContext{
					Namespace:   "test-ns",
					RequestTime: now,
				},
			},
			wantErr: true,
		},
		"empty namespace": {
			request: &BindingRequest{
				WorkloadReference: tmcv1alpha1.WorkloadReference{
					Name: "test-workload",
					Kind: "Deployment",
				},
				Context: BindingContext{
					Namespace:   "", // empty namespace
					RequestTime: now,
				},
			},
			wantErr: true,
		},
		"zero request time": {
			request: &BindingRequest{
				WorkloadReference: tmcv1alpha1.WorkloadReference{
					Name: "test-workload",
					Kind: "Deployment",
				},
				Context: BindingContext{
					Namespace:   "test-ns",
					RequestTime: time.Time{}, // zero time
				},
			},
			wantErr: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result, err := binder.Bind(ctx, tc.request)

			if tc.wantErr {
				if err == nil {
					t.Error("Bind() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Bind() unexpected error: %v", err)
				return
			}

			if result == nil {
				t.Fatal("Bind() returned nil result")
			}

			if len(result.Conditions) != tc.wantConditions {
				t.Errorf("Bind() got %d conditions, want %d", len(result.Conditions), tc.wantConditions)
			}

			if len(result.SelectedTargets) != tc.wantTargets {
				t.Errorf("Bind() got %d targets, want %d", len(result.SelectedTargets), tc.wantTargets)
			}

			if result.SessionState == nil {
				t.Error("Bind() returned nil SessionState")
			} else {
				// Validate session state
				session := result.SessionState
				if session.Spec.WorkloadReference.Name != tc.request.WorkloadReference.Name {
					t.Errorf("SessionState workload name got %q, want %q",
						session.Spec.WorkloadReference.Name, tc.request.WorkloadReference.Name)
				}
				if session.Spec.SessionID == "" {
					t.Error("SessionState missing SessionID")
				}
				if session.Status.Phase != tmcv1alpha1.SessionPending {
					t.Errorf("SessionState phase got %q, want %q",
						session.Status.Phase, tmcv1alpha1.SessionPending)
				}
			}
		})
	}
}

func TestBinder_selectTargets(t *testing.T) {
	binder := NewBinder(NewResolver())
	ctx := context.Background()
	now := time.Now()

	tests := map[string]struct {
		request *BindingRequest
		wantTargets int
		wantReason string
	}{
		"no policy - default selection": {
			request: &BindingRequest{
				WorkloadReference: tmcv1alpha1.WorkloadReference{
					Name: "test-workload",
					Kind: "Deployment",
				},
				Candidates: []tmcv1alpha1.PlacementTarget{
					{ClusterName: "cluster-1", Priority: int32Ptr(80)},
					{ClusterName: "cluster-2", Priority: int32Ptr(60)},
				},
				Context: BindingContext{
					Namespace:   "test-ns",
					RequestTime: now,
				},
			},
			wantTargets: 1,
			wantReason: "default target selection",
		},
		"cluster affinity policy": {
			request: &BindingRequest{
				WorkloadReference: tmcv1alpha1.WorkloadReference{
					Name: "test-workload",
					Kind: "Deployment",
				},
				Policy: &tmcv1alpha1.SessionAffinityPolicy{
					Spec: tmcv1alpha1.SessionAffinityPolicySpec{
						AffinityType: tmcv1alpha1.ClusterAffinity,
					},
				},
				Candidates: []tmcv1alpha1.PlacementTarget{
					{ClusterName: "cluster-1"},
				},
				Context: BindingContext{
					Namespace:   "test-ns",
					RequestTime: now,
				},
			},
			wantTargets: 1,
			wantReason: "cluster affinity applied",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			targets, reason, err := binder.selectTargets(ctx, tc.request)
			if err != nil {
				t.Errorf("selectTargets() unexpected error: %v", err)
				return
			}

			if len(targets) != tc.wantTargets {
				t.Errorf("selectTargets() got %d targets, want %d", len(targets), tc.wantTargets)
			}

			if reason != tc.wantReason {
				t.Errorf("selectTargets() got reason %q, want %q", reason, tc.wantReason)
			}
		})
	}
}

func TestBinder_findExistingClusterTarget(t *testing.T) {
	binder := NewBinder(NewResolver())

	tests := map[string]struct {
		sessions []tmcv1alpha1.SessionState
		want     *tmcv1alpha1.PlacementTarget
	}{
		"active session with placement": {
			sessions: []tmcv1alpha1.SessionState{
				{
					Status: tmcv1alpha1.SessionStateStatus{
						Phase: tmcv1alpha1.SessionActive,
						CurrentPlacement: &tmcv1alpha1.PlacementTarget{
							ClusterName: "cluster-1",
						},
					},
				},
			},
			want: &tmcv1alpha1.PlacementTarget{
				ClusterName: "cluster-1",
			},
		},
		"pending session": {
			sessions: []tmcv1alpha1.SessionState{
				{
					Status: tmcv1alpha1.SessionStateStatus{
						Phase: tmcv1alpha1.SessionPending,
						CurrentPlacement: &tmcv1alpha1.PlacementTarget{
							ClusterName: "cluster-1",
						},
					},
				},
			},
			want: nil,
		},
		"no sessions": {
			sessions: []tmcv1alpha1.SessionState{},
			want:     nil,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result := binder.findExistingClusterTarget(tc.sessions)

			if tc.want == nil {
				if result != nil {
					t.Errorf("findExistingClusterTarget() got %v, want nil", result)
				}
			} else {
				if result == nil {
					t.Errorf("findExistingClusterTarget() got nil, want %v", tc.want)
				} else if result.ClusterName != tc.want.ClusterName {
					t.Errorf("findExistingClusterTarget() got cluster %q, want %q",
						result.ClusterName, tc.want.ClusterName)
				}
			}
		})
	}
}

func TestBinder_generateSessionID(t *testing.T) {
	binder := NewBinder(NewResolver())
	now := time.Now()
	
	workload := tmcv1alpha1.WorkloadReference{
		Kind: "Deployment",
		Name: "test-workload",
	}

	sessionID1 := binder.generateSessionID(workload, now)
	sessionID2 := binder.generateSessionID(workload, now.Add(time.Second))

	if sessionID1 == "" {
		t.Error("generateSessionID() returned empty string")
	}

	if sessionID1 == sessionID2 {
		t.Error("generateSessionID() returned same ID for different timestamps")
	}
}

func TestBinder_validateRequest(t *testing.T) {
	binder := NewBinder(NewResolver())
	now := time.Now()

	tests := map[string]struct {
		request *BindingRequest
		wantErr bool
	}{
		"valid request": {
			request: &BindingRequest{
				WorkloadReference: tmcv1alpha1.WorkloadReference{
					Name: "test-workload",
					Kind: "Deployment",
				},
				Context: BindingContext{
					Namespace:   "test-ns",
					RequestTime: now,
				},
			},
			wantErr: false,
		},
		"empty workload name": {
			request: &BindingRequest{
				WorkloadReference: tmcv1alpha1.WorkloadReference{
					Name: "",
					Kind: "Deployment",
				},
				Context: BindingContext{
					Namespace:   "test-ns",
					RequestTime: now,
				},
			},
			wantErr: true,
		},
		"empty workload kind": {
			request: &BindingRequest{
				WorkloadReference: tmcv1alpha1.WorkloadReference{
					Name: "test-workload",
					Kind: "",
				},
				Context: BindingContext{
					Namespace:   "test-ns",
					RequestTime: now,
				},
			},
			wantErr: true,
		},
		"empty namespace": {
			request: &BindingRequest{
				WorkloadReference: tmcv1alpha1.WorkloadReference{
					Name: "test-workload",
					Kind: "Deployment",
				},
				Context: BindingContext{
					Namespace:   "",
					RequestTime: now,
				},
			},
			wantErr: true,
		},
		"zero request time": {
			request: &BindingRequest{
				WorkloadReference: tmcv1alpha1.WorkloadReference{
					Name: "test-workload",
					Kind: "Deployment",
				},
				Context: BindingContext{
					Namespace:   "test-ns",
					RequestTime: time.Time{},
				},
			},
			wantErr: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := binder.validateRequest(tc.request)
			if tc.wantErr {
				if err == nil {
					t.Error("validateRequest() expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("validateRequest() unexpected error: %v", err)
				}
			}
		})
	}
}

// Helper functions for test cases

func int32Ptr(i int32) *int32 {
	return &i
}

func float64Ptr(f float64) *float64 {
	return &f
}