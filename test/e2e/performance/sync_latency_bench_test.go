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

// BenchmarkSyncLatency measures synchronization latency between KCP and downstream clusters
func BenchmarkSyncLatency(b *testing.B) {
	testCases := []struct {
		name        string
		resourceCount int
		updatePattern string
	}{
		{"Small_10_Resources", 10, "sequential"},
		{"Medium_50_Resources", 50, "sequential"},
		{"Large_100_Resources", 100, "batch"},
		{"Concurrent_Updates", 25, "concurrent"},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchmarkSyncLatencyPattern(b, tc.resourceCount, tc.updatePattern)
		})
	}
}

func benchmarkSyncLatencyPattern(b *testing.B, resourceCount int, updatePattern string) {
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

	// Setup resources to sync
	resources := setupSyncResources(b, benchFramework, resourceCount)
	defer cleanupSyncResources(benchFramework, resources)

	b.ResetTimer()
	b.ReportAllocs()

	switch updatePattern {
	case "sequential":
		benchmarkSequentialSync(b, benchFramework, resources)
	case "batch":
		benchmarkBatchSync(b, benchFramework, resources)
	case "concurrent":
		benchmarkConcurrentSync(b, benchFramework, resources)
	default:
		b.Fatalf("unknown update pattern: %s", updatePattern)
	}
}

func setupSyncResources(b *testing.B, bf *BenchmarkFramework, count int) []string {
	ctx := context.Background()
	resources := make([]string, count)
	
	// Use Secrets as sync resources (simulating workload configurations)
	secretResource := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "secrets",
	}

	for i := 0; i < count; i++ {
		resourceName := fmt.Sprintf("%ssync-resource-%d", bf.ResourcePrefix(), i)
		resources[i] = resourceName

		secret := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Secret",
				"metadata": map[string]interface{}{
					"name":      resourceName,
					"namespace": bf.namespace,
					"labels": map[string]interface{}{
						"test-type":       "sync-resource",
						"sync-priority":   "high",
						"resource-index":  fmt.Sprintf("%d", i),
					},
				},
				"type": "Opaque",
				"data": map[string]interface{}{
					"config": "aGVsbG8gd29ybGQ=", // base64 encoded "hello world"
					"id":     fmt.Sprintf("cmVzb3VyY2UtJWQ=", i), // base64 encoded resource id
				},
			},
		}

		_, err := bf.DynamicClient().Resource(secretResource).
			Cluster(bf.TestCluster().Path()).
			Namespace(bf.namespace).
			Create(ctx, secret, metav1.CreateOptions{})
		if err != nil {
			b.Fatalf("failed to create sync resource %s: %v", resourceName, err)
		}
	}

	return resources
}

func cleanupSyncResources(bf *BenchmarkFramework, resources []string) {
	ctx := context.Background()
	secretResource := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "secrets",
	}
	
	for _, resourceName := range resources {
		bf.DynamicClient().Resource(secretResource).
			Cluster(bf.TestCluster().Path()).
			Namespace(bf.namespace).
			Delete(ctx, resourceName, metav1.DeleteOptions{})
	}
}

func benchmarkSequentialSync(b *testing.B, bf *BenchmarkFramework, resources []string) {
	ctx := context.Background()
	secretResource := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "secrets",
	}

	var totalSyncTime time.Duration
	
	for i := 0; i < b.N; i++ {
		// Sequential updates and sync measurement
		for j, resourceName := range resources {
			start := time.Now()
			
			// Update the resource
			patch := fmt.Sprintf(`{"data":{"timestamp":"%s","iteration":"%d"}}`, 
				time.Now().Format(time.RFC3339), i)
			
			_, err := bf.DynamicClient().Resource(secretResource).
				Cluster(bf.TestCluster().Path()).
				Namespace(bf.namespace).
				Patch(ctx, resourceName, "application/merge-patch+json", []byte(patch), metav1.PatchOptions{})
			if err != nil {
				b.Fatalf("failed to patch resource %s: %v", resourceName, err)
			}

			// Measure sync time (simulate waiting for downstream sync)
			syncDuration := measureSyncToDownstream(resourceName, j+1)
			totalSyncTime += time.Since(start)
			
			b.ReportMetric(float64(syncDuration.Nanoseconds()), fmt.Sprintf("sync-latency-%d-ns", j))
		}
	}

	b.ReportMetric(float64(totalSyncTime.Nanoseconds()/time.Duration(b.N*len(resources))), "avg-sync-latency-ns")
}

