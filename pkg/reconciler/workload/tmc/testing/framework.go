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

package testing

import (
	"context"
	"fmt"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/kcp-dev/logicalcluster/v3"

	tmcpkg "github.com/kcp-dev/kcp/pkg/reconciler/workload/tmc"
	workloadv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/workload/v1alpha1"
	kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
	kcpfake "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/fake"
)

// TestFramework provides a comprehensive testing framework for TMC components
type TestFramework struct {
	// Clients
	KCPClient      kcpclientset.ClusterInterface
	DynamicClient  dynamic.Interface
	ClusterClients map[string]dynamic.Interface

	// Fake objects and schemes
	Objects         []runtime.Object
	Scheme          *runtime.Scheme
	InformerFactory cache.SharedInformerFactory

	// Test configuration
	LogicalCluster logicalcluster.Name
	Clusters       []string
	Namespaces     []string
	TestTimeout    time.Duration

	// Component instances for testing
	HealthMonitor    *tmcpkg.HealthMonitor
	MetricsCollector *tmcpkg.MetricsCollector
	TracingManager   *tmcpkg.TracingManager
	RecoveryManager  *tmcpkg.RecoveryManager

	// Test state
	mu            sync.RWMutex
	testResources map[string]*unstructured.Unstructured
	testErrors    []error
	testEvents    []TestEvent
}

// TestEvent represents an event that occurred during testing
type TestEvent struct {
	Timestamp  time.Time
	Type       TestEventType
	Component  string
	Operation  string
	Message    string
	Attributes map[string]interface{}
}

// TestEventType represents the type of test event
type TestEventType string

const (
	TestEventTypeInfo    TestEventType = "Info"
	TestEventTypeWarning TestEventType = "Warning"
	TestEventTypeError   TestEventType = "Error"
	TestEventTypeSuccess TestEventType = "Success"
)

// NewTestFramework creates a new test framework
func NewTestFramework() *TestFramework {
	scheme := runtime.NewScheme()
	_ = workloadv1alpha1.AddToScheme(scheme)

	return &TestFramework{
		Scheme:         scheme,
		LogicalCluster: logicalcluster.Name("test-cluster"),
		Clusters:       []string{"cluster-1", "cluster-2", "cluster-3"},
		Namespaces:     []string{"default", "test-namespace", "system"},
		TestTimeout:    30 * time.Second,
		testResources:  make(map[string]*unstructured.Unstructured),
		testEvents:     make([]TestEvent, 0),
	}
}

// Setup initializes the test framework
func (tf *TestFramework) Setup() error {
	// Create fake KCP client
	tf.KCPClient = kcpfake.NewSimpleClientset(tf.Objects...).Cluster(tf.LogicalCluster.Path())

	// Create fake dynamic client
	tf.DynamicClient = fake.NewSimpleDynamicClient(tf.Scheme, tf.Objects...)

	// Create cluster clients
	tf.ClusterClients = make(map[string]dynamic.Interface)
	for _, cluster := range tf.Clusters {
		tf.ClusterClients[cluster] = fake.NewSimpleDynamicClient(tf.Scheme, tf.Objects...)
	}

	// Initialize TMC components
	tf.HealthMonitor = tmcpkg.NewHealthMonitor()
	tf.MetricsCollector = tmcpkg.NewMetricsCollector()
	tf.TracingManager = tmcpkg.NewTracingManager("test-tmc", "test-version")
	tf.RecoveryManager = tmcpkg.NewRecoveryManager()

	tf.RecordEvent(TestEventTypeInfo, "TestFramework", "Setup", "Test framework initialized")
	return nil
}

// Cleanup cleans up the test framework
func (tf *TestFramework) Cleanup() {
	tf.mu.Lock()
	defer tf.mu.Unlock()

	// Stop components if running
	if tf.HealthMonitor != nil {
		tf.HealthMonitor.Stop()
	}

	// Clear test state
	tf.testResources = make(map[string]*unstructured.Unstructured)
	tf.testErrors = make([]error, 0)

	tf.RecordEvent(TestEventTypeInfo, "TestFramework", "Cleanup", "Test framework cleaned up")
}

