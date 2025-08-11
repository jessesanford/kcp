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
	"fmt"
	"testing"
	"time"

	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"

	kcpclientsetfake "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster/fake"
	kcpinformers "github.com/kcp-dev/kcp/sdk/client/informers/externalversions"
	"github.com/kcp-dev/logicalcluster/v3"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"
	fakediscovery "k8s.io/client-go/discovery/fake"
)

func TestNewAdvancedClusterController(t *testing.T) {
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
				"cluster-3": {},
			},
			workspace: "root:production",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kcpClient := kcpclientsetfake.NewSimpleClientset()
			informerFactory := kcpinformers.NewSharedInformerFactory(kcpClient, 0)
			workspace := logicalcluster.Name(tt.workspace)

			controller, err := NewAdvancedClusterController(
				kcpClient,
				informerFactory,
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

			// Check advanced components are initialized
			if controller.metricsCollector == nil {
				t.Error("metrics collector should be initialized")
			}
			if controller.capabilityDetector == nil {
				t.Error("capability detector should be initialized")
			}
			if controller.statusUpdater == nil {
				t.Error("status updater should be initialized")
			}
		})
	}
}

func TestAdvancedClusterController_PerformComprehensiveHealthCheck(t *testing.T) {
	tests := []struct {
		name          string
		setupClient   func() *fake.Clientset
		wantHealthy   bool
		wantError     bool
		wantNodeCount int
	}{
		{
			name: "healthy cluster with ready nodes",
			setupClient: func() *fake.Clientset {
				client := fake.NewSimpleClientset()

				// Add ready nodes
				readyNode := &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{Name: "node-1"},
					Status: corev1.NodeStatus{
						Conditions: []corev1.NodeCondition{
							{
								Type:   corev1.NodeReady,
								Status: corev1.ConditionTrue,
							},
						},
					},
				}
				notReadyNode := &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{Name: "node-2"},
					Status: corev1.NodeStatus{
						Conditions: []corev1.NodeCondition{
							{
								Type:   corev1.NodeReady,
								Status: corev1.ConditionFalse,
							},
						},
					},
				}
				client.CoreV1().Nodes().Create(context.TODO(), readyNode, metav1.CreateOptions{})
				client.CoreV1().Nodes().Create(context.TODO(), notReadyNode, metav1.CreateOptions{})

				// Add some pods and namespaces for metrics
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "default",
					},
				}
				client.CoreV1().Pods("default").Create(context.TODO(), pod, metav1.CreateOptions{})

				namespace := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{Name: "test-ns"},
				}
				client.CoreV1().Namespaces().Create(context.TODO(), namespace, metav1.CreateOptions{})

				// Set up fake discovery client
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
			wantHealthy:   true,
			wantError:     false,
			wantNodeCount: 2,
		},
		{
			name: "unhealthy cluster with no ready nodes",
			setupClient: func() *fake.Clientset {
				client := fake.NewSimpleClientset()

				// Add only not-ready nodes
				notReadyNode := &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{Name: "node-1"},
					Status: corev1.NodeStatus{
						Conditions: []corev1.NodeCondition{
							{
								Type:   corev1.NodeReady,
								Status: corev1.ConditionFalse,
							},
						},
					},
				}
				client.CoreV1().Nodes().Create(context.TODO(), notReadyNode, metav1.CreateOptions{})

				// Set up fake discovery client
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
			wantHealthy:   false,
			wantError:     false,
			wantNodeCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kcpClient := kcpclientsetfake.NewSimpleClientset()
			informerFactory := kcpinformers.NewSharedInformerFactory(kcpClient, 0)
			workspace := logicalcluster.Name("root:test")

			controller, err := NewAdvancedClusterController(
				kcpClient,
				informerFactory,
				map[string]*rest.Config{"test-cluster": {}},
				workspace,
				30*time.Second,
				1,
			)
			if err != nil {
				t.Fatalf("failed to create controller: %v", err)
			}

			// Replace with fake client
			client := tt.setupClient()
			controller.clusterClients["test-cluster"] = client

			ctx := context.Background()
			health, err := controller.performComprehensiveHealthCheck(ctx, "test-cluster", client)

			if tt.wantError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if health == nil {
				t.Fatal("expected health status")
			}
			if health.Healthy != tt.wantHealthy {
				t.Errorf("expected healthy=%v, got %v", tt.wantHealthy, health.Healthy)
			}
			if health.NodeCount != tt.wantNodeCount {
				t.Errorf("expected node count=%d, got %d", tt.wantNodeCount, health.NodeCount)
			}
			if len(health.Conditions) == 0 {
				t.Error("expected conditions to be set")
			}
			if health.LastCheck.IsZero() {
				t.Error("expected LastCheck to be set")
			}
		})
	}
}

