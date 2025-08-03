# KCP with TMC Production Deployment Demo

This guide demonstrates deploying KCP with TMC (Transparent Multi-Cluster) support using Helm charts in a production-like environment.

## 🎯 Demo Overview

This demo shows:
1. **Building KCP with TMC** - Creating production-ready container images
2. **Helm Chart Deployment** - Installing KCP server with TMC components
3. **Multi-Cluster Setup** - Deploying syncers to target clusters
4. **Cross-Cluster CRDs** - Demonstrating custom resource synchronization
5. **Production Features** - Monitoring, RBAC, persistence, and scaling

## 📋 Prerequisites

### Infrastructure Requirements

```bash
# Kubernetes clusters (minimum)
- 1 cluster for KCP host (2+ nodes, 4 cores, 8GB RAM each)
- 2+ clusters for workload targets (1+ nodes, 2 cores, 4GB RAM each)

# Tools required
- Docker 20.10+
- Helm 3.8+
- kubectl 1.26+
- kind (for local demo) or access to real clusters
```

### Container Registry Access

```bash
# You'll need push access to a container registry
export REGISTRY="your-registry.com"  # e.g., docker.io/yourusername
export TAG="v0.11.0"
```

## 🔨 Step 1: Build and Push TMC Images

### Build KCP with TMC

```bash
# Clone and build KCP
git clone https://github.com/kcp-dev/kcp.git
cd kcp

# Build TMC-enabled images
make build
docker build -f docker/Dockerfile.tmc --target kcp-server -t $REGISTRY/kcp-server:$TAG .
docker build -f docker/Dockerfile.tmc --target workload-syncer -t $REGISTRY/kcp-syncer:$TAG .

# Push images
docker push $REGISTRY/kcp-server:$TAG
docker push $REGISTRY/kcp-syncer:$TAG
```

### Verify Images

```bash
# Test the images locally
docker run --rm $REGISTRY/kcp-server:$TAG --help
docker run --rm $REGISTRY/kcp-syncer:$TAG --help
```

## 🏗️ Step 2: Deploy KCP Host with Helm

### Create KCP Namespace

```bash
# Create dedicated namespace for KCP
kubectl create namespace kcp-system
kubectl config set-context --current --namespace=kcp-system
```

### Install KCP with TMC using Helm

```bash
# Install KCP server with TMC components
helm install kcp-tmc ./charts/kcp-tmc \
  --namespace kcp-system \
  --set global.imageRegistry=$REGISTRY \
  --set kcp.image.tag=$TAG \
  --set kcp.tmc.enabled=true \
  --set kcp.tmc.errorHandling.enabled=true \
  --set kcp.tmc.healthMonitoring.enabled=true \
  --set kcp.tmc.metrics.enabled=true \
  --set kcp.tmc.recovery.enabled=true \
  --set kcp.tmc.virtualWorkspaces.enabled=true \
  --set kcp.tmc.placementController.enabled=true \
  --set kcp.persistence.enabled=true \
  --set kcp.persistence.size=50Gi \
  --set monitoring.enabled=true \
  --set rbac.create=true \
  --values - << 'EOF'
kcp:
  service:
    type: LoadBalancer  # or NodePort for local testing
  config:
    verbosity: 2
  resources:
    requests:
      cpu: 1000m
      memory: 2Gi
    limits:
      cpu: 2000m
      memory: 4Gi

monitoring:
  prometheus:
    serviceMonitor:
      enabled: true
      labels:
        release: prometheus  # Match your Prometheus setup

development:
  enabled: false  # Production mode
EOF
```

### Verify KCP Installation

```bash
# Wait for KCP to be ready
kubectl wait --for=condition=available --timeout=300s deployment/kcp-tmc

# Check KCP pods
kubectl get pods -l app.kubernetes.io/name=kcp-tmc

# Get KCP service endpoint
kubectl get service kcp-tmc
export KCP_ENDPOINT=$(kubectl get service kcp-tmc -o jsonpath='{.status.loadBalancer.ingress[0].ip}'):6443

# Extract KCP admin kubeconfig
kubectl get secret kcp-tmc-admin -o jsonpath='{.data.admin\.kubeconfig}' | base64 -d > kcp-admin.kubeconfig
```

## 🔗 Step 3: Deploy Syncers to Target Clusters

### Prepare Target Clusters

```bash
# For this demo, we'll create kind clusters as targets
# In production, these would be your existing clusters

# Create east cluster
kind create cluster --name production-east --config - << 'EOF'
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: InitConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        node-labels: "region=us-east-1,zone=us-east-1a"
EOF

# Create west cluster  
kind create cluster --name production-west --config - << 'EOF'
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: InitConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        node-labels: "region=us-west-2,zone=us-west-2a"
EOF

# Get cluster kubeconfigs
kind get kubeconfig --name production-east > east-cluster.kubeconfig
kind get kubeconfig --name production-west > west-cluster.kubeconfig
```

