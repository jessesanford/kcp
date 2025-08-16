# Split Plan for CEL Evaluator Branch

## Current State
- Total lines: 912 (excluding tests and generated code)
- Files: 8 Go files
- Exceeds limit by: 112 lines

## File Distribution
- `pkg/policy/cel/cache.go`: 105 lines
- `pkg/policy/cel/compiler.go`: 115 lines
- `pkg/policy/cel/environment.go`: 101 lines
- `pkg/policy/cel/evaluator.go`: 131 lines
- `pkg/policy/cel/functions.go`: 190 lines
- `pkg/policy/cel/variables.go`: 78 lines
- `pkg/policy/interfaces/interfaces.go`: 87 lines
- `pkg/policy/types/types.go`: 105 lines

## Split Strategy

### Branch 17a: CEL Core Types and Interfaces (297 lines)
**Files:**
- `pkg/policy/interfaces/interfaces.go`: 87 lines
- `pkg/policy/types/types.go`: 105 lines
- `pkg/policy/cel/cache.go`: 105 lines

**Dependencies:** None

**Purpose:** 
- Define core interfaces for policy evaluation
- Establish type definitions for CEL expressions
- Implement caching layer for compiled expressions

### Branch 17b: CEL Environment and Compiler (517 lines)
**Files:**
- `pkg/policy/cel/environment.go`: 101 lines
- `pkg/policy/cel/compiler.go`: 115 lines
- `pkg/policy/cel/variables.go`: 78 lines
- `pkg/policy/cel/functions.go`: 190 lines
- Basic unit tests: ~33 lines

**Dependencies:** Branch 17a (types and interfaces)

**Purpose:**
- Set up CEL environment with custom functions
- Implement expression compiler
- Define variable bindings and custom functions
- Core functionality for expression evaluation

### Branch 17c: CEL Evaluator and Integration (321 lines)
**Files:**
- `pkg/policy/cel/evaluator.go`: 131 lines
- Integration tests: ~190 lines

**Dependencies:** Branch 17a, Branch 17b

**Purpose:**
- Implement the main evaluator logic
- Add comprehensive integration tests
- Complete the CEL evaluation pipeline

## Execution Order
1. **Branch 17a** - Foundation (types, interfaces, cache)
2. **Branch 17b** - Core logic (environment, compiler, functions)
3. **Branch 17c** - Evaluator and tests

## Success Criteria
- Each sub-branch remains under 700 lines
- Maintains atomic functionality per branch
- Clear dependency chain
- Tests included where appropriate
- All branches can be merged sequentially to main

## Implementation Notes
- Branch 17a establishes the foundation without any evaluation logic
- Branch 17b builds the CEL environment but doesn't execute evaluations
- Branch 17c completes the implementation with the evaluator and comprehensive tests
- Each branch will be independently testable
- Documentation should be distributed across branches as relevant to the code