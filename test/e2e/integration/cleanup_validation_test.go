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
	"time"

	"github.com/stretchr/testify/require"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/kcp-dev/logicalcluster/v3"

	e2eframework "github.com/kcp-dev/kcp/test/e2e/framework"
	integrationframework "github.com/kcp-dev/kcp/test/e2e/integration/framework"
)

func TestTMCResourceCleanupValidation(t *testing.T) {
	t.Parallel()
	e2eframework.Suite(t, "integration")

	parentWorkspace := logicalcluster.Name("root:default")
	
	env, err := integrationframework.NewTestEnvironment(t, "TMCResourceCleanupValidation", parentWorkspace)
	require.NoError(t, err, "Failed to create test environment")

	tests := map[string]struct {
		resourceType       string
		dependentResources []map[string]interface{}
		cleanupPolicy      map[string]interface{}
		expectOrphanedResources bool
		cascadeDelete      bool
	}{
		"deployment with dependent resources cleanup": {
			resourceType: "deployment",
			dependentResources: []map[string]interface{}{
				{
					"type": "service",
					"name": "test-service",
				},
				{
					"type": "configmap", 
					"name": "test-config",
				},
			},
			cleanupPolicy: map[string]interface{}{
				"cascadeDelete": true,
				"deletionOrder": []string{"service", "configmap", "deployment"},
			},
			expectOrphanedResources: false,
			cascadeDelete: true,
		},
		"service cleanup without cascade": {
			resourceType: "service",
			dependentResources: []map[string]interface{}{
				{
					"type": "endpoint",
					"name": "test-endpoint",
				},
			},
			cleanupPolicy: map[string]interface{}{
				"cascadeDelete": false,
				"orphanDependents": true,
			},
			expectOrphanedResources: true,
			cascadeDelete: false,
		},
		"cross-workspace resource cleanup": {
			resourceType: "placement",
			dependentResources: []map[string]interface{}{
				{
					"type": "workload",
					"name": "cross-ws-workload",
					"workspace": "target-ws",
				},
			},
			cleanupPolicy: map[string]interface{}{
				"cascadeDelete": true,
				"crossWorkspaceCleanup": true,
			},
			expectOrphanedResources: false,
			cascadeDelete: true,
		},
	}

	for testName, tc := range tests {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()
			
			ctx := env.Context()
			workspaceCluster := env.WorkspaceCluster()
			
			// Create test namespace
			namespace, err := env.CreateTestNamespace("cleanup-test")
			require.NoError(t, err, "Failed to create test namespace")
			
			// Create target workspace if needed for cross-workspace tests
			var targetWS string
			if tc.resourceType == "placement" {
				targetWS, err = createTestWorkspace(env, "cleanup-target")
				require.NoError(t, err, "Failed to create target workspace for cleanup test")
			}
			
			// Create cleanup policy
			t.Logf("Creating cleanup policy for %s", tc.resourceType)
			
			cleanupPolicy := createCleanupPolicy(
				env.TestClient.WithTestPrefix("cleanup-policy"),
				namespace.Name,
				tc.resourceType,
				tc.cleanupPolicy,
			)
			
			policyGVR := schema.GroupVersionResource{
				Group:    "tmc.kcp.io",
				Version:  "v1alpha1",
				Resource: "cleanuppolicies",
			}
			
			policyClient := env.TestClient.DynamicFor(workspaceCluster, policyGVR)
			
			createdPolicy, err := policyClient.Create(ctx, cleanupPolicy, metav1.CreateOptions{})
			require.NoError(t, err, "Failed to create cleanup policy")
			require.NotNil(t, createdPolicy, "Created cleanup policy should not be nil")
			
			// Wait for cleanup policy to become active
			env.Eventually(func() (bool, string) {
				policy, err := policyClient.Get(ctx, createdPolicy.GetName(), metav1.GetOptions{})
				if err != nil {
					return false, fmt.Sprintf("failed to get cleanup policy: %v", err)
				}
				
				phase, found, err := unstructured.NestedString(policy.Object, "status", "phase")
				if err != nil || !found {
					return false, "cleanup policy phase not found"
				}
				
				return phase == "Active", fmt.Sprintf("cleanup policy in phase %s", phase)
			}, "cleanup policy to become active")
			
			// Create primary resource
			t.Logf("Creating primary resource: %s", tc.resourceType)
			
			primaryResource := createCleanupTestResource(
				env.TestClient.WithTestPrefix("cleanup-primary"),
				namespace.Name,
				tc.resourceType,
			)
			
			primaryGVR := getGVRFromResourceType(tc.resourceType)
			primaryClient := env.TestClient.DynamicFor(workspaceCluster, primaryGVR)
			
			if tc.resourceType == "placement" && targetWS != "" {
				// Set target workspace for placement resource
				err = unstructured.SetNestedField(primaryResource.Object, targetWS, "spec", "targetWorkspace")
				require.NoError(t, err, "Failed to set target workspace in placement")
			}
			
			createdPrimary, err := primaryClient.Create(ctx, primaryResource, metav1.CreateOptions{})
			require.NoError(t, err, "Failed to create primary resource")
			require.NotNil(t, createdPrimary, "Created primary resource should not be nil")
			
			// Create dependent resources
			t.Logf("Creating %d dependent resources", len(tc.dependentResources))
			
			createdDependents := make([]*unstructured.Unstructured, len(tc.dependentResources))
			
			for i, depSpec := range tc.dependentResources {
				depName := env.TestClient.WithTestPrefix(depSpec["name"].(string))
				depType := depSpec["type"].(string)
				
				depResource := createCleanupTestResource(depName, namespace.Name, depType)
				
				// Add owner reference to primary resource
				addOwnerReference(depResource, createdPrimary)
				
				depGVR := getGVRFromResourceType(depType)
				
				// Handle cross-workspace dependents
				var depClient dynamic.ResourceInterface
				if wsName, hasWS := depSpec["workspace"]; hasWS && wsName.(string) == "target-ws" && targetWS != "" {
					depClient = env.TestClient.DynamicFor(logicalcluster.Name(targetWS), depGVR)
				} else {
					depClient = env.TestClient.DynamicFor(workspaceCluster, depGVR)
				}
				
				createdDep, err := depClient.Create(ctx, depResource, metav1.CreateOptions{})
				require.NoError(t, err, "Failed to create dependent resource %s", depName)
				
				createdDependents[i] = createdDep
				
				t.Logf("Created dependent resource: %s/%s", depType, depName)
			}
			
			// Verify all resources exist before cleanup
			t.Logf("Verifying all resources exist before cleanup")
			
			_, err = primaryClient.Get(ctx, createdPrimary.GetName(), metav1.GetOptions{})
			require.NoError(t, err, "Primary resource should exist before cleanup")
			
			for i, dep := range createdDependents {
				depSpec := tc.dependentResources[i]
				depGVR := getGVRFromResourceType(depSpec["type"].(string))
				
				var depClient dynamic.ResourceInterface
				if wsName, hasWS := depSpec["workspace"]; hasWS && wsName.(string) == "target-ws" && targetWS != "" {
					depClient = env.TestClient.DynamicFor(logicalcluster.Name(targetWS), depGVR)
				} else {
					depClient = env.TestClient.DynamicFor(workspaceCluster, depGVR)
				}
				
				_, err := depClient.Get(ctx, dep.GetName(), metav1.GetOptions{})
				require.NoError(t, err, "Dependent resource %s should exist before cleanup", dep.GetName())
			}
			
			// Test resource deletion with cleanup validation
			t.Logf("Deleting primary resource to trigger cleanup")
			
			deletionOptions := metav1.DeleteOptions{}
			if tc.cascadeDelete {
				policy := metav1.DeletePropagationForeground
				deletionOptions.PropagationPolicy = &policy
			} else {
				policy := metav1.DeletePropagationOrphan
				deletionOptions.PropagationPolicy = &policy
			}
			
			err = primaryClient.Delete(ctx, createdPrimary.GetName(), deletionOptions)
			require.NoError(t, err, "Failed to delete primary resource")
			
			// Wait for primary resource to be deleted
			env.Eventually(func() (bool, string) {
				_, err := primaryClient.Get(ctx, createdPrimary.GetName(), metav1.GetOptions{})
				if err != nil {
					// Expect NotFound error
					return true, ""
				}
				return false, "primary resource still exists"
			}, "primary resource to be deleted")
			
			// Validate cleanup behavior based on policy
			t.Logf("Validating cleanup behavior - expecting orphaned resources: %v", tc.expectOrphanedResources)
			
			// Give cleanup controller time to process
			time.Sleep(2 * time.Second)
			
			for i, dep := range createdDependents {
				depSpec := tc.dependentResources[i]
				depGVR := getGVRFromResourceType(depSpec["type"].(string))
				
				var depClient dynamic.ResourceInterface
				if wsName, hasWS := depSpec["workspace"]; hasWS && wsName.(string) == "target-ws" && targetWS != "" {
					depClient = env.TestClient.DynamicFor(logicalcluster.Name(targetWS), depGVR)
				} else {
					depClient = env.TestClient.DynamicFor(workspaceCluster, depGVR)
				}
				
				_, err := depClient.Get(ctx, dep.GetName(), metav1.GetOptions{})
				
				if tc.expectOrphanedResources {
					require.NoError(t, err, "Dependent resource %s should still exist (orphaned)", dep.GetName())
					
					// Clean up orphaned resource
					err = depClient.Delete(ctx, dep.GetName(), metav1.DeleteOptions{})
					require.NoError(t, err, "Failed to clean up orphaned resource %s", dep.GetName())
				} else {
					require.Error(t, err, "Dependent resource %s should be deleted with cascade", dep.GetName())
				}
			}
			
			// Validate cleanup policy status
			t.Logf("Validating cleanup policy status")
			
			finalPolicy, err := policyClient.Get(ctx, createdPolicy.GetName(), metav1.GetOptions{})
			require.NoError(t, err, "Failed to get cleanup policy for status validation")
			
			cleanupCount, found, err := unstructured.NestedInt64(finalPolicy.Object, "status", "cleanupCount")
			require.NoError(t, err, "Failed to get cleanup count")
			require.True(t, found, "Cleanup count should be present")
			require.Greater(t, cleanupCount, int64(0), "Cleanup count should be greater than 0")
			
			lastCleanup, found, err := unstructured.NestedString(finalPolicy.Object, "status", "lastCleanup")
			require.NoError(t, err, "Failed to get last cleanup timestamp")
			require.True(t, found, "Last cleanup timestamp should be present")
			require.NotEmpty(t, lastCleanup, "Last cleanup timestamp should not be empty")
			
			t.Logf("Successfully completed resource cleanup validation test: %s", testName)
		})
	}
}

