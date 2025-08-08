## Summary

This PR implements the TMC Metrics API for querying cluster metrics, placement decisions, and operational health data. The implementation provides REST endpoints with comprehensive query parameter support, response formatting options, pagination, and authentication hooks.

## What Type of PR Is This?

/kind feature

## Related Issue(s)

Part of TMC Reimplementation Plan 2 - Phase 5 production features implementation.

## Implementation Details

### Core Features Implemented:

1. **REST Endpoints for Metrics Retrieval**:
   - `/api/v1/metrics/query` - Standard TMC metrics queries
   - `/api/v1/metrics/query_range` - Time-series range queries
   - `/api/v1/metrics/names` - Available metric names

2. **Prometheus Compatibility**:
   - `/api/v1/query` - Prometheus instant queries
   - `/api/v1/query_range` - Prometheus range queries
   - `/api/v1/label/values` - Label value enumeration

3. **Query Parameter Support**:
   - Time range filtering (start, end, step)
   - Workspace and cluster filtering
   - Label-based filtering with `label.{name}=value` syntax
   - Pagination with limit/offset parameters

4. **Response Formatting**:
   - Native JSON format with structured MetricResponse
   - Prometheus-compatible response format
   - Pagination metadata for large result sets

5. **Authentication & Authorization**:
   - User authentication via X-Remote-User header
   - Bearer token support
   - Group-based authorization via X-Remote-Groups
   - Per-request authorization through Authorizer interface

6. **Feature Flag Integration**:
   - Gated by `TMCMetricsAPI` feature flag
   - Safe to deploy with feature disabled

### API Design:

```go
// MetricQuery supports comprehensive filtering
type MetricQuery struct {
    MetricName string
    StartTime  *time.Time
    EndTime    *time.Time
    Step       *time.Duration
    Labels     map[string]string
    Workspace  string
    Cluster    string
    Limit      int
    Offset     int
}

// MetricResponse provides structured results
type MetricResponse struct {
    Status     string
    Data       []MetricSeries
    Pagination *PaginationInfo
    ErrorType  string
    Error      string
}
```

### Testing & Documentation:

- Comprehensive unit tests covering all endpoints
- Authorization and authentication testing
- Prometheus compatibility testing
- Time format parsing validation
- Embedded OpenAPI 3.0 specification at `/openapi.json`
- Health check endpoint at `/healthz`

### Example Usage:

```bash
# Query cluster health metrics
curl -H "X-Remote-User: admin" \
  "/api/v1/metrics/query?metric=tmc_cluster_health&workspace=root:prod&limit=10"

# Range query with time filtering
curl -H "X-Remote-User: admin" \
  "/api/v1/metrics/query_range?metric=tmc_placement_decisions_total&start=1609459200&end=1609462800&step=60s"

# Prometheus-compatible query
curl -H "X-Remote-User: admin" \
  "/api/v1/query?query=tmc_cluster_health{cluster=\"production\"}&time=1609459200"
```

## Testing

- **Unit Tests**: 15 test cases covering all endpoints and error conditions
- **Integration Tests**: Authentication, authorization, and Prometheus compatibility
- **Test Coverage**: Comprehensive coverage of query parsing, response formatting, and middleware

## Breaking Changes

None. This is a new API with no existing dependencies.

## Release Notes

```
Add TMC Metrics API for comprehensive cluster metrics querying with:
- REST endpoints supporting query parameters, time ranges, and filtering
- Prometheus-compatible endpoints for seamless integration
- Pagination support for large result sets  
- Authentication and authorization hooks
- OpenAPI documentation and health checks
- Feature flag protection (TMCMetricsAPI)
```

🤖 Generated with [Claude Code](https://claude.ai/code)

Co-Authored-By: Claude <noreply@anthropic.com>