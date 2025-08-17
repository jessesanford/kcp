package conflict

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"
)

// Resolver is the main conflict resolution engine that coordinates conflict detection
// and applies appropriate resolution strategies for KCP syncer conflicts
type Resolver struct {
	defaultStrategy ResolutionStrategy
	strategies      map[ResolutionStrategy]ResolutionStrategyInterface
	detector        *ConflictDetector
	config          *ResolverConfig
}

// ResolverConfig contains configuration for the conflict resolver
type ResolverConfig struct {
	MaxConflictAge       time.Duration
	StrategyOverrides    map[schema.GroupVersionResource]ResolutionStrategy
	CriticalResources    []schema.GroupVersionResource
}

// NewResolver creates a new conflict resolver with the specified default strategy
func NewResolver(defaultStrategy ResolutionStrategy, config *ResolverConfig) *Resolver {
	if config == nil {
		config = &ResolverConfig{
			MaxConflictAge:    30 * time.Minute,
			StrategyOverrides: make(map[schema.GroupVersionResource]ResolutionStrategy),
		}
	}

	r := &Resolver{
		defaultStrategy: defaultStrategy,
		strategies:      make(map[ResolutionStrategy]ResolutionStrategyInterface),
		detector:        NewConflictDetector(),
		config:          config,
	}

	// Register built-in resolution strategies
	r.strategies[KCPWins] = &KCPWinsStrategy{}
	r.strategies[DownstreamWins] = &DownstreamWinsStrategy{}
	r.strategies[Merge] = &MergeStrategy{}
	r.strategies[Manual] = &ManualStrategy{}

	return r
}

// ResolveConflict is the main entry point for conflict resolution
func (r *Resolver) ResolveConflict(ctx context.Context, kcp, downstream *unstructured.Unstructured) (*ResolutionResult, error) {
	logger := klog.FromContext(ctx)

	// Validate inputs
	if kcp == nil && downstream == nil {
		return nil, fmt.Errorf("both KCP and downstream resources cannot be nil")
	}

	// Detect conflicts using the conflict detector
	conflict := r.detector.DetectConflict(kcp, downstream)
	if conflict == nil {
		logger.V(4).Info("No conflicts detected")
		return &ResolutionResult{Resolved: true}, nil
	}

	logger.V(2).Info("Conflict detected",
		"type", conflict.Type,
		"severity", conflict.Severity,
		"resource", fmt.Sprintf("%s/%s", conflict.Namespace, conflict.Name))


	// Select the appropriate resolution strategy
	strategy := r.selectStrategy(conflict)
	logger.V(3).Info("Selected resolution strategy", "strategy", strategy)

	// Get the strategy implementation
	resolver, exists := r.strategies[strategy]
	if !exists {
		return nil, fmt.Errorf("unknown resolution strategy: %s", strategy)
	}

	// Apply the resolution strategy
	result, err := resolver.Resolve(ctx, kcp, downstream, conflict)
	if err != nil {
		logger.Error(err, "Failed to resolve conflict", "strategy", strategy)
		return nil, fmt.Errorf("resolution strategy %s failed: %w", strategy, err)
	}

	// Set the strategy used in the result
	result.Strategy = strategy

	// Log resolution outcome
	if result.Resolved {
		logger.V(2).Info("Conflict successfully resolved", "strategy", strategy)
	} else {
		logger.Info("Conflict requires manual intervention", "strategy", strategy)
	}

	return result, nil
}

// selectStrategy chooses the appropriate resolution strategy based on conflict characteristics
func (r *Resolver) selectStrategy(conflict *Conflict) ResolutionStrategy {
	// Check for resource-specific strategy overrides first
	if override, exists := r.config.StrategyOverrides[conflict.GVR]; exists {
		return override
	}

	// Check if this is a critical resource that requires manual resolution
	if r.isCriticalResource(conflict.GVR) {
		return Manual
	}

	// Apply strategy selection logic based on conflict characteristics
	switch conflict.Type {
	case DeletedConflict:
		if conflict.Severity == CriticalSeverity {
			return Manual
		}
		return KCPWins // Generally recreate if deleted downstream
	case OwnershipConflict:
		return Manual // Ownership conflicts are always critical
	case VersionConflict:
		if conflict.Severity >= HighSeverity {
			return Manual
		}
		if conflict.Severity == MediumSeverity {
			return Merge
		}
		return r.defaultStrategy
	case SemanticConflict:
		if conflict.Severity >= HighSeverity {
			return Manual
		}
		if conflict.Severity == MediumSeverity {
			return Merge
		}
		return r.defaultStrategy
	default:
		return r.defaultStrategy
	}
}


// isCriticalResource checks if a resource type is marked as critical
func (r *Resolver) isCriticalResource(gvr schema.GroupVersionResource) bool {
	for _, critical := range r.config.CriticalResources {
		if gvr == critical {
			return true
		}
	}
	return false
}

// RegisterStrategy allows registration of custom resolution strategies
func (r *Resolver) RegisterStrategy(strategy ResolutionStrategy, impl ResolutionStrategyInterface) {
	r.strategies[strategy] = impl
}

// SetStrategyOverride sets a strategy override for a specific resource type
func (r *Resolver) SetStrategyOverride(gvr schema.GroupVersionResource, strategy ResolutionStrategy) {
	r.config.StrategyOverrides[gvr] = strategy
}

// GetSupportedStrategies returns a list of all registered resolution strategies
func (r *Resolver) GetSupportedStrategies() []ResolutionStrategy {
	strategies := make([]ResolutionStrategy, 0, len(r.strategies))
	for strategy := range r.strategies {
		strategies = append(strategies, strategy)
	}
	return strategies
}