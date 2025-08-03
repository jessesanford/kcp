# Advanced Syncer Features Example

This example demonstrates advanced features of the TMC syncer system including custom resource synchronization, resource transformations, selective sync, and advanced monitoring.

## Prerequisites

- KCP with TMC components running
- Target Kubernetes cluster
- Custom Resource Definitions (CRDs) installed
- Prometheus for metrics collection

## Step 1: Custom Resource Synchronization

### Define Custom Resources

First, create a custom resource definition that will be synced:

```yaml
# database-crd.yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: databases.example.com
spec:
  group: example.com
  versions:
  - name: v1
    served: true
    storage: true
    schema:
      openAPIV3Schema:
        type: object
        properties:
          spec:
            type: object
            properties:
              engine:
                type: string
                enum: ["postgres", "mysql", "mongodb"]
              version:
                type: string
              replicas:
                type: integer
                minimum: 1
                maximum: 10
              storage:
                type: object
                properties:
                  size:
                    type: string
                  storageClass:
                    type: string
              backup:
                type: object
                properties:
                  enabled:
                    type: boolean
                  schedule:
                    type: string
          status:
            type: object
            properties:
              phase:
                type: string
                enum: ["Pending", "Creating", "Ready", "Failed"]
              conditions:
                type: array
                items:
                  type: object
                  properties:
                    type:
                      type: string
                    status:
                      type: string
                    reason:
                      type: string
                    message:
                      type: string
              endpoints:
                type: array
                items:
                  type: object
                  properties:
                    name:
                      type: string
                    host:
                      type: string
                    port:
                      type: integer
    subresources:
      status: {}
  scope: Namespaced
  names:
    plural: databases
    singular: database
    kind: Database
    shortNames:
    - db
```

Install the CRD in both KCP and the target cluster:

```bash
# Install in KCP
kubectl apply -f database-crd.yaml --context=kcp

# Install in target cluster  
kubectl apply -f database-crd.yaml --context=target-cluster
```

### Create Custom Resource Instances

```yaml
# postgres-database.yaml
apiVersion: example.com/v1
kind: Database
metadata:
  name: user-db
  namespace: production
  labels:
    tier: primary
    backup: enabled
spec:
  engine: postgres
  version: "13.8"
  replicas: 3
  storage:
    size: "100Gi"
    storageClass: "fast-ssd"
  backup:
    enabled: true
    schedule: "0 2 * * *"
---
apiVersion: example.com/v1
kind: Database
metadata:
  name: analytics-db
  namespace: production
  labels:
    tier: analytics
    backup: enabled
spec:
  engine: mongodb
  version: "5.0"
  replicas: 2
  storage:
    size: "500Gi"
    storageClass: "bulk-storage"
  backup:
    enabled: true
    schedule: "0 3 * * *"
```

Apply the custom resources to KCP:

```bash
kubectl apply -f postgres-database.yaml --context=kcp
```

## Step 2: Resource Transformation Examples

### Transform ConfigMaps with Environment-Specific Values

Create a transformation script that the syncer will apply:

```yaml
# transformation-config.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: syncer-transformations
  namespace: kcp-system
data:
  database-transform.yaml: |
    # Transform database resources for different environments
    transformations:
    - match:
        apiVersion: example.com/v1
        kind: Database
      transforms:
      - type: "environment-config"
        target: "spec.replicas"
        rules:
        - if: "metadata.labels.tier == 'analytics'"
          set: 1  # Reduce analytics replicas in non-prod
        - if: "metadata.labels.tier == 'primary'"
          set: 2  # Reduce primary replicas in non-prod
      - type: "storage-class"
        target: "spec.storage.storageClass"
        rules:
        - if: "spec.storage.storageClass == 'fast-ssd'"
          set: "standard"  # Use standard storage in non-prod
```

### Annotation-Based Transformations

