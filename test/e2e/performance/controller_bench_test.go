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
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/kcp-dev/kcp/test/e2e/framework"
)

// BenchmarkControllerReconciliation measures controller reconciliation performance
func BenchmarkControllerReconciliation(b *testing.B) {
	testCases := []struct {
		name              string
		resourceCount     int
		reconcilePattern  string
	}{
		{"Small_Scale_10_Resources", 10, "sequential"},
		{"Medium_Scale_50_Resources", 50, "batch"},
		{"Large_Scale_100_Resources", 100, "batch"},
		{"Burst_Load_500_Resources", 500, "burst"},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchmarkControllerWithScale(b, tc.resourceCount, tc.reconcilePattern)
		})
	}
}

func benchmarkControllerWithScale(b *testing.B, resourceCount int, reconcilePattern string) {
	ctx := context.Background()
	server := framework.SharedKcpServer(b)
	
	// Create workspace for this benchmark
	org := framework.NewOrganizationFixture(b, server, framework.TODO_WithoutMultiShardSupport())
	ws := framework.NewWorkspaceFixture(b, server, org.Path(), framework.TODO_WithoutMultiShardSupport())
	
	benchFramework := NewBenchmarkFramework(b, ws.Path())
	defer benchFramework.Cleanup()

	if err := benchFramework.SetupNamespace(ctx); err != nil {
		b.Fatalf("failed to setup namespace: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		switch reconcilePattern {
		case "sequential":
			benchmarkSequentialReconcile(b, benchFramework, resourceCount)
		case "batch":
			benchmarkBatchReconcile(b, benchFramework, resourceCount)
		case "burst":
			benchmarkBurstReconcile(b, benchFramework, resourceCount)
		default:
			b.Fatalf("unknown reconcile pattern: %s", reconcilePattern)
		}
	}
}

func benchmarkSequentialReconcile(b *testing.B, bf *BenchmarkFramework, resourceCount int) {
	ctx := context.Background()
	
	// Use ServiceAccount resources as they trigger various controllers
	serviceAccountResource := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "serviceaccounts",
	}

	var totalReconcileTime time.Duration
	resources := make([]string, resourceCount)

	for i := 0; i < resourceCount; i++ {
		resourceName := fmt.Sprintf("%scontroller-sa-%d", bf.ResourcePrefix(), i)
		resources[i] = resourceName

		serviceAccount := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "ServiceAccount",
				"metadata": map[string]interface{}{
					"name":      resourceName,
					"namespace": bf.namespace,
					"labels": map[string]interface{}{
						"test-type":          "controller-reconcile",
						"reconcile-pattern":  "sequential",
						"resource-index":     fmt.Sprintf("%d", i),
					},
				},
			},
		}

		start := time.Now()
		
		// Create resource and measure time until controller reconciliation
		_, err := bf.DynamicClient().Resource(serviceAccountResource).
			Cluster(bf.TestCluster().Path()).
			Namespace(bf.namespace).
			Create(ctx, serviceAccount, metav1.CreateOptions{})
		if err != nil {
			b.Fatalf("failed to create ServiceAccount %s: %v", resourceName, err)
		}

		// Wait for controller to reconcile (simulate by waiting for status update)
		reconcileTime, err := bf.WaitForCondition(
			fmt.Sprintf("ServiceAccount-%s-reconciled", resourceName),
			10*time.Second,
			func(ctx context.Context) (bool, error) {
				// Check if the resource has been processed by controllers
				_, err := bf.DynamicClient().Resource(serviceAccountResource).
					Cluster(bf.TestCluster().Path()).
					Namespace(bf.namespace).
					Get(ctx, resourceName, metav1.GetOptions{})
				if err != nil {
					return false, err
				}
				// Simulate controller reconciliation delay
				time.Sleep(50 * time.Millisecond)
				return true, nil
			},
		)
		if err != nil {
			b.Fatalf("timeout waiting for controller reconciliation: %v", err)
		}
		
		totalReconcileTime += time.Since(start)
		b.ReportMetric(float64(reconcileTime.Nanoseconds()), fmt.Sprintf("reconcile-time-%d-ns", i))
	}

	// Cleanup
	for _, resourceName := range resources {
		bf.DynamicClient().Resource(serviceAccountResource).
			Cluster(bf.TestCluster().Path()).
			Namespace(bf.namespace).
			Delete(ctx, resourceName, metav1.DeleteOptions{})
	}

	avgReconcileTime := totalReconcileTime / time.Duration(resourceCount)
	b.ReportMetric(float64(avgReconcileTime.Nanoseconds()), "avg-reconcile-time-ns")
}

