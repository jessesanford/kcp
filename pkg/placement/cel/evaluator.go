/*
Copyright 2023 The KCP Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cel

import (
	"context"
	"fmt"
	"time"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"k8s.io/apimachinery/pkg/api/resource"
)

// evaluator implements the CELEvaluator interface with caching and custom functions.
type evaluator struct {
	env             *cel.Env
	cache           ExpressionCache
	customFunctions map[string]CustomFunction
	options         *EvaluationOptions
}

// NewCELEvaluator creates a new CEL evaluator with KCP-specific functions and types.
func NewCELEvaluator(opts *EvaluationOptions) (CELEvaluator, error) {
	if opts == nil {
		opts = &EvaluationOptions{
			Timeout: 10 * time.Second,
			MaxCost: 1000,
		}
	}

	// Create CEL environment with basic functionality
	env, err := cel.NewEnv(
		// Basic variables for now
		cel.Variable("workspace", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("request", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("resources", cel.MapType(cel.StringType, cel.DynType)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create CEL environment: %w", err)
	}

	eval := &evaluator{
		env:             env,
		cache:           NewMemoryCache(),
		customFunctions: make(map[string]CustomFunction),
		options:         opts,
	}

	// Register built-in KCP functions
	if err := eval.registerBuiltinFunctions(); err != nil {
		return nil, fmt.Errorf("failed to register builtin functions: %w", err)
	}

	return eval, nil
}

// CompileExpression compiles a CEL expression and caches it for efficient reuse.
func (e *evaluator) CompileExpression(expr string) (*CompiledExpression, error) {
	hash := hashExpression(expr)
	
	// Check cache first
	if cached, ok := e.cache.Get(hash); ok {
		return cached, nil
	}

	// Parse and compile the expression
	ast, issues := e.env.Compile(expr)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("expression compilation failed: %w", issues.Err())
	}

	// Create program
	program, err := e.env.Program(ast)
	if err != nil {
		return nil, fmt.Errorf("failed to create program: %w", err)
	}

	compiled := &CompiledExpression{
		Expression: expr,
		Program:    program,
		CompiledAt: time.Now(),
		Hash:       hash,
	}

	// Cache the compiled expression
	e.cache.Set(hash, compiled)

	return compiled, nil
}

// EvaluatePlacement evaluates a placement expression against a placement context.
func (e *evaluator) EvaluatePlacement(ctx context.Context, expr *CompiledExpression, placement *PlacementContext) (bool, error) {
	// Build variable map from placement context
	vars, err := e.buildVariableMap(placement)
	if err != nil {
		return false, fmt.Errorf("failed to build variables: %w", err)
	}

	// Evaluate the expression
	result, err := e.EvaluateWithVariables(ctx, expr, vars)
	if err != nil {
		return false, err
	}

	// Convert result to boolean
	switch r := result.(type) {
	case bool:
		return r, nil
	case ref.Val:
		if r.Type() == types.BoolType {
			return r.Value().(bool), nil
		}
		return false, fmt.Errorf("expression returned non-boolean type: %s", r.Type())
	default:
		return false, fmt.Errorf("expression returned unexpected type: %T", result)
	}
}

// EvaluateWithVariables evaluates an expression with custom variable bindings.
func (e *evaluator) EvaluateWithVariables(ctx context.Context, expr *CompiledExpression, vars map[string]interface{}) (interface{}, error) {
	// Apply timeout if specified
	if e.options.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, e.options.Timeout)
		defer cancel()
	}

	// Convert variables to CEL types
	celVars := make(map[string]interface{})
	for k, v := range vars {
		celVars[k] = v
	}

	// Add default variables if specified
	for k, v := range e.options.Variables {
		if _, exists := celVars[k]; !exists {
			celVars[k] = v
		}
	}

	// Evaluate with cost tracking
	start := time.Now()
	val, _, err := expr.Program.ContextEval(ctx, celVars)
	duration := time.Since(start)

	if err != nil {
		return nil, fmt.Errorf("expression evaluation failed: %w", err)
	}

	if e.options.EnableDebug {
		fmt.Printf("CEL evaluation took %v for expression: %s\n", duration, expr.Expression)
	}

	return val.Value(), nil
}

// RegisterCustomFunction registers a custom function for use in CEL expressions.
func (e *evaluator) RegisterCustomFunction(name string, fn CustomFunction) error {
	if fn.Name() != name {
		return fmt.Errorf("function name mismatch: expected %s, got %s", name, fn.Name())
	}

	// Add function to environment
	newEnv, err := e.env.Extend(cel.Function(name,
		cel.Overload(name+"_overload", 
			[]*cel.Type{cel.StringType, cel.StringType}, 
			cel.BoolType)))
	if err != nil {
		return fmt.Errorf("failed to register function %s: %w", name, err)
	}

	e.env = newEnv
	e.customFunctions[name] = fn

	// Clear cache since the environment changed
	e.cache.Clear()

	return nil
}

// GetEnvironment returns the CEL environment for advanced operations.
func (e *evaluator) GetEnvironment() *cel.Env {
	return e.env
}

// buildVariableMap builds the variable map from a placement context.
func (e *evaluator) buildVariableMap(placement *PlacementContext) (map[string]interface{}, error) {
	vars := make(map[string]interface{})

	// Add workspace context
	if placement.Workspace != nil {
		vars["workspace"] = placement.Workspace
	}

	// Add request context
	if placement.Request != nil {
		vars["request"] = placement.Request
	}

	// Add resource context
	if placement.Resources != nil {
		vars["resources"] = placement.Resources
	}

	// Add custom variables
	for k, v := range placement.Variables {
		vars[k] = v
	}

	return vars, nil
}

// registerBuiltinFunctions registers KCP-specific CEL functions.
func (e *evaluator) registerBuiltinFunctions() error {
	functions := []CustomFunction{
		NewHasLabelFunction(),
		NewInWorkspaceFunction(),
		NewHasCapacityFunction(),
		NewMatchesSelectorFunction(),
		NewDistanceFunction(),
	}

	for _, fn := range functions {
		if err := e.RegisterCustomFunction(fn.Name(), fn); err != nil {
			return fmt.Errorf("failed to register builtin function %s: %w", fn.Name(), err)
		}
	}

	return nil
}

// ValidateExpression validates a CEL expression without compiling it.
func ValidateExpression(expr string, env *cel.Env) *ValidationResult {
	result := &ValidationResult{
		Valid: true,
	}

	// Parse and type-check the expression
	ast, issues := env.Parse(expr)
	if issues != nil && issues.Err() != nil {
		result.Valid = false
		result.Errors = append(result.Errors, issues.Err().Error())
		return result
	}

	// Type check
	checked, issues := env.Check(ast)
	if issues != nil && issues.Err() != nil {
		result.Valid = false
		result.Errors = append(result.Errors, issues.Err().Error())
		return result
	}

	resultType := checked.ResultType()
	
	// Add warnings for non-boolean return types in placement context
	if resultType.String() != "bool" {
		result.Warnings = append(result.Warnings, 
			fmt.Sprintf("expression returns %s, expected bool for placement evaluation", 
				result.ReturnType))
	}

	return result
}

// resourceQuantityToFloat64 converts a resource.Quantity to float64 for CEL evaluation.
func resourceQuantityToFloat64(q resource.Quantity) float64 {
	return float64(q.MilliValue()) / 1000.0
}

// parseResourceQuantity parses a string into a resource.Quantity.
func parseResourceQuantity(s string) (resource.Quantity, error) {
	return resource.ParseQuantity(s)
}