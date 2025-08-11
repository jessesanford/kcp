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
	"container/heap"
	"testing"
	"time"

	"github.com/kcp-dev/logicalcluster/v3"
)

func TestNewPriorityClusterWorkQueue(t *testing.T) {
	queue := NewPriorityClusterWorkQueue("test-priority-queue", nil)
	if queue == nil {
		t.Fatal("Expected non-nil priority queue")
	}

	if queue.Len() != 0 {
		t.Errorf("Expected empty queue, got length %d", queue.Len())
	}

	if queue.ShuttingDown() {
		t.Error("Expected queue not to be shutting down initially")
	}
}

func TestPriorityClusterWorkQueue_WorkspacePriority(t *testing.T) {
	queue := NewPriorityClusterWorkQueue("test-priority-queue", nil)
	
	workspace := logicalcluster.Name("test-workspace")
	priority := 50

	// Initially should have default priority (0)
	if queue.GetWorkspacePriority(workspace) != 0 {
		t.Errorf("Expected default priority 0, got %d", queue.GetWorkspacePriority(workspace))
	}

	// Set workspace priority
	queue.SetWorkspacePriority(workspace, priority)
	
	if queue.GetWorkspacePriority(workspace) != priority {
		t.Errorf("Expected priority %d, got %d", priority, queue.GetWorkspacePriority(workspace))
	}
}

func TestPriorityClusterWorkQueue_PriorityOrdering(t *testing.T) {
	queue := NewPriorityClusterWorkQueue("test-priority-queue", nil)
	
	// Add items with different priorities
	lowPriorityReq := ClusterAwareRequest{
		Key:       "test|low-priority",
		Workspace: logicalcluster.Name("test"),
		Priority:  1,
	}
	
	highPriorityReq := ClusterAwareRequest{
		Key:       "test|high-priority", 
		Workspace: logicalcluster.Name("test"),
		Priority:  10,
	}
	
	mediumPriorityReq := ClusterAwareRequest{
		Key:       "test|medium-priority",
		Workspace: logicalcluster.Name("test"),
		Priority:  5,
	}

	// Add in random order
	queue.Add(lowPriorityReq)
	queue.Add(highPriorityReq)
	queue.Add(mediumPriorityReq)

	if queue.Len() != 3 {
		t.Errorf("Expected queue length 3, got %d", queue.Len())
	}

	// Should get high priority first
	gotReq, shutdown := queue.Get()
	if shutdown {
		t.Error("Expected no shutdown")
	}
	if gotReq.Priority != 10 {
		t.Errorf("Expected priority 10 first, got %d", gotReq.Priority)
	}
	queue.Done(gotReq)

	// Then medium priority
	gotReq, shutdown = queue.Get()
	if shutdown {
		t.Error("Expected no shutdown")
	}
	if gotReq.Priority != 5 {
		t.Errorf("Expected priority 5 second, got %d", gotReq.Priority)
	}
	queue.Done(gotReq)

	// Finally low priority
	gotReq, shutdown = queue.Get()
	if shutdown {
		t.Error("Expected no shutdown")
	}
	if gotReq.Priority != 1 {
		t.Errorf("Expected priority 1 last, got %d", gotReq.Priority)
	}
	queue.Done(gotReq)
}

func TestPriorityClusterWorkQueue_SamePriorityFIFO(t *testing.T) {
	queue := NewPriorityClusterWorkQueue("test-priority-queue", nil)
	
	// Add items with same priority but different times
	firstReq := ClusterAwareRequest{
		Key:         "test|first",
		Workspace:   logicalcluster.Name("test"),
		Priority:    5,
		RequestTime: time.Now(),
	}
	
	// Add small delay to ensure different timestamps
	time.Sleep(1 * time.Millisecond)
	
	secondReq := ClusterAwareRequest{
		Key:         "test|second",
		Workspace:   logicalcluster.Name("test"),
		Priority:    5,
		RequestTime: time.Now(),
	}

	queue.Add(firstReq)
	queue.Add(secondReq)

	// Should get first request first (FIFO for same priority)
	gotReq, shutdown := queue.Get()
	if shutdown {
		t.Error("Expected no shutdown")
	}
	if gotReq.Key != firstReq.Key {
		t.Errorf("Expected first request first, got %s", gotReq.Key)
	}
	queue.Done(gotReq)

	// Then second request
	gotReq, shutdown = queue.Get()
	if shutdown {
		t.Error("Expected no shutdown")
	}
	if gotReq.Key != secondReq.Key {
		t.Errorf("Expected second request second, got %s", gotReq.Key)
	}
	queue.Done(gotReq)
}

func TestPriorityClusterWorkQueue_AddWithPriority(t *testing.T) {
	queue := NewPriorityClusterWorkQueue("test-priority-queue", nil)
	
	req := ClusterAwareRequest{
		Key:       "test|priority-override",
		Workspace: logicalcluster.Name("test"),
		Priority:  1, // Original priority
	}

	// Add with different priority
	queue.AddWithPriority(req, 20)

	gotReq, shutdown := queue.Get()
	if shutdown {
		t.Error("Expected no shutdown")
	}
	if gotReq.Priority != 20 {
		t.Errorf("Expected priority 20 (overridden), got %d", gotReq.Priority)
	}
	queue.Done(gotReq)
}

