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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kcp-dev/kcp/pkg/apis/core"
	"github.com/kcp-dev/kcp/pkg/deployment/types"
)

// DeploymentCoordinator orchestrates deployment execution across logical clusters.
// Implementations must be cluster-aware and maintain workspace isolation.
//
// Example usage:
//   coordinator := NewCoordinator(kcpClient)
//   plan, err := coordinator.Plan(ctx, strategy, target)
//   if err != nil { return err }
//   result, err := coordinator.Execute(ctx, plan)
//
// All operations respect KCP's logical cluster boundaries and workspace permissions.
type DeploymentCoordinator interface {
	// Plan creates a deployment plan from strategy for a specific logical cluster
	Plan(ctx context.Context, strategy types.DeploymentStrategy,
		target DeploymentTarget) (*types.DeploymentPlan, error)

	// PlanForCluster creates a deployment plan for a specific logical cluster
	PlanForCluster(ctx context.Context, cluster core.LogicalCluster, 
		strategy types.DeploymentStrategy, target DeploymentTarget) (*types.DeploymentPlan, error)

	// Execute runs the deployment plan within the target's logical cluster context
	Execute(ctx context.Context, plan *types.DeploymentPlan) (*DeploymentResult, error)

	// Rollback reverses a deployment within its original logical cluster
	Rollback(ctx context.Context, deploymentID string) error

	// GetStatus returns current deployment status from the appropriate logical cluster
	GetStatus(ctx context.Context, deploymentID string) (*DeploymentStatus, error)

	// Pause halts a deployment in its logical cluster
	Pause(ctx context.Context, deploymentID string) error

	// Resume continues a paused deployment in its logical cluster
	Resume(ctx context.Context, deploymentID string) error
}

// DeploymentTarget identifies what to deploy within KCP's logical cluster architecture.
// It includes both workspace-level and logical cluster-level targeting for precise
// resource identification across KCP's multi-tenant environment.
type DeploymentTarget struct {
	Name           string                `json:"name"`
	Namespace      string                `json:"namespace"`
	Workspace      string                `json:"workspace"`
	LogicalCluster core.LogicalCluster   `json:"logicalCluster,omitempty"`
	APIVersion     string                `json:"apiVersion"`
	Kind           string                `json:"kind"`
	Labels         map[string]string     `json:"labels,omitempty"`
}

// DeploymentResult contains execution outcome
type DeploymentResult struct {
	DeploymentID string               `json:"deploymentId"`
	Status       DeploymentStatusType `json:"status"`
	Message      string               `json:"message,omitempty"`
	StartTime    metav1.Time          `json:"startTime"`
	EndTime      *metav1.Time         `json:"endTime,omitempty"`
	Phases       []PhaseResult        `json:"phases"`
}

// DeploymentStatus represents current state
type DeploymentStatus struct {
	DeploymentID string               `json:"deploymentId"`
	Phase        string               `json:"phase"`
	Status       DeploymentStatusType `json:"status"`
	Progress     int32                `json:"progress"`
	Message      string               `json:"message,omitempty"`
}

// DeploymentStatusType defines deployment states
type DeploymentStatusType string

const (
	DeploymentPending     DeploymentStatusType = "Pending"
	DeploymentInProgress  DeploymentStatusType = "InProgress"
	DeploymentSucceeded   DeploymentStatusType = "Succeeded"
	DeploymentFailed      DeploymentStatusType = "Failed"
	DeploymentPaused      DeploymentStatusType = "Paused"
	DeploymentRollingBack DeploymentStatusType = "RollingBack"
)

// PhaseResult contains phase execution details
type PhaseResult struct {
	Name      string               `json:"name"`
	Status    DeploymentStatusType `json:"status"`
	StartTime metav1.Time          `json:"startTime"`
	EndTime   *metav1.Time         `json:"endTime,omitempty"`
	Error     string               `json:"error,omitempty"`
}