func TestTMCOrphanedResourceDetection(t *testing.T) {
	t.Parallel()
	e2eframework.Suite(t, "integration")

	parentWorkspace := logicalcluster.Name("root:default")
	
	env, err := integrationframework.NewTestEnvironment(t, "TMCOrphanedResourceDetection", parentWorkspace)
	require.NoError(t, err, "Failed to create test environment")

	ctx := env.Context()
	workspaceCluster := env.WorkspaceCluster()
	
	// Create test namespace
	namespace, err := env.CreateTestNamespace("orphan-detection")
	require.NoError(t, err, "Failed to create test namespace")

	// Create orphan detection policy
	t.Logf("Creating orphan detection policy")
	
	detectionPolicy := createOrphanDetectionPolicy(
		env.TestClient.WithTestPrefix("orphan-detection-policy"),
		namespace.Name,
	)
	
	policyGVR := schema.GroupVersionResource{
		Group:    "tmc.kcp.io",
		Version:  "v1alpha1",
		Resource: "orphandetectionpolicies",
	}
	
	policyClient := env.TestClient.DynamicFor(workspaceCluster, policyGVR)
	
	createdPolicy, err := policyClient.Create(ctx, detectionPolicy, metav1.CreateOptions{})
	require.NoError(t, err, "Failed to create orphan detection policy")
	
	// Create resources that will become orphaned
	t.Logf("Creating resources that will become orphaned")
	
	// Create a service without a corresponding deployment
	orphanService := createCleanupTestResource(
		env.TestClient.WithTestPrefix("orphan-service"),
		namespace.Name,
		"service",
	)
	
	serviceGVR := getGVRFromResourceType("service")
	serviceClient := env.TestClient.DynamicFor(workspaceCluster, serviceGVR)
	
	createdService, err := serviceClient.Create(ctx, orphanService, metav1.CreateOptions{})
	require.NoError(t, err, "Failed to create orphan service")
	
	// Create a configmap with a dangling reference
	orphanConfigMap := createCleanupTestResource(
		env.TestClient.WithTestPrefix("orphan-configmap"),
		namespace.Name,
		"configmap",
	)
	
	// Add a fake owner reference that doesn't exist
	fakeOwner := metav1.OwnerReference{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Name:       "nonexistent-deployment",
		UID:        "fake-uid-12345",
	}
	
	ownerRefs := []interface{}{
		map[string]interface{}{
			"apiVersion": fakeOwner.APIVersion,
			"kind":       fakeOwner.Kind,
			"name":       fakeOwner.Name,
			"uid":        fakeOwner.UID,
		},
	}
	
	err = unstructured.SetNestedSlice(orphanConfigMap.Object, ownerRefs, "metadata", "ownerReferences")
	require.NoError(t, err, "Failed to set fake owner reference")
	
	configMapGVR := getGVRFromResourceType("configmap")
	configMapClient := env.TestClient.DynamicFor(workspaceCluster, configMapGVR)
	
	createdConfigMap, err := configMapClient.Create(ctx, orphanConfigMap, metav1.CreateOptions{})
	require.NoError(t, err, "Failed to create orphan configmap")
	
	// Wait for orphan detection to run
	t.Logf("Waiting for orphan detection to complete")
	
	env.Eventually(func() (bool, string) {
		policy, err := policyClient.Get(ctx, createdPolicy.GetName(), metav1.GetOptions{})
		if err != nil {
			return false, fmt.Sprintf("failed to get orphan detection policy: %v", err)
		}
		
		orphanCount, found, err := unstructured.NestedInt64(policy.Object, "status", "orphanCount")
		if err != nil || !found {
			return false, "orphan count not found"
		}
		
		return orphanCount > 0, fmt.Sprintf("found %d orphaned resources", orphanCount)
	}, "orphan detection to find orphaned resources")
	
	// Validate orphan detection results
	t.Logf("Validating orphan detection results")
	
	finalPolicy, err := policyClient.Get(ctx, createdPolicy.GetName(), metav1.GetOptions{})
	require.NoError(t, err, "Failed to get orphan detection policy for validation")
	
	orphanedResources, found, err := unstructured.NestedSlice(finalPolicy.Object, "status", "orphanedResources")
	require.NoError(t, err, "Failed to get orphaned resources list")
	require.True(t, found, "Orphaned resources list should be present")
	require.NotEmpty(t, orphanedResources, "Should have detected orphaned resources")
	
	// Verify specific orphans are detected
	hasOrphanConfigMap := false
	
	for _, orphan := range orphanedResources {
		orphanMap, ok := orphan.(map[string]interface{})
		require.True(t, ok, "Orphan entry should be a map")
		
		name, found, err := unstructured.NestedString(orphanMap, "name")
		require.NoError(t, err, "Failed to get orphan name")
		require.True(t, found, "Orphan name should be present")
		
		if name == createdConfigMap.GetName() {
			hasOrphanConfigMap = true
			
			reason, found, err := unstructured.NestedString(orphanMap, "reason")
			require.NoError(t, err, "Failed to get orphan reason")
			require.True(t, found, "Orphan reason should be present")
			require.Contains(t, reason, "DanglingOwnerReference", "Should detect dangling owner reference")
		}
	}
	
	require.True(t, hasOrphanConfigMap, "Should have detected orphaned configmap with dangling reference")
	
	// Clean up test resources
	err = serviceClient.Delete(ctx, createdService.GetName(), metav1.DeleteOptions{})
	require.NoError(t, err, "Failed to delete test service")
	
	err = configMapClient.Delete(ctx, createdConfigMap.GetName(), metav1.DeleteOptions{})
	require.NoError(t, err, "Failed to delete test configmap")
	
	t.Logf("Successfully completed orphaned resource detection test")
}

