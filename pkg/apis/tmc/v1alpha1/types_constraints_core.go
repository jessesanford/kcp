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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	
	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
)

// ConstraintEvaluationEngine provides basic rule evaluation and enforcement for
// session binding constraints. It implements a simple rule-based approach for constraint
// evaluation with basic violation handling.
//
// This resource is workspace-aware and supports KCP logical cluster isolation to ensure
// proper multi-tenancy in KCP environments.
//
// +crd
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={tmc,all}
// +kubebuilder:metadata:annotations="kcp.io/cluster-aware=true"
// +kubebuilder:validation:XValidation:rule="self.metadata.annotations['kcp.io/cluster'] != ''"
// +kubebuilder:printcolumn:name="Engine Type",type=string,JSONPath=`.spec.engineType`
// +kubebuilder:printcolumn:name="Rules Count",type=integer,JSONPath=`.spec.ruleCount`
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:rbac:groups=tmc.kcp.io,resources=constraintevaluationengines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=tmc.kcp.io,resources=constraintevaluationengines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=tmc.kcp.io,resources=constraintevaluationengines/finalizers,verbs=update
type ConstraintEvaluationEngine struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec ConstraintEvaluationEngineSpec `json:"spec,omitempty"`

	// +optional
	Status ConstraintEvaluationEngineStatus `json:"status,omitempty"`
}

// ConstraintEvaluationEngineSpec defines the desired state of the evaluation engine
type ConstraintEvaluationEngineSpec struct {
	// EngineType defines the type of evaluation engine to use
	// Only RuleBasedEngine is supported in this core implementation
	// +kubebuilder:validation:Enum=RuleBasedEngine
	// +kubebuilder:validation:Required
	// +kubebuilder:default="RuleBasedEngine"
	EngineType ConstraintEngineType `json:"engineType"`

	// EvaluationRules defines the rules for constraint evaluation
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=50
	EvaluationRules []ConstraintEvaluationRule `json:"evaluationRules"`

	// RuleCount tracks the number of active evaluation rules
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=50
	RuleCount int32 `json:"ruleCount"`

	// ViolationHandling configures how violations should be processed
	// +optional
	ViolationHandling *BasicViolationHandlingConfig `json:"violationHandling,omitempty"`
}

// ConstraintEngineType defines the types of constraint evaluation engines
type ConstraintEngineType string

const (
	// ConstraintEngineTypeRuleBased uses rule-based evaluation (MVP implementation)
	ConstraintEngineTypeRuleBased ConstraintEngineType = "RuleBasedEngine"
)

// ConstraintEvaluationRule defines a rule for constraint evaluation
type ConstraintEvaluationRule struct {
	// Name is a unique identifier for the rule
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=64
	Name string `json:"name"`

	// RuleType defines the type of evaluation rule
	// Only Threshold and Capacity are supported in this core implementation
	// +kubebuilder:validation:Enum=Threshold;Capacity
	// +kubebuilder:validation:Required
	RuleType EvaluationRuleType `json:"ruleType"`

	// Condition defines when this rule should be evaluated
	// +kubebuilder:validation:Required
	Condition RuleCondition `json:"condition"`

	// Action defines what action to take when rule is triggered
	// +kubebuilder:validation:Required
	Action RuleAction `json:"action"`

	// Priority defines the priority of this rule (higher values are higher priority)
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:default=50
	// +optional
	Priority int32 `json:"priority,omitempty"`

	// Enabled indicates whether this rule is active
	// +kubebuilder:default=true
	Enabled bool `json:"enabled"`

	// Parameters provide rule-specific configuration
	// +optional
	// +kubebuilder:validation:MaxProperties=10
	Parameters map[string]string `json:"parameters,omitempty"`
}

// EvaluationRuleType defines the types of evaluation rules
type EvaluationRuleType string

