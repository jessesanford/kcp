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

package syncer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"

	"github.com/kcp-dev/kcp/pkg/reconciler/workload/tmc"
)

// ResourceController handles synchronization of a specific resource type
// between KCP and a physical cluster
type ResourceController struct {
	// Resource identification
	gvr          schema.GroupVersionResource
	gvk          schema.GroupVersionKind
	namespaced   bool
	
	// Clients and informers
	kcpClient       dynamic.Interface
	clusterClient   dynamic.Interface
	kcpInformer     cache.SharedIndexInformer
	clusterInformer cache.SharedIndexInformer
	
	// Configuration
	syncTargetName   string
	workspaceCluster logicalcluster.Name
	resyncPeriod     time.Duration
	workers          int
	
	// Work queue
	queue workqueue.RateLimitingInterface
	
	// TMC Integration
	tmcMetrics *tmc.MetricsCollector
	
	// State
	started bool
	stopCh  chan struct{}
	mu      sync.RWMutex
	
	// Metrics
	syncCount         int64
	errorCount        int64
	conflictCount     int64
	transformCount    int64
	lastSyncTime      time.Time
}

// ResourceControllerOptions configures a resource controller
type ResourceControllerOptions struct {
	GVR              schema.GroupVersionResource
	GVK              schema.GroupVersionKind
	Namespaced       bool
	SyncTargetName   string
	WorkspaceCluster logicalcluster.Name
	KCPClient        dynamic.Interface
	ClusterClient    dynamic.Interface
	ResyncPeriod     time.Duration
	Workers          int
	TMCMetrics       *tmc.MetricsCollector
}

// NewResourceController creates a new resource controller
func NewResourceController(options ResourceControllerOptions) (*ResourceController, error) {
	logger := klog.Background().WithValues(
		"component", "ResourceController",
		"gvr", options.GVR.String(),
		"syncTarget", options.SyncTargetName,
	)
	logger.Info("Creating resource controller")

	rc := &ResourceController{
		gvr:              options.GVR,
		gvk:              options.GVK,
		namespaced:       options.Namespaced,
		kcpClient:        options.KCPClient,
		clusterClient:    options.ClusterClient,
		syncTargetName:   options.SyncTargetName,
		workspaceCluster: options.WorkspaceCluster,
		resyncPeriod:     options.ResyncPeriod,
		workers:          options.Workers,
		tmcMetrics:       options.TMCMetrics,
		stopCh:           make(chan struct{}),
		queue: workqueue.NewNamedRateLimitingQueue(
			workqueue.DefaultControllerRateLimiter(),
			fmt.Sprintf("syncer-%s-%s", options.SyncTargetName, options.GVR.String()),
		),
	}

	// Set up KCP informer
	kcpInformerFactory := dynamicinformer.NewDynamicSharedInformerFactory(options.KCPClient, options.ResyncPeriod)
	rc.kcpInformer = kcpInformerFactory.ForResource(options.GVR).Informer()

	// Set up cluster informer
	clusterInformerFactory := dynamicinformer.NewDynamicSharedInformerFactory(options.ClusterClient, options.ResyncPeriod)
	rc.clusterInformer = clusterInformerFactory.ForResource(options.GVR).Informer()

	// Set up event handlers
	rc.setupEventHandlers()

	logger.Info("Successfully created resource controller")
	return rc, nil
}

// setupEventHandlers configures event handlers for the informers
func (rc *ResourceController) setupEventHandlers() {
	logger := klog.Background().WithValues(
		"component", "ResourceController",
		"gvr", rc.gvr.String(),
		"operation", "setup-handlers",
	)

	// KCP resource events - sync to cluster
	rc.kcpInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			rc.enqueueResource(obj, "kcp-add")
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			rc.enqueueResource(newObj, "kcp-update")
		},
		DeleteFunc: func(obj interface{}) {
			rc.enqueueResource(obj, "kcp-delete")
		},
	})

	// Cluster resource events - sync to KCP (for status)
	rc.clusterInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			rc.enqueueResource(obj, "cluster-add")
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			rc.enqueueResource(newObj, "cluster-update")
		},
		DeleteFunc: func(obj interface{}) {
			rc.enqueueResource(obj, "cluster-delete")
		},
	})

	logger.V(2).Info("Event handlers configured")
}

