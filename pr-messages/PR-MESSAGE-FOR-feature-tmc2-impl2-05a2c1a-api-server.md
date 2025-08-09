<!--

Thanks for creating a pull request!
If this is your first time, please make sure to review CONTRIBUTING.MD.

-->

## Summary

This PR implements the core TMC metrics API server infrastructure, providing REST endpoints for querying cluster metrics with both TMC-specific and Prometheus-compatible formats. This is the first part of a split from the original oversized metrics API implementation.

**Key Features:**
- MetricsAPIServer with HTTP routing and endpoint handlers
- Support for TMC-specific endpoints (`/api/v1/metrics/*`)
- Prometheus-compatible endpoints (`/api/v1/query*`)
- Feature flag gating with `TMCMetricsAPI` 
- Graceful shutdown with context cancellation
- OpenAPI specification endpoint for documentation
- Extensible interfaces (MetricsStore, Authorizer) for future implementations

**Architecture:**
- Core server types and interfaces are fully defined
- HTTP handlers implement the complete request/response flow
- Placeholder methods reference helper functions (implemented in companion PR)
- Clean separation between server logic and utility functions

**Size:** 639 lines (8% under 700-line target)

## What Type of PR Is This?

/kind feature

## Related Issue(s)

Part of TMC Reimplementation Plan 2 - Phase 5: Metrics API Implementation

## Testing

- Unit tests cover server creation and basic functionality
- Integration points are tested with mock implementations
- Feature flag behavior is verified
- HTTP routing and handler registration tested

## Dependencies

This PR should be merged alongside or before the companion PR for metrics API helpers (`feature/tmc2-impl2/05a2c1b-api-helpers`), which provides the implementation for parsing, authentication, and response formatting functions.

## Release Notes

```markdown
TMC metrics API server infrastructure with REST endpoints for cluster metrics querying. Supports both TMC-specific and Prometheus-compatible query formats with feature flag gating.
```