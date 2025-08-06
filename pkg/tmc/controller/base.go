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
	"sync"
	"time"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	kcpcache "github.com/kcp-dev/apimachinery/v2/pkg/cache"
	kcpinformers "github.com/kcp-dev/kcp/sdk/client/informers/externalversions"
)

// BaseController provides common controller patterns and functionality
// that can be shared across all TMC controllers. This includes work queue
// management, error handling, metrics collection, and graceful shutdown.
type BaseController interface {
	// Start starts the controller and blocks until the context is cancelled
	Start(ctx context.Context) error
	
	// Shutdown gracefully shuts down the controller
	Shutdown(ctx context.Context) error
	
	// IsHealthy returns true if the controller is healthy
	IsHealthy() bool
	
	// Name returns the controller name
	Name() string
}

// BaseControllerConfig contains configuration for a base controller instance.
type BaseControllerConfig struct {
	// Name is the controller name for logging and metrics
	Name string
	
	// ResyncPeriod controls how often the controller resyncs
	ResyncPeriod time.Duration
	
	// WorkerCount controls the number of worker goroutines
	WorkerCount int
	
	// Metrics provides metrics collection for the controller
	Metrics *ManagerMetrics
	
	// InformerFactory provides shared informers
	InformerFactory kcpinformers.SharedInformerFactory
}

// baseControllerImpl implements BaseController with common patterns
// used across all TMC controllers.
type baseControllerImpl struct {
	// Configuration
	name         string
	workerCount  int
	resyncPeriod time.Duration
	
	// Work queue management
	queue workqueue.RateLimitingInterface
	
	// Metrics and observability
	metrics *ManagerMetrics
	
	// Lifecycle management
	mu       sync.RWMutex
	started  bool
	stopping bool
	healthy  bool
	
	// Informer factory
	informerFactory kcpinformers.SharedInformerFactory
}

// NewBaseController creates a new base controller with the given configuration.
// This provides the foundation for all TMC controllers with consistent patterns
// for work queue management, error handling, and observability.
func NewBaseController(config *BaseControllerConfig) BaseController {
	if config == nil {
		panic("BaseControllerConfig cannot be nil")
	}
	
	// Create rate limiting queue
	queue := workqueue.NewNamedRateLimitingQueue(
		workqueue.DefaultControllerRateLimiter(),
		config.Name,
	)
	
	return &baseControllerImpl{
		name:            config.Name,
		workerCount:     config.WorkerCount,
		resyncPeriod:    config.ResyncPeriod,
		queue:           queue,
		metrics:         config.Metrics,
		informerFactory: config.InformerFactory,
		healthy:         true, // Start healthy
	}
}

// Start implements BaseController.Start
func (c *baseControllerImpl) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.started {
		c.mu.Unlock()
		return fmt.Errorf("controller %s already started", c.name)
	}
	c.started = true
	c.mu.Unlock()

	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	klog.InfoS("Starting controller", "controller", c.name)

	// Start metrics collection
	c.startMetricsCollection()

	// Start worker goroutines
	var wg sync.WaitGroup
	for i := 0; i < c.workerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			c.runWorker(ctx, workerID)
		}(i)
	}

	klog.InfoS("Controller started", "controller", c.name, "workers", c.workerCount)

	// Block until context is cancelled
	<-ctx.Done()
	
	klog.InfoS("Shutting down controller", "controller", c.name)
	
	// Mark as stopping
	c.mu.Lock()
	c.stopping = true
	c.mu.Unlock()

	// Wait for workers to finish
	wg.Wait()
	
	klog.InfoS("Controller stopped", "controller", c.name)
	return nil
}

// Shutdown implements BaseController.Shutdown
func (c *baseControllerImpl) Shutdown(ctx context.Context) error {
	c.mu.Lock()
	if !c.started || c.stopping {
		c.mu.Unlock()
		return nil
	}
	c.stopping = true
	c.mu.Unlock()

	klog.InfoS("Gracefully shutting down controller", "controller", c.name)
	
	// Shutdown the work queue to stop accepting new work
	c.queue.ShutDown()
	
	// The actual shutdown happens in Start() method when context is cancelled
	return nil
}

