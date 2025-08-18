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
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/kcp-dev/logicalcluster/v3"

	e2eframework "github.com/kcp-dev/kcp/test/e2e/framework"
	integrationframework "github.com/kcp-dev/kcp/test/e2e/integration/framework"
)

func TestTMCResourceSynchronization(t *testing.T) {
	t.Parallel()
	e2eframework.Suite(t, "integration")

	parentWorkspace := logicalcluster.Name("root:default")
	
	env, err := integrationframework.NewTestEnvironment(t, "TMCResourceSynchronization", parentWorkspace)
	require.NoError(t, err, "Failed to create test environment")

	tests := map[string]struct {
		sourceResource     map[string]interface{}
		syncPolicy         map[string]interface{}
		targetWorkspaces   []string
		expectedSyncStatus string
		testConflicts      bool
	}{
		"sync deployment across workspaces": {
			sourceResource: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name": "sync-test-deployment",
				},
			},
			syncPolicy: map[string]interface{}{
				"syncMode": "bidirectional",
				"conflictResolution": "sourceWins",
			},
			targetWorkspaces:   []string{"target-ws-1", "target-ws-2"},
			expectedSyncStatus: "Synchronized",
			testConflicts:      false,
		},
		"sync configmap with conflict resolution": {
			sourceResource: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]interface{}{
					"name": "sync-test-configmap",
				},
				"data": map[string]interface{}{
					"config.yaml": "key: value",
				},
			},
			syncPolicy: map[string]interface{}{
				"syncMode": "unidirectional",
				"conflictResolution": "manual",
			},
			targetWorkspaces:   []string{"conflict-ws-1"},
			expectedSyncStatus: "ConflictDetected",
			testConflicts:      true,
		},
		"sync service with selector constraints": {
			sourceResource: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Service",
				"metadata": map[string]interface{}{
					"name": "sync-test-service",
				},
				"spec": map[string]interface{}{
					"selector": map[string]interface{}{
						"app": "test",
					},
					"ports": []interface{}{
						map[string]interface{}{
							"port":       80,
							"targetPort": 8080,
						},
					},
				},
			},
			syncPolicy: map[string]interface{}{
				"syncMode": "bidirectional",
				"selector": map[string]interface{}{
					"matchLabels": map[string]interface{}{
						"sync-enabled": "true",
					},
				},
			},
			targetWorkspaces:   []string{"service-ws-1"},
			expectedSyncStatus: "Synchronized",
			testConflicts:      false,
		},
	}

	for testName, tc := range tests {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()
			
			ctx := env.Context()
			workspaceCluster := env.WorkspaceCluster()
			
			// Create test namespace
			namespace, err := env.CreateTestNamespace("sync-test")
			require.NoError(t, err, "Failed to create test namespace")
			
			// Create source and target workspaces
			sourceWS, err := createTestWorkspace(env, "sync-source")
			require.NoError(t, err, "Failed to create source workspace")
			
			targetWorkspaces := make([]string, len(tc.targetWorkspaces))
			for i, wsName := range tc.targetWorkspaces {
				targetWS, err := createTestWorkspace(env, wsName)
				require.NoError(t, err, "Failed to create target workspace %s", wsName)
				targetWorkspaces[i] = targetWS
			}
			
			// Create resource synchronization policy
			t.Logf("Creating resource synchronization policy")
			
			syncPolicy := createResourceSyncPolicy(
				env.TestClient.WithTestPrefix("sync-policy"),
				namespace.Name,
				sourceWS,
				targetWorkspaces,
				tc.syncPolicy,
			)
			
			syncPolicyGVR := schema.GroupVersionResource{
				Group:    "tmc.kcp.io",
				Version:  "v1alpha1",
				Resource: "resourcesyncpolicies",
			}
			
			syncPolicyClient := env.TestClient.DynamicFor(workspaceCluster, syncPolicyGVR)
			
			createdSyncPolicy, err := syncPolicyClient.Create(ctx, syncPolicy, metav1.CreateOptions{})
			require.NoError(t, err, "Failed to create resource sync policy")
			require.NotNil(t, createdSyncPolicy, "Created sync policy should not be nil")
			
			// Wait for sync policy to become active
			env.Eventually(func() (bool, string) {
				policy, err := syncPolicyClient.Get(ctx, createdSyncPolicy.GetName(), metav1.GetOptions{})
				if err != nil {
					return false, fmt.Sprintf("failed to get sync policy: %v", err)
				}
				
				phase, found, err := unstructured.NestedString(policy.Object, "status", "phase")
				if err != nil || !found {
					return false, "sync policy phase not found"
				}
				
				return phase == "Active", fmt.Sprintf("sync policy in phase %s", phase)
			}, "resource sync policy to become active")
			
			// Create source resource
			t.Logf("Creating source resource in workspace %s", sourceWS)
			
			sourceResource := tc.sourceResource
			if metadata, found := sourceResource["metadata"]; found {
				if metadataMap, ok := metadata.(map[string]interface{}); ok {
					metadataMap["namespace"] = namespace.Name
					metadataMap["labels"] = map[string]interface{}{
						"test-suite":   "tmc-integration",
						"test-type":    "resource-sync",
						"sync-enabled": "true",
					}
				}
			}
			
			sourceObj := &unstructured.Unstructured{Object: sourceResource}
			
			// Determine GVR from resource
			gvr := getGVRFromResource(sourceObj)
			
			sourceClient := env.TestClient.DynamicFor(logicalcluster.Name(sourceWS), gvr)
			
			createdResource, err := sourceClient.Create(ctx, sourceObj, metav1.CreateOptions{})
			require.NoError(t, err, "Failed to create source resource")
			require.NotNil(t, createdResource, "Created resource should not be nil")
			
			// Test resource synchronization
			t.Logf("Validating resource synchronization to target workspaces")
			
			for _, targetWS := range targetWorkspaces {
				targetClient := env.TestClient.DynamicFor(logicalcluster.Name(targetWS), gvr)
				
				env.Eventually(func() (bool, string) {
					_, err := targetClient.Get(ctx, createdResource.GetName(), metav1.GetOptions{})
					if err != nil {
						return false, fmt.Sprintf("resource not found in target workspace %s: %v", targetWS, err)
					}
					return true, ""
				}, fmt.Sprintf("resource to be synchronized to workspace %s", targetWS))
				
				// Validate synchronized resource has sync annotations
				syncedResource, err := targetClient.Get(ctx, createdResource.GetName(), metav1.GetOptions{})
				require.NoError(t, err, "Failed to get synchronized resource")
				
				annotations, found, err := unstructured.NestedStringMap(syncedResource.Object, "metadata", "annotations")
				require.NoError(t, err, "Failed to get resource annotations")
				require.True(t, found, "Synchronized resource should have annotations")
				
				_, hasSyncOrigin := annotations["tmc.kcp.io/sync-origin"]
				require.True(t, hasSyncOrigin, "Synchronized resource should have sync origin annotation")
				
				_, hasSyncPolicy := annotations["tmc.kcp.io/sync-policy"]
				require.True(t, hasSyncPolicy, "Synchronized resource should have sync policy annotation")
			}
			
			// Test conflict resolution if enabled
			if tc.testConflicts {
				t.Logf("Testing conflict resolution")
				
				// Modify resource in target workspace to create conflict
				targetClient := env.TestClient.DynamicFor(logicalcluster.Name(targetWorkspaces[0]), gvr)
				
				targetResource, err := targetClient.Get(ctx, createdResource.GetName(), metav1.GetOptions{})
				require.NoError(t, err, "Failed to get target resource for conflict test")
				
				// Modify the resource to create a conflict
				if targetResource.GetKind() == "ConfigMap" {
					err = unstructured.SetNestedField(targetResource.Object, "conflicting: value", "data", "config.yaml")
					require.NoError(t, err, "Failed to set conflicting data")
					
					_, err = targetClient.Update(ctx, targetResource, metav1.UpdateOptions{})
					require.NoError(t, err, "Failed to update target resource to create conflict")
				}
				
				// Also modify source resource to create bidirectional conflict
				err = unstructured.SetNestedField(createdResource.Object, "source: updated", "data", "config.yaml")
				require.NoError(t, err, "Failed to set source data")
				
				_, err = sourceClient.Update(ctx, createdResource, metav1.UpdateOptions{})
				require.NoError(t, err, "Failed to update source resource")
				
				// Wait for conflict to be detected
				env.Eventually(func() (bool, string) {
					policy, err := syncPolicyClient.Get(ctx, createdSyncPolicy.GetName(), metav1.GetOptions{})
					if err != nil {
						return false, fmt.Sprintf("failed to get sync policy: %v", err)
					}
					
					status, found, err := unstructured.NestedString(policy.Object, "status", "syncStatus")
					if err != nil || !found {
						return false, "sync status not found"
					}
					
					return status == "ConflictDetected", fmt.Sprintf("sync status is %s", status)
				}, "conflict to be detected in sync policy")
			}
			
			// Validate final sync status
			t.Logf("Validating final synchronization status")
			
			policy, err := syncPolicyClient.Get(ctx, createdSyncPolicy.GetName(), metav1.GetOptions{})
			require.NoError(t, err, "Failed to get sync policy for final status check")
			
			finalStatus, found, err := unstructured.NestedString(policy.Object, "status", "syncStatus")
			require.NoError(t, err, "Failed to get final sync status")
			require.True(t, found, "Final sync status should be present")
			require.Equal(t, tc.expectedSyncStatus, finalStatus, "Final sync status should match expected")
			
			// Test resource deletion synchronization
			t.Logf("Testing resource deletion synchronization")
			
			err = sourceClient.Delete(ctx, createdResource.GetName(), metav1.DeleteOptions{})
			require.NoError(t, err, "Failed to delete source resource")
			
			// Verify resource is deleted from target workspaces
			for _, targetWS := range targetWorkspaces {
				targetClient := env.TestClient.DynamicFor(logicalcluster.Name(targetWS), gvr)
				
				env.Eventually(func() (bool, string) {
					_, err := targetClient.Get(ctx, createdResource.GetName(), metav1.GetOptions{})
					if err != nil {
						// Expect NotFound error
						return true, ""
					}
					return false, "resource still exists in target workspace"
				}, fmt.Sprintf("resource to be deleted from workspace %s", targetWS))
			}
			
			t.Logf("Successfully completed resource synchronization test: %s", testName)
		})
	}
}

