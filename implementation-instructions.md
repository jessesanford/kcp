# Implementation Instructions: PR2 - Controller Interfaces

## PR Overview

**Purpose**: Define controller interface contracts without implementation
**Target Line Count**: 400 lines (excluding generated code)
**Dependencies**: None (merges to main independently)
**Feature Flag**: `TMCFeatureGate` (uses same master flag)

## Files to Create

### 1. pkg/tmc/controller/interfaces.go (150 lines)
```
Core controller interfaces for TMC
Expected interfaces:
- Controller (30 lines)
  - Start(ctx context.Context) error
  - Stop() error
  - Name() string
  - Ready() bool
- Reconciler (40 lines)
  - Reconcile(ctx context.Context, key string) error
  - SetupWithManager(mgr Manager) error
  - GetLogger() logr.Logger
- WorkQueue (30 lines)
  - Add(item interface{})
  - Get() (interface{}, bool)
  - Done(item interface{})
  - Forget(item interface{})
- Manager (50 lines)
  - AddController(ctrl Controller) error
  - GetClient() client.Client
  - GetScheme() *runtime.Scheme
  - GetEventRecorder() record.EventRecorder
```

### 2. pkg/tmc/controller/reconciler.go (100 lines)
```
Reconciler contracts and patterns
Expected content:
- ReconcilerFactory interface (30 lines)
  - NewReconciler(mgr Manager) (Reconciler, error)
- ReconcileResult interface (20 lines)
  - ShouldRequeue() bool
  - RequeueAfter() time.Duration
  - Error() error
- ReconcileContext interface (30 lines)
  - GetWorkspace() string
  - GetClusterName() string
  - GetNamespace() string
- EventRecorder interface (20 lines)
  - Event(object runtime.Object, eventType, reason, message string)
  - Eventf(object runtime.Object, eventType, reason, format string, args ...interface{})
```

### 3. pkg/tmc/controller/lifecycle.go (80 lines)
```
Controller lifecycle interfaces
Expected interfaces:
- Lifecycle (30 lines)
  - PreStart(ctx context.Context) error
  - PostStart(ctx context.Context) error
  - PreStop(ctx context.Context) error
  - PostStop(ctx context.Context) error
- HealthChecker (25 lines)
  - Check(ctx context.Context) error
  - Ready() bool
  - Live() bool
- LeaderElection (25 lines)
  - IsLeader() bool
  - BecomeLeader(ctx context.Context) error
  - ResignLeader(ctx context.Context) error
```

### 4. pkg/tmc/controller/metrics.go (70 lines)
```
Metrics collection interfaces
Expected interfaces:
- MetricsCollector (35 lines)
  - RecordReconcile(duration time.Duration, result string)
  - RecordError(errorType string)
  - RecordQueueDepth(depth int)
  - GetMetrics() map[string]interface{}
- MetricsExporter (35 lines)
  - Export(ctx context.Context) error
  - RegisterCollector(collector MetricsCollector)
  - GetEndpoint() string
```

## Extraction Instructions

### From Legacy PR3 (03a-pr3-controller-base)

1. **Extract controller interfaces from base.go**:
```bash
# Examine the base controller implementation
cat /workspaces/kcp-worktrees/legacy/phase3-original/03a-pr3-controller-base/pkg/controller/base.go

# Extract patterns for:
# - Controller initialization patterns
# - Reconciler patterns
# - WorkQueue usage patterns
# - Event recording patterns

# DO NOT copy implementation, only extract interface signatures
```

2. **Extract test patterns from base_test.go**:
```bash
# Look at test structure for interface patterns
cat /workspaces/kcp-worktrees/legacy/phase3-original/03a-pr3-controller-base/pkg/controller/base_test.go

# Extract:
# - Mock patterns that suggest interfaces
# - Test helper interfaces
```

### From Legacy PR4 (03a-pr4-tmc-controller)

1. **Extract TMC-specific interfaces from metrics.go**:
```bash
# Examine metrics patterns
cat /workspaces/kcp-worktrees/legacy/phase3-original/03a-pr4-tmc-controller/pkg/tmc/controller/metrics.go

# Extract:
# - Metrics collection interfaces
# - Metrics export patterns
# DO NOT copy implementation
```

### What to Exclude

- ❌ **NO implementation code**
- ❌ **NO concrete types**
- ❌ **NO controller registration code**
- ❌ **NO actual metrics implementation**
- ❌ **NO KCP-specific controller logic**
- ❌ **NO workqueue implementations**

## Implementation Details

### Interface Design Principles

1. **Clean Separation of Concerns**:
```go
// Controller lifecycle is separate from reconciliation
type Controller interface {
    Start(ctx context.Context) error
    Stop() error
}

type Reconciler interface {
    Reconcile(ctx context.Context, key string) error
}
```

2. **Factory Pattern for Extensibility**:
```go
// Factories allow different implementations
type ReconcilerFactory interface {
    // NewReconciler creates a reconciler for the given manager
    // Different implementations can be provided for testing vs production
    NewReconciler(mgr Manager) (Reconciler, error)
}
```

