# Basic Syncer Setup Example

This example demonstrates how to set up a basic syncer configuration to synchronize workloads between KCP and a physical Kubernetes cluster.

## Prerequisites

- KCP running with TMC components
- A target Kubernetes cluster with appropriate permissions
- `kubectl` configured for both KCP and target cluster

## Step 1: Create a SyncTarget

Create a SyncTarget resource in your KCP workspace:

```yaml
# synctarget.yaml
apiVersion: workload.kcp.io/v1alpha1
kind: SyncTarget
metadata:
  name: dev-cluster
  labels:
    environment: development
    region: us-east-1
spec:
  workloadCluster:
    name: "development-cluster"
    endpoint: "https://dev-k8s.example.com"
  supportedAPIExports:
  - export: "kubernetes"
    resource: "apps/v1/deployments"
  - export: "kubernetes"
    resource: "v1/services"
  - export: "kubernetes"
    resource: "v1/configmaps"
  - export: "kubernetes"
    resource: "v1/secrets"
  capabilities:
  - type: "compute"
    resource: "cpu"
    capacity: "100"
  - type: "compute"
    resource: "memory"
    capacity: "200Gi"
  - type: "storage"
    resource: "disk"
    capacity: "1Ti"
```

Apply the SyncTarget:

```bash
kubectl apply -f synctarget.yaml
```

## Step 2: Build and Configure the Syncer

Build the syncer binary:

```bash
go build -o workload-syncer ./cmd/workload-syncer
```

Create a configuration script:

```bash
#!/bin/bash
# run-syncer.sh

SYNC_TARGET_NAME="dev-cluster"
SYNC_TARGET_UID=$(kubectl get synctarget ${SYNC_TARGET_NAME} -o jsonpath='{.metadata.uid}')
WORKSPACE_CLUSTER="root:development"
KCP_KUBECONFIG="${HOME}/.kcp/admin.kubeconfig"
CLUSTER_KUBECONFIG="${HOME}/.kube/dev-cluster-config"

./workload-syncer \
  --sync-target-name=${SYNC_TARGET_NAME} \
  --sync-target-uid=${SYNC_TARGET_UID} \
  --workspace-cluster=${WORKSPACE_CLUSTER} \
  --kcp-kubeconfig=${KCP_KUBECONFIG} \
  --cluster-kubeconfig=${CLUSTER_KUBECONFIG} \
  --resync-period=30s \
  --workers=2 \
  --heartbeat-period=30s \
  --v=2
```

Make the script executable and run it:

```bash
chmod +x run-syncer.sh
./run-syncer.sh
```

## Step 3: Deploy a Sample Application

Create a simple nginx deployment:

```yaml
# nginx-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-app
  namespace: default
  labels:
    app: nginx
spec:
  replicas: 2
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
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 200m
            memory: 256Mi
---
apiVersion: v1
kind: Service
metadata:
  name: nginx-service
  namespace: default
spec:
  selector:
    app: nginx
  ports:
  - port: 80
    targetPort: 80
  type: ClusterIP
```

Apply to KCP:

```bash
kubectl apply -f nginx-deployment.yaml --context=kcp
```

## Step 4: Verify Synchronization

Check that resources were created in the physical cluster:

```bash
# Check deployment in physical cluster
kubectl get deployments nginx-app --context=dev-cluster

# Check service in physical cluster
kubectl get service nginx-service --context=dev-cluster

# Check pods are running
kubectl get pods -l app=nginx --context=dev-cluster
```

Verify status updates in KCP:

```bash
# Check deployment status in KCP
kubectl get deployment nginx-app -o yaml --context=kcp

# Check service status in KCP
kubectl get service nginx-service -o yaml --context=kcp
```

## Step 5: Monitor Syncer Health

Check SyncTarget status:

```bash
kubectl get synctarget dev-cluster -o yaml
```

You should see conditions like:

```yaml
status:
  conditions:
  - type: Ready
    status: "True"
    reason: SyncTargetReady
    message: "SyncTarget is ready and operational"
  - type: SyncerReady
    status: "True"
    reason: SyncerConnected
    message: "Syncer is connected and sending heartbeats"
  - type: HeartbeatReady
    status: "True"
    reason: HeartbeatReceived
    message: "Heartbeat received at 2024-01-15T10:30:00Z"
```

## Step 6: Test Resource Updates

Update the deployment to scale up:

```bash
kubectl scale deployment nginx-app --replicas=4 --context=kcp
```

Verify the change is synchronized:

```bash
# Check replicas in physical cluster
kubectl get deployment nginx-app -o jsonpath='{.spec.replicas}' --context=dev-cluster

# Check that pods are created
kubectl get pods -l app=nginx --context=dev-cluster
```

## Step 7: Test ConfigMap Synchronization

Create a ConfigMap in KCP:

```yaml
# nginx-config.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: nginx-config
  namespace: default
data:
  nginx.conf: |
    server {
        listen 80;
        location / {
            return 200 'Hello from KCP synced nginx!';
            add_header Content-Type text/plain;
        }
    }
  log_level: "info"
```

Apply and verify:

```bash
kubectl apply -f nginx-config.yaml --context=kcp
kubectl get configmap nginx-config --context=dev-cluster
```

## Cleanup

To clean up the example:

```bash
# Delete resources from KCP (will be removed from cluster automatically)
kubectl delete -f nginx-deployment.yaml --context=kcp
kubectl delete -f nginx-config.yaml --context=kcp

# Stop the syncer
# (Ctrl+C in the terminal running the syncer)

# Optionally, delete the SyncTarget
kubectl delete synctarget dev-cluster
```

## Expected Syncer Output

When running successfully, you should see log output like:

```
I0115 10:30:00.123456       1 main.go:XXX] Starting workload syncer syncTarget=dev-cluster workspace=root:development
I0115 10:30:00.234567       1 engine.go:XXX] Successfully created syncer engine
I0115 10:30:00.345678       1 status_reporter.go:XXX] Starting status reporter
I0115 10:30:00.456789       1 engine.go:XXX] Discovered syncable resources count=4
I0115 10:30:00.567890       1 engine.go:XXX] Successfully started resource controllers count=4
I0115 10:30:00.678901       1 syncer.go:XXX] Syncer started successfully
I0115 10:30:05.123456       1 resource_controller.go:XXX] Successfully synced resource to cluster key=default/nginx-app gvr=apps/v1/deployments
I0115 10:30:05.234567       1 resource_controller.go:XXX] Successfully synced resource to cluster key=default/nginx-service gvr=v1/services
I0115 10:30:30.345678       1 status_reporter.go:XXX] Heartbeat sent successfully
```

## Troubleshooting

### Issue: Syncer Won't Start

**Check SyncTarget exists:**
```bash
kubectl get synctarget dev-cluster
```

**Verify kubeconfig files:**
```bash
kubectl cluster-info --kubeconfig ~/.kcp/admin.kubeconfig
kubectl cluster-info --kubeconfig ~/.kube/dev-cluster-config
```

### Issue: Resources Not Syncing

**Check syncer logs with increased verbosity:**
```bash
./run-syncer.sh # but change --v=2 to --v=4
```

**Check RBAC permissions in target cluster:**
```bash
kubectl auth can-i create deployments --context=dev-cluster
kubectl auth can-i update deployments --context=dev-cluster
```

### Issue: Status Not Updating

**Check if status subresource is supported:**
```bash
kubectl get deployment nginx-app -o yaml --context=dev-cluster | grep -A10 status
```

**Verify network connectivity:**
```bash
# From syncer machine to both clusters
curl -k https://kcp.example.com
curl -k https://dev-k8s.example.com
```

This basic setup provides a foundation for understanding how the syncer works and can be extended for more complex scenarios.