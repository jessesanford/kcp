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

// BenchmarkAPIResponseTime measures API server response times for various operations
func BenchmarkAPIResponseTime(b *testing.B) {
	testCases := []struct {
		name       string
		operation  string
		concurrent bool
	}{
		{"GET_Single_Resource", "get", false},
		{"GET_Concurrent", "get", true},
		{"LIST_Resources", "list", false},
		{"LIST_Concurrent", "list", true},
		{"CREATE_Single", "create", false},
		{"CREATE_Concurrent", "create", true},
		{"UPDATE_Single", "update", false},
		{"UPDATE_Concurrent", "update", true},
		{"DELETE_Single", "delete", false},
		{"DELETE_Concurrent", "delete", true},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchmarkAPIOperation(b, tc.operation, tc.concurrent)
		})
	}
}

func benchmarkAPIOperation(b *testing.B, operation string, concurrent bool) {
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

	// Setup test data for operations that need existing resources
	var testResources []string
	if operation == "get" || operation == "update" || operation == "delete" {
		testResources = setupAPITestResources(b, benchFramework, b.N*10)
		defer cleanupAPITestResources(benchFramework, testResources)
	}

	b.ResetTimer()
	b.ReportAllocs()

	if concurrent {
		benchmarkConcurrentAPIOperation(b, benchFramework, operation, testResources)
	} else {
		benchmarkSequentialAPIOperation(b, benchFramework, operation, testResources)
	}
}

func setupAPITestResources(b *testing.B, bf *BenchmarkFramework, count int) []string {
	ctx := context.Background()
	resources := make([]string, count)
	
	configMapResource := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "configmaps",
	}

	for i := 0; i < count; i++ {
		resourceName := fmt.Sprintf("%sapi-test-%d", bf.ResourcePrefix(), i)
		resources[i] = resourceName

		configMap := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]interface{}{
					"name":      resourceName,
					"namespace": bf.namespace,
					"labels": map[string]interface{}{
						"test-type": "api-response-test",
						"index":     fmt.Sprintf("%d", i),
					},
				},
				"data": map[string]interface{}{
					"test-data": fmt.Sprintf("api-test-data-%d", i),
					"size":      "small",
				},
			},
		}

		_, err := bf.DynamicClient().Resource(configMapResource).
			Cluster(bf.TestCluster().Path()).
			Namespace(bf.namespace).
			Create(ctx, configMap, metav1.CreateOptions{})
		if err != nil {
			b.Fatalf("failed to create test resource %s: %v", resourceName, err)
		}
	}

	return resources
}

func cleanupAPITestResources(bf *BenchmarkFramework, resources []string) {
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

func benchmarkSequentialAPIOperation(b *testing.B, bf *BenchmarkFramework, operation string, testResources []string) {
	ctx := context.Background()
	configMapResource := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "configmaps",
	}

	var totalResponseTime time.Duration
	
	for i := 0; i < b.N; i++ {
		var responseTime time.Duration
		
		switch operation {
		case "get":
			if len(testResources) > 0 {
				resourceName := testResources[i%len(testResources)]
				responseTime = bf.MeasureLatency(fmt.Sprintf("get-%s", resourceName), func() error {
					_, err := bf.DynamicClient().Resource(configMapResource).
						Cluster(bf.TestCluster().Path()).
						Namespace(bf.namespace).
						Get(ctx, resourceName, metav1.GetOptions{})
					return err
				})
			}
			
		case "list":
			responseTime = bf.MeasureLatency("list-configmaps", func() error {
				_, err := bf.DynamicClient().Resource(configMapResource).
					Cluster(bf.TestCluster().Path()).
					Namespace(bf.namespace).
					List(ctx, metav1.ListOptions{
						LabelSelector: "test-type=api-response-test",
					})
				return err
			})
			
		case "create":
			resourceName := fmt.Sprintf("%sapi-create-%d", bf.ResourcePrefix(), i)
			configMap := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
					"metadata": map[string]interface{}{
						"name":      resourceName,
						"namespace": bf.namespace,
						"labels": map[string]interface{}{
							"test-type": "api-create-test",
						},
					},
					"data": map[string]interface{}{
						"create-data": fmt.Sprintf("create-test-%d", i),
					},
				},
			}
			
			responseTime = bf.MeasureLatency(fmt.Sprintf("create-%s", resourceName), func() error {
				_, err := bf.DynamicClient().Resource(configMapResource).
					Cluster(bf.TestCluster().Path()).
					Namespace(bf.namespace).
					Create(ctx, configMap, metav1.CreateOptions{})
				return err
			})
			
		case "update":
			if len(testResources) > 0 {
				resourceName := testResources[i%len(testResources)]
				patch := fmt.Sprintf(`{"data":{"updated-at":"%s","iteration":"%d"}}`, 
					time.Now().Format(time.RFC3339), i)
				
				responseTime = bf.MeasureLatency(fmt.Sprintf("update-%s", resourceName), func() error {
					_, err := bf.DynamicClient().Resource(configMapResource).
						Cluster(bf.TestCluster().Path()).
						Namespace(bf.namespace).
						Patch(ctx, resourceName, "application/merge-patch+json", []byte(patch), metav1.PatchOptions{})
					return err
				})
			}
			
		case "delete":
			if len(testResources) > 0 && i < len(testResources) {
				resourceName := testResources[i]
				responseTime = bf.MeasureLatency(fmt.Sprintf("delete-%s", resourceName), func() error {
					return bf.DynamicClient().Resource(configMapResource).
						Cluster(bf.TestCluster().Path()).
						Namespace(bf.namespace).
						Delete(ctx, resourceName, metav1.DeleteOptions{})
				})
			}
		}
		
		totalResponseTime += responseTime
		b.ReportMetric(float64(responseTime.Nanoseconds()), fmt.Sprintf("%s-response-time-ns", operation))
	}
	
	avgResponseTime := totalResponseTime / time.Duration(b.N)
	b.ReportMetric(float64(avgResponseTime.Nanoseconds()), fmt.Sprintf("avg-%s-response-time-ns", operation))
}

