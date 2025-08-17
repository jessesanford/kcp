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
	"fmt"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"
)

// Applier handles resource application to downstream clusters with retry logic and multiple strategies.
type Applier struct {
	// client is the dynamic client for the downstream cluster
	client dynamic.Interface
	// retryStrategy defines how to retry failed operations
	retryStrategy *RetryStrategy
	// applyStrategy defines which apply strategy to use
	applyStrategy ApplyStrategy
	// fieldManager is used for server-side apply operations
	fieldManager string
	// forceConflicts determines whether to force conflicts in server-side apply
	forceConflicts bool
}

// NewApplier creates a new resource applier with default settings.
func NewApplier(client dynamic.Interface, fieldManager string) *Applier {
	return &Applier{
		client:         client,
		retryStrategy:  NewDefaultRetryStrategy(),
		applyStrategy:  ServerSideApply,
		fieldManager:   fieldManager,
		forceConflicts: false,
	}
}

// WithRetryStrategy configures the retry strategy for the applier.
func (a *Applier) WithRetryStrategy(strategy *RetryStrategy) *Applier {
	a.retryStrategy = strategy
	return a
}

// WithApplyStrategy configures the apply strategy for the applier.
func (a *Applier) WithApplyStrategy(strategy ApplyStrategy) *Applier {
	a.applyStrategy = strategy
	return a
}

// WithForceConflicts configures whether to force conflicts in server-side apply.
func (a *Applier) WithForceConflicts(force bool) *Applier {
	a.forceConflicts = force
	return a
}

// Apply creates or updates a resource with retry logic.
func (a *Applier) Apply(ctx context.Context, obj *unstructured.Unstructured) (*ApplyResult, error) {
	logger := klog.FromContext(ctx)
	gvr := a.getGVR(obj)
	start := time.Now()
	
	result := &ApplyResult{
		GVR:       gvr,
		Namespace: obj.GetNamespace(),
		Name:      obj.GetName(),
	}
	
	var attempts int
	err := a.retryStrategy.Execute(ctx, func() error {
		attempts++
		switch a.applyStrategy {
		case ServerSideApply:
			return a.serverSideApply(ctx, gvr, obj, result)
		case StrategicMerge:
			return a.strategicMerge(ctx, gvr, obj, result)
		case Replace:
			return a.replace(ctx, gvr, obj, result)
		default:
			return fmt.Errorf("unknown apply strategy: %v", a.applyStrategy)
		}
	})
	
	result.Duration = time.Since(start)
	result.Attempts = attempts
	
	if err != nil {
		result.Success = false
		result.Error = err
		logger.Error(err, "Failed to apply resource after retries",
			"gvr", gvr,
			"name", obj.GetName(),
			"namespace", obj.GetNamespace(),
			"attempts", attempts)
	} else {
		result.Success = true
		logger.V(4).Info("Successfully applied resource",
			"gvr", gvr,
			"name", obj.GetName(),
			"namespace", obj.GetNamespace(),
			"operation", result.Operation,
			"attempts", attempts,
			"duration", result.Duration)
	}
	
	return result, err
}

// Delete removes a resource with configurable propagation policy.
func (a *Applier) Delete(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string, options metav1.DeleteOptions) error {
	logger := klog.FromContext(ctx)
	
	return a.retryStrategy.Execute(ctx, func() error {
		err := a.client.Resource(gvr).Namespace(namespace).Delete(ctx, name, options)
		if err != nil {
			if errors.IsNotFound(err) {
				logger.V(4).Info("Resource already deleted or not found",
					"gvr", gvr,
					"name", name,
					"namespace", namespace)
				return nil // Not an error if already deleted
			}
			return fmt.Errorf("delete operation failed: %w", err)
		}
		
		logger.V(4).Info("Successfully deleted resource",
			"gvr", gvr,
			"name", name,
			"namespace", namespace)
		return nil
	})
}

