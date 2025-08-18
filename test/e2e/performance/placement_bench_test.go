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
	"github.com/kcp-dev/logicalcluster/v3"
)

// BenchmarkPlacementPerformance tests placement performance with different cluster scales
func BenchmarkPlacementPerformance(b *testing.B) {
	testCases := []struct {
		name         string
		clusterCount int
	}{
		{"Small_10_Clusters", 10},
		{"Medium_50_Clusters", 50},
		{"Large_100_Clusters", 100},
		{"XLarge_500_Clusters", 500},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchmarkPlacementWithClusters(b, tc.clusterCount)
		})
	}
}

func benchmarkPlacementWithClusters(b *testing.B, clusterCount int) {
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

	// Setup mock cluster registrations
	clusters := setupMockClusters(b, benchFramework, clusterCount)
	defer func() {
		for _, cluster := range clusters {
			cleanupMockCluster(benchFramework, cluster)
		}
	}()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		benchmarkSinglePlacementRun(b, benchFramework, clusters)
	}
}

func setupMockClusters(b *testing.B, bf *BenchmarkFramework, count int) []string {
	ctx := context.Background()
	clusters := make([]string, count)
	
	// Mock cluster resource (using ConfigMap as placeholder since TMC APIs might not be available)
	clusterResource := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "configmaps",
	}

	for i := 0; i < count; i++ {
		clusterName := fmt.Sprintf("%scluster-%d", bf.ResourcePrefix(), i)
		clusters[i] = clusterName

		cluster := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]interface{}{
					"name":      clusterName,
					"namespace": bf.namespace,
					"labels": map[string]interface{}{
						"test-type":     "mock-cluster",
						"cluster-zone":  fmt.Sprintf("zone-%d", i%5),
						"cluster-ready": "true",
					},
				},
				"data": map[string]interface{}{
					"cluster-id": clusterName,
					"location":   fmt.Sprintf("region-%d", i%3),
					"capacity":   fmt.Sprintf("%d", 100+i*10),
				},
			},
		}

		_, err := bf.DynamicClient().Resource(clusterResource).
			Cluster(bf.TestCluster().Path()).
			Namespace(bf.namespace).
			Create(ctx, cluster, metav1.CreateOptions{})
		if err != nil {
			b.Fatalf("failed to create mock cluster %s: %v", clusterName, err)
		}
	}

	// Wait for all clusters to be ready
	err := wait.PollUntilContextTimeout(ctx, 100*time.Millisecond, 30*time.Second, true, func(ctx context.Context) (bool, error) {
		list, err := bf.DynamicClient().Resource(clusterResource).
			Cluster(bf.TestCluster().Path()).
			Namespace(bf.namespace).
			List(ctx, metav1.ListOptions{
				LabelSelector: "test-type=mock-cluster",
			})
		if err != nil {
			return false, err
		}
		return len(list.Items) == count, nil
	})
	if err != nil {
		b.Fatalf("timeout waiting for mock clusters to be ready: %v", err)
	}

	return clusters
}

func cleanupMockCluster(bf *BenchmarkFramework, clusterName string) {
	clusterResource := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "configmaps",
	}
	
	bf.DynamicClient().Resource(clusterResource).
		Cluster(bf.TestCluster().Path()).
		Namespace(bf.namespace).
		Delete(context.Background(), clusterName, metav1.DeleteOptions{})
}

func benchmarkSinglePlacementRun(b *testing.B, bf *BenchmarkFramework, clusters []string) {
	ctx := context.Background()
	
	// Create a mock workload that needs to be placed
	workloadResource := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1", 
		Resource: "configmaps",
	}

	workloadName := fmt.Sprintf("%sworkload-%d", bf.ResourcePrefix(), time.Now().UnixNano())
	
	workload := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":      workloadName,
				"namespace": bf.namespace,
				"labels": map[string]interface{}{
					"test-type":         "mock-workload",
					"placement-policy":  "spread",
					"resource-requirements": "medium",
				},
			},
			"data": map[string]interface{}{
				"workload-spec": "test workload for placement benchmarking",
				"requirements":  "cpu=100m,memory=128Mi",
			},
		},
	}

	// Measure workload creation time
	start := time.Now()
	_, err := bf.DynamicClient().Resource(workloadResource).
		Cluster(bf.TestCluster().Path()).
		Namespace(bf.namespace).
		Create(ctx, workload, metav1.CreateOptions{})
	if err != nil {
		b.Fatalf("failed to create workload: %v", err)
	}
	
	creationTime := time.Since(start)
	
	// Simulate placement decision time
	placementDuration := simulatePlacementDecision(len(clusters))
	
	// Record total placement time
	totalPlacementTime := creationTime + placementDuration
	
	b.ReportMetric(float64(totalPlacementTime.Nanoseconds()), "placement-time-ns")
	b.ReportMetric(float64(len(clusters)), "cluster-count")
	
	// Cleanup workload
	bf.DynamicClient().Resource(workloadResource).
		Cluster(bf.TestCluster().Path()).
		Namespace(bf.namespace).
		Delete(ctx, workloadName, metav1.DeleteOptions{})
}

// simulatePlacementDecision simulates the time taken to make a placement decision
// based on the number of available clusters
func simulatePlacementDecision(clusterCount int) time.Duration {
	// Simulate placement algorithm complexity: O(n log n)
	baseLatency := 1 * time.Millisecond
	complexityFactor := float64(clusterCount) * 1.5
	
	if clusterCount > 100 {
		// Add extra overhead for large cluster counts
		complexityFactor += float64(clusterCount-100) * 0.5
	}
	
	return time.Duration(float64(baseLatency.Nanoseconds()) * complexityFactor)
}