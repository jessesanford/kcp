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
	// CrossClusterResourceAggregatorName defines the aggregator name
	CrossClusterResourceAggregatorName = "cross-cluster-resource-aggregator"

	// Default aggregation settings
	defaultAggregationInterval = 30 * time.Second
	defaultResourceAge         = 5 * time.Minute
	maxConcurrentAggregations  = 10
)

// CrossClusterResourceAggregator aggregates resources across multiple clusters
type CrossClusterResourceAggregator struct {
	virtualWorkspace      *VirtualWorkspace
	dynamicClient         dynamic.Interface
	clusterDynamicClients map[string]dynamic.Interface

	// Aggregation state
	aggregatedResources map[schema.GroupVersionKind]*AggregatedResourceView
	lastAggregation     time.Time
	mu                  sync.RWMutex

	// Control
	queue       workqueue.RateLimitingInterface
	stopCh      chan struct{}
	syncTrigger chan struct{}
	running     bool

	// Configuration
	aggregationInterval time.Duration
	resourceAge         time.Duration
	concurrency         int

	// Metrics
	aggregationCount    int64
	lastAggregationTime time.Time
	errorCount          int64
	resourcesProcessed  int64
}

// AggregationRequest represents a request to aggregate specific resources
type AggregationRequest struct {
	GVK          schema.GroupVersionKind
	ClusterName  string
	Namespace    string
	ResourceName string
	RequestTime  time.Time
}

// ResourceMergeStrategy defines how resources from different clusters should be merged
type ResourceMergeStrategy string

const (
	MergeStrategyUnion        ResourceMergeStrategy = "Union"        // Combine all instances
	MergeStrategyIntersection ResourceMergeStrategy = "Intersection" // Only common instances
	MergeStrategyPriority     ResourceMergeStrategy = "Priority"     // Use priority-based selection
	MergeStrategyLatest       ResourceMergeStrategy = "Latest"       // Use most recently updated
)

// AggregationPolicy defines how resources should be aggregated
type AggregationPolicy struct {
	GVK                schema.GroupVersionKind
	MergeStrategy      ResourceMergeStrategy
	ConflictResolution ConflictResolutionStrategy
	HealthAggregation  HealthAggregationStrategy
	StatusMerging      StatusMergingStrategy
	LabelSelectors     []labels.Selector
	FieldSelectors     []string
	Transformations    []AggregationTransformation
}

// ConflictResolutionStrategy defines how to resolve conflicts between cluster instances
type ConflictResolutionStrategy string

const (
	ConflictResolutionLastWriter    ConflictResolutionStrategy = "LastWriter"
	ConflictResolutionFirstWriter   ConflictResolutionStrategy = "FirstWriter"
	ConflictResolutionClusterWeight ConflictResolutionStrategy = "ClusterWeight"
	ConflictResolutionManual        ConflictResolutionStrategy = "Manual"
)

// HealthAggregationStrategy defines how to aggregate health across clusters
type HealthAggregationStrategy string

const (
	HealthAggregationAll      HealthAggregationStrategy = "All"      // All must be healthy
	HealthAggregationMajority HealthAggregationStrategy = "Majority" // Majority must be healthy
	HealthAggregationAny      HealthAggregationStrategy = "Any"      // Any healthy is enough
	HealthAggregationWeighted HealthAggregationStrategy = "Weighted" // Weighted by cluster priority
)

// StatusMergingStrategy defines how to merge status from multiple clusters
type StatusMergingStrategy string

const (
	StatusMergingCombined   StatusMergingStrategy = "Combined"   // Combine all statuses
	StatusMergingPrimary    StatusMergingStrategy = "Primary"    // Use primary cluster status
	StatusMergingAggregated StatusMergingStrategy = "Aggregated" // Create aggregated status
)

// AggregationTransformation defines transformations to apply during aggregation
type AggregationTransformation struct {
	Type            AggregationTransformationType
	JSONPath        string
	Value           interface{}
	Conditional     string
	ClusterSpecific map[string]interface{}
}

