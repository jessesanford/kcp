# Code Review Feedback Instructions

## Review Summary
- Branch: feature/tmc-phase4-sync-01-interfaces
- Lines of Code: 662
- Overall Status: NEEDS_FIXES
- Critical Issues: 7
- Non-Critical Issues: 5

## Critical Issues (Must Fix)

### 1. Missing Test Files
- **Severity**: CRITICAL
- **File**: pkg/syncer/interfaces/
- **Problem**: No test files exist for any of the interfaces. This violates KCP's testing standards.
- **Fix Instructions**: Create the following test files with complete implementations:

#### Create pkg/syncer/interfaces/interfaces_test.go
```go
/*
Copyright 2024 The KCP Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package interfaces_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	
	"github.com/kcp-dev/logicalcluster/v3"
	
	"github.com/kcp-dev/kcp/pkg/syncer/interfaces"
)

func TestSyncDirection(t *testing.T) {
	tests := []struct {
		name      string
		direction interfaces.SyncDirection
		expected  string
	}{
		{
			name:      "upstream direction",
			direction: interfaces.SyncDirectionUpstream,
			expected:  "upstream",
		},
		{
			name:      "downstream direction",
			direction: interfaces.SyncDirectionDownstream,
			expected:  "downstream",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.direction))
		})
	}
}

func TestSyncOperation(t *testing.T) {
	now := time.Now()
	op := interfaces.SyncOperation{
		ID:            "test-op-1",
		Direction:     interfaces.SyncDirectionDownstream,
		SourceCluster: logicalcluster.Name("root:org:ws"),
		TargetCluster: logicalcluster.Name("root:org:target"),
		GVR: schema.GroupVersionResource{
			Group:    "apps",
			Version:  "v1",
			Resource: "deployments",
		},
		Namespace: "default",
		Name:      "test-deployment",
		Priority:  10,
	}
	
	assert.Equal(t, "test-op-1", op.ID)
	assert.Equal(t, interfaces.SyncDirectionDownstream, op.Direction)
	assert.Equal(t, logicalcluster.Name("root:org:ws"), op.SourceCluster)
	assert.Equal(t, "default", op.Namespace)
	assert.Equal(t, int32(10), op.Priority)
}

func TestSyncStatus(t *testing.T) {
	retryDuration := 5 * time.Second
	status := interfaces.SyncStatus{
		Result:       interfaces.SyncResultRetry,
		Message:      "temporary failure",
		RetryAfter:   &retryDuration,
		ConflictType: interfaces.ConflictTypeResourceVersion,
	}
	
	assert.Equal(t, interfaces.SyncResultRetry, status.Result)
	assert.Equal(t, "temporary failure", status.Message)
	assert.NotNil(t, status.RetryAfter)
	assert.Equal(t, 5*time.Second, *status.RetryAfter)
}

func TestConflictTypes(t *testing.T) {
	conflicts := []interfaces.ConflictType{
		interfaces.ConflictTypeResourceVersion,
		interfaces.ConflictTypeOwnership,
		interfaces.ConflictTypeFieldManager,
		interfaces.ConflictTypeAnnotation,
	}
	
	expectedStrings := []string{
		"resource-version",
		"ownership",
		"field-manager",
		"annotation",
	}
	
	for i, conflict := range conflicts {
		assert.Equal(t, expectedStrings[i], string(conflict))
	}
}

func TestSyncMetrics(t *testing.T) {
	metrics := interfaces.SyncMetrics{
		TotalOperations:       100,
		SuccessfulOperations:  85,
		FailedOperations:      10,
		ConflictedOperations:  5,
		AverageProcessingTime: 2 * time.Second,
	}
	
	assert.Equal(t, int64(100), metrics.TotalOperations)
	assert.Equal(t, int64(85), metrics.SuccessfulOperations)
	assert.Equal(t, int64(10), metrics.FailedOperations)
	assert.Equal(t, int64(5), metrics.ConflictedOperations)
	assert.Equal(t, 2*time.Second, metrics.AverageProcessingTime)
	
	// Verify metrics consistency
	totalCalculated := metrics.SuccessfulOperations + metrics.FailedOperations + metrics.ConflictedOperations
	assert.Equal(t, metrics.TotalOperations, totalCalculated)
}

func TestTransformationContext(t *testing.T) {
	ctx := interfaces.TransformationContext{
		SourceWorkspace: logicalcluster.Name("root:org:source"),
		TargetWorkspace: logicalcluster.Name("root:org:target"),
		Direction:       interfaces.SyncDirectionDownstream,
		PlacementName:   "test-placement",
		SyncTargetName:  "cluster-1",
		Annotations: map[string]string{
			"tmc.kcp.io/placement": "test-placement",
			"tmc.kcp.io/sync-target": "cluster-1",
		},
	}
	
	assert.Equal(t, logicalcluster.Name("root:org:source"), ctx.SourceWorkspace)
	assert.Equal(t, logicalcluster.Name("root:org:target"), ctx.TargetWorkspace)
	assert.Equal(t, interfaces.SyncDirectionDownstream, ctx.Direction)
	assert.Equal(t, "test-placement", ctx.PlacementName)
	assert.Equal(t, "cluster-1", ctx.SyncTargetName)
	assert.Contains(t, ctx.Annotations, "tmc.kcp.io/placement")
}

func TestSyncConflict(t *testing.T) {
	source := &unstructured.Unstructured{}
	source.SetName("test-resource")
	source.SetNamespace("default")
	
	target := &unstructured.Unstructured{}
	target.SetName("test-resource")
	target.SetNamespace("default")
	target.SetResourceVersion("123")
	
	conflict := interfaces.SyncConflict{
		Operation: interfaces.SyncOperation{
			ID:        "op-1",
			Direction: interfaces.SyncDirectionDownstream,
		},
		ConflictType:   interfaces.ConflictTypeResourceVersion,
		SourceResource: source,
		TargetResource: target,
		ConflictDetails: map[string]interface{}{
			"sourceVersion": "",
			"targetVersion": "123",
		},
		DetectedAt: time.Now(),
	}
	
	assert.Equal(t, interfaces.ConflictTypeResourceVersion, conflict.ConflictType)
	assert.NotNil(t, conflict.SourceResource)
	assert.NotNil(t, conflict.TargetResource)
	assert.Contains(t, conflict.ConflictDetails, "targetVersion")
}

func TestConflictResolution(t *testing.T) {
	retryDuration := 10 * time.Second
	resolution := interfaces.ConflictResolution{
		Resolved:   true,
		Resolution: &unstructured.Unstructured{},
		Strategy:   "server-side-apply",
		Message:    "Resolved using server-side apply",
		Retry:      false,
		RetryAfter: &retryDuration,
	}
	
	assert.True(t, resolution.Resolved)
	assert.NotNil(t, resolution.Resolution)
	assert.Equal(t, "server-side-apply", resolution.Strategy)
	assert.Equal(t, "Resolved using server-side apply", resolution.Message)
	assert.False(t, resolution.Retry)
	assert.Equal(t, 10*time.Second, *resolution.RetryAfter)
}

func TestSyncEngineConfig(t *testing.T) {
	config := interfaces.SyncEngineConfig{
		WorkerCount: 10,
		QueueDepth:  100,
		Workspace:   logicalcluster.Name("root:org:ws"),
		SupportedGVRs: []schema.GroupVersionResource{
			{Group: "apps", Version: "v1", Resource: "deployments"},
			{Group: "", Version: "v1", Resource: "services"},
		},
	}
	
	assert.Equal(t, 10, config.WorkerCount)
	assert.Equal(t, 100, config.QueueDepth)
	assert.Equal(t, logicalcluster.Name("root:org:ws"), config.Workspace)
	assert.Len(t, config.SupportedGVRs, 2)
	assert.Equal(t, "deployments", config.SupportedGVRs[0].Resource)
}
```

