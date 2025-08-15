# Implementation Instructions: Deployment Interfaces (Branch 14b)

## Overview
This branch implements the interface definitions for the deployment coordination system. It defines the contracts that different deployment strategies, coordinators, and health checkers must implement, enabling a pluggable architecture for deployment management.

## Dependencies
- **Base**: feature/tmc-phase4-14a-deployment-core-types
- **Required for**: Branches 14c, 20, 21, 22

## Files to Create

### 1. `pkg/deployment/interfaces/coordinator.go` (60 lines)
Main deployment coordinator interface.

```go
package interfaces

import (
    "context"
    "github.com/kcp-dev/kcp/pkg/deployment/types"
    "k8s.io/apimachinery/pkg/runtime"
)

// DeploymentCoordinator orchestrates deployments across clusters
type DeploymentCoordinator interface {
    // Plan creates a deployment plan based on strategy
    Plan(ctx context.Context, workload runtime.Object, strategy types.DeploymentStrategy) (*DeploymentPlan, error)
    
    // Execute runs the deployment plan
    Execute(ctx context.Context, plan *DeploymentPlan) (*DeploymentResult, error)
    
    // GetStatus returns current deployment status
    GetStatus(ctx context.Context, deploymentID string) (*types.DeploymentStatus, error)
    
    // Pause pauses an ongoing deployment
    Pause(ctx context.Context, deploymentID string) error
    
    // Resume resumes a paused deployment
    Resume(ctx context.Context, deploymentID string) error
    
    // Abort terminates and rolls back a deployment
    Abort(ctx context.Context, deploymentID string) error
}

// DeploymentPlan represents a planned deployment
type DeploymentPlan struct {
    // ID uniquely identifies this deployment
    ID string
    
    // Strategy to use for deployment
    Strategy types.DeploymentStrategy
    
    // Steps in the deployment plan
    Steps []DeploymentStep
    
    // Dependencies between steps
    Dependencies []types.DeploymentDependency
    
    // Rollback plan if deployment fails
    RollbackPlan *RollbackPlan
}

// DeploymentStep represents a single step in deployment
type DeploymentStep struct {
    Name        string
    Type        StepType
    Target      string
    Config      map[string]interface{}
    Timeout     time.Duration
}

// DeploymentResult captures the result of a deployment
type DeploymentResult struct {
    Success bool
    Message string
    Metrics map[string]float64
}
```

### 2. `pkg/deployment/interfaces/strategy.go` (50 lines)
Strategy interface for different deployment patterns.

```go
package interfaces

import (
    "context"
    "github.com/kcp-dev/kcp/pkg/deployment/types"
    "k8s.io/apimachinery/pkg/runtime"
)

// DeploymentStrategy defines the interface for deployment strategies
type DeploymentStrategy interface {
    // GetName returns the strategy name
    GetName() string
    
    // Validate checks if the strategy can be applied to the workload
    Validate(ctx context.Context, workload runtime.Object) error
    
    // GenerateSteps creates deployment steps for this strategy
    GenerateSteps(ctx context.Context, workload runtime.Object, targets []string) ([]DeploymentStep, error)
    
    // CalculateProgress determines deployment progress
    CalculateProgress(ctx context.Context, status types.DeploymentStatus) (int, error)
    
    // ShouldPromote determines if deployment should proceed to next step
    ShouldPromote(ctx context.Context, metrics map[string]float64) (bool, error)
    
    // HandleFailure manages strategy-specific failure handling
    HandleFailure(ctx context.Context, step DeploymentStep, err error) error
}

// StrategyFactory creates deployment strategies
type StrategyFactory interface {
    // CreateStrategy creates a strategy instance
    CreateStrategy(strategyType types.DeploymentType) (DeploymentStrategy, error)
    
    // RegisterStrategy registers a new strategy type
    RegisterStrategy(strategyType types.DeploymentType, strategy DeploymentStrategy) error
    
    // ListStrategies returns available strategies
    ListStrategies() []types.DeploymentType
}

// StepExecutor executes individual deployment steps
type StepExecutor interface {
    // Execute runs a deployment step
    Execute(ctx context.Context, step DeploymentStep) error
    
    // Validate checks if a step can be executed
    Validate(ctx context.Context, step DeploymentStep) error
    
    // GetStatus returns the status of a step execution
    GetStatus(ctx context.Context, stepID string) (StepStatus, error)
}
```

