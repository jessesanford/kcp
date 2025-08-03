# TMC Health Monitoring System

The TMC Health Monitoring System provides comprehensive health tracking and monitoring for all TMC components across multiple clusters. It enables real-time health assessment, proactive alerting, and automated recovery coordination.

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                    TMC Health Monitoring System                │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐ │
│  │ Health Monitor  │  │ Health          │  │ Component       │ │
│  │ (Central)       │  │ Providers       │  │ Health Checks   │ │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘ │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐ │
│  │ Health          │  │ Cluster Health  │  │ System Health   │ │
│  │ Aggregation     │  │ Providers       │  │ Providers       │ │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

## Health Status Levels

### Component Health States
- **Healthy**: All systems operational, no issues detected
- **Degraded**: Some issues present, partial functionality affected
- **Unhealthy**: Critical issues detected, functionality severely impacted
- **Unknown**: Unable to determine component health status

### Health Transition Rules
- **Healthy → Degraded**: Response time increases or minor errors detected
- **Degraded → Unhealthy**: Error rate exceeds threshold or critical failure
- **Unhealthy → Degraded**: Recovery progress detected, some functionality restored
- **Degraded → Healthy**: All metrics return to normal ranges

## TMC Component Types

### Core Components
- **PlacementController**: Workload placement logic
- **SyncTargetController**: Cluster synchronization management
- **MigrationEngine**: Workload migration orchestration
- **StrategyRegistry**: Migration and placement strategy management

### Data Management
- **ClusterHealthTracker**: Per-cluster health monitoring
- **RolloutCoordinator**: Multi-cluster rollout management
- **VirtualWorkspaceManager**: Virtual workspace coordination
- **ResourceAggregator**: Cross-cluster resource aggregation
- **ProjectionController**: Resource projection management

### System Components
- **RecoveryManager**: Automated recovery coordination

## Health Check Interface

### HealthProvider Interface

```go
type HealthProvider interface {
    // GetHealth returns the current health status of the component
    GetHealth(ctx context.Context) *HealthCheck
    
    // GetComponentID returns a unique identifier for this component instance
    GetComponentID() string
    
    // GetComponentType returns the type of this component
    GetComponentType() ComponentType
}
```

### HealthCheck Structure

```go
type HealthCheck struct {
    ComponentType ComponentType
    ComponentID   string
    Status        HealthStatus
    Message       string
    Details       map[string]interface{}
    Timestamp     time.Time
    Duration      time.Duration
    Error         error
}
```

## Health Provider Implementations

### Base Health Provider

```go
// Create a simple health provider
healthProvider := NewBaseHealthProvider(
    ComponentTypeSyncTargetController,
    "syncer-prod-cluster",
    func(ctx context.Context) *HealthCheck {
        return &HealthCheck{
            ComponentType: ComponentTypeSyncTargetController,
            ComponentID:   "syncer-prod-cluster",
            Status:        HealthStatusHealthy,
            Message:       "Syncer operating normally",
            Details: map[string]interface{}{
                "syncedResources": 1250,
                "errorRate":       0.001,
                "lastSync":        time.Now().Add(-30 * time.Second),
            },
            Timestamp: time.Now(),
        }
    },
)
```

### Cluster Health Provider

```go
// Create cluster-specific health provider
clusterHealth := NewClusterHealthProvider(
    ComponentTypeSyncTargetController,
    "prod-cluster-1",
    logicalcluster.Name("root:production"),
)

// Record activity to update health metrics
clusterHealth.RecordActivity()  // Successful operations
clusterHealth.RecordError()     // Failed operations

// Health assessment considers:
// - Time since last activity
// - Error rate vs success rate
// - Cluster connectivity status
```

### System Health Provider

```go
// Create system-wide health provider
systemHealth := NewSystemHealthProvider(
    ComponentTypeRecoveryManager,
    "recovery-manager-instance",
)

// Update system metrics
systemHealth.UpdateMetrics(map[string]interface{}{
    "activeRecoveries":    3,
    "recoverySuccessRate": 0.95,
    "memoryUsage":         "256MB",
    "goroutineCount":      150,
})
```

## Health Monitor Configuration

### Creating Health Monitor

```go
// Create health monitor
healthMonitor := NewHealthMonitor()

// Configure thresholds
healthMonitor.SetDegradedThreshold(2 * time.Minute)
healthMonitor.SetUnhealthyThreshold(5 * time.Minute)
healthMonitor.SetCheckInterval(30 * time.Second)
healthMonitor.SetHealthTimeout(10 * time.Second)

// Start monitoring
ctx := context.Background()
go healthMonitor.Start(ctx)
```

