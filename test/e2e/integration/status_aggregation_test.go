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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/kcp-dev/logicalcluster/v3"

	e2eframework "github.com/kcp-dev/kcp/test/e2e/framework"
	integrationframework "github.com/kcp-dev/kcp/test/e2e/integration/framework"
)

func TestTMCStatusAggregation(t *testing.T) {
	t.Parallel()
	e2eframework.Suite(t, "integration")

	parentWorkspace := logicalcluster.Name("root:default")
	
	env, err := integrationframework.NewTestEnvironment(t, "TMCStatusAggregation", parentWorkspace)
	require.NoError(t, err, "Failed to create test environment")

	tests := map[string]struct {
		resourceType       string
		clusters           []string
		statusStates       []string
		aggregationPolicy  map[string]interface{}
		expectedAggregate  string
		testFailureState   bool
	}{
		"deployment status across multiple clusters": {
			resourceType: "deployment",
			clusters:     []string{"cluster-1", "cluster-2", "cluster-3"},
			statusStates: []string{"Ready", "Ready", "Ready"},
			aggregationPolicy: map[string]interface{}{
				"mode": "majority",
				"healthCheck": map[string]interface{}{
					"readyReplicas": "minCount:1",
				},
			},
			expectedAggregate: "Ready",
			testFailureState:  false,
		},
		"service status with mixed states": {
			resourceType: "service",
			clusters:     []string{"cluster-1", "cluster-2", "cluster-3"},
			statusStates: []string{"Ready", "Pending", "Ready"},
			aggregationPolicy: map[string]interface{}{
				"mode": "unanimous",
				"healthCheck": map[string]interface{}{
					"endpoints": "minCount:1",
				},
			},
			expectedAggregate: "Degraded",
			testFailureState:  false,
		},
		"configmap status with failure propagation": {
			resourceType: "configmap",
			clusters:     []string{"cluster-1", "cluster-2"},
			statusStates: []string{"Failed", "Ready"},
			aggregationPolicy: map[string]interface{}{
				"mode": "failfast",
				"failurePropagation": true,
			},
			expectedAggregate: "Failed",
			testFailureState:  true,
		},
		"custom resource status aggregation": {
			resourceType: "customresource",
			clusters:     []string{"cluster-1", "cluster-2", "cluster-3", "cluster-4"},
			statusStates: []string{"Ready", "Ready", "Pending", "Ready"},
			aggregationPolicy: map[string]interface{}{
				"mode": "percentage",
				"threshold": 75,
			},
			expectedAggregate: "Ready",
			testFailureState:  false,
		},
	}

	for testName, tc := range tests {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()
			
			ctx := env.Context()
			workspaceCluster := env.WorkspaceCluster()
			
			// Create test namespace
			namespace, err := env.CreateTestNamespace("status-aggregation")
			require.NoError(t, err, "Failed to create test namespace")
			
			// Create target workspaces for each cluster
			clusterWorkspaces := make([]string, len(tc.clusters))
			for i, cluster := range tc.clusters {
				ws, err := createTestWorkspace(env, fmt.Sprintf("%s-ws", cluster))
				require.NoError(t, err, "Failed to create workspace for cluster %s", cluster)
				clusterWorkspaces[i] = ws
			}
			
			// Create status aggregation policy
			t.Logf("Creating status aggregation policy for %s", tc.resourceType)
			
			aggregationPolicy := createStatusAggregationPolicy(
				env.TestClient.WithTestPrefix("status-aggregation-policy"),
				namespace.Name,
				tc.resourceType,
				clusterWorkspaces,
				tc.aggregationPolicy,
			)
			
			aggregationGVR := schema.GroupVersionResource{
				Group:    "tmc.kcp.io",
				Version:  "v1alpha1",
				Resource: "statusaggregationpolicies",
			}
			
			aggregationClient := env.TestClient.DynamicFor(workspaceCluster, aggregationGVR)
			
			createdPolicy, err := aggregationClient.Create(ctx, aggregationPolicy, metav1.CreateOptions{})
			require.NoError(t, err, "Failed to create status aggregation policy")
			require.NotNil(t, createdPolicy, "Created aggregation policy should not be nil")
			
			// Wait for aggregation policy to become active
			env.Eventually(func() (bool, string) {
				policy, err := aggregationClient.Get(ctx, createdPolicy.GetName(), metav1.GetOptions{})
				if err != nil {
					return false, fmt.Sprintf("failed to get aggregation policy: %v", err)
				}
				
				phase, found, err := unstructured.NestedString(policy.Object, "status", "phase")
				if err != nil || !found {
					return false, "aggregation policy phase not found"
				}
				
				return phase == "Active", fmt.Sprintf("aggregation policy in phase %s", phase)
			}, "status aggregation policy to become active")
			
			// Create resources in each cluster workspace with different statuses
			t.Logf("Creating resources across %d cluster workspaces", len(clusterWorkspaces))
			
			for i, clusterWS := range clusterWorkspaces {
				resource := createStatusTestResource(
					env.TestClient.WithTestPrefix(fmt.Sprintf("status-test-%s", tc.clusters[i])),
					namespace.Name,
					tc.resourceType,
					tc.statusStates[i],
				)
				
				gvr := getGVRFromResourceType(tc.resourceType)
				clusterClient := env.TestClient.DynamicFor(logicalcluster.Name(clusterWS), gvr)
				
				createdResource, err := clusterClient.Create(ctx, resource, metav1.CreateOptions{})
				require.NoError(t, err, "Failed to create resource in cluster workspace %s", clusterWS)
				require.NotNil(t, createdResource, "Created resource should not be nil")
				
				// Update resource status to simulate cluster-specific states
				t.Logf("Setting resource status to %s in cluster workspace %s", tc.statusStates[i], clusterWS)
				
				statusResource := createdResource.DeepCopy()
				err = unstructured.SetNestedField(statusResource.Object, tc.statusStates[i], "status", "phase")
				require.NoError(t, err, "Failed to set resource status")
				
				// Add cluster-specific status conditions
				conditions := []interface{}{
					map[string]interface{}{
						"type":               "Ready",
						"status":             getConditionStatus(tc.statusStates[i]),
						"lastTransitionTime": metav1.Now().Format("2006-01-02T15:04:05Z"),
						"reason":             tc.statusStates[i],
						"message":            fmt.Sprintf("Resource is %s in cluster %s", tc.statusStates[i], tc.clusters[i]),
					},
				}
				
				err = unstructured.SetNestedSlice(statusResource.Object, conditions, "status", "conditions")
				require.NoError(t, err, "Failed to set resource conditions")
				
				_, err = clusterClient.UpdateStatus(ctx, statusResource, metav1.UpdateOptions{})
				require.NoError(t, err, "Failed to update resource status in cluster %s", clusterWS)
			}
			
			// Wait for status aggregation
			t.Logf("Waiting for status aggregation to complete")
			
			env.Eventually(func() (bool, string) {
				policy, err := aggregationClient.Get(ctx, createdPolicy.GetName(), metav1.GetOptions{})
				if err != nil {
					return false, fmt.Sprintf("failed to get aggregation policy: %v", err)
				}
				
				aggregatedStatus, found, err := unstructured.NestedString(policy.Object, "status", "aggregatedStatus")
				if err != nil || !found {
					return false, "aggregated status not found"
				}
				
				return aggregatedStatus != "", fmt.Sprintf("aggregated status is %s", aggregatedStatus)
			}, "status aggregation to complete")
			
			// Validate aggregated status
			t.Logf("Validating aggregated status matches expected: %s", tc.expectedAggregate)
			
			finalPolicy, err := aggregationClient.Get(ctx, createdPolicy.GetName(), metav1.GetOptions{})
			require.NoError(t, err, "Failed to get aggregation policy for validation")
			
			aggregatedStatus, found, err := unstructured.NestedString(finalPolicy.Object, "status", "aggregatedStatus")
			require.NoError(t, err, "Failed to get aggregated status")
			require.True(t, found, "Aggregated status should be present")
			require.Equal(t, tc.expectedAggregate, aggregatedStatus, "Aggregated status should match expected")
			
			// Validate cluster status breakdown
			clusterStatuses, found, err := unstructured.NestedSlice(finalPolicy.Object, "status", "clusterStatuses")
			require.NoError(t, err, "Failed to get cluster statuses")
			require.True(t, found, "Cluster statuses should be present")
			require.Len(t, clusterStatuses, len(tc.clusters), "Should have status for each cluster")
			
			// Test status change propagation
			t.Logf("Testing status change propagation")
			
			// Update status in first cluster
			firstClusterWS := clusterWorkspaces[0]
			gvr := getGVRFromResourceType(tc.resourceType)
			firstClusterClient := env.TestClient.DynamicFor(logicalcluster.Name(firstClusterWS), gvr)
			
			resourceName := env.TestClient.WithTestPrefix(fmt.Sprintf("status-test-%s", tc.clusters[0]))
			statusResource, err := firstClusterClient.Get(ctx, resourceName, metav1.GetOptions{})
			require.NoError(t, err, "Failed to get resource for status update")
			
			newStatus := "Updated"
			err = unstructured.SetNestedField(statusResource.Object, newStatus, "status", "phase")
			require.NoError(t, err, "Failed to set new status")
			
			_, err = firstClusterClient.UpdateStatus(ctx, statusResource, metav1.UpdateOptions{})
			require.NoError(t, err, "Failed to update resource status")
			
			// Wait for aggregation to reflect the change
			env.Eventually(func() (bool, string) {
				policy, err := aggregationClient.Get(ctx, createdPolicy.GetName(), metav1.GetOptions{})
				if err != nil {
					return false, fmt.Sprintf("failed to get updated aggregation policy: %v", err)
				}
				
				lastUpdated, found, err := unstructured.NestedString(policy.Object, "status", "lastUpdated")
				if err != nil || !found {
					return false, "last updated timestamp not found"
				}
				
				return lastUpdated != "", ""
			}, "status aggregation to reflect status change")
			
			// Test failure state handling if configured
			if tc.testFailureState {
				t.Logf("Testing failure state handling")
				
				policy, err := aggregationClient.Get(ctx, createdPolicy.GetName(), metav1.GetOptions{})
				require.NoError(t, err, "Failed to get policy for failure state test")
				
				failureConditions, found, err := unstructured.NestedSlice(policy.Object, "status", "conditions")
				require.NoError(t, err, "Failed to get policy conditions")
				require.True(t, found, "Policy should have conditions in failure state")
				require.NotEmpty(t, failureConditions, "Policy should have failure conditions")
				
				// Check for failure-related condition
				hasFailureCondition := false
				for _, condition := range failureConditions {
					conditionMap, ok := condition.(map[string]interface{})
					require.True(t, ok, "Condition should be a map")
					
					conditionType, found, err := unstructured.NestedString(conditionMap, "type")
					require.NoError(t, err, "Failed to get condition type")
					require.True(t, found, "Condition type should be present")
					
					if conditionType == "Failed" {
						hasFailureCondition = true
						status, found, err := unstructured.NestedString(conditionMap, "status")
						require.NoError(t, err, "Failed to get condition status")
						require.True(t, found, "Condition status should be present")
						require.Equal(t, "True", status, "Failed condition should be True")
						break
					}
				}
				
				require.True(t, hasFailureCondition, "Policy should have a Failed condition in failure state")
			}
			
			t.Logf("Successfully completed status aggregation test: %s", testName)
		})
	}
}

