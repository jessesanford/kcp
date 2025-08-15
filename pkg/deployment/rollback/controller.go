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
	"fmt"
	"time"

	"github.com/go-logr/logr"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

// RollbackController orchestrates rollback operations for failed deployments.
type RollbackController interface {
	// Start begins the controller's main processing loop.
	Start(ctx context.Context, workers int) error

	// TriggerRollback manually triggers a rollback for a deployment.
	TriggerRollback(ctx context.Context, deploymentKey DeploymentKey, targetSnapshot string) error

	// GetRollbackStatus returns the current status of rollback operations.
	GetRollbackStatus(ctx context.Context, deploymentKey DeploymentKey) (*RollbackStatus, error)
}

// RollbackStatus represents the current state of rollback operations.
type RollbackStatus struct {
	DeploymentKey    DeploymentKey
	State            RollbackState
	CurrentOperation string
	Progress         int32
	StartTime        metav1.Time
	CompletionTime   *metav1.Time
	ErrorMessage     string
	TargetSnapshot   string
	Steps            []RollbackStep
}

// RollbackState represents the state of a rollback operation.
type RollbackState string

const (
	RollbackStateIdle       RollbackState = "Idle"
	RollbackStatePending    RollbackState = "Pending"
	RollbackStateInProgress RollbackState = "InProgress"
	RollbackStateCompleted  RollbackState = "Completed"
	RollbackStateFailed     RollbackState = "Failed"
)

// RollbackStep represents a step in the rollback process.
type RollbackStep struct {
	Name        string
	Description string
	Status      string
	StartTime   metav1.Time
	EndTime     *metav1.Time
	Error       string
}

// controller implements the RollbackController interface.
type controller struct {
	logger           logr.Logger
	detector         FailureDetector
	snapshotManager  SnapshotManager
	recoveryManager  RecoveryManager
	workqueue        workqueue.TypedRateLimitingInterface[string]
	rollbackStatuses map[string]*RollbackStatus
}

// NewRollbackController creates a new rollback controller instance.
func NewRollbackController(
	detector FailureDetector,
	snapshotManager SnapshotManager,
	recoveryManager RecoveryManager,
) RollbackController {
	return &controller{
		logger:          klog.Background(),
		detector:        detector,
		snapshotManager: snapshotManager,
		recoveryManager: recoveryManager,
		workqueue: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[string](),
			workqueue.TypedRateLimitingQueueConfig[string]{Name: "rollback"},
		),
		rollbackStatuses: make(map[string]*RollbackStatus),
	}
}

// Start begins the controller's main processing loop.
func (c *controller) Start(ctx context.Context, workers int) error {
	defer c.workqueue.ShutDown()

	c.logger.Info("starting rollback controller", "workers", workers)

	// Start worker goroutines
	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, c.runWorker, time.Second)
	}

	<-ctx.Done()
	c.logger.Info("shutting down rollback controller")

	return nil
}

// TriggerRollback manually triggers a rollback operation.
func (c *controller) TriggerRollback(ctx context.Context, deploymentKey DeploymentKey, targetSnapshot string) error {
	logger := c.logger.WithValues("deployment", deploymentKey.Name, "cluster", deploymentKey.LogicalCluster)
	logger.Info("triggering manual rollback", "targetSnapshot", targetSnapshot)

	// Create rollback status
	status := &RollbackStatus{
		DeploymentKey:    deploymentKey,
		State:            RollbackStatePending,
		CurrentOperation: "initializing",
		Progress:         0,
		StartTime:        metav1.Now(),
		TargetSnapshot:   targetSnapshot,
		Steps:            []RollbackStep{},
	}

	key := c.generateStatusKey(deploymentKey)
	c.rollbackStatuses[key] = status

	// Add to work queue
	c.workqueue.Add(key)

	logger.Info("rollback triggered successfully")
	return nil
}

// GetRollbackStatus returns the current rollback status.
func (c *controller) GetRollbackStatus(ctx context.Context, deploymentKey DeploymentKey) (*RollbackStatus, error) {
	key := c.generateStatusKey(deploymentKey)
	if status, exists := c.rollbackStatuses[key]; exists {
		return status, nil
	}

	return &RollbackStatus{
		DeploymentKey: deploymentKey,
		State:         RollbackStateIdle,
	}, nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the workqueue.
func (c *controller) runWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *controller) processNextWorkItem(ctx context.Context) bool {
	key, shutdown := c.workqueue.Get()
	if shutdown {
		return false
	}

	defer c.workqueue.Done(key)

	if err := c.syncRollback(ctx, key); err != nil {
		c.workqueue.AddRateLimited(key)
		c.logger.Error(err, "error syncing rollback", "key", key)
		return true
	}

	c.workqueue.Forget(key)
	return true
}

// syncRollback processes a rollback operation.
func (c *controller) syncRollback(ctx context.Context, key string) error {
	logger := c.logger.WithValues("key", key)
	logger.V(2).Info("syncing rollback")

	status, exists := c.rollbackStatuses[key]
	if !exists {
		logger.V(2).Info("rollback status not found")
		return nil
	}

	// Update status to in progress
	status.State = RollbackStateInProgress
	status.CurrentOperation = "executing rollback"

	// Execute rollback steps
	if err := c.executeRollback(ctx, status); err != nil {
		status.State = RollbackStateFailed
		status.ErrorMessage = err.Error()
		logger.Error(err, "rollback failed")
		return err
	}

	// Mark as completed
	status.State = RollbackStateCompleted
	status.Progress = 100
	now := metav1.Now()
	status.CompletionTime = &now

	logger.Info("rollback completed successfully")
	return nil
}

// executeRollback executes the actual rollback steps.
func (c *controller) executeRollback(ctx context.Context, status *RollbackStatus) error {
	logger := c.logger.WithValues("deployment", status.DeploymentKey.Name)

	// Step 1: Retrieve target snapshot
	snapshot, err := c.snapshotManager.GetSnapshot(ctx, status.TargetSnapshot)
	if err != nil {
		return fmt.Errorf("failed to retrieve target snapshot: %w", err)
	}

	// Step 2: Execute recovery
	if err := c.recoveryManager.RestoreFromSnapshot(ctx, snapshot); err != nil {
		return fmt.Errorf("failed to restore from snapshot: %w", err)
	}

	logger.V(2).Info("rollback executed successfully")
	return nil
}

// generateStatusKey generates a unique key for rollback status tracking.
func (c *controller) generateStatusKey(deploymentKey DeploymentKey) string {
	return fmt.Sprintf("%s/%s/%s", deploymentKey.LogicalCluster, deploymentKey.Namespace, deploymentKey.Name)
}
