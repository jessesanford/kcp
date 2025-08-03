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

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	dto "github.com/prometheus/client_model/go"

	"k8s.io/klog/v2"

	"github.com/kcp-dev/kcp/pkg/reconciler/workload/tmc"
)

// MetricsServer provides syncer-specific metrics collection and reporting
type MetricsServer struct {
	// Configuration
	syncTargetName   string
	workspaceCluster string
	
	// TMC Integration
	tmcMetrics *tmc.MetricsCollector
	
	// Syncer-specific metrics
	resourcesSyncedTotal    *prometheus.CounterVec
	syncDurationSeconds     *prometheus.HistogramVec
	syncErrorsTotal         *prometheus.CounterVec
	syncBacklogSize         *prometheus.GaugeVec
	heartbeatTotal          prometheus.Counter
	heartbeatErrorsTotal    prometheus.Counter
	connectionStatus        *prometheus.GaugeVec
	resourceControllerCount prometheus.Gauge
	syncerUptime            prometheus.Gauge
	
	// State
	started   bool
	startTime time.Time
	mu        sync.RWMutex
}

// MetricsServerOptions configures the metrics server
type MetricsServerOptions struct {
	SyncTargetName   string
	WorkspaceCluster string
	TMCMetrics       *tmc.MetricsCollector
}

// NewMetricsServer creates a new metrics server
func NewMetricsServer(options MetricsServerOptions) *MetricsServer {
	logger := klog.Background().WithValues(
		"component", "MetricsServer",
		"syncTarget", options.SyncTargetName,
	)
	logger.Info("Creating metrics server")

	ms := &MetricsServer{
		syncTargetName:   options.SyncTargetName,
		workspaceCluster: options.WorkspaceCluster,
		tmcMetrics:       options.TMCMetrics,
		startTime:        time.Now(),
	}

	ms.initializeMetrics()

	logger.Info("Successfully created metrics server")
	return ms
}

// initializeMetrics initializes all syncer-specific metrics
func (ms *MetricsServer) initializeMetrics() {
	labels := []string{"sync_target", "workspace", "gvk", "direction"}
	
	ms.resourcesSyncedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "syncer_resources_synced_total",
			Help: "Total number of resources synced by the syncer",
		},
		labels,
	)

	ms.syncDurationSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "syncer_sync_duration_seconds",
			Help:    "Time taken to sync resources",
			Buckets: []float64{.001, .01, .1, .5, 1, 2.5, 5, 10, 30},
		},
		[]string{"sync_target", "workspace", "gvk", "operation"},
	)

	ms.syncErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "syncer_sync_errors_total",
			Help: "Total number of sync errors",
		},
		[]string{"sync_target", "workspace", "gvk", "error_type"},
	)

	ms.syncBacklogSize = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "syncer_sync_backlog_size",
			Help: "Number of pending sync operations",
		},
		[]string{"sync_target", "workspace", "gvk"},
	)

	ms.heartbeatTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "syncer_heartbeat_total",
			Help: "Total number of heartbeats sent",
		},
	)

	ms.heartbeatErrorsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "syncer_heartbeat_errors_total",
			Help: "Total number of heartbeat errors",
		},
	)

	ms.connectionStatus = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "syncer_connection_status",
			Help: "Connection status to KCP and cluster (1=connected, 0=disconnected)",
		},
		[]string{"sync_target", "workspace", "target"},
	)

	ms.resourceControllerCount = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "syncer_resource_controllers",
			Help: "Number of active resource controllers",
		},
	)

	ms.syncerUptime = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "syncer_uptime_seconds",
			Help: "Syncer uptime in seconds",
		},
	)
}

// Start starts the metrics server
func (ms *MetricsServer) Start(ctx context.Context) error {
	logger := klog.FromContext(ctx).WithValues(
		"component", "MetricsServer",
		"syncTarget", ms.syncTargetName,
	)
	logger.Info("Starting metrics server")

	ms.mu.Lock()
	if ms.started {
		ms.mu.Unlock()
		return fmt.Errorf("metrics server already started")
	}
	ms.started = true
	ms.mu.Unlock()

	// Start metrics collection loop
	go ms.metricsLoop(ctx)

	logger.Info("Metrics server started successfully")
	return nil
}

