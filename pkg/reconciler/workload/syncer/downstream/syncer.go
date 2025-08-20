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
	"context"
	"crypto/sha256"
	"fmt"
	"sync"
	"time"

	"github.com/kcp-dev/logicalcluster/v3"
	kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"
)

// SyncTarget represents a sync target resource (placeholder for now)
type SyncTarget struct {
	metav1.ObjectMeta
	Spec SyncTargetSpec
}

// SyncTargetSpec defines the desired state of a sync target
type SyncTargetSpec struct {
	ClusterName string
	Namespace   string
}

// Transformer interface for resource transformations
type Transformer interface {
	TransformForDownstream(ctx context.Context, obj runtime.Object, target *SyncTarget) (runtime.Object, error)
}

// Pipeline implements the Transformer interface
type Pipeline struct {
	workspace logicalcluster.Name
}

// NewPipeline creates a new transformation pipeline
func NewPipeline(workspace logicalcluster.Name) *Pipeline {
	return &Pipeline{workspace: workspace}
}

// TransformForDownstream transforms an object for downstream deployment
func (p *Pipeline) TransformForDownstream(ctx context.Context, obj runtime.Object, target *SyncTarget) (runtime.Object, error) {
	// Basic transformation - just pass through for now
	return obj, nil
}

// Syncer handles downstream synchronization operations from KCP to physical clusters
type Syncer struct {
	// Workspace isolation
	workspace logicalcluster.Name
	
	// Clients
	kcpClient        kcpclientset.ClusterInterface
	downstreamClient dynamic.Interface
	syncTarget       *SyncTarget
	
	// Transformation pipeline for resource modification
	transformer *Pipeline
	
	
	// Configuration
	config *DownstreamConfig
	
	// State tracking
	stateCache map[string]*ResourceState
	mu         sync.RWMutex
}

// NewSyncer creates a new downstream syncer instance
func NewSyncer(
	workspace logicalcluster.Name,
	kcpClient kcpclientset.ClusterInterface,
	downstreamClient dynamic.Interface,
	syncTarget *SyncTarget,
	config *DownstreamConfig,
) (*Syncer, error) {
	if config == nil {
		config = DefaultDownstreamConfig()
	}

	syncer := &Syncer{
		workspace:        workspace,
		kcpClient:        kcpClient,
		downstreamClient: downstreamClient,
		syncTarget:       syncTarget,
		transformer:      NewPipeline(workspace),
		config:           config,
		stateCache:       make(map[string]*ResourceState),
	}

	return syncer, nil
}

// ApplyToDownstream synchronizes a resource from KCP to the downstream cluster
func (s *Syncer) ApplyToDownstream(ctx context.Context, gvr schema.GroupVersionResource, obj *unstructured.Unstructured) (*SyncResult, error) {
	logger := klog.FromContext(ctx).WithValues(
		"operation", "ApplyToDownstream",
		"gvr", gvr,
		"namespace", obj.GetNamespace(),
		"name", obj.GetName(),
		"workspace", s.workspace,
	)

	result := &SyncResult{
		Operation: "noop",
		Success:   false,
	}

	// Generate unique key for state tracking
	stateKey := s.generateStateKey(gvr, obj.GetNamespace(), obj.GetName())

	// Transform the object for downstream deployment
	transformedObj, err := s.transformForDownstream(ctx, obj)
	if err != nil {
		result.Error = fmt.Errorf("failed to transform object for downstream: %w", err)
		return result, err
	}

	// Check if resource exists downstream
	downstreamResource := s.downstreamClient.Resource(gvr)
	var downstreamClient dynamic.ResourceInterface = downstreamResource
	if transformedObj.GetNamespace() != "" {
		downstreamClient = downstreamResource.Namespace(transformedObj.GetNamespace())
	}

	existingObj, err := downstreamClient.Get(ctx, transformedObj.GetName(), metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		result.Error = fmt.Errorf("failed to get existing object from downstream: %w", err)
		return result, err
	}

	if apierrors.IsNotFound(err) {
		// Object doesn't exist downstream, create it
		result.Operation = "create"
		logger.V(2).Info("Creating new object in downstream")

		createdObj, err := downstreamClient.Create(ctx, transformedObj, metav1.CreateOptions{})
		if err != nil {
			result.Error = fmt.Errorf("failed to create object in downstream: %w", err)
			return result, err
		}

		// Update state cache
		s.updateStateCache(stateKey, gvr, createdObj)
		result.Success = true
		logger.V(2).Info("Successfully created object in downstream")

	} else {
		// Object exists, perform update (basic change detection)
		result.Operation = "update"
		logger.V(2).Info("Updating existing object in downstream")

		// Preserve downstream fields before updating
		mergedObj := PreserveDownstreamFields(existingObj, transformedObj)

		// Attempt update with conflict resolution
		updatedObj, updateResult, err := s.updateWithConflictResolution(ctx, downstreamClient, mergedObj, existingObj)
		if err != nil {
			result.Error = err
			result.Conflicts = updateResult.Conflicts
			result.RetryAfter = updateResult.RetryAfter
			return result, err
		}

		// Update state cache
		s.updateStateCache(stateKey, gvr, updatedObj)
		result.Success = true
		result.Conflicts = updateResult.Conflicts
		logger.V(2).Info("Successfully updated object in downstream")
	}

	return result, nil
}

