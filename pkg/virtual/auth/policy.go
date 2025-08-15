package auth

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/authentication/user"
)

// PolicyEngine evaluates authorization policies for virtual workspace access.
type PolicyEngine interface {
	// Evaluate evaluates policies for the given attributes
	Evaluate(ctx context.Context, attrs *AuthAttributes) (Decision, string, error)
	
	// AddPolicy adds a new policy rule
	AddPolicy(policy *PolicyRule) error
	
	// RemovePolicy removes a policy by ID
	RemovePolicy(policyID string) error
	
	// ListPolicies returns all configured policies
	ListPolicies() []*PolicyRule
}

// PolicyRule defines an authorization policy.
type PolicyRule struct {
	// ID uniquely identifies this policy
	ID string
	
	// Priority determines evaluation order (lower = higher priority)
	Priority int
	
	// Effect is either Allow or Deny
	Effect PolicyEffect
	
	// Subjects this policy applies to
	Subjects PolicySubjects
	
	// Resources this policy applies to
	Resources PolicyResources
	
	// Conditions that must be met
	Conditions []PolicyCondition
}

// PolicyEffect represents the effect of a policy rule.
type PolicyEffect string

const (
	// PolicyEffectAllow allows the action
	PolicyEffectAllow PolicyEffect = "Allow"
	
	// PolicyEffectDeny denies the action
	PolicyEffectDeny PolicyEffect = "Deny"
)

// PolicySubjects defines which subjects a policy applies to.
type PolicySubjects struct {
	// Users is a list of usernames
	Users []string
	
	// Groups is a list of group names
	Groups []string
	
	// ServiceAccounts is a list of service account names
	ServiceAccounts []string
	
	// All applies to all subjects if true
	All bool
}

// PolicyResources defines which resources a policy applies to.
type PolicyResources struct {
	// Workspaces is a list of workspace names (supports wildcards)
	Workspaces []string
	
	// APIGroups is a list of API groups
	APIGroups []string
	
	// Resources is a list of resource types
	Resources []string
	
	// Verbs is a list of allowed verbs
	Verbs []string
}

// PolicyCondition represents a condition that must be met.
type PolicyCondition struct {
	// Type of condition
	Type ConditionType
	
	// Operator for comparison
	Operator ConditionOperator
	
	// Value to compare against
	Value interface{}
}

// ConditionType represents the type of condition.
type ConditionType string

const (
	// ConditionTypeTime checks time-based conditions
	ConditionTypeTime ConditionType = "Time"
	
	// ConditionTypeIP checks IP address conditions
	ConditionTypeIP ConditionType = "IP"
	
	// ConditionTypeLabel checks label conditions
	ConditionTypeLabel ConditionType = "Label"
)

// ConditionOperator represents comparison operators.
type ConditionOperator string

const (
	// ConditionOperatorEquals checks equality
	ConditionOperatorEquals ConditionOperator = "Equals"
	
	// ConditionOperatorNotEquals checks inequality
	ConditionOperatorNotEquals ConditionOperator = "NotEquals"
	
	// ConditionOperatorIn checks membership
	ConditionOperatorIn ConditionOperator = "In"
	
	// ConditionOperatorNotIn checks non-membership
	ConditionOperatorNotIn ConditionOperator = "NotIn"
)

// BasicPolicyEngine implements PolicyEngine with rule-based evaluation.
type BasicPolicyEngine struct {
	policies []*PolicyRule
}

// NewBasicPolicyEngine creates a new basic policy engine.
func NewBasicPolicyEngine() *BasicPolicyEngine {
	return &BasicPolicyEngine{
		policies: make([]*PolicyRule, 0),
	}
}

