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

func TestTMCWorkloadPlacementCrossWorkspace(t *testing.T) {
	t.Parallel()
	e2eframework.Suite(t, "integration")

	parentWorkspace := logicalcluster.Name("root:default")
	
	env, err := integrationframework.NewTestEnvironment(t, "TMCWorkloadPlacementCrossWorkspace", parentWorkspace)
	require.NoError(t, err, "Failed to create test environment")

	tests := map[string]struct {
		sourceWorkspace    string
		targetWorkspace    string
		placementPolicy    map[string]interface{}
		workloadType       string
		shouldSucceed      bool
		expectedClusters   []string
	}{
		"single cluster placement": {
			sourceWorkspace: "workspace-a",
			targetWorkspace: "workspace-b",
			placementPolicy: map[string]interface{}{
				"clusterSelector": map[string]interface{}{
					"matchLabels": map[string]interface{}{
						"region": "us-west-2",
					},
				},
			},
			workloadType:     "deployment",
			shouldSucceed:    true,
			expectedClusters: []string{"cluster-west-1"},
		},
		"multi-cluster placement": {
			sourceWorkspace: "workspace-c",
			targetWorkspace: "workspace-d",
			placementPolicy: map[string]interface{}{
				"clusterSelector": map[string]interface{}{
					"matchLabels": map[string]interface{}{
						"env": "prod",
					},
				},
				"replicas": 2,
			},
			workloadType:     "deployment",
			shouldSucceed:    true,
			expectedClusters: []string{"cluster-prod-1", "cluster-prod-2"},
		},
		"no matching clusters": {
			sourceWorkspace: "workspace-e",
			targetWorkspace: "workspace-f",
			placementPolicy: map[string]interface{}{
				"clusterSelector": map[string]interface{}{
					"matchLabels": map[string]interface{}{
						"region": "nonexistent-region",
					},
				},
			},
			workloadType:     "deployment",
			shouldSucceed:    false,
			expectedClusters: []string{},
		},
	}

	for testName, tc := range tests {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()
			
			ctx := env.Context()
			workspaceCluster := env.WorkspaceCluster()
			
			// Create test namespace
			namespace, err := env.CreateTestNamespace("placement-test")
			require.NoError(t, err, "Failed to create test namespace")
			
			// Create source and target workspaces for the test
			sourceWS, err := createTestWorkspace(env, tc.sourceWorkspace)
			require.NoError(t, err, "Failed to create source workspace")
			
			targetWS, err := createTestWorkspace(env, tc.targetWorkspace)
			require.NoError(t, err, "Failed to create target workspace")
			
			// Create workload placement policy
			t.Logf("Creating workload placement policy for %s -> %s", tc.sourceWorkspace, tc.targetWorkspace)
			
			placement := createWorkloadPlacement(
				env.TestClient.WithTestPrefix("placement-policy"),
				namespace.Name,
				sourceWS,
				targetWS,
				tc.placementPolicy,
			)
			
			placementGVR := schema.GroupVersionResource{
				Group:    "tmc.kcp.io",
				Version:  "v1alpha1",
				Resource: "workloadplacements",
			}
			
			placementClient := env.TestClient.DynamicFor(workspaceCluster, placementGVR)
			
			createdPlacement, err := placementClient.Create(ctx, placement, metav1.CreateOptions{})
			require.NoError(t, err, "Failed to create workload placement policy")
			require.NotNil(t, createdPlacement, "Created placement should not be nil")
			
			// Create test workload in source workspace
			t.Logf("Creating test workload in source workspace %s", tc.sourceWorkspace)
			
			workload := createTestWorkload(
				env.TestClient.WithTestPrefix("test-workload"),
				namespace.Name,
				tc.workloadType,
			)
			
			deploymentGVR := schema.GroupVersionResource{
				Group:    "apps",
				Version:  "v1",
				Resource: "deployments",
			}
			
			workloadClient := env.TestClient.DynamicFor(logicalcluster.Name(sourceWS), deploymentGVR)
			
			createdWorkload, err := workloadClient.Create(ctx, workload, metav1.CreateOptions{})
			require.NoError(t, err, "Failed to create test workload")
			require.NotNil(t, createdWorkload, "Created workload should not be nil")
			
			// Test placement resolution
			t.Logf("Validating workload placement resolution")
			
			if tc.shouldSucceed {
				// Wait for placement to be resolved
				env.Eventually(func() (bool, string) {
					placement, err := placementClient.Get(ctx, createdPlacement.GetName(), metav1.GetOptions{})
					if err != nil {
						return false, fmt.Sprintf("failed to get placement: %v", err)
					}
					
					status, found, err := unstructured.NestedMap(placement.Object, "status")
					if err != nil || !found {
						return false, "placement status not found"
					}
					
					clusters, found, err := unstructured.NestedSlice(status, "targetClusters")
					if err != nil || !found {
						return false, "target clusters not found in status"
					}
					
					if len(clusters) != len(tc.expectedClusters) {
						return false, fmt.Sprintf("expected %d clusters, got %d", len(tc.expectedClusters), len(clusters))
					}
					
					return true, ""
				}, "placement to resolve target clusters")
				
				// Validate correct cluster selection
				placement, err := placementClient.Get(ctx, createdPlacement.GetName(), metav1.GetOptions{})
				require.NoError(t, err, "Failed to get placement after resolution")
				
				status, found, err := unstructured.NestedMap(placement.Object, "status")
				require.NoError(t, err, "Failed to get placement status")
				require.True(t, found, "Placement status should be present")
				
				clusters, found, err := unstructured.NestedSlice(status, "targetClusters")
				require.NoError(t, err, "Failed to get target clusters")
				require.True(t, found, "Target clusters should be present")
				require.Len(t, clusters, len(tc.expectedClusters), "Should have expected number of target clusters")
				
			} else {
				// Wait for placement to fail
				env.Eventually(func() (bool, string) {
					placement, err := placementClient.Get(ctx, createdPlacement.GetName(), metav1.GetOptions{})
					if err != nil {
						return false, fmt.Sprintf("failed to get placement: %v", err)
					}
					
					phase, found, err := unstructured.NestedString(placement.Object, "status", "phase")
					if err != nil || !found {
						return false, "placement phase not found"
					}
					
					return phase == "Failed", fmt.Sprintf("placement in phase %s", phase)
				}, "placement to fail due to no matching clusters")
			}
			
			// Test cross-workspace workload synchronization
			if tc.shouldSucceed {
				t.Logf("Validating cross-workspace workload synchronization")
				
				// Check that workload appears in target workspace
				targetWorkloadClient := env.TestClient.DynamicFor(logicalcluster.Name(targetWS), deploymentGVR)
				
				env.Eventually(func() (bool, string) {
					_, err := targetWorkloadClient.Get(ctx, createdWorkload.GetName(), metav1.GetOptions{})
					if err != nil {
						return false, fmt.Sprintf("workload not found in target workspace: %v", err)
					}
					return true, ""
				}, "workload to be synchronized to target workspace")
				
				// Validate workload has placement annotations
				targetWorkload, err := targetWorkloadClient.Get(ctx, createdWorkload.GetName(), metav1.GetOptions{})
				require.NoError(t, err, "Failed to get workload in target workspace")
				
				annotations, found, err := unstructured.NestedStringMap(targetWorkload.Object, "metadata", "annotations")
				require.NoError(t, err, "Failed to get workload annotations")
				require.True(t, found, "Workload should have annotations")
				
				_, hasPlacementAnnotation := annotations["tmc.kcp.io/placement-policy"]
				require.True(t, hasPlacementAnnotation, "Workload should have placement policy annotation")
			}
			
			// Test cleanup
			t.Logf("Testing placement cleanup")
			
			err = placementClient.Delete(ctx, createdPlacement.GetName(), metav1.DeleteOptions{})
			require.NoError(t, err, "Failed to delete placement policy")
			
			err = workloadClient.Delete(ctx, createdWorkload.GetName(), metav1.DeleteOptions{})
			require.NoError(t, err, "Failed to delete workload")
			
			t.Logf("Successfully completed workload placement test for %s", testName)
		})
	}
}

