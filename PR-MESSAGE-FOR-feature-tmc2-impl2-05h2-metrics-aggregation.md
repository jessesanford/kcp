<!--

Thanks for creating a pull request!
If this is your first time, please make sure to review CONTRIBUTING.MD.

-->

## Summary

This PR implements comprehensive metrics aggregation functionality for TMC (Transparent Multi-Cluster), enabling cross-cluster metric collection, aggregation, and time-series consolidation with full KCP workspace awareness.

## What Type of PR Is This?

/kind feature

## Related Issue(s)

Implements TMC metrics aggregation as part of the TMC reimplementation plan phase 5h2.

## Implementation Details

### Core Components Added

1. **MetricsAggregator Interface**: Defines contract for cross-cluster metrics aggregation
2. **WorkspaceAwareMetricsCollector**: KCP-integrated metrics collection with workspace isolation  
3. **Aggregation Strategies**: Sum, average, maximum, minimum aggregation across clusters
4. **Time-Series Consolidation**: Deduplication, gap-filling, and normalization of time-series data
5. **Feature Flag Integration**: Comprehensive feature gate support for controlled rollout

### Key Features

- **Cross-cluster aggregation**: Aggregate metrics from multiple clusters within a workspace
- **Multiple strategies**: Support for sum, avg, max, min aggregation strategies  
- **Time-series support**: Consolidate time-series data with configurable intervals
- **Workspace isolation**: Full KCP logical cluster awareness and isolation
- **Performance optimized**: Caching layer for cluster metadata and metrics
- **Feature gated**: Protected by TMCMetricsAggregation, TMCAdvancedAggregation, and TMCTimeSeriesConsolidation flags

### Architecture Integration

- Integrates with existing MetricsManager for prometheus metrics collection
- Uses KCP cluster client for workspace-aware cluster discovery
- Maintains separation of concerns with collector and aggregator interfaces
- Follows KCP patterns for workspace isolation and logical cluster handling

## Testing

- Comprehensive unit tests for all aggregation strategies
- Mock-based testing for workspace-aware collection
- Feature gate validation tests
- Time-series consolidation tests with gap filling scenarios
- Performance benchmarks for large cluster counts

## Release Notes

```markdown
TMC now supports metrics aggregation across multiple clusters within workspaces.

New features:
- Cross-cluster metrics aggregation with sum, avg, max, min strategies
- Time-series consolidation with automatic gap filling
- Workspace-aware metrics collection through KCP integration
- Performance-optimized caching for cluster metrics

Feature flags:
- TMCMetricsAggregation: Enable basic metrics aggregation
- TMCAdvancedAggregation: Enable advanced aggregation strategies (avg, max, min)  
- TMCTimeSeriesConsolidation: Enable time-series data consolidation

This functionality is disabled by default and requires feature flag activation.
```