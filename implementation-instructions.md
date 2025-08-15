# Implementation Instructions: Deployment Core Types (Branch 14a)

## Overview
This branch implements the core type definitions for the deployment system. It provides the foundational data structures that will be used throughout the deployment coordination system, including strategy types, deployment states, and health check definitions.

## Dependencies
- **Base**: main branch
- **Required for**: Branches 14b, 14c, 14d, 20, 21, 22

## Files to Create

### 1. `pkg/deployment/types/strategy.go` (100 lines)
Core deployment strategy type definitions.

```go
package types

import (
    "time"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeploymentStrategy defines how a workload should be deployed
type DeploymentStrategy struct {
    // Type of deployment strategy (Rolling, Canary, BlueGreen)
    Type DeploymentType `json:"type"`
    
    // RollingUpdate configuration
    RollingUpdate *RollingUpdateStrategy `json:"rollingUpdate,omitempty"`
    
    // Canary configuration
    Canary *CanaryStrategy `json:"canary,omitempty"`
    
    // BlueGreen configuration
    BlueGreen *BlueGreenStrategy `json:"blueGreen,omitempty"`
}

// DeploymentType represents the type of deployment strategy
type DeploymentType string

const (
    RollingDeploymentType   DeploymentType = "Rolling"
    CanaryDeploymentType    DeploymentType = "Canary"
    BlueGreenDeploymentType DeploymentType = "BlueGreen"
)

// RollingUpdateStrategy defines rolling update parameters
type RollingUpdateStrategy struct {
    MaxUnavailable *intstr.IntOrString `json:"maxUnavailable,omitempty"`
    MaxSurge       *intstr.IntOrString `json:"maxSurge,omitempty"`
}

// CanaryStrategy defines canary deployment parameters
type CanaryStrategy struct {
    Steps    []CanaryStep    `json:"steps"`
    Analysis *CanaryAnalysis `json:"analysis,omitempty"`
}

// CanaryStep defines a single step in canary deployment
type CanaryStep struct {
    Weight int           `json:"weight"`
    Pause  time.Duration `json:"pause,omitempty"`
}

// CanaryAnalysis defines metrics for canary analysis
type CanaryAnalysis struct {
    Metrics   []Metric `json:"metrics"`
    Threshold float64  `json:"threshold"`
}

// BlueGreenStrategy defines blue-green deployment parameters
type BlueGreenStrategy struct {
    AutoPromotionEnabled bool          `json:"autoPromotionEnabled"`
    AutoPromotionSeconds int32         `json:"autoPromotionSeconds,omitempty"`
    ScaleDownDelaySeconds int32        `json:"scaleDownDelaySeconds,omitempty"`
    PrePromotionAnalysis  *Analysis    `json:"prePromotionAnalysis,omitempty"`
    PostPromotionAnalysis *Analysis    `json:"postPromotionAnalysis,omitempty"`
}
```

### 2. `pkg/deployment/types/state.go` (80 lines)
Deployment state and status definitions.

```go
package types

import (
    "time"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeploymentState represents the current state of a deployment
type DeploymentState string

const (
    DeploymentStatePending     DeploymentState = "Pending"
    DeploymentStateProgressing DeploymentState = "Progressing"
    DeploymentStatePaused      DeploymentState = "Paused"
    DeploymentStateCompleted   DeploymentState = "Completed"
    DeploymentStateFailed      DeploymentState = "Failed"
    DeploymentStateRollingBack DeploymentState = "RollingBack"
)

// DeploymentStatus captures the current status of a deployment
type DeploymentStatus struct {
    // Current state of the deployment
    State DeploymentState `json:"state"`
    
    // Message providing details about the state
    Message string `json:"message,omitempty"`
    
    // Current step in multi-step deployments
    CurrentStep int `json:"currentStep,omitempty"`
    
    // Total steps in multi-step deployments
    TotalSteps int `json:"totalSteps,omitempty"`
    
    // Replicas status
    Replicas ReplicaStatus `json:"replicas,omitempty"`
    
    // Conditions for the deployment
    Conditions []metav1.Condition `json:"conditions,omitempty"`
    
    // LastUpdateTime is the last time the status was updated
    LastUpdateTime metav1.Time `json:"lastUpdateTime"`
}

// ReplicaStatus tracks replica counts
type ReplicaStatus struct {
    Total     int32 `json:"total"`
    Updated   int32 `json:"updated"`
    Ready     int32 `json:"ready"`
    Available int32 `json:"available"`
}

// DeploymentPhase represents phases of deployment
type DeploymentPhase string

const (
    DeploymentPhaseAnalysis    DeploymentPhase = "Analysis"
    DeploymentPhasePromotion   DeploymentPhase = "Promotion"
    DeploymentPhaseRollback    DeploymentPhase = "Rollback"
    DeploymentPhaseCompleted   DeploymentPhase = "Completed"
)
```

