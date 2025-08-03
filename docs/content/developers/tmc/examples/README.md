# TMC Examples

This directory contains comprehensive examples demonstrating the capabilities of the Transparent Multi-Cluster (TMC) system and its components.

## Syncer Examples

The KCP Workload Syncer is the core component responsible for bidirectional resource synchronization between KCP and physical clusters.

### [Basic Setup](./syncer/basic-setup.md)
Learn how to set up a basic syncer configuration and deploy your first synchronized workload.

**What you'll learn:**
- SyncTarget resource creation
- Syncer configuration and startup
- Basic workload deployment and verification
- Health monitoring fundamentals

**Prerequisites:** KCP running, target cluster access

**Time:** ~30 minutes

### [Multi-Cluster Deployment](./syncer/multi-cluster-deployment.md)
Deploy applications across multiple clusters with intelligent placement and failover capabilities.

**What you'll learn:**
- Managing multiple SyncTargets
- Regional deployment strategies
- Load balancing and failover
- Cross-cluster health monitoring
- Rolling updates across clusters

**Prerequisites:** Multiple Kubernetes clusters, networking setup

**Time:** ~1 hour

### [Advanced Features](./syncer/advanced-features.md)
Explore sophisticated syncer capabilities including custom resources, transformations, and advanced monitoring.

**What you'll learn:**
- Custom Resource Definition synchronization
- Resource transformation and filtering
- Selective synchronization strategies
- Advanced metrics and observability
- Performance optimization techniques
- Disaster recovery scenarios

**Prerequisites:** Advanced Kubernetes knowledge, monitoring tools

**Time:** ~2 hours

## TMC Component Examples

### Error Handling Examples

Examples demonstrating TMC's robust error handling and recovery capabilities.

#### Basic Error Handling
```yaml
# Example: Handling resource conflicts
apiVersion: apps/v1
kind: Deployment
metadata:
  name: conflict-example
  annotations:
    tmc.kcp.io/error-policy: "retry-with-backoff"
    tmc.kcp.io/max-retries: "5"
spec:
  # ... deployment spec
```

#### Advanced Error Recovery
```yaml
# Example: Custom recovery strategies
apiVersion: v1
kind: ConfigMap
metadata:
  name: error-recovery-config
data:
  strategy.yaml: |
    errorRecovery:
      - errorType: "ResourceConflict"
        strategy: "merge-and-retry"
        maxAttempts: 3
      - errorType: "ClusterUnreachable"
        strategy: "failover-to-backup"
        backupClusters: ["backup-cluster-1", "backup-cluster-2"]
```

### Health Monitoring Examples

#### Component Health Checks
```yaml
# Example: Custom health check configuration
apiVersion: v1
kind: ConfigMap
metadata:
  name: health-config
data:
  checks.yaml: |
    healthChecks:
      - name: "api-server-connectivity"
        type: "tcp"
        target: "kubernetes.default.svc.cluster.local:443"
        interval: "30s"
      - name: "application-health"
        type: "http"
        target: "http://my-app:8080/health"
        interval: "15s"
```

#### Health Aggregation
```bash
# Get overall TMC system health
kubectl get componenthealth -o yaml

# Check specific component health
kubectl get componenthealth syncer-cluster-1 -o jsonpath='{.status.overallHealth}'
```

### Metrics and Observability Examples

#### Prometheus Configuration
```yaml
# Example: TMC metrics scrape configuration
apiVersion: v1
kind: ServiceMonitor
metadata:
  name: tmc-metrics
spec:
  selector:
    matchLabels:
      component: tmc
  endpoints:
  - port: metrics
    interval: 30s
    path: /metrics
```

#### Grafana Dashboard Queries
```promql
# Sync operation success rate
rate(tmc_sync_operations_total{status="success"}[5m]) / 
rate(tmc_sync_operations_total[5m])

# Error rate by component
rate(tmc_component_errors_total[5m]) by (component_type)

# Resource sync lag
histogram_quantile(0.95, 
  rate(tmc_sync_duration_seconds_bucket[5m])
) by (cluster)
```

## Virtual Workspace Examples

### Cross-Cluster Resource Aggregation
```yaml
# Example: Virtual workspace configuration
apiVersion: tenancy.kcp.io/v1alpha1
kind: Workspace
metadata:
  name: virtual-production
spec:
  type: virtual
  aggregation:
    clusters:
    - name: "prod-us-west"
      weight: 0.6
    - name: "prod-us-east"
      weight: 0.4
    resources:
    - group: "apps"
      version: "v1"
      resource: "deployments"
    - group: ""
      version: "v1"
      resource: "services"
```

### Resource Projection
```yaml
# Example: Project resources with transformations
apiVersion: workload.kcp.io/v1alpha1
kind: ResourceProjection
metadata:
  name: aggregated-metrics
spec:
  source:
    clusters: ["monitoring-*"]
    resources:
    - group: "monitoring.coreos.com"
      version: "v1"
      resource: "servicemonitors"
  target:
    workspace: "root:observability"
  transformation:
    type: "merge"
    rules:
    - field: "spec.endpoints"
      action: "append"
```

## Placement Controller Examples

### Intelligent Workload Placement
```yaml
# Example: Placement policy
apiVersion: scheduling.kcp.io/v1alpha1
kind: PlacementPolicy
metadata:
  name: multi-region-policy
spec:
  namespaceSelector:
    matchLabels:
      tier: "production"
  clusterSelector:
    matchLabels:
      environment: "production"
  placement:
    spreadConstraints:
    - maxSkew: 1
      topologyKey: "topology.kubernetes.io/region"
    - maxSkew: 2
      topologyKey: "topology.kubernetes.io/zone"
  resourceQuota:
    hard:
      requests.cpu: "100"
      requests.memory: "200Gi"
```

