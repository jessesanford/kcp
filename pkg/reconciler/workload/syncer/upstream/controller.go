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

package upstream

import (
	"context"
	"fmt"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"

	kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
	kcpinformers "github.com/kcp-dev/kcp/sdk/client/informers/externalversions"
	workloadinformers "github.com/kcp-dev/kcp/sdk/client/informers/externalversions/workload/v1alpha1"
	workloadlisters "github.com/kcp-dev/kcp/sdk/client/listers/workload/v1alpha1"
	workloadv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"
)

// UpstreamSyncController manages the upstream synchronization of resources from physical clusters to KCP.
// It watches SyncTarget resources and establishes the foundation for syncing resource states back to KCP.
//
// The controller follows KCP patterns:
// - Uses cluster-aware clients and informers for workspace isolation
// - Implements proper queue-based processing with retries
// - Maintains SyncTarget state and handles lifecycle events
//
// This foundation will be extended in subsequent PRs with:
// - Resource transformation logic (PR 2)
// - Actual sync operations and conflict resolution (PR 3)
type UpstreamSyncController struct {
	// Core KCP components
	kcpClusterClient   kcpclientset.ClusterInterface
	syncTargetInformer cache.SharedIndexInformer
	syncTargetLister   workloadlisters.SyncTargetClusterLister
	
	// Work queue and processing
	queue      workqueue.RateLimitingInterface
	numWorkers int
	
	// Configuration
	syncInterval time.Duration
	
	// Sync target tracking
	syncTargetStatuses map[string]*SyncTargetStatus
	statusMutex        sync.RWMutex
}

// NewController creates a new UpstreamSyncController following KCP patterns.
// It integrates with the SyncTarget informer system and maintains workspace isolation.
//
// Parameters:
//   - kcpClusterClient: Cluster-aware KCP client for workspace operations
//   - syncTargetInformer: Shared informer for SyncTarget resources
//   - syncInterval: Interval between sync operations (0 uses default)
//   - numWorkers: Number of worker goroutines (0 uses default)
//
// Returns:
//   - *UpstreamSyncController: Configured controller ready to start
//   - error: Configuration or setup error
func NewController(
	kcpClusterClient kcpclientset.ClusterInterface,
	syncTargetInformer workloadinformers.SyncTargetClusterInformer,
	syncInterval time.Duration,
	numWorkers int,
) (*UpstreamSyncController, error) {
	if kcpClusterClient == nil {
		return nil, fmt.Errorf("kcpClusterClient cannot be nil")
	}
	
	if syncInterval <= 0 {
		syncInterval = DefaultSyncInterval
	}
	
	if numWorkers <= 0 {
		numWorkers = DefaultNumWorkers
	}

	c := &UpstreamSyncController{
		kcpClusterClient:   kcpClusterClient,
		syncTargetInformer: syncTargetInformer.Informer(),
		syncTargetLister:   syncTargetInformer.Lister(),
		queue: workqueue.NewNamedRateLimitingQueue(
			DefaultRateLimiter(),
			ControllerName,
		),
		numWorkers:         numWorkers,
		syncInterval:       syncInterval,
		syncTargetStatuses: make(map[string]*SyncTargetStatus),
	}

	// Add informer event handlers to track SyncTarget lifecycle
	syncTargetInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    c.enqueueSyncTarget,
			UpdateFunc: func(old, new interface{}) { c.enqueueSyncTarget(new) },
			DeleteFunc: func(obj interface{}) { c.enqueueSyncTargetDelete(obj) },
		},
	)

	return c, nil
}

