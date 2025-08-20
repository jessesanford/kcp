# Implementation Instructions: Heartbeat & Health Monitoring

## Branch: `feature/phase7-syncer-impl/p7w4-heartbeat`

## Overview
This branch implements heartbeat and health monitoring for the syncer, ensuring KCP knows the syncer is alive and healthy. It provides liveness/readiness probes, metrics collection, and health status reporting.

**Target Size**: ~450 lines  
**Complexity**: Medium  
**Priority**: High (ensures reliability)

## Dependencies
- **Phase 5 APIs**: Health check interfaces
- **Phase 6 Infrastructure**: SyncTarget status updates
- **Wave 4 WebSocket**: Uses connection for heartbeat
- **All Waves**: Monitors all sync components

## Files to Create

### 1. Health Monitor Core (~200 lines)
**File**: `pkg/reconciler/workload/syncer/health/monitor.go`
- Main health monitor struct
- Component health tracking
- Aggregated health status
- Health reporting

### 2. Heartbeat Manager (~100 lines)
**File**: `pkg/reconciler/workload/syncer/health/heartbeat.go`
- Heartbeat sender
- Heartbeat interval management
- Missed heartbeat detection
- Lease renewal

### 3. Health Checks (~80 lines)
**File**: `pkg/reconciler/workload/syncer/health/checks.go`
- Component health checks
- Resource availability checks
- Connection health checks
- Performance checks

### 4. Metrics Collection (~40 lines)
**File**: `pkg/reconciler/workload/syncer/health/metrics.go`
- Prometheus metrics
- Health metrics
- Performance metrics
- Error metrics

### 5. Health Tests (~30 lines)
**File**: `pkg/reconciler/workload/syncer/health/monitor_test.go`
- Unit tests for health monitoring
- Heartbeat tests
- Check aggregation tests

## Step-by-Step Implementation Guide

### Step 1: Create Package Structure
```bash
mkdir -p pkg/reconciler/workload/syncer/health
```

### Step 2: Define Health Types
Create `types.go` with:

```go
package health

import (
    "time"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HealthStatus represents overall health
type HealthStatus string

const (
    HealthStatusHealthy   HealthStatus = "Healthy"
    HealthStatusDegraded  HealthStatus = "Degraded"
    HealthStatusUnhealthy HealthStatus = "Unhealthy"
    HealthStatusUnknown   HealthStatus = "Unknown"
)

// ComponentHealth represents a component's health
type ComponentHealth struct {
    Name         string
    Status       HealthStatus
    LastCheck    time.Time
    Message      string
    Error        error
    Metrics      map[string]float64
}

// HealthReport contains complete health information
type HealthReport struct {
    Status         HealthStatus
    Components     map[string]*ComponentHealth
    LastHeartbeat  *metav1.Time
    Uptime         time.Duration
    Version        string
    
    // Performance metrics
    SyncRate       float64
    ErrorRate      float64
    QueueDepth     int
    Latency        time.Duration
    
    // Resource metrics
    MemoryUsage    int64
    CPUUsage       float64
    GoroutineCount int
}

// HeartbeatConfig configures heartbeat behavior
type HeartbeatConfig struct {
    Interval         time.Duration
    Timeout          time.Duration
    FailureThreshold int
    LeaseNamespace   string
    LeaseName        string
}
```

### Step 3: Implement Health Monitor
Create `monitor.go` with:

```go
package health

import (
    "context"
    "fmt"
    "runtime"
    "sync"
    "time"
    
    workloadv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"
    "k8s.io/client-go/kubernetes"
    "k8s.io/klog/v2"
    "github.com/kcp-dev/logicalcluster/v3"
)

// Monitor tracks health of all syncer components
type Monitor struct {
    kcpClient     kubernetes.ClusterInterface
    syncTarget    *workloadv1alpha1.SyncTarget
    workspace     logicalcluster.Name
    
    // Component tracking
    components    map[string]HealthChecker
    componentsMu  sync.RWMutex
    
    // Health state
    status        HealthStatus
    lastReport    *HealthReport
    startTime     time.Time
    
    // Heartbeat
    heartbeater   *Heartbeater
    
    // Metrics
    metrics       *MetricsCollector
    
    // Configuration
    checkInterval time.Duration
    
    // Lifecycle
    ctx           context.Context
    cancel        context.CancelFunc
}

// HealthChecker interface for component health checks
type HealthChecker interface {
    Name() string
    Check(ctx context.Context) *ComponentHealth
}

// NewMonitor creates a new health monitor
func NewMonitor(
    kcpClient kubernetes.ClusterInterface,
    syncTarget *workloadv1alpha1.SyncTarget,
    workspace logicalcluster.Name,
) *Monitor {
    ctx, cancel := context.WithCancel(context.Background())
    
    return &Monitor{
        kcpClient:     kcpClient,
        syncTarget:    syncTarget,
        workspace:     workspace,
        components:    make(map[string]HealthChecker),
        status:        HealthStatusUnknown,
        startTime:     time.Now(),
        checkInterval: 30 * time.Second,
        metrics:       NewMetricsCollector(),
        ctx:           ctx,
        cancel:        cancel,
    }
}

// Start begins health monitoring
func (m *Monitor) Start(ctx context.Context) error {
    logger := klog.FromContext(ctx)
    logger.Info("Starting health monitor")
    
    // Initialize heartbeater
    m.heartbeater = NewHeartbeater(
        m.kcpClient,
        m.syncTarget,
        m.workspace,
        HeartbeatConfig{
            Interval:         10 * time.Second,
            Timeout:          30 * time.Second,
            FailureThreshold: 3,
            LeaseNamespace:   m.syncTarget.Namespace,
            LeaseName:        fmt.Sprintf("%s-heartbeat", m.syncTarget.Name),
        },
    )
    
    // Start heartbeat
    if err := m.heartbeater.Start(ctx); err != nil {
        return fmt.Errorf("failed to start heartbeat: %w", err)
    }
    
    // Start health check loop
    go m.healthCheckLoop(ctx)
    
    // Start metrics collection
    go m.metrics.Start(ctx)
    
    logger.Info("Health monitor started")
    return nil
}

// RegisterComponent registers a component for health checking
func (m *Monitor) RegisterComponent(checker HealthChecker) {
    m.componentsMu.Lock()
    defer m.componentsMu.Unlock()
    
    m.components[checker.Name()] = checker
    klog.V(4).Info("Registered health check", "component", checker.Name())
}

// healthCheckLoop periodically checks component health
func (m *Monitor) healthCheckLoop(ctx context.Context) {
    ticker := time.NewTicker(m.checkInterval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            m.performHealthChecks(ctx)
        }
    }
}

// performHealthChecks checks all components
func (m *Monitor) performHealthChecks(ctx context.Context) {
    logger := klog.FromContext(ctx)
    
    m.componentsMu.RLock()
    checkers := make([]HealthChecker, 0, len(m.components))
    for _, checker := range m.components {
        checkers = append(checkers, checker)
    }
    m.componentsMu.RUnlock()
    
    // Perform checks in parallel
    results := make(map[string]*ComponentHealth)
    var wg sync.WaitGroup
    var mu sync.Mutex
    
    for _, checker := range checkers {
        wg.Add(1)
        go func(c HealthChecker) {
            defer wg.Done()
            
            // Add timeout to check
            checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
            defer cancel()
            
            health := c.Check(checkCtx)
            
            mu.Lock()
            results[c.Name()] = health
            mu.Unlock()
        }(checker)
    }
    
    wg.Wait()
    
    // Aggregate status
    status := m.aggregateStatus(results)
    
    // Create report
    report := &HealthReport{
        Status:         status,
        Components:     results,
        LastHeartbeat:  &metav1.Time{Time: time.Now()},
        Uptime:         time.Since(m.startTime),
        Version:        "v1alpha1",
        
        // Collect metrics
        SyncRate:       m.metrics.GetSyncRate(),
        ErrorRate:      m.metrics.GetErrorRate(),
        QueueDepth:     m.metrics.GetQueueDepth(),
        Latency:        m.metrics.GetLatency(),
        
        // Runtime metrics
        MemoryUsage:    m.getMemoryUsage(),
        CPUUsage:       m.getCPUUsage(),
        GoroutineCount: runtime.NumGoroutine(),
    }
    
    m.lastReport = report
    m.status = status
    
    // Update sync target status
    if err := m.updateSyncTargetStatus(ctx, report); err != nil {
        logger.Error(err, "Failed to update sync target status")
    }
    
    logger.V(4).Info("Health check completed", "status", status)
}

// aggregateStatus determines overall health from component statuses
func (m *Monitor) aggregateStatus(components map[string]*ComponentHealth) HealthStatus {
    if len(components) == 0 {
        return HealthStatusUnknown
    }
    
    unhealthyCount := 0
    degradedCount := 0
    
    for _, health := range components {
        switch health.Status {
        case HealthStatusUnhealthy:
            unhealthyCount++
        case HealthStatusDegraded:
            degradedCount++
        }
    }
    
    // Determine overall status
    if unhealthyCount > 0 {
        return HealthStatusUnhealthy
    }
    if degradedCount > len(components)/2 {
        return HealthStatusDegraded
    }
    if degradedCount > 0 {
        return HealthStatusDegraded
    }
    
    return HealthStatusHealthy
}

// GetHealth returns current health status
func (m *Monitor) GetHealth() HealthStatus {
    return m.status
}

// GetReport returns the latest health report
func (m *Monitor) GetReport() *HealthReport {
    return m.lastReport
}

// LivenessProbe implements liveness check
func (m *Monitor) LivenessProbe() error {
    if m.status == HealthStatusUnhealthy {
        return fmt.Errorf("syncer is unhealthy")
    }
    return nil
}

// ReadinessProbe implements readiness check
func (m *Monitor) ReadinessProbe() error {
    if m.status != HealthStatusHealthy {
        return fmt.Errorf("syncer is not ready: %s", m.status)
    }
    return nil
}
```

