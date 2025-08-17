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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	kcpcache "github.com/kcp-dev/apimachinery/v2/pkg/cache"
	"github.com/kcp-dev/logicalcluster/v3"

	"github.com/kcp-dev/kcp/pkg/logging"
	"github.com/kcp-dev/kcp/pkg/reconciler/committer"
	workloadv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/workload/v1alpha1"
	kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
	workloadv1alpha1client "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/typed/workload/v1alpha1"
	workloadv1alpha1informers "github.com/kcp-dev/kcp/sdk/client/informers/externalversions/workload/v1alpha1"
	workloadv1alpha1listers "github.com/kcp-dev/kcp/sdk/client/listers/workload/v1alpha1"
)

const (
	ControllerName = "kcp-synctarget"

	// SyncTargetHeartbeatTimeout defines the maximum time allowed between heartbeats
	SyncTargetHeartbeatTimeout = 60 * time.Second

	// SyncTargetReconcileInterval defines the interval for regular status updates
	SyncTargetReconcileInterval = 30 * time.Second

	// SyncTargetFinalizer is added to SyncTarget resources to ensure proper cleanup
	SyncTargetFinalizer = "workload.kcp.io/synctarget"
)

// NewController creates a new SyncTarget controller for TMC cluster connectivity.
func NewController(
	kcpClusterClient kcpclientset.ClusterInterface,
	syncTargetInformer workloadv1alpha1informers.SyncTargetClusterInformer,
	virtualWorkspaceInformer workloadv1alpha1informers.VirtualWorkspaceClusterInformer,
) (*Controller, error) {
	c := &Controller{
		queue: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[string](),
			workqueue.TypedRateLimitingQueueConfig[string]{
				Name: ControllerName,
			},
		),
		kcpClusterClient:         kcpClusterClient,
		syncTargetIndexer:        syncTargetInformer.Informer().GetIndexer(),
		syncTargetLister:         syncTargetInformer.Lister(),
		virtualWorkspaceIndexer:  virtualWorkspaceInformer.Informer().GetIndexer(),
		virtualWorkspaceLister:   virtualWorkspaceInformer.Lister(),
		commit:                   committer.NewCommitter[*workloadv1alpha1.SyncTarget, workloadv1alpha1client.SyncTargetInterface, *workloadv1alpha1.SyncTargetSpec, *workloadv1alpha1.SyncTargetStatus](kcpClusterClient.WorkloadV1alpha1().SyncTargets()),
	}

	// Register event handlers for SyncTarget resources
	_, _ = syncTargetInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { c.enqueueSyncTarget(obj) },
		UpdateFunc: func(_, obj interface{}) { c.enqueueSyncTarget(obj) },
		DeleteFunc: func(obj interface{}) { c.enqueueSyncTarget(obj) },
	})

	// Register event handlers for VirtualWorkspace resources
	_, _ = virtualWorkspaceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { c.enqueueVirtualWorkspace(obj) },
		UpdateFunc: func(_, obj interface{}) { c.enqueueVirtualWorkspace(obj) },
		DeleteFunc: func(obj interface{}) { c.enqueueVirtualWorkspace(obj) },
	})

	return c, nil
}

type syncTargetResource = committer.Resource[*workloadv1alpha1.SyncTargetSpec, *workloadv1alpha1.SyncTargetStatus]

// Controller manages SyncTarget resources for cluster connectivity tracking.
type Controller struct {
	queue workqueue.TypedRateLimitingInterface[string]

	kcpClusterClient kcpclientset.ClusterInterface

	syncTargetIndexer cache.Indexer
	syncTargetLister  workloadv1alpha1listers.SyncTargetClusterLister

	virtualWorkspaceIndexer cache.Indexer
	virtualWorkspaceLister  workloadv1alpha1listers.VirtualWorkspaceClusterLister

	// commit creates a patch and submits it, if needed.
	commit func(ctx context.Context, old, new *syncTargetResource) error
}

// enqueueSyncTarget adds a SyncTarget resource to the work queue.
func (c *Controller) enqueueSyncTarget(obj interface{}) {
	key, err := kcpcache.DeletionHandlingMetaClusterNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
	logger := logging.WithQueueKey(logging.WithReconciler(klog.Background(), ControllerName), key)
	logger.V(4).Info("queueing SyncTarget")
	c.queue.Add(key)
}

