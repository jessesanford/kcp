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
)

// RollbackController manages deployment rollbacks
type RollbackController interface {
	// CanRollback checks if rollback is possible
	CanRollback(ctx context.Context, deploymentID string) (bool, string, error)

	// InitiateRollback starts the rollback process
	InitiateRollback(ctx context.Context, deploymentID string,
		reason string) (*RollbackOperation, error)

	// GetRollbackStatus returns rollback progress
	GetRollbackStatus(ctx context.Context, rollbackID string) (*RollbackStatus, error)

	// ListSnapshots returns available snapshots
	ListSnapshots(ctx context.Context, deploymentID string) ([]Snapshot, error)
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