- **Verification**: Run `go test ./pkg/syncer/interfaces/...` and ensure all tests pass

### 2. Missing Mock Implementations
- **Severity**: CRITICAL
- **File**: pkg/syncer/interfaces/mocks/
- **Problem**: No mock implementations for testing. Required for unit testing controllers.
- **Fix Instructions**: Create mock directory and implementations:

#### Create pkg/syncer/interfaces/mocks/doc.go
```go
/*
Copyright 2024 The KCP Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package mocks provides mock implementations of the syncer interfaces for testing.
// These mocks are used throughout the TMC implementation to facilitate unit testing
// of controllers and other components that depend on the syncer interfaces.
package mocks
```

#### Create pkg/syncer/interfaces/mocks/sync_engine.go
```go
/*
Copyright 2024 The KCP Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package mocks

import (
	"context"
	"sync"

	"github.com/kcp-dev/logicalcluster/v3"
	
	"github.com/kcp-dev/kcp/pkg/syncer/interfaces"
)

// MockSyncEngine provides a mock implementation of interfaces.SyncEngine for testing.
type MockSyncEngine struct {
	mu sync.RWMutex
	
	// Function hooks for testing
	StartFunc                func(ctx context.Context) error
	StopFunc                 func(ctx context.Context) error
	EnqueueSyncOperationFunc func(operation interfaces.SyncOperation) error
	ProcessSyncOperationFunc func(ctx context.Context, operation interfaces.SyncOperation) interfaces.SyncStatus
	GetSyncStatusFunc        func(operationID string) (interfaces.SyncStatus, bool)
	ListPendingOperationsFunc func(workspace logicalcluster.Name, direction interfaces.SyncDirection) []interfaces.SyncOperation
	GetMetricsFunc           func() interfaces.SyncMetrics
	IsHealthyFunc            func() error
	
	// State tracking for verification
	StartCalled     bool
	StopCalled      bool
	EnqueuedOps     []interfaces.SyncOperation
	ProcessedOps    []interfaces.SyncOperation
	StatusQueries   []string
}

// NewMockSyncEngine creates a new mock sync engine with default implementations.
func NewMockSyncEngine() *MockSyncEngine {
	return &MockSyncEngine{
		EnqueuedOps:   make([]interfaces.SyncOperation, 0),
		ProcessedOps:  make([]interfaces.SyncOperation, 0),
		StatusQueries: make([]string, 0),
	}
}

func (m *MockSyncEngine) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.StartCalled = true
	if m.StartFunc != nil {
		return m.StartFunc(ctx)
	}
	return nil
}

func (m *MockSyncEngine) Stop(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.StopCalled = true
	if m.StopFunc != nil {
		return m.StopFunc(ctx)
	}
	return nil
}

func (m *MockSyncEngine) EnqueueSyncOperation(operation interfaces.SyncOperation) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.EnqueuedOps = append(m.EnqueuedOps, operation)
	if m.EnqueueSyncOperationFunc != nil {
		return m.EnqueueSyncOperationFunc(operation)
	}
	return nil
}

func (m *MockSyncEngine) ProcessSyncOperation(ctx context.Context, operation interfaces.SyncOperation) interfaces.SyncStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.ProcessedOps = append(m.ProcessedOps, operation)
	if m.ProcessSyncOperationFunc != nil {
		return m.ProcessSyncOperationFunc(ctx, operation)
	}
	return interfaces.SyncStatus{
		Operation: operation,
		Result:    interfaces.SyncResultSuccess,
		Message:   "mock success",
	}
}

func (m *MockSyncEngine) GetSyncStatus(operationID string) (interfaces.SyncStatus, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.StatusQueries = append(m.StatusQueries, operationID)
	if m.GetSyncStatusFunc != nil {
		return m.GetSyncStatusFunc(operationID)
	}
	return interfaces.SyncStatus{}, false
}

func (m *MockSyncEngine) ListPendingOperations(workspace logicalcluster.Name, direction interfaces.SyncDirection) []interfaces.SyncOperation {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if m.ListPendingOperationsFunc != nil {
		return m.ListPendingOperationsFunc(workspace, direction)
	}
	return []interfaces.SyncOperation{}
}

func (m *MockSyncEngine) GetMetrics() interfaces.SyncMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if m.GetMetricsFunc != nil {
		return m.GetMetricsFunc()
	}
	return interfaces.SyncMetrics{}
}

func (m *MockSyncEngine) IsHealthy() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if m.IsHealthyFunc != nil {
		return m.IsHealthyFunc()
	}
	return nil
}

// Reset clears all recorded state for reuse in tests.
func (m *MockSyncEngine) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.StartCalled = false
	m.StopCalled = false
	m.EnqueuedOps = make([]interfaces.SyncOperation, 0)
	m.ProcessedOps = make([]interfaces.SyncOperation, 0)
	m.StatusQueries = make([]string, 0)
}
```

