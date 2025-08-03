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

package virtualworkspace

import (
	"context"
	"fmt"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"

	workloadv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/workload/v1alpha1"
	kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
	workloadinformers "github.com/kcp-dev/kcp/sdk/client/informers/externalversions/workload/v1alpha1"
	workloadlisters "github.com/kcp-dev/kcp/sdk/client/listers/workload/v1alpha1"
)

const (
	// VirtualWorkspaceManagerName defines the virtual workspace manager name
	VirtualWorkspaceManagerName = "virtual-workspace-manager"

	// VirtualWorkspaceFinalizer protects virtual workspaces during cleanup
	VirtualWorkspaceFinalizer = "workload.kcp.io/virtual-workspace-finalizer"

	// Virtual workspace annotations and labels
	VirtualWorkspaceOriginCluster    = "workload.kcp.io/origin-cluster"
	VirtualWorkspaceSourcePlacement  = "workload.kcp.io/source-placement"
	VirtualWorkspaceAggregatedFrom   = "workload.kcp.io/aggregated-from"
	VirtualWorkspaceProjectionStatus = "workload.kcp.io/projection-status"
)

// VirtualWorkspaceManager manages virtual workspace views of distributed workloads
type VirtualWorkspaceManager struct {
	queue workqueue.RateLimitingInterface

	kcpClusterClient      kcpclientset.ClusterInterface
	dynamicClient         dynamic.Interface
	clusterDynamicClients map[string]dynamic.Interface

	placementLister  workloadlisters.PlacementClusterLister
	placementIndexer cache.Indexer

	syncTargetLister  workloadlisters.SyncTargetClusterLister
	syncTargetIndexer cache.Indexer

	// Virtual workspace management
	virtualWorkspaces     map[string]*VirtualWorkspace
	resourceAggregators   map[string]*CrossClusterResourceAggregator
	projectionControllers map[string]*WorkloadProjectionController
	mu                    sync.RWMutex

	// Configuration
	syncInterval       time.Duration
	aggregationEnabled bool
	projectionEnabled  bool
}

// VirtualWorkspace represents a virtual view of distributed workloads
type VirtualWorkspace struct {
	Name                string
	Namespace           string
	LogicalCluster      logicalcluster.Name
	SourcePlacement     *workloadv1alpha1.Placement
	TargetClusters      []ClusterReference
	AggregatedResources map[schema.GroupVersionKind]*AggregatedResourceView
	ProjectedResources  map[schema.GroupVersionKind]*ProjectedResourceView
	Status              VirtualWorkspaceStatus
	CreatedTime         time.Time
	LastUpdated         time.Time
}

// ClusterReference identifies a target cluster
type ClusterReference struct {
	Name           string
	LogicalCluster logicalcluster.Name
	SyncTarget     *workloadv1alpha1.SyncTarget
	Healthy        bool
	LastSeen       time.Time
}

// AggregatedResourceView represents an aggregated view of resources across clusters
type AggregatedResourceView struct {
	GVK            schema.GroupVersionKind
	Resources      map[string]*AggregatedResource
	TotalCount     int
	HealthyCount   int
	UnhealthyCount int
	LastAggregated time.Time
}

// AggregatedResource represents a single resource aggregated from multiple clusters
type AggregatedResource struct {
	Name           string
	Namespace      string
	ClusterOrigins map[string]*ClusterResourceInstance
	AggregatedSpec *unstructured.Unstructured
	Status         AggregatedResourceStatus
	Conditions     []metav1.Condition
}

// ClusterResourceInstance represents a resource instance in a specific cluster
type ClusterResourceInstance struct {
	ClusterName  string
	Resource     *unstructured.Unstructured
	Health       ResourceHealth
	LastObserved time.Time
	Conditions   []metav1.Condition
}

// ProjectedResourceView represents a projected view of resources to target clusters
type ProjectedResourceView struct {
	GVK              schema.GroupVersionKind
	SourceResources  map[string]*unstructured.Unstructured
	ProjectedTo      map[string]*ProjectedResourceInstance
	ProjectionPolicy ProjectionPolicy
	LastProjected    time.Time
}

