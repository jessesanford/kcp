package interfaces

import (
	"context"
)

// PolicyEvaluator evaluates placement policies against cluster targets.
// It provides the capability to compile policy expressions and evaluate
// them against cluster and workload context to make placement decisions.
type PolicyEvaluator interface {
	// Compile validates and compiles a policy expression for efficient evaluation.
	// This preprocessing step catches syntax errors and optimizes the expression
	// for repeated evaluation against multiple targets.
	Compile(expression string) (CompiledExpression, error)

	// Evaluate runs a compiled expression against the provided variables.
	// The variables typically include cluster properties, workload attributes,
	// and context information needed for policy evaluation.
	Evaluate(ctx context.Context, expr CompiledExpression,
		vars map[string]interface{}) (bool, error)

	// EvaluatePolicy evaluates a complete policy against a cluster target.
	// This is a convenience method that handles the full policy evaluation
	// workflow including rule processing and score calculation.
	EvaluatePolicy(ctx context.Context, policy PlacementPolicy,
		target ClusterTarget, workload interface{}) (*PolicyResult, error)

	// EvaluateRule evaluates a single policy rule against a cluster.
	// This enables fine-grained control over policy evaluation and
	// detailed reporting of rule-level results.
	EvaluateRule(ctx context.Context, rule PolicyRule,
		target ClusterTarget, workload interface{}) (*RuleResult, error)

	// ValidatePolicy checks if a policy is syntactically correct and can be compiled.
	// This validation can be performed before storing or applying policies.
	ValidatePolicy(policy PlacementPolicy) error
}

// CompiledExpression represents a compiled policy expression that can be
// efficiently evaluated against multiple variable sets. Implementations
// should cache compilation results for performance.
type CompiledExpression interface {
	// String returns the original expression source code
	String() string

	// IsValid checks if the compiled expression is valid and ready for evaluation
	IsValid() bool

	// Variables returns the list of variables referenced in the expression.
	// This can be used for validation and optimization purposes.
	Variables() []string

	// Type returns the expected return type of the expression (bool, number, string)
	Type() ExpressionType
}

// ExpressionType represents the data type returned by an expression
type ExpressionType string

const (
	// ExpressionTypeBool for boolean expressions (constraints)
	ExpressionTypeBool ExpressionType = "bool"
	// ExpressionTypeNumber for numeric expressions (scoring)
	ExpressionTypeNumber ExpressionType = "number"
	// ExpressionTypeString for string expressions (labeling)
	ExpressionTypeString ExpressionType = "string"
)

// PolicyContext provides context for policy evaluation including
// cluster information, workload details, and user context.
type PolicyContext struct {
	// Cluster is the target cluster being evaluated
	Cluster ClusterTarget

	// Workload is the workload object being placed
	Workload interface{}

	// User is the user requesting the placement
	User string

	// Groups are the user's group memberships
	Groups []string

	// Variables provides additional context variables for policy evaluation
	Variables map[string]interface{}

	// RequestTime is when the placement request was made
	RequestTime string
}