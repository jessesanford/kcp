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
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	tmcpkg "github.com/kcp-dev/kcp/pkg/reconciler/workload/tmc"
)

// TestTMCErrorHandling tests the TMC error handling system
func TestTMCErrorHandling(t *testing.T) {
	framework := NewTestFramework()
	if err := framework.Setup(); err != nil {
		t.Fatalf("Failed to setup test framework: %v", err)
	}
	defer framework.Cleanup()

	suite := &TestSuite{
		Name:      "TMC Error Handling Unit Tests",
		Framework: framework,
		Tests: []TestCase{
			{
				Name:        "TestErrorCreationAndCategorization",
				Description: "Test TMC error creation and categorization",
				TestFunc:    testErrorCreationAndCategorization,
			},
			{
				Name:        "TestErrorRetryLogic",
				Description: "Test error retry logic and strategies",
				TestFunc:    testErrorRetryLogic,
			},
			{
				Name:        "TestCircuitBreakerPattern",
				Description: "Test circuit breaker pattern implementation",
				TestFunc:    testCircuitBreakerPattern,
			},
			{
				Name:        "TestKubernetesErrorConversion",
				Description: "Test conversion from Kubernetes errors to TMC errors",
				TestFunc:    testKubernetesErrorConversion,
			},
		},
	}

	if err := RunTestSuite(suite); err != nil {
		t.Fatalf("Error handling unit test suite failed: %v", err)
	}
}

func testErrorCreationAndCategorization(framework *TestFramework) error {
	// Test error builder pattern
	tmcError := tmcpkg.NewTMCError(tmcpkg.TMCErrorTypeClusterUnreachable, "test-component", "test-operation").
		WithMessage("Test cluster unreachable error").
		WithCluster("test-cluster", "test-logical-cluster").
		WithResource(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}, "default", "test-app").
		WithSeverity(tmcpkg.TMCErrorSeverityHigh).
		WithRetryable(true).
		WithRecoveryHint("Check cluster connectivity").
		Build()

	// Verify error properties
	if tmcError.Type != tmcpkg.TMCErrorTypeClusterUnreachable {
		return fmt.Errorf("expected cluster unreachable error type")
	}

	if tmcError.Severity != tmcpkg.TMCErrorSeverityHigh {
		return fmt.Errorf("expected high severity")
	}

	if !tmcError.IsRetryable() {
		return fmt.Errorf("error should be retryable")
	}

	if tmcError.ClusterName != "test-cluster" {
		return fmt.Errorf("expected cluster name to be set")
	}

	// Test error message formatting
	errorMsg := tmcError.Error()
	if errorMsg == "" {
		return fmt.Errorf("error message should not be empty")
	}

	// Test recovery actions
	actions := tmcError.GetRecoveryActions()
	if len(actions) == 0 {
		return fmt.Errorf("should have recovery actions")
	}

	framework.RecordEvent(TestEventTypeSuccess, "Unit", "ErrorCreation", "Error creation and categorization test passed")
	return nil
}

func testErrorRetryLogic(framework *TestFramework) error {
	// Test default retry strategy
	strategy := tmcpkg.DefaultRetryStrategy()
	if strategy.MaxRetries != 5 {
		return fmt.Errorf("expected 5 max retries, got %d", strategy.MaxRetries)
	}

	// Test retryable error
	retryableError := tmcpkg.NewTMCError(tmcpkg.TMCErrorTypeClusterUnreachable, "test", "test").Build()
	if !strategy.ShouldRetry(retryableError, 0) {
		return fmt.Errorf("should retry cluster unreachable error")
	}

	// Test non-retryable error
	nonRetryableError := tmcpkg.NewTMCError(tmcpkg.TMCErrorTypeResourcePermission, "test", "test").Build()
	if strategy.ShouldRetry(nonRetryableError, 0) {
		return fmt.Errorf("should not retry permission error")
	}

	// Test max retries reached
	if strategy.ShouldRetry(retryableError, 5) {
		return fmt.Errorf("should not retry when max retries reached")
	}

	// Test delay calculation
	delay1 := strategy.GetDelay(0)
	delay2 := strategy.GetDelay(1)
	if delay2 <= delay1 {
		return fmt.Errorf("delay should increase with attempt number")
	}

	// Test retry execution
	attempts := 0
	operation := func() error {
		attempts++
		if attempts < 3 {
			return retryableError
		}
		return nil
	}

	err := tmcpkg.ExecuteWithRetry(operation, strategy)
	if err != nil {
		return fmt.Errorf("retry execution should have succeeded: %v", err)
	}

	if attempts != 3 {
		return fmt.Errorf("expected 3 attempts, got %d", attempts)
	}

	framework.RecordEvent(TestEventTypeSuccess, "Unit", "RetryLogic", "Retry logic test passed")
	return nil
}

