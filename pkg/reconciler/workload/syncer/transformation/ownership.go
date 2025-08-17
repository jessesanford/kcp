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

package transformation

import (
	"context"
	"fmt"
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
)

// ownerReferenceTransformer handles owner reference transformations during synchronization.
// It manages cross-cluster owner references and prevents dangling references.
type ownerReferenceTransformer struct {
	// preserveOwnershipTypes defines which resource types should preserve ownership
	preserveOwnershipTypes map[string]bool
}

// NewOwnerReferenceTransformer creates a new owner reference transformer.
func NewOwnerReferenceTransformer() ResourceTransformer {
	return &ownerReferenceTransformer{
		preserveOwnershipTypes: map[string]bool{
			// Core resources that should maintain ownership
			"Pod":                true,
			"Service":            true,
			"ConfigMap":          true,
			"Secret":             true,
			"PersistentVolume":   false, // PVs are cluster-scoped
			"PersistentVolumeClaim": true,
			
			// App resources
			"Deployment":         true,
			"ReplicaSet":         true,
			"StatefulSet":        true,
			"DaemonSet":          true,
			"Job":                true,
			"CronJob":            true,
			
			// Network resources
			"Ingress":            true,
			"NetworkPolicy":      true,
			
			// Custom resources - default to true
			"CustomResource":     true,
		},
	}
}

// Name returns the transformer name
func (t *ownerReferenceTransformer) Name() string {
	return "ownerreference-transformer"
}

// ShouldTransform returns true for objects that have owner references
func (t *ownerReferenceTransformer) ShouldTransform(obj runtime.Object) bool {
	if obj == nil {
		return false
	}
	
	metaObj, ok := obj.(metav1.Object)
	if !ok {
		return false
	}
	
	// Transform if the object has owner references
	return len(metaObj.GetOwnerReferences()) > 0
}

// TransformForDownstream handles owner references when syncing to physical clusters
func (t *ownerReferenceTransformer) TransformForDownstream(ctx context.Context, obj runtime.Object, target *SyncTarget) (runtime.Object, error) {
	if obj == nil {
		return nil, fmt.Errorf("cannot transform nil object")
	}
	
	metaObj, ok := obj.(metav1.Object)
	if !ok {
		return obj.DeepCopyObject(), nil // Not a metadata object, return copy
	}
	
	ownerRefs := metaObj.GetOwnerReferences()
	if len(ownerRefs) == 0 {
		return obj.DeepCopyObject(), nil // No owner references to transform, return copy
	}
	
	// Create a copy to avoid modifying the original
	result := obj.DeepCopyObject()
	metaResult, _ := result.(metav1.Object)
	
	// Store original owner references for upstream restoration
	originalOwnerRefs := make([]metav1.OwnerReference, len(ownerRefs))
	copy(originalOwnerRefs, ownerRefs)
	
	// Filter and transform owner references
	transformedRefs := t.transformOwnerReferencesForDownstream(ownerRefs, target)
	
	klog.V(5).InfoS("Transforming owner references for downstream",
		"objectKind", getObjectKind(obj),
		"namespace", metaResult.GetNamespace(),
		"name", metaResult.GetName(),
		"originalRefs", len(ownerRefs),
		"transformedRefs", len(transformedRefs),
		"targetCluster", target.Spec.ClusterName)
	
	metaResult.SetOwnerReferences(transformedRefs)
	
	// Store original owner references in annotation for restoration
	if len(originalOwnerRefs) > 0 {
		annotations := metaResult.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}
		
		// Note: In a real implementation, you'd want to serialize this properly
		// For now, we'll store a count for demonstration
		annotations["syncer.kcp.io/original-owner-count"] = fmt.Sprintf("%d", len(originalOwnerRefs))
		metaResult.SetAnnotations(annotations)
	}
	
	return result, nil
}

// TransformForUpstream handles owner references when syncing back to KCP
func (t *ownerReferenceTransformer) TransformForUpstream(ctx context.Context, obj runtime.Object, source *SyncTarget) (runtime.Object, error) {
	if obj == nil {
		return nil, fmt.Errorf("cannot transform nil object")
	}
	
	_, ok := obj.(metav1.Object)
	if !ok {
		return obj.DeepCopyObject(), nil // Not a metadata object, return copy
	}
	
	// Create a copy to avoid modifying the original
	result := obj.DeepCopyObject()
	metaResult, _ := result.(metav1.Object)
	
	// For upstream, we generally want to remove cross-cluster owner references
	// to prevent issues with garbage collection in KCP
	ownerRefs := metaResult.GetOwnerReferences()
	
	if len(ownerRefs) > 0 {
		// Filter owner references for upstream
		filteredRefs := t.filterOwnerReferencesForUpstream(ownerRefs)
		
		klog.V(5).InfoS("Filtering owner references for upstream",
			"objectKind", getObjectKind(obj),
			"namespace", metaResult.GetNamespace(),
			"name", metaResult.GetName(),
			"originalRefs", len(ownerRefs),
			"filteredRefs", len(filteredRefs),
			"sourceCluster", source.Spec.ClusterName)
		
		metaResult.SetOwnerReferences(filteredRefs)
	}
	
	// Always clean up transformation annotations regardless of owner references
	annotations := metaResult.GetAnnotations()
	if annotations != nil {
		delete(annotations, "syncer.kcp.io/original-owner-count")
		if len(annotations) == 0 {
			annotations = nil
		}
		metaResult.SetAnnotations(annotations)
	}
	
	return result, nil
}

