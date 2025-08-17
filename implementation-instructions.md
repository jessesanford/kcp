# Cluster Controller Implementation Instructions

## Overview
This branch implements the Cluster controller responsible for managing ClusterRegistration resources and coordinating cluster lifecycle operations. It builds upon the SyncTarget controller interfaces to provide comprehensive cluster management capabilities.

**Branch**: `feature/tmc-completion/p1w1-cluster-controller`  
**Estimated Lines**: 650 lines  
**Wave**: 2  
**Dependencies**: p1w1-synctarget-controller must be complete  

## Dependencies

### Required Before Starting
- Phase 0 APIs complete (ClusterRegistration, SyncTarget types)
- p1w1-synctarget-controller merged (provides SyncTarget interfaces)
- Core TMC API types available in pkg/apis/

### Blocks These Features
- None directly, but complements Virtual Workspace functionality

## Files to Create/Modify

### Primary Implementation Files (650 lines total)

1. **pkg/reconciler/workload/cluster/controller.go** (200 lines)
   - Main controller implementation
   - Queue management and worker loops
   - Event handler registration

2. **pkg/reconciler/workload/cluster/cluster_controller.go** (180 lines)
   - ClusterRegistration reconciliation logic
   - Cluster lifecycle management
   - Integration with SyncTarget

3. **pkg/reconciler/workload/cluster/registration.go** (120 lines)
   - Registration workflow implementation
   - Validation and approval logic
   - Certificate management

4. **pkg/reconciler/workload/cluster/status.go** (80 lines)
   - Status condition management
   - Phase calculation
   - Health aggregation

5. **pkg/reconciler/workload/cluster/helpers.go** (70 lines)
   - Utility functions
   - Label management
   - Resource creation helpers

### Test Files (not counted in line limit)

1. **pkg/reconciler/workload/cluster/controller_test.go**
2. **pkg/reconciler/workload/cluster/cluster_controller_test.go**
3. **pkg/reconciler/workload/cluster/registration_test.go**

## Step-by-Step Implementation Guide

### Step 1: Setup Controller Structure (Hour 1-2)

```go
// pkg/reconciler/workload/cluster/controller.go
package cluster

import (
    "context"
    "fmt"
    "time"

    tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
    kcpclientset "github.com/kcp-dev/kcp/pkg/client/clientset/versioned/cluster"
    tmcinformers "github.com/kcp-dev/kcp/pkg/client/informers/externalversions/tmc/v1alpha1"
    tmclisters "github.com/kcp-dev/kcp/pkg/client/listers/tmc/v1alpha1"
    synctargetcontroller "github.com/kcp-dev/kcp/pkg/reconciler/workload/synctarget"
    
    "github.com/kcp-dev/logicalcluster/v3"
    
    "k8s.io/apimachinery/pkg/util/runtime"
    "k8s.io/apimachinery/pkg/util/wait"
    "k8s.io/client-go/tools/cache"
    "k8s.io/client-go/util/workqueue"
    "k8s.io/klog/v2"
)

// Controller manages ClusterRegistration resources
type Controller struct {
    queue workqueue.RateLimitingInterface
    
    kcpClusterClient kcpclientset.ClusterInterface
    
    clusterLister  tmclisters.ClusterRegistrationClusterLister
    clusterIndexer cache.Indexer
    
    syncTargetLister tmclisters.SyncTargetClusterLister
    
    // For workspace isolation
    logicalCluster logicalcluster.Name
    
    // SyncTarget integration
    syncTargetManager synctargetcontroller.Manager
}

// NewController creates a new Cluster controller
func NewController(
    kcpClusterClient kcpclientset.ClusterInterface,
    clusterInformer tmcinformers.ClusterRegistrationClusterInformer,
    syncTargetInformer tmcinformers.SyncTargetClusterInformer,
    logicalCluster logicalcluster.Name,
) (*Controller, error) {
    c := &Controller{
        queue:             workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "cluster"),
        kcpClusterClient:  kcpClusterClient,
        clusterLister:     clusterInformer.Lister(),
        clusterIndexer:    clusterInformer.Informer().GetIndexer(),
        syncTargetLister:  syncTargetInformer.Lister(),
        logicalCluster:    logicalCluster,
    }
    
    // Setup indexers for efficient lookups
    clusterInformer.Informer().AddIndexers(cache.Indexers{
        "bySyncTarget": c.indexBySyncTarget,
    })
    
    // Add event handlers
    clusterInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
        FilterFunc: func(obj interface{}) bool {
            return c.filterByWorkspace(obj)
        },
        Handler: cache.ResourceEventHandlerFuncs{
            AddFunc:    c.enqueue,
            UpdateFunc: func(old, new interface{}) { c.enqueueAfter(new, 1*time.Second) },
            DeleteFunc: c.enqueue,
        },
    })
    
    // Watch SyncTargets for changes that affect clusters
    syncTargetInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
        FilterFunc: c.filterByWorkspace,
        Handler: cache.ResourceEventHandlerFuncs{
            UpdateFunc: c.handleSyncTargetUpdate,
            DeleteFunc: c.handleSyncTargetDelete,
        },
    })
    
    return c, nil
}
```

