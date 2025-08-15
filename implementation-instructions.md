# Implementation Instructions: Resource Discovery Implementation

## Overview
- **Branch**: feature/tmc-phase4-vw-07-discovery-impl
- **Purpose**: Implement actual resource discovery with KCP integration, APIExport integration, and OpenAPI aggregation
- **Target Lines**: 450
- **Dependencies**: Branch vw-06 (basic provider)
- **Estimated Time**: 3 days

## Files to Create

### 1. pkg/virtual/discovery/kcp_provider.go (200 lines)
**Purpose**: Implement KCP-aware discovery provider

**Key Components**:
- KCP client integration
- APIExport discovery
- Resource aggregation
- Schema merging

### 2. pkg/virtual/discovery/apiexport_integration.go (100 lines)
**Purpose**: Integrate with KCP APIExport system

**Key Components**:
- APIExport client
- APIBinding resolution
- Permission checking
- Export versioning

### 3. pkg/virtual/discovery/cache_impl.go (80 lines)
**Purpose**: Implement discovery caching

**Key Components**:
- TTL-based cache
- Workspace-scoped caching
- Cache invalidation
- Memory management

### 4. pkg/virtual/discovery/kcp_provider_test.go (70 lines)
**Purpose**: Test KCP discovery provider

## Implementation Steps

1. **Implement KCP provider**:
   - Connect to KCP API server
   - Discover APIExports
   - Aggregate resources

2. **Add APIExport integration**:
   - Resolve APIBindings
   - Check permissions
   - Handle versioning

3. **Implement caching**:
   - Cache discovery results
   - Handle invalidation
   - Manage memory usage

4. **Add comprehensive tests**:
   - Test discovery flow
   - Test caching behavior
   - Test error scenarios

## Testing Requirements
- Unit test coverage: >80%
- Test scenarios:
  - APIExport discovery
  - Resource aggregation
  - Cache hits/misses
  - Permission checks
  - Error handling

## Integration Points
- Uses: Basic provider from branch vw-06
- Provides: Real discovery implementation for virtual workspaces

## Acceptance Criteria
- [ ] KCP discovery provider working
- [ ] APIExport integration complete
- [ ] Caching implemented and tested
- [ ] OpenAPI aggregation functional
- [ ] Tests pass with good coverage
- [ ] Follows KCP patterns
- [ ] No linting errors

## Common Pitfalls
- **Handle APIExport changes**: Dynamic updates
- **Cache consistency**: Invalidate properly
- **Permission boundaries**: Respect access control
- **Schema conflicts**: Handle merging carefully
- **Performance at scale**: Many APIExports
- **Error propagation**: Clear error messages

## Code Review Focus
- KCP integration correctness
- APIExport handling
- Cache invalidation logic
- Permission enforcement
- Performance implications