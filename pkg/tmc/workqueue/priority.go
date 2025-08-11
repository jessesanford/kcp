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
	"sync"
	"time"

	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"
)

// PriorityClusterWorkQueue extends TypedClusterWorkQueue with priority-based processing.
// This allows high-priority requests (like system workspaces) to be processed first.
type PriorityClusterWorkQueue interface {
	TypedClusterWorkQueue
	
	// AddWithPriority adds an item with a specific priority
	AddWithPriority(item ClusterAwareRequest, priority int)
	
	// SetWorkspacePriority sets the default priority for a workspace
	SetWorkspacePriority(workspace logicalcluster.Name, priority int)
	
	// GetWorkspacePriority gets the configured priority for a workspace
	GetWorkspacePriority(workspace logicalcluster.Name) int
}

// priorityQueue implements a priority queue using Go's heap interface
type priorityQueue []*priorityItem

type priorityItem struct {
	request ClusterAwareRequest
	index   int
}

func (pq priorityQueue) Len() int { return len(pq) }

func (pq priorityQueue) Less(i, j int) bool {
	// Higher priority first, then FIFO for same priority
	if pq[i].request.Priority == pq[j].request.Priority {
		return pq[i].request.RequestTime.Before(pq[j].request.RequestTime)
	}
	return pq[i].request.Priority > pq[j].request.Priority
}

func (pq priorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *priorityQueue) Push(x interface{}) {
	n := len(*pq)
	item := x.(*priorityItem)
	item.index = n
	*pq = append(*pq, item)
}

func (pq *priorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // avoid memory leak
	item.index = -1 // for safety
	*pq = old[0 : n-1]
	return item
}

// priorityClusterWorkQueueImpl implements PriorityClusterWorkQueue
type priorityClusterWorkQueueImpl struct {
	name string
	
	// Priority queue for items
	queue        priorityQueue
	processing   map[ClusterAwareRequest]bool
	dirty        map[ClusterAwareRequest]bool
	
	// Workspace priority mapping
	workspacePriorities map[logicalcluster.Name]int
	
	// Rate limiting
	rateLimiter workqueue.TypedRateLimiter[ClusterAwareRequest]
	
	// Synchronization
	mu          sync.RWMutex
	cond        *sync.Cond
	shuttingDown bool
	
	// Metrics
	metrics MetricsCollector
}

// NewPriorityClusterWorkQueue creates a new priority-aware typed work queue for KCP controllers.
// This provides priority-based processing where higher priority items are processed first.
func NewPriorityClusterWorkQueue(name string, metrics MetricsCollector) PriorityClusterWorkQueue {
	if metrics == nil {
		metrics = &NoOpMetricsCollector{}
	}
	
	q := &priorityClusterWorkQueueImpl{
		name:                name,
		queue:               make(priorityQueue, 0),
		processing:          make(map[ClusterAwareRequest]bool),
		dirty:               make(map[ClusterAwareRequest]bool),
		workspacePriorities: make(map[logicalcluster.Name]int),
		rateLimiter:         workqueue.DefaultTypedControllerRateLimiter[ClusterAwareRequest](),
		metrics:             metrics,
	}
	
	q.cond = sync.NewCond(&q.mu)
	heap.Init(&q.queue)
	
	return q
}

// SetWorkspacePriority implements PriorityClusterWorkQueue.SetWorkspacePriority
func (q *priorityClusterWorkQueueImpl) SetWorkspacePriority(workspace logicalcluster.Name, priority int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	
	q.workspacePriorities[workspace] = priority
	klog.V(4).InfoS("Set workspace priority", 
		"queue", q.name,
		"workspace", workspace,
		"priority", priority)
}

// GetWorkspacePriority implements PriorityClusterWorkQueue.GetWorkspacePriority
func (q *priorityClusterWorkQueueImpl) GetWorkspacePriority(workspace logicalcluster.Name) int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	
	if priority, exists := q.workspacePriorities[workspace]; exists {
		return priority
	}
	return 0 // Default priority
}

// Add implements PriorityClusterWorkQueue.Add
func (q *priorityClusterWorkQueueImpl) Add(item ClusterAwareRequest) {
	// Use workspace priority if item priority is not set
	if item.Priority == 0 {
		item.Priority = q.GetWorkspacePriority(item.Workspace)
	}
	q.AddWithPriority(item, item.Priority)
}

// AddWithPriority implements PriorityClusterWorkQueue.AddWithPriority
func (q *priorityClusterWorkQueueImpl) AddWithPriority(item ClusterAwareRequest, priority int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	
	if q.shuttingDown {
		return
	}
	
	item.Priority = priority
	if item.RequestTime.IsZero() {
		item.RequestTime = time.Now()
	}
	
	if _, processing := q.processing[item]; processing {
		q.dirty[item] = true
		return
	}
	
	if _, dirty := q.dirty[item]; dirty {
		return
	}
	
	q.dirty[item] = true
	heap.Push(&q.queue, &priorityItem{request: item})
	q.metrics.RecordAdd(item.Workspace, q.name)
	q.cond.Signal()
	
	klog.V(6).InfoS("Added item to priority work queue", 
		"queue", q.name, 
		"workspace", item.Workspace,
		"key", item.Key,
		"priority", priority)
}

