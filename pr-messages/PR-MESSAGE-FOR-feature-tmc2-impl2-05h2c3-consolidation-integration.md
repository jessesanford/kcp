<!--

Thanks for creating a pull request!
If this is your first time, please make sure to review CONTRIBUTING.MD.

-->

## Summary

This PR introduces the TMC metrics collectors and registry system as the final part of the TMC Reimplementation Plan 2 consolidation split. The implementation provides concrete metrics collection capabilities with a registry pattern for managing multiple collector instances.

### Key Components Added:

1. **Cluster Metrics Collector** (`pkg/tmc/observability/collector_cluster.go`):
   - Direct cluster metrics collection implementation
   - Workspace-aware metric retrieval from individual clusters
   - Proper error handling and context-based operations
   - Integration with KCP logical cluster concepts

2. **Collector Registry** (`pkg/tmc/observability/collector_registry.go`):
   - Central registry for managing multiple collector instances
   - Dynamic collector registration and lookup
   - Support for different collector types and configurations
   - Thread-safe operations for concurrent access

3. **Feature Flag Integration** (`pkg/features/kcp_features.go`):
   - Complete TMC feature flag definitions
   - Support for TMC APIs, metrics aggregation, and advanced features
   - Proper versioning and progressive rollout support

### Architecture Benefits:
- **Extensible Framework**: Registry pattern allows for multiple collector types
- **Workspace Isolation**: All collectors respect KCP workspace boundaries
- **Concurrent Safe**: Thread-safe operations for production use
- **Error Resilient**: Proper error handling throughout collection pipeline
- **Integration Ready**: Works with aggregation core from previous PR

### Collection Flow:
1. Collectors register with the central registry
2. Registry manages collector lifecycle and configuration
3. Aggregation core uses registry to find appropriate collectors
4. Collectors retrieve metrics from workspace-specific clusters
5. Results flow back through aggregation pipeline

This completes the three atomic PRs that split the original 1704-line consolidation implementation into manageable pieces, with total coverage of the original functionality.

## What Type of PR Is This?

/kind feature

## Related Issue(s)

Part of TMC Reimplementation Plan 2 - Integration Components Phase

## Release Notes

```
Add TMC metrics collectors and registry system for workspace-aware metrics collection. Includes cluster-specific collectors and a central registry for managing multiple collector instances. Completes the TMC observability foundation with extensible collection framework.
```