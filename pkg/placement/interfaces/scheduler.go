package interfaces

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
)

// Scheduler implements placement scheduling algorithms to select the best
// clusters for workload placement. Different schedulers can implement
// various algorithms like bin-packing, spreading, or custom strategies.
//
// Context Propagation:
// The Schedule method uses context.Context for:
// - Workspace-aware scheduling decisions and cluster filtering
// - User identity for RBAC-scoped cluster access validation
// - Request timeouts and graceful cancellation of scheduling operations
// - Workspace isolation when evaluating cluster affinity rules
type Scheduler interface {
	// Schedule determines the best placement based on the configured algorithm.
	// It takes a workload and available clusters, then returns scored clusters
	// ordered by their suitability for hosting the workload.
	Schedule(ctx context.Context, workload runtime.Object,
		clusters []ClusterTarget) ([]ScoredTarget, error)

	// Algorithm returns the name of the scheduling algorithm implemented.
	// Examples: "binpack", "spread", "balanced", "custom"
	Algorithm() string

	// Configure applies scheduler-specific options and parameters.
	// This allows runtime configuration of scheduling behavior.
	Configure(options SchedulerOptions) error

	// GetCapabilities returns the capabilities and features supported by this scheduler.
	GetCapabilities() SchedulerCapabilities

	// ValidateConfiguration checks if the provided options are valid for this scheduler.
	ValidateConfiguration(options SchedulerOptions) error
}

// SchedulerOptions configures scheduling behavior and algorithm parameters.
// Different schedulers may use different subsets of these options.
type SchedulerOptions struct {
	// Strategy defines the high-level scheduling strategy to use.
	// Common values: "binpack", "spread", "balanced", "priority"
	Strategy string

	// Weights assigns importance to different scoring factors.
	// Keys are factor names, values are weights (0.0 to 1.0).
	Weights map[string]float64

	// Constraints define hard requirements that must be satisfied.
	// These are scheduler-specific constraint expressions.
	Constraints []string

	// MaxClustersPerPlacement limits how many clusters can be selected.
	// 0 means no limit, positive values set an upper bound.
	MaxClustersPerPlacement int

	// EnablePreemption allows the scheduler to consider displacing existing workloads.
	EnablePreemption bool

	// AffinityRules define workload affinity and anti-affinity preferences.
	AffinityRules []AffinityRule

	// CustomParameters provides scheduler-specific configuration options.
	CustomParameters map[string]interface{}
}

// AffinityRule defines workload placement affinity or anti-affinity rules.
type AffinityRule struct {
	// Type specifies whether this is an affinity or anti-affinity rule
	Type AffinityType

	// Selector identifies workloads this rule applies to
	Selector map[string]string

	// Weight defines the rule strength (0-100, higher means stronger preference)
	Weight int32

	// Required indicates if this is a hard constraint or soft preference
	Required bool
}

// AffinityType represents the type of affinity rule
type AffinityType string

const (
	// AffinityTypeAttraction workloads should be placed on the same cluster
	AffinityTypeAttraction AffinityType = "attraction"
	// AffinityTypeRepulsion workloads should be placed on different clusters
	AffinityTypeRepulsion AffinityType = "repulsion"
)

// SchedulerCapabilities describes what features a scheduler supports.
type SchedulerCapabilities struct {
	// SupportedStrategies lists the scheduling strategies this scheduler can use
	SupportedStrategies []string

	// SupportsBatching indicates if the scheduler can handle batch placement requests
	SupportsBatching bool

	// SupportsPreemption indicates if the scheduler supports workload preemption
	SupportsPreemption bool

	// SupportsAffinity indicates if the scheduler supports affinity rules
	SupportsAffinity bool

	// MaxClustersSupported is the maximum number of clusters this scheduler can handle
	MaxClustersSupported int
}

// SchedulerFactory creates scheduler instances for different algorithms.
// This factory pattern allows runtime selection of scheduling strategies.
type SchedulerFactory interface {
	// Create returns a new scheduler instance for the specified strategy.
	// Returns an error if the strategy is not supported.
	Create(strategy string) (Scheduler, error)

	// ListStrategies returns all available scheduling strategies.
	ListStrategies() []string

	// GetDefaultStrategy returns the recommended default strategy.
	GetDefaultStrategy() string

	// ValidateStrategy checks if a strategy name is supported.
	ValidateStrategy(strategy string) error
}