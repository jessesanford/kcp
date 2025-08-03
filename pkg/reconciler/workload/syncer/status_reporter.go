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

package syncer

import (
	"context"
	"fmt"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"

	workloadv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/workload/v1alpha1"
	"github.com/kcp-dev/kcp/pkg/reconciler/workload/tmc"
)

// StatusReporter handles SyncTarget status management and heartbeats
type StatusReporter struct {
	// Configuration
	syncTargetName   string
	syncTargetUID    string
	workspaceCluster logicalcluster.Name
	heartbeatPeriod  time.Duration
	
	// Clients
	kcpClient dynamic.Interface
	
	// TMC Integration
	tmcMetrics *tmc.MetricsCollector
	
	// State
	started           bool
	stopCh            chan struct{}
	mu                sync.RWMutex
	lastHeartbeat     time.Time
	heartbeatCount    int64
	errorCount        int64
	connectionHealthy bool
}

// StatusReporterOptions configures the status reporter
type StatusReporterOptions struct {
	SyncTargetName   string
	SyncTargetUID    string
	WorkspaceCluster logicalcluster.Name
	KCPClient        dynamic.Interface
	HeartbeatPeriod  time.Duration
	TMCMetrics       *tmc.MetricsCollector
}

// NewStatusReporter creates a new status reporter
func NewStatusReporter(options StatusReporterOptions) (*StatusReporter, error) {
	logger := klog.Background().WithValues(
		"component", "StatusReporter",
		"syncTarget", options.SyncTargetName,
	)
	logger.Info("Creating status reporter")

	// Set default heartbeat period if not specified
	heartbeatPeriod := options.HeartbeatPeriod
	if heartbeatPeriod == 0 {
		heartbeatPeriod = 30 * time.Second
	}

	sr := &StatusReporter{
		syncTargetName:   options.SyncTargetName,
		syncTargetUID:    options.SyncTargetUID,
		workspaceCluster: options.WorkspaceCluster,
		heartbeatPeriod:  heartbeatPeriod,
		kcpClient:        options.KCPClient,
		tmcMetrics:       options.TMCMetrics,
		stopCh:           make(chan struct{}),
		connectionHealthy: true,
	}

	logger.Info("Successfully created status reporter", "heartbeatPeriod", heartbeatPeriod)
	return sr, nil
}

// Start starts the status reporter
func (sr *StatusReporter) Start(ctx context.Context) error {
	logger := klog.FromContext(ctx).WithValues(
		"component", "StatusReporter",
		"syncTarget", sr.syncTargetName,
	)
	logger.Info("Starting status reporter")

	sr.mu.Lock()
	if sr.started {
		sr.mu.Unlock()
		return fmt.Errorf("status reporter already started")
	}
	sr.started = true
	sr.mu.Unlock()

	// Send initial status update
	if err := sr.updateSyncTargetStatus(ctx, true); err != nil {
		logger.Error(err, "Failed to send initial status update")
		return tmc.NewTMCError(tmc.TMCErrorTypeSyncFailure, "status-reporter", "initial-status").
			WithMessage("Failed to send initial status update").
			WithCause(err).
			Build()
	}

	// Start heartbeat loop
	go sr.heartbeatLoop(ctx)

	logger.Info("Status reporter started successfully")
	return nil
}

// Stop stops the status reporter
func (sr *StatusReporter) Stop() {
	logger := klog.Background().WithValues(
		"component", "StatusReporter",
		"syncTarget", sr.syncTargetName,
	)
	logger.Info("Stopping status reporter")

	sr.mu.Lock()
	defer sr.mu.Unlock()

	if !sr.started {
		return
	}

	close(sr.stopCh)
	sr.started = false

	logger.Info("Status reporter stopped")
}

// heartbeatLoop runs the periodic heartbeat updates
func (sr *StatusReporter) heartbeatLoop(ctx context.Context) {
	logger := klog.FromContext(ctx).WithValues(
		"component", "StatusReporter",
		"operation", "heartbeat-loop",
	)
	logger.Info("Starting heartbeat loop")

	ticker := time.NewTicker(sr.heartbeatPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("Heartbeat loop stopping due to context cancellation")
			return
		case <-sr.stopCh:
			logger.Info("Heartbeat loop stopping due to stop signal")
			return
		case <-ticker.C:
			if err := sr.sendHeartbeat(ctx); err != nil {
				logger.Error(err, "Failed to send heartbeat")
				sr.mu.Lock()
				sr.errorCount++
				sr.connectionHealthy = false
				sr.mu.Unlock()
				sr.tmcMetrics.RecordComponentError("status-reporter", sr.syncTargetName, 
					tmc.TMCErrorTypeSyncFailure, tmc.TMCErrorSeverityMedium)
			} else {
				sr.mu.Lock()
				sr.heartbeatCount++
				sr.lastHeartbeat = time.Now()
				sr.connectionHealthy = true
				sr.mu.Unlock()
			}
		}
	}
}