### 3. `pkg/deployment/types/health.go` (70 lines)
Health check and readiness definitions.

```go
package types

import (
    "time"
)

// HealthCheck defines health checking configuration
type HealthCheck struct {
    // Type of health check (HTTP, TCP, Exec)
    Type HealthCheckType `json:"type"`
    
    // HTTP health check configuration
    HTTP *HTTPHealthCheck `json:"http,omitempty"`
    
    // TCP health check configuration
    TCP *TCPHealthCheck `json:"tcp,omitempty"`
    
    // Exec health check configuration
    Exec *ExecHealthCheck `json:"exec,omitempty"`
    
    // Common health check parameters
    InitialDelaySeconds int32 `json:"initialDelaySeconds,omitempty"`
    PeriodSeconds       int32 `json:"periodSeconds,omitempty"`
    TimeoutSeconds      int32 `json:"timeoutSeconds,omitempty"`
    SuccessThreshold    int32 `json:"successThreshold,omitempty"`
    FailureThreshold    int32 `json:"failureThreshold,omitempty"`
}

// HealthCheckType defines the type of health check
type HealthCheckType string

const (
    HTTPHealthCheckType HealthCheckType = "HTTP"
    TCPHealthCheckType  HealthCheckType = "TCP"
    ExecHealthCheckType HealthCheckType = "Exec"
)

// HTTPHealthCheck defines HTTP health check parameters
type HTTPHealthCheck struct {
    Path   string            `json:"path"`
    Port   int32             `json:"port"`
    Scheme string            `json:"scheme,omitempty"`
    Headers map[string]string `json:"headers,omitempty"`
}

// TCPHealthCheck defines TCP health check parameters
type TCPHealthCheck struct {
    Port int32 `json:"port"`
}

// ExecHealthCheck defines command-based health check
type ExecHealthCheck struct {
    Command []string `json:"command"`
}

// HealthStatus represents the health status of a deployment
type HealthStatus struct {
    Healthy   bool      `json:"healthy"`
    Message   string    `json:"message,omitempty"`
    LastCheck time.Time `json:"lastCheck"`
}
```

### 4. `pkg/deployment/types/dependency.go` (60 lines)
Dependency graph type definitions.

```go
package types

import (
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeploymentDependency represents a dependency between deployments
type DeploymentDependency struct {
    // Name of the dependent deployment
    Name string `json:"name"`
    
    // Namespace of the dependent deployment
    Namespace string `json:"namespace,omitempty"`
    
    // Type of dependency (hard or soft)
    Type DependencyType `json:"type"`
    
    // Condition that must be met
    Condition DependencyCondition `json:"condition"`
}

// DependencyType defines the type of dependency
type DependencyType string

const (
    HardDependency DependencyType = "Hard"
    SoftDependency DependencyType = "Soft"
)

// DependencyCondition defines when a dependency is satisfied
type DependencyCondition string

const (
    DependencyConditionReady     DependencyCondition = "Ready"
    DependencyConditionProgressing DependencyCondition = "Progressing"
    DependencyConditionCompleted  DependencyCondition = "Completed"
)

// DependencyGraph represents the deployment dependency graph
type DependencyGraph struct {
    // Nodes in the dependency graph
    Nodes map[string]*DeploymentNode `json:"nodes"`
    
    // Edges representing dependencies
    Edges []DependencyEdge `json:"edges"`
}

// DeploymentNode represents a node in the dependency graph
type DeploymentNode struct {
    Name         string                 `json:"name"`
    Namespace    string                 `json:"namespace"`
    Dependencies []DeploymentDependency `json:"dependencies,omitempty"`
    Status       DeploymentStatus       `json:"status,omitempty"`
}

// DependencyEdge represents an edge in the dependency graph
type DependencyEdge struct {
    From string         `json:"from"`
    To   string         `json:"to"`
    Type DependencyType `json:"type"`
}
```

