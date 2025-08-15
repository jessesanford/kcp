# Implementation Instructions: Deployment Validation (Branch 14c)

## Overview
This branch implements comprehensive validation logic for deployment configurations, strategies, and dependencies. It ensures that all deployment plans are valid before execution and provides detailed validation error messages for debugging.

## Dependencies
- **Base**: feature/tmc-phase4-14b-deployment-interfaces
- **Uses types from**: Branch 14a (core types)
- **Required for**: Branch 14d (tests), Branches 20-22 (implementations)

## Files to Create

### 1. `pkg/deployment/validation/strategy_validator.go` (100 lines)
Validates deployment strategies and their configurations.

```go
package validation

import (
    "fmt"
    "github.com/kcp-dev/kcp/pkg/deployment/types"
    "k8s.io/apimachinery/pkg/util/validation/field"
)

// StrategyValidator validates deployment strategies
type StrategyValidator struct {
    // Minimum and maximum values for validations
    minWeight int
    maxWeight int
    maxSteps  int
}

// NewStrategyValidator creates a new strategy validator
func NewStrategyValidator() *StrategyValidator {
    return &StrategyValidator{
        minWeight: 0,
        maxWeight: 100,
        maxSteps:  10,
    }
}

// ValidateStrategy validates a deployment strategy
func (v *StrategyValidator) ValidateStrategy(strategy types.DeploymentStrategy) field.ErrorList {
    allErrs := field.ErrorList{}
    fldPath := field.NewPath("strategy")
    
    // Validate strategy type
    if strategy.Type == "" {
        allErrs = append(allErrs, field.Required(fldPath.Child("type"), "strategy type is required"))
    }
    
    // Validate based on strategy type
    switch strategy.Type {
    case types.CanaryDeploymentType:
        allErrs = append(allErrs, v.validateCanaryStrategy(strategy.Canary, fldPath.Child("canary"))...)
    case types.RollingDeploymentType:
        allErrs = append(allErrs, v.validateRollingStrategy(strategy.RollingUpdate, fldPath.Child("rollingUpdate"))...)
    case types.BlueGreenDeploymentType:
        allErrs = append(allErrs, v.validateBlueGreenStrategy(strategy.BlueGreen, fldPath.Child("blueGreen"))...)
    default:
        allErrs = append(allErrs, field.Invalid(fldPath.Child("type"), strategy.Type, "unknown strategy type"))
    }
    
    return allErrs
}

// validateCanaryStrategy validates canary deployment configuration
func (v *StrategyValidator) validateCanaryStrategy(canary *types.CanaryStrategy, fldPath *field.Path) field.ErrorList {
    allErrs := field.ErrorList{}
    
    if canary == nil {
        return append(allErrs, field.Required(fldPath, "canary configuration is required for canary strategy"))
    }
    
    // Validate steps
    if len(canary.Steps) == 0 {
        allErrs = append(allErrs, field.Required(fldPath.Child("steps"), "at least one step is required"))
    }
    
    if len(canary.Steps) > v.maxSteps {
        allErrs = append(allErrs, field.TooMany(fldPath.Child("steps"), len(canary.Steps), v.maxSteps))
    }
    
    totalWeight := 0
    for i, step := range canary.Steps {
        stepPath := fldPath.Child("steps").Index(i)
        
        if step.Weight < v.minWeight || step.Weight > v.maxWeight {
            allErrs = append(allErrs, field.Invalid(stepPath.Child("weight"), step.Weight, 
                fmt.Sprintf("weight must be between %d and %d", v.minWeight, v.maxWeight)))
        }
        
        totalWeight += step.Weight
        
        if step.Pause < 0 {
            allErrs = append(allErrs, field.Invalid(stepPath.Child("pause"), step.Pause, "pause cannot be negative"))
        }
    }
    
    // Validate analysis if present
    if canary.Analysis != nil {
        allErrs = append(allErrs, v.validateAnalysis(canary.Analysis, fldPath.Child("analysis"))...)
    }
    
    return allErrs
}
```

### 2. `pkg/deployment/validation/dependency_validator.go` (80 lines)
Validates deployment dependencies and detects cycles.

