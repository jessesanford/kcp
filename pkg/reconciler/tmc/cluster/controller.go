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

package cluster

import (
	"context"
	"fmt"
	"time"

	"github.com/kcp-dev/logicalcluster/v3"

	"github.com/kcp-dev/kcp/pkg/logging"
	"github.com/kcp-dev/kcp/pkg/reconciler/committer"
	tmcv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/tmc/v1alpha1"
	kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
	tmcclientv1alpha1 "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/typed/tmc/v1alpha1"
	tmcinformers "github.com/kcp-dev/kcp/sdk/client/informers/externalversions/tmc/v1alpha1"
	tmclisters "github.com/kcp-dev/kcp/sdk/client/listers/tmc/v1alpha1"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

const (
	// ControllerName defines the name of the cluster registration controller.
	ControllerName = "kcp-tmc-cluster-registration"

	// DefaultResyncPeriod is the default time period for resyncing cluster resources.
	DefaultResyncPeriod = 10 * time.Hour
)

// ClusterQueueKey represents a typed workqueue key for cluster registration resources.
type ClusterQueueKey struct {
	ClusterName logicalcluster.Name
	Name        string
}

// String returns a string representation of the queue key.
func (k ClusterQueueKey) String() string {
	return fmt.Sprintf("%s/%s", k.ClusterName, k.Name)
}

// clusterControllerConfig contains the configuration for the cluster registration controller.
type clusterControllerConfig struct {
	// kcpClusterClient provides access to KCP cluster-aware clients.
	kcpClusterClient kcpclientset.ClusterInterface

	// clusterRegistrationInformer provides informer access to ClusterRegistration resources.
	clusterRegistrationInformer tmcinformers.ClusterRegistrationClusterInformer
}

// clusterController reconciles ClusterRegistration resources within KCP.
// It implements the cluster lifecycle management for TMC workload placement
// including health monitoring, capacity tracking, and capability detection.
type clusterController struct {
	queue workqueue.TypedRateLimitingInterface[ClusterQueueKey]

	// Client and listers for KCP resources
	kcpClusterClient kcpclientset.ClusterInterface

	// Resource listers
	clusterRegistrationLister tmclisters.ClusterRegistrationClusterLister
	clusterRegistrationSynced cache.InformerSynced

	// Committer handles batch status updates with proper resource management
	commit committer.CommitFunc[tmcv1alpha1.ClusterRegistrationSpec, tmcv1alpha1.ClusterRegistrationStatus]
}

// newClusterController creates a new cluster registration controller instance.
// It configures informers, listers, and event handlers for processing
// cluster registration within the KCP TMC system.
//
// Parameters:
//   - config: Configuration containing clients and informers
//
// Returns:
//   - *clusterController: Configured controller ready to start
//   - error: Configuration or setup error
func newClusterController(config clusterControllerConfig) (*clusterController, error) {
	logger := klog.Background().WithValues("controller", ControllerName)

	queue := workqueue.NewTypedRateLimitingQueueWithConfig(
		workqueue.DefaultTypedControllerRateLimiter[ClusterQueueKey](),
		workqueue.TypedRateLimitingQueueConfig[ClusterQueueKey]{
			Name: ControllerName,
		},
	)

	c := &clusterController{
		queue: queue,

		kcpClusterClient: config.kcpClusterClient,

		clusterRegistrationLister: config.clusterRegistrationInformer.Lister(),
		clusterRegistrationSynced: config.clusterRegistrationInformer.Informer().HasSynced,

		commit: committer.NewCommitter[
			*tmcv1alpha1.ClusterRegistration,
			tmcclientv1alpha1.ClusterRegistrationInterface,
			tmcv1alpha1.ClusterRegistrationSpec,
			tmcv1alpha1.ClusterRegistrationStatus,
		](config.kcpClusterClient.TmcV1alpha1().ClusterRegistrations()),
	}

	// Configure cluster registration informer event handlers
	_, err := config.clusterRegistrationInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.enqueueClusterRegistration(logger, obj, "add")
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldCluster, ok := oldObj.(*tmcv1alpha1.ClusterRegistration)
			if !ok {
				runtime.HandleError(fmt.Errorf("expected ClusterRegistration, got %T", oldObj))
				return
			}
			newCluster, ok := newObj.(*tmcv1alpha1.ClusterRegistration)
			if !ok {
				runtime.HandleError(fmt.Errorf("expected ClusterRegistration, got %T", newObj))
				return
			}

			// Only enqueue if spec changed or status needs updating
			if !equality.Semantic.DeepEqual(oldCluster.Spec, newCluster.Spec) ||
				oldCluster.Generation != newCluster.Status.ObservedGeneration {
				c.enqueueClusterRegistration(logger, newObj, "update")
			}
		},
		DeleteFunc: func(obj interface{}) {
			c.enqueueClusterRegistration(logger, obj, "delete")
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to add cluster registration event handler: %w", err)
	}

	return c, nil
}

