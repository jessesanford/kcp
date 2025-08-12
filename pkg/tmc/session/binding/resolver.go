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

package binding

import (
	"context"
	"fmt"
	"math/rand"
	"sort"

	"k8s.io/klog/v2"

	tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
)

// Resolver handles the resolution of placement targets based on various criteria and policies.
// It implements the logic for selecting appropriate targets for session binding.
type Resolver struct {
	// options can be extended with configuration options
}

// NewResolver creates a new target resolver.
func NewResolver() *Resolver {
	return &Resolver{}
}

// ResolveDefaultTargets resolves targets using default selection logic.
// This is used when no specific affinity policy is provided.
func (r *Resolver) ResolveDefaultTargets(ctx context.Context, candidates []tmcv1alpha1.PlacementTarget) ([]tmcv1alpha1.PlacementTarget, error) {
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no candidate targets provided")
	}
	
	klog.V(3).InfoS("Resolving default targets", "candidates", len(candidates))
	
	// Sort candidates by priority (higher priority first)
	sortedCandidates := make([]tmcv1alpha1.PlacementTarget, len(candidates))
	copy(sortedCandidates, candidates)
	sort.Slice(sortedCandidates, func(i, j int) bool {
		priorityI := int32(50) // default priority
		if sortedCandidates[i].Priority != nil {
			priorityI = *sortedCandidates[i].Priority
		}
		priorityJ := int32(50) // default priority
		if sortedCandidates[j].Priority != nil {
			priorityJ = *sortedCandidates[j].Priority
		}
		return priorityI > priorityJ
	})
	
	// Return the highest priority target
	return []tmcv1alpha1.PlacementTarget{sortedCandidates[0]}, nil
}

// ResolveClusterTargets resolves targets based on cluster affinity policy.
func (r *Resolver) ResolveClusterTargets(ctx context.Context, candidates []tmcv1alpha1.PlacementTarget, policy *tmcv1alpha1.SessionAffinityPolicy) ([]tmcv1alpha1.PlacementTarget, error) {
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no candidate targets provided")
	}
	
	klog.V(3).InfoS("Resolving cluster affinity targets", 
		"candidates", len(candidates),
		"policy", policy.Name)
	
	// Filter candidates based on session selector if provided
	filteredCandidates := r.filterBySessionSelector(candidates, policy.Spec.SessionSelector)
	if len(filteredCandidates) == 0 {
		return nil, fmt.Errorf("no targets match session selector criteria")
	}
	
	// Apply cluster-level selection logic
	selectedTargets := r.selectClusterTargets(filteredCandidates, policy)
	
	// Limit based on max sessions per target
	if policy.Spec.MaxSessionsPerTarget != nil {
		selectedTargets = r.limitTargetsByCapacity(selectedTargets, *policy.Spec.MaxSessionsPerTarget)
	}
	
	return selectedTargets, nil
}

// ResolveNodeTargets resolves targets based on node affinity policy.
func (r *Resolver) ResolveNodeTargets(ctx context.Context, candidates []tmcv1alpha1.PlacementTarget, policy *tmcv1alpha1.SessionAffinityPolicy) ([]tmcv1alpha1.PlacementTarget, error) {
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no candidate targets provided")
	}
	
	klog.V(3).InfoS("Resolving node affinity targets",
		"candidates", len(candidates),
		"policy", policy.Name)
	
	// Filter candidates that have node selector requirements
	nodeTargets := r.filterNodeCapableCandidates(candidates)
	if len(nodeTargets) == 0 {
		// Fallback to cluster-level targets
		return r.ResolveClusterTargets(ctx, candidates, policy)
	}
	
	// Filter by session selector if provided
	filteredTargets := r.filterBySessionSelector(nodeTargets, policy.Spec.SessionSelector)
	if len(filteredTargets) == 0 {
		return nil, fmt.Errorf("no node-capable targets match session selector criteria")
	}
	
	// Select based on node affinity criteria
	selectedTargets := r.selectNodeTargets(filteredTargets, policy)
	
	// Apply capacity limits
	if policy.Spec.MaxSessionsPerTarget != nil {
		selectedTargets = r.limitTargetsByCapacity(selectedTargets, *policy.Spec.MaxSessionsPerTarget)
	}
	
	return selectedTargets, nil
}

