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
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"
)

// ConflictResolutionStrategy defines how conflicts should be resolved
type ConflictResolutionStrategy string

const (
	// ServerSideWins - KCP/server version takes precedence
	ServerSideWins ConflictResolutionStrategy = "server-side-wins"
	
	// ClientSideWins - Physical cluster version takes precedence  
	ClientSideWins ConflictResolutionStrategy = "client-side-wins"
	
	// ThreeWayMerge - Attempt intelligent merge of both versions
	ThreeWayMerge ConflictResolutionStrategy = "three-way-merge"
	
	// LastWriteWins - Most recently modified version wins
	LastWriteWins ConflictResolutionStrategy = "last-write-wins"
)

// ConflictInfo contains information about a detected conflict
type ConflictInfo struct {
	// Resource identification
	GroupVersionResource schema.GroupVersionResource
	Name                string
	Namespace           string
	
	// Conflict details
	ConflictType    string
	ConflictReason  string
	DetectedAt      time.Time
	
	// Resolution applied
	ResolutionStrategy ConflictResolutionStrategy
	ResolvedBy         string
}

// conflictResolver handles resource conflicts between upstream and downstream sync
type conflictResolver struct {
	// Default resolution strategy
	defaultStrategy ConflictResolutionStrategy
	
	// Per-resource-type strategies
	resourceStrategies map[schema.GroupVersionResource]ConflictResolutionStrategy
	
	// Conflict history tracking
	conflictHistory []ConflictInfo
	maxHistorySize  int
}

// newConflictResolver creates a new conflict resolver with default settings
func newConflictResolver() *conflictResolver {
	return &conflictResolver{
		defaultStrategy:    ThreeWayMerge,
		resourceStrategies: make(map[schema.GroupVersionResource]ConflictResolutionStrategy),
		conflictHistory:    make([]ConflictInfo, 0),
		maxHistorySize:     100,
	}
}

// resolveConflict resolves a conflict between KCP and physical cluster versions of a resource
func (cr *conflictResolver) resolveConflict(ctx context.Context, kcpResource, physicalResource *unstructured.Unstructured, gvr schema.GroupVersionResource) (*unstructured.Unstructured, error) {
	logger := klog.FromContext(ctx)
	
	// Detect the type of conflict
	conflictType, conflictReason := cr.detectConflictType(kcpResource, physicalResource)
	
	if conflictType == "" {
		// No conflict detected
		return physicalResource, nil
	}
	
	logger.V(3).Info("Conflict detected during upstream sync",
		"resource", gvr.String(),
		"name", physicalResource.GetName(),
		"namespace", physicalResource.GetNamespace(),
		"conflictType", conflictType,
		"reason", conflictReason)
	
	// Get resolution strategy for this resource type
	strategy := cr.getResolutionStrategy(gvr)
	
	// Apply resolution strategy
	resolvedResource, err := cr.applyResolutionStrategy(ctx, kcpResource, physicalResource, strategy)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve conflict using strategy %s: %w", strategy, err)
	}
	
	// Record conflict in history
	conflictInfo := ConflictInfo{
		GroupVersionResource: gvr,
		Name:                 physicalResource.GetName(),
		Namespace:            physicalResource.GetNamespace(),
		ConflictType:         conflictType,
		ConflictReason:       conflictReason,
		DetectedAt:           time.Now(),
		ResolutionStrategy:   strategy,
		ResolvedBy:           "upstream-syncer",
	}
	cr.recordConflict(conflictInfo)
	
	logger.V(3).Info("Conflict resolved successfully",
		"resource", gvr.String(),
		"name", physicalResource.GetName(), 
		"strategy", strategy)
	
	return resolvedResource, nil
}

