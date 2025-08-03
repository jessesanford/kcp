# Multi-Cluster Deployment Example

This example demonstrates how to deploy the same application across multiple clusters using the TMC syncer system, with intelligent placement and health monitoring.

## Scenario

We'll deploy a web application that needs to run in multiple regions for high availability:
- **Production cluster** (us-west-2): Primary deployment
- **Staging cluster** (us-east-1): Testing and failover
- **Edge cluster** (eu-west-1): European users

## Prerequisites

- KCP with TMC components running
- Three target Kubernetes clusters configured
- Network connectivity between clusters for load balancing

## Step 1: Setup Multiple SyncTargets

### Production Cluster SyncTarget

```yaml
# synctarget-production.yaml
apiVersion: workload.kcp.io/v1alpha1
kind: SyncTarget
metadata:
  name: prod-us-west-2
  labels:
    environment: production
    region: us-west-2
    tier: primary
spec:
  workloadCluster:
    name: "production-us-west-2"
    endpoint: "https://prod-k8s-west.example.com"
  capabilities:
  - type: "compute"
    resource: "cpu"
    capacity: "1000"
  - type: "compute"
    resource: "memory"
    capacity: "2000Gi"
  - type: "network"
    resource: "bandwidth"
    capacity: "10Gbps"
  cells:
    zone: "us-west-2a"
    rack: "rack-1"
```

### Staging Cluster SyncTarget

```yaml
# synctarget-staging.yaml
apiVersion: workload.kcp.io/v1alpha1
kind: SyncTarget
metadata:
  name: staging-us-east-1
  labels:
    environment: staging
    region: us-east-1
    tier: secondary
spec:
  workloadCluster:
    name: "staging-us-east-1"
    endpoint: "https://staging-k8s-east.example.com"
  capabilities:
  - type: "compute"
    resource: "cpu"
    capacity: "500"
  - type: "compute"
    resource: "memory"
    capacity: "1000Gi"
  cells:
    zone: "us-east-1a"
    rack: "rack-1"
```

### Edge Cluster SyncTarget

```yaml
# synctarget-edge.yaml
apiVersion: workload.kcp.io/v1alpha1
kind: SyncTarget
metadata:
  name: edge-eu-west-1
  labels:
    environment: production
    region: eu-west-1
    tier: edge
spec:
  workloadCluster:
    name: "edge-eu-west-1"
    endpoint: "https://edge-k8s-eu.example.com"
  capabilities:
  - type: "compute"
    resource: "cpu"
    capacity: "200"
  - type: "compute"
    resource: "memory"
    capacity: "400Gi"
  cells:
    zone: "eu-west-1a"
    rack: "edge-rack-1"
```

Apply all SyncTargets:

```bash
kubectl apply -f synctarget-production.yaml
kubectl apply -f synctarget-staging.yaml
kubectl apply -f synctarget-edge.yaml
```

## Step 2: Deploy Syncers for Each Cluster

### Production Syncer

```bash
#!/bin/bash
# run-syncer-production.sh

./workload-syncer \
  --sync-target-name=prod-us-west-2 \
  --sync-target-uid=$(kubectl get synctarget prod-us-west-2 -o jsonpath='{.metadata.uid}') \
  --workspace-cluster=root:multi-cluster \
  --kcp-kubeconfig=~/.kcp/admin.kubeconfig \
  --cluster-kubeconfig=~/.kube/prod-west-config \
  --workers=4 \
  --heartbeat-period=15s \
  --v=2 &

echo "Production syncer started with PID $!"
echo $! > syncer-prod.pid
```

### Staging Syncer

```bash
#!/bin/bash
# run-syncer-staging.sh

./workload-syncer \
  --sync-target-name=staging-us-east-1 \
  --sync-target-uid=$(kubectl get synctarget staging-us-east-1 -o jsonpath='{.metadata.uid}') \
  --workspace-cluster=root:multi-cluster \
  --kcp-kubeconfig=~/.kcp/admin.kubeconfig \
  --cluster-kubeconfig=~/.kube/staging-east-config \
  --workers=2 \
  --heartbeat-period=30s \
  --v=2 &

echo "Staging syncer started with PID $!"
echo $! > syncer-staging.pid
```

### Edge Syncer

```bash
#!/bin/bash
# run-syncer-edge.sh

./workload-syncer \
  --sync-target-name=edge-eu-west-1 \
  --sync-target-uid=$(kubectl get synctarget edge-eu-west-1 -o jsonpath='{.metadata.uid}') \
  --workspace-cluster=root:multi-cluster \
  --kcp-kubeconfig=~/.kcp/admin.kubeconfig \
  --cluster-kubeconfig=~/.kube/edge-eu-config \
  --workers=1 \
  --heartbeat-period=30s \
  --v=2 &

echo "Edge syncer started with PID $!"
echo $! > syncer-edge.pid
```