// AggregationTransformationType defines the type of aggregation transformation
type AggregationTransformationType string

const (
	AggregationTransformSet     AggregationTransformationType = "Set"
	AggregationTransformMerge   AggregationTransformationType = "Merge"
	AggregationTransformReplace AggregationTransformationType = "Replace"
	AggregationTransformCompute AggregationTransformationType = "Compute"
)

// NewCrossClusterResourceAggregator creates a new cross-cluster resource aggregator
func NewCrossClusterResourceAggregator(
	virtualWorkspace *VirtualWorkspace,
	dynamicClient dynamic.Interface,
	clusterDynamicClients map[string]dynamic.Interface,
) (*CrossClusterResourceAggregator, error) {
	queue := workqueue.NewNamedRateLimitingQueue(
		workqueue.DefaultControllerRateLimiter(),
		fmt.Sprintf("%s-%s", CrossClusterResourceAggregatorName, virtualWorkspace.Name),
	)

	return &CrossClusterResourceAggregator{
		virtualWorkspace:      virtualWorkspace,
		dynamicClient:         dynamicClient,
		clusterDynamicClients: clusterDynamicClients,
		aggregatedResources:   make(map[schema.GroupVersionKind]*AggregatedResourceView),
		queue:                 queue,
		stopCh:                make(chan struct{}),
		syncTrigger:           make(chan struct{}, 1),
		aggregationInterval:   defaultAggregationInterval,
		resourceAge:           defaultResourceAge,
		concurrency:           maxConcurrentAggregations,
	}, nil
}

// Start starts the cross-cluster resource aggregator
func (ccra *CrossClusterResourceAggregator) Start(ctx context.Context) {
	defer ccra.queue.ShutDown()

	logger := klog.FromContext(ctx).WithValues(
		"component", CrossClusterResourceAggregatorName,
		"virtualWorkspace", ccra.virtualWorkspace.Name,
	)
	logger.Info("Starting cross-cluster resource aggregator")
	defer logger.Info("Shutting down cross-cluster resource aggregator")

	ccra.running = true
	defer func() { ccra.running = false }()

	// Start periodic aggregation
	go wait.UntilWithContext(ctx, ccra.performPeriodicAggregation, ccra.aggregationInterval)

	// Start sync trigger handler
	go ccra.handleSyncTriggers(ctx)

	// Start worker threads
	for i := 0; i < ccra.concurrency; i++ {
		go wait.UntilWithContext(ctx, ccra.startWorker, time.Second)
	}

	<-ctx.Done()
}

// Stop stops the aggregator
func (ccra *CrossClusterResourceAggregator) Stop() {
	if ccra.running {
		close(ccra.stopCh)
	}
}

// TriggerSync triggers a synchronization of aggregated resources
func (ccra *CrossClusterResourceAggregator) TriggerSync() {
	select {
	case ccra.syncTrigger <- struct{}{}:
	default:
		// Channel is full, sync already pending
	}
}

func (ccra *CrossClusterResourceAggregator) startWorker(ctx context.Context) {
	for ccra.processNextWorkItem(ctx) {
	}
}

func (ccra *CrossClusterResourceAggregator) processNextWorkItem(ctx context.Context) bool {
	key, quit := ccra.queue.Get()
	if quit {
		return false
	}
	defer ccra.queue.Done(key)

	logger := klog.FromContext(ctx).WithValues("key", key)
	ctx = klog.NewContext(ctx, logger)

	if err := ccra.processAggregationRequest(ctx, key.(*AggregationRequest)); err != nil {
		logger.Error(err, "Failed to process aggregation request")
		ccra.queue.AddRateLimited(key)
		ccra.errorCount++
		return true
	}

	ccra.queue.Forget(key)
	ccra.resourcesProcessed++
	return true
}

