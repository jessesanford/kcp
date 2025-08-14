<!--

Thanks for creating a pull request!
If this is your first time, please make sure to review CONTRIBUTING.MD.

-->

## Summary

This PR implements Wave1-03 of the TMC syncer implementation, focusing on helper functions, conversion support, and comprehensive testing for SyncTarget management. This split provides essential utility functions that TMC controllers will use for managing SyncTarget resources following KCP best practices.

**Key Components Added:**
- **Helper Functions**: Condition management, status utilities, and capacity calculations
- **Conversion Support**: API versioning framework for future TMC evolution  
- **Comprehensive Tests**: Full test coverage for all helper functions
- **API Foundation**: Base SyncTarget types required for helper functionality

**Files Added:**
- `pkg/apis/workload/group.go` - Workload API group definition
- `pkg/apis/workload/v1alpha1/doc.go` - API documentation  
- `pkg/apis/workload/v1alpha1/register.go` - API registration
- `pkg/apis/workload/v1alpha1/synctarget_types.go` - Core SyncTarget API types
- `pkg/apis/workload/v1alpha1/synctarget_helpers.go` - Helper functions for SyncTarget management
- `pkg/apis/workload/v1alpha1/synctarget_conversion.go` - API version conversion support
- `pkg/apis/workload/v1alpha1/synctarget_helpers_test.go` - Comprehensive tests
- `pkg/apis/workload/v1alpha1/zz_generated.deepcopy.go` - Generated deepcopy functions

## What Type of PR Is This?

/kind feature

## Related Issue(s)

Part of TMC Syncer implementation plan - Wave1-03 helpers split.
Depends on Wave1-01 (API types) being merged first.

## Release Notes

```release-note
Add helper functions and conversion support for TMC SyncTarget management, enabling controllers to easily manage resource conditions, status, and capacity following KCP best practices.
```

## Implementation Details

**Helper Functions Provided:**
- `SetCondition`, `GetCondition`, `RemoveCondition` - Condition management
- `IsReady`, `SetReady`, `GetHeartbeatTime` - Status management  
- `GetTotalCapacity`, `GetAvailableCapacity`, `HasSufficientCapacity` - Resource capacity calculations
- Label and annotation helpers for common operations

**Testing Coverage:**
- Condition state transitions and edge cases
- Status helper functionality validation
- Capacity calculation accuracy with various scenarios
- All helper functions tested with comprehensive test cases

**API Evolution Support:**
- Conversion webhook framework for multi-version support
- Hub version markers for backwards compatibility
- Foundation for future TMC API changes

## Validation

- ✅ All tests pass (`go test ./pkg/apis/workload/v1alpha1/ -v`)
- ✅ Code compiles successfully
- ✅ PR size: 793 lines (within 800 line limit)  
- ✅ Comprehensive test coverage for all helper functions
- ✅ Follows KCP API patterns and conventions
- ✅ Ready for controller integration

## Dependencies

- **Requires**: Wave1-01 (API Types) to be merged first
- **Enables**: Future TMC controller implementations
- **Compatible**: Can work in parallel with Wave1-02 (validation/defaults)

🤖 Generated with [Claude Code](https://claude.ai/code)

Co-Authored-By: Claude <noreply@anthropic.com>