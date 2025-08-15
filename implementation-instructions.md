# Implementation Instructions: CEL Evaluator (Branch 17)

## Overview
This branch implements the CEL (Common Expression Language) based policy evaluation engine. It provides compilation, evaluation, caching, and custom TMC-specific functions for flexible policy expressions.

## Dependencies
- **Base**: feature/tmc-phase4-15-policy-framework
- **Uses interfaces from**: Branch 15
- **Required for**: Branches 19, 23

## Files to Create

### 1. `pkg/policy/cel/evaluator.go` (120 lines)
Main CEL evaluator implementation.

```go
package cel

import (
    "context"
    "fmt"
    "github.com/google/cel-go/cel"
    "github.com/kcp-dev/kcp/pkg/policy/interfaces"
    "github.com/kcp-dev/kcp/pkg/policy/types"
)

// Evaluator implements CEL-based policy evaluation
type Evaluator struct {
    compiler *Compiler
    cache    interfaces.ExpressionCache
    env      *cel.Env
}

// NewEvaluator creates a new CEL evaluator
func NewEvaluator() (*Evaluator, error) {
    env, err := createEnvironment()
    if err != nil {
        return nil, err
    }
    
    return &Evaluator{
        compiler: NewCompiler(env),
        cache:    NewExpressionCache(),
        env:      env,
    }, nil
}

// Evaluate evaluates a policy against variables
func (e *Evaluator) Evaluate(ctx context.Context, policy types.Policy, vars map[string]interface{}) (*interfaces.EvaluationResult, error) {
    result := &interfaces.EvaluationResult{
        PolicyName: policy.Name,
        Passed:     true,
        Score:      100.0,
        Violations: []interfaces.Violation{},
        Metadata:   make(map[string]interface{}),
    }
    
    totalWeight := int32(0)
    weightedScore := float64(0)
    
    for _, rule := range policy.Spec.Rules {
        ruleResult, err := e.evaluateRule(ctx, rule, vars)
        if err != nil {
            return nil, fmt.Errorf("failed to evaluate rule %s: %w", rule.Name, err)
        }
        
        if !ruleResult.Passed {
            result.Passed = false
            result.Violations = append(result.Violations, interfaces.Violation{
                Rule:     rule.Name,
                Message:  rule.Message,
                Severity: e.getSeverity(rule.Action),
            })
        }
        
        if rule.Weight > 0 {
            totalWeight += rule.Weight
            if ruleResult.Passed {
                weightedScore += float64(rule.Weight)
            }
        }
    }
    
    if totalWeight > 0 {
        result.Score = (weightedScore / float64(totalWeight)) * 100
    }
    
    return result, nil
}

// evaluateRule evaluates a single policy rule
func (e *Evaluator) evaluateRule(ctx context.Context, rule types.PolicyRule, vars map[string]interface{}) (*ruleResult, error) {
    // Check cache
    cacheKey := fmt.Sprintf("%s:%v", rule.Expression, vars)
    if cached, ok := e.cache.Get(ctx, cacheKey); ok {
        return e.executeExpression(cached, vars)
    }
    
    // Compile expression
    compiled, err := e.compiler.Compile(ctx, rule.Expression)
    if err != nil {
        return nil, err
    }
    
    // Cache compiled expression
    e.cache.Put(ctx, cacheKey, compiled, 0)
    
    // Execute expression
    return e.executeExpression(compiled, vars)
}

// executeExpression executes a compiled expression
func (e *Evaluator) executeExpression(expr interfaces.CompiledExpression, vars map[string]interface{}) (*ruleResult, error) {
    result, err := expr.Evaluate(vars)
    if err != nil {
        return nil, err
    }
    
    // Convert result to boolean
    passed, ok := result.(bool)
    if !ok {
        return nil, fmt.Errorf("expression did not return boolean: %T", result)
    }
    
    return &ruleResult{
        Passed: passed,
    }, nil
}

type ruleResult struct {
    Passed bool
}
```

### 2. `pkg/policy/cel/compiler.go` (100 lines)
CEL expression compiler.