func (ccra *CrossClusterResourceAggregator) processAggregationRequest(ctx context.Context, request *AggregationRequest) error {
	logger := klog.FromContext(ctx).WithValues(
		"gvk", request.GVK.String(),
		"cluster", request.ClusterName,
		"resource", request.ResourceName,
	)

	// Get aggregation policy for this resource type
	policy := ccra.getAggregationPolicy(request.GVK)

	// Aggregate resources of this type from all target clusters
	aggregatedView, err := ccra.aggregateResourcesOfType(ctx, request.GVK, policy)
	if err != nil {
		return fmt.Errorf("failed to aggregate resources of type %s: %w", request.GVK.String(), err)
	}

	// Update the aggregated view
	ccra.mu.Lock()
	ccra.aggregatedResources[request.GVK] = aggregatedView
	ccra.virtualWorkspace.AggregatedResources[request.GVK] = aggregatedView
	ccra.mu.Unlock()

	logger.V(2).Info("Successfully aggregated resources",
		"totalCount", aggregatedView.TotalCount,
		"healthyCount", aggregatedView.HealthyCount,
		"unhealthyCount", aggregatedView.UnhealthyCount)

	return nil
}

func (ccra *CrossClusterResourceAggregator) aggregateResourcesOfType(
	ctx context.Context,
	gvk schema.GroupVersionKind,
	policy *AggregationPolicy,
) (*AggregatedResourceView, error) {
	logger := klog.FromContext(ctx).WithValues("gvk", gvk.String())

	view := &AggregatedResourceView{
		GVK:            gvk,
		Resources:      make(map[string]*AggregatedResource),
		TotalCount:     0,
		HealthyCount:   0,
		UnhealthyCount: 0,
		LastAggregated: time.Now(),
	}

	// Collect resources from all target clusters
	clusterResources := make(map[string]map[string]*unstructured.Unstructured)

	for _, clusterRef := range ccra.virtualWorkspace.TargetClusters {
		if !clusterRef.Healthy {
			logger.V(4).Info("Skipping unhealthy cluster", "cluster", clusterRef.Name)
			continue
		}

		clusterClient, exists := ccra.clusterDynamicClients[clusterRef.Name]
		if !exists {
			logger.V(4).Info("No dynamic client for cluster", "cluster", clusterRef.Name)
			continue
		}

		resources, err := ccra.getResourcesFromCluster(ctx, clusterClient, gvk, policy)
		if err != nil {
			logger.Error(err, "Failed to get resources from cluster", "cluster", clusterRef.Name)
			continue
		}

		clusterResources[clusterRef.Name] = resources
	}

	// Aggregate resources using the specified strategy
	if err := ccra.mergeResourcesUsingStrategy(ctx, view, clusterResources, policy); err != nil {
		return nil, fmt.Errorf("failed to merge resources: %w", err)
	}

	// Calculate health statistics
	ccra.calculateAggregatedHealth(view, policy)

	logger.V(3).Info("Completed resource aggregation",
		"resourceCount", len(view.Resources),
		"totalCount", view.TotalCount,
		"healthyCount", view.HealthyCount)

	return view, nil
}

func (ccra *CrossClusterResourceAggregator) getResourcesFromCluster(
	ctx context.Context,
	clusterClient dynamic.Interface,
	gvk schema.GroupVersionKind,
	policy *AggregationPolicy,
) (map[string]*unstructured.Unstructured, error) {
	resources := make(map[string]*unstructured.Unstructured)

	// Get the resource interface for this GVK
	gvr, err := ccra.getGroupVersionResource(gvk)
	if err != nil {
		return nil, fmt.Errorf("failed to get GVR for %s: %w", gvk.String(), err)
	}

	var resourceClient dynamic.ResourceInterface
	if gvr.Resource == "namespaces" || gvr.Resource == "nodes" {
		// Cluster-scoped resources
		resourceClient = clusterClient.Resource(gvr)
	} else {
		// Namespaced resources - list across all namespaces
		resourceClient = clusterClient.Resource(gvr).Namespace("")
	}

	// List resources with label selectors if specified
	listOptions := metav1.ListOptions{}
	if len(policy.LabelSelectors) > 0 {
		// Use the first label selector for simplicity
		listOptions.LabelSelector = policy.LabelSelectors[0].String()
	}

	list, err := resourceClient.List(ctx, listOptions)
	if err != nil {
		if errors.IsNotFound(err) || meta.IsNoMatchError(err) {
			// Resource type doesn't exist in this cluster
			return resources, nil
		}
		return nil, fmt.Errorf("failed to list resources: %w", err)
	}

	// Process each resource
	for _, item := range list.Items {
		// Apply field selectors if specified
		if len(policy.FieldSelectors) > 0 && !ccra.matchesFieldSelectors(&item, policy.FieldSelectors) {
			continue
		}

		// Apply transformations if specified
		transformedItem := item.DeepCopy()
		if err := ccra.applyTransformations(transformedItem, policy.Transformations); err != nil {
			klog.V(4).Info("Failed to apply transformations", "resource", item.GetName(), "error", err)
			continue
		}

		resourceKey := ccra.getResourceKey(&item)
		resources[resourceKey] = transformedItem
	}

	return resources, nil
}

