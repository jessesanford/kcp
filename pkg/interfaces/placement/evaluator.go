package placement

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ConstraintEvaluator evaluates placement constraints
type ConstraintEvaluator interface {
	// EvaluateTarget checks if target satisfies constraints
	EvaluateTarget(
		ctx context.Context,
		workload *unstructured.Unstructured,
		target *SyncTarget,
		policy *PlacementPolicy,
	) (*EvaluationResult, error)

	// EvaluateTolerations checks tolerations
	EvaluateTolerations(
		tolerations []Toleration,
		taints []Taint,
	) bool

	// EvaluateResources checks resource availability
	EvaluateResources(
		required ResourceList,
		available ResourceList,
	) bool

	// EvaluateAffinity checks affinity rules
	EvaluateAffinity(
		workload *unstructured.Unstructured,
		target *SyncTarget,
		rules *AffinityRules,
	) (*AffinityEvaluationResult, error)
}

// EvaluationResult contains evaluation outcome
type EvaluationResult struct {
	// Suitable for placement
	Suitable bool

	// Score for this target
	Score float64

	// SatisfiedConstraints list
	SatisfiedConstraints []string

	// ViolatedConstraints list
	ViolatedConstraints []string

	// AvailableResources after placement
	AvailableResources ResourceList

	// Message explaining result
	Message string

	// Details provides constraint-specific details
	Details map[string]interface{}
}

// AffinityEvaluationResult contains affinity evaluation details
type AffinityEvaluationResult struct {
	// NodeAffinityScore from node affinity evaluation
	NodeAffinityScore float64

	// PodAffinityScore from pod affinity evaluation
	PodAffinityScore float64

	// PodAntiAffinityScore from anti-affinity evaluation
	PodAntiAffinityScore float64

	// OverallScore combined affinity score
	OverallScore float64

	// Violations contains affinity violations
	Violations []AffinityViolation

	// Satisfactions contains satisfied affinity terms
	Satisfactions []AffinitySatisfaction
}

// AffinityViolation describes an affinity rule violation
type AffinityViolation struct {
	// Type of affinity violated
	Type string

	// Rule that was violated
	Rule string

	// Severity of the violation
	Severity string

	// Message describing the violation
	Message string
}

// AffinitySatisfaction describes satisfied affinity terms
type AffinitySatisfaction struct {
	// Type of affinity satisfied
	Type string

	// Rule that was satisfied
	Rule string

	// Score contribution
	ScoreContribution float64
}

// ResourceEvaluator evaluates resource requirements
type ResourceEvaluator interface {
	// CheckCapacity verifies resource capacity
	CheckCapacity(
		required ResourceList,
		capacity ResourceList,
		allocated ResourceList,
	) bool

	// CalculateUtilization computes resource utilization
	CalculateUtilization(
		allocated ResourceList,
		capacity ResourceList,
	) float64

	// PredictUtilization predicts future utilization
	PredictUtilization(
		current ResourceList,
		additional ResourceList,
		capacity ResourceList,
	) float64

	// GetResourcePressure calculates resource pressure
	GetResourcePressure(
		allocated ResourceList,
		capacity ResourceList,
	) ResourcePressure

	// ComputeFragmentation calculates resource fragmentation
	ComputeFragmentation(
		allocated ResourceList,
		capacity ResourceList,
	) float64
}

// ResourcePressure indicates resource pressure levels
type ResourcePressure struct {
	// MemoryPressure level (0.0-1.0)
	MemoryPressure float64

	// CPUPressure level (0.0-1.0)
	CPUPressure float64

	// DiskPressure level (0.0-1.0)
	DiskPressure float64

	// NetworkPressure level (0.0-1.0)
	NetworkPressure float64

	// OverallPressure aggregate pressure
	OverallPressure float64

	// CriticalResources list of resources under pressure
	CriticalResources []string
}