// ProjectedResourceInstance represents a resource projected to a specific cluster
type ProjectedResourceInstance struct {
	ClusterName       string
	ProjectedResource *unstructured.Unstructured
	Status            ProjectionStatus
	Error             error
	LastProjected     time.Time
}

// VirtualWorkspaceStatus represents the status of a virtual workspace
type VirtualWorkspaceStatus string

const (
	VirtualWorkspaceStatusPending       VirtualWorkspaceStatus = "Pending"
	VirtualWorkspaceStatusActive        VirtualWorkspaceStatus = "Active"
	VirtualWorkspaceStatusSynchronizing VirtualWorkspaceStatus = "Synchronizing"
	VirtualWorkspaceStatusError         VirtualWorkspaceStatus = "Error"
	VirtualWorkspaceStatusTerminating   VirtualWorkspaceStatus = "Terminating"
)

// AggregatedResourceStatus represents the aggregated status of a resource
type AggregatedResourceStatus string

const (
	AggregatedResourceStatusHealthy   AggregatedResourceStatus = "Healthy"
	AggregatedResourceStatusDegraded  AggregatedResourceStatus = "Degraded"
	AggregatedResourceStatusUnhealthy AggregatedResourceStatus = "Unhealthy"
	AggregatedResourceStatusUnknown   AggregatedResourceStatus = "Unknown"
)

// ProjectionStatus represents the status of a resource projection
type ProjectionStatus string

const (
	ProjectionStatusPending   ProjectionStatus = "Pending"
	ProjectionStatusActive    ProjectionStatus = "Active"
	ProjectionStatusFailed    ProjectionStatus = "Failed"
	ProjectionStatusOutOfSync ProjectionStatus = "OutOfSync"
)

// ResourceHealth represents the health of a resource
type ResourceHealth string

const (
	ResourceHealthHealthy   ResourceHealth = "Healthy"
	ResourceHealthDegraded  ResourceHealth = "Degraded"
	ResourceHealthUnhealthy ResourceHealth = "Unhealthy"
	ResourceHealthUnknown   ResourceHealth = "Unknown"
)

// ProjectionPolicy defines how resources should be projected
type ProjectionPolicy struct {
	Mode              ProjectionMode
	TargetClusters    []string
	ResourceSelectors []ResourceSelector
	Transformations   []ResourceTransformation
}

// ProjectionMode defines the projection strategy
type ProjectionMode string

const (
	ProjectionModeAll         ProjectionMode = "All"         // Project to all clusters
	ProjectionModeSelective   ProjectionMode = "Selective"   // Project to selected clusters
	ProjectionModeConditional ProjectionMode = "Conditional" // Project based on conditions
)

// ResourceSelector defines criteria for selecting resources to project
type ResourceSelector struct {
	GVK           schema.GroupVersionKind
	LabelSelector labels.Selector
	FieldSelector string
	NamePattern   string
}

// ResourceTransformation defines how resources should be transformed during projection
type ResourceTransformation struct {
	Type        TransformationType
	JSONPath    string
	Value       interface{}
	Conditional string
}

// TransformationType defines the type of transformation
type TransformationType string

const (
	TransformationTypeSet      TransformationType = "Set"
	TransformationTypeDelete   TransformationType = "Delete"
	TransformationTypeReplace  TransformationType = "Replace"
	TransformationTypeTemplate TransformationType = "Template"
)

// NewVirtualWorkspaceManager creates a new virtual workspace manager
func NewVirtualWorkspaceManager(
	kcpClusterClient kcpclientset.ClusterInterface,
	dynamicClient dynamic.Interface,
	placementInformer workloadinformers.PlacementClusterInformer,
	syncTargetInformer workloadinformers.SyncTargetClusterInformer,
) (*VirtualWorkspaceManager, error) {
	queue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), VirtualWorkspaceManagerName)

	vwm := &VirtualWorkspaceManager{
		queue:                 queue,
		kcpClusterClient:      kcpClusterClient,
		dynamicClient:         dynamicClient,
		clusterDynamicClients: make(map[string]dynamic.Interface),
		placementLister:       placementInformer.Lister(),
		placementIndexer:      placementInformer.Informer().GetIndexer(),
		syncTargetLister:      syncTargetInformer.Lister(),
		syncTargetIndexer:     syncTargetInformer.Informer().GetIndexer(),
		virtualWorkspaces:     make(map[string]*VirtualWorkspace),
		resourceAggregators:   make(map[string]*CrossClusterResourceAggregator),
		projectionControllers: make(map[string]*WorkloadProjectionController),
		syncInterval:          30 * time.Second,
		aggregationEnabled:    true,
		projectionEnabled:     true,
	}

	// Set up event handlers
	_, _ = placementInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    vwm.onPlacementAdd,
		UpdateFunc: vwm.onPlacementUpdate,
		DeleteFunc: vwm.onPlacementDelete,
	})

	_, _ = syncTargetInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    vwm.onSyncTargetAdd,
		UpdateFunc: vwm.onSyncTargetUpdate,
		DeleteFunc: vwm.onSyncTargetDelete,
	})

	return vwm, nil
}

