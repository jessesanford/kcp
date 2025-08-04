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

package tmc

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"k8s.io/klog/v2"
)

// MetricsCollector collects and exposes metrics for TMC operations
type MetricsCollector struct {
	// Component metrics
	componentHealth     *prometheus.GaugeVec
	componentUptime     *prometheus.GaugeVec
	componentOperations *prometheus.CounterVec
	componentErrors     *prometheus.CounterVec
	componentLatency    *prometheus.HistogramVec

	// Placement metrics
	placementTotal        *prometheus.GaugeVec
	placementDuration     *prometheus.HistogramVec
	placementErrors       *prometheus.CounterVec
	placementRetries      *prometheus.CounterVec
	placementClusterCount *prometheus.GaugeVec

	// Sync metrics
	syncTotal       *prometheus.GaugeVec
	syncDuration    *prometheus.HistogramVec
	syncErrors      *prometheus.CounterVec
	syncBacklogSize *prometheus.GaugeVec
	syncLag         *prometheus.GaugeVec

	// Migration metrics
	migrationTotal     *prometheus.GaugeVec
	migrationDuration  *prometheus.HistogramVec
	migrationErrors    *prometheus.CounterVec
	migrationRollbacks *prometheus.CounterVec
	migrationDataSize  *prometheus.HistogramVec

	// Cluster metrics
	clusterHealth       *prometheus.GaugeVec
	clusterConnectivity *prometheus.GaugeVec
	clusterCapacity     *prometheus.GaugeVec
	clusterResources    *prometheus.GaugeVec

	// Resource metrics
	resourceAggregations    *prometheus.CounterVec
	resourceProjections     *prometheus.CounterVec
	resourceConflicts       *prometheus.CounterVec
	resourceTransformations *prometheus.CounterVec

	// Virtual workspace metrics
	virtualWorkspaces       *prometheus.GaugeVec
	virtualWorkspaceLatency *prometheus.HistogramVec
	virtualWorkspaceErrors  *prometheus.CounterVec

	// Recovery metrics
	recoveryAttempts  *prometheus.CounterVec
	recoverySuccesses *prometheus.CounterVec
	recoveryFailures  *prometheus.CounterVec
	recoveryDuration  *prometheus.HistogramVec

	// System metrics
	goRoutines         prometheus.Gauge
	memoryUsage        prometheus.Gauge
	cpuUsage           prometheus.Gauge
	apiRequestDuration *prometheus.HistogramVec
	apiRequestTotal    *prometheus.CounterVec

	// Custom metrics storage
	customMetrics map[string]prometheus.Metric
	mu            sync.RWMutex
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() *MetricsCollector {
	mc := &MetricsCollector{
		customMetrics: make(map[string]prometheus.Metric),
	}

	mc.initializeMetrics()
	return mc
}

func (mc *MetricsCollector) initializeMetrics() {
	// Component metrics
	mc.componentHealth = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "tmc_component_health",
			Help: "Health status of TMC components (1=healthy, 0.5=degraded, 0=unhealthy)",
		},
		[]string{"component_type", "component_id", "cluster"},
	)

	mc.componentUptime = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "tmc_component_uptime_seconds",
			Help: "Uptime of TMC components in seconds",
		},
		[]string{"component_type", "component_id", "cluster"},
	)

	mc.componentOperations = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tmc_component_operations_total",
			Help: "Total number of operations performed by TMC components",
		},
		[]string{"component_type", "component_id", "operation", "status"},
	)

	mc.componentErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tmc_component_errors_total",
			Help: "Total number of errors in TMC components",
		},
		[]string{"component_type", "component_id", "error_type", "severity"},
	)

	mc.componentLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "tmc_component_operation_duration_seconds",
			Help:    "Duration of TMC component operations",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"component_type", "component_id", "operation"},
	)

	// Placement metrics
	mc.placementTotal = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "tmc_placements_total",
			Help: "Total number of active placements",
		},
		[]string{"logical_cluster", "namespace", "status"},
	)

	mc.placementDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "tmc_placement_duration_seconds",
			Help:    "Duration of placement operations",
			Buckets: []float64{.1, .25, .5, 1, 2.5, 5, 10, 30, 60, 300},
		},
		[]string{"logical_cluster", "operation", "status"},
	)

	mc.placementErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tmc_placement_errors_total",
			Help: "Total number of placement errors",
		},
		[]string{"logical_cluster", "error_type", "cluster"},
	)

	mc.placementRetries = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tmc_placement_retries_total",
			Help: "Total number of placement retries",
		},
		[]string{"logical_cluster", "reason"},
	)

	mc.placementClusterCount = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "tmc_placement_clusters",
			Help: "Number of clusters involved in each placement",
		},
		[]string{"logical_cluster", "namespace", "placement"},
	)

	// Sync metrics
	mc.syncTotal = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "tmc_sync_resources_total",
			Help: "Total number of resources being synced",
		},
		[]string{"cluster", "namespace", "gvk", "status"},
	)

	mc.syncDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "tmc_sync_duration_seconds",
			Help:    "Duration of sync operations",
			Buckets: []float64{.1, .25, .5, 1, 2.5, 5, 10, 30},
		},
		[]string{"cluster", "gvk", "operation"},
	)

	mc.syncErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tmc_sync_errors_total",
			Help: "Total number of sync errors",
		},
		[]string{"cluster", "gvk", "error_type"},
	)

	mc.syncBacklogSize = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "tmc_sync_backlog_size",
			Help: "Number of pending sync operations",
		},
		[]string{"cluster", "queue"},
	)

	mc.syncLag = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "tmc_sync_lag_seconds",
			Help: "Lag between resource changes and sync",
		},
		[]string{"cluster", "gvk"},
	)

	// Migration metrics
	mc.migrationTotal = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "tmc_migrations_total",
			Help: "Total number of active migrations",
		},
		[]string{"source_cluster", "target_cluster", "status"},
	)

	mc.migrationDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "tmc_migration_duration_seconds",
			Help:    "Duration of migration operations",
			Buckets: []float64{1, 5, 10, 30, 60, 300, 600, 1800, 3600},
		},
		[]string{"source_cluster", "target_cluster", "strategy"},
	)

	mc.migrationErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tmc_migration_errors_total",
			Help: "Total number of migration errors",
		},
		[]string{"source_cluster", "target_cluster", "error_type", "phase"},
	)

	mc.migrationRollbacks = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tmc_migration_rollbacks_total",
			Help: "Total number of migration rollbacks",
		},
		[]string{"source_cluster", "target_cluster", "reason"},
	)

	mc.migrationDataSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "tmc_migration_data_size_bytes",
			Help:    "Size of data migrated",
			Buckets: []float64{1024, 10240, 102400, 1048576, 10485760, 104857600, 1073741824},
		},
		[]string{"source_cluster", "target_cluster", "resource_type"},
	)

	// Cluster metrics
	mc.clusterHealth = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "tmc_cluster_health",
			Help: "Health status of clusters (1=healthy, 0.5=degraded, 0=unhealthy)",
		},
		[]string{"cluster", "logical_cluster"},
	)

	mc.clusterConnectivity = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "tmc_cluster_connectivity",
			Help: "Connectivity status of clusters (1=connected, 0=disconnected)",
		},
		[]string{"cluster", "logical_cluster"},
	)

	mc.clusterCapacity = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "tmc_cluster_capacity_utilization",
			Help: "Capacity utilization of clusters (0-1)",
		},
		[]string{"cluster", "resource_type"},
	)

	mc.clusterResources = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "tmc_cluster_resources_total",
			Help: "Total number of resources in each cluster",
		},
		[]string{"cluster", "gvk", "namespace"},
	)

	// Resource metrics
	mc.resourceAggregations = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tmc_resource_aggregations_total",
			Help: "Total number of resource aggregations performed",
		},
		[]string{"gvk", "strategy", "status"},
	)

	mc.resourceProjections = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tmc_resource_projections_total",
			Help: "Total number of resource projections performed",
		},
		[]string{"source_cluster", "target_cluster", "gvk", "status"},
	)

	mc.resourceConflicts = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tmc_resource_conflicts_total",
			Help: "Total number of resource conflicts encountered",
		},
		[]string{"cluster", "gvk", "conflict_type", "resolution"},
	)

	mc.resourceTransformations = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tmc_resource_transformations_total",
			Help: "Total number of resource transformations applied",
		},
		[]string{"transformation_type", "gvk", "status"},
	)

	// Virtual workspace metrics
	mc.virtualWorkspaces = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "tmc_virtual_workspaces_total",
			Help: "Total number of active virtual workspaces",
		},
		[]string{"logical_cluster", "status"},
	)

	mc.virtualWorkspaceLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "tmc_virtual_workspace_operation_duration_seconds",
			Help:    "Duration of virtual workspace operations",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"operation", "workspace"},
	)

	mc.virtualWorkspaceErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tmc_virtual_workspace_errors_total",
			Help: "Total number of virtual workspace errors",
		},
		[]string{"workspace", "error_type", "component"},
	)

	// Recovery metrics
	mc.recoveryAttempts = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tmc_recovery_attempts_total",
			Help: "Total number of recovery attempts",
		},
		[]string{"error_type", "strategy", "component"},
	)

	mc.recoverySuccesses = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tmc_recovery_successes_total",
			Help: "Total number of successful recoveries",
		},
		[]string{"error_type", "strategy", "component"},
	)

	mc.recoveryFailures = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tmc_recovery_failures_total",
			Help: "Total number of failed recoveries",
		},
		[]string{"error_type", "strategy", "component"},
	)

	mc.recoveryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "tmc_recovery_duration_seconds",
			Help:    "Duration of recovery operations",
			Buckets: []float64{1, 5, 10, 30, 60, 300, 600},
		},
		[]string{"error_type", "strategy"},
	)

	// System metrics
	mc.goRoutines = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "tmc_goroutines_total",
			Help: "Number of goroutines in the TMC system",
		},
	)

	mc.memoryUsage = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "tmc_memory_usage_bytes",
			Help: "Memory usage of the TMC system",
		},
	)

	mc.cpuUsage = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "tmc_cpu_usage_percent",
			Help: "CPU usage percentage of the TMC system",
		},
	)

	mc.apiRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "tmc_api_request_duration_seconds",
			Help:    "Duration of API requests",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint", "status_code"},
	)

	mc.apiRequestTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tmc_api_requests_total",
			Help: "Total number of API requests",
		},
		[]string{"method", "endpoint", "status_code"},
	)
}

