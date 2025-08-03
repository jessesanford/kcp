# TMC Placement Controller

The TMC Placement Controller provides intelligent workload placement capabilities, managing how workloads are distributed across multiple clusters based on policies, constraints, and real-time cluster conditions. It ensures optimal resource utilization and meets placement requirements while maintaining high availability.

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                    TMC Placement Controller                    │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐ │
│  │ Placement       │  │ Cluster         │  │ Constraint      │ │
│  │ Engine          │  │ Evaluator       │  │ Validator       │ │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘ │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐ │
│  │ Resource        │  │ Affinity        │  │ Load            │ │
│  │ Analyzer        │  │ Processor       │  │ Balancer        │ │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

## Core Concepts

### Placement Strategies

#### Multi-Cluster Placement
- **Spread**: Distribute workloads evenly across available clusters
- **Pack**: Concentrate workloads in minimal number of clusters
- **Balanced**: Balance workload distribution based on resource availability
- **Affinity-Based**: Place workloads according to affinity rules

#### Scheduling Policies
- **Resource-Aware**: Consider CPU, memory, and storage availability
- **Latency-Optimized**: Minimize cross-cluster communication latency
- **Cost-Optimized**: Choose clusters based on operational costs
- **Compliance-Driven**: Ensure regulatory and policy compliance

### Placement Constraints

#### Hard Constraints (Must be satisfied)
- **Resource Requirements**: Minimum CPU, memory, storage
- **Cluster Capabilities**: Required features, APIs, or services
- **Security Policies**: Data residency, compliance requirements
- **Network Constraints**: Connectivity and bandwidth requirements

#### Soft Constraints (Preferred but not required)
- **Performance Preferences**: SSD storage, high-performance networking
- **Cost Preferences**: Lower-cost regions or resource types
- **Redundancy Preferences**: Multiple availability zones
- **Latency Preferences**: Geographic proximity to users

## Placement Controller Implementation

### Core Structure

```go
type PlacementController struct {
    // Core components
    placementEngine     *PlacementEngine
    clusterEvaluator   *ClusterEvaluator
    constraintValidator *ConstraintValidator
    resourceAnalyzer   *ResourceAnalyzer
    
    // Workqueue for processing placement requests
    queue workqueue.RateLimitingInterface
    
    // Cluster information
    clusterCache       map[string]*ClusterInfo
    clusterHealthCache map[string]*ClusterHealth
    
    // Configuration
    placementStrategies map[string]PlacementStrategy
    defaultStrategy     PlacementStrategy
    
    // Metrics
    placementCount     int64
    successfulPlacements int64
    failedPlacements    int64
    
    // Control
    stopCh chan struct{}
    mu     sync.RWMutex
}
```

### Placement Engine

```go
type PlacementEngine struct {
    strategies        map[string]PlacementStrategy
    clusterEvaluator *ClusterEvaluator
    resourceAnalyzer *ResourceAnalyzer
}

type PlacementStrategy interface {
    // Evaluate potential clusters for placement
    EvaluateClusters(ctx context.Context, request *PlacementRequest, 
                    clusters []*ClusterInfo) ([]*ClusterCandidate, error)
    
    // Select final clusters for placement
    SelectClusters(ctx context.Context, candidates []*ClusterCandidate, 
                  request *PlacementRequest) ([]*ClusterSelection, error)
    
    // Get strategy priority (higher = more preferred)
    GetPriority() int
    
    // Get strategy name
    GetName() string
}
```

### Creating Placement Controller

```go
// Initialize placement controller
placementController := NewPlacementController(
    dynamicClient,
    placementInformer,
    syncTargetInformer,
    clusterHealthProvider,
)

// Configure placement strategies
placementController.RegisterStrategy("spread", &SpreadStrategy{})
placementController.RegisterStrategy("pack", &PackStrategy{})
placementController.RegisterStrategy("balanced", &BalancedStrategy{})
placementController.SetDefaultStrategy("balanced")

// Start the controller
ctx := context.Background()
go placementController.Start(ctx)
```

