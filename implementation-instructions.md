# Implementation Instructions: Deployment Strategy Interfaces

## Overview
This branch defines the contracts for deployment coordination and strategies, establishing the abstraction layer for various deployment patterns (canary, blue-green, rolling) and rollback mechanisms. These interfaces enable pluggable deployment strategies with proper health checking and dependency management.

## Dependencies
- **None** - This is a foundation branch with no dependencies on other Phase 4 branches
- Depends on existing KCP core APIs and Kubernetes deployment types

## Files to Create
1. `pkg/deployment/interfaces/coordinator.go` (60 lines) - Deployment orchestration interface
2. `pkg/deployment/interfaces/strategy.go` (50 lines) - Strategy pattern interface
3. `pkg/deployment/interfaces/rollback.go` (40 lines) - Rollback mechanism interface
4. `pkg/deployment/interfaces/health_checker.go` (40 lines) - Health validation interface
5. `pkg/deployment/types/strategy.go` (100 lines) - Strategy configuration types
6. `pkg/deployment/types/dependency.go` (60 lines) - Dependency graph types

**Total Estimated Lines**: 350

## Implementation Steps

### Step 1: Create Package Structure
```bash
mkdir -p pkg/deployment/interfaces
mkdir -p pkg/deployment/types
```

### Step 2: Define Strategy Types
Create `pkg/deployment/types/strategy.go`:

```go
package types

import (
    "time"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/util/intstr"
)

// DeploymentStrategy defines how a deployment should be executed
type DeploymentStrategy struct {
    // Type of deployment strategy
    Type StrategyType `json:"type"`
    
    // Canary configuration
    Canary *CanaryStrategy `json:"canary,omitempty"`
    
    // BlueGreen configuration
    BlueGreen *BlueGreenStrategy `json:"blueGreen,omitempty"`
    
    // Rolling update configuration
    RollingUpdate *RollingUpdateStrategy `json:"rollingUpdate,omitempty"`
    
    // HealthCheck defines health validation
    HealthCheck *HealthCheckConfig `json:"healthCheck,omitempty"`
}

// StrategyType defines the deployment strategy type
type StrategyType string

const (
    CanaryStrategyType        StrategyType = "Canary"
    BlueGreenStrategyType     StrategyType = "BlueGreen"
    RollingUpdateStrategyType StrategyType = "RollingUpdate"
    RecreateStrategyType      StrategyType = "Recreate"
)

// CanaryStrategy defines progressive rollout configuration
type CanaryStrategy struct {
    // Steps define the canary progression
    Steps []CanaryStep `json:"steps"`
    
    // Analysis configuration for automated promotion
    Analysis *AnalysisConfig `json:"analysis,omitempty"`
    
    // TrafficRouting configuration
    TrafficRouting *TrafficRouting `json:"trafficRouting,omitempty"`
}

// CanaryStep represents a stage in canary deployment
type CanaryStep struct {
    // Weight is the percentage of traffic
    Weight int32 `json:"weight"`
    
    // Pause duration before auto-promotion
    Pause *metav1.Duration `json:"pause,omitempty"`
    
    // Replicas override for this step
    Replicas *int32 `json:"replicas,omitempty"`
}

// AnalysisConfig defines metrics-based promotion
type AnalysisConfig struct {
    // Metrics to evaluate
    Metrics []MetricConfig `json:"metrics"`
    
    // Interval between analysis runs
    Interval metav1.Duration `json:"interval"`
    
    // SuccessCondition as a CEL expression
    SuccessCondition string `json:"successCondition,omitempty"`
}

// MetricConfig defines a metric to track
type MetricConfig struct {
    Name      string  `json:"name"`
    Threshold float64 `json:"threshold"`
    Query     string  `json:"query,omitempty"`
}

// TrafficRouting defines traffic management
type TrafficRouting struct {
    // Istio configuration
    Istio *IstioTrafficRouting `json:"istio,omitempty"`
    
    // Nginx configuration
    Nginx *NginxTrafficRouting `json:"nginx,omitempty"`
}

// BlueGreenStrategy defines blue-green deployment
type BlueGreenStrategy struct {
    // PrePromotionAnalysis runs before switching
    PrePromotionAnalysis *AnalysisConfig `json:"prePromotionAnalysis,omitempty"`
    
    // PostPromotionAnalysis runs after switching
    PostPromotionAnalysis *AnalysisConfig `json:"postPromotionAnalysis,omitempty"`
    
    // AutoPromotionEnabled enables automatic promotion
    AutoPromotionEnabled bool `json:"autoPromotionEnabled"`
    
    // ScaleDownDelay before removing old version
    ScaleDownDelay *metav1.Duration `json:"scaleDownDelay,omitempty"`
}

// RollingUpdateStrategy defines rolling update parameters
type RollingUpdateStrategy struct {
    MaxSurge       *intstr.IntOrString `json:"maxSurge,omitempty"`
    MaxUnavailable *intstr.IntOrString `json:"maxUnavailable,omitempty"`
}

// HealthCheckConfig defines health validation
type HealthCheckConfig struct {
    InitialDelay   metav1.Duration `json:"initialDelay"`
    Interval       metav1.Duration `json:"interval"`
    Timeout        metav1.Duration `json:"timeout"`
    SuccessThreshold int32         `json:"successThreshold"`
    FailureThreshold int32         `json:"failureThreshold"`
}

// DeploymentPlan represents an execution plan
type DeploymentPlan struct {
    Strategy DeploymentStrategy `json:"strategy"`
    Phases   []DeploymentPhase  `json:"phases"`
    Dependencies []Dependency    `json:"dependencies,omitempty"`
}

// DeploymentPhase is a stage in deployment
type DeploymentPhase struct {
    Name      string          `json:"name"`
    Actions   []DeploymentAction `json:"actions"`
    Condition string          `json:"condition,omitempty"`
}

// DeploymentAction is an atomic deployment operation
type DeploymentAction struct {
    Type   ActionType `json:"type"`
    Target string     `json:"target"`
    Config map[string]interface{} `json:"config,omitempty"`
}

type ActionType string

const (
    ScaleAction  ActionType = "Scale"
    UpdateAction ActionType = "Update"
    WaitAction   ActionType = "Wait"
    VerifyAction ActionType = "Verify"
)
```

