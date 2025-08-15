# Implementation Instructions: Deployment Tests (Branch 14d)

## Overview
This branch implements comprehensive test suites for the deployment system, including unit tests, integration tests, and test utilities. It provides test fixtures, mocks, and helpers to ensure the deployment system works correctly.

## Dependencies
- **Base**: feature/tmc-phase4-14c-deployment-validation
- **Uses**: Branches 14a (types), 14b (interfaces), 14c (validation)
- **Required for**: Branches 20-22 (implementation testing)

## Files to Create

### 1. `pkg/deployment/testing/fixtures.go` (80 lines)
Test fixtures and sample data for deployment testing.

```go
package testing

import (
    "github.com/kcp-dev/kcp/pkg/deployment/types"
    "github.com/kcp-dev/kcp/pkg/deployment/interfaces"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "time"
)

// DeploymentFixtures provides test fixtures
type DeploymentFixtures struct{}

// NewDeploymentFixtures creates test fixtures
func NewDeploymentFixtures() *DeploymentFixtures {
    return &DeploymentFixtures{}
}

// ValidCanaryStrategy returns a valid canary strategy for testing
func (f *DeploymentFixtures) ValidCanaryStrategy() types.DeploymentStrategy {
    return types.DeploymentStrategy{
        Type: types.CanaryDeploymentType,
        Canary: &types.CanaryStrategy{
            Steps: []types.CanaryStep{
                {Weight: 10, Pause: 5 * time.Minute},
                {Weight: 30, Pause: 10 * time.Minute},
                {Weight: 60, Pause: 10 * time.Minute},
                {Weight: 100, Pause: 0},
            },
            Analysis: &types.CanaryAnalysis{
                Metrics: []types.Metric{
                    {
                        Name:  "error-rate",
                        Query: "rate(errors_total[5m])",
                        Threshold: types.MetricThreshold{
                            Max: floatPtr(0.01),
                        },
                    },
                },
                Threshold: 0.95,
            },
        },
    }
}

// ValidRollingStrategy returns a valid rolling strategy
func (f *DeploymentFixtures) ValidRollingStrategy() types.DeploymentStrategy {
    return types.DeploymentStrategy{
        Type: types.RollingDeploymentType,
        RollingUpdate: &types.RollingUpdateStrategy{
            MaxUnavailable: intOrStringPtr("25%"),
            MaxSurge:       intOrStringPtr("25%"),
        },
    }
}

// ValidDeploymentPlan returns a valid deployment plan
func (f *DeploymentFixtures) ValidDeploymentPlan() interfaces.DeploymentPlan {
    return interfaces.DeploymentPlan{
        ID:       "test-deployment-123",
        Strategy: f.ValidCanaryStrategy(),
        Steps: []interfaces.DeploymentStep{
            {
                Name:    "canary-10",
                Type:    interfaces.StepTypeCanary,
                Target:  "cluster-1",
                Timeout: 5 * time.Minute,
            },
            {
                Name:    "canary-30",
                Type:    interfaces.StepTypeCanary,
                Target:  "cluster-1",
                Timeout: 10 * time.Minute,
            },
        },
    }
}

// InvalidDeploymentPlan returns an invalid deployment plan for testing
func (f *DeploymentFixtures) InvalidDeploymentPlan() interfaces.DeploymentPlan {
    return interfaces.DeploymentPlan{
        ID: "", // Invalid: missing ID
        Strategy: types.DeploymentStrategy{
            Type: types.CanaryDeploymentType,
            // Invalid: missing canary configuration
        },
    }
}
```

### 2. `pkg/deployment/testing/mocks.go` (70 lines)
Mock implementations for testing.

