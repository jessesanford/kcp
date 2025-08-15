# Implementation Instructions: Policy Framework (Branch 15)

## Overview
This branch implements the policy evaluation framework with CEL (Common Expression Language) support. It defines the abstractions for policy compilation, evaluation, and caching, enabling flexible policy-driven placement decisions.

## Dependencies
- **Base**: main branch
- **Required for**: Branch 17 (CEL evaluator implementation)
- **Used by**: Branches 7, 19, 23

## Files to Create

### 1. `pkg/policy/interfaces/evaluator.go` (50 lines)
Core policy evaluator interface.

```go
package interfaces

import (
    "context"
    "github.com/kcp-dev/kcp/pkg/policy/types"
)

// PolicyEvaluator evaluates policies against input data
type PolicyEvaluator interface {
    // Evaluate evaluates a policy against variables
    Evaluate(ctx context.Context, policy types.Policy, vars map[string]interface{}) (*EvaluationResult, error)
    
    // EvaluateBatch evaluates multiple policies
    EvaluateBatch(ctx context.Context, policies []types.Policy, vars map[string]interface{}) ([]*EvaluationResult, error)
    
    // ValidatePolicy checks if a policy is valid
    ValidatePolicy(ctx context.Context, policy types.Policy) error
}

// EvaluationResult contains the result of policy evaluation
type EvaluationResult struct {
    // PolicyName that was evaluated
    PolicyName string
    
    // Passed indicates if the policy passed
    Passed bool
    
    // Score for scoring-based policies (0-100)
    Score float64
    
    // Violations contains any policy violations
    Violations []Violation
    
    // Metadata from evaluation
    Metadata map[string]interface{}
}

// Violation represents a policy violation
type Violation struct {
    // Rule that was violated
    Rule string
    
    // Message describing the violation
    Message string
    
    // Severity of the violation
    Severity ViolationSeverity
}

// ViolationSeverity levels
type ViolationSeverity string

const (
    SeverityError   ViolationSeverity = "Error"
    SeverityWarning ViolationSeverity = "Warning"
    SeverityInfo    ViolationSeverity = "Info"
)
```

### 2. `pkg/policy/interfaces/compiler.go` (40 lines)
Policy compilation interface for CEL expressions.

```go
package interfaces

import (
    "context"
    "github.com/kcp-dev/kcp/pkg/policy/types"
)

// ExpressionCompiler compiles policy expressions
type ExpressionCompiler interface {
    // Compile compiles an expression string
    Compile(ctx context.Context, expression string) (CompiledExpression, error)
    
    // CompileWithEnv compiles with a specific environment
    CompileWithEnv(ctx context.Context, expression string, env *Environment) (CompiledExpression, error)
    
    // Validate checks expression syntax
    Validate(ctx context.Context, expression string) error
}

// CompiledExpression represents a compiled expression
type CompiledExpression interface {
    // Evaluate runs the expression with variables
    Evaluate(vars map[string]interface{}) (interface{}, error)
    
    // Type returns the result type of the expression
    Type() ExpressionType
    
    // Cost returns the computational cost
    Cost() uint64
}

// Environment defines the expression environment
type Environment struct {
    // Variables available in expressions
    Variables map[string]VariableType
    
    // Functions available in expressions
    Functions []FunctionDeclaration
    
    // Options for the environment
    Options EnvironmentOptions
}

// ExpressionType represents expression result types
type ExpressionType string

const (
    TypeBool   ExpressionType = "bool"
    TypeInt    ExpressionType = "int"
    TypeFloat  ExpressionType = "float"
    TypeString ExpressionType = "string"
    TypeList   ExpressionType = "list"
    TypeMap    ExpressionType = "map"
)
```

### 3. `pkg/policy/interfaces/cache.go` (30 lines)
Caching interface for compiled expressions.

```go
package interfaces

import (
    "context"
    "time"
)

// ExpressionCache caches compiled expressions
type ExpressionCache interface {
    // Get retrieves a cached expression
    Get(ctx context.Context, key string) (CompiledExpression, bool)
    
    // Put stores a compiled expression
    Put(ctx context.Context, key string, expr CompiledExpression, ttl time.Duration) error
    
    // Delete removes an expression from cache
    Delete(ctx context.Context, key string) error
    
    // Clear removes all cached expressions
    Clear(ctx context.Context) error
    
    // Stats returns cache statistics
    Stats(ctx context.Context) (*CacheStats, error)
}

// CacheStats contains cache statistics
type CacheStats struct {
    // Size is the number of cached items
    Size int64
    
    // Hits is the number of cache hits
    Hits int64
    
    // Misses is the number of cache misses
    Misses int64
    
    // Evictions is the number of evicted items
    Evictions int64
}
```

### 4. `pkg/policy/types/policy.go` (80 lines)
Core policy type definitions.

```go
package types

import (
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Policy defines a placement policy
type Policy struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`
    
    Spec   PolicySpec   `json:"spec"`
    Status PolicyStatus `json:"status,omitempty"`
}