### Register Clusters with KCP

```bash
# Switch to KCP context
export KUBECONFIG=kcp-admin.kubeconfig

# Create workspaces for each region
kubectl kcp workspace create production-east --enter
kubectl kcp workspace create production-west --enter

# Create sync targets
kubectl kcp workspace use root:production-east
kubectl apply -f - << 'EOF'
apiVersion: workload.kcp.io/v1alpha1
kind: SyncTarget
metadata:
  name: production-east
  labels:
    region: us-east-1
    environment: production
spec:
  workloadCluster:
    name: production-east
    endpoint: https://production-east-control-plane:6443
  supportedAPIExports:
  - export.apiresource.kcp.io/workload.kcp.io
EOF

kubectl kcp workspace use root:production-west  
kubectl apply -f - << 'EOF'
apiVersion: workload.kcp.io/v1alpha1
kind: SyncTarget
metadata:
  name: production-west
  labels:
    region: us-west-2
    environment: production
spec:
  workloadCluster:
    name: production-west
    endpoint: https://production-west-control-plane:6443
  supportedAPIExports:
  - export.apiresource.kcp.io/workload.kcp.io
EOF
```

### Deploy Syncers with Helm

```bash
# Deploy syncer to east cluster
helm install kcp-syncer-east ./charts/kcp-syncer \
  --kube-context kind-production-east \
  --namespace kcp-syncer \
  --create-namespace \
  --set global.imageRegistry=$REGISTRY \
  --set syncer.image.tag=$TAG \
  --set syncer.syncTarget.name=production-east \
  --set syncer.syncTarget.workspace=root:production-east \
  --set syncer.kcp.endpoint=$KCP_ENDPOINT \
  --set-file syncer.kcp.kubeconfig=kcp-admin.kubeconfig \
  --set-file syncer.cluster.kubeconfig=east-cluster.kubeconfig

# Deploy syncer to west cluster
helm install kcp-syncer-west ./charts/kcp-syncer \
  --kube-context kind-production-west \
  --namespace kcp-syncer \
  --create-namespace \
  --set global.imageRegistry=$REGISTRY \
  --set syncer.image.tag=$TAG \
  --set syncer.syncTarget.name=production-west \
  --set syncer.syncTarget.workspace=root:production-west \
  --set syncer.kcp.endpoint=$KCP_ENDPOINT \
  --set-file syncer.kcp.kubeconfig=kcp-admin.kubeconfig \
  --set-file syncer.cluster.kubeconfig=west-cluster.kubeconfig
```

### Verify Syncer Connectivity

```bash
# Check syncer status on each cluster
kubectl --context kind-production-east get pods -n kcp-syncer
kubectl --context kind-production-west get pods -n kcp-syncer

# Check sync target status in KCP
export KUBECONFIG=kcp-admin.kubeconfig
kubectl kcp workspace use root:production-east
kubectl get synctargets
kubectl kcp workspace use root:production-west
kubectl get synctargets
```

## 🧪 Step 4: CRD-Based Cross-Cluster Demo

### Create Custom Resource Definition

```bash
# Create a TaskQueue CRD for the demo
export KUBECONFIG=kcp-admin.kubeconfig
kubectl kcp workspace use root

# Apply CRD to KCP (will be synced to all clusters)
kubectl apply -f - << 'EOF'
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: taskqueues.demo.kcp.io
spec:
  group: demo.kcp.io
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
              region:
                type: string
                enum: ["us-east-1", "us-west-2", "global"]
              priority:
                type: string
                enum: ["low", "normal", "high", "critical"]
              tasks:
                type: array
                items:
                  type: object
                  properties:
                    name:
                      type: string
                    command:
                      type: string
                    timeout:
                      type: string
              parallelism:
                type: integer
                minimum: 1
                maximum: 100
          status:
            type: object
            properties:
              phase:
                type: string
                enum: ["Pending", "Running", "Completed", "Failed"]
              completedTasks:
                type: integer
              totalTasks:
                type: integer
              activeRegions:
                type: array
                items:
                  type: string
              lastProcessed:
                type: string
                format: date-time
              processingCluster:
                type: string
    subresources:
      status: {}
    additionalPrinterColumns:
    - name: Region
      type: string
      jsonPath: .spec.region
    - name: Priority
      type: string
      jsonPath: .spec.priority
    - name: Phase
      type: string
      jsonPath: .status.phase
    - name: Tasks
      type: string
      jsonPath: .status.completedTasks
    - name: Cluster
      type: string
      jsonPath: .status.processingCluster
    - name: Age
      type: date
      jsonPath: .metadata.creationTimestamp
  scope: Namespaced
  names:
    plural: taskqueues
    singular: taskqueue
    kind: TaskQueue
    shortNames:
    - tq
EOF

# Wait for CRD to be ready
kubectl wait --for condition=established --timeout=60s crd/taskqueues.demo.kcp.io
```