```go
package testing

import (
    "context"
    "github.com/kcp-dev/kcp/pkg/deployment/interfaces"
    "github.com/kcp-dev/kcp/pkg/deployment/types"
    "k8s.io/apimachinery/pkg/runtime"
)

// MockDeploymentCoordinator is a mock implementation
type MockDeploymentCoordinator struct {
    PlanFunc    func(ctx context.Context, workload runtime.Object, strategy types.DeploymentStrategy) (*interfaces.DeploymentPlan, error)
    ExecuteFunc func(ctx context.Context, plan *interfaces.DeploymentPlan) (*interfaces.DeploymentResult, error)
    StatusFunc  func(ctx context.Context, deploymentID string) (*types.DeploymentStatus, error)
    PauseFunc   func(ctx context.Context, deploymentID string) error
    ResumeFunc  func(ctx context.Context, deploymentID string) error
    AbortFunc   func(ctx context.Context, deploymentID string) error
}

// Plan calls the mock function
func (m *MockDeploymentCoordinator) Plan(ctx context.Context, workload runtime.Object, strategy types.DeploymentStrategy) (*interfaces.DeploymentPlan, error) {
    if m.PlanFunc != nil {
        return m.PlanFunc(ctx, workload, strategy)
    }
    return &interfaces.DeploymentPlan{}, nil
}

// Execute calls the mock function
func (m *MockDeploymentCoordinator) Execute(ctx context.Context, plan *interfaces.DeploymentPlan) (*interfaces.DeploymentResult, error) {
    if m.ExecuteFunc != nil {
        return m.ExecuteFunc(ctx, plan)
    }
    return &interfaces.DeploymentResult{Success: true}, nil
}

// GetStatus calls the mock function
func (m *MockDeploymentCoordinator) GetStatus(ctx context.Context, deploymentID string) (*types.DeploymentStatus, error) {
    if m.StatusFunc != nil {
        return m.StatusFunc(ctx, deploymentID)
    }
    return &types.DeploymentStatus{State: types.DeploymentStateCompleted}, nil
}

// MockHealthChecker is a mock health checker
type MockHealthChecker struct {
    CheckFunc      func(ctx context.Context, target string, config types.HealthCheck) (*types.HealthStatus, error)
    CheckBatchFunc func(ctx context.Context, targets []string, config types.HealthCheck) (map[string]*types.HealthStatus, error)
}

// Check calls the mock function
func (m *MockHealthChecker) Check(ctx context.Context, target string, config types.HealthCheck) (*types.HealthStatus, error) {
    if m.CheckFunc != nil {
        return m.CheckFunc(ctx, target, config)
    }
    return &types.HealthStatus{Healthy: true}, nil
}

// CheckBatch calls the mock function
func (m *MockHealthChecker) CheckBatch(ctx context.Context, targets []string, config types.HealthCheck) (map[string]*types.HealthStatus, error) {
    if m.CheckBatchFunc != nil {
        return m.CheckBatchFunc(ctx, targets, config)
    }
    result := make(map[string]*types.HealthStatus)
    for _, target := range targets {
        result[target] = &types.HealthStatus{Healthy: true}
    }
    return result, nil
}
```

### 3. `pkg/deployment/validation/validation_test.go` (120 lines)
Comprehensive validation tests.

```go
package validation_test

import (
    "testing"
    "github.com/kcp-dev/kcp/pkg/deployment/validation"
    "github.com/kcp-dev/kcp/pkg/deployment/testing"
    "github.com/kcp-dev/kcp/pkg/deployment/types"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestStrategyValidation(t *testing.T) {
    fixtures := testing.NewDeploymentFixtures()
    validator := validation.NewStrategyValidator()
    
    tests := []struct {
        name     string
        strategy types.DeploymentStrategy
        wantErr  bool
        errCount int
    }{
        {
            name:     "valid canary strategy",
            strategy: fixtures.ValidCanaryStrategy(),
            wantErr:  false,
        },
        {
            name:     "valid rolling strategy",
            strategy: fixtures.ValidRollingStrategy(),
            wantErr:  false,
        },
        {
            name: "invalid canary - no steps",
            strategy: types.DeploymentStrategy{
                Type: types.CanaryDeploymentType,
                Canary: &types.CanaryStrategy{
                    Steps: []types.CanaryStep{},
                },
            },
            wantErr:  true,
            errCount: 1,
        },
        {
            name: "invalid canary - weight out of range",
            strategy: types.DeploymentStrategy{
                Type: types.CanaryDeploymentType,
                Canary: &types.CanaryStrategy{
                    Steps: []types.CanaryStep{
                        {Weight: 150, Pause: 5 * time.Minute},
                    },
                },
            },
            wantErr:  true,
            errCount: 1,
        },
        {
            name: "missing strategy type",
            strategy: types.DeploymentStrategy{},
            wantErr:  true,
            errCount: 1,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            errs := validator.ValidateStrategy(tt.strategy)
            
            if tt.wantErr {
                assert.NotEmpty(t, errs, "expected validation errors")
                if tt.errCount > 0 {
                    assert.Len(t, errs, tt.errCount, "unexpected number of errors")
                }
            } else {
                assert.Empty(t, errs, "unexpected validation errors: %v", errs)
            }
        })
    }
}

func TestDependencyValidation(t *testing.T) {
    validator := validation.NewDependencyValidator()
    
    tests := []struct {
        name    string
        graph   types.DependencyGraph
        wantErr bool
        errMsg  string
    }{
        {
            name: "valid dependency graph",
            graph: types.DependencyGraph{
                Nodes: map[string]*types.DeploymentNode{
                    "app-a": {Name: "app-a"},
                    "app-b": {Name: "app-b"},
                },
                Edges: []types.DependencyEdge{
                    {From: "app-a", To: "app-b", Type: types.HardDependency},
                },
            },
            wantErr: false,
        },
        {
            name: "circular dependency",
            graph: types.DependencyGraph{
                Nodes: map[string]*types.DeploymentNode{
                    "app-a": {Name: "app-a"},
                    "app-b": {Name: "app-b"},
                },
                Edges: []types.DependencyEdge{
                    {From: "app-a", To: "app-b", Type: types.HardDependency},
                    {From: "app-b", To: "app-a", Type: types.HardDependency},
                },
            },
            wantErr: true,
            errMsg:  "circular dependency detected",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            errs := validator.ValidateDependencyGraph(tt.graph)
            
            if tt.wantErr {
                assert.NotEmpty(t, errs, "expected validation errors")
                if tt.errMsg != "" {
                    assert.Contains(t, errs.ToAggregate().Error(), tt.errMsg)
                }
            } else {
                assert.Empty(t, errs, "unexpected validation errors: %v", errs)
            }
        })
    }
}
```

