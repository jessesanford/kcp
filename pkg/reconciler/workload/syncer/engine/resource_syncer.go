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

	"github.com/kcp-dev/kcp/pkg/logging"
	"k8s.io/klog/v2"
	
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
)

// ResourceSyncer handles synchronization for a specific resource type
type ResourceSyncer struct {
	gvr    schema.GroupVersionResource
	engine *Engine
}

// NewResourceSyncer creates a new resource syncer for a specific GVR
func NewResourceSyncer(gvr schema.GroupVersionResource, engine *Engine) (*ResourceSyncer, error) {
	if engine == nil {
		return nil, fmt.Errorf("engine cannot be nil")
	}
	
	return &ResourceSyncer{
		gvr:    gvr,
		engine: engine,
	}, nil
}

// ProcessSyncItem processes a single sync item for this resource type
func (rs *ResourceSyncer) ProcessSyncItem(ctx context.Context, item *SyncItem) error {
	logger := logging.WithObject(logging.WithReconciler(klog.FromContext(ctx), "resource-syncer"), nil).WithValues(
		"gvr", rs.gvr,
		"key", item.Key,
		"action", item.Action,
	)
	
	logger.V(4).Info("Processing sync item")
	
	switch item.Action {
	case ActionAdd, ActionUpdate:
		return rs.syncToDownstream(ctx, item)
	case ActionDelete:
		return rs.deleteFromDownstream(ctx, item)
	case ActionStatus:
		return rs.syncStatusToKCP(ctx, item)
	default:
		return fmt.Errorf("unknown action: %s", item.Action)
	}
}

// syncToDownstream synchronizes a resource from KCP to the downstream cluster
func (rs *ResourceSyncer) syncToDownstream(ctx context.Context, item *SyncItem) error {
	logger := klog.FromContext(ctx).WithValues("operation", "syncToDownstream")
	
	// Extract object from sync item
	obj, ok := item.Object.(*unstructured.Unstructured)
	if !ok {
		return fmt.Errorf("expected *unstructured.Unstructured, got %T", item.Object)
	}
	
	// Apply transformations for downstream deployment
	transformedObj, err := rs.transformForDownstream(obj.DeepCopy())
	if err != nil {
		return fmt.Errorf("failed to transform object for downstream: %w", err)
	}
	
	// Check if the resource should be synchronized
	if !rs.shouldSync(transformedObj) {
		logger.V(4).Info("Skipping resource sync based on filters")
		return nil
	}
	
	namespace := transformedObj.GetNamespace()
	name := transformedObj.GetName()
	
	// Get current state from downstream
	downstreamResource := rs.engine.downstreamClient.Resource(rs.gvr)
	var downstreamClient dynamic.ResourceInterface = downstreamResource
	if namespace != "" {
		downstreamClient = downstreamResource.Namespace(namespace)
	}
	
	existingObj, err := downstreamClient.Get(ctx, name, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to get existing object from downstream: %w", err)
	}
	
	if apierrors.IsNotFound(err) {
		// Object doesn't exist downstream, create it
		logger.V(2).Info("Creating new object in downstream", "name", name, "namespace", namespace)
		_, err = downstreamClient.Create(ctx, transformedObj, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create object in downstream: %w", err)
		}
		logger.V(2).Info("Successfully created object in downstream")
	} else {
		// Object exists, update it
		logger.V(2).Info("Updating existing object in downstream", "name", name, "namespace", namespace)
		
		// Preserve some fields from existing object
		rs.preserveDownstreamFields(transformedObj, existingObj)
		
		_, err = downstreamClient.Update(ctx, transformedObj, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update object in downstream: %w", err)
		}
		logger.V(2).Info("Successfully updated object in downstream")
	}
	
	return nil
}

// deleteFromDownstream removes a resource from the downstream cluster
func (rs *ResourceSyncer) deleteFromDownstream(ctx context.Context, item *SyncItem) error {
	logger := klog.FromContext(ctx).WithValues("operation", "deleteFromDownstream")
	
	namespace, name, err := cache.SplitMetaNamespaceKey(item.Key)
	if err != nil {
		return fmt.Errorf("invalid key format: %w", err)
	}
	
	// Get downstream client
	downstreamResource := rs.engine.downstreamClient.Resource(rs.gvr)
	var downstreamClient dynamic.ResourceInterface = downstreamResource
	if namespace != "" {
		downstreamClient = downstreamResource.Namespace(namespace)
	}
	
	// Check if object exists
	_, err = downstreamClient.Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		// Object already doesn't exist, nothing to do
		logger.V(4).Info("Object already deleted from downstream", "name", name, "namespace", namespace)
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to check object existence in downstream: %w", err)
	}
	
	// Delete the object
	logger.V(2).Info("Deleting object from downstream", "name", name, "namespace", namespace)
	err = downstreamClient.Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete object from downstream: %w", err)
	}
	
	logger.V(2).Info("Successfully deleted object from downstream")
	return nil
}