### Step 2: Implement Core Reconciliation (Hour 3-4)

```go
// pkg/reconciler/workload/cluster/cluster_controller.go
package cluster

import (
    "context"
    "fmt"
    
    tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
    
    apierrors "k8s.io/apimachinery/pkg/api/errors"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/klog/v2"
)

// reconcile handles the main reconciliation logic
func (c *Controller) reconcile(ctx context.Context, key string) error {
    klog.V(4).Infof("Reconciling ClusterRegistration %s", key)
    
    cluster, clusterName, name, err := c.parseKey(key)
    if err != nil {
        return err
    }
    
    // Get the ClusterRegistration
    registration, err := c.clusterLister.Cluster(cluster).Get(name)
    if apierrors.IsNotFound(err) {
        klog.V(2).Infof("ClusterRegistration %s|%s not found", clusterName, name)
        return nil
    }
    if err != nil {
        return err
    }
    
    // Deep copy to avoid mutating cache
    registration = registration.DeepCopy()
    
    // Handle deletion
    if !registration.DeletionTimestamp.IsZero() {
        return c.handleDeletion(ctx, registration)
    }
    
    // Ensure finalizer
    if !contains(registration.Finalizers, ClusterRegistrationFinalizer) {
        registration.Finalizers = append(registration.Finalizers, ClusterRegistrationFinalizer)
        registration, err = c.updateClusterRegistration(ctx, registration)
        if err != nil {
            return err
        }
    }
    
    // Reconcile the cluster
    reconcileErr := c.reconcileCluster(ctx, registration)
    
    // Update status
    if err := c.updateStatus(ctx, registration, reconcileErr); err != nil {
        return fmt.Errorf("failed to update status: %w", err)
    }
    
    return reconcileErr
}

func (c *Controller) reconcileCluster(ctx context.Context, cr *tmcv1alpha1.ClusterRegistration) error {
    // Validate registration
    if err := c.validateRegistration(cr); err != nil {
        return c.updateCondition(cr, tmcv1alpha1.ClusterConditionValid, metav1.ConditionFalse,
            "ValidationFailed", err.Error())
    }
    
    // Process registration based on phase
    switch cr.Status.Phase {
    case "", tmcv1alpha1.ClusterPhasePending:
        return c.processPendingRegistration(ctx, cr)
    case tmcv1alpha1.ClusterPhaseApproved:
        return c.processApprovedRegistration(ctx, cr)
    case tmcv1alpha1.ClusterPhaseActive:
        return c.maintainActiveCluster(ctx, cr)
    case tmcv1alpha1.ClusterPhaseFailed:
        return c.handleFailedCluster(ctx, cr)
    }
    
    return nil
}

func (c *Controller) processPendingRegistration(ctx context.Context, cr *tmcv1alpha1.ClusterRegistration) error {
    // Check if auto-approval is enabled
    if c.shouldAutoApprove(cr) {
        cr.Status.Phase = tmcv1alpha1.ClusterPhaseApproved
        return c.updateCondition(cr, tmcv1alpha1.ClusterConditionApproved, metav1.ConditionTrue,
            "AutoApproved", "Cluster registration auto-approved")
    }
    
    // Wait for manual approval
    return c.updateCondition(cr, tmcv1alpha1.ClusterConditionApproved, metav1.ConditionFalse,
        "WaitingForApproval", "Cluster registration pending approval")
}

func (c *Controller) processApprovedRegistration(ctx context.Context, cr *tmcv1alpha1.ClusterRegistration) error {
    // Create or update associated SyncTarget
    syncTarget, err := c.ensureSyncTarget(ctx, cr)
    if err != nil {
        return c.updateCondition(cr, tmcv1alpha1.ClusterConditionReady, metav1.ConditionFalse,
            "SyncTargetCreationFailed", err.Error())
    }
    
    // Wait for SyncTarget to be ready
    if !c.isSyncTargetReady(syncTarget) {
        return c.updateCondition(cr, tmcv1alpha1.ClusterConditionReady, metav1.ConditionFalse,
            "WaitingForSyncTarget", "Waiting for SyncTarget to be ready")
    }
    
    // Generate cluster credentials
    if cr.Status.Credentials == nil {
        credentials, err := c.generateCredentials(ctx, cr)
        if err != nil {
            return fmt.Errorf("failed to generate credentials: %w", err)
        }
        cr.Status.Credentials = credentials
    }
    
    // Mark as active
    cr.Status.Phase = tmcv1alpha1.ClusterPhaseActive
    return c.updateCondition(cr, tmcv1alpha1.ClusterConditionReady, metav1.ConditionTrue,
        "ClusterActive", "Cluster is active and ready")
}
```

