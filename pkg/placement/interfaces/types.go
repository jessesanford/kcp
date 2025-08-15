package interfaces

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/kcp-dev/logicalcluster/v3"
)

// PlacementDecision represents the result of a placement operation.
// It contains the selected clusters, policy evaluation results,
// and metadata about the scheduling process.
type PlacementDecision struct {
	// TargetClusters lists the selected clusters with their scores
	TargetClusters []ScoredTarget `json:"targetClusters"`

	// PolicyEvaluations contains the results of policy evaluations
	PolicyEvaluations []PolicyResult `json:"policyEvaluations,omitempty"`

	// SchedulingResult contains details about the scheduling algorithm used
	SchedulingResult *SchedulingResult `json:"schedulingResult,omitempty"`

	// Timestamp when the decision was made
	Timestamp metav1.Time `json:"timestamp"`

	// DecisionID uniquely identifies this placement decision
	DecisionID string `json:"decisionID,omitempty"`
}

// ClusterTarget represents a potential target cluster for workload placement.
// It combines cluster identity, location, resource capacity, and metadata.
type ClusterTarget struct {
	// Name is the cluster name
	Name string `json:"name"`

	// Workspace is the logical cluster path where this cluster resides
	Workspace logicalcluster.Name `json:"workspace"`

	// Location is the physical location reference
	Location *LocationInfo `json:"location,omitempty"`

	// Capacity represents available resources in the cluster
	Capacity ResourceCapacity `json:"capacity"`

	// Labels from the cluster registration
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations from the cluster registration
	Annotations map[string]string `json:"annotations,omitempty"`

	// Ready indicates if the cluster is available for placement
	Ready bool `json:"ready"`
}

// ScoredTarget is a cluster with its placement score and reasoning.
// The score is calculated by the scheduler based on various factors
// like resource availability, policy constraints, and scheduling preferences.
type ScoredTarget struct {
	ClusterTarget
	// Score represents the placement score (0-100, higher is better)
	Score int32 `json:"score"`
	// Reasons provides human-readable explanations for the score
	Reasons []string `json:"reasons,omitempty"`
}

// ResourceCapacity describes available resources in a cluster.
// These values are used for bin-packing and resource-aware scheduling.
type ResourceCapacity struct {
	// CPU capacity in Kubernetes resource format (e.g., "4", "4000m")
	CPU string `json:"cpu"`
	// Memory capacity in Kubernetes resource format (e.g., "8Gi", "8192Mi")
	Memory string `json:"memory"`
	// Maximum number of pods the cluster can support
	Pods int32 `json:"pods"`
	// Storage capacity available for persistent volumes
	Storage string `json:"storage,omitempty"`
}

// PlacementPolicy defines placement rules and constraints.
// Policies are evaluated against clusters to determine placement eligibility.
type PlacementPolicy struct {
	// Name identifies the policy
	Name string `json:"name"`

	// Rules contains policy expressions (CEL or other formats)
	Rules []PolicyRule `json:"rules"`

	// Priority determines evaluation order (higher values evaluated first)
	Priority int32 `json:"priority"`

	// Required indicates if this policy must pass for placement
	Required bool `json:"required,omitempty"`
}

// PolicyRule represents a single policy constraint or preference.
// Rules can be hard constraints (must pass) or soft preferences (scoring).
type PolicyRule struct {
	// Expression is the policy rule in CEL or other expression language
	Expression string `json:"expression"`
	// Weight affects scoring when the rule is a preference (0-100)
	Weight int32 `json:"weight,omitempty"`
	// Type indicates if this is a constraint or preference
	Type PolicyRuleType `json:"type,omitempty"`
}

// PolicyRuleType defines the type of policy rule
type PolicyRuleType string

const (
	// PolicyRuleConstraint represents a hard constraint that must pass
	PolicyRuleConstraint PolicyRuleType = "constraint"
	// PolicyRulePreference represents a soft preference used for scoring
	PolicyRulePreference PolicyRuleType = "preference"
)

// PolicyResult contains the outcome of evaluating a policy against a cluster.
type PolicyResult struct {
	// PolicyName identifies which policy was evaluated
	PolicyName string `json:"policyName"`
	// Passed indicates if the policy evaluation succeeded
	Passed bool `json:"passed"`
	// Score is the numeric score from the evaluation (0-100)
	Score int32 `json:"score,omitempty"`
	// Message provides additional context or error information
	Message string `json:"message,omitempty"`
	// RuleResults contains results for individual rules within the policy
	RuleResults []RuleResult `json:"ruleResults,omitempty"`
}

// RuleResult contains the outcome of evaluating a single policy rule.
type RuleResult struct {
	// Expression is the rule expression that was evaluated
	Expression string `json:"expression"`
	// Passed indicates if the rule evaluation succeeded
	Passed bool `json:"passed"`
	// Score is the numeric contribution to the overall policy score
	Score int32 `json:"score,omitempty"`
	// Error contains any evaluation error that occurred
	Error string `json:"error,omitempty"`
}

// SchedulingResult contains details about the scheduling algorithm execution.
type SchedulingResult struct {
	// Algorithm identifies the scheduling algorithm used
	Algorithm string `json:"algorithm"`
	// Duration is how long the scheduling took
	Duration time.Duration `json:"duration"`
	// Iterations is the number of scheduling iterations performed
	Iterations int `json:"iterations,omitempty"`
	// ClustersEvaluated is the total number of clusters considered
	ClustersEvaluated int `json:"clustersEvaluated,omitempty"`
}

// WorkspaceInfo represents a KCP workspace with its metadata and hierarchy position.
type WorkspaceInfo struct {
	// Name is the logical cluster path of the workspace
	Name logicalcluster.Name `json:"name"`
	// Parent is the parent workspace (nil for root workspaces)
	Parent *logicalcluster.Name `json:"parent,omitempty"`
	// Labels from the workspace
	Labels map[string]string `json:"labels,omitempty"`
	// Annotations from the workspace
	Annotations map[string]string `json:"annotations,omitempty"`
	// Ready indicates if the workspace is available for placement
	Ready bool `json:"ready"`
}

// LocationInfo represents a physical location where a cluster resides.
// This is a simplified version of the workload API Location type.
type LocationInfo struct {
	// Name is the location identifier
	Name string `json:"name"`
	// Region is the geographic region
	Region string `json:"region,omitempty"`
	// Zone is the availability zone within the region  
	Zone string `json:"zone,omitempty"`
	// Provider is the cloud or infrastructure provider
	Provider string `json:"provider,omitempty"`
	// Labels for additional location metadata
	Labels map[string]string `json:"labels,omitempty"`
}

// PlacementContext provides context information during placement operations.
// This context is passed to various placement components for decision making.
type PlacementContext struct {
	// Workload is the object being placed
	Workload runtime.Object `json:"-"`
	// User is the user requesting the placement
	User string `json:"user,omitempty"`
	// Groups are the groups the user belongs to
	Groups []string `json:"groups,omitempty"`
	// RequestID uniquely identifies this placement request
	RequestID string `json:"requestID,omitempty"`
	// Timestamp when the placement was requested
	Timestamp metav1.Time `json:"timestamp"`
}