func TestTMCWorkloadPlacementPolicyValidation(t *testing.T) {
	t.Parallel()
	e2eframework.Suite(t, "integration")

	parentWorkspace := logicalcluster.Name("root:default")
	
	env, err := integrationframework.NewTestEnvironment(t, "TMCWorkloadPlacementPolicyValidation", parentWorkspace)
	require.NoError(t, err, "Failed to create test environment")

	ctx := env.Context()
	workspaceCluster := env.WorkspaceCluster()
	
	namespace, err := env.CreateTestNamespace("policy-validation-test")
	require.NoError(t, err, "Failed to create test namespace")

	placementGVR := schema.GroupVersionResource{
		Group:    "tmc.kcp.io",
		Version:  "v1alpha1",
		Resource: "workloadplacements",
	}
	
	placementClient := env.TestClient.DynamicFor(workspaceCluster, placementGVR)

	validationTests := map[string]struct {
		placementSpec   map[string]interface{}
		shouldSucceed   bool
		expectedError   string
	}{
		"valid placement with cluster selector": {
			placementSpec: map[string]interface{}{
				"sourceWorkspace": "workspace-a",
				"targetWorkspace": "workspace-b",
				"clusterSelector": map[string]interface{}{
					"matchLabels": map[string]interface{}{
						"region": "us-west-2",
					},
				},
			},
			shouldSucceed: true,
		},
		"placement missing source workspace": {
			placementSpec: map[string]interface{}{
				"targetWorkspace": "workspace-b",
				"clusterSelector": map[string]interface{}{
					"matchLabels": map[string]interface{}{
						"region": "us-west-2",
					},
				},
			},
			shouldSucceed: false,
			expectedError: "sourceWorkspace",
		},
		"placement missing target workspace": {
			placementSpec: map[string]interface{}{
				"sourceWorkspace": "workspace-a",
				"clusterSelector": map[string]interface{}{
					"matchLabels": map[string]interface{}{
						"region": "us-west-2",
					},
				},
			},
			shouldSucceed: false,
			expectedError: "targetWorkspace",
		},
		"placement missing cluster selector": {
			placementSpec: map[string]interface{}{
				"sourceWorkspace": "workspace-a",
				"targetWorkspace": "workspace-b",
			},
			shouldSucceed: false,
			expectedError: "clusterSelector",
		},
	}

	for testName, tc := range validationTests {
		t.Run(testName, func(t *testing.T) {
			placementName := env.TestClient.WithTestPrefix("validation-placement")
			
			placementObj := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "tmc.kcp.io/v1alpha1",
					"kind":       "WorkloadPlacement",
					"metadata": map[string]interface{}{
						"name":      placementName,
						"namespace": namespace.Name,
					},
					"spec": tc.placementSpec,
				},
			}
			
			_, err := placementClient.Create(ctx, placementObj, metav1.CreateOptions{})
			
			if tc.shouldSucceed {
				require.NoError(t, err, "Valid placement policy should succeed")
				
				// Clean up
				err = placementClient.Delete(ctx, placementName, metav1.DeleteOptions{})
				require.NoError(t, err, "Failed to delete test placement")
			} else {
				require.Error(t, err, "Invalid placement policy should fail")
				require.Contains(t, err.Error(), tc.expectedError, "Error should mention the expected validation failure")
			}
		})
	}
}