### Step 3: Implement Registration Logic (Hour 5)

```go
// pkg/reconciler/workload/cluster/registration.go
package cluster

import (
    "context"
    "crypto/x509"
    "fmt"
    
    tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
    
    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// validateRegistration performs validation on the registration
func (c *Controller) validateRegistration(cr *tmcv1alpha1.ClusterRegistration) error {
    // Validate required fields
    if cr.Spec.Location == "" {
        return fmt.Errorf("location is required")
    }
    
    // Validate cluster type
    if cr.Spec.Type != "" {
        if !isValidClusterType(cr.Spec.Type) {
            return fmt.Errorf("invalid cluster type: %s", cr.Spec.Type)
        }
    }
    
    // Validate labels
    if cr.Labels == nil {
        cr.Labels = make(map[string]string)
    }
    cr.Labels[tmcv1alpha1.LabelWorkspace] = string(c.logicalCluster)
    cr.Labels[tmcv1alpha1.LabelLocation] = cr.Spec.Location
    
    return nil
}

// shouldAutoApprove determines if the registration should be auto-approved
func (c *Controller) shouldAutoApprove(cr *tmcv1alpha1.ClusterRegistration) bool {
    // Check for auto-approval annotation
    if cr.Annotations != nil {
        if val, ok := cr.Annotations[tmcv1alpha1.AnnotationAutoApprove]; ok && val == "true" {
            return true
        }
    }
    
    // Check if from trusted source
    if c.isTrustedSource(cr) {
        return true
    }
    
    return false
}

// generateCredentials creates credentials for the cluster
func (c *Controller) generateCredentials(ctx context.Context, cr *tmcv1alpha1.ClusterRegistration) (*tmcv1alpha1.ClusterCredentials, error) {
    // Generate certificate for cluster authentication
    cert, key, err := c.generateClusterCertificate(cr)
    if err != nil {
        return nil, fmt.Errorf("failed to generate certificate: %w", err)
    }
    
    // Create secret to store credentials
    secret := &corev1.Secret{
        ObjectMeta: metav1.ObjectMeta{
            Name:      fmt.Sprintf("%s-credentials", cr.Name),
            Namespace: cr.Namespace,
            OwnerReferences: []metav1.OwnerReference{
                {
                    APIVersion: tmcv1alpha1.SchemeGroupVersion.String(),
                    Kind:       "ClusterRegistration",
                    Name:       cr.Name,
                    UID:        cr.UID,
                    Controller: ptr.To(true),
                },
            },
        },
        Type: corev1.SecretTypeTLS,
        Data: map[string][]byte{
            "tls.crt": cert,
            "tls.key": key,
            "ca.crt":  c.getClusterCA(),
        },
    }
    
    _, err = c.kcpClusterClient.
        Cluster(logicalcluster.From(cr).Path()).
        CoreV1().
        Secrets(cr.Namespace).
        Create(ctx, secret, metav1.CreateOptions{})
    if err != nil {
        return nil, err
    }
    
    return &tmcv1alpha1.ClusterCredentials{
        SecretRef: &corev1.LocalObjectReference{
            Name: secret.Name,
        },
        CertificateExpirationTime: metav1.NewTime(time.Now().Add(365 * 24 * time.Hour)),
    }, nil
}

// ensureSyncTarget creates or updates the SyncTarget for this cluster
func (c *Controller) ensureSyncTarget(ctx context.Context, cr *tmcv1alpha1.ClusterRegistration) (*tmcv1alpha1.SyncTarget, error) {
    syncTargetName := fmt.Sprintf("%s-synctarget", cr.Name)
    
    // Check if SyncTarget exists
    existing, err := c.syncTargetLister.
        Cluster(logicalcluster.From(cr)).
        Get(syncTargetName)
    if err == nil {
        // Update if needed
        return c.updateSyncTarget(ctx, existing, cr)
    }
    
    // Create new SyncTarget
    syncTarget := &tmcv1alpha1.SyncTarget{
        ObjectMeta: metav1.ObjectMeta{
            Name: syncTargetName,
            Labels: map[string]string{
                tmcv1alpha1.LabelClusterRegistration: cr.Name,
                tmcv1alpha1.LabelWorkspace:          string(c.logicalCluster),
                tmcv1alpha1.LabelLocation:           cr.Spec.Location,
            },
            OwnerReferences: []metav1.OwnerReference{
                {
                    APIVersion: tmcv1alpha1.SchemeGroupVersion.String(),
                    Kind:       "ClusterRegistration",
                    Name:       cr.Name,
                    UID:        cr.UID,
                    Controller: ptr.To(true),
                },
            },
        },
        Spec: tmcv1alpha1.SyncTargetSpec{
            KubeConfig: cr.Spec.KubeConfig,
            Cell:       cr.Spec.Location,
        },
    }
    
    return c.kcpClusterClient.
        Cluster(logicalcluster.From(cr).Path()).
        TmcV1alpha1().
        SyncTargets().
        Create(ctx, syncTarget, metav1.CreateOptions{})
}
```

