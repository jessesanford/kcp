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
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	dto "github.com/prometheus/client_model/go"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/kcp/pkg/reconciler/workload/tmc"
)

// MetricsServer provides Prometheus metrics for the syncer
type MetricsServer struct {
	port      int
	server    *http.Server
	registry  *prometheus.Registry
	tmcMetrics *tmc.MetricsCollector

	// Syncer-specific metrics
	resourcesSynced    *prometheus.CounterVec
	syncDuration       *prometheus.HistogramVec
	syncErrors         *prometheus.CounterVec
	heartbeatsSent     prometheus.Counter
	heartbeatErrors    prometheus.Counter
	syncTargetStatus   *prometheus.GaugeVec

	// Lifecycle
	started bool
	mu      sync.RWMutex
}

// NewMetricsServer creates a new metrics server
func NewMetricsServer(port int, tmcMetrics *tmc.MetricsCollector) (*MetricsServer, error) {
	ms := &MetricsServer{
		port:       port,
		registry:   prometheus.NewRegistry(),
		tmcMetrics: tmcMetrics,
	}

	// Initialize syncer-specific metrics
	ms.initializeMetrics()

	// Set up HTTP server
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(ms.registry, promhttp.HandlerOpts{}))

	ms.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	return ms, nil
}

// Start starts the metrics server
func (ms *MetricsServer) Start(ctx context.Context) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if ms.started {
		return fmt.Errorf("metrics server is already started")
	}

	klog.Infof("Starting metrics server on port %d", ms.port)

	// Start HTTP server
	go func() {
		if err := ms.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			klog.Errorf("Metrics server error: %v", err)
		}
	}()

	ms.started = true
	klog.Info("Metrics server started successfully")
	return nil
}

// Stop stops the metrics server
func (ms *MetricsServer) Stop(ctx context.Context) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if !ms.started {
		return nil
	}

	klog.Info("Stopping metrics server...")

	// Shutdown HTTP server
	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := ms.server.Shutdown(shutdownCtx); err != nil {
		klog.Errorf("Failed to shutdown metrics server: %v", err)
		return err
	}

	ms.started = false
	klog.Info("Metrics server stopped")
	return nil
}

// initializeMetrics sets up all Prometheus metrics
func (ms *MetricsServer) initializeMetrics() {
	// Resources synced counter
	ms.resourcesSynced = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "syncer_resources_synced_total",
			Help: "Total number of resources synced",
		},
		[]string{"cluster", "workspace", "gvr", "direction"},
	)

	// Sync duration histogram
	ms.syncDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "syncer_sync_duration_seconds",
			Help:    "Time taken to sync resources",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"cluster", "workspace", "gvr", "direction"},
	)

	// Sync errors counter
	ms.syncErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "syncer_sync_errors_total",
			Help: "Total number of sync errors",
		},
		[]string{"cluster", "workspace", "gvr", "error_type"},
	)

	// Heartbeats sent counter
	ms.heartbeatsSent = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "syncer_heartbeats_sent_total",
			Help: "Total number of heartbeats sent to KCP",
		},
	)

	// Heartbeat errors counter
	ms.heartbeatErrors = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "syncer_heartbeat_errors_total",
			Help: "Total number of heartbeat errors",
		},
	)

	// SyncTarget status gauge
	ms.syncTargetStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "syncer_sync_target_status",
			Help: "Status of SyncTarget (1 = healthy, 0 = unhealthy)",
		},
		[]string{"cluster", "workspace", "sync_target"},
	)

	// Register all metrics
	ms.registry.MustRegister(
		ms.resourcesSynced,
		ms.syncDuration,
		ms.syncErrors,
		ms.heartbeatsSent,
		ms.heartbeatErrors,
		ms.syncTargetStatus,
	)

	// Register TMC metrics if available
	if ms.tmcMetrics != nil {
		// TMC metrics will be registered through the TMC system
		klog.Info("TMC metrics integration enabled")
	}
}

// RecordResourceSynced records a successful resource sync
func (ms *MetricsServer) RecordResourceSynced(cluster, workspace, gvr, direction string, duration time.Duration) {
	ms.resourcesSynced.WithLabelValues(cluster, workspace, gvr, direction).Inc()
	ms.syncDuration.WithLabelValues(cluster, workspace, gvr, direction).Observe(duration.Seconds())
}

// RecordSyncError records a sync error
func (ms *MetricsServer) RecordSyncError(cluster, workspace, gvr, errorType string) {
	ms.syncErrors.WithLabelValues(cluster, workspace, gvr, errorType).Inc()
}

// RecordHeartbeatSent records a successful heartbeat
func (ms *MetricsServer) RecordHeartbeatSent() {
	ms.heartbeatsSent.Inc()
}

// RecordHeartbeatError records a heartbeat error
func (ms *MetricsServer) RecordHeartbeatError() {
	ms.heartbeatErrors.Inc()
}

// UpdateSyncTargetStatus updates the SyncTarget status metric
func (ms *MetricsServer) UpdateSyncTargetStatus(cluster, workspace, syncTarget string, healthy bool) {
	value := 0.0
	if healthy {
		value = 1.0
	}
	ms.syncTargetStatus.WithLabelValues(cluster, workspace, syncTarget).Set(value)
}

// GetMetrics returns current metric values for debugging
func (ms *MetricsServer) GetMetrics() map[string]interface{} {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	metrics := make(map[string]interface{})
	
	// Get counter values (simplified for now)
	metrics["resources_synced"] = "counter_vec" // TODO: Implement proper counter vec extraction
	metrics["sync_errors"] = "counter_vec"      // TODO: Implement proper counter vec extraction
	metrics["heartbeats_sent"] = ms.getCounterValue(ms.heartbeatsSent)
	metrics["heartbeat_errors"] = ms.getCounterValue(ms.heartbeatErrors)
	
	metrics["started"] = ms.started

	return metrics
}

// Helper methods to extract metric values
func (ms *MetricsServer) getGaugeValue(gauge prometheus.Gauge) float64 {
	metric := &dto.Metric{}
	if err := gauge.Write(metric); err != nil {
		return 0
	}
	return metric.GetGauge().GetValue()
}

func (ms *MetricsServer) getCounterValue(counter prometheus.Counter) float64 {
	metric := &dto.Metric{}
	if err := counter.Write(metric); err != nil {
		return 0
	}
	return metric.GetCounter().GetValue()
}

// Note: CounterVec doesn't have a direct Write method
// This would need to be implemented differently by gathering individual metrics
// For now, this is a placeholder
func (ms *MetricsServer) getCounterVecValue(counterVec *prometheus.CounterVec) map[string]float64 {
	// TODO: Implement proper CounterVec value extraction
	result := make(map[string]float64)
	result["total"] = 0.0 // Placeholder
	return result
}