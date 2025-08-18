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
	"sync"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/kcp-dev/kcp/test/e2e/framework"
)

// BenchmarkResourceThroughput measures throughput for different resource operations
func BenchmarkResourceThroughput(b *testing.B) {
	testCases := []struct {
		name      string
		operation string
		workers   int
	}{
		{"Create_Single_Worker", "create", 1},
		{"Create_4_Workers", "create", 4},
		{"Create_8_Workers", "create", 8},
		{"Update_Single_Worker", "update", 1},
		{"Update_4_Workers", "update", 4},
		{"Update_8_Workers", "update", 8},
		{"Delete_Single_Worker", "delete", 1},
		{"Delete_4_Workers", "delete", 4},
		{"List_Operations", "list", 1},
		{"Mixed_Operations", "mixed", 4},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchmarkResourceOperation(b, tc.operation, tc.workers)
		})
	}
}

func benchmarkResourceOperation(b *testing.B, operation string, workers int) {
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

	// Setup resources if needed for update/delete operations
	var existingResources []string
	if operation == "update" || operation == "delete" || operation == "mixed" {
		existingResources = setupExistingResources(b, benchFramework, b.N*workers)
		defer cleanupExistingResources(benchFramework, existingResources)
	}

	b.ResetTimer()
	b.ReportAllocs()

	switch operation {
	case "create":
		benchmarkCreateThroughput(b, benchFramework, workers)
	case "update":
		benchmarkUpdateThroughput(b, benchFramework, existingResources, workers)
	case "delete":
		benchmarkDeleteThroughput(b, benchFramework, existingResources, workers)
	case "list":
		benchmarkListThroughput(b, benchFramework)
	case "mixed":
		benchmarkMixedOperationsThroughput(b, benchFramework, existingResources, workers)
	default:
		b.Fatalf("unknown operation: %s", operation)
	}
}

func setupExistingResources(b *testing.B, bf *BenchmarkFramework, count int) []string {
	ctx := context.Background()
	resources := make([]string, count)
	
	configMapResource := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "configmaps",
	}

	for i := 0; i < count; i++ {
		resourceName := fmt.Sprintf("%sthroughput-existing-%d", bf.ResourcePrefix(), i)
		resources[i] = resourceName

		configMap := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]interface{}{
					"name":      resourceName,
					"namespace": bf.namespace,
					"labels": map[string]interface{}{
						"test-type": "throughput-existing",
						"index":     fmt.Sprintf("%d", i),
					},
				},
				"data": map[string]interface{}{
					"initial-data": fmt.Sprintf("value-%d", i),
					"created-at":   time.Now().Format(time.RFC3339),
				},
			},
		}

		_, err := bf.DynamicClient().Resource(configMapResource).
			Cluster(bf.TestCluster().Path()).
			Namespace(bf.namespace).
			Create(ctx, configMap, metav1.CreateOptions{})
		if err != nil {
			b.Fatalf("failed to create existing resource %s: %v", resourceName, err)
		}
	}

	return resources
}

func cleanupExistingResources(bf *BenchmarkFramework, resources []string) {
	ctx := context.Background()
	configMapResource := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "configmaps",
	}
	
	for _, resourceName := range resources {
		bf.DynamicClient().Resource(configMapResource).
			Cluster(bf.TestCluster().Path()).
			Namespace(bf.namespace).
			Delete(ctx, resourceName, metav1.DeleteOptions{})
	}
}