// enqueueResource adds a resource to the work queue
func (rc *ResourceController) enqueueResource(obj interface{}, action string) {
	logger := klog.Background().WithValues(
		"component", "ResourceController",
		"gvr", rc.gvr.String(),
		"action", action,
	)

	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Error(err, "Failed to get key for object")
		rc.tmcMetrics.RecordSyncError(rc.syncTargetName, rc.gvr.String(), tmc.TMCErrorTypeInternal)
		return
	}

	// Prefix key with action for context
	queueKey := fmt.Sprintf("%s:%s", action, key)
	rc.queue.Add(queueKey)

	logger.V(4).Info("Enqueued resource", "key", queueKey)
}

// Start starts the resource controller
func (rc *ResourceController) Start(ctx context.Context) error {
	logger := klog.FromContext(ctx).WithValues(
		"component", "ResourceController",
		"gvr", rc.gvr.String(),
		"syncTarget", rc.syncTargetName,
	)
	logger.Info("Starting resource controller")

	rc.mu.Lock()
	if rc.started {
		rc.mu.Unlock()
		return fmt.Errorf("resource controller already started")
	}
	rc.started = true
	rc.mu.Unlock()

	// Start informers
	go rc.kcpInformer.Run(rc.stopCh)
	go rc.clusterInformer.Run(rc.stopCh)

	// Wait for cache sync
	logger.Info("Waiting for caches to sync")
	if !cache.WaitForCacheSync(rc.stopCh, rc.kcpInformer.HasSynced, rc.clusterInformer.HasSynced) {
		return tmc.NewTMCError(tmc.TMCErrorTypeSyncFailure, "resource-controller", "cache-sync").
			WithMessage("Failed to sync caches").
			WithResource(rc.gvk, "", "").
			Build()
	}

	logger.Info("Caches synced, starting workers")

	// Start workers
	for i := 0; i < rc.workers; i++ {
		go wait.UntilWithContext(ctx, rc.runWorker, time.Second)
	}

	logger.Info("Resource controller started successfully", "workers", rc.workers)
	return nil
}

// Stop stops the resource controller
func (rc *ResourceController) Stop() {
	logger := klog.Background().WithValues(
		"component", "ResourceController",
		"gvr", rc.gvr.String(),
	)
	logger.Info("Stopping resource controller")

	rc.mu.Lock()
	defer rc.mu.Unlock()

	if !rc.started {
		return
	}

	close(rc.stopCh)
	rc.queue.ShutDown()
	rc.started = false

	logger.Info("Resource controller stopped")
}

// runWorker runs a single worker goroutine
func (rc *ResourceController) runWorker(ctx context.Context) {
	logger := klog.FromContext(ctx).WithValues(
		"component", "ResourceController",
		"gvr", rc.gvr.String(),
		"operation", "worker",
	)

	for rc.processNextWorkItem(ctx) {
		// Continue processing
	}

	logger.V(4).Info("Worker shutting down")
}

// processNextWorkItem processes the next item in the work queue
func (rc *ResourceController) processNextWorkItem(ctx context.Context) bool {
	obj, shutdown := rc.queue.Get()
	if shutdown {
		return false
	}

	defer rc.queue.Done(obj)

	err := rc.syncResource(ctx, obj.(string))
	if err == nil {
		rc.queue.Forget(obj)
		return true
	}

	logger := klog.FromContext(ctx).WithValues(
		"component", "ResourceController",
		"gvr", rc.gvr.String(),
		"key", obj,
	)

	if rc.queue.NumRequeues(obj) < 5 {
		logger.Error(err, "Error syncing resource, retrying")
		rc.queue.AddRateLimited(obj)
		return true
	}

	logger.Error(err, "Dropping resource from queue after too many retries")
	rc.queue.Forget(obj)
	return true
}