Start all syncers:

```bash
chmod +x run-syncer-*.sh
./run-syncer-production.sh
./run-syncer-staging.sh
./run-syncer-edge.sh
```

## Step 3: Deploy Multi-Region Application

### Namespace and Configuration

```yaml
# app-namespace.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: web-app
  labels:
    app: multi-region-web
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: app-config
  namespace: web-app
data:
  environment: "multi-cluster"
  log_level: "info"
  database_url: "postgres://global-db.example.com:5432/webapp"
  redis_url: "redis://global-redis.example.com:6379"
---
apiVersion: v1
kind: Secret
metadata:
  name: app-secrets
  namespace: web-app
type: Opaque
data:
  db-password: cGFzc3dvcmQxMjM=  # password123
  api-key: YWJjZGVmZ2hpams=      # abcdefghijk
```

### Application Deployment

```yaml
# web-app-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web-app
  namespace: web-app
  labels:
    app: web-app
    version: v1.0.0
spec:
  replicas: 3
  selector:
    matchLabels:
      app: web-app
  template:
    metadata:
      labels:
        app: web-app
        version: v1.0.0
    spec:
      containers:
      - name: web-app
        image: nginx:1.20
        ports:
        - containerPort: 80
        env:
        - name: ENVIRONMENT
          valueFrom:
            configMapKeyRef:
              name: app-config
              key: environment
        - name: LOG_LEVEL
          valueFrom:
            configMapKeyRef:
              name: app-config
              key: log_level
        - name: DB_PASSWORD
          valueFrom:
            secretKeyRef:
              name: app-secrets
              key: db-password
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 500m
            memory: 512Mi
        livenessProbe:
          httpGet:
            path: /health
            port: 80
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 80
          initialDelaySeconds: 5
          periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: web-app-service
  namespace: web-app
  labels:
    app: web-app
spec:
  selector:
    app: web-app
  ports:
  - port: 80
    targetPort: 80
    name: http
  type: LoadBalancer
```

Apply the application:

```bash
kubectl apply -f app-namespace.yaml
kubectl apply -f web-app-deployment.yaml
```

## Step 4: Verify Multi-Cluster Deployment

Check that the application is deployed to all clusters:

```bash
# Production cluster
kubectl get pods -n web-app --context=prod-west
kubectl get services -n web-app --context=prod-west

# Staging cluster
kubectl get pods -n web-app --context=staging-east
kubectl get services -n web-app --context=staging-east

# Edge cluster
kubectl get pods -n web-app --context=edge-eu
kubectl get services -n web-app --context=edge-eu
```

## Step 5: Monitor Deployment Health

### Check SyncTarget Status

```bash
kubectl get synctargets -o wide

# Detailed status for each
kubectl get synctarget prod-us-west-2 -o yaml | grep -A20 status
kubectl get synctarget staging-us-east-1 -o yaml | grep -A20 status
kubectl get synctarget edge-eu-west-1 -o yaml | grep -A20 status
```

### Monitor Application Status in KCP

```bash
# Check deployment status aggregated from all clusters
kubectl get deployment web-app -n web-app -o yaml

# Monitor pods across all clusters
kubectl get pods -n web-app --watch
```

### Create Health Monitoring Script

```bash
#!/bin/bash
# monitor-health.sh

echo "=== Multi-Cluster Health Monitor ==="
echo

echo "SyncTarget Status:"
kubectl get synctargets -o custom-columns=NAME:.metadata.name,READY:.status.conditions[?@.type==\"Ready\"].status,SYNCER:.status.conditions[?@.type==\"SyncerReady\"].status,HEARTBEAT:.status.conditions[?@.type==\"HeartbeatReady\"].status

echo
echo "Application Deployment Status:"
kubectl get deployment web-app -n web-app -o custom-columns=NAME:.metadata.name,READY:.status.readyReplicas,AVAILABLE:.status.availableReplicas,DESIRED:.spec.replicas

echo
echo "Service LoadBalancer IPs:"
kubectl get service web-app-service -n web-app -o custom-columns=NAME:.metadata.name,TYPE:.spec.type,EXTERNAL-IP:.status.loadBalancer.ingress[0].ip

echo
echo "Recent Events:"
kubectl get events -n web-app --sort-by=.metadata.creationTimestamp | tail -10
```

Run the health monitor:

```bash
chmod +x monitor-health.sh
./monitor-health.sh
```

## Step 6: Test Failover Scenarios

### Simulate Production Cluster Failure

