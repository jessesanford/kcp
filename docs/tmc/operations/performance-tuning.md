# Performance Tuning and Monitoring

## TMC Controller Optimization

### Resource Allocation
```yaml
# Controller resource tuning
resources:
  requests:
    cpu: "500m"
    memory: "1Gi"
  limits:
    cpu: "2"
    memory: "4Gi"
```

### Sync Configuration
```yaml
# Optimize sync intervals
syncInterval: "30s"
resyncInterval: "10m"
maxConcurrentSyncs: 10
```

## Monitoring Setup

### Key Metrics

- **Placement latency**: Time from workload creation to placement
- **Cluster heartbeat**: Last successful cluster communication
- **Resource utilization**: CPU/memory usage across clusters
- **API call rate**: TMC API request volume

### Prometheus Configuration
```yaml
# Monitor TMC metrics
scrape_configs:
- job_name: 'tmc-controllers'
  static_configs:
  - targets: ['tmc-controller:8080']
  metrics_path: /metrics
```

### Important Metrics
```promql
# Placement decision time
tmc_placement_decision_duration_seconds

# Cluster sync status
tmc_cluster_sync_status

# Resource utilization
tmc_cluster_resource_utilization_ratio
```

## Performance Best Practices

### Placement Optimization
- Use specific selectors to reduce workload evaluation
- Minimize placement policy complexity
- Cache frequently accessed cluster information

### Resource Management
- Monitor cluster capacity trends
- Set appropriate resource limits
- Use horizontal pod autoscaling for controllers

### Network Optimization
- Co-locate KCP and frequently accessed clusters
- Use dedicated network connections for high-traffic scenarios
- Monitor network latency between KCP and clusters

## Scaling Considerations

### Controller Scaling
- Scale TMC controllers based on cluster count
- Use separate controllers for different workload types
- Monitor controller resource usage

### Database Optimization
- Tune etcd for TMC resource storage
- Consider dedicated etcd cluster for large deployments
- Monitor etcd performance metrics