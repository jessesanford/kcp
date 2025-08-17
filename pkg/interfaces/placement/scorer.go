package placement

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Scorer scores placement options
type Scorer interface {
	// ScoreTarget scores a single target
	ScoreTarget(
		ctx context.Context,
		workload *unstructured.Unstructured,
		target *SyncTarget,
	) (float64, error)

	// ScorePlacement scores entire placement
	ScorePlacement(
		ctx context.Context,
		decision *PlacementDecision,
	) (float64, error)

	// NormalizeScores normalizes scores to 0-100
	NormalizeScores(scores []float64) []float64

	// GetScoringBreakdown provides detailed scoring breakdown
	GetScoringBreakdown(
		workload *unstructured.Unstructured,
		target *SyncTarget,
	) (*ScoringBreakdown, error)
}

// ScoringBreakdown provides detailed score analysis
type ScoringBreakdown struct {
	// TotalScore overall score
	TotalScore float64

	// ComponentScores breakdown by component
	ComponentScores map[string]float64

	// Weights used for each component
	Weights map[string]float64

	// Penalties applied
	Penalties map[string]float64

	// Bonuses applied
	Bonuses map[string]float64

	// Explanation of scoring logic
	Explanation string
}

// ScoringFunction computes scores
type ScoringFunction func(
	workload *unstructured.Unstructured,
	target *SyncTarget,
) float64

// ScorerPlugin is a scoring plugin
type ScorerPlugin interface {
	// Name returns plugin name
	Name() string

	// Weight returns scoring weight
	Weight() float64

	// Score computes score
	Score(
		workload *unstructured.Unstructured,
		target *SyncTarget,
	) (float64, error)

	// Initialize initializes the plugin
	Initialize(config map[string]interface{}) error

	// Validate validates plugin configuration
	Validate() error
}

// ScoringFramework manages scoring plugins
type ScoringFramework interface {
	// AddPlugin adds a scoring plugin
	AddPlugin(plugin ScorerPlugin) error

	// RemovePlugin removes a plugin
	RemovePlugin(name string) error

	// ComputeScore runs all plugins
	ComputeScore(
		workload *unstructured.Unstructured,
		target *SyncTarget,
	) (float64, error)

	// ListPlugins returns registered plugins
	ListPlugins() []string

	// GetPlugin retrieves a specific plugin
	GetPlugin(name string) (ScorerPlugin, error)

	// SetWeights updates plugin weights
	SetWeights(weights map[string]float64) error
}

// ResourceUtilizationScorer scores based on resource utilization
type ResourceUtilizationScorer interface {
	ScorerPlugin

	// ScoreResourceUtilization scores resource usage efficiency
	ScoreResourceUtilization(
		allocated ResourceList,
		capacity ResourceList,
	) float64

	// SetUtilizationTarget sets target utilization level
	SetUtilizationTarget(target float64)

	// GetUtilizationTarget returns current target
	GetUtilizationTarget() float64
}

// LocalityScorer scores based on locality preferences
type LocalityScorer interface {
	ScorerPlugin

	// ScoreLocality scores based on location preferences
	ScoreLocality(
		workload *unstructured.Unstructured,
		target *SyncTarget,
	) float64

	// SetLocalityPreferences sets locality preferences
	SetLocalityPreferences(preferences map[string]float64)

	// GetLocalityPreferences returns current preferences
	GetLocalityPreferences() map[string]float64
}

// AffinityScorer scores based on affinity rules
type AffinityScorer interface {
	ScorerPlugin

	// ScoreAffinity scores affinity compliance
	ScoreAffinity(
		workload *unstructured.Unstructured,
		target *SyncTarget,
		rules *AffinityRules,
	) float64

	// ScoreAntiAffinity scores anti-affinity compliance
	ScoreAntiAffinity(
		workload *unstructured.Unstructured,
		target *SyncTarget,
		rules *AffinityRules,
	) float64
}

// LoadBalancingScorer scores for load distribution
type LoadBalancingScorer interface {
	ScorerPlugin

	// ScoreLoadBalance scores load balancing effectiveness
	ScoreLoadBalance(
		currentLoad map[string]float64,
		targetLoad string,
	) float64

	// SetLoadBalanceStrategy sets balancing strategy
	SetLoadBalanceStrategy(strategy LoadBalanceStrategy)

	// GetLoadBalanceStrategy returns current strategy
	GetLoadBalanceStrategy() LoadBalanceStrategy
}

// LoadBalanceStrategy defines load balancing approach
type LoadBalanceStrategy string

const (
	// LoadBalanceStrategyEven distributes load evenly
	LoadBalanceStrategyEven LoadBalanceStrategy = "Even"

	// LoadBalanceStrategyWeighted uses weighted distribution
	LoadBalanceStrategyWeighted LoadBalanceStrategy = "Weighted"

	// LoadBalanceStrategyCapacityBased bases on target capacity
	LoadBalanceStrategyCapacityBased LoadBalanceStrategy = "CapacityBased"

	// LoadBalanceStrategyMinimizeHotspots avoids hotspots
	LoadBalanceStrategyMinimizeHotspots LoadBalanceStrategy = "MinimizeHotspots"
)

// CustomScorer allows custom scoring implementations
type CustomScorer interface {
	ScorerPlugin

	// SetScoringFunction sets custom scoring function
	SetScoringFunction(fn ScoringFunction)

	// GetScoringFunction returns current function
	GetScoringFunction() ScoringFunction

	// SetParameters sets custom parameters
	SetParameters(params map[string]interface{})
}

// ScoringConfig configures scoring behavior
type ScoringConfig struct {
	// Plugins list of plugins to use
	Plugins []PluginConfig

	// DefaultWeight for unspecified plugins
	DefaultWeight float64

	// MaxScore maximum possible score
	MaxScore float64

	// MinScore minimum possible score
	MinScore float64

	// NormalizationMode how to normalize scores
	NormalizationMode NormalizationMode
}

// NormalizationMode defines score normalization approach
type NormalizationMode string

const (
	// NormalizationModeLinear uses linear normalization
	NormalizationModeLinear NormalizationMode = "Linear"

	// NormalizationModeLogarithmic uses logarithmic normalization
	NormalizationModeLogarithmic NormalizationMode = "Logarithmic"

	// NormalizationModePercentile uses percentile normalization
	NormalizationModePercentile NormalizationMode = "Percentile"

	// NormalizationModeNone disables normalization
	NormalizationModeNone NormalizationMode = "None"
)

// ScoringMetrics tracks scoring performance
type ScoringMetrics interface {
	// RecordScoring records a scoring operation
	RecordScoring(plugin string, duration int64, score float64)

	// GetAverageScore returns average score for plugin
	GetAverageScore(plugin string) float64

	// GetScoringLatency returns scoring latency
	GetScoringLatency(plugin string) int64

	// GetScoringAccuracy measures scoring accuracy
	GetScoringAccuracy(plugin string) float64

	// Reset clears all metrics
	Reset()
}