func TestTMCStatusAggregationPolicyValidation(t *testing.T) {
	t.Parallel()
	e2eframework.Suite(t, "integration")

	parentWorkspace := logicalcluster.Name("root:default")
	
	env, err := integrationframework.NewTestEnvironment(t, "TMCStatusAggregationPolicyValidation", parentWorkspace)
	require.NoError(t, err, "Failed to create test environment")

	ctx := env.Context()
	workspaceCluster := env.WorkspaceCluster()
	
	namespace, err := env.CreateTestNamespace("status-policy-validation")
	require.NoError(t, err, "Failed to create test namespace")

	aggregationGVR := schema.GroupVersionResource{
		Group:    "tmc.kcp.io",
		Version:  "v1alpha1",
		Resource: "statusaggregationpolicies",
	}
	
	aggregationClient := env.TestClient.DynamicFor(workspaceCluster, aggregationGVR)

	validationTests := map[string]struct {
		policySpec      map[string]interface{}
		shouldSucceed   bool
		expectedError   string
	}{
		"valid aggregation policy with majority mode": {
			policySpec: map[string]interface{}{
				"resourceType": "deployments",
				"targetWorkspaces": []string{"workspace-1", "workspace-2", "workspace-3"},
				"aggregationMode": "majority",
				"healthCheck": map[string]interface{}{
					"readyReplicas": "minCount:1",
				},
			},
			shouldSucceed: true,
		},
		"aggregation policy missing resource type": {
			policySpec: map[string]interface{}{
				"targetWorkspaces": []string{"workspace-1", "workspace-2"},
				"aggregationMode": "majority",
			},
			shouldSucceed: false,
			expectedError: "resourceType",
		},
		"aggregation policy missing target workspaces": {
			policySpec: map[string]interface{}{
				"resourceType": "deployments",
				"aggregationMode": "majority",
			},
			shouldSucceed: false,
			expectedError: "targetWorkspaces",
		},
		"aggregation policy with invalid mode": {
			policySpec: map[string]interface{}{
				"resourceType": "deployments",
				"targetWorkspaces": []string{"workspace-1"},
				"aggregationMode": "invalid-mode",
			},
			shouldSucceed: false,
			expectedError: "aggregationMode",
		},
	}

	for testName, tc := range validationTests {
		t.Run(testName, func(t *testing.T) {
			policyName := env.TestClient.WithTestPrefix("validation-aggregation-policy")
			
			policyObj := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "tmc.kcp.io/v1alpha1",
					"kind":       "StatusAggregationPolicy",
					"metadata": map[string]interface{}{
						"name":      policyName,
						"namespace": namespace.Name,
					},
					"spec": tc.policySpec,
				},
			}
			
			_, err := aggregationClient.Create(ctx, policyObj, metav1.CreateOptions{})
			
			if tc.shouldSucceed {
				require.NoError(t, err, "Valid aggregation policy should succeed")
				
				// Clean up
				err = aggregationClient.Delete(ctx, policyName, metav1.DeleteOptions{})
				require.NoError(t, err, "Failed to delete test aggregation policy")
			} else {
				require.Error(t, err, "Invalid aggregation policy should fail")
				require.Contains(t, err.Error(), tc.expectedError, "Error should mention the expected validation failure")
			}
		})
	}
}

