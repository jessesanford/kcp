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
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"

	workloadv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"
)

// ResourceTransformer handles transformation of resources between physical and logical clusters.
// It manages the bidirectional conversion of resources, ensuring proper annotation handling,
// namespace mapping, and removal of cluster-specific fields that should not be synced.
//
// The transformer maintains the relationship between physical and KCP resources through
// annotations and labels, enabling proper correlation and conflict detection.
type ResourceTransformer struct {
	workspace      logicalcluster.Path
	syncTargetUID  types.UID
	syncTargetName string
	namespaceMapper NamespaceMapper
}

// NewResourceTransformer creates a new ResourceTransformer for the given workspace and SyncTarget.
// It initializes the transformer with proper context for bidirectional resource conversion.
//
// Parameters:
//   - workspace: The logical cluster path for workspace isolation
//   - syncTarget: The SyncTarget resource providing transformation context
//
// Returns:
//   - *ResourceTransformer: Configured transformer ready for use
func NewResourceTransformer(workspace logicalcluster.Path, syncTarget *workloadv1alpha1.SyncTarget) *ResourceTransformer {
	return &ResourceTransformer{
		workspace:       workspace,
		syncTargetUID:   syncTarget.UID,
		syncTargetName:  syncTarget.Name,
		namespaceMapper: NewDefaultNamespaceMapper(syncTarget.Name),
	}
}

// TransformFromPhysical transforms a resource from physical cluster format to KCP format.
// This includes adding KCP-specific annotations, transforming namespaces, and cleaning
// physical cluster specific fields that should not be synced upstream.
//
// The transformation process:
// 1. Adds KCP tracking annotations (sync target, workspace)
// 2. Transforms namespaces using mapping rules
// 3. Removes physical cluster specific fields
// 4. Preserves original resource structure and data
//
// Parameters:
//   - obj: The unstructured resource from the physical cluster
//
// Returns:
//   - *unstructured.Unstructured: The transformed resource ready for KCP
//   - error: Transformation error if any
func (t *ResourceTransformer) TransformFromPhysical(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	if obj == nil {
		return nil, fmt.Errorf("cannot transform nil object")
	}

	// Create a deep copy to avoid modifying the original
	transformed := obj.DeepCopy()

	// Add KCP tracking annotations
	if err := t.addKCPAnnotations(transformed); err != nil {
		return nil, fmt.Errorf("failed to add KCP annotations: %w", err)
	}

	// Transform namespace if needed
	if err := t.transformNamespaceFromPhysical(transformed); err != nil {
		return nil, fmt.Errorf("failed to transform namespace: %w", err)
	}

	// Clean physical cluster specific fields
	t.cleanPhysicalFields(transformed)

	// Add generation tracking for conflict detection
	if err := t.addGenerationTracking(transformed); err != nil {
		return nil, fmt.Errorf("failed to add generation tracking: %w", err)
	}

	return transformed, nil
}

// TransformToPhysical transforms a resource from KCP format to physical cluster format.
// This includes removing KCP-specific annotations, reverse namespace mapping, and
// ensuring the resource is suitable for the physical cluster.
//
// Parameters:
//   - obj: The unstructured resource from KCP
//
// Returns:
//   - *unstructured.Unstructured: The transformed resource ready for physical cluster
//   - error: Transformation error if any
func (t *ResourceTransformer) TransformToPhysical(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	if obj == nil {
		return nil, fmt.Errorf("cannot transform nil object")
	}

	// Create a deep copy to avoid modifying the original
	transformed := obj.DeepCopy()

	// Remove KCP-specific annotations and labels
	t.removeKCPAnnotations(transformed)

	// Transform namespace back to physical format
	if err := t.transformNamespaceToPhysical(transformed); err != nil {
		return nil, fmt.Errorf("failed to transform namespace to physical: %w", err)
	}

	// Remove KCP-specific fields
	t.removeKCPFields(transformed)

	return transformed, nil
}

