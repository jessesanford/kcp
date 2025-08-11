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

// Package workqueue provides advanced typed workqueue patterns specifically designed
// for KCP controllers with proper workspace isolation and cluster-aware processing.
package workqueue

import (
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	kcpcache "github.com/kcp-dev/apimachinery/v2/pkg/cache"
	"github.com/kcp-dev/logicalcluster/v3"
)

// ClusterAwareRequest represents a typed reconciliation request that includes
// proper workspace context and priority information for KCP controllers.
type ClusterAwareRequest struct {
	// Key is the cluster-aware object key in format: cluster|namespace/name or cluster|name
	Key string
	
	// Workspace is the logical cluster workspace for this request
	Workspace logicalcluster.Name
	
	// Priority indicates the priority of this reconciliation request
	// Higher values indicate higher priority
	Priority int
	
	// RequestTime is when this request was created (for metrics and debugging)
	RequestTime time.Time
	
	// RetryCount tracks how many times this request has been retried
	RetryCount int
}

// String implements fmt.Stringer for better logging and debugging
func (r ClusterAwareRequest) String() string {
	return fmt.Sprintf("ClusterAwareRequest{Key=%s, Workspace=%s, Priority=%d, Retries=%d}", 
		r.Key, r.Workspace, r.Priority, r.RetryCount)
}

// TypedClusterWorkQueue provides a strongly-typed work queue interface
// specifically designed for KCP controllers with workspace isolation.
type TypedClusterWorkQueue interface {
	// Add adds an item to the work queue
	Add(item ClusterAwareRequest)
	
	// AddAfter adds an item to the work queue after a specified duration
	AddAfter(item ClusterAwareRequest, duration time.Duration)
	
	// AddRateLimited adds an item to the work queue with rate limiting
	AddRateLimited(item ClusterAwareRequest)
	
	// Get blocks until it can return an item to be processed
	Get() (item ClusterAwareRequest, shutdown bool)
	
	// Done marks the item as done processing
	Done(item ClusterAwareRequest)
	
	// Forget tells the queue to stop tracking history for the item
	Forget(item ClusterAwareRequest)
	
	// NumRequeues returns the number of times an item has been requeued
	NumRequeues(item ClusterAwareRequest) int
	
	// Len returns the current depth of the workqueue
	Len() int
	
	// ShutDown will cause Get() to return shutdown=true
	ShutDown()
	
	// ShuttingDown returns whether the queue is shutting down
	ShuttingDown() bool
}

// typedClusterWorkQueueImpl implements TypedClusterWorkQueue using Kubernetes typed workqueue
type typedClusterWorkQueueImpl struct {
	queue workqueue.TypedRateLimitingInterface[ClusterAwareRequest]
	name  string
}

// NewTypedClusterWorkQueue creates a new typed work queue for KCP controllers.
// This provides proper type safety and workspace awareness for work queue operations.
func NewTypedClusterWorkQueue(name string) TypedClusterWorkQueue {
	return &typedClusterWorkQueueImpl{
		queue: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[ClusterAwareRequest](),
			workqueue.TypedRateLimitingQueueConfig[ClusterAwareRequest]{
				Name: name,
			},
		),
		name: name,
	}
}

// Add implements TypedClusterWorkQueue.Add
func (q *typedClusterWorkQueueImpl) Add(item ClusterAwareRequest) {
	if item.RequestTime.IsZero() {
		item.RequestTime = time.Now()
	}
	q.queue.Add(item)
	klog.V(6).InfoS("Added item to work queue", 
		"queue", q.name, 
		"workspace", item.Workspace,
		"key", item.Key)
}

// AddAfter implements TypedClusterWorkQueue.AddAfter
func (q *typedClusterWorkQueueImpl) AddAfter(item ClusterAwareRequest, duration time.Duration) {
	if item.RequestTime.IsZero() {
		item.RequestTime = time.Now()
	}
	q.queue.AddAfter(item, duration)
	klog.V(6).InfoS("Added item to work queue with delay", 
		"queue", q.name, 
		"workspace", item.Workspace,
		"key", item.Key,
		"delay", duration)
}

// AddRateLimited implements TypedClusterWorkQueue.AddRateLimited
func (q *typedClusterWorkQueueImpl) AddRateLimited(item ClusterAwareRequest) {
	if item.RequestTime.IsZero() {
		item.RequestTime = time.Now()
	}
	item.RetryCount++
	q.queue.AddRateLimited(item)
	klog.V(4).InfoS("Added item to work queue with rate limiting", 
		"queue", q.name, 
		"workspace", item.Workspace,
		"key", item.Key,
		"retryCount", item.RetryCount)
}

// Get implements TypedClusterWorkQueue.Get
func (q *typedClusterWorkQueueImpl) Get() (ClusterAwareRequest, bool) {
	return q.queue.Get()
}

// Done implements TypedClusterWorkQueue.Done
func (q *typedClusterWorkQueueImpl) Done(item ClusterAwareRequest) {
	q.queue.Done(item)
}

// Forget implements TypedClusterWorkQueue.Forget
func (q *typedClusterWorkQueueImpl) Forget(item ClusterAwareRequest) {
	q.queue.Forget(item)
}

// NumRequeues implements TypedClusterWorkQueue.NumRequeues
func (q *typedClusterWorkQueueImpl) NumRequeues(item ClusterAwareRequest) int {
	return q.queue.NumRequeues(item)
}

// Len implements TypedClusterWorkQueue.Len
func (q *typedClusterWorkQueueImpl) Len() int {
	return q.queue.Len()
}