### 3. `pkg/deployment/interfaces/rollback.go` (40 lines)
Rollback management interfaces.

```go
package interfaces

import (
    "context"
    "github.com/kcp-dev/kcp/pkg/deployment/types"
    "time"
)

// RollbackController manages deployment rollbacks
type RollbackController interface {
    // ShouldRollback determines if rollback is needed
    ShouldRollback(ctx context.Context, status types.DeploymentStatus, metrics map[string]float64) (bool, string)
    
    // InitiateRollback starts the rollback process
    InitiateRollback(ctx context.Context, deploymentID string, reason string) error
    
    // GetRollbackStatus returns rollback progress
    GetRollbackStatus(ctx context.Context, deploymentID string) (*RollbackStatus, error)
    
    // CompleteRollback finalizes the rollback
    CompleteRollback(ctx context.Context, deploymentID string) error
}

// RollbackPlan defines how to rollback a deployment
type RollbackPlan struct {
    // Strategy for rollback (immediate, gradual, etc.)
    Strategy RollbackStrategy
    
    // Steps to execute during rollback
    Steps []RollbackStep
    
    // Timeout for rollback completion
    Timeout time.Duration
}

// RollbackStep represents a single rollback action
type RollbackStep struct {
    Name        string
    Action      string
    Target      string
    Priority    int
}

// RollbackStatus tracks rollback progress
type RollbackStatus struct {
    State       string
    Progress    int
    Message     string
    StartTime   time.Time
}
```

### 4. `pkg/deployment/interfaces/health_checker.go` (40 lines)
Health checking interface for deployments.

```go
package interfaces

import (
    "context"
    "github.com/kcp-dev/kcp/pkg/deployment/types"
)

// HealthChecker validates deployment health
type HealthChecker interface {
    // Check performs a health check
    Check(ctx context.Context, target string, config types.HealthCheck) (*types.HealthStatus, error)
    
    // CheckBatch performs multiple health checks
    CheckBatch(ctx context.Context, targets []string, config types.HealthCheck) (map[string]*types.HealthStatus, error)
    
    // StartMonitoring begins continuous health monitoring
    StartMonitoring(ctx context.Context, target string, config types.HealthCheck) (chan *types.HealthStatus, error)
    
    // StopMonitoring stops health monitoring
    StopMonitoring(ctx context.Context, target string) error
}

// ReadinessChecker determines if deployments are ready
type ReadinessChecker interface {
    // IsReady checks if a deployment is ready
    IsReady(ctx context.Context, deploymentID string) (bool, error)
    
    // WaitForReady waits until deployment is ready or timeout
    WaitForReady(ctx context.Context, deploymentID string, timeout time.Duration) error
    
    // GetReadinessGates returns configured readiness gates
    GetReadinessGates(ctx context.Context, deploymentID string) ([]ReadinessGate, error)
}

// ReadinessGate defines a condition for readiness
type ReadinessGate struct {
    Name      string
    Type      string
    Condition string
    Value     interface{}
}
```

### 5. `pkg/deployment/interfaces/analyzer.go` (40 lines)
Analysis interfaces for deployment metrics.

```go
package interfaces

import (
    "context"
    "github.com/kcp-dev/kcp/pkg/deployment/types"
    "time"
)

// MetricAnalyzer analyzes deployment metrics
type MetricAnalyzer interface {
    // Analyze evaluates metrics against thresholds
    Analyze(ctx context.Context, metrics []types.Metric, data map[string]float64) (*AnalysisResult, error)
    
    // Query retrieves metric values
    Query(ctx context.Context, metric types.Metric, timeRange TimeRange) (float64, error)
    
    // StartAnalysis begins continuous analysis
    StartAnalysis(ctx context.Context, metrics []types.Metric) (chan *AnalysisResult, error)
    
    // StopAnalysis stops metric analysis
    StopAnalysis(ctx context.Context, analysisID string) error
}

// AnalysisResult contains metric analysis results
type AnalysisResult struct {
    // Overall pass/fail status
    Passed bool
    
    // Individual metric results
    MetricResults []types.MetricResult
    
    // Confidence score (0-100)
    Confidence float64
    
    // Recommendation based on analysis
    Recommendation string
}

// TimeRange defines a time period for queries
type TimeRange struct {
    Start time.Time
    End   time.Time
}
```