```go
package validation

import (
    "fmt"
    "github.com/kcp-dev/kcp/pkg/deployment/types"
    "k8s.io/apimachinery/pkg/util/validation/field"
)

// DependencyValidator validates deployment dependencies
type DependencyValidator struct {
    maxDependencies int
    maxDepth        int
}

// NewDependencyValidator creates a new dependency validator
func NewDependencyValidator() *DependencyValidator {
    return &DependencyValidator{
        maxDependencies: 20,
        maxDepth:        5,
    }
}

// ValidateDependencyGraph validates a dependency graph
func (v *DependencyValidator) ValidateDependencyGraph(graph types.DependencyGraph) field.ErrorList {
    allErrs := field.ErrorList{}
    fldPath := field.NewPath("dependencyGraph")
    
    // Check for cycles
    if cycles := v.detectCycles(graph); len(cycles) > 0 {
        for _, cycle := range cycles {
            allErrs = append(allErrs, field.Invalid(fldPath, cycle, "circular dependency detected"))
        }
    }
    
    // Validate individual nodes
    for name, node := range graph.Nodes {
        nodePath := fldPath.Child("nodes").Key(name)
        allErrs = append(allErrs, v.validateNode(node, nodePath)...)
    }
    
    // Validate edges
    for i, edge := range graph.Edges {
        edgePath := fldPath.Child("edges").Index(i)
        allErrs = append(allErrs, v.validateEdge(edge, graph.Nodes, edgePath)...)
    }
    
    return allErrs
}

// detectCycles detects circular dependencies in the graph
func (v *DependencyValidator) detectCycles(graph types.DependencyGraph) []string {
    cycles := []string{}
    visited := make(map[string]bool)
    recStack := make(map[string]bool)
    
    for node := range graph.Nodes {
        if !visited[node] {
            if v.hasCycleDFS(node, graph, visited, recStack) {
                cycles = append(cycles, fmt.Sprintf("cycle detected starting from %s", node))
            }
        }
    }
    
    return cycles
}

// hasCycleDFS performs depth-first search to detect cycles
func (v *DependencyValidator) hasCycleDFS(node string, graph types.DependencyGraph, 
    visited, recStack map[string]bool) bool {
    visited[node] = true
    recStack[node] = true
    
    // Check all edges from this node
    for _, edge := range graph.Edges {
        if edge.From == node {
            if !visited[edge.To] {
                if v.hasCycleDFS(edge.To, graph, visited, recStack) {
                    return true
                }
            } else if recStack[edge.To] {
                return true
            }
        }
    }
    
    recStack[node] = false
    return false
}
```

### 3. `pkg/deployment/validation/health_validator.go` (70 lines)
Validates health check configurations.

```go
package validation

import (
    "github.com/kcp-dev/kcp/pkg/deployment/types"
    "k8s.io/apimachinery/pkg/util/validation/field"
    "net/url"
)

// HealthValidator validates health check configurations
type HealthValidator struct {
    minTimeout int32
    maxTimeout int32
    minPeriod  int32
}

// NewHealthValidator creates a new health validator
func NewHealthValidator() *HealthValidator {
    return &HealthValidator{
        minTimeout: 1,
        maxTimeout: 300,
        minPeriod:  1,
    }
}

// ValidateHealthCheck validates a health check configuration
func (v *HealthValidator) ValidateHealthCheck(hc types.HealthCheck) field.ErrorList {
    allErrs := field.ErrorList{}
    fldPath := field.NewPath("healthCheck")
    
    // Validate health check type
    if hc.Type == "" {
        allErrs = append(allErrs, field.Required(fldPath.Child("type"), "health check type is required"))
    }
    
    // Validate type-specific configuration
    switch hc.Type {
    case types.HTTPHealthCheckType:
        allErrs = append(allErrs, v.validateHTTPHealthCheck(hc.HTTP, fldPath.Child("http"))...)
    case types.TCPHealthCheckType:
        allErrs = append(allErrs, v.validateTCPHealthCheck(hc.TCP, fldPath.Child("tcp"))...)
    case types.ExecHealthCheckType:
        allErrs = append(allErrs, v.validateExecHealthCheck(hc.Exec, fldPath.Child("exec"))...)
    default:
        allErrs = append(allErrs, field.Invalid(fldPath.Child("type"), hc.Type, "unknown health check type"))
    }
    
    // Validate common parameters
    allErrs = append(allErrs, v.validateCommonParams(hc, fldPath)...)
    
    return allErrs
}

// validateHTTPHealthCheck validates HTTP health check
func (v *HealthValidator) validateHTTPHealthCheck(http *types.HTTPHealthCheck, fldPath *field.Path) field.ErrorList {
    allErrs := field.ErrorList{}
    
    if http == nil {
        return append(allErrs, field.Required(fldPath, "HTTP configuration is required for HTTP health check"))
    }
    
    // Validate path
    if http.Path == "" {
        allErrs = append(allErrs, field.Required(fldPath.Child("path"), "path is required"))
    } else if _, err := url.Parse(http.Path); err != nil {
        allErrs = append(allErrs, field.Invalid(fldPath.Child("path"), http.Path, "invalid URL path"))
    }
    
    // Validate port
    if http.Port <= 0 || http.Port > 65535 {
        allErrs = append(allErrs, field.Invalid(fldPath.Child("port"), http.Port, "port must be between 1 and 65535"))
    }
    
    // Validate scheme
    if http.Scheme != "" && http.Scheme != "HTTP" && http.Scheme != "HTTPS" {
        allErrs = append(allErrs, field.Invalid(fldPath.Child("scheme"), http.Scheme, "scheme must be HTTP or HTTPS"))
    }
    
    return allErrs
}
```