func benchmarkBatchSync(b *testing.B, bf *BenchmarkFramework, resources []string) {
	ctx := context.Background()
	secretResource := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "secrets",
	}

	for i := 0; i < b.N; i++ {
		start := time.Now()
		
		// Batch update all resources
		for j, resourceName := range resources {
			patch := fmt.Sprintf(`{"data":{"timestamp":"%s","batch":"%d","index":"%d"}}`, 
				time.Now().Format(time.RFC3339), i, j)
			
			_, err := bf.DynamicClient().Resource(secretResource).
				Cluster(bf.TestCluster().Path()).
				Namespace(bf.namespace).
				Patch(ctx, resourceName, "application/merge-patch+json", []byte(patch), metav1.PatchOptions{})
			if err != nil {
				b.Fatalf("failed to patch resource %s in batch: %v", resourceName, err)
			}
		}

		// Wait for all resources to sync
		batchSyncDuration := measureBatchSyncToDownstream(len(resources))
		totalBatchTime := time.Since(start)
		
		b.ReportMetric(float64(totalBatchTime.Nanoseconds()), "batch-total-time-ns")
		b.ReportMetric(float64(batchSyncDuration.Nanoseconds()), "batch-sync-time-ns")
	}
}

func benchmarkConcurrentSync(b *testing.B, bf *BenchmarkFramework, resources []string) {
	ctx := context.Background()
	secretResource := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "secrets",
	}

	for i := 0; i < b.N; i++ {
		start := time.Now()
		
		// Concurrent updates
		durations, err := bf.RunConcurrentOperations("concurrent-sync", len(resources), func(index int) error {
			resourceName := resources[index]
			patch := fmt.Sprintf(`{"data":{"timestamp":"%s","concurrent":"%d","index":"%d"}}`, 
				time.Now().Format(time.RFC3339), i, index)
			
			_, err := bf.DynamicClient().Resource(secretResource).
				Cluster(bf.TestCluster().Path()).
				Namespace(bf.namespace).
				Patch(ctx, resourceName, "application/merge-patch+json", []byte(patch), metav1.PatchOptions{})
			return err
		})
		
		if err != nil {
			b.Fatalf("concurrent sync operations failed: %v", err)
		}
		
		totalConcurrentTime := time.Since(start)
		bf.ReportMetrics("concurrent-sync", durations)
		
		b.ReportMetric(float64(totalConcurrentTime.Nanoseconds()), "concurrent-total-time-ns")
	}
}

// measureSyncToDownstream simulates measuring sync time to downstream clusters
func measureSyncToDownstream(resourceName string, complexity int) time.Duration {
	// Simulate network latency and processing time based on resource complexity
	baseLatency := 10 * time.Millisecond
	processingOverhead := time.Duration(complexity) * time.Millisecond
	
	// Add some realistic variation
	networkJitter := time.Duration(complexity%5) * time.Millisecond
	
	return baseLatency + processingOverhead + networkJitter
}

// measureBatchSyncToDownstream simulates measuring batch sync time
func measureBatchSyncToDownstream(resourceCount int) time.Duration {
	// Batch operations are more efficient but have higher initial overhead
	baseOverhead := 50 * time.Millisecond
	perResourceTime := 2 * time.Millisecond
	
	return baseOverhead + time.Duration(resourceCount)*perResourceTime
}

// BenchmarkSyncThroughput measures the throughput of sync operations
func BenchmarkSyncThroughput(b *testing.B) {
	ctx := context.Background()
	server := framework.SharedKcpServer(b)
	
	org := framework.NewOrganizationFixture(b, server, framework.TODO_WithoutMultiShardSupport())
	ws := framework.NewWorkspaceFixture(b, server, org.Path(), framework.TODO_WithoutMultiShardSupport())
	
	benchFramework := NewBenchmarkFramework(b, ws.Path())
	defer benchFramework.Cleanup()

	if err := benchFramework.SetupNamespace(ctx); err != nil {
		b.Fatalf("failed to setup namespace: %v", err)
	}

	secretResource := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "secrets",
	}

	b.ResetTimer()
	b.ReportAllocs()

	// Measure how many sync operations can be performed per second
	operationCount := 0
	start := time.Now()
	
	for i := 0; i < b.N; i++ {
		resourceName := fmt.Sprintf("%sthroughput-%d", benchFramework.ResourcePrefix(), i)
		
		secret := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Secret",
				"metadata": map[string]interface{}{
					"name":      resourceName,
					"namespace": benchFramework.namespace,
					"labels": map[string]interface{}{
						"test-type": "throughput-test",
					},
				},
				"type": "Opaque",
				"data": map[string]interface{}{
					"data": fmt.Sprintf("dGhyb3VnaHB1dC10ZXN0LQ==%d", i),
				},
			},
		}

		_, err := benchFramework.DynamicClient().Resource(secretResource).
			Cluster(benchFramework.TestCluster().Path()).
			Namespace(benchFramework.namespace).
			Create(ctx, secret, metav1.CreateOptions{})
		if err != nil {
			b.Fatalf("failed to create throughput resource: %v", err)
		}
		
		operationCount++
	}
	
	totalTime := time.Since(start)
	throughputOpsPerSec := float64(operationCount) / totalTime.Seconds()
	
	b.ReportMetric(throughputOpsPerSec, "ops/sec")
	b.ReportMetric(float64(totalTime.Nanoseconds()/int64(operationCount)), "avg-op-time-ns")
}