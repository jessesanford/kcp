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
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"

	kcpkubeclientset "github.com/kcp-dev/client-go/kubernetes"
	"github.com/kcp-dev/logicalcluster/v3"
)

// StatusExtractor defines interface for extracting status from resources
type StatusExtractor interface {
	ExtractStatus(obj *unstructured.Unstructured) (interface{}, error)
	ShouldExtract(obj *unstructured.Unstructured) bool
}

// StatusSyncer syncs status from downstream clusters back to KCP
type StatusSyncer struct {
	// Clients
	kcpClient        kcpkubeclientset.ClusterInterface
	downstreamClient dynamic.Interface

	// Sync target configuration
	syncTargetName string
	workspace      logicalcluster.Name

	// Status extraction and aggregation
	extractors map[schema.GroupVersionResource]StatusExtractor
	aggregator *StatusAggregator
	transformer *StatusTransformer

	// Caching to prevent unnecessary updates
	statusCache map[string]interface{}
	mu          sync.RWMutex

	// Configuration
	namespacePrefix string
}

// NewStatusSyncer creates a new upstream status syncer
func NewStatusSyncer(
	kcpClient kcpkubeclientset.ClusterInterface,
	downstreamClient dynamic.Interface,
	syncTargetName string,
	workspace logicalcluster.Name,
) *StatusSyncer {
	return &StatusSyncer{
		kcpClient:        kcpClient,
		downstreamClient: downstreamClient,
		syncTargetName:   syncTargetName,
		workspace:        workspace,
		extractors:       make(map[schema.GroupVersionResource]StatusExtractor),
		aggregator:       NewStatusAggregator(),
		transformer:      NewStatusTransformer(workspace),
		statusCache:      make(map[string]interface{}),
		namespacePrefix:  "kcp-",
	}
}

// SyncStatusToKCP syncs resource status from downstream to KCP
func (s *StatusSyncer) SyncStatusToKCP(ctx context.Context, downstreamObj *unstructured.Unstructured) error {
	logger := klog.FromContext(ctx).WithValues(
		"resource", downstreamObj.GetName(),
		"namespace", downstreamObj.GetNamespace(),
		"kind", downstreamObj.GetKind(),
	)

	// Extract status from downstream object
	status, err := s.extractStatus(downstreamObj)
	if err != nil {
		return fmt.Errorf("failed to extract status: %w", err)
	}

	if status == nil {
		logger.V(4).Info("No status to sync")
		return nil
	}

	// Transform status for upstream
	transformedStatus, err := s.transformer.TransformForUpstream(status, downstreamObj)
	if err != nil {
		return fmt.Errorf("failed to transform status: %w", err)
	}

	// Determine target KCP resource
	gvr := s.getGVR(downstreamObj)
	namespace := s.reverseNamespaceTransform(downstreamObj.GetNamespace())
	name := downstreamObj.GetName()

	// Check if status has changed (use cache to avoid unnecessary updates)
	cacheKey := fmt.Sprintf("%s/%s/%s", gvr.String(), namespace, name)
	if s.isStatusUnchanged(cacheKey, transformedStatus) {
		logger.V(5).Info("Status unchanged, skipping update")
		return nil
	}

	// Patch status to KCP (simplified version using dynamic client)
	if err := s.patchStatus(ctx, gvr, namespace, name, transformedStatus); err != nil {
		return fmt.Errorf("failed to patch status: %w", err)
	}

	// Update cache
	s.updateStatusCache(cacheKey, transformedStatus)

	logger.V(4).Info("Successfully synced status to KCP")
	return nil
}

// extractStatus extracts status from a downstream resource using configured extractors
func (s *StatusSyncer) extractStatus(obj *unstructured.Unstructured) (interface{}, error) {
	gvr := s.getGVR(obj)

	// Use custom extractor if available
	if extractor, exists := s.extractors[gvr]; exists && extractor.ShouldExtract(obj) {
		return extractor.ExtractStatus(obj)
	}

	// Use default extraction
	return s.defaultExtractStatus(obj)
}

