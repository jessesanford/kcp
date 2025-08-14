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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"

	workloadv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"
)

const (
	// discoveryRefreshInterval is how often we refresh resource discovery
	discoveryRefreshInterval = 5 * time.Minute
)

// discoveredResource represents a resource type discovered in a physical cluster
type discoveredResource struct {
	// Resource metadata
	GroupVersionResource schema.GroupVersionResource
	Namespaced           bool
	
	// Discovery timestamp
	LastDiscovered time.Time
	
	// Resource capabilities
	Verbs     sets.Set[string]
	ShortNames []string
	Kind       string
}

// discoveryCache maintains cached discovery information for a physical cluster
type discoveryCache struct {
	// Discovered resources by GVR
	resources map[schema.GroupVersionResource]*discoveredResource
	
	// Last successful discovery time
	lastUpdate time.Time
	
	// Discovery client for the physical cluster
	discoveryClient discovery.DiscoveryInterface
	
	// Mutex for thread-safe access
	mutex sync.RWMutex
}

// discoveryManager manages resource discovery for all physical clusters
type discoveryManager struct {
	// Per-SyncTarget discovery caches
	caches map[string]*discoveryCache
	mutex  sync.RWMutex
	
	// Parent syncer reference
	syncer *UpstreamSyncer
}

// newDiscoveryManager creates a new discovery manager
func newDiscoveryManager(syncer *UpstreamSyncer) (*discoveryManager, error) {
	return &discoveryManager{
		caches: make(map[string]*discoveryCache),
		syncer: syncer,
	}, nil
}

// updateDiscovery refreshes resource discovery for a SyncTarget
func (dm *discoveryManager) updateDiscovery(ctx context.Context, syncTarget *workloadv1alpha1.SyncTarget, physicalClient dynamic.Interface) error {
	logger := klog.FromContext(ctx)
	key := dm.syncer.getSyncTargetKey(syncTarget)
	
	cache := dm.getOrCreateCache(key, physicalClient)
	
	// Check if discovery needs refresh
	if time.Since(cache.lastUpdate) < discoveryRefreshInterval {
		logger.V(5).Info("Discovery cache is still fresh, skipping refresh", "syncTarget", syncTarget.Name)
		return nil
	}
	
	logger.V(3).Info("Refreshing resource discovery", "syncTarget", syncTarget.Name)
	
	// Perform resource discovery
	if err := cache.discover(ctx, syncTarget); err != nil {
		return fmt.Errorf("failed to discover resources: %w", err)
	}
	
	// Filter resources based on SyncTarget configuration
	filteredResources := dm.filterResourcesForSync(syncTarget, cache.getResources())
	
	logger.V(3).Info("Resource discovery completed", 
		"syncTarget", syncTarget.Name,
		"totalResources", len(cache.getResources()),
		"filteredResources", len(filteredResources))
	
	return nil
}

// getOrCreateCache gets or creates a discovery cache for a SyncTarget
func (dm *discoveryManager) getOrCreateCache(key string, physicalClient dynamic.Interface) *discoveryCache {
	dm.mutex.RLock()
	cache, exists := dm.caches[key]
	dm.mutex.RUnlock()
	
	if exists {
		return cache
	}
	
	dm.mutex.Lock()
	defer dm.mutex.Unlock()
	
	// Double-check after acquiring write lock
	if cache, exists := dm.caches[key]; exists {
		return cache
	}
	
	// Create new cache
	cache = &discoveryCache{
		resources:   make(map[schema.GroupVersionResource]*discoveredResource),
		lastUpdate:  time.Time{}, // Force initial discovery
	}
	
	// Set discovery client if available
	if physicalClient != nil {
		// In a real implementation, we would extract the discovery client from the dynamic client
		// For now, we'll set it to nil and handle this in the discover method
		cache.discoveryClient = nil
	}
	
	dm.caches[key] = cache
	return cache
}

// cleanupCache removes the discovery cache for a SyncTarget
func (dm *discoveryManager) cleanupCache(key string) {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()
	delete(dm.caches, key)
}

