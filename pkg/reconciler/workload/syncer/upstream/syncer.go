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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	kcpcache "github.com/kcp-dev/apimachinery/v2/pkg/cache"
	"github.com/kcp-dev/logicalcluster/v3"

	"github.com/kcp-dev/kcp/pkg/logging"
	"github.com/kcp-dev/kcp/pkg/reconciler/committer"
	workloadv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"
	kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
	workloadv1alpha1informers "github.com/kcp-dev/kcp/sdk/client/informers/externalversions/workload/v1alpha1"
)

const (
	ControllerName = "kcp-upstream-syncer"

	// DefaultSyncInterval is the default interval for upstream synchronization
	DefaultSyncInterval = 30 * time.Second

	// MaxConcurrentSyncs is the maximum number of SyncTargets to sync concurrently
	MaxConcurrentSyncs = 5
)

// UpstreamSyncer manages the upstream synchronization of resources from physical clusters to KCP.
// It watches SyncTarget resources and establishes connections to their associated physical clusters
// to pull resource states back into KCP workspaces.
type UpstreamSyncer struct {
	queue workqueue.TypedRateLimitingInterface[string]

	// KCP clients for accessing workspace resources
	kcpClusterClient kcpclientset.ClusterInterface

	// Physical cluster clients per sync target
	physicalClients    map[string]dynamic.Interface
	physicalClientsMux sync.RWMutex

	// Resource discovery and management
	discoveryManager *discoveryManager

	// Conflict resolution strategy
	conflictResolver *conflictResolver

	// Status aggregator for resource health
	statusAggregator *statusAggregator

	// Configuration
	syncInterval time.Duration
	numWorkers   int

	// Informer functions
	listSyncTargets func() ([]*workloadv1alpha1.SyncTarget, error)
	getSyncTarget   func(clusterName logicalcluster.Name, name string) (*workloadv1alpha1.SyncTarget, error)

	// Committer for status updates
	commit committer.CommitFunc
}

// NewUpstreamSyncer creates a new UpstreamSyncer controller following KCP patterns.
// It integrates with the SyncTarget API and maintains workspace isolation.
//
// Parameters:
//   - kcpClusterClient: Cluster-aware KCP client for accessing workspaces
//   - syncTargetInformer: Shared informer for SyncTarget resources
//   - syncInterval: Interval for synchronizing with physical clusters
//
// Returns:
//   - *UpstreamSyncer: Configured upstream syncer ready to start
//   - error: Configuration or setup error
func NewUpstreamSyncer(
	kcpClusterClient kcpclientset.ClusterInterface,
	syncTargetInformer workloadv1alpha1informers.SyncTargetClusterInformer,
	syncInterval time.Duration,
) (*UpstreamSyncer, error) {
	if syncInterval <= 0 {
		syncInterval = DefaultSyncInterval
	}

	us := &UpstreamSyncer{
		queue: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[string](),
			workqueue.TypedRateLimitingQueueConfig[string]{
				Name: ControllerName,
			},
		),

		kcpClusterClient: kcpClusterClient,
		physicalClients:  make(map[string]dynamic.Interface),
		syncInterval:     syncInterval,
		numWorkers:       MaxConcurrentSyncs,

		listSyncTargets: func() ([]*workloadv1alpha1.SyncTarget, error) {
			return syncTargetInformer.Lister().List(labels.Everything())
		},
		getSyncTarget: func(clusterName logicalcluster.Name, name string) (*workloadv1alpha1.SyncTarget, error) {
			return syncTargetInformer.Lister().Cluster(clusterName).Get(name)
		},

		commit: committer.NewCommitter[*SyncTarget, Patcher, *SyncTargetSpec, *SyncTargetStatus](
			kcpClusterClient.WorkloadV1alpha1().SyncTargets(),
		),
	}

	// Initialize sub-components
	var err error
	us.discoveryManager, err = newDiscoveryManager(us)
	if err != nil {
		return nil, fmt.Errorf("failed to create discovery manager: %w", err)
	}

	us.conflictResolver = newConflictResolver()
	us.statusAggregator = newStatusAggregator()

	// Set up event handlers for SyncTarget resources
	_, _ = syncTargetInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			us.enqueueSyncTarget(obj.(*workloadv1alpha1.SyncTarget))
		},
		UpdateFunc: func(_, newObj interface{}) {
			us.enqueueSyncTarget(newObj.(*workloadv1alpha1.SyncTarget))
		},
		DeleteFunc: func(obj interface{}) {
			syncTarget := obj.(*workloadv1alpha1.SyncTarget)
			us.enqueueSyncTarget(syncTarget)
			us.cleanupPhysicalClient(syncTarget)
		},
	})

	return us, nil
}

