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
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
)

// ResourceTransformer handles bidirectional transformation of resources between 
// KCP internal format and the format expected by syncers.
type ResourceTransformer struct {
	syncerID  string
	workspace string
}

// NewResourceTransformer creates a new transformer for a specific syncer and workspace.
func NewResourceTransformer(syncerID, workspace string) *ResourceTransformer {
	return &ResourceTransformer{
		syncerID:  syncerID,
		workspace: workspace,
	}
}

// TransformForDownstream transforms a KCP resource to be sent to a syncer.
// This removes internal KCP metadata and adds syncer-specific annotations.
func (t *ResourceTransformer) TransformForDownstream(obj runtime.Object) (runtime.Object, error) {
	if obj == nil {
		return nil, fmt.Errorf("cannot transform nil object")
	}

	// Get the object's metadata
	objMeta, err := getObjectMeta(obj)
	if err != nil {
		return nil, fmt.Errorf("failed to get object metadata: %w", err)
	}

	// Create a deep copy to avoid modifying the original
	transformed := obj.DeepCopyObject()
	transformedMeta, _ := getObjectMeta(transformed)

	// Remove internal KCP annotations
	if transformedMeta.GetAnnotations() != nil {
		annotations := transformedMeta.GetAnnotations()
		filteredAnnotations := make(map[string]string)
		
		for key, value := range annotations {
			if !isInternalKCPAnnotation(key) {
				filteredAnnotations[key] = value
			}
		}
		
		transformedMeta.SetAnnotations(filteredAnnotations)
	}

	// Add syncer-specific metadata
	t.addSyncerMetadata(transformedMeta)

	// Transform namespace references for multi-tenancy
	if err := t.transformNamespaceReferences(transformed); err != nil {
		return nil, fmt.Errorf("failed to transform namespace references: %w", err)
	}

	klog.V(4).InfoS("Transformed resource for downstream", 
		"syncerID", t.syncerID, 
		"name", objMeta.GetName())

	return transformed, nil
}

// TransformFromUpstream transforms a resource received from a syncer back to KCP format.
// This adds workspace metadata and removes syncer-specific annotations.
func (t *ResourceTransformer) TransformFromUpstream(obj runtime.Object) (runtime.Object, error) {
	if obj == nil {
		return nil, fmt.Errorf("cannot transform nil object")
	}

	// Get the object's metadata
	objMeta, err := getObjectMeta(obj)
	if err != nil {
		return nil, fmt.Errorf("failed to get object metadata: %w", err)
	}

	// Create a deep copy to avoid modifying the original
	transformed := obj.DeepCopyObject()
	transformedMeta, _ := getObjectMeta(transformed)

	// Add workspace metadata for KCP
	t.addWorkspaceMetadata(transformedMeta)

	// Remove syncer-specific annotations that shouldn't be persisted in KCP
	if transformedMeta.GetAnnotations() != nil {
		annotations := transformedMeta.GetAnnotations()
		for key := range annotations {
			if isSyncerSpecificAnnotation(key) {
				delete(annotations, key)
			}
		}
		transformedMeta.SetAnnotations(annotations)
	}

	// Restore internal namespace references
	if err := t.restoreNamespaceReferences(transformed); err != nil {
		return nil, fmt.Errorf("failed to restore namespace references: %w", err)
	}

	klog.V(4).InfoS("Transformed resource from upstream", 
		"syncerID", t.syncerID, 
		"name", objMeta.GetName())

	return transformed, nil
}

// addSyncerMetadata adds metadata that syncers need to properly handle resources.
func (t *ResourceTransformer) addSyncerMetadata(objMeta metav1.Object) {
	annotations := objMeta.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	// Add syncer identity
	annotations["syncer.workload.kcp.io/syncer-id"] = t.syncerID
	annotations["syncer.workload.kcp.io/workspace"] = t.workspace
	
	// Add transformation timestamp
	annotations["syncer.workload.kcp.io/transformed-at"] = metav1.Now().Format(time.RFC3339)

	objMeta.SetAnnotations(annotations)
}

// addWorkspaceMetadata adds KCP workspace metadata to resources coming from syncers.
func (t *ResourceTransformer) addWorkspaceMetadata(objMeta metav1.Object) {
	annotations := objMeta.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	// Add workspace information for KCP routing
	annotations["core.kcp.io/workspace"] = t.workspace
	annotations["workload.kcp.io/synced-from"] = t.syncerID

	objMeta.SetAnnotations(annotations)
}

// transformNamespaceReferences handles namespace mapping for multi-tenant scenarios.
func (t *ResourceTransformer) transformNamespaceReferences(obj runtime.Object) error {
	// This is a placeholder for namespace transformation logic.
	// In a real implementation, this would:
	// - Map KCP logical cluster namespaces to physical cluster namespaces
	// - Handle namespace isolation between different workspaces
	// - Apply namespace prefix/suffix transformations

	klog.V(5).InfoS("Transforming namespace references", "syncerID", t.syncerID)
	return nil
}

// restoreNamespaceReferences reverses namespace transformations for resources from syncers.
func (t *ResourceTransformer) restoreNamespaceReferences(obj runtime.Object) error {
	// This is a placeholder for reverse namespace transformation logic.
	// In a real implementation, this would:
	// - Map physical cluster namespaces back to logical cluster namespaces
	// - Remove namespace transformations applied during downstream transformation

	klog.V(5).InfoS("Restoring namespace references", "syncerID", t.syncerID)
	return nil
}

// isInternalKCPAnnotation determines if an annotation is internal to KCP and should
// be filtered out when sending resources to syncers.
func isInternalKCPAnnotation(key string) bool {
	internalPrefixes := []string{
		"internal.kcp.io/",
		"core.kcp.io/finalizers",
		"core.kcp.io/cluster",
		"tenancy.kcp.io/internal",
	}

	for _, prefix := range internalPrefixes {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}

	return false
}

// isSyncerSpecificAnnotation determines if an annotation is syncer-specific and should
// be removed when transforming resources back to KCP format.
func isSyncerSpecificAnnotation(key string) bool {
	syncerPrefixes := []string{
		"syncer.workload.kcp.io/",
		"kubectl.kubernetes.io/", // kubectl annotations shouldn't be persisted
	}

	for _, prefix := range syncerPrefixes {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}

	return false
}

// getObjectMeta extracts metadata from a runtime.Object.
func getObjectMeta(obj runtime.Object) (metav1.Object, error) {
	objMeta, ok := obj.(metav1.Object)
	if !ok {
		return nil, fmt.Errorf("object does not implement metav1.Object interface")
	}
	return objMeta, nil
}