// IsHealthy implements BaseController.IsHealthy
func (c *baseControllerImpl) IsHealthy() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	if !c.started || c.stopping {
		return false
	}
	
	// Check queue depth as a health indicator
	queueLength := c.queue.Len()
	if queueLength > 1000 {
		klog.V(4).InfoS("Controller queue depth high", 
			"controller", c.name, 
			"depth", queueLength)
		return false
	}
	
	return c.healthy
}

// Name implements BaseController.Name
func (c *baseControllerImpl) Name() string {
	return c.name
}

// runWorker processes items from the work queue
func (c *baseControllerImpl) runWorker(ctx context.Context, workerID int) {
	klog.V(4).InfoS("Starting worker", "controller", c.name, "worker", workerID)
	defer klog.V(4).InfoS("Stopping worker", "controller", c.name, "worker", workerID)

	for c.processNextWorkItem(ctx) {
		// Check if we should stop
		select {
		case <-ctx.Done():
			return
		default:
		}
	}
}

// processNextWorkItem processes a single work item from the queue
func (c *baseControllerImpl) processNextWorkItem(ctx context.Context) bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)

	// Process the item (placeholder for actual reconciliation)
	err := c.processItem(ctx, key.(string))

	if err == nil {
		// Success - forget the item
		c.queue.Forget(key)
		c.metrics.reconcileTotal.WithLabelValues(c.name, "success").Inc()
		return true
	}

	// Handle error
	c.handleError(err, key)
	return true
}

// processItem is a placeholder for actual item processing
// This will be overridden by specific controller implementations
func (c *baseControllerImpl) processItem(ctx context.Context, key string) error {
	// This is the base controller, so just log the key
	klog.V(6).InfoS("Processing base controller item", 
		"controller", c.name, 
		"key", key)
	
	// Simulate some work
	time.Sleep(10 * time.Millisecond)
	return nil
}

// handleError handles errors from work item processing
func (c *baseControllerImpl) handleError(err error, key interface{}) {
	// Record error metrics
	c.metrics.reconcileTotal.WithLabelValues(c.name, "error").Inc()

	// Implement exponential backoff
	if c.queue.NumRequeues(key) < 10 {
		klog.V(4).InfoS("Error processing item, retrying", 
			"controller", c.name,
			"key", key, 
			"error", err,
			"retries", c.queue.NumRequeues(key))
		
		c.queue.AddRateLimited(key)
		return
	}

	// Too many retries, drop the item
	klog.ErrorS(err, "Dropping item after too many retries", 
		"controller", c.name,
		"key", key,
		"retries", c.queue.NumRequeues(key))
	
	c.queue.Forget(key)
	utilruntime.HandleError(err)
	
	// Mark controller as unhealthy if we're dropping items
	c.mu.Lock()
	c.healthy = false
	c.mu.Unlock()
	
	// Recover health after some time
	go func() {
		time.Sleep(30 * time.Second)
		c.mu.Lock()
		c.healthy = true
		c.mu.Unlock()
	}()
}

// startMetricsCollection starts collecting metrics for this controller
func (c *baseControllerImpl) startMetricsCollection() {
	// Simplified metrics collection - just mark as started
	// More detailed metrics will be added in later phases
}

// EnqueueKey adds a key to the controller's work queue
// This is a utility function for controllers that extend the base controller
func (c *baseControllerImpl) EnqueueKey(key string) {
	c.queue.Add(key)
}

// EnqueueObject adds an object to the work queue using the standard key function
func (c *baseControllerImpl) EnqueueObject(obj interface{}) {
	key, err := kcpcache.DeletionHandlingMetaClusterNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %+v: %v", obj, err))
		return
	}
	c.queue.Add(key)
}

// EnqueueAfter adds a key to the work queue after the specified duration
func (c *baseControllerImpl) EnqueueAfter(key string, after time.Duration) {
	c.queue.AddAfter(key, after)
}

// GetQueue returns the controller's work queue (for advanced usage)
func (c *baseControllerImpl) GetQueue() workqueue.RateLimitingInterface {
	return c.queue
}