// detectConflictType identifies the type of conflict between two resource versions
func (cr *conflictResolver) detectConflictType(kcpResource, physicalResource *unstructured.Unstructured) (string, string) {
	// Check for generation mismatch
	kcpGeneration := kcpResource.GetGeneration()
	physicalGeneration := physicalResource.GetGeneration()
	
	if kcpGeneration != physicalGeneration {
		return "generation-mismatch", fmt.Sprintf("KCP generation: %d, Physical generation: %d", kcpGeneration, physicalGeneration)
	}
	
	// Check for spec differences
	kcpSpec, kcpHasSpec, _ := unstructured.NestedMap(kcpResource.Object, "spec")
	physicalSpec, physicalHasSpec, _ := unstructured.NestedMap(physicalResource.Object, "spec")
	
	if kcpHasSpec && physicalHasSpec {
		if !cr.deepEqual(kcpSpec, physicalSpec) {
			return "spec-divergence", "Spec fields differ between KCP and physical cluster"
		}
	}
	
	// Check for annotation conflicts
	kcpAnnotations := kcpResource.GetAnnotations()
	physicalAnnotations := physicalResource.GetAnnotations()
	
	if cr.hasAnnotationConflicts(kcpAnnotations, physicalAnnotations) {
		return "annotation-conflict", "Critical annotations differ"
	}
	
	// Check for label conflicts
	kcpLabels := kcpResource.GetLabels()
	physicalLabels := physicalResource.GetLabels()
	
	if cr.hasLabelConflicts(kcpLabels, physicalLabels) {
		return "label-conflict", "Critical labels differ"
	}
	
	// No significant conflicts detected
	return "", ""
}

// getResolutionStrategy returns the appropriate resolution strategy for a resource type
func (cr *conflictResolver) getResolutionStrategy(gvr schema.GroupVersionResource) ConflictResolutionStrategy {
	if strategy, exists := cr.resourceStrategies[gvr]; exists {
		return strategy
	}
	
	// Apply resource-specific defaults
	switch gvr.Resource {
	case "pods":
		// For pods, physical cluster state is authoritative
		return ClientSideWins
	case "services":
		// Services can be safely merged in most cases
		return ThreeWayMerge
	case "deployments", "statefulsets", "daemonsets":
		// Workload controllers: prefer KCP spec but merge status
		return ThreeWayMerge
	case "configmaps", "secrets":
		// Configuration: prefer KCP version
		return ServerSideWins
	default:
		return cr.defaultStrategy
	}
}

// applyResolutionStrategy applies the chosen conflict resolution strategy
func (cr *conflictResolver) applyResolutionStrategy(ctx context.Context, kcpResource, physicalResource *unstructured.Unstructured, strategy ConflictResolutionStrategy) (*unstructured.Unstructured, error) {
	switch strategy {
	case ServerSideWins:
		return cr.serverSideWins(kcpResource, physicalResource)
	case ClientSideWins:
		return cr.clientSideWins(kcpResource, physicalResource)
	case ThreeWayMerge:
		return cr.threeWayMerge(ctx, kcpResource, physicalResource)
	case LastWriteWins:
		return cr.lastWriteWins(kcpResource, physicalResource)
	default:
		return nil, fmt.Errorf("unsupported resolution strategy: %s", strategy)
	}
}

// serverSideWins implements server-side wins strategy (KCP version preferred)
func (cr *conflictResolver) serverSideWins(kcpResource, physicalResource *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	result := kcpResource.DeepCopy()
	
	// Preserve physical cluster status
	if physicalStatus, hasStatus, _ := unstructured.NestedMap(physicalResource.Object, "status"); hasStatus {
		unstructured.SetNestedMap(result.Object, physicalStatus, "status")
	}
	
	return result, nil
}

// clientSideWins implements client-side wins strategy (physical cluster preferred)
func (cr *conflictResolver) clientSideWins(kcpResource, physicalResource *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	result := physicalResource.DeepCopy()
	
	// Preserve KCP workspace annotations
	kcpAnnotations := kcpResource.GetAnnotations()
	physicalAnnotations := result.GetAnnotations()
	if physicalAnnotations == nil {
		physicalAnnotations = make(map[string]string)
	}
	
	// Copy KCP-specific annotations
	for key, value := range kcpAnnotations {
		if cr.isKCPSpecificAnnotation(key) {
			physicalAnnotations[key] = value
		}
	}
	result.SetAnnotations(physicalAnnotations)
	
	return result, nil
}

