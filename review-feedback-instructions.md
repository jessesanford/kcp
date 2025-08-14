# Review Feedback Instructions - Wave 2B-02: Syncer Integration

## Current State
- Branch: `feature/tmc2-impl2/phase2/wave2b-02-split-from-virtual`
- Focus: Syncer client integration with virtual workspace
- Estimated current lines: ~280 lines

## Priority Issues (P0 - Must Fix)

### 1. Missing Test Coverage
**CRITICAL**: No tests for syncer integration

#### Required Test Files to Create:
1. `pkg/virtual/syncer/client_test.go` (~200 lines)
   - Test client creation
   - Test virtual resource access
   - Test error handling

2. `pkg/virtual/syncer/sync_test.go` (~150 lines)
   - Test sync loop
   - Test resource transformation

### 2. Complete Syncer Client Implementation

#### File: `pkg/virtual/syncer/client.go`
Implement syncer client (~180 lines):
```go
// SyncerClient provides access to virtual workspace
type SyncerClient struct {
    virtualURL   string
    restConfig   *rest.Config
    syncTargets  workloadv1alpha1client.SyncTargetInterface
    resyncPeriod time.Duration
}

// NewSyncerClient creates a client for syncer virtual workspace
func NewSyncerClient(config *rest.Config, virtualURL string) (*SyncerClient, error) {
    // Create REST config for virtual workspace
    virtualConfig := rest.CopyConfig(config)
    virtualConfig.Host = virtualURL
    virtualConfig.APIPath = "/apis"
    
    // Create typed client
    workloadClient, err := workloadv1alpha1client.NewForConfig(virtualConfig)
    if err != nil {
        return nil, fmt.Errorf("failed to create workload client: %w", err)
    }
    
    return &SyncerClient{
        virtualURL:   virtualURL,
        restConfig:   virtualConfig,
        syncTargets:  workloadClient.SyncTargets("kcp-syncer"),
        resyncPeriod: 30 * time.Second,
    }, nil
}

// Start begins the sync loop
func (c *SyncerClient) Start(ctx context.Context) error {
    logger := klog.FromContext(ctx)
    logger.Info("Starting syncer client", "virtualURL", c.virtualURL)
    
    // Initial sync
    if err := c.sync(ctx); err != nil {
        return fmt.Errorf("initial sync failed: %w", err)
    }
    
    // Start periodic sync
    ticker := time.NewTicker(c.resyncPeriod)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-ticker.C:
            if err := c.sync(ctx); err != nil {
                logger.Error(err, "Sync failed")
                // Continue on error, will retry next period
            }
        }
    }
}

// sync performs a sync operation
func (c *SyncerClient) sync(ctx context.Context) error {
    logger := klog.FromContext(ctx)
    
    // List sync targets from virtual workspace
    targets, err := c.syncTargets.List(ctx, metav1.ListOptions{})
    if err != nil {
        return fmt.Errorf("failed to list sync targets: %w", err)
    }
    
    logger.V(2).Info("Retrieved sync targets", "count", len(targets.Items))
    
    // Process each target
    for _, target := range targets.Items {
        if err := c.processTarget(ctx, &target); err != nil {
            logger.Error(err, "Failed to process target", "target", target.Name)
            // Continue processing other targets
        }
    }
    
    return nil
}

// processTarget handles individual sync target
func (c *SyncerClient) processTarget(ctx context.Context, target *workloadv1alpha1.SyncTarget) error {
    logger := klog.FromContext(ctx).WithValues("target", target.Name)
    
    // Update heartbeat
    now := metav1.Now()
    target.Status.LastHeartbeat = &now
    
    // Check target health
    healthy, err := c.checkTargetHealth(ctx, target)
    if err != nil {
        return fmt.Errorf("health check failed: %w", err)
    }
    
    // Update status condition
    condition := metav1.Condition{
        Type:               string(workloadv1alpha1.SyncTargetReady),
        LastTransitionTime: now,
    }
    
    if healthy {
        condition.Status = metav1.ConditionTrue
        condition.Reason = "Healthy"
        condition.Message = "Target is healthy and syncing"
        target.Status.Phase = workloadv1alpha1.SyncTargetPhaseReady
    } else {
        condition.Status = metav1.ConditionFalse
        condition.Reason = "Unhealthy"
        condition.Message = "Target health check failed"
        target.Status.Phase = workloadv1alpha1.SyncTargetPhaseNotReady
    }
    
    target.SetCondition(condition)
    
    // Update status in virtual workspace
    _, err = c.syncTargets.UpdateStatus(ctx, target, metav1.UpdateOptions{})
    if err != nil {
        return fmt.Errorf("failed to update status: %w", err)
    }
    
    logger.V(1).Info("Updated target status", "healthy", healthy)
    return nil
}

// checkTargetHealth verifies target cluster connectivity
func (c *SyncerClient) checkTargetHealth(ctx context.Context, target *workloadv1alpha1.SyncTarget) (bool, error) {
    // In real implementation, would check actual cluster health
    // For now, simplified check based on last heartbeat
    
    if target.Status.LastHeartbeat == nil {
        return false, nil
    }
    
    timeSinceHeartbeat := time.Since(target.Status.LastHeartbeat.Time)
    if timeSinceHeartbeat > 2*time.Minute {
        return false, nil
    }
    
    return true, nil
}
```