```go
package cel

import (
    "context"
    "fmt"
    "github.com/google/cel-go/cel"
    "github.com/google/cel-go/checker/decls"
    "github.com/kcp-dev/kcp/pkg/policy/interfaces"
    exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
)

// Compiler compiles CEL expressions
type Compiler struct {
    env *cel.Env
}

// NewCompiler creates a new expression compiler
func NewCompiler(env *cel.Env) *Compiler {
    return &Compiler{
        env: env,
    }
}

// Compile compiles an expression string
func (c *Compiler) Compile(ctx context.Context, expression string) (interfaces.CompiledExpression, error) {
    ast, issues := c.env.Compile(expression)
    if issues != nil && issues.Err() != nil {
        return nil, fmt.Errorf("compilation failed: %w", issues.Err())
    }
    
    prog, err := c.env.Program(ast)
    if err != nil {
        return nil, fmt.Errorf("program creation failed: %w", err)
    }
    
    return &compiledExpression{
        ast:  ast,
        prog: prog,
    }, nil
}

// CompileWithEnv compiles with a specific environment
func (c *Compiler) CompileWithEnv(ctx context.Context, expression string, env *interfaces.Environment) (interfaces.CompiledExpression, error) {
    // Create custom environment
    customEnv, err := c.createCustomEnvironment(env)
    if err != nil {
        return nil, err
    }
    
    ast, issues := customEnv.Compile(expression)
    if issues != nil && issues.Err() != nil {
        return nil, fmt.Errorf("compilation failed: %w", issues.Err())
    }
    
    prog, err := customEnv.Program(ast)
    if err != nil {
        return nil, err
    }
    
    return &compiledExpression{
        ast:  ast,
        prog: prog,
    }, nil
}

// Validate checks expression syntax
func (c *Compiler) Validate(ctx context.Context, expression string) error {
    _, issues := c.env.Compile(expression)
    if issues != nil && issues.Err() != nil {
        return issues.Err()
    }
    return nil
}

// createCustomEnvironment creates a custom CEL environment
func (c *Compiler) createCustomEnvironment(env *interfaces.Environment) (*cel.Env, error) {
    opts := []cel.EnvOption{
        RegisterCustomFunctions(),
    }
    
    // Add variables
    for name, varType := range env.Variables {
        opts = append(opts, cel.Variable(name, getCELType(varType)))
    }
    
    return cel.NewEnv(opts...)
}

// compiledExpression wraps a compiled CEL expression
type compiledExpression struct {
    ast  *cel.Ast
    prog cel.Program
}

// Evaluate runs the expression with variables
func (e *compiledExpression) Evaluate(vars map[string]interface{}) (interface{}, error) {
    out, _, err := e.prog.Eval(vars)
    if err != nil {
        return nil, err
    }
    return out.Value(), nil
}

// Type returns the result type
func (e *compiledExpression) Type() interfaces.ExpressionType {
    return interfaces.TypeBool
}

// Cost returns computational cost
func (e *compiledExpression) Cost() uint64 {
    return e.ast.OutputType().String()
}
```

### 3. `pkg/policy/cel/environment.go` (80 lines)
CEL environment setup and configuration.