// enqueueClusterRegistration adds a cluster registration resource to the work queue for processing.
func (c *clusterController) enqueueClusterRegistration(logger klog.Logger, obj interface{}, action string) {
	cluster, ok := obj.(*tmcv1alpha1.ClusterRegistration)
	if !ok {
		// Handle deletion state
		if deleted, ok := obj.(cache.DeletedFinalStateUnknown); ok {
			cluster, ok = deleted.Obj.(*tmcv1alpha1.ClusterRegistration)
			if !ok {
				runtime.HandleError(fmt.Errorf("expected ClusterRegistration, got %T", deleted.Obj))
				return
			}
		} else {
			runtime.HandleError(fmt.Errorf("expected ClusterRegistration, got %T", obj))
			return
		}
	}

	key := ClusterQueueKey{
		ClusterName: logicalcluster.From(cluster),
		Name:        cluster.Name,
	}

	logger.V(4).Info("enqueueing cluster registration", "key", key.String(), "action", action)
	c.queue.Add(key)
}

// Start begins the controller's processing loop.
// It waits for informer caches to sync before starting workers.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control
//   - workers: Number of worker goroutines to start
func (c *clusterController) Start(ctx context.Context, workers int) {
	defer runtime.HandleCrash()
	defer c.queue.ShutDown()

	logger := logging.WithReconciler(klog.FromContext(ctx), ControllerName)
	logger.Info("starting controller")
	defer logger.Info("shutting down controller")

	// Wait for informer caches to sync
	if !cache.WaitForNamedCacheSync(ControllerName, ctx.Done(), c.clusterRegistrationSynced) {
		logger.Error(nil, "failed to wait for caches to sync")
		return
	}

	// Start worker goroutines
	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, c.startWorker, time.Second)
	}

	logger.Info("controller started", "workers", workers)
	<-ctx.Done()
}

// startWorker runs a worker thread that processes items from the work queue.
func (c *clusterController) startWorker(ctx context.Context) {
	defer runtime.HandleCrash()

	logger := logging.WithReconciler(klog.FromContext(ctx), ControllerName)
	for c.processNextWorkItem(ctx, logger) {
	}
}

// processNextWorkItem retrieves and processes the next item from the work queue.
func (c *clusterController) processNextWorkItem(ctx context.Context, logger klog.Logger) bool {
	// Get next item from queue
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)

	// Process the item
	err := c.processWorkItem(ctx, key)
	if err == nil {
		// Item processed successfully, remove from queue
		c.queue.Forget(key)
		return true
	}

	// Handle processing error
	runtime.HandleError(fmt.Errorf("failed to process key %q: %w", key.String(), err))

	// Add back to queue with rate limiting
	c.queue.AddRateLimited(key)

	return true
}

// processWorkItem processes a single work item identified by the given key.
func (c *clusterController) processWorkItem(ctx context.Context, key ClusterQueueKey) error {
	logger := logging.WithReconciler(klog.FromContext(ctx), ControllerName).WithValues("key", key.String())

	// Use the structured key directly
	clusterName := key.ClusterName
	name := key.Name

	logger = logger.WithValues("cluster", clusterName, "registration", name)

	// Get the cluster registration resource with proper cluster scoping
	clusterRegistration, err := c.clusterRegistrationLister.Cluster(clusterName).Get(name)
	if errors.IsNotFound(err) {
		logger.V(2).Info("cluster registration has been deleted")
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to get cluster registration from cluster %s: %w", clusterName, err)
	}

	// Verify cluster registration belongs to the expected logical cluster for security
	actualCluster := logicalcluster.From(clusterRegistration)
	if actualCluster != clusterName {
		return fmt.Errorf("cluster registration cluster mismatch: expected %s, got %s", clusterName, actualCluster)
	}

	// Create a deep copy for reconciliation to avoid mutation of cached object
	clusterRegistrationCopy := clusterRegistration.DeepCopy()

	// Delegate to reconciler with proper cluster context
	if err := c.reconcile(ctx, clusterRegistrationCopy); err != nil {
		return fmt.Errorf("reconciliation failed for cluster registration %s in cluster %s: %w", name, clusterName, err)
	}

	return nil
}

// NewController creates a new TMC cluster registration controller following KCP patterns.
// It integrates with the TMC API system and maintains logical cluster isolation.
// The controller implements cluster lifecycle management for KCP's TMC system.
//
// Parameters:
//   - kcpClusterClient: Cluster-aware KCP client for API access
//   - clusterRegistrationInformer: Shared informer for ClusterRegistration resources
//
// Returns:
//   - Interface: Controller interface ready to start
//   - error: Configuration or setup error
func NewController(
	kcpClusterClient kcpclientset.ClusterInterface,
	clusterRegistrationInformer tmcinformers.ClusterRegistrationClusterInformer,
) (Interface, error) {
	config := clusterControllerConfig{
		kcpClusterClient:            kcpClusterClient,
		clusterRegistrationInformer: clusterRegistrationInformer,
	}

	return newClusterController(config)
}

// Interface defines the contract for the cluster registration controller.
type Interface interface {
	// Start begins the controller processing loop with the specified number of workers.
	Start(ctx context.Context, workers int)
}