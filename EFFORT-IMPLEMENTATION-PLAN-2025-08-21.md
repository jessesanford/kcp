# Effort E1.1.4 Implementation Plan - Workload Types
Generated: 2025-08-21 04:45:00 UTC
Created by: TMC Orchestrator Planning Agent
Reviewed Phase Plan: PHASE1-SPECIFIC-IMPL-PLAN-8-20-25.md

## Context Analysis

### Completed Efforts in Current Wave
Based on orchestrator-state.yaml:

1. **E1.1.1: api-types-core** (COMPLETED)
   - Split into 6 compliant branches
   - Established core API type infrastructure
   - Created base type definitions and helpers

2. **E1.1.2: synctarget-types** (COMPLETED)
   - Split into 3 branches
   - Implemented SyncTarget and related types
   - Created ClusterWorkspace API resource types

3. **E1.1.3: placement-types** (COMPLETED)
   - Split into 3 branches
   - Implemented Placement, PlacementDecision, PlacementRule types
   - Created scheduling foundation for workload distribution

### Adjustments Based on Progress
1. **Build on placement types**: Since E1.1.3 established placement types, workload types must properly reference and integrate with PlacementReference structures
2. **Leverage existing infrastructure**: Use the base types and helpers from E1.1.1 for consistency
3. **Follow split pattern**: Given all previous efforts required splitting, plan for modular implementation

## Effort Overview
- **Phase**: 1
- **Wave**: 1
- **Effort**: 4
- **Name**: workload-types
- **Base Branch**: main (fresh start, dependencies from placement-types)
- **Working Copy**: /workspaces/efforts/phase1/wave1/effort4-workload-types

## Specific Requirements

1. **Core Workload Types (MUST implement)**:
   - `WorkloadTemplate` - Abstract workload definition with template support
   - `WorkloadPlacement` - Binding between workload and placement rules
   - `WorkloadStatus` - Aggregated status across clusters
   - `WorkloadDistribution` - Strategy for workload distribution

2. **Support Multiple Workload Types**:
   - Deployment support with full spec
   - StatefulSet support with ordering guarantees
   - Job support with completion tracking
   - DaemonSet support (if space permits)

3. **Advanced Features**:
   - Override capabilities for per-cluster customization
   - Resource transformation hooks
   - Status aggregation from multiple clusters
   - Validation webhooks preparation

4. **Integration Requirements**:
   - Must reference PlacementReference from E1.1.3
   - Must use base types from E1.1.1
   - Must follow KCP API conventions

## Implementation Steps

### Step 1: Create Base Structure
1. Create directory structure: `apis/workload/v1alpha1/`
2. Set up package structure with proper imports
3. Create doc.go with package documentation

### Step 2: Implement Core Types
1. Create `workload_types.go` with WorkloadTemplate type
2. Add WorkloadTemplateSpec with manifest field
3. Add WorkloadTemplateStatus with condition tracking

### Step 3: Implement Placement Binding
1. Create `placement_types.go` with WorkloadPlacement type
2. Add PlacementReference structure (reference E1.1.3)
3. Add binding logic between workload and placement

### Step 4: Implement Distribution Strategy
1. Create `distribution_types.go` with WorkloadDistribution type
2. Add strategy enums (Singleton, Replicated, etc.)
3. Add resource quota and limits per cluster

### Step 5: Implement Status Aggregation
1. Create `status_types.go` with WorkloadStatus type
2. Add per-cluster status tracking
3. Add aggregation logic for overall status

### Step 6: Add Transform and Override Support
1. Create `transform_types.go` with transformation rules
2. Create `override_types.go` with override specifications
3. Add JSONPatch support for modifications

### Step 7: Add Validation and Helpers
1. Create `validation.go` with type validation
2. Add helper methods for type conversion
3. Add printer columns and markers

### Step 8: Generate Code and Tests
1. Add kubebuilder markers for CRD generation
2. Run `make generate` for deepcopy
3. Create comprehensive unit tests

## Files to Create/Modify

### Core Type Files
- `apis/workload/v1alpha1/doc.go` - Package documentation
- `apis/workload/v1alpha1/workload_types.go` - WorkloadTemplate types
- `apis/workload/v1alpha1/placement_types.go` - WorkloadPlacement types
- `apis/workload/v1alpha1/distribution_types.go` - WorkloadDistribution types
- `apis/workload/v1alpha1/status_types.go` - WorkloadStatus types

