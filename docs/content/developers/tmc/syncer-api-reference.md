# Syncer API Reference

This document provides a comprehensive reference for the KCP Workload Syncer APIs, configuration options, and integration interfaces.

## Command Line Interface

### Basic Usage

```bash
workload-syncer [flags]
```

### Required Flags

| Flag | Type | Description |
|------|------|-------------|
| `--sync-target-name` | string | Name of the SyncTarget resource in KCP |
| `--sync-target-uid` | string | UID of the SyncTarget resource |
| `--workspace-cluster` | string | Logical cluster containing the SyncTarget |
| `--kcp-kubeconfig` | string | Path to KCP kubeconfig file |
| `--cluster-kubeconfig` | string | Path to target cluster kubeconfig file |

### Optional Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--resync-period` | duration | 30s | Informer resync period |
| `--workers` | int | 2 | Number of worker goroutines per resource controller |
| `--heartbeat-period` | duration | 30s | Period for sending heartbeats to KCP |
| `--v` | int | 2 | Log verbosity level (0-5) |
| `--log-format` | string | text | Log format (text or json) |
| `--log-file` | string | | Path to log file (logs to stderr if not specified) |
| `--qps` | float32 | 20 | Queries per second limit for Kubernetes clients |
| `--burst` | int | 30 | Burst limit for Kubernetes clients |

### Example Usage

```bash
# Basic configuration
./workload-syncer \
  --sync-target-name=my-cluster \
  --sync-target-uid=12345678-1234-1234-1234-123456789012 \
  --workspace-cluster=root:my-workspace \
  --kcp-kubeconfig=~/.kcp/admin.kubeconfig \
  --cluster-kubeconfig=~/.kube/config

# Production configuration with enhanced settings
./workload-syncer \
  --sync-target-name=prod-cluster \
  --sync-target-uid=87654321-4321-4321-4321-210987654321 \
  --workspace-cluster=root:production \
  --kcp-kubeconfig=/etc/kcp/kubeconfig \
  --cluster-kubeconfig=/etc/kubernetes/cluster-config \
  --workers=8 \
  --resync-period=60s \
  --heartbeat-period=15s \
  --qps=50 \
  --burst=100 \
  --v=3 \
  --log-format=json \
  --log-file=/var/log/syncer.log
```

## SyncTarget Resource API

### SyncTarget Specification

```yaml
apiVersion: workload.kcp.io/v1alpha1
kind: SyncTarget
metadata:
  name: <sync-target-name>
  labels:
    # Standard labels
    environment: <environment>    # e.g., production, staging, development
    region: <region>             # e.g., us-west-2, eu-central-1
    tier: <tier>                 # e.g., primary, secondary, edge
    
    # Custom labels for placement
    <key>: <value>
spec:
  # Required: Cluster connection information
  workloadCluster:
    name: <cluster-name>
    endpoint: <cluster-endpoint>    # e.g., https://k8s.example.com
    credentials:                    # Optional: authentication credentials
      secretRef:
        name: <secret-name>
        namespace: <secret-namespace>
  
  # Optional: Supported API exports
  supportedAPIExports:
  - export: <export-name>          # e.g., "kubernetes"
    resource: <resource-gvr>       # e.g., "apps/v1/deployments"
  
  # Optional: Cluster capabilities
  capabilities:
  - type: <capability-type>        # e.g., "compute", "storage", "network"
    resource: <resource-name>      # e.g., "cpu", "memory", "disk"
    capacity: <capacity-value>     # e.g., "1000", "2000Gi", "10Gbps"
  
  # Optional: Workspace restrictions
  workspaces:
  - name: <workspace-name>
    selector:
      matchLabels:
        <key>: <value>
  
  # Optional: Scheduling control
  unschedulable: false             # Set to true to prevent new workloads
  
  # Optional: Eviction policy
  evictAfter: <duration>           # e.g., "5m", "1h"
  
  # Optional: Cell organization
  cells:
    zone: <zone>                   # e.g., "us-west-2a"
    rack: <rack>                   # e.g., "rack-1"
    <key>: <value>                 # Custom cell attributes
```