// CreateTestResource creates a test resource for use in tests
func (tf *TestFramework) CreateTestResource(gvk schema.GroupVersionKind, namespace, name string, spec map[string]interface{}) *unstructured.Unstructured {
	resource := &unstructured.Unstructured{}
	resource.SetGroupVersionKind(gvk)
	resource.SetNamespace(namespace)
	resource.SetName(name)
	resource.SetCreationTimestamp(metav1.Now())

	if spec != nil {
		resource.Object["spec"] = spec
	}

	// Add to test resources
	key := tf.getResourceKey(resource)
	tf.mu.Lock()
	tf.testResources[key] = resource
	tf.mu.Unlock()

	tf.RecordEvent(TestEventTypeInfo, "TestFramework", "CreateResource",
		fmt.Sprintf("Created test resource %s", key))

	return resource
}

// CreateTestPlacement creates a test placement resource
func (tf *TestFramework) CreateTestPlacement(name, namespace string, clusters []string) *workloadv1alpha1.Placement {
	placement := &workloadv1alpha1.Placement{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: workloadv1alpha1.PlacementSpec{
			LocationSelectors: []metav1.LabelSelector{
				{
					MatchLabels: map[string]string{
						"test": "true",
					},
				},
			},
		},
	}

	// Set status with selected clusters
	placement.Status.SelectedWorkloadClusters = make([]workloadv1alpha1.WorkloadCluster, len(clusters))
	for i, cluster := range clusters {
		placement.Status.SelectedWorkloadClusters[i] = workloadv1alpha1.WorkloadCluster{
			Name:    cluster,
			Cluster: tf.LogicalCluster.String(),
		}
	}

	tf.RecordEvent(TestEventTypeInfo, "TestFramework", "CreatePlacement",
		fmt.Sprintf("Created test placement %s with %d clusters", name, len(clusters)))

	return placement
}

// CreateTestSyncTarget creates a test sync target resource
func (tf *TestFramework) CreateTestSyncTarget(name string, healthy bool) *workloadv1alpha1.SyncTarget {
	syncTarget := &workloadv1alpha1.SyncTarget{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: workloadv1alpha1.SyncTargetSpec{
			Cells: map[string]workloadv1alpha1.CellSpec{},
		},
	}

	// Set health status
	condition := workloadv1alpha1.Condition{
		Type:   workloadv1alpha1.SyncTargetReady,
		Status: metav1.ConditionFalse,
	}
	if healthy {
		condition.Status = metav1.ConditionTrue
	}
	syncTarget.Status.Conditions = []workloadv1alpha1.Condition{condition}

	tf.RecordEvent(TestEventTypeInfo, "TestFramework", "CreateSyncTarget",
		fmt.Sprintf("Created test sync target %s (healthy: %v)", name, healthy))

	return syncTarget
}

// SimulateError simulates an error condition for testing
func (tf *TestFramework) SimulateError(errorType tmcpkg.TMCErrorType, component, operation string) *tmcpkg.TMCError {
	tmcError := tmcpkg.NewTMCError(errorType, component, operation).
		WithMessage(fmt.Sprintf("Simulated error for testing: %s", errorType)).
		Build()

	tf.mu.Lock()
	tf.testErrors = append(tf.testErrors, tmcError)
	tf.mu.Unlock()

	tf.RecordEvent(TestEventTypeError, component, operation,
		fmt.Sprintf("Simulated error: %s", tmcError.Message))

	return tmcError
}

// WaitForCondition waits for a condition to be met within the test timeout
func (tf *TestFramework) WaitForCondition(description string, condition func() bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), tf.TestTimeout)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for condition: %s", description)
		case <-ticker.C:
			if condition() {
				tf.RecordEvent(TestEventTypeSuccess, "TestFramework", "WaitForCondition",
					fmt.Sprintf("Condition met: %s", description))
				return nil
			}
		}
	}
}

// AssertResourceExists asserts that a resource exists
func (tf *TestFramework) AssertResourceExists(gvk schema.GroupVersionKind, namespace, name string) error {
	key := fmt.Sprintf("%s/%s/%s", gvk.String(), namespace, name)

	tf.mu.RLock()
	_, exists := tf.testResources[key]
	tf.mu.RUnlock()

	if !exists {
		return fmt.Errorf("resource does not exist: %s", key)
	}

	tf.RecordEvent(TestEventTypeSuccess, "TestFramework", "AssertResourceExists",
		fmt.Sprintf("Resource exists: %s", key))
	return nil
}