// createCleanupPolicy creates a cleanup policy for testing
func createCleanupPolicy(name, namespace, resourceType string, policy map[string]interface{}) *unstructured.Unstructured {
	spec := map[string]interface{}{
		"resourceType": resourceType,
	}
	
	// Merge policy configuration into spec
	for k, v := range policy {
		spec[k] = v
	}
	
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "tmc.kcp.io/v1alpha1",
			"kind":       "CleanupPolicy",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
				"labels": map[string]interface{}{
					"test-suite": "tmc-integration",
					"test-type":  "cleanup-validation",
				},
			},
			"spec": spec,
		},
	}
}

// createOrphanDetectionPolicy creates an orphan detection policy for testing
func createOrphanDetectionPolicy(name, namespace string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "tmc.kcp.io/v1alpha1",
			"kind":       "OrphanDetectionPolicy",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
				"labels": map[string]interface{}{
					"test-suite": "tmc-integration",
					"test-type":  "orphan-detection",
				},
			},
			"spec": map[string]interface{}{
				"scanInterval": "30s",
				"resourceTypes": []string{
					"services",
					"configmaps",
					"secrets",
				},
				"detectionRules": []interface{}{
					map[string]interface{}{
						"type": "danglingOwnerReference",
						"enabled": true,
					},
					map[string]interface{}{
						"type": "unusedResource",
						"enabled": true,
					},
				},
			},
		},
	}
}

