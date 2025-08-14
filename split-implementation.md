# Split Implementation: Wave2b-03 - Transformation & Tests

## Overview
**Branch:** `feature/tmc-syncer-02b-transform`  
**Target Size:** ~499 lines  
**Dependencies:** Wave2b-01 (Virtual Base) must be complete  
**Can Run In Parallel:** Yes, with Wave2b-02 after Wave2b-01

## Files to Copy

These files should be copied from `/workspaces/kcp-worktrees/phase2/wave2b-virtual-to-be-split/`:

### 1. **pkg/virtual/syncer/transformation.go** (229 lines)
Resource transformation between virtual and physical representations:
- Virtual to physical resource conversion
- Physical to virtual resource conversion
- Metadata transformation
- Status reconciliation

### 2. **pkg/virtual/syncer/virtual_workspace_test.go** (~270 lines)
Comprehensive tests for the virtual workspace implementation:
- Virtual workspace creation tests
- Discovery mechanism tests
- Authentication tests
- Storage operation tests
- Transformation tests

## Implementation Checklist

### Pre-Implementation
- [ ] Ensure Wave2b-01 is available
- [ ] Virtual workspace foundation exists
- [ ] Review transformation requirements

### Implementation Steps

1. **Verify Prerequisites**
   ```bash
   # Check virtual workspace foundation
   ls -la pkg/virtual/syncer/virtual_workspace.go
   ls -la pkg/virtual/syncer/discovery.go
   ```

2. **Copy Transformation Logic**
   ```bash
   cp /workspaces/kcp-worktrees/phase2/wave2b-virtual-to-be-split/pkg/virtual/syncer/transformation.go \
      pkg/virtual/syncer/transformation.go
   ```

3. **Copy Test File**
   ```bash
   cp /workspaces/kcp-worktrees/phase2/wave2b-virtual-to-be-split/pkg/virtual/syncer/virtual_workspace_test.go \
      pkg/virtual/syncer/virtual_workspace_test.go
   ```

4. **Review Transformation Implementation**
   Key components:
   ```go
   // Transformer interface
   type Transformer interface {
       VirtualToPhysical(virtual runtime.Object) (runtime.Object, error)
       PhysicalToVirtual(physical runtime.Object) (runtime.Object, error)
   }
   
   // SyncTarget transformer
   type SyncTargetTransformer struct {
       workspace logicalcluster.Path
       mapper    meta.RESTMapper
   }
   ```

5. **Key Transformation Operations**
   - **Namespace Mapping**: Virtual namespace to physical namespace
   - **Name Translation**: Virtual names to physical names
   - **Label Injection**: Add workspace labels
   - **Annotation Handling**: Preserve/transform annotations
   - **Status Mapping**: Map physical status to virtual

6. **Test Coverage Areas**
   Ensure tests cover:
   - Virtual workspace initialization
   - Discovery registration
   - Authentication flows
   - Storage CRUD operations
   - Transformation bidirectionality
   - Error handling

### Transformation Logic Details

#### Virtual to Physical
```go
func (t *SyncTargetTransformer) VirtualToPhysical(virtual runtime.Object) (runtime.Object, error) {
    // 1. Type assertion
    vTarget, ok := virtual.(*workloadv1alpha1.SyncTarget)
    
    // 2. Create physical representation
    physical := &corev1.ConfigMap{
        ObjectMeta: metav1.ObjectMeta{
            Name:      physicalName(vTarget.Name),
            Namespace: physicalNamespace(t.workspace),
        },
    }
    
    // 3. Transform data
    physical.Data = transformToConfigMapData(vTarget)
    
    // 4. Add workspace labels
    physical.Labels = map[string]string{
        "kcp.io/workspace": t.workspace.String(),
    }
    
    return physical, nil
}
```

#### Physical to Virtual
```go
func (t *SyncTargetTransformer) PhysicalToVirtual(physical runtime.Object) (runtime.Object, error) {
    // 1. Type assertion
    pConfigMap, ok := physical.(*corev1.ConfigMap)
    
    // 2. Create virtual representation
    virtual := &workloadv1alpha1.SyncTarget{
        ObjectMeta: metav1.ObjectMeta{
            Name: virtualName(pConfigMap.Name),
        },
    }
    
    // 3. Transform data back
    virtual.Spec = transformFromConfigMapData(pConfigMap.Data)
    
    // 4. Map status
    virtual.Status = deriveStatus(pConfigMap)
    
    return virtual, nil
}
```

### Test Implementation Structure

```go
func TestVirtualWorkspace(t *testing.T) {
    tests := []struct {
        name string
        test func(t *testing.T)
    }{
        {"Creation", testVirtualWorkspaceCreation},
        {"Discovery", testDiscoveryMechanism},
        {"Authentication", testAuthentication},
        {"Storage", testStorageOperations},
        {"Transformation", testTransformation},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, tt.test)
    }
}
```

### Validation Steps

1. **Run Tests**
   ```bash
   go test ./pkg/virtual/syncer/... -v
   ```

2. **Test Transformation**
   ```go
   // Quick validation
   transformer := NewSyncTargetTransformer(workspace, mapper)
   physical, err := transformer.VirtualToPhysical(virtualObj)
   virtual, err := transformer.PhysicalToVirtual(physical)
   // Verify round-trip works
   ```

3. **Check Line Count**
   ```bash
   /workspaces/kcp-shared-tools/tmc-pr-line-counter.sh -c feature/tmc-syncer-02b-transform
   ```
   Should be ~499 lines

4. **Integration Test**
   ```bash
   # Test end-to-end with virtual workspace
   kubectl --context system:admin get synctargets --virtual
   ```

### Commit Strategy

```bash
# Stage transformation logic
git add pkg/virtual/syncer/transformation.go
git commit -s -S -m "feat(virtual): add resource transformation for virtual workspace

- Implement bidirectional transformation
- Handle virtual to physical conversion
- Map status between representations
- Ensure workspace isolation in transformations"

# Stage tests
git add pkg/virtual/syncer/virtual_workspace_test.go
git commit -s -S -m "test: add comprehensive tests for virtual workspace

- Test virtual workspace creation
- Validate discovery mechanism
- Test authentication flows
- Verify storage operations
- Test transformation logic"
```

### Post-Implementation
- [ ] All tests pass
- [ ] Transformation is bidirectional
- [ ] No data loss in transformation
- [ ] Line count ~499
- [ ] Good test coverage (>80%)
- [ ] Push branch and create PR

## Success Criteria

1. ✅ Transformation preserves all data
2. ✅ Round-trip transformation works
3. ✅ Tests achieve >80% coverage
4. ✅ No workspace data leakage
5. ✅ ~499 lines total
6. ✅ All edge cases handled

## Potential Issues & Solutions

1. **Transformation Data Loss**
   - Ensure all fields are mapped
   - Test round-trip conversions
   - Validate against schema

2. **Test Failures**
   - May need mocks for some components
   - Check test isolation
   - Verify test data setup

3. **Performance Issues**
   - Cache transformation results if needed
   - Optimize serialization
   - Consider batch operations

## Dependencies

- **Requires:** Wave2b-01 (Virtual Base)
- **Can Parallel With:** Wave2b-02 (Auth & Storage)
- **Validates:** Entire virtual workspace implementation

## Notes for Parallel Agents

- Can work simultaneously with Wave2b-02
- Tests validate the entire virtual workspace
- Transformation is critical for correctness
- Must maintain data integrity