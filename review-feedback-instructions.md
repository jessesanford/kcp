# Review Feedback Instructions - Wave 2A-02: ResourceQuota Controller

## Current State
- Branch: `feature/tmc2-impl2/phase2/wave2a-02-split-from-controller`
- Focus: ResourceQuota controller for compute management
- Estimated current lines: ~280 lines

## Priority Issues (P0 - Must Fix)

### 1. Missing Test Coverage
**CRITICAL**: No tests for quota controller

#### Required Test Files to Create:
1. `pkg/reconciler/workload/resourcequota/resourcequota_controller_test.go` (~200 lines)
   - Test quota calculation
   - Test usage aggregation
   - Test enforcement logic

2. `pkg/reconciler/workload/resourcequota/aggregator_test.go` (~150 lines)
   - Test resource aggregation
   - Test multi-namespace quotas

### 2. Complete Quota Controller Implementation

#### File: `pkg/reconciler/workload/resourcequota/resourcequota_controller.go`
Implement quota enforcement (~180 lines):
```go
// Reconcile handles ResourceQuota reconciliation
func (c *Controller) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    logger := logging.WithReconciler(klog.FromContext(ctx), ControllerName)
    ctx = klog.NewContext(ctx, logger)
    
    // Get ResourceQuota
    quota := &workloadv1alpha1.ResourceQuota{}
    if err := c.Get(ctx, req.NamespacedName, quota); err != nil {
        if apierrors.IsNotFound(err) {
            return ctrl.Result{}, nil
        }
        return ctrl.Result{}, err
    }
    
    // Calculate current usage
    usage, err := c.calculateUsage(ctx, quota)
    if err != nil {
        return ctrl.Result{}, err
    }
    
    // Update quota status
    quota.Status.Used = usage
    quota.Status.LastUpdated = metav1.Now()
    
    // Check for violations
    violations := c.checkViolations(quota)
    if len(violations) > 0 {
        c.updateViolationStatus(quota, violations)
    } else {
        c.clearViolationStatus(quota)
    }
    
    // Update status
    if err := c.Status().Update(ctx, quota); err != nil {
        return ctrl.Result{}, err
    }
    
    // Requeue for periodic recalculation
    return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
}

// calculateUsage aggregates resource usage
func (c *Controller) calculateUsage(ctx context.Context, quota *workloadv1alpha1.ResourceQuota) (corev1.ResourceList, error) {
    usage := make(corev1.ResourceList)
    
    // List pods in namespace
    podList := &corev1.PodList{}
    if err := c.List(ctx, podList, client.InNamespace(quota.Namespace)); err != nil {
        return nil, fmt.Errorf("failed to list pods: %w", err)
    }
    
    // Aggregate pod resources
    for _, pod := range podList.Items {
        // Skip terminated pods
        if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
            continue
        }
        
        // Add container requests
        for _, container := range pod.Spec.Containers {
            for name, quantity := range container.Resources.Requests {
                if current, exists := usage[name]; exists {
                    current.Add(quantity)
                    usage[name] = current
                } else {
                    usage[name] = quantity
                }
            }
        }
    }
    
    // Add PVC usage
    pvcList := &corev1.PersistentVolumeClaimList{}
    if err := c.List(ctx, pvcList, client.InNamespace(quota.Namespace)); err != nil {
        return nil, fmt.Errorf("failed to list PVCs: %w", err)
    }
    
    for _, pvc := range pvcList.Items {
        if pvc.Status.Phase != corev1.ClaimBound {
            continue
        }
        
        if storage, exists := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; exists {
            if current, exists := usage[corev1.ResourceStorage]; exists {
                current.Add(storage)
                usage[corev1.ResourceStorage] = current
            } else {
                usage[corev1.ResourceStorage] = storage
            }
        }
    }
    
    return usage, nil
}

// checkViolations identifies quota violations
func (c *Controller) checkViolations(quota *workloadv1alpha1.ResourceQuota) []string {
    var violations []string
    
    for resourceName, hardLimit := range quota.Spec.Hard {
        if used, exists := quota.Status.Used[resourceName]; exists {
            if used.Cmp(hardLimit) > 0 {
                violations = append(violations, fmt.Sprintf(
                    "%s: used %s exceeds limit %s",
                    resourceName,
                    used.String(),
                    hardLimit.String(),
                ))
            }
        }
    }
    
    return violations
}
```

