# TMC Metrics & Observability

The TMC Metrics & Observability system provides comprehensive monitoring, metrics collection, and operational visibility across all TMC components and multi-cluster operations. This system enables proactive monitoring, performance optimization, and operational insights.

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                 TMC Metrics & Observability System             │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐ │
│  │ Metrics         │  │ Operation       │  │ Performance     │ │
│  │ Collector       │  │ Tracker         │  │ Monitor         │ │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘ │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐ │
│  │ Prometheus      │  │ Custom Metrics  │  │ System          │ │
│  │ Integration     │  │ Registry        │  │ Monitoring      │ │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

## Metrics Categories

### Component Metrics
- **Component Health**: Health status and uptime tracking
- **Component Operations**: Operation counts and success rates
- **Component Errors**: Error tracking with categorization
- **Component Latency**: Operation duration and performance

### Workload Management Metrics
- **Placement Metrics**: Workload placement operations and constraints
- **Sync Metrics**: Resource synchronization across clusters
- **Migration Metrics**: Workload migration operations and data transfer
- **Recovery Metrics**: Automated recovery attempts and outcomes

### Cluster Metrics
- **Cluster Health**: Overall cluster health and connectivity
- **Cluster Capacity**: Resource utilization and availability
- **Cluster Resources**: Resource counts and distribution

### System Metrics
- **Performance**: Memory, CPU, and goroutine monitoring
- **API Metrics**: Request rates, latency, and error rates
- **Queue Metrics**: Work queue sizes and processing rates

## MetricsCollector Implementation

### Core Metrics Structure

```go
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

    // System metrics
    goRoutines         prometheus.Gauge
    memoryUsage        prometheus.Gauge
    cpuUsage           prometheus.Gauge
    apiRequestDuration *prometheus.HistogramVec
    apiRequestTotal    *prometheus.CounterVec
}
```

### Creating Metrics Collector

```go
// Initialize metrics collector
metricsCollector := NewMetricsCollector()

// Start metrics collection
ctx := context.Background()
metricsReporter := NewMetricsReporter(metricsCollector, healthMonitor)
metricsReporter.StartMetricsCollection(ctx)
```

## Component Metrics

### Health Tracking

```go
// Record component health status
metricsCollector.RecordComponentHealth(
    "SyncTargetController",  // Component type
    "syncer-prod-cluster",   // Component ID
    "prod-cluster",          // Cluster
    HealthStatusHealthy,     // Health status
)

// Prometheus metric:
// tmc_component_health{component_type="SyncTargetController", component_id="syncer-prod-cluster", cluster="prod-cluster"} 1.0
```

### Operation Tracking

```go
// Record component operations
metricsCollector.RecordComponentOperation(
    "SyncTargetController",
    "syncer-prod-cluster", 
    "sync-resource",
    "success",
)

// Prometheus metric:
// tmc_component_operations_total{component_type="SyncTargetController", component_id="syncer-prod-cluster", operation="sync-resource", status="success"} 125
```

### Error Tracking

```go
// Record component errors
metricsCollector.RecordComponentError(
    "PlacementController",
    "placement-controller-1",
    TMCErrorTypeClusterUnreachable,
    TMCErrorSeverityHigh,
)

// Prometheus metric:
// tmc_component_errors_total{component_type="PlacementController", component_id="placement-controller-1", error_type="ClusterUnreachable", severity="High"} 3
```

### Latency Tracking

```go
// Track operation duration
start := time.Now()
// ... perform operation ...
duration := time.Since(start)

metricsCollector.RecordComponentLatency(
    "VirtualWorkspaceManager",
    "workspace-manager",
    "aggregate-resources",
    duration,
)

// Prometheus metric:
// tmc_component_operation_duration_seconds{component_type="VirtualWorkspaceManager", component_id="workspace-manager", operation="aggregate-resources"} 0.245
```

## Operation Tracking

### Operation Tracker Usage