Stop the production syncer to simulate cluster failure:

```bash
kill $(cat syncer-prod.pid)
rm syncer-prod.pid
```

Monitor the impact:

```bash
# Check SyncTarget status
kubectl get synctarget prod-us-west-2 -o yaml | grep -A10 conditions

# Application should still be available on other clusters
kubectl get pods -n web-app --context=staging-east
kubectl get pods -n web-app --context=edge-eu
```

### Restart Production Cluster

Restart the production syncer:

```bash
./run-syncer-production.sh
```

Verify recovery:

```bash
# Wait for syncer to reconnect
sleep 30

# Check that resources are re-synchronized
kubectl get pods -n web-app --context=prod-west
./monitor-health.sh
```

## Step 7: Rolling Updates Across Clusters

Update the application image:

```bash
kubectl patch deployment web-app -n web-app -p '{"spec":{"template":{"spec":{"containers":[{"name":"web-app","image":"nginx:1.21"}]}}}}'
```

Monitor the rolling update across all clusters:

```bash
# Watch rollout status
kubectl rollout status deployment/web-app -n web-app

# Monitor pods in each cluster during update
watch "echo 'Production:'; kubectl get pods -n web-app --context=prod-west; echo 'Staging:'; kubectl get pods -n web-app --context=staging-east; echo 'Edge:'; kubectl get pods -n web-app --context=edge-eu"
```

## Step 8: Scale Applications Based on Region

Scale applications differently per region:

### Production (High Traffic)

```bash
kubectl scale deployment web-app -n web-app --replicas=6
```

### Staging (Lower Traffic)

For environment-specific scaling, you would typically use placement policies or resource selectors. For this example, we'll use annotations to demonstrate:

```yaml
# scaling-policy.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: scaling-policy
  namespace: web-app
  annotations:
    placement.kcp.io/target: "staging-us-east-1"
data:
  staging-replicas: "2"
  edge-replicas: "4"
```

## Step 9: Load Balancing Configuration

Create a global load balancer configuration:

```yaml
# global-loadbalancer.yaml
apiVersion: v1
kind: Service
metadata:
  name: global-web-app
  namespace: web-app
  annotations:
    service.beta.kubernetes.io/external-traffic-policy: "Local"
    service.beta.kubernetes.io/load-balancer-source-ranges: "0.0.0.0/0"
spec:
  selector:
    app: web-app
  ports:
  - port: 80
    targetPort: 80
    name: http
  type: LoadBalancer
  externalTrafficPolicy: Local
```

## Step 10: Monitoring and Observability

### Create Prometheus Monitoring

```yaml
# monitoring.yaml
apiVersion: v1
kind: ServiceMonitor
metadata:
  name: web-app-metrics
  namespace: web-app
spec:
  selector:
    matchLabels:
      app: web-app
  endpoints:
  - port: http
    path: /metrics
    interval: 30s
---
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: web-app-alerts
  namespace: web-app
spec:
  groups:
  - name: web-app.rules
    rules:
    - alert: WebAppDown
      expr: up{job="web-app"} == 0
      for: 1m
      labels:
        severity: critical
      annotations:
        summary: "Web app is down"
        description: "Web app has been down for more than 1 minute"
    - alert: HighErrorRate
      expr: rate(http_requests_total{status=~"5.."}[5m]) > 0.1
      for: 2m
      labels:
        severity: warning
      annotations:
        summary: "High error rate detected"
        description: "Error rate is above 10% for 2 minutes"
```

## Cleanup

To clean up the multi-cluster deployment:

```bash
# Delete application resources
kubectl delete -f web-app-deployment.yaml
kubectl delete -f app-namespace.yaml

# Stop all syncers
kill $(cat syncer-prod.pid) 2>/dev/null || true
kill $(cat syncer-staging.pid) 2>/dev/null || true
kill $(cat syncer-edge.pid) 2>/dev/null || true

# Clean up PID files
rm -f syncer-*.pid

# Delete SyncTargets
kubectl delete synctarget prod-us-west-2
kubectl delete synctarget staging-us-east-1
kubectl delete synctarget edge-eu-west-1
```

## Best Practices Demonstrated

1. **Environment Separation**: Different SyncTargets for production, staging, and edge
2. **Resource Scaling**: Appropriate resource allocation per cluster type
3. **Health Monitoring**: Comprehensive monitoring across all clusters
4. **Failover Testing**: Simulation of cluster failures and recovery
5. **Rolling Updates**: Coordinated updates across multiple clusters
6. **Load Balancing**: Global traffic distribution
7. **Observability**: Monitoring and alerting setup

This example shows how the TMC syncer system enables sophisticated multi-cluster deployments with minimal complexity in the application configuration.