# Implementation Instructions: Event Syncer

## Branch: `feature/phase7-syncer-impl/p7w3-events`

## Overview
This branch implements event synchronization from physical clusters to KCP, providing visibility into what's happening in downstream clusters. It handles event filtering, transformation, deduplication, and aggregation.

**Target Size**: ~450 lines  
**Complexity**: Medium  
**Priority**: Medium (enhances observability)

## Dependencies
- **Phase 5 APIs**: Uses event interfaces
- **Phase 6 Infrastructure**: Virtual workspace support
- **Wave 1 Sync Engine**: Event queue integration
- **Wave 3 Upstream Status**: Coordinates with status sync

## Files to Create

### 1. Event Syncer Core (~200 lines)
**File**: `pkg/reconciler/workload/syncer/events/syncer.go`
- Main event syncer struct
- Event watching setup
- Event transformation
- KCP event creation

### 2. Event Filtering (~80 lines)
**File**: `pkg/reconciler/workload/syncer/events/filter.go`
- Event importance filtering
- Noise reduction
- Rate limiting per source
- Deduplication logic

### 3. Event Aggregation (~80 lines)
**File**: `pkg/reconciler/workload/syncer/events/aggregation.go`
- Similar event grouping
- Event summarization
- Multi-cluster event correlation
- Event count tracking

### 4. Event Types (~40 lines)
**File**: `pkg/reconciler/workload/syncer/events/types.go`
- Event metadata types
- Filter configuration
- Aggregation rules

### 5. Event Tests (~50 lines)
**File**: `pkg/reconciler/workload/syncer/events/syncer_test.go`
- Unit tests for event sync
- Filter tests
- Aggregation tests

## Step-by-Step Implementation Guide

### Step 1: Create Package Structure
```bash
mkdir -p pkg/reconciler/workload/syncer/events
```

### Step 2: Define Event Types
Create `types.go` with:

```go
package events

import (
    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EventSyncConfig configures event synchronization
type EventSyncConfig struct {
    // Filtering
    MinimumSeverity  string
    IncludeTypes     []string
    ExcludeTypes     []string
    MaxEventsPerMin  int
    
    // Aggregation
    AggregationWindow time.Duration
    MaxAggregatedEvents int
    
    // Transformation
    AddLabels       map[string]string
    AddAnnotations  map[string]string
}

// SyncedEvent represents an event to sync
type SyncedEvent struct {
    Event           *corev1.Event
    SourceCluster   string
    TransformedName string
    Aggregated      bool
    Count           int32
}

// EventFilter defines event filtering rules
type EventFilter struct {
    Severity   []string
    Types      []string
    Reasons    []string
    Namespaces []string
    Objects    []string
}

// AggregationKey identifies similar events
type AggregationKey struct {
    Namespace string
    Name      string
    Type      string
    Reason    string
    Message   string
}
```

### Step 3: Implement Event Syncer Core
Create `syncer.go` with:

```go
package events

import (
    "context"
    "fmt"
    "time"
    
    workloadv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"
    
    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/tools/cache"
    "k8s.io/klog/v2"
    "github.com/kcp-dev/logicalcluster/v3"
)

// EventSyncer synchronizes events from downstream to KCP
type EventSyncer struct {
    kcpClient        kubernetes.ClusterInterface
    downstreamClient kubernetes.Interface
    
    syncTarget       *workloadv1alpha1.SyncTarget
    workspace        logicalcluster.Name
    
    filter           *EventFilter
    aggregator       *EventAggregator
    
    config           EventSyncConfig
    
    // Deduplication
    seenEvents       map[string]time.Time
    mu               sync.RWMutex
}

// NewEventSyncer creates a new event syncer
func NewEventSyncer(
    kcpClient kubernetes.ClusterInterface,
    downstreamClient kubernetes.Interface,
    syncTarget *workloadv1alpha1.SyncTarget,
    workspace logicalcluster.Name,
    config EventSyncConfig,
) *EventSyncer {
    return &EventSyncer{
        kcpClient:        kcpClient,
        downstreamClient: downstreamClient,
        syncTarget:       syncTarget,
        workspace:        workspace,
        filter:           NewEventFilter(config),
        aggregator:       NewEventAggregator(config),
        config:           config,
        seenEvents:       make(map[string]time.Time),
    }
}

// Start begins watching and syncing events
func (s *EventSyncer) Start(ctx context.Context) error {
    logger := klog.FromContext(ctx)
    logger.Info("Starting event syncer")
    
    // Create event informer
    informer := cache.NewSharedInformer(
        &cache.ListWatch{
            ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
                return s.downstreamClient.CoreV1().Events(metav1.NamespaceAll).List(ctx, options)
            },
            WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
                return s.downstreamClient.CoreV1().Events(metav1.NamespaceAll).Watch(ctx, options)
            },
        },
        &corev1.Event{},
        time.Minute,
    )
    
    // Add event handler
    informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
        AddFunc: func(obj interface{}) {
            s.handleEvent(ctx, obj.(*corev1.Event))
        },
        UpdateFunc: func(old, new interface{}) {
            s.handleEvent(ctx, new.(*corev1.Event))
        },
    })
    
    // Start informer
    go informer.Run(ctx.Done())
    
    // Start cleanup routine
    go s.cleanupLoop(ctx)
    
    return nil
}

// handleEvent processes a downstream event
func (s *EventSyncer) handleEvent(ctx context.Context, event *corev1.Event) {
    logger := klog.FromContext(ctx)
    
    // Check if event should be synced
    if !s.filter.ShouldSync(event) {
        logger.V(5).Info("Event filtered out", "event", event.Name)
        return
    }
    
    // Check for deduplication
    if s.isDuplicate(event) {
        logger.V(5).Info("Duplicate event skipped", "event", event.Name)
        return
    }
    
    // Transform event for KCP
    transformed := s.transformEvent(event)
    
    // Check for aggregation
    if s.aggregator.ShouldAggregate(transformed) {
        s.aggregator.AddEvent(transformed)
        return
    }
    
    // Sync to KCP
    if err := s.syncToKCP(ctx, transformed); err != nil {
        logger.Error(err, "Failed to sync event", "event", event.Name)
    }
}

// transformEvent transforms an event for KCP
func (s *EventSyncer) transformEvent(event *corev1.Event) *corev1.Event {
    transformed := event.DeepCopy()
    
    // Update namespace
    if transformed.Namespace != "" {
        transformed.Namespace = s.reverseNamespaceTransform(transformed.Namespace)
    }
    
    // Update involved object
    if transformed.InvolvedObject.Namespace != "" {
        transformed.InvolvedObject.Namespace = s.reverseNamespaceTransform(transformed.InvolvedObject.Namespace)
    }
    
    // Add metadata
    if transformed.Labels == nil {
        transformed.Labels = make(map[string]string)
    }
    transformed.Labels["kcp.io/sync-target"] = s.syncTarget.Name
    transformed.Labels["kcp.io/workspace"] = s.workspace.String()
    
    if transformed.Annotations == nil {
        transformed.Annotations = make(map[string]string)
    }
    transformed.Annotations["kcp.io/source-cluster"] = s.syncTarget.Name
    transformed.Annotations["kcp.io/synced-at"] = time.Now().Format(time.RFC3339)
    
    // Update name to avoid conflicts
    transformed.Name = fmt.Sprintf("%s-%s", s.syncTarget.Name, transformed.Name)
    
    return transformed
}

// syncToKCP creates the event in KCP
func (s *EventSyncer) syncToKCP(ctx context.Context, event *corev1.Event) error {
    // Reset resource version for creation
    event.ResourceVersion = ""
    event.UID = ""
    
    _, err := s.kcpClient.
        Cluster(s.workspace).
        CoreV1().
        Events(event.Namespace).
        Create(ctx, event, metav1.CreateOptions{})
    
    if err != nil {
        return fmt.Errorf("failed to create event in KCP: %w", err)
    }
    
    // Mark as seen
    s.markSeen(event)
    
    return nil
}
```

