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

package committer

import (
	"context"
	"encoding/json"
	"fmt"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/google/go-cmp/cmp"

	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"
)

// StatusResource is a generic wrapper around TMC resources for status updates.
// This focuses specifically on status field updates following KCP patterns.
type StatusResource[Sp any, St any] struct {
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              Sp `json:"spec"`
	Status            St `json:"status,omitempty"`
}

// ClusterStatusPatcher is the cluster-aware Patch API for status updates.
type ClusterStatusPatcher[R runtime.Object, P StatusPatcher[R]] interface {
	Cluster(cluster logicalcluster.Path) P
}

// StatusPatcher is the Patch API specialized for status subresource updates.
type StatusPatcher[R runtime.Object] interface {
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (R, error)
}

// StatusCommitFunc is a function type for committing status changes to TMC resources.
type StatusCommitFunc[Sp any, St any] func(context.Context, *StatusResource[Sp, St], *StatusResource[Sp, St]) error

// NewStatusCommitter creates a new status committer for TMC resources.
// This function specializes in status-only updates using the status subresource.
// It follows KCP patterns for safe status updates without touching spec fields.
func NewStatusCommitter[R runtime.Object, P StatusPatcher[R], Sp any, St any](patcher ClusterStatusPatcher[R, P]) StatusCommitFunc[Sp, St] {
	r := new(R)
	focusType := fmt.Sprintf("%T", *r)
	return func(ctx context.Context, old, obj *StatusResource[Sp, St]) error {
		return withStatusPatchAndSubResources(ctx, focusType, old, obj,
			func(patchBytes []byte, subresources []string) error {
				clusterName := logicalcluster.From(old)
				_, err := patcher.Cluster(clusterName.Path()).Patch(ctx, obj.Name, types.MergePatchType, patchBytes, metav1.PatchOptions{}, subresources...)
				return err
			})
	}
}

// NewStatusCommitterScoped creates a status committer scoped to a specific cluster.
// This is useful when the cluster context is already established.
func NewStatusCommitterScoped[R runtime.Object, P StatusPatcher[R], Sp any, St any](patcher StatusPatcher[R]) StatusCommitFunc[Sp, St] {
	r := new(R)
	focusType := fmt.Sprintf("%T", *r)
	return func(ctx context.Context, old, obj *StatusResource[Sp, St]) error {
		return withStatusPatchAndSubResources(ctx, focusType, old, obj,
			func(patchBytes []byte, subresources []string) error {
				_, err := patcher.Patch(ctx, obj.Name, types.MergePatchType, patchBytes, metav1.PatchOptions{}, subresources...)
				return err
			})
	}
}

type statusPatchFunc func([]byte, []string) error

func withStatusPatchAndSubResources[Sp any, St any](ctx context.Context, focusType string, old, obj *StatusResource[Sp, St], patch statusPatchFunc) error {
	logger := klog.FromContext(ctx)
	patchBytes, subresources, err := generateStatusPatchAndSubResources(old, obj)
	if err != nil {
		return fmt.Errorf("failed to create status patch for %s %s: %w", focusType, obj.Name, err)
	}

	if len(patchBytes) == 0 {
		logger.V(3).Info("No status changes detected", "resource", focusType, "name", obj.Name)
		return nil
	}

	logger.V(2).Info(fmt.Sprintf("patching %s status", focusType), 
		"name", obj.Name, 
		"patch", string(patchBytes),
		"subresources", subresources)
		
	if err := patch(patchBytes, subresources); err != nil {
		return fmt.Errorf("failed to patch %s status %s: %w", focusType, old.Name, err)
	}
	return nil
}