// Component health tracking
func (mc *MetricsCollector) RecordComponentHealth(componentType, componentID, cluster string, health HealthStatus) {
	var value float64
	switch health {
	case HealthStatusHealthy:
		value = 1.0
	case HealthStatusDegraded:
		value = 0.5
	case HealthStatusUnhealthy:
		value = 0.0
	default:
		value = -1.0 // Unknown
	}
	mc.componentHealth.WithLabelValues(componentType, componentID, cluster).Set(value)
}

func (mc *MetricsCollector) RecordComponentUptime(componentType, componentID, cluster string, uptime time.Duration) {
	mc.componentUptime.WithLabelValues(componentType, componentID, cluster).Set(uptime.Seconds())
}

func (mc *MetricsCollector) RecordComponentOperation(componentType, componentID, operation, status string) {
	mc.componentOperations.WithLabelValues(componentType, componentID, operation, status).Inc()
}

func (mc *MetricsCollector) RecordComponentError(componentType, componentID string, errorType TMCErrorType, severity TMCErrorSeverity) {
	mc.componentErrors.WithLabelValues(componentType, componentID, string(errorType), string(severity)).Inc()
}

func (mc *MetricsCollector) RecordComponentLatency(componentType, componentID, operation string, duration time.Duration) {
	mc.componentLatency.WithLabelValues(componentType, componentID, operation).Observe(duration.Seconds())
}

