# TMC Hello World Tutorial

This tutorial demonstrates the key features of the TMC (Transparent Multi-Cluster) system using a local setup with kind clusters. You'll learn how to deploy workloads across multiple clusters, see cross-cluster aggregation in action, and observe TMC's automated management capabilities.

## What You'll Learn

- How to set up KCP with TMC on local kind clusters
- How to create multi-cluster placements
- How TMC handles workload distribution and synchronization
- How to observe cross-cluster resource aggregation
- How TMC's recovery and health monitoring work

## Prerequisites

- Docker installed and running
- At least 8GB of available RAM
- Linux or macOS (Windows with WSL2 also works)

## Overview

In this tutorial, we'll:

1. Set up a KCP instance with TMC enabled
2. Create three kind clusters (kcp-host, cluster-east, cluster-west)
3. Deploy a hello-world application across clusters
4. Demonstrate TMC features like aggregation, projection, and recovery

## Architecture

```
┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐
│   kcp-host      │  │  cluster-east   │  │  cluster-west   │
│                 │  │                 │  │                 │
│ ┌─────────────┐ │  │ ┌─────────────┐ │  │ ┌─────────────┐ │
│ │     KCP     │ │  │ │   Syncer    │ │  │ │   Syncer    │ │
│ │    +TMC     │◄┼──┼─┤   Agent     │ │  │ │   Agent     │ │
│ └─────────────┘ │  │ └─────────────┘ │  │ └─────────────┘ │
│                 │  │                 │  │                 │
│                 │  │ ┌─────────────┐ │  │ ┌─────────────┐ │
│                 │  │ │ Hello World │ │  │ │ Hello World │ │
│                 │  │ │   Workload  │ │  │ │   Workload  │ │
│                 │  │ └─────────────┘ │  │ └─────────────┘ │
└─────────────────┘  └─────────────────┘  └─────────────────┘
```

## Step 1: Setup Script

The tutorial includes an automated setup script that:
- Installs required dependencies (kind, kubectl, etc.)
- Creates the kind clusters
- Builds and deploys KCP with TMC
- Sets up syncer agents
- Configures the environment

Run the setup script:

```bash
./scripts/setup-tmc-tutorial.sh
```

## Step 2: Verify the Setup

After the setup completes, verify that everything is running:

```bash
# Check that all clusters are running
kind get clusters

# Check KCP is running
kubectl --kubeconfig=.kcp/admin.kubeconfig get pods -n kcp-system

# Check that syncers are connected
kubectl --kubeconfig=.kcp/admin.kubeconfig get synctargets
```

You should see:
- Three kind clusters: `kcp-host`, `cluster-east`, `cluster-west`
- KCP pods running in the kcp-host cluster
- Two SyncTargets showing as Ready

## Step 3: Create a Workspace

Create a workspace for our hello-world application:

```bash
# Set up environment
export KUBECONFIG=.kcp/admin.kubeconfig

# Create a workspace
kubectl kcp workspace create hello-world --enter

# Verify we're in the workspace
kubectl kcp workspace current
```

## Step 4: Deploy the Hello World Application

Create the hello-world deployment and service:

```bash
# Create the application deployment
cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: hello-world
  labels:
    app: hello-world
spec:
  replicas: 3
  selector:
    matchLabels:
      app: hello-world
  template:
    metadata:
      labels:
        app: hello-world
    spec:
      containers:
      - name: hello-world
        image: nginx:1.21
        ports:
        - containerPort: 80
        env:
        - name: MESSAGE
          value: "Hello from TMC!"
        - name: CLUSTER_NAME
          valueFrom:
            fieldRef:
              fieldPath: spec.nodeName
        volumeMounts:
        - name: html
          mountPath: /usr/share/nginx/html
      initContainers:
      - name: setup
        image: busybox:1.35
        command:
        - sh
        - -c
        - |
          echo "<h1>Hello from TMC!</h1>" > /html/index.html
          echo "<p>Running on cluster: \${CLUSTER_NAME:-unknown}</p>" >> /html/index.html
          echo "<p>Pod: \$(hostname)</p>" >> /html/index.html
          echo "<p>Time: \$(date)</p>" >> /html/index.html
        volumeMounts:
        - name: html
          mountPath: /html
      volumes:
      - name: html
        emptyDir: {}
---
apiVersion: v1
kind: Service
metadata:
  name: hello-world-service
  labels:
    app: hello-world
spec:
  selector:
    app: hello-world
  ports:
  - port: 80
    targetPort: 80
  type: ClusterIP
EOF
```

## Step 5: Create a Placement

Now create a placement to distribute the workload across clusters:

