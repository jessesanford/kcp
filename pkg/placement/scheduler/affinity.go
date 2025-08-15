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
	"strings"
)

// AffinityAlgorithm implements affinity and anti-affinity based placement.
// It evaluates cluster selection based on workload affinity preferences and requirements,
// supporting both node affinity and pod anti-affinity concepts adapted for cluster placement.
type AffinityAlgorithm struct {
	// affinityWeight controls the influence of positive affinity rules
	affinityWeight float64
	
	// antiAffinityWeight controls the influence of negative affinity rules
	antiAffinityWeight float64
}

// NewAffinityAlgorithm creates a new affinity algorithm with balanced weights.
// Both affinity and anti-affinity rules are weighted equally by default.
func NewAffinityAlgorithm() *AffinityAlgorithm {
	return &AffinityAlgorithm{
		affinityWeight:     1.0,
		antiAffinityWeight: 1.0,
	}
}

// NewAffinityAlgorithmWithWeights creates an affinity algorithm with custom weights.
// This allows fine-tuning the relative importance of affinity vs anti-affinity rules.
func NewAffinityAlgorithmWithWeights(affinityWeight, antiAffinityWeight float64) *AffinityAlgorithm {
	return &AffinityAlgorithm{
		affinityWeight:     affinityWeight,
		antiAffinityWeight: antiAffinityWeight,
	}
}

// Score scores cluster targets based on affinity and anti-affinity rules.
// It evaluates both positive preferences (affinity) and negative constraints (anti-affinity).
func (a *AffinityAlgorithm) Score(ctx context.Context, workload Workload, 
	targets []ClusterTarget) ([]ScoredTarget, error) {
	
	scores := []ScoredTarget{}
	
	for _, target := range targets {
		score := a.calculateScore(workload, target, targets)
		reason := fmt.Sprintf("affinity scoring: affinity weight=%.2f, anti-affinity weight=%.2f", 
			a.affinityWeight, a.antiAffinityWeight)
		
		scores = append(scores, ScoredTarget{
			Target: target,
			Score:  score,
			Reason: reason,
			Details: a.getScoreDetails(workload, target, targets),
		})
	}
	
	return scores, nil
}

// calculateScore computes the overall affinity score for a target cluster.
// It combines node affinity and anti-affinity evaluations with configured weights.
func (a *AffinityAlgorithm) calculateScore(workload Workload, 
	target ClusterTarget, allTargets []ClusterTarget) float64 {
	
	baseScore := 50.0 // Neutral starting score
	
	// Evaluate affinity rules if present
	if workload.Spec.Affinity != nil {
		if workload.Spec.Affinity.NodeAffinity != nil {
			affinityScore := a.evaluateNodeAffinity(workload.Spec.Affinity.NodeAffinity, target.Labels)
			baseScore += affinityScore * a.affinityWeight
		}
		
		// Evaluate anti-affinity rules if present
		if workload.Spec.Affinity.AntiAffinity != nil {
			antiAffinityPenalty := a.evaluateAntiAffinity(workload.Spec.Affinity.AntiAffinity, target, allTargets)
			baseScore -= antiAffinityPenalty * a.antiAffinityWeight
		}
	}
	
	// Normalize score to 0-100 range
	if baseScore < 0 {
		baseScore = 0
	}
	if baseScore > 100 {
		baseScore = 100
	}
	
	return baseScore
}

// evaluateNodeAffinity evaluates node affinity rules against target labels.
// It handles both required and preferred affinity terms with appropriate scoring.
func (a *AffinityAlgorithm) evaluateNodeAffinity(affinity *NodeAffinity, 
	targetLabels map[string]string) float64 {
	
	score := 0.0
	
	// Evaluate required affinity terms (hard constraints)
	for _, requirement := range affinity.RequiredDuringScheduling {
		if a.matchesRequirement(requirement, targetLabels) {
			score += 50.0 // High score for meeting required constraints
		} else {
			score -= 25.0 // Penalty for not meeting required constraints
		}
	}
	
	// Evaluate preferred affinity terms (soft preferences)
	for _, preference := range affinity.PreferredDuringScheduling {
		if a.matchesRequirement(preference.Preference, targetLabels) {
			// Weight-based scoring for preferences
			score += float64(preference.Weight)
		}
	}
	
	return score
}

