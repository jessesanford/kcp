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

package framework

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/kcp-dev/logicalcluster/v3"

	tenancyv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/tenancy/v1alpha1"
)

// WaitForResourceReady waits for a resource to reach the Ready condition.
// This is a common pattern in TMC integration tests where resources need to be
// validated for readiness before proceeding with tests.
func WaitForResourceReady(ctx context.Context, t *testing.T, client *TestClient, cluster logicalcluster.Name, gvr schema.GroupVersionResource, name, namespace string) error {
	t.Helper()
	
	return wait.PollUntilContextTimeout(ctx, ResourceReadyPollInterval, ResourceReadyTimeout, true, func(ctx context.Context) (bool, error) {
		dynamicClient := client.DynamicFor(cluster, gvr)
		
		var resource *unstructured.Unstructured
		var err error
		
		if namespace != "" {
			resource, err = dynamicClient.Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
		} else {
			resource, err = dynamicClient.Get(ctx, name, metav1.GetOptions{})
		}
		
		if err != nil {
			t.Logf("Error getting resource %s/%s: %v", gvr.Resource, name, err)
			return false, nil // Continue polling
		}
		
		// Check for Ready condition
		conditions, found, err := unstructured.NestedSlice(resource.Object, "status", "conditions")
		if err != nil {
			return false, err
		}
		
		if !found {
			t.Logf("Resource %s/%s has no conditions yet", gvr.Resource, name)
			return false, nil
		}
		
		for _, condition := range conditions {
			conditionMap, ok := condition.(map[string]interface{})
			if !ok {
				continue
			}
			
			condType, found, err := unstructured.NestedString(conditionMap, "type")
			if err != nil || !found || condType != ReadyConditionType {
				continue
			}
			
			status, found, err := unstructured.NestedString(conditionMap, "status")
			if err != nil || !found {
				continue
			}
			
			if status == TrueConditionStatus {
				t.Logf("Resource %s/%s is ready", gvr.Resource, name)
				return true, nil
			}
			
			reason, _, _ := unstructured.NestedString(conditionMap, "reason")
			message, _, _ := unstructured.NestedString(conditionMap, "message")
			t.Logf("Resource %s/%s not ready: %s - %s", gvr.Resource, name, reason, message)
		}
		
		return false, nil
	})
}

// WaitForResourceDeleted waits for a resource to be deleted.
// This is useful for validating cleanup operations in TMC integration tests.
func WaitForResourceDeleted(ctx context.Context, t *testing.T, client *TestClient, cluster logicalcluster.Name, gvr schema.GroupVersionResource, name, namespace string) error {
	t.Helper()
	
	return wait.PollUntilContextTimeout(ctx, ResourceDeletionPollInterval, ResourceDeletionTimeout, true, func(ctx context.Context) (bool, error) {
		dynamicClient := client.DynamicFor(cluster, gvr)
		
		var err error
		if namespace != "" {
			_, err = dynamicClient.Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
		} else {
			_, err = dynamicClient.Get(ctx, name, metav1.GetOptions{})
		}
		
		if err != nil {
			// Resource should be gone
			t.Logf("Resource %s/%s successfully deleted", gvr.Resource, name)
			return true, nil
		}
		
		t.Logf("Resource %s/%s still exists, waiting for deletion", gvr.Resource, name)
		return false, nil
	})
}