```go
package cel

import (
    "github.com/google/cel-go/cel"
    "github.com/google/cel-go/checker/decls"
    "github.com/kcp-dev/kcp/pkg/policy/types"
    exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
)

// createEnvironment creates the base CEL environment
func createEnvironment() (*cel.Env, error) {
    return cel.NewEnv(
        // Standard CEL options
        cel.StdLib(),
        
        // Custom TMC variables
        cel.Variable("cluster", cel.MapType(cel.StringType, cel.AnyType)),
        cel.Variable("workload", cel.MapType(cel.StringType, cel.AnyType)),
        cel.Variable("workspace", cel.StringType),
        cel.Variable("user", cel.StringType),
        
        // Register custom functions
        RegisterCustomFunctions(),
        
        // Optimization options
        cel.OptionalTypes(),
        cel.EagerEvaluation(true),
    )
}

// getCELType converts our variable types to CEL types
func getCELType(varType types.VariableType) *exprpb.Type {
    switch varType {
    case types.VarTypeBool:
        return decls.Bool
    case types.VarTypeInt:
        return decls.Int
    case types.VarTypeFloat:
        return decls.Double
    case types.VarTypeString:
        return decls.String
    case types.VarTypeList:
        return decls.NewListType(decls.Any)
    case types.VarTypeMap:
        return decls.NewMapType(decls.String, decls.Any)
    case types.VarTypeCluster:
        return decls.NewMapType(decls.String, decls.Any)
    case types.VarTypeWorkload:
        return decls.NewMapType(decls.String, decls.Any)
    case types.VarTypeWorkspace:
        return decls.String
    default:
        return decls.Any
    }
}

// createVariableMap creates CEL variable declarations
func createVariableMap() map[string]*exprpb.Type {
    return map[string]*exprpb.Type{
        "cluster.name":       decls.String,
        "cluster.region":     decls.String,
        "cluster.zone":       decls.String,
        "cluster.labels":     decls.NewMapType(decls.String, decls.String),
        "cluster.capacity":   decls.NewMapType(decls.String, decls.Int),
        "cluster.available":  decls.NewMapType(decls.String, decls.Int),
        "workload.name":      decls.String,
        "workload.namespace": decls.String,
        "workload.labels":    decls.NewMapType(decls.String, decls.String),
        "workload.replicas":  decls.Int,
        "workspace":          decls.String,
        "user":               decls.String,
    }
}

// DefaultVariables returns default variable values for testing
func DefaultVariables() map[string]interface{} {
    return map[string]interface{}{
        "cluster": map[string]interface{}{
            "name":   "test-cluster",
            "region": "us-west-2",
            "zone":   "us-west-2a",
            "labels": map[string]string{},
            "capacity": map[string]int{
                "cpu":    100,
                "memory": 1000,
            },
            "available": map[string]int{
                "cpu":    50,
                "memory": 500,
            },
        },
        "workload": map[string]interface{}{
            "name":      "test-workload",
            "namespace": "default",
            "labels":    map[string]string{},
            "replicas":  3,
        },
        "workspace": "root:org:team",
        "user":      "test-user",
    }
}
```

### 4. `pkg/policy/cel/variables.go` (70 lines)
Variable management for CEL expressions.

```go
package cel

import (
    "fmt"
    "reflect"
)

// VariableResolver resolves variables for evaluation
type VariableResolver struct {
    defaults map[string]interface{}
}

// NewVariableResolver creates a new variable resolver
func NewVariableResolver() *VariableResolver {
    return &VariableResolver{
        defaults: DefaultVariables(),
    }
}

// Resolve merges provided variables with defaults
func (r *VariableResolver) Resolve(provided map[string]interface{}) map[string]interface{} {
    result := make(map[string]interface{})
    
    // Start with defaults
    for k, v := range r.defaults {
        result[k] = v
    }
    
    // Overlay provided values
    for k, v := range provided {
        result[k] = r.mergeValue(result[k], v)
    }
    
    return result
}

// mergeValue merges two values, preferring the new value
func (r *VariableResolver) mergeValue(old, new interface{}) interface{} {
    if new == nil {
        return old
    }
    
    // If both are maps, merge them
    oldMap, oldOk := old.(map[string]interface{})
    newMap, newOk := new.(map[string]interface{})
    
    if oldOk && newOk {
        merged := make(map[string]interface{})
        for k, v := range oldMap {
            merged[k] = v
        }
        for k, v := range newMap {
            merged[k] = r.mergeValue(merged[k], v)
        }
        return merged
    }
    
    return new
}

// Validate checks if required variables are present
func (r *VariableResolver) Validate(vars map[string]interface{}, required []string) error {
    for _, req := range required {
        if _, ok := vars[req]; !ok {
            return fmt.Errorf("required variable %s not provided", req)
        }
    }
    return nil
}

// ExtractVariables extracts variable names from an expression
func ExtractVariables(expression string) []string {
    // Simple implementation - would use CEL AST in production
    vars := []string{}
    // Parse expression and extract variable references
    return vars
}
```