### Step 4: Implement Status Management (Hour 6)

```go
// pkg/reconciler/workload/cluster/status.go
package cluster

import (
    "context"
    "fmt"
    
    tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
    
    "k8s.io/apimachinery/pkg/api/equality"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *Controller) updateStatus(ctx context.Context, cr *tmcv1alpha1.ClusterRegistration, reconcileErr error) error {
    // Prepare status update
    newStatus := cr.Status.DeepCopy()
    
    // Update phase if needed
    if newStatus.Phase == "" {
        newStatus.Phase = tmcv1alpha1.ClusterPhasePending
    }
    
    // Update last heartbeat
    newStatus.LastHeartbeatTime = &metav1.Time{Time: time.Now()}
    
    // Calculate resource summary if active
    if cr.Status.Phase == tmcv1alpha1.ClusterPhaseActive && reconcileErr == nil {
        summary, err := c.calculateResourceSummary(ctx, cr)
        if err == nil {
            newStatus.ResourceSummary = summary
        }
    }
    
    // Only update if status changed
    if !equality.Semantic.DeepEqual(cr.Status, *newStatus) {
        cr.Status = *newStatus
        _, err := c.kcpClusterClient.
            Cluster(logicalcluster.From(cr).Path()).
            TmcV1alpha1().
            ClusterRegistrations().
            UpdateStatus(ctx, cr, metav1.UpdateOptions{})
        return err
    }
    
    return nil
}

func (c *Controller) updateCondition(cr *tmcv1alpha1.ClusterRegistration,
    conditionType tmcv1alpha1.ClusterConditionType,
    status metav1.ConditionStatus,
    reason, message string) error {
    
    condition := metav1.Condition{
        Type:               string(conditionType),
        Status:             status,
        LastTransitionTime: metav1.Now(),
        Reason:             reason,
        Message:            message,
        ObservedGeneration: cr.Generation,
    }
    
    // Update or append condition
    found := false
    for i, existing := range cr.Status.Conditions {
        if existing.Type == string(conditionType) {
            if existing.Status != status {
                cr.Status.Conditions[i] = condition
            }
            found = true
            break
        }
    }
    
    if !found {
        cr.Status.Conditions = append(cr.Status.Conditions, condition)
    }
    
    return nil
}

func (c *Controller) calculateResourceSummary(ctx context.Context, cr *tmcv1alpha1.ClusterRegistration) (*tmcv1alpha1.ResourceSummary, error) {
    // Get associated SyncTarget
    syncTargetName := fmt.Sprintf("%s-synctarget", cr.Name)
    syncTarget, err := c.syncTargetLister.
        Cluster(logicalcluster.From(cr)).
        Get(syncTargetName)
    if err != nil {
        return nil, err
    }
    
    // Aggregate resources from SyncTarget status
    return &tmcv1alpha1.ResourceSummary{
        TotalNodes:     syncTarget.Status.Capacity.Nodes,
        TotalCPU:       syncTarget.Status.Capacity.CPU,
        TotalMemory:    syncTarget.Status.Capacity.Memory,
        AllocatedCPU:   syncTarget.Status.Allocated.CPU,
        AllocatedMemory: syncTarget.Status.Allocated.Memory,
    }, nil
}
```

