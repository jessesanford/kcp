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

package syncer

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/klog/v2"

	workloadv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"
)

// SyncTargetStorage implements REST storage for SyncTarget resources accessed by syncers.
// It handles transforming resources between internal KCP format and the format expected by syncers.
type SyncTargetStorage struct {
	authConfig *AuthConfig
}

// NewSyncTargetStorage creates a new storage instance for SyncTarget resources.
func NewSyncTargetStorage(authConfig *AuthConfig) *SyncTargetStorage {
	return &SyncTargetStorage{
		authConfig: authConfig,
	}
}

// New returns a new empty SyncTarget instance.
func (s *SyncTargetStorage) New() runtime.Object {
	return &workloadv1alpha1.SyncTarget{}
}

// Destroy cleans up any resources held by the storage.
func (s *SyncTargetStorage) Destroy() {}

// Get retrieves a specific SyncTarget resource for the syncer.
func (s *SyncTargetStorage) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	syncerID, workspace, ok := extractSyncerIdentity(ctx)
	if !ok {
		return nil, fmt.Errorf("no syncer identity in context")
	}

	klog.V(4).InfoS("Getting SyncTarget for syncer", "syncerID", syncerID, "workspace", workspace, "name", name)

	// Get the SyncTarget from the backend
	syncTarget, err := s.authConfig.GetSyncTargetForSyncer(syncerID, workspace)
	if err != nil {
		return nil, fmt.Errorf("failed to get sync target: %w", err)
	}

	if syncTarget == nil {
		return nil, fmt.Errorf("sync target not found")
	}

	// Transform the SyncTarget for syncer consumption
	transformed := s.transformForSyncer(syncTarget, syncerID)
	
	return transformed, nil
}

// List retrieves all SyncTarget resources accessible to the syncer.
func (s *SyncTargetStorage) List(ctx context.Context, options *metav1.ListOptions) (runtime.Object, error) {
	syncerID, workspace, ok := extractSyncerIdentity(ctx)
	if !ok {
		return nil, fmt.Errorf("no syncer identity in context")
	}

	klog.V(4).InfoS("Listing SyncTargets for syncer", "syncerID", syncerID, "workspace", workspace)

	// For now, a syncer can only access its own SyncTarget
	syncTarget, err := s.authConfig.GetSyncTargetForSyncer(syncerID, workspace)
	if err != nil {
		return nil, fmt.Errorf("failed to get sync target: %w", err)
	}

	list := &workloadv1alpha1.SyncTargetList{
		TypeMeta: metav1.TypeMeta{
			APIVersion: workloadv1alpha1.SchemeGroupVersion.String(),
			Kind:       "SyncTargetList",
		},
	}

	if syncTarget != nil {
		transformed := s.transformForSyncer(syncTarget, syncerID)
		list.Items = append(list.Items, *transformed)
	}

	return list, nil
}

// Watch provides a watch stream for SyncTarget resources accessible to the syncer.
func (s *SyncTargetStorage) Watch(ctx context.Context, options *metav1.ListOptions) (watch.Interface, error) {
	syncerID, workspace, ok := extractSyncerIdentity(ctx)
	if !ok {
		return nil, fmt.Errorf("no syncer identity in context")
	}

	klog.V(4).InfoS("Starting watch for SyncTargets", "syncerID", syncerID, "workspace", workspace)

	// Create a basic watch interface that can be extended later
	// For now, return a simple watcher that closes immediately
	watcher := watch.NewEmptyWatch()
	
	// In a real implementation, this would connect to the underlying
	// KCP watch system and filter/transform events for the syncer
	
	return watcher, nil
}

// transformForSyncer transforms a SyncTarget from internal KCP format to syncer format.
// This removes internal annotations and adds syncer-specific metadata.
func (s *SyncTargetStorage) transformForSyncer(syncTarget *workloadv1alpha1.SyncTarget, syncerID string) *workloadv1alpha1.SyncTarget {
	transformed := syncTarget.DeepCopy()
	
	// Remove internal KCP annotations that syncers shouldn't see
	if transformed.Annotations != nil {
		for key := range transformed.Annotations {
			if isInternalAnnotation(key) {
				delete(transformed.Annotations, key)
			}
		}
	}
	
	// Add syncer-specific metadata
	if transformed.Annotations == nil {
		transformed.Annotations = make(map[string]string)
	}
	
	transformed.Annotations["syncer.workload.kcp.io/syncer-id"] = syncerID
	
	// Remove sensitive information from status
	if len(transformed.Status.Conditions) > 0 {
		// Filter out internal conditions that syncers don't need
		var filteredConditions []metav1.Condition
		for _, condition := range transformed.Status.Conditions {
			if !isInternalCondition(string(condition.Type)) {
				filteredConditions = append(filteredConditions, metav1.Condition{
					Type:               string(condition.Type),
					Status:             metav1.ConditionStatus(condition.Status),
					LastTransitionTime: condition.LastTransitionTime,
					Reason:             condition.Reason,
					Message:            condition.Message,
				})
			}
		}
		// Note: In a real implementation, we'd need to properly handle the condition conversion
		// For now, we'll keep the original conditions
	}
	
	return transformed
}

// isInternalAnnotation determines if an annotation is internal to KCP and should be hidden from syncers.
func isInternalAnnotation(key string) bool {
	internalPrefixes := []string{
		"internal.workload.kcp.io/",
		"core.kcp.io/",
		"tenancy.kcp.io/",
	}
	
	for _, prefix := range internalPrefixes {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			return true
		}
	}
	
	return false
}

// isInternalCondition determines if a condition is internal to KCP and should be hidden from syncers.
func isInternalCondition(conditionType string) bool {
	// For now, all conditions are visible to syncers
	// This can be extended based on specific requirements
	return false
}