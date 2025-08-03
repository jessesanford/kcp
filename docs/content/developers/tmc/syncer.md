# KCP Workload Syncer

The KCP Workload Syncer is a critical component of the TMC system that handles bidirectional synchronization of workload resources between KCP logical clusters and physical Kubernetes clusters.

## Overview

The syncer ensures that:
- Resources created in KCP are automatically deployed to target physical clusters
- Status updates from physical clusters are reflected back in KCP
- Resource lifecycle is properly managed across cluster boundaries
- Health and metrics information is reported to the TMC system

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        KCP Logical Cluster                      │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐ │
│  │   Deployments   │  │    Services     │  │  ConfigMaps     │ │
│  │                 │  │                 │  │                 │ │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘ │
└─────────────────────────┬───────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Workload Syncer                              │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐ │
│  │ Syncer Engine   │──│ Resource        │  │ Status Reporter │ │
│  │                 │  │ Controllers     │  │                 │ │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘ │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐ │
│  │ Health Monitor  │  │ Metrics Server  │  │ TMC Integration │ │
│  │                 │  │                 │  │                 │ │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘ │
└─────────────────────────┬───────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────────┐
│                  Physical Kubernetes Cluster                    │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐ │
│  │   Deployments   │  │    Services     │  │  ConfigMaps     │ │
│  │   (synced)      │  │   (synced)      │  │   (synced)      │ │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

## Components

### Syncer Engine
The central orchestrator that manages all syncer operations:

```go
type Engine struct {
    syncTargetName   string
    workspaceCluster logicalcluster.Name
    kcpClient       dynamic.Interface
    clusterClient   dynamic.Interface
    // ... other fields
}
```

**Responsibilities:**
- Discovering available resource types
- Managing resource controller lifecycle
- Coordinating with TMC health and metrics systems
- Performing connectivity health checks

### Resource Controllers
Handle synchronization for specific resource types:

```go
type ResourceController struct {
    gvr          schema.GroupVersionResource
    kcpInformer  cache.SharedIndexInformer
    clusterInformer cache.SharedIndexInformer
    queue        workqueue.RateLimitingInterface
    // ... other fields
}
```

**Responsibilities:**
- Watching resource changes in both KCP and cluster
- Transforming resources for target environments
- Handling conflicts and retries
- Updating resource status

### Status Reporter
Manages SyncTarget status and heartbeats:

```go
type StatusReporter struct {
    syncTargetName   string
    heartbeatPeriod  time.Duration
    kcpClient       dynamic.Interface
    // ... other fields
}
```

**Responsibilities:**
- Sending periodic heartbeats to KCP
- Updating SyncTarget conditions
- Reporting connection health status
- Managing syncer registration

## Installation and Setup

### Building the Syncer

```bash
# Clone the KCP repository
git clone https://github.com/kcp-dev/kcp.git
cd kcp

# Build the syncer
go build -o workload-syncer ./cmd/workload-syncer
```

### Creating a SyncTarget

First, create a SyncTarget resource in your KCP workspace:

```yaml
apiVersion: workload.kcp.io/v1alpha1
kind: SyncTarget
metadata:
  name: my-cluster
  labels:
    location: "us-west-2"
    environment: "production"
spec:
  workloadCluster:
    name: "production-us-west-2"
    endpoint: "https://k8s-cluster.us-west-2.example.com"
  supportedAPIExports:
  - export: "kubernetes"
    resource: "apps/v1/deployments"
  - export: "kubernetes" 
    resource: "v1/services"
  capabilities:
  - type: "compute"
    resource: "cpu"
    capacity: "1000"
  - type: "compute"
    resource: "memory"
    capacity: "4000Gi"
```

### Running the Syncer

#### Basic Configuration

```bash
./workload-syncer \
  --sync-target-name=my-cluster \
  --sync-target-uid=$(kubectl get synctarget my-cluster -o jsonpath='{.metadata.uid}') \
  --workspace-cluster=root:my-workspace \
  --kcp-kubeconfig=~/.kcp/admin.kubeconfig \
  --cluster-kubeconfig=~/.kube/config \
  --v=2
```

#### Advanced Configuration

```bash
./workload-syncer \
  --sync-target-name=my-cluster \
  --sync-target-uid=$(kubectl get synctarget my-cluster -o jsonpath='{.metadata.uid}') \
  --workspace-cluster=root:my-workspace \
  --kcp-kubeconfig=~/.kcp/admin.kubeconfig \
  --cluster-kubeconfig=~/.kube/config \
  --resync-period=30s \
  --workers=4 \
  --heartbeat-period=15s \
  --v=3
```

