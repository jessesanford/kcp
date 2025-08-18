# TMC Troubleshooting Guide

## Common Issues

### Cluster Registration Problems

**Cluster Not Ready**
```bash
kubectl describe clusterregistration my-cluster
# Check: kubeconfig validity, network connectivity, RBAC permissions
```

**Connection Timeouts**
```bash
# Test cluster connectivity
kubectl --kubeconfig=/path/to/cluster-config cluster-info
```

### Placement Issues

**Workloads Not Scheduled**
```bash
kubectl describe workloadplacement my-placement
# Check: resource constraints, cluster capacity, selector matches
```

**Uneven Distribution**
```bash
# Verify cluster weights and capacity
kubectl get clusterregistrations -o wide
```

### Performance Issues

**Slow Placement Decisions**
- Check cluster resource reporting frequency
- Verify network latency between KCP and clusters
- Monitor TMC controller resource usage

**High Resource Usage**
- Tune TMC controller replicas
- Adjust sync intervals
- Optimize placement policies

## Debugging Commands

### Status Checking
```bash
# Cluster health
kubectl get clusterregistrations -o wide

# Placement status
kubectl get workloadplacements -o wide

# TMC controllers
kubectl get pods -n kcp-system -l app=tmc
```

### Log Analysis
```bash
# TMC controller logs
kubectl logs -n kcp-system deployment/tmc-controller

# Syncer logs
kubectl logs -n kcp-system -l app=kcp-syncer
```

## Resource Issues

### Insufficient Capacity
```bash
# Check cluster resources
kubectl get clusterregistration my-cluster -o yaml | grep -A5 resources

# Solution: Add capacity or adjust placement constraints
```

### Policy Conflicts
```bash
# Review placement constraints
kubectl get workloadplacement my-placement -o yaml

# Solution: Relax constraints or update cluster capabilities
```

## Recovery Procedures

### Cluster Reconnection
1. Verify cluster health
2. Update kubeconfig if needed
3. Restart TMC controllers if necessary

### Placement Recovery
1. Check workload selector labels
2. Verify cluster registration status
3. Update placement policies as needed