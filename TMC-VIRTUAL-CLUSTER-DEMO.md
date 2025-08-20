# KCP TMC Virtual Cluster Workload Demo

This demo showcases the **Transparent Multi-Cluster (TMC)** architecture using KCP virtual clusters, where workloads are deployed to KCP virtual clusters and automatically synchronized to physical clusters via syncers.

## Architecture Overview

```
┌─────────────────────────────────────────────────────────┐
│                    KCP Control Plane                    │
│  ┌─────────────────────────────────────────────────┐   │
│  │              TMC Virtual Cluster                │   │
│  │                (Workspace)                      │   │
│  │                                                 │   │
│  │  • Workloads deployed here                      │   │
│  │  • Placement policies                           │   │
│  │  • SyncTarget registrations                     │   │
│  └─────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────┘
                            │
                    ┌───────┴───────┐
                    │    Syncers     │
                    │  (Sync Agents) │
                    └───────┬───────┘
                            │
        ┌───────────────────┼───────────────────┐
        │                   │                   │
        ▼                   ▼                   ▼
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│   West      │    │    East     │    │   Future    │
│   Cluster   │    │   Cluster   │    │  Clusters   │
│             │    │             │    │             │
│ • Receives  │    │ • Receives  │    │ • Can be    │
│   workloads │    │   workloads │    │   added     │
│   from KCP  │    │   from KCP  │    │   easily    │
└─────────────┘    └─────────────┘    └─────────────┘
```

## Key Features Demonstrated

### 1. **Virtual Cluster as Primary Deployment Target**
- Workloads are deployed TO KCP virtual clusters, not directly to physical clusters
- KCP acts as the single source of truth for workload definitions
- Physical clusters receive workloads through synchronization

### 2. **Syncer-Based Workload Propagation**
- Syncers connect physical clusters to KCP virtual clusters
- Automatic propagation based on placement policies
- Bi-directional sync capability (push/pull modes)

### 3. **Location-Based Workload Placement**
- Workloads placed according to location preferences
- Automatic replica distribution across regions
- Dynamic workload movement via placement policy updates

### 4. **Multi-Cluster Management Foundation**
- Foundation for advanced TMC features
- Scalable to many physical clusters
- Unified control plane experience

## Demo Scripts

### Main Demo Script: `tmc-virtual-cluster-demo.sh`

The primary demonstration script that shows the complete TMC workflow:

```bash
# Run basic demo
./tmc-virtual-cluster-demo.sh

# Force recreate all clusters
./tmc-virtual-cluster-demo.sh --force-recreate

# Enable debug logging
./tmc-virtual-cluster-demo.sh --debug

# Skip cleanup to explore environment
./tmc-virtual-cluster-demo.sh --skip-cleanup

# Show help
./tmc-virtual-cluster-demo.sh --help
```

#### What the Main Demo Does:

1. **Physical Cluster Setup**: Creates Kind clusters (`kcp-west`, `kcp-east`)
2. **KCP Virtual Cluster Creation**: Creates TMC workspace acting as virtual cluster
3. **SyncTarget Registration**: Registers physical clusters as sync targets
4. **Syncer Process Startup**: Starts syncer agents to connect clusters
5. **Workload Deployment**: Deploys workloads TO the virtual cluster
6. **Sync Demonstration**: Shows workloads propagating to physical clusters
7. **Placement Changes**: Demonstrates workload movement between clusters

### Helper Script: `tmc-syncer-helper.sh`

Utility script for managing and inspecting syncer processes:

```bash
# Show syncer status
./tmc-syncer-helper.sh status

# View syncer logs
./tmc-syncer-helper.sh logs --cluster kcp-west

# Manually sync a workload
./tmc-syncer-helper.sh sync-workload --cluster kcp-east

# Verify connectivity
./tmc-syncer-helper.sh verify

# Stop all syncers
./tmc-syncer-helper.sh stop

# Show help
./tmc-syncer-helper.sh --help
```

## Prerequisites

1. **KCP Binary**: Ensure KCP is built (`make build`)
2. **Kind**: For creating physical test clusters
3. **kubectl**: For cluster management
4. **Docker**: Required for Kind clusters

## Demo Flow

### Step 1: Environment Setup
- Creates 2 Kind clusters representing physical clusters
- Starts KCP control plane
- Creates TMC virtual cluster workspace

### Step 2: Virtual Cluster Configuration
- Installs core Kubernetes CRDs in virtual cluster
- Installs TMC workload management CRDs
- Registers physical clusters as sync targets

### Step 3: Syncer Deployment
- Starts syncer processes for each physical cluster
- Establishes sync connections between virtual and physical clusters
- Creates placement policies for workload distribution

