<!--

Thanks for creating a pull request!
If this is your first time, please make sure to review CONTRIBUTING.MD.

-->

## Summary

This PR implements the WorkloadPlacement API types as part 2 of the 3-part TMC API split from impl4-03. It provides:

- **WorkloadPlacement CRD**: Core API for defining workload placement policies across clusters
- **Comprehensive Type System**: Including PlacementDecision, PlacedWorkload, and placement status types  
- **Shared Types**: WorkloadSelector, ClusterSelector, and WorkloadReference for cross-cluster operations
- **Helper Functions**: Utility functions for API type manipulation
- **Full Test Coverage**: 96% test coverage with placement-specific test scenarios

## What Type of PR Is This?

/kind feature

## Related Issue(s)

Part of TMC Reimplementation Plan 2 - API type extraction and modularization.

## Release Notes

```
Added WorkloadPlacement API types for TMC (Topology Management Controller) enabling workload placement policies across multiple Kubernetes clusters within KCP workspace hierarchy.
```