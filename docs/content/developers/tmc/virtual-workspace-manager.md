# TMC Virtual Workspace Manager

The TMC Virtual Workspace Manager provides cross-cluster aggregation and projection capabilities, creating unified views of distributed workloads across multiple clusters. It enables transparent multi-cluster resource management through virtual workspace abstractions.

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                Virtual Workspace Manager                       │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐ │
│  │ Virtual         │  │ Resource        │  │ Workload        │ │
│  │ Workspace       │  │ Aggregator      │  │ Projection      │ │
│  │ Manager         │  │                 │  │ Controller      │ │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘ │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐ │
│  │ Placement       │  │ Cluster Health  │  │ Resource        │ │
│  │ Monitoring      │  │ Tracking        │  │ Transformation  │ │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

## Virtual Workspace Concepts

### Virtual Workspace

A Virtual Workspace provides a unified view of distributed workloads across multiple clusters, created from a Placement resource and managing both aggregated and projected resources.

```go
type VirtualWorkspace struct {
    Name                string
    Namespace           string
    LogicalCluster      logicalcluster.Name
    SourcePlacement     *workloadv1alpha1.Placement
    TargetClusters      []ClusterReference
    AggregatedResources map[schema.GroupVersionKind]*AggregatedResourceView
    ProjectedResources  map[schema.GroupVersionKind]*ProjectedResourceView
    Status              VirtualWorkspaceStatus
    CreatedTime         time.Time
    LastUpdated         time.Time
}
```

### Virtual Workspace Status

- **Pending**: Virtual workspace is being initialized
- **Active**: Virtual workspace is operational and managing resources
- **Synchronizing**: Virtual workspace is performing cross-cluster synchronization
- **Error**: Virtual workspace encountered an error condition
- **Terminating**: Virtual workspace is being cleaned up

## Resource Aggregation

### Aggregated Resource View

Resource aggregation provides a unified view of resources that exist across multiple clusters, combining their state and health information.

```go
type AggregatedResourceView struct {
    GVK            schema.GroupVersionKind
    Resources      map[string]*AggregatedResource
    TotalCount     int
    HealthyCount   int
    UnhealthyCount int
    LastAggregated time.Time
}
```

### Aggregated Resource

```go
type AggregatedResource struct {
    Name           string
    Namespace      string
    ClusterOrigins map[string]*ClusterResourceInstance
    AggregatedSpec *unstructured.Unstructured
    Status         AggregatedResourceStatus
    Conditions     []metav1.Condition
}
```

### Cluster Resource Instance

```go
type ClusterResourceInstance struct {
    ClusterName  string
    Resource     *unstructured.Unstructured
    Health       ResourceHealth
    LastObserved time.Time
    Conditions   []metav1.Condition
}
```

### Aggregation Strategies

#### Union Strategy
Combines all resource instances across clusters, providing a comprehensive view of distributed resources.

#### Intersection Strategy
Shows only resources that exist in all target clusters, useful for identifying consistent deployments.

#### Priority Strategy
Uses cluster-based priority to determine which resource instance takes precedence in the aggregated view.

#### Latest Strategy
Uses the most recently updated resource instance as the authoritative source.

## Resource Projection

### Projected Resource View

Resource projection replicates resources from source clusters to target clusters based on projection policies.

```go
type ProjectedResourceView struct {
    GVK              schema.GroupVersionKind
    SourceResources  map[string]*unstructured.Unstructured
    ProjectedTo      map[string]*ProjectedResourceInstance
    ProjectionPolicy ProjectionPolicy
    LastProjected    time.Time
}
```

### Projection Policies

```go
type ProjectionPolicy struct {
    Mode              ProjectionMode
    TargetClusters    []string
    ResourceSelectors []ResourceSelector
    Transformations   []ResourceTransformation
}
```

#### Projection Modes

- **All**: Project resources to all available clusters
- **Selective**: Project only to explicitly specified clusters
- **Conditional**: Project based on runtime conditions and cluster state

#### Resource Selectors

```go
type ResourceSelector struct {
    GVK           schema.GroupVersionKind
    LabelSelector labels.Selector
    FieldSelector string
    NamePattern   string
}
```

#### Resource Transformations

