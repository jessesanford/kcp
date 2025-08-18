# WorkloadPlacement API Reference

The `WorkloadPlacement` API defines policies for placing workloads across registered clusters in the TMC environment.

## API Version

- **Group**: `placement.kcp.io`
- **Version**: `v1alpha1`
- **Kind**: `WorkloadPlacement`

## Resource Scope

- **Scope**: Namespaced
- **Shortnames**: `wlp`, `placement`

## Specification

### WorkloadPlacementSpec

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `selector` | `metav1.LabelSelector` | Yes | Selector for workloads to place |
| `placementPolicy` | `PlacementPolicy` | Yes | Policy defining placement behavior |
| `rolloutStrategy` | `RolloutStrategy` | No | Strategy for rolling out changes |

### PlacementPolicy

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `clusters` | `[]ClusterTarget` | Yes | Target clusters with weights |
| `constraints` | `PlacementConstraints` | No | Placement constraints |
| `scheduling` | `SchedulingPolicy` | No | Advanced scheduling options |

### ClusterTarget

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | `string` | Yes | Name of the target cluster |
| `weight` | `int32` | Yes | Weight for workload distribution (1-100) |

### PlacementConstraints

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `resources` | `ResourceConstraints` | No | Resource requirements |
| `capabilities` | `[]string` | No | Required cluster capabilities |
| `topology` | `TopologyConstraints` | No | Geographic/topology requirements |
| `nodeSelector` | `map[string]string` | No | Node selection criteria |

### ResourceConstraints

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `cpu` | `resource.Quantity` | No | Minimum CPU requirement |
| `memory` | `resource.Quantity` | No | Minimum memory requirement |
| `storage` | `resource.Quantity` | No | Minimum storage requirement |

### TopologyConstraints

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `regions` | `[]string` | No | Allowed regions |
| `zones` | `[]string` | No | Allowed availability zones |
| `excludeRegions` | `[]string` | No | Excluded regions |

### SchedulingPolicy

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `spreadConstraints` | `[]SpreadConstraint` | No | Pod spread constraints |
| `overflowPolicy` | `string` | No | Behavior when cluster is full |
| `maxReplicasPerCluster` | `int32` | No | Maximum replicas per cluster |

### SpreadConstraint

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `maxSkew` | `int32` | Yes | Maximum skew between clusters |
| `topologyKey` | `string` | Yes | Topology key for spreading |

## Status

### WorkloadPlacementStatus

| Field | Type | Description |
|-------|------|-------------|
| `conditions` | `[]metav1.Condition` | Current placement conditions |
| `selectedClusters` | `[]ClusterSelection` | Clusters selected for placement |
| `placementDecisions` | `[]PlacementDecision` | Detailed placement decisions |
| `observedGeneration` | `int64` | Last observed spec generation |

### ClusterSelection

| Field | Type | Description |
|-------|------|-------------|
| `cluster` | `string` | Selected cluster name |
| `reason` | `string` | Reason for selection |
| `score` | `int32` | Placement score |

## Examples

### Basic Workload Placement

```yaml
apiVersion: placement.kcp.io/v1alpha1
kind: WorkloadPlacement
metadata:
  name: web-app-placement
  namespace: default
spec:
  selector:
    matchLabels:
      app: web-app
  placementPolicy:
    clusters:
    - name: production-us-west
      weight: 60
    - name: production-us-east
      weight: 40
    constraints:
      resources:
        cpu: "100m"
        memory: "128Mi"
      capabilities:
      - networking
```

### Geographic Distribution

```yaml
apiVersion: placement.kcp.io/v1alpha1
kind: WorkloadPlacement
metadata:
  name: global-service-placement
  namespace: services
spec:
  selector:
    matchLabels:
      tier: frontend
  placementPolicy:
    clusters:
    - name: us-west-cluster
      weight: 33
    - name: us-east-cluster
      weight: 33
    - name: eu-central-cluster
      weight: 34
    constraints:
      topology:
        regions:
        - us-west-2
        - us-east-1
        - eu-central-1
    scheduling:
      spreadConstraints:
      - maxSkew: 1
        topologyKey: "topology.kubernetes.io/region"
```

### High-Resource Workload