### SyncTarget Status

```yaml
status:
  # Syncer identification
  syncerIdentifier: <syncer-uid>
  
  # Heartbeat information
  lastHeartbeatTime: <timestamp>   # RFC3339 format
  heartbeat:
    count: <heartbeat-count>       # Total heartbeats sent
    lastHeartbeat: <timestamp>     # Last successful heartbeat
    errors: <error-count>          # Failed heartbeat attempts
  
  # Conditions
  conditions:
  - type: Ready                    # Overall readiness
    status: <True|False|Unknown>
    lastTransitionTime: <timestamp>
    reason: <reason>
    message: <message>
  - type: SyncerReady             # Syncer connection status
    status: <True|False|Unknown>
    lastTransitionTime: <timestamp>
    reason: <reason>
    message: <message>
  - type: HeartbeatReady          # Heartbeat health
    status: <True|False|Unknown>
    lastTransitionTime: <timestamp>
    reason: <reason>
    message: <message>
  - type: APIImportsReady         # API import status
    status: <True|False|Unknown>
    lastTransitionTime: <timestamp>
    reason: <reason>
    message: <message>
  
  # Resource synchronization status
  resourceSyncStatus:
    totalResources: <count>
    syncedResources: <count>
    failedResources: <count>
    lastSyncTime: <timestamp>
  
  # Cluster information
  clusterInfo:
    version: <kubernetes-version>
    nodes: <node-count>
    capacity:
      cpu: <cpu-capacity>
      memory: <memory-capacity>
      pods: <pod-capacity>
```

## Resource Annotations

The syncer adds annotations to synchronized resources for tracking and management.

### Syncer-Added Annotations

| Annotation | Description | Example |
|------------|-------------|---------|
| `syncer.kcp.io/sync-target` | Target cluster name | `production-cluster` |
| `syncer.kcp.io/workspace` | Source workspace | `root:my-workspace` |
| `syncer.kcp.io/last-sync` | Last sync timestamp | `2024-01-15T10:30:00Z` |
| `syncer.kcp.io/resource-version` | Source resource version | `12345` |
| `syncer.kcp.io/sync-generation` | Sync generation number | `3` |

### User-Controlled Annotations

| Annotation | Description | Example |
|------------|-------------|---------|
| `syncer.kcp.io/sync` | Control sync behavior | `enabled`, `disabled`, `pause` |
| `syncer.kcp.io/transform` | Apply transformations | `environment-specific`, `none` |
| `syncer.kcp.io/target-clusters` | Specific target clusters | `cluster-a,cluster-b` |
| `syncer.kcp.io/exclude-clusters` | Clusters to exclude | `test-cluster` |
| `syncer.kcp.io/sync-policy` | Sync policy override | `immediate`, `batched`, `delayed` |

### Resource Labels

| Label | Description | Example |
|-------|-------------|---------|
| `syncer.kcp.io/cluster` | Target cluster identifier | `prod-us-west-2` |
| `syncer.kcp.io/origin` | Origin workspace | `root:production` |

## Metrics API

The syncer exposes Prometheus metrics for monitoring and observability.

### Core Metrics

#### Resource Synchronization

```prometheus
# Total number of resources synced
syncer_resources_synced_total{sync_target, workspace, gvk, direction}

# Duration of sync operations
syncer_sync_duration_seconds{sync_target, workspace, gvk, operation}

# Number of sync errors
syncer_sync_errors_total{sync_target, workspace, gvk, error_type}

# Size of sync backlog
syncer_sync_backlog_size{sync_target, workspace, gvk}
```

#### Heartbeat Metrics

```prometheus
# Total heartbeats sent
syncer_heartbeat_total

# Heartbeat errors
syncer_heartbeat_errors_total

# Connection status to KCP and cluster
syncer_connection_status{sync_target, workspace, target}
```

#### Component Health

