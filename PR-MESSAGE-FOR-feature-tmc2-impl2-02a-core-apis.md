<!--

Thanks for creating a pull request!
If this is your first time, please make sure to review CONTRIBUTING.MD.

-->

## Summary

This PR introduces the core TMC (Topology Management Controller) API foundation by implementing ClusterRegistration and WorkloadPlacement APIs. These APIs form the essential building blocks for multi-cluster workload placement and management within KCP's workspace hierarchy.

**Key Changes:**
- **ClusterRegistration API**: Defines how physical clusters register with TMC, including location, connection endpoints, capacity information, and health tracking
- **WorkloadPlacement API**: Provides basic workload placement policies with workload selectors, cluster selectors, and placement strategies (RoundRobin, LeastLoaded, Random, LocationAware)
- **Shared Types**: Common selector types, workload references, and placement decision structures used across TMC APIs
- **Comprehensive Testing**: Full test coverage for all API types with validation scenarios
- **Generated Code**: Proper deepcopy generation and CRD generation following KCP patterns

**Technical Details:**
- **Package Structure**: Created `pkg/apis/tmc/v1alpha1/` following KCP API conventions
- **API Design**: Follows Kubernetes API patterns with proper TypeMeta, ObjectMeta, Spec, and Status structures
- **KCP Integration**: Uses KCP-specific conditions library and workspace-aware design patterns
- **Resource Scoping**: ClusterRegistration is cluster-scoped, WorkloadPlacement is namespace-scoped
- **Extensibility**: Designed for future enhancement with advanced placement features

**API Overview:**
- `ClusterRegistration`: Manages cluster lifecycle, capacity tracking, and health monitoring
- `WorkloadPlacement`: Defines placement policies for workloads across selected clusters
- Shared types for selectors, references, and placement decisions provide consistency

This foundation enables external TMC controllers to discover clusters, make placement decisions, and track workload deployment status across multiple Kubernetes clusters.

## What Type of PR Is This?

/kind feature
/kind api-change

## Related Issue(s)

Implements core TMC API foundation as part of TMC Reimplementation Plan 2.

## Release Notes

```
Add core TMC APIs for multi-cluster workload placement including ClusterRegistration for cluster management and WorkloadPlacement for placement policies. These APIs provide the foundation for TMC's multi-cluster orchestration capabilities within KCP workspaces.
```