- **Verification**: Run `go build ./pkg/syncer/interfaces/mocks/...` to ensure compilation

### 3. Missing KCP Integration Types
- **Severity**: CRITICAL
- **File**: pkg/syncer/interfaces/types.go
- **Problem**: Missing critical KCP annotations and constants used throughout the syncer
- **Fix Instructions**: Add the following to types.go after line 125:

```go
// Common annotations used by the syncer for resource tracking and management.
const (
	// SyncSourceAnnotation indicates the source cluster for a synced resource.
	SyncSourceAnnotation = "tmc.kcp.io/sync-source"
	
	// WorkspaceOriginAnnotation indicates the workspace a resource originated from.
	WorkspaceOriginAnnotation = "tmc.kcp.io/workspace-origin"
	
	// PlacementAnnotation indicates which placement policy selected this resource.
	PlacementAnnotation = "tmc.kcp.io/placement"
	
	// SyncTargetAnnotation indicates the target cluster for synchronization.
	SyncTargetAnnotation = "tmc.kcp.io/sync-target"
	
	// SyncGenerationAnnotation tracks the generation of the synced resource.
	SyncGenerationAnnotation = "tmc.kcp.io/sync-generation"
	
	// SyncTimestampAnnotation records when the resource was last synchronized.
	SyncTimestampAnnotation = "tmc.kcp.io/sync-timestamp"
	
	// ConflictResolutionAnnotation indicates how conflicts were resolved.
	ConflictResolutionAnnotation = "tmc.kcp.io/conflict-resolution"
)

// Common labels used by the syncer.
const (
	// SyncerManagedLabel indicates a resource is managed by the syncer.
	SyncerManagedLabel = "tmc.kcp.io/syncer-managed"
	
	// WorkspaceLabel indicates the workspace a resource belongs to.
	WorkspaceLabel = "tmc.kcp.io/workspace"
	
	// PlacementLabel indicates the placement that selected this resource.
	PlacementLabel = "tmc.kcp.io/placement"
)

// Feature gates for syncer functionality.
const (
	// FeatureGateTMCSyncer enables the TMC syncer functionality.
	FeatureGateTMCSyncer = "TMCSyncer"
	
	// FeatureGateAdvancedConflictResolution enables advanced conflict resolution strategies.
	FeatureGateAdvancedConflictResolution = "TMCAdvancedConflictResolution"
	
	// FeatureGateStatusAggregation enables status aggregation across placements.
	FeatureGateStatusAggregation = "TMCStatusAggregation"
)
```

