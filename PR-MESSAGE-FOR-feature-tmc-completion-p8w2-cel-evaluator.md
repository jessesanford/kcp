<!--

Thanks for creating a pull request!
If this is your first time, please make sure to review CONTRIBUTING.MD.

-->

## Summary

This PR implements the CEL Evaluator component (8.2.2) for Phase 8 Wave 2 of the TMC Cross-Workspace Runtime system. The CEL evaluator enables dynamic placement rules using Google's Common Expression Language (CEL) for flexible workspace selection criteria.

### Key Components Implemented:

- **CEL Evaluator Core** (`evaluator.go`): Main evaluation engine with expression compilation, caching, and custom function registration
- **Expression Compiler** (`compiler.go`): Advanced compilation features with in-memory caching for performance optimization  
- **Custom Functions** (`functions.go`): KCP-specific CEL functions including `hasLabel()`, `inWorkspace()`, `hasCapacity()`, `matchesSelector()`, and `distance()`
- **Context Builders** (`context.go`): Fluent API for constructing placement contexts with workspace, request, and resource information
- **Type Definitions** (`types.go`): Comprehensive type system for CEL integration with KCP placement structures
- **Unit Tests** (`simple_test.go`): Test coverage for core functionality including evaluator creation, function registry, caching, and helper functions

### Features:

- Expression compilation with syntax validation and type checking
- Memory-based caching for improved performance
- Custom KCP functions for workspace evaluation
- Flexible placement context building
- Comprehensive error handling and validation
- Integration with existing placement scheduler types

### Integration Points:

- Used by Placement Scheduler (8.2.1) for custom rule evaluation
- Evaluates workspace selection criteria dynamically
- Works with Decision Maker (8.2.3) for final placement decisions
- Integrates with existing `/pkg/placement/scheduler` types

## What Type of PR Is This?

/kind feature

## Related Issue(s)

Part of TMC Cross-Workspace Runtime implementation - Phase 8 Wave 2 Component 8.2.2

## Release Notes

```release-note
Add CEL evaluator for dynamic placement rules in TMC Cross-Workspace Runtime system. Enables flexible workspace selection using Common Expression Language with custom KCP functions for hasLabel, inWorkspace, hasCapacity operations.
```

🤖 Generated with [Claude Code](https://claude.ai/code)