// threeWayMerge implements intelligent merge strategy
func (cr *conflictResolver) threeWayMerge(ctx context.Context, kcpResource, physicalResource *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	result := physicalResource.DeepCopy()
	
	// Start with physical resource (authoritative for status)
	// Merge in KCP-specific metadata
	cr.mergeKCPMetadata(result, kcpResource)
	
	// For workload controllers, prefer KCP spec but keep physical status
	if cr.isWorkloadController(result) {
		if kcpSpec, hasSpec, _ := unstructured.NestedMap(kcpResource.Object, "spec"); hasSpec {
			unstructured.SetNestedMap(result.Object, kcpSpec, "spec")
		}
	}
	
	return result, nil
}

// lastWriteWins implements timestamp-based conflict resolution
func (cr *conflictResolver) lastWriteWins(kcpResource, physicalResource *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	kcpTime := kcpResource.GetCreationTimestamp()
	physicalTime := physicalResource.GetCreationTimestamp()
	
	if physicalTime.After(kcpTime.Time) {
		return cr.clientSideWins(kcpResource, physicalResource)
	}
	
	return cr.serverSideWins(kcpResource, physicalResource)
}

// Helper methods

func (cr *conflictResolver) deepEqual(a, b interface{}) bool {
	// Simple implementation - in practice would use more sophisticated comparison
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

func (cr *conflictResolver) hasAnnotationConflicts(kcpAnnotations, physicalAnnotations map[string]string) bool {
	// Check for conflicts in critical annotations
	criticalAnnotations := []string{
		"deployment.kubernetes.io/revision",
		"kubectl.kubernetes.io/last-applied-configuration",
	}
	
	for _, annotation := range criticalAnnotations {
		kcpValue, kcpHas := kcpAnnotations[annotation]
		physicalValue, physicalHas := physicalAnnotations[annotation]
		
		if kcpHas && physicalHas && kcpValue != physicalValue {
			return true
		}
	}
	
	return false
}

func (cr *conflictResolver) hasLabelConflicts(kcpLabels, physicalLabels map[string]string) bool {
	// Check for conflicts in critical labels
	criticalLabels := []string{
		"app",
		"version",
		"component",
	}
	
	for _, label := range criticalLabels {
		kcpValue, kcpHas := kcpLabels[label]
		physicalValue, physicalHas := physicalLabels[label]
		
		if kcpHas && physicalHas && kcpValue != physicalValue {
			return true
		}
	}
	
	return false
}

func (cr *conflictResolver) isKCPSpecificAnnotation(key string) bool {
	kcpPrefixes := []string{
		"workload.kcp.io/",
		"apis.kcp.io/",
		"tenancy.kcp.io/",
	}
	
	for _, prefix := range kcpPrefixes {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			return true
		}
	}
	
	return false
}

func (cr *conflictResolver) mergeKCPMetadata(target, source *unstructured.Unstructured) {
	// Merge KCP-specific annotations
	sourceAnnotations := source.GetAnnotations()
	targetAnnotations := target.GetAnnotations()
	if targetAnnotations == nil {
		targetAnnotations = make(map[string]string)
	}
	
	for key, value := range sourceAnnotations {
		if cr.isKCPSpecificAnnotation(key) {
			targetAnnotations[key] = value
		}
	}
	
	target.SetAnnotations(targetAnnotations)
}

func (cr *conflictResolver) isWorkloadController(resource *unstructured.Unstructured) bool {
	gvk := resource.GetObjectKind().GroupVersionKind()
	
	workloadControllers := []string{
		"Deployment",
		"StatefulSet", 
		"DaemonSet",
		"ReplicaSet",
	}
	
	for _, controller := range workloadControllers {
		if gvk.Kind == controller {
			return true
		}
	}
	
	return false
}

func (cr *conflictResolver) recordConflict(conflict ConflictInfo) {
	cr.conflictHistory = append(cr.conflictHistory, conflict)
	
	// Trim history if it exceeds max size
	if len(cr.conflictHistory) > cr.maxHistorySize {
		cr.conflictHistory = cr.conflictHistory[1:]
	}
}

// getConflictHistory returns recent conflict history
func (cr *conflictResolver) getConflictHistory() []ConflictInfo {
	result := make([]ConflictInfo, len(cr.conflictHistory))
	copy(result, cr.conflictHistory)
	return result
}