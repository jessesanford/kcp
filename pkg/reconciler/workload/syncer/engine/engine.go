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

package engine

import (
	"context"
	"fmt"
	"sync"
	"time"

	"strings"
	"k8s.io/klog/v2"
	
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	
	"github.com/kcp-dev/logicalcluster/v3"
	kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
	kcpinformers "github.com/kcp-dev/kcp/sdk/client/informers/externalversions"
)

const (
	// Action types for sync items
	ActionAdd    = "add"
	ActionUpdate = "update"
	ActionDelete = "delete"
	ActionStatus = "status"
)

// Engine manages the synchronization of resources between KCP and downstream clusters
type Engine struct {
	// Workspace isolation
	workspace logicalcluster.Name
	
	// Clients
	kcpClient         kcpclientset.ClusterInterface
	downstreamClient  dynamic.Interface
	
	// Informers
	kcpInformerFactory    kcpinformers.SharedInformerFactory
	downstreamInformerFactory dynamicinformer.DynamicSharedInformerFactory
	
	// Resource management
	resourceSyncers   map[schema.GroupVersionResource]*ResourceSyncer
	resourceSyncersMu sync.RWMutex
	
	// Work queue for processing sync items
	queue workqueue.RateLimitingInterface
	
	// Configuration
	config *EngineConfig
	
	// Status tracking
	status    *SyncStatus
	statusMu  sync.RWMutex
	
	// Lifecycle management
	ctx    context.Context
	cancel context.CancelFunc
	
	// Wait groups for graceful shutdown
	workers sync.WaitGroup
}