func TestTMCResourceSyncPolicyValidation(t *testing.T) {
	t.Parallel()
	e2eframework.Suite(t, "integration")

	parentWorkspace := logicalcluster.Name("root:default")
	
	env, err := integrationframework.NewTestEnvironment(t, "TMCResourceSyncPolicyValidation", parentWorkspace)
	require.NoError(t, err, "Failed to create test environment")

	ctx := env.Context()
	workspaceCluster := env.WorkspaceCluster()
	
	namespace, err := env.CreateTestNamespace("sync-policy-validation")
	require.NoError(t, err, "Failed to create test namespace")

	syncPolicyGVR := schema.GroupVersionResource{
		Group:    "tmc.kcp.io",
		Version:  "v1alpha1",
		Resource: "resourcesyncpolicies",
	}
	
	syncPolicyClient := env.TestClient.DynamicFor(workspaceCluster, syncPolicyGVR)

	validationTests := map[string]struct {
		policySpec      map[string]interface{}
		shouldSucceed   bool
		expectedError   string
	}{
		"valid sync policy with all required fields": {
			policySpec: map[string]interface{}{
				"sourceWorkspace": "workspace-a",
				"targetWorkspaces": []string{"workspace-b", "workspace-c"},
				"syncMode": "bidirectional",
				"conflictResolution": "sourceWins",
				"selector": map[string]interface{}{
					"matchLabels": map[string]interface{}{
						"sync-enabled": "true",
					},
				},
			},
			shouldSucceed: true,
		},
		"sync policy missing source workspace": {
			policySpec: map[string]interface{}{
				"targetWorkspaces": []string{"workspace-b"},
				"syncMode": "bidirectional",
			},
			shouldSucceed: false,
			expectedError: "sourceWorkspace",
		},
		"sync policy missing target workspaces": {
			policySpec: map[string]interface{}{
				"sourceWorkspace": "workspace-a",
				"syncMode": "bidirectional",
			},
			shouldSucceed: false,
			expectedError: "targetWorkspaces",
		},
		"sync policy with invalid sync mode": {
			policySpec: map[string]interface{}{
				"sourceWorkspace": "workspace-a",
				"targetWorkspaces": []string{"workspace-b"},
				"syncMode": "invalid-mode",
			},
			shouldSucceed: false,
			expectedError: "syncMode",
		},
	}

	for testName, tc := range validationTests {
		t.Run(testName, func(t *testing.T) {
			policyName := env.TestClient.WithTestPrefix("validation-sync-policy")
			
			policyObj := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "tmc.kcp.io/v1alpha1",
					"kind":       "ResourceSyncPolicy",
					"metadata": map[string]interface{}{
						"name":      policyName,
						"namespace": namespace.Name,
					},
					"spec": tc.policySpec,
				},
			}
			
			_, err := syncPolicyClient.Create(ctx, policyObj, metav1.CreateOptions{})
			
			if tc.shouldSucceed {
				require.NoError(t, err, "Valid sync policy should succeed")
				
				// Clean up
				err = syncPolicyClient.Delete(ctx, policyName, metav1.DeleteOptions{})
				require.NoError(t, err, "Failed to delete test sync policy")
			} else {
				require.Error(t, err, "Invalid sync policy should fail")
				require.Contains(t, err.Error(), tc.expectedError, "Error should mention the expected validation failure")
			}
		})
	}
}