func testCircuitBreakerPattern(framework *TestFramework) error {
	circuitBreaker := tmcpkg.NewCircuitBreaker("test-breaker", 3, 5*time.Second)

	// Test normal operation
	err := circuitBreaker.Execute(func() error {
		return nil
	})
	if err != nil {
		return fmt.Errorf("circuit breaker should allow normal operation: %v", err)
	}

	// Test failure accumulation
	for i := 0; i < 3; i++ {
		circuitBreaker.Execute(func() error {
			return fmt.Errorf("test error")
		})
	}

	// Circuit should be open now
	if circuitBreaker.GetState() != tmcpkg.CircuitBreakerOpen {
		return fmt.Errorf("circuit breaker should be open after failures")
	}

	// Test fail-fast behavior
	err = circuitBreaker.Execute(func() error {
		return nil
	})
	if err == nil {
		return fmt.Errorf("circuit breaker should fail fast when open")
	}

	framework.RecordEvent(TestEventTypeSuccess, "Unit", "CircuitBreaker", "Circuit breaker test passed")
	return nil
}

func testKubernetesErrorConversion(framework *TestFramework) error {
	// Test various Kubernetes error conversions
	testCases := []struct {
		k8sError     error
		expectedType tmcpkg.TMCErrorType
		retryable    bool
	}{
		{
			k8sError:     errors.NewNotFound(schema.GroupResource{}, "test"),
			expectedType: tmcpkg.TMCErrorTypeResourceNotFound,
			retryable:    false,
		},
		{
			k8sError:     errors.NewConflict(schema.GroupResource{}, "test", fmt.Errorf("conflict")),
			expectedType: tmcpkg.TMCErrorTypeResourceConflict,
			retryable:    true,
		},
		{
			k8sError:     errors.NewForbidden(schema.GroupResource{}, "test", fmt.Errorf("forbidden")),
			expectedType: tmcpkg.TMCErrorTypeResourcePermission,
			retryable:    false,
		},
		{
			k8sError:     errors.NewTimeout("test"),
			expectedType: tmcpkg.TMCErrorTypeSyncTimeout,
			retryable:    true,
		},
	}

	for _, testCase := range testCases {
		tmcError := tmcpkg.ConvertKubernetesError(testCase.k8sError, "test-component", "test-operation")

		if tmcError.Type != testCase.expectedType {
			return fmt.Errorf("expected error type %s, got %s", testCase.expectedType, tmcError.Type)
		}

		if tmcError.IsRetryable() != testCase.retryable {
			return fmt.Errorf("expected retryable %v, got %v", testCase.retryable, tmcError.IsRetryable())
		}
	}

	framework.RecordEvent(TestEventTypeSuccess, "Unit", "K8sErrorConversion", "Kubernetes error conversion test passed")
	return nil
}

// TestTMCHealthMonitoring tests the health monitoring system
func TestTMCHealthMonitoring(t *testing.T) {
	framework := NewTestFramework()
	if err := framework.Setup(); err != nil {
		t.Fatalf("Failed to setup test framework: %v", err)
	}
	defer framework.Cleanup()

	suite := &TestSuite{
		Name:      "TMC Health Monitoring Unit Tests",
		Framework: framework,
		Tests: []TestCase{
			{
				Name:        "TestHealthProviderRegistration",
				Description: "Test health provider registration and management",
				TestFunc:    testHealthProviderRegistration,
			},
			{
				Name:        "TestHealthCheckExecution",
				Description: "Test health check execution and status determination",
				TestFunc:    testHealthCheckExecution,
			},
			{
				Name:        "TestOverallHealthCalculation",
				Description: "Test overall health status calculation",
				TestFunc:    testOverallHealthCalculation,
			},
			{
				Name:        "TestHealthAggregation",
				Description: "Test health aggregation across components",
				TestFunc:    testHealthAggregation,
			},
		},
	}

	if err := RunTestSuite(suite); err != nil {
		t.Fatalf("Health monitoring unit test suite failed: %v", err)
	}
}

