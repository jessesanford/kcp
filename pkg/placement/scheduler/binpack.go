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
)

// BinPackAlgorithm implements bin packing placement strategy.
// It aims to maximize cluster utilization by preferring clusters that
// would achieve higher resource utilization after workload placement.
// This follows the bin packing heuristic for efficient resource usage.
type BinPackAlgorithm struct {
	// cpuWeight defines the weight for CPU utilization in scoring
	cpuWeight float64
	
	// memoryWeight defines the weight for memory utilization in scoring
	memoryWeight float64
}

// NewBinPackAlgorithm creates a new bin packing algorithm with balanced weights.
// CPU and memory are weighted equally by default for comprehensive resource consideration.
func NewBinPackAlgorithm() *BinPackAlgorithm {
	return &BinPackAlgorithm{
		cpuWeight:    0.5,
		memoryWeight: 0.5,
	}
}

// NewBinPackAlgorithmWithWeights creates a bin pack algorithm with custom weights.
// This allows fine-tuning the algorithm for workloads with specific resource profiles.
func NewBinPackAlgorithmWithWeights(cpuWeight, memoryWeight float64) *BinPackAlgorithm {
	return &BinPackAlgorithm{
		cpuWeight:    cpuWeight,
		memoryWeight: memoryWeight,
	}
}

// Score scores cluster targets for bin packing placement.
// Higher scores indicate better bin packing candidates (more utilized after placement).
//
// The algorithm calculates utilization after workload placement and prefers
// clusters that would achieve higher overall utilization without overcommitting.
func (a *BinPackAlgorithm) Score(ctx context.Context, workload Workload, 
	targets []ClusterTarget) ([]ScoredTarget, error) {
	
	scores := []ScoredTarget{}
	
	for _, target := range targets {
		score := a.calculateScore(workload, target)
		reason := fmt.Sprintf("bin-pack scoring: CPU weight=%.2f, Memory weight=%.2f", 
			a.cpuWeight, a.memoryWeight)
		
		scores = append(scores, ScoredTarget{
			Target: target,
			Score:  score,
			Reason: reason,
			Details: map[string]float64{
				"cpu_utilization":    a.calculateUtilization(
					target.Capacity.CPU - target.Available.CPU + workload.Spec.Resources.CPU,
					target.Capacity.CPU,
				) * 100,
				"memory_utilization": a.calculateUtilization(
					target.Capacity.Memory - target.Available.Memory + workload.Spec.Resources.Memory,
					target.Capacity.Memory,
				) * 100,
			},
		})
	}
	
	return scores, nil
}

// calculateScore calculates the bin packing score for a target cluster.
// It considers both CPU and memory utilization after workload placement.
func (a *BinPackAlgorithm) calculateScore(workload Workload, 
	target ClusterTarget) float64 {
	
	// Calculate current utilization
	currentCPUUsed := target.Capacity.CPU - target.Available.CPU
	currentMemoryUsed := target.Capacity.Memory - target.Available.Memory
	
	// Calculate utilization after placement
	cpuUtilization := a.calculateUtilization(
		currentCPUUsed + workload.Spec.Resources.CPU,
		target.Capacity.CPU,
	)
	
	memoryUtilization := a.calculateUtilization(
		currentMemoryUsed + workload.Spec.Resources.Memory,
		target.Capacity.Memory,
	)
	
	// Weighted average - prefer higher utilization for bin packing
	// This encourages consolidation and efficient resource usage
	score := cpuUtilization*a.cpuWeight + memoryUtilization*a.memoryWeight
	
	// Scale to 0-100 range
	return score * 100
}

// calculateUtilization calculates resource utilization ratio.
// Returns a value between 0.0 and 1.0 representing utilization percentage.
func (a *BinPackAlgorithm) calculateUtilization(used, total int64) float64 {
	if total == 0 {
		return 0.0
	}
	
	utilization := float64(used) / float64(total)
	
	// Cap at 100% utilization
	if utilization > 1.0 {
		return 1.0
	}
	
	return utilization
}

// GetName returns the algorithm name for registration and identification.
func (a *BinPackAlgorithm) GetName() string {
	return "binpack"
}

// Validate validates that the algorithm can be used with the given workload.
// Bin packing requires resource specifications to make placement decisions.
func (a *BinPackAlgorithm) Validate(workload Workload) error {
	if workload.Spec.Resources.CPU == 0 && workload.Spec.Resources.Memory == 0 {
		return fmt.Errorf("workload must specify resource requirements for bin packing algorithm")
	}
	
	if a.cpuWeight < 0 || a.memoryWeight < 0 {
		return fmt.Errorf("algorithm weights must be non-negative")
	}
	
	if a.cpuWeight+a.memoryWeight == 0 {
		return fmt.Errorf("at least one resource weight must be positive")
	}
	
	return nil
}

// SetWeights updates the algorithm weights for CPU and memory.
// This allows dynamic reconfiguration of the algorithm behavior.
func (a *BinPackAlgorithm) SetWeights(cpuWeight, memoryWeight float64) error {
	if cpuWeight < 0 || memoryWeight < 0 {
		return fmt.Errorf("weights must be non-negative")
	}
	
	if cpuWeight+memoryWeight == 0 {
		return fmt.Errorf("at least one weight must be positive")
	}
	
	a.cpuWeight = cpuWeight
	a.memoryWeight = memoryWeight
	
	return nil
}

// GetWeights returns the current algorithm weights.
func (a *BinPackAlgorithm) GetWeights() (cpuWeight, memoryWeight float64) {
	return a.cpuWeight, a.memoryWeight
}