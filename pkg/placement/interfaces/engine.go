package interfaces

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
)

// PlacementEngine orchestrates the placement decision process by coordinating
// workspace discovery, policy evaluation, and scheduling algorithms.
// It provides the main entry point for placement operations in the TMC system.
type PlacementEngine interface {
	// FindClusters discovers available clusters across the specified workspaces.
	// It uses the configured WorkspaceDiscovery to traverse workspaces and
	// collect all eligible clusters for placement consideration.
	FindClusters(ctx context.Context, workload runtime.Object,
		workspaces []string) ([]ClusterTarget, error)

	// Evaluate applies placement policies to filter and score clusters.
	// It uses the configured PolicyEvaluator to run policies against each cluster
	// and returns scored results that can be used for scheduling decisions.
	Evaluate(ctx context.Context, policy PlacementPolicy,
		targets []ClusterTarget) ([]ScoredTarget, error)

	// Place makes the final placement decision using the configured scheduler.
	// It takes the scored clusters and applies the scheduling algorithm to
	// determine the optimal placement for the given workload.
	Place(ctx context.Context, workload runtime.Object,
		targets []ScoredTarget) (*PlacementDecision, error)

	// UpdatePlacement updates an existing placement decision.
	// This is used when workload requirements change or when cluster
	// conditions change, requiring recomputation of the placement.
	UpdatePlacement(ctx context.Context, placement *PlacementDecision,
		workload runtime.Object) (*PlacementDecision, error)

	// ValidatePlacement checks if a placement decision is still valid.
	// It verifies that the target clusters are still available and meet
	// the current placement requirements.
	ValidatePlacement(ctx context.Context, placement *PlacementDecision) error

	// GetEngineStatus returns the current status and health of the placement engine.
	GetEngineStatus(ctx context.Context) (*EngineStatus, error)
}

// PlacementEngineOptions configures the placement engine behavior.
// These options control various aspects of the placement process
// including performance tuning and feature enablement.
type PlacementEngineOptions struct {
	// MaxClusters limits the number of clusters to consider during placement.
	// This helps prevent performance issues in large multi-cluster environments.
	MaxClusters int

	// EnableCaching enables result caching for improved performance.
	// When enabled, the engine caches cluster discovery and policy evaluation results.
	EnableCaching bool

	// CacheTTL sets the time-to-live for cached results.
	// After this duration, cached entries are considered stale and refreshed.
	CacheTTL time.Duration

	// ConcurrentEvaluations controls how many policy evaluations run in parallel.
	// Higher values increase throughput but consume more resources.
	ConcurrentEvaluations int

	// MaxPlacementTime sets the maximum time allowed for a placement operation.
	// If exceeded, the placement operation is cancelled and returns an error.
	MaxPlacementTime time.Duration
}

// EngineStatus represents the current status and health of the placement engine.
type EngineStatus struct {
	// Ready indicates if the engine is ready to process placement requests
	Ready bool

	// ActivePlacements is the number of placement operations currently in progress
	ActivePlacements int32

	// TotalPlacements is the total number of placements processed since startup
	TotalPlacements int64

	// AverageProcessingTime is the average time to complete a placement operation
	AverageProcessingTime time.Duration

	// LastError contains the most recent error encountered, if any
	LastError string

	// ComponentStatuses contains the status of individual engine components
	ComponentStatuses map[string]ComponentStatus

	// Timestamp when this status was generated
	Timestamp time.Time
}

// ComponentStatus represents the status of an individual engine component
// such as the workspace discovery, policy evaluator, or scheduler.
type ComponentStatus struct {
	// Name of the component
	Name string

	// Ready indicates if the component is healthy and operational
	Ready bool

	// Message provides additional status information
	Message string

	// LastActivity is the timestamp of the last component activity
	LastActivity time.Time
}