3. **Context-Aware Interfaces**:
```go
// All operations should be context-aware for cancellation
type HealthChecker interface {
    Check(ctx context.Context) error
}
```

### KCP Integration Points

```go
// Workspace-aware interfaces for KCP
type ReconcileContext interface {
    // GetWorkspace returns the logical cluster name
    GetWorkspace() string
    
    // GetClusterName returns the physical cluster name
    GetClusterName() string
}
```

## Testing Requirements

### Test Files to Create

1. **pkg/tmc/controller/interfaces_test.go** (50 lines):
```go
// Test that interfaces can be mocked
type mockController struct{}

func (m *mockController) Start(ctx context.Context) error { return nil }
func (m *mockController) Stop() error { return nil }
func (m *mockController) Name() string { return "mock" }
func (m *mockController) Ready() bool { return true }

func TestControllerInterface(t *testing.T) {
    var _ Controller = &mockController{}
}
```

2. **pkg/tmc/controller/reconciler_test.go** (50 lines):
```go
// Test reconciler patterns
type mockReconciler struct{}

func TestReconcilerFactory(t *testing.T) {
    // Test factory pattern
}
```

## Verification Checklist

- [ ] All files created in pkg/tmc/controller/
- [ ] Only interfaces, no implementations
- [ ] All interfaces are context-aware where appropriate
- [ ] Factory patterns for extensibility
- [ ] Workspace/cluster awareness for KCP
- [ ] Tests compile and pass
- [ ] Line count under 400 (excluding generated):
  ```bash
  /workspaces/kcp-shared-tools/tmc-pr-line-counter.sh -c feature/tmc2-impl2/pr2-controller-interfaces
  ```
- [ ] Code compiles:
  ```bash
  go build ./pkg/tmc/controller/...
  go test ./pkg/tmc/controller/...
  ```

## Code Examples

### Example Controller Interface

```go
package controller

import (
    "context"
    "time"
    
    "k8s.io/apimachinery/pkg/runtime"
    "k8s.io/client-go/tools/record"
    "sigs.k8s.io/controller-runtime/pkg/client"
    "github.com/go-logr/logr"
)

// Controller defines the base controller interface for TMC.
// All TMC controllers MUST implement this interface.
type Controller interface {
    // Start begins the controller's reconciliation loop.
    // It MUST be non-blocking and return quickly.
    Start(ctx context.Context) error
    
    // Stop gracefully shuts down the controller.
    // It MUST wait for in-flight reconciliations to complete.
    Stop() error
    
    // Name returns the controller's unique identifier.
    Name() string
    
    // Ready indicates if the controller is ready to process items.
    Ready() bool
}
```

### Example Reconciler Interface

```go
// Reconciler processes individual work items.
// Implementations MUST be idempotent and reentrant.
type Reconciler interface {
    // Reconcile processes a single work item identified by key.
    // The key format is "namespace/name" or just "name" for cluster-scoped resources.
    // Returns an error if the item should be requeued.
    Reconcile(ctx context.Context, key string) error
    
    // SetupWithManager configures the reconciler with the controller manager.
    SetupWithManager(mgr Manager) error
    
    // GetLogger returns the reconciler's logger for structured logging.
    GetLogger() logr.Logger
}
```

### Example Metrics Interface

```go
// MetricsCollector collects controller metrics.
// Implementations MUST be thread-safe.
type MetricsCollector interface {
    // RecordReconcile records the duration and result of a reconciliation.
    RecordReconcile(duration time.Duration, result string)
    
    // RecordError increments the error counter for the given error type.
    RecordError(errorType string)
    
    // RecordQueueDepth records the current depth of the work queue.
    RecordQueueDepth(depth int)
    
    // GetMetrics returns a snapshot of current metrics.
    GetMetrics() map[string]interface{}
}
```

### Example Factory Pattern

```go
// ReconcilerFactory creates reconcilers for different resource types.
type ReconcilerFactory interface {
    // NewReconciler creates a new reconciler instance.
    // The factory MUST configure the reconciler with appropriate:
    // - Client for API access
    // - Logger for structured logging
    // - Event recorder for Kubernetes events
    NewReconciler(mgr Manager) (Reconciler, error)
}

// Manager provides shared dependencies to controllers.
type Manager interface {
    // GetClient returns the Kubernetes client.
    GetClient() client.Client
    
    // GetScheme returns the runtime scheme.
    GetScheme() *runtime.Scheme
    
    // GetEventRecorder returns the event recorder.
    GetEventRecorder() record.EventRecorder
    
    // AddController registers a new controller.
    AddController(ctrl Controller) error
}
```

## Notes

- This PR enables parallel development of controller implementations
- Interfaces are designed to be testable with mocks
- Context-aware for proper cancellation and timeout handling
- Factory patterns allow different implementations for different environments
- Metrics interfaces enable observability without coupling to specific backends