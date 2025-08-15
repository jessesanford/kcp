# Implementation Instructions for sync-01-interfaces

## Overview
**Branch**: `feature/tmc-phase4-01-sync-interfaces`  
**Purpose**: Define core abstraction layer for workload synchronization (lines 15-27 from plan)  
**Target Lines**: 350 lines

This branch establishes the foundational interfaces that all subsequent branches will implement. It defines the contracts for sync engine, resource transformation, status collection, and conflict resolution without any concrete implementations.

## Dependencies
- **Required branches**: None (this is the foundation)
- **KCP packages to import**:
  ```go
  "github.com/kcp-dev/kcp/sdk/apis/apis/v1alpha1"
  "github.com/kcp-dev/kcp/sdk/apis/core/v1alpha1"
  "github.com/kcp-dev/logicalcluster/v3"
  "k8s.io/apimachinery/pkg/runtime"
  "k8s.io/apimachinery/pkg/runtime/schema"
  "k8s.io/apimachinery/pkg/types"
  "k8s.io/client-go/tools/cache"
  ```

## Step-by-Step Implementation

### Step 1: Create Package Documentation (20 lines)
**File**: `pkg/syncer/interfaces/doc.go`
```go
/*
Copyright 2025 The KCP Authors.

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

// Package interfaces defines the core abstractions for workload synchronization
// between KCP control plane and physical clusters. These interfaces enable
// pluggable implementations for sync, transformation, and status aggregation.
package interfaces
```

### Step 2: Define Core Types (100 lines)
**File**: `pkg/syncer/interfaces/types.go`
```go
package interfaces

import (
    "time"
    
    "github.com/kcp-dev/logicalcluster/v3"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/runtime/schema"
    "k8s.io/apimachinery/pkg/types"
)

// Direction indicates the sync direction between KCP and physical cluster
type Direction string

const (
    // DownSync indicates synchronization from KCP to physical cluster
    DownSync Direction = "DownSync"
    // UpSync indicates synchronization from physical cluster to KCP
    UpSync Direction = "UpSync"
)

// ObjectReference uniquely identifies an object across clusters
type ObjectReference struct {
    // Workspace is the logical cluster containing the object
    Workspace logicalcluster.Name
    // Namespace of the object (empty for cluster-scoped)
    Namespace string
    // Name of the object
    Name string
    // UID is the unique identifier
    UID types.UID
    // GVK is the GroupVersionKind
    GVK schema.GroupVersionKind
}

// SyncStatus represents the current synchronization state
type SyncStatus struct {
    // Phase indicates the current sync phase
    Phase SyncPhase
    // LastSyncTime is when the last sync occurred
    LastSyncTime metav1.Time
    // ObservedGeneration is the generation last seen
    ObservedGeneration int64
    // Conditions represent sync conditions
    Conditions []metav1.Condition
    // Message provides human-readable status
    Message string
    // Reason provides machine-readable reason
    Reason string
}

// SyncPhase represents the phase of synchronization
type SyncPhase string

const (
    // SyncPhasePending indicates sync is pending
    SyncPhasePending SyncPhase = "Pending"
    // SyncPhaseProgressing indicates sync in progress
    SyncPhaseProgressing SyncPhase = "Progressing"
    // SyncPhaseSynced indicates successful sync
    SyncPhaseSynced SyncPhase = "Synced"
    // SyncPhaseFailed indicates sync failure
    SyncPhaseFailed SyncPhase = "Failed"
)

// TransformContext provides context for resource transformation
type TransformContext struct {
    // SourceWorkspace is the source logical cluster
    SourceWorkspace logicalcluster.Name
    // TargetCluster is the destination physical cluster
    TargetCluster string
    // Direction indicates sync direction
    Direction Direction
    // Annotations to add during transformation
    Annotations map[string]string
    // Labels to add during transformation
    Labels map[string]string
}

// ConflictResolutionStrategy defines how to resolve conflicts
type ConflictResolutionStrategy string

const (
    // LastWriteWins uses timestamp-based resolution
    LastWriteWins ConflictResolutionStrategy = "LastWriteWins"
    // GenerationBased uses generation comparison
    GenerationBased ConflictResolutionStrategy = "GenerationBased"
    // ServerWins always prefers server version
    ServerWins ConflictResolutionStrategy = "ServerWins"
    // ClientWins always prefers client version
    ClientWins ConflictResolutionStrategy = "ClientWins"
    // Merge attempts to merge changes
    Merge ConflictResolutionStrategy = "Merge"
)

// ConflictInfo contains information about a detected conflict
type ConflictInfo struct {
    // Object is the conflicting object reference
    Object ObjectReference
    // LocalVersion is the local object version
    LocalVersion string
    // RemoteVersion is the remote object version
    RemoteVersion string
    // DetectedAt is when conflict was detected
    DetectedAt time.Time
    // Strategy is the resolution strategy to use
    Strategy ConflictResolutionStrategy
}
```