// Placement metrics
func (mc *MetricsCollector) RecordPlacementCount(logicalCluster, namespace, status string, count int) {
	mc.placementTotal.WithLabelValues(logicalCluster, namespace, status).Set(float64(count))
}

func (mc *MetricsCollector) RecordPlacementDuration(logicalCluster, operation, status string, duration time.Duration) {
	mc.placementDuration.WithLabelValues(logicalCluster, operation, status).Observe(duration.Seconds())
}

func (mc *MetricsCollector) RecordPlacementError(logicalCluster string, errorType TMCErrorType, cluster string) {
	mc.placementErrors.WithLabelValues(logicalCluster, string(errorType), cluster).Inc()
}

func (mc *MetricsCollector) RecordPlacementRetry(logicalCluster, reason string) {
	mc.placementRetries.WithLabelValues(logicalCluster, reason).Inc()
}

func (mc *MetricsCollector) RecordPlacementClusterCount(logicalCluster, namespace, placement string, count int) {
	mc.placementClusterCount.WithLabelValues(logicalCluster, namespace, placement).Set(float64(count))
}

// Sync metrics
func (mc *MetricsCollector) RecordSyncResourceCount(cluster, namespace, gvk, status string, count int) {
	mc.syncTotal.WithLabelValues(cluster, namespace, gvk, status).Set(float64(count))
}

