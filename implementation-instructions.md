# Implementation Instructions: Placement Scheduler (Branch 18)

## Overview
This branch implements placement scheduling algorithms including bin packing, spread, and affinity-based placement. It provides the core logic for determining optimal cluster placement based on various strategies and scoring mechanisms.

## Dependencies
- **Base**: feature/tmc-phase4-16-workspace-discovery
- **Uses**: Branch 13 (interfaces), Branch 16 (discovery)
- **Required for**: Branch 19 (controller)

## Files to Create

### 1. `pkg/placement/scheduler/engine.go` (100 lines)
Main scheduling engine implementation.

```go
package scheduler

import (
    "context"
    "fmt"
    "sort"
    "github.com/kcp-dev/kcp/pkg/placement/interfaces"
    "github.com/kcp-dev/kcp/pkg/placement/discovery"
)

// Engine implements the placement scheduling engine
type Engine struct {
    discovery  *discovery.WorkspaceTraverser
    algorithms map[string]Algorithm
    scorer     *Scorer
}

// NewEngine creates a new scheduling engine
func NewEngine(discovery *discovery.WorkspaceTraverser) *Engine {
    e := &Engine{
        discovery:  discovery,
        algorithms: make(map[string]Algorithm),
        scorer:     NewScorer(),
    }
    
    // Register default algorithms
    e.RegisterAlgorithm("binpack", NewBinPackAlgorithm())
    e.RegisterAlgorithm("spread", NewSpreadAlgorithm())
    e.RegisterAlgorithm("affinity", NewAffinityAlgorithm())
    
    return e
}

// Schedule determines placement for a workload
func (e *Engine) Schedule(ctx context.Context, workload interfaces.Workload, 
    targets []interfaces.ClusterTarget, strategy string) (*interfaces.PlacementDecision, error) {
    
    algorithm, ok := e.algorithms[strategy]
    if !ok {
        return nil, fmt.Errorf("unknown scheduling algorithm: %s", strategy)
    }
    
    // Filter eligible targets
    eligible := e.filterEligible(workload, targets)
    if len(eligible) == 0 {
        return nil, fmt.Errorf("no eligible clusters found")
    }
    
    // Score targets using the algorithm
    scores, err := algorithm.Score(ctx, workload, eligible)
    if err != nil {
        return nil, fmt.Errorf("scoring failed: %w", err)
    }
    
    // Sort by score
    sort.Slice(scores, func(i, j int) bool {
        return scores[i].Score > scores[j].Score
    })
    
    // Select top candidates based on replica count
    selected := e.selectCandidates(scores, workload.Spec.Replicas)
    
    return &interfaces.PlacementDecision{
        WorkloadName: workload.Name,
        Clusters:     selected,
        Strategy:     strategy,
    }, nil
}

// filterEligible filters targets that can accept the workload
func (e *Engine) filterEligible(workload interfaces.Workload, 
    targets []interfaces.ClusterTarget) []interfaces.ClusterTarget {
    
    eligible := []interfaces.ClusterTarget{}
    for _, target := range targets {
        if e.isEligible(workload, target) {
            eligible = append(eligible, target)
        }
    }
    return eligible
}

// isEligible checks if a target can accept the workload
func (e *Engine) isEligible(workload interfaces.Workload, target interfaces.ClusterTarget) bool {
    // Check capacity
    if workload.Spec.Resources.CPU > target.Available.CPU {
        return false
    }
    if workload.Spec.Resources.Memory > target.Available.Memory {
        return false
    }
    
    // Check taints and tolerations
    for _, taint := range target.Taints {
        if !hasToleration(workload.Spec.Tolerations, taint) {
            return false
        }
    }
    
    return true
}

// selectCandidates selects the top scoring clusters
func (e *Engine) selectCandidates(scores []ScoredTarget, replicas int32) []string {
    selected := []string{}
    for i := 0; i < int(replicas) && i < len(scores); i++ {
        selected = append(selected, scores[i].Target.Name)
    }
    return selected
}

// RegisterAlgorithm registers a scheduling algorithm
func (e *Engine) RegisterAlgorithm(name string, algorithm Algorithm) {
    e.algorithms[name] = algorithm
}
```

