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

// Scorer provides scoring utilities for placement algorithms.
type Scorer struct {
	weights map[string]float64
}

// NewScorer creates a new scorer with default weights.
func NewScorer() *Scorer {
	return &Scorer{
		weights: map[string]float64{
			"capacity":  0.3,
			"latency":   0.2,
			"cost":      0.2,
			"affinity":  0.2,
			"spread":    0.1,
		},
	}
}

// CombineScores combines multiple scoring factors using weighted average.
func (s *Scorer) CombineScores(scores map[string]float64) float64 {
	total := 0.0
	weightSum := 0.0

	for factor, score := range scores {
		if weight, ok := s.weights[factor]; ok {
			total += score * weight
			weightSum += weight
		}
	}

	if weightSum == 0 {
		return 0
	}

	return total / weightSum
}

// NormalizeScore normalizes a score to 0-100 range.
func (s *Scorer) NormalizeScore(value, min, max float64) float64 {
	if max == min {
		return 50.0
	}

	normalized := (value - min) / (max - min) * 100

	if normalized < 0 {
		return 0
	}
	if normalized > 100 {
		return 100
	}

	return normalized
}

// CalculateDistance calculates geographical distance for latency scoring.
// This is a simplified implementation for demonstration purposes.
func (s *Scorer) CalculateDistance(region1, region2 string) float64 {
	// Simplified distance calculation based on predefined regions
	distances := map[string]map[string]float64{
		"us-west-2": {
			"us-west-2": 0,
			"us-east-1": 100,
			"eu-west-1": 200,
		},
		"us-east-1": {
			"us-west-2": 100,
			"us-east-1": 0,
			"eu-west-1": 150,
		},
		"eu-west-1": {
			"us-west-2": 200,
			"us-east-1": 150,
			"eu-west-1": 0,
		},
	}

	if regionMap, ok := distances[region1]; ok {
		if distance, ok := regionMap[region2]; ok {
			return distance
		}
	}

	return 100.0 // Default distance for unknown regions
}

// CalculateCapacityScore calculates a score based on available capacity.
func (s *Scorer) CalculateCapacityScore(required, available, total int64) float64 {
	if available < required {
		return 0.0 // Not enough capacity
	}

	if total == 0 {
		return 0.0
	}

	// Score based on remaining capacity after allocation
	remainingAfter := available - required
	utilizationAfter := float64(total-remainingAfter) / float64(total)

	// Prefer moderate utilization (not too high, not too low)
	if utilizationAfter < 0.2 {
		return 30.0 + (utilizationAfter/0.2)*20.0
	} else if utilizationAfter < 0.8 {
		return 50.0 + ((utilizationAfter-0.2)/0.6)*40.0
	} else {
		return 90.0 - ((utilizationAfter-0.8)/0.2)*40.0
	}
}