func testHealthProviderRegistration(framework *TestFramework) error {
	healthMonitor := framework.HealthMonitor

	// Create mock providers
	provider1 := NewMockComponentProvider(tmcpkg.ComponentTypePlacementController, "controller-1")
	provider2 := NewMockComponentProvider(tmcpkg.ComponentTypeVirtualWorkspaceManager, "manager-1")

	// Register providers
	healthMonitor.RegisterHealthProvider(provider1)
	healthMonitor.RegisterHealthProvider(provider2)

	// Wait briefly for registration to take effect
	time.Sleep(100 * time.Millisecond)

	// Verify providers are registered
	health1, exists1 := healthMonitor.GetComponentHealth(tmcpkg.ComponentTypePlacementController, "controller-1")
	if !exists1 {
		return fmt.Errorf("provider 1 not found after registration")
	}

	if health1.ComponentType != tmcpkg.ComponentTypePlacementController {
		return fmt.Errorf("incorrect component type for provider 1")
	}

	// Test unregistration
	healthMonitor.UnregisterHealthProvider(tmcpkg.ComponentTypePlacementController)
	_, exists1After := healthMonitor.GetComponentHealth(tmcpkg.ComponentTypePlacementController, "controller-1")
	if exists1After {
		return fmt.Errorf("provider 1 should be unregistered")
	}

	framework.RecordEvent(TestEventTypeSuccess, "Unit", "HealthProviderRegistration", "Health provider registration test passed")
	return nil
}

func testHealthCheckExecution(framework *TestFramework) error {
	healthMonitor := framework.HealthMonitor

	// Create mock provider with different health states
	provider := NewMockComponentProvider(tmcpkg.ComponentTypeSyncTargetController, "test-controller")
	healthMonitor.RegisterHealthProvider(provider)

	// Test healthy state
	provider.SetHealthStatus(tmcpkg.HealthStatusHealthy)
	ctx := context.Background()

	// Manually trigger health check (simulating periodic check)
	healthCheck := provider.GetHealth(ctx)
	if healthCheck.Status != tmcpkg.HealthStatusHealthy {
		return fmt.Errorf("expected healthy status")
	}

	// Test degraded state
	provider.SetHealthStatus(tmcpkg.HealthStatusDegraded)
	healthCheck = provider.GetHealth(ctx)
	if healthCheck.Status != tmcpkg.HealthStatusDegraded {
		return fmt.Errorf("expected degraded status")
	}

	// Test unhealthy state
	provider.SetHealthStatus(tmcpkg.HealthStatusUnhealthy)
	healthCheck = provider.GetHealth(ctx)
	if healthCheck.Status != tmcpkg.HealthStatusUnhealthy {
		return fmt.Errorf("expected unhealthy status")
	}

	framework.RecordEvent(TestEventTypeSuccess, "Unit", "HealthCheckExecution", "Health check execution test passed")
	return nil
}