```go
// Create operation tracker
tracker := metricsCollector.NewOperationTracker(
    "SyncTargetController",
    "syncer-cluster-1", 
    "sync-deployment",
)

// Execute operation
err := syncDeployment(deployment)
if err != nil {
    // Record error with TMC error context
    tmcError := ConvertKubernetesError(err, "syncer", "sync-deployment")
    tracker.Error(tmcError)
} else {
    // Record success
    tracker.Success()
}

// Automatically records:
// - Operation count (success/error)
// - Operation duration
// - Error details if applicable
```

### Automatic Latency Tracking

```go
// Function wrapper for automatic tracking
func TrackOperation[T any](
    collector *MetricsCollector,
    componentType, componentID, operation string,
    fn func() (T, error),
) (T, error) {
    tracker := collector.NewOperationTracker(componentType, componentID, operation)
    
    result, err := fn()
    if err != nil {
        if tmcError, ok := err.(*TMCError); ok {
            tracker.Error(tmcError)
        } else {
            tracker.Error(ConvertKubernetesError(err, componentType, operation))
        }
    } else {
        tracker.Success()
    }
    
    return result, err
}

// Usage
deployment, err := TrackOperation(metricsCollector, "syncer", "cluster-1", "get-deployment", func() (*appsv1.Deployment, error) {
    return k8sClient.AppsV1().Deployments("default").Get(ctx, "my-app", metav1.GetOptions{})
})
```

## Workload Management Metrics

### Placement Metrics

```go
// Record placement operations
metricsCollector.RecordPlacementCount("root:production", "default", "scheduled", 15)
metricsCollector.RecordPlacementDuration("root:production", "schedule", "success", 2*time.Second)
metricsCollector.RecordPlacementError("root:production", TMCErrorTypePlacementCapacity, "cluster-1")
metricsCollector.RecordPlacementClusterCount("root:production", "default", "web-app", 3)

// Prometheus metrics:
// tmc_placements_total{logical_cluster="root:production", namespace="default", status="scheduled"} 15
// tmc_placement_duration_seconds{logical_cluster="root:production", operation="schedule", status="success"} 2.0
// tmc_placement_errors_total{logical_cluster="root:production", error_type="PlacementCapacity", cluster="cluster-1"} 1
// tmc_placement_clusters{logical_cluster="root:production", namespace="default", placement="web-app"} 3
```

### Sync Metrics

```go
// Record synchronization metrics
metricsCollector.RecordSyncResourceCount("prod-cluster", "default", "apps/v1/Deployment", "synced", 50)
metricsCollector.RecordSyncDuration("prod-cluster", "apps/v1/Deployment", "sync", 1500*time.Millisecond)
metricsCollector.RecordSyncError("prod-cluster", "apps/v1/Service", TMCErrorTypeSyncFailure)
metricsCollector.RecordSyncBacklogSize("prod-cluster", "deployment-queue", 25)
metricsCollector.RecordSyncLag("prod-cluster", "apps/v1/Deployment", 30*time.Second)

// Prometheus metrics:
// tmc_sync_resources_total{cluster="prod-cluster", namespace="default", gvk="apps/v1/Deployment", status="synced"} 50
// tmc_sync_duration_seconds{cluster="prod-cluster", gvk="apps/v1/Deployment", operation="sync"} 1.5
// tmc_sync_errors_total{cluster="prod-cluster", gvk="apps/v1/Service", error_type="SyncFailure"} 1
// tmc_sync_backlog_size{cluster="prod-cluster", queue="deployment-queue"} 25
// tmc_sync_lag_seconds{cluster="prod-cluster", gvk="apps/v1/Deployment"} 30
```

### Migration Metrics