### Step 3: Define SyncEngine Interface (80 lines)
**File**: `pkg/syncer/interfaces/sync_engine.go`
```go
package interfaces

import (
    "context"
    
    "k8s.io/apimachinery/pkg/runtime"
    "k8s.io/apimachinery/pkg/runtime/schema"
)

// SyncEngine defines the core synchronization operations following KCP patterns.
// Implementations must be workspace-aware and handle multi-tenancy correctly.
type SyncEngine interface {
    // Sync synchronizes an object to the target cluster.
    // It handles both creation and updates based on object state.
    Sync(ctx context.Context, obj runtime.Object) error
    
    // Delete removes an object from the target cluster.
    // It ensures proper cleanup including finalizers.
    Delete(ctx context.Context, obj runtime.Object) error
    
    // GetStatus retrieves the current sync status for an object.
    // Returns nil status if object has never been synced.
    GetStatus(ctx context.Context, ref ObjectReference) (*SyncStatus, error)
    
    // Watch starts watching for changes to sync.
    // The provided handler is called for each change.
    Watch(ctx context.Context, gvk schema.GroupVersionKind, handler SyncHandler) error
}

// SyncHandler processes sync events
type SyncHandler interface {
    // OnAdd handles object addition
    OnAdd(obj runtime.Object) error
    
    // OnUpdate handles object updates
    OnUpdate(oldObj, newObj runtime.Object) error
    
    // OnDelete handles object deletion
    OnDelete(obj runtime.Object) error
}

// SyncEngineFactory creates SyncEngine instances
type SyncEngineFactory interface {
    // NewSyncEngine creates a new sync engine for a workspace
    NewSyncEngine(workspace string, cluster string) (SyncEngine, error)
}

// SyncReconciler provides reconciliation logic for sync operations
type SyncReconciler interface {
    // Reconcile performs reconciliation for an object
    Reconcile(ctx context.Context, ref ObjectReference) error
    
    // NeedsSync determines if an object needs synchronization
    NeedsSync(obj runtime.Object) bool
    
    // SetTransformer sets the resource transformer to use
    SetTransformer(transformer ResourceTransformer)
    
    // SetConflictResolver sets the conflict resolver to use
    SetConflictResolver(resolver ConflictResolver)
}

// SyncCache provides caching for sync operations
type SyncCache interface {
    // Add adds an object to the cache
    Add(obj runtime.Object) error
    
    // Update updates an object in the cache
    Update(obj runtime.Object) error
    
    // Delete removes an object from the cache
    Delete(obj runtime.Object) error
    
    // Get retrieves an object from the cache
    Get(ref ObjectReference) (runtime.Object, bool, error)
    
    // List lists all objects of a given type
    List(gvk schema.GroupVersionKind) ([]runtime.Object, error)
}
```