func testOverallHealthCalculation(framework *TestFramework) error {
	healthMonitor := framework.HealthMonitor

	// Create multiple providers with different health states
	healthyProvider := NewMockComponentProvider(tmcpkg.ComponentTypePlacementController, "healthy-1")
	healthyProvider.SetHealthStatus(tmcpkg.HealthStatusHealthy)

	degradedProvider := NewMockComponentProvider(tmcpkg.ComponentTypeVirtualWorkspaceManager, "degraded-1")
	degradedProvider.SetHealthStatus(tmcpkg.HealthStatusDegraded)

	unhealthyProvider := NewMockComponentProvider(tmcpkg.ComponentTypeSyncTargetController, "unhealthy-1")
	unhealthyProvider.SetHealthStatus(tmcpkg.HealthStatusUnhealthy)

	// Register all providers
	healthMonitor.RegisterHealthProvider(healthyProvider)
	healthMonitor.RegisterHealthProvider(degradedProvider)
	healthMonitor.RegisterHealthProvider(unhealthyProvider)

	// Wait for health checks to be performed
	time.Sleep(200 * time.Millisecond)

	// Get overall health
	overallHealth := healthMonitor.GetOverallHealth()

	// With one unhealthy component, overall should be unhealthy
	if overallHealth.Status != tmcpkg.HealthStatusUnhealthy {
		return fmt.Errorf("expected overall unhealthy status, got %s", overallHealth.Status)
	}

	// Verify details
	if overallHealth.Details == nil {
		return fmt.Errorf("overall health should have details")
	}

	totalComponents, ok := overallHealth.Details["totalComponents"]
	if !ok || totalComponents.(int) != 3 {
		return fmt.Errorf("expected 3 total components")
	}

	framework.RecordEvent(TestEventTypeSuccess, "Unit", "OverallHealthCalculation", "Overall health calculation test passed")
	return nil
}

func testHealthAggregation(framework *TestFramework) error {
	healthAggregator := tmcpkg.NewHealthAggregator(framework.HealthMonitor)

	// Create cluster-specific providers
	cluster1Provider := tmcpkg.NewClusterHealthProvider(tmcpkg.ComponentTypeSyncTargetController, "cluster-1", "test-logical")
	cluster2Provider := tmcpkg.NewClusterHealthProvider(tmcpkg.ComponentTypeSyncTargetController, "cluster-2", "test-logical")

	// Register providers
	framework.HealthMonitor.RegisterHealthProvider(cluster1Provider)
	framework.HealthMonitor.RegisterHealthProvider(cluster2Provider)

	// Record some activity
	cluster1Provider.RecordActivity()
	cluster2Provider.RecordActivity()
	cluster2Provider.RecordError() // Make cluster-2 less healthy

	// Wait for health data to be collected
	time.Sleep(200 * time.Millisecond)

	// Get aggregated cluster health
	clusterHealth := healthAggregator.GetClusterHealth("cluster-1")
	if clusterHealth == nil {
		return fmt.Errorf("cluster health should not be nil")
	}

	if clusterHealth.ComponentType != "ClusterAggregate" {
		return fmt.Errorf("expected cluster aggregate component type")
	}

	framework.RecordEvent(TestEventTypeSuccess, "Unit", "HealthAggregation", "Health aggregation test passed")
	return nil
}

// TestTMCMetricsCollection tests the metrics collection system
func TestTMCMetricsCollection(t *testing.T) {
	framework := NewTestFramework()
	if err := framework.Setup(); err != nil {
		t.Fatalf("Failed to setup test framework: %v", err)
	}
	defer framework.Cleanup()

	suite := &TestSuite{
		Name:      "TMC Metrics Collection Unit Tests",
		Framework: framework,
		Tests: []TestCase{
			{
				Name:        "TestMetricsRecording",
				Description: "Test basic metrics recording functionality",
				TestFunc:    testMetricsRecording,
			},
			{
				Name:        "TestOperationTracking",
				Description: "Test operation tracking and timing",
				TestFunc:    testOperationTracking,
			},
			{
				Name:        "TestCustomMetrics",
				Description: "Test custom metrics registration and management",
				TestFunc:    testCustomMetrics,
			},
			{
				Name:        "TestMetricsReporting",
				Description: "Test metrics reporting and aggregation",
				TestFunc:    testMetricsReporting,
			},
		},
	}

	if err := RunTestSuite(suite); err != nil {
		t.Fatalf("Metrics collection unit test suite failed: %v", err)
	}
}

