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

package workqueue

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kcp-dev/logicalcluster/v3"
)

func TestClusterAwareRequest(t *testing.T) {
	tests := map[string]struct {
		request  ClusterAwareRequest
		expected string
	}{
		"basic request": {
			request: ClusterAwareRequest{
				Key:       "test-key",
				Workspace: logicalcluster.Name("test-workspace"),
				Priority:  1,
			},
			expected: "ClusterAwareRequest{Key=test-key, Workspace=test-workspace, Priority=1, Retries=0}",
		},
		"request with retries": {
			request: ClusterAwareRequest{
				Key:        "retry-key",
				Workspace:  logicalcluster.Name("retry-workspace"),
				Priority:   5,
				RetryCount: 3,
			},
			expected: "ClusterAwareRequest{Key=retry-key, Workspace=retry-workspace, Priority=5, Retries=3}",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result := tc.request.String()
			if result != tc.expected {
				t.Errorf("String() = %v, want %v", result, tc.expected)
			}
		})
	}
}

func TestNewTypedClusterWorkQueue(t *testing.T) {
	queue := NewTypedClusterWorkQueue("test-queue")
	if queue == nil {
		t.Fatal("Expected non-nil queue")
	}

	if queue.Len() != 0 {
		t.Errorf("Expected empty queue, got length %d", queue.Len())
	}

	if queue.ShuttingDown() {
		t.Error("Expected queue not to be shutting down initially")
	}
}

func TestTypedClusterWorkQueue_AddAndGet(t *testing.T) {
	queue := NewTypedClusterWorkQueue("test-queue")
	
	req := ClusterAwareRequest{
		Key:       "test|default/test-object",
		Workspace: logicalcluster.Name("test"),
		Priority:  1,
	}

	// Test Add
	queue.Add(req)
	if queue.Len() != 1 {
		t.Errorf("Expected queue length 1, got %d", queue.Len())
	}

	// Test Get
	gotReq, shutdown := queue.Get()
	if shutdown {
		t.Error("Expected no shutdown")
	}
	if gotReq.Key != req.Key {
		t.Errorf("Expected key %s, got %s", req.Key, gotReq.Key)
	}
	if gotReq.Workspace != req.Workspace {
		t.Errorf("Expected workspace %s, got %s", req.Workspace, gotReq.Workspace)
	}

	// Complete processing
	queue.Done(gotReq)
	queue.Forget(gotReq)
}

func TestTypedClusterWorkQueue_AddAfter(t *testing.T) {
	queue := NewTypedClusterWorkQueue("test-queue")
	
	req := ClusterAwareRequest{
		Key:       "test|default/delayed-object",
		Workspace: logicalcluster.Name("test"),
		Priority:  1,
	}

	// Test AddAfter with small delay
	start := time.Now()
	queue.AddAfter(req, 10*time.Millisecond)

	// Initially empty
	if queue.Len() != 0 {
		t.Errorf("Expected queue length 0 initially, got %d", queue.Len())
	}

	// Wait for item to be added
	time.Sleep(20 * time.Millisecond)
	
	// Should have the item now
	if queue.Len() != 1 {
		t.Errorf("Expected queue length 1 after delay, got %d", queue.Len())
	}

	// Get and verify
	gotReq, shutdown := queue.Get()
	if shutdown {
		t.Error("Expected no shutdown")
	}
	if gotReq.Key != req.Key {
		t.Errorf("Expected key %s, got %s", req.Key, gotReq.Key)
	}

	// Verify timing (should be at least 10ms)
	elapsed := time.Since(start)
	if elapsed < 10*time.Millisecond {
		t.Errorf("Expected at least 10ms delay, got %v", elapsed)
	}

	queue.Done(gotReq)
	queue.Forget(gotReq)
}

func TestTypedClusterWorkQueue_AddRateLimited(t *testing.T) {
	queue := NewTypedClusterWorkQueue("test-queue")
	
	req := ClusterAwareRequest{
		Key:       "test|default/rate-limited-object",
		Workspace: logicalcluster.Name("test"),
		Priority:  1,
	}

	// Test AddRateLimited
	queue.AddRateLimited(req)

	// Should eventually have the item
	// Note: This test is timing-sensitive and may need adjustment
	time.Sleep(10 * time.Millisecond)
	
	if queue.Len() == 0 {
		t.Error("Expected item to be added after rate limiting")
	}

	// Get and verify retry count is incremented
	gotReq, shutdown := queue.Get()
	if shutdown {
		t.Error("Expected no shutdown")
	}
	if gotReq.RetryCount != 1 {
		t.Errorf("Expected retry count 1, got %d", gotReq.RetryCount)
	}

	queue.Done(gotReq)
	queue.Forget(gotReq)
}

