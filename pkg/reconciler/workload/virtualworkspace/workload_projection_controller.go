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
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

const (
	// WorkloadProjectionControllerName defines the controller name
	WorkloadProjectionControllerName = "workload-projection-controller"

	// Default projection settings
	defaultProjectionInterval = 60 * time.Second
	defaultSyncTimeout        = 5 * time.Minute
	maxConcurrentProjections  = 10
)

// WorkloadProjectionController projects resources from source clusters to target clusters
type WorkloadProjectionController struct {
	virtualWorkspace      *VirtualWorkspace
	dynamicClient         dynamic.Interface
	clusterDynamicClients map[string]dynamic.Interface

	// Projection state
	projectedResources map[schema.GroupVersionKind]*ProjectedResourceView
	lastProjection     time.Time
	mu                 sync.RWMutex

	// Control
	queue       workqueue.RateLimitingInterface
	stopCh      chan struct{}
	syncTrigger chan struct{}
	running     bool

	// Configuration
	projectionInterval time.Duration
	syncTimeout        time.Duration
	concurrency        int

	// Metrics
	projectionCount    int64
	lastProjectionTime time.Time
	errorCount         int64
	resourcesProjected int64
}

// ProjectionRequest represents a request to project specific resources
type ProjectionRequest struct {
	GVK           schema.GroupVersionKind
	SourceCluster string
	TargetCluster string
	ResourceName  string
	Namespace     string
	RequestTime   time.Time
}

// ProjectionTransformationEngine handles resource transformations during projection
type ProjectionTransformationEngine struct {
	transformations map[schema.GroupVersionKind][]ResourceTransformation
	mu              sync.RWMutex
}

// NewWorkloadProjectionController creates a new workload projection controller
func NewWorkloadProjectionController(
	virtualWorkspace *VirtualWorkspace,
	dynamicClient dynamic.Interface,
	clusterDynamicClients map[string]dynamic.Interface,
) (*WorkloadProjectionController, error) {
	queue := workqueue.NewNamedRateLimitingQueue(
		workqueue.DefaultControllerRateLimiter(),
		fmt.Sprintf("%s-%s", WorkloadProjectionControllerName, virtualWorkspace.Name),
	)

	return &WorkloadProjectionController{
		virtualWorkspace:      virtualWorkspace,
		dynamicClient:         dynamicClient,
		clusterDynamicClients: clusterDynamicClients,
		projectedResources:    make(map[schema.GroupVersionKind]*ProjectedResourceView),
		queue:                 queue,
		stopCh:                make(chan struct{}),
		syncTrigger:           make(chan struct{}, 1),
		projectionInterval:    defaultProjectionInterval,
		syncTimeout:           defaultSyncTimeout,
		concurrency:           maxConcurrentProjections,
	}, nil
}

// Start starts the workload projection controller
func (wpc *WorkloadProjectionController) Start(ctx context.Context) {
	defer wpc.queue.ShutDown()

	logger := klog.FromContext(ctx).WithValues(
		"component", WorkloadProjectionControllerName,
		"virtualWorkspace", wpc.virtualWorkspace.Name,
	)
	logger.Info("Starting workload projection controller")
	defer logger.Info("Shutting down workload projection controller")

	wpc.running = true
	defer func() { wpc.running = false }()

	// Start periodic projection
	go wait.UntilWithContext(ctx, wpc.performPeriodicProjection, wpc.projectionInterval)

	// Start sync trigger handler
	go wpc.handleSyncTriggers(ctx)

	// Start worker threads
	for i := 0; i < wpc.concurrency; i++ {
		go wait.UntilWithContext(ctx, wpc.startWorker, time.Second)
	}

	<-ctx.Done()
}

// Stop stops the controller
func (wpc *WorkloadProjectionController) Stop() {
	if wpc.running {
		close(wpc.stopCh)
	}
}

// TriggerSync triggers a synchronization of projected resources
func (wpc *WorkloadProjectionController) TriggerSync() {
	select {
	case wpc.syncTrigger <- struct{}{}:
	default:
		// Channel is full, sync already pending
	}
}

func (wpc *WorkloadProjectionController) startWorker(ctx context.Context) {
	for wpc.processNextWorkItem(ctx) {
	}
}

