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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	kcpcache "github.com/kcp-dev/apimachinery/v2/pkg/cache"
	kcpthirdpartyinformers "github.com/kcp-dev/apimachinery/v2/third_party/informers"
	"github.com/kcp-dev/kcp/pkg/reconciler/committer"
	kcpinformers "github.com/kcp-dev/kcp/sdk/client/informers/externalversions"
	"github.com/kcp-dev/logicalcluster/v3"
)

// Request represents a typed reconciliation request that includes workspace context
type Request struct {
	// Key is the cluster-aware object key in format: cluster|namespace/name or cluster|name
	Key string

	// Workspace is the logical cluster workspace for this request
	Workspace logicalcluster.Name

	// Priority indicates the priority of this reconciliation request
	Priority int
}

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

	// HasSynced returns true if the controller's informers have synced
	HasSynced() bool

	// Name returns the controller name
	Name() string
}

// Reconciler defines the interface that specific controllers must implement
// following KCP patterns with support for the committer pattern.
type Reconciler interface {
	// Reconcile handles a single reconciliation request with proper error handling.
	// The key follows KCP's format: cluster|namespace/name or cluster|name for cluster-scoped resources.
	Reconcile(ctx context.Context, key string) error
}

// ReconcilerWithCommit extends Reconciler with committer pattern support for efficient patching.
type ReconcilerWithCommit[Sp any, St any] interface {
	Reconciler

	// GetCommitFunc returns a commit function for the specific resource type.
	// This enables efficient patching following KCP's committer pattern.
	GetCommitFunc() committer.CommitFunc[Sp, St]
}

// BaseControllerConfig contains configuration for a base controller instance.
type BaseControllerConfig struct {
	// Name is the controller name for logging and metrics
	Name string

	// Workspace is the logical cluster workspace for isolation
	Workspace logicalcluster.Name

	// ResyncPeriod controls how often the controller resyncs
	ResyncPeriod time.Duration

	// WorkerCount controls the number of worker goroutines
	WorkerCount int

	// Reconciler implements the business logic for the controller
	Reconciler Reconciler

	// Metrics provides metrics collection for the controller
	Metrics *ManagerMetrics

	// InformerFactory provides shared informers
	InformerFactory kcpinformers.SharedInformerFactory
}

// baseControllerImpl implements BaseController with common patterns
// used across all TMC controllers. It follows KCP architectural patterns
// including typed workqueues and proper workspace isolation.
type baseControllerImpl struct {
	// Configuration
	name         string
	workerCount  int
	resyncPeriod time.Duration
	workspace    logicalcluster.Name

	// Work queue management - uses KCP typed workqueue with Request type
	queue workqueue.TypedRateLimitingInterface[Request]

	// Informers and their HasSynced functions for startup synchronization
	informers      []cache.SharedIndexInformer
	hasSyncedFuncs []cache.InformerSynced

	// Business logic reconciler following KCP patterns
	reconciler Reconciler

	// Metrics and observability
	metrics *ManagerMetrics

	// Lifecycle management
	mu       sync.RWMutex
	started  bool
	stopping bool
	healthy  bool

	// Informer factory for workspace-aware informers
	informerFactory kcpinformers.SharedInformerFactory
}