// sendHeartbeat sends a heartbeat update to the SyncTarget
func (sr *StatusReporter) sendHeartbeat(ctx context.Context) error {
	logger := klog.FromContext(ctx).WithValues(
		"component", "StatusReporter",
		"operation", "send-heartbeat",
	)

	startTime := time.Now()
	err := sr.updateSyncTargetStatus(ctx, false)
	duration := time.Since(startTime)

	if err == nil {
		sr.tmcMetrics.RecordComponentLatency("status-reporter", sr.syncTargetName, "heartbeat", duration)
		logger.V(4).Info("Heartbeat sent successfully", "duration", duration)
	}

	return err
}

// updateSyncTargetStatus updates the SyncTarget status with current information
func (sr *StatusReporter) updateSyncTargetStatus(ctx context.Context, initialUpdate bool) error {
	logger := klog.FromContext(ctx).WithValues(
		"component", "StatusReporter",
		"operation", "update-status",
		"initial", initialUpdate,
	)

	// Get the SyncTarget resource
	syncTargetResource := sr.kcpClient.Resource(workloadv1alpha1.SchemeGroupVersion.WithResource("synctargets"))
	syncTargetObj, err := syncTargetResource.Get(ctx, sr.syncTargetName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get SyncTarget %s: %w", sr.syncTargetName, err)
	}

	// Prepare status update
	now := metav1.NewTime(time.Now())
	
	// Get current status
	status, found, err := unstructured.NestedMap(syncTargetObj.Object, "status")
	if err != nil {
		return fmt.Errorf("failed to get current status: %w", err)
	}
	if !found {
		status = make(map[string]interface{})
	}

	// Update syncer identifier
	status["syncerIdentifier"] = sr.syncTargetUID

	// Update last heartbeat time
	status["lastHeartbeatTime"] = now.Time.Format(time.RFC3339)

	// Prepare conditions
	var conditions []interface{}
	if existingConditions, found, err := unstructured.NestedSlice(status, "conditions"); err == nil && found {
		conditions = existingConditions
	}

	// Update SyncerReady condition
	syncerReadyCondition := map[string]interface{}{
		"type":               string(workloadv1alpha1.SyncTargetSyncerReady),
		"status":             string(metav1.ConditionTrue),
		"lastTransitionTime": now.Time.Format(time.RFC3339),
		"reason":             "SyncerConnected",
		"message":            "Syncer is connected and sending heartbeats",
	}

	// Update HeartbeatReady condition  
	heartbeatReadyCondition := map[string]interface{}{
		"type":               string(workloadv1alpha1.SyncTargetHeartbeatReady),
		"status":             string(metav1.ConditionTrue),
		"lastTransitionTime": now.Time.Format(time.RFC3339),
		"reason":             "HeartbeatReceived",
		"message":            fmt.Sprintf("Heartbeat received at %s", now.Time.Format(time.RFC3339)),
	}

	// Update Ready condition based on overall health
	readyStatus := metav1.ConditionTrue
	readyReason := "SyncTargetReady"
	readyMessage := "SyncTarget is ready and operational"

	sr.mu.RLock()
	if !sr.connectionHealthy {
		readyStatus = metav1.ConditionFalse
		readyReason = "ConnectionUnhealthy"
		readyMessage = "SyncTarget connection is experiencing issues"
	}
	sr.mu.RUnlock()

	readyCondition := map[string]interface{}{
		"type":               string(workloadv1alpha1.SyncTargetReady),
		"status":             string(readyStatus),
		"lastTransitionTime": now.Time.Format(time.RFC3339),
		"reason":             readyReason,
		"message":            readyMessage,
	}

	// Replace or add conditions
	conditions = sr.setCondition(conditions, syncerReadyCondition)
	conditions = sr.setCondition(conditions, heartbeatReadyCondition)
	conditions = sr.setCondition(conditions, readyCondition)

	// Update conditions in status
	status["conditions"] = conditions

	// Add heartbeat statistics
	sr.mu.RLock()
	heartbeatStats := map[string]interface{}{
		"count":         sr.heartbeatCount,
		"lastHeartbeat": sr.lastHeartbeat.Format(time.RFC3339),
		"errors":        sr.errorCount,
	}
	sr.mu.RUnlock()
	status["heartbeat"] = heartbeatStats

	// Set the updated status back
	if err := unstructured.SetNestedMap(syncTargetObj.Object, status, "status"); err != nil {
		return fmt.Errorf("failed to set status: %w", err)
	}

	// Update the SyncTarget
	_, err = syncTargetResource.UpdateStatus(ctx, syncTargetObj, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update SyncTarget status: %w", err)
	}

	logger.V(3).Info("Successfully updated SyncTarget status")
	return nil
}

