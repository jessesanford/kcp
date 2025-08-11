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

package placement

import (
	"context"
	"fmt"
	"time"

	"github.com/kcp-dev/logicalcluster/v3"

	kcpcache "github.com/kcp-dev/apimachinery/v2/pkg/cache"
	kcpfeatures "github.com/kcp-dev/kcp/pkg/features"
	"github.com/kcp-dev/kcp/pkg/logging"
	"github.com/kcp-dev/kcp/pkg/reconciler/committer"
	workloadv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/workload/v1alpha1"
	kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
	workloadclientv1alpha1 "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/typed/workload/v1alpha1"
	workloadinformers "github.com/kcp-dev/kcp/sdk/client/informers/externalversions/workload/v1alpha1"
	workloadlisters "github.com/kcp-dev/kcp/sdk/client/listers/workload/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

const (
	// ControllerName defines the name of the placement controller.
	ControllerName = "kcp-workload-placement"

	// DefaultResyncPeriod is the default time period for resyncing placement resources.
	DefaultResyncPeriod = 10 * time.Hour
)

// PlacementQueueKey represents a typed workqueue key for placement resources.
type PlacementQueueKey struct {
	ClusterName logicalcluster.Name
	Namespace   string
	Name        string
}

// String returns a string representation of the queue key.
func (k PlacementQueueKey) String() string {
	if k.Namespace != "" {
		return fmt.Sprintf("%s/%s/%s", k.ClusterName, k.Namespace, k.Name)
	}
	return fmt.Sprintf("%s/%s", k.ClusterName, k.Name)
}

// placementControllerConfig contains the configuration for the placement controller.
type placementControllerConfig struct {
	// kcpClusterClient provides access to KCP cluster-aware clients.
	kcpClusterClient kcpclientset.ClusterInterface

	// placementInformer provides informer access to Placement resources.
	placementInformer workloadinformers.PlacementClusterInformer

	// locationInformer provides informer access to Location resources.
	locationInformer workloadinformers.LocationClusterInformer
}

// placementController reconciles Placement resources within KCP.
// It implements the placement decision engine that selects appropriate
// clusters for workload placement based on placement specifications.
type placementController struct {
	queue workqueue.TypedRateLimitingInterface[PlacementQueueKey]

	// Client and listers for KCP resources
	kcpClusterClient kcpclientset.ClusterInterface

	// Resource listers
	placementLister workloadlisters.PlacementClusterLister
	placementSynced cache.InformerSynced

	locationLister workloadlisters.LocationClusterLister
	locationSynced cache.InformerSynced

	// Committer handles batch status updates with proper resource management
	commit committer.CommitFunc[workloadv1alpha1.PlacementSpec, workloadv1alpha1.PlacementStatus]
}

// newPlacementController creates a new placement controller instance.
// It configures informers, listers, and event handlers for processing
// placement decisions within the KCP TMC system.
//
// Parameters:
//   - config: Configuration containing clients and informers
//
// Returns:
//   - *placementController: Configured controller ready to start
//   - error: Configuration or setup error
func newPlacementController(config placementControllerConfig) (*placementController, error) {
	logger := klog.Background().WithValues("controller", ControllerName)

	queue := workqueue.NewTypedRateLimitingQueueWithConfig(
		workqueue.DefaultTypedControllerRateLimiter[PlacementQueueKey](),
		workqueue.TypedRateLimitingQueueConfig[PlacementQueueKey]{
			Name: ControllerName,
		},
	)

	c := &placementController{
		queue: queue,

		kcpClusterClient: config.kcpClusterClient,

		placementLister: config.placementInformer.Lister(),
		placementSynced: config.placementInformer.Informer().HasSynced,

		locationLister: config.locationInformer.Lister(),
		locationSynced: config.locationInformer.Informer().HasSynced,

		commit: committer.NewCommitter[
			*workloadv1alpha1.Placement,
			workloadclientv1alpha1.PlacementInterface,
			workloadv1alpha1.PlacementSpec,
			workloadv1alpha1.PlacementStatus,
		](config.kcpClusterClient.WorkloadV1alpha1().Placements()),
	}

	// Configure placement informer event handlers
	_, err := config.placementInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.enqueuePlacement(logger, obj, "add")
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldPlacement, ok := oldObj.(*workloadv1alpha1.Placement)
			if !ok {
				runtime.HandleError(fmt.Errorf("expected Placement, got %T", oldObj))
				return
			}
			newPlacement, ok := newObj.(*workloadv1alpha1.Placement)
			if !ok {
				runtime.HandleError(fmt.Errorf("expected Placement, got %T", newObj))
				return
			}

			// Only enqueue if spec changed or status needs updating
			if !equality.Semantic.DeepEqual(oldPlacement.Spec, newPlacement.Spec) ||
				oldPlacement.Generation != newPlacement.Status.ObservedGeneration {
				c.enqueuePlacement(logger, newObj, "update")
			}
		},
		DeleteFunc: func(obj interface{}) {
			c.enqueuePlacement(logger, obj, "delete")
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to add placement event handler: %w", err)
	}

	// Configure location informer event handlers
	// Location changes can affect placement decisions
	_, err = config.locationInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.handleLocationChange(logger, obj, "add")
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			c.handleLocationChange(logger, newObj, "update")
		},
		DeleteFunc: func(obj interface{}) {
			c.handleLocationChange(logger, obj, "delete")
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to add location event handler: %w", err)
	}

	return c, nil
}