#### Configuration Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `--sync-target-name` | Name of the SyncTarget resource | Required |
| `--sync-target-uid` | UID of the SyncTarget resource | Required |
| `--workspace-cluster` | Logical cluster containing the SyncTarget | Required |
| `--kcp-kubeconfig` | Path to KCP kubeconfig file | Required |
| `--cluster-kubeconfig` | Path to target cluster kubeconfig | Required |
| `--resync-period` | Informer resync period | 30s |
| `--workers` | Number of worker goroutines per controller | 2 |
| `--heartbeat-period` | Heartbeat interval | 30s |
| `--v` | Log verbosity level | 2 |

## Usage Examples

### Example 1: Basic Deployment Sync

1. **Create a deployment in KCP**:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-app
  namespace: default
spec:
  replicas: 3
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:1.20
        ports:
        - containerPort: 80
```

2. **Apply to KCP**:
```bash
kubectl apply -f deployment.yaml --context=kcp
```

3. **Verify sync to physical cluster**:
```bash
# Check deployment in physical cluster
kubectl get deployments --context=physical-cluster

# Check status back in KCP
kubectl get deployments nginx-app -o yaml --context=kcp
```

### Example 2: Service with LoadBalancer

1. **Create a service in KCP**:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: nginx-service
  namespace: default
spec:
  type: LoadBalancer
  selector:
    app: nginx
  ports:
  - port: 80
    targetPort: 80
```

2. **Apply and monitor**:
```bash
kubectl apply -f service.yaml --context=kcp

# Watch for LoadBalancer IP assignment
kubectl get service nginx-service -w --context=kcp
```

The syncer will:
- Create the service in the physical cluster
- Monitor for LoadBalancer IP assignment
- Update the service status in KCP with the assigned IP

### Example 3: ConfigMap and Secret Sync

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: app-config
  namespace: default
data:
  database_url: "postgres://db.example.com:5432/myapp"
  log_level: "info"
---
apiVersion: v1
kind: Secret
metadata:
  name: app-secrets
  namespace: default
type: Opaque
data:
  db-password: cGFzc3dvcmQ=  # base64 encoded "password"
```

## Monitoring and Observability

### Health Monitoring

The syncer provides comprehensive health information:

```bash
# Check SyncTarget status
kubectl get synctarget my-cluster -o yaml

# Look for conditions
kubectl get synctarget my-cluster -o jsonpath='{.status.conditions[*].type}'
```

Expected conditions:
- `Ready`: Overall SyncTarget readiness
- `SyncerReady`: Syncer connection status
- `HeartbeatReady`: Heartbeat health

### Metrics

The syncer exposes Prometheus metrics:

```yaml
# Key metrics to monitor
syncer_resources_synced_total
syncer_sync_duration_seconds
syncer_sync_errors_total
syncer_heartbeat_total
syncer_connection_status
```

### Logs

Enable detailed logging for troubleshooting:

```bash
./workload-syncer ... --v=4
```

Log levels:
- `--v=1`: Basic operational logs
- `--v=2`: Resource sync operations
- `--v=3`: Detailed sync events
- `--v=4`: Debug level logging

## Troubleshooting

### Common Issues

#### 1. Syncer Not Starting

**Symptoms**: Syncer exits immediately or fails to connect

**Diagnosis**:
```bash
# Check SyncTarget exists
kubectl get synctarget my-cluster

# Verify kubeconfig files
kubectl cluster-info --kubeconfig ~/.kcp/admin.kubeconfig
kubectl cluster-info --kubeconfig ~/.kube/config

# Check RBAC permissions
kubectl auth can-i "*" "*" --kubeconfig ~/.kcp/admin.kubeconfig
```

**Solutions**:
- Ensure SyncTarget exists and UID is correct
- Verify kubeconfig file paths and permissions
- Check network connectivity to both KCP and target cluster

#### 2. Resources Not Syncing

**Symptoms**: Resources created in KCP don't appear in physical cluster

**Diagnosis**:
```bash
# Check syncer logs
./workload-syncer ... --v=3

# Check SyncTarget conditions
kubectl get synctarget my-cluster -o yaml

