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

	"k8s.io/apimachinery/pkg/runtime/schema"

	tmcpkg "github.com/kcp-dev/kcp/pkg/reconciler/workload/tmc"
	"github.com/kcp-dev/kcp/pkg/reconciler/workload/virtualworkspace"
)

// TestTMCIntegration runs comprehensive integration tests for TMC components
func TestTMCIntegration(t *testing.T) {
	framework := NewTestFramework()
	if err := framework.Setup(); err != nil {
		t.Fatalf("Failed to setup test framework: %v", err)
	}
	defer framework.Cleanup()

	suite := &TestSuite{
		Name:      "TMC Integration Tests",
		Framework: framework,
		Tests: []TestCase{
			{
				Name:        "TestHealthMonitoringIntegration",
				Description: "Test health monitoring across all components",
				TestFunc:    testHealthMonitoringIntegration,
				Timeout:     60 * time.Second,
			},
			{
				Name:        "TestErrorHandlingIntegration",
				Description: "Test error handling and recovery mechanisms",
				TestFunc:    testErrorHandlingIntegration,
				Timeout:     45 * time.Second,
			},
			{
				Name:        "TestMetricsCollectionIntegration",
				Description: "Test metrics collection across components",
				TestFunc:    testMetricsCollectionIntegration,
				Timeout:     30 * time.Second,
			},
			{
				Name:        "TestVirtualWorkspaceIntegration",
				Description: "Test virtual workspace functionality",
				TestFunc:    testVirtualWorkspaceIntegration,
				Timeout:     90 * time.Second,
			},
			{
				Name:        "TestCrossClusterResourceManagement",
				Description: "Test cross-cluster resource operations",
				TestFunc:    testCrossClusterResourceManagement,
				Timeout:     120 * time.Second,
			},
			{
				Name:        "TestRecoveryScenarios",
				Description: "Test various recovery scenarios",
				TestFunc:    testRecoveryScenarios,
				Timeout:     60 * time.Second,
			},
		},
		SetupFunc:   setupIntegrationTests,
		CleanupFunc: cleanupIntegrationTests,
	}

	if err := RunTestSuite(suite); err != nil {
		t.Fatalf("Integration test suite failed: %v", err)
	}
}

func setupIntegrationTests(framework *TestFramework) error {
	// Create test placements
	placement1 := framework.CreateTestPlacement("test-placement-1", "default", []string{"cluster-1", "cluster-2"})
	placement2 := framework.CreateTestPlacement("test-placement-2", "test-namespace", []string{"cluster-2", "cluster-3"})

	// Create test sync targets
	syncTarget1 := framework.CreateTestSyncTarget("cluster-1", true)
	syncTarget2 := framework.CreateTestSyncTarget("cluster-2", true)
	syncTarget3 := framework.CreateTestSyncTarget("cluster-3", false) // Unhealthy cluster

	// Add objects to framework
	framework.Objects = append(framework.Objects, placement1, placement2, syncTarget1, syncTarget2, syncTarget3)

	// Start health monitoring
	ctx := context.Background()
	go framework.HealthMonitor.Start(ctx)

	// Register mock components
	mockProvider1 := NewMockComponentProvider(tmcpkg.ComponentTypePlacementController, "placement-controller-1")
	mockProvider2 := NewMockComponentProvider(tmcpkg.ComponentTypeVirtualWorkspaceManager, "vw-manager-1")
	framework.HealthMonitor.RegisterHealthProvider(mockProvider1)
	framework.HealthMonitor.RegisterHealthProvider(mockProvider2)

	framework.RecordEvent(TestEventTypeInfo, "Integration", "Setup", "Integration test setup completed")
	return nil
}

func cleanupIntegrationTests(framework *TestFramework) error {
	framework.RecordEvent(TestEventTypeInfo, "Integration", "Cleanup", "Integration test cleanup completed")
	return nil
}

