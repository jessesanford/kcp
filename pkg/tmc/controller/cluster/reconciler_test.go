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

package cluster

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
	tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
	"github.com/kcp-dev/kcp/pkg/tmc/controller"
)

func TestClusterReconciler_ReconcileClusterRegistration(t *testing.T) {
	tests := map[string]struct {
		cluster         *tmcv1alpha1.ClusterRegistration
		wantRequeue     bool
		wantError       bool
		wantConditions  map[conditionsv1alpha1.ConditionType]corev1.ConditionStatus
	}{
		"valid cluster registration": {
			cluster: &tmcv1alpha1.ClusterRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: tmcv1alpha1.ClusterRegistrationSpec{
					Location: "us-west-2",
					ClusterEndpoint: tmcv1alpha1.ClusterEndpoint{
						ServerURL: "https://cluster.example.com:6443",
					},
				},
			},
			wantRequeue: false,
			wantError:   false,
			wantConditions: map[conditionsv1alpha1.ConditionType]corev1.ConditionStatus{
				ClusterValidatedCondition: corev1.ConditionTrue,
				ClusterConnectedCondition: corev1.ConditionTrue,
				ClusterReadyCondition:     corev1.ConditionTrue,
			},
		},
		"invalid server URL": {
			cluster: &tmcv1alpha1.ClusterRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: tmcv1alpha1.ClusterRegistrationSpec{
					Location: "us-west-2",
					ClusterEndpoint: tmcv1alpha1.ClusterEndpoint{
						ServerURL: "http://insecure.example.com:6443",
					},
				},
			},
			wantRequeue: false,
			wantError:   false,
			wantConditions: map[conditionsv1alpha1.ConditionType]corev1.ConditionStatus{
				ClusterValidatedCondition: corev1.ConditionFalse,
				ClusterReadyCondition:     corev1.ConditionFalse,
			},
		},
		"missing server URL": {
			cluster: &tmcv1alpha1.ClusterRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: tmcv1alpha1.ClusterRegistrationSpec{
					Location: "us-west-2",
					ClusterEndpoint: tmcv1alpha1.ClusterEndpoint{
						ServerURL: "",
					},
				},
			},
			wantRequeue: false,
			wantError:   false,
			wantConditions: map[conditionsv1alpha1.ConditionType]corev1.ConditionStatus{
				ClusterValidatedCondition: corev1.ConditionFalse,
				ClusterReadyCondition:     corev1.ConditionFalse,
			},
		},
		"missing location": {
			cluster: &tmcv1alpha1.ClusterRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: tmcv1alpha1.ClusterRegistrationSpec{
					Location: "",
					ClusterEndpoint: tmcv1alpha1.ClusterEndpoint{
						ServerURL: "https://cluster.example.com:6443",
					},
				},
			},
			wantRequeue: false,
			wantError:   false,
			wantConditions: map[conditionsv1alpha1.ConditionType]corev1.ConditionStatus{
				ClusterValidatedCondition: corev1.ConditionFalse,
				ClusterReadyCondition:     corev1.ConditionFalse,
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Create a mock client for testing
			mockClient := &mockClient{}
			reconciler := NewClusterReconciler(mockClient)

			ctx := context.Background()
			requeue, err := reconciler.ReconcileClusterRegistration(ctx, tc.cluster)

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
				for _, condition := range tc.cluster.Status.Conditions {
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

			// Check heartbeat timestamp for successful cases
			if tc.wantConditions[ClusterReadyCondition] == corev1.ConditionTrue {
				if tc.cluster.Status.LastHeartbeat == nil {
					t.Error("expected LastHeartbeat to be set for ready cluster")
				}
			}
		})
	}
}

func TestClusterReconciler_validateClusterEndpoint(t *testing.T) {
	tests := map[string]struct {
		cluster   *tmcv1alpha1.ClusterRegistration
		wantError bool
	}{
		"valid HTTPS URL": {
			cluster: &tmcv1alpha1.ClusterRegistration{
				Spec: tmcv1alpha1.ClusterRegistrationSpec{
					Location: "us-west-2",
					ClusterEndpoint: tmcv1alpha1.ClusterEndpoint{
						ServerURL: "https://cluster.example.com:6443",
					},
				},
			},
			wantError: false,
		},
		"invalid HTTP URL": {
			cluster: &tmcv1alpha1.ClusterRegistration{
				Spec: tmcv1alpha1.ClusterRegistrationSpec{
					Location: "us-west-2",
					ClusterEndpoint: tmcv1alpha1.ClusterEndpoint{
						ServerURL: "http://cluster.example.com:6443",
					},
				},
			},
			wantError: true,
		},
		"empty URL": {
			cluster: &tmcv1alpha1.ClusterRegistration{
				Spec: tmcv1alpha1.ClusterRegistrationSpec{
					Location: "us-west-2",
					ClusterEndpoint: tmcv1alpha1.ClusterEndpoint{
						ServerURL: "",
					},
				},
			},
			wantError: true,
		},
		"malformed URL": {
			cluster: &tmcv1alpha1.ClusterRegistration{
				Spec: tmcv1alpha1.ClusterRegistrationSpec{
					Location: "us-west-2",
					ClusterEndpoint: tmcv1alpha1.ClusterEndpoint{
						ServerURL: "not-a-url",
					},
				},
			},
			wantError: true,
		},
		"missing location": {
			cluster: &tmcv1alpha1.ClusterRegistration{
				Spec: tmcv1alpha1.ClusterRegistrationSpec{
					Location: "",
					ClusterEndpoint: tmcv1alpha1.ClusterEndpoint{
						ServerURL: "https://cluster.example.com:6443",
					},
				},
			},
			wantError: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			mockClient := &mockClient{}
			reconciler := NewClusterReconciler(mockClient)

			ctx := context.Background()
			err := reconciler.validateClusterEndpoint(ctx, tc.cluster)

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