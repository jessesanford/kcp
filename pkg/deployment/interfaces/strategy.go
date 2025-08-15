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

	"github.com/kcp-dev/kcp/pkg/deployment/types"
)

// DeploymentStrategy defines the contract for deployment strategies
type DeploymentStrategy interface {
	// Name returns the strategy name
	Name() string

	// Validate checks if the strategy configuration is valid
	Validate(config types.DeploymentStrategy) error

	// Initialize prepares the strategy for execution
	Initialize(ctx context.Context, config types.DeploymentStrategy) error

	// Execute runs the deployment strategy
	Execute(ctx context.Context, target DeploymentTarget) (*StrategyResult, error)

	// Cleanup performs post-deployment cleanup
	Cleanup(ctx context.Context) error
}

// StrategyFactory creates strategy instances
type StrategyFactory interface {
	// Create returns a strategy for the given type
	Create(strategyType types.StrategyType) (DeploymentStrategy, error)

	// Register adds a new strategy implementation
	Register(strategyType types.StrategyType, strategy DeploymentStrategy) error

	// ListStrategies returns available strategies
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

// ProgressReporter reports deployment progress
type ProgressReporter interface {
	// Report sends progress update
	Report(progress DeploymentProgress) error
}

// DeploymentProgress represents current progress
type DeploymentProgress struct {
	Phase      string  `json:"phase"`
	Percentage float64 `json:"percentage"`
	Message    string  `json:"message,omitempty"`
}