```bash
# Create a placement that schedules to both clusters
cat <<EOF | kubectl apply -f -
apiVersion: scheduling.kcp.io/v1alpha1
kind: Placement
metadata:
  name: hello-world-placement
spec:
  locationSelectors:
  - matchLabels:
      region: east
  - matchLabels:
      region: west
  numberOfClusters: 2
  namespaceSelector:
    matchNames:
    - default
EOF
```

## Step 6: Observe TMC in Action

Now let's see TMC's features in action:

### 6.1 Check Workload Distribution

```bash
# Check the placement status
kubectl get placement hello-world-placement -o yaml

# Check sync targets
kubectl get synctargets

# Verify workloads are deployed to target clusters
echo "=== Checking cluster-east ==="
kubectl --kubeconfig=.kcp/cluster-east.kubeconfig get pods -l app=hello-world

echo "=== Checking cluster-west ==="
kubectl --kubeconfig=.kcp/cluster-west.kubeconfig get pods -l app=hello-world
```

### 6.2 Observe Cross-Cluster Aggregation

TMC provides aggregated views of resources across clusters:

```bash
# Create a script to show aggregated status
cat <<'EOF' > check-aggregation.sh
#!/bin/bash

echo "=== TMC Cross-Cluster Aggregation Demo ==="
echo

echo "📊 Deployment Status Across Clusters:"
echo "------------------------------------"

# Check deployment in each cluster
for cluster in cluster-east cluster-west; do
    echo "Cluster: $cluster"
    kubectl --kubeconfig=.kcp/$cluster.kubeconfig get deployment hello-world \
        -o jsonpath='{.status.readyReplicas}/{.spec.replicas} replicas ready' 2>/dev/null || echo "Not found"
    echo
done

echo "🔄 Pod Distribution:"
echo "-------------------"
for cluster in cluster-east cluster-west; do
    echo "Cluster: $cluster"
    kubectl --kubeconfig=.kcp/$cluster.kubeconfig get pods -l app=hello-world \
        --no-headers 2>/dev/null | wc -l | awk '{print $1 " pods"}'
done

echo
echo "🌐 Service Endpoints:"
echo "--------------------"
for cluster in cluster-east cluster-west; do
    echo "Cluster: $cluster"
    kubectl --kubeconfig=.kcp/$cluster.kubeconfig get endpoints hello-world-service \
        -o jsonpath='{.subsets[0].addresses[*].ip}' 2>/dev/null | tr ' ' '\n' | wc -l | awk '{print $1 " endpoints"}'
done
EOF

chmod +x check-aggregation.sh
./check-aggregation.sh
```

### 6.3 Test TMC Recovery

Let's test TMC's recovery capabilities by simulating a cluster failure:

```bash
# Simulate cluster failure by scaling down workload in one cluster
echo "🔧 Simulating cluster failure (scaling down cluster-east)..."
kubectl --kubeconfig=.kcp/cluster-east.kubeconfig scale deployment hello-world --replicas=0

echo "⏳ Waiting 30 seconds for TMC to detect and respond..."
sleep 30

# Check how TMC responds
./check-aggregation.sh

echo
echo "🔄 Restoring cluster-east..."
kubectl --kubeconfig=.kcp/cluster-east.kubeconfig scale deployment hello-world --replicas=1

echo "⏳ Waiting for recovery..."
sleep 20
./check-aggregation.sh
```

### 6.4 Test Virtual Workspace Features

Create a virtual workspace view:

```bash
# Create a config map to test projection
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: hello-config
  labels:
    app: hello-world
    tmc.kcp.io/project: "true"
data:
  message: "Hello from TMC Virtual Workspace!"
  config.json: |
    {
      "environment": "tutorial",
      "features": ["cross-cluster", "aggregation", "projection"],
      "clusters": ["east", "west"]
    }
EOF

echo "📋 ConfigMap created in KCP workspace"
kubectl get configmap hello-config

echo
echo "🔍 Checking projection to target clusters:"
for cluster in cluster-east cluster-west; do
    echo "Cluster: $cluster"
    kubectl --kubeconfig=.kcp/$cluster.kubeconfig get configmap hello-config -o yaml 2>/dev/null | grep -A 5 data: || echo "Not projected yet"
    echo
done
```

## Step 7: Monitor TMC Health and Metrics

Check the TMC system health:

```bash
# Check TMC component health
cat <<'EOF' > check-tmc-health.sh
#!/bin/bash

echo "🏥 TMC System Health Check"
echo "=========================="
echo

echo "📡 KCP System Pods:"
kubectl --kubeconfig=.kcp/admin.kubeconfig get pods -n kcp-system

echo
echo "🎯 Sync Targets Status:"
kubectl --kubeconfig=.kcp/admin.kubeconfig get synctargets -o wide

echo
echo "📊 Resource Summary:"
echo "Deployments: $(kubectl --kubeconfig=.kcp/admin.kubeconfig get deployments --all-namespaces --no-headers 2>/dev/null | wc -l)"
echo "Services: $(kubectl --kubeconfig=.kcp/admin.kubeconfig get services --all-namespaces --no-headers 2>/dev/null | wc -l)"
echo "ConfigMaps: $(kubectl --kubeconfig=.kcp/admin.kubeconfig get configmaps --all-namespaces --no-headers 2>/dev/null | wc -l)"

echo
echo "🔄 Placement Status:"
kubectl get placements -o custom-columns="NAME:.metadata.name,CLUSTERS:.spec.numberOfClusters,STATUS:.status.phase"
EOF

chmod +x check-tmc-health.sh
./check-tmc-health.sh
```

