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

package placement

import (
	"fmt"

	workloadv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/workload/v1alpha1"
)

// scoreLocationCandidates assigns scores to all location candidates based on
// multiple criteria weighted according to the scoring configuration.
func (e *DecisionEngine) scoreLocationCandidates(
	placement *workloadv1alpha1.Placement,
	candidates []*LocationCandidate,
	weights ScoringWeights,
) error {

	logger := e.logger.WithValues("placement", placement.Name)
	
	for _, candidate := range candidates {
		// Calculate individual scoring components
		candidate.Details.AffinityScore = e.calculateAffinityScore(placement, candidate.Location)
		candidate.Details.CapacityScore = e.calculateCapacityScore(candidate.Location)
		candidate.Details.SpreadScore = e.calculateSpreadScore(candidate.Location)
		candidate.Details.LatencyScore = e.calculateLatencyScore(placement, candidate.Location)

		// Combine weighted scores
		candidate.Score = e.calculateWeightedScore(candidate.Details, weights)
		
		// Generate scoring reasons
		candidate.Reasons = e.generateScoringReasons(candidate.Details, weights)

		logger.V(6).Info("scored location candidate",
			"location", candidate.Location.Name,
			"score", candidate.Score,
			"affinity", candidate.Details.AffinityScore,
			"capacity", candidate.Details.CapacityScore,
			"spread", candidate.Details.SpreadScore,
			"latency", candidate.Details.LatencyScore,
		)
	}

	return nil
}

// calculateAffinityScore evaluates how well a location matches affinity preferences.
func (e *DecisionEngine) calculateAffinityScore(
	placement *workloadv1alpha1.Placement,
	location *workloadv1alpha1.Location,
) int32 {
	
	score := int32(50) // Base score

	constraints := placement.Spec.Constraints
	if constraints == nil || constraints.Affinity == nil {
		return score
	}

	affinity := constraints.Affinity

	// Score preferred affinity terms
	if affinity.PreferredDuringSchedulingIgnoredDuringExecution != nil {
		totalWeight := int32(0)
		matchedWeight := int32(0)

		for _, preferred := range affinity.PreferredDuringSchedulingIgnoredDuringExecution {
			totalWeight += preferred.Weight
			
			for _, term := range preferred.Preference.NodeSelectorTerms {
				if e.nodeSelectionTermMatches(location, term) {
					matchedWeight += preferred.Weight
					break
				}
			}
		}

		if totalWeight > 0 {
			// Scale to 0-100 range based on preference matching
			affinityPercent := (matchedWeight * 100) / totalWeight
			score = (score + affinityPercent) / 2 // Blend with base score
		}
	}

	return score
}

// calculateCapacityScore evaluates available resource capacity at a location.
func (e *DecisionEngine) calculateCapacityScore(location *workloadv1alpha1.Location) int32 {
	// For now, return a mock score based on location properties
	// Real implementation would query actual cluster resource usage
	
	score := int32(75) // Default good capacity score

	// Check for capacity-related annotations or labels
	if capacity, exists := location.Annotations["workload.kcp.io/capacity"]; exists {
		switch capacity {
		case "high":
			score = 90
		case "medium":
			score = 75
		case "low":
			score = 40
		case "full":
			score = 10
		}
	}

	return score
}

// calculateSpreadScore evaluates how placing on this location affects workload spread.
func (e *DecisionEngine) calculateSpreadScore(location *workloadv1alpha1.Location) int32 {
	// For now, return a base spread score
	// Real implementation would consider existing workload distribution
	
	score := int32(60) // Base spread score

	// Prefer locations with spread-encouraging labels
	if zone, exists := location.Labels["topology.kubernetes.io/zone"]; exists {
		// Different zones get higher spread scores
		// This is a simple hash-based approach for demo
		zoneHash := int32(len(zone)) % 30
		score += zoneHash
	}

	if region, exists := location.Labels["topology.kubernetes.io/region"]; exists {
		// Different regions get spread bonuses
		regionHash := int32(len(region)) % 20
		score += regionHash
	}

	// Ensure score stays in valid range
	if score > 100 {
		score = 100
	}

	return score
}

// calculateLatencyScore evaluates network latency considerations for this location.
func (e *DecisionEngine) calculateLatencyScore(
	placement *workloadv1alpha1.Placement,
	location *workloadv1alpha1.Location,
) int32 {
	
	score := int32(70) // Default network score

	// Check for latency-related annotations
	if latency, exists := location.Annotations["workload.kcp.io/network-latency"]; exists {
		switch latency {
		case "very-low":
			score = 95
		case "low":
			score = 85
		case "medium":
			score = 70
		case "high":
			score = 45
		case "very-high":
			score = 20
		}
	}

	return score
}

// calculateWeightedScore combines individual scores using the specified weights.
func (e *DecisionEngine) calculateWeightedScore(
	details ScoringDetails,
	weights ScoringWeights,
) int32 {
	
	totalWeight := weights.LocationAffinity + weights.ResourceCapacity + 
				   weights.WorkloadSpread + weights.NetworkLatency
	
	if totalWeight == 0 {
		return 50 // Default score if no weights configured
	}

	weightedSum := (details.AffinityScore * weights.LocationAffinity) +
				   (details.CapacityScore * weights.ResourceCapacity) +
				   (details.SpreadScore * weights.WorkloadSpread) +
				   (details.LatencyScore * weights.NetworkLatency)

	return weightedSum / totalWeight
}

// generateScoringReasons creates human-readable explanations for the scoring.
func (e *DecisionEngine) generateScoringReasons(
	details ScoringDetails,
	weights ScoringWeights,
) []string {
	
	var reasons []string

	if weights.LocationAffinity > 0 {
		reasons = append(reasons, fmt.Sprintf("affinity score: %d", details.AffinityScore))
	}
	
	if weights.ResourceCapacity > 0 {
		reasons = append(reasons, fmt.Sprintf("capacity score: %d", details.CapacityScore))
	}
	
	if weights.WorkloadSpread > 0 {
		reasons = append(reasons, fmt.Sprintf("spread score: %d", details.SpreadScore))
	}
	
	if weights.NetworkLatency > 0 {
		reasons = append(reasons, fmt.Sprintf("latency score: %d", details.LatencyScore))
	}

	return reasons
}