## Placement Strategies

### Spread Strategy

Distributes workloads evenly across available clusters to maximize availability.

```go
type SpreadStrategy struct {
    maxReplicasPerCluster int
    minimumClusters       int
}

func (s *SpreadStrategy) EvaluateClusters(ctx context.Context, 
    request *PlacementRequest, clusters []*ClusterInfo) ([]*ClusterCandidate, error) {
    
    candidates := make([]*ClusterCandidate, 0)
    
    for _, cluster := range clusters {
        // Check if cluster meets basic requirements
        if !s.meetsRequirements(cluster, request) {
            continue
        }
        
        // Calculate placement score
        score := s.calculateSpreadScore(cluster, request)
        
        candidates = append(candidates, &ClusterCandidate{
            Cluster: cluster,
            Score:   score,
            Reason:  "Spread strategy evaluation",
        })
    }
    
    // Sort by score (higher is better)
    sort.Slice(candidates, func(i, j int) bool {
        return candidates[i].Score > candidates[j].Score
    })
    
    return candidates, nil
}

func (s *SpreadStrategy) calculateSpreadScore(cluster *ClusterInfo, 
    request *PlacementRequest) float64 {
    
    // Base score starts at 100
    score := 100.0
    
    // Reduce score based on current workload density
    workloadDensity := float64(cluster.CurrentWorkloads) / float64(cluster.MaxWorkloads)
    score -= workloadDensity * 50
    
    // Boost score for clusters with lower resource utilization
    resourceUtilization := (cluster.CPUUsage + cluster.MemoryUsage) / 2
    score += (1.0 - resourceUtilization) * 25
    
    // Boost score for healthy clusters
    if cluster.Health == "Healthy" {
        score += 25
    } else if cluster.Health == "Degraded" {
        score -= 15
    } else {
        score -= 50
    }
    
    return score
}
```

### Pack Strategy

Concentrates workloads in minimal number of clusters to optimize resource utilization.

```go
type PackStrategy struct {
    resourceThreshold float64 // Pack until this utilization level
    maxClusters      int      // Maximum clusters to use
}

func (s *PackStrategy) EvaluateClusters(ctx context.Context, 
    request *PlacementRequest, clusters []*ClusterInfo) ([]*ClusterCandidate, error) {
    
    candidates := make([]*ClusterCandidate, 0)
    
    // Sort clusters by current utilization (highest first)
    sort.Slice(clusters, func(i, j int) bool {
        return clusters[i].GetUtilization() > clusters[j].GetUtilization()
    })
    
    for _, cluster := range clusters {
        if !s.meetsRequirements(cluster, request) {
            continue
        }
        
        // Check if cluster can accommodate the workload
        if cluster.GetUtilization() + request.EstimatedResourceUsage > s.resourceThreshold {
            continue
        }
        
        score := s.calculatePackScore(cluster, request)
        
        candidates = append(candidates, &ClusterCandidate{
            Cluster: cluster,
            Score:   score,
            Reason:  "Pack strategy evaluation",
        })
    }
    
    return candidates, nil
}
```

### Balanced Strategy

Balances workload distribution based on multiple factors including resources, health, and capacity.

