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
	"context"

	"github.com/kcp-dev/kcp/pkg/policy/types"
)

// PolicyEvaluator defines the interface for evaluating TMC placement policies.
// Implementations should be thread-safe and support concurrent evaluation.
type PolicyEvaluator interface {
	// EvaluatePolicy evaluates a complete policy against the provided context.
	// Returns detailed evaluation results including pass/fail status and scoring.
	EvaluatePolicy(ctx context.Context, policy types.Policy,
		policyContext types.PolicyContext) (*types.EvaluationResult, error)

	// EvaluatePolicySet evaluates multiple policies and applies conflict resolution.
	// Returns results for all policies in evaluation order.
	EvaluatePolicySet(ctx context.Context, policySet types.PolicySet,
		policyContext types.PolicyContext) ([]types.EvaluationResult, error)

	// EvaluateExpression evaluates a single CEL expression with provided variables.
	// This is useful for testing expressions or one-off evaluations.
	EvaluateExpression(ctx context.Context, expression string,
		variables map[string]interface{}) (*types.EvaluationResult, error)

	// ValidatePolicy checks if a policy is syntactically correct and semantically valid.
	// Should be called before storing or using policies for evaluation.
	ValidatePolicy(policy types.Policy) error
}

// ScoringEvaluator extends PolicyEvaluator with scoring capabilities for ranking placement targets.
// This interface is used when policies need to rank multiple placement options.
type ScoringEvaluator interface {
	PolicyEvaluator

	// ScoreTarget calculates a numerical score for a placement target using the provided policies.
	// Higher scores indicate better placement candidates.
	ScoreTarget(ctx context.Context, policies []types.Policy,
		target interface{}, policyContext types.PolicyContext) (float64, error)

	// RankTargets evaluates and ranks multiple placement targets using scoring policies.
	// Returns targets sorted by score in descending order (best first).
	RankTargets(ctx context.Context, policies []types.Policy,
		targets []interface{}, policyContext types.PolicyContext) ([]ScoredTarget, error)
}

// ScoredTarget represents a placement target with its calculated score and evaluation details.
type ScoredTarget struct {
	// Target is the placement candidate (e.g., cluster, workspace)
	Target interface{} `json:"target"`
	// Score is the calculated placement score (higher is better)
	Score float64 `json:"score"`
	// Details contains per-policy evaluation results for debugging
	Details map[string]types.EvaluationResult `json:"details,omitempty"`
}

// EvaluatorOptions configures the behavior of policy evaluators.
type EvaluatorOptions struct {
	// EnableCaching enables compiled expression caching for performance
	EnableCaching bool
	// EnableTracing enables detailed evaluation tracing for debugging
	EnableTracing bool
	// MaxExpressionLength limits the size of expressions to prevent abuse
	MaxExpressionLength int
	// Timeout sets the maximum duration for policy evaluation (seconds)
	Timeout int
}

// EvaluatorFactory creates policy evaluator instances with specified configurations.
// This allows for different evaluator implementations based on requirements.
type EvaluatorFactory interface {
	// Create returns a basic policy evaluator with the specified options
	Create(options EvaluatorOptions) (PolicyEvaluator, error)

	// CreateScoring returns a scoring-capable policy evaluator
	CreateScoring(options EvaluatorOptions) (ScoringEvaluator, error)
}