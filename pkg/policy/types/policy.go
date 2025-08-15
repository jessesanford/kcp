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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// PolicySet represents a collection of policies for TMC placement evaluation.
// It provides a structured way to group related policies and define conflict resolution.
type PolicySet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Policies contains the set of policies to evaluate
	Policies []Policy `json:"policies"`

	// DefaultAction defines what happens when no policy matches
	DefaultAction PolicyAction `json:"defaultAction"`

	// ConflictResolution defines how to handle multiple matching policies
	ConflictResolution ConflictStrategy `json:"conflictResolution,omitempty"`
}

// Policy defines a placement policy with CEL expressions for TMC evaluation.
// Each policy contains rules that are evaluated against placement targets.
type Policy struct {
	// Name uniquely identifies the policy within a policy set
	Name string `json:"name"`

	// Description provides human-readable explanation of policy purpose
	Description string `json:"description,omitempty"`

	// Rules contains the CEL expressions that define policy logic
	Rules []PolicyRule `json:"rules"`

	// Priority determines evaluation order (higher values evaluated first)
	Priority int32 `json:"priority"`

	// Action specifies what happens when this policy matches
	Action PolicyAction `json:"action"`

	// Labels provide metadata for policy categorization and selection
	Labels map[string]string `json:"labels,omitempty"`
}

// PolicyRule represents a single CEL expression with associated metadata.
// Rules are the building blocks of policies and contain the actual evaluation logic.
type PolicyRule struct {
	// Name identifies the rule within a policy
	Name string `json:"name"`

	// Expression is the CEL expression string to evaluate
	Expression string `json:"expression"`

	// Weight affects scoring when multiple rules contribute to a score
	Weight *int32 `json:"weight,omitempty"`

	// Required indicates if this rule must pass for the policy to succeed
	Required bool `json:"required,omitempty"`

	// ErrorMessage provides custom error message when rule fails
	ErrorMessage string `json:"errorMessage,omitempty"`
}

// PolicyAction defines the action to take when a policy matches during placement.
type PolicyAction string

const (
	// AllowAction permits placement but doesn't require it
	AllowAction PolicyAction = "Allow"
	// DenyAction blocks placement to matching targets
	DenyAction PolicyAction = "Deny"
	// PreferAction increases score for matching targets
	PreferAction PolicyAction = "Prefer"
	// RequireAction mandates placement only to matching targets
	RequireAction PolicyAction = "Require"
)

// ConflictStrategy defines how to handle multiple matching policies with different actions.
type ConflictStrategy string

const (
	// FirstMatchStrategy uses the first policy that matches
	FirstMatchStrategy ConflictStrategy = "FirstMatch"
	// HighestPriorityStrategy uses the policy with highest priority
	HighestPriorityStrategy ConflictStrategy = "HighestPriority"
	// MergeStrategy combines results from all matching policies
	MergeStrategy ConflictStrategy = "Merge"
)

// PolicyBinding associates policies with specific resources and workspaces.
// It defines which policies apply to which resources in which contexts.
type PolicyBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// PolicyRef references the policy set to apply
	PolicyRef PolicyReference `json:"policyRef"`

	// Subjects defines which resources the policies apply to
	Subjects []PolicySubject `json:"subjects"`

	// Workspaces limits policy application to specific logical clusters
	Workspaces []string `json:"workspaces,omitempty"`
}

// PolicyReference identifies a policy set by name and optional namespace.
type PolicyReference struct {
	// Name of the policy set
	Name string `json:"name"`
	// Namespace containing the policy set (optional for cluster-scoped policies)
	Namespace string `json:"namespace,omitempty"`
}

// PolicySubject identifies what resources policies should be applied to.
type PolicySubject struct {
	// APIVersion of the target resource
	APIVersion string `json:"apiVersion"`
	// Kind of the target resource
	Kind string `json:"kind"`
	// Name of specific resource (optional, for targeting specific instances)
	Name string `json:"name,omitempty"`
	// Selector for matching resources by labels (mutually exclusive with Name)
	Selector map[string]string `json:"selector,omitempty"`
}

// PolicyContext provides the execution context for policy evaluation.
// It contains the subject being evaluated and variables available to CEL expressions.
type PolicyContext struct {
	// Subject is the Kubernetes resource being evaluated for placement
	Subject runtime.Object `json:"-"`

	// Variables contains data available to CEL expressions during evaluation
	Variables map[string]interface{} `json:"variables"`

	// Metadata provides additional context about the evaluation
	Metadata PolicyMetadata `json:"metadata"`
}

// PolicyMetadata contains information about the policy evaluation context.
type PolicyMetadata struct {
	// EvaluationTime indicates when the evaluation is being performed
	EvaluationTime metav1.Time `json:"evaluationTime"`
	// Evaluator identifies which component is performing the evaluation
	Evaluator string `json:"evaluator"`
	// Workspace identifies the logical cluster context
	Workspace string `json:"workspace,omitempty"`
	// User identifies the user context for the evaluation (if applicable)
	User string `json:"user,omitempty"`
}