### 3. Add Resource Watching Support

#### File: `pkg/virtual/syncer/watcher.go` (NEW ~120 lines)
```go
package syncer

// ResourceWatcher watches virtual resources
type ResourceWatcher struct {
    client      *SyncerClient
    handlers    map[string]ResourceHandler
    stopCh      chan struct{}
}

// ResourceHandler processes resource events
type ResourceHandler interface {
    OnAdd(obj interface{})
    OnUpdate(oldObj, newObj interface{})
    OnDelete(obj interface{})
}

// NewResourceWatcher creates a watcher for virtual resources
func NewResourceWatcher(client *SyncerClient) *ResourceWatcher {
    return &ResourceWatcher{
        client:   client,
        handlers: make(map[string]ResourceHandler),
        stopCh:   make(chan struct{}),
    }
}

// Watch starts watching for resource changes
func (w *ResourceWatcher) Watch(ctx context.Context) error {
    // Create watch on virtual workspace
    watcher, err := w.client.syncTargets.Watch(ctx, metav1.ListOptions{})
    if err != nil {
        return fmt.Errorf("failed to create watch: %w", err)
    }
    defer watcher.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-w.stopCh:
            return nil
        case event, ok := <-watcher.ResultChan():
            if !ok {
                return fmt.Errorf("watch channel closed")
            }
            
            w.handleEvent(event)
        }
    }
}
```

## Line Count Analysis

### Current Estimate:
- Existing code: ~280 lines
- Required tests: ~350 lines
- Complete implementation: ~180 lines
- Watcher support: ~120 lines
- **Total after fixes: ~930 lines** ❌ OVER LIMIT

### NEEDS SPLIT Strategy:
Split into 2 PRs:
1. **Current PR**: Core syncer client (~500 lines)
2. **Follow-up PR**: Watching + comprehensive tests (~450 lines)

## Specific Tasks for THIS PR

### 1. Focus on Core Syncer Client
Implement only:
- Client creation
- Basic sync loop
- Status updates
- Health checking

### 2. Create Basic Test Coverage
```go
func TestSyncerClient(t *testing.T) {
    // Create test server
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.URL.Path == "/apis/workload.kcp.io/v1alpha1/synctargets" {
            targets := &workloadv1alpha1.SyncTargetList{
                Items: []workloadv1alpha1.SyncTarget{{
                    ObjectMeta: metav1.ObjectMeta{
                        Name: "test-target",
                    },
                }},
            }
            json.NewEncoder(w).Encode(targets)
        }
    }))
    defer server.Close()
    
    config := &rest.Config{
        Host: server.URL,
    }
    
    client, err := NewSyncerClient(config, server.URL)
    require.NoError(t, err)
    require.NotNil(t, client)
    
    // Test sync
    ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
    defer cancel()
    
    err = client.sync(ctx)
    require.NoError(t, err)
}
```

### 3. Defer to Follow-up PR
- Watch implementation
- Event handling
- Comprehensive test suite
- Retry logic
- Connection pooling

## Testing Requirements (This PR)

### Unit Test Coverage Target: 65%
1. **Client Tests**:
   - Client creation
   - Virtual workspace connection
   - Basic sync operation

2. **Health Check Tests**:
   - Heartbeat validation
   - Status updates

## Completion Checklist (This PR)

- [ ] Syncer client implemented
- [ ] Sync loop working
- [ ] Health checks functional
- [ ] Basic tests (65% coverage)
- [ ] Status updates working
- [ ] `make test` passes
- [ ] Line count < 700 lines
- [ ] TODO for watch support
- [ ] Clean commit history

## Follow-up PR Planning
Create Wave 2B-02b for:
- Watch implementation
- Event processing
- Comprehensive testing
- Retry mechanisms
- Performance optimization

## Notes
- Establish syncer pattern first
- Keep health checks simple
- Document virtual workspace protocol
- Design for watch extension