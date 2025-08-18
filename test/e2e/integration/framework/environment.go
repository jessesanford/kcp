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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"

	"github.com/kcp-dev/logicalcluster/v3"

	tenancyv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/tenancy/v1alpha1"
	e2eframework "github.com/kcp-dev/kcp/test/e2e/framework"
)


// TestEnvironment provides isolated test environments for TMC integration testing.
type TestEnvironment struct {
	t        *testing.T
	ctx      context.Context
	cancel   context.CancelFunc
	
	// Core components
	TestClient  *TestClient
	Workspace   *tenancyv1alpha1.Workspace
	Config      *rest.Config
	
	// Test metadata
	TestName    string
	TestPrefix  string
	Namespace   string
	
	// Cleanup functions
	cleanupFunctions []func() error
}

// NewTestEnvironment creates an isolated test environment for TMC integration testing.
func NewTestEnvironment(t *testing.T, testName string, parentWorkspace logicalcluster.Name) (*TestEnvironment, error) {
	t.Helper()
	
	ctx, cancel := context.WithTimeout(context.Background(), TestTimeout)
	
	// Sanitize test name for use in resource names
	sanitizedName := strings.ToLower(strings.ReplaceAll(testName, "_", "-"))
	testPrefix := fmt.Sprintf("it-%s-", sanitizedName)
	
	// Get KCP server configuration
	server := e2eframework.SharedKcpServer(t)
	config := server.BaseConfig(t)
	
	env := &TestEnvironment{
		t:              t,
		ctx:            ctx,
		cancel:         cancel,
		Config:         config,
		TestName:       testName,
		TestPrefix:     testPrefix,
		Namespace:      IntegrationTestNamespace,
		cleanupFunctions: make([]func() error, 0),
	}
	
	// Initialize test client
	testClient, err := NewTestClient(t, config, parentWorkspace, testPrefix, DefaultTestPortBase)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create test client: %w", err)
	}
	env.TestClient = testClient
	
	// Create isolated workspace for this test
	workspace, err := env.createTestWorkspace(parentWorkspace, sanitizedName)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create test workspace: %w", err)
	}
	env.Workspace = workspace
	
	// Schedule cleanup
	t.Cleanup(env.Cleanup)
	
	t.Logf("Created test environment for %s with workspace %s", testName, workspace.Name)
	return env, nil
}

// createTestWorkspace creates an isolated workspace for this test
func (te *TestEnvironment) createTestWorkspace(parentWorkspace logicalcluster.Name, sanitizedName string) (*tenancyv1alpha1.Workspace, error) {
	te.t.Helper()
	
	workspaceName := te.TestPrefix + sanitizedName + "-ws"
	
	workspace := &tenancyv1alpha1.Workspace{
		ObjectMeta: metav1.ObjectMeta{
			Name: workspaceName,
			Labels: map[string]string{
				TestSuiteLabelKey:     TestSuiteLabelValue,
				TestRunLabelKey:       te.TestName,
				TestWorkspaceLabelKey: "true",
			},
			Annotations: map[string]string{
				TestNameAnnotationKey:    te.TestName,
				TestCreatedAnnotationKey: time.Now().Format(time.RFC3339),
			},
		},
		Spec: tenancyv1alpha1.WorkspaceSpec{
			Type: tenancyv1alpha1.WorkspaceTypeReference{
				Name: UniversalWorkspaceType,
				Path: RootWorkspacePath,
			},
		},
	}
	
	client := te.TestClient.ClusterFor(parentWorkspace)
	createdWorkspace, err := client.TenancyV1alpha1().Workspaces().Create(te.ctx, workspace, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create test workspace %s: %w", workspaceName, err)
	}
	
	// Wait for workspace to be ready
	err = te.waitForWorkspaceReady(parentWorkspace, createdWorkspace.Name)
	if err != nil {
		return nil, fmt.Errorf("workspace %s did not become ready: %w", createdWorkspace.Name, err)
	}
	
	// Add workspace cleanup
	te.AddCleanup(func() error {
		return client.TenancyV1alpha1().Workspaces().Delete(context.Background(), createdWorkspace.Name, metav1.DeleteOptions{})
	})
	
	return createdWorkspace, nil
}

