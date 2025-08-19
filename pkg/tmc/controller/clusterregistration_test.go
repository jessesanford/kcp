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
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"

	"github.com/kcp-dev/logicalcluster/v3"

	kcpclientsetfake "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster/fake"
)

func TestNewClusterRegistrationController(t *testing.T) {
	tests := []struct {
		name           string
		clusterConfigs map[string]*rest.Config
		workspace      string
		wantError      bool
	}{
		{
			name:           "empty cluster configs should fail",
			clusterConfigs: map[string]*rest.Config{},
			workspace:      "root:test",
			wantError:      true,
		},
		{
			name: "valid cluster configs should succeed",
			clusterConfigs: map[string]*rest.Config{
				"test-cluster": {},
			},
			workspace: "root:test",
			wantError: false,
		},
		{
			name: "multiple cluster configs should succeed",
			clusterConfigs: map[string]*rest.Config{
				"cluster-1": {},
				"cluster-2": {},
			},
			workspace: "root:production",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kcpClient := kcpclientsetfake.NewSimpleClientset()
			workspace := logicalcluster.Name(tt.workspace)

			controller, err := NewClusterRegistrationController(
				kcpClient,
				tt.clusterConfigs,
				workspace,
				30*time.Second,
				2,
			)

			if tt.wantError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if controller == nil {
				t.Fatal("controller should not be nil")
			}

			if controller.workspace != workspace {
				t.Errorf("expected workspace %s, got %s", workspace, controller.workspace)
			}

			if len(controller.clusterClients) != len(tt.clusterConfigs) {
				t.Errorf("expected %d cluster clients, got %d", len(tt.clusterConfigs), len(controller.clusterClients))
			}

			if len(controller.clusterHealth) != len(tt.clusterConfigs) {
				t.Errorf("expected %d cluster health entries, got %d", len(tt.clusterConfigs), len(controller.clusterHealth))
			}
		})
	}
}

func TestClusterRegistrationController_PerformHealthCheck(t *testing.T) {
	tests := []struct {
		name        string
		setupClient func() *fake.Clientset
		wantHealthy bool
		wantError   bool
	}{
		{
			name: "healthy cluster with nodes",
			setupClient: func() *fake.Clientset {
				client := fake.NewSimpleClientset()

				// Add a node to make the cluster appear healthy
				node := &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-node-1",
					},
					Status: corev1.NodeStatus{
						Conditions: []corev1.NodeCondition{
							{
								Type:   corev1.NodeReady,
								Status: corev1.ConditionTrue,
							},
						},
					},
				}
				client.CoreV1().Nodes().Create(context.TODO(), node, metav1.CreateOptions{})

				// Set up fake discovery client with version info
				fakeDiscovery, ok := client.Discovery().(*fakediscovery.FakeDiscovery)
				if ok {
					fakeDiscovery.FakedServerVersion = &version.Info{
						Major:        "1",
						Minor:        "28",
						GitVersion:   "v1.28.0",
						GitCommit:    "abc123",
						GitTreeState: "clean",
						BuildDate:    "2023-01-01T00:00:00Z",
						GoVersion:    "go1.20.0",
						Compiler:     "gc",
						Platform:     "linux/amd64",
					}
				}

				return client
			},
			wantHealthy: true,
			wantError:   false,
		},
		{
			name: "healthy cluster with no nodes",
			setupClient: func() *fake.Clientset {
				client := fake.NewSimpleClientset()

				// Set up fake discovery client with version info
				fakeDiscovery, ok := client.Discovery().(*fakediscovery.FakeDiscovery)
				if ok {
					fakeDiscovery.FakedServerVersion = &version.Info{
						Major:      "1",
						Minor:      "28",
						GitVersion: "v1.28.0",
					}
				}

				return client
			},
			wantHealthy: true,
			wantError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kcpClient := kcpclientsetfake.NewSimpleClientset()
			workspace := logicalcluster.Name("root:test")

			controller, err := NewClusterRegistrationController(
				kcpClient,
				map[string]*rest.Config{"test-cluster": {}},
				workspace,
				30*time.Second,
				1,
			)
			if err != nil {
				t.Fatalf("failed to create controller: %v", err)
			}

			// Replace the fake client
			client := tt.setupClient()
			controller.clusterClients["test-cluster"] = client

			ctx := context.Background()
			healthy, err := controller.performHealthCheck(ctx, "test-cluster", client)

			if tt.wantError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if healthy != tt.wantHealthy {
				t.Errorf("expected healthy=%v, got %v", tt.wantHealthy, healthy)
			}
		})
	}
}

