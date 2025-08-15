# Implementation Instructions: APIResource Controller

## Overview
- **Branch**: feature/tmc-phase4-vw-09-apiresource-controller
- **Purpose**: Implement APIResource controller with reconciliation logic for managing virtual workspace resources
- **Target Lines**: 450
- **Dependencies**: Branch vw-08 (auth integration)
- **Estimated Time**: 3 days

## Files to Create

### 1. pkg/controllers/apiresource/controller.go (200 lines)
**Purpose**: Implement the main APIResource controller

**Key Components**:
- Controller setup with KCP client
- Informer configuration
- Workqueue management
- Event handlers

### 2. pkg/controllers/apiresource/reconciler.go (100 lines)
**Purpose**: Implement reconciliation logic

**Key Components**:
- Reconcile loop implementation
- Status update logic
- Resource synchronization
- Error handling and retry

### 3. pkg/controllers/apiresource/validator.go (80 lines)
**Purpose**: Validate APIResource specifications

**Key Components**:
- Spec validation
- Schema validation
- Permission checking
- Conflict detection

### 4. pkg/controllers/apiresource/controller_test.go (70 lines)
**Purpose**: Test controller functionality

## Implementation Steps

1. **Setup controller**:
   - Initialize with KCP client
   - Configure informers
   - Setup workqueue
   - Register event handlers

2. **Implement reconciliation**:
   - Process APIResource changes
   - Update virtual workspace configuration
   - Sync with discovery provider
   - Update status conditions

3. **Add validation**:
   - Validate resource definitions
   - Check for conflicts
   - Verify permissions
   - Validate schemas

4. **Add comprehensive tests**:
   - Test reconciliation flow
   - Test error scenarios
   - Test status updates
   - Test validation logic

## Testing Requirements
- Unit test coverage: >80%
- Test scenarios:
  - Resource creation/update/deletion
  - Reconciliation logic
  - Status condition updates
  - Validation failures
  - Error recovery

## Integration Points
- Uses: Auth integration from branch vw-08
- Provides: Controller for managing APIResource lifecycle

## Acceptance Criteria
- [ ] Controller properly initialized
- [ ] Reconciliation loop working
- [ ] Status updates functional
- [ ] Validation comprehensive
- [ ] Tests pass with coverage
- [ ] Follows controller patterns
- [ ] No linting errors

## Common Pitfalls
- **Avoid reconciliation loops**: Proper event filtering
- **Handle errors gracefully**: Use exponential backoff
- **Update status correctly**: Use status subresource
- **Validate thoroughly**: Prevent invalid states
- **Clean up resources**: Handle deletion properly

## Code Review Focus
- Controller pattern adherence
- Reconciliation efficiency
- Error handling strategy
- Status management
- Resource cleanup