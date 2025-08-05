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

package controller

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	kcpcache "github.com/kcp-dev/apimachinery/v2/pkg/cache"
)

const (
	// TMCControllerName is the base name for TMC controllers
	TMCControllerName = "tmc-controller"
)

// SyncHandler defines the function signature for handling resource reconciliation.
// It receives a context and a key (in the format cluster/namespace/name) and
// returns an error if reconciliation fails.
type SyncHandler func(ctx context.Context, key string) error

// HealthChecker defines the function signature for performing health checks.
// It receives a context and returns true if healthy, false otherwise, plus any error.
type HealthChecker func(ctx context.Context) (bool, error)

// TMCController provides a foundation for TMC controllers that manage
// resources following KCP patterns for workspace-aware operations.
// This serves as the base controller infrastructure that will be extended
// with specific TMC functionality in future PRs.
type TMCController struct {
	// name is the controller name for logging and metrics
	name string

	// queue holds work items for processing
	queue workqueue.RateLimitingInterface

	// healthCheckInterval defines how often to perform health checks
	healthCheckInterval time.Duration

	// informer provides event notifications for watched resources
	informer cache.SharedIndexInformer

	// syncHandler handles the reconciliation of individual resources
	syncHandler SyncHandler

	// healthChecker performs periodic health checks (optional)
	healthChecker HealthChecker
}

// TMCControllerOptions contains configuration options for creating a TMC controller.
type TMCControllerOptions struct {
	// Name is the controller name used for logging and metrics
	Name string

	// Informer provides event notifications for the resources to watch
	Informer cache.SharedIndexInformer

	// SyncHandler handles reconciliation of individual resources
	SyncHandler SyncHandler

	// HealthChecker performs periodic health checks (optional)
	HealthChecker HealthChecker

	// HealthCheckInterval defines how often to perform health checks
	HealthCheckInterval time.Duration
}

// NewTMCController creates a new TMC controller foundation following KCP patterns.
// This provides the core infrastructure that specific TMC controllers can build upon.
//
// Parameters:
//   - opts: Configuration options for the controller
//
// Returns:
//   - *TMCController: Configured controller ready to start
//   - error: Configuration or setup error
func NewTMCController(opts TMCControllerOptions) (*TMCController, error) {
	if opts.Name == "" {
		return nil, fmt.Errorf("controller name is required")
	}

	if opts.Informer == nil {
		return nil, fmt.Errorf("informer is required")
	}

	if opts.SyncHandler == nil {
		return nil, fmt.Errorf("sync handler is required")
	}

	if opts.HealthCheckInterval <= 0 {
		opts.HealthCheckInterval = 30 * time.Second
	}

	c := &TMCController{
		name:                opts.Name,
		queue:               workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), opts.Name),
		healthCheckInterval: opts.HealthCheckInterval,
		informer:            opts.Informer,
		syncHandler:         opts.SyncHandler,
		healthChecker:       opts.HealthChecker,
	}

	// Set up event handlers for the informer
	opts.Informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.enqueue(obj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			c.enqueue(newObj)
		},
		DeleteFunc: func(obj interface{}) {
			c.enqueue(obj)
		},
	})

	return c, nil
}

// Start begins the controller's control loop with the specified number of workers.
// It blocks until the context is cancelled.
func (c *TMCController) Start(ctx context.Context, workers int) {
	defer runtime.HandleCrash()
	defer c.queue.ShutDown()

	logger := klog.FromContext(ctx).WithName(c.name)
	logger.Info("Starting TMC controller", "workers", workers)

	// Wait for informer cache to sync
	logger.Info("Waiting for informer cache to sync")
	if !cache.WaitForCacheSync(ctx.Done(), c.informer.HasSynced) {
		logger.Error(fmt.Errorf("failed to sync informer cache"), "Failed to sync")
		return
	}
	logger.Info("Informer cache synced")

	// Start workers
	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, c.runWorker, time.Second)
	}

	// Start periodic health checks if health checker is provided
	if c.healthChecker != nil {
		go wait.UntilWithContext(ctx, c.runHealthChecks, c.healthCheckInterval)
	}

	logger.Info("TMC controller started")
	<-ctx.Done()
	logger.Info("TMC controller stopping")
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the workqueue.
func (c *TMCController) runWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

// processNextWorkItem reads a single work item off the workqueue and
// attempts to process it by calling the syncHandler.
func (c *TMCController) processNextWorkItem(ctx context.Context) bool {
	obj, shutdown := c.queue.Get()
	if shutdown {
		return false
	}

	defer c.queue.Done(obj)

	key, ok := obj.(string)
	if !ok {
		c.queue.Forget(obj)
		runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
		return true
	}

	if err := c.syncHandler(ctx, key); err != nil {
		c.queue.AddRateLimited(key)
		runtime.HandleError(fmt.Errorf("error syncing %q: %w", key, err))
		return true
	}

	c.queue.Forget(obj)
	return true
}

// runHealthChecks periodically runs health checks if a health checker is configured.
func (c *TMCController) runHealthChecks(ctx context.Context) {
	logger := klog.FromContext(ctx).WithName(c.name + "-health")
	
	healthy, err := c.healthChecker(ctx)
	if err != nil {
		logger.Error(err, "Health check failed")
		return
	}

	if healthy {
		logger.V(4).Info("Health check passed")
	} else {
		logger.Info("Health check failed - system is unhealthy")
	}
}

// enqueue adds an object to the controller's work queue.
func (c *TMCController) enqueue(obj interface{}) {
	key, err := kcpcache.MetaClusterNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(fmt.Errorf("failed to enqueue object %T: %w", obj, err))
		return
	}
	c.queue.Add(key)
}

// GetName returns the controller name.
func (c *TMCController) GetName() string {
	return c.name
}

// GetQueueLength returns the current length of the work queue.
func (c *TMCController) GetQueueLength() int {
	return c.queue.Len()
}