// Stop stops the metrics server
func (ms *MetricsServer) Stop() {
	logger := klog.Background().WithValues(
		"component", "MetricsServer",
		"syncTarget", ms.syncTargetName,
	)
	logger.Info("Stopping metrics server")

	ms.mu.Lock()
	defer ms.mu.Unlock()

	if !ms.started {
		return
	}

	ms.started = false
	logger.Info("Metrics server stopped")
}

// metricsLoop runs the metrics collection loop
func (ms *MetricsServer) metricsLoop(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ms.updateSystemMetrics()
		}
	}
}

// updateSystemMetrics updates system-level metrics
func (ms *MetricsServer) updateSystemMetrics() {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	if !ms.started {
		return
	}

	// Update uptime
	uptime := time.Since(ms.startTime)
	ms.syncerUptime.Set(uptime.Seconds())

	// Integrate with TMC metrics
	if ms.tmcMetrics != nil {
		ms.tmcMetrics.RecordComponentUptime("syncer", ms.syncTargetName, ms.syncTargetName, uptime)
	}
}

// RecordResourceSync records a resource sync operation
func (ms *MetricsServer) RecordResourceSync(gvk, direction string, duration time.Duration, success bool) {
	labels := prometheus.Labels{
		"sync_target": ms.syncTargetName,
		"workspace":   ms.workspaceCluster,
		"gvk":         gvk,
		"direction":   direction,
	}

	ms.resourcesSyncedTotal.With(labels).Inc()

	operation := "sync"
	if !success {
		operation = "sync_error"
	}

	ms.syncDurationSeconds.WithLabelValues(
		ms.syncTargetName, ms.workspaceCluster, gvk, operation,
	).Observe(duration.Seconds())

	// Integrate with TMC metrics
	if ms.tmcMetrics != nil {
		ms.tmcMetrics.RecordSyncDuration(ms.syncTargetName, gvk, operation, duration)
		ms.tmcMetrics.RecordSyncResourceCount(ms.syncTargetName, "", gvk, "synced", 1)
	}
}

// RecordSyncError records a sync error
func (ms *MetricsServer) RecordSyncError(gvk string, errorType tmc.TMCErrorType) {
	ms.syncErrorsTotal.WithLabelValues(
		ms.syncTargetName, ms.workspaceCluster, gvk, string(errorType),
	).Inc()

	// Integrate with TMC metrics
	if ms.tmcMetrics != nil {
		ms.tmcMetrics.RecordSyncError(ms.syncTargetName, gvk, errorType)
		ms.tmcMetrics.RecordComponentError("syncer", ms.syncTargetName, errorType, tmc.TMCErrorSeverityMedium)
	}
}

// RecordBacklogSize records the current sync backlog size
func (ms *MetricsServer) RecordBacklogSize(gvk string, size int) {
	ms.syncBacklogSize.WithLabelValues(
		ms.syncTargetName, ms.workspaceCluster, gvk,
	).Set(float64(size))

	// Integrate with TMC metrics
	if ms.tmcMetrics != nil {
		ms.tmcMetrics.RecordSyncBacklogSize(ms.syncTargetName, gvk, size)
	}
}

// RecordHeartbeat records a heartbeat event
func (ms *MetricsServer) RecordHeartbeat(success bool) {
	if success {
		ms.heartbeatTotal.Inc()
	} else {
		ms.heartbeatErrorsTotal.Inc()
	}

	// Integrate with TMC metrics
	if ms.tmcMetrics != nil {
		status := "success"
		if !success {
			status = "error"
		}
		ms.tmcMetrics.RecordComponentOperation("status-reporter", ms.syncTargetName, "heartbeat", status)
	}
}