func TestPriorityClusterWorkQueue_WorkspacePriorityDefault(t *testing.T) {
	queue := NewPriorityClusterWorkQueue("test-priority-queue", nil)
	
	workspace := logicalcluster.Name("high-priority-workspace")
	queue.SetWorkspacePriority(workspace, 30)
	
	req := ClusterAwareRequest{
		Key:       "test|workspace-priority",
		Workspace: workspace,
		Priority:  0, // No explicit priority, should use workspace priority
	}

	queue.Add(req)

	gotReq, shutdown := queue.Get()
	if shutdown {
		t.Error("Expected no shutdown")
	}
	if gotReq.Priority != 30 {
		t.Errorf("Expected priority 30 (from workspace), got %d", gotReq.Priority)
	}
	queue.Done(gotReq)
}

func TestPriorityClusterWorkQueue_AddAfter(t *testing.T) {
	queue := NewPriorityClusterWorkQueue("test-priority-queue", nil)
	
	req := ClusterAwareRequest{
		Key:       "test|delayed-priority",
		Workspace: logicalcluster.Name("test"),
		Priority:  15,
	}

	start := time.Now()
	queue.AddAfter(req, 10*time.Millisecond)

	// Initially empty
	if queue.Len() != 0 {
		t.Errorf("Expected queue length 0 initially, got %d", queue.Len())
	}

	// Wait for item to be added
	time.Sleep(20 * time.Millisecond)
	
	if queue.Len() != 1 {
		t.Errorf("Expected queue length 1 after delay, got %d", queue.Len())
	}

	gotReq, shutdown := queue.Get()
	if shutdown {
		t.Error("Expected no shutdown")
	}
	if gotReq.Priority != 15 {
		t.Errorf("Expected priority 15, got %d", gotReq.Priority)
	}
	
	elapsed := time.Since(start)
	if elapsed < 10*time.Millisecond {
		t.Errorf("Expected at least 10ms delay, got %v", elapsed)
	}
	
	queue.Done(gotReq)
}

func TestConfigurePriorityQueue(t *testing.T) {
	queue := NewPriorityClusterWorkQueue("test-config-queue", nil)
	
	config := WorkspacePriorityConfig{
		SystemWorkspaces: []logicalcluster.Name{
			logicalcluster.Name("system"),
			logicalcluster.Name("kcp-system"),
		},
		HighPriorityWorkspaces: []logicalcluster.Name{
			logicalcluster.Name("important"),
		},
		DefaultPriority: 10,
	}

	ConfigurePriorityQueue(queue, config)
	
	// Test system workspace priority
	if queue.GetWorkspacePriority(logicalcluster.Name("system")) != 100 {
		t.Errorf("Expected system workspace priority 100, got %d", 
			queue.GetWorkspacePriority(logicalcluster.Name("system")))
	}
	
	if queue.GetWorkspacePriority(logicalcluster.Name("kcp-system")) != 100 {
		t.Errorf("Expected kcp-system workspace priority 100, got %d", 
			queue.GetWorkspacePriority(logicalcluster.Name("kcp-system")))
	}
	
	// Test high priority workspace
	if queue.GetWorkspacePriority(logicalcluster.Name("important")) != 50 {
		t.Errorf("Expected important workspace priority 50, got %d", 
			queue.GetWorkspacePriority(logicalcluster.Name("important")))
	}
	
	// Test unspecified workspace (should remain at default 0)
	if queue.GetWorkspacePriority(logicalcluster.Name("unspecified")) != 0 {
		t.Errorf("Expected unspecified workspace priority 0, got %d", 
			queue.GetWorkspacePriority(logicalcluster.Name("unspecified")))
	}
}

func TestPriorityQueue_HeapOperations(t *testing.T) {
	pq := make(priorityQueue, 0)
	
	// Test basic heap operations using heap package
	baseTime := time.Now()
	items := []*priorityItem{
		{request: ClusterAwareRequest{Priority: 1, RequestTime: baseTime.Add(2 * time.Second)}},
		{request: ClusterAwareRequest{Priority: 5, RequestTime: baseTime}},
		{request: ClusterAwareRequest{Priority: 3, RequestTime: baseTime.Add(time.Second)}},
	}
	
	// Push items using heap interface
	for _, item := range items {
		heap.Push(&pq, item)
	}
	
	if pq.Len() != 3 {
		t.Errorf("Expected length 3, got %d", pq.Len())
	}
	
	// Pop should give highest priority first
	item := heap.Pop(&pq).(*priorityItem)
	if item.request.Priority != 5 {
		t.Errorf("Expected priority 5, got %d", item.request.Priority)
	}
	
	item = heap.Pop(&pq).(*priorityItem)
	if item.request.Priority != 3 {
		t.Errorf("Expected priority 3, got %d", item.request.Priority)
	}
	
	item = heap.Pop(&pq).(*priorityItem)
	if item.request.Priority != 1 {
		t.Errorf("Expected priority 1, got %d", item.request.Priority)
	}
}

func TestPriorityQueue_Less(t *testing.T) {
	baseTime := time.Now()
	pq := priorityQueue{
		{request: ClusterAwareRequest{Priority: 5, RequestTime: baseTime}},
		{request: ClusterAwareRequest{Priority: 3, RequestTime: baseTime.Add(time.Second)}},
		{request: ClusterAwareRequest{Priority: 5, RequestTime: baseTime.Add(time.Second)}},
	}
	
	// Same priority: earlier time should be less (higher priority)
	if !pq.Less(0, 2) {
		t.Error("Expected earlier time to have higher priority for same priority level")
	}
	
	// Different priority: higher priority should be less (comes first)
	if !pq.Less(0, 1) {
		t.Error("Expected higher priority to come first")
	}
}