// enqueuePlacement adds a placement resource to the work queue for processing.
func (c *placementController) enqueuePlacement(logger klog.Logger, obj interface{}, action string) {
	placement, ok := obj.(*workloadv1alpha1.Placement)
	if !ok {
		// Handle deletion state
		if deleted, ok := obj.(cache.DeletedFinalStateUnknown); ok {
			placement, ok = deleted.Obj.(*workloadv1alpha1.Placement)
			if !ok {
				runtime.HandleError(fmt.Errorf("expected Placement, got %T", deleted.Obj))
				return
			}
		} else {
			runtime.HandleError(fmt.Errorf("expected Placement, got %T", obj))
			return
		}
	}

	key := PlacementQueueKey{
		ClusterName: logicalcluster.From(placement),
		Namespace:   placement.Namespace,
		Name:        placement.Name,
	}

	logger.V(4).Info("enqueueing placement", "key", key.String(), "action", action)
	c.queue.Add(key)
}

// handleLocationChange processes location resource changes that may affect placement decisions.
func (c *placementController) handleLocationChange(logger klog.Logger, obj interface{}, action string) {
	location, ok := obj.(*workloadv1alpha1.Location)
	if !ok {
		// Handle deletion state
		if deleted, ok := obj.(cache.DeletedFinalStateUnknown); ok {
			location, ok = deleted.Obj.(*workloadv1alpha1.Location)
			if !ok {
				runtime.HandleError(fmt.Errorf("expected Location, got %T", deleted.Obj))
				return
			}
		} else {
			runtime.HandleError(fmt.Errorf("expected Location, got %T", obj))
			return
		}
	}

	logger.V(4).Info("location changed, re-evaluating placements", "location", location.Name, "action", action)

	// Find all placements that might be affected by this location change
	clusterName := logicalcluster.From(location)
	placements, err := c.placementLister.Cluster(clusterName).List(labels.Everything())
	if err != nil {
		runtime.HandleError(fmt.Errorf("failed to list placements for location change: %w", err))
		return
	}

	// Enqueue affected placements for re-processing
	for _, placement := range placements {
		key := PlacementQueueKey{
			ClusterName: logicalcluster.From(placement),
			Namespace:   placement.Namespace,
			Name:        placement.Name,
		}
		logger.V(5).Info("enqueueing placement due to location change", "placement", placement.Name, "key", key.String())
		c.queue.Add(key)
	}
}

// Start begins the controller's processing loop.
// It waits for informer caches to sync before starting workers.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control
//   - workers: Number of worker goroutines to start
func (c *placementController) Start(ctx context.Context, workers int) {
	defer runtime.HandleCrash()
	defer c.queue.ShutDown()

	logger := logging.WithReconciler(klog.FromContext(ctx), ControllerName)
	logger.Info("starting controller")
	defer logger.Info("shutting down controller")

	// Wait for informer caches to sync
	if !cache.WaitForNamedCacheSync(ControllerName, ctx.Done(), c.placementSynced, c.locationSynced) {
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
func (c *placementController) startWorker(ctx context.Context) {
	defer runtime.HandleCrash()

	logger := logging.WithReconciler(klog.FromContext(ctx), ControllerName)
	for c.processNextWorkItem(ctx, logger) {
	}
}

// processNextWorkItem retrieves and processes the next item from the work queue.
func (c *placementController) processNextWorkItem(ctx context.Context, logger klog.Logger) bool {
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
func (c *placementController) processWorkItem(ctx context.Context, key PlacementQueueKey) error {
	logger := logging.WithReconciler(klog.FromContext(ctx), ControllerName).WithValues("key", key.String())

	// Use the structured key directly
	clusterName := key.ClusterName
	name := key.Name

	logger = logger.WithValues("cluster", clusterName, "placement", name)

	// Get the placement resource with proper cluster scoping
	placement, err := c.placementLister.Cluster(clusterName).Get(name)
	if errors.IsNotFound(err) {
		logger.V(2).Info("placement has been deleted")
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to get placement from cluster %s: %w", clusterName, err)
	}

	// Verify placement belongs to the expected logical cluster for security
	actualCluster := logicalcluster.From(placement)
	if actualCluster != clusterName {
		return fmt.Errorf("placement cluster mismatch: expected %s, got %s", clusterName, actualCluster)
	}

	// Create a deep copy for reconciliation to avoid mutation of cached object
	placementCopy := placement.DeepCopy()

	// Delegate to reconciler with proper cluster context
	if err := c.reconcile(ctx, placementCopy); err != nil {
		return fmt.Errorf("reconciliation failed for placement %s in cluster %s: %w", name, clusterName, err)
	}

	return nil
}

// NewController creates a new TMC placement controller following KCP patterns.
// It integrates with the workload API system and maintains logical cluster isolation.
// The controller implements the placement decision engine for KCP's TMC system.
//
// Parameters:
//   - kcpClusterClient: Cluster-aware KCP client for API access
//   - placementInformer: Shared informer for Placement resources
//   - locationInformer: Shared informer for Location resources
//
// Returns:
//   - Interface: Controller interface ready to start
//   - error: Configuration or setup error
func NewController(
	kcpClusterClient kcpclientset.ClusterInterface,
	placementInformer workloadinformers.PlacementClusterInformer,
	locationInformer workloadinformers.LocationClusterInformer,
) (Interface, error) {
	config := placementControllerConfig{
		kcpClusterClient:  kcpClusterClient,
		placementInformer: placementInformer,
		locationInformer:  locationInformer,
	}

	return newPlacementController(config)
}

// Interface defines the contract for the placement controller.
type Interface interface {
	// Start begins the controller processing loop with the specified number of workers.
	Start(ctx context.Context, workers int)
}