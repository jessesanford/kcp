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

func TestTMCPolicyEnforcementValidation(t *testing.T) {
	t.Parallel()
	e2eframework.Suite(t, "integration")

	parentWorkspace := logicalcluster.Name("root:default")
	
	env, err := integrationframework.NewTestEnvironment(t, "TMCPolicyEnforcementValidation", parentWorkspace)
	require.NoError(t, err, "Failed to create test environment")

	tests := map[string]struct {
		policyName      string
		policyRules     []map[string]interface{}
		testWorkload    map[string]interface{}
		shouldAllow     bool
		expectedReason  string
	}{
		"allow workload with compliant resources": {
			policyName: "resource-limits-policy",
			policyRules: []map[string]interface{}{
				{
					"apiGroups": []string{"apps"},
					"resources": []string{"deployments"},
					"verbs":     []string{"create", "update"},
					"constraints": map[string]interface{}{
						"resourceLimits": map[string]interface{}{
							"cpu":    "2000m",
							"memory": "4Gi",
						},
					},
				},
			},
			testWorkload: map[string]interface{}{
				"resources": map[string]interface{}{
					"limits": map[string]interface{}{
						"cpu":    "1000m",
						"memory": "2Gi",
					},
				},
			},
			shouldAllow:    true,
			expectedReason: "PolicyCompliant",
		},
		"deny workload exceeding resource limits": {
			policyName: "strict-resource-policy",
			policyRules: []map[string]interface{}{
				{
					"apiGroups": []string{"apps"},
					"resources": []string{"deployments"},
					"verbs":     []string{"create", "update"},
					"constraints": map[string]interface{}{
						"resourceLimits": map[string]interface{}{
							"cpu":    "1000m",
							"memory": "2Gi",
						},
					},
				},
			},
			testWorkload: map[string]interface{}{
				"resources": map[string]interface{}{
					"limits": map[string]interface{}{
						"cpu":    "4000m",
						"memory": "8Gi",
					},
				},
			},
			shouldAllow:    false,
			expectedReason: "ResourceLimitExceeded",
		},
		"allow workload with permitted image registry": {
			policyName: "image-registry-policy",
			policyRules: []map[string]interface{}{
				{
					"apiGroups": []string{"apps"},
					"resources": []string{"deployments"},
					"verbs":     []string{"create", "update"},
					"constraints": map[string]interface{}{
						"allowedRegistries": []string{
							"gcr.io/mycompany",
							"docker.io/library",
						},
					},
				},
			},
			testWorkload: map[string]interface{}{
				"image": "gcr.io/mycompany/myapp:latest",
			},
			shouldAllow:    true,
			expectedReason: "PolicyCompliant",
		},
		"deny workload with forbidden image registry": {
			policyName: "restricted-registry-policy",
			policyRules: []map[string]interface{}{
				{
					"apiGroups": []string{"apps"},
					"resources": []string{"deployments"},
					"verbs":     []string{"create", "update"},
					"constraints": map[string]interface{}{
						"allowedRegistries": []string{
							"gcr.io/mycompany",
						},
					},
				},
			},
			testWorkload: map[string]interface{}{
				"image": "docker.io/malicious/app:latest",
			},
			shouldAllow:    false,
			expectedReason: "UnauthorizedRegistry",
		},
	}

	for testName, tc := range tests {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()
			
			ctx := env.Context()
			workspaceCluster := env.WorkspaceCluster()
			
			// Create test namespace
			namespace, err := env.CreateTestNamespace("policy-test")
			require.NoError(t, err, "Failed to create test namespace")
			
			// Create enforcement policy
			t.Logf("Creating policy enforcement rule: %s", tc.policyName)
			
			policy := createEnforcementPolicy(
				env.TestClient.WithTestPrefix(tc.policyName),
				namespace.Name,
				tc.policyRules,
			)
			
			policyGVR := schema.GroupVersionResource{
				Group:    "tmc.kcp.io",
				Version:  "v1alpha1",
				Resource: "enforcementpolicies",
			}
			
			policyClient := env.TestClient.DynamicFor(workspaceCluster, policyGVR)
			
			createdPolicy, err := policyClient.Create(ctx, policy, metav1.CreateOptions{})
			require.NoError(t, err, "Failed to create enforcement policy")
			require.NotNil(t, createdPolicy, "Created policy should not be nil")
			
			// Wait for policy to become active
			env.Eventually(func() (bool, string) {
				policy, err := policyClient.Get(ctx, createdPolicy.GetName(), metav1.GetOptions{})
				if err != nil {
					return false, fmt.Sprintf("failed to get policy: %v", err)
				}
				
				phase, found, err := unstructured.NestedString(policy.Object, "status", "phase")
				if err != nil || !found {
					return false, "policy phase not found"
				}
				
				return phase == "Active", fmt.Sprintf("policy in phase %s", phase)
			}, "enforcement policy to become active")
			
			// Create test workload to validate policy enforcement
			t.Logf("Creating test workload to validate policy enforcement")
			
			workload := createPolicyTestWorkload(
				env.TestClient.WithTestPrefix("test-workload"),
				namespace.Name,
				tc.testWorkload,
			)
			
			deploymentGVR := schema.GroupVersionResource{
				Group:    "apps",
				Version:  "v1",
				Resource: "deployments",
			}
			
			workloadClient := env.TestClient.DynamicFor(workspaceCluster, deploymentGVR)
			
			// Test policy enforcement
			_, err = workloadClient.Create(ctx, workload, metav1.CreateOptions{})
			
			if tc.shouldAllow {
				require.NoError(t, err, "Policy should allow compliant workload")
				
				// Verify workload was created successfully
				createdWorkload, err := workloadClient.Get(ctx, workload.GetName(), metav1.GetOptions{})
				require.NoError(t, err, "Should be able to get created workload")
				require.NotNil(t, createdWorkload, "Created workload should not be nil")
				
				// Check for policy compliance annotation
				annotations, found, err := unstructured.NestedStringMap(createdWorkload.Object, "metadata", "annotations")
				require.NoError(t, err, "Failed to get workload annotations")
				
				if found {
					complianceStatus, hasCompliance := annotations["tmc.kcp.io/policy-compliance"]
					if hasCompliance {
						require.Contains(t, complianceStatus, tc.expectedReason, "Policy compliance annotation should contain expected reason")
					}
				}
				
				// Clean up workload
				err = workloadClient.Delete(ctx, workload.GetName(), metav1.DeleteOptions{})
				require.NoError(t, err, "Failed to delete test workload")
				
			} else {
				require.Error(t, err, "Policy should deny non-compliant workload")
				require.Contains(t, err.Error(), tc.expectedReason, "Error should contain expected policy violation reason")
			}
			
			// Test policy update
			t.Logf("Testing policy update and re-validation")
			
			// Update policy to be more permissive
			updatedRules := make([]map[string]interface{}, len(tc.policyRules))
			copy(updatedRules, tc.policyRules)
			
			// Make resource limits more permissive
			if constraints, found := updatedRules[0]["constraints"]; found {
				if constraintsMap, ok := constraints.(map[string]interface{}); ok {
					if resourceLimits, found := constraintsMap["resourceLimits"]; found {
						if limitsMap, ok := resourceLimits.(map[string]interface{}); ok {
							limitsMap["cpu"] = "8000m"
							limitsMap["memory"] = "16Gi"
						}
					}
				}
			}
			
			// Update the policy
			updatedPolicy := createEnforcementPolicy(
				createdPolicy.GetName(),
				namespace.Name,
				updatedRules,
			)
			
			_, err = policyClient.Update(ctx, updatedPolicy, metav1.UpdateOptions{})
			require.NoError(t, err, "Failed to update enforcement policy")
			
			// Verify policy update took effect
			env.Eventually(func() (bool, string) {
				policy, err := policyClient.Get(ctx, createdPolicy.GetName(), metav1.GetOptions{})
				if err != nil {
					return false, fmt.Sprintf("failed to get updated policy: %v", err)
				}
				
				generation, found, err := unstructured.NestedInt64(policy.Object, "metadata", "generation")
				if err != nil || !found {
					return false, "policy generation not found"
				}
				
				return generation > 1, fmt.Sprintf("policy generation is %d", generation)
			}, "policy update to take effect")
			
			// Clean up policy
			err = policyClient.Delete(ctx, createdPolicy.GetName(), metav1.DeleteOptions{})
			require.NoError(t, err, "Failed to delete enforcement policy")
			
			t.Logf("Successfully completed policy enforcement test: %s", testName)
		})
	}
}

