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

package endpoints

import (
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
)

const (
	// VirtualWorkspacePrefix is the prefix used for virtual workspace resources
	VirtualWorkspacePrefix = "vw-"
	
	// ClusterNameAnnotation stores the original cluster name
	ClusterNameAnnotation = "virtual.kcp.io/cluster-name"
	
	// VirtualNameAnnotation stores the virtual resource name
	VirtualNameAnnotation = "virtual.kcp.io/virtual-name"
	
	// ConflictResolutionAnnotation indicates how conflicts should be resolved
	ConflictResolutionAnnotation = "virtual.kcp.io/conflict-resolution"
)

// ResourceTransformer handles transformation between cluster and virtual resource representations.
// It manages name mapping, conflict resolution, and metadata transformation for resources
// accessed through the virtual workspace endpoints.
type ResourceTransformer struct {
	clusterClient cluster.ClusterInterface
	scheme        *runtime.Scheme
}

// TransformerConfig provides configuration for creating a ResourceTransformer
type TransformerConfig struct {
	ClusterClient cluster.ClusterInterface
	Scheme        *runtime.Scheme
}

// NewResourceTransformer creates a new resource transformer for virtual workspace operations.
//
// Parameters:
//   - config: Configuration containing cluster client and runtime scheme
//
// Returns:
//   - *ResourceTransformer: Configured transformer ready to handle resource transformations
func NewResourceTransformer(config *TransformerConfig) *ResourceTransformer {
	return &ResourceTransformer{
		clusterClient: config.ClusterClient,
		scheme:        config.Scheme,
	}
}

// ToVirtual transforms a cluster resource to its virtual workspace representation.
// This includes adjusting names, adding virtual annotations, and handling cluster-specific metadata.
//
// Parameters:
//   - obj: The cluster resource to transform
//   - clusterName: The name of the source cluster
//
// Returns:
//   - *unstructured.Unstructured: The virtual representation of the resource
//   - error: Transformation error if any
func (t *ResourceTransformer) ToVirtual(obj *unstructured.Unstructured, clusterName string) (*unstructured.Unstructured, error) {
	if obj == nil {
		return nil, fmt.Errorf("cannot transform nil object")
	}

	// Deep copy the object to avoid modifying the original
	virtual := obj.DeepCopy()

	// Transform the name to virtual format
	originalName := virtual.GetName()
	virtualName := t.toVirtualName(originalName, clusterName)
	virtual.SetName(virtualName)

	// Add virtual workspace annotations
	annotations := virtual.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	
	annotations[ClusterNameAnnotation] = clusterName
	annotations[VirtualNameAnnotation] = virtualName
	virtual.SetAnnotations(annotations)

	// Transform namespace if present
	if ns := virtual.GetNamespace(); ns != "" {
		virtual.SetNamespace(t.toVirtualNamespace(ns, clusterName))
	}

	// Handle resource-specific transformations
	if err := t.transformResourceSpecific(virtual, clusterName); err != nil {
		return nil, fmt.Errorf("failed to apply resource-specific transformations: %w", err)
	}

	klog.V(6).Infof("Transformed cluster resource %s/%s to virtual %s/%s", 
		clusterName, originalName, virtual.GetNamespace(), virtualName)

	return virtual, nil
}

// ToCluster transforms a virtual workspace resource to its cluster representation.
// This includes reversing name transformations, removing virtual annotations, and preparing
// the resource for operations in the target cluster.
//
// Parameters:
//   - obj: The virtual resource to transform
//   - clusterName: The name of the target cluster
//
// Returns:
//   - *unstructured.Unstructured: The cluster representation of the resource
//   - error: Transformation error if any
func (t *ResourceTransformer) ToCluster(obj *unstructured.Unstructured, clusterName string) (*unstructured.Unstructured, error) {
	if obj == nil {
		return nil, fmt.Errorf("cannot transform nil object")
	}

	// Deep copy the object to avoid modifying the original
	cluster := obj.DeepCopy()

	// Transform name from virtual to cluster format
	virtualName := cluster.GetName()
	clusterName = t.toClusterName(virtualName, clusterName)
	cluster.SetName(clusterName)

	// Remove virtual workspace annotations but preserve others
	annotations := cluster.GetAnnotations()
	if annotations != nil {
		delete(annotations, VirtualNameAnnotation)
		// Keep ClusterNameAnnotation for tracking purposes
		cluster.SetAnnotations(annotations)
	}

	// Transform namespace if present
	if ns := cluster.GetNamespace(); ns != "" {
		cluster.SetNamespace(t.toClusterNamespace(ns, clusterName))
	}

	// Clear virtual workspace specific fields
	t.clearVirtualFields(cluster)

	klog.V(6).Infof("Transformed virtual resource %s to cluster resource %s/%s", 
		virtualName, clusterName, cluster.GetName())

	return cluster, nil
}