```go
type BalancedStrategy struct {
    resourceWeight  float64
    healthWeight    float64
    capacityWeight  float64
    latencyWeight   float64
}

func (s *BalancedStrategy) EvaluateClusters(ctx context.Context, 
    request *PlacementRequest, clusters []*ClusterInfo) ([]*ClusterCandidate, error) {
    
    candidates := make([]*ClusterCandidate, 0)
    
    for _, cluster := range clusters {
        if !s.meetsRequirements(cluster, request) {
            continue
        }
        
        score := s.calculateBalancedScore(cluster, request)
        
        candidates = append(candidates, &ClusterCandidate{
            Cluster: cluster,
            Score:   score,
            Reason:  "Balanced strategy evaluation",
        })
    }
    
    return candidates, nil
}

func (s *BalancedStrategy) calculateBalancedScore(cluster *ClusterInfo, 
    request *PlacementRequest) float64 {
    
    // Calculate individual component scores
    resourceScore := s.calculateResourceScore(cluster, request)
    healthScore := s.calculateHealthScore(cluster)
    capacityScore := s.calculateCapacityScore(cluster, request)
    latencyScore := s.calculateLatencyScore(cluster, request)
    
    // Weighted average
    totalWeight := s.resourceWeight + s.healthWeight + s.capacityWeight + s.latencyWeight
    weightedScore := (resourceScore*s.resourceWeight + 
                     healthScore*s.healthWeight + 
                     capacityScore*s.capacityWeight + 
                     latencyScore*s.latencyWeight) / totalWeight
    
    return weightedScore
}
```

## Cluster Evaluation

### Cluster Information

```go
type ClusterInfo struct {
    Name             string
    Region           string
    Zone             string
    Health           string
    LastSeen         time.Time
    
    // Resource information
    TotalCPU         int64
    TotalMemory      int64
    TotalStorage     int64
    AvailableCPU     int64
    AvailableMemory  int64
    AvailableStorage int64
    
    // Utilization metrics
    CPUUsage         float64
    MemoryUsage      float64
    StorageUsage     float64
    
    // Workload information
    CurrentWorkloads int32
    MaxWorkloads     int32
    
    // Network information
    NetworkLatency   map[string]time.Duration
    Bandwidth        int64
    
    // Capabilities
    SupportedAPIs    []string
    Features         []string
    Labels           map[string]string
    
    // Cost information
    CPUCost          float64
    MemoryCost       float64
    StorageCost      float64
}
```

### Cluster Evaluator

```go
type ClusterEvaluator struct {
    clusterCache  map[string]*ClusterInfo
    healthMonitor *HealthMonitor
    metricsClient *MetricsClient
    mu            sync.RWMutex
}

func (ce *ClusterEvaluator) EvaluateCluster(ctx context.Context, 
    cluster *ClusterInfo, request *PlacementRequest) (*ClusterEvaluation, error) {
    
    evaluation := &ClusterEvaluation{
        Cluster:    cluster,
        Timestamp:  time.Now(),
        Suitable:   true,
        Violations: make([]string, 0),
        Score:      0.0,
    }
    
    // Check hard constraints
    if err := ce.checkHardConstraints(cluster, request, evaluation); err != nil {
        evaluation.Suitable = false
        evaluation.Violations = append(evaluation.Violations, err.Error())
    }
    
    // Check resource availability
    if err := ce.checkResourceAvailability(cluster, request, evaluation); err != nil {
        evaluation.Suitable = false
        evaluation.Violations = append(evaluation.Violations, err.Error())
    }
    
    // Check cluster health
    if err := ce.checkClusterHealth(cluster, evaluation); err != nil {
        evaluation.Suitable = false
        evaluation.Violations = append(evaluation.Violations, err.Error())
    }
    
    // Calculate soft constraint score
    evaluation.Score = ce.calculateSoftConstraintScore(cluster, request)
    
    return evaluation, nil
}
```

## Constraint Processing

### Constraint Types

```go
type PlacementConstraint struct {
    Type        ConstraintType
    Required    bool // Hard constraint if true, soft if false
    Field       string
    Operator    ConstraintOperator
    Values      []string
    Tolerance   float64 // For soft constraints
}

type ConstraintType string

const (
    ConstraintTypeResource     ConstraintType = "Resource"
    ConstraintTypeRegion       ConstraintType = "Region"
    ConstraintTypeZone         ConstraintType = "Zone"
    ConstraintTypeLabel        ConstraintType = "Label"
    ConstraintTypeCapability   ConstraintType = "Capability"
    ConstraintTypeLatency      ConstraintType = "Latency"
    ConstraintTypeCost         ConstraintType = "Cost"
)
```

### Constraint Validator