// evaluateAntiAffinity evaluates anti-affinity rules to avoid cluster co-location.
// It penalizes placement on clusters that violate anti-affinity constraints.
func (a *AffinityAlgorithm) evaluateAntiAffinity(antiAffinity *AntiAffinity, 
	target ClusterTarget, allTargets []ClusterTarget) float64 {
	
	penalty := 0.0
	
	// Check anti-affinity requirements against other clusters
	for _, requirement := range antiAffinity.RequiredDuringScheduling {
		if a.violatesAntiAffinity(requirement, target, allTargets) {
			penalty += 30.0 // Significant penalty for anti-affinity violations
		}
	}
	
	return penalty
}

// matchesRequirement checks if target labels satisfy an affinity requirement.
// It supports simple label matching with key=value syntax.
func (a *AffinityAlgorithm) matchesRequirement(requirement string, labels map[string]string) bool {
	// Simple implementation: support "key=value" format
	if strings.Contains(requirement, "=") {
		parts := strings.SplitN(requirement, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			return labels[key] == value
		}
	}
	
	// Support "key" format (checking key existence)
	key := strings.TrimSpace(requirement)
	_, exists := labels[key]
	return exists
}

// violatesAntiAffinity checks if placement would violate anti-affinity rules.
// It examines whether the target cluster conflicts with anti-affinity constraints.
func (a *AffinityAlgorithm) violatesAntiAffinity(requirement string, 
	target ClusterTarget, allTargets []ClusterTarget) bool {
	
	// For cluster-level anti-affinity, we check if the target matches
	// the anti-affinity requirement (which we want to avoid)
	return a.matchesRequirement(requirement, target.Labels)
}

// getScoreDetails provides detailed scoring breakdown for debugging and monitoring.
func (a *AffinityAlgorithm) getScoreDetails(workload Workload, 
	target ClusterTarget, allTargets []ClusterTarget) map[string]float64 {
	
	details := map[string]float64{
		"base_score": 50.0,
	}
	
	if workload.Spec.Affinity != nil {
		if workload.Spec.Affinity.NodeAffinity != nil {
			details["node_affinity_score"] = a.evaluateNodeAffinity(workload.Spec.Affinity.NodeAffinity, target.Labels)
		}
		
		if workload.Spec.Affinity.AntiAffinity != nil {
			details["anti_affinity_penalty"] = a.evaluateAntiAffinity(workload.Spec.Affinity.AntiAffinity, target, allTargets)
		}
	}
	
	return details
}

// GetName returns the algorithm name for registration and identification.
func (a *AffinityAlgorithm) GetName() string {
	return "affinity"
}

// Validate validates that the algorithm can be used with the given workload.
// Affinity algorithm works best with workloads that have affinity specifications.
func (a *AffinityAlgorithm) Validate(workload Workload) error {
	if a.affinityWeight < 0 || a.antiAffinityWeight < 0 {
		return fmt.Errorf("affinity weights must be non-negative")
	}
	
	return nil
}

// SetWeights updates the algorithm weights for affinity and anti-affinity.
// This allows dynamic reconfiguration of the algorithm behavior.
func (a *AffinityAlgorithm) SetWeights(affinityWeight, antiAffinityWeight float64) error {
	if affinityWeight < 0 || antiAffinityWeight < 0 {
		return fmt.Errorf("weights must be non-negative")
	}
	
	a.affinityWeight = affinityWeight
	a.antiAffinityWeight = antiAffinityWeight
	
	return nil
}

// GetWeights returns the current algorithm weights.
func (a *AffinityAlgorithm) GetWeights() (affinityWeight, antiAffinityWeight float64) {
	return a.affinityWeight, a.antiAffinityWeight
}