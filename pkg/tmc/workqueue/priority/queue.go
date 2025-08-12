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

package priority

import (
	"container/heap"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/client-go/util/workqueue"
)

// priorityQueue implements PriorityQueue interface with KCP typed workqueue patterns.
// It uses a priority heap internally to ensure items are processed in priority order.
type priorityQueue struct {
	// Embed the standard typed rate limiting queue for basic functionality
	workqueue.TypedRateLimitingInterface[string]
	
	// Priority management
	mu              sync.RWMutex
	priorityItems   map[string]*PriorityItem  // Key -> PriorityItem mapping
	heap           *priorityHeap             // Priority heap for ordering
	config         *PriorityConfig           // Configuration
	
	// Metrics and observability
	metrics        PriorityQueueMetrics
	clock          clock.Clock
	
	// Internal tracking
	shutDown       bool
}

// NewPriorityQueue creates a new priority-based workqueue with KCP patterns.
// It wraps the standard TypedRateLimitingInterface but adds priority support.
func NewPriorityQueue(name string, config *PriorityConfig) PriorityQueue {
	if config == nil {
		config = DefaultPriorityConfig()
	}
	
	// Create the underlying typed rate limiting queue
	rateLimiter := workqueue.DefaultTypedControllerRateLimiter[string]()
	underlyingQueue := workqueue.NewTypedRateLimitingQueueWithConfig(
		rateLimiter,
		workqueue.TypedRateLimitingQueueConfig[string]{
			Name: name,
		},
	)
	
	pq := &priorityQueue{
		TypedRateLimitingInterface: underlyingQueue,
		priorityItems:             make(map[string]*PriorityItem),
		heap:                      &priorityHeap{},
		config:                    config,
		clock:                     clock.RealClock{},
	}
	
	heap.Init(pq.heap)
	
	return pq
}

// AddWithPriority implements PriorityQueue.AddWithPriority
func (pq *priorityQueue) AddWithPriority(key string, priority Priority) {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	
	if pq.shutDown {
		return
	}
	
	// Create or update the priority item
	item, exists := pq.priorityItems[key]
	if exists {
		// Update existing item's priority
		item.Priority = priority
		heap.Fix(pq.heap, item.heapIndex)
	} else {
		// Create new priority item
		item = NewPriorityItem(key, priority)
		pq.priorityItems[key] = item
		heap.Push(pq.heap, item)
	}
	
	// Add to underlying queue
	pq.TypedRateLimitingInterface.Add(key)
}

// GetPriority implements PriorityQueue.GetPriority
func (pq *priorityQueue) GetPriority(key string) (Priority, bool) {
	pq.mu.RLock()
	defer pq.mu.RUnlock()
	
	item, exists := pq.priorityItems[key]
	if !exists {
		return Normal, false
	}
	
	return item.Priority, true
}

// UpdatePriority implements PriorityQueue.UpdatePriority
func (pq *priorityQueue) UpdatePriority(key string, priority Priority) bool {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	
	item, exists := pq.priorityItems[key]
	if !exists {
		return false
	}
	
	// Update priority and fix heap
	item.Priority = priority
	heap.Fix(pq.heap, item.heapIndex)
	
	return true
}

// Get overrides the underlying queue's Get to implement priority ordering
func (pq *priorityQueue) Get() (string, bool) {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	
	if pq.shutDown || pq.heap.Len() == 0 {
		return "", false
	}
	
	// Get the highest priority item
	item := heap.Pop(pq.heap).(*PriorityItem)
	key := item.Key
	
	// Remove from our tracking
	delete(pq.priorityItems, key)
	
	// Update metrics
	pq.updateMetrics()
	
	// Delegate to underlying queue for rate limiting
	underlyingKey, quit := pq.TypedRateLimitingInterface.Get()
	if quit {
		return "", true
	}
	
	// Return our priority-ordered key instead of underlying queue's key
	return key, false
}

// Done overrides to handle priority item cleanup
func (pq *priorityQueue) Done(key string) {
	pq.mu.Lock()
	// Clean up if we still have the item (shouldn't happen in normal flow)
	delete(pq.priorityItems, key)
	pq.mu.Unlock()
	
	// Delegate to underlying queue
	pq.TypedRateLimitingInterface.Done(key)
}

// AddRateLimited overrides to add with Normal priority if no priority specified
func (pq *priorityQueue) AddRateLimited(key string) {
	pq.AddWithPriority(key, Normal)
}

// Add overrides to add with Normal priority if no priority specified
func (pq *priorityQueue) Add(key string) {
	pq.AddWithPriority(key, Normal)
}

// Len implements PriorityQueue.Len with accurate count
func (pq *priorityQueue) Len() int {
	pq.mu.RLock()
	defer pq.mu.RUnlock()
	return len(pq.priorityItems)
}

// GetMetrics implements PriorityQueue.GetMetrics
func (pq *priorityQueue) GetMetrics() PriorityQueueMetrics {
	pq.mu.RLock()
	defer pq.mu.RUnlock()
	
	pq.updateMetricsUnsafe()
	return pq.metrics
}

// ShutDown overrides to handle priority queue cleanup
func (pq *priorityQueue) ShutDown() {
	pq.mu.Lock()
	if pq.shutDown {
		pq.mu.Unlock()
		return
	}
	
	pq.shutDown = true
	
	// Clear priority tracking
	pq.priorityItems = make(map[string]*PriorityItem)
	pq.heap = &priorityHeap{}
	
	pq.mu.Unlock()
	
	// Delegate to underlying queue
	pq.TypedRateLimitingInterface.ShutDown()
}

// ShuttingDown returns whether the queue is shutting down
func (pq *priorityQueue) ShuttingDown() bool {
	pq.mu.RLock()
	defer pq.mu.RUnlock()
	return pq.shutDown
}

// updateMetricsUnsafe updates metrics without acquiring lock (caller must hold lock)
func (pq *priorityQueue) updateMetricsUnsafe() {
	pq.metrics.TotalItems = len(pq.priorityItems)
	pq.metrics.ItemsByPriority = make(map[Priority]int)
	
	var totalAge time.Duration
	var oldestAge time.Duration
	
	now := pq.clock.Now()
	for _, item := range pq.priorityItems {
		pq.metrics.ItemsByPriority[item.Priority]++
		
		age := now.Sub(item.AddedAt)
		totalAge += age
		
		if age > oldestAge {
			oldestAge = age
		}
	}
	
	pq.metrics.OldestItem = oldestAge
	if len(pq.priorityItems) > 0 {
		pq.metrics.AverageWaitTime = totalAge / time.Duration(len(pq.priorityItems))
	} else {
		pq.metrics.AverageWaitTime = 0
	}
}

// priorityHeap implements heap.Interface for PriorityItem
type priorityHeap []*PriorityItem

func (h priorityHeap) Len() int { 
	return len(h) 
}

func (h priorityHeap) Less(i, j int) bool {
	// Higher effective priority means higher precedence (earlier in processing)
	return h[i].EffectivePriority() > h[j].EffectivePriority()
}

func (h priorityHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].heapIndex = i
	h[j].heapIndex = j
}

func (h *priorityHeap) Push(x interface{}) {
	item := x.(*PriorityItem)
	item.heapIndex = len(*h)
	*h = append(*h, item)
}

func (h *priorityHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	item.heapIndex = -1
	*h = old[0 : n-1]
	return item
}