# Verify resource discovery
kubectl api-resources --kubeconfig ~/.kube/config
```

**Solutions**:
- Ensure target cluster supports the resource types
- Check for RBAC issues in target cluster
- Verify network connectivity

#### 3. Status Not Updating

**Symptoms**: Resource status in KCP doesn't reflect physical cluster state

**Diagnosis**:
```bash
# Check resource status in both clusters
kubectl get deployment myapp -o yaml --context=kcp
kubectl get deployment myapp -o yaml --context=physical

# Check syncer status updates
./workload-syncer ... --v=4 | grep status
```

**Solutions**:
- Verify status subresource permissions
- Check for network issues
- Review TMC error logs

### Debug Commands

```bash
# Get detailed syncer status
kubectl get synctarget my-cluster -o jsonpath='{.status}' | jq

# Check resource transformations
kubectl get deployment myapp -o yaml | grep -A5 -B5 syncer

# Monitor sync operations
./workload-syncer ... --v=4 | grep -E "(sync|transform|status)"
```

## Advanced Configuration

### Custom Resource Types

The syncer automatically discovers and syncs all supported resource types. To sync custom resources:

1. **Ensure CRDs exist in both clusters**:
```bash
kubectl apply -f my-crd.yaml --context=kcp
kubectl apply -f my-crd.yaml --context=physical
```

2. **Create custom resources normally**:
```yaml
apiVersion: example.com/v1
kind: MyCustomResource
metadata:
  name: my-resource
spec:
  field: value
```

### Resource Transformations

The syncer applies automatic transformations:

- **Metadata cleanup**: Removes KCP-specific fields
- **Annotation addition**: Adds sync tracking annotations
- **Namespace mapping**: Handles namespace differences

### Performance Tuning

For high-throughput environments:

```bash
./workload-syncer \
  --workers=8 \
  --resync-period=60s \
  --heartbeat-period=10s
```

## Integration with TMC Components

### Error Handling Integration

The syncer integrates with TMC error handling:

```go
// Automatic error categorization
err := syncResource(ctx, resource)
if err != nil {
    tmcError := tmc.ConvertKubernetesError(err, "syncer", "sync")
    reporter.HandleError(tmcError)
}
```

### Health System Integration

```go
// Health provider registration
healthProvider := NewSyncerHealthProvider(syncTargetName, engine)
tmcHealth.RegisterHealthProvider(healthProvider)
```

### Metrics Integration

```go
// Automatic metrics collection
metrics.RecordResourceSync(gvk, "kcp-to-cluster", duration, true)
metrics.RecordHeartbeat(true)
```

## API Reference

### SyncTarget Status

```yaml
status:
  syncerIdentifier: "syncer-abc123"
  lastHeartbeatTime: "2024-01-15T10:30:00Z"
  conditions:
  - type: Ready
    status: "True"
    lastTransitionTime: "2024-01-15T10:00:00Z"
    reason: SyncTargetReady
    message: "SyncTarget is ready and operational"
  - type: SyncerReady
    status: "True"
    lastTransitionTime: "2024-01-15T10:00:00Z"
    reason: SyncerConnected
    message: "Syncer is connected and sending heartbeats"
  heartbeat:
    count: 1440
    lastHeartbeat: "2024-01-15T10:30:00Z"
    errors: 0
```

### Resource Annotations

Synced resources include these annotations:

```yaml
metadata:
  annotations:
    syncer.kcp.io/sync-target: "my-cluster"
    syncer.kcp.io/workspace: "root:my-workspace"
    syncer.kcp.io/last-sync: "2024-01-15T10:30:00Z"
```

## Best Practices

### 1. Resource Organization

- Use meaningful SyncTarget names
- Organize resources in appropriate namespaces
- Apply consistent labeling strategies

### 2. Monitoring

- Monitor SyncTarget health conditions
- Set up alerts for sync failures
- Track resource sync metrics

### 3. Security

- Use least-privilege RBAC policies
- Secure kubeconfig files appropriately
- Rotate credentials regularly

### 4. Operations

- Plan for cluster maintenance windows
- Test disaster recovery scenarios
- Monitor resource usage and scaling

## Migration Guide

### From Manual Cluster Management

1. **Assess current workloads**
2. **Create SyncTarget definitions**
3. **Deploy syncers incrementally**
4. **Validate sync operations**
5. **Migrate workloads to KCP**

### From Other Multi-Cluster Solutions

1. **Export current configurations**
2. **Convert to KCP resource formats**
3. **Set up TMC infrastructure**
4. **Parallel operation during transition**
5. **Complete migration and cleanup**