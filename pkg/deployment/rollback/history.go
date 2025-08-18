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
	"sort"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"
)

// HistoryManager manages rollback operation history.
type HistoryManager struct {
	dynamicClient dynamic.Interface
	cluster       logicalcluster.Name
	config        *EngineConfig
	
	// In-memory cache of histories for faster access
	historyCache map[string]*RollbackHistory
	mu           sync.RWMutex
	
	// Storage backend interface
	storage HistoryStorage
}

// HistoryStorage defines the interface for persisting rollback history.
type HistoryStorage interface {
	Store(ctx context.Context, history *RollbackHistory) error
	Load(ctx context.Context, deploymentRef corev1.ObjectReference) (*RollbackHistory, error)
	Delete(ctx context.Context, deploymentRef corev1.ObjectReference) error
	List(ctx context.Context) ([]*RollbackHistory, error)
}

// NewHistoryManager creates a new history manager.
func NewHistoryManager(client dynamic.Interface, cluster logicalcluster.Name, config *EngineConfig) *HistoryManager {
	storage := NewConfigMapHistoryStorage(client, cluster)
	
	return &HistoryManager{
		dynamicClient: client,
		cluster:       cluster,
		config:        config,
		historyCache:  make(map[string]*RollbackHistory),
		storage:       storage,
	}
}

// RecordOperation records a rollback operation in the history.
func (hm *HistoryManager) RecordOperation(ctx context.Context, deploymentRef corev1.ObjectReference, operation RollbackOperation) error {
	klog.V(2).InfoS("Recording rollback operation", "deployment", deploymentRef.Name, "operation", operation.ID, "type", operation.Type)
	
	history, err := hm.getOrCreateHistory(ctx, deploymentRef)
	if err != nil {
		return fmt.Errorf("failed to get deployment history: %w", err)
	}
	
	hm.mu.Lock()
	defer hm.mu.Unlock()
	
	// Add the operation to history
	history.Operations = append(history.Operations, operation)
	history.LastUpdated = metav1.NewTime(time.Now())
	
	// Sort operations by start time (most recent first)
	sort.Slice(history.Operations, func(i, j int) bool {
		return history.Operations[i].StartTime.Time.After(history.Operations[j].StartTime.Time)
	})
	
	// Limit the number of operations stored
	maxOperations := 50 // Default
	if hm.config.MaxSnapshots > 0 {
		maxOperations = hm.config.MaxSnapshots * 2 // Keep more operations than snapshots
	}
	
	if len(history.Operations) > maxOperations {
		history.Operations = history.Operations[:maxOperations]
	}
	
	// Update cache
	key := hm.getDeploymentKey(deploymentRef)
	hm.historyCache[key] = history
	
	// Persist to storage
	if err := hm.storage.Store(ctx, history); err != nil {
		return fmt.Errorf("failed to persist history: %w", err)
	}
	
	klog.V(2).InfoS("Recorded rollback operation", "deployment", deploymentRef.Name, "totalOperations", len(history.Operations))
	return nil
}

// GetHistory retrieves the rollback history for a deployment.
func (hm *HistoryManager) GetHistory(ctx context.Context, deploymentRef corev1.ObjectReference) (*RollbackHistory, error) {
	return hm.getOrCreateHistory(ctx, deploymentRef)
}

// GetOperation retrieves a specific operation from history.
func (hm *HistoryManager) GetOperation(ctx context.Context, deploymentRef corev1.ObjectReference, operationID string) (*RollbackOperation, error) {
	history, err := hm.GetHistory(ctx, deploymentRef)
	if err != nil {
		return nil, fmt.Errorf("failed to get history: %w", err)
	}
	
	for _, operation := range history.Operations {
		if operation.ID == operationID {
			return &operation, nil
		}
	}
	
	return nil, fmt.Errorf("operation %s not found", operationID)
}

// ListOperations returns operations for a deployment, optionally filtered by type.
func (hm *HistoryManager) ListOperations(ctx context.Context, deploymentRef corev1.ObjectReference, operationType *OperationType) ([]RollbackOperation, error) {
	history, err := hm.GetHistory(ctx, deploymentRef)
	if err != nil {
		return nil, fmt.Errorf("failed to get history: %w", err)
	}
	
	var filtered []RollbackOperation
	for _, operation := range history.Operations {
		if operationType == nil || operation.Type == *operationType {
			filtered = append(filtered, operation)
		}
	}
	
	return filtered, nil
}