// transformOwnerReferencesForDownstream processes owner references for downstream sync
func (t *ownerReferenceTransformer) transformOwnerReferencesForDownstream(refs []metav1.OwnerReference, target *SyncTarget) []metav1.OwnerReference {
	var transformedRefs []metav1.OwnerReference
	
	for _, ref := range refs {
		// Check if we should preserve this owner reference type
		if shouldPreserve, exists := t.preserveOwnershipTypes[ref.Kind]; exists && !shouldPreserve {
			klog.V(6).InfoS("Skipping owner reference due to type policy",
				"kind", ref.Kind,
				"name", ref.Name)
			continue
		}
		
		// Create a transformed reference
		transformedRef := ref.DeepCopy()
		
		// For downstream sync, we may need to adjust UIDs since they won't exist
		// in the target cluster. For now, we'll preserve them as-is.
		// In a production implementation, you might want to:
		// 1. Remove the UID to prevent strict reference validation
		// 2. Or maintain a UID mapping table
		
		// For demonstration, we'll clear the UID to prevent validation issues
		transformedRef.UID = ""
		
		transformedRefs = append(transformedRefs, *transformedRef)
		
		klog.V(6).InfoS("Transformed owner reference",
			"kind", ref.Kind,
			"name", ref.Name,
			"originalUID", ref.UID,
			"transformedUID", transformedRef.UID)
	}
	
	return transformedRefs
}

// filterOwnerReferencesForUpstream removes cross-cluster owner references for upstream sync
func (t *ownerReferenceTransformer) filterOwnerReferencesForUpstream(refs []metav1.OwnerReference) []metav1.OwnerReference {
	var filteredRefs []metav1.OwnerReference
	
	for _, ref := range refs {
		// For upstream sync, we're more conservative about owner references
		// Only preserve references that we're confident exist in KCP
		if t.shouldPreserveForUpstream(ref) {
			filteredRefs = append(filteredRefs, ref)
			klog.V(6).InfoS("Preserving owner reference for upstream",
				"kind", ref.Kind,
				"name", ref.Name,
				"uid", ref.UID)
		} else {
			klog.V(6).InfoS("Filtering out owner reference for upstream",
				"kind", ref.Kind,
				"name", ref.Name,
				"reason", "cross-cluster reference")
		}
	}
	
	return filteredRefs
}

// shouldPreserveForUpstream determines if an owner reference should be preserved during upstream sync
func (t *ownerReferenceTransformer) shouldPreserveForUpstream(ref metav1.OwnerReference) bool {
	// In a real implementation, you might check:
	// 1. If the owner object exists in KCP
	// 2. If the owner is managed by the same syncer
	// 3. If the owner is a KCP-native resource type
	
	// For now, we'll use a simple heuristic: preserve references to common
	// application-level resources that are likely to exist in KCP
	preserveKinds := map[string]bool{
		"Deployment":  true,
		"StatefulSet": true,
		"DaemonSet":   true,
		"Job":         true,
		"CronJob":     true,
		"Service":     true,
		"ConfigMap":   true,
		"Secret":      true,
	}
	
	return preserveKinds[ref.Kind]
}

// generateCrossClusterUID generates a deterministic UID for cross-cluster references
func (t *ownerReferenceTransformer) generateCrossClusterUID(originalUID types.UID, clusterName string) types.UID {
	// In a production implementation, you might use a hash of the original UID
	// and cluster name to generate a deterministic cross-cluster UID
	// For demonstration purposes, we'll just append the cluster name
	return types.UID(fmt.Sprintf("%s-%s", string(originalUID), clusterName))
}

// getObjectKind returns a string representation of the object's kind for logging.
func getObjectKind(obj runtime.Object) string {
	if obj == nil {
		return "unknown"
	}
	
	if gvk := obj.GetObjectKind().GroupVersionKind(); !gvk.Empty() {
		return gvk.String()
	}
	
	return reflect.TypeOf(obj).String()
}