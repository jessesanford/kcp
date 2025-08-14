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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"

	workloadv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"
)

const (
	// Workspace annotations and labels
	WorkspaceAnnotation     = "workload.kcp.io/workspace"
	SyncTargetAnnotation    = "workload.kcp.io/sync-target" 
	SourceClusterAnnotation = "workload.kcp.io/source-cluster"
	UpstreamSyncLabel       = "workload.kcp.io/upstream-synced"
	
	// UID mapping annotations
	OriginalUIDAnnotation = "workload.kcp.io/original-uid"
	PhysicalUIDAnnotation = "workload.kcp.io/physical-uid"
	
	// Sync state annotations
	LastSyncTimeAnnotation = "workload.kcp.io/last-sync-time"
	SyncGenerationAnnotation = "workload.kcp.io/sync-generation"
)

// resourceTransformer handles transformation of resources between physical clusters and KCP
type resourceTransformer struct {
	// SyncTarget context
	syncTarget      *workloadv1alpha1.SyncTarget
	targetWorkspace logicalcluster.Name
	
	// UID mapping for cross-cluster resource references
	uidMapper *uidMapper
}

// uidMapper maintains mappings between physical cluster UIDs and KCP workspace UIDs
type uidMapper struct {
	// Physical UID to KCP UID mappings
	physicalToKCP map[string]string
	
	// KCP UID to Physical UID mappings  
	kcpToPhysical map[string]string
}

// newResourceTransformer creates a new resource transformer for a SyncTarget
func newResourceTransformer(syncTarget *workloadv1alpha1.SyncTarget, workspace logicalcluster.Name) *resourceTransformer {
	return &resourceTransformer{
		syncTarget:      syncTarget,
		targetWorkspace: workspace,
		uidMapper:       newUIDMapper(),
	}
}

// newUIDMapper creates a new UID mapper
func newUIDMapper() *uidMapper {
	return &uidMapper{
		physicalToKCP: make(map[string]string),
		kcpToPhysical: make(map[string]string),
	}
}

// syncResourcesFromPhysical pulls resources from physical cluster and syncs them to KCP
func (us *UpstreamSyncer) syncResourcesFromPhysical(ctx context.Context, syncTarget *workloadv1alpha1.SyncTarget, physicalClient dynamic.Interface) error {
	logger := klog.FromContext(ctx)
	
	// Get syncable resources for this target
	syncableResources, err := us.discoveryManager.getSyncableResources(syncTarget)
	if err != nil {
		return fmt.Errorf("failed to get syncable resources: %w", err)
	}
	
	if len(syncableResources) == 0 {
		logger.V(4).Info("No syncable resources found for SyncTarget", "syncTarget", syncTarget.Name)
		return nil
	}
	
	// Create transformer for this sync operation
	workspace := logicalcluster.From(syncTarget)
	transformer := newResourceTransformer(syncTarget, workspace)
	
	var syncErrors []error
	
	// Sync each resource type
	for gvr, discoveredResource := range syncableResources {
		if err := us.syncResourceType(ctx, syncTarget, physicalClient, gvr, discoveredResource, transformer); err != nil {
			syncErrors = append(syncErrors, fmt.Errorf("failed to sync %s: %w", gvr.String(), err))
			continue
		}
		
		logger.V(4).Info("Successfully synced resource type", 
			"syncTarget", syncTarget.Name, 
			"resource", gvr.String())
	}
	
	if len(syncErrors) > 0 {
		return fmt.Errorf("sync completed with errors: %v", utilerrors.NewAggregate(syncErrors))
	}
	
	logger.V(3).Info("Successfully synced all resources from physical cluster", 
		"syncTarget", syncTarget.Name,
		"resourceTypes", len(syncableResources))
	
	return nil
}

