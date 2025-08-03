# Transparent Multi-Cluster (TMC) System

The Transparent Multi-Cluster (TMC) system provides a comprehensive infrastructure for managing workloads across multiple Kubernetes clusters through KCP. The TMC system consists of several integrated components that work together to provide seamless multi-cluster operations.

## Architecture Overview

The TMC system is built around the following core components:

```
┌─────────────────────────────────────────────────────────────────┐
│                        KCP Logical Clusters                      │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐ │
│  │ Virtual         │  │ TMC Error       │  │ TMC Health      │ │
│  │ Workspace       │  │ Handling        │  │ System          │ │
│  │ Manager         │  │                 │  │                 │ │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘ │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐ │
│  │ TMC Metrics     │  │ TMC Recovery    │  │ Placement       │ │
│  │ & Observability │  │ Manager         │  │ Controller      │ │
│  │                 │  │                 │  │                 │ │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘ │
├─────────────────────────────────────────────────────────────────┤
│                      Workload Syncer                           │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐ │
│  │ Syncer Engine   │  │ Resource        │  │ Status Reporter │ │
│  │                 │  │ Controllers     │  │                 │ │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Physical Kubernetes Clusters                 │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐ │
│  │   Cluster A     │  │   Cluster B     │  │   Cluster C     │ │
│  │                 │  │                 │  │                 │ │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

## Components

### Core TMC Infrastructure

- **[TMC Error Handling](./error-handling.md)**: Categorized error types with recovery strategies
- **[TMC Health System](./health-monitoring.md)**: Component health monitoring and aggregation
- **[TMC Metrics & Observability](./metrics-observability.md)**: Comprehensive metrics collection and reporting
- **[TMC Recovery Manager](./recovery-manager.md)**: Automated failure recovery and healing

### Workload Management

- **[Workload Syncer](./syncer.md)**: Bidirectional resource synchronization between KCP and clusters
- **[Virtual Workspace Manager](./virtual-workspace-manager.md)**: Cross-cluster resource aggregation and projection
- **[Placement Controller](./placement-controller.md)**: Intelligent workload placement decisions

## Quick Start

### Prerequisites

- KCP running with TMC components enabled
- One or more target Kubernetes clusters
- Appropriate RBAC permissions configured

### Basic TMC Setup

1. **Install TMC Components**:
```bash
# The TMC components are automatically available in KCP
kubectl apply -f config/tmc/
```

2. **Register a Physical Cluster**:
```yaml
apiVersion: workload.kcp.io/v1alpha1
kind: SyncTarget
metadata:
  name: my-cluster
spec:
  workloadCluster:
    name: "my-physical-cluster"
    endpoint: "https://cluster.example.com"
```

3. **Deploy the Syncer**:
```bash
# Build the syncer
go build ./cmd/workload-syncer

# Run the syncer
./workload-syncer \
  --sync-target-name=my-cluster \
  --sync-target-uid=$(kubectl get synctarget my-cluster -o jsonpath='{.metadata.uid}') \
  --workspace-cluster=root:my-workspace \
  --kcp-kubeconfig=~/.kcp/admin.kubeconfig \
  --cluster-kubeconfig=~/.kube/config
```

### Deploying Workloads

Once the TMC system is set up, you can deploy workloads that will be automatically synchronized:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  namespace: default
spec:
  replicas: 3
  selector:
    matchLabels:
      app: my-app
  template:
    metadata:
      labels:
        app: my-app
    spec:
      containers:
      - name: app
        image: nginx:latest
        ports:
        - containerPort: 80
```

Apply this to your KCP logical cluster, and the syncer will automatically deploy it to the registered physical cluster.

## Key Features

### 🔄 **Bidirectional Synchronization**
- Resources created in KCP are automatically deployed to physical clusters
- Status updates from physical clusters are reflected back in KCP
- Supports all standard Kubernetes resources

### 🏥 **Comprehensive Health Monitoring**
- Real-time health status of all TMC components
- Automated failure detection and alerting
- Integration with Prometheus and other monitoring systems

### 📊 **Rich Metrics & Observability**
- Detailed metrics for sync operations, placement decisions, and system health
- Built-in Prometheus integration
- Distributed tracing support

### 🛡️ **Robust Error Handling**
- Categorized error types with specific recovery strategies
- Automatic retry logic with exponential backoff
- Circuit breaker patterns for failing operations

### 🎯 **Intelligent Placement**
- Automated workload placement based on cluster capabilities
- Support for placement constraints and affinity rules
- Load balancing across available clusters

## Examples

See the [examples directory](./examples/) for comprehensive examples including:

- [Basic TMC Setup](./examples/basic-setup/)
- [Multi-Cluster Deployment](./examples/multi-cluster-deployment/)
- [Health Monitoring Configuration](./examples/health-monitoring/)
- [Custom Metrics Collection](./examples/metrics/)
- [Disaster Recovery Scenarios](./examples/disaster-recovery/)

## API Reference

- [TMC APIs](./api-reference.md)
- [Syncer Configuration](./syncer-config.md)
- [Health Check APIs](./health-api.md)
- [Metrics APIs](./metrics-api.md)

## Architecture

- **[TMC Architecture Overview](./architecture.md)**: Complete system architecture with component relationships and design principles

## Tutorials

- **[TMC Hello World Tutorial](../../tutorials/tmc-hello-world.md)**: Step-by-step tutorial showing TMC features with kind clusters

## Development

- [Contributing to TMC](./CONTRIBUTING.md)
- [Development Setup](./development.md)
- [Testing Guide](./testing.md)

## Troubleshooting

- [Common Issues](./troubleshooting.md)
- [Debug Guide](./debugging.md)
- [Performance Tuning](./performance.md)