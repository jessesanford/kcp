# feat(performance): implement TMC performance framework and metrics collection

## Summary

This PR implements the foundational performance framework and metrics collection infrastructure for TMC performance benchmarking. Split from oversized performance benchmarks PR to comply with size limits.

- Performance test framework with cluster setup and teardown
- Comprehensive metrics collection and aggregation
- Memory and resource usage monitoring infrastructure
- Foundation for performance benchmark execution

## What Type of PR Is This?

/kind feature

## Related Issue(s)

Part of Phase 10 Integration Hardening - Performance Benchmarks (Split 1 of 2)

## Key Components

### Performance Framework (`framework.go` - 262 lines)
- **TestCluster**: Cluster setup and management for performance tests
- **BenchmarkConfig**: Configuration for benchmark execution
- **ResourceMonitor**: Resource usage tracking and monitoring
- **CleanupManager**: Test cleanup and resource management

### Metrics Collection (`metrics.go` - 433 lines) 
- **MetricsCollector**: Comprehensive metrics collection interface
- **MemoryMetrics**: Memory usage monitoring and reporting
- **LatencyMetrics**: Request/response latency measurement
- **ThroughputMetrics**: Operations per second tracking
- **ResourceMetrics**: CPU, memory, and I/O monitoring
- **MetricsAggregator**: Data aggregation and analysis

## Testing

- Framework provides foundation for benchmark tests (implemented in subsequent PR)
- Metrics collection validated through resource monitoring
- Memory leak detection and resource cleanup verification

## Dependencies

- **Merge First**: This PR provides foundation that benchmarks will use
- **Follow-up**: `p10w2b-perf-benchmarks` contains actual benchmark tests

## Release Notes

```
Implement TMC performance framework and metrics collection infrastructure for comprehensive performance testing and monitoring.
```