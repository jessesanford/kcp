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

package integration

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/kcp-dev/logicalcluster/v3"

	e2eframework "github.com/kcp-dev/kcp/test/e2e/framework"
	integrationframework "github.com/kcp-dev/kcp/test/e2e/integration/framework"
)

func TestTMCClusterRegistrationLifecycle(t *testing.T) {
	t.Parallel()
	e2eframework.Suite(t, "integration")

	parentWorkspace := logicalcluster.Name("root:default")
	
	// Create test environment
	env, err := integrationframework.NewTestEnvironment(t, "TMCClusterRegistrationLifecycle", parentWorkspace)
	require.NoError(t, err, "Failed to create test environment")

	tests := map[string]struct {
		clusterName     string
		location        string
		expectReady     bool
		expectedPhases  []string
		testNegative    bool
	}{
		"successful cluster registration": {
			clusterName:    "test-cluster-1",
			location:       "us-west-2",
			expectReady:    true,
			expectedPhases: []string{"Pending", "Ready"},
			testNegative:   false,
		},
		"cluster registration with invalid location": {
			clusterName:    "test-cluster-invalid",
			location:       "",
			expectReady:    false,
			expectedPhases: []string{"Pending", "Failed"},
			testNegative:   true,
		},
		"cluster registration in different region": {
			clusterName:    "test-cluster-east",
			location:       "us-east-1",
			expectReady:    true,
			expectedPhases: []string{"Pending", "Ready"},
			testNegative:   false,
		},
	}

	for testName, tc := range tests {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()
			
			ctx := env.Context()
			workspaceCluster := env.WorkspaceCluster()
			
			// Create test namespace
			namespace, err := env.CreateTestNamespace("cluster-test")
			require.NoError(t, err, "Failed to create test namespace")
			
			// Test cluster registration creation
			t.Logf("Testing cluster registration for %s in location %s", tc.clusterName, tc.location)
			
			clusterReg := createMockClusterRegistration(tc.clusterName, tc.location, namespace.Name)
			
			// Create cluster registration using dynamic client
			gvr := schema.GroupVersionResource{
				Group:    "tmc.kcp.io",
				Version:  "v1alpha1", 
				Resource: "clusterregistrations",
			}
			
			dynamicClient := env.TestClient.DynamicFor(workspaceCluster, gvr)
			
			createdCluster, err := dynamicClient.Create(ctx, clusterReg, metav1.CreateOptions{})
			if tc.testNegative {
				if err == nil {
					t.Errorf("Expected error for invalid cluster registration, but creation succeeded")
				} else {
					t.Logf("Expected error occurred: %v", err)
					return // Skip further testing for negative cases
				}
			} else {
				require.NoError(t, err, "Failed to create cluster registration")
				require.NotNil(t, createdCluster, "Created cluster registration should not be nil")
			}
			
			// Test cluster lifecycle phases
			t.Logf("Validating cluster registration phases for %s", tc.clusterName)
			
			// Wait for expected phases
			env.Eventually(func() (bool, string) {
				cluster, err := dynamicClient.Get(ctx, tc.clusterName, metav1.GetOptions{})
				if err != nil {
					return false, fmt.Sprintf("failed to get cluster: %v", err)
				}
				
				phase, found, err := unstructured.NestedString(cluster.Object, "status", "phase")
				if err != nil || !found {
					return false, "cluster phase not found in status"
				}
				
				// Check if current phase is expected
				for _, expectedPhase := range tc.expectedPhases {
					if phase == expectedPhase {
						if phase == "Ready" && tc.expectReady {
							return true, ""
						} else if phase == "Failed" && !tc.expectReady {
							return true, ""
						}
					}
				}
				
				return false, fmt.Sprintf("cluster in phase %s, waiting for expected phase", phase)
			}, fmt.Sprintf("cluster %s to reach expected phase", tc.clusterName))
			
			// Test cluster status conditions
			t.Logf("Validating cluster registration conditions for %s", tc.clusterName)
			
			cluster, err := dynamicClient.Get(ctx, tc.clusterName, metav1.GetOptions{})
			require.NoError(t, err, "Failed to get cluster for condition validation")
			
			conditions, found, err := unstructured.NestedSlice(cluster.Object, "status", "conditions")
			require.NoError(t, err, "Failed to get conditions from cluster status")
			require.True(t, found, "Conditions should be present in cluster status")
			require.NotEmpty(t, conditions, "Cluster should have status conditions")
			
			// Validate specific conditions based on expected outcome
			hasReadyCondition := false
			for _, condition := range conditions {
				conditionMap, ok := condition.(map[string]interface{})
				require.True(t, ok, "Condition should be a map")
				
				conditionType, found, err := unstructured.NestedString(conditionMap, "type")
				require.NoError(t, err, "Failed to get condition type")
				require.True(t, found, "Condition type should be present")
				
				if conditionType == "Ready" {
					hasReadyCondition = true
					status, found, err := unstructured.NestedString(conditionMap, "status")
					require.NoError(t, err, "Failed to get condition status")
					require.True(t, found, "Condition status should be present")
					
					if tc.expectReady {
						require.Equal(t, "True", status, "Ready condition should be True for successful registration")
					} else {
						require.Equal(t, "False", status, "Ready condition should be False for failed registration")
					}
				}
			}
			
			require.True(t, hasReadyCondition, "Cluster should have a Ready condition")
			
			// Test cluster deletion
			t.Logf("Testing cluster registration deletion for %s", tc.clusterName)
			
			err = dynamicClient.Delete(ctx, tc.clusterName, metav1.DeleteOptions{})
			require.NoError(t, err, "Failed to delete cluster registration")
			
			// Verify cluster is deleted
			env.Eventually(func() (bool, string) {
				_, err := dynamicClient.Get(ctx, tc.clusterName, metav1.GetOptions{})
				if err != nil {
					// Expect NotFound error
					return true, ""
				}
				return false, "cluster still exists"
			}, fmt.Sprintf("cluster %s to be deleted", tc.clusterName))
			
			t.Logf("Successfully completed cluster lifecycle test for %s", tc.clusterName)
		})
	}
}

