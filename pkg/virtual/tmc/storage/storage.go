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

package storage

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"
	kcpdynamic "github.com/kcp-dev/client-go/dynamic"

	"github.com/kcp-dev/kcp/pkg/features"
)

// TMCVirtualWorkspaceStorage provides storage operations for TMC resources
// within virtual workspaces, including cross-cluster resource aggregation.
type TMCVirtualWorkspaceStorage struct {
	dynamicClient kcpdynamic.ClusterInterface
	gvr           schema.GroupVersionResource
	workspace     logicalcluster.Name
	isNamespaced  bool
}

// TMCStorageConfig configures the TMC virtual workspace storage.
type TMCStorageConfig struct {
	// DynamicClient provides access to TMC resources across clusters
	DynamicClient kcpdynamic.ClusterInterface

	// GroupVersionResource specifies the TMC resource type
	GroupVersionResource schema.GroupVersionResource

	// Workspace specifies the target logical cluster
	Workspace logicalcluster.Name

	// IsNamespaced indicates if the resource is namespaced
	IsNamespaced bool

	// AllowedWorkspaces restricts which workspaces can be accessed
	AllowedWorkspaces []logicalcluster.Name
}

// NewTMCVirtualWorkspaceStorage creates a new TMC virtual workspace storage.
func NewTMCVirtualWorkspaceStorage(config TMCStorageConfig) *TMCVirtualWorkspaceStorage {
	return &TMCVirtualWorkspaceStorage{
		dynamicClient: config.DynamicClient,
		gvr:          config.GroupVersionResource,
		workspace:    config.Workspace,
		isNamespaced: config.IsNamespaced,
	}
}

// New returns a new empty object for the storage resource type.
func (s *TMCVirtualWorkspaceStorage) New() runtime.Object {
	// Create a new unstructured object for the TMC resource
	obj := &metav1.PartialObjectMetadata{
		TypeMeta: metav1.TypeMeta{
			APIVersion: s.gvr.GroupVersion().String(),
			Kind:       s.getKind(),
		},
	}
	return obj
}

// Destroy cleans up resources when the storage is no longer needed.
func (s *TMCVirtualWorkspaceStorage) Destroy() {
	// Nothing to clean up for dynamic client
}

// Get retrieves a TMC resource by name from the virtual workspace.
func (s *TMCVirtualWorkspaceStorage) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	if !features.DefaultMutableFeatureGate.Enabled(features.TMCEnabled) {
		return nil, fmt.Errorf("TMC virtual workspace storage disabled")
	}

	// Get the resource from the specific workspace
	resourceInterface := s.getResourceInterface("")
	
	obj, err := resourceInterface.Get(ctx, name, *options)
	if err != nil {
		klog.V(4).Infof("Failed to get TMC resource %s/%s: %v", s.gvr.Resource, name, err)
		return nil, err
	}

	klog.V(4).Infof("Retrieved TMC resource %s/%s from workspace %s", s.gvr.Resource, name, s.workspace)
	return obj, nil
}

// List retrieves all TMC resources matching the given options from the virtual workspace.
func (s *TMCVirtualWorkspaceStorage) List(ctx context.Context, options *metav1.ListOptions) (runtime.Object, error) {
	if !features.DefaultMutableFeatureGate.Enabled(features.TMCEnabled) {
		return nil, fmt.Errorf("TMC virtual workspace storage disabled")
	}

	// List resources from the specific workspace
	resourceInterface := s.getResourceInterface("")

	list, err := resourceInterface.List(ctx, *options)
	if err != nil {
		klog.V(4).Infof("Failed to list TMC resources %s: %v", s.gvr.Resource, err)
		return nil, err
	}

	klog.V(4).Infof("Listed %d TMC resources %s from workspace %s", 
		len(list.Items), s.gvr.Resource, s.workspace)
	
	return list, nil
}

// Create creates a new TMC resource in the virtual workspace.
func (s *TMCVirtualWorkspaceStorage) Create(ctx context.Context, obj runtime.Object, createValidation rest.ValidateObjectFunc, options *metav1.CreateOptions) (runtime.Object, error) {
	if !features.DefaultMutableFeatureGate.Enabled(features.TMCEnabled) {
		return nil, fmt.Errorf("TMC virtual workspace storage disabled")
	}

	// Validate the object if validation is provided
	if createValidation != nil {
		if err := createValidation(ctx, obj); err != nil {
			return nil, err
		}
	}

	// Convert to unstructured for dynamic client
	unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, fmt.Errorf("failed to convert object to unstructured: %w", err)
	}

	unstruct := &metav1.Object{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj, unstruct); err != nil {
		return nil, fmt.Errorf("failed to convert to unstructured: %w", err)
	}

	// Create the resource in the specific workspace
	resourceInterface := s.getResourceInterface("")
	
	created, err := resourceInterface.Create(ctx, unstruct.(*metav1.Object), *options)
	if err != nil {
		klog.V(2).Infof("Failed to create TMC resource %s: %v", s.gvr.Resource, err)
		return nil, err
	}

	klog.V(4).Infof("Created TMC resource %s in workspace %s", s.gvr.Resource, s.workspace)
	return created, nil
}