```go
type ResourceTransformation struct {
    Type        TransformationType
    JSONPath    string
    Value       interface{}
    Conditional string
}
```

**Transformation Types**:
- **Set**: Set a field value
- **Delete**: Remove a field
- **Replace**: Replace field value
- **Template**: Apply template-based transformation

## Using Virtual Workspace Manager

### Initialization

```go
// Create virtual workspace manager
vwm, err := NewVirtualWorkspaceManager(
    kcpClusterClient,
    dynamicClient,
    placementInformer,
    syncTargetInformer,
)
if err != nil {
    log.Fatal(err, "Failed to create virtual workspace manager")
}

// Start the manager
ctx := context.Background()
numThreads := 2
go vwm.Start(ctx, numThreads)
```

### Querying Virtual Workspaces

```go
// Get specific virtual workspace
cluster := logicalcluster.Name("root:production")
vw, exists := vwm.GetVirtualWorkspace(cluster, "web-app-placement")
if exists {
    fmt.Printf("Virtual workspace status: %s\n", vw.Status)
    fmt.Printf("Target clusters: %d\n", len(vw.TargetClusters))
    fmt.Printf("Aggregated resources: %d types\n", len(vw.AggregatedResources))
}

// List all virtual workspaces
workspaces := vwm.ListVirtualWorkspaces()
for _, vw := range workspaces {
    fmt.Printf("Workspace: %s/%s - Status: %s\n", 
        vw.LogicalCluster, vw.Name, vw.Status)
}
```

### Resource Aggregation Examples

#### Viewing Aggregated Deployments

```go
// Get aggregated view of deployments
vw, exists := vwm.GetVirtualWorkspace(cluster, "web-app-placement")
if exists {
    deploymentGVK := schema.GroupVersionKind{
        Group:   "apps",
        Version: "v1", 
        Kind:    "Deployment",
    }
    
    if aggregatedView, exists := vw.AggregatedResources[deploymentGVK]; exists {
        fmt.Printf("Total deployments: %d\n", aggregatedView.TotalCount)
        fmt.Printf("Healthy deployments: %d\n", aggregatedView.HealthyCount)
        
        for name, resource := range aggregatedView.Resources {
            fmt.Printf("Deployment %s:\n", name)
            fmt.Printf("  Status: %s\n", resource.Status)
            fmt.Printf("  Clusters: %v\n", getClusterNames(resource.ClusterOrigins))
        }
    }
}

func getClusterNames(origins map[string]*ClusterResourceInstance) []string {
    clusters := make([]string, 0, len(origins))
    for cluster := range origins {
        clusters = append(clusters, cluster)
    }
    return clusters
}
```

#### Cross-Cluster Health Assessment

```go
// Assess health across clusters
func assessCrossClusterHealth(vw *VirtualWorkspace) {
    for gvk, aggregatedView := range vw.AggregatedResources {
        fmt.Printf("Resource type: %s\n", gvk.String())
        
        healthyPercentage := float64(aggregatedView.HealthyCount) / float64(aggregatedView.TotalCount) * 100
        fmt.Printf("Health: %.1f%% (%d/%d healthy)\n", 
            healthyPercentage, aggregatedView.HealthyCount, aggregatedView.TotalCount)
        
        // Check resource distribution
        for name, resource := range aggregatedView.Resources {
            clusterCount := len(resource.ClusterOrigins)
            expectedClusters := len(vw.TargetClusters)
            
            if clusterCount < expectedClusters {
                fmt.Printf("Warning: %s only in %d/%d clusters\n", 
                    name, clusterCount, expectedClusters)
            }
        }
    }
}
```

### Resource Projection Examples

#### Projecting ConfigMaps Across Clusters

```go
// Set up projection policy for ConfigMaps
projectionPolicy := ProjectionPolicy{
    Mode: ProjectionModeSelective,
    TargetClusters: []string{"cluster-a", "cluster-b"},
    ResourceSelectors: []ResourceSelector{
        {
            GVK: schema.GroupVersionKind{
                Group:   "",
                Version: "v1",
                Kind:    "ConfigMap",
            },
            LabelSelector: labels.SelectorFromSet(labels.Set{
                "app.kubernetes.io/component": "config",
            }),
        },
    },
    Transformations: []ResourceTransformation{
        {
            Type:     TransformationTypeSet,
            JSONPath: "metadata.labels.workload.kcp.io/projected",
            Value:    "true",
        },
        {
            Type:     TransformationTypeTemplate,
            JSONPath: "metadata.name",
            Value:    "{{.OriginalName}}-{{.TargetCluster}}",
        },
    },
}
```

