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
	"context"
	"testing"
	"time"

	"k8s.io/client-go/rest"

	"github.com/kcp-dev/logicalcluster/v3"
)

func TestNewClusterRegistrationController(t *testing.T) {
	tests := map[string]struct {
		clusterConfigs map[string]*rest.Config
		workspace      logicalcluster.Name
		resyncPeriod   time.Duration
		workerCount    int
		wantError      bool
	}{
		"empty cluster configs": {
			clusterConfigs: map[string]*rest.Config{},
			workspace:      logicalcluster.New("test"),
			resyncPeriod:   30 * time.Second,
			workerCount:    1,
			wantError:      true,
		},
		"valid configuration": {
			clusterConfigs: map[string]*rest.Config{
				"test-cluster": {
					Host: "https://test.example.com",
				},
			},
			workspace:    logicalcluster.New("test"),
			resyncPeriod: 30 * time.Second,
			workerCount:  2,
			wantError:    false,
		},
		"multiple clusters": {
			clusterConfigs: map[string]*rest.Config{
				"cluster-1": {
					Host: "https://cluster1.example.com",
				},
				"cluster-2": {
					Host: "https://cluster2.example.com",
				},
			},
			workspace:    logicalcluster.New("multi-cluster"),
			resyncPeriod: 60 * time.Second,
			workerCount:  3,
			wantError:    false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			controller, err := NewClusterRegistrationController(
				nil, // kcpClusterClient not needed for this test
				tc.clusterConfigs,
				tc.workspace,
				tc.resyncPeriod,
				tc.workerCount,
			)

			if tc.wantError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				if controller != nil {
					t.Error("Expected nil controller when error occurs")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if controller == nil {
				t.Error("Expected controller but got nil")
				return
			}

			// Verify controller configuration
			if controller.workspace != tc.workspace {
				t.Errorf("Expected workspace %v, got %v", tc.workspace, controller.workspace)
			}

			if controller.resyncPeriod != tc.resyncPeriod {
				t.Errorf("Expected resyncPeriod %v, got %v", tc.resyncPeriod, controller.resyncPeriod)
			}

			if controller.workerCount != tc.workerCount {
				t.Errorf("Expected workerCount %d, got %d", tc.workerCount, controller.workerCount)
			}

			// Verify cluster health initialization
			if len(controller.clusterHealth) != len(tc.clusterConfigs) {
				t.Errorf("Expected %d cluster health entries, got %d", 
					len(tc.clusterConfigs), len(controller.clusterHealth))
			}

			// Verify all clusters start as unhealthy
			for name := range tc.clusterConfigs {
				health, exists := controller.clusterHealth[name]
				if !exists {
					t.Errorf("Expected health entry for cluster %s", name)
					continue
				}
				if health.Healthy {
					t.Errorf("Expected cluster %s to start as unhealthy", name)
				}
				if health.Name != name {
					t.Errorf("Expected health name %s, got %s", name, health.Name)
				}
			}
		})
	}
}

func TestClusterHealthStatus(t *testing.T) {
	clusterConfigs := map[string]*rest.Config{
		"test-cluster": {
			Host: "https://test.example.com",
		},
	}

	controller, err := NewClusterRegistrationController(
		nil,
		clusterConfigs,
		logicalcluster.New("test"),
		30*time.Second,
		1,
	)
	if err != nil {
		t.Fatalf("Failed to create controller: %v", err)
	}

	// Test GetClusterHealth for existing cluster
	health, exists := controller.GetClusterHealth("test-cluster")
	if !exists {
		t.Error("Expected cluster health to exist")
	}
	if health == nil {
		t.Error("Expected health object but got nil")
	}
	if health.Name != "test-cluster" {
		t.Errorf("Expected health name test-cluster, got %s", health.Name)
	}

	// Test GetClusterHealth for non-existent cluster
	health, exists = controller.GetClusterHealth("non-existent")
	if exists {
		t.Error("Expected cluster health to not exist")
	}
	if health != nil {
		t.Error("Expected nil health object for non-existent cluster")
	}

	// Test GetAllClusterHealth
	allHealth := controller.GetAllClusterHealth()
	if len(allHealth) != 1 {
		t.Errorf("Expected 1 cluster health entry, got %d", len(allHealth))
	}
	if _, exists := allHealth["test-cluster"]; !exists {
		t.Error("Expected test-cluster in all health")
	}

	// Test IsHealthy (should be false initially)
	if controller.IsHealthy() {
		t.Error("Expected controller to be unhealthy initially")
	}
}

func TestClusterHealthStatusCopying(t *testing.T) {
	clusterConfigs := map[string]*rest.Config{
		"test-cluster": {
			Host: "https://test.example.com",
		},
	}

	controller, err := NewClusterRegistrationController(
		nil,
		clusterConfigs,
		logicalcluster.New("test"),
		30*time.Second,
		1,
	)
	if err != nil {
		t.Fatalf("Failed to create controller: %v", err)
	}

	// Update internal health status
	controller.clusterHealth["test-cluster"].Healthy = true
	controller.clusterHealth["test-cluster"].Error = "test error"
	controller.clusterHealth["test-cluster"].NodeCount = 5
	controller.clusterHealth["test-cluster"].Version = "v1.28.0"

	// Get health and verify it's a copy
	health, exists := controller.GetClusterHealth("test-cluster")
	if !exists {
		t.Fatal("Expected cluster health to exist")
	}

	// Modify the returned health object
	health.Error = "modified error"
	health.NodeCount = 10

	// Verify original wasn't modified
	original := controller.clusterHealth["test-cluster"]
	if original.Error == "modified error" {
		t.Error("Original health object was modified")
	}
	if original.NodeCount == 10 {
		t.Error("Original health object was modified")
	}
}