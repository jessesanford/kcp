# Split Plan for Dependency Graph Branch

## Current State
- Total lines: 988 (implementation) + 432 (tests) = 1420 total
- Files: 4 Go files (3 implementation, 1 test)
- Exceeds limit by: 188 lines (implementation only)

## File Distribution
- `pkg/deployment/dependencies/graph.go`: 372 lines
- `pkg/deployment/dependencies/topological.go`: 280 lines
- `pkg/deployment/dependencies/validator.go`: 336 lines
- `pkg/deployment/dependencies/graph_test.go`: 432 lines

## Split Strategy

### Branch 21a: Core Graph Implementation (652 lines)
**Files:**
- `pkg/deployment/dependencies/graph.go`: 372 lines
- `pkg/deployment/dependencies/topological.go`: 280 lines

**Dependencies:** None

**Purpose:**
- Implement core dependency graph data structure
- Add topological sorting for dependency ordering
- Provide basic graph operations (add, remove, traverse)
- Enable cycle detection in dependency chains

### Branch 21b: Dependency Validation and Tests (768 lines)
**Files:**
- `pkg/deployment/dependencies/validator.go`: 336 lines
- `pkg/deployment/dependencies/graph_test.go`: 432 lines

**Dependencies:** Branch 21a (graph and topological sort)

**Purpose:**
- Implement validation logic for dependency constraints
- Add comprehensive test coverage for all graph operations
- Validate circular dependencies and constraint violations
- Ensure robustness of the dependency resolution

## Execution Order
1. **Branch 21a** - Foundation (graph structure, topological sort)
2. **Branch 21b** - Validation and comprehensive testing

## Success Criteria
- Both sub-branches remain under 800 lines
- Branch 21a provides complete graph functionality
- Branch 21b adds validation layer with full test coverage
- Clear dependency relationship
- Each branch is independently valuable

## Implementation Notes
- Branch 21a focuses on the core algorithmic implementation
- The topological sort in 21a is essential for deployment ordering
- Branch 21b adds the business logic for constraint validation
- Tests in 21b cover both the graph operations and validation logic
- The split maintains logical cohesion while respecting size limits

## Alternative Approach (if needed)
If further size reduction is required, consider:
- Moving some test cases to a separate integration test PR
- Splitting validator.go into constraint and rule validators
- Creating a separate PR for advanced graph operations