// createStatusAggregationPolicy creates a StatusAggregationPolicy for testing
func createStatusAggregationPolicy(name, namespace, resourceType string, workspaces []string, policy map[string]interface{}) *unstructured.Unstructured {
	spec := map[string]interface{}{
		"resourceType":     resourceType,
		"targetWorkspaces": workspaces,
	}
	
	// Merge policy configuration into spec
	for k, v := range policy {
		spec[k] = v
	}
	
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "tmc.kcp.io/v1alpha1",
			"kind":       "StatusAggregationPolicy",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
				"labels": map[string]interface{}{
					"test-suite": "tmc-integration",
					"test-type":  "status-aggregation",
				},
			},
			"spec": spec,
		},
	}
}

// createStatusTestResource creates a resource for status aggregation testing
func createStatusTestResource(name, namespace, resourceType, initialStatus string) *unstructured.Unstructured {
	var resource *unstructured.Unstructured
	
	switch resourceType {
	case "deployment":
		resource = &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name":      name,
					"namespace": namespace,
				},
				"spec": map[string]interface{}{
					"replicas": 1,
					"selector": map[string]interface{}{
						"matchLabels": map[string]interface{}{
							"app": name,
						},
					},
					"template": map[string]interface{}{
						"metadata": map[string]interface{}{
							"labels": map[string]interface{}{
								"app": name,
							},
						},
						"spec": map[string]interface{}{
							"containers": []interface{}{
								map[string]interface{}{
									"name":  "test-container",
									"image": "nginx:latest",
								},
							},
						},
					},
				},
			},
		}
	case "service":
		resource = &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Service",
				"metadata": map[string]interface{}{
					"name":      name,
					"namespace": namespace,
				},
				"spec": map[string]interface{}{
					"selector": map[string]interface{}{
						"app": name,
					},
					"ports": []interface{}{
						map[string]interface{}{
							"port":       80,
							"targetPort": 8080,
						},
					},
				},
			},
		}
	case "configmap":
		resource = &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]interface{}{
					"name":      name,
					"namespace": namespace,
				},
				"data": map[string]interface{}{
					"config.yaml": "test: value",
				},
			},
		}
	default: // customresource
		resource = &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "custom.tmc.io/v1",
				"kind":       "CustomResource",
				"metadata": map[string]interface{}{
					"name":      name,
					"namespace": namespace,
				},
				"spec": map[string]interface{}{
					"value": "test",
				},
			},
		}
	}
	
	// Add common labels
	labels, _, _ := unstructured.NestedStringMap(resource.Object, "metadata", "labels")
	if labels == nil {
		labels = make(map[string]string)
	}
	labels["test-suite"] = "tmc-integration"
	labels["test-type"] = "status-aggregation"
	labels["status-test"] = "true"
	
	unstructured.SetNestedStringMap(resource.Object, labels, "metadata", "labels")
	
	// Set initial status
	resource.Object["status"] = map[string]interface{}{
		"phase": initialStatus,
	}
	
	return resource
}

// getGVRFromResourceType returns GVR for a resource type
func getGVRFromResourceType(resourceType string) schema.GroupVersionResource {
	switch resourceType {
	case "deployment":
		return schema.GroupVersionResource{
			Group:    "apps",
			Version:  "v1",
			Resource: "deployments",
		}
	case "service":
		return schema.GroupVersionResource{
			Group:    "",
			Version:  "v1",
			Resource: "services",
		}
	case "configmap":
		return schema.GroupVersionResource{
			Group:    "",
			Version:  "v1",
			Resource: "configmaps",
		}
	default: // customresource
		return schema.GroupVersionResource{
			Group:    "custom.tmc.io",
			Version:  "v1",
			Resource: "customresources",
		}
	}
}

// getConditionStatus converts status phase to condition status
func getConditionStatus(phase string) string {
	switch phase {
	case "Ready":
		return "True"
	case "Failed":
		return "False"
	default:
		return "Unknown"
	}
}