// defaultExtractStatus provides default status extraction logic
func (s *StatusSyncer) defaultExtractStatus(obj *unstructured.Unstructured) (interface{}, error) {
	status, found, err := unstructured.NestedFieldNoCopy(obj.Object, "status")
	if err != nil {
		return nil, fmt.Errorf("failed to get status field: %w", err)
	}
	if !found {
		return nil, nil
	}

	return status, nil
}

// patchStatus patches the status to the KCP resource using dynamic client
func (s *StatusSyncer) patchStatus(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string, status interface{}) error {
	patch := map[string]interface{}{
		"status": status,
	}

	patchData, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("failed to marshal patch: %w", err)
	}

	// Use dynamic client for patching
	dynamicClient := s.kcpClient.Cluster(s.workspace).DynamicInterface()
	
	// Patch status subresource
	_, err = dynamicClient.
		Resource(gvr).
		Namespace(namespace).
		Patch(ctx, name, types.MergePatchType, patchData, metav1.PatchOptions{}, "status")

	if err != nil {
		if errors.IsNotFound(err) {
			klog.FromContext(ctx).V(4).Info("KCP resource not found, skipping status sync", "gvr", gvr, "namespace", namespace, "name", name)
			return nil
		}
		return fmt.Errorf("failed to patch status: %w", err)
	}

	return nil
}

// getGVR extracts GroupVersionResource from an unstructured object
func (s *StatusSyncer) getGVR(obj *unstructured.Unstructured) schema.GroupVersionResource {
	gv, err := schema.ParseGroupVersion(obj.GetAPIVersion())
	if err != nil {
		// Fallback to basic parsing
		parts := strings.Split(obj.GetAPIVersion(), "/")
		if len(parts) == 2 {
			gv = schema.GroupVersion{Group: parts[0], Version: parts[1]}
		} else {
			gv = schema.GroupVersion{Group: "", Version: parts[0]}
		}
	}

	// Convert kind to resource (basic pluralization)
	resource := strings.ToLower(obj.GetKind() + "s")

	return schema.GroupVersionResource{
		Group:    gv.Group,
		Version:  gv.Version,
		Resource: resource,
	}
}

// reverseNamespaceTransform converts downstream namespace back to KCP namespace
func (s *StatusSyncer) reverseNamespaceTransform(downstreamNamespace string) string {
	if downstreamNamespace == "" {
		return ""
	}

	// Remove workspace-specific prefix
	prefix := fmt.Sprintf("%s%s-", s.namespacePrefix, s.workspace)
	if strings.HasPrefix(downstreamNamespace, prefix) {
		return strings.TrimPrefix(downstreamNamespace, prefix)
	}

	return downstreamNamespace
}

// isStatusUnchanged checks if status has changed using cache
func (s *StatusSyncer) isStatusUnchanged(cacheKey string, newStatus interface{}) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cachedStatus, exists := s.statusCache[cacheKey]
	if !exists {
		return false
	}

	// Simple comparison - in production, might want more sophisticated diffing
	cachedJSON, err1 := json.Marshal(cachedStatus)
	newJSON, err2 := json.Marshal(newStatus)

	if err1 != nil || err2 != nil {
		return false
	}

	return string(cachedJSON) == string(newJSON)
}

// updateStatusCache updates the status cache
func (s *StatusSyncer) updateStatusCache(cacheKey string, status interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.statusCache[cacheKey] = status
}

// RegisterExtractor registers a custom status extractor for a specific resource type
func (s *StatusSyncer) RegisterExtractor(gvr schema.GroupVersionResource, extractor StatusExtractor) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.extractors[gvr] = extractor
}

// ClearCache clears the status cache (useful for testing)
func (s *StatusSyncer) ClearCache() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.statusCache = make(map[string]interface{})
}

// GetCacheSize returns the current cache size (for monitoring)
func (s *StatusSyncer) GetCacheSize() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.statusCache)
}