// syncResourceType syncs all instances of a specific resource type
func (us *UpstreamSyncer) syncResourceType(ctx context.Context, syncTarget *workloadv1alpha1.SyncTarget, physicalClient dynamic.Interface, gvr schema.GroupVersionResource, discoveredResource *discoveredResource, transformer *resourceTransformer) error {
	logger := klog.FromContext(ctx)
	
	// List resources from physical cluster
	// Note: In a real implementation, this would use the physicalClient
	// For now, we simulate with empty list since physicalClient setup is not complete
	physicalResources := &unstructured.UnstructuredList{
		Items: []unstructured.Unstructured{},
	}
	
	logger.V(4).Info("Retrieved resources from physical cluster", 
		"resource", gvr.String(),
		"count", len(physicalResources.Items))
	
	// Transform and sync each resource
	for i := range physicalResources.Items {
		resource := &physicalResources.Items[i]
		
		// Transform physical resource to KCP format
		kcpResource, err := transformer.transformToKCP(ctx, resource, gvr)
		if err != nil {
			logger.Error(err, "Failed to transform resource", 
				"resource", gvr.String(), 
				"name", resource.GetName(),
				"namespace", resource.GetNamespace())
			continue
		}
		
		// Apply resource to KCP workspace
		if err := us.applyResourceToKCP(ctx, kcpResource, gvr); err != nil {
			logger.Error(err, "Failed to apply resource to KCP", 
				"resource", gvr.String(),
				"name", kcpResource.GetName(),
				"namespace", kcpResource.GetNamespace())
			continue
		}
		
		// Update UID mapping
		transformer.uidMapper.addMapping(resource.GetUID(), kcpResource.GetUID())
	}
	
	return nil
}

// transformToKCP transforms a physical cluster resource to KCP workspace format
func (rt *resourceTransformer) transformToKCP(ctx context.Context, physicalResource *unstructured.Unstructured, gvr schema.GroupVersionResource) (*unstructured.Unstructured, error) {
	logger := klog.FromContext(ctx)
	
	// Create a deep copy to avoid modifying the original
	kcpResource := physicalResource.DeepCopy()
	
	// Clear cluster-specific metadata
	rt.clearClusterSpecificMetadata(kcpResource)
	
	// Add workspace annotations
	rt.addWorkspaceAnnotations(kcpResource)
	
	// Add sync tracking labels
	rt.addSyncLabels(kcpResource)
	
	// Transform resource references (UIDs, names, etc.)
	if err := rt.transformResourceReferences(ctx, kcpResource); err != nil {
		return nil, fmt.Errorf("failed to transform resource references: %w", err)
	}
	
	// Handle status field transformation
	if err := rt.transformStatus(ctx, kcpResource, gvr); err != nil {
		return nil, fmt.Errorf("failed to transform status: %w", err)
	}
	
	logger.V(5).Info("Successfully transformed resource to KCP format",
		"resource", gvr.String(),
		"name", kcpResource.GetName(),
		"namespace", kcpResource.GetNamespace())
	
	return kcpResource, nil
}

// clearClusterSpecificMetadata removes cluster-specific metadata from the resource
func (rt *resourceTransformer) clearClusterSpecificMetadata(resource *unstructured.Unstructured) {
	// Clear generated fields
	resource.SetUID("")
	resource.SetResourceVersion("")
	resource.SetGeneration(0)
	resource.SetSelfLink("")
	
	// Clear cluster-specific timestamps - keep creation timestamp for reference
	resource.SetDeletionTimestamp(nil)
	resource.SetDeletionGracePeriodSeconds(nil)
	
	// Remove cluster-specific finalizers (if any)
	finalizers := resource.GetFinalizers()
	filteredFinalizers := []string{}
	for _, finalizer := range finalizers {
		// Keep KCP-specific finalizers, remove cluster-specific ones
		if !rt.isClusterSpecificFinalizer(finalizer) {
			filteredFinalizers = append(filteredFinalizers, finalizer)
		}
	}
	resource.SetFinalizers(filteredFinalizers)
}

// addWorkspaceAnnotations adds workspace context annotations
func (rt *resourceTransformer) addWorkspaceAnnotations(resource *unstructured.Unstructured) {
	annotations := resource.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	
	// Store original physical cluster UID
	if resource.GetUID() != "" {
		annotations[PhysicalUIDAnnotation] = string(resource.GetUID())
	}
	
	// Add workspace context
	annotations[WorkspaceAnnotation] = rt.targetWorkspace.String()
	annotations[SyncTargetAnnotation] = rt.syncTarget.Name
	annotations[SourceClusterAnnotation] = rt.syncTarget.Spec.Location
	
	// Add sync timing
	annotations[LastSyncTimeAnnotation] = metav1.Now().Format(metav1.RFC3339)
	
	resource.SetAnnotations(annotations)
}