### Step 4: Implement Heartbeat Manager
Create `heartbeat.go` with:

1. **Heartbeater struct**:
   - Lease-based heartbeat
   - Regular heartbeat sending
   - Failure detection

2. **Heartbeat loop**:
   - Send heartbeat messages
   - Update lease
   - Track failures
   - Trigger alerts

3. **Lease management**:
   - Create/update lease
   - Handle lease conflicts
   - Graceful takeover

### Step 5: Implement Health Checks
Create `checks.go` with:

1. **Connection health check**:
```go
type ConnectionHealthCheck struct {
    manager *tunnel.Manager
}

func (c *ConnectionHealthCheck) Check(ctx context.Context) *ComponentHealth {
    health := &ComponentHealth{
        Name:      "websocket-connection",
        LastCheck: time.Now(),
    }
    
    if c.manager.IsConnected() {
        health.Status = HealthStatusHealthy
        health.Message = "Connected"
    } else {
        health.Status = HealthStatusUnhealthy
        health.Message = "Disconnected"
        health.Error = c.manager.LastError()
    }
    
    return health
}
```

2. **Sync engine health check**:
   - Queue depth check
   - Processing rate
   - Error rate

3. **Resource health check**:
   - Memory usage
   - CPU usage
   - Goroutine count

### Step 6: Implement Metrics Collection
Create `metrics.go` with:

1. **Prometheus metrics**:
   - Health status gauge
   - Component status
   - Heartbeat counter
   - Check duration

2. **Metric aggregation**:
   - Calculate rates
   - Track trends
   - Detect anomalies

### Step 7: Add Comprehensive Tests
Create test files covering:

1. **Health aggregation**:
   - Various component states
   - Status calculation
   - Edge cases

2. **Heartbeat**:
   - Regular heartbeat
   - Failure detection
   - Recovery

3. **Health checks**:
   - Component checks
   - Timeout handling
   - Error scenarios

## Testing Requirements

### Unit Tests:
- Health status aggregation
- Component registration
- Heartbeat logic
- Metrics collection
- Probe methods

### Integration Tests:
- Full health monitoring
- Heartbeat with lease
- Status updates
- Metric export

## Validation Checklist

- [ ] Health checks run periodically
- [ ] Status aggregates correctly
- [ ] Heartbeat maintains lease
- [ ] Metrics are collected
- [ ] Probes work correctly
- [ ] Status updates to KCP
- [ ] Comprehensive logging
- [ ] Prometheus metrics exposed
- [ ] Tests achieve >70% coverage
- [ ] Code follows KCP patterns
- [ ] Feature flags integrated
- [ ] Under 450 lines (excluding tests)

## Common Pitfalls to Avoid

1. **Don't block health checks** - use timeouts
2. **Cache health results** - avoid excessive checking
3. **Handle check failures** - don't crash on errors
4. **Clean up resources** - prevent leaks
5. **Rate limit updates** - don't flood KCP

## Integration Notes

This component:
- Monitors all Wave 1-3 components
- Uses Wave 4 WebSocket for heartbeat
- Updates SyncTarget status
- Provides Kubernetes probes

Should provide:
- HTTP health endpoints
- Prometheus metrics endpoint
- Detailed health reports
- Component registration API

## Success Criteria

The implementation is complete when:
1. Health status accurately reflects system state
2. Heartbeat keeps lease alive
3. Metrics are collected and exported
4. Probes work for Kubernetes
5. All tests pass
6. Can monitor 10+ components efficiently