func benchmarkConcurrentAPIOperation(b *testing.B, bf *BenchmarkFramework, operation string, testResources []string) {
	ctx := context.Background()
	configMapResource := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "configmaps",
	}

	concurrency := 10
	iterations := b.N / concurrency
	if iterations == 0 {
		iterations = 1
	}

	var totalResponseTime time.Duration
	var responseTimes []time.Duration
	var mu sync.Mutex

	for iter := 0; iter < iterations; iter++ {
		start := time.Now()
		
		durations, err := bf.RunConcurrentOperations(fmt.Sprintf("concurrent-%s", operation), concurrency, func(index int) error {
			var opStart time.Time
			var opDuration time.Duration
			
			switch operation {
			case "get":
				if len(testResources) > 0 {
					resourceName := testResources[(iter*concurrency+index)%len(testResources)]
					opStart = time.Now()
					_, err := bf.DynamicClient().Resource(configMapResource).
						Cluster(bf.TestCluster().Path()).
						Namespace(bf.namespace).
						Get(ctx, resourceName, metav1.GetOptions{})
					opDuration = time.Since(opStart)
					if err != nil {
						return err
					}
				}
				
			case "list":
				opStart = time.Now()
				_, err := bf.DynamicClient().Resource(configMapResource).
					Cluster(bf.TestCluster().Path()).
					Namespace(bf.namespace).
					List(ctx, metav1.ListOptions{
						LabelSelector: "test-type=api-response-test",
						Limit:         10,
					})
				opDuration = time.Since(opStart)
				if err != nil {
					return err
				}
				
			case "create":
				resourceName := fmt.Sprintf("%sconcurrent-create-%d-%d", bf.ResourcePrefix(), iter, index)
				configMap := &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]interface{}{
							"name":      resourceName,
							"namespace": bf.namespace,
							"labels": map[string]interface{}{
								"test-type": "concurrent-create-test",
								"iteration": fmt.Sprintf("%d", iter),
								"index":     fmt.Sprintf("%d", index),
							},
						},
						"data": map[string]interface{}{
							"concurrent-data": fmt.Sprintf("concurrent-create-%d-%d", iter, index),
						},
					},
				}
				
				opStart = time.Now()
				_, err := bf.DynamicClient().Resource(configMapResource).
					Cluster(bf.TestCluster().Path()).
					Namespace(bf.namespace).
					Create(ctx, configMap, metav1.CreateOptions{})
				opDuration = time.Since(opStart)
				if err != nil {
					return err
				}
				
			case "update":
				if len(testResources) > 0 {
					resourceName := testResources[(iter*concurrency+index)%len(testResources)]
					patch := fmt.Sprintf(`{"data":{"concurrent-updated":"%s","thread":"%d"}}`, 
						time.Now().Format(time.RFC3339Nano), index)
					
					opStart = time.Now()
					_, err := bf.DynamicClient().Resource(configMapResource).
						Cluster(bf.TestCluster().Path()).
						Namespace(bf.namespace).
						Patch(ctx, resourceName, "application/merge-patch+json", []byte(patch), metav1.PatchOptions{})
					opDuration = time.Since(opStart)
					if err != nil {
						return err
					}
				}
			}
			
			mu.Lock()
			responseTimes = append(responseTimes, opDuration)
			mu.Unlock()
			
			return nil
		})
		
		if err != nil {
			b.Fatalf("concurrent %s operations failed: %v", operation, err)
		}
		
		iterationTime := time.Since(start)
		totalResponseTime += iterationTime
		
		bf.ReportMetrics(fmt.Sprintf("concurrent-%s", operation), durations)
	}
	
	if len(responseTimes) > 0 {
		var totalOpTime time.Duration
		for _, rt := range responseTimes {
			totalOpTime += rt
		}
		avgOpTime := totalOpTime / time.Duration(len(responseTimes))
		b.ReportMetric(float64(avgOpTime.Nanoseconds()), fmt.Sprintf("avg-concurrent-%s-op-time-ns", operation))
	}
	
	avgIterationTime := totalResponseTime / time.Duration(iterations)
	b.ReportMetric(float64(avgIterationTime.Nanoseconds()), fmt.Sprintf("avg-concurrent-%s-iteration-time-ns", operation))
}

