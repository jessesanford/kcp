<!--

Thanks for creating a pull request!
If this is your first time, please make sure to review CONTRIBUTING.MD.

-->

## Summary

This PR implements helper functions and utilities for the TMC metrics API, providing comprehensive request parsing, authentication, response formatting, and middleware functionality. This is the second part of a split from the original oversized metrics API implementation.

**Key Features:**
- Complete HTTP request parsing for TMC and Prometheus query formats
- Authentication context extraction from multiple header sources
- Response formatting for JSON and Prometheus-compatible outputs
- Authentication and logging middleware with proper error handling
- Multi-format time parsing (Unix timestamp, RFC3339, ISO 8601)
- Pagination, label filtering, and workspace-aware query support

**Functionality:**
- `ParseQuery()`, `ParseQueryRange()` for TMC-specific queries
- `ParsePrometheusQuery()`, `ParsePrometheusQueryRange()` for Prometheus compatibility  
- `GetAuthContext()` for authentication information extraction
- `ConvertToPrometheusFormat()` for response format conversion
- `WriteJSONResponse()`, `WriteErrorResponse()` for HTTP responses
- `AuthMiddleware()`, `LoggingMiddleware()` for request processing

**Size:** 629 lines (10% under 700-line target)

## What Type of PR Is This?

/kind feature

## Related Issue(s)

Part of TMC Reimplementation Plan 2 - Phase 5: Metrics API Implementation

## Testing

- Comprehensive unit tests for all parsing functions
- Authentication context extraction tested with various header combinations
- Response formatting verified for both TMC and Prometheus formats
- Middleware behavior tested for authentication and logging
- Error handling paths thoroughly tested
- Edge cases covered (missing parameters, invalid formats)

## Dependencies

This PR provides helper functions used by the companion TMC metrics API server (`feature/tmc2-impl2/05a2c1a-api-server`). The server PR contains placeholder implementations that will be replaced with these helper functions in integration.

## Release Notes

```markdown
TMC metrics API helper functions providing request parsing, authentication, response formatting, and middleware for comprehensive metrics query processing with TMC and Prometheus compatibility.
```