func (wpc *WorkloadProjectionController) processNextWorkItem(ctx context.Context) bool {
	key, quit := wpc.queue.Get()
	if quit {
		return false
	}
	defer wpc.queue.Done(key)

	logger := klog.FromContext(ctx).WithValues("key", key)
	ctx = klog.NewContext(ctx, logger)

	if err := wpc.processProjectionRequest(ctx, key.(*ProjectionRequest)); err != nil {
		logger.Error(err, "Failed to process projection request")
		wpc.queue.AddRateLimited(key)
		wpc.errorCount++
		return true
	}

	wpc.queue.Forget(key)
	wpc.resourcesProjected++
	return true
}

func (wpc *WorkloadProjectionController) processProjectionRequest(ctx context.Context, request *ProjectionRequest) error {
	logger := klog.FromContext(ctx).WithValues(
		"gvk", request.GVK.String(),
		"sourceCluster", request.SourceCluster,
		"targetCluster", request.TargetCluster,
		"resource", request.ResourceName,
	)

	// Get projection policy for this resource type
	policy := wpc.getProjectionPolicy(request.GVK)

	// Get source resource
	sourceResource, err := wpc.getSourceResource(ctx, request)
	if err != nil {
		return fmt.Errorf("failed to get source resource: %w", err)
	}

	if sourceResource == nil {
		// Resource no longer exists, clean up projections
		return wpc.cleanupProjections(ctx, request)
	}

	// Check if resource should be projected based on policy
	if !wpc.shouldProjectResource(sourceResource, policy) {
		logger.V(4).Info("Resource does not match projection criteria")
		return nil
	}

	// Transform resource for projection
	projectedResource, err := wpc.transformResourceForProjection(ctx, sourceResource, request.TargetCluster, policy)
	if err != nil {
		return fmt.Errorf("failed to transform resource: %w", err)
	}

	// Project resource to target cluster
	if err := wpc.projectResourceToCluster(ctx, projectedResource, request.TargetCluster); err != nil {
		return fmt.Errorf("failed to project resource to cluster %s: %w", request.TargetCluster, err)
	}

	// Update projection tracking
	wpc.updateProjectionTracking(ctx, request, projectedResource)

	logger.V(2).Info("Successfully projected resource")
	return nil
}

func (wpc *WorkloadProjectionController) getSourceResource(ctx context.Context, request *ProjectionRequest) (*unstructured.Unstructured, error) {
	sourceClient, exists := wpc.clusterDynamicClients[request.SourceCluster]
	if !exists {
		return nil, fmt.Errorf("no dynamic client for source cluster %s", request.SourceCluster)
	}

	gvr, err := wpc.getGroupVersionResource(request.GVK)
	if err != nil {
		return nil, fmt.Errorf("failed to get GVR for %s: %w", request.GVK.String(), err)
	}

	var resourceClient dynamic.ResourceInterface
	if request.Namespace != "" {
		resourceClient = sourceClient.Resource(gvr).Namespace(request.Namespace)
	} else {
		resourceClient = sourceClient.Resource(gvr)
	}

	resource, err := resourceClient.Get(ctx, request.ResourceName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil // Resource no longer exists
		}
		return nil, fmt.Errorf("failed to get resource: %w", err)
	}

	return resource, nil
}

func (wpc *WorkloadProjectionController) shouldProjectResource(resource *unstructured.Unstructured, policy *ProjectionPolicy) bool {
	// Check resource selectors
	for _, selector := range policy.ResourceSelectors {
		if selector.GVK != resource.GroupVersionKind() {
			continue
		}

		// Check label selector
		if selector.LabelSelector != nil {
			if !selector.LabelSelector.Matches(labels.Set(resource.GetLabels())) {
				return false
			}
		}

		// Check name pattern
		if selector.NamePattern != "" {
			// Simple pattern matching - in real implementation would use regex
			if resource.GetName() != selector.NamePattern {
				return false
			}
		}

		return true
	}

	// If no selectors match and we have selectors, don't project
	if len(policy.ResourceSelectors) > 0 {
		return false
	}

	// Default to projecting if no specific selectors
	return true
}