### 4. `pkg/deployment/types/types_test.go` (100 lines)
Tests for type definitions and marshaling.

```go
package types_test

import (
    "encoding/json"
    "testing"
    "github.com/kcp-dev/kcp/pkg/deployment/types"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "time"
)

func TestDeploymentStrategyMarshaling(t *testing.T) {
    tests := []struct {
        name     string
        strategy types.DeploymentStrategy
    }{
        {
            name: "canary strategy",
            strategy: types.DeploymentStrategy{
                Type: types.CanaryDeploymentType,
                Canary: &types.CanaryStrategy{
                    Steps: []types.CanaryStep{
                        {Weight: 10, Pause: 5 * time.Minute},
                        {Weight: 50, Pause: 10 * time.Minute},
                    },
                },
            },
        },
        {
            name: "rolling strategy",
            strategy: types.DeploymentStrategy{
                Type: types.RollingDeploymentType,
                RollingUpdate: &types.RollingUpdateStrategy{
                    MaxUnavailable: intOrStringPtr("25%"),
                    MaxSurge:       intOrStringPtr("25%"),
                },
            },
        },
        {
            name: "blue-green strategy",
            strategy: types.DeploymentStrategy{
                Type: types.BlueGreenDeploymentType,
                BlueGreen: &types.BlueGreenStrategy{
                    AutoPromotionEnabled: true,
                    AutoPromotionSeconds: 300,
                },
            },
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Marshal to JSON
            data, err := json.Marshal(tt.strategy)
            require.NoError(t, err, "failed to marshal strategy")
            
            // Unmarshal back
            var decoded types.DeploymentStrategy
            err = json.Unmarshal(data, &decoded)
            require.NoError(t, err, "failed to unmarshal strategy")
            
            // Compare
            assert.Equal(t, tt.strategy.Type, decoded.Type)
            
            // Type-specific comparison
            switch tt.strategy.Type {
            case types.CanaryDeploymentType:
                assert.Equal(t, tt.strategy.Canary, decoded.Canary)
            case types.RollingDeploymentType:
                assert.Equal(t, tt.strategy.RollingUpdate, decoded.RollingUpdate)
            case types.BlueGreenDeploymentType:
                assert.Equal(t, tt.strategy.BlueGreen, decoded.BlueGreen)
            }
        })
    }
}

func TestDeploymentStateTransitions(t *testing.T) {
    validTransitions := map[types.DeploymentState][]types.DeploymentState{
        types.DeploymentStatePending:     {types.DeploymentStateProgressing, types.DeploymentStateFailed},
        types.DeploymentStateProgressing: {types.DeploymentStatePaused, types.DeploymentStateCompleted, types.DeploymentStateFailed, types.DeploymentStateRollingBack},
        types.DeploymentStatePaused:      {types.DeploymentStateProgressing, types.DeploymentStateFailed, types.DeploymentStateRollingBack},
        types.DeploymentStateCompleted:   {}, // Terminal state
        types.DeploymentStateFailed:      {types.DeploymentStateRollingBack},
        types.DeploymentStateRollingBack: {types.DeploymentStateCompleted, types.DeploymentStateFailed},
    }
    
    for fromState, toStates := range validTransitions {
        t.Run(string(fromState), func(t *testing.T) {
            status := types.DeploymentStatus{State: fromState}
            
            for _, toState := range toStates {
                // Test valid transition
                assert.True(t, isValidTransition(fromState, toState), 
                    "transition from %s to %s should be valid", fromState, toState)
            }
        })
    }
}

// Helper function to check valid state transitions
func isValidTransition(from, to types.DeploymentState) bool {
    // Implementation would check state machine rules
    return true
}
```

### 5. `pkg/deployment/testing/helpers.go` (80 lines)
Test helper utilities.

