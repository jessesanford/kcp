# KCP Workload Syncer PRD

## Executive Summary

The KCP Workload Syncer is a critical component of the Transparent Multi-Cluster (TMC) system that handles bidirectional synchronization of workload resources between KCP logical clusters and physical Kubernetes clusters. This PRD addresses the compilation issues identified during the TMC integration and provides a comprehensive specification for implementing a robust, production-ready syncer.

## Background

### Current State
During the TMC integration phase, the syncer component was identified as having multiple compilation issues that prevented successful build:

1. **Logging Interface Mismatches**: Usage of undefined `logging.WithValues` instead of proper klog patterns
2. **Type Compatibility Issues**: Mismatched types for condition status and API interfaces
3. **Client Generation Problems**: Missing method implementations and interface mismatches
4. **Import Dependencies**: Missing or incorrect package imports for metrics and tracing

### Relationship to TMC Architecture
The syncer is a foundational component that works in conjunction with the successfully integrated TMC components:

- **Virtual Workspace Manager**: The syncer populates the physical cluster data that virtual workspaces aggregate
- **TMC Metrics & Health System**: The syncer reports its health and metrics to the centralized TMC observability infrastructure
- **TMC Error Handling & Recovery**: The syncer leverages TMC's categorized error types and recovery strategies
- **Placement Controller**: The syncer executes placement decisions made by the placement controller
- **Location Controller**: The syncer registers cluster capabilities and status with location resources

## Goals

### Primary Goals
1. **Fix Compilation Issues**: Resolve all compilation errors preventing syncer from building
2. **Integrate with TMC Infrastructure**: Ensure syncer properly uses TMC's error handling, metrics, health, and recovery systems
3. **Production Readiness**: Implement robust synchronization with proper error handling, retries, and observability
4. **Workspace Awareness**: Enable proper multi-workspace synchronization with authentication and authorization

### Secondary Goals
1. **Performance Optimization**: Efficient resource synchronization with minimal overhead
2. **Extensibility**: Pluggable architecture for custom resource transformation and filtering
3. **Testing Coverage**: Comprehensive unit and integration tests

## Architecture

### High-Level Design

The syncer consists of four main components:

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Syncer Main   │────│  Engine         │────│ Resource        │
│   (cmd/main.go) │    │  (engine.go)    │    │ Controllers     │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         │               ┌─────────────────┐    ┌─────────────────┐
         └───────────────│ Status Reporter │────│ Health Monitor  │
                         │ (status_*.go)   │    │ (health.go)     │
                         └─────────────────┘    └─────────────────┘