// setCondition sets or updates a condition in the conditions slice
func (sr *StatusReporter) setCondition(conditions []interface{}, newCondition map[string]interface{}) []interface{} {
	conditionType := newCondition["type"].(string)
	
	// Find existing condition with the same type
	for i, cond := range conditions {
		if condMap, ok := cond.(map[string]interface{}); ok {
			if existingType, found := condMap["type"]; found && existingType == conditionType {
				// Update existing condition
				conditions[i] = newCondition
				return conditions
			}
		}
	}
	
	// Add new condition
	return append(conditions, newCondition)
}

// UpdateSyncTargetWithError updates the SyncTarget status with an error condition
func (sr *StatusReporter) UpdateSyncTargetWithError(ctx context.Context, tmcError *tmc.TMCError) error {
	logger := klog.FromContext(ctx).WithValues(
		"component", "StatusReporter",
		"operation", "update-error",
		"errorType", tmcError.Type,
	)

	// Get the SyncTarget resource
	syncTargetResource := sr.kcpClient.Resource(workloadv1alpha1.SchemeGroupVersion.WithResource("synctargets"))
	syncTargetObj, err := syncTargetResource.Get(ctx, sr.syncTargetName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get SyncTarget %s: %w", sr.syncTargetName, err)
	}

	// Get current status
	status, found, err := unstructured.NestedMap(syncTargetObj.Object, "status")
	if err != nil {
		return fmt.Errorf("failed to get current status: %w", err)
	}
	if !found {
		status = make(map[string]interface{})
	}

	// Get existing conditions
	var conditions []interface{}
	if existingConditions, found, err := unstructured.NestedSlice(status, "conditions"); err == nil && found {
		conditions = existingConditions
	}

	// Create error condition
	now := metav1.NewTime(time.Now())
	errorCondition := map[string]interface{}{
		"type":               string(workloadv1alpha1.SyncTargetReady),
		"status":             string(metav1.ConditionFalse),
		"lastTransitionTime": now.Time.Format(time.RFC3339),
		"reason":             string(tmcError.Type),
		"message":            tmcError.Message,
	}

	// Update conditions
	conditions = sr.setCondition(conditions, errorCondition)
	status["conditions"] = conditions

	// Set the updated status back
	if err := unstructured.SetNestedMap(syncTargetObj.Object, status, "status"); err != nil {
		return fmt.Errorf("failed to set status: %w", err)
	}

	// Update the SyncTarget
	_, err = syncTargetResource.UpdateStatus(ctx, syncTargetObj, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update SyncTarget status: %w", err)
	}

	logger.Info("Updated SyncTarget with error condition")
	return nil
}

// GetStatus returns the current status of the status reporter
func (sr *StatusReporter) GetStatus() *StatusReporterStatus {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	return &StatusReporterStatus{
		SyncTargetName:    sr.syncTargetName,
		Started:           sr.started,
		HeartbeatCount:    sr.heartbeatCount,
		ErrorCount:        sr.errorCount,
		LastHeartbeat:     sr.lastHeartbeat,
		ConnectionHealthy: sr.connectionHealthy,
		HeartbeatPeriod:   sr.heartbeatPeriod,
	}
}

// StatusReporterStatus represents the status of the status reporter
type StatusReporterStatus struct {
	SyncTargetName    string
	Started           bool
	HeartbeatCount    int64
	ErrorCount        int64
	LastHeartbeat     time.Time
	ConnectionHealthy bool
	HeartbeatPeriod   time.Duration
}