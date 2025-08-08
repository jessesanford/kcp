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

package engine

import (
	"context"
	"fmt"
	"sort"

	tmcv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/tmc/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// ResourceAwareEngine implements PlacementEngine using resource-aware strategies.
// It considers cluster capacity, current resource usage, and workload requirements
// to make intelligent placement decisions that optimize resource utilization.
type ResourceAwareEngine struct {
	// strategy defines the resource-aware placement strategy
	strategy ResourceStrategy
}

// ResourceStrategy defines the strategy for resource-aware placement decisions
type ResourceStrategy int

const (
	// LeastLoadedStrategy places workloads on clusters with the lowest resource utilization
	LeastLoadedStrategy ResourceStrategy = iota
	// BestFitStrategy places workloads on clusters that best fit the resource requirements
	BestFitStrategy
	// BalancedStrategy balances between resource efficiency and load distribution
	BalancedStrategy
)

// NewResourceAwareEngine creates a new resource-aware placement engine.
// It uses the LeastLoaded strategy by default, which places workloads on
// clusters with the lowest current resource utilization.
func NewResourceAwareEngine() *ResourceAwareEngine {
	return &ResourceAwareEngine{
		strategy: LeastLoadedStrategy,
	}
}

// NewResourceAwareEngineWithStrategy creates a resource-aware engine with a specific strategy.
func NewResourceAwareEngineWithStrategy(strategy ResourceStrategy) *ResourceAwareEngine {
	return &ResourceAwareEngine{
		strategy: strategy,
	}
}

// SelectClusters implements PlacementEngine.SelectClusters using resource-aware algorithms.
// It evaluates clusters based on their resource capacity, current utilization, and
// availability to make optimal placement decisions.
func (e *ResourceAwareEngine) SelectClusters(ctx context.Context,
	workload *tmcv1alpha1.WorkloadPlacement,
	clusters []*tmcv1alpha1.ClusterRegistration,
) ([]PlacementDecision, error) {
	if workload == nil {
		return nil, fmt.Errorf("workload placement cannot be nil")
	}

	// Filter clusters based on cluster selector
	eligibleClusters, err := e.filterClusters(clusters, &workload.Spec.ClusterSelector)
	if err != nil {
		return nil, fmt.Errorf("failed to filter clusters: %w", err)
	}

	if len(eligibleClusters) == 0 {
		return nil, fmt.Errorf("no eligible clusters found matching selector")
	}

	// Filter clusters by resource availability
	availableClusters := e.filterByResourceAvailability(eligibleClusters)
	if len(availableClusters) == 0 {
		return nil, fmt.Errorf("no clusters with sufficient resources available")
	}

	// Determine number of clusters to select
	requestedClusters := int(1) // default
	if workload.Spec.NumberOfClusters != nil {
		requestedClusters = int(*workload.Spec.NumberOfClusters)
	}

	if requestedClusters > len(availableClusters) {
		requestedClusters = len(availableClusters)
	}

	// Select clusters using resource-aware strategy
	decisions, err := e.selectByResourceStrategy(availableClusters, requestedClusters, e.strategy)
	if err != nil {
		return nil, fmt.Errorf("failed to select clusters by resource strategy: %w", err)
	}

	return decisions, nil
}

// filterClusters filters the provided clusters based on the cluster selector.
func (e *ResourceAwareEngine) filterClusters(clusters []*tmcv1alpha1.ClusterRegistration,
	selector *tmcv1alpha1.ClusterSelector) ([]*tmcv1alpha1.ClusterRegistration, error) {
	
	if selector == nil {
		return clusters, nil
	}

	var filtered []*tmcv1alpha1.ClusterRegistration

	for _, cluster := range clusters {
		// Check explicit cluster names
		if len(selector.ClusterNames) > 0 {
			found := false
			for _, name := range selector.ClusterNames {
				if cluster.Name == name {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Check location selector
		if len(selector.LocationSelector) > 0 {
			found := false
			for _, location := range selector.LocationSelector {
				if cluster.Spec.Location == location {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Check label selector
		if selector.LabelSelector != nil {
			labelSelector, err := metav1.LabelSelectorAsSelector(selector.LabelSelector)
			if err != nil {
				return nil, fmt.Errorf("invalid label selector: %w", err)
			}
			if !labelSelector.Matches(labels.Set(cluster.Labels)) {
				continue
			}
		}

		filtered = append(filtered, cluster)
	}

	return filtered, nil
}

// filterByResourceAvailability filters clusters that have available resources.
// Clusters without capacity information or with unknown resource status are included
// to maintain backward compatibility.
func (e *ResourceAwareEngine) filterByResourceAvailability(clusters []*tmcv1alpha1.ClusterRegistration) []*tmcv1alpha1.ClusterRegistration {
	var available []*tmcv1alpha1.ClusterRegistration

	for _, cluster := range clusters {
		// If cluster has no capacity information, consider it available
		if cluster.Spec.Capacity.CPU == nil && cluster.Spec.Capacity.Memory == nil {
			available = append(available, cluster)
			continue
		}

		// Check if cluster has available resources
		if e.hasAvailableResources(cluster) {
			available = append(available, cluster)
		}
	}

	return available
}

// hasAvailableResources checks if a cluster has available resources based on capacity and usage.
func (e *ResourceAwareEngine) hasAvailableResources(cluster *tmcv1alpha1.ClusterRegistration) bool {
	capacity := cluster.Spec.Capacity
	status := cluster.Status

	// If no allocated resources tracked, assume resources are available
	if status.AllocatedResources == nil {
		return true
	}

	allocated := status.AllocatedResources

	// Check CPU availability (require at least 10% free capacity)
	if capacity.CPU != nil && allocated.CPU != nil {
		cpuUtil := float64(*allocated.CPU) / float64(*capacity.CPU)
		if cpuUtil >= 0.9 { // 90% utilization threshold
			return false
		}
	}

	// Check memory availability (require at least 10% free capacity)
	if capacity.Memory != nil && allocated.Memory != nil {
		memUtil := float64(*allocated.Memory) / float64(*capacity.Memory)
		if memUtil >= 0.9 { // 90% utilization threshold
			return false
		}
	}

	// Check pod availability (require at least 10% free capacity)
	if capacity.MaxPods != nil && allocated.Pods != nil {
		podUtil := float64(*allocated.Pods) / float64(*capacity.MaxPods)
		if podUtil >= 0.9 { // 90% utilization threshold
			return false
		}
	}

	return true
}

// selectByResourceStrategy selects clusters using the specified resource strategy.
func (e *ResourceAwareEngine) selectByResourceStrategy(clusters []*tmcv1alpha1.ClusterRegistration,
	count int, strategy ResourceStrategy) ([]PlacementDecision, error) {

	switch strategy {
	case LeastLoadedStrategy:
		return e.selectLeastLoaded(clusters, count)
	case BestFitStrategy:
		return e.selectBestFit(clusters, count)
	case BalancedStrategy:
		return e.selectBalanced(clusters, count)
	default:
		return e.selectLeastLoaded(clusters, count)
	}
}

// selectLeastLoaded selects clusters with the lowest resource utilization.
func (e *ResourceAwareEngine) selectLeastLoaded(clusters []*tmcv1alpha1.ClusterRegistration,
	count int) ([]PlacementDecision, error) {

	// Calculate utilization scores for each cluster
	type clusterScore struct {
		cluster     *tmcv1alpha1.ClusterRegistration
		utilization float64
		score       int
	}

	scores := make([]clusterScore, 0, len(clusters))

	for _, cluster := range clusters {
		util := e.calculateUtilization(cluster)
		// Higher score for lower utilization (invert utilization for scoring)
		score := int((1.0 - util) * 100)
		scores = append(scores, clusterScore{
			cluster:     cluster,
			utilization: util,
			score:       score,
		})
	}

	// Sort by lowest utilization first (highest score)
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score > scores[j].score
	})

	// Select top clusters up to requested count
	decisions := make([]PlacementDecision, 0, count)
	for i := 0; i < count && i < len(scores); i++ {
		cs := scores[i]
		reason := fmt.Sprintf("LeastLoaded strategy: %.1f%% utilization (score: %d)",
			cs.utilization*100, cs.score)
		
		decisions = append(decisions, PlacementDecision{
			ClusterName: cs.cluster.Name,
			Score:       cs.score,
			Reason:      reason,
		})
	}

	return decisions, nil
}

// selectBestFit attempts to find clusters that best fit the resource requirements.
// Currently simplified to use least loaded strategy as workload requirements are not yet specified.
func (e *ResourceAwareEngine) selectBestFit(clusters []*tmcv1alpha1.ClusterRegistration,
	count int) ([]PlacementDecision, error) {

	// TODO: When workload resource requirements are added to WorkloadPlacementSpec,
	// implement true best-fit algorithm that matches workload requirements to cluster capacity

	// For now, fall back to least loaded strategy
	decisions, err := e.selectLeastLoaded(clusters, count)
	if err != nil {
		return nil, err
	}

	// Update reasons to reflect best-fit strategy
	for i := range decisions {
		decisions[i].Reason = fmt.Sprintf("BestFit strategy (using least loaded): %s", decisions[i].Reason)
	}

	return decisions, nil
}

// selectBalanced selects clusters using a balanced approach that considers both
// resource utilization and load distribution.
func (e *ResourceAwareEngine) selectBalanced(clusters []*tmcv1alpha1.ClusterRegistration,
	count int) ([]PlacementDecision, error) {

	// Calculate balanced scores considering both utilization and distribution
	type clusterScore struct {
		cluster        *tmcv1alpha1.ClusterRegistration
		utilization    float64
		balancedScore  int
	}

	scores := make([]clusterScore, 0, len(clusters))

	for _, cluster := range clusters {
		util := e.calculateUtilization(cluster)
		// Balanced scoring: prefer moderate utilization over extremely low utilization
		// This helps distribute load more evenly
		var balancedScore float64
		if util < 0.3 {
			// Low utilization: good but not optimal for distribution
			balancedScore = 0.8 + util*0.7 // Scale between 80-101%
		} else if util < 0.7 {
			// Moderate utilization: optimal for balanced placement
			balancedScore = 1.0 - (util-0.3)*0.5 // Scale between 100-80%
		} else {
			// High utilization: less preferred
			balancedScore = 0.8 - (util-0.7)*2.7 // Scale between 80-0%
		}

		score := int(balancedScore * 100)
		scores = append(scores, clusterScore{
			cluster:       cluster,
			utilization:   util,
			balancedScore: score,
		})
	}

	// Sort by balanced score
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].balancedScore > scores[j].balancedScore
	})

	// Select top clusters up to requested count
	decisions := make([]PlacementDecision, 0, count)
	for i := 0; i < count && i < len(scores); i++ {
		cs := scores[i]
		reason := fmt.Sprintf("Balanced strategy: %.1f%% utilization (balanced score: %d)",
			cs.utilization*100, cs.balancedScore)
		
		decisions = append(decisions, PlacementDecision{
			ClusterName: cs.cluster.Name,
			Score:       cs.balancedScore,
			Reason:      reason,
		})
	}

	return decisions, nil
}

// calculateUtilization calculates the overall resource utilization for a cluster.
// Returns a value between 0.0 (no utilization) and 1.0 (full utilization).
func (e *ResourceAwareEngine) calculateUtilization(cluster *tmcv1alpha1.ClusterRegistration) float64 {
	capacity := cluster.Spec.Capacity
	status := cluster.Status

	// If no capacity or allocation information, assume low utilization
	if status.AllocatedResources == nil {
		return 0.1 // Default to 10% utilization for unknown clusters
	}

	allocated := status.AllocatedResources
	var totalUtil float64
	var resourceCount int

	// Calculate CPU utilization
	if capacity.CPU != nil && allocated.CPU != nil && *capacity.CPU > 0 {
		cpuUtil := float64(*allocated.CPU) / float64(*capacity.CPU)
		totalUtil += cpuUtil
		resourceCount++
	}

	// Calculate memory utilization
	if capacity.Memory != nil && allocated.Memory != nil && *capacity.Memory > 0 {
		memUtil := float64(*allocated.Memory) / float64(*capacity.Memory)
		totalUtil += memUtil
		resourceCount++
	}

	// Calculate pod utilization
	if capacity.MaxPods != nil && allocated.Pods != nil && *capacity.MaxPods > 0 {
		podUtil := float64(*allocated.Pods) / float64(*capacity.MaxPods)
		totalUtil += podUtil
		resourceCount++
	}

	// Return average utilization across tracked resources
	if resourceCount == 0 {
		return 0.1 // Default to 10% if no resources tracked
	}

	avgUtil := totalUtil / float64(resourceCount)
	
	// Ensure utilization is within bounds
	if avgUtil < 0 {
		return 0
	}
	if avgUtil > 1 {
		return 1
	}

	return avgUtil
}