// enqueueVirtualWorkspace processes VirtualWorkspace changes and enqueues related SyncTargets.
func (c *Controller) enqueueVirtualWorkspace(obj interface{}) {
	key, err := kcpcache.DeletionHandlingMetaClusterNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}

	clusterName, _, _, err := kcpcache.SplitMetaClusterNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid key %q: %w", key, err))
		return
	}

	// Find all SyncTargets in the same cluster as the VirtualWorkspace
	syncTargets, err := c.syncTargetLister.Cluster(clusterName).List(metav1.ListOptions{}.LabelSelector)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("failed to list SyncTargets for VirtualWorkspace %q: %w", key, err))
		return
	}

	logger := logging.WithQueueKey(logging.WithReconciler(klog.Background(), ControllerName), key)
	logger.V(4).Info("queueing SyncTargets for VirtualWorkspace", "count", len(syncTargets))

	// Enqueue each related SyncTarget for reconciliation
	for _, syncTarget := range syncTargets {
		c.enqueueSyncTarget(syncTarget)
	}
}

// Start begins the controller's main processing loop.
func (c *Controller) Start(ctx context.Context, numThreads int) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	logger := logging.WithReconciler(klog.FromContext(ctx), ControllerName)
	ctx = klog.NewContext(ctx, logger)
	logger.Info("Starting controller")
	defer logger.Info("Shutting down controller")

	for range numThreads {
		go wait.Until(func() { c.startWorker(ctx) }, time.Second, ctx.Done())
	}

	<-ctx.Done()
}

// startWorker processes work items from the queue.
func (c *Controller) startWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

// processNextWorkItem retrieves and processes the next work item.
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

	if requeue, err := c.process(ctx, key); err != nil {
		utilruntime.HandleError(fmt.Errorf("%q controller failed to sync %q, err: %w", ControllerName, key, err))
		c.queue.AddRateLimited(key)
		return true
	} else if requeue {
		// only requeue if we didn't error, but we still want to requeue
		c.queue.Add(key)
		return true
	}
	c.queue.Forget(key)
	return true
}

// process handles the reconciliation of a single SyncTarget resource.
func (c *Controller) process(ctx context.Context, key string) (bool, error) {
	logger := klog.FromContext(ctx)

	clusterName, _, name, err := kcpcache.SplitMetaClusterNamespaceKey(key)
	if err != nil {
		logger.Error(err, "unable to decode key")
		return false, nil
	}

	syncTarget, err := c.syncTargetLister.Cluster(clusterName).Get(name)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			logger.Error(err, "failed to get SyncTarget from lister", "cluster", clusterName, "name", name)
		}
		return false, nil // nothing we can do here
	}

	old := syncTarget
	syncTarget = syncTarget.DeepCopy()

	logger = logging.WithObject(logger, syncTarget)
	ctx = klog.NewContext(ctx, logger)

	var errs []error
	requeue, err := c.reconcile(ctx, syncTarget)
	if err != nil {
		errs = append(errs, err)
	}

	// If the object being reconciled changed as a result, update it.
	oldResource := &syncTargetResource{ObjectMeta: old.ObjectMeta, Spec: &old.Spec, Status: &old.Status}
	newResource := &syncTargetResource{ObjectMeta: syncTarget.ObjectMeta, Spec: &syncTarget.Spec, Status: &syncTarget.Status}
	if err := c.commit(ctx, oldResource, newResource); err != nil {
		errs = append(errs, err)
	}

	return requeue, utilerrors.NewAggregate(errs)
}

// reconcile performs the main reconciliation logic for a SyncTarget resource.
func (c *Controller) reconcile(ctx context.Context, syncTarget *workloadv1alpha1.SyncTarget) (bool, error) {
	reconcilers := []reconciler{
		&heartbeatReconciler{},
		&resourceCapacityReconciler{},
		&virtualWorkspaceReconciler{
			virtualWorkspaceLister: c.virtualWorkspaceLister,
		},
		&statusReconciler{},
	}

	var errs []error
	requeue := false

	for _, r := range reconcilers {
		var err error
		var status reconcileStatus
		status, err = r.reconcile(ctx, syncTarget)
		if err != nil {
			errs = append(errs, err)
		}
		if status == reconcileStatusStopAndRequeue {
			requeue = true
			break
		}
	}

	return requeue, utilerrors.NewAggregate(errs)
}

// reconcileStatus defines the result of a reconciliation step
type reconcileStatus int

const (
	reconcileStatusStopAndRequeue reconcileStatus = iota
	reconcileStatusContinue
)

// reconciler defines the interface for SyncTarget reconciliation steps
type reconciler interface {
	reconcile(ctx context.Context, syncTarget *workloadv1alpha1.SyncTarget) (reconcileStatus, error)
}