#### Conditional Projection Based on Cluster Capacity

```go
// Project resources based on cluster capacity
func setupConditionalProjection() ProjectionPolicy {
    return ProjectionPolicy{
        Mode: ProjectionModeConditional,
        ResourceSelectors: []ResourceSelector{
            {
                GVK: schema.GroupVersionKind{
                    Group:   "apps",
                    Version: "v1",
                    Kind:    "Deployment",
                },
                LabelSelector: labels.SelectorFromSet(labels.Set{
                    "workload.kcp.io/auto-project": "true",
                }),
            },
        },
        Transformations: []ResourceTransformation{
            {
                Type:        TransformationTypeSet,
                JSONPath:    "spec.replicas",
                Conditional: "cluster.capacity.cpu > 4",
                Value:       3,
            },
            {
                Type:        TransformationTypeSet,
                JSONPath:    "spec.replicas", 
                Conditional: "cluster.capacity.cpu <= 4",
                Value:       1,
            },
        },
    }
}
```

## Cross-Cluster Resource Aggregator

### Creating Resource Aggregator

```go
// Initialize cross-cluster resource aggregator
aggregator, err := NewCrossClusterResourceAggregator(
    virtualWorkspace,
    dynamicClient,
    clusterDynamicClients,
)
if err != nil {
    return fmt.Errorf("failed to create aggregator: %w", err)
}

// Start aggregation
ctx := context.Background()
go aggregator.Start(ctx)
```

### Aggregation Policies

```go
// Configure aggregation policy
type AggregationPolicy struct {
    GVK                schema.GroupVersionKind
    MergeStrategy      ResourceMergeStrategy
    ConflictResolution ConflictResolutionStrategy
    HealthAggregation  HealthAggregationStrategy
    StatusMerging      StatusMergingStrategy
    LabelSelectors     []labels.Selector
    FieldSelectors     []string
    Transformations    []AggregationTransformation
}

// Example aggregation policy
policy := &AggregationPolicy{
    GVK: schema.GroupVersionKind{
        Group:   "apps",
        Version: "v1",
        Kind:    "Deployment",
    },
    MergeStrategy:      MergeStrategyUnion,
    ConflictResolution: ConflictResolutionLastWriter,
    HealthAggregation:  HealthAggregationMajority,
    StatusMerging:      StatusMergingCombined,
}
```

### Merge Strategies

#### Union Merge Strategy
```go
// Combines all resource instances from different clusters
func (ccra *CrossClusterResourceAggregator) mergeSpecsUnion(
    clusterInstances map[string]*ClusterResourceInstance,
) (*unstructured.Unstructured, error) {
    // Start with the first instance as base
    var base *unstructured.Unstructured
    for _, instance := range clusterInstances {
        base = instance.Resource.DeepCopy()
        break
    }
    
    // Merge additional fields from other instances
    // Implementation would merge arrays, maps, etc.
    return base, nil
}
```

#### Priority Merge Strategy
```go
// Uses cluster priority to determine authoritative source
func (ccra *CrossClusterResourceAggregator) mergeSpecsPriority(
    clusterInstances map[string]*ClusterResourceInstance,
) (*unstructured.Unstructured, error) {
    // Use the first healthy instance with highest priority
    for _, instance := range clusterInstances {
        if instance.Health == ResourceHealthHealthy {
            return instance.Resource.DeepCopy(), nil
        }
    }
    
    // Fallback to any available instance
    for _, instance := range clusterInstances {
        return instance.Resource.DeepCopy(), nil
    }
    
    return nil, fmt.Errorf("no instances to merge")
}
```

## Workload Projection Controller

### Creating Projection Controller

```go
// Initialize workload projection controller
controller, err := NewWorkloadProjectionController(
    virtualWorkspace,
    dynamicClient,
    clusterDynamicClients,
)
if err != nil {
    return fmt.Errorf("failed to create projection controller: %w", err)
}

// Start projection
ctx := context.Background() 
go controller.Start(ctx)
```