### Step 3: Define Dependency Types
Create `pkg/deployment/types/dependency.go`:

```go
package types

import (
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Dependency represents a deployment dependency
type Dependency struct {
    // Name of the dependency
    Name string `json:"name"`
    
    // Type of dependency
    Type DependencyType `json:"type"`
    
    // Target resource
    Target DependencyTarget `json:"target"`
    
    // Condition to satisfy
    Condition string `json:"condition,omitempty"`
    
    // Timeout for dependency resolution
    Timeout *metav1.Duration `json:"timeout,omitempty"`
}

// DependencyType defines the type of dependency
type DependencyType string

const (
    HardDependency DependencyType = "Hard"  // Must be satisfied
    SoftDependency DependencyType = "Soft"  // Best effort
)

// DependencyTarget identifies the dependency target
type DependencyTarget struct {
    APIVersion string `json:"apiVersion"`
    Kind       string `json:"kind"`
    Name       string `json:"name"`
    Namespace  string `json:"namespace,omitempty"`
    Workspace  string `json:"workspace,omitempty"`
}

// DependencyGraph represents deployment dependencies
type DependencyGraph struct {
    Nodes map[string]*DependencyNode `json:"nodes"`
    Edges []DependencyEdge           `json:"edges"`
}

// DependencyNode is a node in the dependency graph
type DependencyNode struct {
    ID         string            `json:"id"`
    Resource   DependencyTarget  `json:"resource"`
    Status     DependencyStatus  `json:"status"`
    StartTime  *metav1.Time      `json:"startTime,omitempty"`
    EndTime    *metav1.Time      `json:"endTime,omitempty"`
}

// DependencyEdge represents a dependency relationship
type DependencyEdge struct {
    From string         `json:"from"`
    To   string         `json:"to"`
    Type DependencyType `json:"type"`
}

// DependencyStatus represents the state of a dependency
type DependencyStatus string

const (
    DependencyPending  DependencyStatus = "Pending"
    DependencyReady    DependencyStatus = "Ready"
    DependencyFailed   DependencyStatus = "Failed"
    DependencySkipped  DependencyStatus = "Skipped"
)

// DependencyResolver configuration
type DependencyResolverConfig struct {
    // MaxConcurrency limits parallel operations
    MaxConcurrency int `json:"maxConcurrency"`
    
    // RetryPolicy for failed dependencies
    RetryPolicy RetryPolicy `json:"retryPolicy"`
    
    // IgnoreSoftFailures continues on soft dependency failures
    IgnoreSoftFailures bool `json:"ignoreSoftFailures"`
}

// RetryPolicy defines retry behavior
type RetryPolicy struct {
    MaxRetries int              `json:"maxRetries"`
    Backoff    BackoffStrategy  `json:"backoff"`
}

// BackoffStrategy for retries
type BackoffStrategy struct {
    Type     string           `json:"type"` // exponential, linear, fixed
    Interval metav1.Duration  `json:"interval"`
    MaxDelay metav1.Duration  `json:"maxDelay,omitempty"`
}
```