### Registering Health Providers

```go
// Register component health providers
healthMonitor.RegisterHealthProvider(syncerHealthProvider)
healthMonitor.RegisterHealthProvider(placementHealthProvider)
healthMonitor.RegisterHealthProvider(migrationHealthProvider)

// Providers are automatically checked at configured intervals
```

## Health Assessment Criteria

### Response Time Based Assessment

```go
// Health status determined by check duration
func assessHealthByDuration(duration time.Duration) HealthStatus {
    if duration > 5*time.Minute {
        return HealthStatusUnhealthy
    } else if duration > 2*time.Minute {
        return HealthStatusDegraded
    }
    return HealthStatusHealthy
}
```

### Error Rate Based Assessment

```go
// Health based on error rate
func assessHealthByErrorRate(errorCount, successCount int64) HealthStatus {
    if successCount == 0 {
        return HealthStatusUnknown
    }
    
    errorRate := float64(errorCount) / float64(errorCount + successCount)
    if errorRate > 0.5 {
        return HealthStatusUnhealthy
    } else if errorRate > 0.1 {
        return HealthStatusDegraded
    }
    return HealthStatusHealthy
}
```

### Activity Based Assessment

```go
// Health based on recent activity
func assessHealthByActivity(lastActivity time.Time) HealthStatus {
    timeSinceActivity := time.Since(lastActivity)
    if timeSinceActivity > 5*time.Minute {
        return HealthStatusDegraded
    }
    return HealthStatusHealthy
}
```

## Health Aggregation

### Overall System Health

```go
// Get overall TMC system health
overallHealth := healthMonitor.GetOverallHealth()

// Aggregation logic:
// - If any component is unhealthy → System unhealthy
// - If any component is degraded → System degraded  
// - If all components are healthy → System healthy
// - If no components registered → System unknown
```

### Cluster-Specific Health Aggregation

```go
// Health aggregator for cluster-specific views
healthAggregator := NewHealthAggregator(healthMonitor)

// Get aggregated health for specific cluster
clusterHealth := healthAggregator.GetClusterHealth("prod-cluster-1")

// Returns health summary for all components in the cluster
```

### Component Type Aggregation

```go
// Get health for all syncers
syncerComponents := healthMonitor.GetAllComponentHealth()
syncerHealth := filterByComponentType(syncerComponents, ComponentTypeSyncTargetController)

// Aggregate syncer health across all clusters
aggregatedSyncerHealth := aggregateComponentHealth(syncerHealth)
```

## Health Check Examples

### Syncer Health Check

```go
func (s *Syncer) GetHealth(ctx context.Context) *HealthCheck {
    status := HealthStatusHealthy
    message := "Syncer operational"
    details := make(map[string]interface{})
    
    // Check if engine is started
    if !s.engine.IsStarted() {
        return &HealthCheck{
            ComponentType: ComponentTypeSyncTargetController,
            ComponentID:   s.syncTargetName,
            Status:        HealthStatusUnhealthy,
            Message:       "Syncer engine not started",
            Timestamp:     time.Now(),
        }
    }
    
    // Check resource controller count
    activeControllers := s.engine.GetActiveControllerCount()
    details["activeControllers"] = activeControllers
    
    if activeControllers == 0 {
        status = HealthStatusUnhealthy
        message = "No active resource controllers"
    }
    
    // Check error rate
    errorRate := s.metrics.GetErrorRate()
    details["errorRate"] = errorRate
    
    if errorRate > 0.5 {
        status = HealthStatusUnhealthy
        message = fmt.Sprintf("High error rate: %.2f%%", errorRate*100)
    } else if errorRate > 0.1 {
        status = HealthStatusDegraded
        message = fmt.Sprintf("Elevated error rate: %.2f%%", errorRate*100)
    }
    
    // Check last sync time
    lastSync := s.engine.GetLastSyncTime()
    timeSinceSync := time.Since(lastSync)
    details["lastSync"] = lastSync
    details["timeSinceLastSync"] = timeSinceSync.String()
    
    if timeSinceSync > 10*time.Minute {
        status = HealthStatusDegraded
        message = "No recent sync activity"
    }
    
    // Check heartbeat status
    if s.statusReporter != nil {
        heartbeatHealth := s.statusReporter.GetHeartbeatHealth()
        details["heartbeatHealth"] = heartbeatHealth
        
        if !heartbeatHealth.Healthy {
            status = HealthStatusUnhealthy
            message = "Heartbeat not healthy"
        }
    }
    
    return &HealthCheck{
        ComponentType: ComponentTypeSyncTargetController,
        ComponentID:   s.syncTargetName,
        Status:        status,
        Message:       message,
        Details:       details,
        Timestamp:     time.Now(),
    }
}
```