### Support Files
- `apis/workload/v1alpha1/transform_types.go` - Transformation rules
- `apis/workload/v1alpha1/override_types.go` - Override specifications
- `apis/workload/v1alpha1/validation.go` - Validation logic
- `apis/workload/v1alpha1/helpers.go` - Helper functions

### Test Files
- `test/apis/workload_types_test.go` - Core type tests
- `test/apis/workload_validation_test.go` - Validation tests
- `test/apis/workload_helpers_test.go` - Helper function tests

### Generated Files (auto-generated)
- `apis/workload/v1alpha1/zz_generated.deepcopy.go`
- `config/crds/workload.kcp.io_*.yaml`

## Cherry-Pick Instructions
```bash
# No direct cherry-picks needed for this effort
# This is a fresh implementation based on requirements
# Will reference types from placement-types branches as needed
```

## Test Requirements
- **Coverage Target**: 80% minimum
- **Test Categories**:
  1. Type validation tests
  2. Helper function tests
  3. Status aggregation tests
  4. Transform/override tests
  5. Integration with placement types

### Specific Test Cases Required
1. **WorkloadTemplate Tests**:
   - Validate support for multiple workload types
   - Test manifest validation
   - Verify condition tracking

2. **WorkloadPlacement Tests**:
   - Test linking to Placement objects
   - Validate placement reference resolution
   - Test binding lifecycle

3. **WorkloadDistribution Tests**:
   - Validate strategy enum values
   - Test resource quota calculations
   - Verify distribution logic

4. **Status Aggregation Tests**:
   - Test multi-cluster status collection
   - Verify aggregation algorithms
   - Test error condition handling

## Size Constraints
- **Target**: 700 lines of hand-written code
- **Maximum**: 800 lines (measured by tmc-pr-line-counter.sh)
- **Measurement**: Excludes generated files, comments, and blank lines

### Split Strategy if Exceeded
If size exceeds 800 lines, split as follows:
1. **Part 1**: Core types (WorkloadTemplate, WorkloadPlacement) - ~400 lines
2. **Part 2**: Distribution and status types - ~300 lines
3. **Part 3**: Transform, override, and helpers - ~300 lines

## Success Criteria
- [ ] All 4 core workload types implemented (Template, Placement, Status, Distribution)
- [ ] Support for at least 3 workload kinds (Deployment, StatefulSet, Job)
- [ ] Override and transformation capabilities implemented
- [ ] Integration with placement types from E1.1.3
- [ ] Tests achieve 80% coverage minimum
- [ ] Size under 800 lines per tmc-pr-line-counter.sh
- [ ] No hardcoded values or magic strings
- [ ] Follows KCP API conventions and style guide
- [ ] All kubebuilder markers properly configured
- [ ] Generated deepcopy code compiles without errors

## Dependencies
- **Depends on**: 
  - E1.1.1 (api-types-core) - for base types
  - E1.1.2 (synctarget-types) - for cluster references
  - E1.1.3 (placement-types) - for PlacementReference
- **Blocks**: 
  - E1.1.5 (scheduling-types)
  - E1.2.1 (crd-generation)

## Implementation Notes

### Key Design Decisions
1. **Use unstructured.Unstructured for workload templates** to support any Kubernetes resource type
2. **Implement PlacementReference as embedded struct** for consistency with placement-types
3. **Use conditions pattern** for status tracking per KCP conventions
4. **Support JSONPatch** for override specifications

### Integration Points
1. Must import placement types from the correct split branch
2. Reference SyncTarget types for cluster identification
3. Use shared validation utilities from api-types-core

### Risk Mitigation
1. Keep types minimal initially - can extend in later efforts
2. Focus on core functionality over advanced features
3. Ensure clean separation between type categories for easier splitting

## Validation Checklist
Before marking complete:
- [ ] All types have proper JSON/YAML tags
- [ ] DeepCopy methods generated successfully
- [ ] Validation methods return appropriate errors
- [ ] Helper functions have unit tests
- [ ] Documentation comments on all exported types
- [ ] Integration with placement types verified
- [ ] No circular dependencies introduced