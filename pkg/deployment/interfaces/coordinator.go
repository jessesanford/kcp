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

	"github.com/kcp-dev/kcp/pkg/deployment/types"
)

// DeploymentCoordinator orchestrates deployment execution
type DeploymentCoordinator interface {
	// Plan creates a deployment plan from strategy
	Plan(ctx context.Context, strategy types.DeploymentStrategy,
		target DeploymentTarget) (*types.DeploymentPlan, error)

	// Execute runs the deployment plan
	Execute(ctx context.Context, plan *types.DeploymentPlan) (*DeploymentResult, error)

	// Rollback reverses a deployment
	Rollback(ctx context.Context, deploymentID string) error

	// GetStatus returns current deployment status
	GetStatus(ctx context.Context, deploymentID string) (*DeploymentStatus, error)

	// Pause halts a deployment
	Pause(ctx context.Context, deploymentID string) error

	// Resume continues a paused deployment
	Resume(ctx context.Context, deploymentID string) error
}

// DeploymentTarget identifies what to deploy
type DeploymentTarget struct {
	Name       string            `json:"name"`
	Namespace  string            `json:"namespace"`
	Workspace  string            `json:"workspace"`
	APIVersion string            `json:"apiVersion"`
	Kind       string            `json:"kind"`
	Labels     map[string]string `json:"labels,omitempty"`
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