```go
type ConstraintValidator struct {
    evaluators map[ConstraintType]ConstraintEvaluator
}

type ConstraintEvaluator interface {
    Evaluate(ctx context.Context, cluster *ClusterInfo, 
            constraint *PlacementConstraint) (*ConstraintResult, error)
}

// Example: Resource constraint evaluator
type ResourceConstraintEvaluator struct{}

func (rce *ResourceConstraintEvaluator) Evaluate(ctx context.Context, 
    cluster *ClusterInfo, constraint *PlacementConstraint) (*ConstraintResult, error) {
    
    result := &ConstraintResult{
        Constraint: constraint,
        Satisfied:  false,
        Score:      0.0,
        Reason:     "",
    }
    
    switch constraint.Field {
    case "cpu":
        required := parseResourceQuantity(constraint.Values[0])
        available := cluster.AvailableCPU
        
        if available >= required {
            result.Satisfied = true
            result.Score = float64(available-required) / float64(required)
            result.Reason = "Sufficient CPU available"
        } else {
            result.Reason = fmt.Sprintf("Insufficient CPU: need %d, have %d", 
                required, available)
        }
        
    case "memory":
        required := parseResourceQuantity(constraint.Values[0])
        available := cluster.AvailableMemory
        
        if available >= required {
            result.Satisfied = true
            result.Score = float64(available-required) / float64(required)
            result.Reason = "Sufficient memory available"
        } else {
            result.Reason = fmt.Sprintf("Insufficient memory: need %d, have %d", 
                required, available)
        }
    }
    
    return result, nil
}
```

## Placement Execution

### Placement Request Processing

```go
type PlacementRequest struct {
    ID                    string
    WorkloadType          string
    ResourceRequirements  *ResourceRequirements
    Constraints           []*PlacementConstraint
    Preferences           []*PlacementPreference
    Strategy              string
    NumberOfClusters      int
    Replicas              int32
    Namespace             string
    Labels                map[string]string
    
    // Estimated resource usage for planning
    EstimatedResourceUsage float64
    
    // Timing
    RequestTime    time.Time
    Deadline       *time.Time
    
    // Context
    UserID         string
    RequestContext map[string]interface{}
}

func (pc *PlacementController) ProcessPlacementRequest(ctx context.Context, 
    request *PlacementRequest) (*PlacementDecision, error) {
    
    logger := klog.FromContext(ctx).WithValues("placementID", request.ID)
    
    // Get available clusters
    clusters, err := pc.getAvailableClusters(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to get available clusters: %w", err)
    }
    
    // Get placement strategy
    strategy, err := pc.getPlacementStrategy(request.Strategy)
    if err != nil {
        return nil, fmt.Errorf("failed to get placement strategy: %w", err)
    }
    
    // Evaluate clusters
    candidates, err := strategy.EvaluateClusters(ctx, request, clusters)
    if err != nil {
        return nil, fmt.Errorf("failed to evaluate clusters: %w", err)
    }
    
    // Validate constraints
    validCandidates, err := pc.validateConstraints(ctx, candidates, request)
    if err != nil {
        return nil, fmt.Errorf("failed to validate constraints: %w", err)
    }
    
    // Select final clusters
    selections, err := strategy.SelectClusters(ctx, validCandidates, request)
    if err != nil {
        return nil, fmt.Errorf("failed to select clusters: %w", err)
    }
    
    // Create placement decision
    decision := &PlacementDecision{
        RequestID:     request.ID,
        Selections:    selections,
        Strategy:      strategy.GetName(),
        DecisionTime:  time.Now(),
        Metadata:      make(map[string]interface{}),
    }
    
    logger.Info("Placement decision created", 
        "strategy", decision.Strategy,
        "selectedClusters", len(decision.Selections))
    
    return decision, nil
}
```

### Placement Decision

