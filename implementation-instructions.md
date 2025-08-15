# Implementation Instructions: HTTP Handler Implementation

## Overview
- **Branch**: feature/tmc-phase4-vw-10-http-handlers
- **Purpose**: Implement HTTP handlers for virtual workspace API serving, including REST and WebSocket support
- **Target Lines**: 400
- **Dependencies**: Branch vw-09 (apiresource controller)
- **Estimated Time**: 2 days

## Files to Create

### 1. pkg/virtual/handlers/rest_handler.go (150 lines)
**Purpose**: Implement REST API handlers

**Key Components**:
- GET/POST/PUT/PATCH/DELETE handlers
- Content negotiation
- Response formatting
- Error responses

### 2. pkg/virtual/handlers/websocket.go (100 lines)
**Purpose**: Implement WebSocket support for watch

**Key Components**:
- WebSocket upgrade
- Watch stream handling
- Event serialization
- Connection management

### 3. pkg/virtual/handlers/middleware.go (80 lines)
**Purpose**: Implement middleware chain

**Key Components**:
- Authentication middleware
- Authorization middleware
- Logging middleware
- Rate limiting middleware

### 4. pkg/virtual/handlers/rest_handler_test.go (70 lines)
**Purpose**: Test REST handlers

## Implementation Steps

1. **Implement REST handlers**:
   - Handle all HTTP methods
   - Support content negotiation
   - Format responses properly
   - Handle errors gracefully

2. **Add WebSocket support**:
   - Upgrade HTTP to WebSocket
   - Stream watch events
   - Handle disconnections
   - Manage connection pool

3. **Create middleware chain**:
   - Authentication first
   - Authorization check
   - Request logging
   - Rate limiting

4. **Add comprehensive tests**:
   - Test each HTTP method
   - Test WebSocket upgrade
   - Test middleware chain
   - Test error scenarios

## Testing Requirements
- Unit test coverage: >80%
- Test scenarios:
  - All HTTP methods
  - WebSocket connections
  - Middleware processing
  - Error responses
  - Content types

## Integration Points
- Uses: APIResource controller from branch vw-09
- Provides: HTTP API for virtual workspaces

## Acceptance Criteria
- [ ] REST handlers working
- [ ] WebSocket support functional
- [ ] Middleware chain operational
- [ ] Content negotiation working
- [ ] Tests pass with coverage
- [ ] Follows HTTP standards
- [ ] No linting errors

## Common Pitfalls
- **Handle all content types**: JSON, YAML, Protobuf
- **Proper status codes**: Follow REST conventions
- **WebSocket cleanup**: Close connections properly
- **Middleware ordering**: Critical for security
- **Error messages**: Don't leak sensitive info

## Code Review Focus
- HTTP standards compliance
- WebSocket implementation
- Middleware security
- Error handling
- Performance under load