### Step 5: Add Helper Functions (Hour 7)

```go
// pkg/reconciler/workload/cluster/helpers.go
package cluster

import (
    "context"
    "fmt"
    "strings"
    
    tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
    "github.com/kcp-dev/logicalcluster/v3"
)

const (
    ClusterRegistrationFinalizer = "tmc.kcp.dev/cluster-registration"
)

// parseKey extracts cluster and name from a queue key
func (c *Controller) parseKey(key string) (logicalcluster.Path, string, string, error) {
    parts := strings.Split(key, "|")
    if len(parts) != 2 {
        return logicalcluster.Path{}, "", "", fmt.Errorf("invalid key format: %s", key)
    }
    
    cluster, err := logicalcluster.NewPath(parts[0])
    if err != nil {
        return logicalcluster.Path{}, "", "", err
    }
    
    return cluster, parts[0], parts[1], nil
}

// filterByWorkspace ensures we only process resources in our workspace
func (c *Controller) filterByWorkspace(obj interface{}) bool {
    metaObj, err := meta.Accessor(obj)
    if err != nil {
        return false
    }
    
    cluster := logicalcluster.From(metaObj)
    return cluster == c.logicalCluster
}

// indexBySyncTarget creates an index by SyncTarget reference
func (c *Controller) indexBySyncTarget(obj interface{}) ([]string, error) {
    cr, ok := obj.(*tmcv1alpha1.ClusterRegistration)
    if !ok {
        return nil, fmt.Errorf("object is not a ClusterRegistration")
    }
    
    syncTargetName := fmt.Sprintf("%s-synctarget", cr.Name)
    return []string{syncTargetName}, nil
}

// handleSyncTargetUpdate handles updates to related SyncTargets
func (c *Controller) handleSyncTargetUpdate(old, new interface{}) {
    newST := new.(*tmcv1alpha1.SyncTarget)
    
    // Find associated ClusterRegistrations
    if clusterName, ok := newST.Labels[tmcv1alpha1.LabelClusterRegistration]; ok {
        key := fmt.Sprintf("%s|%s", logicalcluster.From(newST), clusterName)
        c.queue.Add(key)
    }
}

// handleSyncTargetDelete handles deletion of related SyncTargets
func (c *Controller) handleSyncTargetDelete(obj interface{}) {
    st := obj.(*tmcv1alpha1.SyncTarget)
    
    // Find and requeue associated ClusterRegistration
    if clusterName, ok := st.Labels[tmcv1alpha1.LabelClusterRegistration]; ok {
        key := fmt.Sprintf("%s|%s", logicalcluster.From(st), clusterName)
        c.queue.Add(key)
    }
}

// isValidClusterType validates cluster type
func isValidClusterType(clusterType string) bool {
    validTypes := []string{"physical", "virtual", "edge", "cloud"}
    for _, valid := range validTypes {
        if clusterType == valid {
            return true
        }
    }
    return false
}

// isTrustedSource checks if the registration is from a trusted source
func (c *Controller) isTrustedSource(cr *tmcv1alpha1.ClusterRegistration) bool {
    // Check for trusted annotation
    if cr.Annotations != nil {
        if val, ok := cr.Annotations["tmc.kcp.dev/trusted-source"]; ok {
            return val == "true"
        }
    }
    return false
}

// contains checks if a string is in a slice
func contains(slice []string, item string) bool {
    for _, s := range slice {
        if s == item {
            return true
        }
    }
    return false
}
```

