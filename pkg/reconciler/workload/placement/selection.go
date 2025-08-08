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
	"sort"

	workloadv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/workload/v1alpha1"
)

// selectLocationsByStrategy applies the configured selection strategy to choose
// the final set of locations for placement.
func (e *DecisionEngine) selectLocationsByStrategy(
	placement *workloadv1alpha1.Placement,
	candidates []*LocationCandidate,
	config DecisionConfig,
) []*LocationCandidate {

	// Determine desired cluster count
	desiredCount := e.getDesiredClusterCount(placement, len(candidates))
	if desiredCount >= len(candidates) {
		return candidates
	}

	// Sort by score (highest first)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})

	switch config.Strategy {
	case SelectionStrategyBalanced:
		return e.selectBalanced(candidates, desiredCount)
	case SelectionStrategyPacked:
		return e.selectPacked(candidates, desiredCount)
	case SelectionStrategySpread:
		return e.selectSpread(candidates, desiredCount)
	case SelectionStrategyScore:
		return e.selectByScore(candidates, desiredCount)
	default:
		// Default to score-based selection
		return e.selectByScore(candidates, desiredCount)
	}
}

// selectBalanced selects locations to achieve balanced distribution.
func (e *DecisionEngine) selectBalanced(candidates []*LocationCandidate, count int) []*LocationCandidate {
	// For balanced strategy, try to distribute across different zones/regions
	selected := make([]*LocationCandidate, 0, count)
	usedZones := make(map[string]bool)
	usedRegions := make(map[string]bool)

	// First pass: select one from each unique zone
	for _, candidate := range candidates {
		if len(selected) >= count {
			break
		}

		zone := candidate.Location.Labels["topology.kubernetes.io/zone"]
		if zone != "" && !usedZones[zone] {
			selected = append(selected, candidate)
			usedZones[zone] = true
			continue
		}
	}

	// Second pass: select one from each unique region not yet covered
	for _, candidate := range candidates {
		if len(selected) >= count {
			break
		}

		region := candidate.Location.Labels["topology.kubernetes.io/region"]
		if region != "" && !usedRegions[region] {
			alreadySelected := false
			for _, sel := range selected {
				if sel.Location.Name == candidate.Location.Name {
					alreadySelected = true
					break
				}
			}
			if !alreadySelected {
				selected = append(selected, candidate)
				usedRegions[region] = true
			}
		}
	}

	// Third pass: fill remaining slots with highest scoring candidates
	for _, candidate := range candidates {
		if len(selected) >= count {
			break
		}

		alreadySelected := false
		for _, sel := range selected {
			if sel.Location.Name == candidate.Location.Name {
				alreadySelected = true
				break
			}
		}
		if !alreadySelected {
			selected = append(selected, candidate)
		}
	}

	return selected
}

// selectPacked selects locations to concentrate workloads on fewer clusters.
func (e *DecisionEngine) selectPacked(candidates []*LocationCandidate, count int) []*LocationCandidate {
	// For packed strategy, prefer locations in same region/zone
	selected := make([]*LocationCandidate, 0, count)

	// Group by region, then by zone
	regionGroups := make(map[string][]*LocationCandidate)
	for _, candidate := range candidates {
		region := candidate.Location.Labels["topology.kubernetes.io/region"]
		if region == "" {
			region = "unknown"
		}
		regionGroups[region] = append(regionGroups[region], candidate)
	}

	// Select from the region with the most high-scoring candidates
	var bestRegion string
	var bestScore int32
	for region, regionCandidates := range regionGroups {
		totalScore := int32(0)
		for _, candidate := range regionCandidates {
			totalScore += candidate.Score
		}
		avgScore := totalScore / int32(len(regionCandidates))
		if avgScore > bestScore {
			bestScore = avgScore
			bestRegion = region
		}
	}

	// Select from the best region first
	regionCandidates := regionGroups[bestRegion]
	for i := 0; i < count && i < len(regionCandidates); i++ {
		selected = append(selected, regionCandidates[i])
	}

	return selected
}

// selectSpread selects locations to maximize geographic spread.
func (e *DecisionEngine) selectSpread(candidates []*LocationCandidate, count int) []*LocationCandidate {
	// Similar to balanced but prioritizes maximum geographic diversity
	selected := make([]*LocationCandidate, 0, count)
	usedRegions := make(map[string]bool)

	// First pass: one per region
	for _, candidate := range candidates {
		if len(selected) >= count {
			break
		}

		region := candidate.Location.Labels["topology.kubernetes.io/region"]
		if region == "" {
			region = candidate.Location.Name // Use location name as fallback
		}

		if !usedRegions[region] {
			selected = append(selected, candidate)
			usedRegions[region] = true
		}
	}

	// If we still need more, select highest scoring remaining candidates
	for _, candidate := range candidates {
		if len(selected) >= count {
			break
		}

		alreadySelected := false
		for _, sel := range selected {
			if sel.Location.Name == candidate.Location.Name {
				alreadySelected = true
				break
			}
		}
		if !alreadySelected {
			selected = append(selected, candidate)
		}
	}

	return selected
}

// selectByScore selects locations purely by highest score.
func (e *DecisionEngine) selectByScore(candidates []*LocationCandidate, count int) []*LocationCandidate {
	// Simply return top N candidates by score (already sorted)
	if count >= len(candidates) {
		return candidates
	}
	return candidates[:count]
}

// getDesiredClusterCount determines the target number of clusters for placement.
func (e *DecisionEngine) getDesiredClusterCount(
	placement *workloadv1alpha1.Placement,
	availableCount int,
) int {
	if placement.Spec.NumberOfClusters != nil {
		desired := int(*placement.Spec.NumberOfClusters)
		if desired > availableCount {
			return availableCount
		}
		return desired
	}

	// Default: place on 1 cluster if not specified
	if availableCount > 0 {
		return 1
	}
	return 0
}

// createPlacementDecisions converts location candidates to placement decisions.
func (e *DecisionEngine) createPlacementDecisions(
	candidates []*LocationCandidate,
) []workloadv1alpha1.PlacementDecision {
	
	var decisions []workloadv1alpha1.PlacementDecision

	for _, candidate := range candidates {
		decision := workloadv1alpha1.PlacementDecision{
			ClusterName: fmt.Sprintf("cluster-%s", candidate.Location.Name),
			Location:    candidate.Location.Name,
			Reason:      fmt.Sprintf("Advanced placement decision (score: %d)", candidate.Score),
			Score:       &candidate.Score,
		}

		// Add detailed reasons if available
		if len(candidate.Reasons) > 0 {
			for j, reason := range candidate.Reasons {
				if j == 0 {
					decision.Reason += fmt.Sprintf(": %s", reason)
				} else {
					decision.Reason += fmt.Sprintf(", %s", reason)
				}
			}
		}

		decisions = append(decisions, decision)
	}

	return decisions
}