func (mc *MetricsCollector) RecordSyncDuration(cluster, gvk, operation string, duration time.Duration) {
	mc.syncDuration.WithLabelValues(cluster, gvk, operation).Observe(duration.Seconds())
}

func (mc *MetricsCollector) RecordSyncError(cluster, gvk string, errorType TMCErrorType) {
	mc.syncErrors.WithLabelValues(cluster, gvk, string(errorType)).Inc()
}

func (mc *MetricsCollector) RecordSyncBacklogSize(cluster, queue string, size int) {
	mc.syncBacklogSize.WithLabelValues(cluster, queue).Set(float64(size))
}

func (mc *MetricsCollector) RecordSyncLag(cluster, gvk string, lag time.Duration) {
	mc.syncLag.WithLabelValues(cluster, gvk).Set(lag.Seconds())
}

// Migration metrics
func (mc *MetricsCollector) RecordMigrationCount(sourceCluster, targetCluster, status string, count int) {
	mc.migrationTotal.WithLabelValues(sourceCluster, targetCluster, status).Set(float64(count))
}

func (mc *MetricsCollector) RecordMigrationDuration(sourceCluster, targetCluster, strategy string, duration time.Duration) {
	mc.migrationDuration.WithLabelValues(sourceCluster, targetCluster, strategy).Observe(duration.Seconds())
}

func (mc *MetricsCollector) RecordMigrationError(sourceCluster, targetCluster string, errorType TMCErrorType, phase string) {
	mc.migrationErrors.WithLabelValues(sourceCluster, targetCluster, string(errorType), phase).Inc()
}

func (mc *MetricsCollector) RecordMigrationRollback(sourceCluster, targetCluster, reason string) {
	mc.migrationRollbacks.WithLabelValues(sourceCluster, targetCluster, reason).Inc()
}

func (mc *MetricsCollector) RecordMigrationDataSize(sourceCluster, targetCluster, resourceType string, size int64) {
	mc.migrationDataSize.WithLabelValues(sourceCluster, targetCluster, resourceType).Observe(float64(size))
}

// Cluster metrics
func (mc *MetricsCollector) RecordClusterHealth(cluster, logicalCluster string, health HealthStatus) {
	var value float64
	switch health {
	case HealthStatusHealthy:
		value = 1.0
	case HealthStatusDegraded:
		value = 0.5
	case HealthStatusUnhealthy:
		value = 0.0
	default:
		value = -1.0
	}
	mc.clusterHealth.WithLabelValues(cluster, logicalCluster).Set(value)
}

func (mc *MetricsCollector) RecordClusterConnectivity(cluster, logicalCluster string, connected bool) {
	value := 0.0
	if connected {
		value = 1.0
	}
	mc.clusterConnectivity.WithLabelValues(cluster, logicalCluster).Set(value)
}

func (mc *MetricsCollector) RecordClusterCapacity(cluster, resourceType string, utilization float64) {
	mc.clusterCapacity.WithLabelValues(cluster, resourceType).Set(utilization)
}