// ListToVirtual transforms a list of cluster resources to their virtual representations.
//
// Parameters:
//   - list: The cluster resource list to transform
//   - clusterName: The name of the source cluster
//
// Returns:
//   - *unstructured.UnstructuredList: The virtual representation of the resource list
//   - error: Transformation error if any
func (t *ResourceTransformer) ListToVirtual(list *unstructured.UnstructuredList, clusterName string) (*unstructured.UnstructuredList, error) {
	if list == nil {
		return nil, fmt.Errorf("cannot transform nil list")
	}

	// Deep copy the list
	virtualList := list.DeepCopy()
	
	// Transform each item in the list
	virtualItems := make([]unstructured.Unstructured, 0, len(list.Items))
	for _, item := range list.Items {
		virtualItem, err := t.ToVirtual(&item, clusterName)
		if err != nil {
			klog.Errorf("Failed to transform list item %s: %v", item.GetName(), err)
			continue // Skip problematic items rather than failing the entire list
		}
		virtualItems = append(virtualItems, *virtualItem)
	}
	
	virtualList.Items = virtualItems
	
	klog.V(6).Infof("Transformed %d cluster resources from %s to virtual list", 
		len(virtualItems), clusterName)

	return virtualList, nil
}

// toVirtualName converts a cluster resource name to virtual format
func (t *ResourceTransformer) toVirtualName(name, clusterName string) string {
	if name == "" {
		return name
	}
	
	// Avoid double-prefixing if already virtual
	if strings.HasPrefix(name, VirtualWorkspacePrefix) {
		return name
	}
	
	return fmt.Sprintf("%s%s-%s", VirtualWorkspacePrefix, clusterName, name)
}

// toClusterName converts a virtual resource name back to cluster format
func (t *ResourceTransformer) toClusterName(virtualName, clusterName string) string {
	if !strings.HasPrefix(virtualName, VirtualWorkspacePrefix) {
		return virtualName
	}
	
	expectedPrefix := fmt.Sprintf("%s%s-", VirtualWorkspacePrefix, clusterName)
	if strings.HasPrefix(virtualName, expectedPrefix) {
		return strings.TrimPrefix(virtualName, expectedPrefix)
	}
	
	return virtualName
}

// toVirtualNamespace converts a cluster namespace to virtual format
func (t *ResourceTransformer) toVirtualNamespace(namespace, clusterName string) string {
	if namespace == "" || namespace == "default" || namespace == "kube-system" {
		return namespace
	}
	return t.toVirtualName(namespace, clusterName)
}

// toClusterNamespace converts a virtual namespace back to cluster format
func (t *ResourceTransformer) toClusterNamespace(virtualNamespace, clusterName string) string {
	if virtualNamespace == "default" || virtualNamespace == "kube-system" {
		return virtualNamespace
	}
	return t.toClusterName(virtualNamespace, clusterName)
}

// transformResourceSpecific applies resource-type specific transformations
// TODO: Implement detailed transformations in follow-up PR
func (t *ResourceTransformer) transformResourceSpecific(obj *unstructured.Unstructured, clusterName string) error {
	// Placeholder for resource-specific transformations
	// Will be implemented in follow-up PR with detailed logic
	return nil
}

// clearVirtualFields removes virtual workspace specific fields from cluster resources
func (t *ResourceTransformer) clearVirtualFields(obj *unstructured.Unstructured) {
	// Remove resource version as it should be set by the target cluster
	obj.SetResourceVersion("")
	
	// Remove UID as it should be generated by the target cluster
	obj.SetUID("")
	
	// Remove creation timestamp as it should be set by the target cluster
	obj.SetCreationTimestamp(metav1.Time{})
}