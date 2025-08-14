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

package synctarget

import (
	"context"
	"fmt"
	"time"

	"github.com/kcp-dev/logicalcluster/v3"

	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

const (
	// ControllerName is the name of this controller
	ControllerName = "kcp-synctarget"

	// WorkerCount is the number of sync workers
	WorkerCount = 10
)

// ControllerFoundation provides the basic controller structure and patterns
// that can be extended when the workload API types become available.
type ControllerFoundation struct {
	queue workqueue.RateLimitingInterface

	// Generic informer interface that will be replaced
	// with concrete types when workload APIs are available
	informer cache.SharedIndexInformer
}

// NewControllerFoundation creates a new SyncTarget controller foundation
func NewControllerFoundation() *ControllerFoundation {
	return &ControllerFoundation{
		queue: workqueue.NewNamedRateLimitingQueue(
			workqueue.DefaultControllerRateLimiter(),
			ControllerName,
		),
	}
}

// SetupInformer configures the informer and event handlers
func (c *ControllerFoundation) SetupInformer(informer cache.SharedIndexInformer) {
	c.informer = informer

	// Set up event handlers
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.enqueue,
		UpdateFunc: func(old, new interface{}) {
			c.enqueue(new)
		},
		DeleteFunc: c.enqueue,
	})
}

// enqueue adds a resource to the work queue
func (c *ControllerFoundation) enqueue(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}

	klog.V(4).Infof("Enqueuing SyncTarget %s", key)
	c.queue.Add(key)
}

// Start begins the controller loops
func (c *ControllerFoundation) Start(ctx context.Context, workers int) {
	defer runtime.HandleCrash()
	defer c.queue.ShutDown()

	klog.Info("Starting SyncTarget controller foundation")
	defer klog.Info("Shutting down SyncTarget controller foundation")

	if c.informer != nil {
		if !cache.WaitForCacheSync(ctx.Done(), c.informer.HasSynced) {
			runtime.HandleError(fmt.Errorf("failed to sync caches"))
			return
		}
	}

	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, c.worker, time.Second)
	}

	<-ctx.Done()
}

// worker processes items from the queue
func (c *ControllerFoundation) worker(ctx context.Context) {
	for c.processNextItem(ctx) {
	}
}

// processNextItem handles one item from the queue
func (c *ControllerFoundation) processNextItem(ctx context.Context) bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)

	err := c.reconcile(ctx, key.(string))
	if err == nil {
		c.queue.Forget(key)
		return true
	}

	runtime.HandleError(fmt.Errorf("error reconciling %v: %v", key, err))
	c.queue.AddRateLimited(key)

	return true
}

// reconcile processes a single resource
// This is a placeholder implementation that will be overridden by the full Controller
func (c *ControllerFoundation) reconcile(ctx context.Context, key string) error {
	klog.V(2).Infof("Reconciling SyncTarget: %s", key)

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return fmt.Errorf("invalid key %q: %w", key, err)
	}

	klog.V(4).Infof("Processing SyncTarget %s/%s", namespace, name)

	// This is a basic placeholder - full reconciliation logic is in Controller.reconcile()
	return nil
}

// StartWithDefaultWorkers starts the controller with the default number of workers
func (c *ControllerFoundation) StartWithDefaultWorkers(ctx context.Context) {
	c.Start(ctx, WorkerCount)
}

// Key represents a logical cluster and resource
type Key struct {
	Cluster   logicalcluster.Path
	Namespace string
	Name      string
}