### 4. `pkg/deployment/validation/plan_validator.go` (80 lines)
Validates complete deployment plans.

```go
package validation

import (
    "github.com/kcp-dev/kcp/pkg/deployment/interfaces"
    "k8s.io/apimachinery/pkg/util/validation/field"
)

// PlanValidator validates deployment plans
type PlanValidator struct {
    strategyValidator    *StrategyValidator
    dependencyValidator  *DependencyValidator
    healthValidator      *HealthValidator
    maxSteps            int
}

// NewPlanValidator creates a new plan validator
func NewPlanValidator() *PlanValidator {
    return &PlanValidator{
        strategyValidator:   NewStrategyValidator(),
        dependencyValidator: NewDependencyValidator(),
        healthValidator:     NewHealthValidator(),
        maxSteps:           50,
    }
}

// ValidateDeploymentPlan validates a complete deployment plan
func (v *PlanValidator) ValidateDeploymentPlan(plan interfaces.DeploymentPlan) field.ErrorList {
    allErrs := field.ErrorList{}
    fldPath := field.NewPath("deploymentPlan")
    
    // Validate ID
    if plan.ID == "" {
        allErrs = append(allErrs, field.Required(fldPath.Child("id"), "deployment ID is required"))
    }
    
    // Validate strategy
    allErrs = append(allErrs, v.strategyValidator.ValidateStrategy(plan.Strategy)...)
    
    // Validate steps
    if len(plan.Steps) == 0 {
        allErrs = append(allErrs, field.Required(fldPath.Child("steps"), "at least one step is required"))
    }
    
    if len(plan.Steps) > v.maxSteps {
        allErrs = append(allErrs, field.TooMany(fldPath.Child("steps"), len(plan.Steps), v.maxSteps))
    }
    
    for i, step := range plan.Steps {
        stepPath := fldPath.Child("steps").Index(i)
        allErrs = append(allErrs, v.validateStep(step, stepPath)...)
    }
    
    // Validate dependencies if present
    if len(plan.Dependencies) > 0 {
        for i, dep := range plan.Dependencies {
            depPath := fldPath.Child("dependencies").Index(i)
            allErrs = append(allErrs, v.validateDependency(dep, depPath)...)
        }
    }
    
    // Validate rollback plan if present
    if plan.RollbackPlan != nil {
        allErrs = append(allErrs, v.validateRollbackPlan(plan.RollbackPlan, fldPath.Child("rollbackPlan"))...)
    }
    
    return allErrs
}

// validateStep validates a deployment step
func (v *PlanValidator) validateStep(step interfaces.DeploymentStep, fldPath *field.Path) field.ErrorList {
    allErrs := field.ErrorList{}
    
    if step.Name == "" {
        allErrs = append(allErrs, field.Required(fldPath.Child("name"), "step name is required"))
    }
    
    if step.Target == "" {
        allErrs = append(allErrs, field.Required(fldPath.Child("target"), "step target is required"))
    }
    
    if step.Timeout <= 0 {
        allErrs = append(allErrs, field.Invalid(fldPath.Child("timeout"), step.Timeout, "timeout must be positive"))
    }
    
    return allErrs
}
```

### 5. `pkg/deployment/validation/metrics_validator.go` (60 lines)
Validates metric configurations for analysis.

```go
package validation

import (
    "github.com/kcp-dev/kcp/pkg/deployment/types"
    "k8s.io/apimachinery/pkg/util/validation/field"
    "regexp"
)

// MetricsValidator validates metric configurations
type MetricsValidator struct {
    queryPattern *regexp.Regexp
}

// NewMetricsValidator creates a new metrics validator
func NewMetricsValidator() *MetricsValidator {
    return &MetricsValidator{
        queryPattern: regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*(\{[^}]*\})?(\[[0-9]+[smhd]\])?$`),
    }
}

