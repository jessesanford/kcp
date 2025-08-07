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
	"fmt"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	clientgotesting "k8s.io/client-go/testing"

	"github.com/kcp-dev/logicalcluster/v3"

	tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
	"github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/util/conditions"
	kcpfake "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster/fake"
	kcpinformers "github.com/kcp-dev/kcp/sdk/client/informers/externalversions"
)

func TestClusterRegistrationController(t *testing.T) {
	tests := map[string]struct {
		clusterRegistration     *tmcv1alpha1.ClusterRegistration
		workspace               string
		clusterHealthy          bool
		wantError               bool
		wantConditions          []conditionsv1alpha1.Condition
		expectStatusUpdate      bool
	}{
		"healthy cluster registration": {
			clusterRegistration: &tmcv1alpha1.ClusterRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
					Annotations: map[string]string{
						logicalcluster.AnnotationKey: "root:test",
					},
				},
				Spec: tmcv1alpha1.ClusterRegistrationSpec{
					Location: "us-west-2",
					ClusterEndpoint: tmcv1alpha1.ClusterEndpoint{
						ServerURL: "https://test-cluster.example.com",
					},
				},
			},
			workspace:          "root:test",
			clusterHealthy:     true,
			wantError:          false,
			expectStatusUpdate: true,
			wantConditions: []conditionsv1alpha1.Condition{
				{
					Type:   tmcv1alpha1.ClusterRegistrationReady,
					Status: metav1.ConditionTrue,
					Reason: "ClusterReady",
				},
			},
		},
		"unhealthy cluster registration": {
			clusterRegistration: &tmcv1alpha1.ClusterRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "unhealthy-cluster",
					Annotations: map[string]string{
						logicalcluster.AnnotationKey: "root:test",
					},
				},
				Spec: tmcv1alpha1.ClusterRegistrationSpec{
					Location: "us-east-1",
					ClusterEndpoint: tmcv1alpha1.ClusterEndpoint{
						ServerURL: "https://unhealthy-cluster.example.com",
					},
				},
			},
			workspace:          "root:test",
			clusterHealthy:     false,
			wantError:          false,
			expectStatusUpdate: true,
			wantConditions: []conditionsv1alpha1.Condition{
				{
					Type:   tmcv1alpha1.ClusterRegistrationReady,
					Status: metav1.ConditionFalse,
					Reason: "ClusterUnhealthy",
				},
			},
		},
		"cluster from different workspace": {
			clusterRegistration: &tmcv1alpha1.ClusterRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "other-workspace-cluster",
					Annotations: map[string]string{
						logicalcluster.AnnotationKey: "root:other",
					},
				},
				Spec: tmcv1alpha1.ClusterRegistrationSpec{
					Location: "us-central-1",
					ClusterEndpoint: tmcv1alpha1.ClusterEndpoint{
						ServerURL: "https://other-cluster.example.com",
					},
				},
			},
			workspace:          "root:test",
			clusterHealthy:     true,
			wantError:          false,
			expectStatusUpdate: false, // Should not update status for different workspace
		},
		"cluster without client": {
			clusterRegistration: &tmcv1alpha1.ClusterRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "no-client-cluster",
					Annotations: map[string]string{
						logicalcluster.AnnotationKey: "root:test",
					},
				},
				Spec: tmcv1alpha1.ClusterRegistrationSpec{
					Location: "eu-west-1",
					ClusterEndpoint: tmcv1alpha1.ClusterEndpoint{
						ServerURL: "https://no-client-cluster.example.com",
					},
				},
			},
			workspace:          "root:test",
			clusterHealthy:     true, // Not relevant since no client
			wantError:          false,
			expectStatusUpdate: true,
			wantConditions: []conditionsv1alpha1.Condition{
				{
					Type:   tmcv1alpha1.ClusterRegistrationReady,
					Status: metav1.ConditionFalse,
					Reason: "ClusterNotConfigured",
				},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			workspace := logicalcluster.Name(tc.workspace)

			// Create fake KCP client
			kcpClient := kcpfake.NewSimpleClientset(tc.clusterRegistration)

			// Create informer factory
			informerFactory := kcpinformers.NewSharedInformerFactoryWithOptions(
				kcpClient, time.Minute,
				kcpinformers.WithCluster(workspace),
			)

			// Setup cluster clients (only for test-cluster)
			clusterClients := make(map[string]*fake.Clientset)
			if tc.clusterRegistration.Name == "test-cluster" || tc.clusterRegistration.Name == "unhealthy-cluster" {
				fakeClient := fake.NewSimpleClientset()
				
				if !tc.clusterHealthy {
					// Make the client return an error for health checks
					fakeClient.PrependReactor("list", "nodes", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, fmt.Errorf("cluster is unreachable")
					})
				}
				
				clusterClients[tc.clusterRegistration.Name] = fakeClient
			}

			// Create controller
			controller, err := NewClusterRegistrationController(
				kcpClient,
				informerFactory.Tmc().V1alpha1().ClusterRegistrations(),
				convertToRestConfigMap(clusterClients),
				workspace,
				30*time.Second, // health check interval
				true,           // enable health checking
			)
			if err != nil {
				t.Fatalf("Failed to create controller: %v", err)
			}

			// Start informers
			informerFactory.Start(ctx.Done())

			// Add the cluster registration to the informer cache
			clusterRegInformer := informerFactory.Tmc().V1alpha1().ClusterRegistrations()
			if err := clusterRegInformer.Informer().GetStore().Add(tc.clusterRegistration); err != nil {
				t.Fatalf("Failed to add ClusterRegistration to store: %v", err)
			}

			// Clear any existing actions from setup
			kcpClient.ClearActions()

			// Test the sync
			err = controller.syncClusterRegistration(ctx, tc.clusterRegistration)

			// Check error expectation
			if tc.wantError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tc.wantError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			// Check if status update was called
			actions := kcpClient.Actions()
			var statusUpdated bool
			for _, action := range actions {
				if action.GetVerb() == "update" && action.GetSubresource() == "status" {
					statusUpdated = true
					break
				}
			}

			if tc.expectStatusUpdate && !statusUpdated {
				t.Errorf("Expected status update but none occurred")
			}
			if !tc.expectStatusUpdate && statusUpdated {
				t.Errorf("Expected no status update but one occurred")
			}

			// Check conditions if status was updated
			if statusUpdated && len(tc.wantConditions) > 0 {
				// Find the update action
				for _, action := range actions {
					if updateAction, ok := action.(clientgotesting.UpdateAction); ok && action.GetSubresource() == "status" {
						updatedCluster := updateAction.GetObject().(*tmcv1alpha1.ClusterRegistration)
						
						for _, wantCondition := range tc.wantConditions {
							condition := conditions.Get(updatedCluster, wantCondition.Type)
							if condition == nil {
								t.Errorf("Expected condition %s not found", wantCondition.Type)
								continue
							}
							if condition.Status != wantCondition.Status {
								t.Errorf("Expected condition %s status %s, got %s", 
									wantCondition.Type, wantCondition.Status, condition.Status)
							}
							if wantCondition.Reason != "" && condition.Reason != wantCondition.Reason {
								t.Errorf("Expected condition %s reason %s, got %s",
									wantCondition.Type, wantCondition.Reason, condition.Reason)
							}
						}
						break
					}
				}
			}
		})
	}
}

