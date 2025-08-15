<!--

Thanks for creating a pull request!
If this is your first time, please make sure to review CONTRIBUTING.MD.

-->

## Summary

This PR implements the core workspace interfaces for virtual workspace management in KCP as part of the vw-03a split (first of two parts from the original vw-03 branch).

This implementation provides:

- **WorkspaceProvider Interface**: Comprehensive interface for workspace lifecycle management including creation, deletion, updates, and access control
- **WorkspaceCache Interface**: High-performance caching layer for workspace metadata, clients, and capabilities
- **Core Types**: Complete type definitions for workspace references, metadata, configuration, and state management
- **KCP Integration**: Full integration with logical clusters, workspace isolation, and KCP architectural patterns

Key Features:
- Thread-safe concurrent operations with proper synchronization patterns
- Comprehensive error handling and structured error types  
- Performance optimization through intelligent caching strategies
- Extensible interface-driven design for multiple implementation approaches
- Complete workspace lifecycle support (create, read, update, delete, watch)
- Authentication and authorization integration hooks
- Observability and metrics collection capabilities

## What Type of PR Is This?

/kind feature

## Related Issue(s)

Part of Virtual Workspace Phase 4 implementation - vw-03a core interfaces split.

## Release Notes

```
feat(virtual-workspace): Add core workspace interfaces for virtual workspace management

This adds the foundational interfaces and types needed for virtual workspace 
management in KCP, including WorkspaceProvider for lifecycle operations and 
WorkspaceCache for performance optimization. All interfaces follow KCP patterns 
for workspace isolation and logical cluster integration.

Note: This PR contains interfaces only - no implementations provided.
```

## Technical Details

**Files Added:**
- `pkg/virtual/workspace/doc.go` - Package documentation and usage examples
- `pkg/virtual/workspace/types.go` - Core types, constants, and data structures  
- `pkg/virtual/workspace/provider.go` - WorkspaceProvider interface definition
- `pkg/virtual/workspace/cache.go` - WorkspaceCache interface definition

**Line Count:** 785 lines (within acceptable limits)
**Test Coverage:** Interfaces only - tests will be added with implementations
**Dependencies:** Leverages existing KCP packages for logical clusters and authorization

**Design Principles:**
- Interface segregation for clean separation of concerns
- Thread-safe concurrent access patterns
- Comprehensive error handling with context
- Performance-first caching design
- KCP workspace isolation compliance
- Extensible configuration system

This is part 1 of the vw-03 split - part 2 (vw-03b) will contain workspace lifecycle interfaces.