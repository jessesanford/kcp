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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSchedulingEngine(t *testing.T) {
	ctx := context.Background()
	engine := NewEngine()

	workload := Workload{
		Name: "test-workload",
		Spec: WorkloadSpec{
			Replicas: 2,
			Resources: ResourceRequirements{
				CPU:    10,
				Memory: 100,
			},
		},
	}

	targets := []ClusterTarget{
		{
			Name: "cluster-1",
			Available: Resources{
				CPU:    50,
				Memory: 500,
			},
			Capacity: Resources{
				CPU:    100,
				Memory: 1000,
			},
			Labels: map[string]string{
				"zone": "us-west-2a",
			},
		},
		{
			Name: "cluster-2",
			Available: Resources{
				CPU:    80,
				Memory: 800,
			},
			Capacity: Resources{
				CPU:    100,
				Memory: 1000,
			},
			Labels: map[string]string{
				"zone": "us-west-2b",
			},
		},
		{
			Name: "cluster-3",
			Available: Resources{
				CPU:    5, // Not enough capacity
				Memory: 50,
			},
			Capacity: Resources{
				CPU:    100,
				Memory: 1000,
			},
			Labels: map[string]string{
				"zone": "us-west-2c",
			},
		},
	}

	tests := []struct {
		name         string
		strategy     string
		expectedLen  int
		expectError  bool
	}{
		{
			name:        "bin packing strategy",
			strategy:    "binpack",
			expectedLen: 2,
			expectError: false,
		},
		{
			name:        "spread strategy",
			strategy:    "spread",
			expectedLen: 2,
			expectError: false,
		},
		{
			name:        "affinity strategy",
			strategy:    "affinity",
			expectedLen: 2,
			expectError: false,
		},
		{
			name:        "unknown strategy",
			strategy:    "unknown",
			expectedLen: 0,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision, err := engine.Schedule(ctx, workload, targets, tt.strategy)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, decision)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, decision)

			assert.Equal(t, tt.strategy, decision.Strategy)
			assert.Equal(t, workload.Name, decision.WorkloadName)
			assert.Len(t, decision.Clusters, tt.expectedLen)

			// Verify selected clusters are eligible (not cluster-3 due to insufficient capacity)
			for _, cluster := range decision.Clusters {
				assert.NotEqual(t, "cluster-3", cluster)
			}
		})
	}
}

func TestBinPackAlgorithm(t *testing.T) {
	ctx := context.Background()
	algo := NewBinPackAlgorithm()

	workload := Workload{
		Spec: WorkloadSpec{
			Resources: ResourceRequirements{
				CPU:    20,
				Memory: 200,
			},
		},
	}

	targets := []ClusterTarget{
		{
			Name:      "high-utilization",
			Available: Resources{CPU: 30, Memory: 300},
			Capacity:  Resources{CPU: 100, Memory: 1000},
		},
		{
			Name:      "low-utilization",
			Available: Resources{CPU: 90, Memory: 900},
			Capacity:  Resources{CPU: 100, Memory: 1000},
		},
	}

	scores, err := algo.Score(ctx, workload, targets)
	require.NoError(t, err)
	require.Len(t, scores, 2)

	// Bin packing should prefer the cluster that would achieve higher utilization
	var highUtilScore, lowUtilScore float64
	for _, score := range scores {
		if score.Target.Name == "high-utilization" {
			highUtilScore = score.Score
		} else if score.Target.Name == "low-utilization" {
			lowUtilScore = score.Score
		}
	}

	// High utilization cluster should score higher for bin packing
	assert.Greater(t, highUtilScore, lowUtilScore)
	assert.Contains(t, scores[0].Reason, "bin-pack scoring")
}

func TestSpreadAlgorithm(t *testing.T) {
	ctx := context.Background()
	algo := NewSpreadAlgorithm()

	workload := Workload{}

	targets := []ClusterTarget{
		{
			Name:   "cluster-zone-a-1",
			Labels: map[string]string{"zone": "a"},
		},
		{
			Name:   "cluster-zone-b",
			Labels: map[string]string{"zone": "b"},
		},
		{
			Name:   "cluster-zone-a-2",
			Labels: map[string]string{"zone": "a"},
		},
	}

	scores, err := algo.Score(ctx, workload, targets)
	require.NoError(t, err)
	require.Len(t, scores, 3)

	// Find scores for each zone
	var zoneBScore, zoneAScore float64
	for _, score := range scores {
		zone := score.Target.Labels["zone"]
		if zone == "b" {
			zoneBScore = score.Score
		} else if zone == "a" && zoneAScore == 0 {
			zoneAScore = score.Score // Take first zone A score
		}
	}

	// Zone B should score higher as it has fewer clusters (1 vs 2)
	assert.Greater(t, zoneBScore, zoneAScore)
	assert.Contains(t, scores[0].Reason, "spread scoring")
}