// Start starts the virtual workspace manager
func (vwm *VirtualWorkspaceManager) Start(ctx context.Context, numThreads int) {
	defer vwm.queue.ShutDown()

	logger := klog.FromContext(ctx).WithValues("controller", VirtualWorkspaceManagerName)
	logger.Info("Starting virtual workspace manager")
	defer logger.Info("Shutting down virtual workspace manager")

	// Start periodic synchronization
	go wait.UntilWithContext(ctx, vwm.syncVirtualWorkspaces, vwm.syncInterval)

	// Start worker threads
	for i := 0; i < numThreads; i++ {
		go wait.UntilWithContext(ctx, vwm.startWorker, time.Second)
	}

	<-ctx.Done()
}

func (vwm *VirtualWorkspaceManager) startWorker(ctx context.Context) {
	for vwm.processNextWorkItem(ctx) {
	}
}

func (vwm *VirtualWorkspaceManager) processNextWorkItem(ctx context.Context) bool {
	key, quit := vwm.queue.Get()
	if quit {
		return false
	}
	defer vwm.queue.Done(key)

	logger := klog.FromContext(ctx).WithValues("key", key)
	ctx = klog.NewContext(ctx, logger)
	err := vwm.processOne(ctx, key.(string))
	if err == nil {
		vwm.queue.Forget(key)
		return true
	}

	logger.Error(err, "Failed to sync virtual workspace")
	vwm.queue.AddRateLimited(key)
	return true
}

func (vwm *VirtualWorkspaceManager) processOne(ctx context.Context, key string) error {
	logger := klog.FromContext(ctx)

	clusterName, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return fmt.Errorf("failed to split key %s: %w", key, err)
	}
	cluster := logicalcluster.Name(clusterName)

	placement, err := vwm.placementLister.Cluster(cluster).Get(name)
	if errors.IsNotFound(err) {
		logger.V(2).Info("placement deleted, cleaning up virtual workspace")
		return vwm.cleanupVirtualWorkspace(ctx, cluster, name)
	}
	if err != nil {
		return fmt.Errorf("failed to get placement %s|%s: %w", clusterName, name, err)
	}

	return vwm.reconcileVirtualWorkspace(ctx, placement)
}