func TestTMCClusterRegistrationValidation(t *testing.T) {
	t.Parallel()
	e2eframework.Suite(t, "integration")

	parentWorkspace := logicalcluster.Name("root:default")
	
	env, err := integrationframework.NewTestEnvironment(t, "TMCClusterRegistrationValidation", parentWorkspace)
	require.NoError(t, err, "Failed to create test environment")

	ctx := env.Context()
	workspaceCluster := env.WorkspaceCluster()
	
	// Create test namespace
	namespace, err := env.CreateTestNamespace("validation-test")
	require.NoError(t, err, "Failed to create test namespace")

	gvr := schema.GroupVersionResource{
		Group:    "tmc.kcp.io",
		Version:  "v1alpha1",
		Resource: "clusterregistrations",
	}
	
	dynamicClient := env.TestClient.DynamicFor(workspaceCluster, gvr)

	validationTests := map[string]struct {
		clusterSpec     map[string]interface{}
		shouldSucceed   bool
		expectedError   string
	}{
		"valid cluster with all required fields": {
			clusterSpec: map[string]interface{}{
				"location": "us-west-2",
				"capacity": map[string]interface{}{
					"cpu":    "1000m",
					"memory": "2Gi",
				},
			},
			shouldSucceed: true,
		},
		"cluster missing location": {
			clusterSpec: map[string]interface{}{
				"capacity": map[string]interface{}{
					"cpu":    "1000m",
					"memory": "2Gi",
				},
			},
			shouldSucceed: false,
			expectedError: "location",
		},
		"cluster with invalid capacity": {
			clusterSpec: map[string]interface{}{
				"location": "us-west-2",
				"capacity": "invalid-capacity-format",
			},
			shouldSucceed: false,
			expectedError: "capacity",
		},
	}

	for testName, tc := range validationTests {
		t.Run(testName, func(t *testing.T) {
			clusterName := env.TestClient.WithTestPrefix("validation-cluster")
			
			clusterObj := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "tmc.kcp.io/v1alpha1",
					"kind":       "ClusterRegistration",
					"metadata": map[string]interface{}{
						"name":      clusterName,
						"namespace": namespace.Name,
					},
					"spec": tc.clusterSpec,
				},
			}
			
			_, err := dynamicClient.Create(ctx, clusterObj, metav1.CreateOptions{})
			
			if tc.shouldSucceed {
				require.NoError(t, err, "Valid cluster registration should succeed")
				
				// Clean up
				err = dynamicClient.Delete(ctx, clusterName, metav1.DeleteOptions{})
				require.NoError(t, err, "Failed to delete test cluster")
			} else {
				require.Error(t, err, "Invalid cluster registration should fail")
				require.Contains(t, err.Error(), tc.expectedError, "Error should mention the expected validation failure")
			}
		})
	}
}

// createMockClusterRegistration creates a mock ClusterRegistration object for testing
func createMockClusterRegistration(name, location, namespace string) *unstructured.Unstructured {
	cluster := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "tmc.kcp.io/v1alpha1",
			"kind":       "ClusterRegistration",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
				"labels": map[string]interface{}{
					"test-suite": "tmc-integration",
					"test-type":  "cluster-lifecycle",
				},
			},
			"spec": map[string]interface{}{
				"location": location,
				"capacity": map[string]interface{}{
					"cpu":    "1000m",
					"memory": "2Gi",
					"storage": "10Gi",
				},
				"labels": map[string]interface{}{
					"region": location,
					"env":    "test",
				},
			},
		},
	}
	
	// Add mock status for testing
	cluster.Object["status"] = map[string]interface{}{
		"phase": "Pending",
		"conditions": []interface{}{
			map[string]interface{}{
				"type":               "Ready",
				"status":             "False",
				"lastTransitionTime": metav1.Now().Format("2006-01-02T15:04:05Z"),
				"reason":             "Initializing",
				"message":            "Cluster registration is being processed",
			},
		},
	}
	
	return cluster
}