```prometheus
# Number of active resource controllers
syncer_resource_controllers

# Syncer uptime in seconds
syncer_uptime_seconds
```

### TMC Integration Metrics

```prometheus
# Component health status
tmc_component_health{component_type, component_id, cluster}

# Component operation count
tmc_component_operations_total{component_type, component_id, operation, status}

# Component error count
tmc_component_errors_total{component_type, component_id, error_type, severity}

# Operation duration
tmc_component_operation_duration_seconds{component_type, component_id, operation}
```

### Example Queries

```prometheus
# Average sync duration by resource type
rate(syncer_sync_duration_seconds_sum[5m]) / rate(syncer_sync_duration_seconds_count[5m])

# Error rate by sync target
rate(syncer_sync_errors_total[5m]) / rate(syncer_resources_synced_total[5m])

# Resource controller health
syncer_resource_controllers > 0

# Heartbeat success rate
rate(syncer_heartbeat_total[5m]) / (rate(syncer_heartbeat_total[5m]) + rate(syncer_heartbeat_errors_total[5m]))
```

## Health Check API

### Component Health Status

The syncer provides health information through the TMC health system.

#### Health Check Response

```json
{
  "componentType": "SyncTargetController",
  "componentID": "syncer-my-cluster",
  "status": "Healthy|Degraded|Unhealthy|Unknown",
  "message": "Component status description",
  "timestamp": "2024-01-15T10:30:00Z",
  "duration": "5ms",
  "details": {
    "engineStarted": true,
    "resourceControllers": 4,
    "syncCount": 1234,
    "errorCount": 5,
    "errorRate": 0.004,
    "lastSyncTime": "2024-01-15T10:29:45Z",
    "timeSinceLastSync": "15s",
    "statusReporter": {
      "started": true,
      "heartbeatCount": 500,
      "errorCount": 0,
      "connectionHealthy": true,
      "lastHeartbeat": "2024-01-15T10:30:00Z"
    }
  }
}
```

#### Health Status Criteria

| Status | Criteria |
|--------|----------|
| **Healthy** | All conditions met: engine started, active controllers, error rate < 10%, recent sync activity, healthy heartbeat |
| **Degraded** | Some issues present: error rate 10-50%, no sync activity > 10min, no heartbeat > 2x period |
| **Unhealthy** | Critical issues: engine not started, error rate > 50%, no controllers, no heartbeat > 5min |
| **Unknown** | Unable to determine status |

## Events API

The syncer generates Kubernetes events for important operations and state changes.

### Event Types

#### Sync Events

```yaml
# Successful resource sync
type: Normal
reason: Synced
message: "Successfully synced Deployment default/my-app to cluster"

# Sync failure
type: Warning
reason: SyncFailed
message: "Failed to sync Service default/my-service: connection refused"

# Resource transformation
type: Normal
reason: Transformed
message: "Applied environment-specific transformations to ConfigMap"
```

#### Heartbeat Events

```yaml
# Heartbeat success
type: Normal
reason: HeartbeatSent
message: "Heartbeat sent successfully"

# Heartbeat failure
type: Warning
reason: HeartbeatFailed
message: "Failed to send heartbeat: connection timeout"
```

#### Controller Events

```yaml
# Controller start
type: Normal
reason: ControllerStarted
message: "Started resource controller for apps/v1/deployments"

# Controller error
type: Warning
reason: ControllerError
message: "Resource controller error: failed to process work item"
```

## Error Codes and Recovery

### Error Categories

The syncer integrates with TMC error handling and categorizes errors for appropriate recovery strategies.

#### TMC Error Types