// reconcileVirtualWorkspace creates or updates a virtual workspace for a placement
func (vwm *VirtualWorkspaceManager) reconcileVirtualWorkspace(ctx context.Context, placement *workloadv1alpha1.Placement) error {
	logger := klog.FromContext(ctx).WithValues("placement", placement.Name)

	vwm.mu.Lock()
	defer vwm.mu.Unlock()

	virtualWorkspaceKey := vwm.getVirtualWorkspaceKey(placement)
	virtualWorkspace, exists := vwm.virtualWorkspaces[virtualWorkspaceKey]

	if !exists {
		// Create new virtual workspace
		virtualWorkspace = &VirtualWorkspace{
			Name:                placement.Name,
			Namespace:           placement.Namespace,
			LogicalCluster:      logicalcluster.From(placement),
			SourcePlacement:     placement,
			TargetClusters:      make([]ClusterReference, 0),
			AggregatedResources: make(map[schema.GroupVersionKind]*AggregatedResourceView),
			ProjectedResources:  make(map[schema.GroupVersionKind]*ProjectedResourceView),
			Status:              VirtualWorkspaceStatusPending,
			CreatedTime:         time.Now(),
			LastUpdated:         time.Now(),
		}
		vwm.virtualWorkspaces[virtualWorkspaceKey] = virtualWorkspace
		logger.Info("Created virtual workspace", "key", virtualWorkspaceKey)
	}

	// Update target clusters from placement
	if err := vwm.updateVirtualWorkspaceTargets(ctx, virtualWorkspace, placement); err != nil {
		return fmt.Errorf("failed to update virtual workspace targets: %w", err)
	}

	// Initialize or update resource aggregators
	if vwm.aggregationEnabled {
		if err := vwm.ensureResourceAggregator(ctx, virtualWorkspace); err != nil {
			logger.Error(err, "Failed to ensure resource aggregator")
		}
	}

	// Initialize or update projection controllers
	if vwm.projectionEnabled {
		if err := vwm.ensureProjectionController(ctx, virtualWorkspace); err != nil {
			logger.Error(err, "Failed to ensure projection controller")
		}
	}

	virtualWorkspace.Status = VirtualWorkspaceStatusActive
	virtualWorkspace.LastUpdated = time.Now()

	logger.V(2).Info("Successfully reconciled virtual workspace",
		"targetClusters", len(virtualWorkspace.TargetClusters),
		"aggregatedResources", len(virtualWorkspace.AggregatedResources),
		"projectedResources", len(virtualWorkspace.ProjectedResources))

	return nil
}

// updateVirtualWorkspaceTargets updates the target clusters for a virtual workspace
func (vwm *VirtualWorkspaceManager) updateVirtualWorkspaceTargets(ctx context.Context, vw *VirtualWorkspace, placement *workloadv1alpha1.Placement) error {
	logger := klog.FromContext(ctx)

	// Clear existing targets
	vw.TargetClusters = vw.TargetClusters[:0]

	// Add clusters from placement status
	for _, workloadCluster := range placement.Status.SelectedWorkloadClusters {
		cluster := logicalcluster.Name(workloadCluster.Cluster)

		// Get sync target for additional information
		syncTarget, err := vwm.syncTargetLister.Cluster(cluster).Get(workloadCluster.Name)
		if err != nil {
			logger.V(4).Info("Could not find sync target", "cluster", workloadCluster.Name, "error", err)
			syncTarget = nil
		}

		clusterRef := ClusterReference{
			Name:           workloadCluster.Name,
			LogicalCluster: cluster,
			SyncTarget:     syncTarget,
			Healthy:        vwm.isClusterHealthy(syncTarget),
			LastSeen:       time.Now(),
		}

		vw.TargetClusters = append(vw.TargetClusters, clusterRef)
	}

	logger.V(3).Info("Updated virtual workspace targets", "count", len(vw.TargetClusters))
	return nil
}

// isClusterHealthy determines if a cluster is healthy
func (vwm *VirtualWorkspaceManager) isClusterHealthy(syncTarget *workloadv1alpha1.SyncTarget) bool {
	if syncTarget == nil {
		return false
	}

	// Check ready condition
	for _, condition := range syncTarget.Status.Conditions {
		if condition.Type == workloadv1alpha1.SyncTargetReady {
			return condition.Status == metav1.ConditionTrue
		}
	}

	return false
}

// ensureResourceAggregator ensures a resource aggregator exists for the virtual workspace
func (vwm *VirtualWorkspaceManager) ensureResourceAggregator(ctx context.Context, vw *VirtualWorkspace) error {
	aggregatorKey := vwm.getAggregatorKey(vw)

	if _, exists := vwm.resourceAggregators[aggregatorKey]; !exists {
		aggregator, err := NewCrossClusterResourceAggregator(vw, vwm.dynamicClient, vwm.clusterDynamicClients)
		if err != nil {
			return fmt.Errorf("failed to create resource aggregator: %w", err)
		}

		vwm.resourceAggregators[aggregatorKey] = aggregator

		// Start aggregator in background
		go aggregator.Start(ctx)
	}

	return nil
}

