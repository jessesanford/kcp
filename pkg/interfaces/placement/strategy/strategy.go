package strategy

import (
	"context"

	"github.com/kcp-dev/kcp/pkg/interfaces/placement"
)

// PlacementStrategy implements a placement strategy
type PlacementStrategy interface {
	// Name returns strategy name
	Name() string

	// ComputeDistribution calculates replica distribution
	ComputeDistribution(
		ctx context.Context,
		totalReplicas int32,
		targets []*placement.SyncTarget,
		constraints []placement.SpreadConstraint,
	) (map[string]int32, error)

	// Rebalance adjusts existing distribution
	Rebalance(
		ctx context.Context,
		current map[string]int32,
		targets []*placement.SyncTarget,
	) (map[string]int32, error)

	// Validate checks if distribution is valid
	Validate(distribution map[string]int32) error
}

// StrategyFactory creates placement strategies
type StrategyFactory interface {
	// CreateStrategy creates a strategy instance
	CreateStrategy(strategyType placement.PlacementStrategy) (PlacementStrategy, error)

	// RegisterStrategy registers a custom strategy
	RegisterStrategy(name string, strategy PlacementStrategy) error

	// ListStrategies returns available strategies
	ListStrategies() []string

	// GetStrategy retrieves a registered strategy
	GetStrategy(name string) (PlacementStrategy, error)
}

// SpreadStrategy implements spread placement
type SpreadStrategy interface {
	PlacementStrategy

	// SetSpreadConstraints configures constraints
	SetSpreadConstraints(constraints []placement.SpreadConstraint)

	// GetSpreadConstraints returns current constraints
	GetSpreadConstraints() []placement.SpreadConstraint

	// SetMaxSkew sets maximum distribution skew
	SetMaxSkew(maxSkew int32)
}

// BinpackStrategy implements binpack placement
type BinpackStrategy interface {
	PlacementStrategy

	// SetResourceWeights sets resource priorities
	SetResourceWeights(weights map[string]float64)

	// GetResourceWeights returns current weights
	GetResourceWeights() map[string]float64

	// SetPackingEfficiency sets target packing efficiency
	SetPackingEfficiency(efficiency float64)
}

// HighAvailabilityStrategy implements HA placement
type HighAvailabilityStrategy interface {
	PlacementStrategy

	// SetFailureDomains configures failure domains
	SetFailureDomains(domains []string)

	// GetFailureDomains returns configured domains
	GetFailureDomains() []string

	// SetMinReplicas sets minimum replicas per domain
	SetMinReplicas(min int32)

	// GetMinReplicas returns minimum replica requirement
	GetMinReplicas() int32
}

// SingletonStrategy implements singleton placement
type SingletonStrategy interface {
	PlacementStrategy

	// SetPreferredLocation sets preferred placement location
	SetPreferredLocation(location string)

	// GetPreferredLocation returns preferred location
	GetPreferredLocation() string

	// SetFallbackStrategy sets fallback if preferred unavailable
	SetFallbackStrategy(strategy PlacementStrategy)
}

// AffinityStrategy handles affinity rules
type AffinityStrategy interface {
	// ApplyAffinity applies affinity rules
	ApplyAffinity(
		ctx context.Context,
		decision *placement.PlacementDecision,
		rules *placement.AffinityRules,
	) (*placement.PlacementDecision, error)

	// CheckAntiAffinity validates anti-affinity
	CheckAntiAffinity(
		workloadLabels map[string]string,
		targetWorkloads []WorkloadInfo,
	) bool

	// CalculateAffinityScore computes affinity score
	CalculateAffinityScore(
		workload WorkloadInfo,
		target *placement.SyncTarget,
		colocatedWorkloads []WorkloadInfo,
	) (float64, error)
}

// WorkloadInfo contains workload information
type WorkloadInfo struct {
	Name      string
	Namespace string
	Labels    map[string]string
	Location  string
	Priority  int32
	Resources placement.ResourceList
}

// StrategyConfig configures strategy behavior
type StrategyConfig struct {
	// Name of the strategy
	Name string

	// Type of strategy
	Type placement.PlacementStrategy

	// Parameters for the strategy
	Parameters map[string]interface{}

	// Weight for multi-strategy scenarios
	Weight float64

	// Enabled flag for the strategy
	Enabled bool
}

// MultiStrategy combines multiple strategies
type MultiStrategy interface {
	PlacementStrategy

	// AddStrategy adds a weighted strategy
	AddStrategy(strategy PlacementStrategy, weight float64) error

	// RemoveStrategy removes a strategy
	RemoveStrategy(name string) error

	// GetStrategies returns configured strategies
	GetStrategies() map[string]StrategyInfo

	// SetCombinationMode sets how strategies are combined
	SetCombinationMode(mode CombinationMode)
}

// StrategyInfo provides information about a strategy
type StrategyInfo struct {
	// Strategy instance
	Strategy PlacementStrategy

	// Weight in combination
	Weight float64

	// Enabled state
	Enabled bool
}

// CombinationMode defines how multiple strategies are combined
type CombinationMode string

const (
	// CombinationModeWeighted uses weighted average
	CombinationModeWeighted CombinationMode = "Weighted"

	// CombinationModeFirstMatch uses first matching strategy
	CombinationModeFirstMatch CombinationMode = "FirstMatch"

	// CombinationModeBestScore uses strategy with best score
	CombinationModeBestScore CombinationMode = "BestScore"

	// CombinationModeConsensus requires agreement from majority
	CombinationModeConsensus CombinationMode = "Consensus"
)

// StrategyResult contains strategy execution result
type StrategyResult struct {
	// Distribution computed by strategy
	Distribution map[string]int32

	// Score of the distribution
	Score float64

	// Strategy used
	Strategy string

	// ExecutionTime taken
	ExecutionTime int64

	// Metadata about the result
	Metadata map[string]interface{}
}

// StrategyMetrics provides strategy performance metrics
type StrategyMetrics interface {
	// RecordExecution records strategy execution
	RecordExecution(strategy string, duration int64, success bool)

	// RecordDistribution records distribution quality
	RecordDistribution(strategy string, score float64, skew float64)

	// GetMetrics returns current metrics
	GetMetrics() map[string]interface{}

	// Reset clears all metrics
	Reset()
}