func (wpc *WorkloadProjectionController) transformResourceForProjection(
	ctx context.Context,
	sourceResource *unstructured.Unstructured,
	targetCluster string,
	policy *ProjectionPolicy,
) (*unstructured.Unstructured, error) {
	logger := klog.FromContext(ctx).WithValues("targetCluster", targetCluster)

	// Create a deep copy for transformation
	transformed := sourceResource.DeepCopy()

	// Clear cluster-specific fields
	transformed.SetResourceVersion("")
	transformed.SetUID("")
	transformed.SetGeneration(0)
	transformed.SetCreationTimestamp(metav1.Time{})
	transformed.SetDeletionTimestamp(nil)
	transformed.SetDeletionGracePeriodSeconds(nil)

	// Add projection annotations
	annotations := transformed.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[VirtualWorkspaceOriginCluster] = sourceResource.GetClusterName()
	annotations[VirtualWorkspaceProjectionStatus] = string(ProjectionStatusPending)
	annotations["workload.kcp.io/projected-at"] = time.Now().Format(time.RFC3339)
	annotations["workload.kcp.io/projected-to"] = targetCluster
	transformed.SetAnnotations(annotations)

	// Add projection labels
	labels := transformed.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	labels["workload.kcp.io/projected"] = "true"
	labels["workload.kcp.io/projection-source"] = sourceResource.GetClusterName()
	transformed.SetLabels(labels)

	// Apply transformations
	for _, transformation := range policy.Transformations {
		if err := wpc.applyTransformation(transformed, transformation, targetCluster); err != nil {
			logger.Error(err, "Failed to apply transformation", "type", transformation.Type)
			continue
		}
	}

	// Remove status to let target cluster populate it
	unstructured.RemoveNestedField(transformed.Object, "status")

	return transformed, nil
}

func (wpc *WorkloadProjectionController) applyTransformation(
	resource *unstructured.Unstructured,
	transformation ResourceTransformation,
	targetCluster string,
) error {
	switch transformation.Type {
	case TransformationTypeSet:
		return wpc.applySetTransformation(resource, transformation, targetCluster)
	case TransformationTypeDelete:
		return wpc.applyDeleteTransformation(resource, transformation)
	case TransformationTypeReplace:
		return wpc.applyReplaceTransformation(resource, transformation, targetCluster)
	case TransformationTypeTemplate:
		return wpc.applyTemplateTransformation(resource, transformation, targetCluster)
	default:
		return fmt.Errorf("unsupported transformation type: %s", transformation.Type)
	}
}

func (wpc *WorkloadProjectionController) applySetTransformation(
	resource *unstructured.Unstructured,
	transformation ResourceTransformation,
	targetCluster string,
) error {
	// Parse JSONPath and set value
	if transformation.JSONPath == "" {
		return fmt.Errorf("JSONPath is required for Set transformation")
	}

	// Simple implementation - in real world would use proper JSONPath parsing
	if transformation.JSONPath == "metadata.namespace" {
		if value, ok := transformation.Value.(string); ok {
			resource.SetNamespace(value)
		}
	} else if transformation.JSONPath == "metadata.name" {
		if value, ok := transformation.Value.(string); ok {
			resource.SetName(value)
		}
	} else {
		// Generic nested field setting
		path := wpc.parseJSONPath(transformation.JSONPath)
		if err := unstructured.SetNestedField(resource.Object, transformation.Value, path...); err != nil {
			return fmt.Errorf("failed to set nested field %s: %w", transformation.JSONPath, err)
		}
	}

	return nil
}

func (wpc *WorkloadProjectionController) applyDeleteTransformation(
	resource *unstructured.Unstructured,
	transformation ResourceTransformation,
) error {
	if transformation.JSONPath == "" {
		return fmt.Errorf("JSONPath is required for Delete transformation")
	}

	path := wpc.parseJSONPath(transformation.JSONPath)
	unstructured.RemoveNestedField(resource.Object, path...)
	return nil
}

func (wpc *WorkloadProjectionController) applyReplaceTransformation(
	resource *unstructured.Unstructured,
	transformation ResourceTransformation,
	targetCluster string,
) error {
	// First delete, then set
	if err := wpc.applyDeleteTransformation(resource, transformation); err != nil {
		return err
	}
	return wpc.applySetTransformation(resource, transformation, targetCluster)
}

