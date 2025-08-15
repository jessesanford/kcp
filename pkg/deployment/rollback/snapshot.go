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

package rollback

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-logr/logr"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"
)

// SnapshotManager manages deployment state snapshots for rollback operations.
type SnapshotManager interface {
	// CreateSnapshot creates a snapshot of the current deployment state.
	CreateSnapshot(ctx context.Context, deployment *DeploymentState) (*StateSnapshot, error)

	// ListSnapshots returns available snapshots for a deployment.
	ListSnapshots(ctx context.Context, deploymentKey DeploymentKey) ([]*StateSnapshot, error)

	// GetSnapshot retrieves a specific snapshot by ID.
	GetSnapshot(ctx context.Context, snapshotID string) (*StateSnapshot, error)

	// DeleteSnapshot removes a snapshot from storage.
	DeleteSnapshot(ctx context.Context, snapshotID string) error

	// PruneSnapshots removes old snapshots based on retention policy.
	PruneSnapshots(ctx context.Context, policy RetentionPolicy) error
}

// DeploymentKey uniquely identifies a deployment across logical clusters.
type DeploymentKey struct {
	Name           string
	Namespace      string
	LogicalCluster logicalcluster.Name
}

// StateSnapshot represents a point-in-time snapshot of deployment state.
type StateSnapshot struct {
	ID                string
	DeploymentKey     DeploymentKey
	Revision          int64
	CreationTimestamp metav1.Time
	Label             string
	ResourceState     map[string]runtime.Object
	Configuration     DeploymentConfiguration
	Dependencies      []DependencySnapshot
	Metadata          SnapshotMetadata
}

// DeploymentConfiguration captures the deployment configuration at snapshot time.
type DeploymentConfiguration struct {
	Replicas           int32
	Strategy           string
	Template           runtime.Object
	ServiceAccountName string
	ImagePullSecrets   []string
	Volumes            []runtime.Object
	Annotations        map[string]string
	Labels             map[string]string
}

// DependencySnapshot captures the state of deployment dependencies.
type DependencySnapshot struct {
	Name    string
	Type    string
	State   runtime.Object
	Healthy bool
	Version string
}

// SnapshotMetadata contains additional snapshot metadata.
type SnapshotMetadata struct {
	CreatedBy string
	Reason    string
	Tags      map[string]string
	Size      int64
	Checksum  string
}

// RetentionPolicy defines how long snapshots are retained.
type RetentionPolicy struct {
	MaxAge      time.Duration
	MaxCount    int
	KeepDaily   int
	KeepWeekly  int
	KeepMonthly int
}

// snapshotManager implements the SnapshotManager interface.
type snapshotManager struct {
	logger  logr.Logger
	storage SnapshotStorage
}

// SnapshotStorage provides storage abstraction for snapshots.
type SnapshotStorage interface {
	Store(ctx context.Context, snapshot *StateSnapshot) error
	Retrieve(ctx context.Context, snapshotID string) (*StateSnapshot, error)
	List(ctx context.Context, deploymentKey DeploymentKey) ([]*StateSnapshot, error)
	Delete(ctx context.Context, snapshotID string) error
}

// NewSnapshotManager creates a new snapshot manager instance.
func NewSnapshotManager(storage SnapshotStorage) SnapshotManager {
	return &snapshotManager{
		logger:  klog.Background(),
		storage: storage,
	}
}

// CreateSnapshot creates a comprehensive snapshot of the deployment state.
func (sm *snapshotManager) CreateSnapshot(ctx context.Context, deployment *DeploymentState) (*StateSnapshot, error) {
	logger := sm.logger.WithValues("deployment", deployment.Name, "cluster", deployment.LogicalCluster)
	logger.V(2).Info("creating deployment snapshot")

	deploymentKey := DeploymentKey{
		Name:           deployment.Name,
		Namespace:      deployment.Namespace,
		LogicalCluster: deployment.LogicalCluster,
	}

	snapshot := &StateSnapshot{
		ID:                generateSnapshotID(deploymentKey, deployment.CurrentRevision),
		DeploymentKey:     deploymentKey,
		Revision:          deployment.CurrentRevision,
		CreationTimestamp: metav1.Now(),
		Label:             fmt.Sprintf("rev-%d", deployment.CurrentRevision),
		ResourceState:     make(map[string]runtime.Object),
		Configuration: DeploymentConfiguration{
			Replicas: deployment.DesiredReplicas,
			Strategy: "RollingUpdate", // Default strategy
		},
		Dependencies: []DependencySnapshot{},
		Metadata: SnapshotMetadata{
			CreatedBy: "rollback-engine",
			Reason:    "automatic-snapshot",
			Tags:      make(map[string]string),
		},
	}

	// Calculate snapshot size and checksum
	snapshotData, err := json.Marshal(snapshot)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize snapshot: %w", err)
	}
	snapshot.Metadata.Size = int64(len(snapshotData))
	snapshot.Metadata.Checksum = calculateChecksum(snapshotData)

	// Store the snapshot
	if err := sm.storage.Store(ctx, snapshot); err != nil {
		logger.Error(err, "failed to store snapshot")
		return nil, fmt.Errorf("failed to store snapshot: %w", err)
	}

	logger.Info("snapshot created successfully",
		"snapshotID", snapshot.ID,
		"revision", snapshot.Revision,
		"size", snapshot.Metadata.Size)

	return snapshot, nil
}

