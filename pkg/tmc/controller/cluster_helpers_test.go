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

package controller

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestClusterHealthHelper_SetAndGet(t *testing.T) {
	helper := NewClusterHealthHelper()

	// Test setting and getting cluster health
	status := &ClusterHealthStatus{
		Name:      "test-cluster",
		LastCheck: time.Now(),
		Healthy:   true,
		Error:     "",
		NodeCount: 5,
		Version:   "v1.24.0",
	}

	err := helper.SetClusterHealth("test-cluster", status)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, exists := helper.GetClusterHealth("test-cluster")
	if !exists {
		t.Fatal("expected cluster health to exist")
	}

	if got.Name != status.Name {
		t.Errorf("expected name %s, got %s", status.Name, got.Name)
	}
	if got.Healthy != status.Healthy {
		t.Errorf("expected healthy %v, got %v", status.Healthy, got.Healthy)
	}
	if got.NodeCount != status.NodeCount {
		t.Errorf("expected node count %d, got %d", status.NodeCount, got.NodeCount)
	}
}

func TestClusterHealthHelper_SetNil(t *testing.T) {
	helper := NewClusterHealthHelper()

	// Should return error when setting nil
	err := helper.SetClusterHealth("test-cluster", nil)
	if err == nil {
		t.Error("expected error when setting nil status")
	}

	_, exists := helper.GetClusterHealth("test-cluster")
	if exists {
		t.Error("expected cluster health to not exist after setting nil")
	}
}

func TestClusterHealthHelper_SetEmptyClusterName(t *testing.T) {
	helper := NewClusterHealthHelper()

	// Should return error when setting empty cluster name
	status := &ClusterHealthStatus{
		Name:      "test-cluster",
		LastCheck: time.Now(),
		Healthy:   true,
	}

	err := helper.SetClusterHealth("", status)
	if err == nil {
		t.Error("expected error when setting empty cluster name")
	}
}

func TestClusterHealthHelper_SetMismatchedNames(t *testing.T) {
	helper := NewClusterHealthHelper()

	// Should return error when names don't match
	status := &ClusterHealthStatus{
		Name:      "different-name",
		LastCheck: time.Now(),
		Healthy:   true,
	}

	err := helper.SetClusterHealth("test-cluster", status)
	if err == nil {
		t.Error("expected error when status name doesn't match cluster name")
	}
}

func TestClusterHealthHelper_GetNonExistent(t *testing.T) {
	helper := NewClusterHealthHelper()

	got, exists := helper.GetClusterHealth("non-existent")
	if exists {
		t.Error("expected non-existent cluster to not exist")
	}
	if got != nil {
		t.Error("expected nil for non-existent cluster")
	}
}

func TestClusterHealthHelper_GetAllClusterHealth(t *testing.T) {
	helper := NewClusterHealthHelper()

	// Add multiple clusters
	clusters := []string{"cluster-1", "cluster-2", "cluster-3"}
	for i, name := range clusters {
		status := &ClusterHealthStatus{
			Name:      name,
			LastCheck: time.Now(),
			Healthy:   i%2 == 0,
			NodeCount: i + 1,
			Version:   "v1.24.0",
		}
		err := helper.SetClusterHealth(name, status)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	all := helper.GetAllClusterHealth()
	if len(all) != len(clusters) {
		t.Errorf("expected %d clusters, got %d", len(clusters), len(all))
	}

	// Verify all clusters are present
	for _, name := range clusters {
		if _, exists := all[name]; !exists {
			t.Errorf("expected cluster %s to exist", name)
		}
	}
}

func TestClusterHealthHelper_IsHealthy(t *testing.T) {
	tests := map[string]struct {
		clusters map[string]bool
		expected bool
	}{
		"all healthy": {
			clusters: map[string]bool{
				"cluster-1": true,
				"cluster-2": true,
				"cluster-3": true,
			},
			expected: true,
		},
		"one unhealthy": {
			clusters: map[string]bool{
				"cluster-1": true,
				"cluster-2": false,
				"cluster-3": true,
			},
			expected: false,
		},
		"all unhealthy": {
			clusters: map[string]bool{
				"cluster-1": false,
				"cluster-2": false,
			},
			expected: false,
		},
		"no clusters": {
			clusters: map[string]bool{},
			expected: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			helper := NewClusterHealthHelper()

			for clusterName, healthy := range tc.clusters {
				status := &ClusterHealthStatus{
					Name:      clusterName,
					LastCheck: time.Now(),
					Healthy:   healthy,
				}
				err := helper.SetClusterHealth(clusterName, status)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}

			got := helper.IsHealthy()
			if got != tc.expected {
				t.Errorf("expected IsHealthy to return %v, got %v", tc.expected, got)
			}
		})
	}
}