## Testing Requirements

### Unit Tests

1. **Controller Creation Tests**
   - Test NewController initialization
   - Test indexer setup
   - Test event handler registration

2. **Reconciliation Tests**
   - Test pending registration processing
   - Test approval workflow
   - Test active cluster maintenance
   - Test deletion handling

3. **Registration Tests**
   - Test validation logic
   - Test auto-approval
   - Test credential generation
   - Test SyncTarget creation

4. **Status Tests**
   - Test condition updates
   - Test phase transitions
   - Test resource summary calculation

### Integration Tests

1. **End-to-End Workflow**
   - Register new cluster
   - Approve registration
   - Verify SyncTarget creation
   - Check active status

2. **Multi-Cluster Management**
   - Multiple registrations
   - Workspace isolation
   - Resource aggregation

## KCP Patterns to Follow

### Workspace Isolation
- All operations scoped to logical cluster
- Filter events by workspace
- Enforce tenant boundaries

### Controller Runtime Patterns
- Use workqueue for rate limiting
- Implement exponential backoff
- Handle transient errors gracefully

### Informer/Lister Patterns
- Use shared informer factory
- Cache with indexers for efficiency
- Minimize API calls

## Integration Points

### With SyncTarget Controller (p1w1-synctarget-controller)
- Creates and manages SyncTarget resources
- Monitors SyncTarget health
- Aggregates capacity information

### With Virtual Workspace (p1w2-vw-core)
- Provides cluster information for VW
- Shares connection details

### With Quota Manager (p1w3-quota-manager)
- Reports cluster capacity
- Updates resource availability

## Validation Checklist

### Before Commit
- [ ] All files created as specified
- [ ] Line count under 650 (run `/workspaces/kcp-shared-tools/tmc-pr-line-counter.sh`)
- [ ] All tests passing (`make test`)
- [ ] Linting passes (`make lint`)
- [ ] Generated code updated (`make codegen`)

### Functionality Complete
- [ ] Controller creates and starts properly
- [ ] ClusterRegistration reconciliation works
- [ ] Registration workflow complete
- [ ] SyncTarget integration functional
- [ ] Status updates correctly
- [ ] Workspace isolation enforced

### Integration Ready
- [ ] Interfaces compatible with SyncTarget controller
- [ ] Status API complete
- [ ] Metrics exposed
- [ ] Logging comprehensive
- [ ] Error handling robust

### Documentation Complete
- [ ] Code comments on all exported types
- [ ] API documentation updated
- [ ] Usage examples provided
- [ ] Architecture documented

## Commit Message Template
```
feat(cluster): implement Cluster controller with registration workflow

- Add ClusterRegistration controller with full lifecycle management
- Implement registration, approval, and activation workflow
- Integrate with SyncTarget controller for cluster connectivity
- Add credential generation and management
- Ensure workspace isolation throughout
- Add comprehensive status tracking and conditions

Part of TMC Phase 1 Wave 2 implementation
Depends on: p1w1-synctarget-controller
```

## Next Steps
After this branch is complete:
1. Virtual Workspace components can utilize cluster information
2. Resource management features can aggregate cluster data
3. Placement logic can target registered clusters