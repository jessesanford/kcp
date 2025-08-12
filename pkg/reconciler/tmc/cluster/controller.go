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

package cluster

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	kcpcache "github.com/kcp-dev/apimachinery/v2/pkg/cache"
	"github.com/kcp-dev/client-go/kubernetes"

	"github.com/kcp-dev/kcp/pkg/logging"
	kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
)

const (
	ControllerName = "tmc-cluster"
)

// ClusterRegistration is a placeholder interface for TMC ClusterRegistration type
// This will be replaced with actual TMC API types when they are available
type ClusterRegistration interface {
	GetName() string
	GetNamespace() string
	DeepCopy() ClusterRegistration
}

// ClusterRegistrationLister is a placeholder interface for cluster registration lister
type ClusterRegistrationLister interface {
	Get(name string) (ClusterRegistration, error)
}

// ClusterRegistrationInformer is a placeholder interface for cluster registration informer
type ClusterRegistrationInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() ClusterRegistrationLister
}

// NewController creates a new TMC cluster controller following KCP patterns.
// It integrates with the KCP cluster client and maintains workspace isolation.
//
// Parameters:
//   - kcpClusterClient: Cluster-aware KCP client for multi-tenant operations
//   - kubeInformerFactory: Standard Kubernetes informer factory
//   - tmcInformerFactory: TMC-specific informer factory (placeholder interface)
//
// Returns:
//   - *Controller: Configured controller ready to start
//   - error: Configuration or setup error
func NewController(
	kcpClusterClient kcpclientset.ClusterInterface,
	kubeClusterClient kubernetes.ClusterInterface,
	clusterInformer ClusterRegistrationInformer,
) (*Controller, error) {
	c := &Controller{
		queue: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[string](),
			workqueue.TypedRateLimitingQueueConfig[string]{
				Name: ControllerName,
			},
		),

		kcpClusterClient:  kcpClusterClient,
		kubeClusterClient: kubeClusterClient,

		clusterIndexer: clusterInformer.Informer().GetIndexer(),
		clusterLister:  clusterInformer.Lister(),
	}

	// Add event handlers for ClusterRegistration resources
	_, _ = clusterInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { c.enqueue(obj) },
		UpdateFunc: func(_, obj interface{}) { c.enqueue(obj) },
		DeleteFunc: func(obj interface{}) { c.enqueue(obj) },
	})

	return c, nil
}

// Controller manages ClusterRegistration resources with workspace isolation.
// It follows KCP controller patterns with proper cluster-aware client usage.
type Controller struct {
	queue workqueue.TypedRateLimitingInterface[string]

	kcpClusterClient  kcpclientset.ClusterInterface
	kubeClusterClient kubernetes.ClusterInterface

	clusterIndexer cache.Indexer
	clusterLister  ClusterRegistrationLister
}

// enqueue adds a ClusterRegistration resource to the work queue.
// It uses KCP's cluster-aware key generation for proper workspace isolation.
func (c *Controller) enqueue(obj interface{}) {
	key, err := kcpcache.MetaClusterNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
	logger := logging.WithQueueKey(logging.WithReconciler(klog.Background(), ControllerName), key)
	logger.V(4).Info("queueing ClusterRegistration")
	c.queue.Add(key)
}

// Start begins the controller's main reconciliation loop with the specified number of worker threads.
// It maintains workspace isolation and follows KCP controller lifecycle patterns.
func (c *Controller) Start(ctx context.Context, numThreads int) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	logger := logging.WithReconciler(klog.FromContext(ctx), ControllerName)
	ctx = klog.NewContext(ctx, logger)
	logger.Info("Starting controller")

	for i := 0; i < numThreads; i++ {
		go wait.UntilWithContext(ctx, c.startWorker, time.Second)
	}

	<-ctx.Done()
}

// startWorker runs a single worker thread that processes work items from the queue.
func (c *Controller) startWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

// processNextWorkItem processes a single work item from the queue.
// It handles errors and requeuing following KCP controller patterns.
func (c *Controller) processNextWorkItem(ctx context.Context) bool {
	// Wait until there is a new item in the working queue
	k, quit := c.queue.Get()
	if quit {
		return false
	}
	key := k

	logger := logging.WithQueueKey(klog.FromContext(ctx), key)
	ctx = klog.NewContext(ctx, logger)
	logger.V(4).Info("processing key")

	// No matter what, tell the queue we're done with this key, to unblock
	// other workers.
	defer c.queue.Done(key)

	if err := c.sync(ctx, key); err != nil {
		utilruntime.HandleError(fmt.Errorf("%q controller failed to sync %q, err: %w", ControllerName, key, err))
		c.queue.AddRateLimited(key)
		return true
	}

	c.queue.Forget(key)
	return true
}

// sync is the main reconciliation method that handles ClusterRegistration resources.
// It extracts the workspace-aware key and delegates to the process method.
func (c *Controller) sync(ctx context.Context, key string) error {
	_, _, name, err := kcpcache.SplitMetaClusterNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(err)
		return nil
	}

	logger := klog.FromContext(ctx).WithValues("cluster", name)
	ctx = klog.NewContext(ctx, logger)

	cluster, err := c.clusterLister.Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.V(2).Info("ClusterRegistration deleted before processing")
			return nil // object deleted before we handled it
		}
		return err
	}

	return c.process(ctx, cluster)
}

// process handles the main reconciliation logic for a ClusterRegistration resource.
// This is where cluster lifecycle management, health monitoring, and status updates occur.
//
// Parameters:
//   - ctx: Context with logging and workspace information
//   - cluster: The ClusterRegistration resource to process
//
// Returns:
//   - error: Any error encountered during processing
func (c *Controller) process(ctx context.Context, cluster ClusterRegistration) error {
	logger := klog.FromContext(ctx)
	logger.V(2).Info("processing ClusterRegistration", "cluster", cluster.GetName())

	// Create a working copy to avoid mutating the cached object
	cluster = cluster.DeepCopy()

	// TODO: Implement cluster lifecycle management
	// - Validate cluster configuration
	// - Establish connection to cluster API server
	// - Monitor cluster health and capacity
	// - Update cluster status and conditions
	// - Handle cluster registration and deregistration

	logger.V(3).Info("ClusterRegistration processed successfully", "cluster", cluster.GetName())
	return nil
}