// Update updates an existing TMC resource in the virtual workspace.
func (s *TMCVirtualWorkspaceStorage) Update(ctx context.Context, name string, objInfo rest.UpdatedObjectInfo, createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc, forceAllowCreate bool, options *metav1.UpdateOptions) (runtime.Object, bool, error) {
	if !features.DefaultMutableFeatureGate.Enabled(features.TMCEnabled) {
		return nil, false, fmt.Errorf("TMC virtual workspace storage disabled")
	}

	// Get the current object
	resourceInterface := s.getResourceInterface("")
	current, err := resourceInterface.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if !forceAllowCreate {
			return nil, false, err
		}
		// Create new object if not found and creation is allowed
		obj, err := objInfo.UpdatedObject(ctx, nil)
		if err != nil {
			return nil, false, err
		}
		created, err := s.Create(ctx, obj, createValidation, &metav1.CreateOptions{})
		return created, true, err
	}

	// Get the updated object
	updated, err := objInfo.UpdatedObject(ctx, current)
	if err != nil {
		return nil, false, err
	}

	// Validate the update if validation is provided
	if updateValidation != nil {
		if err := updateValidation(ctx, updated, current); err != nil {
			return nil, false, err
		}
	}

	// Convert to unstructured for dynamic client
	unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(updated)
	if err != nil {
		return nil, false, fmt.Errorf("failed to convert object to unstructured: %w", err)
	}

	unstruct := &metav1.Object{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj, unstruct); err != nil {
		return nil, false, fmt.Errorf("failed to convert to unstructured: %w", err)
	}

	// Update the resource in the specific workspace
	result, err := resourceInterface.Update(ctx, unstruct.(*metav1.Object), *options)
	if err != nil {
		klog.V(2).Infof("Failed to update TMC resource %s/%s: %v", s.gvr.Resource, name, err)
		return nil, false, err
	}

	klog.V(4).Infof("Updated TMC resource %s/%s in workspace %s", s.gvr.Resource, name, s.workspace)
	return result, false, nil
}

// Delete deletes a TMC resource from the virtual workspace.
func (s *TMCVirtualWorkspaceStorage) Delete(ctx context.Context, name string, deleteValidation rest.ValidateObjectFunc, options *metav1.DeleteOptions) (runtime.Object, bool, error) {
	if !features.DefaultMutableFeatureGate.Enabled(features.TMCEnabled) {
		return nil, false, fmt.Errorf("TMC virtual workspace storage disabled")
	}

	// Get the object first for validation
	resourceInterface := s.getResourceInterface("")
	obj, err := resourceInterface.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, false, err
	}

	// Validate the delete if validation is provided
	if deleteValidation != nil {
		if err := deleteValidation(ctx, obj); err != nil {
			return nil, false, err
		}
	}

	// Delete the resource from the specific workspace
	if err := resourceInterface.Delete(ctx, name, *options); err != nil {
		klog.V(2).Infof("Failed to delete TMC resource %s/%s: %v", s.gvr.Resource, name, err)
		return nil, false, err
	}

	klog.V(4).Infof("Deleted TMC resource %s/%s from workspace %s", s.gvr.Resource, name, s.workspace)
	return obj, true, nil
}

// getResourceInterface returns the appropriate dynamic resource interface.
func (s *TMCVirtualWorkspaceStorage) getResourceInterface(namespace string) dynamic.ResourceInterface {
	clusterResource := s.dynamicClient.Cluster(s.workspace).Resource(s.gvr)
	
	if s.isNamespaced && namespace != "" {
		return clusterResource.Namespace(namespace)
	}
	
	return clusterResource
}

// getKind returns the Kind name for the resource.
func (s *TMCVirtualWorkspaceStorage) getKind() string {
	// Convert resource name to Kind name (e.g., clusterregistrations -> ClusterRegistration)
	resource := s.gvr.Resource
	if len(resource) == 0 {
		return ""
	}
	
	// Simple conversion: remove 's' suffix and capitalize
	kind := resource
	if strings.HasSuffix(kind, "s") {
		kind = kind[:len(kind)-1]
	}
	
	// Capitalize first letter
	if len(kind) > 0 {
		kind = strings.ToUpper(kind[:1]) + kind[1:]
	}
	
	return kind
}