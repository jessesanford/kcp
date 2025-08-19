// Package tmc provides a comprehensive testing framework for TMC components,
// following KCP testing patterns and ensuring workspace isolation.
package tmc

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"

	"github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
	"github.com/kcp-dev/kcp/test/e2e/framework"
)

// TestContext provides a comprehensive testing environment for TMC components
type TestContext struct {
	t      *testing.T
	ctx    context.Context
	cancel context.CancelFunc

	// KCP connection details
	cfg           *rest.Config
	clusterClient cluster.ClusterInterface

	// Workspace configuration
	orgClusterName   logicalcluster.Name
	workspaceName    string
	workspaceCluster logicalcluster.Name

	// Test configuration
	testNamespace string
	timeout       time.Duration
}

// NewTestContext creates a new TMC test context with workspace isolation
func NewTestContext(t *testing.T) *TestContext {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)

	cfg := framework.ConfigOrDie()
	clusterClient, err := cluster.NewForConfig(cfg)
	require.NoError(t, err, "Failed to create cluster client")

	return &TestContext{
		t:             t,
		ctx:           ctx,
		cancel:        cancel,
		cfg:           cfg,
		clusterClient: clusterClient,
		timeout:       10 * time.Minute,
		testNamespace: "default",
	}
}

// SetupWorkspace creates an isolated workspace for testing TMC components
func (tc *TestContext) SetupWorkspace(workspaceName string) error {
	tc.t.Helper()

	tc.workspaceName = workspaceName

	// Create organization workspace if it doesn't exist
	orgPath := framework.NewOrganizationFixture(tc.t, tc.cfg)
	tc.orgClusterName = orgPath.Join("default")

	// Create test workspace within organization
	workspacePath := framework.NewWorkspaceFixture(tc.t, tc.cfg, orgPath, framework.WithName(workspaceName))
	tc.workspaceCluster = workspacePath

	tc.t.Logf("Created test workspace: %s", tc.workspaceCluster)
	return nil
}

// Cleanup performs test cleanup
func (tc *TestContext) Cleanup() {
	if tc.cancel != nil {
		tc.cancel()
	}
}

// Eventually provides KCP-aware eventually assertions
func (tc *TestContext) Eventually(condition func() (bool, error), msgAndArgs ...interface{}) {
	tc.t.Helper()

	err := wait.PollImmediate(100*time.Millisecond, tc.timeout, condition)
	if err != nil {
		tc.t.Fatalf("Eventually condition failed: %v, %v", err, msgAndArgs)
	}
}

// EventuallyWithContext provides context-aware eventually assertions
func (tc *TestContext) EventuallyWithContext(ctx context.Context, condition func(context.Context) (bool, error), msgAndArgs ...interface{}) {
	tc.t.Helper()

	err := wait.PollImmediateWithContext(ctx, 100*time.Millisecond, tc.timeout, condition)
	if err != nil {
		tc.t.Fatalf("Eventually condition with context failed: %v, %v", err, msgAndArgs)
	}
}

// WaitForWorkspaceReady waits for workspace to be ready for TMC testing
func (tc *TestContext) WaitForWorkspaceReady() error {
	tc.t.Helper()

	klog.V(2).Infof("Waiting for workspace %s to be ready", tc.workspaceCluster)

	tc.Eventually(func() (bool, error) {
		workspace, err := tc.clusterClient.Cluster(tc.orgClusterName.Path()).
			TenancyV1alpha1().
			Workspaces().
			Get(tc.ctx, tc.workspaceName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		// Check if workspace is ready
		for _, condition := range workspace.Status.Conditions {
			if condition.Type == "Ready" && condition.Status == metav1.ConditionTrue {
				return true, nil
			}
		}

		return false, nil
	}, "Workspace should become ready")

	return nil
}

// CreateTestNamespace creates a namespace in the test workspace
func (tc *TestContext) CreateTestNamespace(name string) error {
	tc.t.Helper()

	tc.testNamespace = name

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	_, err := tc.clusterClient.Cluster(tc.workspaceCluster.Path()).
		CoreV1().
		Namespaces().
		Create(tc.ctx, namespace, metav1.CreateOptions{})

	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create test namespace %s: %w", name, err)
	}

	tc.t.Logf("Created test namespace: %s in workspace %s", name, tc.workspaceCluster)
	return nil
}

