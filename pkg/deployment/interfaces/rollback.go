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
)

// RollbackController manages deployment rollbacks across KCP logical clusters.
// All rollback operations must maintain workspace isolation and operate within
// the appropriate logical cluster boundaries. Implementations must be thread-safe
// to support concurrent rollback operations across multiple deployments.
//
// Security Considerations:
//   - Snapshot data should be encrypted at rest when stored externally
//   - Access to rollback operations must be authorized per logical cluster
//   - Rollback history must respect workspace boundaries
//
// Example usage:
//   controller := NewRollbackController(kcpClient)
//   canRollback, reason, err := controller.CanRollback(ctx, deploymentID)
//   if canRollback {
//     op, err := controller.InitiateRollback(ctx, deploymentID, "performance degradation")
//   }
type RollbackController interface {
	// CanRollback checks if rollback is possible within the deployment's logical cluster
	CanRollback(ctx context.Context, deploymentID string) (bool, string, error)

	// CanRollbackInCluster checks rollback possibility in a specific logical cluster
	CanRollbackInCluster(ctx context.Context, cluster core.LogicalCluster, deploymentID string) (bool, string, error)

	// InitiateRollback starts the rollback process within the deployment's logical cluster
	InitiateRollback(ctx context.Context, deploymentID string,
		reason string) (*RollbackOperation, error)

	// GetRollbackStatus returns rollback progress (must be thread-safe)
	GetRollbackStatus(ctx context.Context, rollbackID string) (*RollbackStatus, error)

	// ListSnapshots returns available snapshots within the deployment's logical cluster
	ListSnapshots(ctx context.Context, deploymentID string) ([]Snapshot, error)

	// CreateSnapshot creates a new deployment state snapshot
	CreateSnapshot(ctx context.Context, deploymentID string, description string) (*Snapshot, error)
}

// RollbackOperation represents an active rollback
type RollbackOperation struct {
	ID           string      `json:"id"`
	DeploymentID string      `json:"deploymentId"`
	Reason       string      `json:"reason"`
	StartTime    metav1.Time `json:"startTime"`
	TargetState  string      `json:"targetState"`
}

// RollbackStatus contains rollback progress
type RollbackStatus struct {
	OperationID string             `json:"operationId"`
	Status      RollbackStatusType `json:"status"`
	Progress    int32              `json:"progress"`
	Message     string             `json:"message,omitempty"`
}

// RollbackStatusType defines rollback states
type RollbackStatusType string

const (
	RollbackPending    RollbackStatusType = "Pending"
	RollbackInProgress RollbackStatusType = "InProgress"
	RollbackSucceeded  RollbackStatusType = "Succeeded"
	RollbackFailed     RollbackStatusType = "Failed"
)

// Snapshot represents a deployment state snapshot
type Snapshot struct {
	ID        string      `json:"id"`
	Timestamp metav1.Time `json:"timestamp"`
	State     []byte      `json:"state"`
	Version   string      `json:"version"`
}