// Type aliases for committer pattern
type SyncTarget = workloadv1alpha1.SyncTarget
type SyncTargetSpec = workloadv1alpha1.SyncTargetSpec
type SyncTargetStatus = workloadv1alpha1.SyncTargetStatus
type Patcher = kcpclientset.WorkloadV1alpha1Interface
type Resource = committer.Resource[*SyncTargetSpec, *SyncTargetStatus]

// enqueueSyncTarget enqueues a SyncTarget for processing.
func (us *UpstreamSyncer) enqueueSyncTarget(syncTarget *workloadv1alpha1.SyncTarget) {
	key, err := kcpcache.DeletionHandlingMetaClusterNamespaceKeyFunc(syncTarget)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}

	logger := logging.WithQueueKey(logging.WithReconciler(klog.Background(), ControllerName), key)
	logger.V(4).Info("queueing SyncTarget for upstream sync")
	us.queue.Add(key)
}

// Start starts the upstream syncer controller, which stops when ctx.Done() is closed.
func (us *UpstreamSyncer) Start(ctx context.Context, numThreads int) {
	defer utilruntime.HandleCrash()
	defer us.queue.ShutDown()

	logger := logging.WithReconciler(klog.FromContext(ctx), ControllerName)
	ctx = klog.NewContext(ctx, logger)
	logger.Info("Starting upstream syncer controller")
	defer logger.Info("Shutting down upstream syncer controller")

	// Use provided numThreads or default
	workers := numThreads
	if workers <= 0 {
		workers = us.numWorkers
	}

	// Start worker goroutines
	for i := range workers {
		go wait.UntilWithContext(ctx, us.startWorker, time.Second)
	}

	// Start periodic sync for all targets
	go us.periodicSync(ctx)

	<-ctx.Done()
}

// startWorker processes work items from the queue.
func (us *UpstreamSyncer) startWorker(ctx context.Context) {
	for us.processNextWorkItem(ctx) {
	}
}

// processNextWorkItem processes the next work item in the queue.
func (us *UpstreamSyncer) processNextWorkItem(ctx context.Context) bool {
	key, quit := us.queue.Get()
	if quit {
		return false
	}

	logger := logging.WithQueueKey(klog.FromContext(ctx), key)
	ctx = klog.NewContext(ctx, logger)
	logger.V(4).Info("processing SyncTarget key")

	defer us.queue.Done(key)

	if err := us.process(ctx, key); err != nil {
		utilruntime.HandleError(fmt.Errorf("%q controller failed to sync %q, err: %w", ControllerName, key, err))
		us.queue.AddRateLimited(key)
		return true
	}

	us.queue.Forget(key)
	return true
}

// process handles the reconciliation of a single SyncTarget.
func (us *UpstreamSyncer) process(ctx context.Context, key string) error {
	cluster, _, name, err := kcpcache.SplitMetaClusterNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(err)
		return nil
	}

	obj, err := us.getSyncTarget(cluster, name)
	if err != nil {
		if metav1.IsNotFound(err) {
			return nil // object deleted before we handled it
		}
		return err
	}

	old := obj
	obj = obj.DeepCopy()

	logger := logging.WithObject(klog.FromContext(ctx), obj)
	ctx = klog.NewContext(ctx, logger)

	var errs []error
	if err := us.reconcile(ctx, obj); err != nil {
		errs = append(errs, err)
	}

	// Update status if needed
	oldResource := &Resource{ObjectMeta: old.ObjectMeta, Spec: &old.Spec, Status: &old.Status}
	newResource := &Resource{ObjectMeta: obj.ObjectMeta, Spec: &obj.Spec, Status: &obj.Status}
	if err := us.commit(ctx, oldResource, newResource); err != nil {
		errs = append(errs, err)
	}

	return utilerrors.NewAggregate(errs)
}