```yaml
# app-with-transformations.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web-app
  namespace: production
  annotations:
    syncer.kcp.io/transform: "environment-specific"
    syncer.kcp.io/target-replicas: "2"  # Override for target cluster
    syncer.kcp.io/resource-limits: "reduced"
spec:
  replicas: 5  # Original replicas in KCP
  selector:
    matchLabels:
      app: web-app
  template:
    metadata:
      labels:
        app: web-app
    spec:
      containers:
      - name: app
        image: myapp:latest
        resources:
          requests:
            cpu: 500m     # Will be transformed to 100m
            memory: 1Gi   # Will be transformed to 256Mi
          limits:
            cpu: 1000m    # Will be transformed to 200m
            memory: 2Gi   # Will be transformed to 512Mi
```

## Step 3: Selective Synchronization

### Namespace-Based Filtering

Configure the syncer to only sync specific namespaces:

```bash
# Enhanced syncer configuration
./workload-syncer \
  --sync-target-name=selective-cluster \
  --sync-target-uid=$(kubectl get synctarget selective-cluster -o jsonpath='{.metadata.uid}') \
  --workspace-cluster=root:selective \
  --kcp-kubeconfig=~/.kcp/admin.kubeconfig \
  --cluster-kubeconfig=~/.kube/config \
  --sync-namespaces="production,staging" \
  --exclude-namespaces="internal,kcp-system" \
  --v=3
```

### Label-Based Resource Filtering

Create resources with sync control labels:

```yaml
# selective-resources.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: sync-enabled-config
  namespace: production
  labels:
    syncer.kcp.io/sync: "enabled"
    environment: production
data:
  config: "This will be synced"
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: sync-disabled-config
  namespace: production
  labels:
    syncer.kcp.io/sync: "disabled"
    environment: production
data:
  config: "This will NOT be synced"
---
apiVersion: v1
kind: Secret
metadata:
  name: cluster-specific-secret
  namespace: production
  annotations:
    syncer.kcp.io/target-clusters: "cluster-a,cluster-b"
type: Opaque
data:
  password: dGVzdC1wYXNzd29yZA==
```

## Step 4: Advanced Health Monitoring

### Custom Health Checks

Create custom health check configuration:

```yaml
# health-config.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: syncer-health-config
  namespace: kcp-system
data:
  health-checks.yaml: |
    healthChecks:
    - name: "database-connectivity"
      type: "tcp"
      target: "postgres.production.svc.cluster.local:5432"
      interval: "30s"
      timeout: "5s"
      successThreshold: 1
      failureThreshold: 3
    - name: "application-health"
      type: "http"
      target: "http://web-app.production.svc.cluster.local/health"
      interval: "15s"
      timeout: "10s"
      successThreshold: 1
      failureThreshold: 2
    - name: "storage-health"
      type: "exec"
      command: ["df", "-h", "/data"]
      interval: "60s"
      timeout: "10s"
```

### Health Status Aggregation

Monitor health across multiple dimensions:

```bash
#!/bin/bash
# advanced-health-monitor.sh

echo "=== Advanced TMC Health Monitor ==="
echo

# Check SyncTarget health
echo "SyncTarget Health Summary:"
kubectl get synctargets -o json | jq -r '
  .items[] | 
  .metadata.name as $name |
  .status.conditions[] |
  select(.type == "Ready") |
  "\($name): \(.status) - \(.message)"
'

echo
echo "Component Health Details:"
kubectl get synctargets -o json | jq -r '
  .items[] |
  .metadata.name as $name |
  .status.conditions[] |
  "\($name).\(.type): \(.status)"
' | column -t

echo
echo "Resource Sync Status:"
kubectl get deployments,services,configmaps -A -o json | jq -r '
  .items[] |
  select(.metadata.annotations["syncer.kcp.io/sync-target"]) |
  "\(.kind)/\(.metadata.name): \(.metadata.annotations["syncer.kcp.io/sync-target"])"
'

echo
echo "Recent Sync Events:"
kubectl get events --field-selector reason=Synced --sort-by=.metadata.creationTimestamp | tail -5
```

## Step 5: Advanced Metrics and Observability

### Custom Metrics Collection

Configure Prometheus to scrape syncer metrics:

```yaml
# prometheus-config.yaml
apiVersion: v1
kind: ServiceMonitor
metadata:
  name: syncer-metrics
  namespace: monitoring
spec:
  selector:
    matchLabels:
      app: workload-syncer
  endpoints:
  - port: metrics
    interval: 15s
    path: /metrics
---
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: syncer-alerts
  namespace: monitoring
spec:
  groups:
  - name: syncer.rules
    rules:
    - alert: SyncerDown
      expr: up{job="workload-syncer"} == 0
      for: 1m
      labels:
        severity: critical
      annotations:
        summary: "Syncer is down"
        description: "Workload syncer {{ $labels.instance }} has been down for more than 1 minute"
    
    - alert: HighSyncErrorRate
      expr: rate(syncer_sync_errors_total[5m]) > 0.1
      for: 2m
      labels:
        severity: warning
      annotations:
        summary: "High sync error rate"
        description: "Sync error rate is {{ $value }} errors per second"
    
    - alert: SyncBacklogHigh
      expr: syncer_sync_backlog_size > 100
      for: 5m
      labels:
        severity: warning
      annotations:
        summary: "Sync backlog is high"
        description: "Sync backlog has {{ $value }} pending operations"
    
    - alert: HeartbeatMissing
      expr: time() - syncer_heartbeat_total > 120
      for: 1m
      labels:
        severity: critical
      annotations:
        summary: "Syncer heartbeat missing"
        description: "No heartbeat received from syncer for more than 2 minutes"
```

### Grafana Dashboard

Create a comprehensive dashboard:

```json
{
  "dashboard": {
    "title": "TMC Syncer Dashboard",
    "panels": [
      {
        "title": "Syncer Health",
        "type": "stat",
        "targets": [
          {
            "expr": "up{job=\"workload-syncer\"}",
            "legendFormat": "{{ instance }}"
          }
        ]
      },
      {
        "title": "Sync Operations Rate",
        "type": "graph",
        "targets": [
          {
            "expr": "rate(syncer_resources_synced_total[5m])",
            "legendFormat": "{{ direction }} - {{ gvk }}"
          }
        ]
      },
      {
        "title": "Sync Duration",
        "type": "graph",
        "targets": [
          {
            "expr": "histogram_quantile(0.95, rate(syncer_sync_duration_seconds_bucket[5m]))",
            "legendFormat": "95th percentile"
          },
          {
            "expr": "histogram_quantile(0.50, rate(syncer_sync_duration_seconds_bucket[5m]))",
            "legendFormat": "50th percentile"
          }
        ]
      },
      {
        "title": "Error Rate",
        "type": "graph",
        "targets": [
          {
            "expr": "rate(syncer_sync_errors_total[5m])",
            "legendFormat": "{{ error_type }}"
          }
        ]
      }
    ]
  }
}
```

## Step 6: Disaster Recovery Scenarios

### Backup and Restore Configuration

```yaml
# backup-config.yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: syncer-backup
  namespace: kcp-system
spec:
  schedule: "0 2 * * *"
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: backup
            image: backup-tool:latest
            command:
            - /bin/sh
            - -c
            - |
              # Backup SyncTarget configurations
              kubectl get synctargets -o yaml > /backup/synctargets-$(date +%Y%m%d).yaml
              
              # Backup TMC configuration
              kubectl get configmaps -n kcp-system -l component=tmc -o yaml > /backup/tmc-config-$(date +%Y%m%d).yaml
              
              # Upload to storage
              aws s3 cp /backup/ s3://kcp-backups/$(date +%Y/%m/%d)/ --recursive
            volumeMounts:
            - name: backup-storage
              mountPath: /backup
          volumes:
          - name: backup-storage
            emptyDir: {}
          restartPolicy: OnFailure
```

### Failover Automation

```bash
#!/bin/bash
# failover-script.sh

FAILED_CLUSTER=$1
BACKUP_CLUSTER=$2

echo "Initiating failover from $FAILED_CLUSTER to $BACKUP_CLUSTER"

# Mark failed cluster as unschedulable
kubectl patch synctarget $FAILED_CLUSTER -p '{"spec":{"unschedulable":true}}'

# Scale up applications on backup cluster
kubectl get deployments -A -o json | jq -r '
  .items[] |
  select(.metadata.annotations["syncer.kcp.io/primary-cluster"] == "'$FAILED_CLUSTER'") |
  "\(.metadata.namespace) \(.metadata.name)"
' | while read namespace deployment; do
  echo "Scaling up $deployment in $namespace"
  kubectl scale deployment $deployment -n $namespace --replicas=3
done

# Update DNS/Load balancer to point to backup cluster
echo "Updating DNS records..."
# Implementation depends on your DNS provider

# Send notifications
echo "Failover completed. Services migrated from $FAILED_CLUSTER to $BACKUP_CLUSTER"
```

