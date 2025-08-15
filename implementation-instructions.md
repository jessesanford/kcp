# Implementation Instructions: Authorization Integration

## Overview
- **Branch**: feature/tmc-phase4-vw-08-auth-integration
- **Purpose**: Integrate KCP authorization with virtual workspaces, implement workspace-scoped permissions
- **Target Lines**: 400
- **Dependencies**: Branch vw-07 (discovery implementation)
- **Estimated Time**: 2 days

## Files to Create

### 1. pkg/virtual/auth/kcp_provider.go (150 lines)
**Purpose**: Implement KCP authorization provider

**Key Components**:
- KCP SubjectAccessReview integration
- Workspace-scoped authorization
- Permission aggregation
- Impersonation support

### 2. pkg/virtual/auth/workspace_context.go (80 lines)
**Purpose**: Manage workspace-specific authorization context

**Key Components**:
- Workspace permission resolution
- Context enrichment
- User attribute extraction
- Group membership handling

### 3. pkg/virtual/auth/permission_cache.go (70 lines)
**Purpose**: Implement permission caching

**Key Components**:
- Permission cache with TTL
- Workspace-based invalidation
- LRU eviction policy
- Cache statistics

### 4. pkg/virtual/auth/audit.go (50 lines)
**Purpose**: Implement audit logging

**Key Components**:
- Audit event creation
- Decision logging
- Performance metrics
- Compliance tracking

### 5. pkg/virtual/auth/kcp_provider_test.go (50 lines)
**Purpose**: Test KCP authorization provider

## Implementation Steps

1. **Implement KCP provider**:
   - Integrate with KCP authorization
   - Handle SubjectAccessReviews
   - Support impersonation

2. **Add workspace context**:
   - Extract workspace from requests
   - Enrich authorization context
   - Handle multi-workspace scenarios

3. **Implement caching**:
   - Cache authorization decisions
   - Handle invalidation events
   - Monitor cache performance

4. **Add audit logging**:
   - Log all authorization decisions
   - Track permission usage
   - Support compliance requirements

## Testing Requirements
- Unit test coverage: >80%
- Test scenarios:
  - Permission granted/denied
  - Cache behavior
  - Workspace isolation
  - Audit logging
  - Impersonation

## Integration Points
- Uses: Discovery implementation from branch vw-07
- Provides: KCP-integrated authorization for virtual workspaces

## Acceptance Criteria
- [ ] KCP authorization working
- [ ] Workspace isolation enforced
- [ ] Permission caching functional
- [ ] Audit logging implemented
- [ ] Tests pass with coverage
- [ ] Follows KCP patterns
- [ ] No linting errors

## Common Pitfalls
- **Workspace isolation**: Critical for security
- **Cache invalidation**: Maintain consistency
- **Audit completeness**: Log all decisions
- **Performance impact**: Minimize auth overhead
- **Error handling**: Clear permission denials

## Code Review Focus
- Security of authorization logic
- Workspace isolation correctness
- Cache invalidation strategy
- Audit logging completeness
- Performance optimization