// syncResource synchronizes a single resource
func (rc *ResourceController) syncResource(ctx context.Context, key string) error {
	startTime := time.Now()
	logger := klog.FromContext(ctx).WithValues(
		"component", "ResourceController",
		"gvr", rc.gvr.String(),
		"key", key,
	)

	defer func() {
		duration := time.Since(startTime)
		rc.tmcMetrics.RecordSyncDuration(rc.syncTargetName, rc.gvr.String(), "sync", duration)
		rc.lastSyncTime = time.Now()
		rc.syncCount++
	}()

	// Parse action and resource key
	parts := split(key, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid key format: %s", key)
	}
	action := parts[0]
	resourceKey := parts[1]

	logger.V(3).Info("Syncing resource", "action", action)

	switch action {
	case "kcp-add", "kcp-update":
		return rc.syncKCPToCluster(ctx, resourceKey)
	case "kcp-delete":
		return rc.syncKCPDeletion(ctx, resourceKey)
	case "cluster-add", "cluster-update":
		return rc.syncClusterToKCP(ctx, resourceKey)
	case "cluster-delete":
		return rc.handleClusterDeletion(ctx, resourceKey)
	default:
		return fmt.Errorf("unknown action: %s", action)
	}
}

// syncKCPToCluster synchronizes a resource from KCP to the cluster
func (rc *ResourceController) syncKCPToCluster(ctx context.Context, key string) error {
	logger := klog.FromContext(ctx).WithValues(
		"component", "ResourceController",
		"operation", "sync-kcp-to-cluster",
		"key", key,
	)

	// Get resource from KCP
	kcpResource, err := rc.getResourceFromKCP(ctx, key)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.V(2).Info("Resource not found in KCP, skipping")
			return nil
		}
		rc.errorCount++
		return tmc.NewTMCError(tmc.TMCErrorTypeSyncFailure, "resource-controller", "get-kcp-resource").
			WithMessage("Failed to get resource from KCP").
			WithCause(err).
			WithResource(rc.gvk, "", key).
			Build()
	}

	// Transform resource for cluster
	clusterResource, err := rc.transformForCluster(kcpResource)
	if err != nil {
		rc.errorCount++
		return tmc.NewTMCError(tmc.TMCErrorTypeSyncFailure, "resource-controller", "transform-resource").
			WithMessage("Failed to transform resource for cluster").
			WithCause(err).
			WithResource(rc.gvk, kcpResource.GetNamespace(), kcpResource.GetName()).
			Build()
	}

	// Apply to cluster
	err = rc.applyToCluster(ctx, clusterResource)
	if err != nil {
		rc.errorCount++
		if errors.IsConflict(err) {
			rc.conflictCount++
			return tmc.NewTMCError(tmc.TMCErrorTypeResourceConflict, "resource-controller", "apply-to-cluster").
				WithMessage("Resource conflict applying to cluster").
				WithCause(err).
				WithResource(rc.gvk, clusterResource.GetNamespace(), clusterResource.GetName()).
				Build()
		}
		return tmc.NewTMCError(tmc.TMCErrorTypeSyncFailure, "resource-controller", "apply-to-cluster").
			WithMessage("Failed to apply resource to cluster").
			WithCause(err).
			WithResource(rc.gvk, clusterResource.GetNamespace(), clusterResource.GetName()).
			Build()
	}

	rc.tmcMetrics.RecordResourceProjection(rc.syncTargetName, rc.syncTargetName, rc.gvr.String(), "success")
	logger.V(3).Info("Successfully synced resource to cluster")
	return nil
}

// syncKCPDeletion handles deletion of a resource from KCP
func (rc *ResourceController) syncKCPDeletion(ctx context.Context, key string) error {
	logger := klog.FromContext(ctx).WithValues(
		"component", "ResourceController",
		"operation", "sync-kcp-deletion",
		"key", key,
	)

	// Delete from cluster
	err := rc.deleteFromCluster(ctx, key)
	if err != nil && !errors.IsNotFound(err) {
		rc.errorCount++
		return tmc.NewTMCError(tmc.TMCErrorTypeSyncFailure, "resource-controller", "delete-from-cluster").
			WithMessage("Failed to delete resource from cluster").
			WithCause(err).
			WithResource(rc.gvk, "", key).
			Build()
	}

	logger.V(3).Info("Successfully deleted resource from cluster")
	return nil
}

