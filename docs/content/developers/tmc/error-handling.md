# TMC Error Handling System

The TMC Error Handling System provides comprehensive error categorization, automatic recovery strategies, and robust failure handling for all TMC operations. This system ensures reliable multi-cluster workload management with intelligent error recovery.

## Architecture Overview

The error handling system consists of several key components:

```
┌─────────────────────────────────────────────────────────────────┐
│                    TMC Error Handling System                   │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐ │
│  │ Error           │  │ Recovery        │  │ Circuit         │ │
│  │ Categorization  │  │ Strategies      │  │ Breaker         │ │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘ │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐ │
│  │ Retry Logic     │  │ Error           │  │ Condition       │ │
│  │ & Backoff       │  │ Reporting       │  │ Management      │ │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

## Error Types and Categories

### Resource Errors
- **ResourceNotFound**: Resource missing in source cluster
- **ResourceConflict**: Resource version conflicts during sync
- **ResourceValidation**: Invalid resource specifications
- **ResourcePermission**: Insufficient RBAC permissions

### Cluster Errors
- **ClusterUnreachable**: Cannot connect to cluster endpoint
- **ClusterUnavailable**: Cluster API server unavailable
- **ClusterAuthentication**: Authentication failures
- **ClusterConfiguration**: Cluster configuration issues

### Placement Errors
- **PlacementConstraint**: Constraint violations during placement
- **PlacementCapacity**: Insufficient cluster capacity
- **PlacementPolicy**: Policy validation failures

### Sync Errors
- **SyncFailure**: General synchronization failures
- **SyncConflict**: Resource conflicts during sync
- **SyncTimeout**: Sync operations timing out

### Migration Errors
- **MigrationFailure**: Workload migration failures
- **MigrationTimeout**: Migration operations timing out
- **MigrationRollback**: Rollback operation failures

### System Errors
- **InternalError**: Internal system errors
- **ConfigurationError**: Configuration validation failures
- **NetworkConnectivity**: Network connectivity issues

## Error Severity Levels

### Critical Errors
- **ClusterAuthentication**: Immediate attention required
- **MigrationRollback**: Data integrity at risk
- **InternalError**: System stability compromised

### High Severity
- **ClusterUnreachable**: Service disruption possible
- **MigrationFailure**: Workload availability at risk
- **PlacementConstraint**: Resource placement blocked

### Medium Severity
- **ResourceConflict**: Temporary sync issues
- **SyncFailure**: Isolated sync problems
- **AggregationFailure**: Reduced visibility

### Low Severity
- **ResourceNotFound**: Expected cleanup scenarios
- **ResourceValidation**: User input errors
- **SyncTimeout**: Transient timing issues

## TMCError Structure

```go
type TMCError struct {
    Type         TMCErrorType     // Error category
    Severity     TMCErrorSeverity // Severity level
    Component    string           // Component that generated error
    Operation    string           // Operation that failed
    Message      string           // Human-readable description
    Cause        error           // Underlying error
    Timestamp    time.Time       // When error occurred
    Context      map[string]interface{} // Additional context
    Retryable    bool            // Whether error is retryable
    RecoveryHint string          // Suggested recovery action

    // Cluster context
    ClusterName    string
    LogicalCluster string

    // Resource context
    GVK       schema.GroupVersionKind
    Namespace string
    Name      string
}
```

## Error Builder Pattern

### Creating TMC Errors

```go
// Basic error creation
tmcError := NewTMCError(TMCErrorTypeClusterUnreachable, "syncer", "sync-resource").
    WithMessage("Failed to connect to cluster").
    WithCluster("prod-cluster", "root:production").
    WithResource(deploymentGVK, "default", "my-app").
    WithSeverity(TMCErrorSeverityHigh).
    WithCause(originalError).
    Build()

// Resource-specific error
resourceError := NewTMCError(TMCErrorTypeResourceConflict, "placement-controller", "place-workload").
    WithMessage("Resource version conflict detected").
    WithResource(
        schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
        "production",
        "web-service",
    ).
    WithContext("resourceVersion", "12345").
    WithContext("conflictingVersion", "12344").
    WithRetryable(true).
    Build()