```go
// Record migration operations
metricsCollector.RecordMigrationCount("cluster-a", "cluster-b", "in-progress", 2)
metricsCollector.RecordMigrationDuration("cluster-a", "cluster-b", "live-migration", 10*time.Minute)
metricsCollector.RecordMigrationError("cluster-a", "cluster-b", TMCErrorTypeMigrationFailure, "data-transfer")
metricsCollector.RecordMigrationDataSize("cluster-a", "cluster-b", "PersistentVolume", 5*1024*1024*1024) // 5GB

// Prometheus metrics:
// tmc_migrations_total{source_cluster="cluster-a", target_cluster="cluster-b", status="in-progress"} 2
// tmc_migration_duration_seconds{source_cluster="cluster-a", target_cluster="cluster-b", strategy="live-migration"} 600
// tmc_migration_errors_total{source_cluster="cluster-a", target_cluster="cluster-b", error_type="MigrationFailure", phase="data-transfer"} 1
// tmc_migration_data_size_bytes{source_cluster="cluster-a", target_cluster="cluster-b", resource_type="PersistentVolume"} 5368709120
```

## Virtual Workspace Metrics

### Aggregation Metrics

```go
// Record resource aggregation
metricsCollector.RecordResourceAggregation("apps/v1/Deployment", "union", "success")
metricsCollector.RecordResourceProjection("cluster-1", "cluster-2", "v1/Service", "active")
metricsCollector.RecordResourceConflict("cluster-1", "apps/v1/StatefulSet", "version-conflict", "last-writer-wins")

// Virtual workspace counts
metricsCollector.RecordVirtualWorkspaceCount("root:production", "active", 15)
metricsCollector.RecordVirtualWorkspaceLatency("aggregate", "prod-workspace", 500*time.Millisecond)
```

## Cluster Health Metrics

### Connectivity and Health

```go
// Record cluster health and connectivity
metricsCollector.RecordClusterHealth("prod-cluster-1", "root:production", HealthStatusHealthy)
metricsCollector.RecordClusterConnectivity("prod-cluster-1", "root:production", true)
metricsCollector.RecordClusterCapacity("prod-cluster-1", "cpu", 0.75) // 75% utilized
metricsCollector.RecordClusterResourceCount("prod-cluster-1", "apps/v1/Deployment", "default", 25)

// Prometheus metrics:
// tmc_cluster_health{cluster="prod-cluster-1", logical_cluster="root:production"} 1.0
// tmc_cluster_connectivity{cluster="prod-cluster-1", logical_cluster="root:production"} 1.0
// tmc_cluster_capacity_utilization{cluster="prod-cluster-1", resource_type="cpu"} 0.75
// tmc_cluster_resources_total{cluster="prod-cluster-1", gvk="apps/v1/Deployment", namespace="default"} 25
```

## System Performance Metrics

### Resource Usage

```go
// System metrics automatically collected
metricsCollector.RecordGoRoutines(150)
metricsCollector.RecordMemoryUsage(256 * 1024 * 1024) // 256MB
metricsCollector.RecordCPUUsage(25.5) // 25.5%

// API request metrics
metricsCollector.RecordAPIRequest("GET", "/api/v1/placements", "200", 150*time.Millisecond)

// Prometheus metrics:
// tmc_goroutines_total 150
// tmc_memory_usage_bytes 268435456
// tmc_cpu_usage_percent 25.5
// tmc_api_requests_total{method="GET", endpoint="/api/v1/placements", status_code="200"} 1
// tmc_api_request_duration_seconds{method="GET", endpoint="/api/v1/placements", status_code="200"} 0.15
```

## Custom Metrics

### Registering Custom Metrics

```go
// Create custom metric
customCounter := prometheus.NewCounterVec(
    prometheus.CounterOpts{
        Name: "tmc_custom_operations_total",
        Help: "Total number of custom operations",
    },
    []string{"operation_type", "cluster"},
)

// Register with metrics collector
err := metricsCollector.RegisterCustomMetric("custom_operations", customCounter)
if err != nil {
    log.Error(err, "Failed to register custom metric")
}

// Use custom metric
customCounter.WithLabelValues("special-sync", "prod-cluster").Inc()
```

### Retrieving Custom Metrics

```go
// Get custom metric
metric, exists := metricsCollector.GetCustomMetric("custom_operations")
if exists {
    if counter, ok := metric.(*prometheus.CounterVec); ok {
        counter.WithLabelValues("batch-operation", "staging-cluster").Add(5)
    }
}
```

## Prometheus Integration

### Metrics Endpoint Configuration