### Step 4: Define ResourceTransformer Interface (60 lines)
**File**: `pkg/syncer/interfaces/resource_transformer.go`
```go
package interfaces

import (
    "context"
    
    "k8s.io/apimachinery/pkg/runtime"
    "k8s.io/apimachinery/pkg/runtime/schema"
)

// ResourceTransformer handles resource transformation during synchronization.
// It follows the pipeline pattern allowing multiple transformations to be chained.
type ResourceTransformer interface {
    // Transform applies transformations to a resource based on sync direction.
    // Returns the transformed object or error if transformation fails.
    Transform(ctx context.Context, in runtime.Object, tctx TransformContext) (runtime.Object, error)
    
    // CanTransform checks if this transformer can handle the given GVK.
    // Used to build transformation pipelines dynamically.
    CanTransform(gvk schema.GroupVersionKind) bool
    
    // Priority returns the priority of this transformer.
    // Lower values execute first in the pipeline.
    Priority() int
}

// TransformPipeline chains multiple transformers
type TransformPipeline interface {
    // AddTransformer adds a transformer to the pipeline
    AddTransformer(transformer ResourceTransformer) error
    
    // RemoveTransformer removes a transformer from the pipeline
    RemoveTransformer(name string) error
    
    // Transform executes the pipeline on a resource
    Transform(ctx context.Context, in runtime.Object, tctx TransformContext) (runtime.Object, error)
    
    // ListTransformers returns all transformers in execution order
    ListTransformers() []ResourceTransformer
}

// TransformStage represents a single stage in the transformation pipeline
type TransformStage interface {
    ResourceTransformer
    
    // Name returns the stage name for identification
    Name() string
    
    // Configure applies configuration to the stage
    Configure(config map[string]interface{}) error
    
    // Validate checks if the stage is properly configured
    Validate() error
}

// TransformerRegistry manages available transformers
type TransformerRegistry interface {
    // Register registers a new transformer
    Register(name string, transformer ResourceTransformer) error
    
    // Get retrieves a transformer by name
    Get(name string) (ResourceTransformer, bool)
    
    // List returns all registered transformer names
    List() []string
}
```

### Step 5: Define StatusCollector Interface (50 lines)
**File**: `pkg/syncer/interfaces/status_collector.go`
```go
package interfaces

import (
    "context"
    "time"
    
    "k8s.io/apimachinery/pkg/runtime"
)

// StatusCollector gathers status information from synchronized resources.
// It must handle workspace isolation and multi-cluster scenarios.
type StatusCollector interface {
    // Collect gathers status from a specific cluster
    Collect(ctx context.Context, cluster string) ([]WorkloadStatus, error)
    
    // CollectForWorkspace gathers status for a specific workspace
    CollectForWorkspace(ctx context.Context, workspace string) ([]WorkloadStatus, error)
    
    // Subscribe registers for status updates
    Subscribe(handler StatusHandler) error
    
    // Unsubscribe removes a status handler
    Unsubscribe(handler StatusHandler) error
}

// WorkloadStatus represents status for a workload
type WorkloadStatus struct {
    // Reference to the workload
    Reference ObjectReference
    // Cluster where workload is running
    Cluster string
    // Conditions from the workload
    Conditions []metav1.Condition
    // Replicas information
    Replicas ReplicaStatus
    // LastUpdated timestamp
    LastUpdated time.Time
}

// ReplicaStatus contains replica information
type ReplicaStatus struct {
    Total     int32
    Ready     int32
    Available int32
    Updated   int32
}

// StatusHandler processes status updates
type StatusHandler interface {
    // OnStatusUpdate handles status updates
    OnStatusUpdate(status WorkloadStatus) error
    
    // OnStatusDelete handles status deletion
    OnStatusDelete(ref ObjectReference) error
}
```

