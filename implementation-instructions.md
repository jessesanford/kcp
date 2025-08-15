# Implementation Instructions: Placement Engine Interfaces

## Overview
This branch implements the core abstraction layer for cross-workspace placement, defining the fundamental interfaces that all placement components will use. These interfaces establish the contracts for workspace discovery, policy evaluation, scheduling algorithms, and placement decisions.

## Dependencies
- **None** - This is a foundation branch with no dependencies on other Phase 4 branches
- Depends on existing KCP core APIs and types

## Files to Create
1. `pkg/placement/interfaces/engine.go` (70 lines) - Core placement engine interface
2. `pkg/placement/interfaces/workspace_discovery.go` (60 lines) - Workspace traversal abstraction
3. `pkg/placement/interfaces/policy_evaluator.go` (50 lines) - Policy evaluation contract
4. `pkg/placement/interfaces/scheduler.go` (50 lines) - Scheduling algorithm interface
5. `pkg/placement/interfaces/types.go` (120 lines) - Common types and structures
6. `pkg/placement/interfaces/doc.go` (20 lines) - Package documentation

**Total Estimated Lines**: 370

## Implementation Steps

### Step 1: Create Package Structure
```bash
mkdir -p pkg/placement/interfaces
```

### Step 2: Implement Package Documentation
Create `pkg/placement/interfaces/doc.go`:
```go
/*
Package interfaces defines the core abstractions for cross-workspace placement
in KCP. It provides pluggable interfaces for workspace discovery, policy
evaluation, and placement scheduling.

This package follows the strategy pattern to allow different implementations
of placement algorithms, policy evaluators, and workspace discovery mechanisms.
*/
package interfaces
```

### Step 3: Define Core Types
Create `pkg/placement/interfaces/types.go` with essential structures:

```go
package interfaces

import (
    "time"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/runtime"
    "github.com/kcp-dev/kcp/sdk/apis/core"
    workloadv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/workload/v1alpha1"
)

// PlacementDecision represents the result of a placement operation
type PlacementDecision struct {
    // TargetClusters lists the selected clusters with their scores
    TargetClusters []ScoredTarget `json:"targetClusters"`
    
    // PolicyEvaluations contains the results of policy evaluations
    PolicyEvaluations []PolicyResult `json:"policyEvaluations,omitempty"`
    
    // SchedulingResult contains details about the scheduling algorithm used
    SchedulingResult *SchedulingResult `json:"schedulingResult,omitempty"`
    
    // Timestamp when the decision was made
    Timestamp metav1.Time `json:"timestamp"`
}

// ClusterTarget represents a potential target cluster
type ClusterTarget struct {
    // Name is the cluster name
    Name string `json:"name"`
    
    // Workspace is the logical cluster path
    Workspace core.LogicalCluster `json:"workspace"`
    
    // Location is the physical location reference
    Location *workloadv1alpha1.Location `json:"location,omitempty"`
    
    // Capacity represents available resources
    Capacity ResourceCapacity `json:"capacity"`
    
    // Labels from the cluster
    Labels map[string]string `json:"labels,omitempty"`
    
    // Annotations from the cluster
    Annotations map[string]string `json:"annotations,omitempty"`
}

// ScoredTarget is a cluster with its placement score
type ScoredTarget struct {
    ClusterTarget
    Score int32 `json:"score"`
    Reasons []string `json:"reasons,omitempty"`
}

// ResourceCapacity describes available resources in a cluster
type ResourceCapacity struct {
    CPU    string `json:"cpu"`
    Memory string `json:"memory"`
    Pods   int32  `json:"pods"`
}

// PlacementPolicy defines placement rules and constraints
type PlacementPolicy struct {
    // Name identifies the policy
    Name string `json:"name"`
    
    // Rules contains CEL expressions or other policy rules
    Rules []PolicyRule `json:"rules"`
    
    // Priority determines evaluation order
    Priority int32 `json:"priority"`
}

// PolicyRule represents a single policy constraint
type PolicyRule struct {
    Expression string `json:"expression"`
    Weight     int32  `json:"weight,omitempty"`
}

// PolicyResult contains the outcome of policy evaluation
type PolicyResult struct {
    PolicyName string `json:"policyName"`
    Passed     bool   `json:"passed"`
    Message    string `json:"message,omitempty"`
}

// SchedulingResult contains scheduling algorithm details
type SchedulingResult struct {
    Algorithm string `json:"algorithm"`
    Duration  time.Duration `json:"duration"`
    Iterations int `json:"iterations,omitempty"`
}

// WorkspaceInfo represents a KCP workspace
type WorkspaceInfo struct {
    Name core.LogicalCluster `json:"name"`
    Parent *core.LogicalCluster `json:"parent,omitempty"`
    Labels map[string]string `json:"labels,omitempty"`
}
```