### Placement Controller Health Check

```go
func (pc *PlacementController) GetHealth(ctx context.Context) *HealthCheck {
    status := HealthStatusHealthy
    message := "Placement controller operational"
    details := make(map[string]interface{})
    
    // Check queue size
    queueSize := pc.queue.Len()
    details["queueSize"] = queueSize
    
    if queueSize > 1000 {
        status = HealthStatusDegraded
        message = fmt.Sprintf("High queue size: %d", queueSize)
    }
    
    // Check placement success rate
    successRate := pc.metrics.GetPlacementSuccessRate()
    details["placementSuccessRate"] = successRate
    
    if successRate < 0.8 {
        status = HealthStatusUnhealthy
        message = fmt.Sprintf("Low placement success rate: %.2f%%", successRate*100)
    } else if successRate < 0.95 {
        status = HealthStatusDegraded
        message = fmt.Sprintf("Reduced placement success rate: %.2f%%", successRate*100)
    }
    
    // Check cluster availability
    availableClusters := pc.getAvailableClusterCount()
    totalClusters := pc.getTotalClusterCount()
    details["availableClusters"] = availableClusters
    details["totalClusters"] = totalClusters
    
    availabilityRatio := float64(availableClusters) / float64(totalClusters)
    if availabilityRatio < 0.5 {
        status = HealthStatusUnhealthy
        message = "Insufficient cluster availability"
    } else if availabilityRatio < 0.8 {
        status = HealthStatusDegraded
        message = "Reduced cluster availability"
    }
    
    return &HealthCheck{
        ComponentType: ComponentTypePlacementController,
        ComponentID:   pc.controllerID,
        Status:        status,
        Message:       message,
        Details:       details,
        Timestamp:     time.Now(),
    }
}
```

### Virtual Workspace Health Check

```go
func (vwm *VirtualWorkspaceManager) GetHealth(ctx context.Context) *HealthCheck {
    status := HealthStatusHealthy
    message := "Virtual workspace manager operational"
    details := make(map[string]interface{})
    
    // Check active virtual workspaces
    activeWorkspaces := len(vwm.virtualWorkspaces)
    details["activeWorkspaces"] = activeWorkspaces
    
    // Check aggregation health
    aggregationErrors := 0
    projectionErrors := 0
    
    for _, aggregator := range vwm.resourceAggregators {
        if aggregator.GetErrorCount() > 0 {
            aggregationErrors++
        }
    }
    
    for _, controller := range vwm.projectionControllers {
        if controller.GetErrorCount() > 0 {
            projectionErrors++
        }
    }
    
    details["aggregationErrors"] = aggregationErrors
    details["projectionErrors"] = projectionErrors
    
    totalErrors := aggregationErrors + projectionErrors
    if totalErrors > activeWorkspaces/2 {
        status = HealthStatusUnhealthy
        message = "High error rate in workspace operations"
    } else if totalErrors > 0 {
        status = HealthStatusDegraded
        message = "Some workspace operations experiencing errors"
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

## Health Monitoring API

### Querying Component Health

```go
// Get specific component health
health, exists := healthMonitor.GetComponentHealth(
    ComponentTypeSyncTargetController,
    "syncer-prod-cluster",
)

if exists {
    fmt.Printf("Syncer health: %s - %s\n", health.Status, health.Message)
}
```

### Getting All Component Health

```go
// Get all component health statuses
allHealth := healthMonitor.GetAllComponentHealth()

for componentKey, health := range allHealth {
    fmt.Printf("%s: %s\n", componentKey, health.Status)
}
```

### Health Status Filtering

```go
// Filter by component type
syncerComponents := make(map[string]*HealthCheck)
for key, health := range allHealth {
    if health.ComponentType == ComponentTypeSyncTargetController {
        syncerComponents[key] = health
    }
}

// Filter by health status
unhealthyComponents := make(map[string]*HealthCheck)
for key, health := range allHealth {
    if health.Status == HealthStatusUnhealthy {
        unhealthyComponents[key] = health
    }
}
```

## Health Metrics Integration

### Prometheus Metrics

```go
// Health status metrics
tmc_component_health{component_type="SyncTargetController", component_id="syncer-1", cluster="prod"} 1.0
tmc_component_health{component_type="PlacementController", component_id="placement-1", cluster="prod"} 0.5