// reconcile performs the actual synchronization logic for a SyncTarget.
func (us *UpstreamSyncer) reconcile(ctx context.Context, syncTarget *workloadv1alpha1.SyncTarget) error {
	logger := klog.FromContext(ctx)

	// Check if SyncTarget is ready for upstream sync
	if !us.isSyncTargetReady(syncTarget) {
		logger.V(3).Info("SyncTarget not ready for upstream sync")
		return nil
	}

	// Ensure we have a physical cluster client
	physicalClient, err := us.getOrCreatePhysicalClient(ctx, syncTarget)
	if err != nil {
		return fmt.Errorf("failed to get physical client: %w", err)
	}

	// Perform resource discovery
	if err := us.discoveryManager.updateDiscovery(ctx, syncTarget, physicalClient); err != nil {
		return fmt.Errorf("resource discovery failed: %w", err)
	}

	// Sync resources from physical cluster
	if err := us.syncResourcesFromPhysical(ctx, syncTarget, physicalClient); err != nil {
		return fmt.Errorf("upstream sync failed: %w", err)
	}

	// Update SyncTarget status
	us.updateSyncTargetStatus(syncTarget)

	logger.V(3).Info("Successfully completed upstream sync for SyncTarget")
	return nil
}

// periodicSync performs periodic synchronization of all SyncTargets.
func (us *UpstreamSyncer) periodicSync(ctx context.Context) {
	ticker := time.NewTicker(us.syncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			us.syncAllTargets(ctx)
		}
	}
}

// syncAllTargets triggers sync for all active SyncTargets.
func (us *UpstreamSyncer) syncAllTargets(ctx context.Context) {
	logger := klog.FromContext(ctx)
	
	syncTargets, err := us.listSyncTargets()
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("failed to list SyncTargets: %w", err))
		return
	}

	logger.V(4).Info("Triggering periodic sync for all SyncTargets", "count", len(syncTargets))

	for _, syncTarget := range syncTargets {
		if us.isSyncTargetReady(syncTarget) {
			us.enqueueSyncTarget(syncTarget)
		}
	}
}

// Helper methods
func (us *UpstreamSyncer) isSyncTargetReady(syncTarget *workloadv1alpha1.SyncTarget) bool {
	// Check if SyncTarget has required conditions set to Ready
	for _, condition := range syncTarget.Status.Conditions {
		if condition.Type == workloadv1alpha1.SyncTargetReady && condition.Status == metav1.ConditionTrue {
			return true
		}
	}
	return false
}

func (us *UpstreamSyncer) getOrCreatePhysicalClient(ctx context.Context, syncTarget *workloadv1alpha1.SyncTarget) (dynamic.Interface, error) {
	key := us.getSyncTargetKey(syncTarget)
	
	us.physicalClientsMux.RLock()
	client, exists := us.physicalClients[key]
	us.physicalClientsMux.RUnlock()
	
	if exists {
		return client, nil
	}

	// Create new physical client (implementation would use cluster credentials)
	// For now, return nil as this requires cluster-specific authentication setup
	return nil, fmt.Errorf("physical client creation not yet implemented for SyncTarget %s", key)
}

func (us *UpstreamSyncer) cleanupPhysicalClient(syncTarget *workloadv1alpha1.SyncTarget) {
	key := us.getSyncTargetKey(syncTarget)
	
	us.physicalClientsMux.Lock()
	delete(us.physicalClients, key)
	us.physicalClientsMux.Unlock()
}

func (us *UpstreamSyncer) getSyncTargetKey(syncTarget *workloadv1alpha1.SyncTarget) string {
	cluster := logicalcluster.From(syncTarget)
	return fmt.Sprintf("%s/%s", cluster.String(), syncTarget.Name)
}

func (us *UpstreamSyncer) syncResourcesFromPhysical(ctx context.Context, syncTarget *workloadv1alpha1.SyncTarget, physicalClient dynamic.Interface) error {
	// This would implement the actual resource sync logic
	// For now, just log that sync would happen
	logger := klog.FromContext(ctx)
	logger.V(3).Info("Would sync resources from physical cluster", "syncTarget", syncTarget.Name)
	return nil
}

func (us *UpstreamSyncer) updateSyncTargetStatus(syncTarget *workloadv1alpha1.SyncTarget) {
	now := metav1.Now()
	syncTarget.Status.LastSyncTime = &now
}