```go
type PlacementDecision struct {
    RequestID       string
    Selections      []*ClusterSelection
    Strategy        string
    DecisionTime    time.Time
    Metadata        map[string]interface{}
    
    // Quality metrics
    OverallScore    float64
    ConstraintScore float64
    ResourceScore   float64
    
    // Execution details
    ExecutionPlan   *ExecutionPlan
    Fallbacks       []*ClusterSelection
}

type ClusterSelection struct {
    Cluster      *ClusterInfo
    Replicas     int32
    Resources    *ResourceAllocation
    Score        float64
    Reason       string
    Constraints  []*ConstraintResult
}

type ExecutionPlan struct {
    Steps         []*ExecutionStep
    Dependencies  map[string][]string
    Rollback      *RollbackPlan
    Timeline      *ExecutionTimeline
}
```

## Affinity and Anti-Affinity

### Pod Affinity Rules

```go
type PodAffinity struct {
    RequiredDuringSchedulingIgnoredDuringExecution  []PodAffinityTerm
    PreferredDuringSchedulingIgnoredDuringExecution []WeightedPodAffinityTerm
}

type PodAffinityTerm struct {
    LabelSelector *metav1.LabelSelector
    Namespaces    []string
    TopologyKey   string
}

// Example: Process pod affinity
func (pc *PlacementController) processPodAffinity(ctx context.Context, 
    request *PlacementRequest, candidates []*ClusterCandidate) ([]*ClusterCandidate, error) {
    
    filteredCandidates := make([]*ClusterCandidate, 0)
    
    for _, candidate := range candidates {
        affinityScore := 0.0
        
        // Check required affinity
        if request.PodAffinity != nil {
            for _, term := range request.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution {
                if pc.evaluateAffinityTerm(candidate.Cluster, term) {
                    affinityScore += 100.0
                } else {
                    // Hard requirement not met, skip this candidate
                    continue
                }
            }
            
            // Check preferred affinity
            for _, weightedTerm := range request.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
                if pc.evaluateAffinityTerm(candidate.Cluster, weightedTerm.PodAffinityTerm) {
                    affinityScore += float64(weightedTerm.Weight)
                }
            }
        }
        
        candidate.Score += affinityScore
        filteredCandidates = append(filteredCandidates, candidate)
    }
    
    return filteredCandidates, nil
}
```

### Node Affinity

```go
type NodeAffinity struct {
    RequiredDuringSchedulingIgnoredDuringExecution  *NodeSelector
    PreferredDuringSchedulingIgnoredDuringExecution []PreferredSchedulingTerm
}

func (pc *PlacementController) processNodeAffinity(ctx context.Context, 
    request *PlacementRequest, candidates []*ClusterCandidate) ([]*ClusterCandidate, error) {
    
    if request.NodeAffinity == nil {
        return candidates, nil
    }
    
    filteredCandidates := make([]*ClusterCandidate, 0)
    
    for _, candidate := range candidates {
        // Check required node selector
        if request.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {
            if !pc.evaluateNodeSelector(candidate.Cluster, 
                request.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution) {
                // Hard requirement not met
                continue
            }
        }
        
        // Calculate preference score
        preferenceScore := 0.0
        for _, preferred := range request.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
            if pc.evaluateNodeSelectorTerm(candidate.Cluster, preferred.Preference) {
                preferenceScore += float64(preferred.Weight)
            }
        }
        
        candidate.Score += preferenceScore
        filteredCandidates = append(filteredCandidates, candidate)
    }
    
    return filteredCandidates, nil
}
```

## Load Balancing

### Load Balancing Strategies

