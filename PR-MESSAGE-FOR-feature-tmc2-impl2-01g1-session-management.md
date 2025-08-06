<!--

Thanks for creating a pull request!
If this is your first time, please make sure to review CONTRIBUTING.MD.

-->

## Summary

This PR introduces the foundational session management APIs for the TMC (Transparent Multi-Cluster) placement system. It implements four core APIs that enable coordinated, session-based workload placement across multiple Kubernetes clusters:

- **PlacementSession**: Provides comprehensive session lifecycle management for placement operations
- **SessionState**: Handles persistent state tracking and recovery for distributed placement sessions  
- **PlacementDecision**: Coordinates placement decisions with conflict resolution and rollback capabilities
- **SessionValidator**: Offers a comprehensive validation framework with rule-based evaluation

## What Type of PR Is This?

/kind feature

## Related Issue(s)

Related to TMC implementation Phase 1: Session Management Foundation

## Technical Details

### Key Features

#### PlacementSession API
- Session-based placement coordination with configurable lifecycle management
- Support for placement policies with priorities and constraint evaluation
- Resource constraint validation and management
- Configurable conflict resolution strategies (Override, Merge, Fail)
- Comprehensive session recovery policies with exponential backoff

#### SessionState API  
- Persistent state tracking across cluster boundaries
- Multi-cluster session synchronization with configurable strategies
- Resource allocation tracking and conflict history
- State checkpointing and recovery mechanisms
- Event tracking and audit trails

#### PlacementDecision API
- Decision coordination with comprehensive context tracking
- Cluster evaluation with weighted scoring algorithms
- Policy application tracking and impact assessment
- Rollback policies with automated trigger conditions
- Detailed decision metrics and performance tracking

#### SessionValidator API
- Rule-based validation framework with multiple validator types
- Conflict detection policies with configurable scopes  
- Resource validation with capacity thresholds
- Custom validation scripts with Lua support
- Dependency validation with circular dependency detection

### Implementation Highlights

- **Comprehensive Test Coverage**: 860+ lines of tests covering validation scenarios, edge cases, and API interactions
- **KCP Integration**: Proper integration with KCP patterns including workspace isolation and logical cluster support
- **Generated Code**: Includes proper deepcopy generation and API registration
- **Extensible Design**: Validation framework supports custom validators and extensible conflict resolution

### File Structure

```
pkg/apis/tmc/v1alpha1/
├── doc.go                          # Package documentation
├── register.go                     # API registration  
├── types_placement_session.go     # PlacementSession API (200+ lines)
├── types_session_state.go         # SessionState API (400+ lines)
├── types_placement_decision.go    # PlacementDecision API (300+ lines)
├── types_session_validation.go    # SessionValidator API (634+ lines) 
├── types_shared.go                 # Shared types and constants
├── types_session_management_test.go # Comprehensive tests (860+ lines)
└── zz_generated.deepcopy.go       # Generated deepcopy methods
```

## Test Plan

- [x] All API validation tests pass
- [x] Comprehensive test coverage for all four APIs
- [x] Edge case validation (empty selectors, invalid scores, etc.)
- [x] Deep copy functionality verified
- [x] API registration and type system integration confirmed
- [x] Import and compilation verification completed

## Additional Notes

This is the first sub-branch of the 01g-placement-session split, focusing on the session management foundation. The second sub-branch (01g2-session-affinity) will implement session affinity and sticky placement capabilities.

🤖 Generated with [Claude Code](https://claude.ai/code)

Co-Authored-By: Claude <noreply@anthropic.com>