// createTestWorkspace creates a test workspace for placement testing
func createTestWorkspace(env *integrationframework.TestEnvironment, workspaceName string) (string, error) {
	ctx := env.Context()
	parentWorkspace := logicalcluster.Name("root:default")
	
	fullWorkspaceName := env.TestClient.WithTestPrefix(workspaceName)
	
	workspace := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "tenancy.kcp.io/v1alpha1",
			"kind":       "Workspace",
			"metadata": map[string]interface{}{
				"name": fullWorkspaceName,
				"labels": map[string]interface{}{
					"test-suite": "tmc-integration",
					"test-type":  "placement",
				},
			},
			"spec": map[string]interface{}{
				"type": map[string]interface{}{
					"name": "universal",
					"path": "root",
				},
			},
		},
	}
	
	wsGVR := schema.GroupVersionResource{
		Group:    "tenancy.kcp.io",
		Version:  "v1alpha1",
		Resource: "workspaces",
	}
	
	wsClient := env.TestClient.DynamicFor(parentWorkspace, wsGVR)
	
	_, err := wsClient.Create(ctx, workspace, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to create workspace %s: %w", fullWorkspaceName, err)
	}
	
	// Add cleanup
	env.AddCleanup(func() error {
		return wsClient.Delete(context.Background(), fullWorkspaceName, metav1.DeleteOptions{})
	})
	
	return fullWorkspaceName, nil
}

// createWorkloadPlacement creates a WorkloadPlacement object for testing
func createWorkloadPlacement(name, namespace, sourceWS, targetWS string, policy map[string]interface{}) *unstructured.Unstructured {
	spec := map[string]interface{}{
		"sourceWorkspace": sourceWS,
		"targetWorkspace": targetWS,
	}
	
	// Merge policy into spec
	for k, v := range policy {
		spec[k] = v
	}
	
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "tmc.kcp.io/v1alpha1",
			"kind":       "WorkloadPlacement",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
				"labels": map[string]interface{}{
					"test-suite": "tmc-integration",
					"test-type":  "placement",
				},
			},
			"spec": spec,
		},
	}
}

// createTestWorkload creates a test workload (deployment) for placement testing
func createTestWorkload(name, namespace, workloadType string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
				"labels": map[string]interface{}{
					"test-suite": "tmc-integration",
					"test-type":  "workload",
				},
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
								"ports": []interface{}{
									map[string]interface{}{
										"containerPort": 80,
									},
								},
							},
						},
					},
				},
			},
		},
	}
}