# Disaster Recovery

## Backup Strategy

### KCP Data Backup
```bash
# Backup etcd containing TMC state
etcdctl snapshot save tmc-backup.db

# Verify backup
etcdctl snapshot status tmc-backup.db
```

### Configuration Backup
```bash
# Export TMC resources
kubectl get clusterregistrations -o yaml > cluster-registrations-backup.yaml
kubectl get workloadplacements -A -o yaml > placement-policies-backup.yaml
```

## Recovery Procedures

### KCP Recovery
1. Restore etcd from backup
2. Restart KCP with TMC features enabled
3. Verify TMC controllers are running
4. Check cluster registrations

### Cluster Reconnection
```bash
# Re-register clusters after KCP recovery
kubectl apply -f cluster-registrations-backup.yaml

# Verify cluster connectivity
kubectl get clusterregistrations -o wide
```

### Workload Recovery
```bash
# Restore placement policies
kubectl apply -f placement-policies-backup.yaml

# Trigger workload re-evaluation
kubectl annotate workloadplacements --all recovery.tmc.io/trigger=$(date +%s)
```

## High Availability Setup

### Multi-Region KCP
- Deploy KCP in multiple regions
- Use external etcd cluster with geographic distribution
- Configure load balancer for KCP API access

### Cluster Failover
```yaml
# Configure automatic failover
apiVersion: placement.kcp.io/v1alpha1
kind: WorkloadPlacement
metadata:
  name: ha-placement
spec:
  placementPolicy:
    clusters:
    - name: primary-cluster
      weight: 100
    - name: dr-cluster
      weight: 0
  constraints:
    scheduling:
      overflowPolicy: spillover
```

## Testing Procedures

### DR Testing Checklist
- [ ] Backup procedures validated
- [ ] Recovery time objectives met
- [ ] Cluster reconnection verified
- [ ] Workload placement restored
- [ ] Performance impact measured

### Automated Testing
```bash
# DR simulation script
#!/bin/bash
echo "Starting DR test..."
kubectl delete clusterregistration primary-cluster
sleep 30
kubectl get workloadplacements -o wide
echo "DR test complete"
```