// Cluster connectivity error
clusterError := NewTMCError(TMCErrorTypeClusterUnreachable, "syncer", "heartbeat").
    WithMessage("Cluster endpoint is unreachable").
    WithCluster("edge-cluster-1", "root:edge").
    WithSeverity(TMCErrorSeverityHigh).
    WithRecoveryHint("Check cluster network connectivity and endpoint configuration").
    Build()
```

### Converting Kubernetes Errors

```go
// Automatic conversion from Kubernetes API errors
k8sError := errors.NewNotFound(schema.GroupResource{Group: "apps", Resource: "deployments"}, "my-app")
tmcError := ConvertKubernetesError(k8sError, "syncer", "get-resource")

// Results in TMCErrorTypeResourceNotFound with appropriate context
```

## Recovery Actions

### Automatic Recovery Suggestions

```go
// Get recovery actions for an error
actions := tmcError.GetRecoveryActions()

// Example actions for ClusterUnreachable:
// - "Check cluster connectivity"
// - "Verify cluster endpoint configuration" 
// - "Check network policies and firewall rules"
// - "Retry the operation"
```

### Error-Specific Recovery Strategies

#### Cluster Connectivity Issues
```go
if tmcError.Type == TMCErrorTypeClusterUnreachable {
    // Automatic recovery actions:
    // 1. Test cluster endpoint
    // 2. Refresh client connections
    // 3. Update cluster health status
    // 4. Retry with exponential backoff
}
```

#### Resource Conflicts
```go
if tmcError.Type == TMCErrorTypeResourceConflict {
    // Automatic recovery actions:
    // 1. Fetch latest resource version
    // 2. Apply three-way merge if possible
    // 3. Retry operation with updated resource
    // 4. Escalate to manual resolution if needed
}
```

## Retry Strategies

### Default Retry Configuration

```go
type RetryStrategy struct {
    MaxRetries      int           // 5
    InitialDelay    time.Duration // 1s
    MaxDelay        time.Duration // 30s
    BackoffFactor   float64       // 2.0
    RetryableErrors []TMCErrorType
}

// Default retryable errors
retryableErrors := []TMCErrorType{
    TMCErrorTypeClusterUnreachable,
    TMCErrorTypeClusterUnavailable,
    TMCErrorTypeSyncTimeout,
    TMCErrorTypeAggregationFailure,
    TMCErrorTypeProjectionFailure,
    TMCErrorTypeNetworkConnectivity,
    TMCErrorTypeInternal,
}
```

### Custom Retry Logic

```go
// Execute operation with retry
strategy := DefaultRetryStrategy()
err := ExecuteWithRetry(func() error {
    return syncResource(deployment)
}, strategy)

if err != nil {
    // Handle final failure after all retries exhausted
    log.Error(err, "Operation failed after retries")
}
```

### Per-Error-Type Retry Configuration

```go
// Custom retry for specific error types
customStrategy := &RetryStrategy{
    MaxRetries:    3,
    InitialDelay:  500 * time.Millisecond,
    MaxDelay:     10 * time.Second,
    BackoffFactor: 1.5,
    RetryableErrors: []TMCErrorType{
        TMCErrorTypeSyncFailure,
        TMCErrorTypeResourceConflict,
    },
}
```

## Circuit Breaker Pattern

### Protecting Against Cascading Failures

```go
// Create circuit breaker for cluster operations
clusterBreaker := NewCircuitBreaker("cluster-operations", 5, 60*time.Second)

// Execute operations through circuit breaker
err := clusterBreaker.Execute(func() error {
    return performClusterOperation()
})

if err != nil {
    // Circuit breaker may have prevented execution
    if strings.Contains(err.Error(), "circuit breaker") {
        log.Info("Circuit breaker is open, skipping operation")
    }
}
```

### Circuit Breaker States

- **Closed**: Normal operation, requests pass through
- **Open**: Failing fast, requests immediately return error
- **Half-Open**: Testing if service has recovered

### Configuration

```go
circuitBreaker := NewCircuitBreaker(
    "sync-operations",  // Name
    5,                  // Max failures before opening
    60*time.Second,     // Reset timeout
)
```

## Error Conditions Integration

### Converting to Kubernetes Conditions

```go
// Convert TMC error to Kubernetes condition
condition := tmcError.ToCondition("SyncReady")