### 2. `pkg/placement/scheduler/algorithms/binpack.go` (100 lines)
Bin packing algorithm implementation.

```go
package algorithms

import (
    "context"
    "github.com/kcp-dev/kcp/pkg/placement/interfaces"
    "github.com/kcp-dev/kcp/pkg/placement/scheduler"
)

// BinPackAlgorithm implements bin packing placement
type BinPackAlgorithm struct {
    cpuWeight    float64
    memoryWeight float64
}

// NewBinPackAlgorithm creates a new bin pack algorithm
func NewBinPackAlgorithm() *BinPackAlgorithm {
    return &BinPackAlgorithm{
        cpuWeight:    0.5,
        memoryWeight: 0.5,
    }
}

// Score scores targets for bin packing
func (a *BinPackAlgorithm) Score(ctx context.Context, workload interfaces.Workload, 
    targets []interfaces.ClusterTarget) ([]scheduler.ScoredTarget, error) {
    
    scores := []scheduler.ScoredTarget{}
    
    for _, target := range targets {
        score := a.calculateScore(workload, target)
        scores = append(scores, scheduler.ScoredTarget{
            Target: target,
            Score:  score,
            Reason: "bin-pack scoring",
        })
    }
    
    return scores, nil
}

// calculateScore calculates bin packing score
func (a *BinPackAlgorithm) calculateScore(workload interfaces.Workload, 
    target interfaces.ClusterTarget) float64 {
    
    // Calculate utilization after placement
    cpuUtilization := a.calculateUtilization(
        target.Capacity.CPU - target.Available.CPU + workload.Spec.Resources.CPU,
        target.Capacity.CPU,
    )
    
    memoryUtilization := a.calculateUtilization(
        target.Capacity.Memory - target.Available.Memory + workload.Spec.Resources.Memory,
        target.Capacity.Memory,
    )
    
    // Weighted average (prefer higher utilization for bin packing)
    score := cpuUtilization*a.cpuWeight + memoryUtilization*a.memoryWeight
    
    return score * 100 // Scale to 0-100
}

// calculateUtilization calculates resource utilization ratio
func (a *BinPackAlgorithm) calculateUtilization(used, total int64) float64 {
    if total == 0 {
        return 0
    }
    return float64(used) / float64(total)
}

// GetName returns the algorithm name
func (a *BinPackAlgorithm) GetName() string {
    return "binpack"
}

// Validate validates the algorithm can be used
func (a *BinPackAlgorithm) Validate(workload interfaces.Workload) error {
    if workload.Spec.Resources.CPU == 0 && workload.Spec.Resources.Memory == 0 {
        return fmt.Errorf("workload must specify resource requirements for bin packing")
    }
    return nil
}
```

### 3. `pkg/placement/scheduler/algorithms/spread.go` (80 lines)
Spread algorithm for distributing workloads.

```go
package algorithms

import (
    "context"
    "github.com/kcp-dev/kcp/pkg/placement/interfaces"
    "github.com/kcp-dev/kcp/pkg/placement/scheduler"
)

// SpreadAlgorithm implements spread placement
type SpreadAlgorithm struct {
    spreadKey string // Label key to spread across
}

// NewSpreadAlgorithm creates a new spread algorithm
func NewSpreadAlgorithm() *SpreadAlgorithm {
    return &SpreadAlgorithm{
        spreadKey: "zone",
    }
}

// Score scores targets for spreading
func (a *SpreadAlgorithm) Score(ctx context.Context, workload interfaces.Workload, 
    targets []interfaces.ClusterTarget) ([]scheduler.ScoredTarget, error) {
    
    // Count existing workloads per spread key
    distribution := a.getDistribution(targets)
    
    scores := []scheduler.ScoredTarget{}
    
    for _, target := range targets {
        score := a.calculateScore(target, distribution)
        scores = append(scores, scheduler.ScoredTarget{
            Target: target,
            Score:  score,
            Reason: "spread scoring",
        })
    }
    
    return scores, nil
}

// getDistribution gets current workload distribution
func (a *SpreadAlgorithm) getDistribution(targets []interfaces.ClusterTarget) map[string]int {
    distribution := make(map[string]int)
    
    for _, target := range targets {
        key := target.Labels[a.spreadKey]
        if key != "" {
            // Count would include existing workloads in production
            distribution[key]++
        }
    }
    
    return distribution
}

// calculateScore calculates spread score
func (a *SpreadAlgorithm) calculateScore(target interfaces.ClusterTarget, 
    distribution map[string]int) float64 {
    
    key := target.Labels[a.spreadKey]
    if key == "" {
        return 50.0 // Neutral score for unknown zones
    }
    
    count := distribution[key]
    
    // Lower count = higher score (prefer less populated zones)
    if count == 0 {
        return 100.0
    }
    
    // Inverse scoring - fewer existing workloads = higher score
    maxCount := 0
    for _, c := range distribution {
        if c > maxCount {
            maxCount = c
        }
    }
    
    if maxCount == 0 {
        return 100.0
    }
    
    score := float64(maxCount-count) / float64(maxCount) * 100
    return score
}

// GetName returns the algorithm name
func (a *SpreadAlgorithm) GetName() string {
    return "spread"
}
```