### Step 4: Define Placement Engine Interface
Create `pkg/placement/interfaces/engine.go`:

```go
package interfaces

import (
    "context"
    "k8s.io/apimachinery/pkg/runtime"
    "k8s.io/apimachinery/pkg/labels"
)

// PlacementEngine orchestrates the placement decision process
type PlacementEngine interface {
    // FindClusters discovers available clusters across workspaces
    FindClusters(ctx context.Context, workload runtime.Object, 
                 workspaces []string) ([]ClusterTarget, error)
    
    // Evaluate applies policies to filter and score clusters
    Evaluate(ctx context.Context, policy PlacementPolicy, 
             targets []ClusterTarget) ([]ScoredTarget, error)
    
    // Place makes the final placement decision
    Place(ctx context.Context, workload runtime.Object, 
          targets []ScoredTarget) (*PlacementDecision, error)
    
    // UpdatePlacement updates an existing placement
    UpdatePlacement(ctx context.Context, placement *PlacementDecision,
                   workload runtime.Object) (*PlacementDecision, error)
}

// PlacementEngineOptions configures the placement engine
type PlacementEngineOptions struct {
    // MaxClusters limits the number of clusters to consider
    MaxClusters int
    
    // EnableCaching enables result caching
    EnableCaching bool
    
    // CacheTTL sets cache expiration time
    CacheTTL time.Duration
}
```

### Step 5: Define Workspace Discovery Interface
Create `pkg/placement/interfaces/workspace_discovery.go`:

```go
package interfaces

import (
    "context"
    "k8s.io/apimachinery/pkg/labels"
    "github.com/kcp-dev/kcp/sdk/apis/core"
)

// WorkspaceDiscovery provides workspace traversal and cluster discovery
type WorkspaceDiscovery interface {
    // ListWorkspaces returns workspaces matching the selector
    ListWorkspaces(ctx context.Context, selector labels.Selector) ([]WorkspaceInfo, error)
    
    // GetClusters returns available clusters in a workspace
    GetClusters(ctx context.Context, workspace core.LogicalCluster) ([]ClusterTarget, error)
    
    // CheckAccess verifies permission to place workloads in a workspace
    CheckAccess(ctx context.Context, workspace core.LogicalCluster, 
                verb string, resource string) (bool, error)
    
    // GetWorkspaceHierarchy returns parent-child relationships
    GetWorkspaceHierarchy(ctx context.Context, 
                         root core.LogicalCluster) (*WorkspaceTree, error)
}

// WorkspaceTree represents workspace hierarchy
type WorkspaceTree struct {
    Root     WorkspaceInfo
    Children map[string]*WorkspaceTree
}

// DiscoveryOptions configures workspace discovery
type DiscoveryOptions struct {
    // MaxDepth limits traversal depth
    MaxDepth int
    
    // IncludeSystemWorkspaces includes system workspaces
    IncludeSystemWorkspaces bool
}
```

### Step 6: Define Policy Evaluator Interface
Create `pkg/placement/interfaces/policy_evaluator.go`:

```go
package interfaces

import (
    "context"
)

// PolicyEvaluator evaluates placement policies
type PolicyEvaluator interface {
    // Compile validates and compiles a policy expression
    Compile(expression string) (CompiledExpression, error)
    
    // Evaluate runs a compiled expression against variables
    Evaluate(ctx context.Context, expr CompiledExpression, 
             vars map[string]interface{}) (bool, error)
    
    // EvaluatePolicy evaluates a complete policy
    EvaluatePolicy(ctx context.Context, policy PlacementPolicy,
                  target ClusterTarget, workload interface{}) (*PolicyResult, error)
}

// CompiledExpression represents a compiled policy expression
type CompiledExpression interface {
    // String returns the original expression
    String() string
    
    // IsValid checks if the expression is valid
    IsValid() bool
}

// PolicyContext provides context for policy evaluation
type PolicyContext struct {
    Cluster  ClusterTarget
    Workload interface{}
    User     string
}
```

### Step 7: Define Scheduler Interface
Create `pkg/placement/interfaces/scheduler.go`:

```go
package interfaces

import (
    "context"
    "k8s.io/apimachinery/pkg/runtime"
)

// Scheduler implements placement scheduling algorithms
type Scheduler interface {
    // Schedule determines placement based on algorithm
    Schedule(ctx context.Context, workload runtime.Object,
            clusters []ClusterTarget) ([]ScoredTarget, error)
    
    // Algorithm returns the scheduling algorithm name
    Algorithm() string
    
    // Configure sets scheduler options
    Configure(options SchedulerOptions) error
}

// SchedulerOptions configures scheduling behavior
type SchedulerOptions struct {
    // Strategy is the scheduling strategy (binpack, spread, etc.)
    Strategy string
    
    // Weights for different scoring factors
    Weights map[string]float64
    
    // Constraints for scheduling
    Constraints []string
}

// SchedulerFactory creates scheduler instances
type SchedulerFactory interface {
    // Create returns a new scheduler for the given strategy
    Create(strategy string) (Scheduler, error)
    
    // ListStrategies returns available strategies
    ListStrategies() []string
}
```

## Key Interfaces/Types

### Critical Types to Implement:
1. **PlacementDecision** - Core result type containing selected clusters
2. **ClusterTarget** - Represents a potential placement target
3. **PlacementPolicy** - Defines placement rules and constraints
4. **WorkspaceInfo** - Workspace metadata and hierarchy

### Core Interfaces:
1. **PlacementEngine** - Main orchestrator interface
2. **WorkspaceDiscovery** - Workspace traversal abstraction
3. **PolicyEvaluator** - Policy evaluation contract
4. **Scheduler** - Scheduling algorithm interface

## Testing Requirements

### Unit Tests Required:
1. Type validation tests for all structures
2. Interface mock generation
3. Basic interface compliance tests

Create `pkg/placement/interfaces/interfaces_test.go`:
```go
package interfaces_test

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/kcp-dev/kcp/pkg/placement/interfaces"
)

func TestPlacementDecisionValidation(t *testing.T) {
    decision := &interfaces.PlacementDecision{
        TargetClusters: []interfaces.ScoredTarget{
            {
                ClusterTarget: interfaces.ClusterTarget{
                    Name: "cluster-1",
                },
                Score: 100,
            },
        },
    }
    
    assert.NotNil(t, decision)
    assert.Len(t, decision.TargetClusters, 1)
}

func TestClusterTargetFields(t *testing.T) {
    target := interfaces.ClusterTarget{
        Name: "test-cluster",
        Labels: map[string]string{
            "region": "us-west",
        },
    }
    
    assert.Equal(t, "test-cluster", target.Name)
    assert.Equal(t, "us-west", target.Labels["region"])
}
```

## Integration Points

### Connections with Other Components:
1. **Workspace Discovery** (Branch 4) will implement the `WorkspaceDiscovery` interface
2. **CEL Evaluator** (Branch 5) will implement the `PolicyEvaluator` interface
3. **Placement Scheduler** (Branch 6) will implement the `Scheduler` interface
4. **Cross-Workspace Controller** (Branch 7) will use the `PlacementEngine` interface

### API Registration:
- These interfaces will be used internally, no CRD registration needed
- Types will be used in status fields of placement CRDs

## Code Examples

### Using the PlacementEngine:
```go
// Example usage in a controller
func (c *Controller) reconcilePlacement(ctx context.Context, workload runtime.Object) error {
    // Discover clusters
    clusters, err := c.engine.FindClusters(ctx, workload, []string{"root:org:prod"})
    if err != nil {
        return err
    }
    
    // Apply policies
    policy := interfaces.PlacementPolicy{
        Name: "data-residency",
        Rules: []interfaces.PolicyRule{
            {Expression: "cluster.region == 'us-west'"},
        },
    }
    
    scored, err := c.engine.Evaluate(ctx, policy, clusters)
    if err != nil {
        return err
    }
    
    // Make placement decision
    decision, err := c.engine.Place(ctx, workload, scored)
    if err != nil {
        return err
    }
    
    return c.applyPlacement(decision)
}
```

## Validation Checklist

Before marking this branch complete, ensure:

- [ ] All interface files are created with proper documentation
- [ ] Types are well-defined with JSON tags
- [ ] Interfaces follow KCP patterns and conventions
- [ ] Package documentation explains the abstraction strategy
- [ ] Mock generation is possible for all interfaces
- [ ] Basic unit tests validate type structures
- [ ] No compilation errors in the package
- [ ] Interfaces are cohesive and follow single responsibility principle
- [ ] Error handling patterns are consistent
- [ ] Comments explain design decisions and usage patterns
- [ ] Total lines of code is under 400 (target: 370)
- [ ] No dependencies on other Phase 4 branches
- [ ] Ready for dependent branches to build upon