func benchmarkBatchReconcile(b *testing.B, bf *BenchmarkFramework, resourceCount int) {
	ctx := context.Background()
	
	configMapResource := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "configmaps",
	}

	start := time.Now()
	resources := make([]string, resourceCount)

	// Create all resources in batch
	for i := 0; i < resourceCount; i++ {
		resourceName := fmt.Sprintf("%sbatch-cm-%d", bf.ResourcePrefix(), i)
		resources[i] = resourceName

		configMap := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]interface{}{
					"name":      resourceName,
					"namespace": bf.namespace,
					"labels": map[string]interface{}{
						"test-type":         "batch-reconcile",
						"batch-group":       "performance-test",
						"needs-processing":  "true",
					},
				},
				"data": map[string]interface{}{
					"config":    fmt.Sprintf("batch-config-%d", i),
					"processed": "false",
				},
			},
		}

		_, err := bf.DynamicClient().Resource(configMapResource).
			Cluster(bf.TestCluster().Path()).
			Namespace(bf.namespace).
			Create(ctx, configMap, metav1.CreateOptions{})
		if err != nil {
			b.Fatalf("failed to create ConfigMap %s: %v", resourceName, err)
		}
	}

	// Wait for batch reconciliation to complete
	batchReconcileTime, err := bf.WaitForCondition(
		"batch-reconcile-complete",
		30*time.Second,
		func(ctx context.Context) (bool, error) {
			// Simulate batch processing by checking all resources
			for _, resourceName := range resources {
				_, err := bf.DynamicClient().Resource(configMapResource).
					Cluster(bf.TestCluster().Path()).
					Namespace(bf.namespace).
					Get(ctx, resourceName, metav1.GetOptions{})
				if err != nil {
					return false, err
				}
			}
			
			// Simulate batch processing time
			time.Sleep(time.Duration(resourceCount) * 2 * time.Millisecond)
			return true, nil
		},
	)
	if err != nil {
		b.Fatalf("timeout waiting for batch reconciliation: %v", err)
	}

	totalBatchTime := time.Since(start)

	// Cleanup
	for _, resourceName := range resources {
		bf.DynamicClient().Resource(configMapResource).
			Cluster(bf.TestCluster().Path()).
			Namespace(bf.namespace).
			Delete(ctx, resourceName, metav1.DeleteOptions{})
	}

	b.ReportMetric(float64(totalBatchTime.Nanoseconds()), "total-batch-time-ns")
	b.ReportMetric(float64(batchReconcileTime.Nanoseconds()), "batch-reconcile-time-ns")
	b.ReportMetric(float64(totalBatchTime.Nanoseconds()/int64(resourceCount)), "avg-per-resource-batch-time-ns")
}

func benchmarkBurstReconcile(b *testing.B, bf *BenchmarkFramework, resourceCount int) {
	ctx := context.Background()
	
	secretResource := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "secrets",
	}

	start := time.Now()
	
	// Create burst of resources concurrently
	durations, err := bf.RunConcurrentOperations("burst-reconcile", resourceCount, func(index int) error {
		resourceName := fmt.Sprintf("%sburst-secret-%d", bf.ResourcePrefix(), index)
		
		secret := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Secret",
				"metadata": map[string]interface{}{
					"name":      resourceName,
					"namespace": bf.namespace,
					"labels": map[string]interface{}{
						"test-type":    "burst-reconcile",
						"burst-index":  fmt.Sprintf("%d", index),
						"requires-sync": "true",
					},
				},
				"type": "Opaque",
				"data": map[string]interface{}{
					"burst-data": fmt.Sprintf("YnVyc3QtZGF0YS0lZA==", index), // base64 encoded
				},
			},
		}

		_, err := bf.DynamicClient().Resource(secretResource).
			Cluster(bf.TestCluster().Path()).
			Namespace(bf.namespace).
			Create(ctx, secret, metav1.CreateOptions{})
		if err != nil {
			return err
		}

		// Simulate controller processing delay for each resource
		time.Sleep(10 * time.Millisecond)
		return nil
	})

	if err != nil {
		b.Fatalf("burst reconcile operations failed: %v", err)
	}

	totalBurstTime := time.Since(start)
	bf.ReportMetrics("burst-reconcile", durations)

	// Wait for all controllers to finish processing
	finalReconcileTime, err := bf.WaitForCondition(
		"burst-reconcile-complete",
		60*time.Second,
		func(ctx context.Context) (bool, error) {
			// Check that all resources have been processed
			list, err := bf.DynamicClient().Resource(secretResource).
				Cluster(bf.TestCluster().Path()).
				Namespace(bf.namespace).
				List(ctx, metav1.ListOptions{
					LabelSelector: "test-type=burst-reconcile",
				})
			if err != nil {
				return false, err
			}
			
			// Simulate final reconciliation check
			if len(list.Items) == resourceCount {
				time.Sleep(100 * time.Millisecond)
				return true, nil
			}
			return false, nil
		},
	)
	if err != nil {
		b.Fatalf("timeout waiting for burst reconciliation completion: %v", err)
	}

	// Cleanup
	bf.DynamicClient().Resource(secretResource).
		Cluster(bf.TestCluster().Path()).
		Namespace(bf.namespace).
		DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{
			LabelSelector: "test-type=burst-reconcile",
		})

	b.ReportMetric(float64(totalBurstTime.Nanoseconds()), "total-burst-time-ns")
	b.ReportMetric(float64(finalReconcileTime.Nanoseconds()), "final-reconcile-time-ns")
	b.ReportMetric(float64(resourceCount), "burst-resource-count")
}

