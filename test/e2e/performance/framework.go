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
	"runtime"
	"sync"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"

	kcpdynamic "github.com/kcp-dev/client-go/dynamic"

	kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
	kcptesting "github.com/kcp-dev/kcp/sdk/testing"
	"github.com/kcp-dev/logicalcluster/v3"
)

// BenchmarkFramework provides performance testing utilities built on top of the existing E2E framework
type BenchmarkFramework struct {
	t                   *testing.B
	kcpClusterClient    kcpclientset.ClusterInterface
	dynamicClient       kcpdynamic.ClusterInterface
	discoveryClient     discovery.DiscoveryInterface
	testCluster         logicalcluster.Name
	namespace           string
	resourceCleanupFunc []func()
	mu                  sync.RWMutex
}

// NewBenchmarkFramework creates a new performance benchmark framework
func NewBenchmarkFramework(b *testing.B, testCluster logicalcluster.Name) *BenchmarkFramework {
	server := kcptesting.SharedKcpServer(b)
	cfg := server.BaseConfig(b)
	
	kcpClusterClient, err := kcpclientset.NewForConfig(cfg)
	if err != nil {
		b.Fatalf("failed to create kcp cluster client: %v", err)
	}

	dynamicClient, err := kcpdynamic.NewForConfig(cfg)
	if err != nil {
		b.Fatalf("failed to create dynamic client: %v", err)
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		b.Fatalf("failed to create discovery client: %v", err)
	}

	return &BenchmarkFramework{
		t:                b,
		kcpClusterClient: kcpClusterClient,
		dynamicClient:    dynamicClient,
		discoveryClient:  discoveryClient,
		testCluster:      testCluster,
		namespace:        "perf-benchmarks",
	}
}

// SetupNamespace creates a dedicated namespace for performance tests
func (bf *BenchmarkFramework) SetupNamespace(ctx context.Context) error {
	// Create namespace for performance tests
	nsResource := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "namespaces",
	}
	
	namespace := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Namespace",
			"metadata": map[string]interface{}{
				"name": bf.namespace,
				"labels": map[string]interface{}{
					"test-type": "performance-benchmark",
					"test-run":  fmt.Sprintf("%d", time.Now().Unix()),
				},
			},
		},
	}

	_, err := bf.dynamicClient.Resource(nsResource).
		Cluster(bf.testCluster.Path()).
		Create(ctx, namespace, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	// Register cleanup
	bf.addCleanupFunc(func() {
		bf.dynamicClient.Resource(nsResource).
			Cluster(bf.testCluster.Path()).
			Delete(context.Background(), bf.namespace, metav1.DeleteOptions{})
	})

	return nil
}

// Cleanup runs all registered cleanup functions
func (bf *BenchmarkFramework) Cleanup() {
	bf.mu.RLock()
	cleanupFuncs := make([]func(), len(bf.resourceCleanupFunc))
	copy(cleanupFuncs, bf.resourceCleanupFunc)
	bf.mu.RUnlock()

	for i := len(cleanupFuncs) - 1; i >= 0; i-- {
		cleanupFuncs[i]()
	}
}

// addCleanupFunc registers a cleanup function to be called during cleanup
func (bf *BenchmarkFramework) addCleanupFunc(f func()) {
	bf.mu.Lock()
	defer bf.mu.Unlock()
	bf.resourceCleanupFunc = append(bf.resourceCleanupFunc, f)
}

// MeasureLatency measures operation latency and reports it
func (bf *BenchmarkFramework) MeasureLatency(operation string, fn func() error) time.Duration {
	start := time.Now()
	err := fn()
	duration := time.Since(start)
	
	if err != nil {
		bf.t.Errorf("operation %s failed: %v", operation, err)
		return duration
	}

	return duration
}

// RunConcurrentOperations executes operations concurrently and measures performance
func (bf *BenchmarkFramework) RunConcurrentOperations(operationName string, concurrency int, operationFn func(int) error) ([]time.Duration, error) {
	results := make([]time.Duration, concurrency)
	errors := make([]error, concurrency)
	var wg sync.WaitGroup

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			start := time.Now()
			err := operationFn(index)
			results[index] = time.Since(start)
			errors[index] = err
		}(i)
	}

	wg.Wait()

	// Check for errors
	var firstError error
	for i, err := range errors {
		if err != nil {
			bf.t.Logf("concurrent operation %s[%d] failed: %v", operationName, i, err)
			if firstError == nil {
				firstError = err
			}
		}
	}

	return results, firstError
}

// WaitForCondition waits for a condition to be met with timeout and measures the wait time
func (bf *BenchmarkFramework) WaitForCondition(conditionName string, timeout time.Duration, conditionFn wait.ConditionWithContextFunc) (time.Duration, error) {
	start := time.Now()
	err := wait.PollUntilContextTimeout(context.Background(), 100*time.Millisecond, timeout, true, conditionFn)
	duration := time.Since(start)
	
	if err != nil {
		return duration, fmt.Errorf("condition %s not met within %v: %w", conditionName, timeout, err)
	}
	
	return duration, nil
}

// ReportMetrics reports performance metrics for the benchmark
func (bf *BenchmarkFramework) ReportMetrics(operationName string, durations []time.Duration) {
	if len(durations) == 0 {
		return
	}

	var total time.Duration
	min := durations[0]
	max := durations[0]

	for _, d := range durations {
		total += d
		if d < min {
			min = d
		}
		if d > max {
			max = d
		}
	}

	avg := total / time.Duration(len(durations))
	
	bf.t.Logf("Performance metrics for %s:", operationName)
	bf.t.Logf("  Operations: %d", len(durations))
	bf.t.Logf("  Average: %v", avg)
	bf.t.Logf("  Min: %v", min)
	bf.t.Logf("  Max: %v", max)
	bf.t.Logf("  Total: %v", total)
}

// GetMemoryStats returns current memory statistics
func (bf *BenchmarkFramework) GetMemoryStats() (runtime.MemStats, error) {
	var memStats runtime.MemStats
	runtime.GC() // Force GC to get accurate stats
	runtime.ReadMemStats(&memStats)
	return memStats, nil
}

// ResourcePrefix returns a unique prefix for test resources
func (bf *BenchmarkFramework) ResourcePrefix() string {
	return "pb-"
}

// TestCluster returns the logical cluster for this benchmark
func (bf *BenchmarkFramework) TestCluster() logicalcluster.Name {
	return bf.testCluster
}

// KcpClusterClient returns the KCP cluster client
func (bf *BenchmarkFramework) KcpClusterClient() kcpclientset.ClusterInterface {
	return bf.kcpClusterClient
}

// DynamicClient returns the dynamic client
func (bf *BenchmarkFramework) DynamicClient() kcpdynamic.ClusterInterface {
	return bf.dynamicClient
}

// DiscoveryClient returns the discovery client
func (bf *BenchmarkFramework) DiscoveryClient() discovery.DiscoveryInterface {
	return bf.discoveryClient
}