# Review Feedback Instructions - Wave 2A-01: SyncTarget Controller Core

## Current State
- Branch: `feature/tmc2-impl2/phase2/wave2a-01-split-from-controller`
- Focus: Core SyncTarget controller with reconciliation loop
- Estimated current lines: ~300 lines

## Priority Issues (P0 - Must Fix)

### 1. Missing Test Coverage
**CRITICAL**: No controller tests exist

#### Required Test Files to Create:
1. `pkg/reconciler/workload/synctarget/synctarget_controller_test.go` (~250 lines)
   - Test reconciliation loop
   - Test status updates
   - Test error handling

2. `pkg/reconciler/workload/synctarget/fake_test.go` (~100 lines)
   - Create fake clients
   - Mock helpers

### 2. Complete Controller Implementation

#### File: `pkg/reconciler/workload/synctarget/synctarget_controller.go`
Fix placeholder implementations (~150 lines):
```go
// Reconcile handles SyncTarget reconciliation
func (c *Controller) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    logger := logging.WithReconciler(klog.FromContext(ctx), ControllerName)
    ctx = klog.NewContext(ctx, logger)
    
    // Get the SyncTarget
    syncTarget := &workloadv1alpha1.SyncTarget{}
    if err := c.Get(ctx, req.NamespacedName, syncTarget); err != nil {
        if apierrors.IsNotFound(err) {
            return ctrl.Result{}, nil
        }
        return ctrl.Result{}, err
    }
    
    // Handle deletion
    if !syncTarget.DeletionTimestamp.IsZero() {
        return c.handleDeletion(ctx, syncTarget)
    }
    
    // Ensure finalizer
    if !controllerutil.ContainsFinalizer(syncTarget, FinalizerName) {
        controllerutil.AddFinalizer(syncTarget, FinalizerName)
        if err := c.Update(ctx, syncTarget); err != nil {
            return ctrl.Result{}, err
        }
    }
    
    // Validate connection
    if err := c.validateConnection(ctx, syncTarget); err != nil {
        return c.updateStatusError(ctx, syncTarget, err)
    }
    
    // Update status to ready
    return c.updateStatusReady(ctx, syncTarget)
}

// validateConnection checks SyncTarget connectivity
func (c *Controller) validateConnection(ctx context.Context, st *workloadv1alpha1.SyncTarget) error {
    // Get kubeconfig secret
    secret := &corev1.Secret{}
    secretKey := types.NamespacedName{
        Namespace: st.Namespace,
        Name:      st.Spec.KubeConfig,
    }
    
    if err := c.Get(ctx, secretKey, secret); err != nil {
        return fmt.Errorf("failed to get kubeconfig secret: %w", err)
    }
    
    // Parse kubeconfig
    kubeconfig, exists := secret.Data["kubeconfig"]
    if !exists {
        return fmt.Errorf("kubeconfig key not found in secret")
    }
    
    // Create client and test connection
    restConfig, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
    if err != nil {
        return fmt.Errorf("invalid kubeconfig: %w", err)
    }
    
    // Test with discovery client
    discoveryClient, err := discovery.NewDiscoveryClientForConfig(restConfig)
    if err != nil {
        return fmt.Errorf("failed to create discovery client: %w", err)
    }
    
    _, err = discoveryClient.ServerVersion()
    if err != nil {
        return fmt.Errorf("failed to connect to target cluster: %w", err)
    }
    
    return nil
}

// updateStatusReady updates status to ready condition
func (c *Controller) updateStatusReady(ctx context.Context, st *workloadv1alpha1.SyncTarget) (ctrl.Result, error) {
    condition := metav1.Condition{
        Type:               string(workloadv1alpha1.SyncTargetReady),
        Status:             metav1.ConditionTrue,
        Reason:             "Connected",
        Message:            "Successfully connected to target cluster",
        LastTransitionTime: metav1.Now(),
    }
    
    st.SetCondition(condition)
    st.Status.Phase = workloadv1alpha1.SyncTargetPhaseReady
    st.Status.LastHeartbeat = &metav1.Time{Time: time.Now()}
    
    if err := c.Status().Update(ctx, st); err != nil {
        return ctrl.Result{}, err
    }
    
    // Requeue for periodic health check
    return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}
```