### Step 4: Define Coordinator Interface
Create `pkg/deployment/interfaces/coordinator.go`:

```go
package interfaces

import (
    "context"
    "github.com/kcp-dev/kcp/pkg/deployment/types"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeploymentCoordinator orchestrates deployment execution
type DeploymentCoordinator interface {
    // Plan creates a deployment plan from strategy
    Plan(ctx context.Context, strategy types.DeploymentStrategy,
         target DeploymentTarget) (*types.DeploymentPlan, error)
    
    // Execute runs the deployment plan
    Execute(ctx context.Context, plan *types.DeploymentPlan) (*DeploymentResult, error)
    
    // Rollback reverses a deployment
    Rollback(ctx context.Context, deploymentID string) error
    
    // GetStatus returns current deployment status
    GetStatus(ctx context.Context, deploymentID string) (*DeploymentStatus, error)
    
    // Pause halts a deployment
    Pause(ctx context.Context, deploymentID string) error
    
    // Resume continues a paused deployment
    Resume(ctx context.Context, deploymentID string) error
}

// DeploymentTarget identifies what to deploy
type DeploymentTarget struct {
    Name       string            `json:"name"`
    Namespace  string            `json:"namespace"`
    Workspace  string            `json:"workspace"`
    APIVersion string            `json:"apiVersion"`
    Kind       string            `json:"kind"`
    Labels     map[string]string `json:"labels,omitempty"`
}

// DeploymentResult contains execution outcome
type DeploymentResult struct {
    DeploymentID string                `json:"deploymentId"`
    Status       DeploymentStatusType  `json:"status"`
    Message      string                `json:"message,omitempty"`
    StartTime    metav1.Time           `json:"startTime"`
    EndTime      *metav1.Time          `json:"endTime,omitempty"`
    Phases       []PhaseResult         `json:"phases"`
}

// DeploymentStatus represents current state
type DeploymentStatus struct {
    DeploymentID string               `json:"deploymentId"`
    Phase        string               `json:"phase"`
    Status       DeploymentStatusType `json:"status"`
    Progress     int32                `json:"progress"`
    Message      string               `json:"message,omitempty"`
}

// DeploymentStatusType defines deployment states
type DeploymentStatusType string

const (
    DeploymentPending    DeploymentStatusType = "Pending"
    DeploymentInProgress DeploymentStatusType = "InProgress"
    DeploymentSucceeded  DeploymentStatusType = "Succeeded"
    DeploymentFailed     DeploymentStatusType = "Failed"
    DeploymentPaused     DeploymentStatusType = "Paused"
    DeploymentRollingBack DeploymentStatusType = "RollingBack"
)

// PhaseResult contains phase execution details
type PhaseResult struct {
    Name      string               `json:"name"`
    Status    DeploymentStatusType `json:"status"`
    StartTime metav1.Time          `json:"startTime"`
    EndTime   *metav1.Time         `json:"endTime,omitempty"`
    Error     string               `json:"error,omitempty"`
}
```

### Step 5: Define Strategy Interface
Create `pkg/deployment/interfaces/strategy.go`:

```go
package interfaces

import (
    "context"
    "github.com/kcp-dev/kcp/pkg/deployment/types"
)

// DeploymentStrategy defines the contract for deployment strategies
type DeploymentStrategy interface {
    // Name returns the strategy name
    Name() string
    
    // Validate checks if the strategy configuration is valid
    Validate(config types.DeploymentStrategy) error
    
    // Initialize prepares the strategy for execution
    Initialize(ctx context.Context, config types.DeploymentStrategy) error
    
    // Execute runs the deployment strategy
    Execute(ctx context.Context, target DeploymentTarget) (*StrategyResult, error)
    
    // Cleanup performs post-deployment cleanup
    Cleanup(ctx context.Context) error
}

// StrategyFactory creates strategy instances
type StrategyFactory interface {
    // Create returns a strategy for the given type
    Create(strategyType types.StrategyType) (DeploymentStrategy, error)
    
    // Register adds a new strategy implementation
    Register(strategyType types.StrategyType, strategy DeploymentStrategy) error
    
    // ListStrategies returns available strategies
    ListStrategies() []types.StrategyType
}

// StrategyResult contains strategy execution outcome
type StrategyResult struct {
    Success    bool                    `json:"success"`
    Message    string                  `json:"message,omitempty"`
    Metrics    map[string]interface{}  `json:"metrics,omitempty"`
    NextAction StrategyAction          `json:"nextAction,omitempty"`
}

// StrategyAction defines next steps
type StrategyAction string

const (
    ContinueAction  StrategyAction = "Continue"
    PauseAction     StrategyAction = "Pause"
    RollbackAction  StrategyAction = "Rollback"
    CompleteAction  StrategyAction = "Complete"
)

// ProgressReporter reports deployment progress
type ProgressReporter interface {
    // Report sends progress update
    Report(progress DeploymentProgress) error
}

// DeploymentProgress represents current progress
type DeploymentProgress struct {
    Phase      string  `json:"phase"`
    Percentage float64 `json:"percentage"`
    Message    string  `json:"message,omitempty"`
}
```

### Step 6: Define Rollback Interface
Create `pkg/deployment/interfaces/rollback.go`:

```go
package interfaces

import (
    "context"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RollbackController manages deployment rollbacks
type RollbackController interface {
    // CanRollback checks if rollback is possible
    CanRollback(ctx context.Context, deploymentID string) (bool, string, error)
    
    // InitiateRollback starts the rollback process
    InitiateRollback(ctx context.Context, deploymentID string, 
                    reason string) (*RollbackOperation, error)
    
    // GetRollbackStatus returns rollback progress
    GetRollbackStatus(ctx context.Context, rollbackID string) (*RollbackStatus, error)
    
    // ListSnapshots returns available snapshots
    ListSnapshots(ctx context.Context, deploymentID string) ([]Snapshot, error)
}

// RollbackOperation represents an active rollback
type RollbackOperation struct {
    ID           string          `json:"id"`
    DeploymentID string          `json:"deploymentId"`
    Reason       string          `json:"reason"`
    StartTime    metav1.Time     `json:"startTime"`
    TargetState  string          `json:"targetState"`
}

// RollbackStatus contains rollback progress
type RollbackStatus struct {
    OperationID string              `json:"operationId"`
    Status      RollbackStatusType  `json:"status"`
    Progress    int32               `json:"progress"`
    Message     string              `json:"message,omitempty"`
}

// RollbackStatusType defines rollback states
type RollbackStatusType string

const (
    RollbackPending    RollbackStatusType = "Pending"
    RollbackInProgress RollbackStatusType = "InProgress"
    RollbackSucceeded  RollbackStatusType = "Succeeded"
    RollbackFailed     RollbackStatusType = "Failed"
)

// Snapshot represents a deployment state snapshot
type Snapshot struct {
    ID        string      `json:"id"`
    Timestamp metav1.Time `json:"timestamp"`
    State     []byte      `json:"state"`
    Version   string      `json:"version"`
}
```

### Step 7: Define Health Checker Interface
Create `pkg/deployment/interfaces/health_checker.go`:

```go
package interfaces

import (
    "context"
    "github.com/kcp-dev/kcp/pkg/deployment/types"
)

// HealthChecker validates deployment health
type HealthChecker interface {
    // Check performs health validation
    Check(ctx context.Context, target HealthTarget) (*HealthStatus, error)
    
    // WaitForReady waits until target is healthy
    WaitForReady(ctx context.Context, target HealthTarget, 
                config types.HealthCheckConfig) error
    
    // RegisterProbe adds a custom health probe
    RegisterProbe(name string, probe HealthProbe) error
}

// HealthTarget identifies what to check
type HealthTarget struct {
    Name      string            `json:"name"`
    Namespace string            `json:"namespace"`
    Type      string            `json:"type"`
    Selector  map[string]string `json:"selector,omitempty"`
}

// HealthStatus represents health check result
type HealthStatus struct {
    Healthy    bool              `json:"healthy"`
    Ready      bool              `json:"ready"`
    Message    string            `json:"message,omitempty"`
    Conditions []HealthCondition `json:"conditions,omitempty"`
}

// HealthCondition is a specific health aspect
type HealthCondition struct {
    Type    string `json:"type"`
    Status  bool   `json:"status"`
    Message string `json:"message,omitempty"`
}

// HealthProbe defines a custom health check
type HealthProbe interface {
    // Name returns the probe name
    Name() string
    
    // Check executes the health probe
    Check(ctx context.Context, target HealthTarget) (bool, string, error)
}
```