// GetSuccessfulOperations returns only successful operations.
func (hm *HistoryManager) GetSuccessfulOperations(ctx context.Context, deploymentRef corev1.ObjectReference) ([]RollbackOperation, error) {
	history, err := hm.GetHistory(ctx, deploymentRef)
	if err != nil {
		return nil, fmt.Errorf("failed to get history: %w", err)
	}
	
	var successful []RollbackOperation
	for _, operation := range history.Operations {
		if operation.Success {
			successful = append(successful, operation)
		}
	}
	
	return successful, nil
}

// GetFailedOperations returns only failed operations.
func (hm *HistoryManager) GetFailedOperations(ctx context.Context, deploymentRef corev1.ObjectReference) ([]RollbackOperation, error) {
	history, err := hm.GetHistory(ctx, deploymentRef)
	if err != nil {
		return nil, fmt.Errorf("failed to get history: %w", err)
	}
	
	var failed []RollbackOperation
	for _, operation := range history.Operations {
		if !operation.Success {
			failed = append(failed, operation)
		}
	}
	
	return failed, nil
}

// GetRecentOperations returns operations from the specified time window.
func (hm *HistoryManager) GetRecentOperations(ctx context.Context, deploymentRef corev1.ObjectReference, since time.Duration) ([]RollbackOperation, error) {
	history, err := hm.GetHistory(ctx, deploymentRef)
	if err != nil {
		return nil, fmt.Errorf("failed to get history: %w", err)
	}
	
	cutoff := time.Now().Add(-since)
	var recent []RollbackOperation
	
	for _, operation := range history.Operations {
		if operation.StartTime.Time.After(cutoff) {
			recent = append(recent, operation)
		}
	}
	
	return recent, nil
}

// GetStatistics returns statistics about rollback operations.
func (hm *HistoryManager) GetStatistics(ctx context.Context, deploymentRef corev1.ObjectReference) (*OperationStatistics, error) {
	history, err := hm.GetHistory(ctx, deploymentRef)
	if err != nil {
		return nil, fmt.Errorf("failed to get history: %w", err)
	}
	
	stats := &OperationStatistics{
		TotalOperations: len(history.Operations),
	}
	
	var totalDuration time.Duration
	operationCounts := make(map[OperationType]int)
	
	for _, operation := range history.Operations {
		if operation.Success {
			stats.SuccessfulOperations++
		} else {
			stats.FailedOperations++
		}
		
		operationCounts[operation.Type]++
		totalDuration += operation.Duration
	}
	
	if stats.TotalOperations > 0 {
		stats.SuccessRate = float64(stats.SuccessfulOperations) / float64(stats.TotalOperations) * 100
		stats.AverageDuration = totalDuration / time.Duration(stats.TotalOperations)
	}
	
	stats.OperationsByType = operationCounts
	
	// Find last operation
	if len(history.Operations) > 0 {
		stats.LastOperation = &history.Operations[0]
	}
	
	return stats, nil
}