const (
	// EvaluationRuleTypeThreshold evaluates threshold-based constraints
	EvaluationRuleTypeThreshold EvaluationRuleType = "Threshold"

	// EvaluationRuleTypeCapacity evaluates capacity-based constraints
	EvaluationRuleTypeCapacity EvaluationRuleType = "Capacity"
)

// RuleCondition defines when a rule should be evaluated
type RuleCondition struct {
	// Expression defines the condition expression (CEL format)
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=1024
	Expression string `json:"expression"`

	// EvaluationInterval defines how often to evaluate the condition
	// +kubebuilder:default="60s"
	// +optional
	EvaluationInterval metav1.Duration `json:"evaluationInterval,omitempty"`
}

// RuleAction defines actions to take when a rule is triggered
type RuleAction struct {
	// Type defines the action type
	// Only basic actions are supported in this core implementation
	// +kubebuilder:validation:Enum=Block;Allow;Warn
	// +kubebuilder:validation:Required
	Type ActionType `json:"type"`

	// Configuration provides action-specific settings
	// +optional
	// +kubebuilder:validation:MaxProperties=5
	Configuration map[string]string `json:"configuration,omitempty"`

	// Remediation defines basic remediation actions
	// +optional
	Remediation *BasicRemediationAction `json:"remediation,omitempty"`
}

// ActionType defines the types of rule actions
type ActionType string

const (
	// ActionTypeBlock blocks the operation
	ActionTypeBlock ActionType = "Block"

	// ActionTypeAllow allows the operation
	ActionTypeAllow ActionType = "Allow"

	// ActionTypeWarn allows with warnings
	ActionTypeWarn ActionType = "Warn"
)

// BasicRemediationAction defines basic remediation actions for violations
type BasicRemediationAction struct {
	// Strategy defines the remediation strategy
	// Only basic strategies are supported in this core implementation
	// +kubebuilder:validation:Enum=AutoRemediate;LogOnly
	// +kubebuilder:validation:Required
	Strategy RemediationStrategy `json:"strategy"`

	// MaxAttempts defines maximum remediation attempts
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=5
	// +kubebuilder:default=3
	// +optional
	MaxAttempts int32 `json:"maxAttempts,omitempty"`
}

// RemediationStrategy defines approaches to remediation
type RemediationStrategy string

const (
	// RemediationStrategyAutoRemediate attempts automatic remediation
	RemediationStrategyAutoRemediate RemediationStrategy = "AutoRemediate"

	// RemediationStrategyLogOnly only logs the issue
	RemediationStrategyLogOnly RemediationStrategy = "LogOnly"
)

// BasicViolationHandlingConfig defines how violations should be processed
type BasicViolationHandlingConfig struct {
	// TrackingEnabled enables violation tracking
	// +kubebuilder:default=true
	TrackingEnabled bool `json:"trackingEnabled"`

	// MaxViolationHistory defines how many violations to track
	// +kubebuilder:validation:Minimum=10
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:default=50
	// +optional
	MaxViolationHistory int32 `json:"maxViolationHistory,omitempty"`

	// AlertingEnabled enables basic alerting for violations
	// +kubebuilder:default=false
	// +optional
	AlertingEnabled bool `json:"alertingEnabled,omitempty"`
}

// ConstraintEvaluationEngineStatus represents the observed state
type ConstraintEvaluationEngineStatus struct {
	// Conditions represent the latest available observations
	// +optional
	Conditions conditionsv1alpha1.Conditions `json:"conditions,omitempty"`

	// Phase indicates the current engine phase
	// +kubebuilder:default="Active"
	// +optional
	Phase ConstraintEnginePhase `json:"phase,omitempty"`

	// LastUpdateTime is when the status was last updated
	// +optional
	LastUpdateTime *metav1.Time `json:"lastUpdateTime,omitempty"`

	// EvaluationMetrics provides basic performance metrics
	// +optional
	EvaluationMetrics *BasicEvaluationMetrics `json:"evaluationMetrics,omitempty"`

	// RuleStatistics provides rule execution statistics
	// +optional
	RuleStatistics []RuleExecutionStats `json:"ruleStatistics,omitempty"`

	// ViolationSummary provides basic violation summary information
	// +optional
	ViolationSummary *BasicViolationSummary `json:"violationSummary,omitempty"`

	// Message provides additional information about the engine state
	// +optional
	Message string `json:"message,omitempty"`
}