func (wpc *WorkloadProjectionController) applyTemplateTransformation(
	resource *unstructured.Unstructured,
	transformation ResourceTransformation,
	targetCluster string,
) error {
	// Template transformation would replace placeholders in the value
	// For example: "cluster-{{.TargetCluster}}-service"
	if valueStr, ok := transformation.Value.(string); ok {
		// Simple template replacement
		templateValue := valueStr
		templateValue = fmt.Sprintf(templateValue, targetCluster)

		transformation.Value = templateValue
		return wpc.applySetTransformation(resource, transformation, targetCluster)
	}

	return fmt.Errorf("template transformation requires string value")
}

func (wpc *WorkloadProjectionController) parseJSONPath(jsonPath string) []string {
	// Simple JSONPath parsing - in real implementation would use proper parser
	// Convert "metadata.labels.app" to ["metadata", "labels", "app"]
	if jsonPath == "" {
		return []string{}
	}

	// Remove leading dot if present
	if jsonPath[0] == '.' {
		jsonPath = jsonPath[1:]
	}

	// Split by dots
	parts := make([]string, 0)
	for _, part := range splitJSONPath(jsonPath) {
		if part != "" {
			parts = append(parts, part)
		}
	}

	return parts
}

func splitJSONPath(path string) []string {
	// Simple split by dots - real implementation would handle array indices, etc.
	result := make([]string, 0)
	current := ""

	for _, char := range path {
		if char == '.' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(char)
		}
	}

	if current != "" {
		result = append(result, current)
	}

	return result
}

func (wpc *WorkloadProjectionController) projectResourceToCluster(
	ctx context.Context,
	resource *unstructured.Unstructured,
	targetCluster string,
) error {
	logger := klog.FromContext(ctx).WithValues("targetCluster", targetCluster)

	targetClient, exists := wpc.clusterDynamicClients[targetCluster]
	if !exists {
		return fmt.Errorf("no dynamic client for target cluster %s", targetCluster)
	}

	gvr, err := wpc.getGroupVersionResource(resource.GroupVersionKind())
	if err != nil {
		return fmt.Errorf("failed to get GVR: %w", err)
	}

	var resourceClient dynamic.ResourceInterface
	if resource.GetNamespace() != "" {
		resourceClient = targetClient.Resource(gvr).Namespace(resource.GetNamespace())
	} else {
		resourceClient = targetClient.Resource(gvr)
	}

	// Try to get existing resource
	existing, err := resourceClient.Get(ctx, resource.GetName(), metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// Create new resource
			logger.V(3).Info("Creating new projected resource")
			_, err := resourceClient.Create(ctx, resource, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("failed to create resource: %w", err)
			}
			return nil
		}
		return fmt.Errorf("failed to get existing resource: %w", err)
	}

	// Update existing resource
	logger.V(3).Info("Updating existing projected resource")

	// Preserve target cluster specific fields
	resource.SetResourceVersion(existing.GetResourceVersion())
	resource.SetUID(existing.GetUID())
	resource.SetCreationTimestamp(existing.GetCreationTimestamp())

	// Merge annotations to preserve target cluster annotations
	existingAnnotations := existing.GetAnnotations()
	newAnnotations := resource.GetAnnotations()
	if existingAnnotations != nil {
		for key, value := range existingAnnotations {
			if !isProjectionAnnotation(key) {
				newAnnotations[key] = value
			}
		}
		resource.SetAnnotations(newAnnotations)
	}

	_, err = resourceClient.Update(ctx, resource, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update resource: %w", err)
	}

	return nil
}

func isProjectionAnnotation(key string) bool {
	projectionAnnotations := []string{
		VirtualWorkspaceOriginCluster,
		VirtualWorkspaceProjectionStatus,
		"workload.kcp.io/projected-at",
		"workload.kcp.io/projected-to",
	}

	for _, annotation := range projectionAnnotations {
		if key == annotation {
			return true
		}
	}

	return false
}

