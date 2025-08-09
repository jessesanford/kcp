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

package placement

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
	tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
	"github.com/kcp-dev/kcp/pkg/tmc/controller"
)

func TestPlacementReconciler_ReconcileWorkloadPlacement(t *testing.T) {
	tests := map[string]struct {
		placement      *tmcv1alpha1.WorkloadPlacement
		wantRequeue    bool
		wantError      bool
		wantConditions map[conditionsv1alpha1.ConditionType]corev1.ConditionStatus
	}{
		"valid workload placement": {
			placement: &tmcv1alpha1.WorkloadPlacement{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-placement",
				},
				Spec: tmcv1alpha1.WorkloadPlacementSpec{
					WorkloadReference: tmcv1alpha1.WorkloadReference{
						Name: "my-deployment",
						Type: tmcv1alpha1.DeploymentWorkload,
					},
					ClusterSelector: &tmcv1alpha1.ClusterSelector{
						Location: "us-west-2",
					},
				},
			},
			wantRequeue: false,
			wantError:   false,
			wantConditions: map[conditionsv1alpha1.ConditionType]corev1.ConditionStatus{
				PlacementValidCondition:     corev1.ConditionTrue,
				PlacementScheduledCondition: corev1.ConditionTrue,
				PlacementReadyCondition:     corev1.ConditionTrue,
			},
		},
		"missing workload name": {
			placement: &tmcv1alpha1.WorkloadPlacement{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-placement",
				},
				Spec: tmcv1alpha1.WorkloadPlacementSpec{
					WorkloadReference: tmcv1alpha1.WorkloadReference{
						Name: "",
						Type: tmcv1alpha1.DeploymentWorkload,
					},
				},
			},
			wantRequeue: false,
			wantError:   false,
			wantConditions: map[conditionsv1alpha1.ConditionType]corev1.ConditionStatus{
				PlacementValidCondition: corev1.ConditionFalse,
				PlacementReadyCondition: corev1.ConditionFalse,
			},
		},
		"missing workload type": {
			placement: &tmcv1alpha1.WorkloadPlacement{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-placement",
				},
				Spec: tmcv1alpha1.WorkloadPlacementSpec{
					WorkloadReference: tmcv1alpha1.WorkloadReference{
						Name: "my-deployment",
						Type: "",
					},
				},
			},
			wantRequeue: false,
			wantError:   false,
			wantConditions: map[conditionsv1alpha1.ConditionType]corev1.ConditionStatus{
				PlacementValidCondition: corev1.ConditionFalse,
				PlacementReadyCondition: corev1.ConditionFalse,
			},
		},
		"placement without cluster selector": {
			placement: &tmcv1alpha1.WorkloadPlacement{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-placement",
				},
				Spec: tmcv1alpha1.WorkloadPlacementSpec{
					WorkloadReference: tmcv1alpha1.WorkloadReference{
						Name: "my-deployment",
						Type: tmcv1alpha1.DeploymentWorkload,
					},
					// ClusterSelector is nil - should still be valid
				},
			},
			wantRequeue: false,
			wantError:   false,
			wantConditions: map[conditionsv1alpha1.ConditionType]corev1.ConditionStatus{
				PlacementValidCondition:     corev1.ConditionTrue,
				PlacementScheduledCondition: corev1.ConditionTrue,
				PlacementReadyCondition:     corev1.ConditionTrue,
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Create a mock client for testing
			mockClient := &mockClient{}
			reconciler := NewPlacementReconciler(mockClient)

			ctx := context.Background()
			requeue, err := reconciler.ReconcileWorkloadPlacement(ctx, tc.placement)

			// Check return values
			if requeue != tc.wantRequeue {
				t.Errorf("expected requeue=%v, got %v", tc.wantRequeue, requeue)
			}

			if (err != nil) != tc.wantError {
				t.Errorf("expected error=%v, got %v", tc.wantError, err != nil)
			}

			// Check conditions
			for expectedType, expectedStatus := range tc.wantConditions {
				found := false
				for _, condition := range tc.placement.Status.Conditions {
					if condition.Type == expectedType {
						found = true
						if condition.Status != expectedStatus {
							t.Errorf("expected condition %s status=%s, got %s", 
								expectedType, expectedStatus, condition.Status)
						}
						if condition.LastTransitionTime.IsZero() {
							t.Errorf("expected condition %s to have LastTransitionTime set", expectedType)
						}
						break
					}
				}
				if !found {
					t.Errorf("expected condition %s not found", expectedType)
				}
			}
		})
	}
}

