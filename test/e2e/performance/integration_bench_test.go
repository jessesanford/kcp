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

package performance

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/kcp-dev/kcp/test/e2e/framework"
)

// BenchmarkTMCPerformanceIntegration runs comprehensive TMC performance tests with profiling
func BenchmarkTMCPerformanceIntegration(b *testing.B) {
	// Setup metrics collection
	metricsCollector := NewMetricsCollector("tmc-integration-performance", "performance_reports")
	
	// Setup profiling
	profiler := NewProfiler(GetDefaultProfileConfig("TMC_Integration"))
	
	// Run profiled benchmark
	profiler.ProfiledBenchmark(b, func(b *testing.B) {
		runTMCIntegrationBenchmark(b, metricsCollector)
	})
	
	// Analyze performance and generate report
	if err := metricsCollector.AnalyzePerformance(); err != nil {
		b.Errorf("failed to analyze performance: %v", err)
	}
	
	// Save performance report
	reportFile := fmt.Sprintf("tmc_integration_report_%s.json", time.Now().Format("20060102_150405"))
	if err := metricsCollector.SaveReport(reportFile); err != nil {
		b.Errorf("failed to save performance report: %v", err)
	}
	
	// Save as baseline if environment variable is set
	if os.Getenv("SAVE_BASELINE") == "true" {
		if err := metricsCollector.SaveBaseline(); err != nil {
			b.Errorf("failed to save baseline: %v", err)
		}
		b.Logf("Baseline saved for future regression testing")
	}
	
	// Print summary
	metricsCollector.PrintSummary()
}

func runTMCIntegrationBenchmark(b *testing.B, collector *MetricsCollector) {
	ctx := context.Background()
	server := framework.SharedKcpServer(b)
	
	// Create workspace for integration benchmark
	org := framework.NewOrganizationFixture(b, server, framework.TODO_WithoutMultiShardSupport())
	ws := framework.NewWorkspaceFixture(b, server, org.Path(), framework.TODO_WithoutMultiShardSupport())
	
	benchFramework := NewBenchmarkFramework(b, ws.Path())
	defer benchFramework.Cleanup()

	if err := benchFramework.SetupNamespace(ctx); err != nil {
		b.Fatalf("failed to setup namespace: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	// Collect baseline memory metrics
	collector.RecordMemoryMetrics(map[string]string{"phase": "baseline"})

	for i := 0; i < b.N; i++ {
		iterationLabels := map[string]string{
			"iteration": fmt.Sprintf("%d", i),
			"phase":     "benchmark",
		}

		// Test 1: Placement performance
		placementLatency := benchPlacementLatency(b, benchFramework)
		collector.RecordMetric("placement_latency", "latency", "ns", float64(placementLatency.Nanoseconds()), iterationLabels)

		// Test 2: Sync throughput
		syncThroughput := benchSyncThroughput(b, benchFramework)
		collector.RecordMetric("sync_throughput", "throughput", "ops/sec", syncThroughput, iterationLabels)

		// Test 3: API response times
		apiLatency := benchAPIResponseTime(b, benchFramework)
		collector.RecordMetric("api_response_latency", "latency", "ns", float64(apiLatency.Nanoseconds()), iterationLabels)

		// Test 4: Controller reconciliation
		reconcileLatency := benchControllerReconciliation(b, benchFramework)
		collector.RecordMetric("controller_reconcile_latency", "latency", "ns", float64(reconcileLatency.Nanoseconds()), iterationLabels)

		// Record memory usage during benchmark
		collector.RecordMemoryMetrics(iterationLabels)
	}

	// Final memory collection
	collector.RecordMemoryMetrics(map[string]string{"phase": "final"})
}

func benchPlacementLatency(b *testing.B, bf *BenchmarkFramework) time.Duration {
	ctx := context.Background()
	
	// Create mock cluster for placement
	clusterResource := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "configmaps",
	}
	
	clusterName := fmt.Sprintf("%sintegration-cluster-%d", bf.ResourcePrefix(), time.Now().UnixNano())
	cluster := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":      clusterName,
				"namespace": bf.namespace,
				"labels": map[string]interface{}{
					"test-type": "integration-cluster",
				},
			},
			"data": map[string]interface{}{
				"cluster-id": clusterName,
				"location":   "test-region",
			},
		},
	}

	start := time.Now()
	_, err := bf.DynamicClient().Resource(clusterResource).
		Cluster(bf.TestCluster().Path()).
		Namespace(bf.namespace).
		Create(ctx, cluster, metav1.CreateOptions{})
	if err != nil {
		b.Errorf("failed to create cluster for placement test: %v", err)
		return time.Duration(0)
	}
	
	// Simulate placement decision time
	placementTime := time.Since(start)
	
	// Cleanup
	bf.DynamicClient().Resource(clusterResource).
		Cluster(bf.TestCluster().Path()).
		Namespace(bf.namespace).
		Delete(ctx, clusterName, metav1.DeleteOptions{})
	
	return placementTime
}

