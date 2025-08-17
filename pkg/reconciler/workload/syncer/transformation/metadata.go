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
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
)

// metadataTransformer handles label and annotation transformations during synchronization.
// It adds management labels, preserves important annotations, and filters sensitive metadata.
type metadataTransformer struct {
	// Annotations to preserve when syncing upstream
	preserveAnnotations map[string]bool
	
	// Annotations to remove when syncing downstream (KCP-internal)
	removeAnnotations map[string]bool
	
	// Labels to preserve during transformations
	preserveLabels map[string]bool
}

// NewMetadataTransformer creates a new metadata transformer with default filtering rules.
func NewMetadataTransformer() ResourceTransformer {
	return &metadataTransformer{
		preserveAnnotations: map[string]bool{
			// Kubernetes system annotations to preserve
			"kubernetes.io/ingress.class":                    true,
			"kubernetes.io/ingress.global-static-ip-name":   true,
			"service.beta.kubernetes.io/aws-load-balancer-": true, // prefix match
			"service.beta.kubernetes.io/azure-":             true, // prefix match
			"service.beta.kubernetes.io/gce-":               true, // prefix match
			
			// Application annotations
			"app.kubernetes.io/name":       true,
			"app.kubernetes.io/instance":   true,
			"app.kubernetes.io/version":    true,
			"app.kubernetes.io/component":  true,
			"app.kubernetes.io/part-of":    true,
			"app.kubernetes.io/managed-by": true,
		},
		
		removeAnnotations: map[string]bool{
			// KCP internal annotations to filter out
			"apis.kcp.io/":              true, // prefix match
			"core.kcp.io/":              true, // prefix match
			"tenancy.kcp.io/":           true, // prefix match
			"topology.kcp.io/":          true, // prefix match
			"scheduling.kcp.io/":        true, // prefix match
			"workload.kcp.io/":          true, // prefix match
			"internal.kcp.io/":          true, // prefix match
			
			// kubectl annotations that shouldn't be synced
			"kubectl.kubernetes.io/last-applied-configuration": true,
		},
		
		preserveLabels: map[string]bool{
			// Standard Kubernetes labels
			"app.kubernetes.io/name":       true,
			"app.kubernetes.io/instance":   true,
			"app.kubernetes.io/version":    true,
			"app.kubernetes.io/component":  true,
			"app.kubernetes.io/part-of":    true,
			"app.kubernetes.io/managed-by": true,
			
			// Common application labels
			"app":     true,
			"version": true,
			"tier":    true,
			"role":    true,
		},
	}
}

// Name returns the transformer name
func (t *metadataTransformer) Name() string {
	return "metadata-transformer"
}

// ShouldTransform returns true for all objects with metadata
func (t *metadataTransformer) ShouldTransform(obj runtime.Object) bool {
	if obj == nil {
		return false
	}
	
	_, ok := obj.(metav1.Object)
	return ok
}

// TransformForDownstream adds management labels and filters sensitive annotations when syncing to physical clusters
func (t *metadataTransformer) TransformForDownstream(ctx context.Context, obj runtime.Object, target *SyncTarget) (runtime.Object, error) {
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
	
	// Transform labels
	t.transformLabelsForDownstream(metaResult, target)
	
	// Transform annotations
	t.transformAnnotationsForDownstream(metaResult, target)
	
	klog.V(5).InfoS("Applied metadata transformations for downstream",
		"objectKind", getObjectKind(obj),
		"namespace", metaResult.GetNamespace(),
		"name", metaResult.GetName(),
		"targetCluster", target.Spec.ClusterName)
	
	return result, nil
}

// TransformForUpstream removes management labels and restores original annotations when syncing back to KCP
func (t *metadataTransformer) TransformForUpstream(ctx context.Context, obj runtime.Object, source *SyncTarget) (runtime.Object, error) {
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
	
	// Transform labels for upstream
	t.transformLabelsForUpstream(metaResult, source)
	
	// Transform annotations for upstream
	t.transformAnnotationsForUpstream(metaResult, source)
	
	klog.V(5).InfoS("Applied metadata transformations for upstream",
		"objectKind", getObjectKind(obj),
		"namespace", metaResult.GetNamespace(),
		"name", metaResult.GetName(),
		"sourceCluster", source.Spec.ClusterName)
	
	return result, nil
}