func (mc *MetricsCollector) RecordClusterResourceCount(cluster, gvk, namespace string, count int) {
	mc.clusterResources.WithLabelValues(cluster, gvk, namespace).Set(float64(count))
}

// Resource operation metrics
func (mc *MetricsCollector) RecordResourceAggregation(gvk, strategy, status string) {
	mc.resourceAggregations.WithLabelValues(gvk, strategy, status).Inc()
}

func (mc *MetricsCollector) RecordResourceProjection(sourceCluster, targetCluster, gvk, status string) {
	mc.resourceProjections.WithLabelValues(sourceCluster, targetCluster, gvk, status).Inc()
}

func (mc *MetricsCollector) RecordResourceConflict(cluster, gvk, conflictType, resolution string) {
	mc.resourceConflicts.WithLabelValues(cluster, gvk, conflictType, resolution).Inc()
}

func (mc *MetricsCollector) RecordResourceTransformation(transformationType, gvk, status string) {
	mc.resourceTransformations.WithLabelValues(transformationType, gvk, status).Inc()
}

// Virtual workspace metrics
func (mc *MetricsCollector) RecordVirtualWorkspaceCount(logicalCluster, status string, count int) {
	mc.virtualWorkspaces.WithLabelValues(logicalCluster, status).Set(float64(count))
}

func (mc *MetricsCollector) RecordVirtualWorkspaceLatency(operation, workspace string, duration time.Duration) {
	mc.virtualWorkspaceLatency.WithLabelValues(operation, workspace).Observe(duration.Seconds())
}

func (mc *MetricsCollector) RecordVirtualWorkspaceError(workspace string, errorType TMCErrorType, component string) {
	mc.virtualWorkspaceErrors.WithLabelValues(workspace, string(errorType), component).Inc()
}

// Recovery metrics
func (mc *MetricsCollector) RecordRecoveryAttempt(errorType TMCErrorType, strategy, component string) {
	mc.recoveryAttempts.WithLabelValues(string(errorType), strategy, component).Inc()
}

func (mc *MetricsCollector) RecordRecoverySuccess(errorType TMCErrorType, strategy, component string) {
	mc.recoverySuccesses.WithLabelValues(string(errorType), strategy, component).Inc()
}

func (mc *MetricsCollector) RecordRecoveryFailure(errorType TMCErrorType, strategy, component string) {
	mc.recoveryFailures.WithLabelValues(string(errorType), strategy, component).Inc()
}

func (mc *MetricsCollector) RecordRecoveryDuration(errorType TMCErrorType, strategy string, duration time.Duration) {
	mc.recoveryDuration.WithLabelValues(string(errorType), strategy).Observe(duration.Seconds())
}

// System metrics
func (mc *MetricsCollector) RecordGoRoutines(count int) {
	mc.goRoutines.Set(float64(count))
}

func (mc *MetricsCollector) RecordMemoryUsage(bytes int64) {
	mc.memoryUsage.Set(float64(bytes))
}

func (mc *MetricsCollector) RecordCPUUsage(percentage float64) {
	mc.cpuUsage.Set(percentage)
}

func (mc *MetricsCollector) RecordAPIRequest(method, endpoint, statusCode string, duration time.Duration) {
	mc.apiRequestTotal.WithLabelValues(method, endpoint, statusCode).Inc()
	mc.apiRequestDuration.WithLabelValues(method, endpoint, statusCode).Observe(duration.Seconds())
}

// Custom metrics support
func (mc *MetricsCollector) RegisterCustomMetric(name string, metric prometheus.Metric) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if _, exists := mc.customMetrics[name]; exists {
		return fmt.Errorf("metric %s already registered", name)
	}

	mc.customMetrics[name] = metric
	return nil
}

func (mc *MetricsCollector) GetCustomMetric(name string) (prometheus.Metric, bool) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	metric, exists := mc.customMetrics[name]
	return metric, exists
}

// MetricsReporter provides high-level metrics reporting functionality
type MetricsReporter struct {
	collector     *MetricsCollector
	healthMonitor *HealthMonitor
	mu            sync.RWMutex
}

