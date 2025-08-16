# Split Plan for Branch 6 - Placement Scheduler

## Current State
- Total lines: 1,471
- Files: 
  - types.go (121 lines)
  - scorer.go (126 lines)
  - engine.go (195 lines)
  - binpack.go (174 lines)
  - spread.go (207 lines)
  - affinity.go (231 lines)
  - scheduler_test.go (417 lines)
- Exceeds limit by: 771 lines (target was 700)

## Split Strategy

### Branch 6a: Scheduler Foundation (442 lines)
- Files:
  - types.go (121 lines) - Core types and interfaces
  - scorer.go (126 lines) - Base scoring framework
  - engine.go (195 lines) - Core scheduling engine
- Dependencies: None
- Purpose: Establishes the core scheduling framework, types, and engine that all strategies depend on

### Branch 6b: Basic Strategies (381 lines)
- Files:
  - binpack.go (174 lines) - Resource optimization strategy
  - spread.go (207 lines) - Distribution strategy  
- Dependencies: Branch 6a (requires types, scorer, and engine)
- Purpose: Implements the fundamental scheduling strategies for resource optimization and workload distribution

### Branch 6c: Advanced Strategies & Tests (648 lines)
- Files:
  - affinity.go (231 lines) - Affinity/anti-affinity rules
  - scheduler_test.go (417 lines) - Comprehensive test suite
- Dependencies: Branch 6a, Branch 6b
- Purpose: Adds advanced scheduling features and comprehensive testing for all strategies

## Execution Order
1. Branch 6a - Foundation (must merge first)
2. Branch 6b - Basic strategies (depends on 6a)
3. Branch 6c - Advanced features and tests (depends on both 6a and 6b)

## Success Criteria
- Each sub-branch is under 700 lines ✓
- Maintains atomic functionality:
  - 6a provides working scheduler with basic scoring
  - 6b adds practical scheduling strategies
  - 6c completes advanced features and testing
- Clear dependencies established
- Sequential execution required to maintain build integrity

## Implementation Notes

### Branch 6a Details
The foundation branch establishes:
- `PlacementScheduler` interface and core types
- `SchedulingDecision`, `SchedulerOptions`, and configuration types
- Basic `Scorer` interface and scoring framework
- Core `Engine` implementation with scheduling loop
- This provides a minimally functional scheduler

### Branch 6b Details
The basic strategies branch adds:
- `BinPackStrategy` for resource-efficient placement
- `SpreadStrategy` for high-availability distribution
- Both strategies integrate with the scoring framework from 6a
- These are the most commonly used scheduling strategies

### Branch 6c Details
The advanced features branch completes:
- `AffinityStrategy` for complex placement rules
- Comprehensive test coverage for all components
- Integration tests validating strategy interactions
- Performance benchmarks if applicable

## Risk Mitigation
- Each branch can be independently tested
- Core functionality (6a) can be deployed without strategies
- Strategies (6b, 6c) are additive and don't break existing functionality
- Tests in 6c validate the entire integrated system