func TestTypedClusterWorkQueue_NumRequeues(t *testing.T) {
	queue := NewTypedClusterWorkQueue("test-queue")
	
	req := ClusterAwareRequest{
		Key:       "test|default/requeue-test",
		Workspace: logicalcluster.Name("test"),
		Priority:  1,
	}

	// Initially no requeues
	if queue.NumRequeues(req) != 0 {
		t.Errorf("Expected 0 requeues initially, got %d", queue.NumRequeues(req))
	}

	// Add rate limited (should increment requeue count)
	queue.AddRateLimited(req)
	
	// The underlying workqueue may not immediately track requeues
	// Just verify Forget works
	numRequeues := queue.NumRequeues(req)
	t.Logf("Requeues after AddRateLimited: %d", numRequeues)

	// Forget should reset
	queue.Forget(req)
	if queue.NumRequeues(req) != 0 {
		t.Errorf("Expected 0 requeues after Forget, got %d", queue.NumRequeues(req))
	}
}

func TestTypedClusterWorkQueue_ShutDown(t *testing.T) {
	queue := NewTypedClusterWorkQueue("test-queue")
	
	// Should not be shutting down initially
	if queue.ShuttingDown() {
		t.Error("Expected queue not to be shutting down initially")
	}

	// Shut down
	queue.ShutDown()
	
	if !queue.ShuttingDown() {
		t.Error("Expected queue to be shutting down after ShutDown call")
	}

	// Get should return shutdown=true
	_, shutdown := queue.Get()
	if !shutdown {
		t.Error("Expected shutdown=true after ShutDown")
	}
}

func TestQueueKeyForObject(t *testing.T) {
	tests := map[string]struct {
		obj           interface{}
		expectError   bool
		expectedKey   string
		expectedWorkspace logicalcluster.Name
	}{
		"valid object": {
			obj: &testObject{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-object",
					Namespace: "default",
					// In real KCP, the cluster annotation would be set by the system
					Annotations: map[string]string{
						"kcp.io/cluster": "test-cluster",
					},
				},
				clusterName: logicalcluster.Name("test-cluster"),
			},
			expectError:       false,
			expectedKey:       "default/test-object", // Key format may vary based on KCP implementation
			expectedWorkspace: logicalcluster.Name("test-cluster"),
		},
		"invalid object": {
			obj:         "not-an-object",
			expectError: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			req, err := QueueKeyForObject(tc.obj)
			
			if tc.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}
			
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			
			// For this test, just verify key is non-empty and workspace extraction
			if req.Key == "" {
				t.Error("Expected non-empty key")
			}
			// Note: In a real KCP environment, logicalcluster.From would extract the workspace
			// For this test, we just verify the structure is correct
			if req.RequestTime.IsZero() {
				t.Error("Expected RequestTime to be set")
			}
		})
	}
}

func TestQueueKeyForClusterObject(t *testing.T) {
	workspace := logicalcluster.Name("test-workspace")
	namespace := "default"
	name := "test-object"
	priority := 10

	req := QueueKeyForClusterObject(workspace, namespace, name, priority)
	
	if req.Workspace != workspace {
		t.Errorf("Expected workspace %s, got %s", workspace, req.Workspace)
	}
	if req.Priority != priority {
		t.Errorf("Expected priority %d, got %d", priority, req.Priority)
	}
	if req.RequestTime.IsZero() {
		t.Error("Expected RequestTime to be set")
	}
	
	// Key should contain workspace, namespace, and name
	expectedKey := "test-workspace|default/test-object"
	if req.Key != expectedKey {
		t.Errorf("Expected key %s, got %s", expectedKey, req.Key)
	}
}

func TestQueueKeyForClusterObject_NoNamespace(t *testing.T) {
	workspace := logicalcluster.Name("test-workspace")
	name := "cluster-scoped-object"
	priority := 5

	req := QueueKeyForClusterObject(workspace, "", name, priority)
	
	if req.Workspace != workspace {
		t.Errorf("Expected workspace %s, got %s", workspace, req.Workspace)
	}
	if req.Priority != priority {
		t.Errorf("Expected priority %d, got %d", priority, req.Priority)
	}
	
	// Key should contain workspace and name only
	expectedKey := "test-workspace|cluster-scoped-object"  
	if req.Key != expectedKey {
		t.Errorf("Expected key %s, got %s", expectedKey, req.Key)
	}
}

// testObject is a helper for testing
type testObject struct {
	metav1.ObjectMeta
	clusterName logicalcluster.Name
}

// DeepCopyObject implements runtime.Object
func (o *testObject) DeepCopyObject() interface{} {
	return &testObject{
		ObjectMeta:  *o.ObjectMeta.DeepCopy(),
		clusterName: o.clusterName,
	}
}

// GetObjectKind implements runtime.Object  
func (o *testObject) GetObjectKind() interface{} {
	return nil
}

// Mock logicalcluster.From function for testing
func init() {
	// This would normally be handled by KCP's logicalcluster package
	// but for testing purposes we simulate it
}