// ensureProjectionController ensures a projection controller exists for the virtual workspace
func (vwm *VirtualWorkspaceManager) ensureProjectionController(ctx context.Context, vw *VirtualWorkspace) error {
	controllerKey := vwm.getProjectionControllerKey(vw)

	if _, exists := vwm.projectionControllers[controllerKey]; !exists {
		controller, err := NewWorkloadProjectionController(vw, vwm.dynamicClient, vwm.clusterDynamicClients)
		if err != nil {
			return fmt.Errorf("failed to create projection controller: %w", err)
		}

		vwm.projectionControllers[controllerKey] = controller

		// Start controller in background
		go controller.Start(ctx)
	}

	return nil
}

// cleanupVirtualWorkspace cleans up a virtual workspace when placement is deleted
func (vwm *VirtualWorkspaceManager) cleanupVirtualWorkspace(ctx context.Context, cluster logicalcluster.Name, placementName string) error {
	logger := klog.FromContext(ctx).WithValues("cluster", cluster, "placement", placementName)

	vwm.mu.Lock()
	defer vwm.mu.Unlock()

	virtualWorkspaceKey := fmt.Sprintf("%s/%s", cluster, placementName)

	// Clean up resource aggregator
	aggregatorKey := virtualWorkspaceKey
	if aggregator, exists := vwm.resourceAggregators[aggregatorKey]; exists {
		aggregator.Stop()
		delete(vwm.resourceAggregators, aggregatorKey)
	}

	// Clean up projection controller
	controllerKey := virtualWorkspaceKey
	if controller, exists := vwm.projectionControllers[controllerKey]; exists {
		controller.Stop()
		delete(vwm.projectionControllers, controllerKey)
	}

	// Remove virtual workspace
	if vw, exists := vwm.virtualWorkspaces[virtualWorkspaceKey]; exists {
		vw.Status = VirtualWorkspaceStatusTerminating
		delete(vwm.virtualWorkspaces, virtualWorkspaceKey)
		logger.Info("Cleaned up virtual workspace")
	}

	return nil
}

// syncVirtualWorkspaces performs periodic synchronization of all virtual workspaces
func (vwm *VirtualWorkspaceManager) syncVirtualWorkspaces(ctx context.Context) {
	logger := klog.FromContext(ctx).WithValues("component", "VirtualWorkspaceSync")

	vwm.mu.RLock()
	workspaces := make([]*VirtualWorkspace, 0, len(vwm.virtualWorkspaces))
	for _, vw := range vwm.virtualWorkspaces {
		workspaces = append(workspaces, vw)
	}
	vwm.mu.RUnlock()

	for _, vw := range workspaces {
		if err := vwm.syncVirtualWorkspace(ctx, vw); err != nil {
			logger.Error(err, "Failed to sync virtual workspace", "workspace", vw.Name)
		}
	}

	logger.V(4).Info("Completed virtual workspace sync", "count", len(workspaces))
}

// syncVirtualWorkspace synchronizes a single virtual workspace
func (vwm *VirtualWorkspaceManager) syncVirtualWorkspace(ctx context.Context, vw *VirtualWorkspace) error {
	logger := klog.FromContext(ctx).WithValues("workspace", vw.Name)

	// Update cluster health status
	for i := range vw.TargetClusters {
		cluster := &vw.TargetClusters[i]
		cluster.Healthy = vwm.isClusterHealthy(cluster.SyncTarget)
		cluster.LastSeen = time.Now()
	}

	// Trigger aggregation if enabled
	if vwm.aggregationEnabled {
		aggregatorKey := vwm.getAggregatorKey(vw)
		if aggregator, exists := vwm.resourceAggregators[aggregatorKey]; exists {
			aggregator.TriggerSync()
		}
	}

	// Trigger projection if enabled
	if vwm.projectionEnabled {
		controllerKey := vwm.getProjectionControllerKey(vw)
		if controller, exists := vwm.projectionControllers[controllerKey]; exists {
			controller.TriggerSync()
		}
	}

	vw.LastUpdated = time.Now()
	logger.V(4).Info("Synced virtual workspace")

	return nil
}

// Event handlers

func (vwm *VirtualWorkspaceManager) onPlacementAdd(obj interface{}) {
	placement, ok := obj.(*workloadv1alpha1.Placement)
	if !ok {
		return
	}
	vwm.enqueuePlacement(placement)
}

func (vwm *VirtualWorkspaceManager) onPlacementUpdate(oldObj, newObj interface{}) {
	placement, ok := newObj.(*workloadv1alpha1.Placement)
	if !ok {
		return
	}
	vwm.enqueuePlacement(placement)
}