func TestClusterHealthHelper_Counts(t *testing.T) {
	helper := NewClusterHealthHelper()

	// Add mixed healthy and unhealthy clusters
	statuses := []struct {
		name    string
		healthy bool
	}{
		{"cluster-1", true},
		{"cluster-2", false},
		{"cluster-3", true},
		{"cluster-4", false},
		{"cluster-5", true},
	}

	for _, s := range statuses {
		status := &ClusterHealthStatus{
			Name:      s.name,
			LastCheck: time.Now(),
			Healthy:   s.healthy,
		}
		err := helper.SetClusterHealth(s.name, status)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	// Test counts
	if got := helper.GetTotalClusterCount(); got != 5 {
		t.Errorf("expected total count 5, got %d", got)
	}

	if got := helper.GetHealthyClusterCount(); got != 3 {
		t.Errorf("expected healthy count 3, got %d", got)
	}

	if got := helper.GetUnhealthyClusterCount(); got != 2 {
		t.Errorf("expected unhealthy count 2, got %d", got)
	}
}

func TestClusterHealthHelper_RemoveCluster(t *testing.T) {
	helper := NewClusterHealthHelper()

	// Add cluster
	status := &ClusterHealthStatus{
		Name:      "test-cluster",
		LastCheck: time.Now(),
		Healthy:   true,
	}
	err := helper.SetClusterHealth("test-cluster", status)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify it exists
	_, exists := helper.GetClusterHealth("test-cluster")
	if !exists {
		t.Fatal("expected cluster to exist before removal")
	}

	// Remove cluster
	helper.RemoveCluster("test-cluster")

	// Verify it's gone
	_, exists = helper.GetClusterHealth("test-cluster")
	if exists {
		t.Error("expected cluster to not exist after removal")
	}
}

func TestClusterHealthHelper_ClearAll(t *testing.T) {
	helper := NewClusterHealthHelper()

	// Add multiple clusters
	for i := 0; i < 5; i++ {
		status := &ClusterHealthStatus{
			Name:      fmt.Sprintf("cluster-%d", i),
			LastCheck: time.Now(),
			Healthy:   true,
		}
		err := helper.SetClusterHealth(status.Name, status)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	// Verify they exist
	if got := helper.GetTotalClusterCount(); got != 5 {
		t.Fatalf("expected 5 clusters before clear, got %d", got)
	}

	// Clear all
	helper.ClearAll()

	// Verify all gone
	if got := helper.GetTotalClusterCount(); got != 0 {
		t.Errorf("expected 0 clusters after clear, got %d", got)
	}
}

func TestClusterHealthHelper_ConcurrentAccess(t *testing.T) {
	helper := NewClusterHealthHelper()

	// Test concurrent writes and reads
	var wg sync.WaitGroup
	numGoroutines := 100

	// Start writers
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			status := &ClusterHealthStatus{
				Name:      fmt.Sprintf("cluster-%d", id),
				LastCheck: time.Now(),
				Healthy:   id%2 == 0,
				NodeCount: id,
			}
			err := helper.SetClusterHealth(status.Name, status)
			if err != nil {
				t.Errorf("unexpected error in goroutine %d: %v", id, err)
			}
		}(i)
	}

	// Start readers
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			helper.GetClusterHealth(fmt.Sprintf("cluster-%d", id))
			helper.GetAllClusterHealth()
			helper.IsHealthy()
			helper.GetHealthyClusterCount()
			helper.GetUnhealthyClusterCount()
			helper.GetTotalClusterCount()
		}(i)
	}

	// Start removers
	for i := 0; i < numGoroutines/2; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			helper.RemoveCluster(fmt.Sprintf("cluster-%d", id))
		}(i)
	}

	wg.Wait()

	// No assertion needed - test passes if no panic/race
}