// ValidateMetric validates a single metric configuration
func (v *MetricsValidator) ValidateMetric(metric types.Metric) field.ErrorList {
    allErrs := field.ErrorList{}
    fldPath := field.NewPath("metric")
    
    // Validate name
    if metric.Name == "" {
        allErrs = append(allErrs, field.Required(fldPath.Child("name"), "metric name is required"))
    }
    
    // Validate query if present
    if metric.Query != "" {
        if !v.queryPattern.MatchString(metric.Query) {
            allErrs = append(allErrs, field.Invalid(fldPath.Child("query"), metric.Query, 
                "invalid Prometheus query syntax"))
        }
    }
    
    // Validate threshold
    allErrs = append(allErrs, v.validateThreshold(metric.Threshold, fldPath.Child("threshold"))...)
    
    // Validate weight
    if metric.Weight < 0 || metric.Weight > 1 {
        allErrs = append(allErrs, field.Invalid(fldPath.Child("weight"), metric.Weight, 
            "weight must be between 0 and 1"))
    }
    
    return allErrs
}

// validateThreshold validates metric threshold
func (v *MetricsValidator) validateThreshold(threshold types.MetricThreshold, fldPath *field.Path) field.ErrorList {
    allErrs := field.ErrorList{}
    
    // At least one threshold must be set
    if threshold.Min == nil && threshold.Max == nil {
        allErrs = append(allErrs, field.Required(fldPath, "at least one threshold (min or max) must be set"))
    }
    
    // If both are set, min must be less than max
    if threshold.Min != nil && threshold.Max != nil {
        if *threshold.Min >= *threshold.Max {
            allErrs = append(allErrs, field.Invalid(fldPath, threshold, "min must be less than max"))
        }
    }
    
    return allErrs
}
```

### 6. `pkg/deployment/validation/doc.go` (10 lines)
Package documentation.

```go
// Package validation provides comprehensive validation for deployment configurations,
// strategies, dependencies, and health checks. It ensures all deployment plans are
// valid before execution and provides detailed error messages for debugging.
package validation
```

## Implementation Steps

### Step 1: Setup Dependencies
```bash
# Ensure branches 14a and 14b are available
git fetch origin feature/tmc-phase4-14a-deployment-core-types
git fetch origin feature/tmc-phase4-14b-deployment-interfaces
```

### Step 2: Create Package Structure
```bash
mkdir -p pkg/deployment/validation
```

### Step 3: Implement Validators
1. Start with `strategy_validator.go` - strategy validation
2. Add `dependency_validator.go` - dependency and cycle detection
3. Create `health_validator.go` - health check validation
4. Add `plan_validator.go` - complete plan validation
5. Create `metrics_validator.go` - metric configuration validation
6. Add `doc.go` - package documentation

### Step 4: Add Validation Tests
Create comprehensive test coverage for each validator.

### Step 5: Add Benchmarks
Add performance benchmarks for cycle detection and large plan validation.

## KCP Patterns to Follow

1. **Field Path Errors**: Use field.Path for precise error locations
2. **Error Aggregation**: Collect all errors, don't fail fast
3. **Descriptive Messages**: Provide actionable error messages
4. **Validation Depth**: Validate nested structures completely
5. **Performance**: Optimize for large graphs and plans

## Testing Requirements

### Unit Tests Required
- [ ] Strategy validation tests (all types)
- [ ] Cycle detection tests
- [ ] Health check validation tests
- [ ] Plan validation tests
- [ ] Metric validation tests
- [ ] Edge case tests

### Performance Tests
- [ ] Large dependency graph validation
- [ ] Complex strategy validation
- [ ] Benchmark cycle detection

## Integration Points

This validation will be:
- **Used by**: All deployment-related branches (20-22)
- **Tested by**: Branch 14d
- **Called by**: Branch 19 (Controller)

## Validation Checklist

- [ ] All validators handle nil inputs gracefully
- [ ] Error messages are descriptive and actionable
- [ ] Field paths are accurate
- [ ] Performance optimized for large inputs
- [ ] Thread-safe implementation
- [ ] Comprehensive test coverage
- [ ] Documentation complete
- [ ] Follows K8s validation patterns
- [ ] Compatible with KCP error handling
- [ ] Feature flag aware

## Line Count Validation
Run before committing:
```bash
/workspaces/kcp-shared-tools/tmc-pr-line-counter.sh -c feature/tmc-phase4-14c-deployment-validation
```

Target: ~390 lines