### 6. `pkg/deployment/interfaces/event.go` (30 lines)
Event handling interfaces for deployment lifecycle.

```go
package interfaces

import (
    "context"
    "github.com/kcp-dev/kcp/pkg/deployment/types"
)

// DeploymentEventHandler handles deployment lifecycle events
type DeploymentEventHandler interface {
    // OnDeploymentStarted handles deployment start
    OnDeploymentStarted(ctx context.Context, deploymentID string, plan *DeploymentPlan) error
    
    // OnStepCompleted handles step completion
    OnStepCompleted(ctx context.Context, deploymentID string, step DeploymentStep, result StepResult) error
    
    // OnDeploymentCompleted handles deployment completion
    OnDeploymentCompleted(ctx context.Context, deploymentID string, result DeploymentResult) error
    
    // OnRollbackInitiated handles rollback start
    OnRollbackInitiated(ctx context.Context, deploymentID string, reason string) error
}

// EventPublisher publishes deployment events
type EventPublisher interface {
    // Publish sends an event
    Publish(ctx context.Context, event DeploymentEvent) error
    
    // Subscribe registers an event handler
    Subscribe(handler DeploymentEventHandler) error
}

// DeploymentEvent represents a deployment lifecycle event
type DeploymentEvent struct {
    Type        EventType
    DeploymentID string
    Timestamp   time.Time
    Data        map[string]interface{}
}
```

### 7. `pkg/deployment/interfaces/doc.go` (10 lines)
Package documentation.

```go
// Package interfaces defines the contracts for the deployment coordination system.
// These interfaces enable a pluggable architecture where different deployment
// strategies, health checkers, and analyzers can be implemented and swapped
// without changing the core deployment logic.
package interfaces
```

## Implementation Steps

### Step 1: Setup Dependencies
```bash
# Ensure branch 14a is merged or available locally
git fetch origin feature/tmc-phase4-14a-deployment-core-types
```

### Step 2: Create Package Structure
```bash
mkdir -p pkg/deployment/interfaces
```

### Step 3: Implement Interfaces
1. Start with `coordinator.go` - main orchestration interface
2. Add `strategy.go` - strategy pattern interfaces
3. Create `rollback.go` - rollback management
4. Add `health_checker.go` - health validation
5. Create `analyzer.go` - metric analysis
6. Add `event.go` - event handling
7. Add `doc.go` - package documentation

### Step 4: Add Mock Implementations
Create mocks for testing:
```bash
mockgen -source=pkg/deployment/interfaces/coordinator.go -destination=pkg/deployment/mocks/coordinator.go
```

### Step 5: Create Interface Tests
Add interface compliance tests to ensure implementations meet contracts.

## KCP Patterns to Follow

1. **Context Propagation**: All methods accept context.Context
2. **Error Handling**: Return explicit errors, no panics
3. **Workspace Awareness**: Consider workspace isolation in design
4. **Resource Management**: Clean shutdown/cleanup methods
5. **Observability**: Methods for status and monitoring

## Testing Requirements

### Unit Tests Required
- [ ] Interface compliance tests
- [ ] Mock generation tests
- [ ] Contract validation tests

### Integration Tests
- [ ] Cross-interface interaction tests
- [ ] Event flow tests

## Integration Points

These interfaces will be:
- **Implemented by**: Branches 20 (Canary), 21 (Dependencies), 22 (Rollback)
- **Used by**: Branch 19 (Controller), Branch 23 (Integration)
- **Validated by**: Branch 14c (Validation)
- **Tested by**: Branch 14d (Tests)

## Validation Checklist

- [ ] All interfaces follow Go best practices
- [ ] Context used for cancellation
- [ ] Errors are descriptive
- [ ] Methods are cohesive and focused
- [ ] No circular dependencies
- [ ] Thread-safe design considerations
- [ ] Mockable for testing
- [ ] Documentation complete
- [ ] Compatible with KCP patterns
- [ ] Feature flag integration considered

## Line Count Validation
Run before committing:
```bash
/workspaces/kcp-shared-tools/tmc-pr-line-counter.sh -c feature/tmc-phase4-14b-deployment-interfaces
```

Target: ~300 lines (excluding generated mocks)