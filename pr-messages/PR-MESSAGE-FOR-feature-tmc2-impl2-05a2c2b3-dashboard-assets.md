<!--

Thanks for creating a pull request!
If this is your first time, please make sure to review CONTRIBUTING.MD.

-->

## Summary

Implements TMC dashboard observability specialized components as part of split PR strategy. This PR provides the collector implementations, registry management, and configuration assets required to complete TMC observability infrastructure.

**Part 2 of 2** - Specialized collectors and dashboard assets (728 lines). Complements dashboard-core PR for complete TMC observability functionality.

### Key Components Added:
- **Prometheus Metrics Collector**: HTTP API integration with workspace-aware queries
- **Cluster Collector**: Direct cluster metrics collection with KCP logical cluster support
- **Collector Registry**: Management and discovery of available metrics collectors
- **Configuration Assets**: CRDs and API schemas for workload management
- **Test Coverage**: Comprehensive test suite for collector implementations

### Technical Implementation:
- Prometheus HTTP API integration with proper error handling
- Cluster-aware metric collection with workspace isolation
- Registry pattern for extensible collector architecture
- Timeout handling and connection management
- Label-aware metric parsing and aggregation

### Integration Points:
- Requires dashboard-core PR for aggregation interfaces
- Implements MetricsSource interface from core foundation
- Extends workload management APIs with proper schemas
- Compatible with KCP workspace security boundaries

### Atomic Functionality:
- Prometheus collector operational standalone
- Cluster collector functional with proper isolation
- Registry management system complete
- All configuration assets properly integrated
- Test coverage validates collector behavior

## What Type of PR Is This?

/kind feature

## Related Issue(s)

Fixes # (TMC Observability Collectors and Assets)

## Release Notes

```
Add TMC observability specialized collectors and configuration assets.

This change implements TMC observability collector components including:
- Prometheus metrics collector with HTTP API integration
- Cluster metrics collector with KCP logical cluster support  
- Collector registry for extensible metrics architecture
- Workload management CRDs and API resource schemas
- Comprehensive test coverage for collector implementations

The collectors implement workspace-aware metric collection and integrate
with the core aggregation engine. All components follow KCP architectural
patterns and maintain security boundaries.
```