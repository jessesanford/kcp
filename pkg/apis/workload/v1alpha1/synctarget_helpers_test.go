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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
)

// TestSyncTarget_SetCondition tests the helper function for setting conditions
func TestSyncTarget_SetCondition(t *testing.T) {
	tests := map[string]struct {
		initial  *SyncTarget
		condType string
		status   corev1.ConditionStatus
		reason   string
		message  string
		wantLen  int
	}{
		"new condition": {
			initial:  &SyncTarget{},
			condType: string(SyncTargetReady),
			status:   corev1.ConditionTrue,
			reason:   "Ready",
			message:  "SyncTarget is ready",
			wantLen:  1,
		},
		"update existing condition": {
			initial: &SyncTarget{
				Status: SyncTargetStatus{
					Conditions: conditionsv1alpha1.Conditions{
						{Type: SyncTargetReady, Status: corev1.ConditionFalse, Reason: "NotReady"},
					},
				},
			},
			condType: string(SyncTargetReady),
			status:   corev1.ConditionTrue,
			reason:   "Ready",
			message:  "Now ready",
			wantLen:  1,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			tc.initial.SetCondition(conditionsv1alpha1.ConditionType(tc.condType), tc.status, tc.reason, tc.message)

			if len(tc.initial.Status.Conditions) != tc.wantLen {
				t.Errorf("expected %d conditions, got %d", tc.wantLen, len(tc.initial.Status.Conditions))
			}

			cond := tc.initial.GetCondition(conditionsv1alpha1.ConditionType(tc.condType))
			if cond == nil {
				t.Fatal("expected condition to exist")
			}
			if cond.Status != tc.status {
				t.Errorf("expected status %v, got %v", tc.status, cond.Status)
			}
		})
	}
}

// TestSyncTarget_IsReady tests the helper function for checking readiness
func TestSyncTarget_IsReady(t *testing.T) {
	tests := map[string]struct {
		target *SyncTarget
		want   bool
	}{
		"no conditions": {
			target: &SyncTarget{},
			want:   false,
		},
		"ready condition true": {
			target: &SyncTarget{
				Status: SyncTargetStatus{
					Conditions: conditionsv1alpha1.Conditions{
						{Type: SyncTargetReady, Status: corev1.ConditionTrue},
					},
				},
			},
			want: true,
		},
		"ready condition false": {
			target: &SyncTarget{
				Status: SyncTargetStatus{
					Conditions: conditionsv1alpha1.Conditions{
						{Type: SyncTargetReady, Status: corev1.ConditionFalse},
					},
				},
			},
			want: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if got := tc.target.IsReady(); got != tc.want {
				t.Errorf("IsReady() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestSyncTarget_HasSufficientCapacity tests the capacity helper function
func TestSyncTarget_HasSufficientCapacity(t *testing.T) {
	tests := map[string]struct {
		target    *SyncTarget
		requested *ResourceCapacity
		want      bool
	}{
		"nil requested capacity": {
			target:    &SyncTarget{},
			requested: nil,
			want:      true,
		},
		"no allocatable capacity": {
			target:    &SyncTarget{},
			requested: &ResourceCapacity{CPU: resource.NewQuantity(1, resource.DecimalSI)},
			want:      true, // unlimited capacity assumed
		},
		"sufficient capacity": {
			target: &SyncTarget{
				Status: SyncTargetStatus{
					Allocatable: ResourceCapacity{
						CPU: resource.NewQuantity(10, resource.DecimalSI),
					},
					Allocated: ResourceCapacity{
						CPU: resource.NewQuantity(2, resource.DecimalSI),
					},
				},
			},
			requested: &ResourceCapacity{CPU: resource.NewQuantity(5, resource.DecimalSI)},
			want:      true,
		},
		"insufficient capacity": {
			target: &SyncTarget{
				Status: SyncTargetStatus{
					Allocatable: ResourceCapacity{
						CPU: resource.NewQuantity(10, resource.DecimalSI),
					},
					Allocated: ResourceCapacity{
						CPU: resource.NewQuantity(8, resource.DecimalSI),
					},
				},
			},
			requested: &ResourceCapacity{CPU: resource.NewQuantity(5, resource.DecimalSI)},
			want:      false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if got := tc.target.HasSufficientCapacity(tc.requested); got != tc.want {
				t.Errorf("HasSufficientCapacity() = %v, want %v", got, tc.want)
			}
		})
	}
}