// Patch applies a patch to a resource.
func (a *Applier) Patch(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string, patchType types.PatchType, data []byte) (*unstructured.Unstructured, error) {
	logger := klog.FromContext(ctx)
	
	var result *unstructured.Unstructured
	err := a.retryStrategy.Execute(ctx, func() error {
		patched, err := a.client.Resource(gvr).Namespace(namespace).Patch(ctx, name, patchType, data, metav1.PatchOptions{})
		if err != nil {
			return fmt.Errorf("patch operation failed: %w", err)
		}
		result = patched
		return nil
	})
	
	if err != nil {
		logger.Error(err, "Failed to patch resource",
			"gvr", gvr,
			"name", name,
			"namespace", namespace)
		return nil, err
	}
	
	logger.V(4).Info("Successfully patched resource",
		"gvr", gvr,
		"name", name,
		"namespace", namespace)
	
	return result, nil
}

// ApplyBatch applies multiple resources concurrently with limited parallelism.
func (a *Applier) ApplyBatch(ctx context.Context, objects []*unstructured.Unstructured, maxConcurrency int) *BatchResult {
	if len(objects) == 0 {
		return &BatchResult{}
	}
	
	start := time.Now()
	
	// Limit concurrency to prevent overwhelming the API server
	if maxConcurrency <= 0 {
		maxConcurrency = 10
	}
	semaphore := make(chan struct{}, maxConcurrency)
	
	var wg sync.WaitGroup
	results := make(chan *ApplyResult, len(objects))
	
	// Apply objects concurrently
	for _, obj := range objects {
		wg.Add(1)
		go func(obj *unstructured.Unstructured) {
			defer wg.Done()
			
			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			
			result, _ := a.Apply(ctx, obj)
			results <- result
		}(obj)
	}
	
	// Wait for all operations to complete
	go func() {
		wg.Wait()
		close(results)
	}()
	
	// Collect results
	batchResult := &BatchResult{
		Total:    len(objects),
		Duration: time.Since(start),
	}
	
	for result := range results {
		batchResult.Results = append(batchResult.Results, result)
		if result.Success {
			batchResult.Succeeded++
		} else {
			batchResult.Failed++
		}
	}
	
	return batchResult
}

// getGVR extracts the GroupVersionResource from an unstructured object.
func (a *Applier) getGVR(obj *unstructured.Unstructured) schema.GroupVersionResource {
	gvk := obj.GroupVersionKind()
	
	// Convert Kind to Resource (simple pluralization)
	resource := a.kindToResource(gvk.Kind)
	
	return gvk.GroupVersion().WithResource(resource)
}

// kindToResource converts a Kind to a resource name with simple pluralization.
// In a full implementation, this should use discovery to get accurate resource names.
func (a *Applier) kindToResource(kind string) string {
	// Simple pluralization rules - in reality, use discovery client
	switch kind {
	case "":
		return ""
	default:
		// Basic pluralization
		if kind[len(kind)-1] == 'y' {
			return kind[:len(kind)-1] + "ies"
		} else if kind[len(kind)-1] == 's' {
			return kind + "es"
		} else {
			return kind + "s"
		}
	}
}

// DeleteOptions creates common delete options for resource deletion.
func DeleteOptions() metav1.DeleteOptions {
	return metav1.DeleteOptions{
		PropagationPolicy: &[]metav1.DeletionPropagation{metav1.DeletePropagationBackground}[0],
	}
}

// CascadeDeleteOptions creates delete options that cascade deletion to dependent resources.
func CascadeDeleteOptions() metav1.DeleteOptions {
	return metav1.DeleteOptions{
		PropagationPolicy: &[]metav1.DeletionPropagation{metav1.DeletePropagationForeground}[0],
	}
}

// OrphanDeleteOptions creates delete options that orphan dependent resources.
func OrphanDeleteOptions() metav1.DeleteOptions {
	return metav1.DeleteOptions{
		PropagationPolicy: &[]metav1.DeletionPropagation{metav1.DeletePropagationOrphan}[0],
	}
}