### Projection Transformations

#### Set Transformation
```go
// Set field values during projection
transformation := ResourceTransformation{
    Type:     TransformationTypeSet,
    JSONPath: "metadata.labels.projected-from",
    Value:    sourceCluster,
}
```

#### Template Transformation
```go
// Apply template-based transformations
transformation := ResourceTransformation{
    Type:     TransformationTypeTemplate,
    JSONPath: "spec.template.spec.containers[0].env[0].value",
    Value:    "https://api.{{.TargetCluster}}.example.com",
}
```

#### Conditional Transformation
```go
// Apply transformations based on conditions
transformation := ResourceTransformation{
    Type:        TransformationTypeSet,
    JSONPath:    "spec.resources.requests.memory", 
    Value:       "1Gi",
    Conditional: "cluster.type == 'edge'",
}
```

## Monitoring and Status

### Virtual Workspace Status API

```go
// Get comprehensive status
func getVirtualWorkspaceStatus(vwm *VirtualWorkspaceManager, cluster logicalcluster.Name, name string) {
    vw, exists := vwm.GetVirtualWorkspace(cluster, name)
    if !exists {
        fmt.Printf("Virtual workspace not found\n")
        return
    }
    
    fmt.Printf("Virtual Workspace: %s/%s\n", vw.LogicalCluster, vw.Name)
    fmt.Printf("Status: %s\n", vw.Status)
    fmt.Printf("Created: %s\n", vw.CreatedTime.Format(time.RFC3339))
    fmt.Printf("Last Updated: %s\n", vw.LastUpdated.Format(time.RFC3339))
    
    // Target clusters
    fmt.Printf("\nTarget Clusters (%d):\n", len(vw.TargetClusters))
    for _, cluster := range vw.TargetClusters {
        status := "healthy"
        if !cluster.Healthy {
            status = "unhealthy"
        }
        fmt.Printf("  - %s (%s) - %s\n", cluster.Name, cluster.LogicalCluster, status)
    }
    
    // Aggregated resources
    fmt.Printf("\nAggregated Resources (%d types):\n", len(vw.AggregatedResources))
    for gvk, view := range vw.AggregatedResources {
        fmt.Printf("  - %s: %d total (%d healthy, %d unhealthy)\n", 
            gvk.Kind, view.TotalCount, view.HealthyCount, view.UnhealthyCount)
    }
    
    // Projected resources
    fmt.Printf("\nProjected Resources (%d types):\n", len(vw.ProjectedResources))
    for gvk, view := range vw.ProjectedResources {
        fmt.Printf("  - %s: %d source, %d projections\n",
            gvk.Kind, len(view.SourceResources), len(view.ProjectedTo))
    }
}
```

### Health Monitoring Integration

```go
// Virtual workspace manager health check
func (vwm *VirtualWorkspaceManager) GetHealth(ctx context.Context) *HealthCheck {
    status := HealthStatusHealthy
    message := "Virtual workspace manager operational"
    details := make(map[string]interface{})
    
    vwm.mu.RLock()
    workspaceCount := len(vwm.virtualWorkspaces)
    aggregatorCount := len(vwm.resourceAggregators)
    projectionCount := len(vwm.projectionControllers)
    vwm.mu.RUnlock()
    
    details["virtualWorkspaces"] = workspaceCount
    details["activeAggregators"] = aggregatorCount
    details["activeProjectionControllers"] = projectionCount
    
    // Check for workspace errors
    errorCount := 0
    for _, vw := range vwm.ListVirtualWorkspaces() {
        if vw.Status == VirtualWorkspaceStatusError {
            errorCount++
        }
    }
    
    if errorCount > 0 {
        status = HealthStatusDegraded
        message = fmt.Sprintf("%d virtual workspaces in error state", errorCount)
        details["errorWorkspaces"] = errorCount
    }
    
    return &HealthCheck{
        ComponentType: ComponentTypeVirtualWorkspaceManager,
        ComponentID:   "virtual-workspace-manager",
        Status:        status,
        Message:       message,
        Details:       details,
        Timestamp:     time.Now(),
    }
}
```

### Metrics Collection