### 4. `pkg/placement/scheduler/algorithms/affinity.go` (90 lines)
Affinity and anti-affinity placement.

```go
package algorithms

import (
    "context"
    "github.com/kcp-dev/kcp/pkg/placement/interfaces"
    "github.com/kcp-dev/kcp/pkg/placement/scheduler"
    "k8s.io/apimachinery/pkg/labels"
)

// AffinityAlgorithm implements affinity-based placement
type AffinityAlgorithm struct {
    affinityWeight     float64
    antiAffinityWeight float64
}

// NewAffinityAlgorithm creates a new affinity algorithm
func NewAffinityAlgorithm() *AffinityAlgorithm {
    return &AffinityAlgorithm{
        affinityWeight:     1.0,
        antiAffinityWeight: 1.0,
    }
}

// Score scores targets based on affinity rules
func (a *AffinityAlgorithm) Score(ctx context.Context, workload interfaces.Workload, 
    targets []interfaces.ClusterTarget) ([]scheduler.ScoredTarget, error) {
    
    scores := []scheduler.ScoredTarget{}
    
    for _, target := range targets {
        score := a.calculateScore(workload, target)
        scores = append(scores, scheduler.ScoredTarget{
            Target: target,
            Score:  score,
            Reason: "affinity scoring",
        })
    }
    
    return scores, nil
}

// calculateScore calculates affinity score
func (a *AffinityAlgorithm) calculateScore(workload interfaces.Workload, 
    target interfaces.ClusterTarget) float64 {
    
    score := 50.0 // Base score
    
    // Check node affinity
    if workload.Spec.Affinity != nil {
        if workload.Spec.Affinity.NodeAffinity != nil {
            affinityScore := a.evaluateNodeAffinity(
                workload.Spec.Affinity.NodeAffinity,
                target.Labels,
            )
            score += affinityScore * a.affinityWeight
        }
        
        // Check anti-affinity
        if workload.Spec.Affinity.AntiAffinity != nil {
            antiAffinityScore := a.evaluateAntiAffinity(
                workload.Spec.Affinity.AntiAffinity,
                target,
            )
            score -= antiAffinityScore * a.antiAffinityWeight
        }
    }
    
    // Normalize to 0-100
    if score < 0 {
        score = 0
    }
    if score > 100 {
        score = 100
    }
    
    return score
}

// evaluateNodeAffinity evaluates node affinity rules
func (a *AffinityAlgorithm) evaluateNodeAffinity(affinity *interfaces.NodeAffinity, 
    targetLabels map[string]string) float64 {
    
    score := 0.0
    
    // Check required affinity
    for _, requirement := range affinity.RequiredDuringScheduling {
        selector, _ := labels.Parse(requirement)
        if selector.Matches(labels.Set(targetLabels)) {
            score += 50.0
        }
    }
    
    // Check preferred affinity
    for _, preference := range affinity.PreferredDuringScheduling {
        selector, _ := labels.Parse(preference.Preference)
        if selector.Matches(labels.Set(targetLabels)) {
            score += float64(preference.Weight)
        }
    }
    
    return score
}

// GetName returns the algorithm name
func (a *AffinityAlgorithm) GetName() string {
    return "affinity"
}
```