// TopologyEvaluator evaluates topology constraints
type TopologyEvaluator interface {
	// EvaluateSpread checks spread constraints
	EvaluateSpread(
		currentDistribution map[string]int32,
		constraint SpreadConstraint,
		topology map[string][]string,
	) bool

	// CalculateSkew computes topology skew
	CalculateSkew(
		distribution map[string]int32,
		topologyKey string,
	) int32

	// EvaluateZoneDistribution checks zone distribution
	EvaluateZoneDistribution(
		distribution map[string]int32,
		zoneMap map[string]string,
		minZones int,
	) (*ZoneDistributionResult, error)

	// ValidateTopologySpread validates topology spread constraints
	ValidateTopologySpread(
		workload *unstructured.Unstructured,
		currentPlacements []LocationPlacement,
		constraint SpreadConstraint,
	) bool
}

// ZoneDistributionResult contains zone distribution analysis
type ZoneDistributionResult struct {
	// ZonesUsed number of zones with replicas
	ZonesUsed int

	// ZoneSkew maximum skew between zones
	ZoneSkew int32

	// ZoneDistribution replicas per zone
	ZoneDistribution map[string]int32

	// Compliant whether distribution meets requirements
	Compliant bool

	// Recommendations for improving distribution
	Recommendations []string
}

// TaintTolerationEvaluator evaluates taint/toleration rules
type TaintTolerationEvaluator interface {
	// EvaluateTolerations checks if tolerations match taints
	EvaluateTolerations(
		tolerations []Toleration,
		taints []Taint,
	) (*TolerationEvaluationResult, error)

	// CheckToleration verifies specific toleration
	CheckToleration(
		toleration Toleration,
		taint Taint,
	) bool

	// GetToleratedTaints returns tolerated taints
	GetToleratedTaints(
		tolerations []Toleration,
		taints []Taint,
	) []Taint

	// GetUntoleratedTaints returns untolerated taints
	GetUntoleratedTaints(
		tolerations []Toleration,
		taints []Taint,
	) []Taint
}

// TolerationEvaluationResult contains toleration evaluation details
type TolerationEvaluationResult struct {
	// Tolerated whether all taints are tolerated
	Tolerated bool

	// ToleratedTaints list of tolerated taints
	ToleratedTaints []Taint

	// UntoleratedTaints list of untolerated taints
	UntoleratedTaints []Taint

	// Score impact of toleration matching
	Score float64

	// Details provides additional information
	Details map[string]string
}

// PolicyEvaluator evaluates placement policies
type PolicyEvaluator interface {
	// EvaluatePolicy evaluates entire placement policy
	EvaluatePolicy(
		ctx context.Context,
		workload *unstructured.Unstructured,
		target *SyncTarget,
		policy *PlacementPolicy,
	) (*PolicyEvaluationResult, error)

	// ValidatePolicy validates policy syntax and semantics
	ValidatePolicy(policy *PlacementPolicy) error

	// GetPolicyViolations identifies policy violations
	GetPolicyViolations(
		workload *unstructured.Unstructured,
		placement *PlacementDecision,
		policy *PlacementPolicy,
	) []PolicyViolation
}

// PolicyEvaluationResult contains policy evaluation outcome
type PolicyEvaluationResult struct {
	// Compliant whether target meets policy requirements
	Compliant bool

	// Score overall policy compliance score
	Score float64

	// ConstraintResults detailed constraint evaluation results
	ConstraintResults map[string]*EvaluationResult

	// Violations list of policy violations
	Violations []PolicyViolation

	// Recommendations for improving compliance
	Recommendations []string
}

// PolicyViolation describes a policy violation
type PolicyViolation struct {
	// Type of violation
	Type string

	// Constraint that was violated
	Constraint string

	// Severity of violation
	Severity PolicyViolationSeverity

	// Message describing the violation
	Message string

	// Impact on scheduling
	Impact string

	// Remediation suggestions
	Remediation []string
}

// PolicyViolationSeverity represents violation severity levels
type PolicyViolationSeverity string

const (
	// PolicyViolationSeverityLow indicates low severity violations
	PolicyViolationSeverityLow PolicyViolationSeverity = "Low"

	// PolicyViolationSeverityMedium indicates medium severity violations
	PolicyViolationSeverityMedium PolicyViolationSeverity = "Medium"

	// PolicyViolationSeverityHigh indicates high severity violations
	PolicyViolationSeverityHigh PolicyViolationSeverity = "High"

	// PolicyViolationSeverityCritical indicates critical violations
	PolicyViolationSeverityCritical PolicyViolationSeverity = "Critical"
)