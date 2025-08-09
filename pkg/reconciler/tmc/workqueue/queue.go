/*
Copyright 2025 The KCP Authors.

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

package workqueue

import (
	"context"
	"fmt"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

// tmcWorkQueue implements TMCWorkQueue with enhanced retry and rate limiting.
type tmcWorkQueue struct {
	// queue is the underlying workqueue
	queue workqueue.TypedRateLimitingInterface[string]

	// items stores work items by their keys
	items map[string]*WorkItem

	// priorities maintains priority-ordered items
	priorities map[Priority][]string

	// options configures the workqueue behavior
	options WorkQueueOptions

	// circuitBreaker protects against cascading failures
	circuitBreaker *circuitBreaker

	// metrics tracks queue performance
	metrics *queueMetrics

	// mu protects concurrent access
	mu sync.RWMutex

	// shutdownOnce ensures graceful shutdown happens only once
	shutdownOnce sync.Once
}

// NewTMCWorkQueue creates a new TMC-enhanced workqueue.
func NewTMCWorkQueue(options WorkQueueOptions) (TMCWorkQueue, error) {
	if options.Name == "" {
		return nil, fmt.Errorf("workqueue name cannot be empty")
	}

	// Set defaults
	if options.RetryPolicy.MaxAttempts == 0 {
		options.RetryPolicy.MaxAttempts = 5
	}
	if options.RetryPolicy.BaseDelay == 0 {
		options.RetryPolicy.BaseDelay = time.Second
	}
	if options.RetryPolicy.MaxDelay == 0 {
		options.RetryPolicy.MaxDelay = time.Minute
	}
	if options.RetryPolicy.BackoffFactor == 0 {
		options.RetryPolicy.BackoffFactor = 2.0
	}
	if options.ProcessingTimeout == 0 {
		options.ProcessingTimeout = 30 * time.Second
	}
	if options.ShutdownTimeout == 0 {
		options.ShutdownTimeout = 30 * time.Second
	}

	// Create rate limiter
	rateLimiter := workqueue.NewTypedItemExponentialFailureRateLimiter[string](
		options.RetryPolicy.BaseDelay,
		options.RetryPolicy.MaxDelay,
	)

	// Create underlying workqueue
	queue := workqueue.NewTypedRateLimitingQueueWithConfig(
		rateLimiter,
		workqueue.TypedRateLimitingQueueConfig[string]{
			Name: options.Name,
		},
	)

	tmcQueue := &tmcWorkQueue{
		queue:      queue,
		items:      make(map[string]*WorkItem),
		priorities: make(map[Priority][]string),
		options:    options,
		metrics:    newQueueMetrics(options.Name),
	}

	// Setup circuit breaker if configured
	if options.CircuitBreaker != nil {
		cb, err := newCircuitBreaker(*options.CircuitBreaker)
		if err != nil {
			return nil, fmt.Errorf("failed to create circuit breaker: %w", err)
		}
		tmcQueue.circuitBreaker = cb
	}

	return tmcQueue, nil
}

// Add adds a work item to the queue.
func (q *tmcWorkQueue) Add(ctx context.Context, item *WorkItem) error {
	return q.AddWithPriority(ctx, item, PriorityNormal)
}

// AddAfter adds a work item to be processed after the specified duration.
func (q *tmcWorkQueue) AddAfter(ctx context.Context, item *WorkItem, duration time.Duration) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.queue.ShuttingDown() {
		return fmt.Errorf("queue is shutting down")
	}

	// Generate ID if not provided
	if item.ID == "" {
		item.ID = string(uuid.NewUUID())
	}

	// Set creation time
	if item.CreatedAt.IsZero() {
		item.CreatedAt = time.Now()
	}

	// Generate key if not provided
	if item.Key == "" {
		item.Key = fmt.Sprintf("%s:%s:%s", item.Type, item.Workspace, item.ID)
	}

	// Store item
	q.items[item.Key] = item

	// Add to priority queue
	q.addToPriorityQueue(item.Key, item.Priority)

	// Add to workqueue after delay
	q.queue.AddAfter(item.Key, duration)

	q.metrics.recordAdd(item)

	klog.V(6).Infof("Added work item %s to queue %s with delay %v", 
		item.Key, q.options.Name, duration)

	return nil
}

// AddWithPriority adds a work item with specific priority.
func (q *tmcWorkQueue) AddWithPriority(ctx context.Context, item *WorkItem, priority Priority) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.queue.ShuttingDown() {
		return fmt.Errorf("queue is shutting down")
	}

	// Generate ID if not provided
	if item.ID == "" {
		item.ID = string(uuid.NewUUID())
	}

	// Set priority
	item.Priority = priority

	// Set creation time
	if item.CreatedAt.IsZero() {
		item.CreatedAt = time.Now()
	}

	// Generate key if not provided
	if item.Key == "" {
		item.Key = fmt.Sprintf("%s:%s:%s", item.Type, item.Workspace, item.ID)
	}

	// Store item
	q.items[item.Key] = item

	// Add to priority queue
	q.addToPriorityQueue(item.Key, priority)

	// Add to workqueue
	q.queue.Add(item.Key)

	q.metrics.recordAdd(item)

	klog.V(6).Infof("Added work item %s to queue %s with priority %v", 
		item.Key, q.options.Name, priority)

	return nil
}

// Get retrieves the next work item from the queue.
func (q *tmcWorkQueue) Get() (*WorkItem, bool) {
	key, quit := q.queue.Get()
	if quit {
		return nil, false
	}

	q.mu.RLock()
	item, exists := q.items[key]
	q.mu.RUnlock()

	if !exists {
		// Item was deleted, mark as done and try next
		q.queue.Done(key)
		return q.Get()
	}

	// Update attempt tracking
	item.Attempts++
	item.LastAttemptAt = time.Now()

	q.metrics.recordGet(item)

	klog.V(8).Infof("Retrieved work item %s from queue %s (attempt %d)", 
		item.Key, q.options.Name, item.Attempts)

	return item, true
}

// Done marks a work item as processed.
func (q *tmcWorkQueue) Done(item *WorkItem) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Remove from items map
	delete(q.items, item.Key)

	// Remove from priority queues
	q.removeFromPriorityQueue(item.Key, item.Priority)

	// Mark done in workqueue
	q.queue.Done(item.Key)

	q.metrics.recordDone(item)

	klog.V(8).Infof("Marked work item %s as done in queue %s", 
		item.Key, q.options.Name)
}

// AddRateLimited adds a work item with rate limiting applied.
func (q *tmcWorkQueue) AddRateLimited(ctx context.Context, item *WorkItem) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.queue.ShuttingDown() {
		return fmt.Errorf("queue is shutting down")
	}

	// Check circuit breaker
	if q.circuitBreaker != nil && !q.circuitBreaker.Allow() {
		return fmt.Errorf("circuit breaker open, rejecting work item")
	}

	// Generate ID if not provided
	if item.ID == "" {
		item.ID = string(uuid.NewUUID())
	}

	// Set creation time
	if item.CreatedAt.IsZero() {
		item.CreatedAt = time.Now()
	}

	// Generate key if not provided
	if item.Key == "" {
		item.Key = fmt.Sprintf("%s:%s:%s", item.Type, item.Workspace, item.ID)
	}

	// Store item
	q.items[item.Key] = item

	// Add to priority queue
	q.addToPriorityQueue(item.Key, item.Priority)

	// Add to workqueue with rate limiting
	q.queue.AddRateLimited(item.Key)

	q.metrics.recordRateLimited(item)

	klog.V(6).Infof("Added rate-limited work item %s to queue %s", 
		item.Key, q.options.Name)

	return nil
}

// Forget removes rate limiting tracking for a work item.
func (q *tmcWorkQueue) Forget(item *WorkItem) {
	q.queue.Forget(item.Key)
	q.metrics.recordForget(item)

	klog.V(8).Infof("Forgot work item %s in queue %s", 
		item.Key, q.options.Name)
}

// NumRequeues returns the number of times an item has been requeued.
func (q *tmcWorkQueue) NumRequeues(item *WorkItem) int {
	return q.queue.NumRequeues(item.Key)
}

// Len returns the current queue length.
func (q *tmcWorkQueue) Len() int {
	return q.queue.Len()
}

// ShutDown shuts down the work queue.
func (q *tmcWorkQueue) ShutDown() {
	q.shutdownOnce.Do(func() {
		klog.V(4).Infof("Shutting down TMC workqueue %s", q.options.Name)
		q.queue.ShutDown()
		q.metrics.recordShutdown()
	})
}

// ShuttingDown returns true if the queue is shutting down.
func (q *tmcWorkQueue) ShuttingDown() bool {
	return q.queue.ShuttingDown()
}

// addToPriorityQueue adds an item to the priority queue.
func (q *tmcWorkQueue) addToPriorityQueue(key string, priority Priority) {
	if q.priorities[priority] == nil {
		q.priorities[priority] = make([]string, 0)
	}
	q.priorities[priority] = append(q.priorities[priority], key)
}

// removeFromPriorityQueue removes an item from the priority queue.
func (q *tmcWorkQueue) removeFromPriorityQueue(key string, priority Priority) {
	items := q.priorities[priority]
	for i, item := range items {
		if item == key {
			q.priorities[priority] = append(items[:i], items[i+1:]...)
			break
		}
	}
}

// GetMetrics returns current queue metrics.
func (q *tmcWorkQueue) GetMetrics() WorkQueueMetrics {
	q.mu.RLock()
	defer q.mu.RUnlock()

	return q.metrics.getMetrics(q.Len(), len(q.items))
}