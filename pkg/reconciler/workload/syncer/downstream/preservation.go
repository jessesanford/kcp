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

package downstream

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"
)

// PreserveDownstreamFields merges desired changes with downstream-managed fields
func PreserveDownstreamFields(existing, desired *unstructured.Unstructured) *unstructured.Unstructured {
	if existing == nil || desired == nil {
		return desired
	}

	// Start with desired object as base
	merged := desired.DeepCopy()

	// Preserve essential server-managed metadata
	preserveServerManagedMetadata(existing, merged)

	// Preserve status (managed separately)
	preserveStatus(existing, merged)

	// Preserve server-managed fields based on resource type
	preserveResourceSpecificFields(existing, merged)

	return merged
}

// preserveServerManagedMetadata preserves metadata fields managed by the downstream cluster
func preserveServerManagedMetadata(existing, merged *unstructured.Unstructured) {
	// Preserve resource version (required for updates)
	if rv := existing.GetResourceVersion(); rv != "" {
		merged.SetResourceVersion(rv)
	}

	// Preserve UID (assigned by downstream cluster)
	if uid := existing.GetUID(); uid != "" {
		merged.SetUID(uid)
	}

	// Preserve creation timestamp
	if ct := existing.GetCreationTimestamp(); !ct.IsZero() {
		merged.SetCreationTimestamp(ct)
	}

	// Preserve managed fields (server-managed)
	if managedFields := existing.GetManagedFields(); managedFields != nil {
		merged.SetManagedFields(managedFields)
	}

	// Merge finalizers carefully (preserve existing, add new ones)
	preserveFinalizers(existing, merged)

	// Merge owner references (preserve downstream ones)
	preserveOwnerReferences(existing, merged)
}

// preserveStatus preserves the status field from the existing object
func preserveStatus(existing, merged *unstructured.Unstructured) {
	if status, found, err := unstructured.NestedFieldNoCopy(existing.Object, "status"); found && err == nil {
		if err := unstructured.SetNestedField(merged.Object, status, "status"); err != nil {
			klog.V(2).Info("Failed to preserve status field", "error", err)
		}
	}
}

// preserveFinalizers merges finalizers from both objects
func preserveFinalizers(existing, merged *unstructured.Unstructured) {
	existingFinalizers := existing.GetFinalizers()
	desiredFinalizers := merged.GetFinalizers()

	if len(existingFinalizers) == 0 {
		return
	}

	// Create a map to track unique finalizers
	finalizerMap := make(map[string]bool)

	// Add existing finalizers
	for _, finalizer := range existingFinalizers {
		finalizerMap[finalizer] = true
	}

	// Add desired finalizers
	for _, finalizer := range desiredFinalizers {
		finalizerMap[finalizer] = true
	}

	// Convert back to slice
	mergedFinalizers := make([]string, 0, len(finalizerMap))
	for finalizer := range finalizerMap {
		mergedFinalizers = append(mergedFinalizers, finalizer)
	}

	merged.SetFinalizers(mergedFinalizers)
}

// preserveOwnerReferences merges owner references, preserving downstream-managed ones
func preserveOwnerReferences(existing, merged *unstructured.Unstructured) {
	existingOwnerRefs := existing.GetOwnerReferences()
	desiredOwnerRefs := merged.GetOwnerReferences()

	if len(existingOwnerRefs) == 0 {
		return
	}

	// Preserve existing downstream owner references and add desired ones
	ownerRefMap := make(map[string]interface{})

	// Add existing owner references
	for _, ownerRef := range existingOwnerRefs {
		key := ownerRef.APIVersion + "/" + ownerRef.Kind + "/" + ownerRef.Name
		ownerRefMap[key] = ownerRef
	}

	// Add desired owner references (may override)
	for _, ownerRef := range desiredOwnerRefs {
		key := ownerRef.APIVersion + "/" + ownerRef.Kind + "/" + ownerRef.Name
		ownerRefMap[key] = ownerRef
	}

	// Convert back to slice
	mergedOwnerRefs := make([]interface{}, 0, len(ownerRefMap))
	for _, ownerRef := range ownerRefMap {
		mergedOwnerRefs = append(mergedOwnerRefs, ownerRef)
	}

	if len(mergedOwnerRefs) > 0 {
		if err := unstructured.SetNestedSlice(merged.Object, mergedOwnerRefs, "metadata", "ownerReferences"); err != nil {
			klog.V(2).Info("Failed to preserve owner references", "error", err)
		}
	}
}

// preserveResourceSpecificFields preserves fields based on the resource type
func preserveResourceSpecificFields(existing, merged *unstructured.Unstructured) {
	gvk := existing.GroupVersionKind()

	switch {
	case gvk.Group == "" && gvk.Kind == "Service":
		preserveServiceFields(existing, merged)
	case gvk.Group == "" && gvk.Kind == "PersistentVolume":
		preservePVFields(existing, merged)
	}
}

// preserveServiceFields preserves Service-specific server-managed fields
func preserveServiceFields(existing, merged *unstructured.Unstructured) {
	// Preserve ClusterIP (assigned by server)
	if clusterIP, found, err := unstructured.NestedString(existing.Object, "spec", "clusterIP"); found && err == nil {
		if err := unstructured.SetNestedField(merged.Object, clusterIP, "spec", "clusterIP"); err != nil {
			klog.V(2).Info("Failed to preserve Service clusterIP", "error", err)
		}
	}
}

// preservePVFields preserves PV-specific server-managed fields
func preservePVFields(existing, merged *unstructured.Unstructured) {
	// Preserve binding information
	if claimRef, found, err := unstructured.NestedFieldNoCopy(existing.Object, "spec", "claimRef"); found && err == nil {
		if err := unstructured.SetNestedField(merged.Object, claimRef, "spec", "claimRef"); err != nil {
			klog.V(2).Info("Failed to preserve PV claimRef", "error", err)
		}
	}
}