### Step 4: Implement Event Filtering
Create `filter.go` with:

1. **EventFilter struct**:
   - Severity-based filtering
   - Type/reason filtering
   - Namespace filtering
   - Rate limiting

2. **ShouldSync method**:
   - Apply filter rules
   - Check rate limits
   - Validate importance

3. **Dynamic filter updates**:
   - Runtime filter changes
   - Filter statistics
   - Filter effectiveness metrics

### Step 5: Implement Event Aggregation
Create `aggregation.go` with:

1. **EventAggregator struct**:
```go
type EventAggregator struct {
    window    time.Duration
    events    map[AggregationKey][]*corev1.Event
    mu        sync.RWMutex
}
```

2. **Aggregation logic**:
   - Group similar events
   - Count occurrences
   - Create summary events
   - Flush aggregated events

3. **Correlation**:
   - Cross-cluster correlation
   - Related event linking
   - Causal chain detection

### Step 6: Add Advanced Features

1. **Event enrichment**:
   - Add cluster context
   - Include resource details
   - Add timing information

2. **Event routing**:
   - Route to appropriate workspace
   - Severity-based routing
   - Custom routing rules

3. **Event retention**:
   - Cleanup old events
   - Archive important events
   - Compress event history

### Step 7: Add Comprehensive Tests
Create test files covering:

1. **Filtering tests**:
   - Various filter rules
   - Rate limiting
   - Edge cases

2. **Aggregation tests**:
   - Event grouping
   - Summary generation
   - Window timing

3. **Transformation tests**:
   - Namespace mapping
   - Metadata addition
   - Name uniqueness

## Testing Requirements

### Unit Tests:
- Event filtering logic
- Aggregation algorithms
- Transformation accuracy
- Deduplication
- Rate limiting

### Integration Tests:
- Full event sync flow
- High volume scenarios
- Multi-cluster events
- Event ordering

## Validation Checklist

- [ ] Events filtered appropriately
- [ ] Aggregation reduces noise effectively
- [ ] Transformations maintain event integrity
- [ ] Deduplication prevents duplicates
- [ ] Rate limiting prevents overload
- [ ] Comprehensive logging
- [ ] Metrics for event sync
- [ ] Tests achieve >70% coverage
- [ ] Code follows KCP patterns
- [ ] Feature flags integrated
- [ ] Under 450 lines (excluding tests)

## Common Pitfalls to Avoid

1. **Don't sync all events** - filter aggressively
2. **Avoid event storms** - use rate limiting
3. **Prevent duplicates** - track seen events
4. **Clean up old data** - implement retention
5. **Preserve event ordering** - maintain timestamps

## Integration Notes

This component:
- Works alongside Wave 3 status syncer
- Uses Wave 1 transformation patterns
- Provides events for monitoring
- Enhances debugging capabilities

Should provide:
- Configurable filtering
- Flexible aggregation
- Event metrics
- Query interface

## Success Criteria

The implementation is complete when:
1. Important events sync to KCP
2. Noise is effectively filtered
3. Similar events are aggregated
4. No duplicate events created
5. All tests pass
6. Can handle 1000+ events per minute