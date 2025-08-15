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

// getSeverity maps rule action to violation severity
func (e *Evaluator) getSeverity(action types.RuleAction) interfaces.ViolationSeverity {
	switch action {
	case types.ActionDeny:
		return interfaces.SeverityHigh
	case types.ActionWarn:
		return interfaces.SeverityMedium
	case types.ActionAllow:
		return interfaces.SeverityLow
	default:
		return interfaces.SeverityMedium
	}
}

type ruleResult struct {
	Passed bool
}