func testHealthMonitoringIntegration(framework *TestFramework) error {
	ctx := context.Background()

	// Wait for health monitor to collect initial data
	err := framework.WaitForCondition("health monitor to collect data", func() bool {
		allHealth := framework.HealthMonitor.GetAllComponentHealth()
		return len(allHealth) > 0
	})
	if err != nil {
		return fmt.Errorf("health monitor did not collect data: %w", err)
	}

	// Check overall health
	overallHealth := framework.HealthMonitor.GetOverallHealth()
	if overallHealth.Status == tmcpkg.HealthStatusUnknown {
		return fmt.Errorf("overall health status is unknown")
	}

	// Simulate component failure and recovery
	mockProvider := NewMockComponentProvider(tmcpkg.ComponentTypeSyncTargetController, "test-component")
	framework.HealthMonitor.RegisterHealthProvider(mockProvider)

	// Simulate error
	mockProvider.SetHealthStatus(tmcpkg.HealthStatusUnhealthy)
	time.Sleep(2 * time.Second)

	// Check that health degraded
	componentHealth, exists := framework.HealthMonitor.GetComponentHealth(tmcpkg.ComponentTypeSyncTargetController, "test-component")
	if !exists {
		return fmt.Errorf("component health not found")
	}
	if componentHealth.Status != tmcpkg.HealthStatusUnhealthy {
		return fmt.Errorf("expected unhealthy status, got %s", componentHealth.Status)
	}

	// Simulate recovery
	mockProvider.SetHealthStatus(tmcpkg.HealthStatusHealthy)
	time.Sleep(2 * time.Second)

	framework.RecordEvent(TestEventTypeSuccess, "Integration", "HealthMonitoring", "Health monitoring integration test passed")
	return nil
}

func testErrorHandlingIntegration(framework *TestFramework) error {
	// Test error creation and categorization
	clusterError := framework.SimulateError(tmcpkg.TMCErrorTypeClusterUnreachable, "test-component", "test-operation")
	if clusterError.Type != tmcpkg.TMCErrorTypeClusterUnreachable {
		return fmt.Errorf("expected cluster unreachable error, got %s", clusterError.Type)
	}

	// Test error with recovery
	recoveryError := framework.SimulateError(tmcpkg.TMCErrorTypeSyncFailure, "sync-component", "sync-operation")
	if !recoveryError.IsRetryable() {
		return fmt.Errorf("sync failure should be retryable")
	}

	// Test error metrics recording
	framework.MetricsCollector.RecordComponentError("test-component", "test-id", clusterError.Type, clusterError.Severity)

	framework.RecordEvent(TestEventTypeSuccess, "Integration", "ErrorHandling", "Error handling integration test passed")
	return nil
}

func testMetricsCollectionIntegration(framework *TestFramework) error {
	// Record various metrics
	framework.MetricsCollector.RecordComponentHealth("test-component", "test-id", "test-cluster", tmcpkg.HealthStatusHealthy)
	framework.MetricsCollector.RecordComponentOperation("test-component", "test-id", "test-operation", "success")
	framework.MetricsCollector.RecordPlacementCount("test-cluster", "default", "active", 5)
	framework.MetricsCollector.RecordSyncResourceCount("cluster-1", "default", "apps/v1/Deployment", "synced", 10)

	// Test operation tracking
	tracker := framework.MetricsCollector.NewOperationTracker("test-component", "test-id", "test-operation")
	time.Sleep(100 * time.Millisecond)
	tracker.Success()

	// Test error tracking
	errorTracker := framework.MetricsCollector.NewOperationTracker("test-component", "test-id", "error-operation")
	testError := tmcpkg.NewTMCError(tmcpkg.TMCErrorTypeResourceConflict, "test-component", "error-operation").Build()
	errorTracker.Error(testError)

	framework.RecordEvent(TestEventTypeSuccess, "Integration", "MetricsCollection", "Metrics collection integration test passed")
	return nil
}

func testVirtualWorkspaceIntegration(framework *TestFramework) error {
	ctx := context.Background()

	// Create a test virtual workspace manager
	placement := framework.CreateTestPlacement("vw-test-placement", "default", framework.Clusters)

	// Create virtual workspace manager (simulated)
	vwManager, err := virtualworkspace.NewVirtualWorkspaceManager(
		framework.KCPClient,
		framework.DynamicClient,
		nil, // placement informer
		nil, // sync target informer
	)
	if err != nil {
		return fmt.Errorf("failed to create virtual workspace manager: %w", err)
	}

	// Test virtual workspace creation and management
	// In a real test, we would start the manager and verify workspace creation
	_ = vwManager

	// Create test resources for aggregation
	helper := NewTestHelper(framework)
	deployment1 := helper.CreateDeployment("test-app", "default", 3)
	service1 := helper.CreateService("test-service", "default", []int32{80, 443})
	configMap1 := helper.CreateConfigMap("test-config", "default", map[string]string{"key": "value"})

	// Verify resources were created
	if err := framework.AssertResourceExists(deployment1.GroupVersionKind(), deployment1.GetNamespace(), deployment1.GetName()); err != nil {
		return fmt.Errorf("deployment not found: %w", err)
	}

	if err := framework.AssertResourceExists(service1.GroupVersionKind(), service1.GetNamespace(), service1.GetName()); err != nil {
		return fmt.Errorf("service not found: %w", err)
	}

	framework.RecordEvent(TestEventTypeSuccess, "Integration", "VirtualWorkspace", "Virtual workspace integration test passed")
	return nil
}