// waitForWorkspaceReady waits for the workspace to reach Ready phase
func (te *TestEnvironment) waitForWorkspaceReady(parentWorkspace logicalcluster.Name, workspaceName string) error {
	te.t.Helper()
	
	client := te.TestClient.ClusterFor(parentWorkspace)
	
	return wait.PollUntilContextTimeout(te.ctx, TestPollInterval, TestTimeout, true, func(ctx context.Context) (bool, error) {
		workspace, err := client.TenancyV1alpha1().Workspaces().Get(ctx, workspaceName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		
		if workspace.Status.Phase == tenancyv1alpha1.WorkspacePhaseReady {
			te.t.Logf("Workspace %s is ready", workspaceName)
			return true, nil
		}
		
		te.t.Logf("Workspace %s is in phase %s, waiting...", workspaceName, workspace.Status.Phase)
		return false, nil
	})
}

// CreateTestNamespace creates a namespace within the test workspace for resource isolation.
// All test resources should be created in namespaces created through this method.
func (te *TestEnvironment) CreateTestNamespace(namespaceSuffix string) (*corev1.Namespace, error) {
	te.t.Helper()
	
	namespaceName := te.TestPrefix + namespaceSuffix
	workspaceCluster := logicalcluster.Name(te.Workspace.Status.URL)
	
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespaceName,
			Labels: map[string]string{
				TestSuiteLabelKey:      TestSuiteLabelValue,
				TestRunLabelKey:        te.TestName,
				TestNamespaceLabelKey:  "true",
			},
		},
	}
	
	kubeClient := te.TestClient.KubeClient
	createdNamespace, err := kubeClient.CoreV1().Namespaces().Create(te.ctx, namespace, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create test namespace %s: %w", namespaceName, err)
	}
	
	// Add namespace cleanup
	te.AddCleanup(func() error {
		return kubeClient.CoreV1().Namespaces().Delete(context.Background(), createdNamespace.Name, metav1.DeleteOptions{})
	})
	
	te.t.Logf("Created test namespace %s in workspace %s", namespaceName, workspaceCluster)
	return createdNamespace, nil
}

// AddCleanup registers a cleanup function to be called when the test environment is torn down.
// Cleanup functions are called in reverse order (LIFO) to ensure proper dependency cleanup.
func (te *TestEnvironment) AddCleanup(cleanup func() error) {
	te.cleanupFunctions = append([]func() error{cleanup}, te.cleanupFunctions...)
}

// Eventually is a helper for polling operations with the test environment's timeout settings.
// It provides consistent timeout and polling intervals across all integration tests.
func (te *TestEnvironment) Eventually(condition func() (bool, string), description string) {
	te.t.Helper()
	
	err := wait.PollUntilContextTimeout(te.ctx, TestPollInterval, TestTimeout, true, func(ctx context.Context) (bool, error) {
		satisfied, reason := condition()
		if !satisfied && reason != "" {
			te.t.Logf("Waiting for %s: %s", description, reason)
		}
		return satisfied, nil
	})
	
	require.NoError(te.t, err, "Timed out waiting for %s", description)
}

// Context returns the test context with timeout
func (te *TestEnvironment) Context() context.Context {
	return te.ctx
}

// WorkspaceCluster returns the logical cluster for the test workspace
func (te *TestEnvironment) WorkspaceCluster() logicalcluster.Name {
	if te.Workspace == nil || te.Workspace.Status.URL == "" {
		return logicalcluster.Name("")
	}
	return logicalcluster.Name(te.Workspace.Status.URL)
}

// Cleanup performs cleanup of all resources created by this test environment.
// This method is automatically registered with t.Cleanup() and will be called when the test completes.
func (te *TestEnvironment) Cleanup() {
	te.t.Helper()
	
	te.t.Logf("Starting cleanup for test environment %s", te.TestName)
	
	// Execute all cleanup functions in reverse order (LIFO)
	for i := len(te.cleanupFunctions) - 1; i >= 0; i-- {
		cleanup := te.cleanupFunctions[i]
		if err := cleanup(); err != nil {
			te.t.Logf("Warning: cleanup function failed: %v", err)
		}
	}
	
	// Clean up test client resources
	if te.TestClient != nil {
		if err := te.TestClient.CleanupTestResources(context.Background()); err != nil {
			te.t.Logf("Warning: failed to cleanup test client resources: %v", err)
		}
	}
	
	// Cancel the context
	if te.cancel != nil {
		te.cancel()
	}
	
	te.t.Logf("Completed cleanup for test environment %s", te.TestName)
}