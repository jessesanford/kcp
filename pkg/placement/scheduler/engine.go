// Copyright 2024 The KCP Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package scheduler

import (
	"context"
	"fmt"
	"sort"
)

// Engine implements the placement scheduling engine following KCP patterns.
// It provides pluggable algorithms for different placement strategies while
// maintaining workspace isolation and resource tracking.
type Engine struct {
	// algorithms stores the registered scheduling algorithms
	algorithms map[string]Algorithm
	
	// scorer provides utilities for scoring and normalization
	scorer *Scorer
}

// NewEngine creates a new scheduling engine with default algorithms.
// It follows KCP's dependency injection pattern and registers common algorithms.
func NewEngine() *Engine {
	e := &Engine{
		algorithms: make(map[string]Algorithm),
		scorer:     NewScorer(),
	}

	// Register default algorithms following the plugin pattern
	e.RegisterAlgorithm("binpack", NewBinPackAlgorithm())
	e.RegisterAlgorithm("spread", NewSpreadAlgorithm())
	e.RegisterAlgorithm("affinity", NewAffinityAlgorithm())

	return e
}

// Schedule determines optimal placement for a workload across target clusters.
// It implements the core scheduling logic using the specified strategy and returns
// a placement decision that respects capacity constraints and placement policies.
//
// Parameters:
//   - ctx: Request context for cancellation and tracing
//   - workload: Workload to be placed with resource requirements and constraints
//   - targets: Available cluster targets for placement
//   - strategy: Scheduling algorithm to use ("binpack", "spread", "affinity")
//
// Returns:
//   - *PlacementDecision: Selected clusters and placement details
//   - error: Scheduling error or constraint violations
func (e *Engine) Schedule(ctx context.Context, workload Workload, 
	targets []ClusterTarget, strategy string) (*PlacementDecision, error) {

	algorithm, ok := e.algorithms[strategy]
	if !ok {
		return nil, fmt.Errorf("unknown scheduling algorithm: %s", strategy)
	}

	// Filter eligible targets based on capacity and constraints
	eligible := e.filterEligible(workload, targets)
	if len(eligible) == 0 {
		return nil, fmt.Errorf("no eligible clusters found for workload %s", workload.Name)
	}

	// Score targets using the selected algorithm
	scores, err := algorithm.Score(ctx, workload, eligible)
	if err != nil {
		return nil, fmt.Errorf("scoring failed for algorithm %s: %w", strategy, err)
	}

	// Sort by score in descending order (highest first)
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].Score > scores[j].Score
	})

	// Select top candidates based on replica count
	selected := e.selectCandidates(scores, workload.Spec.Replicas)

	return &PlacementDecision{
		WorkloadName: workload.Name,
		Clusters:     selected,
		Strategy:     strategy,
	}, nil
}

// filterEligible filters cluster targets that can accept the workload.
// It checks resource capacity, taints/tolerations, and other constraints.
func (e *Engine) filterEligible(workload Workload, targets []ClusterTarget) []ClusterTarget {
	eligible := []ClusterTarget{}
	
	for _, target := range targets {
		if e.isEligible(workload, target) {
			eligible = append(eligible, target)
		}
	}
	
	return eligible
}

// isEligible checks if a target cluster can accept the workload.
// It verifies resource capacity and taint tolerations.
func (e *Engine) isEligible(workload Workload, target ClusterTarget) bool {
	// Check CPU capacity
	if workload.Spec.Resources.CPU > target.Available.CPU {
		return false
	}
	
	// Check memory capacity
	if workload.Spec.Resources.Memory > target.Available.Memory {
		return false
	}
	
	// Check taint tolerations
	for _, taint := range target.Taints {
		if !hasToleration(workload.Spec.Tolerations, taint) {
			return false
		}
	}
	
	return true
}

// selectCandidates selects the top scoring clusters up to the replica count.
// It ensures we don't select more clusters than needed for the workload.
func (e *Engine) selectCandidates(scores []ScoredTarget, replicas int32) []string {
	selected := []string{}
	
	// Select up to replica count or available clusters, whichever is smaller
	maxSelection := int(replicas)
	if len(scores) < maxSelection {
		maxSelection = len(scores)
	}
	
	for i := 0; i < maxSelection; i++ {
		selected = append(selected, scores[i].Target.Name)
	}
	
	return selected
}

// RegisterAlgorithm registers a scheduling algorithm with the engine.
// This follows KCP's plugin pattern for extensibility.
func (e *Engine) RegisterAlgorithm(name string, algorithm Algorithm) {
	e.algorithms[name] = algorithm
}

// GetAlgorithm retrieves a registered algorithm by name.
func (e *Engine) GetAlgorithm(name string) (Algorithm, bool) {
	algorithm, ok := e.algorithms[name]
	return algorithm, ok
}

// ListAlgorithms returns the names of all registered algorithms.
func (e *Engine) ListAlgorithms() []string {
	names := make([]string, 0, len(e.algorithms))
	for name := range e.algorithms {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ValidateWorkload validates that a workload can be scheduled.
// It checks for basic requirements like resource specifications.
func (e *Engine) ValidateWorkload(workload Workload) error {
	if workload.Name == "" {
		return fmt.Errorf("workload name cannot be empty")
	}
	
	if workload.Spec.Replicas <= 0 {
		return fmt.Errorf("workload replicas must be greater than 0")
	}
	
	if workload.Spec.Resources.CPU <= 0 && workload.Spec.Resources.Memory <= 0 {
		return fmt.Errorf("workload must specify at least one resource requirement")
	}
	
	return nil
}

// GetScorer returns the scorer instance for advanced use cases.
func (e *Engine) GetScorer() *Scorer {
	return e.scorer
}