- **Verification**: Compile with `go build ./pkg/syncer/interfaces/...`

### 4. Missing Package Documentation
- **Severity**: CRITICAL
- **File**: pkg/syncer/interfaces/doc.go
- **Problem**: No package documentation file exists
- **Fix Instructions**: Create pkg/syncer/interfaces/doc.go:

```go
/*
Copyright 2024 The KCP Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package interfaces defines the core interfaces for the TMC (Transparent Multi-Cluster)
// syncer component. These interfaces provide the contract for synchronizing resources
// between logical clusters in KCP and physical Kubernetes clusters.
//
// The syncer is responsible for:
//   - Bidirectional synchronization of resources between logical and physical clusters
//   - Transformation of resources to adapt them for their target environment
//   - Conflict detection and resolution during synchronization
//   - Status collection and aggregation across multiple placements
//   - Workspace-aware resource management
//
// Core Interfaces:
//
// SyncEngine - The main synchronization engine that orchestrates resource synchronization
// between clusters. It manages the sync queue, processes operations, and coordinates
// with other components.
//
// ResourceTransformer - Handles the transformation of resources as they move between
// logical and physical clusters. This includes adding/removing annotations, labels,
// and adapting resources for their target environment.
//
// ConflictResolver - Detects and resolves conflicts that occur during synchronization.
// Supports multiple resolution strategies including server-side apply, three-way merge,
// and custom resolution logic.
//
// StatusCollector - Collects and aggregates status information from sync operations
// across multiple clusters and workspaces. Provides metrics and health monitoring.
//
// Architecture:
//
// The syncer operates as a controller in both the KCP control plane and in physical
// clusters (via deployed syncer pods). It watches for resources that need to be
// synchronized based on placement decisions and manages their lifecycle across clusters.
//
//	┌─────────────────┐     ┌─────────────────┐
//	│  Logical        │     │  Physical       │
//	│  Cluster (KCP)  │◄───►│  Cluster        │
//	└─────────────────┘     └─────────────────┘
//	        │                        │
//	        ▼                        ▼
//	┌─────────────────┐     ┌─────────────────┐
//	│  Sync Engine    │     │  Sync Engine    │
//	│  (Downstream)   │     │  (Upstream)     │
//	└─────────────────┘     └─────────────────┘
//	        │                        │
//	        ▼                        ▼
//	┌─────────────────────────────────────────┐
//	│         Resource Transformer            │
//	│         Conflict Resolver                │
//	│         Status Collector                 │
//	└─────────────────────────────────────────┘
//
// Usage:
//
// Implementations of these interfaces are used by the TMC placement controller
// and the syncer controller to manage workload distribution across clusters:
//
//	engine := syncer.NewSyncEngine(config)
//	engine.Start(ctx)
//	defer engine.Stop(ctx)
//	
//	operation := interfaces.SyncOperation{
//	    Direction: interfaces.SyncDirectionDownstream,
//	    SourceCluster: logicalCluster,
//	    TargetCluster: physicalCluster,
//	    GVR: schema.GroupVersionResource{
//	        Group: "apps",
//	        Version: "v1",
//	        Resource: "deployments",
//	    },
//	    Namespace: "default",
//	    Name: "my-app",
//	}
//	
//	engine.EnqueueSyncOperation(operation)
//
package interfaces
```

- **Verification**: Run `go doc github.com/kcp-dev/kcp/pkg/syncer/interfaces`

### 5. Missing Error Types
- **Severity**: CRITICAL
- **File**: pkg/syncer/interfaces/errors.go
- **Problem**: No error types defined for syncer operations
- **Fix Instructions**: Create pkg/syncer/interfaces/errors.go:

```go
/*
Copyright 2024 The KCP Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package interfaces

import (
	"fmt"
	
	"k8s.io/apimachinery/pkg/runtime/schema"
	
	"github.com/kcp-dev/logicalcluster/v3"
)

// SyncError represents an error that occurred during synchronization.
type SyncError struct {
	Operation SyncOperation
	Cause     error
	Message   string
}

func (e *SyncError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("sync error for %s/%s in %s: %s: %v",
			e.Operation.Namespace, e.Operation.Name,
			e.Operation.SourceCluster, e.Message, e.Cause)
	}
	return fmt.Sprintf("sync error for %s/%s in %s: %s",
		e.Operation.Namespace, e.Operation.Name,
		e.Operation.SourceCluster, e.Message)
}

func (e *SyncError) Unwrap() error {
	return e.Cause
}

// TransformationError represents an error during resource transformation.
type TransformationError struct {
	GVR       schema.GroupVersionResource
	Namespace string
	Name      string
	Direction SyncDirection
	Workspace logicalcluster.Name
	Cause     error
}

func (e *TransformationError) Error() string {
	return fmt.Sprintf("transformation error for %s %s/%s (direction: %s, workspace: %s): %v",
		e.GVR.String(), e.Namespace, e.Name, e.Direction, e.Workspace, e.Cause)
}

func (e *TransformationError) Unwrap() error {
	return e.Cause
}

// ConflictError represents a conflict that could not be resolved.
type ConflictError struct {
	Conflict   SyncConflict
	Resolution *ConflictResolution
	Cause      error
}

func (e *ConflictError) Error() string {
	if e.Resolution != nil {
		return fmt.Sprintf("conflict resolution failed (type: %s, strategy: %s): %v",
			e.Conflict.ConflictType, e.Resolution.Strategy, e.Cause)
	}
	return fmt.Sprintf("conflict detected (type: %s): %v",
		e.Conflict.ConflictType, e.Cause)
}

func (e *ConflictError) Unwrap() error {
	return e.Cause
}

// Common error values
var (
	// ErrSyncEngineStopped indicates the sync engine has been stopped.
	ErrSyncEngineStopped = fmt.Errorf("sync engine stopped")
	
	// ErrQueueFull indicates the sync queue is full and cannot accept more operations.
	ErrQueueFull = fmt.Errorf("sync queue full")
	
	// ErrOperationNotFound indicates the requested operation was not found.
	ErrOperationNotFound = fmt.Errorf("operation not found")
	
	// ErrInvalidDirection indicates an invalid sync direction was specified.
	ErrInvalidDirection = fmt.Errorf("invalid sync direction")
	
	// ErrWorkspaceNotFound indicates the specified workspace does not exist.
	ErrWorkspaceNotFound = fmt.Errorf("workspace not found")
	
	// ErrTransformationFailed indicates resource transformation failed.
	ErrTransformationFailed = fmt.Errorf("transformation failed")
	
	// ErrConflictUnresolvable indicates a conflict could not be resolved.
	ErrConflictUnresolvable = fmt.Errorf("conflict unresolvable")
	
	// ErrInvalidResource indicates the resource is invalid for synchronization.
	ErrInvalidResource = fmt.Errorf("invalid resource")
)

// IsRetryableError determines if an error should trigger a retry.
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}
	
	// Check for specific non-retryable errors
	switch err {
	case ErrSyncEngineStopped, ErrInvalidDirection, ErrInvalidResource:
		return false
	}
	
	// Check for conflict errors - some may be retryable
	if conflictErr, ok := err.(*ConflictError); ok {
		return conflictErr.Conflict.ConflictType == ConflictTypeResourceVersion
	}
	
	// Default to retryable for unknown errors
	return true
}
```

- **Verification**: Run `go build ./pkg/syncer/interfaces/...`

### 6. Missing KCP Client Integration Interfaces
- **Severity**: CRITICAL  
- **File**: pkg/syncer/interfaces/clients.go
- **Problem**: Missing interfaces for KCP-specific client operations
- **Fix Instructions**: Create pkg/syncer/interfaces/clients.go:

```go
/*
Copyright 2024 The KCP Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package interfaces

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"

	"github.com/kcp-dev/logicalcluster/v3"
)

// DynamicClusterClient provides cluster-aware dynamic client operations.
type DynamicClusterClient interface {
	// Cluster returns a dynamic interface for the specified logical cluster.
	Cluster(cluster logicalcluster.Path) dynamic.Interface
	
	// Resource returns a namespaced resource client for the specified GVR and cluster.
	Resource(cluster logicalcluster.Path, gvr schema.GroupVersionResource) dynamic.NamespaceableResourceInterface
}

// ClusterAwareInformerFactory provides cluster-aware informer creation.
type ClusterAwareInformerFactory interface {
	// ForCluster returns an informer factory for the specified cluster.
	ForCluster(cluster logicalcluster.Path) cache.SharedIndexInformer
	
	// Start starts all informers managed by this factory.
	Start(stopCh <-chan struct{})
	
	// WaitForCacheSync waits for all started informers' caches to sync.
	WaitForCacheSync(stopCh <-chan struct{}) map[schema.GroupVersionResource]bool
}

// SyncEventRecorder provides event recording for sync operations.
type SyncEventRecorder interface {
	record.EventRecorder
	
	// RecordSyncEvent records a sync-specific event.
	RecordSyncEvent(object runtime.Object, operation SyncOperation, eventType, reason, message string)
}

// ResourceWatcher watches resources across clusters for sync operations.
type ResourceWatcher interface {
	// Watch starts watching resources in the specified cluster.
	Watch(ctx context.Context, cluster logicalcluster.Path, gvr schema.GroupVersionResource, handler ResourceEventHandler) error
	
	// StopWatch stops watching resources in the specified cluster.
	StopWatch(cluster logicalcluster.Path, gvr schema.GroupVersionResource)
	
	// IsWatching returns true if actively watching the specified resource.
	IsWatching(cluster logicalcluster.Path, gvr schema.GroupVersionResource) bool
}

// ResourceEventHandler handles resource events for synchronization.
type ResourceEventHandler interface {
	// OnAdd is called when a resource is added.
	OnAdd(obj *unstructured.Unstructured, cluster logicalcluster.Path)
	
	// OnUpdate is called when a resource is updated.
	OnUpdate(oldObj, newObj *unstructured.Unstructured, cluster logicalcluster.Path)
	
	// OnDelete is called when a resource is deleted.
	OnDelete(obj *unstructured.Unstructured, cluster logicalcluster.Path)
}

// ResourceAccessor provides unified access to resources across clusters.
type ResourceAccessor interface {
	// Get retrieves a resource from the specified cluster.
	Get(ctx context.Context, cluster logicalcluster.Path, gvr schema.GroupVersionResource, namespace, name string) (*unstructured.Unstructured, error)
	
	// List lists resources in the specified cluster.
	List(ctx context.Context, cluster logicalcluster.Path, gvr schema.GroupVersionResource, namespace string) (*unstructured.UnstructuredList, error)
	
	// Create creates a resource in the specified cluster.
	Create(ctx context.Context, cluster logicalcluster.Path, gvr schema.GroupVersionResource, obj *unstructured.Unstructured) (*unstructured.Unstructured, error)
	
	// Update updates a resource in the specified cluster.
	Update(ctx context.Context, cluster logicalcluster.Path, gvr schema.GroupVersionResource, obj *unstructured.Unstructured) (*unstructured.Unstructured, error)
	
	// Delete deletes a resource from the specified cluster.
	Delete(ctx context.Context, cluster logicalcluster.Path, gvr schema.GroupVersionResource, namespace, name string) error
	
	// Patch patches a resource in the specified cluster.
	Patch(ctx context.Context, cluster logicalcluster.Path, gvr schema.GroupVersionResource, namespace, name string, pt types.PatchType, data []byte) (*unstructured.Unstructured, error)
}
```

- **Verification**: Run `go build ./pkg/syncer/interfaces/...` and ensure imports are resolved

### 7. Missing Example Implementations
- **Severity**: CRITICAL
- **File**: pkg/syncer/interfaces/examples_test.go
- **Problem**: No example code showing how to use the interfaces
- **Fix Instructions**: Create pkg/syncer/interfaces/examples_test.go:

```go
/*
Copyright 2024 The KCP Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package interfaces_test

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/kcp-dev/logicalcluster/v3"

	"github.com/kcp-dev/kcp/pkg/syncer/interfaces"
)

// ExampleSyncOperation demonstrates creating a sync operation.
func ExampleSyncOperation() {
	op := interfaces.SyncOperation{
		ID:        "sync-deployment-12345",
		Direction: interfaces.SyncDirectionDownstream,
		SourceCluster: logicalcluster.Name("root:org:workspace"),
		TargetCluster: logicalcluster.Name("root:org:cluster-1"),
		GVR: schema.GroupVersionResource{
			Group:    "apps",
			Version:  "v1", 
			Resource: "deployments",
		},
		Namespace: "production",
		Name:      "web-app",
		Priority:  10,
		Timestamp: metav1.Now(),
	}
	
	fmt.Printf("Sync operation %s: %s/%s from %s to %s\n",
		op.ID, op.Namespace, op.Name, op.SourceCluster, op.TargetCluster)
	// Output: Sync operation sync-deployment-12345: production/web-app from root:org:workspace to root:org:cluster-1
}

// ExampleSyncEngine demonstrates using the sync engine interface.
func ExampleSyncEngine() {
	// This would normally be a real implementation
	var engine interfaces.SyncEngine
	
	ctx := context.Background()
	
	// Start the sync engine
	if err := engine.Start(ctx); err != nil {
		fmt.Printf("Failed to start engine: %v\n", err)
		return
	}
	
	// Enqueue a sync operation
	op := interfaces.SyncOperation{
		ID:        "op-1",
		Direction: interfaces.SyncDirectionDownstream,
		// ... other fields
	}
	
	if err := engine.EnqueueSyncOperation(op); err != nil {
		fmt.Printf("Failed to enqueue: %v\n", err)
		return
	}
	
	// Check status
	if status, found := engine.GetSyncStatus("op-1"); found {
		fmt.Printf("Operation status: %s\n", status.Result)
	}
	
	// Stop the engine
	if err := engine.Stop(ctx); err != nil {
		fmt.Printf("Failed to stop engine: %v\n", err)
	}
}

// ExampleResourceTransformer demonstrates resource transformation.
func ExampleResourceTransformer() {
	// This would normally be a real implementation
	var transformer interfaces.ResourceTransformer
	
	ctx := context.Background()
	resource := &unstructured.Unstructured{}
	resource.SetName("my-deployment")
	resource.SetNamespace("default")
	
	// Transform for downstream sync
	transformed, err := transformer.TransformForDownstream(
		ctx,
		logicalcluster.Name("root:org:workspace"),
		logicalcluster.Name("root:org:cluster-1"),
		resource,
	)
	
	if err != nil {
		fmt.Printf("Transformation failed: %v\n", err)
		return
	}
	
	// Check if transformation added required annotations
	annotations := transformed.GetAnnotations()
	if source, ok := annotations[interfaces.SyncSourceAnnotation]; ok {
		fmt.Printf("Sync source: %s\n", source)
	}
}

// ExampleConflictResolver demonstrates conflict resolution.
func ExampleConflictResolver() {
	// This would normally be a real implementation
	var resolver interfaces.ConflictResolver
	
	ctx := context.Background()
	
	conflict := interfaces.SyncConflict{
		Operation: interfaces.SyncOperation{
			ID:        "op-1",
			Direction: interfaces.SyncDirectionDownstream,
		},
		ConflictType: interfaces.ConflictTypeResourceVersion,
		SourceResource: &unstructured.Unstructured{},
		TargetResource: &unstructured.Unstructured{},
		DetectedAt: time.Now(),
	}
	
	// Attempt to resolve the conflict
	resolution, err := resolver.ResolveConflict(ctx, conflict)
	if err != nil {
		fmt.Printf("Failed to resolve conflict: %v\n", err)
		return
	}
	
	if resolution.Resolved {
		fmt.Printf("Conflict resolved using strategy: %s\n", resolution.Strategy)
	} else if resolution.Retry {
		fmt.Printf("Retry after %v\n", *resolution.RetryAfter)
	}
}

// ExampleStatusCollector demonstrates status collection.
func ExampleStatusCollector() {
	// This would normally be a real implementation
	var collector interfaces.StatusCollector
	
	ctx := context.Background()
	
	// Record a sync status
	status := interfaces.SyncStatus{
		Operation: interfaces.SyncOperation{
			ID: "op-1",
		},
		Result:         interfaces.SyncResultSuccess,
		Message:        "Resource synchronized successfully",
		ProcessingTime: 150 * time.Millisecond,
		Timestamp:      metav1.Now(),
	}
	
	if err := collector.RecordSyncStatus(ctx, status); err != nil {
		fmt.Printf("Failed to record status: %v\n", err)
		return
	}
	
	// Get metrics for a workspace
	workspace := logicalcluster.Name("root:org:workspace")
	since := time.Now().Add(-1 * time.Hour)
	metrics := collector.GetWorkspaceMetrics(workspace, &since)
	
	fmt.Printf("Workspace metrics: %d successful, %d failed\n",
		metrics.SuccessfulOperations, metrics.FailedOperations)
}

// ExampleTransformationContext demonstrates creating a transformation context.
func ExampleTransformationContext() {
	ctx := interfaces.TransformationContext{
		SourceWorkspace: logicalcluster.Name("root:org:source"),
		TargetWorkspace: logicalcluster.Name("root:org:target"),
		Direction:       interfaces.SyncDirectionDownstream,
		PlacementName:   "production-placement",
		SyncTargetName:  "us-west-cluster",
		Annotations: map[string]string{
			interfaces.PlacementAnnotation:  "production-placement",
			interfaces.SyncTargetAnnotation: "us-west-cluster",
			"tmc.kcp.io/region":             "us-west",
		},
	}
	
	fmt.Printf("Transforming from %s to %s for placement %s\n",
		ctx.SourceWorkspace, ctx.TargetWorkspace, ctx.PlacementName)
	// Output: Transforming from root:org:source to root:org:target for placement production-placement
}
```

- **Verification**: Run `go test ./pkg/syncer/interfaces/... -run Example`

## Non-Critical Issues