### Step 4: Workload Deployment
- Deploys workloads TO the KCP virtual cluster
- Workloads include proper TMC labeling and annotations
- Demonstrates namespace and deployment creation

### Step 5: Sync Verification
- Shows workloads appearing on physical clusters
- Verifies syncer propagation with proper labeling
- Demonstrates placement policy enforcement

### Step 6: Dynamic Placement
- Changes placement preferences
- Shows workload movement between clusters
- Demonstrates replica redistribution

## Key Concepts

### Virtual Cluster
- KCP workspace acting as a virtual Kubernetes cluster
- Single deployment target for all workloads
- Abstracts away physical cluster complexity

### SyncTarget
- Represents a physical cluster in the virtual cluster
- Defines cluster capabilities and location
- Used by placement policies for workload assignment

### Syncers
- Agents that synchronize workloads from virtual to physical clusters
- Handle resource translation and status reporting
- Support different sync modes (push, pull, bidirectional)

### Placement Policies
- Define where workloads should be deployed
- Support location preferences and requirements
- Enable dynamic workload movement

## Workload Labeling

Workloads are automatically labeled to track their TMC lifecycle:

```yaml
labels:
  workload.kcp.io/managed: "true"
  workload.kcp.io/synced-from: "tmc-workloads"
annotations:
  workload.kcp.io/source-cluster: "kcp-virtual"
  workload.kcp.io/sync-target: "west-target"
  workload.kcp.io/sync-timestamp: "2025-08-20T10:30:00Z"
```

## Exploring the Demo Environment

After running the demo, you can explore the environment:

### Virtual Cluster (KCP)
```bash
# Set KCP kubeconfig
export KUBECONFIG=/tmp/kcp-demo-*/admin.kubeconfig

# List workloads in virtual cluster
kubectl get deployments,services,pods -n demo-app

# Check sync targets
kubectl get synctargets -o wide

# View placement policies
kubectl get clusterworkloadplacements -o yaml
```

### Physical Clusters
```bash
# West cluster workloads
kubectl --context kind-kcp-west get all -n demo-app --show-labels

# East cluster workloads  
kubectl --context kind-kcp-east get all -n demo-app --show-labels
```

### Syncer Monitoring
```bash
# Check syncer status
./tmc-syncer-helper.sh status

# Follow syncer logs
./tmc-syncer-helper.sh logs --cluster kcp-west
```

## Comparison with Basic Multi-Cluster

| Aspect | Basic Multi-Cluster | TMC Virtual Cluster |
|--------|-------------------|-------------------|
| **Deployment Target** | Direct to physical clusters | KCP virtual cluster |
| **Workload Source** | Multiple cluster APIs | Single virtual cluster API |
| **Synchronization** | Manual or external tools | Built-in syncers |
| **Placement Control** | External orchestration | KCP placement policies |
| **Abstraction Level** | Cluster-aware | Cluster-transparent |
| **Management Complexity** | High (N clusters) | Low (1 virtual cluster) |

## Troubleshooting

### KCP Not Starting
- Check logs: `tail -f /tmp/kcp-demo-*/kcp.log`
- Verify ports are not in use
- Ensure sufficient disk space

### Syncers Not Working  
- Check syncer processes: `./tmc-syncer-helper.sh status`
- Verify physical cluster connectivity
- Review syncer logs for errors

### Workloads Not Syncing
- Verify SyncTarget registration: `kubectl get synctargets`
- Check placement policy matching
- Ensure proper workload labeling

## Future Extensions

This demo provides the foundation for:

- **Real Syncer Implementation**: Replace mock syncers with actual syncer binary
- **Advanced Placement Policies**: Resource-based, affinity-based placement  
- **Multi-Tenant Workspaces**: Separate virtual clusters per tenant
- **Cross-Cluster Service Discovery**: Services spanning multiple clusters
- **Federated Storage**: Persistent volumes across clusters
- **Monitoring Integration**: Unified monitoring across virtual and physical clusters

## Production Considerations

For production TMC deployments:

1. **High Availability KCP**: Multi-replica KCP deployment
2. **Secure Communication**: TLS certificates for all components  
3. **RBAC Integration**: Fine-grained access control
4. **Monitoring**: Comprehensive observability stack
5. **Backup/Recovery**: KCP data persistence strategies
6. **Network Policies**: Secure cluster-to-cluster communication
7. **Resource Quotas**: Prevent resource exhaustion
8. **Audit Logging**: Complete audit trail for compliance

This demo showcases the core TMC concepts and provides a foundation for building production-ready transparent multi-cluster systems with KCP.