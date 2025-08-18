# feat(performance): implement TMC performance benchmarks and profiling

## Summary

This PR implements comprehensive performance benchmarks and profiling capabilities for TMC. Split from oversized performance benchmarks PR to comply with size limits. Depends on performance framework from `p10w2a-perf-framework`.

- Performance profiling and CPU/memory analysis 
- Comprehensive benchmark test suite covering all TMC operations
- API response time and throughput benchmarks
- Placement algorithm and controller performance tests

## What Type of PR Is This?

/kind feature

## Related Issue(s)

Part of Phase 10 Integration Hardening - Performance Benchmarks (Split 2 of 2)

## Key Components

### Profiling Infrastructure (`profiling.go` - 355 lines)
- **CPUProfiler**: CPU usage analysis and bottleneck detection
- **MemoryProfiler**: Memory allocation and leak detection  
- **ProfilingManager**: Coordinated profiling session management
- **ProfileAnalyzer**: Performance data analysis and reporting

### Benchmark Test Suite (6 test files - don't count toward limits)
- **API Response Benchmarks**: REST API latency and throughput
- **Controller Benchmarks**: TMC controller reconciliation performance
- **Integration Benchmarks**: End-to-end TMC operation benchmarks
- **Placement Benchmarks**: Placement decision algorithm performance
- **Sync Latency Benchmarks**: Cross-cluster synchronization performance  
- **Throughput Benchmarks**: High-load throughput analysis

## Testing

- All benchmark tests validate performance against established baselines
- Profiling infrastructure tested with memory leak detection
- CPU profiling validated against known performance patterns
- Integration with CI/CD performance regression detection

## Dependencies

- **Requires**: `p10w2a-perf-framework` (performance framework foundation)
- **Must merge after**: Framework PR provides required infrastructure

## Release Notes

```
Implement comprehensive TMC performance benchmarks and profiling capabilities for performance regression detection and optimization analysis.
```