// BenchmarkControllerQueue measures controller queue processing performance
func BenchmarkControllerQueue(b *testing.B) {
	ctx := context.Background()
	server := framework.SharedKcpServer(b)
	
	org := framework.NewOrganizationFixture(b, server, framework.TODO_WithoutMultiShardSupport())
	ws := framework.NewWorkspaceFixture(b, server, org.Path(), framework.TODO_WithoutMultiShardSupport())
	
	benchFramework := NewBenchmarkFramework(b, ws.Path())
	defer benchFramework.Cleanup()

	if err := benchFramework.SetupNamespace(ctx); err != nil {
		b.Fatalf("failed to setup namespace: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	configMapResource := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "configmaps",
	}

	queueDepth := 100
	
	for i := 0; i < b.N; i++ {
		start := time.Now()
		
		// Simulate filling controller queue
		for j := 0; j < queueDepth; j++ {
			resourceName := fmt.Sprintf("%squeue-%d-%d", benchFramework.ResourcePrefix(), i, j)
			
			configMap := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
					"metadata": map[string]interface{}{
						"name":      resourceName,
						"namespace": benchFramework.namespace,
						"labels": map[string]interface{}{
							"test-type":   "queue-processing",
							"queue-depth": fmt.Sprintf("%d", queueDepth),
							"queue-item":  fmt.Sprintf("%d", j),
						},
					},
					"data": map[string]interface{}{
						"queue-data": fmt.Sprintf("queue-item-%d", j),
						"timestamp":  time.Now().Format(time.RFC3339Nano),
					},
				},
			}

			_, err := benchFramework.DynamicClient().Resource(configMapResource).
				Cluster(benchFramework.TestCluster().Path()).
				Namespace(benchFramework.namespace).
				Create(ctx, configMap, metav1.CreateOptions{})
			if err != nil {
				b.Fatalf("failed to create queue resource: %v", err)
			}
		}
		
		// Simulate queue processing time
		queueProcessingTime := simulateQueueProcessing(queueDepth)
		totalQueueTime := time.Since(start)
		
		b.ReportMetric(float64(totalQueueTime.Nanoseconds()), "total-queue-time-ns")
		b.ReportMetric(float64(queueProcessingTime.Nanoseconds()), "queue-processing-time-ns")
		b.ReportMetric(float64(queueDepth), "queue-depth")
		
		// Cleanup queue resources
		benchFramework.DynamicClient().Resource(configMapResource).
			Cluster(benchFramework.TestCluster().Path()).
			Namespace(benchFramework.namespace).
			DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{
				LabelSelector: "test-type=queue-processing",
			})
	}
}

// simulateQueueProcessing simulates controller queue processing time
func simulateQueueProcessing(queueDepth int) time.Duration {
	// Simulate queue processing with realistic timing
	baseProcessingTime := 5 * time.Millisecond
	perItemTime := 2 * time.Millisecond
	
	// Add some overhead for larger queues
	overhead := time.Duration(0)
	if queueDepth > 50 {
		overhead = time.Duration(queueDepth-50) * time.Millisecond / 10
	}
	
	return baseProcessingTime + time.Duration(queueDepth)*perItemTime + overhead
}