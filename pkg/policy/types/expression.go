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

package types

import (
	"time"
)

// Expression represents a CEL expression with metadata about its structure and requirements.
// It provides introspection capabilities for expressions used in TMC placement policies.
type Expression struct {
	// Source contains the original CEL expression string
	Source string `json:"source"`

	// Compiled indicates whether this expression has been compiled and validated
	Compiled bool `json:"compiled"`

	// Variables lists all variables referenced in the expression
	Variables []Variable `json:"variables,omitempty"`

	// Functions lists custom functions used in the expression
	Functions []string `json:"functions,omitempty"`
}

// Variable represents a variable that can be used in CEL expressions.
// Variables provide typed inputs to policy evaluation with optional defaults.
type Variable struct {
	// Name is the variable identifier used in expressions
	Name string `json:"name"`
	// Type specifies the expected variable type
	Type VariableType `json:"type"`
	// Required indicates if variable must be provided
	Required bool `json:"required"`
	// Default provides fallback value when variable is not required
	Default interface{} `json:"default,omitempty"`
	// Description explains the variable's purpose
	Description string `json:"description,omitempty"`
}

// VariableType defines the supported types for CEL expression variables.
type VariableType string

const (
	// StringType represents string values
	StringType VariableType = "string"
	// IntType represents integer values  
	IntType VariableType = "int"
	// BoolType represents boolean values
	BoolType VariableType = "bool"
	// MapType represents key-value mappings
	MapType VariableType = "map"
	// ListType represents arrays/slices
	ListType VariableType = "list"
	// ObjectType represents complex structured data
	ObjectType VariableType = "object"
)

// EvaluationResult contains the outcome of evaluating a policy or expression.
// It provides detailed information about the evaluation process and results.
type EvaluationResult struct {
	// Passed indicates whether the evaluation succeeded
	Passed bool `json:"passed"`

	// Value contains the actual result of expression evaluation
	Value interface{} `json:"value,omitempty"`

	// Score provides numerical rating for scoring-based policies
	Score *float64 `json:"score,omitempty"`

	// Error contains error message if evaluation failed
	Error string `json:"error,omitempty"`

	// Details provides additional information about the evaluation
	Details EvaluationDetails `json:"details,omitempty"`
}

// EvaluationDetails provides comprehensive information about policy evaluation.
// This is useful for debugging, auditing, and performance analysis.
type EvaluationDetails struct {
	// Expression that was evaluated
	Expression string `json:"expression"`
	// Variables provided to the evaluation
	Variables map[string]interface{} `json:"variables"`
	// Duration of the evaluation
	Duration time.Duration `json:"duration"`
	// CacheHit indicates if result came from cache
	CacheHit bool `json:"cacheHit"`
	// TraceMessage provides debugging information
	TraceMessage string `json:"traceMessage,omitempty"`
}

// CompilationResult represents the outcome of compiling a CEL expression.
// It includes success status and any warnings or errors encountered.
type CompilationResult struct {
	// Success indicates if compilation was successful
	Success bool `json:"success"`
	// Error contains compilation error message if failed
	Error string `json:"error,omitempty"`
	// Warnings contains non-fatal issues found during compilation
	Warnings []string `json:"warnings,omitempty"`
}

// CacheStats provides statistics about expression cache performance.
// This information is useful for monitoring and tuning cache behavior.
type CacheStats struct {
	// Hits is the number of successful cache lookups
	Hits int64 `json:"hits"`
	// Misses is the number of cache lookups that failed
	Misses int64 `json:"misses"`
	// Evictions is the number of entries removed from cache
	Evictions int64 `json:"evictions"`
	// Size is the current number of entries in cache
	Size int `json:"size"`
	// MaxSize is the maximum allowed cache size
	MaxSize int `json:"maxSize"`
	// AvgLatency is the average time for cache operations
	AvgLatency time.Duration `json:"avgLatency"`
}