### 3. Add Event Recording

#### Enhance controller with events (~50 lines):
```go
// Add to controller struct
type Controller struct {
    client.Client
    Scheme   *runtime.Scheme
    Recorder record.EventRecorder
}

// In reconciliation
func (c *Controller) recordEvent(st *workloadv1alpha1.SyncTarget, eventType, reason, message string) {
    c.Recorder.Event(st, eventType, reason, message)
}

// Usage in reconcile
c.recordEvent(syncTarget, corev1.EventTypeNormal, "Connected", "Successfully connected to target cluster")
```

## Line Count Analysis

### Current Estimate:
- Existing code: ~300 lines
- Required tests: ~350 lines
- Complete implementation: ~150 lines
- Event recording: ~50 lines
- **Total after fixes: ~850 lines** ❌ OVER LIMIT

### NEEDS SPLIT Strategy:
Split into 2 PRs:
1. **Current PR**: Core controller with basic reconciliation (~500 lines)
2. **Follow-up PR**: Comprehensive tests and advanced features (~400 lines)

## Specific Tasks for THIS PR

### 1. Complete Core Reconciliation
Focus only on:
- Basic reconciliation loop
- Connection validation
- Status updates
- Finalizer handling

### 2. Create Minimal Test
```go
func TestBasicReconciliation(t *testing.T) {
    // Setup
    scheme := runtime.NewScheme()
    _ = workloadv1alpha1.AddToScheme(scheme)
    _ = corev1.AddToScheme(scheme)
    
    syncTarget := &workloadv1alpha1.SyncTarget{
        ObjectMeta: metav1.ObjectMeta{
            Name:      "test-target",
            Namespace: "default",
        },
        Spec: workloadv1alpha1.SyncTargetSpec{
            KubeConfig: "test-secret",
        },
    }
    
    secret := &corev1.Secret{
        ObjectMeta: metav1.ObjectMeta{
            Name:      "test-secret",
            Namespace: "default",
        },
        Data: map[string][]byte{
            "kubeconfig": []byte(testKubeconfig),
        },
    }
    
    client := fake.NewClientBuilder().
        WithScheme(scheme).
        WithObjects(syncTarget, secret).
        Build()
    
    controller := &Controller{
        Client: client,
        Scheme: scheme,
    }
    
    // Test reconciliation
    req := ctrl.Request{
        NamespacedName: types.NamespacedName{
            Name:      "test-target",
            Namespace: "default",
        },
    }
    
    result, err := controller.Reconcile(context.TODO(), req)
    require.NoError(t, err)
    require.True(t, result.RequeueAfter > 0)
}
```

### 3. Defer to Follow-up PR
- Comprehensive test suite
- Event recording
- Metrics
- Advanced error handling
- Connection pooling

## Testing Requirements (This PR)

### Unit Test Coverage Target: 60%
1. **Basic Controller Tests**:
   - Reconciliation success path
   - Not found handling
   - Finalizer management

2. **Status Tests**:
   - Condition updates
   - Phase transitions

## Completion Checklist (This PR)

- [ ] Core reconciliation implemented
- [ ] Basic test coverage (60%)
- [ ] Connection validation working
- [ ] Status updates functional
- [ ] Finalizer handling complete
- [ ] `make test` passes
- [ ] Line count < 700 lines
- [ ] TODO comments for follow-up work
- [ ] Clean commit history

## Follow-up PR Planning
Create Wave 2A-01b for:
- Comprehensive test suite
- Event recording
- Metrics integration
- Performance optimization
- Connection pooling

## Notes
- Focus on establishing controller pattern
- Keep error handling simple but correct
- Document assumptions and limitations
- Ensure clean interfaces for extension