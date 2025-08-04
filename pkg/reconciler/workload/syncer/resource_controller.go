/*
Copyright 2022 The KCP Authors.

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
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/kcp/cmd/workload-syncer/options"
)

// ResourceController handles synchronization for a specific resource type
type ResourceController struct {
	gvr schema.GroupVersionResource

	// Clients
	kcpClient     dynamic.Interface
	clusterClient dynamic.Interface

	// Informers
	kcpInformer     cache.SharedIndexInformer
	clusterInformer cache.SharedIndexInformer

	// Work queues
	kcpQueue     workqueue.RateLimitingInterface
	clusterQueue workqueue.RateLimitingInterface

	// Configuration
	options *options.SyncerOptions

	// TMC integration (placeholder for future enhancements)

	// State management
	started   bool
	stopCh    chan struct{}
	waitGroup sync.WaitGroup
	mu        sync.RWMutex

	// Metrics
	syncedResources   int64
	syncErrors        int64
	lastSyncTime      time.Time
	metricsLock       sync.RWMutex
}

// ResourceControllerOptions contains options for creating a resource controller
type ResourceControllerOptions struct {
	GVR                    schema.GroupVersionResource
	KCPInformerFactory     dynamicinformer.DynamicSharedInformerFactory
	ClusterInformerFactory dynamicinformer.DynamicSharedInformerFactory
	KCPClient              dynamic.Interface
	ClusterClient   dynamic.Interface
	SyncerOptions   *options.SyncerOptions
}

// NewResourceController creates a new resource controller
func NewResourceController(ctx context.Context, opts ResourceControllerOptions) (*ResourceController, error) {
	rc := &ResourceController{
		gvr:             opts.GVR,
		kcpClient:     opts.KCPClient,
		clusterClient: opts.ClusterClient,
		options:       opts.SyncerOptions,
		stopCh:        make(chan struct{}),
	}

	// Create work queues
	rc.kcpQueue = workqueue.NewNamedRateLimitingQueue(
		workqueue.DefaultControllerRateLimiter(),
		fmt.Sprintf("kcp-%s", opts.GVR.Resource),
	)
	rc.clusterQueue = workqueue.NewNamedRateLimitingQueue(
		workqueue.DefaultControllerRateLimiter(),
		fmt.Sprintf("cluster-%s", opts.GVR.Resource),
	)

	// Set up informers
	rc.kcpInformer = opts.KCPInformerFactory.ForResource(opts.GVR).Informer()
	rc.clusterInformer = opts.ClusterInformerFactory.ForResource(opts.GVR).Informer()

	// Add event handlers
	rc.kcpInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    rc.onKCPResourceAdded,
		UpdateFunc: rc.onKCPResourceUpdated,
		DeleteFunc: rc.onKCPResourceDeleted,
	})

	rc.clusterInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    rc.onClusterResourceAdded,
		UpdateFunc: rc.onClusterResourceUpdated,
		DeleteFunc: rc.onClusterResourceDeleted,
	})

	return rc, nil
}

// Start starts the resource controller
func (rc *ResourceController) Start(ctx context.Context, workers int) error {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	if rc.started {
		return fmt.Errorf("resource controller for %s is already started", rc.gvr)
	}

	logger := klog.FromContext(ctx).WithValues("gvr", rc.gvr)
	logger.Info("Starting resource controller")

	// Start informers
	go rc.kcpInformer.Run(rc.stopCh)
	go rc.clusterInformer.Run(rc.stopCh)

	// Wait for caches to sync
	if !cache.WaitForCacheSync(rc.stopCh, rc.kcpInformer.HasSynced, rc.clusterInformer.HasSynced) {
		return fmt.Errorf("failed to wait for %s informer caches to sync", rc.gvr)
	}

	// Start worker goroutines
	for i := 0; i < workers; i++ {
		rc.waitGroup.Add(2)
		go rc.runKCPWorker(ctx)
		go rc.runClusterWorker(ctx)
	}

	rc.started = true
	logger.Info("Resource controller started successfully")
	return nil
}

// Stop stops the resource controller
func (rc *ResourceController) Stop(ctx context.Context) error {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	if !rc.started {
		return nil
	}

	logger := klog.FromContext(ctx).WithValues("gvr", rc.gvr)
	logger.Info("Stopping resource controller")

	// Signal workers to stop
	close(rc.stopCh)

	// Shutdown work queues
	rc.kcpQueue.ShutDown()
	rc.clusterQueue.ShutDown()

	// Wait for workers to finish
	done := make(chan struct{})
	go func() {
		rc.waitGroup.Wait()
		close(done)
	}()

	select {
	case <-done:
		logger.Info("Resource controller stopped")
	case <-ctx.Done():
		logger.Info("Resource controller shutdown context cancelled")
	}

	rc.started = false
	return nil
}

// runKCPWorker runs a worker that processes KCP resource changes
func (rc *ResourceController) runKCPWorker(ctx context.Context) {
	defer rc.waitGroup.Done()
	defer handlePanic(fmt.Sprintf("kcp-worker-%s", rc.gvr))

	logger := klog.FromContext(ctx).WithValues("gvr", rc.gvr, "worker", "kcp")
	
	for rc.processNextKCPWorkItem(ctx) {
	}
	
	logger.Info("KCP worker finished")
}

// runClusterWorker runs a worker that processes cluster resource changes
func (rc *ResourceController) runClusterWorker(ctx context.Context) {
	defer rc.waitGroup.Done()
	defer handlePanic(fmt.Sprintf("cluster-worker-%s", rc.gvr))

	logger := klog.FromContext(ctx).WithValues("gvr", rc.gvr, "worker", "cluster")
	
	for rc.processNextClusterWorkItem(ctx) {
	}
	
	logger.Info("Cluster worker finished")
}

// processNextKCPWorkItem processes the next item from the KCP work queue
func (rc *ResourceController) processNextKCPWorkItem(ctx context.Context) bool {
	obj, shutdown := rc.kcpQueue.Get()
	if shutdown {
		return false
	}
	defer rc.kcpQueue.Done(obj)

	err := rc.syncKCPResource(ctx, obj.(string))
	if err == nil {
		rc.kcpQueue.Forget(obj)
		return true
	}

	// Handle error with TMC error categorization
	tmcErr := rc.categorizeAndHandleError(err, "kcp-sync", obj.(string))
	
	logger := klog.FromContext(ctx).WithValues("gvr", rc.gvr, "key", obj, "error", tmcErr)
	
	if rc.kcpQueue.NumRequeues(obj) < 5 {
		logger.Info("Retrying KCP resource sync")
		rc.kcpQueue.AddRateLimited(obj)
	} else {
		logger.Info("Dropping KCP resource sync after too many retries")
		rc.kcpQueue.Forget(obj)
		runtime.HandleError(tmcErr)
	}

	return true
}

// processNextClusterWorkItem processes the next item from the cluster work queue
func (rc *ResourceController) processNextClusterWorkItem(ctx context.Context) bool {
	obj, shutdown := rc.clusterQueue.Get()
	if shutdown {
		return false
	}
	defer rc.clusterQueue.Done(obj)

	err := rc.syncClusterResource(ctx, obj.(string))
	if err == nil {
		rc.clusterQueue.Forget(obj)
		return true
	}

	// Handle error with TMC error categorization
	tmcErr := rc.categorizeAndHandleError(err, "cluster-sync", obj.(string))
	
	logger := klog.FromContext(ctx).WithValues("gvr", rc.gvr, "key", obj, "error", tmcErr)
	
	if rc.clusterQueue.NumRequeues(obj) < 5 {
		logger.Info("Retrying cluster resource sync")
		rc.clusterQueue.AddRateLimited(obj)
	} else {
		logger.Info("Dropping cluster resource sync after too many retries")
		rc.clusterQueue.Forget(obj)
		runtime.HandleError(tmcErr)
	}

	return true
}

// syncKCPResource syncs a resource from KCP to the physical cluster
func (rc *ResourceController) syncKCPResource(ctx context.Context, key string) error {
	startTime := time.Now()
	defer func() {
		rc.updateSyncMetrics(time.Since(startTime))
	}()

	logger := klog.FromContext(ctx).WithValues("gvr", rc.gvr, "key", key, "direction", "kcp-to-cluster")

	// Get the resource from KCP
	obj, exists, err := rc.kcpInformer.GetIndexer().GetByKey(key)
	if err != nil {
		return fmt.Errorf("failed to get resource from KCP informer: %w", err)
	}

	if !exists {
		// Resource was deleted in KCP, delete from cluster
		return rc.deleteResourceInCluster(ctx, key)
	}

	unstructuredObj := obj.(*unstructured.Unstructured)
	
	// Transform resource for cluster
	clusterObj, err := rc.transformForCluster(unstructuredObj.DeepCopy())
	if err != nil {
		return fmt.Errorf("failed to transform resource for cluster: %w", err)
	}

	// Apply resource to cluster
	if err := rc.applyResourceToCluster(ctx, clusterObj); err != nil {
		return fmt.Errorf("failed to apply resource to cluster: %w", err)
	}

	logger.Info("Successfully synced resource from KCP to cluster")
	return nil
}

// syncClusterResource syncs a resource from the physical cluster to KCP
func (rc *ResourceController) syncClusterResource(ctx context.Context, key string) error {
	startTime := time.Now()
	defer func() {
		rc.updateSyncMetrics(time.Since(startTime))
	}()

	logger := klog.FromContext(ctx).WithValues("gvr", rc.gvr, "key", key, "direction", "cluster-to-kcp")

	// Get the resource from cluster
	obj, exists, err := rc.clusterInformer.GetIndexer().GetByKey(key)
	if err != nil {
		return fmt.Errorf("failed to get resource from cluster informer: %w", err)
	}

	if !exists {
		// Resource was deleted in cluster, handle accordingly
		return rc.handleClusterResourceDeletion(ctx, key)
	}

	unstructuredObj := obj.(*unstructured.Unstructured)
	
	// Transform resource status for KCP
	kcpStatusUpdate, err := rc.transformStatusForKCP(unstructuredObj.DeepCopy())
	if err != nil {
		return fmt.Errorf("failed to transform status for KCP: %w", err)
	}

	// Update status in KCP
	if err := rc.updateStatusInKCP(ctx, kcpStatusUpdate); err != nil {
		return fmt.Errorf("failed to update status in KCP: %w", err)
	}

	logger.Info("Successfully synced resource status from cluster to KCP")
	return nil
}

// Event handlers
func (rc *ResourceController) onKCPResourceAdded(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(fmt.Errorf("couldn't get key for object %+v: %v", obj, err))
		return
	}
	rc.kcpQueue.Add(key)
}

func (rc *ResourceController) onKCPResourceUpdated(oldObj, newObj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(newObj)
	if err != nil {
		runtime.HandleError(fmt.Errorf("couldn't get key for object %+v: %v", newObj, err))
		return
	}
	rc.kcpQueue.Add(key)
}

func (rc *ResourceController) onKCPResourceDeleted(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(fmt.Errorf("couldn't get key for object %+v: %v", obj, err))
		return
	}
	rc.kcpQueue.Add(key)
}

func (rc *ResourceController) onClusterResourceAdded(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(fmt.Errorf("couldn't get key for object %+v: %v", obj, err))
		return
	}
	rc.clusterQueue.Add(key)
}

func (rc *ResourceController) onClusterResourceUpdated(oldObj, newObj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(newObj)
	if err != nil {
		runtime.HandleError(fmt.Errorf("couldn't get key for object %+v: %v", newObj, err))
		return
	}
	rc.clusterQueue.Add(key)
}

func (rc *ResourceController) onClusterResourceDeleted(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(fmt.Errorf("couldn't get key for object %+v: %v", obj, err))
		return
	}
	rc.clusterQueue.Add(key)
}

// Helper methods
func (rc *ResourceController) transformForCluster(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	// Remove KCP-specific annotations and labels
	annotations := obj.GetAnnotations()
	if annotations != nil {
		delete(annotations, "kcp.io/cluster")
		obj.SetAnnotations(annotations)
	}

	// Clear resource version and UID for cluster creation
	obj.SetResourceVersion("")
	obj.SetUID("")
	obj.SetSelfLink("")

	return obj, nil
}

func (rc *ResourceController) transformStatusForKCP(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	// Extract only status information
	status, found, err := unstructured.NestedMap(obj.Object, "status")
	if err != nil {
		return nil, fmt.Errorf("failed to extract status: %w", err)
	}
	if !found {
		return nil, nil // No status to sync
	}

	// Create a minimal object with just status
	statusObj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"name":      obj.GetName(),
				"namespace": obj.GetNamespace(),
			},
			"status": status,
		},
	}
	statusObj.SetGroupVersionKind(obj.GetObjectKind().GroupVersionKind())

	return statusObj, nil
}

func (rc *ResourceController) applyResourceToCluster(ctx context.Context, obj *unstructured.Unstructured) error {
	var resourceInterface dynamic.ResourceInterface = rc.clusterClient.Resource(rc.gvr)
	if obj.GetNamespace() != "" {
		resourceInterface = rc.clusterClient.Resource(rc.gvr).Namespace(obj.GetNamespace())
	}

	// Try to create, if it exists, update
	_, err := resourceInterface.Create(ctx, obj, metav1.CreateOptions{})
	if errors.IsAlreadyExists(err) {
		_, err = resourceInterface.Update(ctx, obj, metav1.UpdateOptions{})
	}
	return err
}

func (rc *ResourceController) updateStatusInKCP(ctx context.Context, obj *unstructured.Unstructured) error {
	if obj == nil {
		return nil
	}

	var resourceInterface dynamic.ResourceInterface = rc.kcpClient.Resource(rc.gvr)
	if obj.GetNamespace() != "" {
		resourceInterface = rc.kcpClient.Resource(rc.gvr).Namespace(obj.GetNamespace())
	}

	_, err := resourceInterface.UpdateStatus(ctx, obj, metav1.UpdateOptions{})
	return err
}

func (rc *ResourceController) deleteResourceInCluster(ctx context.Context, key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return fmt.Errorf("invalid key format: %w", err)
	}

	var resourceInterface dynamic.ResourceInterface = rc.clusterClient.Resource(rc.gvr)
	if namespace != "" {
		resourceInterface = rc.clusterClient.Resource(rc.gvr).Namespace(namespace)
	}

	err = resourceInterface.Delete(ctx, name, metav1.DeleteOptions{})
	if errors.IsNotFound(err) {
		return nil // Already deleted
	}
	return err
}

func (rc *ResourceController) handleClusterResourceDeletion(ctx context.Context, key string) error {
	// For now, we don't delete resources in KCP when they're deleted in the cluster
	// This behavior should be configurable based on the sync policy
	return nil
}

func (rc *ResourceController) categorizeAndHandleError(err error, operation, resource string) error {
	// For now, just wrap the error with context
	// TODO: Integrate with TMC error handling system
	wrappedErr := fmt.Errorf("%s failed for resource %s: %w", operation, resource, err)
	
	// Increment error metrics
	rc.metricsLock.Lock()
	rc.syncErrors++
	rc.metricsLock.Unlock()

	return wrappedErr
}

func (rc *ResourceController) updateSyncMetrics(duration time.Duration) {
	rc.metricsLock.Lock()
	defer rc.metricsLock.Unlock()
	
	rc.syncedResources++
	rc.lastSyncTime = time.Now()
}

// GetMetrics returns controller metrics
func (rc *ResourceController) GetMetrics() map[string]interface{} {
	rc.metricsLock.RLock()
	defer rc.metricsLock.RUnlock()

	return map[string]interface{}{
		"synced_resources": rc.syncedResources,
		"sync_errors":      rc.syncErrors,
		"last_sync_time":   rc.lastSyncTime,
		"started":          rc.started,
	}
}