// ShutDown implements TypedClusterWorkQueue.ShutDown
func (q *typedClusterWorkQueueImpl) ShutDown() {
	klog.InfoS("Shutting down work queue", "queue", q.name)
	q.queue.ShutDown()
}

// ShuttingDown implements TypedClusterWorkQueue.ShuttingDown
func (q *typedClusterWorkQueueImpl) ShuttingDown() bool {
	return q.queue.ShuttingDown()
}

// QueueKeyForObject generates a cluster-aware queue key for the given Kubernetes object.
// This is the standard way to create keys that respect workspace boundaries in KCP.
func QueueKeyForObject(obj interface{}) (ClusterAwareRequest, error) {
	key, err := kcpcache.DeletionHandlingMetaClusterNamespaceKeyFunc(obj)
	if err != nil {
		return ClusterAwareRequest{}, fmt.Errorf("couldn't get key for object %+v: %v", obj, err)
	}
	
	// Extract workspace from object using KCP's logicalcluster.From pattern
	var workspace logicalcluster.Name
	if metaObj, ok := obj.(metav1.Object); ok {
		workspace = logicalcluster.From(metaObj)
	} else {
		workspace = logicalcluster.Name("")
	}
	
	return ClusterAwareRequest{
		Key:         key,
		Workspace:   workspace,
		Priority:    0,
		RequestTime: time.Now(),
		RetryCount:  0,
	}, nil
}

// QueueKeyForClusterObject creates a cluster-aware queue key for objects
// that already have cluster information extracted.
func QueueKeyForClusterObject(workspace logicalcluster.Name, namespace, name string, priority int) ClusterAwareRequest {
	var key string
	if namespace != "" {
		key = kcpcache.ToClusterAwareKey(workspace.String(), namespace, name)
	} else {
		key = kcpcache.ToClusterAwareKey(workspace.String(), "", name)
	}
	
	return ClusterAwareRequest{
		Key:         key,
		Workspace:   workspace,
		Priority:    priority,
		RequestTime: time.Now(),
		RetryCount:  0,
	}
}

// WorkQueueMetrics provides metrics collection for typed work queues
type WorkQueueMetrics struct {
	// TotalAdds tracks total number of items added to the queue
	TotalAdds int64
	
	// TotalGets tracks total number of items retrieved from the queue
	TotalGets int64
	
	// TotalRetries tracks total number of retries
	TotalRetries int64
	
	// CurrentDepth tracks current queue depth
	CurrentDepth int
	
	// MaxDepth tracks maximum queue depth seen
	MaxDepth int
}

// MetricsCollector provides an interface for collecting work queue metrics
type MetricsCollector interface {
	// RecordAdd records an item being added to the queue
	RecordAdd(workspace logicalcluster.Name, queueName string)
	
	// RecordGet records an item being retrieved from the queue
	RecordGet(workspace logicalcluster.Name, queueName string)
	
	// RecordRetry records an item being retried
	RecordRetry(workspace logicalcluster.Name, queueName string, retryCount int)
	
	// RecordDepth records the current queue depth
	RecordDepth(queueName string, depth int)
	
	// GetMetrics returns current metrics for the queue
	GetMetrics(queueName string) WorkQueueMetrics
}

// NoOpMetricsCollector provides a no-op implementation of MetricsCollector
type NoOpMetricsCollector struct{}

// RecordAdd implements MetricsCollector.RecordAdd
func (n *NoOpMetricsCollector) RecordAdd(workspace logicalcluster.Name, queueName string) {}

// RecordGet implements MetricsCollector.RecordGet  
func (n *NoOpMetricsCollector) RecordGet(workspace logicalcluster.Name, queueName string) {}

// RecordRetry implements MetricsCollector.RecordRetry
func (n *NoOpMetricsCollector) RecordRetry(workspace logicalcluster.Name, queueName string, retryCount int) {}

// RecordDepth implements MetricsCollector.RecordDepth
func (n *NoOpMetricsCollector) RecordDepth(queueName string, depth int) {}

// GetMetrics implements MetricsCollector.GetMetrics
func (n *NoOpMetricsCollector) GetMetrics(queueName string) WorkQueueMetrics {
	return WorkQueueMetrics{}
}

// ProcessWorkItem is a helper function for processing work queue items with
// proper error handling and retry logic following KCP patterns.
func ProcessWorkItem[T any](
	queue TypedClusterWorkQueue,
	processor func(req ClusterAwareRequest) error,
	maxRetries int,
	queueName string,
) bool {
	req, quit := queue.Get()
	if quit {
		return false
	}
	defer queue.Done(req)

	err := func() error {
		defer runtime.HandleCrash()
		return processor(req)
	}()

	if err == nil {
		// Success - forget the item
		queue.Forget(req)
		klog.V(4).InfoS("Successfully processed item",
			"queue", queueName,
			"workspace", req.Workspace,
			"key", req.Key)
		return true
	}

	// Handle error with exponential backoff
	numRequeues := queue.NumRequeues(req)
	if numRequeues < maxRetries {
		klog.V(4).InfoS("Error processing item, retrying",
			"queue", queueName,
			"workspace", req.Workspace,
			"key", req.Key,
			"error", err,
			"retries", numRequeues)
		
		queue.AddRateLimited(req)
		return true
	}

	// Too many retries, drop the item
	klog.ErrorS(err, "Dropping item after too many retries",
		"queue", queueName,
		"workspace", req.Workspace,
		"key", req.Key,
		"retries", numRequeues)
	
	queue.Forget(req)
	runtime.HandleError(err)
	return true
}