package conflict

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"
)

// ResolutionStrategyInterface defines the interface for conflict resolution strategies
type ResolutionStrategyInterface interface {
	Resolve(ctx context.Context, kcp, downstream *unstructured.Unstructured, conflict *Conflict) (*ResolutionResult, error)
}

// KCPWinsStrategy resolves conflicts by preferring KCP resource version
type KCPWinsStrategy struct{}

func (s *KCPWinsStrategy) Resolve(ctx context.Context, kcp, downstream *unstructured.Unstructured, conflict *Conflict) (*ResolutionResult, error) {
	klog.FromContext(ctx).V(3).Info("Applying KCP wins strategy", "resource", fmt.Sprintf("%s/%s", conflict.Namespace, conflict.Name))

	if kcp == nil {
		return nil, fmt.Errorf("KCP resource is nil, cannot apply KCP wins strategy")
	}

	merged := kcp.DeepCopy()
	if downstream != nil {
		preserveDownstreamOnlyFields(downstream, merged)
	}

	return &ResolutionResult{
		Resolved: true,
		Merged:   merged,
	}, nil
}

// DownstreamWinsStrategy resolves conflicts by preferring downstream resource version
type DownstreamWinsStrategy struct{}

func (s *DownstreamWinsStrategy) Resolve(ctx context.Context, kcp, downstream *unstructured.Unstructured, conflict *Conflict) (*ResolutionResult, error) {
	klog.FromContext(ctx).V(3).Info("Applying downstream wins strategy", "resource", fmt.Sprintf("%s/%s", conflict.Namespace, conflict.Name))

	if downstream == nil {
		return nil, fmt.Errorf("downstream resource is nil, cannot apply downstream wins strategy")
	}

	merged := downstream.DeepCopy()
	markForUpstreamSync(merged)

	return &ResolutionResult{
		Resolved: true,
		Merged:   merged,
	}, nil
}

// MergeStrategy attempts to merge both versions intelligently
type MergeStrategy struct{}

func (s *MergeStrategy) Resolve(ctx context.Context, kcp, downstream *unstructured.Unstructured, conflict *Conflict) (*ResolutionResult, error) {
	klog.FromContext(ctx).V(3).Info("Applying merge strategy", "resource", fmt.Sprintf("%s/%s", conflict.Namespace, conflict.Name))

	if kcp == nil || downstream == nil {
		return nil, fmt.Errorf("both KCP and downstream resources required for merge strategy")
	}

	merged := kcp.DeepCopy()
	var unresolvedConflicts []FieldConflict

	// Attempt to merge each conflicting field
	for _, fieldConflict := range conflict.Fields {
		if !s.resolveFieldConflict(fieldConflict, merged, downstream) {
			unresolvedConflicts = append(unresolvedConflicts, fieldConflict)
		}
	}

	preserveDownstreamOnlyFields(downstream, merged)

	return &ResolutionResult{
		Resolved:  len(unresolvedConflicts) == 0,
		Merged:    merged,
		Conflicts: unresolvedConflicts,
	}, nil
}

func (s *MergeStrategy) resolveFieldConflict(conflict FieldConflict, merged, downstream *unstructured.Unstructured) bool {
	switch {
	case strings.Contains(conflict.Path, "metadata.annotations"):
		return s.mergeAnnotations(conflict, merged)
	case strings.Contains(conflict.Path, "metadata.labels"):
		return s.mergeLabels(conflict, merged)
	case conflict.Resolution == "missing_in_downstream":
		return true // Keep KCP value
	default:
		return true // Default to KCP value
	}
}

func (s *MergeStrategy) mergeAnnotations(conflict FieldConflict, merged *unstructured.Unstructured) bool {
	kcpAnnotations, ok1 := conflict.KCPValue.(map[string]interface{})
	downstreamAnnotations, ok2 := conflict.DownstreamValue.(map[string]interface{})
	if ok1 && ok2 {
		for key, value := range downstreamAnnotations {
			if _, exists := kcpAnnotations[key]; !exists {
				kcpAnnotations[key] = value
			}
		}
		unstructured.SetNestedField(merged.Object, kcpAnnotations, "metadata", "annotations")
		return true
	}
	return false
}

func (s *MergeStrategy) mergeLabels(conflict FieldConflict, merged *unstructured.Unstructured) bool {
	kcpLabels, ok1 := conflict.KCPValue.(map[string]interface{})
	downstreamLabels, ok2 := conflict.DownstreamValue.(map[string]interface{})
	if ok1 && ok2 {
		for key, value := range downstreamLabels {
			if _, exists := kcpLabels[key]; !exists {
				kcpLabels[key] = value
			}
		}
		unstructured.SetNestedField(merged.Object, kcpLabels, "metadata", "labels")
		return true
	}
	return false
}

// ManualStrategy marks conflicts for manual intervention
type ManualStrategy struct{}

func (s *ManualStrategy) Resolve(ctx context.Context, kcp, downstream *unstructured.Unstructured, conflict *Conflict) (*ResolutionResult, error) {
	klog.FromContext(ctx).V(2).Info("Marking resource for manual resolution", "resource", fmt.Sprintf("%s/%s", conflict.Namespace, conflict.Name))

	base := kcp
	if kcp == nil {
		base = downstream
	}
	if base == nil {
		return nil, fmt.Errorf("both KCP and downstream resources are nil")
	}

	merged := base.DeepCopy()
	addConflictAnnotations(merged, conflict)

	return &ResolutionResult{
		Resolved:  false,
		Merged:    merged,
		Conflicts: conflict.Fields,
	}, nil
}

// Helper functions for field preservation and metadata management

func preserveDownstreamOnlyFields(downstream, merged *unstructured.Unstructured) {
	// Preserve status from downstream as it reflects actual state
	if status, found, _ := unstructured.NestedFieldNoCopy(downstream.Object, "status"); found {
		unstructured.SetNestedField(merged.Object, status, "status")
	}

	// Preserve server-managed fields
	serverFields := []string{"metadata.uid", "metadata.resourceVersion", "metadata.generation"}
	for _, field := range serverFields {
		if value, found, _ := unstructured.NestedFieldNoCopy(downstream.Object, field); found {
			unstructured.SetNestedField(merged.Object, value, field)
		}
	}
}

func markForUpstreamSync(merged *unstructured.Unstructured) {
	annotations := merged.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations["syncer.kcp.io/upstream-sync"] = "pending"
	merged.SetAnnotations(annotations)
}

func addConflictAnnotations(resource *unstructured.Unstructured, conflict *Conflict) {
	annotations := resource.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	annotations["syncer.kcp.io/conflict-type"] = string(conflict.Type)
	annotations["syncer.kcp.io/conflict-severity"] = conflict.Severity.String()
	annotations["syncer.kcp.io/sync-paused"] = "true"
	
	resource.SetAnnotations(annotations)
}