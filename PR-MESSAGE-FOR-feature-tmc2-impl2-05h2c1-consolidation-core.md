<!--

Thanks for creating a pull request!
If this is your first time, please make sure to review CONTRIBUTING.MD.

-->

## Summary

This PR introduces the foundational TMC controller infrastructure and cluster registration system as part of the TMC Reimplementation Plan 2. The implementation provides a solid base for TMC functionality while maintaining KCP architectural patterns and workspace isolation.

### Key Components Added:

1. **TMC Controller Foundation** (`pkg/tmc/controller/foundation.go`):
   - Base controller infrastructure following KCP patterns
   - Health checking, work queue management, and informer integration
   - Designed for extensibility with specific TMC controllers

2. **Cluster Registration Controller** (`pkg/tmc/controller/clusterregistration.go`):
   - Manages cluster lifecycle within TMC workspaces
   - Workspace-aware cluster operations with proper isolation
   - Foundation for advanced cluster management features

3. **TMC Controller Binary** (`cmd/tmc-controller/main.go`):
   - Dedicated binary for TMC controller operations
   - Proper feature flag integration and configuration
   - Ready for deployment in KCP environments

4. **Feature Flag Updates** (`pkg/features/kcp_features.go`):
   - TMC-specific feature flags for progressive rollout
   - Support for TMC APIs, metrics aggregation, and advanced features

### Architecture Highlights:
- **Workspace Isolation**: All components respect KCP workspace boundaries
- **Feature Flag Gating**: All functionality is behind appropriate feature flags
- **Extensible Design**: Foundation supports future TMC controller additions
- **KCP Patterns**: Follows established KCP conventions for consistency

This is the first of three atomic PRs that split the original 1704-line consolidation implementation into manageable pieces.

## What Type of PR Is This?

/kind feature

## Related Issue(s)

Part of TMC Reimplementation Plan 2 - Controller Foundation Phase

## Release Notes

```
Add TMC controller foundation and cluster registration system. This provides the base infrastructure for TMC workload management with proper KCP integration, workspace isolation, and feature flag support. The foundation enables future TMC controllers while maintaining established KCP architectural patterns.
```