### 5. `pkg/deployment/types/metrics.go` (50 lines)
Metric definitions for deployment analysis.

```go
package types

// Metric defines a metric for deployment analysis
type Metric struct {
    // Name of the metric
    Name string `json:"name"`
    
    // Query for the metric (Prometheus query language)
    Query string `json:"query,omitempty"`
    
    // Threshold for the metric
    Threshold MetricThreshold `json:"threshold"`
    
    // Weight of this metric in overall analysis
    Weight float64 `json:"weight,omitempty"`
}

// MetricThreshold defines threshold for metrics
type MetricThreshold struct {
    // Maximum allowed value
    Max *float64 `json:"max,omitempty"`
    
    // Minimum required value
    Min *float64 `json:"min,omitempty"`
}

// MetricResult represents the result of metric evaluation
type MetricResult struct {
    // Name of the metric
    Name string `json:"name"`
    
    // Current value of the metric
    Value float64 `json:"value"`
    
    // Whether the metric passed threshold check
    Passed bool `json:"passed"`
    
    // Error if metric evaluation failed
    Error string `json:"error,omitempty"`
}

// Analysis configuration for deployments
type Analysis struct {
    // Metrics to evaluate
    Metrics []Metric `json:"metrics"`
    
    // Minimum successful metrics required
    SuccessfulMetricsRequired int `json:"successfulMetricsRequired,omitempty"`
}
```

### 6. `pkg/deployment/types/doc.go` (10 lines)
Package documentation.

```go
// Package types provides core type definitions for the deployment system.
// These types form the foundation for deployment strategies, health checks,
// dependency management, and deployment analysis across the TMC system.
package types
```

## Implementation Steps

### Step 1: Create Package Structure
```bash
mkdir -p pkg/deployment/types
```

### Step 2: Implement Core Types
1. Start with `strategy.go` - define all deployment strategy types
2. Add `state.go` - implement state and status tracking
3. Create `health.go` - add health check definitions
4. Add `dependency.go` - implement dependency graph types
5. Create `metrics.go` - add metric analysis types
6. Add `doc.go` - package documentation

### Step 3: Add Validation Tags
Add validation tags to all structs using standard Kubernetes validation:
- Required fields: `json:"field" validate:"required"`
- Optional fields: `json:"field,omitempty"`
- Enums: Use typed constants

### Step 4: Generate DeepCopy Methods
```bash
# Add deepcopy generation markers
# Run code generation
make generate
```

### Step 5: Add Unit Tests
Create `pkg/deployment/types/types_test.go` with:
- Validation tests for all types
- JSON marshaling/unmarshaling tests
- DeepCopy verification tests

## KCP Patterns to Follow

1. **Type Safety**: Use typed strings for enums
2. **Kubernetes Conventions**: Follow K8s API conventions for types
3. **Validation**: Add kubebuilder validation markers
4. **Documentation**: Document all exported types and fields
5. **Immutability**: Design types to be immutable where possible

## Testing Requirements

### Unit Tests Required
- [ ] Type validation tests
- [ ] JSON serialization tests  
- [ ] DeepCopy tests
- [ ] Default value tests

### Test Coverage Target
- Minimum 80% code coverage
- 100% coverage for validation logic

## Integration Points

These types will be used by:
- **Branch 14b**: Deployment interfaces will use these types
- **Branch 14c**: Validation logic will validate these types
- **Branch 14d**: Tests will verify these types
- **Branch 20**: Canary strategy will extend these types
- **Branch 21**: Dependency graph will use dependency types
- **Branch 22**: Rollback will use state types

## Validation Checklist

- [ ] All types have proper JSON tags
- [ ] Validation tags added where needed
- [ ] DeepCopy methods generated
- [ ] Documentation for all exported types
- [ ] Unit tests achieve 80% coverage
- [ ] No circular dependencies
- [ ] Follows Kubernetes API conventions
- [ ] Compatible with client-go serialization
- [ ] Thread-safe design
- [ ] Feature flag integration points identified

## Line Count Validation
Run before committing:
```bash
/workspaces/kcp-shared-tools/tmc-pr-line-counter.sh -c feature/tmc-phase4-14a-deployment-core-types
```

Target: ~370 lines (excluding generated code)