### 3. Add Admission Webhook Support

#### File: `pkg/reconciler/workload/resourcequota/admission.go` (NEW ~120 lines)
```go
package resourcequota

// AdmissionController validates resource requests against quotas
type AdmissionController struct {
    client client.Client
}

// ValidateCreate checks if resource creation would exceed quota
func (a *AdmissionController) ValidateCreate(ctx context.Context, obj runtime.Object) error {
    // Extract resource requirements
    resources := a.extractResources(obj)
    if resources == nil {
        return nil // No resources to check
    }
    
    // Get applicable quotas
    quotas, err := a.getApplicableQuotas(ctx, obj)
    if err != nil {
        return err
    }
    
    // Check each quota
    for _, quota := range quotas {
        if err := a.checkQuota(quota, resources); err != nil {
            return err
        }
    }
    
    return nil
}
```

## Line Count Analysis

### Current Estimate:
- Existing code: ~280 lines
- Required tests: ~350 lines
- Complete implementation: ~180 lines
- Admission support: ~120 lines
- **Total after fixes: ~930 lines** ❌ OVER LIMIT

### NEEDS SPLIT Strategy:
Split into 2 PRs:
1. **Current PR**: Core quota controller (~450 lines)
2. **Follow-up PR**: Admission webhook + comprehensive tests (~500 lines)

## Specific Tasks for THIS PR

### 1. Focus on Core Quota Logic
Implement only:
- Basic reconciliation
- Usage calculation
- Status updates
- Simple violation detection

### 2. Create Basic Test Coverage
```go
func TestQuotaUsageCalculation(t *testing.T) {
    scheme := runtime.NewScheme()
    _ = workloadv1alpha1.AddToScheme(scheme)
    _ = corev1.AddToScheme(scheme)
    
    quota := &workloadv1alpha1.ResourceQuota{
        ObjectMeta: metav1.ObjectMeta{
            Name:      "test-quota",
            Namespace: "default",
        },
        Spec: workloadv1alpha1.ResourceQuotaSpec{
            Hard: corev1.ResourceList{
                corev1.ResourceCPU:    resource.MustParse("10"),
                corev1.ResourceMemory: resource.MustParse("10Gi"),
            },
        },
    }
    
    pod := &corev1.Pod{
        ObjectMeta: metav1.ObjectMeta{
            Name:      "test-pod",
            Namespace: "default",
        },
        Spec: corev1.PodSpec{
            Containers: []corev1.Container{{
                Name: "test",
                Resources: corev1.ResourceRequirements{
                    Requests: corev1.ResourceList{
                        corev1.ResourceCPU:    resource.MustParse("1"),
                        corev1.ResourceMemory: resource.MustParse("1Gi"),
                    },
                },
            }},
        },
        Status: corev1.PodStatus{
            Phase: corev1.PodRunning,
        },
    }
    
    client := fake.NewClientBuilder().
        WithScheme(scheme).
        WithObjects(quota, pod).
        Build()
    
    controller := &Controller{Client: client}
    
    usage, err := controller.calculateUsage(context.TODO(), quota)
    require.NoError(t, err)
    require.Equal(t, "1", usage[corev1.ResourceCPU].String())
}
```

### 3. Defer to Follow-up PR
- Admission webhook
- Comprehensive test suite
- Scope selectors
- Advanced aggregation
- Metrics

## Testing Requirements (This PR)

### Unit Test Coverage Target: 65%
1. **Usage Calculation Tests**:
   - Pod resource aggregation
   - PVC storage counting
   - Terminated pod exclusion

2. **Violation Detection Tests**:
   - Over-limit detection
   - Status updates

## Completion Checklist (This PR)

- [ ] Core reconciliation complete
- [ ] Usage calculation working
- [ ] Basic tests (65% coverage)
- [ ] Violation detection implemented
- [ ] Status updates functional
- [ ] `make test` passes
- [ ] Line count < 700 lines
- [ ] TODO for admission webhook
- [ ] Clean commit history

## Follow-up PR Planning
Create Wave 2A-02b for:
- Admission webhook implementation
- Comprehensive test coverage
- Scope selector support
- Performance optimization

## Notes
- Keep quota logic simple and correct
- Focus on accuracy over performance initially
- Document quota calculation methodology
- Ensure extensibility for admission control