// ResolveWorkspaceTargets resolves targets based on workspace affinity policy.
func (r *Resolver) ResolveWorkspaceTargets(ctx context.Context, candidates []tmcv1alpha1.PlacementTarget, policy *tmcv1alpha1.SessionAffinityPolicy, namespace string) ([]tmcv1alpha1.PlacementTarget, error) {
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no candidate targets provided")
	}
	
	klog.V(3).InfoS("Resolving workspace affinity targets",
		"candidates", len(candidates),
		"policy", policy.Name,
		"namespace", namespace)
	
	// Filter candidates based on workspace context
	workspaceTargets := r.filterByWorkspaceContext(candidates, namespace, policy.Spec.SessionSelector)
	if len(workspaceTargets) == 0 {
		// Fallback to cluster-level targets
		return r.ResolveClusterTargets(ctx, candidates, policy)
	}
	
	// Select based on workspace affinity criteria
	selectedTargets := r.selectWorkspaceTargets(workspaceTargets, policy, namespace)
	
	// Apply capacity limits
	if policy.Spec.MaxSessionsPerTarget != nil {
		selectedTargets = r.limitTargetsByCapacity(selectedTargets, *policy.Spec.MaxSessionsPerTarget)
	}
	
	return selectedTargets, nil
}

// filterBySessionSelector filters candidates based on session selector criteria.
func (r *Resolver) filterBySessionSelector(candidates []tmcv1alpha1.PlacementTarget, selector *tmcv1alpha1.SessionSelector) []tmcv1alpha1.PlacementTarget {
	if selector == nil {
		return candidates
	}
	
	filtered := make([]tmcv1alpha1.PlacementTarget, 0, len(candidates))
	
	for _, candidate := range candidates {
		if r.matchesSelector(candidate, selector) {
			filtered = append(filtered, candidate)
		}
	}
	
	return filtered
}

// matchesSelector checks if a candidate matches the session selector criteria.
func (r *Resolver) matchesSelector(candidate tmcv1alpha1.PlacementTarget, selector *tmcv1alpha1.SessionSelector) bool {
	// For simplicity, we'll just check basic criteria
	// In a real implementation, this would be more sophisticated
	
	// If no specific criteria, match all
	if selector.MatchLabels == nil && len(selector.MatchExpressions) == 0 {
		return true
	}
	
	// This would typically match against cluster labels or annotations
	// For now, we'll consider all targets as matching
	return true
}

// filterNodeCapableCandidates filters candidates that can support node-level affinity.
func (r *Resolver) filterNodeCapableCandidates(candidates []tmcv1alpha1.PlacementTarget) []tmcv1alpha1.PlacementTarget {
	filtered := make([]tmcv1alpha1.PlacementTarget, 0, len(candidates))
	
	for _, candidate := range candidates {
		// Consider targets with existing node selectors or those that can support them
		if len(candidate.NodeSelector) > 0 {
			filtered = append(filtered, candidate)
		}
	}
	
	// If no targets have node selectors, return all (they can potentially support node affinity)
	if len(filtered) == 0 {
		return candidates
	}
	
	return filtered
}

// filterByWorkspaceContext filters candidates based on workspace context and namespace.
func (r *Resolver) filterByWorkspaceContext(candidates []tmcv1alpha1.PlacementTarget, namespace string, selector *tmcv1alpha1.SessionSelector) []tmcv1alpha1.PlacementTarget {
	if selector == nil {
		return candidates
	}
	
	// Check if namespace is in the selector's namespace list
	if len(selector.Namespaces) > 0 {
		namespaceMatches := false
		for _, ns := range selector.Namespaces {
			if ns == namespace {
				namespaceMatches = true
				break
			}
		}
		if !namespaceMatches {
			// No targets match if namespace doesn't match selector
			return []tmcv1alpha1.PlacementTarget{}
		}
	}
	
	// Filter by other selector criteria
	return r.filterBySessionSelector(candidates, selector)
}

// selectClusterTargets selects targets based on cluster affinity logic.
func (r *Resolver) selectClusterTargets(candidates []tmcv1alpha1.PlacementTarget, policy *tmcv1alpha1.SessionAffinityPolicy) []tmcv1alpha1.PlacementTarget {
	if len(candidates) == 0 {
		return candidates
	}
	
	// Sort by priority and weight
	sorted := r.sortTargetsByPriority(candidates)
	
	// For cluster affinity, typically select one primary target
	return []tmcv1alpha1.PlacementTarget{sorted[0]}
}