// NewMetricsReporter creates a new metrics reporter
func NewMetricsReporter(collector *MetricsCollector, healthMonitor *HealthMonitor) *MetricsReporter {
	return &MetricsReporter{
		collector:     collector,
		healthMonitor: healthMonitor,
	}
}

// StartMetricsCollection starts collecting system-wide metrics
func (mr *MetricsReporter) StartMetricsCollection(ctx context.Context) {
	logger := klog.FromContext(ctx).WithValues("component", "MetricsReporter")
	logger.Info("Starting metrics collection")

	// Start system metrics collection
	go mr.collectSystemMetrics(ctx)

	// Start health metrics collection
	go mr.collectHealthMetrics(ctx)
}

func (mr *MetricsReporter) collectSystemMetrics(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Collect system metrics
			// In a real implementation, this would gather actual system metrics
			// For now, we'll use placeholder values

			// Record goroutine count
			mr.collector.RecordGoRoutines(100) // Placeholder

			// Record memory usage
			mr.collector.RecordMemoryUsage(1024 * 1024 * 100) // 100MB placeholder

			// Record CPU usage
			mr.collector.RecordCPUUsage(25.5) // 25.5% placeholder
		}
	}
}

func (mr *MetricsReporter) collectHealthMetrics(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if mr.healthMonitor != nil {
				// Collect health metrics from health monitor
				allHealth := mr.healthMonitor.GetAllComponentHealth()
				for _, health := range allHealth {
					cluster := "unknown"
					if health.Details != nil {
						if clusterName, exists := health.Details["clusterName"]; exists {
							if clusterStr, ok := clusterName.(string); ok {
								cluster = clusterStr
							}
						}
					}

					mr.collector.RecordComponentHealth(
						string(health.ComponentType),
						health.ComponentID,
						cluster,
						health.Status,
					)
				}
			}
		}
	}
}

// GetMetricsSummary returns a summary of current metrics
func (mr *MetricsReporter) GetMetricsSummary() map[string]interface{} {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	summary := make(map[string]interface{})

	// Add health metrics if available
	if mr.healthMonitor != nil {
		overallHealth := mr.healthMonitor.GetOverallHealth()
		summary["overall_health"] = map[string]interface{}{
			"status":  overallHealth.Status,
			"message": overallHealth.Message,
			"details": overallHealth.Details,
		}

		healthMetrics := mr.healthMonitor.HealthMetrics()
		summary["health_metrics"] = healthMetrics
	}

	// Add timestamp
	summary["timestamp"] = time.Now().Unix()
	summary["collection_active"] = true

	return summary
}

// OperationTracker helps track the duration and status of operations
type OperationTracker struct {
	collector     *MetricsCollector
	componentType string
	componentID   string
	operation     string
	startTime     time.Time
}

// NewOperationTracker creates a new operation tracker
func (mc *MetricsCollector) NewOperationTracker(componentType, componentID, operation string) *OperationTracker {
	return &OperationTracker{
		collector:     mc,
		componentType: componentType,
		componentID:   componentID,
		operation:     operation,
		startTime:     time.Now(),
	}
}

// Success marks the operation as successful and records metrics
func (ot *OperationTracker) Success() {
	duration := time.Since(ot.startTime)
	ot.collector.RecordComponentOperation(ot.componentType, ot.componentID, ot.operation, "success")
	ot.collector.RecordComponentLatency(ot.componentType, ot.componentID, ot.operation, duration)
}

// Error marks the operation as failed and records metrics
func (ot *OperationTracker) Error(err *TMCError) {
	duration := time.Since(ot.startTime)
	ot.collector.RecordComponentOperation(ot.componentType, ot.componentID, ot.operation, "error")
	ot.collector.RecordComponentLatency(ot.componentType, ot.componentID, ot.operation, duration)
	ot.collector.RecordComponentError(ot.componentType, ot.componentID, err.Type, err.Severity)
}

// Timeout marks the operation as timed out and records metrics
func (ot *OperationTracker) Timeout() {
	duration := time.Since(ot.startTime)
	ot.collector.RecordComponentOperation(ot.componentType, ot.componentID, ot.operation, "timeout")
	ot.collector.RecordComponentLatency(ot.componentType, ot.componentID, ot.operation, duration)
}