func benchSyncThroughput(b *testing.B, bf *BenchmarkFramework) float64 {
	ctx := context.Background()
	
	secretResource := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "secrets",
	}

	resourceCount := 10
	start := time.Now()
	
	// Create multiple resources to measure sync throughput
	for i := 0; i < resourceCount; i++ {
		resourceName := fmt.Sprintf("%ssync-throughput-%d", bf.ResourcePrefix(), i)
		
		secret := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Secret",
				"metadata": map[string]interface{}{
					"name":      resourceName,
					"namespace": bf.namespace,
					"labels": map[string]interface{}{
						"test-type": "sync-throughput",
					},
				},
				"type": "Opaque",
				"data": map[string]interface{}{
					"data": "aGVsbG8gd29ybGQ=", // base64 "hello world"
				},
			},
		}

		_, err := bf.DynamicClient().Resource(secretResource).
			Cluster(bf.TestCluster().Path()).
			Namespace(bf.namespace).
			Create(ctx, secret, metav1.CreateOptions{})
		if err != nil {
			b.Errorf("failed to create secret for sync test: %v", err)
			continue
		}
	}
	
	totalTime := time.Since(start)
	throughput := float64(resourceCount) / totalTime.Seconds()
	
	// Cleanup
	bf.DynamicClient().Resource(secretResource).
		Cluster(bf.TestCluster().Path()).
		Namespace(bf.namespace).
		DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{
			LabelSelector: "test-type=sync-throughput",
		})
	
	return throughput
}

func benchAPIResponseTime(b *testing.B, bf *BenchmarkFramework) time.Duration {
	ctx := context.Background()
	
	configMapResource := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "configmaps",
	}

	// Create a resource to test GET latency
	resourceName := fmt.Sprintf("%sapi-test-%d", bf.ResourcePrefix(), time.Now().UnixNano())
	configMap := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":      resourceName,
				"namespace": bf.namespace,
			},
			"data": map[string]interface{}{
				"test": "data",
			},
		},
	}

	_, err := bf.DynamicClient().Resource(configMapResource).
		Cluster(bf.TestCluster().Path()).
		Namespace(bf.namespace).
		Create(ctx, configMap, metav1.CreateOptions{})
	if err != nil {
		b.Errorf("failed to create configmap for API test: %v", err)
		return time.Duration(0)
	}

	// Measure GET latency
	start := time.Now()
	_, err = bf.DynamicClient().Resource(configMapResource).
		Cluster(bf.TestCluster().Path()).
		Namespace(bf.namespace).
		Get(ctx, resourceName, metav1.GetOptions{})
	apiLatency := time.Since(start)
	
	if err != nil {
		b.Errorf("failed to get configmap: %v", err)
	}

	// Cleanup
	bf.DynamicClient().Resource(configMapResource).
		Cluster(bf.TestCluster().Path()).
		Namespace(bf.namespace).
		Delete(ctx, resourceName, metav1.DeleteOptions{})
	
	return apiLatency
}

func benchControllerReconciliation(b *testing.B, bf *BenchmarkFramework) time.Duration {
	ctx := context.Background()
	
	serviceAccountResource := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "serviceaccounts",
	}

	resourceName := fmt.Sprintf("%scontroller-test-%d", bf.ResourcePrefix(), time.Now().UnixNano())
	serviceAccount := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ServiceAccount",
			"metadata": map[string]interface{}{
				"name":      resourceName,
				"namespace": bf.namespace,
				"labels": map[string]interface{}{
					"test-type": "controller-reconciliation",
				},
			},
		},
	}

	start := time.Now()
	_, err := bf.DynamicClient().Resource(serviceAccountResource).
		Cluster(bf.TestCluster().Path()).
		Namespace(bf.namespace).
		Create(ctx, serviceAccount, metav1.CreateOptions{})
	if err != nil {
		b.Errorf("failed to create serviceaccount for controller test: %v", err)
		return time.Duration(0)
	}

	// Simulate waiting for controller reconciliation
	reconcileTime, err := bf.WaitForCondition(
		"controller-reconciliation-complete",
		5*time.Second,
		func(ctx context.Context) (bool, error) {
			_, err := bf.DynamicClient().Resource(serviceAccountResource).
				Cluster(bf.TestCluster().Path()).
				Namespace(bf.namespace).
				Get(ctx, resourceName, metav1.GetOptions{})
			return err == nil, err
		},
	)
	
	if err != nil {
		b.Errorf("controller reconciliation test failed: %v", err)
		return time.Since(start)
	}

	// Cleanup
	bf.DynamicClient().Resource(serviceAccountResource).
		Cluster(bf.TestCluster().Path()).
		Namespace(bf.namespace).
		Delete(ctx, resourceName, metav1.DeleteOptions{})
	
	return reconcileTime
}

// BenchmarkTMCPerformanceDetailed runs detailed performance tests with full profiling
func BenchmarkTMCPerformanceDetailed(b *testing.B) {
	// Skip unless explicitly requested
	if os.Getenv("DETAILED_BENCHMARKS") != "true" {
		b.Skip("Detailed benchmarks require DETAILED_BENCHMARKS=true")
	}
	
	// Setup detailed metrics collection
	metricsCollector := NewMetricsCollector("tmc-detailed-performance", "detailed_performance_reports")
	
	// Setup detailed profiling (includes trace and blocking profiles)
	profiler := NewProfiler(GetDetailedProfileConfig("TMC_Detailed"))
	
	// Run profiled benchmark
	profiler.ProfiledBenchmark(b, func(b *testing.B) {
		runTMCIntegrationBenchmark(b, metricsCollector)
	})
	
	// Analyze performance and generate report
	if err := metricsCollector.AnalyzePerformance(); err != nil {
		b.Errorf("failed to analyze performance: %v", err)
	}
	
	// Save detailed performance report
	reportFile := fmt.Sprintf("tmc_detailed_report_%s.json", time.Now().Format("20060102_150405"))
	if err := metricsCollector.SaveReport(reportFile); err != nil {
		b.Errorf("failed to save detailed performance report: %v", err)
	}
	
	// Print summary
	metricsCollector.PrintSummary()
	
	b.Logf("Detailed performance analysis completed. Check detailed_performance_reports/ for full results.")
}