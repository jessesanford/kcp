# TMC Branch Split Plan: 01g-placement-session

## Overview
- **Original Branch**: `feature/tmc2-impl2/01g-placement-session`
- **Original Size**: 1,076 lines
- **Target Size**: 700 lines per sub-branch
- **Split Strategy**: 2 sub-branches focusing on session management aspects

## Split Plan

### Sub-branch 1: 01g1-session-management (Pending)
- **Branch**: `feature/tmc2-impl2/01g1-session-management`
- **Estimated Size**: ~550 lines
- **Content**: Placement session management foundation
  - PlacementSession API with session lifecycle management
  - Session state tracking and persistence
  - Session-based placement decisions and coordination
  - Session validation and conflict resolution

### Sub-branch 2: 01g2-session-affinity (Pending)
- **Branch**: `feature/tmc2-impl2/01g2-session-affinity`
- **Estimated Size**: ~526 lines
- **Content**: Session affinity and sticky placement
  - SessionAffinity API for workload placement stickiness
  - Affinity rules and constraint management
  - Session-based routing and load balancing
  - Affinity validation and enforcement mechanisms

## Implementation Order
1. ⏳ 01g1-session-management (Session foundation)
2. ⏳ 01g2-session-affinity (Affinity and stickiness)

## Dependencies
- 01g2 depends on 01g1 for basic session management
- Both sub-branches share session state and lifecycle types
- Integration with placement algorithms from other branches

## Notes
- Split maintains full session management functionality
- Session persistence and state management in first sub-branch
- Affinity and routing policies in second sub-branch
- Proper separation of concerns for session lifecycle vs affinity rules