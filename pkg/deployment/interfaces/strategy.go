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

package interfaces

import (
	"context"

	"github.com/kcp-dev/kcp/pkg/apis/core"
	"github.com/kcp-dev/kcp/pkg/deployment/types"
)

// DeploymentStrategy defines the contract for deployment strategies within KCP's 
// logical cluster architecture. All strategy implementations must be cluster-aware
// and respect workspace isolation boundaries.
//
// Implementations must handle:
//   - Cross-logical-cluster deployments
//   - Workspace-scoped resource access
//   - KCP API export/binding patterns
//   - Proper error handling for multi-tenant environments
//
// Example implementation pattern:
//   func (s *CanaryStrategy) Execute(ctx context.Context, target DeploymentTarget) (*StrategyResult, error) {
//     // Extract logical cluster from target
//     cluster := target.LogicalCluster
//     // Use cluster-scoped client operations
//     client := s.clientFor(cluster)
//     // Perform deployment within cluster boundary
//     return s.executeInCluster(ctx, client, target)
//   }
type DeploymentStrategy interface {
	// Name returns the strategy name for identification and logging
	Name() string

	// Validate checks if the strategy configuration is valid for the target environment
	Validate(config types.DeploymentStrategy) error

	// Initialize prepares the strategy for execution in the target logical cluster
	Initialize(ctx context.Context, config types.DeploymentStrategy) error

	// Execute runs the deployment strategy within the target's logical cluster context
	Execute(ctx context.Context, target DeploymentTarget) (*StrategyResult, error)

	// ExecuteInCluster runs the deployment strategy in a specific logical cluster
	ExecuteInCluster(ctx context.Context, cluster core.LogicalCluster, target DeploymentTarget) (*StrategyResult, error)

	// Cleanup performs post-deployment cleanup across all affected logical clusters
	Cleanup(ctx context.Context) error
}

// StrategyFactory creates strategy instances and manages strategy registration.
// Factory implementations MUST be thread-safe and support concurrent access
// from multiple goroutines across different logical clusters.
//
// Thread Safety Requirements:
//   - All methods must be safe for concurrent use
//   - Strategy registration must be atomic
//   - Strategy creation must not modify shared state
//   - Use sync.RWMutex or similar for protection
//
// Example thread-safe implementation:
//   type SafeStrategyFactory struct {
//     mu         sync.RWMutex
//     strategies map[types.StrategyType]DeploymentStrategy
//   }
type StrategyFactory interface {
	// Create returns a strategy for the given type (must be thread-safe)
	Create(strategyType types.StrategyType) (DeploymentStrategy, error)

	// CreateForCluster returns a cluster-aware strategy instance
	CreateForCluster(strategyType types.StrategyType, cluster core.LogicalCluster) (DeploymentStrategy, error)

	// Register adds a new strategy implementation (must be thread-safe)
	Register(strategyType types.StrategyType, strategy DeploymentStrategy) error

	// ListStrategies returns available strategies (must be thread-safe)
	ListStrategies() []types.StrategyType
}

// StrategyResult contains strategy execution outcome
type StrategyResult struct {
	Success    bool                   `json:"success"`
	Message    string                 `json:"message,omitempty"`
	Metrics    map[string]interface{} `json:"metrics,omitempty"`
	NextAction StrategyAction         `json:"nextAction,omitempty"`
}

// StrategyAction defines next steps
type StrategyAction string

const (
	ContinueAction StrategyAction = "Continue"
	PauseAction    StrategyAction = "Pause"
	RollbackAction StrategyAction = "Rollback"
	CompleteAction StrategyAction = "Complete"
)

// ProgressReporter reports deployment progress across logical clusters.
// Implementations should be thread-safe and handle concurrent progress updates
// from multiple deployment strategies running in parallel.
//
// Progress reporting must respect workspace boundaries and only expose
// progress information to authorized users within the appropriate logical cluster.
type ProgressReporter interface {
	// Report sends progress update (must be thread-safe)
	Report(progress DeploymentProgress) error

	// ReportForCluster sends progress update for a specific logical cluster
	ReportForCluster(cluster core.LogicalCluster, progress DeploymentProgress) error
}

// DeploymentProgress represents current progress
type DeploymentProgress struct {
	Phase      string  `json:"phase"`
	Percentage float64 `json:"percentage"`
	Message    string  `json:"message,omitempty"`
}