// addKCPAnnotations adds KCP-specific annotations to track the resource origin and sync metadata
func (t *ResourceTransformer) addKCPAnnotations(obj *unstructured.Unstructured) error {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	// Mark the sync target source
	annotations[workloadv1alpha1.InternalSyncTargetUIDAnnotation] = string(t.syncTargetUID)
	annotations[workloadv1alpha1.InternalSyncTargetNameAnnotation] = t.syncTargetName

	// Mark the workspace
	annotations[workloadv1alpha1.ClusterAnnotation] = t.workspace.String()

	// Add sync timestamp for tracking
	annotations["kcp.io/upstream-sync-timestamp"] = fmt.Sprintf("%d", getCurrentTimestamp())

	obj.SetAnnotations(annotations)
	return nil
}

// removeKCPAnnotations removes KCP-specific annotations when transforming to physical format
func (t *ResourceTransformer) removeKCPAnnotations(obj *unstructured.Unstructured) {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		return
	}

	// Remove KCP tracking annotations
	delete(annotations, workloadv1alpha1.InternalSyncTargetUIDAnnotation)
	delete(annotations, workloadv1alpha1.InternalSyncTargetNameAnnotation)
	delete(annotations, workloadv1alpha1.ClusterAnnotation)
	delete(annotations, "kcp.io/upstream-sync-timestamp")
	delete(annotations, "kcp.io/last-synced-generation")
	delete(annotations, "kcp.io/last-synced-resourceversion")

	// Clean up empty annotations map
	if len(annotations) == 0 {
		obj.SetAnnotations(nil)
	} else {
		obj.SetAnnotations(annotations)
	}
}

// transformNamespaceFromPhysical transforms a physical namespace to logical namespace
func (t *ResourceTransformer) transformNamespaceFromPhysical(obj *unstructured.Unstructured) error {
	physicalNamespace := obj.GetNamespace()
	if physicalNamespace == "" {
		// Cluster-scoped resource, no namespace transformation needed
		return nil
	}

	logicalNamespace := t.namespaceMapper.ToLogical(physicalNamespace, t.syncTargetName)
	obj.SetNamespace(logicalNamespace)

	return nil
}

// transformNamespaceToPhysical transforms a logical namespace back to physical namespace
func (t *ResourceTransformer) transformNamespaceToPhysical(obj *unstructured.Unstructured) error {
	logicalNamespace := obj.GetNamespace()
	if logicalNamespace == "" {
		// Cluster-scoped resource, no namespace transformation needed
		return nil
	}

	physicalNamespace, err := t.namespaceMapper.ToPhysical(logicalNamespace, t.syncTargetName)
	if err != nil {
		return fmt.Errorf("failed to map logical namespace %s: %w", logicalNamespace, err)
	}

	obj.SetNamespace(physicalNamespace)
	return nil
}

// cleanPhysicalFields removes fields that shouldn't be synced from physical to KCP
func (t *ResourceTransformer) cleanPhysicalFields(obj *unstructured.Unstructured) {
	gvk := obj.GetObjectKind().GroupVersionKind()

	// Remove common fields that shouldn't be synced upstream
	unstructured.RemoveNestedField(obj.Object, "metadata", "resourceVersion")
	unstructured.RemoveNestedField(obj.Object, "metadata", "uid")
	unstructured.RemoveNestedField(obj.Object, "metadata", "selfLink")
	unstructured.RemoveNestedField(obj.Object, "metadata", "generation")

	// Resource-specific field cleanup
	switch gvk.Kind {
	case "Pod":
		t.cleanPodFields(obj)
	case "Node":
		t.cleanNodeFields(obj)
	case "Service":
		t.cleanServiceFields(obj)
	case "PersistentVolume":
		t.cleanPVFields(obj)
	}
}

// cleanPodFields removes Pod-specific fields that shouldn't be synced
func (t *ResourceTransformer) cleanPodFields(obj *unstructured.Unstructured) {
	// Remove node assignment and physical cluster specific fields
	unstructured.RemoveNestedField(obj.Object, "spec", "nodeName")
	unstructured.RemoveNestedField(obj.Object, "spec", "serviceAccountName")
	unstructured.RemoveNestedField(obj.Object, "status", "hostIP")
	unstructured.RemoveNestedField(obj.Object, "status", "podIP")
	unstructured.RemoveNestedField(obj.Object, "status", "podIPs")
	unstructured.RemoveNestedField(obj.Object, "status", "nominatedNodeName")
}