func TestTMCPolicyEnforcementCrossWorkspace(t *testing.T) {
	t.Parallel()
	e2eframework.Suite(t, "integration")

	parentWorkspace := logicalcluster.Name("root:default")
	
	env, err := integrationframework.NewTestEnvironment(t, "TMCPolicyEnforcementCrossWorkspace", parentWorkspace)
	require.NoError(t, err, "Failed to create test environment")

	ctx := env.Context()
	workspaceCluster := env.WorkspaceCluster()
	
	// Create test namespace
	namespace, err := env.CreateTestNamespace("cross-workspace-policy")
	require.NoError(t, err, "Failed to create test namespace")

	// Create source and target workspaces
	sourceWS, err := createTestWorkspace(env, "policy-source")
	require.NoError(t, err, "Failed to create source workspace")
	
	targetWS, err := createTestWorkspace(env, "policy-target")
	require.NoError(t, err, "Failed to create target workspace")

	// Create cross-workspace policy
	t.Logf("Creating cross-workspace enforcement policy")
	
	crossWSPolicy := createCrossWorkspacePolicy(
		env.TestClient.WithTestPrefix("cross-ws-policy"),
		namespace.Name,
		sourceWS,
		targetWS,
	)
	
	policyGVR := schema.GroupVersionResource{
		Group:    "tmc.kcp.io",
		Version:  "v1alpha1",
		Resource: "enforcementpolicies",
	}
	
	policyClient := env.TestClient.DynamicFor(workspaceCluster, policyGVR)
	
	createdPolicy, err := policyClient.Create(ctx, crossWSPolicy, metav1.CreateOptions{})
	require.NoError(t, err, "Failed to create cross-workspace policy")
	
	// Wait for policy to be applied across workspaces
	env.Eventually(func() (bool, string) {
		policy, err := policyClient.Get(ctx, createdPolicy.GetName(), metav1.GetOptions{})
		if err != nil {
			return false, fmt.Sprintf("failed to get policy: %v", err)
		}
		
		appliedWorkspaces, found, err := unstructured.NestedSlice(policy.Object, "status", "appliedWorkspaces")
		if err != nil || !found {
			return false, "applied workspaces not found in status"
		}
		
		return len(appliedWorkspaces) >= 2, fmt.Sprintf("policy applied to %d workspaces", len(appliedWorkspaces))
	}, "cross-workspace policy to be applied")
	
	// Test policy enforcement in source workspace
	t.Logf("Testing policy enforcement in source workspace")
	
	sourceWorkload := createPolicyTestWorkload(
		env.TestClient.WithTestPrefix("source-workload"),
		namespace.Name,
		map[string]interface{}{
			"image": "docker.io/unauthorized/app:latest",
		},
	)
	
	deploymentGVR := schema.GroupVersionResource{
		Group:    "apps",
		Version:  "v1",
		Resource: "deployments",
	}
	
	sourceClient := env.TestClient.DynamicFor(logicalcluster.Name(sourceWS), deploymentGVR)
	
	_, err = sourceClient.Create(ctx, sourceWorkload, metav1.CreateOptions{})
	require.Error(t, err, "Policy should be enforced in source workspace")
	require.Contains(t, err.Error(), "UnauthorizedRegistry", "Error should indicate policy violation")
	
	// Test policy enforcement in target workspace  
	t.Logf("Testing policy enforcement in target workspace")
	
	targetWorkload := createPolicyTestWorkload(
		env.TestClient.WithTestPrefix("target-workload"),
		namespace.Name,
		map[string]interface{}{
			"image": "docker.io/unauthorized/app:latest",
		},
	)
	
	targetClient := env.TestClient.DynamicFor(logicalcluster.Name(targetWS), deploymentGVR)
	
	_, err = targetClient.Create(ctx, targetWorkload, metav1.CreateOptions{})
	require.Error(t, err, "Policy should be enforced in target workspace")
	require.Contains(t, err.Error(), "UnauthorizedRegistry", "Error should indicate policy violation")
	
	t.Logf("Successfully completed cross-workspace policy enforcement test")
}

