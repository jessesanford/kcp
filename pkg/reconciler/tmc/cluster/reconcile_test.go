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
	"fmt"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
)

func TestReconcilerHealthChecks(t *testing.T) {
	tests := map[string]struct {
		cluster           *tmcv1alpha1.ClusterRegistration
		mockClientError   bool
		expectedCondition conditionsv1alpha1.ConditionStatus
		expectError       bool
	}{
		"healthy cluster": {
			cluster: createTestCluster("healthy-cluster", "https://healthy.example.com"),
			mockClientError:   false,
			expectedCondition: conditionsv1alpha1.ConditionTrue,
			expectError:       false,
		},
		"unhealthy cluster": {
			cluster: createTestCluster("unhealthy-cluster", "https://unhealthy.example.com"),
			mockClientError:   true,
			expectedCondition: conditionsv1alpha1.ConditionFalse,
			expectError:       true,
		},
		"cluster with TLS config": {
			cluster: createTestClusterWithTLS("tls-cluster", "https://tls.example.com", true),
			mockClientError:   false,
			expectedCondition: conditionsv1alpha1.ConditionTrue,
			expectError:       false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			
			// Create test reconciler
			controller := &Controller{
				commitClusterRegistration: mockCommitFunc,
			}
			reconciler := &reconciler{controller: controller}

			// Override getClusterClient for testing
			originalGetClusterClient := reconciler.getClusterClient
			reconciler.getClusterClient = func(cluster *tmcv1alpha1.ClusterRegistration) (kubernetes.Interface, error) {
				if tc.mockClientError {
					return nil, fmt.Errorf("mock client error")
				}
				return fake.NewSimpleClientset(), nil
			}
			defer func() {
				reconciler.getClusterClient = originalGetClusterClient
			}()

			// Test connectivity check
			err := reconciler.ensureClusterConnectivity(ctx, tc.cluster)
			if tc.expectError && err == nil {
				t.Error("expected connectivity error but got none")
			} else if !tc.expectError && err != nil {
				t.Errorf("expected no connectivity error but got: %v", err)
			}

			// Test access validation
			err = reconciler.validateClusterAccess(ctx, tc.cluster)
			if tc.expectError && err == nil {
				t.Error("expected access validation error but got none")
			} else if !tc.expectError && err != nil {
				t.Errorf("expected no access validation error but got: %v", err)
			}
		})
	}
}

func TestReconcilerStatusUpdates(t *testing.T) {
	tests := map[string]struct {
		cluster       *tmcv1alpha1.ClusterRegistration
		conditionType string
		status        conditionsv1alpha1.ConditionStatus
		reason        string
		message       string
	}{
		"ready condition": {
			cluster:       createTestCluster("test-cluster", "https://test.example.com"),
			conditionType: ClusterReadyCondition,
			status:        conditionsv1alpha1.ConditionTrue,
			reason:        "ClusterReady",
			message:       "Cluster is ready",
		},
		"connectivity condition": {
			cluster:       createTestCluster("test-cluster", "https://test.example.com"),
			conditionType: ClusterConnectivityCondition,
			status:        conditionsv1alpha1.ConditionTrue,
			reason:        "ConnectivityHealthy",
			message:       "API server accessible",
		},
		"health condition failure": {
			cluster:       createTestCluster("test-cluster", "https://test.example.com"),
			conditionType: ClusterHealthCondition,
			status:        conditionsv1alpha1.ConditionFalse,
			reason:        "HealthCheckFailed",
			message:       "Health check failed",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			
			// Create test reconciler with mock commit function
			controller := &Controller{
				commitClusterRegistration: mockCommitFunc,
			}
			reconciler := &reconciler{controller: controller}

			// Test status update
			err := reconciler.updateClusterStatus(ctx, tc.cluster, tc.conditionType, tc.status, tc.reason, tc.message)
			if err != nil {
				t.Errorf("unexpected error updating status: %v", err)
			}
		})
	}
}

