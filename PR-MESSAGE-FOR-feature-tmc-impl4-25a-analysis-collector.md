<!--

Thanks for creating a pull request!
If this is your first time, please make sure to review CONTRIBUTING.MD.

-->

## Summary

This PR implements the TMC placement analysis data collector component as part of splitting the larger placement analysis functionality into focused, reviewable components. The collector provides:

- **Data Collection Framework**: Core data structures for placement analysis data with timestamp tracking, cluster identification, and resource utilization metrics
- **In-Memory Data Store**: Thread-safe storage with configurable size limits and automatic data rotation
- **Prometheus Metrics Integration**: Comprehensive metrics collection including placement counts, resource utilization, health status tracking, and operation duration monitoring
- **Configurable Collection Options**: Flexible configuration for collection intervals, data retention, and metrics namespace customization

Key components:
- `PlacementData` struct with resource utilization tracking (CPU, memory, storage, pod counts)
- `DataStore` for thread-safe in-memory storage with size-based rotation
- `MetricsCollector` with Prometheus integration for monitoring and observability
- Comprehensive test coverage with table-driven tests

## What Type of PR Is This?

/kind feature
/kind api-change

## Related Issue(s)

Part of TMC Reimplementation Plan 2 - Phase 4: Placement Analysis Implementation (Component 1 of 3)

## Release Notes

```yaml
kind: feature
area: tmc/placement-analysis
description: |
  Add TMC placement analysis data collector with Prometheus metrics integration.
  Provides foundation for collecting and monitoring placement analysis data 
  across logical clusters with configurable retention and metrics collection.
```

🤖 Generated with [Claude Code](https://claude.ai/code)

Co-Authored-By: Claude <noreply@anthropic.com>