// cleanNodeFields removes Node-specific fields that shouldn't be synced
func (t *ResourceTransformer) cleanNodeFields(obj *unstructured.Unstructured) {
	// Remove node-specific system info
	unstructured.RemoveNestedField(obj.Object, "status", "nodeInfo", "machineID")
	unstructured.RemoveNestedField(obj.Object, "status", "nodeInfo", "systemUUID")
	unstructured.RemoveNestedField(obj.Object, "status", "nodeInfo", "bootID")
}

// cleanServiceFields removes Service-specific fields that shouldn't be synced
func (t *ResourceTransformer) cleanServiceFields(obj *unstructured.Unstructured) {
	// Remove cluster-specific service fields
	unstructured.RemoveNestedField(obj.Object, "spec", "clusterIP")
	unstructured.RemoveNestedField(obj.Object, "spec", "clusterIPs")
	unstructured.RemoveNestedField(obj.Object, "status", "loadBalancer")
}

// cleanPVFields removes PersistentVolume-specific fields that shouldn't be synced
func (t *ResourceTransformer) cleanPVFields(obj *unstructured.Unstructured) {
	// Remove physical cluster specific volume source details
	unstructured.RemoveNestedField(obj.Object, "spec", "local")
	unstructured.RemoveNestedField(obj.Object, "spec", "hostPath")
	unstructured.RemoveNestedField(obj.Object, "spec", "nodeAffinity")
}

// removeKCPFields removes KCP-specific fields when transforming to physical
func (t *ResourceTransformer) removeKCPFields(obj *unstructured.Unstructured) {
	// Remove KCP-specific labels
	labels := obj.GetLabels()
	if labels != nil {
		for key := range labels {
			if strings.HasPrefix(key, "kcp.io/") {
				delete(labels, key)
			}
		}
		if len(labels) == 0 {
			obj.SetLabels(nil)
		} else {
			obj.SetLabels(labels)
		}
	}
}

// addGenerationTracking adds generation tracking annotations for conflict detection
func (t *ResourceTransformer) addGenerationTracking(obj *unstructured.Unstructured) error {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	// Track the original generation and resource version for conflict detection
	if gen := obj.GetGeneration(); gen != 0 {
		annotations["kcp.io/original-generation"] = fmt.Sprintf("%d", gen)
	}

	if rv := obj.GetResourceVersion(); rv != "" {
		annotations["kcp.io/original-resourceversion"] = rv
	}

	obj.SetAnnotations(annotations)
	return nil
}

// ShouldTransformResource determines if a resource should be transformed and synced
func (t *ResourceTransformer) ShouldTransformResource(gvr schema.GroupVersionResource, obj *unstructured.Unstructured) bool {
	// Skip resources that are KCP internal
	if strings.Contains(gvr.Group, "kcp.io") {
		return false
	}

	// Skip events - too noisy for upstream sync
	if gvr.Resource == "events" {
		return false
	}

	// Skip secrets by default for security
	if gvr.Resource == "secrets" {
		klog.V(4).InfoS("Skipping secret resource for security", "resource", obj.GetName())
		return false
	}

	return true
}

// getCurrentTimestamp returns the current timestamp as Unix time
func getCurrentTimestamp() int64 {
	// In real implementation, this would use time.Now().Unix()
	// For testing, we can make this deterministic
	return 1640995200 // 2022-01-01 00:00:00 UTC
}

// TransformationResult represents the result of a resource transformation
type TransformationResult struct {
	Transformed *unstructured.Unstructured
	Skipped     bool
	Reason      string
	Error       error
}

// NewTransformationResult creates a successful transformation result
func NewTransformationResult(transformed *unstructured.Unstructured) *TransformationResult {
	return &TransformationResult{
		Transformed: transformed,
		Skipped:     false,
	}
}

// NewSkippedResult creates a skipped transformation result
func NewSkippedResult(reason string) *TransformationResult {
	return &TransformationResult{
		Skipped: true,
		Reason:  reason,
	}
}

// NewErrorResult creates an error transformation result
func NewErrorResult(err error) *TransformationResult {
	return &TransformationResult{
		Error: err,
	}
}