### 5. `pkg/policy/cel/cache.go` (60 lines)
Expression caching implementation.

```go
package cel

import (
    "context"
    "sync"
    "time"
    "github.com/kcp-dev/kcp/pkg/policy/interfaces"
)

// ExpressionCache caches compiled expressions
type ExpressionCache struct {
    mu      sync.RWMutex
    entries map[string]*cacheEntry
    maxSize int
    stats   interfaces.CacheStats
}

type cacheEntry struct {
    expr      interfaces.CompiledExpression
    timestamp time.Time
    hits      int64
}

// NewExpressionCache creates a new expression cache
func NewExpressionCache() *ExpressionCache {
    return &ExpressionCache{
        entries: make(map[string]*cacheEntry),
        maxSize: 1000,
    }
}

// Get retrieves a cached expression
func (c *ExpressionCache) Get(ctx context.Context, key string) (interfaces.CompiledExpression, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    
    entry, ok := c.entries[key]
    if !ok {
        c.stats.Misses++
        return nil, false
    }
    
    entry.hits++
    c.stats.Hits++
    return entry.expr, true
}

// Put stores a compiled expression
func (c *ExpressionCache) Put(ctx context.Context, key string, expr interfaces.CompiledExpression, ttl time.Duration) error {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    // Simple eviction if at capacity
    if len(c.entries) >= c.maxSize {
        c.evictOldest()
    }
    
    c.entries[key] = &cacheEntry{
        expr:      expr,
        timestamp: time.Now(),
    }
    
    c.stats.Size = int64(len(c.entries))
    return nil
}

// evictOldest removes the oldest cache entry
func (c *ExpressionCache) evictOldest() {
    var oldestKey string
    var oldestTime time.Time
    
    for k, v := range c.entries {
        if oldestKey == "" || v.timestamp.Before(oldestTime) {
            oldestKey = k
            oldestTime = v.timestamp
        }
    }
    
    if oldestKey != "" {
        delete(c.entries, oldestKey)
        c.stats.Evictions++
    }
}
```

### 6. `pkg/policy/cel/evaluator_test.go` (150 lines)
Comprehensive tests for CEL evaluator.

```go
package cel_test

import (
    "context"
    "testing"
    "github.com/kcp-dev/kcp/pkg/policy/cel"
    "github.com/kcp-dev/kcp/pkg/policy/types"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestEvaluator(t *testing.T) {
    ctx := context.Background()
    evaluator, err := cel.NewEvaluator()
    require.NoError(t, err)
    
    tests := []struct {
        name     string
        policy   types.Policy
        vars     map[string]interface{}
        expected bool
        score    float64
    }{
        {
            name: "simple region check",
            policy: types.Policy{
                Spec: types.PolicySpec{
                    Rules: []types.PolicyRule{
                        {
                            Name:       "region-check",
                            Expression: `cluster.region == "us-west-2"`,
                            Action:     types.ActionAllow,
                        },
                    },
                },
            },
            vars: map[string]interface{}{
                "cluster": map[string]interface{}{
                    "region": "us-west-2",
                },
            },
            expected: true,
            score:    100.0,
        },
        {
            name: "capacity check",
            policy: types.Policy{
                Spec: types.PolicySpec{
                    Rules: []types.PolicyRule{
                        {
                            Name:       "capacity-check",
                            Expression: `cluster.available.cpu >= 10`,
                            Action:     types.ActionAllow,
                        },
                    },
                },
            },
            vars: map[string]interface{}{
                "cluster": map[string]interface{}{
                    "available": map[string]interface{}{
                        "cpu": 20,
                    },
                },
            },
            expected: true,
            score:    100.0,
        },
        {
            name: "label existence",
            policy: types.Policy{
                Spec: types.PolicySpec{
                    Rules: []types.PolicyRule{
                        {
                            Name:       "label-check",
                            Expression: `hasLabel(cluster.labels, "environment")`,
                            Action:     types.ActionDeny,
                        },
                    },
                },
            },
            vars: map[string]interface{}{
                "cluster": map[string]interface{}{
                    "labels": map[string]string{
                        "environment": "production",
                    },
                },
            },
            expected: false,
            score:    0.0,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := evaluator.Evaluate(ctx, tt.policy, tt.vars)
            require.NoError(t, err)
            
            assert.Equal(t, tt.expected, result.Passed)
            assert.Equal(t, tt.score, result.Score)
        })
    }
}

func TestCompiler(t *testing.T) {
    ctx := context.Background()
    env, err := cel.createEnvironment()
    require.NoError(t, err)
    
    compiler := cel.NewCompiler(env)
    
    tests := []struct {
        name       string
        expression string
        wantErr    bool
    }{
        {
            name:       "valid expression",
            expression: `cluster.region == "us-west-2"`,
            wantErr:    false,
        },
        {
            name:       "invalid syntax",
            expression: `cluster.region ==`,
            wantErr:    true,
        },
        {
            name:       "undefined variable",
            expression: `undefined.field == "value"`,
            wantErr:    true,
        },
        {
            name:       "custom function",
            expression: `hasLabel(cluster.labels, "key")`,
            wantErr:    false,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := compiler.Validate(ctx, tt.expression)
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}

func TestCaching(t *testing.T) {
    ctx := context.Background()
    cache := cel.NewExpressionCache()
    
    // Test cache miss
    _, ok := cache.Get(ctx, "test-key")
    assert.False(t, ok)
    
    // Store expression
    mockExpr := &mockCompiledExpression{}
    err := cache.Put(ctx, "test-key", mockExpr, 0)
    require.NoError(t, err)
    
    // Test cache hit
    cached, ok := cache.Get(ctx, "test-key")
    assert.True(t, ok)
    assert.Equal(t, mockExpr, cached)
    
    // Check stats
    stats, err := cache.Stats(ctx)
    require.NoError(t, err)
    assert.Equal(t, int64(1), stats.Size)
    assert.Equal(t, int64(1), stats.Hits)
    assert.Equal(t, int64(1), stats.Misses)
}
```

