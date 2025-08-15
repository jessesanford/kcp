/*
Copyright 2024 The KCP Authors.

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

package interfaces

import (
	"github.com/kcp-dev/kcp/pkg/policy/types"
)

// ExpressionCompiler compiles and validates CEL expressions for TMC policy evaluation.
// Implementations should provide thread-safe compilation with caching support.
type ExpressionCompiler interface {
	// Compile validates and compiles a CEL expression using the default environment.
	// Returns a compiled expression ready for evaluation or an error if compilation fails.
	Compile(expression string) (CompiledExpression, error)

	// CompileWithEnv compiles an expression using a custom compilation environment.
	// This allows for different variable types and function declarations per context.
	CompileWithEnv(expression string, env CompilerEnvironment) (CompiledExpression, error)

	// Validate checks if an expression is syntactically and semantically correct.
	// This is lighter weight than full compilation for validation-only scenarios.
	Validate(expression string) *types.CompilationResult

	// ExtractVariables analyzes an expression to identify all referenced variables.
	// Useful for determining what context must be provided for evaluation.
	ExtractVariables(expression string) ([]types.Variable, error)
}

// CompiledExpression represents a successfully compiled CEL expression ready for evaluation.
// Instances should be thread-safe and reusable across multiple evaluations.
type CompiledExpression interface {
	// Eval evaluates the expression with the provided variable context.
	// Returns the evaluation result or an error if evaluation fails.
	Eval(variables map[string]interface{}) (interface{}, error)

	// Source returns the original expression string that was compiled.
	Source() string

	// Cost returns an estimate of the computational cost of evaluating this expression.
	// Higher values indicate more expensive operations.
	Cost() int64

	// IsConstant returns true if this expression produces the same result regardless of variables.
	// Constant expressions can be cached indefinitely.
	IsConstant() bool
}

// CompilerEnvironment defines the compilation context for CEL expressions.
// It specifies available variables, functions, and compilation options.
type CompilerEnvironment interface {
	// AddVariable declares a variable that will be available in expressions.
	// The variable type helps with compile-time type checking.
	AddVariable(name string, varType types.VariableType) error

	// AddFunction declares a custom function available to expressions.
	// The function parameter should be a valid CEL function implementation.
	AddFunction(name string, function interface{}) error

	// SetOption configures compiler behavior with key-value settings.
	// Common options include strictness level and optimization flags.
	SetOption(key string, value interface{}) error
}

// CompilerOptions configures the behavior of expression compilation.
type CompilerOptions struct {
	// StrictTypes enables strict type checking during compilation
	StrictTypes bool
	// OptimizeConstants pre-evaluates constant sub-expressions at compile time
	OptimizeConstants bool
	// MaxCost limits the computational complexity of expressions
	MaxCost int64
}