func TestClusterRegistrationController_GetClusterHealth(t *testing.T) {
	kcpClient := kcpclientsetfake.NewSimpleClientset()
	workspace := logicalcluster.Name("root:test")

	controller, err := NewClusterRegistrationController(
		kcpClient,
		map[string]*rest.Config{
			"cluster-1": {},
			"cluster-2": {},
		},
		workspace,
		30*time.Second,
		1,
	)
	if err != nil {
		t.Fatalf("failed to create controller: %v", err)
	}

	// Test getting health for existing cluster
	health, exists := controller.GetClusterHealth("cluster-1")
	if !exists {
		t.Error("expected cluster-1 to exist")
	}
	if health.Name != "cluster-1" {
		t.Errorf("expected cluster name cluster-1, got %s", health.Name)
	}
	if health.Healthy {
		t.Error("expected cluster to start as unhealthy")
	}

	// Test getting health for non-existent cluster
	health, exists = controller.GetClusterHealth("non-existent")
	if exists {
		t.Error("expected non-existent cluster to not exist")
	}
	if health != nil {
		t.Error("expected health to be nil for non-existent cluster")
	}
}

func TestClusterRegistrationController_GetAllClusterHealth(t *testing.T) {
	kcpClient := kcpclientsetfake.NewSimpleClientset()
	workspace := logicalcluster.Name("root:test")

	expectedClusters := map[string]*rest.Config{
		"cluster-1": {},
		"cluster-2": {},
		"cluster-3": {},
	}

	controller, err := NewClusterRegistrationController(
		kcpClient,
		expectedClusters,
		workspace,
		30*time.Second,
		1,
	)
	if err != nil {
		t.Fatalf("failed to create controller: %v", err)
	}

	allHealth := controller.GetAllClusterHealth()

	if len(allHealth) != len(expectedClusters) {
		t.Errorf("expected %d clusters, got %d", len(expectedClusters), len(allHealth))
	}

	for clusterName := range expectedClusters {
		health, exists := allHealth[clusterName]
		if !exists {
			t.Errorf("expected cluster %s to exist in health map", clusterName)
			continue
		}
		if health.Name != clusterName {
			t.Errorf("expected cluster name %s, got %s", clusterName, health.Name)
		}
	}
}

func TestClusterRegistrationController_IsHealthy(t *testing.T) {
	kcpClient := kcpclientsetfake.NewSimpleClientset()
	workspace := logicalcluster.Name("root:test")

	controller, err := NewClusterRegistrationController(
		kcpClient,
		map[string]*rest.Config{
			"cluster-1": {},
			"cluster-2": {},
		},
		workspace,
		30*time.Second,
		1,
	)
	if err != nil {
		t.Fatalf("failed to create controller: %v", err)
	}

	// Initially, all clusters should be unhealthy
	if controller.IsHealthy() {
		t.Error("expected controller to be unhealthy initially")
	}

	// Mark one cluster as healthy
	controller.clusterHealth["cluster-1"].Healthy = true
	if controller.IsHealthy() {
		t.Error("expected controller to be unhealthy with one unhealthy cluster")
	}

	// Mark both clusters as healthy
	controller.clusterHealth["cluster-2"].Healthy = true
	if !controller.IsHealthy() {
		t.Error("expected controller to be healthy with all clusters healthy")
	}
}

func TestClusterRegistrationController_SyncCluster(t *testing.T) {
	kcpClient := kcpclientsetfake.NewSimpleClientset()
	workspace := logicalcluster.Name("root:test")

	controller, err := NewClusterRegistrationController(
		kcpClient,
		map[string]*rest.Config{"test-cluster": {}},
		workspace,
		30*time.Second,
		1,
	)
	if err != nil {
		t.Fatalf("failed to create controller: %v", err)
	}

	// Set up fake client with nodes
	client := fake.NewSimpleClientset()
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "test-node"},
	}
	client.CoreV1().Nodes().Create(context.TODO(), node, metav1.CreateOptions{})

	// Set up fake discovery
	fakeDiscovery, ok := client.Discovery().(*fakediscovery.FakeDiscovery)
	if ok {
		fakeDiscovery.FakedServerVersion = &version.Info{
			Major:      "1",
			Minor:      "28",
			GitVersion: "v1.28.0",
		}
	}

	controller.clusterClients["test-cluster"] = client

	ctx := context.Background()
	err = controller.syncCluster(ctx, "test-cluster")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Check that health status was updated
	health, exists := controller.GetClusterHealth("test-cluster")
	if !exists {
		t.Fatal("expected health status to exist")
	}
	if !health.Healthy {
		t.Errorf("expected cluster to be healthy after sync, error: %s", health.Error)
	}
	if health.LastCheck.IsZero() {
		t.Error("expected LastCheck to be set")
	}

	// Test sync with non-existent cluster
	err = controller.syncCluster(ctx, "non-existent")
	if err == nil {
		t.Error("expected error for non-existent cluster")
	}
}