## Step 8: Test Cross-Cluster Communication

Test connectivity between clusters:

```bash
# Create a test job to check cross-cluster networking
cat <<EOF | kubectl apply -f -
apiVersion: batch/v1
kind: Job
metadata:
  name: network-test
spec:
  template:
    spec:
      containers:
      - name: network-test
        image: busybox:1.35
        command:
        - sh
        - -c
        - |
          echo "Testing cross-cluster networking..."
          echo "Pod running on: \$(hostname)"
          echo "Available services:"
          nslookup hello-world-service
          echo "Done."
      restartPolicy: Never
  backoffLimit: 4
EOF

# Wait for job to complete and check logs
echo "⏳ Waiting for network test to complete..."
kubectl wait --for=condition=complete job/network-test --timeout=60s

echo "📋 Network test results:"
kubectl logs job/network-test
```

## Step 9: Explore TMC APIs

Use kubectl to explore TMC-specific resources:

```bash
# Check TMC custom resources
echo "🔍 TMC Custom Resources:"
echo "======================"

echo "SyncTargets:"
kubectl api-resources | grep synctarget

echo
echo "Placements:"  
kubectl api-resources | grep placement

echo
echo "Current placement details:"
kubectl get placement hello-world-placement -o json | jq '.status'
```

## Step 10: Scale and Load Test

Test TMC with scaling:

```bash
# Scale up the deployment
echo "📈 Scaling up hello-world deployment..."
kubectl scale deployment hello-world --replicas=6

echo "⏳ Waiting for scale up..."
sleep 30

# Check distribution
./check-aggregation.sh

echo
echo "📉 Scaling back down..."
kubectl scale deployment hello-world --replicas=2

sleep 20
./check-aggregation.sh
```

## Step 11: Cleanup

When you're done with the tutorial:

```bash
# Delete the hello-world resources
kubectl delete deployment hello-world
kubectl delete service hello-world-service
kubectl delete configmap hello-config
kubectl delete placement hello-world-placement
kubectl delete job network-test

# Exit the workspace
kubectl kcp workspace use root

# Delete the workspace
kubectl kcp workspace delete hello-world

# Clean up clusters (optional)
# kind delete cluster --name kcp-host
# kind delete cluster --name cluster-east  
# kind delete cluster --name cluster-west
```

## What We Demonstrated

In this tutorial, you've seen:

1. **Multi-Cluster Deployment**: How TMC distributes workloads across multiple clusters based on placement policies
2. **Cross-Cluster Aggregation**: How TMC provides unified views of resources across clusters
3. **Virtual Workspaces**: How TMC creates virtual workspace abstractions for distributed workloads
4. **Health Monitoring**: How TMC monitors cluster and workload health
5. **Recovery Capabilities**: How TMC responds to cluster failures and maintains workload availability
6. **Resource Projection**: How TMC can replicate resources across clusters with transformations

## Next Steps

To learn more about TMC:

1. Explore the [TMC Architecture Documentation](../developers/tmc/README.md)
2. Learn about [TMC Error Handling](../developers/tmc/error-handling.md)
3. Understand [TMC Health Monitoring](../developers/tmc/health-monitoring.md)
4. Dive into [TMC Metrics & Observability](../developers/tmc/metrics-observability.md)
5. Study [TMC Recovery Manager](../developers/tmc/recovery-manager.md)
6. Explore [Virtual Workspace Manager](../developers/tmc/virtual-workspace-manager.md)

## Troubleshooting

### Common Issues

**Clusters not connecting:**
- Check kind clusters are running: `kind get clusters`
- Verify network connectivity between clusters
- Check syncer logs: `kubectl logs -n kcp-system deployment/syncer-<cluster-name>`

**Placements not working:**
- Verify sync targets are ready: `kubectl get synctargets`
- Check placement status: `kubectl describe placement hello-world-placement`
- Ensure namespace selector matches your target namespace

**Workloads not appearing in target clusters:**
- Check syncer connectivity
- Verify RBAC permissions
- Check for resource conflicts or validation errors

**TMC features not working:**
- Ensure TMC is enabled in KCP configuration
- Check TMC component health
- Verify feature flags are set correctly

For additional help, check the KCP documentation and TMC troubleshooting guides.