// createResourceSyncPolicy creates a ResourceSyncPolicy for testing
func createResourceSyncPolicy(name, namespace, sourceWS string, targetWSs []string, policy map[string]interface{}) *unstructured.Unstructured {
	spec := map[string]interface{}{
		"sourceWorkspace":  sourceWS,
		"targetWorkspaces": targetWSs,
	}
	
	// Merge policy configuration into spec
	for k, v := range policy {
		spec[k] = v
	}
	
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "tmc.kcp.io/v1alpha1",
			"kind":       "ResourceSyncPolicy",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
				"labels": map[string]interface{}{
					"test-suite": "tmc-integration",
					"test-type":  "resource-sync",
				},
			},
			"spec": spec,
		},
	}
}

// getGVRFromResource extracts GVR from an unstructured resource
func getGVRFromResource(obj *unstructured.Unstructured) schema.GroupVersionResource {
	gv, err := schema.ParseGroupVersion(obj.GetAPIVersion())
	if err != nil {
		// Fallback for core resources
		gv = schema.GroupVersion{Version: "v1"}
	}
	
	kind := obj.GetKind()
	resource := ""
	
	switch kind {
	case "Deployment":
		resource = "deployments"
	case "ConfigMap":
		resource = "configmaps"
	case "Service":
		resource = "services"
	case "Secret":
		resource = "secrets"
	default:
		// Convert kind to lowercase plural (simple heuristic)
		resource = strings.ToLower(kind) + "s"
	}
	
	return gv.WithResource(resource)
}