// NewEngine creates a new sync engine instance
func NewEngine(
	workspace logicalcluster.Name,
	kcpClient kcpclientset.ClusterInterface,
	downstreamClient dynamic.Interface,
	kcpInformerFactory kcpinformers.SharedInformerFactory,
	downstreamInformerFactory dynamicinformer.DynamicSharedInformerFactory,
	config *EngineConfig,
) *Engine {
	if config == nil {
		config = DefaultEngineConfig()
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Engine{
		workspace:                 workspace,
		kcpClient:                 kcpClient,
		downstreamClient:          downstreamClient,
		kcpInformerFactory:        kcpInformerFactory,
		downstreamInformerFactory: downstreamInformerFactory,
		resourceSyncers:           make(map[schema.GroupVersionResource]*ResourceSyncer),
		queue:                     workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), fmt.Sprintf("sync-engine-%s", workspace)),
		config:                    config,
		status: &SyncStatus{
			Connected:        false,
			SyncedResources:  make(map[schema.GroupVersionResource]int),
			PendingResources: make(map[schema.GroupVersionResource]int),
			FailedResources:  make(map[schema.GroupVersionResource]int),
		},
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start begins the synchronization engine
func (e *Engine) Start(ctx context.Context) error {
	logger := klog.FromContext(ctx).WithValues("component", "sync-engine")
	logger.Info("Starting sync engine", "workerCount", e.config.WorkerCount)
	
	// Start informer factories
	e.kcpInformerFactory.Start(ctx.Done())
	e.downstreamInformerFactory.Start(ctx.Done())
	// Wait for caches to sync
	logger.Info("Waiting for caches to sync")
	
	// Wait for KCP informer factory caches to sync
	kcpCaches := e.kcpInformerFactory.WaitForCacheSync(ctx.Done())
	for informer, synced := range kcpCaches {
		if !synced {
			return fmt.Errorf("failed to sync KCP informer cache for %v", informer)
		}
	}
	
	// Wait for downstream informer factory caches to sync
	downstreamCaches := e.downstreamInformerFactory.WaitForCacheSync(ctx.Done())
	for gvr, synced := range downstreamCaches {
		if !synced {
			return fmt.Errorf("failed to sync downstream informer cache for %v", gvr)
		}
	}
	// Update status to connected
	e.statusMu.Lock()
	e.status.Connected = true
	now := metav1.Now()
	e.status.LastSyncTime = &now
	e.statusMu.Unlock()
	logger.Info("Caches synced, starting workers")
	// Start worker goroutines
	for i := 0; i < e.config.WorkerCount; i++ {
		e.workers.Add(1)
		go e.worker(ctx, i)
	}
	
	// Start status reporting
	e.workers.Add(1)
	go e.statusReporter(ctx)
	logger.Info("Sync engine started successfully")
	return nil
}

// Stop gracefully shuts down the engine
func (e *Engine) Stop() {
	klog.InfoS("Stopping sync engine")
	
	// Cancel context to signal shutdown
	e.cancel()
	// Shutdown queue
	e.queue.ShutDown()
	// Wait for all workers to finish
	e.workers.Wait()
	// Update status
	e.statusMu.Lock()
	e.status.Connected = false
	e.statusMu.Unlock()
	
	klog.InfoS("Sync engine stopped")
}

// RegisterResourceSyncer adds a resource syncer for a specific GVR
func (e *Engine) RegisterResourceSyncer(gvr schema.GroupVersionResource) error {
	e.resourceSyncersMu.Lock()
	defer e.resourceSyncersMu.Unlock()
	
	if _, exists := e.resourceSyncers[gvr]; exists {
		return fmt.Errorf("resource syncer for %s already registered", gvr)
	}
	syncer, err := NewResourceSyncer(gvr, e)
	if err != nil {
		return fmt.Errorf("failed to create resource syncer for %s: %w", gvr, err)
	}
	e.resourceSyncers[gvr] = syncer
	// Setup informers for this GVR
	err = e.setupInformers(gvr)
	if err != nil {
		delete(e.resourceSyncers, gvr)
		return fmt.Errorf("failed to setup informers for %s: %w", gvr, err)
	}
	
	klog.V(2).InfoS("Registered resource syncer", "gvr", gvr)
	return nil
}

// setupInformers configures informers for the given GVR
func (e *Engine) setupInformers(gvr schema.GroupVersionResource) error {
	logger := klog.Background().WithValues("gvr", gvr, "workspace", e.workspace)
	logger.V(4).Info("Setting up informers for resource")
	
	// Setup KCP informer for this GVR
	kcpInformer, err := e.kcpInformerFactory.ForResource(gvr)
	if err != nil {
		return fmt.Errorf("failed to create KCP informer for %s: %w", gvr, err)
	}
	if kcpInformer == nil {
		return fmt.Errorf("KCP informer is nil for %s", gvr)
	}
	
	// Add event handlers for KCP resources
	kcpInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    e.handleKCPAdd,
		UpdateFunc: e.handleKCPUpdate,
		DeleteFunc: e.handleKCPDelete,
	})
	
	// Setup downstream informer for this GVR (for status sync)
	downstreamInformer := e.downstreamInformerFactory.ForResource(gvr)
	if downstreamInformer == nil {
		return fmt.Errorf("failed to create downstream informer for %s", gvr)
	}
	
	// Add event handlers for downstream resources
	downstreamInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    e.handleDownstreamAdd,
		UpdateFunc: e.handleDownstreamUpdate,
		DeleteFunc: e.handleDownstreamDelete,
	})
	
	logger.V(2).Info("Successfully set up informers for resource")
	return nil
}

// Event handlers for KCP resources
func (e *Engine) handleKCPAdd(obj interface{}) {
	e.enqueueWorkItem(obj, ActionAdd)
}

func (e *Engine) handleKCPUpdate(oldObj, newObj interface{}) {
	e.enqueueWorkItem(newObj, ActionUpdate)
}

func (e *Engine) handleKCPDelete(obj interface{}) {
	e.enqueueWorkItem(obj, ActionDelete)
}

// Event handlers for downstream resources (for status sync)
func (e *Engine) handleDownstreamAdd(obj interface{}) {
	e.enqueueWorkItem(obj, ActionStatus)
}

