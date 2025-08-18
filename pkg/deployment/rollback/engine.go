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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"
)

// Engine coordinates rollback operations across all components.
type Engine struct {
	dynamicClient dynamic.Interface
	cluster       logicalcluster.Name
	config        *EngineConfig

	// Component managers
	snapshotManager    *SnapshotManager
	restorationManager *RestorationManager
	triggerManager     *TriggerManager
	historyManager     *HistoryManager

	// State management
	activeRollbacks map[string]*RollbackExecution
	
	// Shutdown channel
	stopCh chan struct{}
}

// RollbackExecution tracks an active rollback operation.
type RollbackExecution struct {
	Request     *RollbackRequest
	OperationID string
	StartTime   time.Time
	Phase       RollbackPhase
	Context     context.Context
	Cancel      context.CancelFunc
}

// NewEngine creates a new rollback engine.
func NewEngine(client dynamic.Interface, cluster logicalcluster.Name, config *EngineConfig) (*Engine, error) {
	if config == nil {
		config = &EngineConfig{
			MaxSnapshots:            10,
			EnableAutomaticTriggers: false,
		}
	}

	engine := &Engine{
		dynamicClient:   client,
		cluster:         cluster,
		config:          config,
		activeRollbacks: make(map[string]*RollbackExecution),
		stopCh:          make(chan struct{}),
	}

	// Initialize component managers
	engine.snapshotManager = NewSnapshotManager(client, cluster, config)
	engine.restorationManager = NewRestorationManager(client, cluster, config)
	engine.triggerManager = NewTriggerManager(client, cluster, config)
	engine.historyManager = NewHistoryManager(client, cluster, config)

	klog.InfoS("Rollback engine created", "cluster", cluster, "config", config)
	return engine, nil
}

// Start initializes and starts the rollback engine.
func (e *Engine) Start(ctx context.Context) error {
	klog.InfoS("Starting rollback engine")

	// Start trigger manager if enabled
	if e.config.EnableAutomaticTriggers {
		if err := e.triggerManager.Start(ctx); err != nil {
			return fmt.Errorf("failed to start trigger manager: %w", err)
		}

		// Start monitoring trigger events
		go e.monitorTriggerEvents(ctx)
	}

	// Start background maintenance tasks
	go e.runMaintenanceTasks(ctx)

	klog.InfoS("Rollback engine started successfully")
	return nil
}

// Stop gracefully shuts down the rollback engine.
func (e *Engine) Stop() {
	klog.InfoS("Stopping rollback engine")

	// Signal shutdown
	close(e.stopCh)

	// Stop trigger manager
	e.triggerManager.Stop()

	// Cancel all active rollbacks
	for id, execution := range e.activeRollbacks {
		klog.InfoS("Canceling active rollback", "rollbackID", id)
		execution.Cancel()
	}

	klog.InfoS("Rollback engine stopped")
}

// CreateSnapshot captures the current state of a deployment.
func (e *Engine) CreateSnapshot(ctx context.Context, deploymentRef corev1.ObjectReference, version string) (*DeploymentSnapshot, error) {
	klog.InfoS("Creating deployment snapshot", "deployment", deploymentRef.Name, "version", version)

	// Record operation start
	operationID, err := e.historyManager.StartOperation(ctx, deploymentRef, OperationTypeSnapshot, "Manual snapshot creation", "rollback-engine")
	if err != nil {
		return nil, fmt.Errorf("failed to start operation tracking: %w", err)
	}

	// Create snapshot
	snapshot, err := e.snapshotManager.CreateSnapshot(ctx, deploymentRef, version)
	if err != nil {
		// Record failure
		e.historyManager.CompleteOperation(ctx, deploymentRef, operationID, false, err.Error(), "", "")
		return nil, fmt.Errorf("failed to create snapshot: %w", err)
	}

	// Record success
	if err := e.historyManager.CompleteOperation(ctx, deploymentRef, operationID, true, "", "", snapshot.ID); err != nil {
		klog.ErrorS(err, "Failed to record successful snapshot operation", "deployment", deploymentRef.Name)
	}

	// Cleanup old snapshots
	if err := e.snapshotManager.CleanupExpiredSnapshots(ctx, deploymentRef); err != nil {
		klog.ErrorS(err, "Failed to cleanup expired snapshots", "deployment", deploymentRef.Name)
	}

	klog.InfoS("Successfully created deployment snapshot", "deployment", deploymentRef.Name, "snapshotID", snapshot.ID)
	return snapshot, nil
}