func TestClusterHealthCheck(t *testing.T) {
	tests := map[string]struct {
		setupClient     func() *fake.Clientset
		expectHealthy   bool
		expectError     bool
	}{
		"healthy cluster": {
			setupClient: func() *fake.Clientset {
				return fake.NewSimpleClientset()
			},
			expectHealthy: true,
			expectError:   false,
		},
		"unhealthy cluster - nodes list fails": {
			setupClient: func() *fake.Clientset {
				client := fake.NewSimpleClientset()
				client.PrependReactor("list", "nodes", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, fmt.Errorf("connection refused")
				})
				return client
			},
			expectHealthy: false,
			expectError:   true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			
			// Create minimal controller for testing health check method
			controller := &ClusterRegistrationController{}
			
			client := tc.setupClient()
			
			healthy, err := controller.testClusterHealth(ctx, client, "test-cluster")
			
			if tc.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tc.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
			if healthy != tc.expectHealthy {
				t.Errorf("Expected healthy=%v but got %v", tc.expectHealthy, healthy)
			}
		})
	}
}

func TestPeriodicHealthChecks(t *testing.T) {
	ctx := context.Background()
	workspace := logicalcluster.Name("root:test")

	// Create test cluster registration
	clusterReg := &tmcv1alpha1.ClusterRegistration{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-cluster",
			Annotations: map[string]string{
				logicalcluster.AnnotationKey: string(workspace),
			},
		},
		Spec: tmcv1alpha1.ClusterRegistrationSpec{
			Location: "us-west-2",
			ClusterEndpoint: tmcv1alpha1.ClusterEndpoint{
				ServerURL: "https://test-cluster.example.com",
			},
		},
	}

	// Create fake clients
	kcpClient := kcpfake.NewSimpleClientset(clusterReg)
	fakeClusterClient := fake.NewSimpleClientset()

	// Create informer factory
	informerFactory := kcpinformers.NewSharedInformerFactoryWithOptions(
		kcpClient, time.Minute,
		kcpinformers.WithCluster(workspace),
	)

	// Create controller with short health check interval for testing
	controller, err := NewClusterRegistrationController(
		kcpClient,
		informerFactory.Tmc().V1alpha1().ClusterRegistrations(),
		map[string]*rest.Config{
			"test-cluster": {}, // Config not used in this test
		},
		workspace,
		100*time.Millisecond, // Very short interval for testing
		true,
	)
	if err != nil {
		t.Fatalf("Failed to create controller: %v", err)
	}

	// Replace the fake client manually for testing
	controller.clusterClients["test-cluster"] = fakeClusterClient

	// Start informers
	informerFactory.Start(ctx.Done())

	// Add cluster registration to informer cache
	clusterRegInformer := informerFactory.Tmc().V1alpha1().ClusterRegistrations()
	if err := clusterRegInformer.Informer().GetStore().Add(clusterReg); err != nil {
		t.Fatalf("Failed to add ClusterRegistration to store: %v", err)
	}

	// Clear initial actions
	kcpClient.ClearActions()

	// Run health checker for a short time
	testCtx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()

	go controller.performPeriodicHealthChecks(testCtx)

	// Wait for some health checks to complete
	time.Sleep(200 * time.Millisecond)

	// Verify that health check was recorded
	if lastCheck, exists := controller.lastHealthCheckTime.Load("test-cluster"); !exists {
		t.Errorf("Expected health check time to be recorded")
	} else {
		checkTime := lastCheck.(time.Time)
		if time.Since(checkTime) > time.Second {
			t.Errorf("Health check time seems too old: %v", checkTime)
		}
	}
}

// Helper function to convert fake clients to rest.Config map (simplified for testing)
func convertToRestConfigMap(clients map[string]*fake.Clientset) map[string]*rest.Config {
	configs := make(map[string]*rest.Config)
	for name := range clients {
		configs[name] = &rest.Config{
			Host: fmt.Sprintf("https://%s.example.com", name),
		}
	}
	return configs
}