## Testing Requirements

### Unit Tests Required:
1. Strategy type validation tests
2. Dependency graph construction tests
3. Interface mock generation tests

Create basic test file `pkg/deployment/interfaces/interfaces_test.go`:
```go
package interfaces_test

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/kcp-dev/kcp/pkg/deployment/types"
)

func TestStrategyTypeValidation(t *testing.T) {
    strategy := types.DeploymentStrategy{
        Type: types.CanaryStrategyType,
        Canary: &types.CanaryStrategy{
            Steps: []types.CanaryStep{
                {Weight: 10},
                {Weight: 50},
                {Weight: 100},
            },
        },
    }
    
    assert.Equal(t, types.CanaryStrategyType, strategy.Type)
    assert.Len(t, strategy.Canary.Steps, 3)
}

func TestDependencyGraphConstruction(t *testing.T) {
    graph := &types.DependencyGraph{
        Nodes: make(map[string]*types.DependencyNode),
        Edges: []types.DependencyEdge{},
    }
    
    // Add nodes
    graph.Nodes["app1"] = &types.DependencyNode{
        ID: "app1",
        Status: types.DependencyPending,
    }
    
    graph.Nodes["app2"] = &types.DependencyNode{
        ID: "app2",
        Status: types.DependencyPending,
    }
    
    // Add edge
    graph.Edges = append(graph.Edges, types.DependencyEdge{
        From: "app1",
        To: "app2",
        Type: types.HardDependency,
    })
    
    assert.Len(t, graph.Nodes, 2)
    assert.Len(t, graph.Edges, 1)
}
```

## Integration Points

### Connections with Other Components:
1. **Canary Strategy** (Branch 8) will implement the `DeploymentStrategy` interface
2. **Dependency Graph** (Branch 9) will use the dependency types
3. **Rollback Engine** (Branch 10) will implement the `RollbackController` interface
4. **Cross-Workspace Controller** (Branch 7) will use the `DeploymentCoordinator`

### Future Implementations:
- Branch 8 will provide concrete canary strategy
- Branch 9 will implement dependency resolution
- Branch 10 will implement rollback mechanisms

## Code Examples

### Using the DeploymentCoordinator:
```go
// Example usage in a controller
func (c *Controller) deployWorkload(ctx context.Context) error {
    strategy := types.DeploymentStrategy{
        Type: types.CanaryStrategyType,
        Canary: &types.CanaryStrategy{
            Steps: []types.CanaryStep{
                {Weight: 20, Pause: &metav1.Duration{Duration: 5 * time.Minute}},
                {Weight: 50, Pause: &metav1.Duration{Duration: 10 * time.Minute}},
                {Weight: 100},
            },
        },
    }
    
    target := interfaces.DeploymentTarget{
        Name: "my-app",
        Namespace: "production",
        Kind: "Deployment",
    }
    
    // Create deployment plan
    plan, err := c.coordinator.Plan(ctx, strategy, target)
    if err != nil {
        return err
    }
    
    // Execute deployment
    result, err := c.coordinator.Execute(ctx, plan)
    if err != nil {
        return err
    }
    
    if result.Status == interfaces.DeploymentFailed {
        // Initiate rollback
        return c.coordinator.Rollback(ctx, result.DeploymentID)
    }
    
    return nil
}
```

## Validation Checklist

Before marking this branch complete, ensure:

- [ ] All interface files are created with documentation
- [ ] Types are well-defined with proper JSON tags
- [ ] Strategy types cover canary, blue-green, and rolling updates
- [ ] Dependency types support graph construction
- [ ] Health check interfaces are flexible for various probes
- [ ] Rollback interfaces support snapshot-based recovery
- [ ] Package structure follows KCP conventions
- [ ] Mock generation is possible for testing
- [ ] Basic unit tests validate type structures
- [ ] No compilation errors
- [ ] Total lines of code is under 350
- [ ] No dependencies on other Phase 4 branches
- [ ] Ready for strategy implementations to build upon