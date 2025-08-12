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

package cluster

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	fakediscovery "k8s.io/client-go/discovery/fake"

	"github.com/kcp-dev/logicalcluster/v3"
	kcpclientsetfake "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster/fake"
)

func TestNewController(t *testing.T) {
	tests := []struct {
		name           string
		clusterConfigs map[string]*rest.Config
		workspace      string
		opts           *ControllerOptions
		wantError      bool
	}{
		{
			name:           "empty cluster configs should fail",
			clusterConfigs: map[string]*rest.Config{},
			workspace:      "root:test",
			opts:           DefaultControllerOptions(),
			wantError:      true,
		},
		{
			name: "valid single cluster should succeed",
			clusterConfigs: map[string]*rest.Config{
				"test-cluster": {},
			},
			workspace: "root:test",
			opts:      DefaultControllerOptions(),
			wantError: false,
		},
		{
			name: "multiple clusters should succeed",
			clusterConfigs: map[string]*rest.Config{
				"cluster-1": {},
				"cluster-2": {},
				"cluster-3": {},
			},
			workspace: "root:production",
			opts:      DefaultControllerOptions(),
			wantError: false,
		},
		{
			name: "nil options should use defaults",
			clusterConfigs: map[string]*rest.Config{
				"test-cluster": {},
			},
			workspace: "root:test",
			opts:      nil, // Should use defaults
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kcpClient := kcpclientsetfake.NewSimpleClientset()
			workspace := logicalcluster.Name(tt.workspace)

			controller, err := NewController(kcpClient, tt.clusterConfigs, workspace, tt.opts)

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

			// Check that default options are applied when nil
			if tt.opts == nil {
				if controller.resyncPeriod != DefaultControllerOptions().ResyncPeriod {
					t.Errorf("expected default resync period %v, got %v", DefaultControllerOptions().ResyncPeriod, controller.resyncPeriod)
				}
				if controller.workerCount != DefaultControllerOptions().WorkerCount {
					t.Errorf("expected default worker count %d, got %d", DefaultControllerOptions().WorkerCount, controller.workerCount)
				}
			}
		})
	}
}

func TestController_PerformComprehensiveHealthCheck(t *testing.T) {
	tests := []struct {
		name        string
		setupClient func() *fake.Clientset
		wantHealthy bool
		wantError   bool
		expectNodes int
	}{
		{
			name: "healthy cluster with ready nodes",
			setupClient: func() *fake.Clientset {
				client := fake.NewSimpleClientset()

				// Add ready nodes
				for i := 0; i < 3; i++ {
					node := &corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name: fmt.Sprintf("test-node-%d", i+1),
						},
						Status: corev1.NodeStatus{
							Conditions: []corev1.NodeCondition{
								{
									Type:   corev1.NodeReady,
									Status: corev1.ConditionTrue,
								},
							},
							Capacity: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("2"),
								corev1.ResourceMemory: resource.MustParse("8Gi"),
							},
						},
					}
					client.CoreV1().Nodes().Create(context.TODO(), node, metav1.CreateOptions{})
				}

				// Set up fake discovery
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
			expectNodes: 3,
		},
		{
			name: "healthy cluster with no nodes",
			setupClient: func() *fake.Clientset {
				client := fake.NewSimpleClientset()

				// Set up fake discovery
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
			expectNodes: 0,
		},
		{
			name: "cluster with unready nodes",
			setupClient: func() *fake.Clientset {
				client := fake.NewSimpleClientset()

				// Add unready node
				node := &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "unready-node",
					},
					Status: corev1.NodeStatus{
						Conditions: []corev1.NodeCondition{
							{
								Type:   corev1.NodeReady,
								Status: corev1.ConditionFalse,
							},
						},
					},
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

				return client
			},
			wantHealthy: false,
			wantError:   true,
			expectNodes: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kcpClient := kcpclientsetfake.NewSimpleClientset()
			workspace := logicalcluster.Name("root:test")

			controller, err := NewController(
				kcpClient,
				map[string]*rest.Config{"test-cluster": {}},
				workspace,
				DefaultControllerOptions(),
			)
			if err != nil {
				t.Fatalf("failed to create controller: %v", err)
			}

			// Replace with test client
			client := tt.setupClient()
			controller.clusterClients["test-cluster"] = client

			ctx := context.Background()
			healthStatus, err := controller.performComprehensiveHealthCheck(ctx, "test-cluster", client)

			if tt.wantError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if !tt.wantError {
				if healthStatus == nil {
					t.Fatal("health status should not be nil")
				}

				if healthStatus.Healthy != tt.wantHealthy {
					t.Errorf("expected healthy=%v, got %v", tt.wantHealthy, healthStatus.Healthy)
				}

				if healthStatus.NodeCount != tt.expectNodes {
					t.Errorf("expected %d nodes, got %d", tt.expectNodes, healthStatus.NodeCount)
				}

				if healthStatus.LastCheck.IsZero() {
					t.Error("expected LastCheck to be set")
				}

				if len(healthStatus.Conditions) == 0 {
					t.Error("expected conditions to be set")
				}
			}
		})
	}
}

func TestController_GetClusterHealth(t *testing.T) {
	kcpClient := kcpclientsetfake.NewSimpleClientset()
	workspace := logicalcluster.Name("root:test")

	controller, err := NewController(
		kcpClient,
		map[string]*rest.Config{
			"cluster-1": {},
			"cluster-2": {},
		},
		workspace,
		DefaultControllerOptions(),
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


func TestController_IsHealthy(t *testing.T) {
	kcpClient := kcpclientsetfake.NewSimpleClientset()
	workspace := logicalcluster.Name("root:test")

	controller, err := NewController(
		kcpClient,
		map[string]*rest.Config{
			"cluster-1": {},
			"cluster-2": {},
		},
		workspace,
		DefaultControllerOptions(),
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

	// Mark all clusters as healthy
	controller.clusterHealth["cluster-2"].Healthy = true
	if !controller.IsHealthy() {
		t.Error("expected controller to be healthy with all clusters healthy")
	}

	// Test healthy cluster count
	if count := controller.GetHealthyClusterCount(); count != 2 {
		t.Errorf("expected 2 healthy clusters, got %d", count)
	}
}