```

### Component Breakdown

#### 1. Syncer Main (`cmd/workload-syncer/main.go`)
- **Purpose**: CLI entrypoint and lifecycle management
- **Current Status**: ✅ Compiles successfully
- **Dependencies**: Options, Engine

#### 2. Engine (`pkg/reconciler/workload/syncer/engine.go`)
- **Purpose**: Core synchronization orchestration
- **Current Issues**: 
  - `logging.WithValues` usage (line 309, 341)
  - Incorrect `WaitForCacheSync` usage (line 105)
- **Key Responsibilities**:
  - Manage resource controller lifecycle
  - Handle workspace discovery and filtering
  - Coordinate with TMC health and metrics systems

#### 3. Resource Controllers (`pkg/reconciler/workload/syncer/resource_controller.go`)
- **Purpose**: Per-resource-type synchronization logic
- **Current Issues**:
  - Multiple `logging.WithValues` usage (lines 140, 189, 209, 234, 267, 293)
  - Unused logger variables
- **Key Responsibilities**:
  - Bidirectional resource synchronization
  - Conflict resolution and transformation
  - Error categorization using TMC error types

#### 4. Status Reporter (`pkg/reconciler/workload/syncer/status_reporter.go`)
- **Purpose**: SyncTarget status management and heartbeats
- **Current Issues**:
  - Type mismatches with condition constants (lines 122, 131, 140)
  - Incorrect condition type usage (line 148-150)
- **Key Responsibilities**:
  - Report syncer health to KCP
  - Update SyncTarget status and conditions
  - Handle connection state management

#### 5. Health Monitor (`pkg/reconciler/workload/syncer/health.go`)
- **Purpose**: Local health checking and diagnostics
- **Current Status**: Needs integration with TMC health system
- **Key Responsibilities**:
  - Component health aggregation
  - Integration with TMC health monitoring
  - Readiness and liveness probe endpoints

#### 6. Metrics (`pkg/reconciler/workload/syncer/metrics.go`)
- **Purpose**: Performance and operational metrics
- **Current Issues**:
  - Missing prometheus dto import (line 237)
  - Incorrect MetricFamily usage
- **Key Responsibilities**:
  - Resource sync metrics
  - Performance tracking
  - Integration with TMC metrics system

## Detailed Specifications

### Error Handling Integration

The syncer must integrate with TMC's error handling system (`pkg/reconciler/workload/tmc/errors.go`):

```go
// Use TMC error types for categorization
func (rc *ResourceController) handleSyncError(err error, resource *unstructured.Unstructured) error {
    switch {
    case isNetworkError(err):
        return tmc.NewTMCError(tmc.TMCErrorTypeClusterUnreachable, err, 
            "Failed to reach target cluster", 
            tmc.TMCErrorSeverityHigh)
    case isConflictError(err):
        return tmc.NewTMCError(tmc.TMCErrorTypeResourceConflict, err,
            "Resource conflict during sync",
            tmc.TMCErrorSeverityMedium)
    default:
        return tmc.NewTMCError(tmc.TMCErrorTypeUnknown, err,
            "Unknown sync error",
            tmc.TMCErrorSeverityLow)
    }
}
```

### Metrics Integration

Integrate with TMC metrics system (`pkg/reconciler/workload/tmc/metrics.go`):

```go
// Register syncer metrics with TMC metrics collector
func (s *Syncer) initializeMetrics() error {
    s.tmcMetrics = tmc.NewTMCMetrics("syncer")
    
    // Register syncer-specific metrics
    s.tmcMetrics.RegisterCounterVec("syncer_resources_synced_total",
        "Total number of resources synced",
        []string{"cluster", "workspace", "gvk", "direction"})
        
    s.tmcMetrics.RegisterHistogramVec("syncer_sync_duration_seconds",
        "Time taken to sync resources",
        []string{"cluster", "workspace", "gvk"})
        
    return nil
}
```

### Health Integration

Use TMC health monitoring (`pkg/reconciler/workload/tmc/health.go`):

```go
// Register syncer components with TMC health system
func (s *Syncer) registerHealthChecks() error {
    s.healthManager = tmc.NewComponentHealthManager("syncer")
    
    // Register component health checks
    s.healthManager.RegisterComponent("kcp-connection", s.checkKCPConnection)
    s.healthManager.RegisterComponent("cluster-connection", s.checkClusterConnection)
    s.healthManager.RegisterComponent("resource-controllers", s.checkResourceControllers)
    
    return nil
}
```

### API Type Integration

Ensure proper usage of KCP workload API types:

```go
import (
    workloadv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/workload/v1alpha1"
    conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Correct condition type usage
func (sr *StatusReporter) updateSyncTargetConditions(syncTarget *workloadv1alpha1.SyncTarget) {
    now := metav1.NewTime(time.Now())
    
    // Use string conversion for condition types
    syncerReadyCondition := conditionsv1alpha1.Condition{
        Type:               string(workloadv1alpha1.SyncTargetSyncerReady),
        Status:             metav1.ConditionTrue,
        LastTransitionTime: now,
        Reason:             "SyncerConnected",
        Message:            "Syncer is connected and sending heartbeats",
    }
    
    // Use proper condition helper
    conditionsv1alpha1.SetCondition(&syncTarget.Status.Conditions, syncerReadyCondition)
}
```

## Critical Debugging Focus Areas

### 1. Logging Interface Fixes

**Problem**: Multiple instances of undefined `logging.WithValues` usage.

**Solution**: Replace with proper klog context patterns:

```go
// WRONG (causes compilation error):
logger := logging.WithValues(klog.FromContext(ctx), "key", value)

// CORRECT:
logger := klog.FromContext(ctx).WithValues("key", value)
```

**Files to Fix**:
- `pkg/reconciler/workload/syncer/engine.go` (lines 309, 341)
- `pkg/reconciler/workload/syncer/resource_controller.go` (lines 140, 189, 209, 234, 267, 293)

### 2. Type System Issues

**Problem**: Mismatched condition types and API interfaces.

**Solution**:
```go
// Import correct types
import (
    conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Use proper type conversion
condition := conditionsv1alpha1.Condition{
    Type:   string(workloadv1alpha1.SyncTargetReady), // Convert to string
    Status: metav1.ConditionTrue,                     // Use metav1 type
}
```

### 3. Client Generation Issues

**Problem**: Missing methods and interface incompatibilities.

**Solution**: Ensure proper client generation by running:
```bash
make codegen
```

**Files to Verify**:
- All generated client files in `sdk/client/`
- Deep copy methods in `zz_generated.deepcopy.go` files

### 4. Import Dependencies

**Problem**: Missing package imports for prometheus and other dependencies.

**Solution**:
```go
import (
    "github.com/prometheus/client_golang/prometheus"
    dto "github.com/prometheus/client_model/go"  // Add missing import
)

func (ms *MetricsServer) getGaugeValue(gauge prometheus.Gauge) float64 {
    metric := &dto.MetricFamily{}  // Use dto.MetricFamily
    // ... rest of implementation
}
```

## Testing Strategy

### Unit Tests
1. **Resource Controller Tests**: Test sync logic for various resource types
2. **Status Reporter Tests**: Verify condition management and heartbeat logic
3. **Engine Tests**: Test controller lifecycle and workspace management
4. **Error Handling Tests**: Verify TMC error integration

### Integration Tests
1. **End-to-End Sync Tests**: Full KCP → Cluster → KCP sync cycles
2. **Multi-Workspace Tests**: Verify workspace isolation and authentication
3. **Failure Recovery Tests**: Network partitions, API server restarts
4. **Performance Tests**: Resource sync throughput and latency

### Test Isolation Strategy

Create focused test environments:

```bash
# Test individual components
go test ./pkg/reconciler/workload/syncer/... -v -run TestResourceController

# Test TMC integration
go test ./pkg/reconciler/workload/syncer/... -v -run TestTMCIntegration

# Test compilation only
go build ./cmd/workload-syncer/...
```

## Implementation Phases

### Phase 1: Fix Compilation Issues (Priority: Critical)
1. Fix all logging.WithValues usage
2. Correct type mismatches in status reporter
3. Add missing imports for metrics
4. Ensure proper code generation

**Acceptance Criteria**: `make build` succeeds without errors

### Phase 2: TMC Integration (Priority: High)
1. Integrate with TMC error handling system
2. Connect to TMC metrics and health monitoring
3. Use TMC recovery strategies for failure scenarios
4. Update configuration to use TMC config patterns

**Acceptance Criteria**: Syncer properly reports to TMC infrastructure

### Phase 3: Production Hardening (Priority: Medium)
1. Implement comprehensive error recovery
2. Add performance optimizations
3. Enhance observability and diagnostics
4. Complete test coverage

**Acceptance Criteria**: Ready for production deployment

## Success Metrics

### Build Health
- ✅ Compilation succeeds without errors
- ✅ All unit tests pass
- ✅ Integration tests pass with TMC components

### Runtime Health
- ✅ Successful resource synchronization rates > 99%
- ✅ Heartbeat reliability > 99.9%
- ✅ Recovery time from failures < 30 seconds
- ✅ Memory usage stable over 24+ hours

### Integration Health
- ✅ TMC metrics properly collected and exported
- ✅ TMC health status accurately reflects syncer state
- ✅ TMC error categorization working correctly
- ✅ Virtual workspace integration functional

## Risk Mitigation

### High Risks
1. **API Compatibility**: Extensive testing with multiple KCP and Kubernetes versions
2. **Resource Conflicts**: Implement robust conflict resolution with TMC error handling
3. **Performance Impact**: Continuous monitoring and optimization of sync operations

### Medium Risks
1. **Authentication Changes**: Flexible auth configuration and proper error handling
2. **Network Partitions**: Robust reconnection logic using TMC recovery patterns
3. **Resource Transformation**: Comprehensive validation and rollback capabilities

## Dependencies

### Internal Dependencies
- ✅ TMC Error Handling System (`pkg/reconciler/workload/tmc/errors.go`)
- ✅ TMC Metrics System (`pkg/reconciler/workload/tmc/metrics.go`)
- ✅ TMC Health System (`pkg/reconciler/workload/tmc/health.go`)
- ✅ TMC Recovery System (`pkg/reconciler/workload/tmc/recovery.go`)
- ✅ Workload API Types (`sdk/apis/workload/v1alpha1/`)

### External Dependencies
- Kubernetes client-go (existing)
- Prometheus metrics (existing)
- KCP logicalcluster libraries (existing)

## Appendix

### File Structure
```
cmd/workload-syncer/
├── main.go                    # ✅ CLI entrypoint
└── options/
    └── options.go            # ✅ Configuration options

pkg/reconciler/workload/syncer/
├── engine.go                 # ❌ Logging issues
├── resource_controller.go    # ❌ Logging and type issues  
├── status_reporter.go        # ❌ Type mismatches
├── health.go                 # ⚠️  Needs TMC integration
├── metrics.go               # ❌ Import issues
├── syncer.go                # ⚠️  Needs review
└── syncer_test.go           # ⚠️  Needs implementation
```

### Key Compilation Errors Summary
1. **`logging.WithValues` undefined** - Replace with `klog.FromContext(ctx).WithValues()`
2. **`prometheus.MetricFamily` undefined** - Add `dto "github.com/prometheus/client_model/go"` import
3. **Condition type mismatches** - Use proper string conversion and metav1 types
4. **Unused logger variables** - Add log statements or remove declarations
5. **Client interface mismatches** - Ensure proper code generation

This PRD provides the complete specification needed to fix the syncer compilation issues and integrate it properly with the TMC infrastructure. The focus should be on Phase 1 (compilation fixes) first, followed by proper TMC integration in Phase 2.