// createCleanupTestResource creates a resource for cleanup testing
func createCleanupTestResource(name, namespace, resourceType string) *unstructured.Unstructured {
	switch resourceType {
	case "deployment":
		return &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name":      name,
					"namespace": namespace,
					"labels": map[string]interface{}{
						"test-suite": "tmc-integration",
						"test-type":  "cleanup-test",
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
								},
							},
						},
					},
				},
			},
		}
	case "service":
		return &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Service",
				"metadata": map[string]interface{}{
					"name":      name,
					"namespace": namespace,
					"labels": map[string]interface{}{
						"test-suite": "tmc-integration",
						"test-type":  "cleanup-test",
					},
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
		return &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]interface{}{
					"name":      name,
					"namespace": namespace,
					"labels": map[string]interface{}{
						"test-suite": "tmc-integration",
						"test-type":  "cleanup-test",
					},
				},
				"data": map[string]interface{}{
					"config.yaml": "test: value",
				},
			},
		}
	default: // placement
		return &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "tmc.kcp.io/v1alpha1",
				"kind":       "WorkloadPlacement",
				"metadata": map[string]interface{}{
					"name":      name,
					"namespace": namespace,
					"labels": map[string]interface{}{
						"test-suite": "tmc-integration",
						"test-type":  "cleanup-test",
					},
				},
				"spec": map[string]interface{}{
					"sourceWorkspace": "cleanup-source",
					"clusterSelector": map[string]interface{}{
						"matchLabels": map[string]interface{}{
							"env": "test",
						},
					},
				},
			},
		}
	}
}

// addOwnerReference adds an owner reference to a resource
func addOwnerReference(resource, owner *unstructured.Unstructured) {
	ownerRef := map[string]interface{}{
		"apiVersion": owner.GetAPIVersion(),
		"kind":       owner.GetKind(),
		"name":       owner.GetName(),
		"uid":        owner.GetUID(),
	}
	
	unstructured.SetNestedSlice(resource.Object, []interface{}{ownerRef}, "metadata", "ownerReferences")
}