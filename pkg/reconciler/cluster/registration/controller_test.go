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

package registration

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
	tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
)

func TestClusterRegistrationController_reconcile(t *testing.T) {
	tests := map[string]struct {
		cluster         *tmcv1alpha1.ClusterRegistration
		wantRequeue     bool
		wantError       bool
		wantConditions  []conditionsv1alpha1.Condition
		wantHeartbeat   bool
	}{
		"healthy cluster registration with valid endpoint": {
			cluster: &tmcv1alpha1.ClusterRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: tmcv1alpha1.ClusterRegistrationSpec{
					Location: "us-west-2",
					ClusterEndpoint: tmcv1alpha1.ClusterEndpoint{
						ServerURL: "https://api.test-cluster.example.com",
					},
				},
			},
			wantRequeue: false,
			wantError:   false,
			wantConditions: []conditionsv1alpha1.Condition{
				{
					Type:    ClusterRegistrationConnectedCondition,
					Status:  corev1.ConditionTrue,
					Reason:  "ConnectivityTestSucceeded",
					Message: "Cluster is reachable and responding",
				},
				{
					Type:    ClusterRegistrationReadyCondition,
					Status:  corev1.ConditionTrue,
					Reason:  "ClusterRegistrationReady",
					Message: "Cluster registration is ready for workload placement",
				},
			},
			wantHeartbeat: true,
		},
		"invalid endpoint with empty server URL": {
			cluster: &tmcv1alpha1.ClusterRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-invalid",
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
			wantConditions: []conditionsv1alpha1.Condition{
				{
					Type:   ClusterRegistrationConnectedCondition,
					Status: corev1.ConditionFalse,
					Reason: "EndpointValidationFailed",
				},
				{
					Type:   ClusterRegistrationReadyCondition,
					Status: corev1.ConditionFalse,
					Reason: "EndpointValidationFailed",
				},
			},
			wantHeartbeat: false,
		},
		"invalid endpoint with non-HTTPS URL": {
			cluster: &tmcv1alpha1.ClusterRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-http",
				},
				Spec: tmcv1alpha1.ClusterRegistrationSpec{
					Location: "us-east-1",
					ClusterEndpoint: tmcv1alpha1.ClusterEndpoint{
						ServerURL: "http://api.test-cluster.example.com",
					},
				},
			},
			wantRequeue: false,
			wantError:   false,
			wantConditions: []conditionsv1alpha1.Condition{
				{
					Type:   ClusterRegistrationConnectedCondition,
					Status: corev1.ConditionFalse,
					Reason: "EndpointValidationFailed",
				},
				{
					Type:   ClusterRegistrationReadyCondition,
					Status: corev1.ConditionFalse,
					Reason: "EndpointValidationFailed",
				},
			},
			wantHeartbeat: false,
		},
		"cluster with TLS config": {
			cluster: &tmcv1alpha1.ClusterRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-tls",
				},
				Spec: tmcv1alpha1.ClusterRegistrationSpec{
					Location: "eu-west-1",
					ClusterEndpoint: tmcv1alpha1.ClusterEndpoint{
						ServerURL: "https://api.cluster-tls.example.com",
						TLSConfig: &tmcv1alpha1.TLSConfig{
							InsecureSkipVerify: false,
						},
					},
				},
			},
			wantRequeue: false,
			wantError:   false,
			wantConditions: []conditionsv1alpha1.Condition{
				{
					Type:   ClusterRegistrationConnectedCondition,
					Status: corev1.ConditionTrue,
					Reason: "ConnectivityTestSucceeded",
				},
				{
					Type:   ClusterRegistrationReadyCondition,
					Status: corev1.ConditionTrue,
					Reason: "ClusterRegistrationReady",
				},
			},
			wantHeartbeat: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Create a minimal controller for testing
			controller := &Controller{}
			
			ctx := context.Background()
			
			// Record the time before reconciliation to check heartbeat
			beforeReconcile := time.Now()
			
			// Run reconciliation
			requeue, err := controller.reconcile(ctx, tc.cluster)
			
			// Check error expectation
			if tc.wantError && err == nil {
				t.Errorf("Expected error, but got none")
			}
			if !tc.wantError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			
			// Check requeue expectation
			if requeue != tc.wantRequeue {
				t.Errorf("Expected requeue=%v, got %v", tc.wantRequeue, requeue)
			}
			
			// Check conditions
			for _, wantCondition := range tc.wantConditions {
				condition := getCondition(tc.cluster.Status.Conditions, wantCondition.Type)
				if condition == nil {
					t.Errorf("Expected condition %s not found", wantCondition.Type)
					continue
				}
				
				if condition.Status != wantCondition.Status {
					t.Errorf("Condition %s: expected status %s, got %s",
						wantCondition.Type, wantCondition.Status, condition.Status)
				}
				
				if condition.Reason != wantCondition.Reason {
					t.Errorf("Condition %s: expected reason %s, got %s",
						wantCondition.Type, wantCondition.Reason, condition.Reason)
				}
			}
			
			// Check heartbeat
			if tc.wantHeartbeat {
				if tc.cluster.Status.LastHeartbeat == nil {
					t.Errorf("Expected heartbeat to be set, but it was nil")
				} else if tc.cluster.Status.LastHeartbeat.Time.Before(beforeReconcile) {
					t.Errorf("Expected heartbeat to be updated during reconciliation")
				}
			} else {
				if tc.cluster.Status.LastHeartbeat != nil {
					t.Errorf("Expected heartbeat to be nil for failed registration")
				}
			}
		})
	}
}

func TestValidateClusterEndpoint(t *testing.T) {
	tests := map[string]struct {
		endpoint  tmcv1alpha1.ClusterEndpoint
		wantError bool
	}{
		"valid HTTPS endpoint": {
			endpoint: tmcv1alpha1.ClusterEndpoint{
				ServerURL: "https://api.cluster.example.com",
			},
			wantError: false,
		},
		"empty server URL": {
			endpoint: tmcv1alpha1.ClusterEndpoint{
				ServerURL: "",
			},
			wantError: true,
		},
		"HTTP endpoint (should fail)": {
			endpoint: tmcv1alpha1.ClusterEndpoint{
				ServerURL: "http://api.cluster.example.com",
			},
			wantError: true,
		},
		"malformed URL": {
			endpoint: tmcv1alpha1.ClusterEndpoint{
				ServerURL: "not-a-valid-url",
			},
			wantError: true,
		},
		"URL without host": {
			endpoint: tmcv1alpha1.ClusterEndpoint{
				ServerURL: "https://",
			},
			wantError: true,
		},
	}

	controller := &Controller{}
	ctx := context.Background()

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			cluster := &tmcv1alpha1.ClusterRegistration{
				Spec: tmcv1alpha1.ClusterRegistrationSpec{
					ClusterEndpoint: tc.endpoint,
				},
			}
			
			err := controller.validateClusterEndpoint(ctx, cluster)
			
			if tc.wantError && err == nil {
				t.Errorf("Expected validation error, but got none")
			}
			if !tc.wantError && err != nil {
				t.Errorf("Unexpected validation error: %v", err)
			}
		})
	}
}

// getCondition finds and returns the condition with the given type
func getCondition(conditions conditionsv1alpha1.Conditions, conditionType conditionsv1alpha1.ConditionType) *conditionsv1alpha1.Condition {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			return &condition
		}
	}
	return nil
}