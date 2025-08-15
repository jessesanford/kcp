# Implementation Instructions: Comprehensive Integration Tests

## Overview
- **Branch**: feature/tmc-phase4-vw-13-integration-tests
- **Purpose**: Add comprehensive integration tests for all virtual workspace components
- **Target Lines**: 600
- **Dependencies**: Branch vw-12 (advanced features)
- **Estimated Time**: 3 days

## Files to Create

### 1. test/integration/virtual/workspace_test.go (200 lines)
**Purpose**: End-to-end workspace tests

**Test Scenarios**:
- Workspace creation and deletion
- Multi-workspace isolation
- Resource discovery
- Authorization enforcement
- Status synchronization

### 2. test/integration/virtual/apiresource_test.go (150 lines)
**Purpose**: APIResource controller tests

**Test Scenarios**:
- APIResource CRUD operations
- Controller reconciliation
- Status updates
- Schema validation
- Error recovery

### 3. test/integration/virtual/discovery_test.go (100 lines)
**Purpose**: Discovery integration tests

**Test Scenarios**:
- Resource discovery flow
- APIExport integration
- Schema aggregation
- Cache behavior
- Dynamic updates

### 4. test/integration/virtual/auth_test.go (100 lines)
**Purpose**: Authorization integration tests

**Test Scenarios**:
- Permission evaluation
- Workspace isolation
- Impersonation
- Audit logging
- Cache invalidation

### 5. test/integration/virtual/helpers.go (50 lines)
**Purpose**: Test helper utilities

**Helper Functions**:
- Test client creation
- Workspace setup/teardown
- Resource creation helpers
- Assertion utilities
- Mock data generators

## Implementation Steps

1. **Setup test environment**:
   - Create test fixtures
   - Initialize test clients
   - Setup mock resources
   - Configure test namespaces

2. **Implement workspace tests**:
   - Test full lifecycle
   - Verify isolation
   - Check discovery
   - Test authorization

3. **Add controller tests**:
   - Test reconciliation
   - Verify status updates
   - Check error handling
   - Test scaling

4. **Create discovery tests**:
   - Test resource discovery
   - Verify caching
   - Check updates
   - Test errors

5. **Add auth tests**:
   - Test permissions
   - Verify isolation
   - Check audit logs
   - Test caching

## Testing Requirements
- Integration test coverage: >70%
- Test scenarios:
  - Happy path flows
  - Error conditions
  - Edge cases
  - Performance limits
  - Concurrent operations

## Integration Points
- Uses: All components from previous branches
- Provides: Comprehensive validation of system behavior

## Acceptance Criteria
- [ ] All integration tests passing
- [ ] Workspace isolation verified
- [ ] Controller behavior validated
- [ ] Discovery flow tested
- [ ] Authorization working
- [ ] Performance acceptable
- [ ] No flaky tests

## Common Pitfalls
- **Test isolation**: Clean up properly
- **Timing issues**: Use proper waits
- **Resource leaks**: Clean up after tests
- **Flaky tests**: Make deterministic
- **Test data**: Use realistic scenarios

## Code Review Focus
- Test coverage completeness
- Test reliability
- Resource cleanup
- Performance impact
- Realistic scenarios