func (e *Engine) handleDownstreamUpdate(oldObj, newObj interface{}) {
	e.enqueueWorkItem(newObj, ActionStatus)
}

func (e *Engine) handleDownstreamDelete(obj interface{}) {
	// No action needed for downstream deletes
}

// enqueueWorkItem adds a work item to the queue
func (e *Engine) enqueueWorkItem(obj interface{}, action string) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(fmt.Errorf("error getting key for object: %w", err))
		return
	}
	
	// Determine GVR from object
	gvr, err := e.getGVRFromObject(obj)
	if err != nil {
		runtime.HandleError(fmt.Errorf("error getting GVR for object: %w", err))
		return
	}
	
	item := &SyncItem{
		GVR:       gvr,
		Key:       key,
		Action:    action,
		Object:    obj,
		Timestamp: metav1.Now(),
	}
	
	e.queue.Add(item)
}

// worker processes work items from the queue
func (e *Engine) worker(ctx context.Context, workerID int) {
	defer e.workers.Done()
	logger := klog.FromContext(ctx).WithValues("worker", workerID)
	
	logger.Info("Starting worker")
	defer logger.Info("Stopping worker")
	
	for e.processNextWorkItem(ctx) {
	}
}

// processNextWorkItem processes a single work item
func (e *Engine) processNextWorkItem(ctx context.Context) bool {
	obj, shutdown := e.queue.Get()
	if shutdown {
		return false
	}
	
	defer e.queue.Done(obj)
	
	item, ok := obj.(*SyncItem)
	if !ok {
		runtime.HandleError(fmt.Errorf("unexpected object type in queue: %T", obj))
		e.queue.Forget(obj)
		return true
	}
	
	err := e.processSyncItem(ctx, item)
	if err == nil {
		e.queue.Forget(obj)
		e.updateStatusCounter(item.GVR, "synced")
		return true
	}
	
	// Handle retry logic
	if item.Retries >= e.config.MaxRetries {
		klog.ErrorS(err, "Dropping sync item after max retries", "item", item, "retries", item.Retries)
		e.queue.Forget(obj)
		e.updateStatusCounter(item.GVR, "failed")
		return true
	}
	
	item.Retries++
	e.queue.AddRateLimited(obj)
	e.updateStatusCounter(item.GVR, "pending")
	
	return true
}

// processSyncItem processes a single sync item
func (e *Engine) processSyncItem(ctx context.Context, item *SyncItem) error {
	e.resourceSyncersMu.RLock()
	syncer, exists := e.resourceSyncers[item.GVR]
	e.resourceSyncersMu.RUnlock()
	
	if !exists {
		return fmt.Errorf("no resource syncer registered for %s", item.GVR)
	}
	
	return syncer.ProcessSyncItem(ctx, item)
}

// statusReporter periodically reports status
func (e *Engine) statusReporter(ctx context.Context) {
	defer e.workers.Done()
	
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.reportStatus()
		}
	}
}

// reportStatus logs current status
func (e *Engine) reportStatus() {
	e.statusMu.RLock()
	defer e.statusMu.RUnlock()
	
	totalSynced := 0
	totalPending := 0
	totalFailed := 0
	
	for _, count := range e.status.SyncedResources {
		totalSynced += count
	}
	for _, count := range e.status.PendingResources {
		totalPending += count
	}
	for _, count := range e.status.FailedResources {
		totalFailed += count
	}
	
	klog.V(4).InfoS("Sync engine status",
		"connected", e.status.Connected,
		"synced", totalSynced,
		"pending", totalPending,
		"failed", totalFailed,
		"queueDepth", e.queue.Len())
}