func benchmarkCreateThroughput(b *testing.B, bf *BenchmarkFramework, workers int) {
	ctx := context.Background()
	configMapResource := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "configmaps",
	}

	var totalOperations int
	operationsChan := make(chan int, workers)
	var wg sync.WaitGroup

	start := time.Now()
	
	// Launch worker goroutines
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			operationCount := 0
			
			for i := 0; i < b.N/workers; i++ {
				resourceName := fmt.Sprintf("%screate-%d-%d", bf.ResourcePrefix(), workerID, i)
				
				configMap := &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]interface{}{
							"name":      resourceName,
							"namespace": bf.namespace,
							"labels": map[string]interface{}{
								"test-type": "create-throughput",
								"worker":    fmt.Sprintf("%d", workerID),
							},
						},
						"data": map[string]interface{}{
							"worker-data": fmt.Sprintf("worker-%d-item-%d", workerID, i),
							"timestamp":   time.Now().Format(time.RFC3339Nano),
						},
					},
				}

				_, err := bf.DynamicClient().Resource(configMapResource).
					Cluster(bf.TestCluster().Path()).
					Namespace(bf.namespace).
					Create(ctx, configMap, metav1.CreateOptions{})
				if err != nil {
					b.Errorf("worker %d failed to create resource: %v", workerID, err)
					continue
				}
				
				operationCount++
			}
			
			operationsChan <- operationCount
		}(w)
	}

	wg.Wait()
	close(operationsChan)
	
	for ops := range operationsChan {
		totalOperations += ops
	}
	
	totalTime := time.Since(start)
	throughput := float64(totalOperations) / totalTime.Seconds()
	
	b.ReportMetric(throughput, "create-ops/sec")
	b.ReportMetric(float64(totalTime.Nanoseconds()/int64(totalOperations)), "avg-create-time-ns")
}

func benchmarkUpdateThroughput(b *testing.B, bf *BenchmarkFramework, existingResources []string, workers int) {
	ctx := context.Background()
	configMapResource := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "configmaps",
	}

	var totalOperations int
	operationsChan := make(chan int, workers)
	var wg sync.WaitGroup

	start := time.Now()
	
	resourceIndex := 0
	var resourceMutex sync.Mutex
	
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			operationCount := 0
			
			for i := 0; i < b.N/workers && resourceIndex < len(existingResources); i++ {
				resourceMutex.Lock()
				if resourceIndex >= len(existingResources) {
					resourceMutex.Unlock()
					break
				}
				resourceName := existingResources[resourceIndex]
				resourceIndex++
				resourceMutex.Unlock()
				
				patch := fmt.Sprintf(`{
					"data": {
						"updated-by": "worker-%d",
						"update-iteration": "%d",
						"updated-at": "%s"
					}
				}`, workerID, i, time.Now().Format(time.RFC3339Nano))

				_, err := bf.DynamicClient().Resource(configMapResource).
					Cluster(bf.TestCluster().Path()).
					Namespace(bf.namespace).
					Patch(ctx, resourceName, "application/merge-patch+json", []byte(patch), metav1.PatchOptions{})
				if err != nil {
					b.Errorf("worker %d failed to update resource: %v", workerID, err)
					continue
				}
				
				operationCount++
			}
			
			operationsChan <- operationCount
		}(w)
	}

	wg.Wait()
	close(operationsChan)
	
	for ops := range operationsChan {
		totalOperations += ops
	}
	
	totalTime := time.Since(start)
	throughput := float64(totalOperations) / totalTime.Seconds()
	
	b.ReportMetric(throughput, "update-ops/sec")
	b.ReportMetric(float64(totalTime.Nanoseconds()/int64(totalOperations)), "avg-update-time-ns")
}

func benchmarkDeleteThroughput(b *testing.B, bf *BenchmarkFramework, existingResources []string, workers int) {
	ctx := context.Background()
	configMapResource := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "configmaps",
	}

	var totalOperations int
	operationsChan := make(chan int, workers)
	var wg sync.WaitGroup

	start := time.Now()
	
	resourceIndex := 0
	var resourceMutex sync.Mutex
	
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			operationCount := 0
			
			for i := 0; i < b.N/workers && resourceIndex < len(existingResources); i++ {
				resourceMutex.Lock()
				if resourceIndex >= len(existingResources) {
					resourceMutex.Unlock()
					break
				}
				resourceName := existingResources[resourceIndex]
				resourceIndex++
				resourceMutex.Unlock()

				err := bf.DynamicClient().Resource(configMapResource).
					Cluster(bf.TestCluster().Path()).
					Namespace(bf.namespace).
					Delete(ctx, resourceName, metav1.DeleteOptions{})
				if err != nil {
					b.Errorf("worker %d failed to delete resource: %v", workerID, err)
					continue
				}
				
				operationCount++
			}
			
			operationsChan <- operationCount
		}(w)
	}

	wg.Wait()
	close(operationsChan)
	
	for ops := range operationsChan {
		totalOperations += ops
	}
	
	totalTime := time.Since(start)
	throughput := float64(totalOperations) / totalTime.Seconds()
	
	b.ReportMetric(throughput, "delete-ops/sec")
	b.ReportMetric(float64(totalTime.Nanoseconds()/int64(totalOperations)), "avg-delete-time-ns")
}