// syncStatusToKCP synchronizes status from downstream back to KCP
func (rs *ResourceSyncer) syncStatusToKCP(ctx context.Context, item *SyncItem) error {
	logger := klog.FromContext(ctx).WithValues("operation", "syncStatusToKCP")
	
	// Extract object from sync item (this is the downstream object)
	downstreamObj, ok := item.Object.(*unstructured.Unstructured)
	if !ok {
		return fmt.Errorf("expected *unstructured.Unstructured, got %T", item.Object)
	}
	
	namespace := downstreamObj.GetNamespace()
	name := downstreamObj.GetName()
	
	// For now, we'll use a placeholder implementation for KCP client access
	// In a real implementation, this would use the proper cluster path
	// TODO: Implement proper KCP cluster client access with logical cluster paths
	logger.V(4).Info("KCP status sync not implemented yet - placeholder", "name", name, "namespace", namespace)
	
	// This is a placeholder - in the real implementation we would:
	// 1. Get the KCP object using the proper cluster client
	// 2. Extract status from downstream object
	// 3. Transform status for KCP
	// 4. Update the KCP object status
	
	return nil
}

// transformForDownstream applies transformations needed for downstream deployment
func (rs *ResourceSyncer) transformForDownstream(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	// Remove KCP-specific fields
	rs.removeKCPFields(obj)
	
	// Remove status field (will be managed by downstream controller)
	unstructured.RemoveNestedField(obj.Object, "status")
	
	// Reset resource version for creation/update
	obj.SetResourceVersion("")
	
	// Add downstream-specific labels and annotations
	rs.addDownstreamMetadata(obj)
	
	return obj, nil
}

// transformStatusForKCP applies transformations to status for KCP sync
func (rs *ResourceSyncer) transformStatusForKCP(status map[string]interface{}) (map[string]interface{}, error) {
	// This is a placeholder for status transformation logic
	// In a real implementation, you might need to:
	// - Filter out downstream-specific status fields
	// - Aggregate multiple downstream statuses
	// - Transform status format for KCP consumption
	
	return status, nil
}

// shouldSync determines if a resource should be synchronized based on filters
func (rs *ResourceSyncer) shouldSync(obj *unstructured.Unstructured) bool {
	// Check for sync annotations
	annotations := obj.GetAnnotations()
	if annotations != nil {
		if skip, exists := annotations["syncer.kcp.io/skip"]; exists && skip == "true" {
			return false
		}
	}
	
	// Add more filter logic as needed
	return true
}

// removeKCPFields removes KCP-specific fields from objects
func (rs *ResourceSyncer) removeKCPFields(obj *unstructured.Unstructured) {
	// Remove KCP-specific annotations
	annotations := obj.GetAnnotations()
	if annotations != nil {
		delete(annotations, "kcp.io/cluster")
		delete(annotations, "experimental.status.kcp.io/cluster")
		obj.SetAnnotations(annotations)
	}
	
	// Remove KCP-specific labels
	labels := obj.GetLabels()
	if labels != nil {
		for key := range labels {
			if isKCPLabel(key) {
				delete(labels, key)
			}
		}
		obj.SetLabels(labels)
	}
	
	// Remove UID (will be assigned by downstream cluster)
	obj.SetUID("")
}

// addDownstreamMetadata adds metadata needed for downstream tracking
func (rs *ResourceSyncer) addDownstreamMetadata(obj *unstructured.Unstructured) {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	
	// Add syncer identification
	annotations["syncer.kcp.io/synced-from"] = "kcp"
	annotations["syncer.kcp.io/gvr"] = rs.gvr.String()
	
	obj.SetAnnotations(annotations)
}

// preserveDownstreamFields preserves certain fields from the existing downstream object
func (rs *ResourceSyncer) preserveDownstreamFields(newObj, existingObj *unstructured.Unstructured) {
	// Preserve resource version for updates
	newObj.SetResourceVersion(existingObj.GetResourceVersion())
	
	// Preserve UID
	newObj.SetUID(existingObj.GetUID())
	
	// Preserve creation timestamp
	newObj.SetCreationTimestamp(existingObj.GetCreationTimestamp())
}

// isKCPLabel determines if a label key is KCP-specific
func isKCPLabel(key string) bool {
	kcpPrefixes := []string{
		"kcp.io/",
		"experimental.kcp.io/",
		"internal.kcp.io/",
	}
	
	for _, prefix := range kcpPrefixes {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			return true
		}
	}
	
	return false
}