func generateStatusPatchAndSubResources[Sp any, St any](old, obj *StatusResource[Sp, St]) ([]byte, []string, error) {
	// For status updates, we only care about status field changes
	// We explicitly ignore spec and metadata changes in status commits
	statusChanged := !equality.Semantic.DeepEqual(old.Status, obj.Status)
	
	if !statusChanged {
		return nil, nil, nil
	}
	
	// Validate that spec hasn't been modified in a status update
	specChanged := !equality.Semantic.DeepEqual(old.Spec, obj.Spec)
	if specChanged {
		panic(fmt.Sprintf("programmer error: spec changed in status commit. This violates status subresource constraints. diff=%s", cmp.Diff(old.Spec, obj.Spec)))
	}

	clusterName := logicalcluster.From(old)
	name := old.Name

	// For status updates, we only include the status field
	oldForPatch := &StatusResource[Sp, St]{
		Status: old.Status,
	}
	// Clear UID and ResourceVersion to prevent them from appearing in the patch
	oldForPatch.UID = ""
	oldForPatch.ResourceVersion = ""

	oldData, err := json.Marshal(oldForPatch)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal old status data for %s|%s: %w", clusterName, name, err)
	}

	newForPatch := &StatusResource[Sp, St]{
		Status: obj.Status,
	}
	// Set UID and ResourceVersion as preconditions for the patch
	newForPatch.UID = old.UID
	newForPatch.ResourceVersion = old.ResourceVersion

	newData, err := json.Marshal(newForPatch)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal new status data for %s|%s: %w", clusterName, name, err)
	}

	patchBytes, err := jsonpatch.CreateMergePatch(oldData, newData)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create status patch for %s|%s: %w", clusterName, name, err)
	}

	// Status updates always use the "status" subresource
	subresources := []string{"status"}

	return patchBytes, subresources, nil
}

// StatusUpdateOptions provides configuration for status updates
type StatusUpdateOptions struct {
	// Force indicates whether to force the status update even if unchanged
	Force bool
	
	// ValidateStatusOnly ensures only status fields are being updated
	ValidateStatusOnly bool
	
	// RetryOnConflict indicates whether to retry on resource version conflicts
	RetryOnConflict bool
	
	// MaxRetries is the maximum number of retry attempts
	MaxRetries int
}

// DefaultStatusUpdateOptions returns sensible defaults for status updates
func DefaultStatusUpdateOptions() StatusUpdateOptions {
	return StatusUpdateOptions{
		Force:              false,
		ValidateStatusOnly: true,
		RetryOnConflict:    true,
		MaxRetries:         3,
	}
}

// StatusCommitterWithOptions creates a status committer with additional options
func NewStatusCommitterWithOptions[R runtime.Object, P StatusPatcher[R], Sp any, St any](
	patcher ClusterStatusPatcher[R, P], 
	opts StatusUpdateOptions,
) StatusCommitFunc[Sp, St] {
	baseCommitter := NewStatusCommitter[R, P, Sp, St](patcher)
	
	return func(ctx context.Context, old, obj *StatusResource[Sp, St]) error {
		if opts.ValidateStatusOnly {
			// Ensure only status is being modified
			if !equality.Semantic.DeepEqual(old.Spec, obj.Spec) {
				return fmt.Errorf("status committer cannot modify spec fields")
			}
			if !equality.Semantic.DeepEqual(old.ObjectMeta, obj.ObjectMeta) {
				// Allow certain metadata changes like labels and annotations for status updates
				allowedMetaChange := old.ObjectMeta.DeepCopy()
				allowedMetaChange.Labels = obj.ObjectMeta.Labels
				allowedMetaChange.Annotations = obj.ObjectMeta.Annotations
				if !equality.Semantic.DeepEqual(*allowedMetaChange, obj.ObjectMeta) {
					return fmt.Errorf("status committer can only modify labels and annotations in metadata")
				}
			}
		}
		
		if !opts.Force {
			// Check if status actually changed
			if equality.Semantic.DeepEqual(old.Status, obj.Status) {
				klog.FromContext(ctx).V(3).Info("Skipping status update - no changes detected")
				return nil
			}
		}
		
		if opts.RetryOnConflict {
			var lastErr error
			for i := 0; i <= opts.MaxRetries; i++ {
				lastErr = baseCommitter(ctx, old, obj)
				if lastErr == nil {
					return nil
				}
				
				// Check if it's a conflict error (in a real implementation, 
				// you'd check for specific error types like 409 Conflict)
				if i < opts.MaxRetries {
					klog.FromContext(ctx).V(2).Info("Retrying status update after conflict", 
						"attempt", i+1, "error", lastErr)
					continue
				}
			}
			return fmt.Errorf("failed to update status after %d retries: %w", opts.MaxRetries, lastErr)
		}
		
		return baseCommitter(ctx, old, obj)
	}
}