```go
type LoadBalancer struct {
    strategies map[string]LoadBalancingStrategy
    metrics    *LoadBalancingMetrics
}

type LoadBalancingStrategy interface {
    DistributeReplicas(ctx context.Context, request *PlacementRequest, 
                      clusters []*ClusterSelection) error
    GetName() string
}

// Round-robin distribution
type RoundRobinStrategy struct{}

func (rr *RoundRobinStrategy) DistributeReplicas(ctx context.Context, 
    request *PlacementRequest, clusters []*ClusterSelection) error {
    
    if len(clusters) == 0 {
        return fmt.Errorf("no clusters available for distribution")
    }
    
    totalReplicas := request.Replicas
    replicasPerCluster := totalReplicas / int32(len(clusters))
    remainingReplicas := totalReplicas % int32(len(clusters))
    
    for i, cluster := range clusters {
        cluster.Replicas = replicasPerCluster
        if i < int(remainingReplicas) {
            cluster.Replicas++
        }
    }
    
    return nil
}

// Weighted distribution based on cluster capacity
type WeightedStrategy struct{}

func (ws *WeightedStrategy) DistributeReplicas(ctx context.Context, 
    request *PlacementRequest, clusters []*ClusterSelection) error {
    
    // Calculate total capacity
    totalCapacity := 0.0
    for _, cluster := range clusters {
        capacity := float64(cluster.Cluster.AvailableCPU + cluster.Cluster.AvailableMemory)
        totalCapacity += capacity
    }
    
    // Distribute based on capacity ratios
    totalReplicas := request.Replicas
    for _, cluster := range clusters {
        capacity := float64(cluster.Cluster.AvailableCPU + cluster.Cluster.AvailableMemory)
        ratio := capacity / totalCapacity
        cluster.Replicas = int32(float64(totalReplicas) * ratio)
    }
    
    // Adjust for rounding errors
    actualTotal := int32(0)
    for _, cluster := range clusters {
        actualTotal += cluster.Replicas
    }
    
    if actualTotal < totalReplicas {
        clusters[0].Replicas += totalReplicas - actualTotal
    }
    
    return nil
}
```

## Health Integration

### Placement Controller Health

```go
func (pc *PlacementController) GetHealth(ctx context.Context) *HealthCheck {
    status := HealthStatusHealthy
    message := "Placement controller operational"
    details := make(map[string]interface{})
    
    pc.mu.RLock()
    queueSize := pc.queue.Len()
    activePlacements := len(pc.activePlacements)
    totalClusters := len(pc.clusterCache)
    healthyClusters := 0
    
    for _, cluster := range pc.clusterCache {
        if cluster.Health == "Healthy" {
            healthyClusters++
        }
    }
    pc.mu.RUnlock()
    
    details["queueSize"] = queueSize
    details["activePlacements"] = activePlacements
    details["totalClusters"] = totalClusters
    details["healthyClusters"] = healthyClusters
    details["placementSuccessRate"] = pc.getSuccessRate()
    
    // Check queue size
    if queueSize > 1000 {
        status = HealthStatusDegraded
        message = fmt.Sprintf("High queue size: %d", queueSize)
    }
    
    // Check cluster availability
    availabilityRatio := float64(healthyClusters) / float64(totalClusters)
    if availabilityRatio < 0.5 {
        status = HealthStatusUnhealthy
        message = "Insufficient healthy clusters"
    } else if availabilityRatio < 0.8 {
        status = HealthStatusDegraded
        message = "Reduced cluster availability"
    }
    
    // Check success rate
    successRate := pc.getSuccessRate()
    if successRate < 0.8 {
        status = HealthStatusUnhealthy
        message = fmt.Sprintf("Low placement success rate: %.2f%%", successRate*100)
    } else if successRate < 0.95 {
        status = HealthStatusDegraded
        message = fmt.Sprintf("Reduced placement success rate: %.2f%%", successRate*100)
    }
    
    return &HealthCheck{
        ComponentType: ComponentTypePlacementController,
        ComponentID:   "placement-controller",
        Status:        status,
        Message:       message,
        Details:       details,
        Timestamp:     time.Now(),
    }
}
```

## Metrics and Monitoring

### Placement Metrics

```go
// Prometheus metrics automatically tracked
tmc_placement_requests_total{strategy="balanced", status="success"} 1250
tmc_placement_requests_total{strategy="spread", status="failed"} 25
tmc_placement_duration_seconds{strategy="balanced"} 0.125
tmc_placement_clusters_selected{strategy="spread"} 3
tmc_placement_constraints_violated{type="resource"} 5
tmc_placement_queue_size 15
tmc_cluster_capacity_utilization{cluster="prod-east"} 0.75
tmc_placement_success_rate 0.98
```