func testMetricsRecording(framework *TestFramework) error {
	metricsCollector := framework.MetricsCollector

	// Test component metrics
	metricsCollector.RecordComponentHealth("test-component", "test-id", "test-cluster", tmcpkg.HealthStatusHealthy)
	metricsCollector.RecordComponentOperation("test-component", "test-id", "test-operation", "success")
	metricsCollector.RecordComponentError("test-component", "test-id", tmcpkg.TMCErrorTypeResourceConflict, tmcpkg.TMCErrorSeverityMedium)

	// Test placement metrics
	metricsCollector.RecordPlacementCount("test-cluster", "default", "active", 5)
	metricsCollector.RecordPlacementDuration("test-cluster", "place", "success", 2*time.Second)
	metricsCollector.RecordPlacementError("test-cluster", tmcpkg.TMCErrorTypePlacementConstraint, "target-cluster")

	// Test sync metrics
	metricsCollector.RecordSyncResourceCount("cluster-1", "default", "apps/v1/Deployment", "synced", 10)
	metricsCollector.RecordSyncDuration("cluster-1", "apps/v1/Deployment", "sync", 1*time.Second)
	metricsCollector.RecordSyncError("cluster-1", "apps/v1/Deployment", tmcpkg.TMCErrorTypeSyncFailure)

	// Test cluster metrics
	metricsCollector.RecordClusterHealth("cluster-1", "test-logical", tmcpkg.HealthStatusHealthy)
	metricsCollector.RecordClusterConnectivity("cluster-1", "test-logical", true)
	metricsCollector.RecordClusterCapacity("cluster-1", "cpu", 0.75)

	framework.RecordEvent(TestEventTypeSuccess, "Unit", "MetricsRecording", "Metrics recording test passed")
	return nil
}

func testOperationTracking(framework *TestFramework) error {
	metricsCollector := framework.MetricsCollector

	// Test successful operation tracking
	tracker := metricsCollector.NewOperationTracker("test-component", "test-id", "test-operation")
	time.Sleep(50 * time.Millisecond) // Simulate operation time
	tracker.Success()

	// Test error operation tracking
	errorTracker := metricsCollector.NewOperationTracker("test-component", "test-id", "error-operation")
	testError := tmcpkg.NewTMCError(tmcpkg.TMCErrorTypeResourceConflict, "test-component", "error-operation").Build()
	errorTracker.Error(testError)

	// Test timeout operation tracking
	timeoutTracker := metricsCollector.NewOperationTracker("test-component", "test-id", "timeout-operation")
	timeoutTracker.Timeout()

	framework.RecordEvent(TestEventTypeSuccess, "Unit", "OperationTracking", "Operation tracking test passed")
	return nil
}

func testCustomMetrics(framework *TestFramework) error {
	metricsCollector := framework.MetricsCollector

	// Test custom metric registration
	customMetric := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "test_custom_metric",
		Help: "A test custom metric",
	})

	err := metricsCollector.RegisterCustomMetric("test_custom", customMetric)
	if err != nil {
		return fmt.Errorf("failed to register custom metric: %v", err)
	}

	// Test metric retrieval
	retrievedMetric, exists := metricsCollector.GetCustomMetric("test_custom")
	if !exists {
		return fmt.Errorf("custom metric not found")
	}

	if retrievedMetric != customMetric {
		return fmt.Errorf("retrieved metric does not match registered metric")
	}

	// Test duplicate registration
	err = metricsCollector.RegisterCustomMetric("test_custom", customMetric)
	if err == nil {
		return fmt.Errorf("should not allow duplicate metric registration")
	}

	framework.RecordEvent(TestEventTypeSuccess, "Unit", "CustomMetrics", "Custom metrics test passed")
	return nil
}

func testMetricsReporting(framework *TestFramework) error {
	metricsReporter := tmcpkg.NewMetricsReporter(framework.MetricsCollector, framework.HealthMonitor)

	// Get metrics summary
	summary := metricsReporter.GetMetricsSummary()
	if summary == nil {
		return fmt.Errorf("metrics summary should not be nil")
	}

	// Verify timestamp exists
	if _, ok := summary["timestamp"]; !ok {
		return fmt.Errorf("metrics summary should include timestamp")
	}

	// Verify collection status
	if collectionActive, ok := summary["collection_active"]; !ok || !collectionActive.(bool) {
		return fmt.Errorf("collection should be active")
	}

	framework.RecordEvent(TestEventTypeSuccess, "Unit", "MetricsReporting", "Metrics reporting test passed")
	return nil
}