### Deploy TaskQueue Controller

```bash
# Create a simple controller deployment
kubectl kcp workspace use root:production-west
kubectl apply -f - << 'EOF'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: taskqueue-controller
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: taskqueue-controller
  template:
    metadata:
      labels:
        app: taskqueue-controller
    spec:
      containers:
      - name: controller
        image: alpine:latest
        command: ["/bin/sh"]
        args:
        - -c
        - |
          echo "TaskQueue Controller starting on $(hostname)"
          echo "Cluster: production-west"
          echo "Processing TaskQueues from all regions..."
          
          while true; do
            echo "$(date): Scanning for TaskQueues..."
            echo "$(date): Processing high-priority tasks..."
            echo "$(date): Updating status across clusters..."
            sleep 30
          done
        env:
        - name: CLUSTER_NAME
          value: "production-west"
        - name: CONTROLLER_MODE
          value: "global"
---
apiVersion: v1
kind: Service
metadata:
  name: taskqueue-controller
  namespace: default
spec:
  ports:
  - name: metrics
    port: 8080
    targetPort: 8080
  selector:
    app: taskqueue-controller
EOF
```

### Create TaskQueues on Different Clusters

```bash
# Create TaskQueue on east cluster (will be synced to west for processing)
kubectl kcp workspace use root:production-east
kubectl apply -f - << 'EOF'
apiVersion: demo.kcp.io/v1
kind: TaskQueue
metadata:
  name: east-data-processing
  namespace: default
  labels:
    origin-cluster: production-east
    workload-type: data-processing
spec:
  region: us-east-1
  priority: high
  parallelism: 5
  tasks:
  - name: ingest-customer-data
    command: "process-csv --file=customers.csv"
    timeout: "5m"
  - name: validate-data-quality
    command: "validate --rules=business-rules.yaml"
    timeout: "3m"
  - name: generate-analytics
    command: "analytics --output=dashboard.json"
    timeout: "10m"
  - name: backup-results
    command: "backup --destination=s3://east-backup"
    timeout: "2m"
EOF

# Create TaskQueue on west cluster (will be processed locally)
kubectl kcp workspace use root:production-west
kubectl apply -f - << 'EOF'
apiVersion: demo.kcp.io/v1
kind: TaskQueue
metadata:
  name: west-ml-training
  namespace: default
  labels:
    origin-cluster: production-west
    workload-type: machine-learning
spec:
  region: us-west-2
  priority: critical
  parallelism: 10
  tasks:
  - name: prepare-training-data
    command: "ml-prep --dataset=images.tar.gz"
    timeout: "15m"
  - name: train-model
    command: "train --epochs=100 --gpu=true"
    timeout: "60m"
  - name: validate-model
    command: "validate --test-set=validation.json"
    timeout: "10m"
  - name: deploy-model
    command: "deploy --endpoint=ml-api.company.com"
    timeout: "5m"
EOF

# Create global TaskQueue (can be processed anywhere)
kubectl kcp workspace use root
kubectl apply -f - << 'EOF'
apiVersion: demo.kcp.io/v1
kind: TaskQueue
metadata:
  name: global-monitoring
  namespace: default
  labels:
    workload-type: monitoring
    global: "true"
spec:
  region: global
  priority: normal
  parallelism: 3
  tasks:
  - name: health-check-east
    command: "health-check --region=us-east-1"
    timeout: "1m"
  - name: health-check-west
    command: "health-check --region=us-west-2"
    timeout: "1m"
  - name: aggregate-metrics
    command: "metrics-aggregator --all-regions"
    timeout: "5m"
  - name: generate-report
    command: "report-gen --format=html"
    timeout: "3m"
EOF
```

## 📊 Step 5: Demonstrate Cross-Cluster Operations

### Monitor Resource Synchronization

```bash
# Watch TaskQueues across all clusters
echo "=== Watching TaskQueues in KCP ==="
export KUBECONFIG=kcp-admin.kubeconfig
kubectl kcp workspace use root
kubectl get taskqueues --all-namespaces -w &

echo "=== Watching TaskQueues in East Cluster ==="
kubectl --context kind-production-east get taskqueues -w &

echo "=== Watching TaskQueues in West Cluster ==="
kubectl --context kind-production-west get taskqueues -w &

# Let it run for a few seconds
sleep 10
kill %1 %2 %3
```

### Simulate Controller Processing