// selectNodeTargets selects targets based on node affinity logic.
func (r *Resolver) selectNodeTargets(candidates []tmcv1alpha1.PlacementTarget, policy *tmcv1alpha1.SessionAffinityPolicy) []tmcv1alpha1.PlacementTarget {
	if len(candidates) == 0 {
		return candidates
	}
	
	// For node affinity, prefer targets with specific node selectors
	targetsWithNodeSelector := make([]tmcv1alpha1.PlacementTarget, 0)
	for _, target := range candidates {
		if len(target.NodeSelector) > 0 {
			targetsWithNodeSelector = append(targetsWithNodeSelector, target)
		}
	}
	
	if len(targetsWithNodeSelector) > 0 {
		sorted := r.sortTargetsByPriority(targetsWithNodeSelector)
		return []tmcv1alpha1.PlacementTarget{sorted[0]}
	}
	
	// Fallback to regular selection
	sorted := r.sortTargetsByPriority(candidates)
	return []tmcv1alpha1.PlacementTarget{sorted[0]}
}

// selectWorkspaceTargets selects targets based on workspace affinity logic.
func (r *Resolver) selectWorkspaceTargets(candidates []tmcv1alpha1.PlacementTarget, policy *tmcv1alpha1.SessionAffinityPolicy, namespace string) []tmcv1alpha1.PlacementTarget {
	if len(candidates) == 0 {
		return candidates
	}
	
	// For workspace affinity, distribute across multiple targets if available
	sorted := r.sortTargetsByPriority(candidates)
	
	// Select up to 2 targets for workspace distribution
	maxTargets := 2
	if len(sorted) < maxTargets {
		maxTargets = len(sorted)
	}
	
	return sorted[:maxTargets]
}

// sortTargetsByPriority sorts targets by priority (higher first) and then by weight.
func (r *Resolver) sortTargetsByPriority(targets []tmcv1alpha1.PlacementTarget) []tmcv1alpha1.PlacementTarget {
	sorted := make([]tmcv1alpha1.PlacementTarget, len(targets))
	copy(sorted, targets)
	
	sort.Slice(sorted, func(i, j int) bool {
		priorityI := int32(50) // default priority
		if sorted[i].Priority != nil {
			priorityI = *sorted[i].Priority
		}
		priorityJ := int32(50) // default priority
		if sorted[j].Priority != nil {
			priorityJ = *sorted[j].Priority
		}
		
		if priorityI != priorityJ {
			return priorityI > priorityJ
		}
		
		// If priorities are equal, sort by weight
		weightI := int32(1) // default weight
		if sorted[i].Weight != nil {
			weightI = *sorted[i].Weight
		}
		weightJ := int32(1) // default weight
		if sorted[j].Weight != nil {
			weightJ = *sorted[j].Weight
		}
		
		return weightI > weightJ
	})
	
	return sorted
}

// limitTargetsByCapacity limits the number of targets based on capacity constraints.
func (r *Resolver) limitTargetsByCapacity(targets []tmcv1alpha1.PlacementTarget, maxSessions int32) []tmcv1alpha1.PlacementTarget {
	// This is a simplified implementation
	// In practice, this would check actual session counts per target
	
	if maxSessions <= 0 {
		return targets
	}
	
	// For now, just return the targets as-is
	// In a real implementation, we would:
	// 1. Query existing sessions per target
	// 2. Filter out targets that have reached capacity
	// 3. Prefer targets with lower session counts
	
	return targets
}

// WeightedSelection performs weighted random selection from targets.
// This can be used for load balancing across multiple viable targets.
func (r *Resolver) WeightedSelection(targets []tmcv1alpha1.PlacementTarget) *tmcv1alpha1.PlacementTarget {
	if len(targets) == 0 {
		return nil
	}
	
	if len(targets) == 1 {
		return &targets[0]
	}
	
	// Calculate total weight
	totalWeight := int32(0)
	for _, target := range targets {
		weight := int32(1) // default weight
		if target.Weight != nil {
			weight = *target.Weight
		}
		totalWeight += weight
	}
	
	if totalWeight <= 0 {
		// If no weights, return random target
		return &targets[rand.Intn(len(targets))]
	}
	
	// Select based on weight
	randomValue := rand.Int31n(totalWeight)
	currentWeight := int32(0)
	
	for _, target := range targets {
		weight := int32(1) // default weight
		if target.Weight != nil {
			weight = *target.Weight
		}
		currentWeight += weight
		
		if randomValue < currentWeight {
			targetCopy := target
			return &targetCopy
		}
	}
	
	// Fallback to last target
	return &targets[len(targets)-1]
}