```go
// Virtual workspace metrics
tmc_virtual_workspaces_total{status="active"} 15
tmc_virtual_workspaces_total{status="error"} 1
tmc_aggregated_resources_total{gvk="apps/v1/Deployment"} 45
tmc_projected_resources_total{gvk="v1/ConfigMap"} 23
tmc_aggregation_duration_seconds{workspace="web-app"} 2.5
tmc_projection_duration_seconds{workspace="api-service"} 1.2
tmc_cluster_health{cluster="prod-east", workspace="web-app"} 1.0
tmc_cluster_health{cluster="prod-west", workspace="web-app"} 0.0
```

## Configuration

### Virtual Workspace Manager Configuration

```go
// Configure virtual workspace manager
type VirtualWorkspaceManagerConfig struct {
    SyncInterval        time.Duration `json:"syncInterval"`
    AggregationEnabled  bool          `json:"aggregationEnabled"`
    ProjectionEnabled   bool          `json:"projectionEnabled"`
    MaxWorkers          int           `json:"maxWorkers"`
    HealthCheckInterval time.Duration `json:"healthCheckInterval"`
}

// Apply configuration
config := &VirtualWorkspaceManagerConfig{
    SyncInterval:        30 * time.Second,
    AggregationEnabled:  true,
    ProjectionEnabled:   true,
    MaxWorkers:          5,
    HealthCheckInterval: 60 * time.Second,
}

vwm.ApplyConfig(config)
```

### Cluster-Specific Configuration

```go
// Configure cluster-specific settings
type ClusterConfig struct {
    Name               string        `json:"name"`
    Priority           int           `json:"priority"`
    HealthCheckTimeout time.Duration `json:"healthCheckTimeout"`
    AggregationWeight  float64       `json:"aggregationWeight"`
    ProjectionEnabled  bool          `json:"projectionEnabled"`
}

clusterConfigs := map[string]ClusterConfig{
    "prod-east": {
        Priority:           90,
        HealthCheckTimeout: 30 * time.Second,
        AggregationWeight:  1.0,
        ProjectionEnabled:  true,
    },
    "prod-west": {
        Priority:           85,
        HealthCheckTimeout: 45 * time.Second,
        AggregationWeight:  0.8,
        ProjectionEnabled:  true,
    },
    "edge-cluster": {
        Priority:           50,
        HealthCheckTimeout: 60 * time.Second,
        AggregationWeight:  0.3,
        ProjectionEnabled:  false,
    },
}
```

## Best Practices

### Virtual Workspace Design

1. **Resource Scope**: Design virtual workspaces around logical application boundaries
2. **Cluster Selection**: Include clusters that share common operational characteristics
3. **Resource Types**: Focus on resources that benefit from cross-cluster visibility
4. **Update Frequency**: Configure sync intervals based on resource change frequency
5. **Health Monitoring**: Implement comprehensive health checks for all target clusters

### Aggregation Strategy

1. **Merge Policies**: Choose appropriate merge strategies based on resource semantics
2. **Conflict Resolution**: Define clear conflict resolution policies for resource differences
3. **Health Assessment**: Use appropriate health aggregation strategies for application requirements
4. **Performance**: Monitor aggregation performance and adjust polling intervals
5. **Data Consistency**: Account for eventual consistency in distributed environments

### Projection Strategy

1. **Transformation Design**: Keep transformations simple and idempotent
2. **Target Selection**: Carefully select target clusters based on resource requirements
3. **Security Context**: Ensure projected resources maintain appropriate security boundaries
4. **Resource Lifecycle**: Handle resource lifecycle events consistently across clusters
5. **Rollback Planning**: Design projection policies with rollback capabilities

### Monitoring and Alerting

1. **Health Dashboards**: Create dashboards showing virtual workspace health across clusters
2. **Resource Tracking**: Monitor resource distribution and health across target clusters
3. **Performance Metrics**: Track aggregation and projection performance
4. **Error Alerting**: Set up alerts for virtual workspace errors and unhealthy clusters
5. **Capacity Planning**: Monitor resource usage trends across virtual workspaces

The TMC Virtual Workspace Manager provides a powerful foundation for multi-cluster resource management with intelligent aggregation and projection capabilities that enable transparent distributed workload operation.