// CleanupOldHistory removes old history entries based on retention policy.
func (hm *HistoryManager) CleanupOldHistory(ctx context.Context) error {
	klog.InfoS("Starting history cleanup")
	
	histories, err := hm.storage.List(ctx)
	if err != nil {
		return fmt.Errorf("failed to list histories: %w", err)
	}
	
	if hm.config.SnapshotRetentionDuration == nil {
		klog.V(2).InfoS("No retention policy configured, skipping cleanup")
		return nil
	}
	
	cutoff := time.Now().Add(-hm.config.SnapshotRetentionDuration.Duration)
	var cleaned int
	
	for _, history := range histories {
		// Clean up old operations within each history
		originalCount := len(history.Operations)
		var keepOperations []RollbackOperation
		
		for _, operation := range history.Operations {
			if operation.StartTime.Time.After(cutoff) {
				keepOperations = append(keepOperations, operation)
			}
		}
		
		if len(keepOperations) != originalCount {
			history.Operations = keepOperations
			history.LastUpdated = metav1.NewTime(time.Now())
			
			// Update storage
			if err := hm.storage.Store(ctx, history); err != nil {
				klog.ErrorS(err, "Failed to update history after cleanup", "deployment", history.DeploymentRef.Name)
			} else {
				cleaned += originalCount - len(keepOperations)
				
				// Update cache
				key := hm.getDeploymentKey(history.DeploymentRef)
				hm.mu.Lock()
				hm.historyCache[key] = history
				hm.mu.Unlock()
			}
		}
		
		// If no operations remain, consider removing the entire history
		if len(history.Operations) == 0 && history.CreatedAt.Time.Before(cutoff) {
			if err := hm.storage.Delete(ctx, history.DeploymentRef); err != nil {
				klog.ErrorS(err, "Failed to delete empty history", "deployment", history.DeploymentRef.Name)
			} else {
				key := hm.getDeploymentKey(history.DeploymentRef)
				hm.mu.Lock()
				delete(hm.historyCache, key)
				hm.mu.Unlock()
				klog.V(2).InfoS("Deleted empty history", "deployment", history.DeploymentRef.Name)
			}
		}
	}
	
	klog.InfoS("Completed history cleanup", "cleanedOperations", cleaned)
	return nil
}

// StartOperation creates a new operation record and returns its ID.
func (hm *HistoryManager) StartOperation(ctx context.Context, deploymentRef corev1.ObjectReference, operationType OperationType, reason, triggeredBy string) (string, error) {
	operationID := hm.generateOperationID(operationType)
	
	operation := RollbackOperation{
		ID:            operationID,
		Type:          operationType,
		StartTime:     metav1.NewTime(time.Now()),
		Success:       false, // Will be updated when operation completes
		Reason:        reason,
		TriggeredBy:   triggeredBy,
	}
	
	if err := hm.RecordOperation(ctx, deploymentRef, operation); err != nil {
		return "", fmt.Errorf("failed to record operation start: %w", err)
	}
	
	klog.InfoS("Started operation tracking", "deployment", deploymentRef.Name, "operation", operationID, "type", operationType)
	return operationID, nil
}

// CompleteOperation marks an operation as completed with success/failure status.
func (hm *HistoryManager) CompleteOperation(ctx context.Context, deploymentRef corev1.ObjectReference, operationID string, success bool, errorMsg string, fromSnapshot, toSnapshot string) error {
	history, err := hm.getOrCreateHistory(ctx, deploymentRef)
	if err != nil {
		return fmt.Errorf("failed to get history: %w", err)
	}
	
	hm.mu.Lock()
	defer hm.mu.Unlock()
	
	// Find and update the operation
	for i, operation := range history.Operations {
		if operation.ID == operationID {
			endTime := metav1.NewTime(time.Now())
			history.Operations[i].EndTime = &endTime
			history.Operations[i].Success = success
			history.Operations[i].Duration = endTime.Time.Sub(operation.StartTime.Time)
			history.Operations[i].FromSnapshot = fromSnapshot
			history.Operations[i].ToSnapshot = toSnapshot
			
			if errorMsg != "" {
				history.Operations[i].Error = errorMsg
			}
			
			history.LastUpdated = metav1.NewTime(time.Now())
			
			// Update cache
			key := hm.getDeploymentKey(deploymentRef)
			hm.historyCache[key] = history
			
			// Persist to storage
			if err := hm.storage.Store(ctx, history); err != nil {
				return fmt.Errorf("failed to persist updated history: %w", err)
			}
			
			klog.InfoS("Completed operation", "deployment", deploymentRef.Name, "operation", operationID, "success", success, "duration", history.Operations[i].Duration)
			return nil
		}
	}
	
	return fmt.Errorf("operation %s not found", operationID)
}

// Private methods