func TestReconcilerClusterDeletion(t *testing.T) {
	tests := map[string]struct {
		cluster          *tmcv1alpha1.ClusterRegistration
		expectCleanup    bool
		expectedStatus   reconcileStatus
	}{
		"successful deletion": {
			cluster:        createDeletingCluster("deleting-cluster", "https://deleting.example.com"),
			expectCleanup:  true,
			expectedStatus: reconcileStatusStop,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			
			// Create test reconciler
			controller := &Controller{
				commitClusterRegistration: mockCommitFunc,
			}
			reconciler := &reconciler{controller: controller}

			// Test deletion handling
			status, err := reconciler.handleClusterDeletion(ctx, tc.cluster)
			if err != nil {
				t.Errorf("unexpected error handling deletion: %v", err)
			}

			if status != tc.expectedStatus {
				t.Errorf("expected status %v, got %v", tc.expectedStatus, status)
			}
		})
	}
}

func TestReconcilerFullReconciliation(t *testing.T) {
	tests := map[string]struct {
		cluster        *tmcv1alpha1.ClusterRegistration
		mockError      bool
		expectedStatus reconcileStatus
		expectError    bool
	}{
		"successful reconciliation": {
			cluster:        createTestCluster("success-cluster", "https://success.example.com"),
			mockError:      false,
			expectedStatus: reconcileStatusContinue,
			expectError:    false,
		},
		"failed reconciliation": {
			cluster:        createTestCluster("fail-cluster", "https://fail.example.com"),
			mockError:      true,
			expectedStatus: reconcileStatusContinue,
			expectError:    true,
		},
		"deletion reconciliation": {
			cluster:        createDeletingCluster("delete-cluster", "https://delete.example.com"),
			mockError:      false,
			expectedStatus: reconcileStatusStop,
			expectError:    false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			
			// Create test reconciler
			controller := &Controller{
				commitClusterRegistration: mockCommitFunc,
			}
			reconciler := &reconciler{controller: controller}

			// Override getClusterClient for testing
			reconciler.getClusterClient = func(cluster *tmcv1alpha1.ClusterRegistration) (kubernetes.Interface, error) {
				if tc.mockError && tc.cluster.GetDeletionTimestamp() == nil {
					return nil, fmt.Errorf("mock client error")
				}
				return fake.NewSimpleClientset(), nil
			}

			// Test full reconciliation
			status, err := reconciler.reconcileCluster(ctx, tc.cluster)
			
			if tc.expectError && err == nil {
				t.Error("expected reconciliation error but got none")
			} else if !tc.expectError && err != nil {
				t.Errorf("expected no reconciliation error but got: %v", err)
			}

			if status != tc.expectedStatus {
				t.Errorf("expected status %v, got %v", tc.expectedStatus, status)
			}
		})
	}
}

// Helper functions for creating test objects
func createTestCluster(name, serverURL string) *tmcv1alpha1.ClusterRegistration {
	return &tmcv1alpha1.ClusterRegistration{
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Generation: 1,
			Annotations: map[string]string{
				"kcp.io/cluster": "root:test",
			},
		},
		Spec: tmcv1alpha1.ClusterRegistrationSpec{
			Location: "us-west-2",
			ClusterEndpoint: tmcv1alpha1.ClusterEndpoint{
				ServerURL: serverURL,
			},
		},
		Status: tmcv1alpha1.ClusterRegistrationStatus{
			ObservedGeneration: 0,
			Conditions:         []conditionsv1alpha1.Condition{},
		},
	}
}

func createTestClusterWithTLS(name, serverURL string, insecureSkipVerify bool) *tmcv1alpha1.ClusterRegistration {
	cluster := createTestCluster(name, serverURL)
	cluster.Spec.ClusterEndpoint.TLSConfig = &tmcv1alpha1.TLSConfig{
		InsecureSkipVerify: insecureSkipVerify,
	}
	return cluster
}

func createDeletingCluster(name, serverURL string) *tmcv1alpha1.ClusterRegistration {
	cluster := createTestCluster(name, serverURL)
	now := metav1.NewTime(time.Now())
	cluster.DeletionTimestamp = &now
	return cluster
}

// Mock commit function for testing
func mockCommitFunc(ctx context.Context, old, new *ClusterRegistrationResource) error {
	// Simulate successful commit
	return nil
}