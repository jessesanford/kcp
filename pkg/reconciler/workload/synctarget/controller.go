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
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
	kcpinformers "github.com/kcp-dev/kcp/sdk/client/informers/externalversions/cluster"
	workloadv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/workload/v1alpha1"
	workloadlisters "github.com/kcp-dev/kcp/sdk/client/listers/workload/v1alpha1/cluster"
)

// Controller manages SyncTarget resources and their associated syncer deployments
type Controller struct {
	queue workqueue.RateLimitingInterface

	kcpClient      kcpclientset.ClusterInterface
	physicalClient kubernetes.Interface

	syncTargetLister workloadlisters.SyncTargetClusterLister
	syncTargetSynced cache.InformerSynced

	deploymentMgr *DeploymentManager
	finalizerMgr  *FinalizerManager
}

// NewController creates a new SyncTarget controller with deployment management
func NewController(
	kcpClient kcpclientset.ClusterInterface,
	physicalClient kubernetes.Interface,
	informerFactory kcpinformers.SharedInformerFactory,
) (*Controller, error) {

	// Get the SyncTarget informer
	syncTargetInformer := informerFactory.Workload().V1alpha1().SyncTargets()

	// Create deployment manager
	deploymentMgr := NewDeploymentManager(physicalClient)

	// Create finalizer manager
	finalizerMgr := NewFinalizerManager(kcpClient, physicalClient, deploymentMgr)

	c := &Controller{
		queue: workqueue.NewNamedRateLimitingQueue(
			workqueue.DefaultControllerRateLimiter(),
			ControllerName,
		),

		kcpClient:      kcpClient,
		physicalClient: physicalClient,

		syncTargetLister: syncTargetInformer.Lister(),
		syncTargetSynced: syncTargetInformer.Informer().HasSynced,

		deploymentMgr: deploymentMgr,
		finalizerMgr:  finalizerMgr,
	}

	// Set up event handlers
	syncTargetInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.enqueue,
		UpdateFunc: func(old, new interface{}) {
			c.enqueue(new)
		},
		DeleteFunc: c.enqueue,
	})

	return c, nil
}

// enqueue adds a SyncTarget to the work queue
func (c *Controller) enqueue(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}

	klog.V(4).Infof("Enqueuing SyncTarget %s", key)
	c.queue.Add(key)
}

// Start begins the controller
func (c *Controller) Start(ctx context.Context, workers int) {
	defer runtime.HandleCrash()
	defer c.queue.ShutDown()

	klog.Info("Starting SyncTarget controller")
	defer klog.Info("Shutting down SyncTarget controller")

	if !cache.WaitForCacheSync(ctx.Done(), c.syncTargetSynced) {
		runtime.HandleError(fmt.Errorf("failed to sync caches"))
		return
	}

	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, c.worker, time.Second)
	}

	<-ctx.Done()
}

// worker processes items from the queue
func (c *Controller) worker(ctx context.Context) {
	for c.processNextItem(ctx) {
	}
}

// processNextItem handles one item from the queue
func (c *Controller) processNextItem(ctx context.Context) bool {
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

// reconcile processes a single SyncTarget
func (c *Controller) reconcile(ctx context.Context, key string) error {
	klog.V(2).Infof("Reconciling SyncTarget: %s", key)

	clusterName, namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return fmt.Errorf("invalid key %q: %w", key, err)
	}

	cluster := logicalcluster.Name(clusterName).Path()

	// Get the SyncTarget
	syncTarget, err := c.syncTargetLister.Cluster(cluster).Get(name)
	if err != nil {
		klog.V(2).Infof("SyncTarget %s was deleted", key)
		return nil
	}

	klog.V(4).Infof("Processing SyncTarget %s/%s in cluster %s", namespace, name, cluster.String())

	// Handle deletion
	if !syncTarget.DeletionTimestamp.IsZero() {
		return c.finalizerMgr.HandleDeletion(ctx, cluster, syncTarget)
	}

	// Ensure finalizer
	if err := c.finalizerMgr.EnsureFinalizer(ctx, cluster, syncTarget); err != nil {
		return fmt.Errorf("failed to ensure finalizer: %w", err)
	}

	// Ensure deployment
	if err := c.deploymentMgr.EnsureDeployment(ctx, cluster, syncTarget); err != nil {
		return fmt.Errorf("failed to ensure deployment: %w", err)
	}

	return nil
}

// StartWithDefaultWorkers starts the controller with the default number of workers
func (c *Controller) StartWithDefaultWorkers(ctx context.Context) {
	c.Start(ctx, WorkerCount)
}