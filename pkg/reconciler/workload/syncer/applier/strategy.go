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

package applier

import (
	"context"
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
)

// ApplyStrategy defines how resources should be applied.
type ApplyStrategy string

const (
	// ServerSideApply uses Kubernetes server-side apply
	ServerSideApply ApplyStrategy = "server-side-apply"
	// StrategicMerge uses strategic merge patch
	StrategicMerge ApplyStrategy = "strategic-merge"
	// Replace replaces the entire resource
	Replace ApplyStrategy = "replace"
)

// serverSideApply applies a resource using server-side apply.
func (a *Applier) serverSideApply(ctx context.Context, gvr schema.GroupVersionResource, obj *unstructured.Unstructured, result *ApplyResult) error {
	logger := klog.FromContext(ctx)
	
	// Marshal the object for the patch
	data, err := json.Marshal(obj)
	if err != nil {
		return fmt.Errorf("failed to marshal object for server-side apply: %w", err)
	}
	
	// Apply using server-side apply
	applied, err := a.client.Resource(gvr).
		Namespace(obj.GetNamespace()).
		Patch(ctx, obj.GetName(), types.ApplyPatchType, data, metav1.PatchOptions{
			FieldManager: a.fieldManager,
			Force:        &a.forceConflicts,
		})
	
	if err != nil {
		return fmt.Errorf("server-side apply failed: %w", err)
	}
	
	result.Applied = applied
	result.Operation = "apply"
	
	logger.V(4).Info("Successfully applied resource using server-side apply",
		"gvr", gvr,
		"name", obj.GetName(),
		"namespace", obj.GetNamespace())
	
	return nil
}

// strategicMerge applies a resource using strategic merge patch.
func (a *Applier) strategicMerge(ctx context.Context, gvr schema.GroupVersionResource, obj *unstructured.Unstructured, result *ApplyResult) error {
	logger := klog.FromContext(ctx)
	
	// First, try to get the existing resource
	existing, err := a.client.Resource(gvr).Namespace(obj.GetNamespace()).Get(ctx, obj.GetName(), metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// Resource doesn't exist, create it
			return a.createResource(ctx, gvr, obj, result)
		}
		return fmt.Errorf("failed to get existing resource for strategic merge: %w", err)
	}
	
	// Calculate the patch by comparing desired vs current
	patch, err := a.calculateStrategicMergePatch(existing, obj)
	if err != nil {
		return fmt.Errorf("failed to calculate strategic merge patch: %w", err)
	}
	
	// If patch is empty, no update needed
	if len(patch) == 0 {
		result.Applied = existing
		result.Operation = "noop"
		logger.V(4).Info("No changes needed for strategic merge", "gvr", gvr, "name", obj.GetName())
		return nil
	}
	
	// Apply the patch
	applied, err := a.client.Resource(gvr).
		Namespace(obj.GetNamespace()).
		Patch(ctx, obj.GetName(), types.StrategicMergePatchType, patch, metav1.PatchOptions{})
	
	if err != nil {
		return fmt.Errorf("strategic merge patch failed: %w", err)
	}
	
	result.Applied = applied
	result.Operation = "update"
	
	logger.V(4).Info("Successfully updated resource using strategic merge",
		"gvr", gvr,
		"name", obj.GetName(),
		"namespace", obj.GetNamespace())
	
	return nil
}

// replace replaces the entire resource.
func (a *Applier) replace(ctx context.Context, gvr schema.GroupVersionResource, obj *unstructured.Unstructured, result *ApplyResult) error {
	logger := klog.FromContext(ctx)
	
	// First, try to get the existing resource to preserve resourceVersion
	existing, err := a.client.Resource(gvr).Namespace(obj.GetNamespace()).Get(ctx, obj.GetName(), metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// Resource doesn't exist, create it
			return a.createResource(ctx, gvr, obj, result)
		}
		return fmt.Errorf("failed to get existing resource for replace: %w", err)
	}
	
	// Preserve the resourceVersion for optimistic concurrency control
	obj.SetResourceVersion(existing.GetResourceVersion())
	
	// Replace the resource
	applied, err := a.client.Resource(gvr).
		Namespace(obj.GetNamespace()).
		Update(ctx, obj, metav1.UpdateOptions{})
	
	if err != nil {
		return fmt.Errorf("replace operation failed: %w", err)
	}
	
	result.Applied = applied
	result.Operation = "replace"
	
	logger.V(4).Info("Successfully replaced resource",
		"gvr", gvr,
		"name", obj.GetName(),
		"namespace", obj.GetNamespace())
	
	return nil
}

// createResource creates a new resource.
func (a *Applier) createResource(ctx context.Context, gvr schema.GroupVersionResource, obj *unstructured.Unstructured, result *ApplyResult) error {
	logger := klog.FromContext(ctx)
	
	// Clear resourceVersion for create
	obj.SetResourceVersion("")
	
	applied, err := a.client.Resource(gvr).
		Namespace(obj.GetNamespace()).
		Create(ctx, obj, metav1.CreateOptions{})
	
	if err != nil {
		return fmt.Errorf("create operation failed: %w", err)
	}
	
	result.Applied = applied
	result.Operation = "create"
	
	logger.V(4).Info("Successfully created resource",
		"gvr", gvr,
		"name", obj.GetName(),
		"namespace", obj.GetNamespace())
	
	return nil
}

// calculateStrategicMergePatch calculates the strategic merge patch between current and desired resources.
func (a *Applier) calculateStrategicMergePatch(current, desired *unstructured.Unstructured) ([]byte, error) {
	// For simplicity, we'll use a basic comparison approach
	// In a full implementation, you'd use the strategic merge patch logic from kubectl
	
	// Get the specs to compare
	currentSpec, found, err := unstructured.NestedMap(current.Object, "spec")
	if err != nil {
		return nil, fmt.Errorf("failed to get current spec: %w", err)
	}
	if !found {
		currentSpec = make(map[string]interface{})
	}
	
	desiredSpec, found, err := unstructured.NestedMap(desired.Object, "spec")
	if err != nil {
		return nil, fmt.Errorf("failed to get desired spec: %w", err)
	}
	if !found {
		desiredSpec = make(map[string]interface{})
	}
	
	// Simple comparison - in reality, this should use proper strategic merge logic
	currentSpecJSON, _ := json.Marshal(currentSpec)
	desiredSpecJSON, _ := json.Marshal(desiredSpec)
	
	if string(currentSpecJSON) == string(desiredSpecJSON) {
		// No changes needed
		return nil, nil
	}
	
	// Create patch with the desired spec
	patch := map[string]interface{}{
		"spec": desiredSpec,
	}
	
	return json.Marshal(patch)
}