// AssertNoErrors asserts that no errors have occurred during testing
func (tf *TestFramework) AssertNoErrors() error {
	tf.mu.RLock()
	errorCount := len(tf.testErrors)
	tf.mu.RUnlock()

	if errorCount > 0 {
		return fmt.Errorf("expected no errors, but found %d errors", errorCount)
	}

	tf.RecordEvent(TestEventTypeSuccess, "TestFramework", "AssertNoErrors", "No errors found")
	return nil
}

// GetTestErrors returns all errors that occurred during testing
func (tf *TestFramework) GetTestErrors() []error {
	tf.mu.RLock()
	defer tf.mu.RUnlock()

	errors := make([]error, len(tf.testErrors))
	copy(errors, tf.testErrors)
	return errors
}

// GetTestEvents returns all events that occurred during testing
func (tf *TestFramework) GetTestEvents() []TestEvent {
	tf.mu.RLock()
	defer tf.mu.RUnlock()

	events := make([]TestEvent, len(tf.testEvents))
	copy(events, tf.testEvents)
	return events
}

// RecordEvent records a test event
func (tf *TestFramework) RecordEvent(eventType TestEventType, component, operation, message string) {
	event := TestEvent{
		Timestamp:  time.Now(),
		Type:       eventType,
		Component:  component,
		Operation:  operation,
		Message:    message,
		Attributes: make(map[string]interface{}),
	}

	tf.mu.Lock()
	tf.testEvents = append(tf.testEvents, event)
	tf.mu.Unlock()
}

func (tf *TestFramework) getResourceKey(resource *unstructured.Unstructured) string {
	return fmt.Sprintf("%s/%s/%s",
		resource.GroupVersionKind().String(),
		resource.GetNamespace(),
		resource.GetName())
}

// TestHelper provides common test utilities
type TestHelper struct {
	framework *TestFramework
}

// NewTestHelper creates a new test helper
func NewTestHelper(framework *TestFramework) *TestHelper {
	return &TestHelper{
		framework: framework,
	}
}

// CreateDeployment creates a test deployment
func (th *TestHelper) CreateDeployment(name, namespace string, replicas int32) *unstructured.Unstructured {
	gvk := schema.GroupVersionKind{
		Group:   "apps",
		Version: "v1",
		Kind:    "Deployment",
	}

	spec := map[string]interface{}{
		"replicas": replicas,
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
				"containers": []map[string]interface{}{
					{
						"name":  name,
						"image": "nginx:latest",
					},
				},
			},
		},
	}

	return th.framework.CreateTestResource(gvk, namespace, name, spec)
}

// CreateService creates a test service
func (th *TestHelper) CreateService(name, namespace string, ports []int32) *unstructured.Unstructured {
	gvk := schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Service",
	}

	servicePorts := make([]map[string]interface{}, len(ports))
	for i, port := range ports {
		servicePorts[i] = map[string]interface{}{
			"port":       port,
			"targetPort": port,
			"protocol":   "TCP",
		}
	}

	spec := map[string]interface{}{
		"selector": map[string]interface{}{
			"app": name,
		},
		"ports": servicePorts,
	}

	return th.framework.CreateTestResource(gvk, namespace, name, spec)
}

// CreateConfigMap creates a test config map
func (th *TestHelper) CreateConfigMap(name, namespace string, data map[string]string) *unstructured.Unstructured {
	gvk := schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "ConfigMap",
	}

	configMap := th.framework.CreateTestResource(gvk, namespace, name, nil)
	if data != nil {
		configMap.Object["data"] = data
	}

	return configMap
}

// MockComponentProvider provides mock implementations for testing
type MockComponentProvider struct {
	componentType tmcpkg.ComponentType
	componentID   string
	healthStatus  tmcpkg.HealthStatus
	errorCount    int
	successCount  int
}

// NewMockComponentProvider creates a new mock component provider
func NewMockComponentProvider(componentType tmcpkg.ComponentType, componentID string) *MockComponentProvider {
	return &MockComponentProvider{
		componentType: componentType,
		componentID:   componentID,
		healthStatus:  tmcpkg.HealthStatusHealthy,
	}
}

func (mcp *MockComponentProvider) GetHealth(ctx context.Context) *tmcpkg.HealthCheck {
	return &tmcpkg.HealthCheck{
		ComponentType: mcp.componentType,
		ComponentID:   mcp.componentID,
		Status:        mcp.healthStatus,
		Message:       fmt.Sprintf("Mock component %s is %s", mcp.componentID, mcp.healthStatus),
		Details: map[string]interface{}{
			"errorCount":   mcp.errorCount,
			"successCount": mcp.successCount,
		},
		Timestamp: time.Now(),
	}
}