### Performance Monitoring

```go
func (pc *PlacementController) recordPlacementMetrics(decision *PlacementDecision, 
    duration time.Duration, err error) {
    
    strategy := decision.Strategy
    status := "success"
    if err != nil {
        status = "failed"
    }
    
    // Record basic metrics
    pc.metricsCollector.RecordPlacementTotal(strategy, status)
    pc.metricsCollector.RecordPlacementDuration(strategy, duration)
    pc.metricsCollector.RecordPlacementClusters(strategy, len(decision.Selections))
    
    // Record quality metrics
    pc.metricsCollector.RecordPlacementScore(strategy, decision.OverallScore)
    pc.metricsCollector.RecordConstraintScore(strategy, decision.ConstraintScore)
    
    // Update success rate
    pc.updateSuccessRate(err == nil)
}
```

## Usage Examples

### Basic Placement

```go
// Create placement request
request := &PlacementRequest{
    ID:           "web-app-placement",
    WorkloadType: "web-application",
    ResourceRequirements: &ResourceRequirements{
        CPU:    "2",
        Memory: "4Gi",
        Storage: "10Gi",
    },
    Strategy:         "balanced",
    NumberOfClusters: 3,
    Replicas:         6,
    Namespace:        "production",
}

// Process placement
decision, err := placementController.ProcessPlacementRequest(ctx, request)
if err != nil {
    log.Error(err, "Placement failed")
    return
}

// Execute placement
err = placementController.ExecutePlacement(ctx, decision)
if err != nil {
    log.Error(err, "Placement execution failed")
    return
}
```

### Advanced Placement with Constraints

```go
// Create placement with constraints
request := &PlacementRequest{
    ID:           "database-placement",
    WorkloadType: "database",
    ResourceRequirements: &ResourceRequirements{
        CPU:     "8",
        Memory:  "32Gi",
        Storage: "100Gi",
    },
    Constraints: []*PlacementConstraint{
        {
            Type:     ConstraintTypeRegion,
            Required: true,
            Field:    "region",
            Operator: ConstraintOperatorIn,
            Values:   []string{"us-east-1", "us-west-2"},
        },
        {
            Type:     ConstraintTypeLabel,
            Required: true,
            Field:    "workload-type",
            Operator: ConstraintOperatorEquals,
            Values:   []string{"database"},
        },
        {
            Type:      ConstraintTypeLatency,
            Required:  false,
            Field:     "latency",
            Operator:  ConstraintOperatorLessThan,
            Values:    []string{"10ms"},
            Tolerance: 0.2,
        },
    },
    Strategy: "pack",
    Replicas: 3,
}
```

## Best Practices

### Placement Strategy Selection

1. **Use Spread for High Availability**: When availability is more important than efficiency
2. **Use Pack for Resource Efficiency**: When minimizing resource waste is priority
3. **Use Balanced for General Workloads**: Good default for most applications
4. **Consider Workload Characteristics**: CPU-intensive vs. memory-intensive vs. I/O-intensive

### Constraint Design

1. **Minimize Hard Constraints**: Use soft constraints when possible for flexibility
2. **Design for Failure**: Ensure constraints don't eliminate all placement options
3. **Consider Compliance**: Include regulatory and security requirements as hard constraints
4. **Balance Performance and Cost**: Use appropriate weights for optimization goals

### Monitoring and Optimization

1. **Monitor Placement Success Rates**: Track and alert on placement failures
2. **Analyze Cluster Utilization**: Ensure even distribution and optimal utilization
3. **Review Constraint Violations**: Identify and address common constraint conflicts
4. **Tune Strategy Parameters**: Adjust weights and thresholds based on performance

The TMC Placement Controller provides intelligent, policy-driven workload placement across multiple clusters, ensuring optimal resource utilization while meeting application requirements and organizational constraints.