// ConstraintEnginePhase represents the current phase of the engine
type ConstraintEnginePhase string

const (
	// ConstraintEnginePhaseActive indicates the engine is active
	ConstraintEnginePhaseActive ConstraintEnginePhase = "Active"

	// ConstraintEnginePhasePending indicates the engine is starting up
	ConstraintEnginePhasePending ConstraintEnginePhase = "Pending"

	// ConstraintEnginePhaseFailed indicates the engine has failed
	ConstraintEnginePhaseFailed ConstraintEnginePhase = "Failed"

	// ConstraintEnginePhaseUpdating indicates the engine is updating
	ConstraintEnginePhaseUpdating ConstraintEnginePhase = "Updating"
)

// BasicEvaluationMetrics provides basic performance metrics for the engine
type BasicEvaluationMetrics struct {
	// TotalEvaluations tracks total evaluations performed
	TotalEvaluations int64 `json:"totalEvaluations"`

	// SuccessfulEvaluations tracks successful evaluations
	SuccessfulEvaluations int64 `json:"successfulEvaluations"`

	// FailedEvaluations tracks failed evaluations
	FailedEvaluations int64 `json:"failedEvaluations"`

	// AverageEvaluationTime tracks average evaluation time
	// +optional
	AverageEvaluationTime metav1.Duration `json:"averageEvaluationTime,omitempty"`
}

// RuleExecutionStats provides statistics for rule execution
type RuleExecutionStats struct {
	// RuleName identifies the rule
	RuleName string `json:"ruleName"`

	// ExecutionCount tracks how many times the rule was executed
	ExecutionCount int64 `json:"executionCount"`

	// TriggerCount tracks how many times the rule was triggered
	TriggerCount int64 `json:"triggerCount"`

	// LastExecutionTime tracks when the rule was last executed
	// +optional
	LastExecutionTime *metav1.Time `json:"lastExecutionTime,omitempty"`

	// ErrorCount tracks execution errors
	ErrorCount int64 `json:"errorCount"`
}

// BasicViolationSummary provides basic violation summary statistics
type BasicViolationSummary struct {
	// TotalViolations tracks total violations detected
	TotalViolations int64 `json:"totalViolations"`

	// ActiveViolations tracks currently active violations
	ActiveViolations int64 `json:"activeViolations"`

	// ResolvedViolations tracks resolved violations
	ResolvedViolations int64 `json:"resolvedViolations"`
}

// ConstraintEvaluationEngineList is a list of ConstraintEvaluationEngine resources
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ConstraintEvaluationEngineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ConstraintEvaluationEngine `json:"items"`
}

// GetConditions returns the conditions for the ConstraintEvaluationEngine.
func (cee *ConstraintEvaluationEngine) GetConditions() conditionsv1alpha1.Conditions {
	return cee.Status.Conditions
}

// SetConditions sets the conditions for the ConstraintEvaluationEngine.
func (cee *ConstraintEvaluationEngine) SetConditions(conditions conditionsv1alpha1.Conditions) {
	cee.Status.Conditions = conditions
}

// ConstraintEvaluationEngine condition types
const (
	// ConstraintEvaluationEngineConditionReady indicates the engine is ready
	ConstraintEvaluationEngineConditionReady conditionsv1alpha1.ConditionType = "Ready"

	// ConstraintEvaluationEngineConditionEvaluating indicates the engine is evaluating
	ConstraintEvaluationEngineConditionEvaluating conditionsv1alpha1.ConditionType = "Evaluating"
)