### Capacity-Based Placement
```yaml
# Example: Placement based on cluster capacity
apiVersion: scheduling.kcp.io/v1alpha1
kind: Placement
metadata:
  name: high-compute-placement
spec:
  clusterSelector:
    matchExpressions:
    - key: "capacity.cpu"
      operator: GreaterThan
      values: ["1000"]
    - key: "capacity.memory"
      operator: GreaterThan
      values: ["2000Gi"]
  constraints:
    resources:
      requests:
        cpu: "500m"
        memory: "1Gi"
```

## Testing and Validation Examples

### Integration Test Suite
```bash
#!/bin/bash
# TMC integration test script

# Test 1: Basic sync functionality
echo "Testing basic sync..."
kubectl apply -f test-deployment.yaml
wait_for_sync "test-deployment" "target-cluster"

# Test 2: Multi-cluster deployment
echo "Testing multi-cluster deployment..."
kubectl apply -f multi-cluster-app.yaml
verify_deployment_in_clusters "multi-cluster-app" "cluster-1,cluster-2"

# Test 3: Failover scenarios
echo "Testing failover..."
simulate_cluster_failure "cluster-1"
verify_workload_migration "cluster-1" "cluster-2"

# Test 4: Health monitoring
echo "Testing health monitoring..."
verify_health_status "all-components"

echo "All tests passed!"
```

### Performance Testing
```yaml
# Example: Load test configuration
apiVersion: batch/v1
kind: Job
metadata:
  name: tmc-load-test
spec:
  template:
    spec:
      containers:
      - name: load-generator
        image: load-tester:latest
        env:
        - name: TARGET_CLUSTERS
          value: "5"
        - name: RESOURCES_PER_CLUSTER
          value: "100"
        - name: SYNC_RATE
          value: "10/s"
        command:
        - /bin/sh
        - -c
        - |
          echo "Starting TMC load test..."
          
          # Generate test workloads
          for i in $(seq 1 $RESOURCES_PER_CLUSTER); do
            generate_test_deployment "test-app-$i"
            kubectl apply -f "test-app-$i.yaml"
            sleep 0.1  # Rate limiting
          done
          
          # Monitor sync performance
          monitor_sync_metrics
          
          echo "Load test completed"
```

## Troubleshooting Examples

### Common Issues and Solutions

#### Issue: Syncer Connection Problems
```bash
# Diagnostic commands
kubectl get synctargets -o yaml | grep -A10 conditions
kubectl describe synctarget my-cluster

# Check connectivity
curl -k https://kcp-endpoint/api/v1/namespaces
curl -k https://cluster-endpoint/api/v1/namespaces

# Verify RBAC
kubectl auth can-i "*" "*" --as=system:serviceaccount:kcp-system:syncer
```

#### Issue: Resource Sync Failures
```bash
# Check sync status
kubectl get all -A -o json | jq '.items[] | select(.metadata.annotations["syncer.kcp.io/sync-target"]) | {name: .metadata.name, lastSync: .metadata.annotations["syncer.kcp.io/last-sync"]}'

# Examine events
kubectl get events --field-selector reason=SyncFailed

# Review syncer logs
kubectl logs -f deployment/workload-syncer -n kcp-system
```

### Debug Tools

#### Log Analysis Script
```bash
#!/bin/bash
# analyze-syncer-logs.sh

LOG_FILE=${1:-/var/log/syncer.log}

echo "=== Syncer Log Analysis ==="
echo "Log file: $LOG_FILE"
echo

echo "Error Summary:"
grep "ERROR\|WARN" "$LOG_FILE" | cut -d' ' -f3- | sort | uniq -c | sort -nr

echo
echo "Sync Operations:"
grep "Successfully synced\|Failed to sync" "$LOG_FILE" | wc -l
echo "Success: $(grep "Successfully synced" "$LOG_FILE" | wc -l)"
echo "Failures: $(grep "Failed to sync" "$LOG_FILE" | wc -l)"

echo
echo "Resource Types:"
grep "synced.*gvr=" "$LOG_FILE" | sed 's/.*gvr=\([^ ]*\).*/\1/' | sort | uniq -c

echo
echo "Recent Errors:"
grep "ERROR\|WARN" "$LOG_FILE" | tail -10
```

## Getting Started

1. **Start with [Basic Setup](./syncer/basic-setup.md)** to understand fundamental concepts
2. **Progress to [Multi-Cluster Deployment](./syncer/multi-cluster-deployment.md)** for production scenarios
3. **Explore [Advanced Features](./syncer/advanced-features.md)** for sophisticated use cases
4. **Use the API Reference** for detailed configuration options
5. **Refer to troubleshooting examples** when issues arise

## Contributing Examples

We welcome contributions to improve and expand these examples. When contributing:

1. Include clear prerequisites and setup instructions
2. Provide expected outputs and verification steps
3. Include troubleshooting guidance
4. Test examples in realistic environments
5. Document any special considerations or limitations

## Additional Resources

- [TMC Architecture Documentation](../architecture.md)
- [API Reference](../syncer-api-reference.md)
- [Development Guide](../development.md)
- [Troubleshooting Guide](../troubleshooting.md)