func (vwm *VirtualWorkspaceManager) onPlacementDelete(obj interface{}) {
	placement, ok := obj.(*workloadv1alpha1.Placement)
	if !ok {
		// Handle tombstone
		if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
			placement, ok = tombstone.Obj.(*workloadv1alpha1.Placement)
			if !ok {
				return
			}
		} else {
			return
		}
	}
	vwm.enqueuePlacement(placement)
}

func (vwm *VirtualWorkspaceManager) onSyncTargetAdd(obj interface{}) {
	vwm.onSyncTargetChange(obj)
}

func (vwm *VirtualWorkspaceManager) onSyncTargetUpdate(oldObj, newObj interface{}) {
	vwm.onSyncTargetChange(newObj)
}

func (vwm *VirtualWorkspaceManager) onSyncTargetDelete(obj interface{}) {
	vwm.onSyncTargetChange(obj)
}

func (vwm *VirtualWorkspaceManager) onSyncTargetChange(obj interface{}) {
	syncTarget, ok := obj.(*workloadv1alpha1.SyncTarget)
	if !ok {
		if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
			syncTarget, ok = tombstone.Obj.(*workloadv1alpha1.SyncTarget)
			if !ok {
				return
			}
		} else {
			return
		}
	}

	// Find placements that use this sync target and re-queue them
	cluster := logicalcluster.From(syncTarget)
	placements, err := vwm.placementLister.Cluster(cluster).List(labels.Everything())
	if err != nil {
		klog.Error(err, "Failed to list placements for sync target change")
		return
	}

	for _, placement := range placements {
		for _, workloadCluster := range placement.Status.SelectedWorkloadClusters {
			if workloadCluster.Name == syncTarget.Name {
				vwm.enqueuePlacement(placement)
				break
			}
		}
	}
}

func (vwm *VirtualWorkspaceManager) enqueuePlacement(placement *workloadv1alpha1.Placement) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(placement)
	if err != nil {
		klog.Error(err, "Failed to create key for placement")
		return
	}
	vwm.queue.Add(key)
}

// Helper methods

func (vwm *VirtualWorkspaceManager) getVirtualWorkspaceKey(placement *workloadv1alpha1.Placement) string {
	return fmt.Sprintf("%s/%s", logicalcluster.From(placement), placement.Name)
}

func (vwm *VirtualWorkspaceManager) getAggregatorKey(vw *VirtualWorkspace) string {
	return fmt.Sprintf("%s/%s", vw.LogicalCluster, vw.Name)
}

func (vwm *VirtualWorkspaceManager) getProjectionControllerKey(vw *VirtualWorkspace) string {
	return fmt.Sprintf("%s/%s", vw.LogicalCluster, vw.Name)
}

// Public API methods

// GetVirtualWorkspace returns a virtual workspace by key
func (vwm *VirtualWorkspaceManager) GetVirtualWorkspace(cluster logicalcluster.Name, name string) (*VirtualWorkspace, bool) {
	vwm.mu.RLock()
	defer vwm.mu.RUnlock()

	key := fmt.Sprintf("%s/%s", cluster, name)
	vw, exists := vwm.virtualWorkspaces[key]
	return vw, exists
}

// ListVirtualWorkspaces returns all virtual workspaces
func (vwm *VirtualWorkspaceManager) ListVirtualWorkspaces() []*VirtualWorkspace {
	vwm.mu.RLock()
	defer vwm.mu.RUnlock()

	workspaces := make([]*VirtualWorkspace, 0, len(vwm.virtualWorkspaces))
	for _, vw := range vwm.virtualWorkspaces {
		workspaces = append(workspaces, vw)
	}

	return workspaces
}

// GetVirtualWorkspaceStatus returns status information for a virtual workspace
func (vwm *VirtualWorkspaceManager) GetVirtualWorkspaceStatus(cluster logicalcluster.Name, name string) (VirtualWorkspaceStatus, bool) {
	vw, exists := vwm.GetVirtualWorkspace(cluster, name)
	if !exists {
		return VirtualWorkspaceStatusPending, false
	}
	return vw.Status, true
}