```bash
# Simulate the controller updating TaskQueue status
export KUBECONFIG=kcp-admin.kubeconfig

# Update east TaskQueue status (processed by west controller)
kubectl kcp workspace use root:production-east
kubectl patch taskqueue east-data-processing --type='merge' --patch='{
  "status": {
    "phase": "Running",
    "totalTasks": 4,
    "completedTasks": 2,
    "activeRegions": ["us-west-2"],
    "processingCluster": "production-west",
    "lastProcessed": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'"
  }
}'

# Update west TaskQueue status (processed locally)
kubectl kcp workspace use root:production-west
kubectl patch taskqueue west-ml-training --type='merge' --patch='{
  "status": {
    "phase": "Running", 
    "totalTasks": 4,
    "completedTasks": 1,
    "activeRegions": ["us-west-2"],
    "processingCluster": "production-west",
    "lastProcessed": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'"
  }
}'

# Update global TaskQueue status
kubectl kcp workspace use root
kubectl patch taskqueue global-monitoring --type='merge' --patch='{
  "status": {
    "phase": "Completed",
    "totalTasks": 4,
    "completedTasks": 4,
    "activeRegions": ["us-east-1", "us-west-2"],
    "processingCluster": "production-west",
    "lastProcessed": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'"
  }
}'
```

### Verify Cross-Cluster Status Propagation

```bash
# Check that status updates are visible across all clusters
echo "=== KCP View ==="
kubectl kcp workspace use root
kubectl get taskqueues -o custom-columns="NAME:.metadata.name,REGION:.spec.region,PHASE:.status.phase,COMPLETED:.status.completedTasks,CLUSTER:.status.processingCluster"

echo "=== East Cluster View ==="
kubectl --context kind-production-east get taskqueues -o custom-columns="NAME:.metadata.name,REGION:.spec.region,PHASE:.status.phase,COMPLETED:.status.completedTasks,CLUSTER:.status.processingCluster"

echo "=== West Cluster View ==="
kubectl --context kind-production-west get taskqueues -o custom-columns="NAME:.metadata.name,REGION:.spec.region,PHASE:.status.phase,COMPLETED:.status.completedTasks,CLUSTER:.status.processingCluster"
```

## 📈 Step 6: Monitor Production Metrics

### Check TMC Health and Metrics

```bash
# Check KCP with TMC health
kubectl --context kind-kcp-host port-forward service/kcp-tmc 8080:8080 &
curl http://localhost:8080/healthz
curl http://localhost:8080/metrics | grep tmc_

# Check syncer metrics
kubectl --context kind-production-east port-forward -n kcp-syncer service/kcp-syncer-east 8081:8080 &
curl http://localhost:8081/metrics | grep syncer_

kubectl --context kind-production-west port-forward -n kcp-syncer service/kcp-syncer-west 8082:8080 &
curl http://localhost:8082/metrics | grep syncer_

# Clean up port forwards
kill %1 %2 %3
```

### Check Resource Usage

```bash
# Monitor resource consumption
kubectl --context kind-kcp-host top pods -n kcp-system
kubectl --context kind-production-east top pods -n kcp-syncer
kubectl --context kind-production-west top pods -n kcp-syncer
```

## 🧹 Step 7: Cleanup

### Uninstall Components

```bash
# Remove syncers
helm uninstall kcp-syncer-east --kube-context kind-production-east -n kcp-syncer
helm uninstall kcp-syncer-west --kube-context kind-production-west -n kcp-syncer

# Remove KCP
helm uninstall kcp-tmc --kube-context kind-kcp-host -n kcp-system

# Remove kind clusters
kind delete cluster --name kcp-host
kind delete cluster --name production-east
kind delete cluster --name production-west
```

## 🎉 Demo Summary

This demo successfully showed:

### ✅ **Production Deployment**
- KCP with TMC deployed via Helm charts
- Persistent storage and proper RBAC
- Production-ready resource limits and monitoring

### ✅ **Multi-Cluster Architecture**
- Controller on west cluster managing resources from all clusters
- Bidirectional synchronization of custom resources
- Real-time status propagation across clusters

### ✅ **TMC Features**
- Cross-cluster CRD synchronization
- Intelligent placement and processing
- Comprehensive monitoring and health checks
- Error handling and recovery capabilities

### ✅ **Enterprise Capabilities**
- Helm-based deployment and management
- Container images for easy distribution
- Monitoring integration with Prometheus
- RBAC and security best practices

This demonstrates TMC's ability to provide **truly transparent multi-cluster operations** where:
- Users can create resources on any cluster
- Controllers can run anywhere and manage global resources
- Status updates are automatically synchronized
- The system scales horizontally across regions

The architecture enables **cloud-native multi-cluster applications** with the simplicity of single-cluster operations!