// getOrCreateHistory retrieves or creates history for a deployment.
func (hm *HistoryManager) getOrCreateHistory(ctx context.Context, deploymentRef corev1.ObjectReference) (*RollbackHistory, error) {
	key := hm.getDeploymentKey(deploymentRef)
	
	// Check cache first
	hm.mu.RLock()
	if history, exists := hm.historyCache[key]; exists {
		hm.mu.RUnlock()
		return history, nil
	}
	hm.mu.RUnlock()
	
	// Try to load from storage
	history, err := hm.storage.Load(ctx, deploymentRef)
	if err != nil {
		// Create new history if not found
		history = &RollbackHistory{
			DeploymentRef: deploymentRef,
			Operations:    []RollbackOperation{},
			CreatedAt:     metav1.NewTime(time.Now()),
			LastUpdated:   metav1.NewTime(time.Now()),
		}
		
		klog.V(2).InfoS("Created new rollback history", "deployment", deploymentRef.Name)
	}
	
	// Update cache
	hm.mu.Lock()
	hm.historyCache[key] = history
	hm.mu.Unlock()
	
	return history, nil
}

// generateOperationID creates a unique identifier for an operation.
func (hm *HistoryManager) generateOperationID(operationType OperationType) string {
	timestamp := time.Now().Unix()
	return fmt.Sprintf("%s-%d", operationType, timestamp)
}

// getDeploymentKey creates a unique key for a deployment.
func (hm *HistoryManager) getDeploymentKey(deploymentRef corev1.ObjectReference) string {
	return fmt.Sprintf("%s/%s", deploymentRef.Namespace, deploymentRef.Name)
}

// OperationStatistics holds statistics about rollback operations.
type OperationStatistics struct {
	TotalOperations      int                        `json:"totalOperations"`
	SuccessfulOperations int                        `json:"successfulOperations"`
	FailedOperations     int                        `json:"failedOperations"`
	SuccessRate          float64                    `json:"successRate"`
	AverageDuration      time.Duration              `json:"averageDuration"`
	OperationsByType     map[OperationType]int      `json:"operationsByType"`
	LastOperation        *RollbackOperation         `json:"lastOperation,omitempty"`
}

// ConfigMapHistoryStorage implements HistoryStorage using ConfigMaps.
type ConfigMapHistoryStorage struct {
	client  dynamic.Interface
	cluster logicalcluster.Name
}

// NewConfigMapHistoryStorage creates a new ConfigMap-based history storage.
func NewConfigMapHistoryStorage(client dynamic.Interface, cluster logicalcluster.Name) *ConfigMapHistoryStorage {
	return &ConfigMapHistoryStorage{
		client:  client,
		cluster: cluster,
	}
}

// Store persists history to a ConfigMap.
func (cms *ConfigMapHistoryStorage) Store(ctx context.Context, history *RollbackHistory) error {
	// Convert history to JSON
	data, err := json.Marshal(history)
	if err != nil {
		return fmt.Errorf("failed to marshal history: %w", err)
	}
	
	// Create ConfigMap name
	cmName := fmt.Sprintf("rollback-history-%s", history.DeploymentRef.Name)
	
	// This would create/update a ConfigMap with the history data
	klog.V(2).InfoS("Storing history in ConfigMap", "configmap", cmName, "deployment", history.DeploymentRef.Name)
	
	// Placeholder - actual implementation would use ConfigMap resources
	return nil
}

// Load retrieves history from a ConfigMap.
func (cms *ConfigMapHistoryStorage) Load(ctx context.Context, deploymentRef corev1.ObjectReference) (*RollbackHistory, error) {
	cmName := fmt.Sprintf("rollback-history-%s", deploymentRef.Name)
	
	// This would load from a ConfigMap
	klog.V(2).InfoS("Loading history from ConfigMap", "configmap", cmName, "deployment", deploymentRef.Name)
	
	// Placeholder - return not found error to trigger creation of new history
	return nil, fmt.Errorf("history not found")
}

// Delete removes history from storage.
func (cms *ConfigMapHistoryStorage) Delete(ctx context.Context, deploymentRef corev1.ObjectReference) error {
	cmName := fmt.Sprintf("rollback-history-%s", deploymentRef.Name)
	
	// This would delete the ConfigMap
	klog.V(2).InfoS("Deleting history ConfigMap", "configmap", cmName, "deployment", deploymentRef.Name)
	
	// Placeholder
	return nil
}

// List returns all stored histories.
func (cms *ConfigMapHistoryStorage) List(ctx context.Context) ([]*RollbackHistory, error) {
	// This would list all rollback history ConfigMaps
	klog.V(2).InfoS("Listing all history ConfigMaps")
	
	// Placeholder - return empty list
	return []*RollbackHistory{}, nil
}