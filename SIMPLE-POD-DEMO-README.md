# TMC Simple Pod Demo

A minimal demonstration of KCP's Transparent Multi-Cluster (TMC) capabilities using just Pods - the simplest possible Kubernetes resource.

## What This Demo Shows

This demo demonstrates the **absolute minimum TMC functionality**:

1. **Single Pod Deployment**: Create a simple Pod in a KCP virtual cluster
2. **Automatic Placement**: Show it being placed on a physical cluster via syncers
3. **Dynamic Movement**: Move the Pod between physical clusters using placement policies

## Why Start with Pods?

- **Simplicity**: Pods are the most basic Kubernetes resource
- **No Dependencies**: No complex controllers or service dependencies
- **Clear Behavior**: Easy to understand placement and movement
- **Foundation**: Building block for more complex workloads

## Prerequisites

1. **KCP Binary**: Ensure KCP is built (should be present in `./bin/kcp`)
2. **Kind**: For creating physical test clusters
3. **kubectl**: For cluster management
4. **Docker**: Required for Kind clusters

## Running the Demo

### Basic Demo
```bash
./tmc-simple-pod-demo.sh
```

### With Options
```bash
# Force recreate Kind clusters
./tmc-simple-pod-demo.sh --force-recreate

# Enable debug logging
./tmc-simple-pod-demo.sh --debug

# Skip cleanup to explore environment
./tmc-simple-pod-demo.sh --skip-cleanup

# Show help
./tmc-simple-pod-demo.sh --help
```

## Demo Flow

### 1. Physical Cluster Setup
- Creates 2 Kind clusters (`kcp-west`, `kcp-east`)
- These represent physical clusters in different regions

### 2. KCP Control Plane
- Starts KCP with a fresh workspace
- Creates the control plane for virtual clusters

### 3. Virtual Cluster Creation
- Creates `simple-pods` workspace (virtual cluster)
- Installs **ONLY** the Pod and Namespace CRDs
- Installs TMC workload management CRDs

### 4. Sync Target Registration
- Registers both Kind clusters as sync targets
- Configures them to support Pod synchronization

### 5. Syncer Processes
- Starts mock syncer processes for each cluster
- In production, these would be real syncer binaries

### 6. Placement Policy
- Creates a simple placement policy
- Prefers `us-west-2` initially
- Supports both locations

### 7. Pod Deployment
- Deploys a single nginx Pod TO the KCP virtual cluster
- Pod includes TMC labels and annotations

### 8. Sync Demonstration
- Shows the Pod being synced to the preferred cluster (west)
- Demonstrates syncer propagation behavior

### 9. Pod Movement
- Updates placement policy to prefer east
- Shows the Pod being moved from west to east cluster
- Demonstrates dynamic placement capabilities

### 10. Verification
- Shows Pod status in virtual cluster (source of truth)
- Shows Pod status in physical clusters (sync results)
- Verifies complete TMC workflow

## Key Concepts Demonstrated

### Virtual Cluster as Primary Target
- Pod is deployed TO the KCP virtual cluster
- Physical clusters receive workloads through synchronization
- KCP acts as single source of truth

### Automatic Placement
- Placement policy determines which physical cluster gets the Pod
- Syncers automatically propagate based on policy
- No manual cluster selection required

### Dynamic Movement
- Placement policy changes trigger Pod movement
- Old Pod is removed from previous cluster
- New Pod is created on new preferred cluster

### TMC Labeling
The demo shows proper TMC labeling:
```yaml
labels:
  pod.kcp.io/managed: "true"
  pod.kcp.io/synced-from: "simple-pods"
annotations:
  pod.kcp.io/source-cluster: "kcp-virtual"
  pod.kcp.io/sync-target: "west-target"
  pod.kcp.io/sync-timestamp: "2025-08-20T10:30:00Z"
```

## Environment Exploration

After running the demo, explore the environment:

### Virtual Cluster (KCP)
```bash
# Set KCP kubeconfig
export KUBECONFIG=/tmp/kcp-simple-pod-demo-*/admin.kubeconfig

# List virtual Pod
kubectl get pods -n simple-demo -o wide

# Check sync targets
kubectl get synctargets -o wide

# View placement policies
kubectl get clusterworkloadplacements -o yaml
```

### Physical Clusters
```bash
# West cluster
kubectl --context kind-kcp-west get pods -n simple-demo --show-labels

# East cluster  
kubectl --context kind-kcp-east get pods -n simple-demo --show-labels
```

### Syncer Monitoring
```bash
# Check syncer logs
tail -f /tmp/kcp-simple-pod-demo-*/syncers/kcp-west-syncer.log
tail -f /tmp/kcp-simple-pod-demo-*/syncers/kcp-east-syncer.log

# KCP logs
tail -f /tmp/kcp-simple-pod-demo-*/kcp.log
```

## What Makes This Different

| Aspect | Traditional Multi-Cluster | TMC with KCP |
|--------|---------------------------|-------------|
| **Deployment Target** | Direct to physical clusters | KCP virtual cluster |
| **Pod Source** | Multiple cluster APIs | Single virtual cluster API |
| **Synchronization** | Manual or external tools | Built-in syncers |
| **Placement Control** | External orchestration | KCP placement policies |
| **Abstraction** | Cluster-aware | Cluster-transparent |
| **Management** | Complex (N clusters) | Simple (1 virtual cluster) |

## Next Steps

This minimal demo provides the foundation for:

1. **More Pod Variations**: Different placement requirements, resource constraints
2. **Additional Resources**: Services, ConfigMaps, Secrets
3. **Complex Workloads**: Deployments, StatefulSets, DaemonSets
4. **Advanced Placement**: Resource-based, affinity-based policies
5. **Real Syncers**: Replace mock syncers with actual syncer binary
6. **Multi-Tenant**: Multiple virtual clusters per physical cluster setup

## Troubleshooting

### KCP Not Starting
```bash
# Check logs
tail -f /tmp/kcp-simple-pod-demo-*/kcp.log

# Check ports
lsof -i :6443
```

### Kind Clusters Issues
```bash
# List clusters
kind get clusters

# Check cluster status
kubectl cluster-info --context kind-kcp-west
kubectl cluster-info --context kind-kcp-east
```

### Pod CRD Issues
```bash
# Check CRD status
kubectl get crd pods.core -o wide
kubectl describe crd pods.core
```

### Syncer Problems
```bash
# Check syncer processes
ps aux | grep syncer

# Check syncer logs
tail -f /tmp/kcp-simple-pod-demo-*/syncers/*.log
```

## Clean Up

The demo includes automatic cleanup, or you can clean up manually:

```bash
# Stop KCP
pkill -f "bin/kcp start"

# Delete Kind clusters
kind delete cluster --name kcp-west
kind delete cluster --name kcp-east

# Remove temp files
rm -rf /tmp/kcp-simple-pod-demo-*
```

This simple Pod demo shows that TMC concepts work at the most fundamental level - proving the architecture scales from simple Pods to complex applications.