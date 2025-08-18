# Placement Policies and Resource Management

## Policy Types

### Geographic Policies

```yaml
# Distribute across regions
constraints:
  topology:
    regions: ["us-west-2", "us-east-1"]
    excludeRegions: ["eu-west-1"]
```

### Resource-Based Policies

```yaml
# High-memory workloads
constraints:
  resources:
    cpu: "4"
    memory: "16Gi"
  nodeSelector:
    node-type: memory-optimized
```

### Capability Policies

```yaml
# GPU workloads
constraints:
  capabilities: ["gpu", "high-performance"]
  nodeSelector:
    accelerator: nvidia-v100
```

## Scheduling Strategies

### Even Distribution

```yaml
scheduling:
  spreadConstraints:
  - maxSkew: 1
    topologyKey: "topology.kubernetes.io/zone"
```

### Overflow Handling

```yaml
scheduling:
  overflowPolicy: spillover
  maxReplicasPerCluster: 50
```

## Resource Management

### Quota Management

- **Cluster-level quotas**: Enforced per registered cluster
- **Namespace quotas**: Applied within TMC workspaces
- **Resource monitoring**: Continuous tracking of utilization

### Best Practices

1. **Plan capacity**: Monitor cluster resources regularly
2. **Set realistic limits**: Account for system overhead
3. **Use priorities**: Critical workloads get placement preference
4. **Monitor utilization**: Track resource usage trends