func benchmarkListThroughput(b *testing.B, bf *BenchmarkFramework) {
	ctx := context.Background()
	configMapResource := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "configmaps",
	}

	start := time.Now()
	var totalResources int
	
	for i := 0; i < b.N; i++ {
		list, err := bf.DynamicClient().Resource(configMapResource).
			Cluster(bf.TestCluster().Path()).
			Namespace(bf.namespace).
			List(ctx, metav1.ListOptions{
				LabelSelector: "test-type=throughput-existing",
			})
		if err != nil {
			b.Fatalf("failed to list resources: %v", err)
		}
		
		totalResources += len(list.Items)
	}
	
	totalTime := time.Since(start)
	listThroughput := float64(b.N) / totalTime.Seconds()
	avgResourcesPerList := float64(totalResources) / float64(b.N)
	
	b.ReportMetric(listThroughput, "list-ops/sec")
	b.ReportMetric(avgResourcesPerList, "avg-resources-per-list")
	b.ReportMetric(float64(totalTime.Nanoseconds()/int64(b.N)), "avg-list-time-ns")
}

func benchmarkMixedOperationsThroughput(b *testing.B, bf *BenchmarkFramework, existingResources []string, workers int) {
	ctx := context.Background()
	configMapResource := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "configmaps",
	}

	var totalOperations int
	operationsChan := make(chan int, workers)
	var wg sync.WaitGroup

	start := time.Now()
	
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			operationCount := 0
			
			for i := 0; i < b.N/workers; i++ {
				operation := i % 3 // cycle through create, update, list
				
				switch operation {
				case 0: // Create
					resourceName := fmt.Sprintf("%smixed-create-%d-%d", bf.ResourcePrefix(), workerID, i)
					
					configMap := &unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]interface{}{
								"name":      resourceName,
								"namespace": bf.namespace,
								"labels": map[string]interface{}{
									"test-type": "mixed-operations",
									"operation": "create",
								},
							},
							"data": map[string]interface{}{
								"data": fmt.Sprintf("mixed-create-data-%d", i),
							},
						},
					}

					_, err := bf.DynamicClient().Resource(configMapResource).
						Cluster(bf.TestCluster().Path()).
						Namespace(bf.namespace).
						Create(ctx, configMap, metav1.CreateOptions{})
					if err == nil {
						operationCount++
					}
					
				case 1: // Update
					if i < len(existingResources) {
						resourceName := existingResources[i]
						patch := fmt.Sprintf(`{"data":{"mixed-update":"%d"}}`, i)

						_, err := bf.DynamicClient().Resource(configMapResource).
							Cluster(bf.TestCluster().Path()).
							Namespace(bf.namespace).
							Patch(ctx, resourceName, "application/merge-patch+json", []byte(patch), metav1.PatchOptions{})
						if err == nil {
							operationCount++
						}
					}
					
				case 2: // List
					_, err := bf.DynamicClient().Resource(configMapResource).
						Cluster(bf.TestCluster().Path()).
						Namespace(bf.namespace).
						List(ctx, metav1.ListOptions{
							LabelSelector: "test-type",
						})
					if err == nil {
						operationCount++
					}
				}
			}
			
			operationsChan <- operationCount
		}(w)
	}

	wg.Wait()
	close(operationsChan)
	
	for ops := range operationsChan {
		totalOperations += ops
	}
	
	totalTime := time.Since(start)
	throughput := float64(totalOperations) / totalTime.Seconds()
	
	b.ReportMetric(throughput, "mixed-ops/sec")
	b.ReportMetric(float64(totalTime.Nanoseconds()/int64(totalOperations)), "avg-mixed-op-time-ns")
}