// discover performs resource discovery on the physical cluster
func (dc *discoveryCache) discover(ctx context.Context, syncTarget *workloadv1alpha1.SyncTarget) error {
	logger := klog.FromContext(ctx)
	
	dc.mutex.Lock()
	defer dc.mutex.Unlock()
	
	// In a real implementation, this would use the discovery client to enumerate resources
	// For now, we'll simulate discovery with common Kubernetes resources
	
	now := time.Now()
	commonResources := []discoveredResource{
		{
			GroupVersionResource: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			Namespaced:          true,
			LastDiscovered:      now,
			Verbs:              sets.New("get", "list", "watch", "create", "update", "patch", "delete"),
			Kind:               "Pod",
		},
		{
			GroupVersionResource: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "services"},
			Namespaced:          true,
			LastDiscovered:      now,
			Verbs:              sets.New("get", "list", "watch", "create", "update", "patch", "delete"),
			Kind:               "Service",
		},
		{
			GroupVersionResource: schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
			Namespaced:          true,
			LastDiscovered:      now,
			Verbs:              sets.New("get", "list", "watch", "create", "update", "patch", "delete"),
			Kind:               "Deployment",
		},
		{
			GroupVersionResource: schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"},
			Namespaced:          true,
			LastDiscovered:      now,
			Verbs:              sets.New("get", "list", "watch", "create", "update", "patch", "delete"),
			Kind:               "StatefulSet",
		},
		{
			GroupVersionResource: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"},
			Namespaced:          true,
			LastDiscovered:      now,
			Verbs:              sets.New("get", "list", "watch", "create", "update", "patch", "delete"),
			Kind:               "ConfigMap",
		},
		{
			GroupVersionResource: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"},
			Namespaced:          true,
			LastDiscovered:      now,
			Verbs:              sets.New("get", "list", "watch", "create", "update", "patch", "delete"),
			Kind:               "Secret",
		},
	}
	
	// Clear existing resources and add discovered ones
	dc.resources = make(map[schema.GroupVersionResource]*discoveredResource)
	for _, resource := range commonResources {
		resourceCopy := resource // Avoid pointer to loop variable
		dc.resources[resource.GroupVersionResource] = &resourceCopy
	}
	
	dc.lastUpdate = now
	
	logger.V(4).Info("Simulated resource discovery completed", 
		"syncTarget", syncTarget.Name,
		"resourceCount", len(dc.resources))
	
	return nil
}

// getResources returns a copy of discovered resources
func (dc *discoveryCache) getResources() map[schema.GroupVersionResource]*discoveredResource {
	dc.mutex.RLock()
	defer dc.mutex.RUnlock()
	
	result := make(map[schema.GroupVersionResource]*discoveredResource)
	for gvr, resource := range dc.resources {
		resourceCopy := *resource
		result[gvr] = &resourceCopy
	}
	
	return result
}

// filterResourcesForSync filters discovered resources based on SyncTarget configuration
func (dm *discoveryManager) filterResourcesForSync(syncTarget *workloadv1alpha1.SyncTarget, allResources map[schema.GroupVersionResource]*discoveredResource) map[schema.GroupVersionResource]*discoveredResource {
	filtered := make(map[schema.GroupVersionResource]*discoveredResource)
	
	// If SyncTarget specifies supported resource types, filter by them
	if len(syncTarget.Spec.SupportedResourceTypes) > 0 {
		supportedTypes := sets.New(syncTarget.Spec.SupportedResourceTypes...)
		
		for gvr, resource := range allResources {
			resourceType := gvr.Resource
			if gvr.Group != "" {
				resourceType = fmt.Sprintf("%s.%s", gvr.Resource, gvr.Group)
			}
			
			if supportedTypes.Has(resourceType) || supportedTypes.Has(gvr.Resource) {
				filtered[gvr] = resource
			}
		}
	} else {
		// No filter specified, include all discovered resources
		for gvr, resource := range allResources {
			filtered[gvr] = resource
		}
	}
	
	// Filter out resources that don't have required verbs for syncing
	result := make(map[schema.GroupVersionResource]*discoveredResource)
	requiredVerbs := sets.New("get", "list", "watch")
	
	for gvr, resource := range filtered {
		if resource.Verbs.HasAll(requiredVerbs.UnsortedList()...) {
			result[gvr] = resource
		}
	}
	
	return result
}

// getSyncableResources returns the resources that should be synced for a given SyncTarget
func (dm *discoveryManager) getSyncableResources(syncTarget *workloadv1alpha1.SyncTarget) (map[schema.GroupVersionResource]*discoveredResource, error) {
	key := dm.syncer.getSyncTargetKey(syncTarget)
	
	dm.mutex.RLock()
	cache, exists := dm.caches[key]
	dm.mutex.RUnlock()
	
	if !exists {
		return nil, fmt.Errorf("no discovery cache found for SyncTarget %s", key)
	}
	
	allResources := cache.getResources()
	return dm.filterResourcesForSync(syncTarget, allResources), nil
}

// isResourceSupported checks if a specific resource is supported by the SyncTarget
func (dm *discoveryManager) isResourceSupported(syncTarget *workloadv1alpha1.SyncTarget, gvr schema.GroupVersionResource) bool {
	syncableResources, err := dm.getSyncableResources(syncTarget)
	if err != nil {
		return false
	}
	
	_, supported := syncableResources[gvr]
	return supported
}

// getLastDiscoveryTime returns the last discovery time for a SyncTarget
func (dm *discoveryManager) getLastDiscoveryTime(syncTarget *workloadv1alpha1.SyncTarget) time.Time {
	key := dm.syncer.getSyncTargetKey(syncTarget)
	
	dm.mutex.RLock()
	cache, exists := dm.caches[key]
	dm.mutex.RUnlock()
	
	if !exists {
		return time.Time{}
	}
	
	cache.mutex.RLock()
	lastUpdate := cache.lastUpdate
	cache.mutex.RUnlock()
	
	return lastUpdate
}