### 5. `pkg/placement/scheduler/scorer.go` (80 lines)
Scoring utilities and normalization.

```go
package scheduler

import (
    "math"
    "github.com/kcp-dev/kcp/pkg/placement/interfaces"
)

// Scorer provides scoring utilities
type Scorer struct {
    weights map[string]float64
}

// NewScorer creates a new scorer
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

// ScoredTarget represents a scored cluster target
type ScoredTarget struct {
    Target interfaces.ClusterTarget
    Score  float64
    Reason string
    Details map[string]float64
}

// CombineScores combines multiple scoring factors
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

// NormalizeScore normalizes a score to 0-100 range
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

// CalculateDistance calculates geographical distance for latency scoring
func (s *Scorer) CalculateDistance(region1, region2 string) float64 {
    // Simplified distance calculation
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
    }
    
    if d, ok := distances[region1][region2]; ok {
        return d
    }
    
    return 100.0 // Default distance
}

// Algorithm defines the interface for scheduling algorithms
type Algorithm interface {
    Score(ctx context.Context, workload interfaces.Workload, 
        targets []interfaces.ClusterTarget) ([]ScoredTarget, error)
    GetName() string
}
```

### 6. `pkg/placement/scheduler/scheduler_test.go` (150 lines)
Comprehensive scheduler tests.

```go
package scheduler_test

import (
    "context"
    "testing"
    "github.com/kcp-dev/kcp/pkg/placement/scheduler"
    "github.com/kcp-dev/kcp/pkg/placement/scheduler/algorithms"
    "github.com/kcp-dev/kcp/pkg/placement/interfaces"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestSchedulingEngine(t *testing.T) {
    ctx := context.Background()
    engine := scheduler.NewEngine(nil)
    
    workload := interfaces.Workload{
        Name: "test-workload",
        Spec: interfaces.WorkloadSpec{
            Replicas: 3,
            Resources: interfaces.ResourceRequirements{
                CPU:    10,
                Memory: 100,
            },
        },
    }
    
    targets := []interfaces.ClusterTarget{
        {
            Name: "cluster-1",
            Available: interfaces.Resources{
                CPU:    50,
                Memory: 500,
            },
            Capacity: interfaces.Resources{
                CPU:    100,
                Memory: 1000,
            },
        },
        {
            Name: "cluster-2",
            Available: interfaces.Resources{
                CPU:    80,
                Memory: 800,
            },
            Capacity: interfaces.Resources{
                CPU:    100,
                Memory: 1000,
            },
        },
        {
            Name: "cluster-3",
            Available: interfaces.Resources{
                CPU:    5, // Not enough capacity
                Memory: 50,
            },
            Capacity: interfaces.Resources{
                CPU:    100,
                Memory: 1000,
            },
        },
    }
    
    tests := []struct {
        name     string
        strategy string
        expected []string
    }{
        {
            name:     "bin packing",
            strategy: "binpack",
            expected: []string{"cluster-1"}, // Prefer more utilized cluster
        },
        {
            name:     "spread",
            strategy: "spread",
            expected: []string{"cluster-2", "cluster-1"}, // Spread across clusters
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            decision, err := engine.Schedule(ctx, workload, targets, tt.strategy)
            require.NoError(t, err)
            
            assert.Equal(t, tt.strategy, decision.Strategy)
            assert.NotEmpty(t, decision.Clusters)
        })
    }
}

func TestBinPackAlgorithm(t *testing.T) {
    ctx := context.Background()
    algo := algorithms.NewBinPackAlgorithm()
    
    workload := interfaces.Workload{
        Spec: interfaces.WorkloadSpec{
            Resources: interfaces.ResourceRequirements{
                CPU:    20,
                Memory: 200,
            },
        },
    }
    
    targets := []interfaces.ClusterTarget{
        {
            Name: "high-utilization",
            Available: interfaces.Resources{CPU: 30, Memory: 300},
            Capacity:  interfaces.Resources{CPU: 100, Memory: 1000},
        },
        {
            Name: "low-utilization",
            Available: interfaces.Resources{CPU: 90, Memory: 900},
            Capacity:  interfaces.Resources{CPU: 100, Memory: 1000},
        },
    }
    
    scores, err := algo.Score(ctx, workload, targets)
    require.NoError(t, err)
    
    // Bin packing should prefer the already utilized cluster
    assert.Greater(t, scores[0].Score, scores[1].Score)
}

func TestSpreadAlgorithm(t *testing.T) {
    ctx := context.Background()
    algo := algorithms.NewSpreadAlgorithm()
    
    workload := interfaces.Workload{}
    
    targets := []interfaces.ClusterTarget{
        {
            Name:   "cluster-zone-a",
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
    
    // Should prefer zone-b as it has fewer clusters
    maxScore := 0.0
    maxZone := ""
    for i, score := range scores {
        if score.Score > maxScore {
            maxScore = score.Score
            maxZone = targets[i].Labels["zone"]
        }
    }
    
    assert.Equal(t, "b", maxZone)
}

func TestScorer(t *testing.T) {
    scorer := scheduler.NewScorer()
    
    // Test score combination
    scores := map[string]float64{
        "capacity": 80.0,
        "latency":  60.0,
        "cost":     70.0,
    }
    
    combined := scorer.CombineScores(scores)
    assert.Greater(t, combined, 0.0)
    assert.LessOrEqual(t, combined, 100.0)
    
    // Test normalization
    normalized := scorer.NormalizeScore(5, 0, 10)
    assert.Equal(t, 50.0, normalized)
}
```