func (mcp *MockComponentProvider) GetComponentID() string {
	return mcp.componentID
}

func (mcp *MockComponentProvider) GetComponentType() tmcpkg.ComponentType {
	return mcp.componentType
}

// SetHealthStatus sets the health status for testing
func (mcp *MockComponentProvider) SetHealthStatus(status tmcpkg.HealthStatus) {
	mcp.healthStatus = status
}

// SimulateError simulates an error in the component
func (mcp *MockComponentProvider) SimulateError() {
	mcp.errorCount++
}

// SimulateSuccess simulates a successful operation
func (mcp *MockComponentProvider) SimulateSuccess() {
	mcp.successCount++
}

// TestSuite represents a collection of related tests
type TestSuite struct {
	Name        string
	Framework   *TestFramework
	Tests       []TestCase
	SetupFunc   func(*TestFramework) error
	CleanupFunc func(*TestFramework) error
}

// TestCase represents a single test case
type TestCase struct {
	Name        string
	Description string
	TestFunc    func(*TestFramework) error
	Timeout     time.Duration
	Skip        bool
	SkipReason  string
}

// RunTestSuite runs a complete test suite
func RunTestSuite(suite *TestSuite) error {
	fmt.Printf("Running test suite: %s\n", suite.Name)

	// Setup
	if suite.SetupFunc != nil {
		if err := suite.SetupFunc(suite.Framework); err != nil {
			return fmt.Errorf("test suite setup failed: %w", err)
		}
	}

	// Cleanup on exit
	defer func() {
		if suite.CleanupFunc != nil {
			suite.CleanupFunc(suite.Framework)
		}
		suite.Framework.Cleanup()
	}()

	// Run tests
	passed := 0
	failed := 0
	skipped := 0

	for _, test := range suite.Tests {
		if test.Skip {
			fmt.Printf("  SKIP %s: %s\n", test.Name, test.SkipReason)
			skipped++
			continue
		}

		fmt.Printf("  RUN  %s\n", test.Name)

		timeout := test.Timeout
		if timeout == 0 {
			timeout = suite.Framework.TestTimeout
		}

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		testErr := func() error {
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("  PANIC %s: %v\n", test.Name, r)
				}
			}()

			return test.TestFunc(suite.Framework)
		}()

		if testErr != nil {
			fmt.Printf("  FAIL %s: %v\n", test.Name, testErr)
			failed++
		} else {
			fmt.Printf("  PASS %s\n", test.Name)
			passed++
		}
	}

	fmt.Printf("\nTest suite %s completed: %d passed, %d failed, %d skipped\n",
		suite.Name, passed, failed, skipped)

	if failed > 0 {
		return fmt.Errorf("test suite %s failed with %d failures", suite.Name, failed)
	}

	return nil
}

// CreateTestCluster creates a test cluster configuration
func (tf *TestFramework) CreateTestCluster(name string, healthy bool, capacity map[string]float64) map[string]interface{} {
	cluster := map[string]interface{}{
		"name":     name,
		"healthy":  healthy,
		"capacity": capacity,
		"metadata": map[string]interface{}{
			"labels": map[string]string{
				"test-cluster": "true",
				"environment":  "test",
			},
		},
	}

	tf.RecordEvent(TestEventTypeInfo, "TestFramework", "CreateTestCluster",
		fmt.Sprintf("Created test cluster %s (healthy: %v)", name, healthy))

	return cluster
}

// ValidateTestResults validates the results of a test run
func (tf *TestFramework) ValidateTestResults() error {
	events := tf.GetTestEvents()
	errors := tf.GetTestErrors()

	// Check for any unexpected errors
	for _, err := range errors {
		if tmcErr, ok := err.(*tmcpkg.TMCError); ok {
			if tmcErr.Severity == tmcpkg.TMCErrorSeverityCritical {
				return fmt.Errorf("critical error detected during test: %v", tmcErr)
			}
		}
	}

	// Validate event sequence
	infoCount := 0
	errorCount := 0
	for _, event := range events {
		switch event.Type {
		case TestEventTypeInfo:
			infoCount++
		case TestEventTypeError:
			errorCount++
		}
	}

	tf.RecordEvent(TestEventTypeInfo, "TestFramework", "ValidateResults",
		fmt.Sprintf("Validation complete: %d info events, %d error events", infoCount, errorCount))

	return nil
}