func (ccra *CrossClusterResourceAggregator) mergeResourcesUsingStrategy(
	ctx context.Context,
	view *AggregatedResourceView,
	clusterResources map[string]map[string]*unstructured.Unstructured,
	policy *AggregationPolicy,
) error {
	logger := klog.FromContext(ctx)

	// Collect all unique resource keys across clusters
	allResourceKeys := make(map[string]bool)
	for _, resources := range clusterResources {
		for resourceKey := range resources {
			allResourceKeys[resourceKey] = true
		}
	}

	// Process each unique resource
	for resourceKey := range allResourceKeys {
		clusterInstances := make(map[string]*ClusterResourceInstance)

		// Collect instances from all clusters
		for clusterName, resources := range clusterResources {
			if resource, exists := resources[resourceKey]; exists {
				instance := &ClusterResourceInstance{
					ClusterName:  clusterName,
					Resource:     resource,
					Health:       ccra.assessResourceHealth(resource),
					LastObserved: time.Now(),
					Conditions:   ccra.extractResourceConditions(resource),
				}
				clusterInstances[clusterName] = instance
			}
		}

		// Create aggregated resource
		aggregatedResource, err := ccra.createAggregatedResource(resourceKey, clusterInstances, policy)
		if err != nil {
			logger.Error(err, "Failed to create aggregated resource", "resource", resourceKey)
			continue
		}

		view.Resources[resourceKey] = aggregatedResource
		view.TotalCount++
	}

	return nil
}

func (ccra *CrossClusterResourceAggregator) createAggregatedResource(
	resourceKey string,
	clusterInstances map[string]*ClusterResourceInstance,
	policy *AggregationPolicy,
) (*AggregatedResource, error) {
	if len(clusterInstances) == 0 {
		return nil, fmt.Errorf("no cluster instances provided")
	}

	// Get a reference instance (use the first one)
	var referenceInstance *ClusterResourceInstance
	var resourceName, resourceNamespace string
	for _, instance := range clusterInstances {
		referenceInstance = instance
		resourceName = instance.Resource.GetName()
		resourceNamespace = instance.Resource.GetNamespace()
		break
	}

	aggregatedResource := &AggregatedResource{
		Name:           resourceName,
		Namespace:      resourceNamespace,
		ClusterOrigins: clusterInstances,
		Status:         ccra.aggregateResourceStatus(clusterInstances, policy),
		Conditions:     ccra.aggregateResourceConditions(clusterInstances, policy),
	}

	// Create aggregated spec based on merge strategy
	var err error
	switch policy.MergeStrategy {
	case MergeStrategyUnion:
		aggregatedResource.AggregatedSpec, err = ccra.mergeSpecsUnion(clusterInstances)
	case MergeStrategyIntersection:
		aggregatedResource.AggregatedSpec, err = ccra.mergeSpecsIntersection(clusterInstances)
	case MergeStrategyPriority:
		aggregatedResource.AggregatedSpec, err = ccra.mergeSpecsPriority(clusterInstances)
	case MergeStrategyLatest:
		aggregatedResource.AggregatedSpec, err = ccra.mergeSpecsLatest(clusterInstances)
	default:
		// Default to using the reference instance
		aggregatedResource.AggregatedSpec = referenceInstance.Resource.DeepCopy()
	}

	if err != nil {
		return nil, fmt.Errorf("failed to merge specs: %w", err)
	}

	return aggregatedResource, nil
}