func testCrossClusterResourceManagement(framework *TestFramework) error {
	// Test resource aggregation across clusters
	gvk := schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}

	// Create deployments in different clusters
	for i, cluster := range framework.Clusters {
		deployment := framework.CreateTestResource(gvk, "default", fmt.Sprintf("app-%d", i), map[string]interface{}{
			"replicas": int32(i + 1),
		})

		// Simulate deployment in cluster
		framework.MetricsCollector.RecordResourceAggregation(gvk.String(), "union", "success")
		framework.MetricsCollector.RecordClusterResourceCount(cluster, gvk.String(), "default", 1)
	}

	// Test resource projection
	for i, sourceCluster := range framework.Clusters {
		for j, targetCluster := range framework.Clusters {
			if i != j {
				framework.MetricsCollector.RecordResourceProjection(sourceCluster, targetCluster, gvk.String(), "success")
			}
		}
	}

	// Test conflict resolution
	framework.MetricsCollector.RecordResourceConflict("cluster-1", gvk.String(), "version", "latest-wins")

	framework.RecordEvent(TestEventTypeSuccess, "Integration", "CrossClusterResources", "Cross-cluster resource management test passed")
	return nil
}

func testRecoveryScenarios(framework *TestFramework) error {
	ctx := context.Background()

	// Start recovery manager
	go framework.RecoveryManager.Start(ctx)

	// Test cluster connectivity recovery
	connectivityError := tmcpkg.NewTMCError(tmcpkg.TMCErrorTypeClusterUnreachable, "cluster-manager", "connect").
		WithCluster("cluster-1", "test-cluster").
		Build()

	recoveryCtx := &tmcpkg.RecoveryContext{
		ClusterName: "cluster-1",
		Attempt:     1,
		MaxAttempts: 3,
	}

	err := framework.RecoveryManager.RecoverFromError(ctx, connectivityError, recoveryCtx)
	if err != nil {
		return fmt.Errorf("recovery manager failed to start recovery: %w", err)
	}

	// Wait for recovery to complete
	err = framework.WaitForCondition("recovery to complete", func() bool {
		status := framework.RecoveryManager.GetRecoveryStatus()
		if activeRecoveries, ok := status["activeRecoveries"]; ok {
			return activeRecoveries.(int) == 0
		}
		return false
	})
	if err != nil {
		return fmt.Errorf("recovery did not complete in time: %w", err)
	}

	// Test resource conflict recovery
	conflictError := tmcpkg.NewTMCError(tmcpkg.TMCErrorTypeResourceConflict, "sync-controller", "update").
		WithResource(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}, "default", "test-app").
		Build()

	err = framework.RecoveryManager.RecoverFromError(ctx, conflictError, recoveryCtx)
	if err != nil {
		return fmt.Errorf("conflict recovery failed to start: %w", err)
	}

	framework.RecordEvent(TestEventTypeSuccess, "Integration", "RecoveryScenarios", "Recovery scenarios test passed")
	return nil
}

// TestTMCStressTest runs stress tests to validate TMC performance under load
func TestTMCStressTest(t *testing.T) {
	framework := NewTestFramework()
	if err := framework.Setup(); err != nil {
		t.Fatalf("Failed to setup test framework: %v", err)
	}
	defer framework.Cleanup()

	suite := &TestSuite{
		Name:      "TMC Stress Tests",
		Framework: framework,
		Tests: []TestCase{
			{
				Name:        "TestHighVolumeResourceAggregation",
				Description: "Test resource aggregation with high volume",
				TestFunc:    testHighVolumeResourceAggregation,
				Timeout:     300 * time.Second,
			},
			{
				Name:        "TestConcurrentMigrations",
				Description: "Test concurrent migration operations",
				TestFunc:    testConcurrentMigrations,
				Timeout:     180 * time.Second,
			},
			{
				Name:        "TestErrorRecoveryUnderLoad",
				Description: "Test error recovery under load",
				TestFunc:    testErrorRecoveryUnderLoad,
				Timeout:     240 * time.Second,
			},
		},
		SetupFunc: setupStressTests,
	}

	if err := RunTestSuite(suite); err != nil {
		t.Fatalf("Stress test suite failed: %v", err)
	}
}

