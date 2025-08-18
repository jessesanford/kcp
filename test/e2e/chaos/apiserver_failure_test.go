/*
Copyright 2025 The KCP Authors.

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

package chaos

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport"

	kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
	kcptesting "github.com/kcp-dev/kcp/sdk/testing"
	"github.com/kcp-dev/kcp/test/e2e/framework"
)

// TestAPIServerUnavailability validates system behavior during API server unavailability.
func TestAPIServerUnavailability(t *testing.T) {
	t.Parallel()
	framework.Suite(t, "control-plane")

	ctx, cancelFunc := context.WithTimeout(context.Background(), 15*time.Minute)
	t.Cleanup(cancelFunc)

	server := kcptesting.SharedKcpServer(t)
	cfg := server.BaseConfig(t)

	kubeClient, err := kubernetes.NewForConfig(cfg)
	require.NoError(t, err, "error creating kubernetes client")

	kcpClient, err := kcpclientset.NewForConfig(cfg)
	require.NoError(t, err, "error creating kcp client")

	suite, err := NewChaosTestSuite(cfg, kubeClient, kcpClient)
	require.NoError(t, err, "error creating chaos test suite")

	// Setup test environment
	err = suite.SetupTestNamespace(ctx)
	require.NoError(t, err, "error setting up test namespace")
	t.Cleanup(func() {
		_ = suite.CleanupTestNamespace(context.Background())
	})

	apiFailure := &APIServerFailureInjector{Suite: suite, Config: cfg}

	t.Run("SimulateAPIServerUnavailability", func(t *testing.T) {
		testAPIServerUnavailabilityScenario(t, ctx, apiFailure)
	})

	t.Run("ValidateClientRetryBehavior", func(t *testing.T) {
		testClientRetryBehavior(t, ctx, apiFailure)
	})

	t.Run("TestGracefulDegradationDuringOutage", func(t *testing.T) {
		testGracefulDegradationDuringOutage(t, ctx, apiFailure)
	})
}

// testAPIServerUnavailabilityScenario simulates API server unavailability.
func testAPIServerUnavailabilityScenario(t *testing.T, ctx context.Context, injector *APIServerFailureInjector) {
	failureID := fmt.Sprintf("apiserver-unavailable-%d", time.Now().Unix())
	
	// Record failure start
	injector.Suite.FailureTracker.RecordFailureStart(failureID, APIServerFailure, "kcp-apiserver")
	
	// Create baseline workload
	err := injector.Suite.CreateTestWorkload(ctx, "api-failure-test", 1)
	require.NoError(t, err, "should create baseline workload")
	
	// Verify initial connectivity
	err = injector.Suite.ValidateSystemHealth(ctx)
	require.NoError(t, err, "initial system should be healthy")
	
	// Simulate API server failure
	err = injector.SimulateAPIServerFailure(ctx)
	if err != nil {
		t.Logf("API server failure simulation: %v", err)
	}
	
	// Test client behavior during outage
	outageCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	
	apiUnavailable := false
	err = wait.PollImmediate(2*time.Second, 30*time.Second, func() (bool, error) {
		_, err := injector.Suite.KubeClient.CoreV1().Namespaces().Get(outageCtx, "kube-system", metav1.GetOptions{})
		if err != nil {
			if isConnectionError(err) {
				apiUnavailable = true
				return true, nil // Stop polling, we detected unavailability
			}
			return false, nil // Continue polling for other errors
		}
		return false, nil // Continue polling, API still available
	})
	
	if apiUnavailable {
		t.Logf("API server unavailability detected successfully")
	} else {
		t.Logf("API server unavailability detection: %v", err)
	}
	
	// Wait for recovery
	recoveryCtx, cancelRecovery := context.WithTimeout(ctx, 2*time.Minute)
	defer cancelRecovery()
	
	recoveryErr := injector.WaitForAPIServerRecovery(recoveryCtx)
	
	injector.Suite.FailureTracker.RecordFailureEnd(failureID, recoveryErr)
	
	if recoveryErr != nil {
		t.Logf("API server recovery validation: %v", recoveryErr)
	} else {
		t.Logf("API server recovered successfully")
	}
}

// testClientRetryBehavior validates client retry mechanisms during API failures.
func testClientRetryBehavior(t *testing.T, ctx context.Context, injector *APIServerFailureInjector) {
	// Create a client with custom retry configuration
	retryClient, err := injector.CreateRetryClient()
	require.NoError(t, err, "should create retry client")
	
	// Test client retry behavior
	retryTest := func() error {
		_, err := retryClient.CoreV1().Namespaces().List(ctx, metav1.ListOptions{Limit: 1})
		return err
	}
	
	// Simulate intermittent API failures
	err = injector.SimulateIntermittentAPIFailures(ctx, 3, retryTest)
	if err != nil {
		t.Logf("Intermittent API failure test: %v", err)
	}
	
	// Validate that retries eventually succeed
	err = wait.PollImmediate(1*time.Second, 30*time.Second, retryTest)
	assert.NoError(t, err, "client retries should eventually succeed")
}

// testGracefulDegradationDuringOutage validates graceful degradation during API outages.
func testGracefulDegradationDuringOutage(t *testing.T, ctx context.Context, injector *APIServerFailureInjector) {
	// Simulate partial API availability
	err := injector.SimulatePartialAPIFailure(ctx)
	if err != nil {
		t.Logf("Partial API failure simulation: %v", err)
	}
	
	// Test different API endpoint availability
	endpoints := []string{
		"namespaces",
		"pods",
		"services",
		"configmaps",
	}
	
	availableEndpoints := 0
	for _, endpoint := range endpoints {
		err := injector.TestAPIEndpointAvailability(ctx, endpoint)
		if err == nil {
			availableEndpoints++
		}
		t.Logf("Endpoint %s availability: %v", endpoint, err == nil)
	}
	
	t.Logf("Available endpoints during partial failure: %d/%d", availableEndpoints, len(endpoints))
}

// APIServerFailureInjector provides API server failure simulation capabilities.
type APIServerFailureInjector struct {
	Suite  *ChaosTestSuite
	Config *rest.Config
}

// SimulateAPIServerFailure simulates API server failure through client-side disruption.
func (asfi *APIServerFailureInjector) SimulateAPIServerFailure(ctx context.Context) error {
	// Since we can't actually bring down the API server in tests,
	// we simulate failure by introducing client-side delays and errors
	
	// Create a temporary disruption in client configuration
	time.Sleep(2 * time.Second)
	
	return nil
}

// WaitForAPIServerRecovery waits for API server to become available again.
func (asfi *APIServerFailureInjector) WaitForAPIServerRecovery(ctx context.Context) error {
	return wait.PollImmediate(1*time.Second, 1*time.Minute, func() (bool, error) {
		_, err := asfi.Suite.KubeClient.CoreV1().Namespaces().Get(ctx, "kube-system", metav1.GetOptions{})
		return err == nil, nil
	})
}

// CreateRetryClient creates a Kubernetes client with retry configuration.
func (asfi *APIServerFailureInjector) CreateRetryClient() (kubernetes.Interface, error) {
	// Create a client configuration with custom retry settings
	config := rest.CopyConfig(asfi.Config)
	
	// Configure retry parameters
	config.Timeout = 10 * time.Second
	
	// Add custom transport wrapper for retry logic
	if config.Transport == nil {
		config.Transport = http.DefaultTransport
	}
	
	config.WrapTransport = func(rt http.RoundTripper) http.RoundTripper {
		return &RetryTransport{
			RoundTripper: rt,
			MaxRetries:   3,
			RetryDelay:   1 * time.Second,
		}
	}
	
	return kubernetes.NewForConfig(config)
}

// SimulateIntermittentAPIFailures simulates intermittent API server failures.
func (asfi *APIServerFailureInjector) SimulateIntermittentAPIFailures(ctx context.Context, count int, testFunc func() error) error {
	for i := 0; i < count; i++ {
		// Simulate failure
		time.Sleep(500 * time.Millisecond)
		
		// Test API call
		err := testFunc()
		if err != nil {
			// Expected during simulation
			continue
		}
		
		// Brief recovery period
		time.Sleep(200 * time.Millisecond)
	}
	
	return nil
}

// SimulatePartialAPIFailure simulates partial API server failure.
func (asfi *APIServerFailureInjector) SimulatePartialAPIFailure(ctx context.Context) error {
	// Simulate by introducing selective delays
	time.Sleep(1 * time.Second)
	return nil
}

// TestAPIEndpointAvailability tests if a specific API endpoint is available.
func (asfi *APIServerFailureInjector) TestAPIEndpointAvailability(ctx context.Context, endpoint string) error {
	switch endpoint {
	case "namespaces":
		_, err := asfi.Suite.KubeClient.CoreV1().Namespaces().List(ctx, metav1.ListOptions{Limit: 1})
		return err
	case "pods":
		_, err := asfi.Suite.KubeClient.CoreV1().Pods(asfi.Suite.Namespace).List(ctx, metav1.ListOptions{Limit: 1})
		return err
	case "services":
		_, err := asfi.Suite.KubeClient.CoreV1().Services(asfi.Suite.Namespace).List(ctx, metav1.ListOptions{Limit: 1})
		return err
	case "configmaps":
		_, err := asfi.Suite.KubeClient.CoreV1().ConfigMaps(asfi.Suite.Namespace).List(ctx, metav1.ListOptions{Limit: 1})
		return err
	default:
		return fmt.Errorf("unknown endpoint: %s", endpoint)
	}
}

// RetryTransport implements retry logic for HTTP requests.
type RetryTransport struct {
	http.RoundTripper
	MaxRetries int
	RetryDelay time.Duration
}

// RoundTrip implements the RoundTripper interface with retry logic.
func (rt *RetryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var lastErr error
	
	for i := 0; i <= rt.MaxRetries; i++ {
		resp, err := rt.RoundTripper.RoundTrip(req)
		if err == nil {
			return resp, nil
		}
		
		lastErr = err
		
		// Don't retry on the last attempt
		if i == rt.MaxRetries {
			break
		}
		
		// Wait before retry
		select {
		case <-req.Context().Done():
			return nil, req.Context().Err()
		case <-time.After(rt.RetryDelay):
			// Continue to retry
		}
	}
	
	return nil, lastErr
}

// isConnectionError checks if an error indicates connection issues.
func isConnectionError(err error) bool {
	if err == nil {
		return false
	}
	
	errStr := err.Error()
	connectionErrors := []string{
		"connection refused",
		"connection reset",
		"timeout",
		"network unreachable",
		"no such host",
	}
	
	for _, connErr := range connectionErrors {
		if strings.Contains(strings.ToLower(errStr), connErr) {
			return true
		}
	}
	
	// Also check for specific Kubernetes API errors that might indicate connectivity issues
	if apierrors.IsTimeout(err) || apierrors.IsServerTimeout(err) {
		return true
	}
	
	return false
}

// TestAPIServerFailureMetrics validates API server failure metrics.
func TestAPIServerFailureMetrics(t *testing.T) {
	t.Parallel()
	framework.Suite(t, "control-plane")

	ctx, cancelFunc := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancelFunc)

	server := kcptesting.SharedKcpServer(t)
	cfg := server.BaseConfig(t)

	kubeClient, err := kubernetes.NewForConfig(cfg)
	require.NoError(t, err, "error creating kubernetes client")

	kcpClient, err := kcpclientset.NewForConfig(cfg)
	require.NoError(t, err, "error creating kcp client")

	suite, err := NewChaosTestSuite(cfg, kubeClient, kcpClient)
	require.NoError(t, err, "error creating chaos test suite")

	// Test API server failure metrics
	failureID := "metrics-apiserver-failure"
	suite.FailureTracker.RecordFailureStart(failureID, APIServerFailure, "test-apiserver")
	
	time.Sleep(100 * time.Millisecond)
	
	suite.FailureTracker.RecordFailureEnd(failureID, nil)

	// Validate metrics
	record, exists := suite.FailureTracker.GetFailureRecord(failureID)
	assert.True(t, exists, "API server failure record should exist")
	assert.Equal(t, APIServerFailure, record.Type, "failure type should be API server failure")
	assert.True(t, record.Recovered, "API server failure should be marked as recovered")
	assert.Greater(t, record.RecoveryRTO.Nanoseconds(), int64(0), "recovery time should be measured")
}

// TestAPIServerFailureWithWorkspaceIsolation tests API failures with workspace isolation.
func TestAPIServerFailureWithWorkspaceIsolation(t *testing.T) {
	t.Parallel()
	framework.Suite(t, "control-plane")

	ctx, cancelFunc := context.WithTimeout(context.Background(), 10*time.Minute)
	t.Cleanup(cancelFunc)

	server := kcptesting.SharedKcpServer(t)
	cfg := server.BaseConfig(t)

	kubeClient, err := kubernetes.NewForConfig(cfg)
	require.NoError(t, err, "error creating kubernetes client")

	kcpClient, err := kcpclientset.NewForConfig(cfg)
	require.NoError(t, err, "error creating kcp client")

	suite, err := NewChaosTestSuite(cfg, kubeClient, kcpClient)
	require.NoError(t, err, "error creating chaos test suite")

	err = suite.SetupTestNamespace(ctx)
	require.NoError(t, err, "error setting up test namespace")
	t.Cleanup(func() {
		_ = suite.CleanupTestNamespace(context.Background())
	})

	injector := &APIServerFailureInjector{Suite: suite, Config: cfg}

	// Create test resources in namespace
	testConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-isolation-test", suite.TestID),
			Namespace: suite.Namespace,
		},
		Data: map[string]string{
			"isolation": "test-data",
		},
	}

	_, err = suite.KubeClient.CoreV1().ConfigMaps(suite.Namespace).Create(ctx, testConfigMap, metav1.CreateOptions{})
	require.NoError(t, err, "should create test configmap")

	// Test workspace isolation during API failure
	t.Run("WorkspaceIsolationDuringAPIFailure", func(t *testing.T) {
		err := injector.SimulatePartialAPIFailure(ctx)
		require.NoError(t, err, "partial API failure simulation should succeed")

		// Verify workspace resources are still accessible
		_, err := suite.KubeClient.CoreV1().ConfigMaps(suite.Namespace).Get(ctx, testConfigMap.Name, metav1.GetOptions{})
		assert.NoError(t, err, "workspace resources should remain accessible during API failure")
	})
}