func (ccra *CrossClusterResourceAggregator) assessResourceHealth(resource *unstructured.Unstructured) ResourceHealth {
	// Extract conditions from the resource
	conditions, found, err := unstructured.NestedSlice(resource.Object, "status", "conditions")
	if err != nil || !found {
		return ResourceHealthUnknown
	}

	// Check for common health conditions
	for _, conditionObj := range conditions {
		condition, ok := conditionObj.(map[string]interface{})
		if !ok {
			continue
		}

		conditionType, found, err := unstructured.NestedString(condition, "type")
		if err != nil || !found {
			continue
		}

		status, found, err := unstructured.NestedString(condition, "status")
		if err != nil || !found {
			continue
		}

		// Check for common "healthy" condition types
		switch conditionType {
		case "Ready", "Available", "Healthy":
			if status == "True" {
				return ResourceHealthHealthy
			} else {
				return ResourceHealthUnhealthy
			}
		case "Failed", "Error":
			if status == "True" {
				return ResourceHealthUnhealthy
			}
		}
	}

	// Check resource phase/state
	phase, found, err := unstructured.NestedString(resource.Object, "status", "phase")
	if err == nil && found {
		switch phase {
		case "Running", "Active", "Bound", "Available":
			return ResourceHealthHealthy
		case "Failed", "Error", "Terminating":
			return ResourceHealthUnhealthy
		case "Pending", "Provisioning":
			return ResourceHealthDegraded
		}
	}

	return ResourceHealthUnknown
}

func (ccra *CrossClusterResourceAggregator) extractResourceConditions(resource *unstructured.Unstructured) []metav1.Condition {
	conditions := make([]metav1.Condition, 0)

	conditionsRaw, found, err := unstructured.NestedSlice(resource.Object, "status", "conditions")
	if err != nil || !found {
		return conditions
	}

	for _, conditionObj := range conditionsRaw {
		conditionMap, ok := conditionObj.(map[string]interface{})
		if !ok {
			continue
		}

		condition := metav1.Condition{}
		if conditionType, found, err := unstructured.NestedString(conditionMap, "type"); err == nil && found {
			condition.Type = conditionType
		}
		if status, found, err := unstructured.NestedString(conditionMap, "status"); err == nil && found {
			condition.Status = metav1.ConditionStatus(status)
		}
		if reason, found, err := unstructured.NestedString(conditionMap, "reason"); err == nil && found {
			condition.Reason = reason
		}
		if message, found, err := unstructured.NestedString(conditionMap, "message"); err == nil && found {
			condition.Message = message
		}

		conditions = append(conditions, condition)
	}

	return conditions
}

func (ccra *CrossClusterResourceAggregator) aggregateResourceStatus(
	clusterInstances map[string]*ClusterResourceInstance,
	policy *AggregationPolicy,
) AggregatedResourceStatus {
	healthyClusters := 0
	unhealthyClusters := 0
	degradedClusters := 0

	for _, instance := range clusterInstances {
		switch instance.Health {
		case ResourceHealthHealthy:
			healthyClusters++
		case ResourceHealthUnhealthy:
			unhealthyClusters++
		case ResourceHealthDegraded:
			degradedClusters++
		}
	}

	// Apply health aggregation strategy
	switch policy.HealthAggregation {
	case HealthAggregationAll:
		if unhealthyClusters > 0 {
			return AggregatedResourceStatusUnhealthy
		} else if degradedClusters > 0 {
			return AggregatedResourceStatusDegraded
		} else if healthyClusters > 0 {
			return AggregatedResourceStatusHealthy
		}
		return AggregatedResourceStatusUnknown

	case HealthAggregationMajority:
		totalClusters := len(clusterInstances)
		if healthyClusters > totalClusters/2 {
			return AggregatedResourceStatusHealthy
		} else if unhealthyClusters > totalClusters/2 {
			return AggregatedResourceStatusUnhealthy
		} else {
			return AggregatedResourceStatusDegraded
		}

	case HealthAggregationAny:
		if healthyClusters > 0 {
			return AggregatedResourceStatusHealthy
		} else if degradedClusters > 0 {
			return AggregatedResourceStatusDegraded
		} else if unhealthyClusters > 0 {
			return AggregatedResourceStatusUnhealthy
		}
		return AggregatedResourceStatusUnknown

	default:
		// Default behavior similar to majority
		if healthyClusters >= unhealthyClusters {
			return AggregatedResourceStatusHealthy
		} else {
			return AggregatedResourceStatusUnhealthy
		}
	}
}

