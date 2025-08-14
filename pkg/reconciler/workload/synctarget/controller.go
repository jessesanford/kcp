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

	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

const (
	// ControllerName is the name of this controller
	ControllerName = "synctarget-deployment"

	// WorkerCount is the default number of workers
	WorkerCount = 2

	// SyncTargetFinalizer is the finalizer we add to SyncTargets
	SyncTargetFinalizer = "workload.kcp.io/synctarget-deployment"
)

// Controller manages SyncTarget resources and their associated syncer deployments
type Controller struct {
	queue workqueue.RateLimitingInterface

	kubeClient kubernetes.Interface

	syncTargetSynced cache.InformerSynced

	// Abstractions for deployment and status management
	deploymentManager DeploymentManager
	statusUpdater     StatusUpdater
}

// NewController creates a new SyncTarget controller with deployment management
func NewController(
	kubeClient kubernetes.Interface,
	syncTargetInformer cache.SharedIndexInformer,
	deploymentManager DeploymentManager,
	statusUpdater StatusUpdater,
) (*Controller, error) {
	c := &Controller{
		queue: workqueue.NewNamedRateLimitingQueue(
			workqueue.DefaultControllerRateLimiter(),
			ControllerName,
		),

		kubeClient: kubeClient,

		syncTargetSynced: syncTargetInformer.HasSynced,

		deploymentManager: deploymentManager,
		statusUpdater:     statusUpdater,
	}

	// Set up event handlers
	syncTargetInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.enqueue,
		UpdateFunc: func(old, new interface{}) {
			c.enqueue(new)
		},
		DeleteFunc: c.enqueue,
	})

	// Add indexes for efficient lookups
	if err := AddIndexes(syncTargetInformer); err != nil {
		return nil, fmt.Errorf("failed to add indexes: %w", err)
	}

	return c, nil
}

// enqueue adds a SyncTarget to the work queue
func (c *Controller) enqueue(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}

	klog.V(4).Infof("Enqueuing SyncTarget %s", key)
	c.queue.Add(key)
}

// Start begins processing items from the work queue
func (c *Controller) Start(ctx context.Context) {
	defer runtime.HandleCrash()
	defer c.queue.ShutDown()

	klog.Info("Starting SyncTarget deployment controller")
	defer klog.Info("Shutting down SyncTarget deployment controller")

	if !cache.WaitForCacheSync(ctx.Done(), c.syncTargetSynced) {
		runtime.HandleError(fmt.Errorf("failed to sync caches"))
		return
	}

	klog.Info("Caches synced, starting workers")

	for i := 0; i < WorkerCount; i++ {
		go wait.Until(c.runWorker, time.Second, ctx.Done())
	}

	<-ctx.Done()
}

// runWorker processes items from the queue
func (c *Controller) runWorker() {
	for c.processNextItem() {
	}
}

// processNextItem handles a single item from the queue
func (c *Controller) processNextItem() bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)

	err := c.reconcile(key.(string))
	if err == nil {
		c.queue.Forget(key)
		return true
	}

	runtime.HandleError(fmt.Errorf("error reconciling %v: %v", key, err))
	c.queue.AddRateLimited(key)

	return true
}

// reconcile processes a single SyncTarget - delegating to abstractions
func (c *Controller) reconcile(key string) error {
	klog.V(4).Infof("Reconciling SyncTarget %s", key)

	// This will be implemented in PR3c with full reconciliation logic
	// For now, just return nil to allow compilation
	return nil
}