// RecordConnectionStatus records the connection status to KCP or cluster
func (ms *MetricsServer) RecordConnectionStatus(target string, connected bool) {
	value := 0.0
	if connected {
		value = 1.0
	}

	ms.connectionStatus.WithLabelValues(
		ms.syncTargetName, ms.workspaceCluster, target,
	).Set(value)

	// Integrate with TMC metrics
	if ms.tmcMetrics != nil {
		ms.tmcMetrics.RecordClusterConnectivity(ms.syncTargetName, ms.workspaceCluster, connected)
	}
}

// RecordResourceControllerCount records the number of active resource controllers
func (ms *MetricsServer) RecordResourceControllerCount(count int) {
	ms.resourceControllerCount.Set(float64(count))

	// Integrate with TMC metrics
	if ms.tmcMetrics != nil {
		ms.tmcMetrics.RecordComponentHealth("syncer-engine", ms.syncTargetName, ms.syncTargetName, tmc.HealthStatusHealthy)
	}
}

// GetMetricValue retrieves the current value of a gauge metric
func (ms *MetricsServer) GetMetricValue(metric prometheus.Gauge) float64 {
	metricDto := &dto.Metric{}
	if err := metric.Write(metricDto); err != nil {
		return 0
	}
	return metricDto.GetGauge().GetValue()
}

// GetMetricsSnapshot returns a snapshot of current metrics values
func (ms *MetricsServer) GetMetricsSnapshot() map[string]interface{} {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	snapshot := map[string]interface{}{
		"sync_target":       ms.syncTargetName,
		"workspace":         ms.workspaceCluster,
		"started":           ms.started,
		"uptime_seconds":    ms.GetMetricValue(ms.syncerUptime),
		"resource_controllers": ms.GetMetricValue(ms.resourceControllerCount),
	}

	// Add TMC metrics integration status
	if ms.tmcMetrics != nil {
		snapshot["tmc_integration"] = "enabled"
	} else {
		snapshot["tmc_integration"] = "disabled"
	}

	return snapshot
}

// SyncerMetrics provides convenience methods for recording syncer metrics
type SyncerMetrics struct {
	metricsServer *MetricsServer
	gvk           string
}

// NewSyncerMetrics creates a new syncer metrics instance for a specific GVK
func (ms *MetricsServer) NewSyncerMetrics(gvk string) *SyncerMetrics {
	return &SyncerMetrics{
		metricsServer: ms,
		gvk:           gvk,
	}
}

// RecordSync records a sync operation for this GVK
func (sm *SyncerMetrics) RecordSync(direction string, duration time.Duration, success bool) {
	sm.metricsServer.RecordResourceSync(sm.gvk, direction, duration, success)
}

// RecordError records an error for this GVK
func (sm *SyncerMetrics) RecordError(errorType tmc.TMCErrorType) {
	sm.metricsServer.RecordSyncError(sm.gvk, errorType)
}

// RecordBacklog records the backlog size for this GVK
func (sm *SyncerMetrics) RecordBacklog(size int) {
	sm.metricsServer.RecordBacklogSize(sm.gvk, size)
}

// OperationTimer helps track operation duration
type OperationTimer struct {
	syncerMetrics *SyncerMetrics
	direction     string
	startTime     time.Time
}

// NewOperationTimer creates a new operation timer
func (sm *SyncerMetrics) NewOperationTimer(direction string) *OperationTimer {
	return &OperationTimer{
		syncerMetrics: sm,
		direction:     direction,
		startTime:     time.Now(),
	}
}

// Success records a successful operation
func (ot *OperationTimer) Success() {
	duration := time.Since(ot.startTime)
	ot.syncerMetrics.RecordSync(ot.direction, duration, true)
}

// Error records a failed operation
func (ot *OperationTimer) Error(errorType tmc.TMCErrorType) {
	duration := time.Since(ot.startTime)
	ot.syncerMetrics.RecordSync(ot.direction, duration, false)
	ot.syncerMetrics.RecordError(errorType)
}