func (ccra *CrossClusterResourceAggregator) aggregateResourceConditions(
	clusterInstances map[string]*ClusterResourceInstance,
	policy *AggregationPolicy,
) []metav1.Condition {
	// For now, return conditions from the first healthy instance
	for _, instance := range clusterInstances {
		if instance.Health == ResourceHealthHealthy {
			return instance.Conditions
		}
	}

	// If no healthy instances, return conditions from any instance
	for _, instance := range clusterInstances {
		return instance.Conditions
	}

	return []metav1.Condition{}
}

func (ccra *CrossClusterResourceAggregator) calculateAggregatedHealth(view *AggregatedResourceView, policy *AggregationPolicy) {
	for _, resource := range view.Resources {
		switch resource.Status {
		case AggregatedResourceStatusHealthy:
			view.HealthyCount++
		case AggregatedResourceStatusUnhealthy:
			view.UnhealthyCount++
		}
	}
}

// Merge strategy implementations

func (ccra *CrossClusterResourceAggregator) mergeSpecsUnion(clusterInstances map[string]*ClusterResourceInstance) (*unstructured.Unstructured, error) {
	// Start with the first instance as base
	var base *unstructured.Unstructured
	for _, instance := range clusterInstances {
		base = instance.Resource.DeepCopy()
		break
	}

	if base == nil {
		return nil, fmt.Errorf("no instances to merge")
	}

	// For union strategy, we primarily use the base but could merge arrays/maps
	// This is a simplified implementation
	return base, nil
}

func (ccra *CrossClusterResourceAggregator) mergeSpecsIntersection(clusterInstances map[string]*ClusterResourceInstance) (*unstructured.Unstructured, error) {
	// For intersection strategy, keep only common fields
	// This is a simplified implementation that returns the first instance
	for _, instance := range clusterInstances {
		return instance.Resource.DeepCopy(), nil
	}
	return nil, fmt.Errorf("no instances to merge")
}

func (ccra *CrossClusterResourceAggregator) mergeSpecsPriority(clusterInstances map[string]*ClusterResourceInstance) (*unstructured.Unstructured, error) {
	// For priority strategy, use cluster with highest priority
	// For now, just use the first healthy instance
	for _, instance := range clusterInstances {
		if instance.Health == ResourceHealthHealthy {
			return instance.Resource.DeepCopy(), nil
		}
	}

	// If no healthy instances, use any instance
	for _, instance := range clusterInstances {
		return instance.Resource.DeepCopy(), nil
	}

	return nil, fmt.Errorf("no instances to merge")
}

func (ccra *CrossClusterResourceAggregator) mergeSpecsLatest(clusterInstances map[string]*ClusterResourceInstance) (*unstructured.Unstructured, error) {
	var latest *ClusterResourceInstance
	var latestTime time.Time

	for _, instance := range clusterInstances {
		if instance.LastObserved.After(latestTime) {
			latest = instance
			latestTime = instance.LastObserved
		}
	}

	if latest == nil {
		return nil, fmt.Errorf("no instances to merge")
	}

	return latest.Resource.DeepCopy(), nil
}

// Helper methods

