package interfaces

import (
	"context"
	"time"
)

// CompiledExpression represents a compiled CEL expression
type CompiledExpression interface {
	// Evaluate executes the expression with provided variables
	Evaluate(vars map[string]interface{}) (interface{}, error)
	
	// Type returns the result type of the expression
	Type() ExpressionType
	
	// Cost returns the computational cost estimate
	Cost() uint64
}

// ExpressionCache provides caching for compiled expressions
type ExpressionCache interface {
	// Get retrieves a cached expression
	Get(ctx context.Context, key string) (CompiledExpression, bool)
	
	// Put stores a compiled expression with optional TTL
	Put(ctx context.Context, key string, expr CompiledExpression, ttl time.Duration) error
	
	// Stats returns cache statistics
	Stats(ctx context.Context) (*CacheStats, error)
	
	// Clear removes all cached expressions
	Clear(ctx context.Context) error
}

// EvaluationResult contains the result of policy evaluation
type EvaluationResult struct {
	PolicyName string
	Passed     bool
	Score      float64
	Violations []Violation
	Metadata   map[string]interface{}
}

// Violation represents a policy rule violation
type Violation struct {
	Rule     string
	Message  string
	Severity ViolationSeverity
}

// ViolationSeverity indicates the severity level of a violation
type ViolationSeverity string

const (
	SeverityLow      ViolationSeverity = "low"
	SeverityMedium   ViolationSeverity = "medium"
	SeverityHigh     ViolationSeverity = "high"
	SeverityCritical ViolationSeverity = "critical"
)

// CacheStats provides cache performance metrics
type CacheStats struct {
	Size      int64
	Hits      int64
	Misses    int64
	Evictions int64
}

// Environment represents a CEL evaluation environment
type Environment struct {
	Variables map[string]VariableType
}

// VariableType represents supported variable types in CEL
type VariableType string

// ExpressionType represents supported expression result types
type ExpressionType string

const (
	TypeBool   ExpressionType = "bool"
	TypeInt    ExpressionType = "int"
	TypeFloat  ExpressionType = "float"
	TypeString ExpressionType = "string"
	TypeList   ExpressionType = "list"
	TypeMap    ExpressionType = "map"
)