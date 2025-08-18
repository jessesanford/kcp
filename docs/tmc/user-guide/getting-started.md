# Getting Started with TMC

This guide walks you through setting up your first TMC environment, from installation to deploying your first multi-cluster workload.

## Prerequisites

- KCP server installed and running (see [KCP Setup Guide](../../setup/quickstart.md))
- At least one Kubernetes cluster for workload placement
- `kubectl` configured with access to your target clusters
- `kubectl-kcp` plugin installed

## Installation

### Enable TMC Feature

TMC is controlled by feature flags. Enable it in your KCP configuration:

```bash
# Enable TMC features
kcp start --feature-gates=TMCEnabled=true,WorkloadPlacement=true
```

### Verify TMC APIs

Check that TMC APIs are available:

```bash
kubectl api-resources --api-group=workload.kcp.io
kubectl api-resources --api-group=placement.kcp.io
```

Expected output:
```
NAME                    SHORTNAMES   APIVERSION                   NAMESPACED   KIND
clusterregistrations    clusters     workload.kcp.io/v1alpha1     false        ClusterRegistration
workloadplacements      wlp          placement.kcp.io/v1alpha1    true         WorkloadPlacement
```

## Cluster Registration

### Prepare Target Cluster

Ensure your target cluster meets the requirements:

```bash
# Check cluster version (1.24+ recommended)
kubectl version --short

# Verify cluster health
kubectl get nodes
kubectl get pods -A | grep -v Running
```

### Create ClusterRegistration

Register your first cluster:

```bash
cat <<EOF | kubectl apply -f -
apiVersion: workload.kcp.io/v1alpha1
kind: ClusterRegistration
metadata:
  name: production-us-west
spec:
  location: "us-west-2"
  kubeconfig:
    secretRef:
      name: cluster-kubeconfig
      key: kubeconfig
  capabilities:
    - networking
    - storage
    - compute
  resources:
    cpu: "100"
    memory: "400Gi"
    storage: "1Ti"
EOF
```

### Create Cluster Access Secret

Store the target cluster's kubeconfig:

```bash
# Create secret with cluster kubeconfig
kubectl create secret generic cluster-kubeconfig \
  --from-file=kubeconfig=/path/to/target-cluster-kubeconfig.yaml
```

### Verify Registration

Check cluster registration status:

```bash
# View cluster status
kubectl get clusterregistrations
kubectl describe clusterregistration production-us-west

# Check for Ready condition
kubectl get clusterregistration production-us-west -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}'
```

## Workspace Setup

### Create TMC Workspace

Create a workspace for your TMC workloads:

```bash
# Create workspace
kubectl kcp workspace create my-app --type universal

# Switch to the workspace
kubectl kcp workspace use my-app
```

### Bind TMC APIs

Bind necessary APIs to your workspace:

```bash
# Bind workload placement API
cat <<EOF | kubectl apply -f -
apiVersion: apis.kcp.io/v1alpha1
kind: APIBinding
metadata:
  name: placement-api
spec:
  reference:
    export:
      path: root:compute
      name: placement.kcp.io
EOF

# Verify binding
kubectl get apibindings
```

## Configuration

### Create Default Placement Policy

Set up a basic placement policy:

```bash
cat <<EOF | kubectl apply -f -
apiVersion: placement.kcp.io/v1alpha1
kind: WorkloadPlacement
metadata:
  name: default-placement
spec:
  selector:
    matchLabels:
      app.kubernetes.io/managed-by: tmc
  placementPolicy:
    clusters:
    - name: production-us-west
      weight: 100
    constraints:
      resources:
        cpu: "100m"
        memory: "128Mi"
EOF
```

## Validation

### Deploy Test Workload

Create a simple test deployment:

```bash
cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app
  labels:
    app.kubernetes.io/managed-by: tmc
spec:
  replicas: 2
  selector:
    matchLabels:
      app: test-app
  template:
    metadata:
      labels:
        app: test-app
        app.kubernetes.io/managed-by: tmc
    spec:
      containers:
      - name: web
        image: nginx:1.21
        ports:
        - containerPort: 80
        resources:
          requests:
            cpu: "100m"
            memory: "128Mi"
          limits:
            cpu: "200m"
            memory: "256Mi"
EOF
```

### Verify Deployment

Check that the workload was placed correctly:

```bash
# Check deployment status
kubectl get deployment test-app
kubectl get pods -l app=test-app

# Verify placement occurred
kubectl describe workloadplacement default-placement

# Check on target cluster
kubectl --kubeconfig=/path/to/target-cluster-kubeconfig.yaml get pods -l app=test-app
```

## Next Steps

Congratulations! You've successfully:

1. ✅ Enabled TMC in your KCP installation
2. ✅ Registered your first cluster
3. ✅ Set up a workspace with TMC APIs
4. ✅ Created placement policies
5. ✅ Deployed your first multi-cluster workload

### Continue Learning

- [Basic Usage Examples](basic-usage.md) - Common patterns and workflows
- [API Reference](../api-reference/) - Detailed API documentation
- [Troubleshooting](../troubleshooting/) - Common issues and solutions

### Advanced Topics

- Adding multiple clusters for high availability
- Implementing custom placement policies
- Setting up monitoring and alerting
- Configuring disaster recovery scenarios