// BenchmarkAPILatencyUnderLoad measures API latency under various load conditions
func BenchmarkAPILatencyUnderLoad(b *testing.B) {
	ctx := context.Background()
	server := framework.SharedKcpServer(b)
	
	org := framework.NewOrganizationFixture(b, server, framework.TODO_WithoutMultiShardSupport())
	ws := framework.NewWorkspaceFixture(b, server, org.Path(), framework.TODO_WithoutMultiShardSupport())
	
	benchFramework := NewBenchmarkFramework(b, ws.Path())
	defer benchFramework.Cleanup()

	if err := benchFramework.SetupNamespace(ctx); err != nil {
		b.Fatalf("failed to setup namespace: %v", err)
	}

	configMapResource := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "configmaps",
	}

	b.ResetTimer()
	b.ReportAllocs()

	loadLevels := []int{5, 10, 20, 50}
	
	for _, loadLevel := range loadLevels {
		b.Run(fmt.Sprintf("Load_%d_Concurrent", loadLevel), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				start := time.Now()
				
				// Generate load with concurrent operations
				durations, err := benchFramework.RunConcurrentOperations("load-test", loadLevel, func(index int) error {
					resourceName := fmt.Sprintf("%sload-test-%d-%d", benchFramework.ResourcePrefix(), i, index)
					
					// Mix of operations to simulate real load
					switch index % 4 {
					case 0: // Create
						configMap := &unstructured.Unstructured{
							Object: map[string]interface{}{
								"apiVersion": "v1",
								"kind":       "ConfigMap",
								"metadata": map[string]interface{}{
									"name":      resourceName,
									"namespace": benchFramework.namespace,
									"labels": map[string]interface{}{
										"test-type":  "load-test",
										"load-level": fmt.Sprintf("%d", loadLevel),
									},
								},
								"data": map[string]interface{}{
									"load-data": fmt.Sprintf("load-test-data-%d", index),
								},
							},
						}
						
						_, err := benchFramework.DynamicClient().Resource(configMapResource).
							Cluster(benchFramework.TestCluster().Path()).
							Namespace(benchFramework.namespace).
							Create(ctx, configMap, metav1.CreateOptions{})
						return err
						
					case 1: // List
						_, err := benchFramework.DynamicClient().Resource(configMapResource).
							Cluster(benchFramework.TestCluster().Path()).
							Namespace(benchFramework.namespace).
							List(ctx, metav1.ListOptions{
								LabelSelector: "test-type=load-test",
								Limit:         5,
							})
						return err
						
					case 2, 3: // Get (more frequent)
						// Try to get a previously created resource
						testResourceName := fmt.Sprintf("%sload-test-%d-%d", benchFramework.ResourcePrefix(), i, index-2)
						_, err := benchFramework.DynamicClient().Resource(configMapResource).
							Cluster(benchFramework.TestCluster().Path()).
							Namespace(benchFramework.namespace).
							Get(ctx, testResourceName, metav1.GetOptions{})
						// Ignore not found errors for get operations
						if err != nil && !metav1.IsNotFound(err.(*metav1.StatusError).ErrStatus) {
							return err
						}
						return nil
					}
					return nil
				})
				
				if err != nil {
					b.Fatalf("load test failed: %v", err)
				}
				
				totalLoadTime := time.Since(start)
				benchFramework.ReportMetrics(fmt.Sprintf("load-%d", loadLevel), durations)
				
				b.ReportMetric(float64(totalLoadTime.Nanoseconds()), fmt.Sprintf("load-%d-total-time-ns", loadLevel))
				b.ReportMetric(float64(loadLevel), "concurrent-operations")
			}
		})
	}
}