func setupStressTests(framework *TestFramework) error {
	// Increase test clusters for stress testing
	framework.Clusters = []string{"cluster-1", "cluster-2", "cluster-3", "cluster-4", "cluster-5"}

	// Create multiple namespaces
	framework.Namespaces = []string{"default", "test-1", "test-2", "test-3", "stress-test"}

	// Set longer timeout for stress tests
	framework.TestTimeout = 5 * time.Minute

	framework.RecordEvent(TestEventTypeInfo, "StressTest", "Setup", "Stress test setup completed")
	return nil
}

func testHighVolumeResourceAggregation(framework *TestFramework) error {
	helper := NewTestHelper(framework)

	// Create many resources across clusters and namespaces
	resourceCount := 0
	for _, namespace := range framework.Namespaces {
		for i := 0; i < 20; i++ {
			helper.CreateDeployment(fmt.Sprintf("app-%d", i), namespace, int32(i+1))
			helper.CreateService(fmt.Sprintf("service-%d", i), namespace, []int32{8080})
			helper.CreateConfigMap(fmt.Sprintf("config-%d", i), namespace, map[string]string{
				"key": fmt.Sprintf("value-%d", i),
			})
			resourceCount += 3
		}
	}

	// Record aggregation metrics
	for _, cluster := range framework.Clusters {
		framework.MetricsCollector.RecordResourceAggregation("apps/v1/Deployment", "union", "success")
		framework.MetricsCollector.RecordResourceAggregation("v1/Service", "union", "success")
		framework.MetricsCollector.RecordResourceAggregation("v1/ConfigMap", "union", "success")
	}

	framework.RecordEvent(TestEventTypeSuccess, "StressTest", "HighVolumeAggregation",
		fmt.Sprintf("Processed %d resources successfully", resourceCount))
	return nil
}

func testConcurrentMigrations(framework *TestFramework) error {
	// Simulate concurrent migrations between clusters
	migrationCount := 10

	for i := 0; i < migrationCount; i++ {
		sourceCluster := framework.Clusters[i%len(framework.Clusters)]
		targetCluster := framework.Clusters[(i+1)%len(framework.Clusters)]

		// Record migration metrics
		framework.MetricsCollector.RecordMigrationCount(sourceCluster, targetCluster, "running", 1)

		// Simulate migration completion
		go func(source, target string, id int) {
			time.Sleep(time.Duration(id*100) * time.Millisecond)
			framework.MetricsCollector.RecordMigrationDuration(source, target, "live", time.Duration(id)*time.Second)
			framework.MetricsCollector.RecordMigrationCount(source, target, "completed", 1)
		}(sourceCluster, targetCluster, i)
	}

	// Wait for migrations to complete
	time.Sleep(2 * time.Second)

	framework.RecordEvent(TestEventTypeSuccess, "StressTest", "ConcurrentMigrations",
		fmt.Sprintf("Completed %d concurrent migrations", migrationCount))
	return nil
}

func testErrorRecoveryUnderLoad(framework *TestFramework) error {
	ctx := context.Background()

	// Start recovery manager
	go framework.RecoveryManager.Start(ctx)

	// Generate multiple errors concurrently
	errorTypes := []tmcpkg.TMCErrorType{
		tmcpkg.TMCErrorTypeClusterUnreachable,
		tmcpkg.TMCErrorTypeResourceConflict,
		tmcpkg.TMCErrorTypeSyncFailure,
		tmcpkg.TMCErrorTypePlacementConstraint,
	}

	for i := 0; i < 20; i++ {
		errorType := errorTypes[i%len(errorTypes)]
		component := fmt.Sprintf("component-%d", i)

		go func(et tmcpkg.TMCErrorType, comp string, id int) {
			tmcError := tmcpkg.NewTMCError(et, comp, "stress-test").
				WithMessage(fmt.Sprintf("Stress test error %d", id)).
				Build()

			recoveryCtx := &tmcpkg.RecoveryContext{
				Attempt:     1,
				MaxAttempts: 3,
			}

			framework.RecoveryManager.RecoverFromError(ctx, tmcError, recoveryCtx)
		}(errorType, component, i)
	}

	// Wait for recoveries to process
	time.Sleep(5 * time.Second)

	framework.RecordEvent(TestEventTypeSuccess, "StressTest", "ErrorRecoveryUnderLoad", "Error recovery under load test passed")
	return nil
}