## Implementation Steps

### Step 1: Setup Dependencies
```bash
# Ensure branch 15 is available
git fetch origin feature/tmc-phase4-15-policy-framework
# Install CEL-Go library
go get github.com/google/cel-go
```

### Step 2: Create Package Structure
```bash
mkdir -p pkg/policy/cel
```

### Step 3: Implement CEL Components
1. Start with `environment.go` - environment setup
2. Add `compiler.go` - expression compilation
3. Create `evaluator.go` - main evaluator
4. Add `variables.go` - variable management
5. Create `cache.go` - expression caching
6. Add `evaluator_test.go` - comprehensive tests

### Step 4: Test Custom Functions
Ensure all custom CEL functions work correctly.

### Step 5: Performance Testing
Add benchmarks for expression evaluation.

## KCP Patterns to Follow

1. **Context Usage**: Pass context through evaluation
2. **Caching Strategy**: Cache compiled expressions
3. **Error Handling**: Clear error messages
4. **Variable Merging**: Support partial variables
5. **Performance**: Optimize hot paths

## Testing Requirements

### Unit Tests Required
- [ ] Expression compilation tests
- [ ] Evaluation tests with various policies
- [ ] Custom function tests
- [ ] Cache functionality tests
- [ ] Variable resolution tests

### Performance Tests
- [ ] Expression evaluation benchmarks
- [ ] Cache performance tests
- [ ] Complex policy benchmarks

## Integration Points

This CEL evaluator will be:
- **Used by**: Branch 19 (Controller)
- **Tested in**: Branch 23 (Integration)

## Validation Checklist

- [ ] CEL expressions compile correctly
- [ ] Custom functions work as expected
- [ ] Caching improves performance
- [ ] Variable merging works correctly
- [ ] Thread-safe implementation
- [ ] Error messages are helpful
- [ ] Performance optimized
- [ ] Documentation complete
- [ ] Test coverage >85%
- [ ] Feature flag ready

## Line Count Validation
Run before committing:
```bash
/workspaces/kcp-shared-tools/tmc-pr-line-counter.sh -c feature/tmc-phase4-17-cel-evaluator
```

Target: ~580 lines