// NewBaseController creates a new base controller with the given configuration.
// This provides the foundation for all TMC controllers with consistent patterns
// for work queue management, error handling, and observability following KCP patterns.
func NewBaseController(config *BaseControllerConfig) BaseController {
	if config == nil {
		panic("BaseControllerConfig cannot be nil")
	}

	if config.Workspace.Empty() {
		panic("Workspace cannot be empty - workspace isolation is required")
	}

	if config.Reconciler == nil {
		panic("Reconciler cannot be nil - business logic implementation required")
	}

	// Create KCP typed rate limiting queue with Request type
	queue := workqueue.NewTypedRateLimitingQueueWithConfig(
		workqueue.DefaultTypedControllerRateLimiter[Request](),
		workqueue.TypedRateLimitingQueueConfig[Request]{
			Name: config.Name,
		},
	)

	return &baseControllerImpl{
		name:            config.Name,
		workspace:       config.Workspace,
		workerCount:     config.WorkerCount,
		resyncPeriod:    config.ResyncPeriod,
		queue:           queue,
		reconciler:      config.Reconciler,
		metrics:         config.Metrics,
		informerFactory: config.InformerFactory,
		healthy:         true, // Start healthy
		informers:       make([]cache.SharedIndexInformer, 0),
		hasSyncedFuncs:  make([]cache.InformerSynced, 0),
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

	// Wait for informer caches to sync
	klog.InfoS("Waiting for informer caches to sync", "controller", c.name)
	if !cache.WaitForCacheSync(ctx.Done(), c.hasSyncedFuncs...) {
		return fmt.Errorf("failed to wait for caches to sync")
	}
	klog.InfoS("Informer caches synced", "controller", c.name)

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

// HasSynced implements BaseController.HasSynced
func (c *baseControllerImpl) HasSynced() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Wait for all registered informers to sync
	for _, hasSynced := range c.hasSyncedFuncs {
		if !hasSynced() {
			return false
		}
	}

	return true
}

// Name implements BaseController.Name
func (c *baseControllerImpl) Name() string {
	return c.name
}

// GetWorkspace returns the logical cluster workspace for this controller
func (c *baseControllerImpl) GetWorkspace() logicalcluster.Name {
	return c.workspace
}

// GetReconciler returns the reconciler implementation for this controller.
// This can be used to check for committer pattern support.
func (c *baseControllerImpl) GetReconciler() Reconciler {
	return c.reconciler
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
// using KCP typed workqueue patterns for type safety and better error handling.
func (c *baseControllerImpl) processNextWorkItem(ctx context.Context) bool {
	req, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(req)

	// Process the item with proper workspace context
	err := c.processItem(ctx, req)

	if err == nil {
		// Success - forget the item
		c.queue.Forget(req)
		c.metrics.reconcileTotal.WithLabelValues(c.name, "success").Inc()
		return true
	}

	// Handle error with typed queue
	c.handleError(err, req)
	return true
}

// processItem delegates to the configured reconciler for actual business logic.
// This follows KCP patterns by passing the key to the reconciler implementation.
func (c *baseControllerImpl) processItem(ctx context.Context, req Request) error {
	klog.V(6).InfoS("Processing item",
		"controller", c.name,
		"workspace", req.Workspace,
		"key", req.Key,
		"priority", req.Priority)

	// Delegate to the reconciler implementation with proper key
	return c.reconciler.Reconcile(ctx, req.Key)
}

// handleError handles errors from work item processing using KCP patterns
// for proper error tracking and exponential backoff with typed queue.
func (c *baseControllerImpl) handleError(err error, req Request) {
	// Record error metrics
	c.metrics.reconcileTotal.WithLabelValues(c.name, "error").Inc()

	// Implement exponential backoff with workspace context
	if c.queue.NumRequeues(req) < 10 {
		klog.V(4).InfoS("Error processing item, retrying",
			"controller", c.name,
			"workspace", req.Workspace,
			"key", req.Key,
			"error", err,
			"retries", c.queue.NumRequeues(req))

		c.queue.AddRateLimited(req)
		return
	}

	// Too many retries, drop the item
	klog.ErrorS(err, "Dropping item after too many retries",
		"controller", c.name,
		"workspace", req.Workspace,
		"key", req.Key,
		"retries", c.queue.NumRequeues(req))

	c.queue.Forget(req)
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
	req := Request{
		Key:       key,
		Workspace: c.workspace,
		Priority:  0,
	}
	c.queue.Add(req)
}

// EnqueueObject adds an object to the work queue using the KCP key function.
// This respects workspace isolation by including the logical cluster in the key.
func (c *baseControllerImpl) EnqueueObject(obj interface{}) {
	key, err := kcpcache.DeletionHandlingMetaClusterNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %+v: %v", obj, err))
		return
	}

	// Extract workspace from the object using KCP's logicalcluster.From pattern
	workspace := c.workspace
	if metaObj, ok := obj.(metav1.Object); ok {
		workspace = logicalcluster.From(metaObj)
	}

	req := Request{
		Key:       key,
		Workspace: workspace,
		Priority:  0,
	}
	c.queue.Add(req)
}

// EnqueueAfter adds a key to the work queue after the specified duration
func (c *baseControllerImpl) EnqueueAfter(key string, after time.Duration) {
	req := Request{
		Key:       key,
		Workspace: c.workspace,
		Priority:  0,
	}
	c.queue.AddAfter(req, after)
}

// GetQueue returns the controller's typed work queue (for advanced usage)
func (c *baseControllerImpl) GetQueue() workqueue.TypedRateLimitingInterface[Request] {
	return c.queue
}

// AddInformer adds an informer to the controller for proper sync checking.
// This is essential for ensuring the controller waits for cache synchronization.
func (c *baseControllerImpl) AddInformer(informer cache.SharedIndexInformer) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.informers = append(c.informers, informer)
	c.hasSyncedFuncs = append(c.hasSyncedFuncs, informer.HasSynced)
}

// AddClusterAwareInformer adds a KCP cluster-aware informer to the controller for proper sync checking.
// This handles ScopeableSharedIndexInformer types which embed SharedIndexInformer.
func (c *baseControllerImpl) AddClusterAwareInformer(informer kcpcache.ScopeableSharedIndexInformer) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// ScopeableSharedIndexInformer embeds SharedIndexInformer, so we can store it directly
	c.informers = append(c.informers, informer)
	c.hasSyncedFuncs = append(c.hasSyncedFuncs, informer.HasSynced)
}

// NewClusterAwareInformer creates a cluster-aware informer using KCP patterns.
// This creates a proper KCP ScopeableSharedIndexInformer for workspace isolation.
func (c *baseControllerImpl) NewClusterAwareInformer(
	lw cache.ListerWatcher,
	objType runtime.Object,
	resyncPeriod time.Duration,
	indexers cache.Indexers,
) kcpcache.ScopeableSharedIndexInformer {
	// Ensure cluster indexer is present for proper KCP cluster awareness
	if indexers == nil {
		indexers = cache.Indexers{}
	}
	indexers[kcpcache.ClusterIndexName] = kcpcache.ClusterIndexFunc

	// Create KCP cluster-aware informer using third-party informers
	informer := kcpthirdpartyinformers.NewSharedIndexInformer(
		lw,
		objType,
		resyncPeriod,
		indexers,
	)

	// Automatically register this informer for sync checking
	c.AddClusterAwareInformer(informer)

	return informer
}

// QueueKeyFor generates a cluster-aware queue key for the given object.
// This is the standard way to create keys that respect workspace boundaries.
func QueueKeyFor(obj interface{}) (Request, error) {
	key, err := kcpcache.DeletionHandlingMetaClusterNamespaceKeyFunc(obj)
	if err != nil {
		return Request{}, fmt.Errorf("couldn't get key for object %+v: %v", obj, err)
	}

	// Extract workspace from object using KCP's logicalcluster.From pattern
	var workspace logicalcluster.Name
	if metaObj, ok := obj.(metav1.Object); ok {
		workspace = logicalcluster.From(metaObj)
	} else {
		workspace = logicalcluster.Name("")
	}

	return Request{
		Key:       key,
		Workspace: workspace,
		Priority:  0,
	}, nil
}
