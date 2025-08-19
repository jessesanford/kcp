# TMC (Transparent Multi-Cluster) Implementation 4 Documentation

This document provides comprehensive documentation for the TMC Implementation 4 (impl4) architecture, APIs, controllers, and integration with KCP's virtual workspace framework.

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [API Types and Resources](#api-types-and-resources)
3. [Controller Components](#controller-components)
4. [Virtual Workspace Integration](#virtual-workspace-integration)
5. [Setup and Configuration](#setup-and-configuration)
6. [Testing Strategy](#testing-strategy)
7. [PR Structure and Dependencies](#pr-structure-and-dependencies)
8. [Feature Flags](#feature-flags)

## Architecture Overview

TMC Implementation 4 is designed as a comprehensive multi-cluster management solution that integrates deeply with KCP's workspace and logical cluster architecture. The system provides transparent workload placement, cluster management, and resource scaling across multiple physical Kubernetes clusters.

### Core Architectural Principles

```
┌─────────────────────────────────────────────────────────────────┐
│                       KCP Root Cluster                         │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐ │
│  │   Workspace A   │  │   Workspace B   │  │   Workspace C   │ │
│  │                 │  │                 │  │                 │ │
│  │ ┌─────────────┐ │  │ ┌─────────────┐ │  │ ┌─────────────┐ │ │
│  │ │ TMC APIs    │ │  │ │ TMC APIs    │ │  │ │ TMC APIs    │ │ │
│  │ │             │ │  │ │             │ │  │ │             │ │ │
│  │ │ ClusterReg  │ │  │ │ ClusterReg  │ │  │ │ ClusterReg  │ │ │
│  │ │ Placement   │ │  │ │ Placement   │ │  │ │ Placement   │ │ │
│  │ │ HPAPolicy   │ │  │ │ HPAPolicy   │ │  │ │ HPAPolicy   │ │ │
│  │ └─────────────┘ │  │ └─────────────┘ │  │ └─────────────┘ │ │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                    TMC Virtual Workspace                       │
│                  (/services/tmc/clusters/*)                    │
│                                                                 │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐ │
│  │  Controller 1   │  │  Controller 2   │  │  Controller 3   │ │
│  │                 │  │                 │  │                 │ │
│  │ Cluster Reg     │  │ Placement       │  │ HPA + Shard     │ │
│  │ Controller      │  │ Controller      │  │ Controllers     │ │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                     Physical Clusters                          │
│                                                                 │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐        │
│  │  Cluster A  │    │  Cluster B  │    │  Cluster C  │        │
│  │             │    │             │    │             │        │
│  │ us-west-2   │    │ us-east-1   │    │ eu-west-1   │        │
│  │             │    │             │    │             │        │
│  │ Workloads   │    │ Workloads   │    │ Workloads   │        │
│  │ + Syncers   │    │ + Syncers   │    │ + Syncers   │        │
│  └─────────────┘    └─────────────┘    └─────────────┘        │
└─────────────────────────────────────────────────────────────────┘
```

### Key Components

1. **Workspace-Aware APIs**: TMC APIs are available in every KCP workspace
2. **Virtual Workspace**: Unified access point for TMC operations
3. **Controllers**: Distributed controllers managing different aspects
4. **Physical Integration**: Direct connection to physical clusters
5. **Placement Engine**: Intelligent workload placement decisions

## API Types and Resources

TMC provides several custom resource types that integrate with KCP's API pattern.

### ClusterRegistration

The `ClusterRegistration` resource represents a physical Kubernetes cluster registered with TMC.

```yaml
apiVersion: tmc.kcp.io/v1alpha1
kind: ClusterRegistration
metadata:
  name: production-us-west-2
spec:
  location: us-west-2
  clusterEndpoint:
    serverURL: https://k8s-cluster.us-west-2.example.com
    caBundle: LS0t...  # Base64 encoded CA bundle
    tlsConfig:
      insecureSkipVerify: false
  capacity:
    cpu: 1000000      # 1000 CPU cores in milliCPU
    memory: 4000000000000  # 4TB in bytes
    maxPods: 10000
status:
  conditions:
  - type: Ready
    status: "True"
    reason: ClusterHealthy
    message: "Cluster is healthy and ready for workloads"
  lastHeartbeat: "2024-08-11T22:00:00Z"
  allocatedResources:
    cpu: 450000       # 450 cores allocated
    memory: 1800000000000  # 1.8TB allocated
    pods: 2400
  capabilities:
    kubernetesVersion: "v1.28.2"
    supportedAPIVersions:
    - v1
    - apps/v1
    - batch/v1
    availableResources:
    - pods
    - services
    - deployments
    nodeCount: 50
    features:
    - "LoadBalancer"
    - "PersistentVolumes"
    lastDetected: "2024-08-11T21:55:00Z"
```

#### API Fields

**Spec Fields:**
- `location`: Geographical/logical location identifier
- `clusterEndpoint`: Connection information for the physical cluster
  - `serverURL`: Kubernetes API server URL
  - `caBundle`: Certificate authority bundle for secure connections
  - `tlsConfig`: Additional TLS configuration options
- `capacity`: Resource capacity information
  - `cpu`: Total CPU capacity in milliCPU
  - `memory`: Total memory capacity in bytes
  - `maxPods`: Maximum number of pods

**Status Fields:**
- `conditions`: Standard Kubernetes conditions
- `lastHeartbeat`: Last successful health check timestamp
- `allocatedResources`: Currently allocated resources
- `capabilities`: Detected cluster capabilities and features

### WorkloadPlacement

The `WorkloadPlacement` resource defines policies for placing workloads across clusters.

```yaml
apiVersion: tmc.kcp.io/v1alpha1
kind: WorkloadPlacement
metadata:
  name: web-app-placement
  namespace: production
spec:
  workloadSelector:
    matchLabels:
      app: web-app
      tier: frontend
  clusterSelector:
    matchLabels:
      region: us-west
    requirements:
    - key: cluster-type
      operator: In
      values: ["production"]
  placementPolicy: LoadBalanced
status:
  conditions:
  - type: Ready
    status: "True"
    reason: PlacementActive
    message: "Placement policy is active"
  selectedClusters:
  - production-us-west-1
  - production-us-west-2
  placedWorkloads:
  - workloadRef:
      apiVersion: apps/v1
      kind: Deployment
      name: web-app
      namespace: production
    clusterName: production-us-west-1
    placementTime: "2024-08-11T22:00:00Z"
    status: Running
  lastPlacementTime: "2024-08-11T22:00:00Z"
```

#### Placement Policies

- `RoundRobin`: Distribute workloads evenly across selected clusters
- `LoadBalanced`: Consider cluster load when placing workloads
- `LocationAware`: Prefer clusters in specific locations
- `ResourceOptimized`: Optimize for resource utilization

### HPA Policy Types

TMC extends Kubernetes HPA with cluster-aware policies:

```yaml
apiVersion: tmc.kcp.io/v1alpha1
kind: HPAPolicy
metadata:
  name: web-app-scaling
spec:
  targetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: web-app
  minReplicas: 3
  maxReplicas: 100
  clusterScaling:
    enabled: true
    maxClusters: 5
    scaleOutThreshold: 80  # CPU percentage
    scaleInThreshold: 20   # CPU percentage
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
```

## Controller Components

TMC Implementation 4 consists of several specialized controllers that work together to provide comprehensive multi-cluster management.

### Cluster Registration Controller

**Location**: `pkg/tmc/controller/clusterregistration.go`

The Cluster Registration Controller manages the lifecycle of physical cluster connections.

#### Responsibilities

1. **Health Monitoring**: Continuously monitors physical cluster health
2. **Capability Detection**: Discovers cluster capabilities and resources
3. **Connection Management**: Maintains secure connections to physical clusters
4. **Status Reporting**: Updates cluster status and conditions

#### Key Features

```go
type ClusterRegistrationController struct {
    // Core components
    queue workqueue.RateLimitingInterface
    
    // KCP client for TMC API access
    kcpClusterClient kcpclientset.ClusterInterface
    
    // Physical cluster clients
    clusterClients map[string]kubernetes.Interface
    
    // Configuration
    workspace    logicalcluster.Name
    resyncPeriod time.Duration
    workerCount  int
    
    // Health tracking
    clusterHealth map[string]*ClusterHealthStatus
}
```

#### Health Checking

- **Periodic Health Checks**: Configurable interval (default: 30 seconds)
- **Node Status Monitoring**: Tracks cluster node availability
- **API Server Connectivity**: Verifies API server accessibility
- **Resource Discovery**: Updates available resources and capabilities

#### Error Handling

- **Retry Logic**: Exponential backoff for failed connections
- **Circuit Breaker**: Prevents overwhelming unhealthy clusters
- **Graceful Degradation**: Maintains service during partial failures

### Placement Controller

**Location**: `pkg/tmc/controller/placement.go`

The Placement Controller implements intelligent workload placement across clusters.

#### Responsibilities

1. **Workload Selection**: Identifies workloads matching placement policies
2. **Cluster Selection**: Chooses optimal clusters based on policies
3. **Placement Execution**: Coordinates workload deployment
4. **Status Tracking**: Monitors placed workload status

#### Placement Algorithms

##### Round Robin
```go
// Distributes workloads evenly across available clusters
func (pc *PlacementController) roundRobinPlacement(
    workloads []WorkloadReference,
    clusters []string,
) PlacementDecision {
    // Implementation ensures even distribution
}
```

##### Load Balanced
```go
// Considers cluster load when making placement decisions
func (pc *PlacementController) loadBalancedPlacement(
    workloads []WorkloadReference,
    clusters []ClusterInfo,
) PlacementDecision {
    // Analyzes CPU, memory, and pod utilization
}
```

#### Integration Points

- **Syncer Integration**: Coordinates with KCP syncers
- **Resource Monitoring**: Integrates with cluster resource tracking
- **Policy Engine**: Evaluates complex placement constraints

### HPA Controller

**Location**: `pkg/tmc/controller/hpa.go`

The HPA Controller extends Kubernetes Horizontal Pod Autoscaling with cluster-aware features.

#### Responsibilities

1. **Cross-Cluster Scaling**: Scale workloads across multiple clusters
2. **Resource Monitoring**: Aggregate metrics from multiple clusters
3. **Policy Enforcement**: Apply cluster-specific scaling policies
4. **Decision Making**: Determine when and where to scale

#### Scaling Strategies

##### Vertical Scaling (Single Cluster)
```go
// Scale within a single cluster first
func (hc *HPAController) scaleVertical(
    target HPATarget,
    currentReplicas int32,
    desiredReplicas int32,
) error {
    // Implementation for single-cluster scaling
}
```

##### Horizontal Scaling (Multi-Cluster)
```go
// Scale across multiple clusters when needed
func (hc *HPAController) scaleHorizontal(
    target HPATarget,
    availableClusters []string,
) error {
    // Implementation for multi-cluster scaling
}
```

#### Metrics Integration

- **Custom Metrics**: Support for application-specific metrics
- **External Metrics**: Integration with external monitoring systems
- **Cluster Metrics**: Aggregate resource usage across clusters

### Shard Controller

**Location**: `pkg/tmc/controller/shard.go`

The Shard Controller manages KCP shard assignment and workload distribution.

#### Responsibilities

1. **Shard Management**: Coordinate with KCP's sharding system
2. **Load Distribution**: Distribute TMC workload across shards
3. **Failover Handling**: Manage shard failures and recovery
4. **Resource Balancing**: Balance resources across shards

#### Sharding Strategy

```go
type ShardController struct {
    // Shard assignment logic
    shardAssigner ShardAssigner
    
    // Health monitoring
    shardHealth map[string]*ShardStatus
    
    // Load balancing
    loadBalancer LoadBalancer
}
```

## Virtual Workspace Integration

TMC integrates with KCP's virtual workspace framework to provide unified API access across logical clusters.

### Virtual Workspace Architecture

**Location**: `pkg/virtual/tmc/builder/build.go`

The TMC virtual workspace provides:

1. **Unified API Access**: Single endpoint for all TMC operations
2. **Workspace Isolation**: Maintain workspace boundaries
3. **Cross-Cluster Operations**: Operations spanning multiple clusters
4. **Authorization Integration**: Leverage KCP's authorization system

### URL Structure

TMC virtual workspace URLs follow this pattern:

```
/services/tmc/clusters/<cluster>/apis/tmc.kcp.io/v1alpha1/<resource>
```

Where:
- `<cluster>`: Logical cluster name or wildcard `*`
- `<resource>`: TMC resource type (clusterregistrations, workloadplacements, etc.)

### Implementation Details

```go
func BuildVirtualWorkspace(
    rootPathPrefix string,
    cfg *rest.Config,
    kubeClusterClient kcpkubernetesclientset.ClusterInterface,
    kcpClusterClient kcpclientset.ClusterInterface,
    cachedKcpInformers, kcpInformers kcpinformers.SharedInformerFactory,
) ([]rootapiserver.NamedVirtualWorkspace, error) {
    // Creates TMC virtual workspace with proper routing
}
```

#### URL Parsing

```go
func digestURL(urlPath, rootPathPrefix string) (
    cluster genericapirequest.Cluster,
    prefixToStrip string,
    accepted bool,
) {
    // Parses TMC URLs and extracts logical cluster information
    // Handles both specific clusters and wildcard operations
}
```

### Authorization

TMC virtual workspace integrates with KCP's authorization system:

- **Workspace Boundaries**: Respects logical cluster isolation
- **RBAC Integration**: Uses standard Kubernetes RBAC
- **Permission Claims**: Integrates with KCP permission claims
- **Maximal Permission Policy**: Follows KCP authorization patterns

## Setup and Configuration

### Prerequisites

1. **KCP Installation**: Running KCP cluster with workspace support
2. **Physical Clusters**: Kubernetes clusters to be managed
3. **Network Connectivity**: Secure network access between KCP and physical clusters
4. **Certificates**: Proper TLS certificates for secure communication

### Installation Steps

#### 1. Enable TMC Feature Flags

TMC uses feature flags to control functionality:

```yaml
# kcp-config.yaml
features:
  tmc:
    enabled: true
    user: "@jessesanford"
    version: "0.1"
    controllers:
      cluster-registration: true
      placement: true
      hpa: true
      shard: true
```

#### 2. Deploy TMC Controllers

```bash
# Deploy TMC controller binary
kubectl apply -f config/tmc-controller-deployment.yaml

# Apply RBAC permissions
kubectl apply -f config/rbac/
```

#### 3. Configure Virtual Workspace

```yaml
# virtual-workspace-config.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: tmc-virtual-workspace-config
data:
  config.yaml: |
    tmc:
      rootPathPrefix: "/services/tmc"
      authorization:
        enabled: true
        mode: "rbac"
```

#### 4. Register Physical Clusters

```yaml
apiVersion: tmc.kcp.io/v1alpha1
kind: ClusterRegistration
metadata:
  name: production-cluster-1
spec:
  location: us-west-2
  clusterEndpoint:
    serverURL: https://k8s-api.example.com
    caBundle: LS0t...
```

### Configuration Options

#### Controller Configuration

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: tmc-controller-config
data:
  config.yaml: |
    controllers:
      clusterRegistration:
        enabled: true
        resyncPeriod: 30s
        healthCheckInterval: 10s
        workerCount: 5
      placement:
        enabled: true
        defaultPolicy: "LoadBalanced"
        evaluationInterval: 15s
      hpa:
        enabled: true
        metricsInterval: 30s
        scaleUpCooldown: 3m
        scaleDownCooldown: 5m
      shard:
        enabled: true
        balancingInterval: 1m
```

#### Network Configuration

```yaml
networking:
  clusterCIDR: "10.0.0.0/8"
  serviceCIDR: "172.20.0.0/16"
  tlsConfig:
    minVersion: "1.2"
    cipherSuites:
    - "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"
    - "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384"
```

## Testing Strategy

TMC Implementation 4 includes comprehensive testing at multiple levels.

### Unit Testing

#### Controller Tests

```go
func TestClusterRegistrationController(t *testing.T) {
    tests := map[string]struct {
        cluster   *tmcv1alpha1.ClusterRegistration
        workspace string
        wantError bool
        wantConditions []metav1.Condition
    }{
        "healthy cluster registration": {
            cluster: &tmcv1alpha1.ClusterRegistration{
                ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
                Spec: tmcv1alpha1.ClusterRegistrationSpec{
                    Location: "us-west-2",
                    ClusterEndpoint: tmcv1alpha1.ClusterEndpoint{
                        ServerURL: "https://test.example.com",
                    },
                },
            },
            workspace: "root:test",
            wantError: false,
            wantConditions: []metav1.Condition{
                {Type: "Ready", Status: "True"},
            },
        },
        "invalid cluster endpoint": {
            cluster: &tmcv1alpha1.ClusterRegistration{
                ObjectMeta: metav1.ObjectMeta{Name: "invalid-cluster"},
                Spec: tmcv1alpha1.ClusterRegistrationSpec{
                    Location: "us-west-2",
                    ClusterEndpoint: tmcv1alpha1.ClusterEndpoint{
                        ServerURL: "invalid-url",
                    },
                },
            },
            workspace: "root:test",
            wantError: true,
            wantConditions: []metav1.Condition{
                {Type: "Ready", Status: "False", Reason: "InvalidEndpoint"},
            },
        },
    }
    
    for name, tc := range tests {
        t.Run(name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

#### API Validation Tests

```go
func TestClusterRegistrationValidation(t *testing.T) {
    // Test API validation logic
    // Covers field validation, webhook validation, etc.
}

func TestWorkloadPlacementValidation(t *testing.T) {
    // Test placement policy validation
    // Covers selector validation, policy constraints, etc.
}
```

### Integration Testing

#### Virtual Workspace Tests

```go
func TestTMCVirtualWorkspace(t *testing.T) {
    // Test virtual workspace URL routing
    // Test authorization integration
    // Test API discovery
}
```

#### Controller Integration Tests

```go
func TestControllerIntegration(t *testing.T) {
    // Test controller interactions
    // Test event processing
    // Test error handling
}
```

### End-to-End Testing

#### Cluster Registration E2E

```bash
# Test complete cluster registration flow
./test/e2e/cluster-registration/
├── register_cluster_test.go      # Basic registration
├── health_monitoring_test.go     # Health check functionality  
├── capability_detection_test.go  # Capability discovery
└── failure_scenarios_test.go     # Error handling
```

#### Workload Placement E2E

```bash
# Test workload placement across clusters
./test/e2e/placement/
├── placement_policies_test.go    # Policy evaluation
├── multi_cluster_test.go        # Cross-cluster operations
├── scaling_test.go              # HPA integration
└── failover_test.go             # Cluster failure scenarios
```

### Performance Testing

#### Load Testing

```go
func BenchmarkClusterHealthCheck(b *testing.B) {
    // Benchmark health check performance
    // Test with varying cluster counts
}

func BenchmarkPlacementDecisions(b *testing.B) {
    // Benchmark placement algorithm performance
    // Test with varying workload counts
}
```

#### Scale Testing

- **Cluster Scale**: Test with 100+ registered clusters
- **Workload Scale**: Test with 10,000+ workloads
- **Request Scale**: Test API throughput under load

## PR Structure and Dependencies

TMC Implementation 4 is delivered through a series of atomic, well-tested PRs.

### Implementation Phases

#### Phase 1: Foundation (PRs 00-08)
- **impl4-00-feature-flags**: Feature flag infrastructure
- **impl4-01-base-controller**: Controller foundation
- **impl4-02-workqueue**: Work queue implementation
- **impl4-03-api-types**: Core API types
- **impl4-04-api-resources**: API resource definitions
- **impl4-05-rbac**: RBAC policies
- **impl4-06-auth**: Authentication integration
- **impl4-07-controller-binary**: Controller binary
- **impl4-08-controller-config**: Configuration system

#### Phase 2: Core Controllers (PRs 09-17)
- **impl4-09-cluster-controller**: Cluster registration controller
- **impl4-10-cluster-logic**: Cluster management logic
- **impl4-11-placement-controller**: Workload placement controller
- **impl4-12-server-integration**: Server integration
- **impl4-13-metrics-storage**: Metrics storage
- **impl4-14-metrics-api**: Metrics API
- **impl4-15-dashboards**: Monitoring dashboards
- **impl4-16-collectors**: Metrics collectors
- **impl4-17-physical-interfaces**: Physical cluster interfaces

#### Phase 3: Advanced Features (PRs 18-31)
- **impl4-18-syncer-core**: Syncer integration
- **impl4-19-factory-core**: Factory patterns
- **impl4-20-scaling-basic**: Basic scaling
- **impl4-21-scaling-config**: Scaling configuration
- **impl4-22-hpa-policy**: HPA policies
- **impl4-23-hpa-controller**: HPA controller
- **impl4-24-placement-advanced**: Advanced placement
- **impl4-25-placement-analysis**: Placement analysis
- **impl4-26-constraint-engine**: Constraint evaluation
- **impl4-27-placement-session**: Session management
- **impl4-28-session-affinity**: Session affinity
- **impl4-29-placement-decision**: Decision engine
- **impl4-30-traffic-monitoring**: Traffic monitoring
- **impl4-31-status-aggregation**: Status aggregation

#### Phase 4: Integration (PRs 33-48)
- **impl4-33-placement-health**: Placement health monitoring
- **impl4-39-virtual-workspace**: Virtual workspace integration
- **impl4-40-shard-controller**: Shard controller
- **impl4-41-admission-webhooks**: Admission webhooks
- **impl4-43-apiresourceschema**: API resource schemas
- **impl4-45-apibinding-controller**: API binding controller
- **impl4-48-cost-optimizer**: Cost optimization

### Dependency Graph

```
Foundation (00-08)
    │
    ▼
Core Controllers (09-17)
    │
    ▼
Advanced Features (18-31)
    │
    ▼  
Integration (33-48)
```

### PR Requirements

Each PR must meet these criteria:

1. **Size**: 400-700 lines of new code (excluding generated code)
2. **Testing**: Comprehensive unit and integration tests
3. **Documentation**: Code comments and user documentation
4. **Quality**: Pass all linting and quality checks
5. **Atomicity**: Complete, standalone functionality

### Branching Strategy

- **Base Branch**: All PRs branch from `main`
- **Branch Naming**: `feature/tmc-impl4/XX-description`
- **Independence**: PRs can be reviewed and merged independently
- **Rebase**: Rebase on main if conflicts arise

## Feature Flags

TMC uses a hierarchical feature flag system to control functionality rollout.

### Master Feature Flag

```yaml
features:
  tmc:
    enabled: true
    user: "@jessesanford"
    version: "0.1"
```

When the master TMC flag is disabled, all sub-features are automatically disabled.

### Sub-Feature Flags

```yaml
features:
  tmc:
    enabled: true
    controllers:
      cluster-registration: true
      placement: true
      hpa: true
      shard: true
    virtual-workspace:
      enabled: true
    metrics:
      enabled: true
      storage: "prometheus"
```

### Feature Flag Implementation

```go
// Feature flag checking
func (c *Controller) isFeatureEnabled(feature string) bool {
    if !c.featureGates.Enabled(features.TMC) {
        return false
    }
    
    return c.featureGates.Enabled(feature)
}

// Usage in controller
func (c *ClusterRegistrationController) shouldProcessCluster(cluster *tmcv1alpha1.ClusterRegistration) bool {
    return c.isFeatureEnabled(features.TMCClusterRegistration)
}
```

### Rollout Strategy

1. **Alpha (0.1)**: Basic functionality behind feature flags
2. **Beta (0.2)**: Extended functionality, broader testing
3. **GA (1.0)**: Full functionality, feature flags removed

---

*This documentation covers TMC Implementation 4 as of August 2024. For the latest updates and changes, please refer to the individual PR documentation and commit messages.*