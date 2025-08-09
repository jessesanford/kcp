## Summary

This PR implements the TMC time-series consolidation functionality, providing advanced time-series data processing and consolidation capabilities for the TMC observability system.

**Key Features Implemented:**
- `TimeSeriesConsolidator` for consolidating time-series data across multiple clusters
- `ConsolidationConfig` with configurable maximum data points and consolidation functions
- Support for multiple consolidation functions: Average, Max, and Min
- Time-series data reduction that preserves trends while reducing storage requirements
- Feature-flag protected implementation using `TMCTimeSeriesConsolidation`

**Implementation Details:**
- **consolidation.go** (308 lines): Core consolidation logic with time-window-based data reduction
- **consolidation_test.go** (180 lines): Comprehensive test coverage including feature flag validation
- **Total Implementation**: 488 lines (well within 700-line target)

The consolidation module reduces large time-series datasets into manageable sizes while preserving the essential characteristics of the data through configurable consolidation functions.

## What Type of PR Is This?

/kind feature

## Release Notes

```release-note
Add TMC time-series consolidation functionality for reducing time-series data while preserving trends. Includes configurable consolidation functions (average, max, min) and is protected by the TMCTimeSeriesConsolidation feature flag.
```