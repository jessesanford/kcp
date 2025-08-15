package types

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Policy represents a TMC policy with CEL-based rules
type Policy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PolicySpec   `json:"spec"`
	Status PolicyStatus `json:"status,omitempty"`
}

// PolicySpec defines the desired state of a Policy
type PolicySpec struct {
	// Rules defines the policy rules
	Rules []PolicyRule `json:"rules"`
	
	// Target defines what the policy applies to
	Target PolicyTarget `json:"target"`
	
	// Mode defines how the policy is enforced
	Mode PolicyMode `json:"mode,omitempty"`
}

// PolicyRule represents a single policy rule
type PolicyRule struct {
	// Name is the unique identifier for this rule
	Name string `json:"name"`
	
	// Expression is the CEL expression to evaluate
	Expression string `json:"expression"`
	
	// Action defines what to do when the rule matches
	Action RuleAction `json:"action"`
	
	// Message provides a human-readable description
	Message string `json:"message,omitempty"`
	
	// Weight for scoring (0 means no weight)
	Weight int32 `json:"weight,omitempty"`
}

// PolicyTarget defines what the policy applies to
type PolicyTarget struct {
	// Clusters target specific clusters
	Clusters []string `json:"clusters,omitempty"`
	
	// Workspaces target specific workspaces
	Workspaces []string `json:"workspaces,omitempty"`
	
	// Labels target resources with specific labels
	Labels map[string]string `json:"labels,omitempty"`
}

// PolicyStatus represents the current status of a Policy
type PolicyStatus struct {
	// Conditions represents the current conditions
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	
	// LastEvaluated is when the policy was last evaluated
	LastEvaluated *metav1.Time `json:"lastEvaluated,omitempty"`
}

// RuleAction defines what action to take when a rule matches
type RuleAction string

const (
	ActionAllow RuleAction = "allow"
	ActionDeny  RuleAction = "deny"
	ActionWarn  RuleAction = "warn"
)

// PolicyMode defines how the policy is enforced
type PolicyMode string

const (
	ModeEnforce PolicyMode = "enforce"
	ModeDryRun  PolicyMode = "dryrun"
	ModeWarn    PolicyMode = "warn"
)

// VariableType represents supported variable types in policy expressions
type VariableType string

const (
	VarTypeBool      VariableType = "bool"
	VarTypeInt       VariableType = "int"
	VarTypeFloat     VariableType = "float"
	VarTypeString    VariableType = "string"
	VarTypeList      VariableType = "list"
	VarTypeMap       VariableType = "map"
	VarTypeCluster   VariableType = "cluster"
	VarTypeWorkload  VariableType = "workload"
	VarTypeWorkspace VariableType = "workspace"
)

// PolicyList contains a list of Policy
type PolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Policy `json:"items"`
}