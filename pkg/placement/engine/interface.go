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

package engine

import (
	"context"

	tmcv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/tmc/v1alpha1"
)

// PlacementEngine defines the interface for cluster placement algorithms.
// Implementations determine which clusters should host workloads based on
// placement policies, resource requirements, and cluster availability.
type PlacementEngine interface {
	// SelectClusters evaluates available clusters and returns placement decisions
	// for the specified workload placement request.
	//
	// Parameters:
	//   - ctx: Request context for cancellation and timeout
	//   - workload: WorkloadPlacement resource defining placement requirements
	//   - clusters: Available ClusterRegistration resources to consider
	//
	// Returns:
	//   - []PlacementDecision: Ordered list of placement decisions (highest score first)
	//   - error: Selection error or validation failure
	SelectClusters(ctx context.Context,
		workload *tmcv1alpha1.WorkloadPlacement,
		clusters []*tmcv1alpha1.ClusterRegistration,
	) ([]PlacementDecision, error)
}

// PlacementDecision represents a placement decision made by the engine.
// Each decision includes the target cluster, placement score, and rationale.
type PlacementDecision struct {
	// ClusterName identifies the target cluster for placement
	ClusterName string

	// Score indicates placement preference (higher values preferred)
	// Range: 0-100, where 100 represents optimal placement
	Score int

	// Reason provides human-readable explanation for the placement decision
	Reason string
}