## Implementation Steps

### Step 1: Setup Dependencies
```bash
# Ensure branches 13 and 16 are available
git fetch origin feature/tmc-phase4-13-placement-interfaces
git fetch origin feature/tmc-phase4-16-workspace-discovery
```

### Step 2: Create Package Structure
```bash
mkdir -p pkg/placement/scheduler
mkdir -p pkg/placement/scheduler/algorithms
```

### Step 3: Implement Scheduler Components
1. Start with `scorer.go` - scoring utilities
2. Add `engine.go` - main scheduling engine
3. Create `algorithms/binpack.go` - bin packing
4. Add `algorithms/spread.go` - spread algorithm
5. Create `algorithms/affinity.go` - affinity rules
6. Add `scheduler_test.go` - comprehensive tests

### Step 4: Add Benchmarks
Create performance benchmarks for scheduling algorithms.

### Step 5: Integration Testing
Test with various workload patterns and cluster configurations.

## KCP Patterns to Follow

1. **Resource Management**: Accurate capacity tracking
2. **Label Matching**: Kubernetes-style label selectors
3. **Affinity Rules**: Follow K8s affinity patterns
4. **Scoring Model**: Normalized 0-100 scoring
5. **Extensibility**: Pluggable algorithm design

## Testing Requirements

### Unit Tests Required
- [ ] Engine scheduling tests
- [ ] Bin packing algorithm tests
- [ ] Spread algorithm tests
- [ ] Affinity algorithm tests
- [ ] Scorer utility tests

### Performance Tests
- [ ] Large cluster set scheduling
- [ ] Complex affinity rules
- [ ] Algorithm comparison benchmarks

## Integration Points

This scheduler will be:
- **Used by**: Branch 19 (Controller)
- **Tested in**: Branch 23 (Integration)

## Validation Checklist

- [ ] All algorithms produce valid scores
- [ ] Capacity constraints respected
- [ ] Affinity rules work correctly
- [ ] Spread algorithm distributes evenly
- [ ] Bin packing maximizes utilization
- [ ] Thread-safe implementation
- [ ] Performance optimized
- [ ] Documentation complete
- [ ] Test coverage >80%
- [ ] Feature flag ready

## Line Count Validation
Run before committing:
```bash
/workspaces/kcp-shared-tools/tmc-pr-line-counter.sh -c feature/tmc-phase4-18-placement-scheduler
```

Target: ~600 lines