# ClusterRegistration API Reference

The `ClusterRegistration` API manages the lifecycle of Kubernetes clusters within the TMC environment.

## API Version

- **Group**: `workload.kcp.io`
- **Version**: `v1alpha1`
- **Kind**: `ClusterRegistration`

## Resource Scope

- **Scope**: Cluster-scoped
- **Shortnames**: `clusters`, `cluster`

## Specification

### ClusterRegistrationSpec

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `location` | `string` | Yes | Geographic location identifier for the cluster |
| `kubeconfig` | `KubeconfigRef` | Yes | Reference to kubeconfig for cluster access |
| `capabilities` | `[]string` | No | List of cluster capabilities (e.g., networking, storage) |
| `resources` | `ResourceQuota` | No | Available cluster resources |
| `metadata` | `map[string]string` | No | Additional cluster metadata |

### KubeconfigRef

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `secretRef` | `SecretReference` | Yes | Reference to secret containing kubeconfig |

### SecretReference

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | `string` | Yes | Name of the secret |
| `key` | `string` | Yes | Key within the secret containing kubeconfig |

### ResourceQuota

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `cpu` | `resource.Quantity` | No | Total CPU capacity |
| `memory` | `resource.Quantity` | No | Total memory capacity |
| `storage` | `resource.Quantity` | No | Total storage capacity |
| `pods` | `int32` | No | Maximum pod count |

## Status

### ClusterRegistrationStatus

| Field | Type | Description |
|-------|------|-------------|
| `conditions` | `[]metav1.Condition` | Current cluster conditions |
| `phase` | `string` | Current registration phase |
| `syncTarget` | `ObjectReference` | Reference to associated SyncTarget |
| `lastHeartbeat` | `metav1.Time` | Last successful communication |
| `capabilities` | `[]string` | Discovered cluster capabilities |
| `resources` | `ResourceStatus` | Current resource utilization |

### Condition Types

- **Ready**: Cluster is ready for workload placement
- **Connected**: Network connectivity established
- **Synchronized**: Cluster state synchronized with KCP
- **Healthy**: Cluster is healthy and responsive

### Phase Values

- **Pending**: Registration in progress
- **Active**: Cluster available for workloads
- **Unavailable**: Cluster temporarily unavailable
- **Terminating**: Cluster being deregistered

## Examples

### Basic Cluster Registration

```yaml
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
    pods: 500
```

### Cluster with Custom Metadata

```yaml
apiVersion: workload.kcp.io/v1alpha1
kind: ClusterRegistration
metadata:
  name: edge-cluster-seattle
spec:
  location: "us-west-2a"
  kubeconfig:
    secretRef:
      name: edge-cluster-config
      key: config
  capabilities:
    - networking
    - edge-compute
  resources:
    cpu: "20"
    memory: "64Gi"
    storage: "500Gi"
  metadata:
    cluster-type: "edge"
    provider: "aws"
    instance-types: "c5.large,c5.xlarge"
    network-tier: "premium"
```

### High-Capacity Cluster

```yaml
apiVersion: workload.kcp.io/v1alpha1
kind: ClusterRegistration
metadata:
  name: compute-cluster-large
spec:
  location: "us-east-1"
  kubeconfig:
    secretRef:
      name: large-cluster-access
      key: kubeconfig
  capabilities:
    - networking
    - storage
    - compute
    - gpu
  resources:
    cpu: "1000"
    memory: "4Ti"
    storage: "10Ti"
    pods: 2000
  metadata:
    gpu-types: "nvidia-v100,nvidia-a100"
    storage-classes: "fast-ssd,standard"
    network-plugins: "calico,cilium"
```

## Operations

### Creating a ClusterRegistration

```bash
# Prepare kubeconfig secret
kubectl create secret generic my-cluster-config \
  --from-file=kubeconfig=/path/to/cluster-kubeconfig.yaml

# Create cluster registration
kubectl apply -f cluster-registration.yaml
```

### Checking Registration Status

```bash
# List all registered clusters
kubectl get clusterregistrations

# Get detailed status
kubectl describe clusterregistration production-us-west

# Check specific condition
kubectl get clusterregistration production-us-west \
  -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}'
```

### Updating Cluster Resources

```bash
# Update resource capacity
kubectl patch clusterregistration production-us-west --type='merge' -p='
{
  "spec": {
    "resources": {
      "cpu": "200",
      "memory": "800Gi"
    }
  }
}'
```

### Deregistering a Cluster

```bash
# Remove cluster registration
kubectl delete clusterregistration production-us-west

# Verify cleanup
kubectl get synctargets
```

## Best Practices

### Naming Conventions

- Use descriptive names including location and environment
- Examples: `production-us-west`, `staging-eu-central`, `edge-seattle`

### Resource Specification

- Specify realistic resource limits based on actual cluster capacity
- Leave buffer for system processes and overhead
- Update resource limits when cluster capacity changes

### Security Considerations

- Store kubeconfig in encrypted secrets
- Use dedicated service accounts with minimal required permissions
- Regularly rotate cluster access credentials

### Monitoring

- Monitor cluster heartbeat timestamps
- Set up alerts for condition changes
- Track resource utilization trends

## Troubleshooting

### Common Issues

**Cluster Not Ready**
```bash
# Check conditions for details
kubectl describe clusterregistration my-cluster

# Common causes:
# - Invalid kubeconfig
# - Network connectivity issues  
# - Insufficient permissions
```

**Resource Conflicts**
```bash
# Verify resource specifications
kubectl get clusterregistration my-cluster -o yaml

# Check actual cluster capacity
kubectl --kubeconfig=/path/to/cluster-config top nodes
```

### Related Resources

- [WorkloadPlacement API](workload-placement.md)
- [SyncTarget Documentation](../troubleshooting/common-issues.md)
- [Monitoring Guide](../operations/monitoring.md)