func (wpc *WorkloadProjectionController) updateProjectionTracking(
	ctx context.Context,
	request *ProjectionRequest,
	projectedResource *unstructured.Unstructured,
) {
	wpc.mu.Lock()
	defer wpc.mu.Unlock()

	gvk := request.GVK
	projectionView, exists := wpc.projectedResources[gvk]
	if !exists {
		projectionView = &ProjectedResourceView{
			GVK:             gvk,
			SourceResources: make(map[string]*unstructured.Unstructured),
			ProjectedTo:     make(map[string]*ProjectedResourceInstance),
			LastProjected:   time.Now(),
		}
		wpc.projectedResources[gvk] = projectionView
	}

	// Update source resource tracking
	sourceKey := wpc.getResourceKey(projectedResource)
	projectionView.SourceResources[sourceKey] = projectedResource

	// Update projection instance tracking
	projectionKey := fmt.Sprintf("%s/%s", request.TargetCluster, sourceKey)
	projectionView.ProjectedTo[projectionKey] = &ProjectedResourceInstance{
		ClusterName:       request.TargetCluster,
		ProjectedResource: projectedResource,
		Status:            ProjectionStatusActive,
		LastProjected:     time.Now(),
	}

	projectionView.LastProjected = time.Now()

	// Update virtual workspace tracking
	wpc.virtualWorkspace.ProjectedResources[gvk] = projectionView
}

func (wpc *WorkloadProjectionController) cleanupProjections(ctx context.Context, request *ProjectionRequest) error {
	logger := klog.FromContext(ctx).WithValues("cleanup", true)

	// Remove from target cluster
	targetClient, exists := wpc.clusterDynamicClients[request.TargetCluster]
	if !exists {
		logger.V(4).Info("No client for target cluster", "cluster", request.TargetCluster)
		return nil
	}

	gvr, err := wpc.getGroupVersionResource(request.GVK)
	if err != nil {
		return fmt.Errorf("failed to get GVR: %w", err)
	}

	var resourceClient dynamic.ResourceInterface
	if request.Namespace != "" {
		resourceClient = targetClient.Resource(gvr).Namespace(request.Namespace)
	} else {
		resourceClient = targetClient.Resource(gvr)
	}

	err = resourceClient.Delete(ctx, request.ResourceName, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete projected resource: %w", err)
	}

	// Update tracking
	wpc.removeProjectionTracking(request)

	logger.V(2).Info("Cleaned up projected resource")
	return nil
}

func (wpc *WorkloadProjectionController) removeProjectionTracking(request *ProjectionRequest) {
	wpc.mu.Lock()
	defer wpc.mu.Unlock()

	gvk := request.GVK
	projectionView, exists := wpc.projectedResources[gvk]
	if !exists {
		return
	}

	resourceKey := fmt.Sprintf("%s/%s", request.Namespace, request.ResourceName)
	if request.Namespace == "" {
		resourceKey = request.ResourceName
	}

	projectionKey := fmt.Sprintf("%s/%s", request.TargetCluster, resourceKey)
	delete(projectionView.ProjectedTo, projectionKey)

	// If no more projections for this resource, remove source tracking
	hasProjections := false
	for key := range projectionView.ProjectedTo {
		if key[len(request.TargetCluster)+1:] == resourceKey {
			hasProjections = true
			break
		}
	}

	if !hasProjections {
		delete(projectionView.SourceResources, resourceKey)
	}

	// If no resources in this view, remove it
	if len(projectionView.SourceResources) == 0 {
		delete(wpc.projectedResources, gvk)
		delete(wpc.virtualWorkspace.ProjectedResources, gvk)
	}
}

func (wpc *WorkloadProjectionController) performPeriodicProjection(ctx context.Context) {
	logger := klog.FromContext(ctx).WithValues("component", "PeriodicProjection")

	// Define common resource types to project
	commonGVKs := []schema.GroupVersionKind{
		{Group: "apps", Version: "v1", Kind: "Deployment"},
		{Group: "apps", Version: "v1", Kind: "StatefulSet"},
		{Group: "apps", Version: "v1", Kind: "DaemonSet"},
		{Group: "", Version: "v1", Kind: "Service"},
		{Group: "", Version: "v1", Kind: "ConfigMap"},
		{Group: "", Version: "v1", Kind: "Secret"},
	}

	// Project from each source cluster to each target cluster
	for _, sourceCluster := range wpc.virtualWorkspace.TargetClusters {
		if !sourceCluster.Healthy {
			continue
		}

		for _, targetCluster := range wpc.virtualWorkspace.TargetClusters {
			if targetCluster.Name == sourceCluster.Name || !targetCluster.Healthy {
				continue
			}

			for _, gvk := range commonGVKs {
				request := &ProjectionRequest{
					GVK:           gvk,
					SourceCluster: sourceCluster.Name,
					TargetCluster: targetCluster.Name,
					RequestTime:   time.Now(),
				}
				wpc.queue.Add(request)
			}
		}
	}

	wpc.projectionCount++
	wpc.lastProjectionTime = time.Now()

	logger.V(4).Info("Triggered periodic projection", "gvkCount", len(commonGVKs))
}

