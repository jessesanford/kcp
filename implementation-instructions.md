# Implementation Instructions: Transformation Pipeline

## Branch: `feature/phase7-syncer-impl/p7w1-transform`

## Overview
This branch implements the transformation pipeline that modifies resources as they move between KCP and physical clusters. It handles namespace mapping, label/annotation management, secret sanitization, and other critical transformations required for multi-cluster synchronization.

**Target Size**: ~650 lines  
**Complexity**: Medium  
**Priority**: Critical (required by Wave 2 downstream sync)

## Dependencies
- **Phase 5 APIs**: Uses transformation interfaces from `pkg/apis/syncer/v1alpha1`
- **Phase 6 Infrastructure**: Leverages workspace context
- **Wave 1 Sync Engine**: Will be registered with the engine

## Files to Create

### 1. Pipeline Core (~150 lines)
**File**: `pkg/reconciler/workload/syncer/transformation/pipeline.go`
- Pipeline struct and initialization
- Transform execution chain
- Transformer registration
- Error handling and recovery

### 2. Namespace Transformer (~100 lines)
**File**: `pkg/reconciler/workload/syncer/transformation/namespace.go`
- Namespace prefix/suffix handling
- Workspace-based namespace mapping
- Reverse transformation for upstream

### 3. Label & Annotation Transformer (~100 lines)
**File**: `pkg/reconciler/workload/syncer/transformation/metadata.go`
- Label injection/removal
- Annotation management
- Metadata preservation rules

### 4. Secret Transformer (~100 lines)
**File**: `pkg/reconciler/workload/syncer/transformation/secret.go`
- Secret type filtering
- Service account token handling
- Docker config validation
- Sensitive data sanitization

### 5. Owner Reference Transformer (~80 lines)
**File**: `pkg/reconciler/workload/syncer/transformation/ownership.go`
- Owner reference adjustment
- Cross-cluster reference handling
- Garbage collection coordination

### 6. Transformation Tests (~120 lines)
**File**: `pkg/reconciler/workload/syncer/transformation/pipeline_test.go`
- Unit tests for each transformer
- Pipeline integration tests
- Edge case coverage

## Step-by-Step Implementation Guide

### Step 1: Create Package Structure
```bash
mkdir -p pkg/reconciler/workload/syncer/transformation
```

### Step 2: Define Pipeline Core
Create `pipeline.go` with:

```go
package transformation

import (
    "fmt"
    
    workloadv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"
    "github.com/kcp-dev/kcp/pkg/syncer/interfaces"
    
    "k8s.io/apimachinery/pkg/runtime"
    "github.com/kcp-dev/logicalcluster/v3"
)

// Pipeline coordinates resource transformations
type Pipeline struct {
    transformers []interfaces.ResourceTransformer
    workspace    logicalcluster.Name
}

// NewPipeline creates a transformation pipeline with default transformers
func NewPipeline(workspace logicalcluster.Name) *Pipeline {
    return &Pipeline{
        workspace: workspace,
        transformers: []interfaces.ResourceTransformer{
            NewNamespaceTransformer(workspace),
            NewMetadataTransformer(),
            NewOwnerReferenceTransformer(),
            NewSecretTransformer(),
        },
    }
}

// TransformForDownstream applies all transformations for downstream sync
func (p *Pipeline) TransformForDownstream(obj runtime.Object, target *workloadv1alpha1.SyncTarget) (runtime.Object, error) {
    result := obj.DeepCopyObject()
    
    for _, transformer := range p.transformers {
        if !transformer.ShouldTransform(result) {
            continue
        }
        
        var err error
        result, err = transformer.TransformForDownstream(result, target)
        if err != nil {
            return nil, fmt.Errorf("transformer %T failed: %w", transformer, err)
        }
    }
    
    return result, nil
}

// TransformForUpstream reverses transformations for upstream sync
func (p *Pipeline) TransformForUpstream(obj runtime.Object, source *workloadv1alpha1.SyncTarget) (runtime.Object, error) {
    result := obj.DeepCopyObject()
    
    // Apply transformers in reverse order
    for i := len(p.transformers) - 1; i >= 0; i-- {
        transformer := p.transformers[i]
        if !transformer.ShouldTransform(result) {
            continue
        }
        
        var err error
        result, err = transformer.TransformForUpstream(result, source)
        if err != nil {
            return nil, fmt.Errorf("transformer %T failed: %w", transformer, err)
        }
    }
    
    return result, nil
}

// RegisterTransformer adds a custom transformer
func (p *Pipeline) RegisterTransformer(t interfaces.ResourceTransformer) {
    p.transformers = append(p.transformers, t)
}
```