// Health check duration
tmc_health_check_duration_seconds{component_type="SyncTargetController"} 0.045

// Health check error rate
tmc_health_check_errors_total{component_type="VirtualWorkspaceManager"} 3
```

### Health Alerting

```go
// Prometheus alerting rules
groups:
- name: tmc.health
  rules:
  - alert: TMCComponentUnhealthy
    expr: tmc_component_health < 0.5
    for: 2m
    labels:
      severity: critical
    annotations:
      summary: "TMC component {{ $labels.component_type }} is unhealthy"
      description: "Component {{ $labels.component_id }} has been unhealthy for more than 2 minutes"
  
  - alert: TMCSystemDegraded
    expr: avg(tmc_component_health) < 0.8
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "TMC system performance is degraded"
      description: "Overall system health is below 80% for more than 5 minutes"
```

## Health Dashboard

### Grafana Dashboard Queries

```promql
# Overall system health
avg(tmc_component_health)

# Component health by type
avg(tmc_component_health) by (component_type)

# Unhealthy component count
count(tmc_component_health < 0.5)

# Health check failure rate
rate(tmc_health_check_errors_total[5m]) / rate(tmc_health_checks_total[5m])
```

### Health Status Dashboard

```json
{
  "dashboard": {
    "title": "TMC Health Monitoring",
    "panels": [
      {
        "title": "Overall System Health",
        "type": "stat",
        "targets": [{
          "expr": "avg(tmc_component_health)",
          "legendFormat": "System Health"
        }]
      },
      {
        "title": "Component Health by Type",
        "type": "bargauge",
        "targets": [{
          "expr": "avg(tmc_component_health) by (component_type)",
          "legendFormat": "{{ component_type }}"
        }]
      },
      {
        "title": "Unhealthy Components",
        "type": "table",
        "targets": [{
          "expr": "tmc_component_health < 0.5",
          "format": "table"
        }]
      }
    ]
  }
}
```

## Integration Examples

### Syncer Integration

```go
// Register syncer health provider
func (s *Syncer) RegisterHealthProvider(healthMonitor *HealthMonitor) {
    healthProvider := &SyncerHealthProvider{
        syncer: s,
        componentType: ComponentTypeSyncTargetController,
        componentID:   s.syncTargetName,
    }
    
    healthMonitor.RegisterHealthProvider(healthProvider)
}

// Health provider implementation
type SyncerHealthProvider struct {
    syncer        *Syncer
    componentType ComponentType
    componentID   string
}

func (shp *SyncerHealthProvider) GetHealth(ctx context.Context) *HealthCheck {
    return shp.syncer.GetHealth(ctx)
}

func (shp *SyncerHealthProvider) GetComponentID() string {
    return shp.componentID
}

func (shp *SyncerHealthProvider) GetComponentType() ComponentType {
    return shp.componentType
}
```

### Recovery Manager Integration

```go
// Health-triggered recovery
func (rm *RecoveryManager) monitorHealthAndRecover(ctx context.Context, healthMonitor *HealthMonitor) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            // Check for unhealthy components
            allHealth := healthMonitor.GetAllComponentHealth()
            for _, health := range allHealth {
                if health.Status == HealthStatusUnhealthy {
                    // Trigger recovery for unhealthy components
                    rm.TriggerComponentRecovery(health)
                }
            }
        }
    }
}
```

## Best Practices

### Health Check Implementation

1. **Keep Checks Fast**: Health checks should complete in < 5 seconds
2. **Provide Rich Details**: Include relevant metrics and status information  
3. **Use Appropriate Thresholds**: Set realistic degraded/unhealthy thresholds
4. **Handle Errors Gracefully**: Don't let health check failures cascade
5. **Include Recovery Hints**: Provide actionable information in health messages

### Health Provider Registration

1. **Register Early**: Register health providers during component initialization
2. **Use Descriptive IDs**: Component IDs should be unique and informative
3. **Clean Up**: Unregister providers when components shut down
4. **Handle Lifecycle**: Account for component start/stop states in health checks

### Health Monitoring Configuration

1. **Tune Check Intervals**: Balance between responsiveness and overhead
2. **Set Appropriate Timeouts**: Prevent health checks from hanging
3. **Configure Thresholds**: Align degraded/unhealthy thresholds with SLAs
4. **Monitor the Monitors**: Ensure health monitoring system itself is healthy

The TMC Health Monitoring System provides comprehensive visibility into system health with automated alerting and recovery coordination capabilities.