```go
// Metrics server configuration
metricsConfig := MetricsConfig{
    Enabled:            true,
    PrometheusEnabled:  true,
    PrometheusPort:     8080,
    PrometheusPath:     "/metrics",
    CollectionInterval: 30 * time.Second,
    RetentionPeriod:    24 * time.Hour,
}

// HTTP metrics endpoint automatically exposed at :8080/metrics
```

### Prometheus Scrape Configuration

```yaml
# prometheus.yml
scrape_configs:
- job_name: 'tmc-components'
  static_configs:
  - targets: ['kcp-server:8080']
  scrape_interval: 30s
  metrics_path: /metrics
  scheme: http
```

### Common Prometheus Queries

```promql
# Component health overview
avg(tmc_component_health) by (component_type)

# Error rate by component
rate(tmc_component_errors_total[5m]) by (component_type, error_type)

# Sync operation success rate
(
  rate(tmc_component_operations_total{operation="sync-resource", status="success"}[5m]) / 
  rate(tmc_component_operations_total{operation="sync-resource"}[5m])
) * 100

# Placement latency 95th percentile
histogram_quantile(0.95, rate(tmc_placement_duration_seconds_bucket[5m]))

# Cluster connectivity status
tmc_cluster_connectivity

# Resource sync lag
avg(tmc_sync_lag_seconds) by (cluster, gvk)

# Migration success rate
(
  rate(tmc_migrations_total{status="completed"}[10m]) /
  rate(tmc_migrations_total[10m])
) * 100

# Active recovery operations
tmc_recovery_attempts_total - tmc_recovery_successes_total - tmc_recovery_failures_total
```

## Alerting Rules

### Prometheus Alerting

```yaml
# TMC alerting rules
groups:
- name: tmc.alerts
  rules:
  # Component health alerts
  - alert: TMCComponentUnhealthy
    expr: tmc_component_health < 0.5
    for: 2m
    labels:
      severity: critical
    annotations:
      summary: "TMC component {{ $labels.component_type }} is unhealthy"
      description: "Component {{ $labels.component_id }} in cluster {{ $labels.cluster }} has been unhealthy for more than 2 minutes"

  # High error rate alert
  - alert: TMCHighErrorRate
    expr: (rate(tmc_component_errors_total[5m]) / rate(tmc_component_operations_total[5m])) > 0.1
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "High error rate in TMC component {{ $labels.component_type }}"
      description: "Error rate is {{ $value | humanizePercentage }} in component {{ $labels.component_id }}"

  # Sync lag alert
  - alert: TMCSyncLagHigh
    expr: tmc_sync_lag_seconds > 300
    for: 10m
    labels:
      severity: warning
    annotations:
      summary: "High sync lag detected"
      description: "Sync lag for {{ $labels.gvk }} in cluster {{ $labels.cluster }} is {{ $value }} seconds"

  # Cluster connectivity alert
  - alert: TMCClusterDisconnected
    expr: tmc_cluster_connectivity == 0
    for: 1m
    labels:
      severity: critical
    annotations:
      summary: "TMC cluster {{ $labels.cluster }} is disconnected"
      description: "Cluster {{ $labels.cluster }} has been disconnected for more than 1 minute"

  # Migration failure alert
  - alert: TMCMigrationFailed
    expr: increase(tmc_migration_errors_total[10m]) > 0
    labels:
      severity: warning
    annotations:
      summary: "TMC migration failure detected"
      description: "Migration from {{ $labels.source_cluster }} to {{ $labels.target_cluster }} failed in phase {{ $labels.phase }}"

  # Queue backlog alert
  - alert: TMCSyncBacklogHigh
    expr: tmc_sync_backlog_size > 1000
    for: 15m
    labels:
      severity: warning
    annotations:
      summary: "High sync backlog in cluster {{ $labels.cluster }}"
      description: "Sync queue {{ $labels.queue }} has {{ $value }} pending items"
```

## Grafana Dashboards

### TMC Overview Dashboard