// syncClusterToKCP synchronizes status from cluster back to KCP
func (rc *ResourceController) syncClusterToKCP(ctx context.Context, key string) error {
	logger := klog.FromContext(ctx).WithValues(
		"component", "ResourceController",
		"operation", "sync-cluster-to-kcp",
		"key", key,
	)

	// Get resource from cluster
	clusterResource, err := rc.getResourceFromCluster(ctx, key)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.V(4).Info("Resource not found in cluster, skipping status sync")
			return nil
		}
		rc.errorCount++
		return tmc.NewTMCError(tmc.TMCErrorTypeSyncFailure, "resource-controller", "get-cluster-resource").
			WithMessage("Failed to get resource from cluster").
			WithCause(err).
			WithResource(rc.gvk, "", key).
			Build()
	}

	// Update status in KCP
	err = rc.updateStatusInKCP(ctx, clusterResource)
	if err != nil {
		rc.errorCount++
		return tmc.NewTMCError(tmc.TMCErrorTypeSyncFailure, "resource-controller", "update-kcp-status").
			WithMessage("Failed to update status in KCP").
			WithCause(err).
			WithResource(rc.gvk, clusterResource.GetNamespace(), clusterResource.GetName()).
			Build()
	}

	logger.V(4).Info("Successfully synced status to KCP")
	return nil
}

// handleClusterDeletion handles deletion of a resource from the cluster
func (rc *ResourceController) handleClusterDeletion(ctx context.Context, key string) error {
	logger := klog.FromContext(ctx).WithValues(
		"component", "ResourceController",
		"operation", "handle-cluster-deletion",
		"key", key,
	)

	// For now, we just log cluster deletions
	// In a full implementation, we might need to update KCP status
	logger.V(3).Info("Resource deleted from cluster")
	return nil
}

// Helper functions for resource operations

func (rc *ResourceController) getResourceFromKCP(ctx context.Context, key string) (*unstructured.Unstructured, error) {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return nil, err
	}

	var resourceInterface dynamic.ResourceInterface
	if rc.namespaced {
		resourceInterface = rc.kcpClient.Resource(rc.gvr).Namespace(namespace)
	} else {
		resourceInterface = rc.kcpClient.Resource(rc.gvr)
	}

	return resourceInterface.Get(ctx, name, metav1.GetOptions{})
}

func (rc *ResourceController) getResourceFromCluster(ctx context.Context, key string) (*unstructured.Unstructured, error) {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return nil, err
	}

	var resourceInterface dynamic.ResourceInterface
	if rc.namespaced {
		resourceInterface = rc.clusterClient.Resource(rc.gvr).Namespace(namespace)
	} else {
		resourceInterface = rc.clusterClient.Resource(rc.gvr)
	}

	return resourceInterface.Get(ctx, name, metav1.GetOptions{})
}

// transformForCluster transforms a KCP resource for application to the cluster
func (rc *ResourceController) transformForCluster(kcpResource *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	// Create a copy for transformation
	clusterResource := kcpResource.DeepCopy()

	// Remove KCP-specific metadata
	unstructured.RemoveNestedField(clusterResource.Object, "metadata", "managedFields")
	unstructured.RemoveNestedField(clusterResource.Object, "metadata", "resourceVersion")
	unstructured.RemoveNestedField(clusterResource.Object, "metadata", "uid")
	unstructured.RemoveNestedField(clusterResource.Object, "metadata", "selfLink")
	unstructured.RemoveNestedField(clusterResource.Object, "metadata", "generation")

	// Add syncer annotations
	annotations := clusterResource.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations["syncer.kcp.io/sync-target"] = rc.syncTargetName
	annotations["syncer.kcp.io/workspace"] = rc.workspaceCluster.String()
	clusterResource.SetAnnotations(annotations)

	rc.transformCount++
	rc.tmcMetrics.RecordResourceTransformation("cluster-sync", rc.gvr.String(), "success")

	return clusterResource, nil
}

