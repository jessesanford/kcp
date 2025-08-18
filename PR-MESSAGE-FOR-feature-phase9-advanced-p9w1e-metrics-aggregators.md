# PR Message for feature/phase9-advanced/p9w1e-metrics-aggregators

<!--

Thanks for creating a pull request!
If this is your first time, please make sure to review CONTRIBUTING.MD.

-->

## Summary

Add TMC metrics aggregators for latency and utilization analysis. These aggregators process raw metrics data to provide higher-level insights and statistical analysis for TMC operational monitoring.

**Features Added:**
- LatencyAggregator for request/operation latency analysis with percentiles
- UtilizationAggregator for resource utilization tracking and capacity planning
- Statistical processing with histograms, averages, and percentile calculations
- Time-series data windowing and rolling aggregations
- Integration with TMC metrics collectors and exporters
- Configurable aggregation intervals and retention policies

## What Type of PR Is This?

/kind feature

## Related Issue(s)

Fixes #TMC-Phase9-Metrics-Aggregators

## Release Notes

```markdown
Add TMC metrics aggregators including:
* LatencyAggregator for operation latency analysis with percentiles
* UtilizationAggregator for resource utilization and capacity tracking
* Statistical processing with histograms and percentile calculations
* Time-series windowing and rolling aggregations
* Configurable aggregation intervals and retention policies

🤖 Generated with [Claude Code](https://claude.ai/code)
```