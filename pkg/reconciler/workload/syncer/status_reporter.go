/*
Copyright 2022 The KCP Authors.

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

package syncer

import (
	"context"
	"fmt"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/kcp/cmd/workload-syncer/options"
	workloadv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/workload/v1alpha1"
	kcpclusterclient "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
	conditionsutil "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/util/conditions"
	"github.com/kcp-dev/logicalcluster/v3"
)

// StatusReporter manages SyncTarget status reporting and heartbeats
type StatusReporter struct {
	// Configuration
	options *options.SyncerOptions

	// Clients
	kcpClient kcpclusterclient.ClusterInterface

	// State management
	started         bool
	healthy         bool
	stopCh          chan struct{}
	mu              sync.RWMutex
	
	// Heartbeat tracking
	lastHeartbeat   time.Time
	heartbeatErrors int64
	statusUpdates   int64
}

// NewStatusReporter creates a new status reporter
func NewStatusReporter(ctx context.Context, kcpClient kcpclusterclient.ClusterInterface, opts *options.SyncerOptions) (*StatusReporter, error) {
	return &StatusReporter{
		options:   opts,
		kcpClient: kcpClient,
		stopCh:    make(chan struct{}),
		healthy:   true,
	}, nil
}

// Start starts the status reporter
func (sr *StatusReporter) Start(ctx context.Context) error {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	if sr.started {
		return fmt.Errorf("status reporter is already started")
	}

	klog.Info("Starting status reporter...")

	// Start heartbeat loop
	go sr.heartbeatLoop(ctx)

	sr.started = true
	klog.Info("Status reporter started successfully")
	return nil
}

// Stop stops the status reporter
func (sr *StatusReporter) Stop(ctx context.Context) error {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	if !sr.started {
		return nil
	}

	klog.Info("Stopping status reporter...")

	// Signal heartbeat loop to stop
	close(sr.stopCh)

	sr.started = false
	klog.Info("Status reporter stopped")
	return nil
}

// IsHealthy returns true if the status reporter is healthy
func (sr *StatusReporter) IsHealthy() bool {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	return sr.started && sr.healthy
}

// GetMetrics returns status reporter metrics
func (sr *StatusReporter) GetMetrics() map[string]interface{} {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	return map[string]interface{}{
		"started":           sr.started,
		"healthy":           sr.healthy,
		"last_heartbeat":    sr.lastHeartbeat,
		"heartbeat_errors":  sr.heartbeatErrors,
		"status_updates":    sr.statusUpdates,
	}
}

// heartbeatLoop runs the periodic heartbeat and status update loop
func (sr *StatusReporter) heartbeatLoop(ctx context.Context) {
	logger := klog.FromContext(ctx).WithValues("component", "status-reporter")
	logger.Info("Starting heartbeat loop")

	// Send initial status update
	if err := sr.updateSyncTargetStatus(ctx); err != nil {
		logger.Error(err, "Failed to send initial status update")
	}

	// Set up periodic heartbeat
	ticker := time.NewTicker(30 * time.Second) // 30 second heartbeat interval
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := sr.sendHeartbeat(ctx); err != nil {
				logger.Error(err, "Failed to send heartbeat")
				sr.handleHeartbeatError()
			} else {
				sr.handleHeartbeatSuccess()
			}
		case <-sr.stopCh:
			logger.Info("Heartbeat loop stopping")
			return
		case <-ctx.Done():
			logger.Info("Heartbeat loop context cancelled")
			return
		}
	}
}

// sendHeartbeat sends a heartbeat to KCP
func (sr *StatusReporter) sendHeartbeat(ctx context.Context) error {
	logger := klog.FromContext(ctx).WithValues("component", "status-reporter", "operation", "heartbeat")

	// Parse logical cluster from workspace
	clusterName := logicalcluster.Name(sr.options.SyncTargetWorkspace)
	workspaceClient := sr.kcpClient.Cluster(clusterName.Path()).WorkloadV1alpha1()
	
	// Get current SyncTarget
	syncTarget, err := workspaceClient.SyncTargets().Get(ctx, sr.options.SyncTargetName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get SyncTarget %s: %w", sr.options.SyncTargetName, err)
	}

	// Update heartbeat timestamp
	now := metav1.NewTime(time.Now())
	syncTarget.Status.HeartbeatTime = now

	// Update syncer ready condition
	sr.updateSyncTargetConditions(syncTarget, true, "HeartbeatSent", "Syncer heartbeat sent successfully")

	// Update the SyncTarget status
	_, err = workspaceClient.SyncTargets().UpdateStatus(ctx, syncTarget, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update SyncTarget status: %w", err)
	}

	logger.Info("Heartbeat sent successfully")
	return nil
}

// updateSyncTargetStatus updates the overall SyncTarget status
func (sr *StatusReporter) updateSyncTargetStatus(ctx context.Context) error {
	logger := klog.FromContext(ctx).WithValues("component", "status-reporter", "operation", "status-update")

	// Parse logical cluster from workspace
	clusterName := logicalcluster.Name(sr.options.SyncTargetWorkspace)
	workspaceClient := sr.kcpClient.Cluster(clusterName.Path()).WorkloadV1alpha1()
	
	// Get current SyncTarget
	syncTarget, err := workspaceClient.SyncTargets().Get(ctx, sr.options.SyncTargetName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get SyncTarget %s: %w", sr.options.SyncTargetName, err)
	}

	// Update overall syncer status
	sr.updateSyncTargetConditions(syncTarget, sr.healthy, "SyncerHealthy", "Syncer is running and healthy")

	// Update the SyncTarget status
	_, err = workspaceClient.SyncTargets().UpdateStatus(ctx, syncTarget, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update SyncTarget status: %w", err)
	}

	sr.mu.Lock()
	sr.statusUpdates++
	sr.mu.Unlock()

	logger.Info("Status updated successfully")
	return nil
}

// updateSyncTargetConditions updates the conditions on the SyncTarget
func (sr *StatusReporter) updateSyncTargetConditions(syncTarget *workloadv1alpha1.SyncTarget, ready bool, reason, message string) {
	now := metav1.NewTime(time.Now())

	// Determine condition status
	var conditionStatus corev1.ConditionStatus
	if ready {
		conditionStatus = corev1.ConditionTrue
	} else {
		conditionStatus = corev1.ConditionFalse
	}

	// Create the syncer ready condition
	syncerReadyCondition := conditionsv1alpha1.Condition{
		Type:               conditionsv1alpha1.ConditionType(workloadv1alpha1.SyncTargetSyncerReady),
		Status:             conditionStatus,
		LastTransitionTime: now,
		Reason:             reason,
		Message:            message,
	}

	// Set the condition using the proper helper
	conditionsutil.Set(syncTarget, &syncerReadyCondition)
}

// handleHeartbeatSuccess handles a successful heartbeat
func (sr *StatusReporter) handleHeartbeatSuccess() {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	sr.lastHeartbeat = time.Now()
	sr.healthy = true
	
	// Reset error count on success
	sr.heartbeatErrors = 0
}

// handleHeartbeatError handles a heartbeat error
func (sr *StatusReporter) handleHeartbeatError() {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	sr.heartbeatErrors++
	
	// Mark as unhealthy after 3 consecutive failures
	if sr.heartbeatErrors >= 3 {
		sr.healthy = false
	}
}

// ReportSyncerError reports a syncer error to the SyncTarget status
func (sr *StatusReporter) ReportSyncerError(ctx context.Context, err error) {
	logger := klog.FromContext(ctx).WithValues("component", "status-reporter", "operation", "error-report")

	// Parse logical cluster from workspace
	clusterName := logicalcluster.Name(sr.options.SyncTargetWorkspace)
	workspaceClient := sr.kcpClient.Cluster(clusterName.Path()).WorkloadV1alpha1()
	
	// Get current SyncTarget
	syncTarget, syncTargetErr := workspaceClient.SyncTargets().Get(ctx, sr.options.SyncTargetName, metav1.GetOptions{})
	if syncTargetErr != nil {
		logger.Error(syncTargetErr, "Failed to get SyncTarget for error reporting")
		return
	}

	// Update condition with error
	sr.updateSyncTargetConditions(syncTarget, false, "SyncerError", err.Error())

	// Update the SyncTarget status
	_, updateErr := workspaceClient.SyncTargets().UpdateStatus(ctx, syncTarget, metav1.UpdateOptions{})
	if updateErr != nil {
		logger.Error(updateErr, "Failed to update SyncTarget status with error")
		return
	}

	logger.WithValues("error", err).Info("Reported syncer error to SyncTarget status")
}

// ReportSyncerRecovery reports syncer recovery to the SyncTarget status
func (sr *StatusReporter) ReportSyncerRecovery(ctx context.Context) {
	logger := klog.FromContext(ctx).WithValues("component", "status-reporter", "operation", "recovery-report")

	sr.mu.Lock()
	sr.healthy = true
	sr.mu.Unlock()

	if err := sr.updateSyncTargetStatus(ctx); err != nil {
		logger.Error(err, "Failed to report syncer recovery")
		return
	}

	logger.Info("Reported syncer recovery to SyncTarget status")
}