func (wpc *WorkloadProjectionController) handleSyncTriggers(ctx context.Context) {
	logger := klog.FromContext(ctx).WithValues("component", "SyncTriggerHandler")

	for {
		select {
		case <-ctx.Done():
			return
		case <-wpc.stopCh:
			return
		case <-wpc.syncTrigger:
			logger.V(4).Info("Received sync trigger")
			wpc.performPeriodicProjection(ctx)
		}
	}
}

// Helper methods

func (wpc *WorkloadProjectionController) getProjectionPolicy(gvk schema.GroupVersionKind) *ProjectionPolicy {
	// Return default policy for now
	// In a real implementation, this would be configurable per resource type
	return &ProjectionPolicy{
		Mode:           ProjectionModeSelective,
		TargetClusters: []string{}, // Empty means all clusters
		ResourceSelectors: []ResourceSelector{
			{
				GVK: gvk,
			},
		},
		Transformations: []ResourceTransformation{
			{
				Type:     TransformationTypeSet,
				JSONPath: "metadata.labels.workload.kcp.io/projected",
				Value:    "true",
			},
		},
	}
}

func (wpc *WorkloadProjectionController) getGroupVersionResource(gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
	// This is a simplified mapping - in reality, you'd use RESTMapper
	resource := gvk.Kind
	if resource[len(resource)-1] == 'y' {
		resource = resource[:len(resource)-1] + "ies"
	} else if resource[len(resource)-1] == 's' {
		resource = resource + "es"
	} else {
		resource = resource + "s"
	}

	return schema.GroupVersionResource{
		Group:    gvk.Group,
		Version:  gvk.Version,
		Resource: resource,
	}, nil
}

func (wpc *WorkloadProjectionController) getResourceKey(resource *unstructured.Unstructured) string {
	if resource.GetNamespace() != "" {
		return fmt.Sprintf("%s/%s", resource.GetNamespace(), resource.GetName())
	}
	return resource.GetName()
}

// GetProjectedResources returns all currently projected resources
func (wpc *WorkloadProjectionController) GetProjectedResources() map[schema.GroupVersionKind]*ProjectedResourceView {
	wpc.mu.RLock()
	defer wpc.mu.RUnlock()

	result := make(map[schema.GroupVersionKind]*ProjectedResourceView)
	for gvk, view := range wpc.projectedResources {
		result[gvk] = view
	}
	return result
}

// GetProjectedResource returns projected resources of a specific type
func (wpc *WorkloadProjectionController) GetProjectedResource(gvk schema.GroupVersionKind) (*ProjectedResourceView, bool) {
	wpc.mu.RLock()
	defer wpc.mu.RUnlock()

	view, exists := wpc.projectedResources[gvk]
	return view, exists
}

// GetProjectionStatus returns the status of projections for a virtual workspace
func (wpc *WorkloadProjectionController) GetProjectionStatus() map[string]interface{} {
	wpc.mu.RLock()
	defer wpc.mu.RUnlock()

	totalProjections := 0
	activeProjections := 0

	for _, view := range wpc.projectedResources {
		totalProjections += len(view.ProjectedTo)
		for _, instance := range view.ProjectedTo {
			if instance.Status == ProjectionStatusActive {
				activeProjections++
			}
		}
	}

	return map[string]interface{}{
		"totalProjections":   totalProjections,
		"activeProjections":  activeProjections,
		"projectionCount":    wpc.projectionCount,
		"lastProjectionTime": wpc.lastProjectionTime,
		"errorCount":         wpc.errorCount,
		"resourcesProjected": wpc.resourcesProjected,
	}
}