// PolicySpec defines the policy specification
type PolicySpec struct {
    // Type of policy (Placement, Security, Cost, etc.)
    Type PolicyType `json:"type"`
    
    // Priority for policy evaluation (higher = more important)
    Priority int32 `json:"priority"`
    
    // Rules in the policy
    Rules []PolicyRule `json:"rules"`
    
    // MatchLabels for selecting resources
    MatchLabels map[string]string `json:"matchLabels,omitempty"`
    
    // MatchExpressions for advanced selection
    MatchExpressions []metav1.LabelSelectorRequirement `json:"matchExpressions,omitempty"`
    
    // EnforcementMode determines how policy is enforced
    EnforcementMode EnforcementMode `json:"enforcementMode"`
}

// PolicyType defines types of policies
type PolicyType string

const (
    PolicyTypePlacement   PolicyType = "Placement"
    PolicyTypeSecurity    PolicyType = "Security"
    PolicyTypeCost        PolicyType = "Cost"
    PolicyTypePerformance PolicyType = "Performance"
    PolicyTypeCompliance  PolicyType = "Compliance"
)

// PolicyRule defines a single rule in a policy
type PolicyRule struct {
    // Name of the rule
    Name string `json:"name"`
    
    // Expression is the CEL expression
    Expression string `json:"expression"`
    
    // Weight for scoring (0-100)
    Weight int32 `json:"weight,omitempty"`
    
    // Action to take when rule matches
    Action RuleAction `json:"action"`
    
    // Message for violations
    Message string `json:"message,omitempty"`
}

// RuleAction defines actions for rules
type RuleAction string

const (
    ActionAllow    RuleAction = "Allow"
    ActionDeny     RuleAction = "Deny"
    ActionWarn     RuleAction = "Warn"
    ActionScore    RuleAction = "Score"
)

// EnforcementMode defines how policies are enforced
type EnforcementMode string

const (
    EnforcementStrict   EnforcementMode = "Strict"
    EnforcementBestEffort EnforcementMode = "BestEffort"
    EnforcementDryRun   EnforcementMode = "DryRun"
)

// PolicyStatus represents the status of a policy
type PolicyStatus struct {
    // Conditions for the policy
    Conditions []metav1.Condition `json:"conditions,omitempty"`
    
    // ObservedGeneration tracks the generation
    ObservedGeneration int64 `json:"observedGeneration,omitempty"`
    
    // LastEvaluationTime of the policy
    LastEvaluationTime metav1.Time `json:"lastEvaluationTime,omitempty"`
}
```

### 5. `pkg/policy/types/expression.go` (60 lines)
Expression and variable type definitions.

```go
package types

// Expression represents a policy expression
type Expression struct {
    // Raw expression string
    Raw string `json:"raw"`
    
    // Type of expression (CEL, Rego, etc.)
    Type ExpressionLanguage `json:"type"`
    
    // Variables used in the expression
    Variables []Variable `json:"variables,omitempty"`
    
    // Expected result type
    ResultType string `json:"resultType"`
    
    // Estimated cost of evaluation
    Cost uint64 `json:"cost,omitempty"`
}

// ExpressionLanguage defines supported expression languages
type ExpressionLanguage string

const (
    LanguageCEL  ExpressionLanguage = "CEL"
    LanguageRego ExpressionLanguage = "Rego"
)

// Variable represents a variable in expressions
type Variable struct {
    // Name of the variable
    Name string `json:"name"`
    
    // Type of the variable
    Type VariableType `json:"type"`
    
    // Description of the variable
    Description string `json:"description,omitempty"`
    
    // Required indicates if variable must be provided
    Required bool `json:"required"`
    
    // Default value if not provided
    Default interface{} `json:"default,omitempty"`
}

// VariableType defines variable types
type VariableType string

const (
    VarTypeBool      VariableType = "bool"
    VarTypeInt       VariableType = "int"
    VarTypeFloat     VariableType = "float"
    VarTypeString    VariableType = "string"
    VarTypeList      VariableType = "list"
    VarTypeMap       VariableType = "map"
    VarTypeCluster   VariableType = "cluster"
    VarTypeWorkload  VariableType = "workload"
    VarTypeWorkspace VariableType = "workspace"
)

// ExpressionContext provides context for expression evaluation
type ExpressionContext struct {
    // Workspace being evaluated
    Workspace string
    
    // Cluster being evaluated
    Cluster string
    
    // User performing the evaluation
    User string
    
    // Additional context data
    Data map[string]interface{}
}
```

### 6. `pkg/policy/cel/functions.go` (100 lines)
Custom CEL functions for policy evaluation.

```go
package cel

import (
    "github.com/google/cel-go/cel"
    "github.com/google/cel-go/common/types"
    "github.com/google/cel-go/common/types/ref"
    "github.com/google/cel-go/interpreter/functions"
)

// RegisterCustomFunctions registers TMC-specific CEL functions
func RegisterCustomFunctions() cel.EnvOption {
    return cel.Lib(customLibrary{})
}

type customLibrary struct{}