// transformLabelsForDownstream adds management labels for downstream sync
func (t *metadataTransformer) transformLabelsForDownstream(obj metav1.Object, target *SyncTarget) {
	labels := obj.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	
	// Add TMC management labels
	labels["syncer.kcp.io/managed"] = "true"
	labels["syncer.kcp.io/cluster"] = target.Spec.ClusterName
	labels["syncer.kcp.io/sync-target"] = target.GetName()
	
	// Add sync timestamp
	labels["syncer.kcp.io/last-sync"] = fmt.Sprintf("%d", time.Now().Unix())
	
	obj.SetLabels(labels)
}

// transformLabelsForUpstream removes management labels for upstream sync
func (t *metadataTransformer) transformLabelsForUpstream(obj metav1.Object, source *SyncTarget) {
	labels := obj.GetLabels()
	if labels == nil {
		return
	}
	
	// Remove syncer-added labels
	for key := range labels {
		if strings.HasPrefix(key, "syncer.kcp.io/") {
			delete(labels, key)
		}
	}
	
	// Clean up empty labels map
	if len(labels) == 0 {
		labels = nil
	}
	
	obj.SetLabels(labels)
}

// transformAnnotationsForDownstream adds management annotations and filters sensitive ones for downstream sync
func (t *metadataTransformer) transformAnnotationsForDownstream(obj metav1.Object, target *SyncTarget) {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	
	// Store original annotations for potential restoration
	originalAnnotations := make(map[string]string)
	for key, value := range annotations {
		originalAnnotations[key] = value
	}
	
	// Remove KCP-internal annotations
	for key := range annotations {
		if t.shouldRemoveAnnotation(key) {
			delete(annotations, key)
		}
	}
	
	// Add management annotations
	annotations["syncer.kcp.io/managed"] = "true"
	annotations["syncer.kcp.io/cluster"] = target.Spec.ClusterName
	annotations["syncer.kcp.io/sync-target"] = target.GetName()
	annotations["syncer.kcp.io/sync-time"] = time.Now().Format(time.RFC3339)
	
	// Store original generation for comparison
	annotations["syncer.kcp.io/generation"] = fmt.Sprintf("%d", obj.GetGeneration())
	
	obj.SetAnnotations(annotations)
}

// transformAnnotationsForUpstream removes management annotations and preserves important ones for upstream sync
func (t *metadataTransformer) transformAnnotationsForUpstream(obj metav1.Object, source *SyncTarget) {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		return
	}
	
	// Remove syncer-added annotations
	for key := range annotations {
		if strings.HasPrefix(key, "syncer.kcp.io/") {
			delete(annotations, key)
		}
	}
	
	// Clean up empty annotations map
	if len(annotations) == 0 {
		annotations = nil
	}
	
	obj.SetAnnotations(annotations)
}

// shouldRemoveAnnotation checks if an annotation should be removed during downstream sync
func (t *metadataTransformer) shouldRemoveAnnotation(key string) bool {
	// Direct match
	if remove, exists := t.removeAnnotations[key]; exists && remove {
		return true
	}
	
	// Check prefix matches
	for prefix := range t.removeAnnotations {
		if strings.HasSuffix(prefix, "/") && strings.HasPrefix(key, prefix) {
			return true
		}
	}
	
	return false
}

// shouldPreserveAnnotation checks if an annotation should be preserved during transformations
func (t *metadataTransformer) shouldPreserveAnnotation(key string) bool {
	// Direct match
	if preserve, exists := t.preserveAnnotations[key]; exists && preserve {
		return true
	}
	
	// Check prefix matches
	for prefix := range t.preserveAnnotations {
		if strings.HasSuffix(prefix, "-") && strings.HasPrefix(key, prefix) {
			return true
		}
	}
	
	return false
}