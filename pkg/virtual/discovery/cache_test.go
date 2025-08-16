/*
Copyright 2023 The KCP Authors.

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

package discovery

import (
	"context"
	"sync"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/kcp-dev/kcp/pkg/virtual/interfaces"
	"github.com/kcp-dev/logicalcluster/v3"
)

func TestDiscoveryCache_BasicOperations(t *testing.T) {
	cache := NewDiscoveryCache(60).(*DiscoveryCache) // 60 second TTL
	defer cache.Stop()

	workspace := logicalcluster.New("test-workspace")
	resources := []interfaces.ResourceInfo{
		{
			GroupVersionResource: schema.GroupVersionResource{
				Group: "apps", Version: "v1", Resource: "deployments",
			},
			APIResource: metav1.APIResource{Name: "deployments"},
			Workspace:   workspace,
		},
	}

	// Test cache miss
	result, found := cache.GetResources(workspace)
	if found {
		t.Error("Expected cache miss but got hit")
	}
	if result != nil {
		t.Error("Expected nil result on cache miss")
	}

	// Test cache set
	cache.SetResources(workspace, resources, 0) // Use default TTL

	// Test cache hit
	result, found = cache.GetResources(workspace)
	if !found {
		t.Error("Expected cache hit but got miss")
	}
	if len(result) != len(resources) {
		t.Errorf("Expected %d resources, got %d", len(resources), len(result))
	}
}

func TestDiscoveryCache_TTLExpiration(t *testing.T) {
	cache := NewDiscoveryCache(1).(*DiscoveryCache) // 1 second TTL
	defer cache.Stop()

	workspace := logicalcluster.New("test-workspace")
	resources := []interfaces.ResourceInfo{
		{
			GroupVersionResource: schema.GroupVersionResource{
				Group: "apps", Version: "v1", Resource: "deployments",
			},
			APIResource: metav1.APIResource{Name: "deployments"},
			Workspace:   workspace,
		},
	}

	// Cache the resources
	cache.SetResources(workspace, resources, 1) // 1 second TTL

	// Should be available immediately
	_, found := cache.GetResources(workspace)
	if !found {
		t.Error("Expected cache hit immediately after set")
	}

	// Wait for expiration
	time.Sleep(1200 * time.Millisecond)

	// Should now be expired
	_, found = cache.GetResources(workspace)
	if found {
		t.Error("Expected cache miss after TTL expiration")
	}
}

func TestDiscoveryCache_WorkspaceIsolation(t *testing.T) {
	cache := NewDiscoveryCache(60).(*DiscoveryCache)
	defer cache.Stop()

	workspace1 := logicalcluster.New("workspace-1")
	workspace2 := logicalcluster.New("workspace-2")

	resources1 := []interfaces.ResourceInfo{
		{
			GroupVersionResource: schema.GroupVersionResource{
				Group: "apps", Version: "v1", Resource: "deployments",
			},
			APIResource: metav1.APIResource{Name: "deployments"},
			Workspace:   workspace1,
		},
	}

	resources2 := []interfaces.ResourceInfo{
		{
			GroupVersionResource: schema.GroupVersionResource{
				Group: "v1", Version: "", Resource: "pods",
			},
			APIResource: metav1.APIResource{Name: "pods"},
			Workspace:   workspace2,
		},
	}

	// Cache resources for both workspaces
	cache.SetResources(workspace1, resources1, 0)
	cache.SetResources(workspace2, resources2, 0)

	// Verify isolation
	result1, found1 := cache.GetResources(workspace1)
	result2, found2 := cache.GetResources(workspace2)

	if !found1 || !found2 {
		t.Error("Expected cache hits for both workspaces")
	}

	if len(result1) != 1 || result1[0].APIResource.Name != "deployments" {
		t.Error("Workspace1 should have deployments")
	}

	if len(result2) != 1 || result2[0].APIResource.Name != "pods" {
		t.Error("Workspace2 should have pods")
	}

	// Invalidate workspace1
	cache.InvalidateWorkspace(workspace1)

	// Workspace1 should be gone, workspace2 should remain
	_, found1 = cache.GetResources(workspace1)
	_, found2 = cache.GetResources(workspace2)

	if found1 {
		t.Error("Workspace1 should be invalidated")
	}
	if !found2 {
		t.Error("Workspace2 should still be cached")
	}
}

func TestDiscoveryCache_ConcurrentAccess(t *testing.T) {
	cache := NewDiscoveryCache(60).(*DiscoveryCache)
	defer cache.Stop()

	workspace := logicalcluster.New("concurrent-test")
	resources := []interfaces.ResourceInfo{
		{
			GroupVersionResource: schema.GroupVersionResource{
				Group: "apps", Version: "v1", Resource: "deployments",
			},
			APIResource: metav1.APIResource{Name: "deployments"},
			Workspace:   workspace,
		},
	}

	var wg sync.WaitGroup
	errors := make(chan error, 100)

	// Concurrent writes
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cache.SetResources(workspace, resources, 0)
		}()
	}

	// Concurrent reads
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = cache.GetResources(workspace)
		}()
	}

	// Concurrent invalidations
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cache.InvalidateWorkspace(workspace)
		}()
	}

	wg.Wait()
	close(errors)

	// Check for any errors
	for err := range errors {
		if err != nil {
			t.Errorf("Concurrent operation failed: %v", err)
		}
	}
}

func TestDiscoveryCache_Clear(t *testing.T) {
	cache := NewDiscoveryCache(60).(*DiscoveryCache)
	defer cache.Stop()

	workspace1 := logicalcluster.New("workspace-1")
	workspace2 := logicalcluster.New("workspace-2")

	resources := []interfaces.ResourceInfo{
		{
			GroupVersionResource: schema.GroupVersionResource{
				Group: "apps", Version: "v1", Resource: "deployments",
			},
			APIResource: metav1.APIResource{Name: "deployments"},
		},
	}

	// Cache for multiple workspaces
	cache.SetResources(workspace1, resources, 0)
	cache.SetResources(workspace2, resources, 0)

	// Verify both are cached
	_, found1 := cache.GetResources(workspace1)
	_, found2 := cache.GetResources(workspace2)
	if !found1 || !found2 {
		t.Error("Expected both workspaces to be cached")
	}

	// Clear all
	cache.Clear()

	// Verify both are gone
	_, found1 = cache.GetResources(workspace1)
	_, found2 = cache.GetResources(workspace2)
	if found1 || found2 {
		t.Error("Expected both workspaces to be cleared")
	}
}

func TestCacheStoreManager_Operations(t *testing.T) {
	store := NewInMemoryCacheStore()
	manager := NewCacheStoreManager(store, time.Minute)
	defer manager.Stop()

	workspace := logicalcluster.New("test-workspace")
	resources := []interfaces.ResourceInfo{
		{
			GroupVersionResource: schema.GroupVersionResource{
				Group: "apps", Version: "v1", Resource: "deployments",
			},
			APIResource: metav1.APIResource{Name: "deployments"},
			Workspace:   workspace,
		},
	}

	ctx := context.Background()

	// Test cache miss
	result, found := manager.GetResources(ctx, workspace)
	if found {
		t.Error("Expected cache miss")
	}
	if result != nil {
		t.Error("Expected nil result on cache miss")
	}

	// Test cache set
	err := manager.SetResources(ctx, workspace, resources, time.Minute)
	if err != nil {
		t.Errorf("Failed to set resources: %v", err)
	}

	// Test cache hit
	result, found = manager.GetResources(ctx, workspace)
	if !found {
		t.Error("Expected cache hit")
	}
	if len(result) != len(resources) {
		t.Errorf("Expected %d resources, got %d", len(resources), len(result))
	}
}

// Benchmark tests
func BenchmarkDiscoveryCache_Get(b *testing.B) {
	cache := NewDiscoveryCache(300).(*DiscoveryCache) // 5 minute TTL
	defer cache.Stop()

	workspace := logicalcluster.New("bench-workspace")
	resources := []interfaces.ResourceInfo{
		{
			GroupVersionResource: schema.GroupVersionResource{
				Group: "apps", Version: "v1", Resource: "deployments",
			},
			APIResource: metav1.APIResource{Name: "deployments"},
			Workspace:   workspace,
		},
	}

	// Pre-populate cache
	cache.SetResources(workspace, resources, 0)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = cache.GetResources(workspace)
		}
	})
}

func BenchmarkDiscoveryCache_Set(b *testing.B) {
	cache := NewDiscoveryCache(300).(*DiscoveryCache)
	defer cache.Stop()

	workspace := logicalcluster.New("bench-workspace")
	resources := []interfaces.ResourceInfo{
		{
			GroupVersionResource: schema.GroupVersionResource{
				Group: "apps", Version: "v1", Resource: "deployments",
			},
			APIResource: metav1.APIResource{Name: "deployments"},
			Workspace:   workspace,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.SetResources(workspace, resources, 0)
	}
}

func BenchmarkDiscoveryCache_ConcurrentGetSet(b *testing.B) {
	cache := NewDiscoveryCache(300).(*DiscoveryCache)
	defer cache.Stop()

	workspace := logicalcluster.New("bench-workspace")
	resources := []interfaces.ResourceInfo{
		{
			GroupVersionResource: schema.GroupVersionResource{
				Group: "apps", Version: "v1", Resource: "deployments",
			},
			APIResource: metav1.APIResource{Name: "deployments"},
			Workspace:   workspace,
		},
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if pb.Next() { // Alternate between get and set
				cache.SetResources(workspace, resources, 0)
			} else {
				_, _ = cache.GetResources(workspace)
			}
		}
	})
}