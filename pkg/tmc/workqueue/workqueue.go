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
	"context"
	"fmt"
	"time"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"
)

// TMCWorkqueue wraps the standard Kubernetes workqueue with TMC-specific functionality
// including workspace-aware key handling and observability.
type TMCWorkqueue interface {
	workqueue.RateLimitingInterface
	
	// AddWorkspace adds an item to the workqueue with workspace context
	AddWorkspace(workspace logicalcluster.Name, name string)
	
	// GetWithWorkspace gets an item from the workqueue and parses workspace information
	GetWithWorkspace() (workspace logicalcluster.Name, name string, shutdown bool)
	
	// Name returns the name of this workqueue
	Name() string
	
	// Metrics returns queue metrics
	Metrics() TMCWorkqueueMetrics
}

// TMCWorkqueueMetrics provides observability into workqueue performance
type TMCWorkqueueMetrics struct {
	// Depth is the current depth of the workqueue
	Depth int
	
	// Adds is the total number of items added to the workqueue
	Adds int64
	
	// Duration is the time since the oldest item was added
	Duration time.Duration
	
	// UnfinishedWork is the number of items currently being processed
	UnfinishedWork int
}

// tmcWorkqueueImpl implements TMCWorkqueue
type tmcWorkqueueImpl struct {
	workqueue.RateLimitingInterface
	name string
}

// NewTMCWorkqueue creates a new TMC workqueue with rate limiting
func NewTMCWorkqueue(name string, rateLimiter workqueue.RateLimiter) TMCWorkqueue {
	if rateLimiter == nil {
		rateLimiter = workqueue.DefaultControllerRateLimiter()
	}
	
	return &tmcWorkqueueImpl{
		RateLimitingInterface: workqueue.NewNamedRateLimitingQueue(rateLimiter, name),
		name:                 name,
	}
}

// NewDefaultTMCWorkqueue creates a TMC workqueue with default rate limiting
func NewDefaultTMCWorkqueue(name string) TMCWorkqueue {
	return NewTMCWorkqueue(name, nil)
}

// AddWorkspace adds an item to the workqueue with workspace context
func (q *tmcWorkqueueImpl) AddWorkspace(workspace logicalcluster.Name, name string) {
	key := makeWorkspaceKey(workspace, name)
	q.Add(key)
}

// GetWithWorkspace gets an item from the workqueue and parses workspace information
func (q *tmcWorkqueueImpl) GetWithWorkspace() (logicalcluster.Name, string, bool) {
	obj, shutdown := q.Get()
	if shutdown {
		return "", "", true
	}
	
	key, ok := obj.(string)
	if !ok {
		utilruntime.HandleError(fmt.Errorf("expected string key, got %T", obj))
		return "", "", false
	}
	
	workspace, name, err := parseWorkspaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid workspace key %q: %v", key, err))
		q.Done(obj)
		return "", "", false
	}
	
	return workspace, name, false
}

// Name returns the name of this workqueue
func (q *tmcWorkqueueImpl) Name() string {
	return q.name
}

// Metrics returns queue metrics
func (q *tmcWorkqueueImpl) Metrics() TMCWorkqueueMetrics {
	return TMCWorkqueueMetrics{
		Depth:          q.Len(),
		UnfinishedWork: q.NumRequeues(""),
	}
}

// makeWorkspaceKey creates a workspace-aware key for TMC objects
func makeWorkspaceKey(workspace logicalcluster.Name, name string) string {
	return fmt.Sprintf("%s|%s", workspace, name)
}

// parseWorkspaceKey parses a workspace key into its components
func parseWorkspaceKey(key string) (logicalcluster.Name, string, error) {
	for i, r := range key {
		if r == '|' {
			workspace := logicalcluster.Name(key[:i])
			name := key[i+1:]
			if workspace == "" || name == "" {
				return "", "", fmt.Errorf("invalid key format")
			}
			return workspace, name, nil
		}
	}
	return "", "", fmt.Errorf("missing workspace delimiter")
}