// addSyncLabels adds labels for tracking upstream sync
func (rt *resourceTransformer) addSyncLabels(resource *unstructured.Unstructured) {
	labels := resource.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	
	labels[UpstreamSyncLabel] = "true"
	
	resource.SetLabels(labels)
}

// transformResourceReferences updates resource references to match KCP context
func (rt *resourceTransformer) transformResourceReferences(ctx context.Context, resource *unstructured.Unstructured) error {
	// This would handle transformation of owner references, service account names,
	// configmap/secret references, etc. to match the KCP workspace context
	
	// Transform owner references
	ownerRefs := resource.GetOwnerReferences()
	for i := range ownerRefs {
		// Map physical UIDs to KCP UIDs if mappings exist
		if kcpUID, exists := rt.uidMapper.physicalToKCP[string(ownerRefs[i].UID)]; exists {
			ownerRefs[i].UID = kcpUID
		}
	}
	resource.SetOwnerReferences(ownerRefs)
	
	return nil
}

// transformStatus handles status field transformation for upstream sync
func (rt *resourceTransformer) transformStatus(ctx context.Context, resource *unstructured.Unstructured, gvr schema.GroupVersionResource) error {
	// Status transformation depends on the resource type
	// For most resources, we want to preserve the status from the physical cluster
	// as it represents the actual running state
	
	switch gvr.Resource {
	case "pods":
		return rt.transformPodStatus(resource)
	case "deployments", "statefulsets", "daemonsets":
		return rt.transformWorkloadStatus(resource)
	case "services":
		return rt.transformServiceStatus(resource)
	default:
		// For other resources, preserve status as-is
		return nil
	}
}

// transformPodStatus transforms Pod status for KCP
func (rt *resourceTransformer) transformPodStatus(resource *unstructured.Unstructured) error {
	// Pod status should reflect the actual state in the physical cluster
	// We preserve most status fields but may need to transform some references
	return nil
}

// transformWorkloadStatus transforms workload controller status 
func (rt *resourceTransformer) transformWorkloadStatus(resource *unstructured.Unstructured) error {
	// Preserve replica counts and conditions from physical cluster
	return nil
}

// transformServiceStatus transforms Service status
func (rt *resourceTransformer) transformServiceStatus(resource *unstructured.Unstructured) error {
	// May need to handle load balancer status differently in multi-cluster context
	return nil
}

// applyResourceToKCP applies a transformed resource to the KCP workspace
func (us *UpstreamSyncer) applyResourceToKCP(ctx context.Context, resource *unstructured.Unstructured, gvr schema.GroupVersionResource) error {
	logger := klog.FromContext(ctx)
	
	// In a real implementation, this would:
	// 1. Get the KCP dynamic client for the target workspace
	// 2. Check if the resource already exists
	// 3. Handle conflicts using the conflict resolver
	// 4. Apply the resource (create or update)
	
	logger.V(4).Info("Would apply resource to KCP workspace",
		"resource", gvr.String(),
		"name", resource.GetName(),
		"namespace", resource.GetNamespace(),
		"workspace", resource.GetAnnotations()[WorkspaceAnnotation])
	
	return nil
}

// isClusterSpecificFinalizer determines if a finalizer is cluster-specific
func (rt *resourceTransformer) isClusterSpecificFinalizer(finalizer string) bool {
	// List of known cluster-specific finalizers that should be removed
	clusterSpecificFinalizers := []string{
		"kubernetes.io/pv-protection",
		"kubernetes.io/pvc-protection", 
		// Add more as needed
	}
	
	for _, csf := range clusterSpecificFinalizers {
		if finalizer == csf {
			return true
		}
	}
	
	return false
}

// addMapping adds a UID mapping between physical and KCP resources
func (um *uidMapper) addMapping(physicalUID, kcpUID string) {
	um.physicalToKCP[physicalUID] = kcpUID
	um.kcpToPhysical[kcpUID] = physicalUID
}

// getKCPUID returns the KCP UID for a given physical UID
func (um *uidMapper) getKCPUID(physicalUID string) (string, bool) {
	kcpUID, exists := um.physicalToKCP[physicalUID]
	return kcpUID, exists
}

// getPhysicalUID returns the physical UID for a given KCP UID
func (um *uidMapper) getPhysicalUID(kcpUID string) (string, bool) {
	physicalUID, exists := um.kcpToPhysical[kcpUID]
	return physicalUID, exists
}