# Upstream Sync Implementation Split Plan

The current implementation is 2998 lines, which exceeds our target. It needs to be split into multiple atomic PRs:

## Current Structure (Too Large)
1. Wave 1 API Types (411 lines - already implemented, keep)
2. Core Upstream Syncer (393 lines - keep simplified version)
3. Resource Discovery (328 lines - MOVE TO SEPARATE PR)
4. Sync Logic (377 lines - MOVE TO SEPARATE PR)  
5. Conflict Resolution (391 lines - MOVE TO SEPARATE PR)
6. Status Aggregation (288 lines - MOVE TO SEPARATE PR)
7. Tests (528 lines - keep only core tests)

## Proposed Split into 4 PRs:

### PR 1: Wave 2C Core - Upstream Syncer Foundation (THIS PR)
**Target: ~500-600 lines**
- Wave 1 API Types (411 lines - already copied)
- Basic UpstreamSyncer struct with minimal functionality
- Basic controller pattern following KCP conventions
- Placeholder methods for discovery, sync, conflict resolution
- Core tests for syncer creation and basic functionality
- **Focus**: Establish the foundation and controller pattern

### PR 2: Wave 2D - Resource Discovery  
**Target: ~400-500 lines**
- Complete resource discovery implementation
- Discovery cache management
- Resource filtering based on SyncTarget configuration
- Discovery tests

### PR 3: Wave 2E - Sync Logic & Conflict Resolution
**Target: ~400-500 lines** 
- Resource sync from physical cluster to KCP
- Resource transformation logic
- Conflict detection and resolution strategies
- Sync and conflict resolution tests

### PR 4: Wave 2F - Status Aggregation
**Target: ~300-400 lines**
- Status aggregation from multiple clusters
- Health determination logic  
- SyncTarget status updates
- Status aggregation tests

## Implementation Strategy for This PR
Keep only:
1. API types (already done)
2. Basic syncer struct with interface methods
3. Controller startup/shutdown logic
4. Placeholder implementations that log intentions
5. Basic unit tests for syncer creation

This creates a solid foundation that later PRs can build upon.