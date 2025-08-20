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

	"github.com/kcp-dev/logicalcluster/v3"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
)

// namespaceTransformer handles namespace mapping between KCP workspaces and physical clusters.
// It adds workspace prefixes to namespace names when syncing downstream and removes them
// when syncing upstream to maintain namespace isolation.
type namespaceTransformer struct {
	workspace      logicalcluster.Name
	namespacePrefix string
	
	// systemNamespaces that should not be transformed
	systemNamespaces map[string]bool
}

// NewNamespaceTransformer creates a new namespace transformer for the given workspace.
func NewNamespaceTransformer(workspace logicalcluster.Name) ResourceTransformer {
	return &namespaceTransformer{
		workspace: workspace,
		namespacePrefix: generateNamespacePrefix(workspace),
		systemNamespaces: map[string]bool{
			"kube-system":         true,
			"kube-public":         true,
			"kube-node-lease":     true,
			"default":             false, // default namespace should be transformed
			"kcp-system":          true,
			"local-path-storage":  true,
		},
	}
}

// Name returns the transformer name
func (t *namespaceTransformer) Name() string {
	return "namespace-transformer"
}

// ShouldTransform returns true if the object has a namespace that needs transformation
func (t *namespaceTransformer) ShouldTransform(obj runtime.Object) bool {
	if obj == nil {
		return false
	}
	
	// Check if object is namespace-scoped
	metaObj, ok := obj.(metav1.Object)
	if !ok {
		return false
	}
	
	// Only transform objects that have namespaces
	namespace := metaObj.GetNamespace()
	return namespace != ""
}

// TransformForDownstream adds workspace prefix to namespace names when syncing to physical clusters
func (t *namespaceTransformer) TransformForDownstream(ctx context.Context, obj runtime.Object, target *SyncTarget) (runtime.Object, error) {
	if obj == nil {
		return nil, fmt.Errorf("cannot transform nil object")
	}
	
	metaObj, ok := obj.(metav1.Object)
	if !ok {
		return obj, nil // Not a metadata object, pass through
	}
	
	originalNamespace := metaObj.GetNamespace()
	if originalNamespace == "" {
		return obj, nil // Cluster-scoped resource
	}
	
	// Check if this is a system namespace that should be preserved
	if shouldPreserve, exists := t.systemNamespaces[originalNamespace]; exists && shouldPreserve {
		klog.V(5).InfoS("Preserving system namespace",
			"namespace", originalNamespace,
			"workspace", t.workspace)
		return obj, nil
	}
	
	// Transform the namespace by adding prefix
	transformedNamespace := t.addNamespacePrefix(originalNamespace)
	
	klog.V(5).InfoS("Transforming namespace for downstream",
		"originalNamespace", originalNamespace,
		"transformedNamespace", transformedNamespace,
		"workspace", t.workspace,
		"objectKind", getObjectKind(obj))
	
	// Create a copy and update the namespace
	result := obj.DeepCopyObject()
	metaResult, _ := result.(metav1.Object)
	metaResult.SetNamespace(transformedNamespace)
	
	// Store original namespace in annotation for reverse transformation
	annotations := metaResult.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations["syncer.kcp.io/original-namespace"] = originalNamespace
	metaResult.SetAnnotations(annotations)
	
	return result, nil
}

// TransformForUpstream removes workspace prefix from namespace names when syncing back to KCP
func (t *namespaceTransformer) TransformForUpstream(ctx context.Context, obj runtime.Object, source *SyncTarget) (runtime.Object, error) {
	if obj == nil {
		return nil, fmt.Errorf("cannot transform nil object")
	}
	
	metaObj, ok := obj.(metav1.Object)
	if !ok {
		return obj, nil // Not a metadata object, pass through
	}
	
	transformedNamespace := metaObj.GetNamespace()
	if transformedNamespace == "" {
		return obj, nil // Cluster-scoped resource
	}
	
	// Check if this is a system namespace that should be preserved
	if shouldPreserve, exists := t.systemNamespaces[transformedNamespace]; exists && shouldPreserve {
		klog.V(5).InfoS("Preserving system namespace",
			"namespace", transformedNamespace,
			"workspace", t.workspace)
		return obj, nil
	}
	
	// First try to get original namespace from annotation
	annotations := metaObj.GetAnnotations()
	if annotations != nil {
		if originalNamespace, exists := annotations["syncer.kcp.io/original-namespace"]; exists {
			klog.V(5).InfoS("Restoring namespace from annotation for upstream",
				"transformedNamespace", transformedNamespace,
				"originalNamespace", originalNamespace,
				"workspace", t.workspace,
				"objectKind", getObjectKind(obj))
			
			// Create a copy and restore the original namespace
			result := obj.DeepCopyObject()
			metaResult, _ := result.(metav1.Object)
			metaResult.SetNamespace(originalNamespace)
			
			// Remove the transformation annotation
			resultAnnotations := metaResult.GetAnnotations()
			delete(resultAnnotations, "syncer.kcp.io/original-namespace")
			if len(resultAnnotations) == 0 {
				resultAnnotations = nil
			}
			metaResult.SetAnnotations(resultAnnotations)
			
			return result, nil
		}
	}
	
	// Fallback: try to remove the prefix if present
	originalNamespace := t.removeNamespacePrefix(transformedNamespace)
	if originalNamespace != transformedNamespace {
		klog.V(5).InfoS("Removing namespace prefix for upstream",
			"transformedNamespace", transformedNamespace,
			"originalNamespace", originalNamespace,
			"workspace", t.workspace,
			"objectKind", getObjectKind(obj))
		
		// Create a copy and restore the namespace
		result := obj.DeepCopyObject()
		metaResult, _ := result.(metav1.Object)
		metaResult.SetNamespace(originalNamespace)
		
		return result, nil
	}
	
	// No transformation needed
	return obj, nil
}

// generateNamespacePrefix creates a namespace prefix based on the workspace name
func generateNamespacePrefix(workspace logicalcluster.Name) string {
	// Convert workspace name to a valid DNS label
	prefix := string(workspace)
	if prefix == "" {
		prefix = "root"
	}
	
	// Replace invalid characters with dashes and ensure it meets DNS requirements
	prefix = strings.ReplaceAll(prefix, ":", "-")
	prefix = strings.ReplaceAll(prefix, "/", "-")
	prefix = strings.ToLower(prefix)
	
	// Ensure it doesn't exceed length limits and ends appropriately
	if len(prefix) > 15 { // Leave room for namespace name
		prefix = prefix[:15]
	}
	
	// Remove trailing dash if present
	prefix = strings.TrimRight(prefix, "-")
	
	return prefix
}

// addNamespacePrefix adds the workspace prefix to a namespace name
func (t *namespaceTransformer) addNamespacePrefix(namespace string) string {
	if namespace == "" {
		return ""
	}
	
	// Don't add prefix if already present
	expectedPrefix := t.namespacePrefix + "-"
	if strings.HasPrefix(namespace, expectedPrefix) {
		return namespace
	}
	
	return t.namespacePrefix + "-" + namespace
}

// removeNamespacePrefix removes the workspace prefix from a namespace name
func (t *namespaceTransformer) removeNamespacePrefix(namespace string) string {
	if namespace == "" {
		return ""
	}
	
	expectedPrefix := t.namespacePrefix + "-"
	if strings.HasPrefix(namespace, expectedPrefix) {
		return namespace[len(expectedPrefix):]
	}
	
	return namespace
}