### Step 6: Define ConflictResolver Interface (40 lines)
**File**: `pkg/syncer/interfaces/conflict_resolver.go`
```go
package interfaces

import (
    "context"
    
    "k8s.io/apimachinery/pkg/runtime"
)

// ConflictResolver handles conflict resolution during bidirectional sync.
// Implementations must be deterministic and workspace-aware.
type ConflictResolver interface {
    // Resolve attempts to resolve a conflict between versions.
    // Returns the resolved object or an error if resolution fails.
    Resolve(ctx context.Context, local, remote runtime.Object, info ConflictInfo) (runtime.Object, error)
    
    // DetectConflict checks if two objects are in conflict.
    // Returns true if conflict exists, false otherwise.
    DetectConflict(local, remote runtime.Object) (bool, *ConflictInfo)
    
    // SetStrategy sets the default resolution strategy
    SetStrategy(strategy ConflictResolutionStrategy)
    
    // GetStrategy returns the current resolution strategy
    GetStrategy() ConflictResolutionStrategy
}

// ConflictDetector identifies conflicts
type ConflictDetector interface {
    // HasConflict checks for conflicts
    HasConflict(local, remote runtime.Object) bool
    
    // GetConflictDetails returns detailed conflict information
    GetConflictDetails(local, remote runtime.Object) *ConflictInfo
}

// ConflictRecorder records conflict history
type ConflictRecorder interface {
    // Record records a conflict occurrence
    Record(info ConflictInfo) error
    
    // GetHistory retrieves conflict history for an object
    GetHistory(ref ObjectReference) ([]ConflictInfo, error)
}
```

## Integration Points
- This branch provides the foundation for ALL other sync branches
- Branches 4, 5, 6, 7, 9 will implement these interfaces
- No external dependencies, pure interface definitions
- Must be merged first before any implementation work begins

## Testing Requirements
- Interface compilation tests only (no implementations to test)
- Verify all interfaces follow Go best practices
- Ensure proper documentation for all exported types
- Create mock implementations using `mockgen` for use by other branches:
  ```bash
  mockgen -source=pkg/syncer/interfaces/sync_engine.go -destination=pkg/syncer/mocks/sync_engine_mock.go
  mockgen -source=pkg/syncer/interfaces/resource_transformer.go -destination=pkg/syncer/mocks/transformer_mock.go
  mockgen -source=pkg/syncer/interfaces/status_collector.go -destination=pkg/syncer/mocks/collector_mock.go
  mockgen -source=pkg/syncer/interfaces/conflict_resolver.go -destination=pkg/syncer/mocks/resolver_mock.go
  ```

## KCP Patterns to Follow
1. **Workspace Awareness**: All interfaces include workspace/logical cluster references
2. **Multi-tenancy**: Use `logicalcluster.Name` for workspace identification
3. **Condition-based Status**: Follow metav1.Condition patterns
4. **Context Propagation**: All operations accept context.Context
5. **Error Handling**: Return errors explicitly, no panics
6. **Immutability**: Interfaces should not modify input objects directly

## Common Pitfalls to Avoid
- ❌ Don't use string for workspace names - use `logicalcluster.Name`
- ❌ Don't forget context in interface methods
- ❌ Don't make interfaces too granular - keep them cohesive
- ❌ Don't include implementation details in interfaces
- ❌ Don't forget to handle both cluster-scoped and namespaced resources

## Success Criteria
- [ ] All interfaces compile without errors
- [ ] Complete godoc documentation for all exported types
- [ ] Mock implementations generated for testing
- [ ] No dependencies on concrete implementations
- [ ] Follows KCP workspace and multi-tenancy patterns
- [ ] Total lines under 350 (current estimate: 350)
- [ ] All files properly licensed with Apache 2.0 header

## Feature Flag Configuration
```go
// Add to pkg/features/kcp_features.go
const (
    // WorkloadSync enables the workload synchronization feature
    WorkloadSync featuregate.Feature = "WorkloadSync"
)

// Add to defaultKCPFeatureGates
WorkloadSync: {Default: false, PreRelease: featuregate.Alpha},
```

## Next Steps After Completion
1. Generate mocks for all interfaces
2. Create interface compliance tests
3. Document interface contracts in detail
4. Prepare for parallel implementation work
5. Ensure CI passes before creating PR