// ValidateWorkspaceIsolation ensures workspace isolation is maintained during TMC testing
func (tc *TestContext) ValidateWorkspaceIsolation() error {
	tc.t.Helper()

	// Verify we can only see resources in our workspace
	workspaces, err := tc.clusterClient.Cluster(tc.orgClusterName.Path()).
		TenancyV1alpha1().
		Workspaces().
		List(tc.ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list workspaces for isolation validation: %w", err)
	}

	// We should see at least our workspace
	found := false
	for _, ws := range workspaces.Items {
		if ws.Name == tc.workspaceName {
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("workspace isolation failed: cannot see own workspace %s", tc.workspaceName)
	}

	tc.t.Logf("Workspace isolation validated for %s", tc.workspaceCluster)
	return nil
}

// GetClusterClient returns the cluster client for the test workspace
func (tc *TestContext) GetClusterClient() cluster.ClusterInterface {
	return tc.clusterClient
}

// GetWorkspaceCluster returns the workspace cluster name
func (tc *TestContext) GetWorkspaceCluster() logicalcluster.Name {
	return tc.workspaceCluster
}

// GetContext returns the test context
func (tc *TestContext) GetContext() context.Context {
	return tc.ctx
}

// GetTestNamespace returns the test namespace name
func (tc *TestContext) GetTestNamespace() string {
	return tc.testNamespace
}

// TMCTestSuite represents a test suite for TMC components
type TMCTestSuite struct {
	Name         string
	Description  string
	TestCases    []TMCTestCase
	SetupFunc    func(*TestContext) error
	TeardownFunc func(*TestContext) error
}

// TMCTestCase represents an individual TMC test case
type TMCTestCase struct {
	Name        string
	Description string
	TestFunc    func(*TestContext) error
	Timeout     time.Duration
	Parallel    bool
}

// RunTMCTestSuite runs a complete TMC test suite with proper workspace isolation
func RunTMCTestSuite(t *testing.T, suite TMCTestSuite) {
	t.Helper()

	t.Run(suite.Name, func(t *testing.T) {
		ctx := NewTestContext(t)
		defer ctx.Cleanup()

		// Setup workspace for the test suite
		workspaceName := fmt.Sprintf("test-%s-%d", suite.Name, time.Now().Unix())
		require.NoError(t, ctx.SetupWorkspace(workspaceName))
		require.NoError(t, ctx.WaitForWorkspaceReady())
		require.NoError(t, ctx.ValidateWorkspaceIsolation())

		// Run suite setup if provided
		if suite.SetupFunc != nil {
			require.NoError(t, suite.SetupFunc(ctx), "Suite setup failed")
		}

		// Run test cases
		for _, testCase := range suite.TestCases {
			tc := testCase // Capture for closure

			t.Run(tc.Name, func(t *testing.T) {
				if tc.Parallel {
					t.Parallel()
				}

				// Create test context for this specific test
				testCtx := NewTestContext(t)
				defer testCtx.Cleanup()

				// Inherit workspace from suite
				testCtx.workspaceCluster = ctx.workspaceCluster
				testCtx.orgClusterName = ctx.orgClusterName
				testCtx.workspaceName = ctx.workspaceName

				if tc.Timeout > 0 {
					var cancel context.CancelFunc
					testCtx.ctx, cancel = context.WithTimeout(testCtx.ctx, tc.Timeout)
					defer cancel()
				}

				// Run the test case
				require.NoError(t, tc.TestFunc(testCtx), "Test case %s failed", tc.Name)
			})
		}

		// Run suite teardown if provided
		if suite.TeardownFunc != nil {
			require.NoError(t, suite.TeardownFunc(ctx), "Suite teardown failed")
		}
	})
}