## Step 7: Performance Optimization

### High-Throughput Configuration

```bash
#!/bin/bash
# high-performance-syncer.sh

./workload-syncer \
  --sync-target-name=high-perf-cluster \
  --sync-target-uid=$(kubectl get synctarget high-perf-cluster -o jsonpath='{.metadata.uid}') \
  --workspace-cluster=root:production \
  --kcp-kubeconfig=~/.kcp/admin.kubeconfig \
  --cluster-kubeconfig=~/.kube/config \
  --workers=16 \
  --resync-period=60s \
  --heartbeat-period=10s \
  --qps=100 \
  --burst=200 \
  --max-concurrent-syncs=50 \
  --v=1
```

### Resource Optimization

```yaml
# resource-limits.yaml
apiVersion: v1
kind: LimitRange
metadata:
  name: syncer-limits
  namespace: kcp-system
spec:
  limits:
  - type: Container
    defaultRequest:
      cpu: 100m
      memory: 128Mi
    default:
      cpu: 500m
      memory: 512Mi
    max:
      cpu: 2000m
      memory: 4Gi
```

## Step 8: Debugging and Troubleshooting

### Enhanced Logging Configuration

```bash
# Debug mode syncer
./workload-syncer \
  --sync-target-name=debug-cluster \
  --sync-target-uid=$(kubectl get synctarget debug-cluster -o jsonpath='{.metadata.uid}') \
  --workspace-cluster=root:debug \
  --kcp-kubeconfig=~/.kcp/admin.kubeconfig \
  --cluster-kubeconfig=~/.kube/config \
  --v=5 \
  --log-format=json \
  --log-file=/var/log/syncer/debug.log \
  --alsologtostderr=true
```

### Diagnostic Tools

```bash
#!/bin/bash
# syncer-diagnostics.sh

echo "=== Syncer Diagnostic Report ==="
echo "Generated at: $(date)"
echo

echo "1. SyncTarget Status:"
kubectl get synctargets -o yaml | yq eval '.items[].status' -

echo
echo "2. Recent Events:"
kubectl get events --sort-by=.metadata.creationTimestamp | tail -20

echo
echo "3. Resource Sync Status:"
kubectl get all -A -o json | jq -r '
  .items[] |
  select(.metadata.annotations["syncer.kcp.io/sync-target"]) |
  "\(.kind)/\(.metadata.name): Last sync: \(.metadata.annotations["syncer.kcp.io/last-sync"] // "never")"
'

echo
echo "4. Network Connectivity:"
# Test connectivity to target clusters
for cluster in $(kubectl get synctargets -o jsonpath='{.items[*].spec.workloadCluster.endpoint}'); do
  echo "Testing connection to $cluster"
  curl -k --connect-timeout 5 $cluster/healthz && echo " - OK" || echo " - FAILED"
done

echo
echo "5. Resource Usage:"
kubectl top nodes 2>/dev/null || echo "Metrics server not available"

echo
echo "Diagnostic report complete."
```

## Cleanup

```bash
# Stop enhanced syncer
pkill -f workload-syncer

# Clean up resources
kubectl delete -f postgres-database.yaml
kubectl delete -f database-crd.yaml
kubectl delete crd databases.example.com
kubectl delete configmap syncer-transformations -n kcp-system
kubectl delete configmap syncer-health-config -n kcp-system

# Clean up monitoring
kubectl delete servicemonitor syncer-metrics -n monitoring
kubectl delete prometheusrule syncer-alerts -n monitoring
```

This advanced example demonstrates the sophisticated capabilities of the TMC syncer system for complex production environments with custom resources, transformations, selective synchronization, and comprehensive monitoring.