// ExecuteRollback performs a rollback operation.
func (e *Engine) ExecuteRollback(ctx context.Context, request *RollbackRequest) (*RollbackStatus, error) {
	klog.InfoS("Executing rollback", "deployment", request.Spec.TargetRef.Name, "snapshotID", request.Spec.RollbackTo.SnapshotID)

	// Validate request
	if err := e.validateRollbackRequest(request); err != nil {
		return nil, fmt.Errorf("invalid rollback request: %w", err)
	}

	// Check for existing active rollback
	if e.hasActiveRollback(request.Spec.TargetRef) {
		return nil, fmt.Errorf("rollback already in progress for deployment %s", request.Spec.TargetRef.Name)
	}

	// Initialize rollback execution
	execution := e.initializeExecution(ctx, request)
	e.activeRollbacks[execution.OperationID] = execution

	// Execute rollback in background
	go e.executeRollbackAsync(execution)

	// Return initial status
	return &RollbackStatus{
		Phase:     RollbackPhasePending,
		StartTime: &metav1.Time{Time: execution.StartTime},
		Message:   "Rollback initiated",
	}, nil
}

// GetRollbackStatus returns the current status of a rollback operation.
func (e *Engine) GetRollbackStatus(operationID string) (*RollbackStatus, error) {
	execution, exists := e.activeRollbacks[operationID]
	if !exists {
		return nil, fmt.Errorf("rollback operation %s not found", operationID)
	}

	return &execution.Request.Status, nil
}

// ListSnapshots returns available snapshots for a deployment.
func (e *Engine) ListSnapshots(ctx context.Context, deploymentRef corev1.ObjectReference) ([]*DeploymentSnapshot, error) {
	return e.snapshotManager.ListSnapshots(ctx, deploymentRef)
}

// GetSnapshot retrieves a specific snapshot.
func (e *Engine) GetSnapshot(ctx context.Context, snapshotID string) (*DeploymentSnapshot, error) {
	return e.snapshotManager.GetSnapshot(ctx, snapshotID)
}

// GetRollbackHistory returns the rollback history for a deployment.
func (e *Engine) GetRollbackHistory(ctx context.Context, deploymentRef corev1.ObjectReference) (*RollbackHistory, error) {
	return e.historyManager.GetHistory(ctx, deploymentRef)
}

// RegisterTrigger adds a new automatic rollback trigger.
func (e *Engine) RegisterTrigger(trigger *RollbackTrigger) error {
	return e.triggerManager.RegisterTrigger(trigger)
}

// Private methods

// validateRollbackRequest validates a rollback request.
func (e *Engine) validateRollbackRequest(request *RollbackRequest) error {
	if request.Spec.TargetRef.Name == "" {
		return fmt.Errorf("target deployment name cannot be empty")
	}

	if request.Spec.RollbackTo.SnapshotID == "" {
		return fmt.Errorf("snapshot ID cannot be empty")
	}

	// Validate timeout
	if request.Spec.TimeoutSeconds != nil && *request.Spec.TimeoutSeconds <= 0 {
		return fmt.Errorf("timeout must be positive")
	}

	return nil
}

// hasActiveRollback checks if there's an active rollback for a deployment.
func (e *Engine) hasActiveRollback(deploymentRef corev1.ObjectReference) bool {
	for _, execution := range e.activeRollbacks {
		if execution.Request.Spec.TargetRef.Name == deploymentRef.Name &&
			execution.Request.Spec.TargetRef.Namespace == deploymentRef.Namespace {
			return true
		}
	}
	return false
}