// Start starts the upstream sync controller and runs until the context is cancelled.
// It waits for informer caches to sync, then starts the configured number of workers.
func (c *UpstreamSyncController) Start(ctx context.Context) {
	defer runtime.HandleCrash()
	defer c.queue.ShutDown()

	logger := klog.FromContext(ctx).WithName(ControllerName)
	ctx = klog.NewContext(ctx, logger)

	logger.Info("Starting upstream sync controller",
		"syncInterval", c.syncInterval,
		"numWorkers", c.numWorkers)

	// Wait for the informer caches to be synced before starting workers
	if !cache.WaitForNamedCacheSync(ControllerName, ctx.Done(), c.syncTargetInformer.HasSynced) {
		logger.Error(nil, "Failed to wait for caches to sync")
		return
	}

	logger.Info("Caches are synced, starting workers")

	// Start worker goroutines
	for i := 0; i < c.numWorkers; i++ {
		go wait.UntilWithContext(ctx, c.worker, time.Second)
	}

	logger.Info("Started workers", "count", c.numWorkers)

	// Block until the context is cancelled
	<-ctx.Done()
	logger.Info("Shutting down upstream sync controller")
}

// worker runs a worker thread that processes work items from the queue
func (c *UpstreamSyncController) worker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

// processNextWorkItem processes the next work item from the queue
func (c *UpstreamSyncController) processNextWorkItem(ctx context.Context) bool {
	logger := klog.FromContext(ctx)

	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)

	// Convert the key back to a work item
	workItem, ok := key.(*WorkItem)
	if !ok {
		logger.Error(nil, "Expected WorkItem from queue", "key", key)
		c.queue.Forget(key)
		return true
	}

	err := c.processWorkItem(ctx, workItem)
	c.handleWorkItemResult(workItem, err)

	return true
}

// processWorkItem processes a single work item
func (c *UpstreamSyncController) processWorkItem(ctx context.Context, item *WorkItem) error {
	logger := klog.FromContext(ctx).WithValues("syncTargetKey", item.Key.String(), "action", item.Action)

	switch item.Action {
	case ActionSync, ActionReconcile:
		return c.reconcileSyncTarget(ctx, item.Key)
	case ActionDelete:
		return c.deleteSyncTarget(ctx, item.Key)
	default:
		return fmt.Errorf("unknown work action: %s", item.Action)
	}
}

// handleWorkItemResult handles the result of processing a work item
func (c *UpstreamSyncController) handleWorkItemResult(item *WorkItem, err error) {
	if err == nil {
		// Success - forget the item
		c.queue.Forget(item)
		c.updateSyncTargetStatus(item.Key, &SyncResult{
			Success:   true,
			Timestamp: time.Now(),
		})
		return
	}

	// Handle error with exponential backoff
	item.Retries++
	if item.Retries < MaxRetries {
		klog.V(2).InfoS("Retrying work item", 
			"syncTargetKey", item.Key.String(),
			"retries", item.Retries,
			"error", err)
		c.queue.AddRateLimited(item)
	} else {
		klog.ErrorS(err, "Dropping work item after max retries", 
			"syncTargetKey", item.Key.String(),
			"retries", item.Retries)
		c.queue.Forget(item)
	}

	c.updateSyncTargetStatus(item.Key, &SyncResult{
		Success:   false,
		Error:     err,
		Timestamp: time.Now(),
	})
}

// reconcileSyncTarget handles the reconciliation of a SyncTarget
func (c *UpstreamSyncController) reconcileSyncTarget(ctx context.Context, key SyncTargetKey) error {
	logger := klog.FromContext(ctx).WithValues("syncTargetKey", key.String())

	// Get the SyncTarget from the lister
	syncTarget, err := c.syncTargetLister.Cluster(key.Cluster).Get(key.Name)
	if err != nil {
		// SyncTarget was deleted, handle cleanup
		logger.V(3).Info("SyncTarget not found, likely deleted")
		return c.deleteSyncTarget(ctx, key)
	}

	logger.V(4).Info("Reconciling SyncTarget",
		"location", syncTarget.Spec.Location,
		"ready", c.isSyncTargetReady(syncTarget))

	// Check if SyncTarget is ready for syncing
	if !c.isSyncTargetReady(syncTarget) {
		logger.V(3).Info("SyncTarget not ready for upstream sync")
		return nil
	}

	// TODO: In subsequent PRs, this will:
	// - Set up physical cluster client (PR 2)
	// - Perform actual resource synchronization (PR 3)
	// - Handle conflicts and transformations (PR 2 & 3)

	logger.V(4).Info("Successfully processed SyncTarget (placeholder)")
	return nil
}