func TestAffinityAlgorithm(t *testing.T) {
	ctx := context.Background()
	algo := NewAffinityAlgorithm()

	workload := Workload{
		Spec: WorkloadSpec{
			Affinity: &Affinity{
				NodeAffinity: &NodeAffinity{
					RequiredDuringScheduling: []string{"zone=us-west"},
					PreferredDuringScheduling: []WeightedPreference{
						{Weight: 10, Preference: "instance=compute-optimized"},
					},
				},
			},
		},
	}

	targets := []ClusterTarget{
		{
			Name: "matching-cluster",
			Labels: map[string]string{
				"zone":     "us-west",
				"instance": "compute-optimized",
			},
		},
		{
			Name: "partial-match",
			Labels: map[string]string{
				"zone": "us-west",
			},
		},
		{
			Name: "no-match",
			Labels: map[string]string{
				"zone": "us-east",
			},
		},
	}

	scores, err := algo.Score(ctx, workload, targets)
	require.NoError(t, err)
	require.Len(t, scores, 3)

	// Find scores for each cluster
	scoreMap := make(map[string]float64)
	for _, score := range scores {
		scoreMap[score.Target.Name] = score.Score
	}

	// Matching cluster should score highest
	assert.Greater(t, scoreMap["matching-cluster"], scoreMap["partial-match"])
	assert.Greater(t, scoreMap["partial-match"], scoreMap["no-match"])
}

func TestScorer(t *testing.T) {
	scorer := NewScorer()

	t.Run("combine scores", func(t *testing.T) {
		scores := map[string]float64{
			"capacity": 80.0,
			"latency":  60.0,
			"cost":     70.0,
		}

		combined := scorer.CombineScores(scores)
		assert.Greater(t, combined, 0.0)
		assert.LessOrEqual(t, combined, 100.0)
	})

	t.Run("normalize score", func(t *testing.T) {
		normalized := scorer.NormalizeScore(5, 0, 10)
		assert.Equal(t, 50.0, normalized)

		// Test edge cases
		assert.Equal(t, 50.0, scorer.NormalizeScore(5, 5, 5)) // min == max
		assert.Equal(t, 0.0, scorer.NormalizeScore(-5, 0, 10)) // below min
		assert.Equal(t, 100.0, scorer.NormalizeScore(15, 0, 10)) // above max
	})

	t.Run("calculate distance", func(t *testing.T) {
		distance := scorer.CalculateDistance("us-west-2", "us-east-1")
		assert.Equal(t, 100.0, distance)

		sameDistance := scorer.CalculateDistance("us-west-2", "us-west-2")
		assert.Equal(t, 0.0, sameDistance)

		unknownDistance := scorer.CalculateDistance("unknown", "unknown")
		assert.Equal(t, 100.0, unknownDistance)
	})

	t.Run("calculate capacity score", func(t *testing.T) {
		// Test sufficient capacity
		score := scorer.CalculateCapacityScore(10, 50, 100)
		assert.Greater(t, score, 0.0)

		// Test insufficient capacity
		score = scorer.CalculateCapacityScore(60, 50, 100)
		assert.Equal(t, 0.0, score)

		// Test zero total capacity
		score = scorer.CalculateCapacityScore(10, 50, 0)
		assert.Equal(t, 0.0, score)
	})
}

func TestEngineValidation(t *testing.T) {
	engine := NewEngine()

	tests := []struct {
		name        string
		workload    Workload
		expectError bool
	}{
		{
			name: "valid workload",
			workload: Workload{
				Name: "test",
				Spec: WorkloadSpec{
					Replicas: 1,
					Resources: ResourceRequirements{
						CPU: 10,
					},
				},
			},
			expectError: false,
		},
		{
			name: "empty name",
			workload: Workload{
				Name: "",
				Spec: WorkloadSpec{
					Replicas: 1,
					Resources: ResourceRequirements{CPU: 10},
				},
			},
			expectError: true,
		},
		{
			name: "zero replicas",
			workload: Workload{
				Name: "test",
				Spec: WorkloadSpec{
					Replicas: 0,
					Resources: ResourceRequirements{CPU: 10},
				},
			},
			expectError: true,
		},
		{
			name: "no resources",
			workload: Workload{
				Name: "test",
				Spec: WorkloadSpec{
					Replicas:  1,
					Resources: ResourceRequirements{},
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := engine.ValidateWorkload(tt.workload)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestEngineAlgorithmManagement(t *testing.T) {
	engine := NewEngine()

	// Test listing algorithms
	algos := engine.ListAlgorithms()
	assert.Contains(t, algos, "binpack")
	assert.Contains(t, algos, "spread")
	assert.Contains(t, algos, "affinity")

	// Test getting algorithm
	algo, ok := engine.GetAlgorithm("binpack")
	assert.True(t, ok)
	assert.Equal(t, "binpack", algo.GetName())

	// Test unknown algorithm
	_, ok = engine.GetAlgorithm("unknown")
	assert.False(t, ok)
}