### 1. Import Ordering
- **Severity**: NON-CRITICAL
- **File**: All interface files
- **Problem**: Imports should follow KCP convention: stdlib, k8s.io, kcp-dev, local
- **Fix Instructions**: Reorder imports in all files to follow:
  1. Standard library
  2. k8s.io packages
  3. github.com/kcp-dev packages
  4. Local packages

### 2. Missing Validation Methods
- **Severity**: NON-CRITICAL
- **File**: pkg/syncer/interfaces/types.go
- **Problem**: No validation methods for types
- **Fix Instructions**: Add validation methods to types.go:

```go
// Validate validates the SyncOperation.
func (o *SyncOperation) Validate() error {
	if o.ID == "" {
		return fmt.Errorf("operation ID is required")
	}
	if o.Direction != SyncDirectionUpstream && o.Direction != SyncDirectionDownstream {
		return fmt.Errorf("invalid sync direction: %s", o.Direction)
	}
	if o.SourceCluster.Empty() {
		return fmt.Errorf("source cluster is required")
	}
	if o.TargetCluster.Empty() {
		return fmt.Errorf("target cluster is required")
	}
	if o.GVR.Resource == "" {
		return fmt.Errorf("resource is required in GVR")
	}
	if o.Name == "" {
		return fmt.Errorf("resource name is required")
	}
	return nil
}
```

### 3. Missing String Methods
- **Severity**: NON-CRITICAL
- **File**: pkg/syncer/interfaces/types.go
- **Problem**: Types lack String() methods for debugging
- **Fix Instructions**: Add String() methods for key types

### 4. Documentation Improvements
- **Severity**: NON-CRITICAL
- **File**: All interface files
- **Problem**: Some methods could use more detailed documentation
- **Fix Instructions**: Expand documentation with examples and edge cases

### 5. Thread Safety Documentation
- **Severity**: NON-CRITICAL
- **File**: All interface files
- **Problem**: No documentation about thread safety requirements
- **Fix Instructions**: Add thread safety requirements to interface documentation

## Test Coverage Report

Create the following test file structure:

```
pkg/syncer/interfaces/
├── interfaces_test.go (provided above)
├── types_test.go (create)
├── errors_test.go (create)
├── examples_test.go (provided above)
└── mocks/
    ├── doc.go (provided above)
    ├── sync_engine.go (provided above)
    ├── resource_transformer.go (create)
    ├── conflict_resolver.go (create)
    └── status_collector.go (create)
```

## KCP Pattern Compliance Checklist

✅ Copyright headers present
✅ Proper use of logicalcluster.Name
✅ Unstructured resources for flexibility
❌ Missing KCP-specific annotations
❌ Missing DynamicClusterClient interface
❌ Missing InformerFactory integration
❌ Missing EventRecorder integration
❌ Missing error types
❌ Missing mock implementations
❌ Missing test coverage
❌ Missing package documentation

## PR Readiness Assessment

**Current Status**: NOT READY FOR PR

**Required Actions Before PR**:
1. Add all test files (critical)
2. Add mock implementations (critical)
3. Add KCP integration types and annotations (critical)
4. Add package documentation (critical)
5. Add error types (critical)
6. Add client interfaces (critical)
7. Add example code (critical)

**Estimated Additional Lines**: ~800-1000 lines (will require splitting into 2 PRs)

**Recommendation**: 
1. Complete all critical fixes in this PR
2. If total exceeds 800 lines, split mocks into separate PR
3. Ensure all tests pass before submission
4. Run linting and formatting tools

## Commands to Run After Fixes

```bash
# Format code
go fmt ./pkg/syncer/interfaces/...

# Run tests
go test ./pkg/syncer/interfaces/...

# Build verification
go build ./pkg/syncer/interfaces/...

# Linting
golangci-lint run ./pkg/syncer/interfaces/...

# Verify line count
/workspaces/kcp-shared-tools/tmc-pr-line-counter.sh -c feature/tmc-phase4-sync-01-interfaces
```

## Git Commit Structure

After implementing fixes, commit with:

```bash
git add pkg/syncer/interfaces/
git commit -s -S -m "test(syncer): add comprehensive test coverage for interfaces

- Add unit tests for all interface types
- Add mock implementations for testing
- Add example code demonstrating usage
- Add validation and error handling tests"

git add pkg/syncer/interfaces/doc.go pkg/syncer/interfaces/errors.go
git commit -s -S -m "feat(syncer): add package documentation and error types

- Add comprehensive package documentation
- Define error types for sync operations
- Add error helper functions"

git add pkg/syncer/interfaces/types.go pkg/syncer/interfaces/clients.go
git commit -s -S -m "feat(syncer): add KCP integration types and annotations

- Add common annotations and labels
- Add client interfaces for KCP integration
- Add feature gate constants
- Add validation methods"
```