func TestClusterHealthHelper_DataIsolation(t *testing.T) {
	helper := NewClusterHealthHelper()

	// Set initial status
	original := &ClusterHealthStatus{
		Name:      "test-cluster",
		LastCheck: time.Now(),
		Healthy:   true,
		Error:     "",
		NodeCount: 5,
		Version:   "v1.24.0",
	}
	err := helper.SetClusterHealth("test-cluster", original)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Modify original after setting
	original.Healthy = false
	original.Error = "modified"
	original.NodeCount = 10

	// Get status and verify it wasn't affected
	got, _ := helper.GetClusterHealth("test-cluster")
	if !got.Healthy {
		t.Error("expected status to remain healthy after external modification")
	}
	if got.Error != "" {
		t.Error("expected error to remain empty after external modification")
	}
	if got.NodeCount != 5 {
		t.Errorf("expected node count to remain 5, got %d", got.NodeCount)
	}

	// Modify returned status
	got.Healthy = false
	got.NodeCount = 20

	// Get again and verify internal state wasn't affected
	got2, _ := helper.GetClusterHealth("test-cluster")
	if !got2.Healthy {
		t.Error("expected internal status to remain healthy after modifying returned copy")
	}
	if got2.NodeCount != 5 {
		t.Errorf("expected internal node count to remain 5, got %d", got2.NodeCount)
	}
}

func TestClusterHealthHelper_WorkspaceAwareness(t *testing.T) {
	helper := NewClusterHealthHelper()

	// Test workspace-aware methods
	status1 := &ClusterHealthStatus{
		Name:      "cluster-1",
		LastCheck: time.Now(),
		Healthy:   true,
		NodeCount: 3,
		Version:   "v1.24.0",
	}

	status2 := &ClusterHealthStatus{
		Name:      "cluster-1", // Same cluster name in different workspace
		LastCheck: time.Now(),
		Healthy:   false,
		NodeCount: 1,
		Version:   "v1.23.0",
	}

	// Set cluster health in different workspaces
	helper.SetClusterHealthForWorkspace("root:org1:ws1", "cluster-1", status1)
	helper.SetClusterHealthForWorkspace("root:org2:ws2", "cluster-1", status2)

	// Get from first workspace
	got1, exists1 := helper.GetClusterHealthForWorkspace("root:org1:ws1", "cluster-1")
	if !exists1 {
		t.Fatal("expected cluster to exist in first workspace")
	}
	if !got1.Healthy {
		t.Error("expected cluster to be healthy in first workspace")
	}
	if got1.NodeCount != 3 {
		t.Errorf("expected node count 3 in first workspace, got %d", got1.NodeCount)
	}

	// Get from second workspace
	got2, exists2 := helper.GetClusterHealthForWorkspace("root:org2:ws2", "cluster-1")
	if !exists2 {
		t.Fatal("expected cluster to exist in second workspace")
	}
	if got2.Healthy {
		t.Error("expected cluster to be unhealthy in second workspace")
	}
	if got2.NodeCount != 1 {
		t.Errorf("expected node count 1 in second workspace, got %d", got2.NodeCount)
	}

	// Verify isolation - check non-existent workspace
	_, exists3 := helper.GetClusterHealthForWorkspace("root:other:ws", "cluster-1")
	if exists3 {
		t.Error("expected cluster to not exist in non-existent workspace")
	}
}