// initializeExecution creates a new rollback execution context.
func (e *Engine) initializeExecution(ctx context.Context, request *RollbackRequest) *RollbackExecution {
	operationID := fmt.Sprintf("rollback-%d", time.Now().Unix())
	
	// Set timeout
	timeout := 10 * time.Minute // default
	if e.config.DefaultTimeout != nil {
		timeout = e.config.DefaultTimeout.Duration
	}
	if request.Spec.TimeoutSeconds != nil {
		timeout = time.Duration(*request.Spec.TimeoutSeconds) * time.Second
	}

	execCtx, cancel := context.WithTimeout(ctx, timeout)

	return &RollbackExecution{
		Request:     request,
		OperationID: operationID,
		StartTime:   time.Now(),
		Phase:       RollbackPhasePending,
		Context:     execCtx,
		Cancel:      cancel,
	}
}

// executeRollbackAsync performs the actual rollback operation.
func (e *Engine) executeRollbackAsync(execution *RollbackExecution) {
	defer func() {
		execution.Cancel()
		delete(e.activeRollbacks, execution.OperationID)
	}()

	request := execution.Request
	ctx := execution.Context

	// Start operation tracking
	operationID, err := e.historyManager.StartOperation(ctx, request.Spec.TargetRef, OperationTypeRollback, request.Spec.Reason, "rollback-engine")
	if err != nil {
		klog.ErrorS(err, "Failed to start operation tracking")
		e.updateExecutionStatus(execution, RollbackPhaseFailed, fmt.Sprintf("Failed to start operation tracking: %v", err))
		return
	}

	// Phase 1: Validate snapshot
	e.updateExecutionStatus(execution, RollbackPhaseValidating, "Validating snapshot")
	snapshot, err := e.snapshotManager.GetSnapshot(ctx, request.Spec.RollbackTo.SnapshotID)
	if err != nil {
		e.historyManager.CompleteOperation(ctx, request.Spec.TargetRef, operationID, false, err.Error(), "", request.Spec.RollbackTo.SnapshotID)
		e.updateExecutionStatus(execution, RollbackPhaseFailed, fmt.Sprintf("Failed to get snapshot: %v", err))
		return
	}

	if err := e.snapshotManager.ValidateSnapshot(ctx, snapshot); err != nil {
		e.historyManager.CompleteOperation(ctx, request.Spec.TargetRef, operationID, false, err.Error(), "", request.Spec.RollbackTo.SnapshotID)
		e.updateExecutionStatus(execution, RollbackPhaseFailed, fmt.Sprintf("Snapshot validation failed: %v", err))
		return
	}

	// Phase 2: Restore resources
	e.updateExecutionStatus(execution, RollbackPhaseRestoring, "Restoring resources from snapshot")
	restoredResources, err := e.restorationManager.RestoreFromSnapshot(ctx, snapshot, request.Spec.DryRun)
	if err != nil {
		e.historyManager.CompleteOperation(ctx, request.Spec.TargetRef, operationID, false, err.Error(), "", request.Spec.RollbackTo.SnapshotID)
		e.updateExecutionStatus(execution, RollbackPhaseFailed, fmt.Sprintf("Resource restoration failed: %v", err))
		return
	}

	// Update status with restored resources
	request.Status.RestoredResources = restoredResources

	// Phase 3: Validate restoration (if not dry run)
	if !request.Spec.DryRun {
		if err := e.restorationManager.ValidateRestoration(ctx, restoredResources); err != nil {
			e.historyManager.CompleteOperation(ctx, request.Spec.TargetRef, operationID, false, err.Error(), "", request.Spec.RollbackTo.SnapshotID)
			e.updateExecutionStatus(execution, RollbackPhaseFailed, fmt.Sprintf("Restoration validation failed: %v", err))
			return
		}
	}

	// Complete successfully
	e.historyManager.CompleteOperation(ctx, request.Spec.TargetRef, operationID, true, "", "", request.Spec.RollbackTo.SnapshotID)
	
	message := "Rollback completed successfully"
	if request.Spec.DryRun {
		message = "Dry run rollback completed successfully"
	}
	
	e.updateExecutionStatus(execution, RollbackPhaseCompleted, message)
	
	klog.InfoS("Rollback completed successfully", "deployment", request.Spec.TargetRef.Name, "snapshotID", request.Spec.RollbackTo.SnapshotID)
}

