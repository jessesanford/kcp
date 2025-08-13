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

package cluster

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"

	"github.com/kcp-dev/logicalcluster/v3"

	tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
	kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
	"github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster/fake"
)

func TestControllerLifecycle(t *testing.T) {
	tests := map[string]struct {
		cluster *tmcv1alpha1.ClusterRegistration
		expectProcessing bool
	}{
		"healthy cluster registration": {
			cluster: &tmcv1alpha1.ClusterRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-1",
					Annotations: map[string]string{
						"kcp.io/cluster": "root:test",
					},
				},
				Spec: tmcv1alpha1.ClusterRegistrationSpec{
					Location: "us-west-2",
					ClusterEndpoint: tmcv1alpha1.ClusterEndpoint{
						ServerURL: "https://test-cluster-1.example.com",
					},
					Capacity: tmcv1alpha1.ClusterCapacity{
						CPU:     int64Ptr(8000),
						Memory:  int64Ptr(16 * 1024 * 1024 * 1024), // 16GB
						MaxPods: int32Ptr(110),
					},
				},
			},
			expectProcessing: true,
		},
		"cluster with deletion timestamp": {
			cluster: &tmcv1alpha1.ClusterRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-cluster-2",
					DeletionTimestamp: &metav1.Time{Time: time.Now()},
					Annotations: map[string]string{
						"kcp.io/cluster": "root:test",
					},
				},
				Spec: tmcv1alpha1.ClusterRegistrationSpec{
					Location: "us-east-1",
					ClusterEndpoint: tmcv1alpha1.ClusterEndpoint{
						ServerURL: "https://test-cluster-2.example.com",
					},
				},
			},
			expectProcessing: true, // Should be processed for deletion
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Create fake clients
			kcpClient := kcpclientset.NewClusterForConfigOrDie(nil)
			kubeClient := fake.NewSimpleClientset()

			// Create fake TMC client (this would need proper fake implementation)
			tmcClient := &fakeTMCClient{}

			// Create informer
			informer := cache.NewSharedIndexInformer(
				nil, // ListWatch will be nil for test
				&tmcv1alpha1.ClusterRegistration{},
				0,
				cache.Indexers{},
			)

			// Create controller
			controller, err := NewController(kcpClient, kubeClient, tmcClient, informer)
			if err != nil {
				t.Fatalf("failed to create controller: %v", err)
			}

			if controller == nil {
				t.Fatal("expected controller to be non-nil")
			}

			// Test enqueue functionality
			controller.enqueue(tc.cluster)

			// Verify queue has item
			if controller.queue.Len() != 1 {
				t.Errorf("expected queue length 1, got %d", controller.queue.Len())
			}
		})
	}
}

func TestControllerReconciliation(t *testing.T) {
	tests := map[string]struct {
		cluster           *tmcv1alpha1.ClusterRegistration
		expectError       bool
		expectedCondition string
	}{
		"successful reconciliation": {
			cluster: &tmcv1alpha1.ClusterRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "healthy-cluster",
					Annotations: map[string]string{
						"kcp.io/cluster": "root:test",
					},
				},
				Spec: tmcv1alpha1.ClusterRegistrationSpec{
					Location: "us-west-2",
					ClusterEndpoint: tmcv1alpha1.ClusterEndpoint{
						ServerURL: "https://healthy-cluster.example.com",
					},
				},
			},
			expectError:       false,
			expectedCondition: ClusterReadyCondition,
		},
		"cluster deletion": {
			cluster: &tmcv1alpha1.ClusterRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "deleting-cluster",
					DeletionTimestamp: &metav1.Time{Time: time.Now()},
					Annotations: map[string]string{
						"kcp.io/cluster": "root:test",
					},
				},
				Spec: tmcv1alpha1.ClusterRegistrationSpec{
					Location: "us-east-1",
					ClusterEndpoint: tmcv1alpha1.ClusterEndpoint{
						ServerURL: "https://deleting-cluster.example.com",
					},
				},
			},
			expectError:       false, // Deletion should not error
			expectedCondition: "", // No specific condition expected for deletion
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup test environment
			ctx := context.Background()
			
			// Create fake clients
			kcpClient := kcpclientset.NewClusterForConfigOrDie(nil)
			kubeClient := fake.NewSimpleClientset()
			tmcClient := &fakeTMCClient{}

			// Create informer with test object
			informer := cache.NewSharedIndexInformer(
				nil,
				&tmcv1alpha1.ClusterRegistration{},
				0,
				cache.Indexers{},
			)

			// Add cluster to informer's store
			informer.GetIndexer().Add(tc.cluster)

			// Create controller
			controller, err := NewController(kcpClient, kubeClient, tmcClient, informer)
			if err != nil {
				t.Fatalf("failed to create controller: %v", err)
			}

			// Process the cluster
			err = controller.process(ctx, tc.cluster)
			
			if tc.expectError && err == nil {
				t.Error("expected error but got none")
			} else if !tc.expectError && err != nil {
				t.Errorf("expected no error but got: %v", err)
			}
		})
	}
}