func TestPlacementReconciler_validatePlacementPolicy(t *testing.T) {
	tests := map[string]struct {
		placement *tmcv1alpha1.WorkloadPlacement
		wantError bool
	}{
		"valid placement policy": {
			placement: &tmcv1alpha1.WorkloadPlacement{
				Spec: tmcv1alpha1.WorkloadPlacementSpec{
					WorkloadReference: tmcv1alpha1.WorkloadReference{
						Name: "my-deployment",
						Type: tmcv1alpha1.DeploymentWorkload,
					},
					ClusterSelector: &tmcv1alpha1.ClusterSelector{
						Location: "us-west-2",
					},
				},
			},
			wantError: false,
		},
		"missing workload name": {
			placement: &tmcv1alpha1.WorkloadPlacement{
				Spec: tmcv1alpha1.WorkloadPlacementSpec{
					WorkloadReference: tmcv1alpha1.WorkloadReference{
						Name: "",
						Type: tmcv1alpha1.DeploymentWorkload,
					},
				},
			},
			wantError: true,
		},
		"missing workload type": {
			placement: &tmcv1alpha1.WorkloadPlacement{
				Spec: tmcv1alpha1.WorkloadPlacementSpec{
					WorkloadReference: tmcv1alpha1.WorkloadReference{
						Name: "my-deployment",
						Type: "",
					},
				},
			},
			wantError: true,
		},
		"nil cluster selector is valid": {
			placement: &tmcv1alpha1.WorkloadPlacement{
				Spec: tmcv1alpha1.WorkloadPlacementSpec{
					WorkloadReference: tmcv1alpha1.WorkloadReference{
						Name: "my-deployment",
						Type: tmcv1alpha1.DeploymentWorkload,
					},
					ClusterSelector: nil,
				},
			},
			wantError: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			mockClient := &mockClient{}
			reconciler := NewPlacementReconciler(mockClient)

			ctx := context.Background()
			err := reconciler.validatePlacementPolicy(ctx, tc.placement)

			if (err != nil) != tc.wantError {
				t.Errorf("expected error=%v, got %v", tc.wantError, err != nil)
			}
		})
	}
}

func TestPlacementReconciler_validateClusterSelector(t *testing.T) {
	tests := map[string]struct {
		selector  *tmcv1alpha1.ClusterSelector
		wantError bool
	}{
		"valid location selector": {
			selector: &tmcv1alpha1.ClusterSelector{
				Location: "us-west-2",
			},
			wantError: false,
		},
		"empty location is invalid": {
			selector: &tmcv1alpha1.ClusterSelector{
				Location: "",
			},
			wantError: true,
		},
		"no location specified is valid": {
			selector: &tmcv1alpha1.ClusterSelector{
				// No location specified - uses all clusters
			},
			wantError: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			mockClient := &mockClient{}
			reconciler := NewPlacementReconciler(mockClient)

			ctx := context.Background()
			err := reconciler.validateClusterSelector(ctx, tc.selector)

			if (err != nil) != tc.wantError {
				t.Errorf("expected error=%v, got %v", tc.wantError, err != nil)
			}
		})
	}
}

// mockClient provides a mock implementation of the controller.Client interface
// for testing purposes.
type mockClient struct{}

func (c *mockClient) Get(ctx context.Context, key string, obj interface{}) error {
	// Mock implementation - not needed for current tests
	return nil
}

func (c *mockClient) Update(ctx context.Context, obj interface{}) error {
	// Mock implementation - not needed for current tests
	return nil
}

func (c *mockClient) UpdateStatus(ctx context.Context, obj interface{}) error {
	// Mock implementation - not needed for current tests
	return nil
}