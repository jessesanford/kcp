# Basic Usage Examples

This guide demonstrates common TMC usage patterns and workflows through practical examples.

## Deploying Workloads

### Simple Application Deployment

Deploy a basic web application across multiple clusters:

```bash
cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web-app
  labels:
    app.kubernetes.io/name: web-app
    app.kubernetes.io/managed-by: tmc
spec:
  replicas: 3
  selector:
    matchLabels:
      app: web-app
  template:
    metadata:
      labels:
        app: web-app
        app.kubernetes.io/managed-by: tmc
    spec:
      containers:
      - name: web
        image: nginx:1.21
        ports:
        - containerPort: 80
        resources:
          requests:
            cpu: "200m"
            memory: "256Mi"
          limits:
            cpu: "500m"
            memory: "512Mi"
---
apiVersion: v1
kind: Service
metadata:
  name: web-app-service
  labels:
    app.kubernetes.io/name: web-app
    app.kubernetes.io/managed-by: tmc
spec:
  selector:
    app: web-app
  ports:
  - port: 80
    targetPort: 80
  type: ClusterIP
EOF
```

### Database with Persistent Storage

Deploy a database with specific placement requirements:

```bash
cat <<EOF | kubectl apply -f -
apiVersion: placement.kcp.io/v1alpha1
kind: WorkloadPlacement
metadata:
  name: database-placement
spec:
  selector:
    matchLabels:
      app: postgres-db
  placementPolicy:
    clusters:
    - name: production-us-west
      weight: 100
    constraints:
      resources:
        cpu: "1"
        memory: "2Gi"
        storage: "50Gi"
      capabilities:
      - storage
      - networking
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: postgres-db
  labels:
    app: postgres-db
    app.kubernetes.io/managed-by: tmc
spec:
  serviceName: postgres
  replicas: 1
  selector:
    matchLabels:
      app: postgres-db
  template:
    metadata:
      labels:
        app: postgres-db
        app.kubernetes.io/managed-by: tmc
    spec:
      containers:
      - name: postgres
        image: postgres:13
        ports:
        - containerPort: 5432
        env:
        - name: POSTGRES_DB
          value: appdb
        - name: POSTGRES_USER
          value: dbuser
        - name: POSTGRES_PASSWORD
          value: secretpassword
        volumeMounts:
        - name: postgres-storage
          mountPath: /var/lib/postgresql/data
        resources:
          requests:
            cpu: "1"
            memory: "2Gi"
          limits:
            cpu: "2"
            memory: "4Gi"
  volumeClaimTemplates:
  - metadata:
      name: postgres-storage
    spec:
      accessModes:
      - ReadWriteOnce
      resources:
        requests:
          storage: 50Gi
EOF
```

## Placement Policies

### Geographic Distribution

Distribute replicas across geographic regions:

```bash
cat <<EOF | kubectl apply -f -
apiVersion: placement.kcp.io/v1alpha1
kind: WorkloadPlacement
metadata:
  name: geo-distributed-app
spec:
  selector:
    matchLabels:
      tier: frontend
  placementPolicy:
    clusters:
    - name: production-us-west
      weight: 50
    - name: production-us-east
      weight: 50
    constraints:
      topology:
        regions:
        - us-west-2
        - us-east-1
    scheduling:
      spreadConstraints:
      - maxSkew: 1
        topologyKey: "topology.kubernetes.io/region"
EOF
```

### Resource-Based Placement

Place workloads based on resource requirements:

```bash
cat <<EOF | kubectl apply -f -
apiVersion: placement.kcp.io/v1alpha1
kind: WorkloadPlacement
metadata:
  name: high-memory-workload
spec:
  selector:
    matchLabels:
      workload-type: memory-intensive
  placementPolicy:
    clusters:
    - name: high-memory-cluster
      weight: 100
    constraints:
      resources:
        memory: "8Gi"
      nodeSelector:
        node-type: memory-optimized
EOF
```

## Multi-Cluster Services

### Service Discovery

Access services across clusters:

```bash
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Service
metadata:
  name: distributed-api
  labels:
    app.kubernetes.io/managed-by: tmc
  annotations:
    placement.kcp.io/cluster-set: "production"
spec:
  selector:
    app: api-server
  ports:
  - port: 8080
    targetPort: 8080
  type: LoadBalancer
  clusterIPs:
  - auto  # TMC will manage cross-cluster IPs
EOF
```

### Cross-Cluster Communication

Configure service mesh integration:

```bash
cat <<EOF | kubectl apply -f -
apiVersion: placement.kcp.io/v1alpha1
kind: WorkloadPlacement
metadata:
  name: microservices-placement
spec:
  selector:
    matchLabels:
      app.kubernetes.io/part-of: microservices-app
  placementPolicy:
    clusters:
    - name: production-us-west
      weight: 60
    - name: production-us-east
      weight: 40
    constraints:
      networking:
        serviceMesh: istio
        crossClusterCommunication: enabled
EOF
```

## Scaling Scenarios

### Horizontal Scaling

Scale across multiple clusters:

```bash
# Create initial deployment
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: scalable-app
  labels:
    app.kubernetes.io/managed-by: tmc
spec:
  replicas: 5
  selector:
    matchLabels:
      app: scalable-app
  template:
    metadata:
      labels:
        app: scalable-app
        app.kubernetes.io/managed-by: tmc
    spec:
      containers:
      - name: app
        image: nginx:1.21
        resources:
          requests:
            cpu: "100m"
            memory: "128Mi"
EOF

# Scale up
kubectl scale deployment scalable-app --replicas=10

# TMC will automatically distribute across available clusters
```

### Cluster Capacity Management

Handle cluster resource exhaustion:

```bash
cat <<EOF | kubectl apply -f -
apiVersion: placement.kcp.io/v1alpha1
kind: WorkloadPlacement
metadata:
  name: overflow-handling
spec:
  selector:
    matchLabels:
      scaling: elastic
  placementPolicy:
    clusters:
    - name: primary-cluster
      weight: 80
    - name: overflow-cluster
      weight: 20
    constraints:
      scheduling:
        overflowPolicy: spillover
        maxReplicasPerCluster: 50
EOF
```

## Monitoring and Observability

### Health Checking

Monitor workload health across clusters:

```bash
# Add health check annotations
kubectl annotate deployment web-app \
  placement.kcp.io/health-check="http://localhost:80/health"
  
# View health status
kubectl get workloadplacements -o wide
kubectl describe workloadplacement default-placement
```

### Metrics Collection

Configure metrics aggregation:

```bash
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: monitoring-config
data:
  prometheus.yml: |
    global:
      scrape_interval: 15s
    scrape_configs:
    - job_name: 'tmc-workloads'
      kubernetes_sd_configs:
      - role: pod
        namespaces:
          names: ['default']
      relabel_configs:
      - source_labels: [__meta_kubernetes_pod_label_app_kubernetes_io_managed_by]
        action: keep
        regex: tmc
EOF
```

## Best Practices

### Resource Management

1. **Set Resource Limits**: Always specify CPU and memory limits
2. **Use Quality of Service**: Configure appropriate QoS classes
3. **Plan for Overhead**: Account for TMC operational overhead

### Placement Strategy

1. **Start Simple**: Begin with basic placement policies
2. **Monitor Distribution**: Watch how workloads are distributed
3. **Iterate and Optimize**: Refine policies based on observed behavior

### Security Considerations

1. **Network Policies**: Configure appropriate network segmentation
2. **RBAC**: Use workspace-scoped permissions
3. **Secret Management**: Secure sensitive data across clusters

## Common Patterns

### Blue-Green Deployments

```bash
# Deploy to staging cluster first
cat <<EOF | kubectl apply -f -
apiVersion: placement.kcp.io/v1alpha1
kind: WorkloadPlacement
metadata:
  name: blue-green-staging
spec:
  selector:
    matchLabels:
      version: v2
  placementPolicy:
    clusters:
    - name: staging-cluster
      weight: 100
EOF

# After validation, promote to production
kubectl patch workloadplacement blue-green-staging --type='merge' -p='
{
  "spec": {
    "placementPolicy": {
      "clusters": [
        {"name": "production-cluster", "weight": 100}
      ]
    }
  }
}'
```

### Canary Releases

```bash
# Deploy small percentage to canary cluster
cat <<EOF | kubectl apply -f -
apiVersion: placement.kcp.io/v1alpha1
kind: WorkloadPlacement
metadata:
  name: canary-release
spec:
  selector:
    matchLabels:
      release: canary
  placementPolicy:
    clusters:
    - name: production-cluster
      weight: 90
    - name: canary-cluster
      weight: 10
EOF
```

## Next Steps

- Explore [API Reference](../api-reference/) for detailed API specifications
- Learn about [Performance Tuning](../operations/performance-tuning.md)
- Review [Troubleshooting](../troubleshooting/) for common issues