func TestAdvancedClusterController_CollectClusterMetrics(t *testing.T) {
	kcpClient := kcpclientsetfake.NewSimpleClientset()
	informerFactory := kcpinformers.NewSharedInformerFactory(kcpClient, 0)
	workspace := logicalcluster.Name("root:test")

	controller, err := NewAdvancedClusterController(
		kcpClient,
		informerFactory,
		map[string]*rest.Config{"test-cluster": {}},
		workspace,
		30*time.Second,
		1,
	)
	if err != nil {
		t.Fatalf("failed to create controller: %v", err)
	}

	// Set up fake client with resources
	client := fake.NewSimpleClientset()

	// Add pods
	for i := 0; i < 3; i++ {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("pod-%d", i),
				Namespace: "default",
			},
		}
		client.CoreV1().Pods("default").Create(context.TODO(), pod, metav1.CreateOptions{})
	}

	// Add services
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "default",
		},
	}
	client.CoreV1().Services("default").Create(context.TODO(), service, metav1.CreateOptions{})

	// Add namespaces
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "test-ns"},
	}
	client.CoreV1().Namespaces().Create(context.TODO(), namespace, metav1.CreateOptions{})

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
	metrics, err := controller.collectClusterMetrics(ctx, "test-cluster", client)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if metrics == nil {
		t.Fatal("expected metrics")
	}
	if metrics.PodCount != 3 {
		t.Errorf("expected 3 pods, got %d", metrics.PodCount)
	}
	if metrics.ServiceCount != 1 {
		t.Errorf("expected 1 service, got %d", metrics.ServiceCount)
	}
	if metrics.NamespaceCount == 0 {
		t.Error("expected namespace count > 0")
	}
	if metrics.ResponseTime < 0 {
		t.Error("expected response time >= 0")
	}
}

func TestAdvancedClusterController_GetAdvancedClusterHealth(t *testing.T) {
	kcpClient := kcpclientsetfake.NewSimpleClientset()
	informerFactory := kcpinformers.NewSharedInformerFactory(kcpClient, 0)
	workspace := logicalcluster.Name("root:test")

	controller, err := NewAdvancedClusterController(
		kcpClient,
		informerFactory,
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
	health, exists := controller.GetAdvancedClusterHealth("cluster-1")
	if !exists {
		t.Error("expected cluster-1 to exist")
	}
	if health.Name != "cluster-1" {
		t.Errorf("expected cluster name cluster-1, got %s", health.Name)
	}
	if health.Healthy {
		t.Error("expected cluster to start as unhealthy")
	}
	if len(health.Conditions) == 0 {
		t.Error("expected initial conditions to be set")
	}

	// Test getting health for non-existent cluster
	health, exists = controller.GetAdvancedClusterHealth("non-existent")
	if exists {
		t.Error("expected non-existent cluster to not exist")
	}
	if health != nil {
		t.Error("expected health to be nil for non-existent cluster")
	}
}

func TestAdvancedClusterController_GetAllAdvancedClusterHealth(t *testing.T) {
	kcpClient := kcpclientsetfake.NewSimpleClientset()
	informerFactory := kcpinformers.NewSharedInformerFactory(kcpClient, 0)
	workspace := logicalcluster.Name("root:test")

	expectedClusters := map[string]*rest.Config{
		"cluster-1": {},
		"cluster-2": {},
		"cluster-3": {},
	}

	controller, err := NewAdvancedClusterController(
		kcpClient,
		informerFactory,
		expectedClusters,
		workspace,
		30*time.Second,
		1,
	)
	if err != nil {
		t.Fatalf("failed to create controller: %v", err)
	}

	allHealth := controller.GetAllAdvancedClusterHealth()

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
		if len(health.Conditions) == 0 {
			t.Errorf("expected conditions for cluster %s", clusterName)
		}
	}
}

func TestAdvancedClusterController_UpdateFailureStatus(t *testing.T) {
	kcpClient := kcpclientsetfake.NewSimpleClientset()
	informerFactory := kcpinformers.NewSharedInformerFactory(kcpClient, 0)
	workspace := logicalcluster.Name("root:test")

	controller, err := NewAdvancedClusterController(
		kcpClient,
		informerFactory,
		map[string]*rest.Config{"test-cluster": {}},
		workspace,
		30*time.Second,
		1,
	)
	if err != nil {
		t.Fatalf("failed to create controller: %v", err)
	}

	// Initially should have 0 consecutive failures
	health, exists := controller.GetAdvancedClusterHealth("test-cluster")
	if !exists {
		t.Fatal("expected test-cluster to exist")
	}
	if health.ConsecutiveFailures != 0 {
		t.Errorf("expected 0 consecutive failures initially, got %d", health.ConsecutiveFailures)
	}

	// Update failure status
	testErr := fmt.Errorf("test error")
	controller.updateFailureStatus("test-cluster", testErr)

	// Check that failure was recorded
	health, exists = controller.GetAdvancedClusterHealth("test-cluster")
	if !exists {
		t.Fatal("expected test-cluster to exist after failure")
	}
	if health.ConsecutiveFailures != 1 {
		t.Errorf("expected 1 consecutive failure, got %d", health.ConsecutiveFailures)
	}
	if health.Error != testErr.Error() {
		t.Errorf("expected error %s, got %s", testErr.Error(), health.Error)
	}
	if health.Healthy {
		t.Error("expected cluster to be unhealthy after failure")
	}

	// Update failure status again
	controller.updateFailureStatus("test-cluster", testErr)

	health, exists = controller.GetAdvancedClusterHealth("test-cluster")
	if !exists {
		t.Fatal("expected test-cluster to exist after second failure")
	}
	if health.ConsecutiveFailures != 2 {
		t.Errorf("expected 2 consecutive failures, got %d", health.ConsecutiveFailures)
	}
}