// CreateTestWorkspace creates a workspace for testing purposes.
// This helper encapsulates the common pattern of creating isolated workspaces
// for TMC integration tests.
func CreateTestWorkspace(ctx context.Context, t *testing.T, env *TestEnvironment, name string) (*tenancyv1alpha1.Workspace, error) {
	t.Helper()
	
	parentWorkspace := logicalcluster.Name(DefaultParentWorkspace)
	workspaceName := env.TestClient.WithTestPrefix(name)
	
	workspace := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "tenancy.kcp.io/v1alpha1",
			"kind":       "Workspace",
			"metadata": map[string]interface{}{
				"name": workspaceName,
				"labels": map[string]interface{}{
					TestSuiteLabelKey:      TestSuiteLabelValue,
					TestWorkspaceLabelKey:  "true",
				},
				"annotations": map[string]interface{}{
					TestNameAnnotationKey: env.TestName,
				},
			},
			"spec": map[string]interface{}{
				"type": map[string]interface{}{
					"name": UniversalWorkspaceType,
					"path": RootWorkspacePath,
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
	
	createdWS, err := wsClient.Create(ctx, workspace, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create workspace %s: %w", workspaceName, err)
	}
	
	// Wait for workspace to be ready
	err = WaitForResourceReady(ctx, t, env.TestClient, parentWorkspace, wsGVR, workspaceName, "")
	if err != nil {
		return nil, fmt.Errorf("workspace %s did not become ready: %w", workspaceName, err)
	}
	
	// Convert to typed workspace object for convenience
	ws := &tenancyv1alpha1.Workspace{
		ObjectMeta: metav1.ObjectMeta{
			Name: createdWS.GetName(),
		},
	}
	
	// Extract status URL if available
	statusURL, found, err := unstructured.NestedString(createdWS.Object, "status", "URL")
	if err == nil && found {
		ws.Status.URL = statusURL
	}
	
	// Add cleanup
	env.AddCleanup(func() error {
		return wsClient.Delete(context.Background(), workspaceName, metav1.DeleteOptions{})
	})
	
	t.Logf("Created test workspace %s with URL %s", workspaceName, ws.Status.URL)
	return ws, nil
}

// ValidateResourceConditions validates that a resource has expected conditions.
// This is a common validation pattern in TMC tests where resources should
// have specific status conditions.
func ValidateResourceConditions(t *testing.T, resource *unstructured.Unstructured, expectedConditions []ExpectedCondition) {
	t.Helper()
	
	conditions, found, err := unstructured.NestedSlice(resource.Object, "status", "conditions")
	require.NoError(t, err, "Failed to get resource conditions")
	require.True(t, found, "Resource should have conditions")
	require.NotEmpty(t, conditions, "Resource should have at least one condition")
	
	// Convert to map for easier lookup
	actualConditions := make(map[string]map[string]interface{})
	for _, condition := range conditions {
		conditionMap, ok := condition.(map[string]interface{})
		require.True(t, ok, "Condition should be a map")
		
		condType, found, err := unstructured.NestedString(conditionMap, "type")
		require.NoError(t, err, "Failed to get condition type")
		require.True(t, found, "Condition type should be present")
		
		actualConditions[condType] = conditionMap
	}
	
	// Validate expected conditions
	for _, expected := range expectedConditions {
		actualCondition, found := actualConditions[expected.Type]
		require.True(t, found, "Should have condition of type %s", expected.Type)
		
		if expected.Status != "" {
			status, found, err := unstructured.NestedString(actualCondition, "status")
			require.NoError(t, err, "Failed to get condition status")
			require.True(t, found, "Condition status should be present")
			require.Equal(t, expected.Status, status, "Condition %s status should match", expected.Type)
		}
		
		if expected.Reason != "" {
			reason, found, err := unstructured.NestedString(actualCondition, "reason")
			require.NoError(t, err, "Failed to get condition reason")
			require.True(t, found, "Condition reason should be present")
			require.Equal(t, expected.Reason, reason, "Condition %s reason should match", expected.Type)
		}
	}
}

// ExpectedCondition represents an expected condition for validation
type ExpectedCondition struct {
	Type   string
	Status string
	Reason string
}

// CreateResourceLabel creates a standardized label set for test resources.
// This ensures consistent labeling across all TMC integration test resources.
func CreateResourceLabel(testSuite, testType, testName string) map[string]string {
	return map[string]string{
		TestSuiteLabelKey:       testSuite,
		TestTypeLabelKey:        testType,
		TestNameLabelKey:        testName,
		ManagedByLabelKey:       ManagedByLabelValue,
		CleanupPolicyLabelKey:   AutomaticCleanupPolicyValue,
	}
}

// ValidateResourceLabels validates that a resource has expected labels.
// This helper ensures test resources are properly labeled for identification and cleanup.
func ValidateResourceLabels(t *testing.T, resource *unstructured.Unstructured, expectedLabels map[string]string) {
	t.Helper()
	
	actualLabels, found, err := unstructured.NestedStringMap(resource.Object, "metadata", "labels")
	require.NoError(t, err, "Failed to get resource labels")
	require.True(t, found, "Resource should have labels")
	
	for expectedKey, expectedValue := range expectedLabels {
		actualValue, found := actualLabels[expectedKey]
		require.True(t, found, "Resource should have label %s", expectedKey)
		require.Equal(t, expectedValue, actualValue, "Label %s should have expected value", expectedKey)
	}
}

// GetGVRFromKind returns the GroupVersionResource for common Kubernetes kinds.
// This helper simplifies GVR construction for standard resource types.
func GetGVRFromKind(apiVersion, kind string) (schema.GroupVersionResource, error) {
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return schema.GroupVersionResource{}, fmt.Errorf("invalid apiVersion %s: %w", apiVersion, err)
	}
	
	var resource string
	switch kind {
	case "Deployment":
		resource = "deployments"
	case "Service":
		resource = "services"
	case "ConfigMap":
		resource = "configmaps"
	case "Secret":
		resource = "secrets"
	case "Workspace":
		resource = "workspaces"
	case "ClusterRegistration":
		resource = "clusterregistrations"
	case "WorkloadPlacement":
		resource = "workloadplacements"
	case "EnforcementPolicy":
		resource = "enforcementpolicies"
	case "ResourceSyncPolicy":
		resource = "resourcesyncpolicies"
	case "StatusAggregationPolicy":
		resource = "statusaggregationpolicies"
	case "CleanupPolicy":
		resource = "cleanuppolicies"
	case "OrphanDetectionPolicy":
		resource = "orphandetectionpolicies"
	default:
		// Default conversion: kind to lowercase + "s"
		resource = fmt.Sprintf("%ss", strings.ToLower(kind))
	}
	
	return gv.WithResource(resource), nil
}