<!--

Thanks for creating a pull request!
If this is your first time, please make sure to review CONTRIBUTING.MD.

-->

## Summary

This PR introduces the TMC metrics aggregation core functionality as the second part of the TMC Reimplementation Plan 2. The implementation provides comprehensive metrics aggregation capabilities across multiple clusters with workspace awareness and feature flag protection.

### Key Components Added:

1. **Aggregation Core Implementation** (`pkg/tmc/observability/aggregation_core.go`):
   - MetricsAggregator interface and implementation
   - Support for multiple aggregation strategies (sum, avg, max, min)
   - Workspace-aware metrics collection from multiple clusters
   - Feature flag integration for progressive enablement

2. **Aggregation Types and Interfaces** (`pkg/tmc/observability/aggregation_types.go`):
   - Comprehensive data structures for aggregated metrics
   - Time series support interfaces for future consolidation
   - Strategy definitions and validation
   - Proper error handling types

3. **Feature Flag Extensions** (`pkg/features/kcp_features.go`):
   - TMCMetricsAggregation for basic aggregation features  
   - TMCAdvancedAggregation for complex strategies
   - Proper versioning and gate management

### Key Features:
- **Multi-Strategy Aggregation**: Sum, average, maximum, and minimum strategies
- **Workspace Isolation**: Metrics collection respects KCP workspace boundaries
- **Feature Flag Protection**: Basic and advanced aggregation behind separate flags
- **Error Resilience**: Continues aggregation even if some clusters fail
- **Extensible Design**: Ready for time-series consolidation in future PRs

### Integration Points:
- Works with KCP logical cluster concepts
- Integrates with feature gate system
- Supports multiple metrics source implementations
- Ready for collector integration

This is the second of three atomic PRs that split the original 1704-line consolidation implementation into manageable pieces.

## What Type of PR Is This?

/kind feature

## Related Issue(s)

Part of TMC Reimplementation Plan 2 - Observability Core Phase

## Release Notes

```  
Add TMC metrics aggregation core with support for sum, average, max, and min strategies across multiple clusters. Includes workspace-aware metrics collection, feature flag protection, and comprehensive data types. Foundation for advanced observability features in TMC workload management.
```