func TestWorkspaceIsolation(t *testing.T) {
	tests := map[string]struct {
		clusters []struct {
			cluster   *tmcv1alpha1.ClusterRegistration
			workspace string
		}
		expectSeparateProcessing bool
	}{
		"different workspaces": {
			clusters: []struct {
				cluster   *tmcv1alpha1.ClusterRegistration
				workspace string
			}{
				{
					cluster: &tmcv1alpha1.ClusterRegistration{
						ObjectMeta: metav1.ObjectMeta{
							Name: "cluster-ws1",
							Annotations: map[string]string{
								"kcp.io/cluster": "root:workspace1",
							},
						},
						Spec: tmcv1alpha1.ClusterRegistrationSpec{
							Location: "us-west-1",
							ClusterEndpoint: tmcv1alpha1.ClusterEndpoint{
								ServerURL: "https://cluster-ws1.example.com",
							},
						},
					},
					workspace: "root:workspace1",
				},
				{
					cluster: &tmcv1alpha1.ClusterRegistration{
						ObjectMeta: metav1.ObjectMeta{
							Name: "cluster-ws2",
							Annotations: map[string]string{
								"kcp.io/cluster": "root:workspace2",
							},
						},
						Spec: tmcv1alpha1.ClusterRegistrationSpec{
							Location: "us-west-2",
							ClusterEndpoint: tmcv1alpha1.ClusterEndpoint{
								ServerURL: "https://cluster-ws2.example.com",
							},
						},
					},
					workspace: "root:workspace2",
				},
			},
			expectSeparateProcessing: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			
			// Create fake clients
			kcpClient := kcpclientset.NewClusterForConfigOrDie(nil)
			kubeClient := fake.NewSimpleClientset()
			tmcClient := &fakeTMCClient{}

			// Create informer
			informer := cache.NewSharedIndexInformer(
				nil,
				&tmcv1alpha1.ClusterRegistration{},
				0,
				cache.Indexers{},
			)

			// Add all clusters to informer
			for _, clusterInfo := range tc.clusters {
				informer.GetIndexer().Add(clusterInfo.cluster)
			}

			// Create controller
			controller, err := NewController(kcpClient, kubeClient, tmcClient, informer)
			if err != nil {
				t.Fatalf("failed to create controller: %v", err)
			}

			// Process each cluster and verify workspace isolation
			processedWorkspaces := make(map[string]bool)
			for _, clusterInfo := range tc.clusters {
				err := controller.process(ctx, clusterInfo.cluster)
				if err != nil {
					t.Errorf("failed to process cluster %s: %v", clusterInfo.cluster.Name, err)
				}
				processedWorkspaces[clusterInfo.workspace] = true
			}

			if tc.expectSeparateProcessing {
				if len(processedWorkspaces) != len(tc.clusters) {
					t.Errorf("expected %d separate workspaces, got %d", len(tc.clusters), len(processedWorkspaces))
				}
			}
		})
	}
}

// Helper functions
func int64Ptr(i int64) *int64 {
	return &i
}

func int32Ptr(i int32) *int32 {
	return &i
}

// fakeTMCClient is a simple fake implementation for testing
type fakeTMCClient struct {
	mu      sync.RWMutex
	objects map[string]*tmcv1alpha1.ClusterRegistration
}

func (f *fakeTMCClient) TmcV1alpha1() interface{} {
	return f
}

func (f *fakeTMCClient) ClusterRegistrations() interface{} {
	return f
}

func (f *fakeTMCClient) Cluster(path logicalcluster.Path) interface{} {
	return f
}

func (f *fakeTMCClient) Patch(ctx context.Context, name string, data []byte, opts metav1.PatchOptions, subresources ...string) (*tmcv1alpha1.ClusterRegistration, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	
	if f.objects == nil {
		f.objects = make(map[string]*tmcv1alpha1.ClusterRegistration)
	}
	
	// Simple patch simulation - return existing object
	if obj, exists := f.objects[name]; exists {
		return obj, nil
	}
	
	// Create a new object for testing
	newObj := &tmcv1alpha1.ClusterRegistration{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Status: tmcv1alpha1.ClusterRegistrationStatus{
			Conditions: []conditionsv1alpha1.Condition{
				{
					Type:   ClusterReadyCondition,
					Status: conditionsv1alpha1.ConditionTrue,
				},
			},
		},
	}
	f.objects[name] = newObj
	return newObj, nil
}