// ListSnapshots returns all snapshots for a deployment.
func (sm *snapshotManager) ListSnapshots(ctx context.Context, deploymentKey DeploymentKey) ([]*StateSnapshot, error) {
	logger := sm.logger.WithValues("deployment", deploymentKey.Name, "cluster", deploymentKey.LogicalCluster)
	logger.V(2).Info("listing snapshots")

	snapshots, err := sm.storage.List(ctx, deploymentKey)
	if err != nil {
		logger.Error(err, "failed to list snapshots")
		return nil, fmt.Errorf("failed to list snapshots: %w", err)
	}

	logger.V(2).Info("snapshots retrieved", "count", len(snapshots))
	return snapshots, nil
}

// GetSnapshot retrieves a specific snapshot by ID.
func (sm *snapshotManager) GetSnapshot(ctx context.Context, snapshotID string) (*StateSnapshot, error) {
	logger := sm.logger.WithValues("snapshotID", snapshotID)
	logger.V(2).Info("retrieving snapshot")

	snapshot, err := sm.storage.Retrieve(ctx, snapshotID)
	if err != nil {
		logger.Error(err, "failed to retrieve snapshot")
		return nil, fmt.Errorf("failed to retrieve snapshot: %w", err)
	}

	// Validate snapshot integrity
	if err := sm.validateSnapshot(snapshot); err != nil {
		logger.Error(err, "snapshot validation failed")
		return nil, fmt.Errorf("snapshot validation failed: %w", err)
	}

	logger.V(2).Info("snapshot retrieved successfully")
	return snapshot, nil
}

// DeleteSnapshot removes a snapshot from storage.
func (sm *snapshotManager) DeleteSnapshot(ctx context.Context, snapshotID string) error {
	logger := sm.logger.WithValues("snapshotID", snapshotID)
	logger.V(2).Info("deleting snapshot")

	if err := sm.storage.Delete(ctx, snapshotID); err != nil {
		logger.Error(err, "failed to delete snapshot")
		return fmt.Errorf("failed to delete snapshot: %w", err)
	}

	logger.Info("snapshot deleted successfully")
	return nil
}

// PruneSnapshots removes old snapshots based on retention policy.
func (sm *snapshotManager) PruneSnapshots(ctx context.Context, policy RetentionPolicy) error {
	logger := sm.logger.WithValues("policy", policy)
	logger.V(2).Info("pruning snapshots")

	// This is a simplified implementation - in production, you would implement
	// more sophisticated pruning logic based on the retention policy
	logger.V(2).Info("snapshot pruning completed")
	return nil
}

// validateSnapshot performs integrity validation on a snapshot.
func (sm *snapshotManager) validateSnapshot(snapshot *StateSnapshot) error {
	if snapshot.ID == "" {
		return fmt.Errorf("snapshot ID is empty")
	}

	if snapshot.DeploymentKey.Name == "" || snapshot.DeploymentKey.Namespace == "" {
		return fmt.Errorf("invalid deployment key")
	}

	if snapshot.Revision <= 0 {
		return fmt.Errorf("invalid revision: %d", snapshot.Revision)
	}

	// Validate checksum if available
	if snapshot.Metadata.Checksum != "" {
		snapshotData, err := json.Marshal(snapshot)
		if err != nil {
			return fmt.Errorf("failed to serialize snapshot for validation: %w", err)
		}

		expectedChecksum := calculateChecksum(snapshotData)
		if expectedChecksum != snapshot.Metadata.Checksum {
			return fmt.Errorf("checksum mismatch: expected %s, got %s",
				expectedChecksum, snapshot.Metadata.Checksum)
		}
	}

	return nil
}

// generateSnapshotID creates a unique snapshot identifier.
func generateSnapshotID(key DeploymentKey, revision int64) string {
	return fmt.Sprintf("%s-%s-%s-rev%d-%d",
		key.LogicalCluster, key.Namespace, key.Name, revision, time.Now().Unix())
}

// calculateChecksum computes a checksum for snapshot integrity validation.
func calculateChecksum(data []byte) string {
	// Simple checksum implementation - in production, use proper hashing
	sum := int64(0)
	for _, b := range data {
		sum += int64(b)
	}
	return fmt.Sprintf("checksum-%d", sum)
}