func (ccra *CrossClusterResourceAggregator) getAggregationPolicy(gvk schema.GroupVersionKind) *AggregationPolicy {
	// Return default policy for now
	// In a real implementation, this would be configurable
	return &AggregationPolicy{
		GVK:                gvk,
		MergeStrategy:      MergeStrategyUnion,
		ConflictResolution: ConflictResolutionLastWriter,
		HealthAggregation:  HealthAggregationMajority,
		StatusMerging:      StatusMergingCombined,
		LabelSelectors:     []labels.Selector{},
		FieldSelectors:     []string{},
		Transformations:    []AggregationTransformation{},
	}
}

func (ccra *CrossClusterResourceAggregator) getGroupVersionResource(gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
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

func (ccra *CrossClusterResourceAggregator) getResourceKey(resource *unstructured.Unstructured) string {
	if resource.GetNamespace() != "" {
		return fmt.Sprintf("%s/%s", resource.GetNamespace(), resource.GetName())
	}
	return resource.GetName()
}

func (ccra *CrossClusterResourceAggregator) matchesFieldSelectors(resource *unstructured.Unstructured, fieldSelectors []string) bool {
	// Simplified field selector matching
	// In a real implementation, this would properly parse and evaluate field selectors
	return true
}

func (ccra *CrossClusterResourceAggregator) applyTransformations(resource *unstructured.Unstructured, transformations []AggregationTransformation) error {
	// Apply transformations to the resource
	// This is a simplified implementation
	for _, transformation := range transformations {
		switch transformation.Type {
		case AggregationTransformSet:
			if err := unstructured.SetNestedField(resource.Object, transformation.Value, transformation.JSONPath); err != nil {
				return fmt.Errorf("failed to set field %s: %w", transformation.JSONPath, err)
			}
		}
	}
	return nil
}

func (ccra *CrossClusterResourceAggregator) performPeriodicAggregation(ctx context.Context) {
	logger := klog.FromContext(ctx).WithValues("component", "PeriodicAggregation")

	// Define common resource types to aggregate
	commonGVKs := []schema.GroupVersionKind{
		{Group: "apps", Version: "v1", Kind: "Deployment"},
		{Group: "apps", Version: "v1", Kind: "StatefulSet"},
		{Group: "apps", Version: "v1", Kind: "DaemonSet"},
		{Group: "", Version: "v1", Kind: "Service"},
		{Group: "", Version: "v1", Kind: "ConfigMap"},
		{Group: "", Version: "v1", Kind: "Secret"},
		{Group: "", Version: "v1", Kind: "PersistentVolumeClaim"},
	}

	for _, gvk := range commonGVKs {
		request := &AggregationRequest{
			GVK:         gvk,
			RequestTime: time.Now(),
		}
		ccra.queue.Add(request)
	}

	ccra.aggregationCount++
	ccra.lastAggregationTime = time.Now()

	logger.V(4).Info("Triggered periodic aggregation", "gvkCount", len(commonGVKs))
}

func (ccra *CrossClusterResourceAggregator) handleSyncTriggers(ctx context.Context) {
	logger := klog.FromContext(ctx).WithValues("component", "SyncTriggerHandler")

	for {
		select {
		case <-ctx.Done():
			return
		case <-ccra.stopCh:
			return
		case <-ccra.syncTrigger:
			logger.V(4).Info("Received sync trigger")
			ccra.performPeriodicAggregation(ctx)
		}
	}
}

// GetAggregatedResources returns all currently aggregated resources
func (ccra *CrossClusterResourceAggregator) GetAggregatedResources() map[schema.GroupVersionKind]*AggregatedResourceView {
	ccra.mu.RLock()
	defer ccra.mu.RUnlock()

	result := make(map[schema.GroupVersionKind]*AggregatedResourceView)
	for gvk, view := range ccra.aggregatedResources {
		result[gvk] = view
	}
	return result
}

// GetAggregatedResource returns aggregated resources of a specific type
func (ccra *CrossClusterResourceAggregator) GetAggregatedResource(gvk schema.GroupVersionKind) (*AggregatedResourceView, bool) {
	ccra.mu.RLock()
	defer ccra.mu.RUnlock()

	view, exists := ccra.aggregatedResources[gvk]
	return view, exists
}