```json
{
  "dashboard": {
    "title": "TMC System Overview",
    "panels": [
      {
        "title": "System Health",
        "type": "stat",
        "targets": [{
          "expr": "avg(tmc_component_health)",
          "legendFormat": "Overall Health"
        }]
      },
      {
        "title": "Component Health by Type",
        "type": "bargauge", 
        "targets": [{
          "expr": "avg(tmc_component_health) by (component_type)",
          "legendFormat": "{{ component_type }}"
        }]
      },
      {
        "title": "Operation Rate",
        "type": "graph",
        "targets": [{
          "expr": "rate(tmc_component_operations_total[5m])",
          "legendFormat": "{{ component_type }} - {{ operation }}"
        }]
      },
      {
        "title": "Error Rate by Component",
        "type": "graph",
        "targets": [{
          "expr": "rate(tmc_component_errors_total[5m])",
          "legendFormat": "{{ component_type }} - {{ error_type }}"
        }]
      }
    ]
  }
}
```

### Sync Operations Dashboard

```json
{
  "dashboard": {
    "title": "TMC Sync Operations",
    "panels": [
      {
        "title": "Sync Success Rate",
        "type": "stat",
        "targets": [{
          "expr": "(rate(tmc_component_operations_total{operation=\"sync-resource\", status=\"success\"}[5m]) / rate(tmc_component_operations_total{operation=\"sync-resource\"}[5m])) * 100",
          "legendFormat": "Success Rate %"
        }]
      },
      {
        "title": "Sync Duration",
        "type": "graph",
        "targets": [{
          "expr": "histogram_quantile(0.95, rate(tmc_sync_duration_seconds_bucket[5m]))",
          "legendFormat": "95th percentile"
        }, {
          "expr": "histogram_quantile(0.50, rate(tmc_sync_duration_seconds_bucket[5m]))",
          "legendFormat": "50th percentile"
        }]
      },
      {
        "title": "Sync Backlog",
        "type": "graph",
        "targets": [{
          "expr": "tmc_sync_backlog_size",
          "legendFormat": "{{ cluster }} - {{ queue }}"
        }]
      }
    ]
  }
}
```

## Metrics Summary API

### Getting Metrics Summary

```go
// Get comprehensive metrics summary
summary := metricsReporter.GetMetricsSummary()

// Example summary structure:
{
  "overall_health": {
    "status": "Healthy",
    "message": "All 25 components healthy",
    "details": {
      "totalComponents": 25,
      "healthyComponents": 25,
      "degradedComponents": 0,
      "unhealthyComponents": 0
    }
  },
  "health_metrics": {
    "totalChecks": 5000,
    "healthyChecks": 4950,
    "degradedChecks": 30,
    "unhealthyChecks": 20,
    "errorChecks": 0
  },
  "timestamp": 1640995200,
  "collection_active": true
}
```

## Best Practices

### Metrics Design

1. **Use Consistent Labels**: Maintain consistent label naming across all metrics
2. **Avoid High Cardinality**: Limit label value combinations to prevent memory issues
3. **Include Context**: Add cluster, component, and operation context to metrics
4. **Use Appropriate Types**: Counter for cumulative values, Gauge for current state, Histogram for distributions
5. **Set Reasonable Buckets**: Configure histogram buckets appropriate for your latency distributions

### Performance Considerations

1. **Batch Metric Updates**: Avoid frequent individual metric updates
2. **Use Operation Tracking**: Leverage operation tracker for automatic latency and error tracking
3. **Monitor Collection Overhead**: Ensure metrics collection doesn't impact system performance
4. **Regular Cleanup**: Clean up unused custom metrics to prevent memory leaks

### Alerting Best Practices

1. **Set Meaningful Thresholds**: Base alert thresholds on actual SLA requirements
2. **Use Rate Windows**: Apply appropriate time windows for rate-based alerts
3. **Provide Context**: Include helpful information in alert annotations
4. **Avoid Alert Fatigue**: Ensure alerts are actionable and not too frequent
5. **Test Alert Rules**: Validate alerting rules in non-production environments

The TMC Metrics & Observability system provides comprehensive visibility into system performance and health with rich Prometheus integration and automated operational insights.