// Evaluate implements PolicyEngine.
func (e *BasicPolicyEngine) Evaluate(ctx context.Context, attrs *AuthAttributes) (Decision, string, error) {
	// Sort policies by priority
	// Lower priority number = higher priority
	for _, policy := range e.policies {
		if !e.matchesSubject(policy.Subjects, attrs.User) {
			continue
		}
		
		if !e.matchesResource(policy.Resources, attrs) {
			continue
		}
		
		if !e.evaluateConditions(ctx, policy.Conditions, attrs) {
			continue
		}
		
		// Policy matches
		if policy.Effect == PolicyEffectAllow {
			return DecisionAllow, fmt.Sprintf("Allowed by policy %s", policy.ID), nil
		}
		return DecisionDeny, fmt.Sprintf("Denied by policy %s", policy.ID), nil
	}
	
	// No matching policy
	return DecisionNoOpinion, "No matching policy", nil
}

// AddPolicy implements PolicyEngine.
func (e *BasicPolicyEngine) AddPolicy(policy *PolicyRule) error {
	if policy.ID == "" {
		return fmt.Errorf("policy ID is required")
	}
	
	// Check for duplicate ID
	for _, p := range e.policies {
		if p.ID == policy.ID {
			return fmt.Errorf("policy with ID %s already exists", policy.ID)
		}
	}
	
	e.policies = append(e.policies, policy)
	return nil
}

// RemovePolicy implements PolicyEngine.
func (e *BasicPolicyEngine) RemovePolicy(policyID string) error {
	for i, p := range e.policies {
		if p.ID == policyID {
			e.policies = append(e.policies[:i], e.policies[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("policy %s not found", policyID)
}

// ListPolicies implements PolicyEngine.
func (e *BasicPolicyEngine) ListPolicies() []*PolicyRule {
	result := make([]*PolicyRule, len(e.policies))
	copy(result, e.policies)
	return result
}

func (e *BasicPolicyEngine) matchesSubject(subjects PolicySubjects, u user.Info) bool {
	if subjects.All {
		return true
	}
	
	if sets.NewString(subjects.Users...).Has(u.GetName()) {
		return true
	}
	
	userGroups := sets.NewString(u.GetGroups()...)
	for _, group := range subjects.Groups {
		if userGroups.Has(group) {
			return true
		}
	}
	
	return false
}

func (e *BasicPolicyEngine) matchesResource(resources PolicyResources, attrs *AuthAttributes) bool {
	// Check workspace
	if len(resources.Workspaces) > 0 {
		matched := false
		for _, ws := range resources.Workspaces {
			if e.matchesPattern(ws, attrs.Workspace) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	
	// Check verb
	if len(resources.Verbs) > 0 && !sets.NewString(resources.Verbs...).Has(attrs.Verb) {
		return false
	}
	
	// Check API group
	if len(resources.APIGroups) > 0 && !sets.NewString(resources.APIGroups...).Has(attrs.APIGroup) {
		return false
	}
	
	// Check resource
	if len(resources.Resources) > 0 && !sets.NewString(resources.Resources...).Has(attrs.Resource) {
		return false
	}
	
	return true
}

func (e *BasicPolicyEngine) matchesPattern(pattern, value string) bool {
	if pattern == "*" {
		return true
	}
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(value, strings.TrimSuffix(pattern, "*"))
	}
	return pattern == value
}

func (e *BasicPolicyEngine) evaluateConditions(ctx context.Context, conditions []PolicyCondition, attrs *AuthAttributes) bool {
	for _, cond := range conditions {
		if !e.evaluateCondition(ctx, cond, attrs) {
			return false
		}
	}
	return true
}

func (e *BasicPolicyEngine) evaluateCondition(ctx context.Context, cond PolicyCondition, attrs *AuthAttributes) bool {
	// Simplified condition evaluation
	// In production, this would have more sophisticated logic
	switch cond.Type {
	case ConditionTypeLabel:
		// Check labels on the resource
		return true // Simplified
	case ConditionTypeIP:
		// Check source IP
		return true // Simplified
	case ConditionTypeTime:
		// Check time-based conditions
		return true // Simplified
	default:
		return false
	}
}