```go
package testing

import (
    "context"
    "testing"
    "time"
    "github.com/kcp-dev/kcp/pkg/deployment/types"
    "github.com/stretchr/testify/require"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/util/intstr"
)

// TestContext creates a test context with timeout
func TestContext(t *testing.T) context.Context {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    t.Cleanup(cancel)
    return ctx
}

// WaitForCondition waits for a condition to be true
func WaitForCondition(t *testing.T, timeout time.Duration, condition func() bool, message string) {
    deadline := time.Now().Add(timeout)
    for time.Now().Before(deadline) {
        if condition() {
            return
        }
        time.Sleep(100 * time.Millisecond)
    }
    t.Fatalf("condition not met within timeout: %s", message)
}

// AssertDeploymentStatus asserts deployment status matches expected
func AssertDeploymentStatus(t *testing.T, got, want types.DeploymentStatus) {
    require.Equal(t, want.State, got.State, "deployment state mismatch")
    require.Equal(t, want.CurrentStep, got.CurrentStep, "current step mismatch")
    require.Equal(t, want.TotalSteps, got.TotalSteps, "total steps mismatch")
}

// CreateTestHealthCheck creates a test health check configuration
func CreateTestHealthCheck(checkType types.HealthCheckType) types.HealthCheck {
    base := types.HealthCheck{
        Type:                checkType,
        InitialDelaySeconds: 10,
        PeriodSeconds:       5,
        TimeoutSeconds:      3,
        SuccessThreshold:    1,
        FailureThreshold:    3,
    }
    
    switch checkType {
    case types.HTTPHealthCheckType:
        base.HTTP = &types.HTTPHealthCheck{
            Path:   "/health",
            Port:   8080,
            Scheme: "HTTP",
        }
    case types.TCPHealthCheckType:
        base.TCP = &types.TCPHealthCheck{
            Port: 8080,
        }
    case types.ExecHealthCheckType:
        base.Exec = &types.ExecHealthCheck{
            Command: []string{"/bin/sh", "-c", "exit 0"},
        }
    }
    
    return base
}

// Helper functions for creating pointers
func floatPtr(f float64) *float64 {
    return &f
}

func intOrStringPtr(s string) *intstr.IntOrString {
    v := intstr.FromString(s)
    return &v
}

func int32Ptr(i int32) *int32 {
    return &i
}
```

### 6. `pkg/deployment/testing/doc.go` (10 lines)
Package documentation.

```go
// Package testing provides test fixtures, mocks, and utilities for testing
// the deployment system. It includes sample data, mock implementations,
// and helper functions to simplify writing tests for deployment components.
package testing
```

## Implementation Steps

### Step 1: Setup Dependencies
```bash
# Ensure previous branches are available
git fetch origin feature/tmc-phase4-14a-deployment-core-types
git fetch origin feature/tmc-phase4-14b-deployment-interfaces
git fetch origin feature/tmc-phase4-14c-deployment-validation
```

### Step 2: Create Test Package Structure
```bash
mkdir -p pkg/deployment/testing
```

### Step 3: Implement Test Components
1. Start with `fixtures.go` - test data and fixtures
2. Add `mocks.go` - mock implementations
3. Create `validation_test.go` - validation tests
4. Add `types_test.go` - type tests
5. Create `helpers.go` - test utilities
6. Add `doc.go` - package documentation

### Step 4: Run Tests
```bash
go test ./pkg/deployment/...
```

### Step 5: Check Coverage
```bash
go test -cover ./pkg/deployment/...
```

## KCP Patterns to Follow

1. **Table-Driven Tests**: Use test tables for comprehensive coverage
2. **Test Fixtures**: Provide reusable test data
3. **Mock Interfaces**: Create testable mock implementations
4. **Context Usage**: Use test contexts with timeouts
5. **Cleanup**: Use t.Cleanup for resource cleanup

## Testing Requirements

### Test Coverage Targets
- [ ] Types package: 90% coverage
- [ ] Validation package: 95% coverage
- [ ] Interfaces: Mock coverage for all interfaces
- [ ] Edge cases: All boundary conditions tested

### Test Categories
- [ ] Unit tests for all validators
- [ ] Marshaling/unmarshaling tests
- [ ] State transition tests
- [ ] Cycle detection tests
- [ ] Performance benchmarks

## Integration Points

These tests will:
- **Validate**: All types from Branch 14a
- **Mock**: All interfaces from Branch 14b
- **Test**: All validators from Branch 14c
- **Support**: Testing for Branches 20-22

## Validation Checklist

- [ ] All test cases pass
- [ ] Coverage targets met
- [ ] Mocks implement all interface methods
- [ ] Test fixtures cover common scenarios
- [ ] Helper functions are reusable
- [ ] Tests are maintainable
- [ ] Performance benchmarks included
- [ ] Documentation complete
- [ ] No test pollution
- [ ] Parallel test execution safe

## Line Count Validation
Run before committing:
```bash
/workspaces/kcp-shared-tools/tmc-pr-line-counter.sh -c feature/tmc-phase4-14d-deployment-tests
```

Target: ~460 lines