// Example condition:
// Type: SyncReady
// Status: False
// Reason: ClusterUnreachable
// Message: Failed to connect to cluster: connection timeout
```

### Status Integration

```go
// Update resource status with error condition
syncTarget := &workloadv1alpha1.SyncTarget{}
errorCondition := tmcError.ToCondition("SyncerReady")

// Add to status conditions
conditionsv1alpha1.SetCondition(&syncTarget.Status.Conditions, errorCondition)
```

## Observability and Monitoring

### Error Metrics

```go
// Error metrics automatically tracked
tmc_errors_total{error_type="ClusterUnreachable", severity="High", component="syncer"}
tmc_error_recovery_attempts_total{error_type="ResourceConflict", strategy="retry"}
tmc_error_recovery_success_rate{error_type="SyncFailure"}
```

### Error Events

```go
// Kubernetes events generated for errors
Event{
    Type:    "Warning",
    Reason:  "SyncFailed", 
    Message: "Failed to sync Deployment default/my-app: ClusterUnreachable",
}
```

### Structured Logging

```go
// Structured error logging
logger.Error(tmcError, "TMC operation failed",
    "errorType", tmcError.Type,
    "severity", tmcError.Severity,
    "component", tmcError.Component,
    "cluster", tmcError.ClusterName,
    "resource", fmt.Sprintf("%s/%s", tmcError.Namespace, tmcError.Name),
)
```

## Integration Examples

### Syncer Integration

```go
// Error handling in syncer
func (rc *ResourceController) handleSyncError(err error, resource *unstructured.Unstructured) error {
    tmcError := ConvertKubernetesError(err, "syncer", "sync-resource")
    tmcError = tmcError.WithResource(resource.GroupVersionKind(), 
                                   resource.GetNamespace(), 
                                   resource.GetName())
    
    // Report to TMC error handling system
    rc.tmcErrorReporter.HandleError(tmcError)
    
    // Update resource condition
    if syncTarget := rc.getSyncTarget(); syncTarget != nil {
        condition := tmcError.ToCondition("SyncReady")
        conditionsv1alpha1.SetCondition(&syncTarget.Status.Conditions, condition)
    }
    
    return tmcError
}
```

### Placement Controller Integration

```go
// Error handling in placement controller
func (pc *PlacementController) handlePlacementError(err error, placement *workloadv1alpha1.Placement) {
    tmcError := NewTMCError(TMCErrorTypePlacementConstraint, "placement-controller", "schedule-workload").
        WithMessage("Failed to place workload: insufficient capacity").
        WithCluster(targetCluster, logicalCluster).
        WithCause(err).
        Build()
    
    // Update placement status
    condition := tmcError.ToCondition("PlacementReady")
    conditionsv1alpha1.SetCondition(&placement.Status.Conditions, condition)
    
    // Trigger recovery if retryable
    if tmcError.IsRetryable() {
        pc.requeuePlacement(placement, tmcError)
    }
}
```

## Best Practices

### Error Handling Guidelines

1. **Always Categorize**: Use appropriate TMCErrorType for proper handling
2. **Include Context**: Add relevant cluster, resource, and operation context
3. **Set Appropriate Severity**: Help operators prioritize response
4. **Provide Recovery Hints**: Include actionable guidance
5. **Preserve Original Errors**: Wrap rather than replace underlying errors

### Component Integration

1. **Use Error Builder**: Leverage the builder pattern for consistent error creation
2. **Convert K8s Errors**: Use ConvertKubernetesError for API errors
3. **Update Conditions**: Reflect errors in resource status conditions
4. **Report to Recovery**: Feed errors to recovery manager for automatic handling
5. **Log Structured**: Use structured logging with TMC error context

### Testing Error Scenarios

```go
// Test error handling
func TestErrorHandling(t *testing.T) {
    // Create test error
    tmcError := NewTMCError(TMCErrorTypeClusterUnreachable, "test", "operation").
        WithMessage("Test cluster unreachable").
        WithCluster("test-cluster", "root:test").
        Build()
    
    // Test retry logic
    strategy := DefaultRetryStrategy()
    shouldRetry := strategy.ShouldRetry(tmcError, 2)
    assert.True(t, shouldRetry)
    
    // Test recovery actions
    actions := tmcError.GetRecoveryActions()
    assert.Contains(t, actions, "Check cluster connectivity")
}
```

The TMC Error Handling System provides a robust foundation for reliable multi-cluster operations with intelligent error recovery and comprehensive observability.