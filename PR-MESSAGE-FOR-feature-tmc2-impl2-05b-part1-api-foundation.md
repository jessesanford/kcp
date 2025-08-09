<!--

Thanks for creating a pull request!
If this is your first time, please make sure to review CONTRIBUTING.MD.

-->

## Summary

This PR introduces the foundational TMC (Tanzu Mission Control) API types as part of splitting the oversized cluster controller implementation into 10 atomic PRs. This is **Part 1/10** of the split, focusing exclusively on the core API type definitions.

**Key Components:**
- **ClusterRegistration API**: Core API for managing cluster lifecycle and registration
- **WorkloadPlacement API**: API for defining workload placement policies and constraints  
- **Shared Types**: Common types used across TMC APIs
- **Code Generation**: Infrastructure for generating client code and deepcopy methods

## What Type of PR Is This?

/kind feature
/kind api-change

## Related Issue(s)

Part of TMC Reimplementation Plan 2 - Phase 1 Critical Blockers
Addresses oversized PR issue: Original branch was 6,578 lines (839% over target)

## Technical Details

**Size Metrics:**
- Hand-written lines: 511 (27% under 700-line target)
- Generated lines: 472 (deepcopy, not counted toward limit)
- Test coverage: 0% (API-only foundation, tests in subsequent PRs)

**API Design:**
- Follows KCP architectural patterns with workspace isolation
- Implements proper condition handling and status reporting
- Includes comprehensive field validation and documentation
- Uses standard Kubernetes API conventions

**Files Added:**
- `pkg/apis/tmc/v1alpha1/types_cluster.go` - ClusterRegistration API (168 lines)
- `pkg/apis/tmc/v1alpha1/types_placement.go` - WorkloadPlacement API (136 lines)  
- `pkg/apis/tmc/v1alpha1/types_shared.go` - Shared types (116 lines)
- `pkg/apis/tmc/v1alpha1/doc.go` - Package documentation (31 lines)
- `pkg/apis/tmc/v1alpha1/register.go` - Scheme registration (60 lines)
- `hack/update-tmc-codegen.sh` - Code generation script (76 lines)
- `pkg/apis/tmc/v1alpha1/zz_generated.deepcopy.go` - Generated deepcopy (472 lines)

## Dependencies

This PR establishes the foundation that subsequent PRs will build upon:
- Part 2: Client generation infrastructure
- Part 3: Controller base structure  
- Part 4: Reconciler core logic
- Parts 5-10: Capabilities, discovery, informers, CRDs, integration

## Breaking Changes

None - this is new functionality.

## Release Notes

```
Add TMC API foundation with ClusterRegistration and WorkloadPlacement types

The Tanzu Mission Control (TMC) feature introduces comprehensive cluster management
capabilities to KCP. This initial PR provides the core API types that enable:

- Cluster registration and lifecycle management
- Workload placement policies and constraints
- Multi-cluster resource scheduling
- Status aggregation across clusters

This is the first of 10 atomic PRs that together implement the complete TMC feature
set while maintaining reviewability and quality standards.
```