// WorkerPool manages a pool of worker goroutines processing items from a TMC workqueue
type WorkerPool struct {
	name       string
	workqueue  TMCWorkqueue
	handler    WorkerHandler
	numWorkers int
	shutdown   chan struct{}
}

// WorkerHandler processes work items from the queue
type WorkerHandler interface {
	// ProcessWorkItem processes a single work item
	ProcessWorkItem(ctx context.Context, workspace logicalcluster.Name, name string) error
}

// WorkerHandlerFunc is an adapter to allow functions to be used as WorkerHandlers
type WorkerHandlerFunc func(ctx context.Context, workspace logicalcluster.Name, name string) error

// ProcessWorkItem implements WorkerHandler
func (f WorkerHandlerFunc) ProcessWorkItem(ctx context.Context, workspace logicalcluster.Name, name string) error {
	return f(ctx, workspace, name)
}

// NewWorkerPool creates a new worker pool for processing TMC workqueue items
func NewWorkerPool(name string, workqueue TMCWorkqueue, handler WorkerHandler, numWorkers int) *WorkerPool {
	if numWorkers <= 0 {
		numWorkers = 1
	}
	
	return &WorkerPool{
		name:       name,
		workqueue:  workqueue,
		handler:    handler,
		numWorkers: numWorkers,
		shutdown:   make(chan struct{}),
	}
}

// Start starts the worker pool
func (wp *WorkerPool) Start(ctx context.Context) error {
	klog.V(2).Infof("Starting %s worker pool with %d workers", wp.name, wp.numWorkers)
	
	for i := 0; i < wp.numWorkers; i++ {
		go wp.runWorker(ctx, i)
	}
	
	<-ctx.Done()
	close(wp.shutdown)
	
	// Wait briefly for workers to finish current items
	time.Sleep(time.Second)
	wp.workqueue.ShutDown()
	
	klog.V(2).Infof("Stopped %s worker pool", wp.name)
	return nil
}

// runWorker runs a single worker goroutine
func (wp *WorkerPool) runWorker(ctx context.Context, workerID int) {
	klog.V(4).Infof("Starting worker %d for %s", workerID, wp.name)
	defer klog.V(4).Infof("Stopping worker %d for %s", workerID, wp.name)
	
	for wp.processNextWorkItem(ctx) {
		select {
		case <-wp.shutdown:
			return
		default:
		}
	}
}

// processNextWorkItem processes a single item from the workqueue
func (wp *WorkerPool) processNextWorkItem(ctx context.Context) bool {
	workspace, name, quit := wp.workqueue.GetWithWorkspace()
	if quit {
		return false
	}
	defer wp.workqueue.Done(makeWorkspaceKey(workspace, name))
	
	err := wp.handler.ProcessWorkItem(ctx, workspace, name)
	wp.handleError(err, workspace, name)
	
	return true
}

// handleError handles errors from processing work items
func (wp *WorkerPool) handleError(err error, workspace logicalcluster.Name, name string) {
	if err == nil {
		// Item processed successfully, forget any previous failures
		wp.workqueue.Forget(makeWorkspaceKey(workspace, name))
		return
	}
	
	key := makeWorkspaceKey(workspace, name)
	
	if wp.workqueue.NumRequeues(key) < 5 {
		klog.V(2).Infof("Error processing %s: %v, retrying", key, err)
		wp.workqueue.AddRateLimited(key)
		return
	}
	
	// Too many retries, drop the item
	wp.workqueue.Forget(key)
	klog.Errorf("Dropping %s after too many retries: %v", key, err)
}

// IsShutdown returns true if the worker pool is shutting down
func (wp *WorkerPool) IsShutdown() bool {
	select {
	case <-wp.shutdown:
		return true
	default:
		return false
	}
}