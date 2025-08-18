/*
Copyright 2023 The KCP Authors.

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

package decision

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"
)

// DecisionRecorder provides decision recording, auditing, and historical tracking
// for TMC placement decisions. It maintains an audit trail, emits events and metrics,
// and provides query capabilities for decision history.
type DecisionRecorder interface {
	// RecordDecision records a placement decision with full audit trail
	RecordDecision(ctx context.Context, decision *PlacementDecision) error
	
	// RecordDecisionAttempt records a decision attempt (including failures)
	RecordDecisionAttempt(ctx context.Context, attempt *DecisionAttempt) error
	
	// QueryDecisionHistory retrieves historical decisions based on criteria
	QueryDecisionHistory(ctx context.Context, query *HistoryQuery) ([]*DecisionRecord, error)
	
	// GetDecisionMetrics returns decision-making metrics for observability
	GetDecisionMetrics(ctx context.Context, timeRange TimeRange) (*DecisionMetrics, error)
	
	// PurgeOldRecords removes records older than the retention policy
	PurgeOldRecords(ctx context.Context, retentionPolicy *RetentionPolicy) error
	
	// EmitDecisionEvent emits Kubernetes events for decision changes
	EmitDecisionEvent(ctx context.Context, decision *PlacementDecision, eventType DecisionEventType, reason, message string) error
}

// decisionRecorder implements DecisionRecorder with in-memory storage and metrics.
type decisionRecorder struct {
	// Storage for decision records
	storage DecisionStorage
	
	// Event recorder for Kubernetes events
	eventRecorder record.EventRecorder
	
	// Metrics collectors
	metrics *DecisionMetricsCollector
	
	// Configuration
	config *RecorderConfig
	
	// Synchronization
	mu sync.RWMutex
	
	// Background cleanup
	stopCh chan struct{}
	
	// Audit logger
	auditLogger klog.Logger
}

// NewDecisionRecorder creates a new decision recorder with the specified configuration.
func NewDecisionRecorder(
	storage DecisionStorage,
	eventRecorder record.EventRecorder,
	config *RecorderConfig,
) (DecisionRecorder, error) {
	if storage == nil {
		return nil, fmt.Errorf("decision storage cannot be nil")
	}
	if eventRecorder == nil {
		return nil, fmt.Errorf("event recorder cannot be nil")
	}
	if config == nil {
		config = DefaultRecorderConfig()
	}
	
	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid recorder configuration: %w", err)
	}
	
	metrics, err := NewDecisionMetricsCollector()
	if err != nil {
		return nil, fmt.Errorf("failed to create metrics collector: %w", err)
	}
	
	recorder := &decisionRecorder{
		storage:       storage,
		eventRecorder: eventRecorder,
		metrics:       metrics,
		config:        config,
		stopCh:        make(chan struct{}),
		auditLogger:   klog.Background().WithName("decision-recorder").WithName("audit"),
	}
	
	// Start background cleanup goroutine
	go recorder.cleanupLoop()
	
	return recorder, nil
}

// RecordDecision records a completed placement decision with full audit trail.
func (r *decisionRecorder) RecordDecision(ctx context.Context, decision *PlacementDecision) error {
	if decision == nil {
		return fmt.Errorf("decision cannot be nil")
	}
	
	r.mu.Lock()
	defer r.mu.Unlock()
	
	// Create decision record
	record := &DecisionRecord{
		DecisionID:      decision.ID,
		RequestID:       decision.RequestID,
		Decision:        decision,
		RecordedAt:      time.Now(),
		RecorderVersion: r.config.Version,
		TTL:             r.config.DefaultTTL,
	}
	
	// Store the record
	if err := r.storage.Store(ctx, record); err != nil {
		r.metrics.RecordStorageError("decision", err)
		return fmt.Errorf("failed to store decision record: %w", err)
	}
	
	// Record metrics
	r.metrics.RecordDecision(decision)
	
	// Emit audit log
	r.auditDecision(decision, "DECISION_RECORDED")
	
	// Emit Kubernetes event
	eventType := DecisionEventTypeNormal
	if decision.Status == DecisionStatusError {
		eventType = DecisionEventTypeWarning
	}
	
	if err := r.EmitDecisionEvent(ctx, decision, eventType, "DecisionRecorded", 
		fmt.Sprintf("Placement decision recorded: %s", decision.DecisionRationale.Summary)); err != nil {
		// Log error but don't fail the operation
		klog.V(2).ErrorS(err, "Failed to emit decision event", "decisionID", decision.ID)
	}
	
	return nil
}

// RecordDecisionAttempt records a decision attempt, including partial or failed attempts.
func (r *decisionRecorder) RecordDecisionAttempt(ctx context.Context, attempt *DecisionAttempt) error {
	if attempt == nil {
		return fmt.Errorf("decision attempt cannot be nil")
	}
	
	r.mu.Lock()
	defer r.mu.Unlock()
	
	// Store the attempt
	if err := r.storage.StoreAttempt(ctx, attempt); err != nil {
		r.metrics.RecordStorageError("attempt", err)
		return fmt.Errorf("failed to store decision attempt: %w", err)
	}
	
	// Record metrics
	r.metrics.RecordDecisionAttempt(attempt)
	
	// Emit audit log
	r.auditAttempt(attempt, "DECISION_ATTEMPT")
	
	return nil
}

// QueryDecisionHistory retrieves historical decisions based on query criteria.
func (r *decisionRecorder) QueryDecisionHistory(ctx context.Context, query *HistoryQuery) ([]*DecisionRecord, error) {
	if query == nil {
		return nil, fmt.Errorf("history query cannot be nil")
	}
	
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	// Validate query
	if err := query.Validate(); err != nil {
		return nil, fmt.Errorf("invalid history query: %w", err)
	}
	
	// Query storage
	records, err := r.storage.Query(ctx, query)
	if err != nil {
		r.metrics.RecordQueryError(err)
		return nil, fmt.Errorf("failed to query decision history: %w", err)
	}
	
	// Record query metrics
	r.metrics.RecordHistoryQuery(query, len(records))
	
	return records, nil
}

// GetDecisionMetrics returns aggregated metrics for the specified time range.
func (r *decisionRecorder) GetDecisionMetrics(ctx context.Context, timeRange TimeRange) (*DecisionMetrics, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	// Validate time range
	if err := timeRange.Validate(); err != nil {
		return nil, fmt.Errorf("invalid time range: %w", err)
	}
	
	// Get metrics from storage
	metrics, err := r.storage.GetMetrics(ctx, timeRange)
	if err != nil {
		return nil, fmt.Errorf("failed to get decision metrics: %w", err)
	}
	
	return metrics, nil
}

// PurgeOldRecords removes records older than the specified retention policy.
func (r *decisionRecorder) PurgeOldRecords(ctx context.Context, retentionPolicy *RetentionPolicy) error {
	if retentionPolicy == nil {
		retentionPolicy = r.config.DefaultRetentionPolicy
	}
	
	r.mu.Lock()
	defer r.mu.Unlock()
	
	// Validate retention policy
	if err := retentionPolicy.Validate(); err != nil {
		return fmt.Errorf("invalid retention policy: %w", err)
	}
	
	// Purge from storage
	purged, err := r.storage.Purge(ctx, retentionPolicy)
	if err != nil {
		r.metrics.RecordPurgeError(err)
		return fmt.Errorf("failed to purge old records: %w", err)
	}
	
	// Record purge metrics
	r.metrics.RecordPurge(purged)
	
	klog.V(4).InfoS("Purged old decision records", "recordCount", purged, "policy", retentionPolicy)
	
	return nil
}

// EmitDecisionEvent emits a Kubernetes event for a decision change.
func (r *decisionRecorder) EmitDecisionEvent(ctx context.Context, decision *PlacementDecision, eventType DecisionEventType, reason, message string) error {
	if decision == nil {
		return fmt.Errorf("decision cannot be nil")
	}
	
	// For now, we'll log the event since we don't have a specific object to attach it to
	// In a real implementation, this would be attached to the placement request object
	eventTypeStr := "Normal"
	if eventType == DecisionEventTypeWarning {
		eventTypeStr = "Warning"
	}
	
	klog.V(3).InfoS("Decision event", 
		"type", eventTypeStr,
		"reason", reason,
		"message", message,
		"decisionID", decision.ID,
		"requestID", decision.RequestID,
	)
	
	// Record event metrics
	r.metrics.RecordEvent(eventType)
	
	return nil
}

// auditDecision logs an audit entry for a decision.
func (r *decisionRecorder) auditDecision(decision *PlacementDecision, action string) {
	r.auditLogger.Info("Decision audit",
		"action", action,
		"decisionID", decision.ID,
		"requestID", decision.RequestID,
		"status", decision.Status,
		"workspaceCount", len(decision.SelectedWorkspaces),
		"decisionTime", decision.DecisionTime,
		"duration", decision.DecisionDuration,
	)
}

// auditAttempt logs an audit entry for a decision attempt.
func (r *decisionRecorder) auditAttempt(attempt *DecisionAttempt, action string) {
	r.auditLogger.Info("Decision attempt audit",
		"action", action,
		"attemptID", attempt.ID,
		"requestID", attempt.RequestID,
		"success", attempt.Success,
		"startTime", attempt.StartTime,
		"duration", attempt.Duration,
	)
}

// cleanupLoop runs periodic cleanup operations.
func (r *decisionRecorder) cleanupLoop() {
	ticker := time.NewTicker(r.config.CleanupInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			r.performCleanup()
		case <-r.stopCh:
			return
		}
	}
}

// performCleanup performs periodic cleanup of old records.
func (r *decisionRecorder) performCleanup() {
	ctx, cancel := context.WithTimeout(context.Background(), r.config.CleanupTimeout)
	defer cancel()
	
	// Use default retention policy for automatic cleanup
	if err := r.PurgeOldRecords(ctx, r.config.DefaultRetentionPolicy); err != nil {
		runtime.HandleError(fmt.Errorf("failed to perform automatic cleanup: %w", err))
	}
}

// Stop stops the recorder and its background operations.
func (r *decisionRecorder) Stop() {
	close(r.stopCh)
}

// DecisionMetricsCollector collects Prometheus metrics for decision recording.
type DecisionMetricsCollector struct {
	decisionsTotal          *prometheus.CounterVec
	decisionDuration        *prometheus.HistogramVec
	decisionAttemptsTotal   *prometheus.CounterVec
	storageOperationsTotal  *prometheus.CounterVec
	queryOperationsTotal    *prometheus.CounterVec
	purgeOperationsTotal    prometheus.Counter
	eventsEmittedTotal      *prometheus.CounterVec
}

// NewDecisionMetricsCollector creates a new metrics collector.
func NewDecisionMetricsCollector() (*DecisionMetricsCollector, error) {
	collector := &DecisionMetricsCollector{
		decisionsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "tmc_placement_decisions_total",
				Help: "Total number of placement decisions recorded",
			},
			[]string{"status", "workspace"},
		),
		decisionDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "tmc_placement_decision_duration_seconds",
				Help:    "Duration of placement decision making",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"status"},
		),
		decisionAttemptsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "tmc_placement_decision_attempts_total",
				Help: "Total number of placement decision attempts",
			},
			[]string{"success", "error_type"},
		),
		storageOperationsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "tmc_decision_storage_operations_total",
				Help: "Total number of decision storage operations",
			},
			[]string{"operation", "status"},
		),
		queryOperationsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "tmc_decision_query_operations_total",
				Help: "Total number of decision history query operations",
			},
			[]string{"query_type", "status"},
		),
		purgeOperationsTotal: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "tmc_decision_purge_operations_total",
				Help: "Total number of decision record purge operations",
			},
		),
		eventsEmittedTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "tmc_decision_events_emitted_total",
				Help: "Total number of decision events emitted",
			},
			[]string{"event_type"},
		),
	}
	
	// Register metrics
	prometheus.MustRegister(
		collector.decisionsTotal,
		collector.decisionDuration,
		collector.decisionAttemptsTotal,
		collector.storageOperationsTotal,
		collector.queryOperationsTotal,
		collector.purgeOperationsTotal,
		collector.eventsEmittedTotal,
	)
	
	return collector, nil
}

// RecordDecision records metrics for a decision.
func (c *DecisionMetricsCollector) RecordDecision(decision *PlacementDecision) {
	c.decisionsTotal.WithLabelValues(string(decision.Status), "all").Inc()
	c.decisionDuration.WithLabelValues(string(decision.Status)).Observe(decision.DecisionDuration.Seconds())
	
	// Record per-workspace metrics
	for _, wp := range decision.SelectedWorkspaces {
		c.decisionsTotal.WithLabelValues(string(decision.Status), string(wp.Workspace)).Inc()
	}
}

// RecordDecisionAttempt records metrics for a decision attempt.
func (c *DecisionMetricsCollector) RecordDecisionAttempt(attempt *DecisionAttempt) {
	successLabel := "true"
	errorType := "none"
	
	if !attempt.Success {
		successLabel = "false"
		if attempt.Error != nil {
			errorType = fmt.Sprintf("%T", attempt.Error)
		} else {
			errorType = "unknown"
		}
	}
	
	c.decisionAttemptsTotal.WithLabelValues(successLabel, errorType).Inc()
}

// RecordStorageError records a storage operation error.
func (c *DecisionMetricsCollector) RecordStorageError(operation string, err error) {
	c.storageOperationsTotal.WithLabelValues(operation, "error").Inc()
}

// RecordQueryError records a query operation error.
func (c *DecisionMetricsCollector) RecordQueryError(err error) {
	c.queryOperationsTotal.WithLabelValues("history", "error").Inc()
}

// RecordHistoryQuery records a successful history query.
func (c *DecisionMetricsCollector) RecordHistoryQuery(query *HistoryQuery, resultCount int) {
	c.queryOperationsTotal.WithLabelValues("history", "success").Inc()
}

// RecordPurgeError records a purge operation error.
func (c *DecisionMetricsCollector) RecordPurgeError(err error) {
	c.storageOperationsTotal.WithLabelValues("purge", "error").Inc()
}

// RecordPurge records a successful purge operation.
func (c *DecisionMetricsCollector) RecordPurge(recordCount int) {
	c.purgeOperationsTotal.Inc()
	c.storageOperationsTotal.WithLabelValues("purge", "success").Inc()
}

// RecordEvent records an event emission.
func (c *DecisionMetricsCollector) RecordEvent(eventType DecisionEventType) {
	c.eventsEmittedTotal.WithLabelValues(string(eventType)).Inc()
}