// applyToCluster applies a resource to the cluster
func (rc *ResourceController) applyToCluster(ctx context.Context, resource *unstructured.Unstructured) error {
	var resourceInterface dynamic.ResourceInterface
	if rc.namespaced {
		resourceInterface = rc.clusterClient.Resource(rc.gvr).Namespace(resource.GetNamespace())
	} else {
		resourceInterface = rc.clusterClient.Resource(rc.gvr)
	}

	// Try to get existing resource
	existing, err := resourceInterface.Get(ctx, resource.GetName(), metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// Create new resource
			_, err = resourceInterface.Create(ctx, resource, metav1.CreateOptions{})
			return err
		}
		return err
	}

	// Update existing resource
	resource.SetResourceVersion(existing.GetResourceVersion())
	_, err = resourceInterface.Update(ctx, resource, metav1.UpdateOptions{})
	return err
}

// deleteFromCluster deletes a resource from the cluster
func (rc *ResourceController) deleteFromCluster(ctx context.Context, key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	var resourceInterface dynamic.ResourceInterface
	if rc.namespaced {
		resourceInterface = rc.clusterClient.Resource(rc.gvr).Namespace(namespace)
	} else {
		resourceInterface = rc.clusterClient.Resource(rc.gvr)
	}

	return resourceInterface.Delete(ctx, name, metav1.DeleteOptions{})
}

// updateStatusInKCP updates the status of a resource in KCP
func (rc *ResourceController) updateStatusInKCP(ctx context.Context, clusterResource *unstructured.Unstructured) error {
	// Get the key for the KCP resource
	key, err := cache.MetaNamespaceKeyFunc(clusterResource)
	if err != nil {
		return err
	}
	
	// Get the corresponding KCP resource
	kcpResource, err := rc.getResourceFromKCP(ctx, key)
	if err != nil {
		if errors.IsNotFound(err) {
			// KCP resource doesn't exist, nothing to update
			return nil
		}
		return err
	}

	// Extract status from cluster resource
	status, found, err := unstructured.NestedMap(clusterResource.Object, "status")
	if err != nil || !found {
		// No status to sync
		return nil
	}

	// Update status in KCP resource
	if err := unstructured.SetNestedMap(kcpResource.Object, status, "status"); err != nil {
		return err
	}

	// Update in KCP
	var resourceInterface dynamic.ResourceInterface
	if rc.namespaced {
		resourceInterface = rc.kcpClient.Resource(rc.gvr).Namespace(kcpResource.GetNamespace())
	} else {
		resourceInterface = rc.kcpClient.Resource(rc.gvr)
	}

	_, err = resourceInterface.UpdateStatus(ctx, kcpResource, metav1.UpdateOptions{})
	return err
}

// GetStatus returns the current status of the resource controller
func (rc *ResourceController) GetStatus() *ResourceControllerStatus {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	return &ResourceControllerStatus{
		GVR:            rc.gvr,
		SyncTargetName: rc.syncTargetName,
		Started:        rc.started,
		QueueLength:    rc.queue.Len(),
		SyncCount:      rc.syncCount,
		ErrorCount:     rc.errorCount,
		ConflictCount:  rc.conflictCount,
		TransformCount: rc.transformCount,
		LastSyncTime:   rc.lastSyncTime,
	}
}

// ResourceControllerStatus represents the status of a resource controller
type ResourceControllerStatus struct {
	GVR            schema.GroupVersionResource
	SyncTargetName string
	Started        bool
	QueueLength    int
	SyncCount      int64
	ErrorCount     int64
	ConflictCount  int64
	TransformCount int64
	LastSyncTime   time.Time
}

// Helper function to split strings
func split(s, sep string, n int) []string {
	if n == 0 {
		return nil
	}
	if n == 1 {
		return []string{s}
	}
	c := 0
	start := 0
	result := make([]string, 0, n)
	for i := 0; i < len(s) && len(result) < n-1; i++ {
		if s[i:i+len(sep)] == sep {
			result = append(result, s[start:i])
			start = i + len(sep)
			c++
		}
	}
	result = append(result, s[start:])
	return result
}