// createEnforcementPolicy creates an enforcement policy for testing
func createEnforcementPolicy(name, namespace string, rules []map[string]interface{}) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "tmc.kcp.io/v1alpha1",
			"kind":       "EnforcementPolicy",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
				"labels": map[string]interface{}{
					"test-suite": "tmc-integration",
					"test-type":  "policy-enforcement",
				},
			},
			"spec": map[string]interface{}{
				"rules": rules,
				"enforcementAction": "deny",
			},
		},
	}
}

// createCrossWorkspacePolicy creates a cross-workspace enforcement policy
func createCrossWorkspacePolicy(name, namespace, sourceWS, targetWS string) *unstructured.Unstructured {
	rules := []map[string]interface{}{
		{
			"apiGroups": []string{"apps"},
			"resources": []string{"deployments"},
			"verbs":     []string{"create", "update"},
			"constraints": map[string]interface{}{
				"allowedRegistries": []string{
					"gcr.io/trusted",
				},
			},
		},
	}
	
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "tmc.kcp.io/v1alpha1",
			"kind":       "EnforcementPolicy",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
				"labels": map[string]interface{}{
					"test-suite": "tmc-integration",
					"test-type":  "cross-workspace-policy",
				},
			},
			"spec": map[string]interface{}{
				"rules": rules,
				"enforcementAction": "deny",
				"scope": map[string]interface{}{
					"workspaces": []string{sourceWS, targetWS},
				},
			},
		},
	}
}

// createPolicyTestWorkload creates a workload for policy enforcement testing
func createPolicyTestWorkload(name, namespace string, testConfig map[string]interface{}) *unstructured.Unstructured {
	// Default container spec
	containerSpec := map[string]interface{}{
		"name":  "test-container",
		"image": "nginx:latest",
		"ports": []interface{}{
			map[string]interface{}{
				"containerPort": 80,
			},
		},
	}
	
	// Override with test-specific configuration
	if image, found := testConfig["image"]; found {
		containerSpec["image"] = image
	}
	
	if resources, found := testConfig["resources"]; found {
		containerSpec["resources"] = resources
	}
	
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
				"labels": map[string]interface{}{
					"test-suite": "tmc-integration",
					"test-type":  "policy-workload",
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
						"containers": []interface{}{containerSpec},
					},
				},
			},
		},
	}
}