// GetStatus returns current engine status
func (e *Engine) GetStatus() *SyncStatus {
	e.statusMu.RLock()
	defer e.statusMu.RUnlock()
	
	// Deep copy status
	status := &SyncStatus{
		Connected:        e.status.Connected,
		SyncedResources:  make(map[schema.GroupVersionResource]int),
		PendingResources: make(map[schema.GroupVersionResource]int),
		FailedResources:  make(map[schema.GroupVersionResource]int),
		ErrorMessage:     e.status.ErrorMessage,
	}
	
	if e.status.LastSyncTime != nil {
		lastSync := *e.status.LastSyncTime
		status.LastSyncTime = &lastSync
	}
	
	for gvr, count := range e.status.SyncedResources {
		status.SyncedResources[gvr] = count
	}
	for gvr, count := range e.status.PendingResources {
		status.PendingResources[gvr] = count
	}
	for gvr, count := range e.status.FailedResources {
		status.FailedResources[gvr] = count
	}
	
	return status
}

// updateStatusCounter updates status counters
func (e *Engine) updateStatusCounter(gvr schema.GroupVersionResource, category string) {
	e.statusMu.Lock()
	defer e.statusMu.Unlock()
	
	now := metav1.Now()
	e.status.LastSyncTime = &now
	
	switch category {
	case "synced":
		e.status.SyncedResources[gvr]++
	case "pending":
		e.status.PendingResources[gvr]++
	case "failed":
		e.status.FailedResources[gvr]++
	}
}

// getGVRFromObject extracts GVR from an object
func (e *Engine) getGVRFromObject(obj interface{}) (schema.GroupVersionResource, error) {
	if obj == nil {
		return schema.GroupVersionResource{}, fmt.Errorf("object is nil")
	}
	
	// Try to get GVR from unstructured object
	if u, ok := obj.(*unstructured.Unstructured); ok {
		gvk := u.GroupVersionKind()
		if gvk.Empty() {
			return schema.GroupVersionResource{}, fmt.Errorf("object has empty GroupVersionKind")
		}
		
		// Convert GVK to GVR by pluralizing the kind
		// This is a basic implementation - in production, you'd use discovery client
		gvr := schema.GroupVersionResource{
			Group:    gvk.Group,
			Version:  gvk.Version,
			Resource: pluralizeKind(gvk.Kind),
		}
		return gvr, nil
	}
	
	// Try to extract from cache.MetaNamespaceKeyFunc compatible objects
	// For most Kubernetes objects, we need to rely on the informer or client to provide GVR context
	// This is a limitation of the current implementation - in production, you'd pass GVR context
	
	return schema.GroupVersionResource{}, fmt.Errorf("unable to extract GVR from object of type %T", obj)
}

// pluralizeKind provides basic pluralization for Kubernetes kinds
// This is a simplified version - in production, use discovery client or a proper pluralization library
func pluralizeKind(kind string) string {
	if kind == "" {
		return ""
	}
	
	// Handle common irregular plurals
	specialCases := map[string]string{
		"Endpoints":                    "endpoints",
		"NetworkPolicy":                "networkpolicies",
		"PodSecurityPolicy":            "podsecuritypolicies",
		"ComponentStatus":              "componentstatuses",
		"HorizontalPodAutoscaler":      "horizontalpodautoscalers",
		"CronJob":                      "cronjobs",
		"StorageClass":                 "storageclasses",
		"VolumeAttachment":             "volumeattachments",
		"CSIDriver":                    "csidrivers",
		"CSINode":                      "csinodes",
		"RuntimeClass":                 "runtimeclasses",
		"PriorityClass":                "priorityclasses",
		"VolumeSnapshotClass":          "volumesnapshotclasses",
		"VolumeSnapshotContent":        "volumesnapshotcontents",
		"VolumeSnapshot":               "volumesnapshots",
	}
	
	if plural, exists := specialCases[kind]; exists {
		return plural
	}
	
	// Basic pluralization rules
	lower := strings.ToLower(kind)
	if strings.HasSuffix(lower, "s") {
		return lower + "es"
	}
	if strings.HasSuffix(lower, "y") {
		return lower[:len(lower)-1] + "ies"
	}
	return lower + "s"
}