```yaml
apiVersion: placement.kcp.io/v1alpha1
kind: WorkloadPlacement
metadata:
  name: ml-workload-placement
  namespace: ml
spec:
  selector:
    matchLabels:
      workload-type: machine-learning
  placementPolicy:
    clusters:
    - name: gpu-cluster-west
      weight: 50
    - name: gpu-cluster-east
      weight: 50
    constraints:
      resources:
        cpu: "4"
        memory: "16Gi"
      capabilities:
      - gpu
      - high-memory
      nodeSelector:
        accelerator: nvidia-v100
    scheduling:
      maxReplicasPerCluster: 10
      overflowPolicy: spillover
```

### Anti-Affinity Placement

```yaml
apiVersion: placement.kcp.io/v1alpha1
kind: WorkloadPlacement
metadata:
  name: database-placement
  namespace: data
spec:
  selector:
    matchLabels:
      component: database
      tier: critical
  placementPolicy:
    clusters:
    - name: primary-cluster
      weight: 100
    constraints:
      resources:
        cpu: "2"
        memory: "8Gi"
        storage: "100Gi"
      capabilities:
      - storage
      - backup
    scheduling:
      spreadConstraints:
      - maxSkew: 0
        topologyKey: "topology.kubernetes.io/zone"
  rolloutStrategy:
    type: RollingUpdate
    maxSurge: 1
    maxUnavailable: 0
```

## Operations

### Creating WorkloadPlacements

```bash
# Apply placement policy
kubectl apply -f workload-placement.yaml

# Verify creation
kubectl get workloadplacements
```

### Checking Placement Status

```bash
# Get placement details
kubectl describe workloadplacement web-app-placement

# Check selected clusters
kubectl get workloadplacement web-app-placement \
  -o jsonpath='{.status.selectedClusters[*].cluster}'

# View placement decisions
kubectl get workloadplacement web-app-placement -o yaml
```

### Updating Placement Policies

```bash
# Update cluster weights
kubectl patch workloadplacement web-app-placement --type='merge' -p='
{
  "spec": {
    "placementPolicy": {
      "clusters": [
        {"name": "production-us-west", "weight": 80},
        {"name": "production-us-east", "weight": 20}
      ]
    }
  }
}'
```

### Testing Placement

```bash
# Deploy test workload
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-workload
  labels:
    app: web-app
spec:
  replicas: 6
  selector:
    matchLabels:
      app: web-app
  template:
    metadata:
      labels:
        app: web-app
    spec:
      containers:
      - name: web
        image: nginx:1.21
EOF

# Check placement results
kubectl describe workloadplacement web-app-placement
```

## Best Practices

### Selector Design

- Use specific labels to avoid unintended workload selection
- Include application and component identifiers
- Consider using label selectors with multiple criteria

### Weight Distribution

- Ensure weights sum to 100 for predictable distribution
- Use weights that reflect actual cluster capacity
- Consider geographic latency in weight assignment

### Resource Constraints

- Specify realistic resource requirements
- Include both requests and limits in calculations
- Account for cluster overhead and system processes

### Topology Planning

- Plan placement based on failure domains
- Consider network latency between regions
- Account for data locality requirements

## Advanced Patterns

### Canary Deployments

```yaml
# Canary placement with 10% traffic
apiVersion: placement.kcp.io/v1alpha1
kind: WorkloadPlacement
metadata:
  name: canary-placement
spec:
  selector:
    matchLabels:
      version: canary
  placementPolicy:
    clusters:
    - name: canary-cluster
      weight: 10
    - name: production-cluster
      weight: 90
```

### Disaster Recovery

```yaml
# DR placement with primary/backup
apiVersion: placement.kcp.io/v1alpha1
kind: WorkloadPlacement
metadata:
  name: dr-placement
spec:
  selector:
    matchLabels:
      criticality: high
  placementPolicy:
    clusters:
    - name: primary-datacenter
      weight: 100
    - name: backup-datacenter
      weight: 0  # Activated during failover
```

## Troubleshooting

### Common Issues

**No Clusters Selected**
```bash
# Check placement conditions
kubectl describe workloadplacement my-placement

# Common causes:
# - No clusters match constraints
# - All clusters at capacity
# - Resource requirements too high
```

**Uneven Distribution**
```bash
# Verify cluster weights
kubectl get workloadplacement my-placement -o yaml

# Check cluster capacity
kubectl get clusterregistrations -o wide
```

### Related Resources

- [ClusterRegistration API](cluster-registration.md)
- [Placement Policies](placement-policies.md)
- [Troubleshooting Guide](../troubleshooting/common-issues.md)