// AddAfter implements PriorityClusterWorkQueue.AddAfter
func (q *priorityClusterWorkQueueImpl) AddAfter(item ClusterAwareRequest, duration time.Duration) {
	// Use workspace priority if item priority is not set
	if item.Priority == 0 {
		item.Priority = q.GetWorkspacePriority(item.Workspace)
	}
	
	// Simple implementation - just delay and add
	go func() {
		time.Sleep(duration)
		q.AddWithPriority(item, item.Priority)
	}()
	
	klog.V(6).InfoS("Added item to priority work queue with delay", 
		"queue", q.name, 
		"workspace", item.Workspace,
		"key", item.Key,
		"priority", item.Priority,
		"delay", duration)
}

// AddRateLimited implements PriorityClusterWorkQueue.AddRateLimited
func (q *priorityClusterWorkQueueImpl) AddRateLimited(item ClusterAwareRequest) {
	item.RetryCount++
	q.metrics.RecordRetry(item.Workspace, q.name, item.RetryCount)
	
	delay := q.rateLimiter.When(item)
	q.AddAfter(item, delay)
	
	klog.V(4).InfoS("Added item to priority work queue with rate limiting", 
		"queue", q.name, 
		"workspace", item.Workspace,
		"key", item.Key,
		"priority", item.Priority,
		"retryCount", item.RetryCount,
		"delay", delay)
}

// Get implements PriorityClusterWorkQueue.Get
func (q *priorityClusterWorkQueueImpl) Get() (ClusterAwareRequest, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	
	for len(q.queue) == 0 && !q.shuttingDown {
		q.cond.Wait()
	}
	
	if len(q.queue) == 0 {
		// We're shutting down
		return ClusterAwareRequest{}, true
	}
	
	item := heap.Pop(&q.queue).(*priorityItem)
	req := item.request
	
	q.processing[req] = true
	delete(q.dirty, req)
	
	q.metrics.RecordGet(req.Workspace, q.name)
	q.metrics.RecordDepth(q.name, len(q.queue))
	
	return req, false
}

// Done implements PriorityClusterWorkQueue.Done
func (q *priorityClusterWorkQueueImpl) Done(item ClusterAwareRequest) {
	q.mu.Lock()
	defer q.mu.Unlock()
	
	delete(q.processing, item)
	
	if q.dirty[item] {
		heap.Push(&q.queue, &priorityItem{request: item})
		q.cond.Signal()
	}
}

// Forget implements PriorityClusterWorkQueue.Forget
func (q *priorityClusterWorkQueueImpl) Forget(item ClusterAwareRequest) {
	q.rateLimiter.Forget(item)
}

// NumRequeues implements PriorityClusterWorkQueue.NumRequeues
func (q *priorityClusterWorkQueueImpl) NumRequeues(item ClusterAwareRequest) int {
	return q.rateLimiter.NumRequeues(item)
}

// Len implements PriorityClusterWorkQueue.Len
func (q *priorityClusterWorkQueueImpl) Len() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.queue)
}

// ShutDown implements PriorityClusterWorkQueue.ShutDown
func (q *priorityClusterWorkQueueImpl) ShutDown() {
	q.mu.Lock()
	defer q.mu.Unlock()
	
	q.shuttingDown = true
	q.cond.Broadcast()
	
	klog.InfoS("Shutting down priority work queue", "queue", q.name)
}

// ShuttingDown implements PriorityClusterWorkQueue.ShuttingDown
func (q *priorityClusterWorkQueueImpl) ShuttingDown() bool {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.shuttingDown
}

// WorkspacePriorityConfig provides configuration for workspace priorities
type WorkspacePriorityConfig struct {
	// SystemWorkspaces are given the highest priority
	SystemWorkspaces []logicalcluster.Name
	
	// HighPriorityWorkspaces are given high priority
	HighPriorityWorkspaces []logicalcluster.Name
	
	// DefaultPriority is the priority for unspecified workspaces
	DefaultPriority int
}

// ConfigurePriorityQueue configures workspace priorities for the given queue
func ConfigurePriorityQueue(queue PriorityClusterWorkQueue, config WorkspacePriorityConfig) {
	// System workspaces get highest priority (100)
	for _, workspace := range config.SystemWorkspaces {
		queue.SetWorkspacePriority(workspace, 100)
	}
	
	// High priority workspaces get priority 50
	for _, workspace := range config.HighPriorityWorkspaces {
		queue.SetWorkspacePriority(workspace, 50)
	}
	
	klog.V(2).InfoS("Configured workspace priorities",
		"systemWorkspaces", len(config.SystemWorkspaces),
		"highPriorityWorkspaces", len(config.HighPriorityWorkspaces),
		"defaultPriority", config.DefaultPriority)
}