| Error Type | Description | Retryable | Recovery Strategy |
|------------|-------------|-----------|-------------------|
| `ResourceNotFound` | Resource missing in source | No | Create or update source |
| `ResourceConflict` | Resource version conflict | Yes | Fetch latest and retry |
| `ResourceValidation` | Invalid resource specification | No | Fix resource definition |
| `ResourcePermission` | Insufficient RBAC permissions | No | Update permissions |
| `ClusterUnreachable` | Cannot connect to cluster | Yes | Check network/retry |
| `ClusterUnavailable` | Cluster API unavailable | Yes | Wait and retry |
| `ClusterAuth` | Authentication failure | No | Update credentials |
| `SyncFailure` | General sync operation failure | Yes | Retry with backoff |
| `SyncTimeout` | Sync operation timeout | Yes | Retry with longer timeout |

### Retry Configuration

```go
// Default retry strategy
type RetryStrategy struct {
    MaxRetries      int           // 5
    InitialDelay    time.Duration // 1s
    MaxDelay        time.Duration // 30s
    BackoffFactor   float64       // 2.0
    RetryableErrors []TMCErrorType
}
```

## Configuration API

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `SYNCER_LOG_LEVEL` | Log verbosity level | `2` |
| `SYNCER_METRICS_PORT` | Metrics server port | `8080` |
| `SYNCER_HEALTH_PORT` | Health check port | `8081` |
| `SYNCER_WORKERS` | Number of workers | `2` |
| `SYNCER_RESYNC_PERIOD` | Informer resync period | `30s` |
| `SYNCER_HEARTBEAT_PERIOD` | Heartbeat interval | `30s` |

### Configuration File

The syncer can be configured using a YAML file:

```yaml
# syncer-config.yaml
syncTarget:
  name: my-cluster
  uid: 12345678-1234-1234-1234-123456789012
  workspace: root:my-workspace

kubeconfig:
  kcp: ~/.kcp/admin.kubeconfig
  cluster: ~/.kube/config

performance:
  workers: 4
  resyncPeriod: 60s
  qps: 50
  burst: 100

sync:
  heartbeatPeriod: 15s
  transformations:
    enabled: true
    configMap: syncer-transformations
  filters:
    namespaces:
      include: ["production", "staging"]
      exclude: ["kube-system"]
    labels:
      require:
        syncer.kcp.io/sync: enabled

logging:
  level: 3
  format: json
  file: /var/log/syncer.log

metrics:
  enabled: true
  port: 8080
  path: /metrics

health:
  enabled: true
  port: 8081
  path: /health
```

Usage with configuration file:

```bash
./workload-syncer --config=syncer-config.yaml
```

## Integration APIs

### TMC Error Handling Integration

```go
// Error reporting to TMC
func (rc *ResourceController) handleSyncError(err error, resource *unstructured.Unstructured) error {
    tmcError := tmc.ConvertKubernetesError(err, "syncer", "sync-resource")
    tmcError = tmcError.WithResource(resource.GroupVersionKind(), 
                                   resource.GetNamespace(), 
                                   resource.GetName())
    
    // Report to TMC error handling system
    rc.tmcErrorReporter.HandleError(tmcError)
    
    return tmcError
}
```

### TMC Metrics Integration

```go
// Metrics collection
func (rc *ResourceController) recordSyncOperation(gvr string, duration time.Duration, success bool) {
    // Local syncer metrics
    rc.metricsServer.RecordResourceSync(gvr, "kcp-to-cluster", duration, success)
    
    // TMC metrics integration
    rc.tmcMetrics.RecordSyncDuration(rc.syncTargetName, gvr, "sync", duration)
    if success {
        rc.tmcMetrics.RecordSyncResourceCount(rc.syncTargetName, "", gvr, "synced", 1)
    } else {
        rc.tmcMetrics.RecordSyncError(rc.syncTargetName, gvr, tmc.TMCErrorTypeSyncFailure)
    }
}
```

### TMC Health Integration

```go
// Health provider registration
func (s *Syncer) registerWithTMCHealth() error {
    healthProvider := NewSyncerHealthProvider(s.options.SyncTargetName, s.engine)
    s.tmcHealth.RegisterHealthProvider(healthProvider)
    return nil
}
```

This API reference provides comprehensive information for integrating with and operating the KCP Workload Syncer component within the TMC system.