// DeleteFromDownstream removes a resource from the downstream cluster
func (s *Syncer) DeleteFromDownstream(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string) (*SyncResult, error) {
	logger := klog.FromContext(ctx).WithValues(
		"operation", "DeleteFromDownstream",
		"gvr", gvr,
		"namespace", namespace,
		"name", name,
		"workspace", s.workspace,
	)

	result := &SyncResult{
		Operation: "delete",
		Success:   false,
	}

	// Generate unique key for state tracking
	stateKey := s.generateStateKey(gvr, namespace, name)

	// Get downstream client
	downstreamResource := s.downstreamClient.Resource(gvr)
	var downstreamClient dynamic.ResourceInterface = downstreamResource
	if namespace != "" {
		downstreamClient = downstreamResource.Namespace(namespace)
	}

	// Check if object exists
	_, err := downstreamClient.Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		// Object already doesn't exist, clean up state cache
		s.removeFromStateCache(stateKey)
		result.Success = true
		logger.V(4).Info("Object already deleted from downstream")
		return result, nil
	}
	if err != nil {
		result.Error = fmt.Errorf("failed to check object existence in downstream: %w", err)
		return result, err
	}

	// Skip finalizer handling for simplicity

	// Delete the object
	logger.V(2).Info("Deleting object from downstream")
	err = downstreamClient.Delete(ctx, name, metav1.DeleteOptions{
		PropagationPolicy: &s.config.DeletionPropagation,
	})
	if err != nil && !apierrors.IsNotFound(err) {
		result.Error = fmt.Errorf("failed to delete object from downstream: %w", err)
		return result, err
	}

	// Clean up state cache
	s.removeFromStateCache(stateKey)
	result.Success = true
	logger.V(2).Info("Successfully deleted object from downstream")

	return result, nil
}

// updateWithConflictResolution attempts to update an object with conflict resolution
func (s *Syncer) updateWithConflictResolution(
	ctx context.Context,
	client dynamic.ResourceInterface,
	desired, existing *unstructured.Unstructured,
) (*unstructured.Unstructured, *SyncResult, error) {
	result := &SyncResult{
		Operation: "update",
		Success:   false,
		Conflicts: []string{},
	}

	for attempt := 0; attempt <= s.config.ConflictRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(s.config.ConflictRetryDelay)
			// Refresh and re-merge
			var err error
			existing, err = client.Get(ctx, existing.GetName(), metav1.GetOptions{})
			if err != nil {
				return nil, result, err
			}
			desired = PreserveDownstreamFields(existing, desired)
		}

		updatedObj, err := client.Update(ctx, desired, metav1.UpdateOptions{})
		if err == nil {
			result.Success = true
			return updatedObj, result, nil
		}

		if !apierrors.IsConflict(err) {
			return nil, result, err
		}

		result.Conflicts = append(result.Conflicts, "resource version conflict")
	}

	return nil, result, fmt.Errorf("failed to update after %d attempts", s.config.ConflictRetries)
}

// transformForDownstream applies transformations for downstream deployment
func (s *Syncer) transformForDownstream(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	// Use local SyncTarget
	target := s.syncTarget

	transformedObj, err := s.transformer.TransformForDownstream(ctx, obj, target)
	if err != nil {
		return nil, err
	}

	result, ok := transformedObj.(*unstructured.Unstructured)
	if !ok {
		return nil, fmt.Errorf("transformation result is not an unstructured object")
	}

	return result, nil
}

// generateStateKey creates a unique key for state cache
func (s *Syncer) generateStateKey(gvr schema.GroupVersionResource, namespace, name string) string {
	key := fmt.Sprintf("%s/%s/%s", gvr.String(), namespace, name)
	return key
}

// updateStateCache updates the state cache for a resource
func (s *Syncer) updateStateCache(key string, gvr schema.GroupVersionResource, obj *unstructured.Unstructured) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Generate hash for change detection
	hash := s.generateObjectHash(obj)

	state := &ResourceState{
		GVR:             gvr,
		Namespace:       obj.GetNamespace(),
		Name:            obj.GetName(),
		ResourceVersion: obj.GetResourceVersion(),
		Generation:      obj.GetGeneration(),
		LastSyncTime:    metav1.Now(),
		Hash:            hash,
	}

	s.stateCache[key] = state
}

// removeFromStateCache removes a resource from the state cache
func (s *Syncer) removeFromStateCache(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.stateCache, key)
}

// generateObjectHash creates a hash of the object for change detection
func (s *Syncer) generateObjectHash(obj *unstructured.Unstructured) string {
	// Create a copy without fields we want to ignore for hash calculation
	hashObj := obj.DeepCopy()
	
	// Remove fields that change frequently but aren't meaningful for sync
	unstructured.RemoveNestedField(hashObj.Object, "metadata", "resourceVersion")
	unstructured.RemoveNestedField(hashObj.Object, "metadata", "generation")
	unstructured.RemoveNestedField(hashObj.Object, "metadata", "managedFields")
	unstructured.RemoveNestedField(hashObj.Object, "status")

	// Convert to string and hash
	data := fmt.Sprintf("%v", hashObj.Object)
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash)
}