// deleteSyncTarget handles cleanup when a SyncTarget is deleted
func (c *UpstreamSyncController) deleteSyncTarget(ctx context.Context, key SyncTargetKey) error {
	logger := klog.FromContext(ctx).WithValues("syncTargetKey", key.String())

	logger.V(3).Info("Cleaning up deleted SyncTarget")

	// Clean up sync target status tracking
	c.statusMutex.Lock()
	delete(c.syncTargetStatuses, key.String())
	c.statusMutex.Unlock()

	// TODO: In subsequent PRs, this will:
	// - Clean up physical cluster clients
	// - Remove cached resources
	// - Clean up any ongoing sync operations

	return nil
}

// enqueueSyncTarget enqueues a SyncTarget for processing
func (c *UpstreamSyncController) enqueueSyncTarget(obj interface{}) {
	key, err := c.keyFunc(obj)
	if err != nil {
		klog.ErrorS(err, "Failed to get key for SyncTarget", "object", obj)
		return
	}

	workItem := &WorkItem{
		Key:          key,
		Action:       ActionSync,
		EnqueuedTime: time.Now(),
		Retries:      0,
	}

	c.queue.Add(workItem)
}

// enqueueSyncTargetDelete enqueues a SyncTarget for deletion cleanup
func (c *UpstreamSyncController) enqueueSyncTargetDelete(obj interface{}) {
	key, err := c.keyFunc(obj)
	if err != nil {
		klog.ErrorS(err, "Failed to get key for deleted SyncTarget", "object", obj)
		return
	}

	workItem := &WorkItem{
		Key:          key,
		Action:       ActionDelete,
		EnqueuedTime: time.Now(),
		Retries:      0,
	}

	c.queue.Add(workItem)
}

// keyFunc extracts a SyncTargetKey from a SyncTarget object
func (c *UpstreamSyncController) keyFunc(obj interface{}) (SyncTargetKey, error) {
	syncTarget, ok := obj.(*workloadv1alpha1.SyncTarget)
	if !ok {
		return SyncTargetKey{}, fmt.Errorf("expected SyncTarget, got %T", obj)
	}

	cluster := logicalcluster.From(syncTarget)
	return SyncTargetKey{
		Cluster: cluster,
		Name:    syncTarget.Name,
	}, nil
}

// isSyncTargetReady checks if a SyncTarget is ready for upstream synchronization
func (c *UpstreamSyncController) isSyncTargetReady(syncTarget *workloadv1alpha1.SyncTarget) bool {
	// Check if SyncTarget has the Ready condition set to True
	for _, condition := range syncTarget.Status.Conditions {
		if condition.Type == workloadv1alpha1.SyncTargetReady && condition.Status == "True" {
			return true
		}
	}
	return false
}

// updateSyncTargetStatus updates the internal status tracking for a SyncTarget
func (c *UpstreamSyncController) updateSyncTargetStatus(key SyncTargetKey, result *SyncResult) {
	c.statusMutex.Lock()
	defer c.statusMutex.Unlock()

	keyStr := key.String()
	status, exists := c.syncTargetStatuses[keyStr]
	if !exists {
		status = &SyncTargetStatus{}
		c.syncTargetStatuses[keyStr] = status
	}

	status.LastSync = result
	status.SyncCount++

	if !result.Success && result.Error != nil {
		status.ErrorCount++
		now := time.Now()
		status.LastErrorTime = &now
	}
}

// GetSyncTargetStatus returns the current status for a SyncTarget
func (c *UpstreamSyncController) GetSyncTargetStatus(key SyncTargetKey) (*SyncTargetStatus, bool) {
	c.statusMutex.RLock()
	defer c.statusMutex.RUnlock()

	status, exists := c.syncTargetStatuses[key.String()]
	if !exists {
		return nil, false
	}

	// Return a copy to avoid race conditions
	statusCopy := *status
	return &statusCopy, true
}