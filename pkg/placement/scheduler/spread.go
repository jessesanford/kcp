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

// SpreadAlgorithm implements spread placement strategy for high availability.
// It distributes workloads across different failure domains (zones, regions, etc.)
// to minimize the impact of infrastructure failures on application availability.
type SpreadAlgorithm struct {
	// spreadKey is the label key used to identify spread domains (e.g., "zone", "region")
	spreadKey string
}

// NewSpreadAlgorithm creates a new spread algorithm with default zone spreading.
// By default, it spreads workloads across availability zones for resilience.
func NewSpreadAlgorithm() *SpreadAlgorithm {
	return &SpreadAlgorithm{
		spreadKey: "zone",
	}
}

// NewSpreadAlgorithmWithKey creates a spread algorithm with custom spread key.
// This allows spreading across different dimensions like regions, datacenters, etc.
func NewSpreadAlgorithmWithKey(spreadKey string) *SpreadAlgorithm {
	return &SpreadAlgorithm{
		spreadKey: spreadKey,
	}
}

// Score scores cluster targets for spread placement.
// Higher scores indicate better candidates for spreading (less populated domains).
//
// The algorithm analyzes the current distribution of targets across spread domains
// and prefers domains with fewer existing placements to achieve balanced distribution.
func (a *SpreadAlgorithm) Score(ctx context.Context, workload Workload, 
	targets []ClusterTarget) ([]ScoredTarget, error) {
	
	// Count existing targets per spread domain
	distribution := a.getDistribution(targets)
	
	scores := []ScoredTarget{}
	
	for _, target := range targets {
		score := a.calculateScore(target, distribution)
		
		spreadValue := target.Labels[a.spreadKey]
		reason := fmt.Sprintf("spread scoring across %s=%s", a.spreadKey, spreadValue)
		
		scores = append(scores, ScoredTarget{
			Target: target,
			Score:  score,
			Reason: reason,
			Details: map[string]float64{
				"spread_domain_count": float64(distribution[spreadValue]),
				"spread_key":          1.0, // Indicator that spread key exists
			},
		})
	}
	
	return scores, nil
}

// getDistribution analyzes current cluster distribution across spread domains.
// It counts how many clusters are in each domain to inform spread decisions.
func (a *SpreadAlgorithm) getDistribution(targets []ClusterTarget) map[string]int {
	distribution := make(map[string]int)
	
	for _, target := range targets {
		key := target.Labels[a.spreadKey]
		if key != "" {
			distribution[key]++
		} else {
			// Count targets without spread key as "unknown" domain
			distribution["unknown"]++
		}
	}
	
	return distribution
}

// calculateScore calculates the spread score for a target cluster.
// It prefers clusters in domains with fewer existing clusters for better distribution.
func (a *SpreadAlgorithm) calculateScore(target ClusterTarget, 
	distribution map[string]int) float64 {
	
	key := target.Labels[a.spreadKey]
	if key == "" {
		key = "unknown"
	}
	
	count := distribution[key]
	
	// Find maximum count across all domains
	maxCount := 0
	for _, c := range distribution {
		if c > maxCount {
			maxCount = c
		}
	}
	
	// Handle edge cases
	if maxCount == 0 {
		return 100.0 // All domains empty, perfect score
	}
	
	if count == 0 {
		return 100.0 // Empty domain, highest priority
	}
	
	// Inverse scoring: fewer existing clusters = higher score
	// This encourages spreading across less populated domains
	score := float64(maxCount-count+1) / float64(maxCount+1) * 100
	
	// Ensure score is in valid range
	if score < 0 {
		return 0.0
	}
	if score > 100 {
		return 100.0
	}
	
	return score
}

// GetName returns the algorithm name for registration and identification.
func (a *SpreadAlgorithm) GetName() string {
	return "spread"
}

// Validate validates that the algorithm can be used with the given workload.
// Spread algorithm works with any workload but is most effective with labeled clusters.
func (a *SpreadAlgorithm) Validate(workload Workload) error {
	if a.spreadKey == "" {
		return fmt.Errorf("spread key cannot be empty")
	}
	
	return nil
}

// GetSpreadKey returns the current spread key used by the algorithm.
func (a *SpreadAlgorithm) GetSpreadKey() string {
	return a.spreadKey
}

// SetSpreadKey updates the spread key for the algorithm.
// This allows dynamic reconfiguration of the spread dimension.
func (a *SpreadAlgorithm) SetSpreadKey(key string) error {
	if key == "" {
		return fmt.Errorf("spread key cannot be empty")
	}
	
	a.spreadKey = key
	return nil
}

// AnalyzeDistribution provides insights into the current cluster distribution.
// This is useful for monitoring and debugging spread effectiveness.
func (a *SpreadAlgorithm) AnalyzeDistribution(targets []ClusterTarget) map[string]interface{} {
	distribution := a.getDistribution(targets)
	
	totalClusters := len(targets)
	uniqueDomains := len(distribution)
	
	analysis := map[string]interface{}{
		"total_clusters":  totalClusters,
		"unique_domains":  uniqueDomains,
		"distribution":    distribution,
		"spread_key":      a.spreadKey,
	}
	
	// Calculate balance metrics
	if uniqueDomains > 0 {
		idealPerDomain := float64(totalClusters) / float64(uniqueDomains)
		
		maxDeviation := 0.0
		for _, count := range distribution {
			deviation := float64(count) - idealPerDomain
			if deviation < 0 {
				deviation = -deviation
			}
			if deviation > maxDeviation {
				maxDeviation = deviation
			}
		}
		
		analysis["ideal_per_domain"] = idealPerDomain
		analysis["max_deviation"] = maxDeviation
		analysis["balance_score"] = (idealPerDomain - maxDeviation) / idealPerDomain * 100
	}
	
	return analysis
}