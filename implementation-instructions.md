# Implementation Instructions: Deployment Dependency Graph (Branch 21)

## Overview
This branch implements dependency resolution and ordering for deployments. It provides topological sorting, cycle detection, and execution ordering to ensure deployments happen in the correct sequence based on their dependencies.

## Dependencies
- **Base**: feature/tmc-phase4-14b-deployment-interfaces
- **Uses**: Branches 14a (types), 14b (interfaces)
- **Required for**: Branch 22 (rollback)

## Files to Create

### 1. `pkg/deployment/dependencies/graph.go` (100 lines)
Dependency graph data structure and operations.

### 2. `pkg/deployment/dependencies/resolver.go` (90 lines)
Dependency resolution logic.

### 3. `pkg/deployment/dependencies/validator.go` (70 lines)
Validation for dependency configurations.

### 4. `pkg/deployment/dependencies/executor.go` (100 lines)
Ordered execution of dependent deployments.

### 5. `pkg/deployment/dependencies/topological.go` (60 lines)
Topological sorting implementation.

### 6. `pkg/deployment/dependencies/graph_test.go` (120 lines)
Comprehensive dependency tests.

## Implementation Steps

### Step 1: Setup Dependencies
Ensure deployment types and interfaces are available.

### Step 2: Create Package Structure
```bash
mkdir -p pkg/deployment/dependencies
```

### Step 3: Implement Dependency Components
1. Start with graph.go - graph structure
2. Add topological.go - sorting algorithm
3. Create validator.go - cycle detection
4. Add resolver.go - dependency resolution
5. Create executor.go - execution ordering
6. Add comprehensive tests

### Step 4: Cycle Detection Testing
Thoroughly test cycle detection scenarios.

## KCP Patterns to Follow

1. **Graph Algorithms**: Efficient graph operations
2. **Cycle Detection**: Prevent circular dependencies
3. **Topological Sort**: Correct ordering
4. **Parallel Execution**: Where possible
5. **Error Propagation**: Handle dependency failures

## Testing Requirements

- [ ] Graph construction tests
- [ ] Cycle detection tests
- [ ] Topological sort tests
- [ ] Parallel execution tests
- [ ] Failure propagation tests

## Line Count Target: ~540 lines