### Step 3: Implement Namespace Transformer
Create `namespace.go` with:

1. **Namespace mapping logic**:
   - Add workspace prefix to namespaces
   - Handle special namespaces (kube-system, etc.)
   - Maintain mapping for reverse transformation

2. **Transform methods**:
   - TransformForDownstream: Add prefix
   - TransformForUpstream: Remove prefix
   - ShouldTransform: Check if namespaced

3. **Configuration**:
   - Configurable prefix pattern
   - Exclusion list for system namespaces
   - Collision detection

### Step 4: Implement Metadata Transformer
Create `metadata.go` with:

1. **Label management**:
   - Add TMC management labels
   - Add workspace identifier
   - Add sync target reference
   - Preserve user labels

2. **Annotation handling**:
   - Add sync timestamp
   - Add generation tracking
   - Remove KCP-internal annotations
   - Preserve important annotations

3. **Filtering logic**:
   - System label preservation
   - Annotation allowlist/blocklist
   - Size limit enforcement

### Step 5: Implement Secret Transformer
Create `secret.go` with:

1. **Secret type handling**:
   - Filter service account tokens
   - Validate docker configs
   - Handle TLS certificates
   - Process opaque secrets

2. **Sanitization**:
   - Remove sensitive annotations
   - Validate secret data
   - Check size limits
   - Ensure proper encoding

3. **Security checks**:
   - Prevent token leakage
   - Validate certificate chains
   - Check for embedded credentials

### Step 6: Implement Owner Reference Transformer
Create `ownership.go` with:

1. **Reference adjustment**:
   - Update UIDs for cross-cluster
   - Handle missing owners
   - Prevent orphaning

2. **Garbage collection**:
   - Maintain proper chains
   - Handle cascade deletion
   - Coordinate with finalizers

### Step 7: Add Comprehensive Tests
Create `pipeline_test.go` and related test files:

1. **Pipeline tests**:
   - Test transformation order
   - Test error propagation
   - Test custom transformer registration

2. **Individual transformer tests**:
   - Namespace mapping accuracy
   - Label/annotation preservation
   - Secret sanitization
   - Owner reference integrity

3. **Integration tests**:
   - Full pipeline execution
   - Bi-directional transformation
   - Edge cases and errors

## Testing Requirements

### Unit Tests:
- Each transformer's transform methods
- ShouldTransform logic
- Error handling
- Edge cases (nil objects, missing fields)
- Bi-directional transformation consistency

### Integration Tests:
- Full pipeline with all transformers
- Complex resource types
- Large objects
- Performance under load

## Validation Checklist

- [ ] All transformers implement the interface correctly
- [ ] Bi-directional transformations are symmetric
- [ ] No data loss during transformation
- [ ] Proper error handling and reporting
- [ ] Performance is acceptable (<10ms per transform)
- [ ] Security considerations addressed
- [ ] Comprehensive logging
- [ ] Tests achieve >80% coverage
- [ ] Code follows KCP patterns
- [ ] Feature flags integrated
- [ ] Under 650 lines (excluding tests)

## Common Pitfalls to Avoid

1. **Don't modify the original object** - always deep copy first
2. **Preserve round-trip integrity** - transformations should be reversible
3. **Handle nil/empty cases** - don't assume fields exist
4. **Watch for size limits** - transformed objects might exceed limits
5. **Maintain security** - never expose sensitive data

## Integration Notes

This pipeline will be:
- Registered with the sync engine from Wave 1
- Used by downstream syncer in Wave 2
- Applied to upstream status in Wave 3

The pipeline should:
- Be stateless and thread-safe
- Support dynamic transformer registration
- Provide transformation metrics
- Log all transformations at debug level

## Success Criteria

The implementation is complete when:
1. All default transformers are implemented
2. Pipeline can execute transformations in order
3. Bi-directional transformations work correctly
4. No data loss or corruption occurs
5. All tests pass
6. Performance meets requirements (<10ms per object)