func (customLibrary) CompileOptions() []cel.EnvOption {
    return []cel.EnvOption{
        cel.Function("hasLabel",
            cel.Overload("hasLabel_string",
                []*cel.Type{cel.MapType(cel.StringType, cel.StringType), cel.StringType},
                cel.BoolType,
                cel.BinaryBinding(hasLabel),
            ),
        ),
        cel.Function("matchesRegex",
            cel.Overload("matchesRegex_string_string",
                []*cel.Type{cel.StringType, cel.StringType},
                cel.BoolType,
                cel.BinaryBinding(matchesRegex),
            ),
        ),
        cel.Function("inNamespace",
            cel.Overload("inNamespace_string",
                []*cel.Type{cel.StringType},
                cel.BoolType,
                cel.UnaryBinding(inNamespace),
            ),
        ),
        cel.Function("hasCapacity",
            cel.Overload("hasCapacity_int",
                []*cel.Type{cel.IntType},
                cel.BoolType,
                cel.UnaryBinding(hasCapacity),
            ),
        ),
        cel.Function("costPerHour",
            cel.Overload("costPerHour_void",
                []*cel.Type{},
                cel.DoubleType,
                cel.FunctionBinding(costPerHour),
            ),
        ),
        cel.Function("distance",
            cel.Overload("distance_string_string",
                []*cel.Type{cel.StringType, cel.StringType},
                cel.DoubleType,
                cel.BinaryBinding(distance),
            ),
        ),
    }
}

func (customLibrary) ProgramOptions() []cel.ProgramOption {
    return []cel.ProgramOption{}
}

// hasLabel checks if a label exists in a map
func hasLabel(labels, key ref.Val) ref.Val {
    labelsMap, ok := labels.(types.Map)
    if !ok {
        return types.False
    }
    
    keyStr, ok := key.(types.String)
    if !ok {
        return types.False
    }
    
    return types.Bool(labelsMap.Contains(keyStr))
}

// matchesRegex checks if a string matches a regex pattern
func matchesRegex(str, pattern ref.Val) ref.Val {
    // Implementation would compile and match regex
    return types.Bool(true)
}

// inNamespace checks if resource is in specified namespace
func inNamespace(namespace ref.Val) ref.Val {
    // Implementation would check namespace
    return types.Bool(true)
}

// hasCapacity checks if cluster has required capacity
func hasCapacity(required ref.Val) ref.Val {
    // Implementation would check cluster capacity
    return types.Bool(true)
}

// costPerHour returns the cost per hour for a cluster
func costPerHour() ref.Val {
    // Implementation would calculate cost
    return types.Double(0.5)
}

// distance calculates distance between two locations
func distance(from, to ref.Val) ref.Val {
    // Implementation would calculate geographical distance
    return types.Double(100.0)
}
```

### 7. `pkg/policy/cel/doc.go` (10 lines)
Package documentation.

```go
// Package cel provides CEL (Common Expression Language) support for policy evaluation.
// It includes custom functions specific to TMC placement decisions and integrates
// with the policy framework to enable flexible, expression-based policies.
package cel
```

## Implementation Steps

### Step 1: Create Package Structure
```bash
mkdir -p pkg/policy/interfaces
mkdir -p pkg/policy/types
mkdir -p pkg/policy/cel
```

### Step 2: Implement Core Interfaces
1. Start with `interfaces/evaluator.go` - evaluation interface
2. Add `interfaces/compiler.go` - compilation interface
3. Create `interfaces/cache.go` - caching interface

### Step 3: Define Types
1. Create `types/policy.go` - policy definitions
2. Add `types/expression.go` - expression types

### Step 4: Add CEL Functions
1. Create `cel/functions.go` - custom CEL functions
2. Add `cel/doc.go` - package documentation

### Step 5: Add Validation
Create validation for policy expressions and rules.

## KCP Patterns to Follow

1. **Workspace Isolation**: Policies respect workspace boundaries
2. **RBAC Integration**: Policy evaluation considers permissions
3. **Condition Patterns**: Use standard Kubernetes conditions
4. **Label Selectors**: Follow Kubernetes selector patterns
5. **Resource Typing**: Use TypeMeta for policy resources

## Testing Requirements

### Unit Tests Required
- [ ] Policy compilation tests
- [ ] Expression evaluation tests
- [ ] Cache functionality tests
- [ ] Custom function tests
- [ ] Policy matching tests

### Integration Tests
- [ ] Multi-policy evaluation
- [ ] Complex expression evaluation
- [ ] Performance benchmarks

## Integration Points

This framework will be:
- **Implemented by**: Branch 17 (CEL evaluator)
- **Used by**: Branch 7 (placement decisions)
- **Used by**: Branch 19 (controller)
- **Tested in**: Branch 23 (integration)

## Validation Checklist

- [ ] All interfaces are mockable
- [ ] CEL functions are documented
- [ ] Policy types follow K8s conventions
- [ ] Expression compilation is cached
- [ ] Thread-safe implementation
- [ ] Performance optimized
- [ ] Error messages are clear
- [ ] Feature flag integration ready
- [ ] Workspace awareness included
- [ ] Documentation complete

## Line Count Validation
Run before committing:
```bash
/workspaces/kcp-shared-tools/tmc-pr-line-counter.sh -c feature/tmc-phase4-15-policy-framework
```

Target: ~360 lines