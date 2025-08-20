// Copyright The KCP Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"k8s.io/client-go/rest"
	"k8s.io/apiserver/pkg/server"
	"github.com/kcp-dev/logicalcluster/v3"
)

func TestNewClusterRegistrationController(t *testing.T) {
	tests := []struct {
		name           string
		clusterConfigs map[string]*rest.Config
		workspace      logicalcluster.Name
		wantErr        bool
	}{
		{
			name:           "no cluster configs should error",
			clusterConfigs: map[string]*rest.Config{},
			workspace:      logicalcluster.Name("root:test"),
			wantErr:        true,
		},
		{
			name: "valid config should succeed",
			clusterConfigs: map[string]*rest.Config{
				"test-cluster": {
					Host: "https://localhost:6443",
				},
			},
			workspace: logicalcluster.Name("root:test"),
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controller, err := NewClusterRegistrationController(
				nil, // KCP client not needed for this test
				tt.clusterConfigs,
				tt.workspace,
				30*time.Second,
				1,
			)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				if controller != nil {
					t.Error("Expected nil controller on error")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if controller == nil {
					t.Error("Expected non-nil controller")
				}
			}
		})
	}
}

func TestClusterRegistrationController_HealthChecking(t *testing.T) {
	// Create a mock kubernetes API server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/nodes":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"apiVersion": "v1",
				"kind": "NodeList",
				"items": [
					{
						"metadata": {"name": "node1"},
						"spec": {},
						"status": {"conditions": []}
					}
				]
			}`))
		case "/version":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"major": "1",
				"minor": "28",
				"gitVersion": "v1.28.0"
			}`))
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
	defer server.Close()

	clusterConfig := &rest.Config{
		Host: server.URL,
		ContentConfig: rest.ContentConfig{
			NegotiatedSerializer: server.Config.Serializer,
		},
	}

	controller, err := NewClusterRegistrationController(
		nil, // KCP client not needed for this test
		map[string]*rest.Config{
			"test-cluster": clusterConfig,
		},
		logicalcluster.Name("root:test"),
		time.Second, // Short resync period for testing
		1,
	)
	if err != nil {
		t.Fatalf("Failed to create controller: %v", err)
	}

	// Test initial state
	if controller.IsHealthy() {
		t.Error("Controller should not be healthy initially")
	}

	// Test health status retrieval
	health, exists := controller.GetClusterHealth("test-cluster")
	if !exists {
		t.Error("Expected cluster health to exist")
	}
	if health.Healthy {
		t.Error("Expected cluster to be unhealthy initially")
	}

	// Test sync cluster directly
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = controller.syncCluster(ctx, "test-cluster")
	if err != nil {
		t.Errorf("Unexpected error during sync: %v", err)
	}

	// Check health after sync
	health, exists = controller.GetClusterHealth("test-cluster")
	if !exists {
		t.Error("Expected cluster health to exist after sync")
	}
	if !health.Healthy {
		t.Errorf("Expected cluster to be healthy after sync, got error: %s", health.Error)
	}
	if health.NodeCount != 1 {
		t.Errorf("Expected 1 node, got %d", health.NodeCount)
	}

	// Test all cluster health
	allHealth := controller.GetAllClusterHealth()
	if len(allHealth) != 1 {
		t.Errorf("Expected 1 cluster in health map, got %d", len(allHealth))
	}

	// Test overall controller health
	if !controller.IsHealthy() {
		t.Error("Controller should be healthy after successful sync")
	}
}

func TestClusterHealthStatus_Copy(t *testing.T) {
	original := &ClusterHealthStatus{
		Name:      "test",
		LastCheck: time.Now(),
		Healthy:   true,
		Error:     "",
		NodeCount: 3,
		Version:   "v1.28.0",
	}

	controller := &ClusterRegistrationController{
		clusterHealth: map[string]*ClusterHealthStatus{
			"test": original,
		},
	}

	// Get health should return a copy
	health, exists := controller.GetClusterHealth("test")
	if !exists {
		t.Error("Expected health to exist")
	}

	// Modify the copy
	health.Healthy = false
	health.Error = "modified"

	// Original should be unchanged
	if !original.Healthy {
		t.Error("Original health was modified")
	}
	if original.Error != "" {
		t.Error("Original error was modified")
	}
}