// updateExecutionStatus updates the status of a rollback execution.
func (e *Engine) updateExecutionStatus(execution *RollbackExecution, phase RollbackPhase, message string) {
	execution.Phase = phase
	execution.Request.Status.Phase = phase
	execution.Request.Status.Message = message

	if phase == RollbackPhaseCompleted || phase == RollbackPhaseFailed {
		now := metav1.NewTime(time.Now())
		execution.Request.Status.CompletionTime = &now
	}

	klog.V(2).InfoS("Updated rollback status", "operationID", execution.OperationID, "phase", phase, "message", message)
}

// monitorTriggerEvents monitors for automatic trigger events.
func (e *Engine) monitorTriggerEvents(ctx context.Context) {
	triggerEvents := e.triggerManager.GetTriggerEvents()

	for {
		select {
		case <-ctx.Done():
			return
		case <-e.stopCh:
			return
		case event := <-triggerEvents:
			e.handleTriggerEvent(ctx, event)
		}
	}
}

// handleTriggerEvent handles an automatic trigger event.
func (e *Engine) handleTriggerEvent(ctx context.Context, event TriggerEvent) {
	klog.InfoS("Handling automatic trigger event", "trigger", event.TriggerName, "deployment", event.DeploymentRef.Name)

	// Get the latest snapshot for rollback
	snapshots, err := e.snapshotManager.ListSnapshots(ctx, event.DeploymentRef)
	if err != nil || len(snapshots) == 0 {
		klog.ErrorS(err, "No snapshots available for automatic rollback", "deployment", event.DeploymentRef.Name)
		return
	}

	// Use the most recent snapshot (excluding any recent ones that might be the problematic deployment)
	var targetSnapshot *DeploymentSnapshot
	for _, snapshot := range snapshots {
		// Skip very recent snapshots (within last 5 minutes) as they might be the problematic version
		if time.Since(snapshot.CreatedAt.Time) > 5*time.Minute {
			targetSnapshot = snapshot
			break
		}
	}

	if targetSnapshot == nil {
		klog.ErrorS(nil, "No suitable snapshot found for automatic rollback", "deployment", event.DeploymentRef.Name)
		return
	}

	// Create rollback request
	rollbackRequest := &RollbackRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("auto-rollback-%d", time.Now().Unix()),
			Namespace: event.DeploymentRef.Namespace,
		},
		Spec: RollbackSpec{
			TargetRef:      event.DeploymentRef,
			RollbackTo:     RollbackTarget{SnapshotID: targetSnapshot.ID},
			Reason:         fmt.Sprintf("Automatic rollback triggered by %s: %s", event.TriggerName, event.Reason),
			AutoTriggered:  true,
			RestoreTraffic: true,
		},
	}

	// Execute automatic rollback
	_, err = e.ExecuteRollback(ctx, rollbackRequest)
	if err != nil {
		klog.ErrorS(err, "Failed to execute automatic rollback", "deployment", event.DeploymentRef.Name, "trigger", event.TriggerName)
	} else {
		klog.InfoS("Initiated automatic rollback", "deployment", event.DeploymentRef.Name, "trigger", event.TriggerName, "snapshotID", targetSnapshot.ID)
	}
}

// runMaintenanceTasks runs periodic maintenance tasks.
func (e *Engine) runMaintenanceTasks(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-e.stopCh:
			return
		case <-ticker.C:
			e.runMaintenance(ctx)
		}
	}
}

// runMaintenance performs maintenance tasks.
func (e *Engine) runMaintenance(ctx context.Context) {
	klog.V(2).InfoS("Running rollback engine maintenance")

	// Cleanup old history
	if err := e.historyManager.CleanupOldHistory(ctx); err != nil {
		klog.ErrorS(err, "Failed to cleanup old history")
	}

	klog.V(2).InfoS("Rollback engine maintenance completed")
}