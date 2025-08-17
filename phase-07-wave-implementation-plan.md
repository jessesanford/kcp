# Phase 7: Wave-Based Implementation Plan for Syncer Development

## Executive Summary

**Critical Context**: Phase 7 implements the MOST CRITICAL missing functionality - the actual syncer that moves resources between KCP and physical clusters. Without this, TMC cannot function.

**Parallelization Strategy**: 
- **9 total efforts** organized into **4 waves**
- **Maximum 3 parallel agents** per wave
- **Total duration**: 4-5 days with optimal parallelization
- **Critical path**: Sync Engine → Downstream Core → WebSocket Connection

## Dependency Graph

```mermaid
graph TD
    %% Wave 1 - Core Foundation (Can run in parallel)
    W1E1[7.1.1 Sync Engine<br/>750 lines] 
    W1E2[7.1.2 Transform Pipeline<br/>650 lines]
    
    %% Wave 2 - Downstream Sync (Depends on Wave 1)
    W2E1[7.2.1 Downstream Core<br/>700 lines]
    W2E2[7.2.2 Resource Applier<br/>600 lines]
    W2E3[7.2.3 Conflict Resolution<br/>500 lines]
    
    %% Wave 3 - Upstream Sync (Can parallel with Wave 2)
    W3E1[7.3.1 Status Syncer<br/>650 lines]
    W3E2[7.3.2 Event Syncer<br/>450 lines]
    
    %% Wave 4 - Integration (Depends on Waves 2&3)
    W4E1[7.4.1 WebSocket Manager<br/>600 lines]
    W4E2[7.4.2 Heartbeat Monitor<br/>450 lines]
    
    %% Dependencies
    W1E1 --> W2E1
    W1E2 --> W2E1
    W1E1 --> W3E1
    W1E1 --> W3E2
    W2E1 --> W2E2
    W2E1 --> W2E3
    W2E1 --> W4E1
    W3E1 --> W4E1
    W3E2 --> W4E1
    W4E1 --> W4E2
    
    %% Critical Path Highlighting
    W1E1 -.->|CRITICAL PATH| W2E1
    W2E1 -.->|CRITICAL PATH| W4E1
    
    style W1E1 fill:#ff9999
    style W2E1 fill:#ff9999
    style W4E1 fill:#ff9999
```

## Wave Summary Table

| Wave | Branches | Max Parallel Agents | Dependencies | Critical Path | Total Lines |
|------|----------|-------------------|--------------|---------------|-------------|
| 1    | 7.1.1, 7.1.2 | 2 | Phase 5 & 1 | Yes (7.1.1) | 1,400 |
| 2    | 7.2.1, 7.2.2, 7.2.3, 7.3.1, 7.3.2 | 3* | Wave 1 | Yes (7.2.1) | 2,900 |
| 3    | 7.4.1 | 1 | Waves 2 | Yes | 600 |
| 4    | 7.4.2 | 1 | Wave 3 | No | 450 |

*Wave 2 can have efforts split between downstream (7.2.x) and upstream (7.3.x) teams

## Execution Timeline

| Wave | Start Condition | Branches | Agents | Duration | Cumulative Progress |
|------|----------------|----------|--------|----------|-------------------|
| 1 | Phase 6 complete | 7.1.1, 7.1.2 | 2 | 1 day | 22% |
| 2 | Wave 1 complete | 7.2.1-3, 7.3.1-2 | 3 | 2 days | 69% |
| 3 | Wave 2 critical path done | 7.4.1 | 1 | 0.5 day | 79% |
| 4 | Wave 3 complete | 7.4.2 | 1 | 0.5 day | 100% |

## Wave 1: Core Foundation (Day 1)

### Parallelization Strategy
- **2 agents working in parallel**
- No inter-dependencies within wave
- Both must complete before Wave 2

### Agent Assignments

#### Agent A: Sync Engine Specialist
**Branch**: `feature/tmc-completion/p7w1-sync-engine`
**Effort**: 7.1.1 - Sync Engine Implementation
**Lines**: ~750
**Focus**: Core synchronization engine, work queue, informer setup

**Key Files**:
- `pkg/syncer/engine/engine.go`
- `pkg/syncer/engine/types.go`
- `pkg/syncer/engine/status.go`

**Integration Points**:
- Implements Phase 5 syncer interfaces
- Creates foundation for all sync operations
- Defines work queue patterns

#### Agent B: Transform Specialist  
**Branch**: `feature/tmc-completion/p7w1-transform`
**Effort**: 7.1.2 - Transformation Pipeline
**Lines**: ~650
**Focus**: Resource transformation pipeline, namespace mapping, label/annotation handling

**Key Files**:
- `pkg/syncer/transformation/pipeline.go`
- `pkg/syncer/transformation/namespace.go`
- `pkg/syncer/transformation/labels.go`

**Integration Points**:
- Implements Phase 5 transformation interfaces
- Provides transformation capabilities to sync engine
- No direct dependency on sync engine during development

### Coordination Protocol
```bash
# Agent A starts
cd /workspaces/kcp-worktrees
wt-create feature/tmc-completion/p7w1-sync-engine p7w1-sync-engine
# Implements sync engine

# Agent B starts (parallel)
cd /workspaces/kcp-worktrees  
wt-create feature/tmc-completion/p7w1-transform p7w1-transform
# Implements transformation pipeline

# End of day sync point
# Both agents push their branches
# Integration testing begins
```

## Wave 2: Bidirectional Synchronization (Days 2-3)

### Parallelization Strategy
- **3 agents maximum**
- Split between downstream team (2 agents) and upstream team (1 agent)
- Downstream core (7.2.1) is critical path - must prioritize

### Agent Assignments

#### Agent A: Downstream Core Specialist (CRITICAL PATH)
**Branch**: `feature/tmc-completion/p7w2-downstream-core`
**Effort**: 7.2.1 - Downstream Syncer Core
**Lines**: ~700
**Dependencies**: Wave 1 (7.1.1, 7.1.2)
**Focus**: Core downstream synchronization logic

**Key Files**:
- `pkg/syncer/downstream/syncer.go`
- `pkg/syncer/downstream/controller.go`

**Blocking**: 7.2.2 (Applier) and 7.4.1 (WebSocket)

#### Agent B: Resource Management Specialist
**Branches**: 
- `feature/tmc-completion/p7w2-applier` (7.2.2)
- `feature/tmc-completion/p7w2-conflict` (7.2.3)

**Efforts**: 
- 7.2.2 - Resource Applier (~600 lines)
- 7.2.3 - Conflict Resolution (~500 lines)

**Dependencies**: 7.2.1 completion
**Focus**: Apply resources and handle conflicts

**Sequencing**:
1. Wait for 7.2.1 interfaces
2. Implement applier first
3. Then implement conflict resolution

#### Agent C: Upstream Specialist
**Branches**:
- `feature/tmc-completion/p7w3-upstream-status` (7.3.1)
- `feature/tmc-completion/p7w3-events` (7.3.2)

**Efforts**:
- 7.3.1 - Upstream Status Syncer (~650 lines)
- 7.3.2 - Event Syncer (~450 lines)

**Dependencies**: Wave 1 (7.1.1) only
**Focus**: Status and event synchronization to KCP

**Can work in parallel** with downstream team

### Resource Conflict Analysis

**Shared Files/Packages**:
- `pkg/syncer/interfaces` (read-only from Phase 5)
- `pkg/apis/workload` (read-only from Phase 5)

**No conflicts expected** - teams work in separate packages:
- Downstream team: `pkg/syncer/downstream/`
- Upstream team: `pkg/syncer/upstream/`

### Coordination Protocol
```bash
# Day 2 Morning - All agents start
# Agent A (Critical Path)
wt-create feature/tmc-completion/p7w2-downstream-core p7w2-downstream-core
# Implements downstream core - PRIORITY

# Agent C (Parallel)
wt-create feature/tmc-completion/p7w3-upstream-status p7w3-upstream-status
# Can start immediately on upstream

# Agent B (Waits for A)
# Monitors Agent A progress
# Prepares applier design while waiting

# Day 2 Afternoon
# Agent A completes core interfaces
# Agent B begins applier implementation

# Day 3
# All agents continue implementation
# Integration testing of downstream+upstream
```

## Wave 3: WebSocket Integration (Day 4 Morning)

### Parallelization Strategy
- **1 agent only** (critical integration point)
- Requires both downstream and upstream complete
- Critical for end-to-end functionality

### Agent Assignment

#### Agent A: Network Integration Specialist
**Branch**: `feature/tmc-completion/p7w4-websocket`
**Effort**: 7.4.1 - WebSocket Connection Manager
**Lines**: ~600
**Dependencies**: 7.2.1 (downstream), 7.3.1 (upstream)

**Key Files**:
- `pkg/syncer/tunnel/websocket.go`
- `pkg/syncer/tunnel/connection.go`

**Integration Requirements**:
- Wire downstream syncer to websocket
- Wire upstream status to websocket
- Implement reconnection logic
- Handle connection lifecycle

### No Parallelization Possible
This is a critical integration point that requires:
- Understanding of downstream sync implementation
- Understanding of upstream sync implementation  
- Careful integration testing

## Wave 4: Monitoring Layer (Day 4 Afternoon)

### Parallelization Strategy
- **1 agent only**
- Depends on WebSocket connection
- Final component for production readiness

### Agent Assignment

#### Agent A: Monitoring Specialist
**Branch**: `feature/tmc-completion/p7w4-heartbeat`
**Effort**: 7.4.2 - Heartbeat & Health Monitoring
**Lines**: ~450
**Dependencies**: 7.4.1 (websocket)

**Key Files**:
- `pkg/syncer/tunnel/heartbeat.go`
- `pkg/syncer/tunnel/health.go`

**Focus**:
- Heartbeat mechanism
- Health checks
- Connection monitoring
- Metrics collection

## Agent Orchestration Instructions

### Optimal Agent Allocation

**3 Software Engineering Agents Total**:

1. **Sync Core Agent** (Days 1-3)
   - Wave 1: Effort 7.1.1 (Sync Engine)
   - Wave 2: Effort 7.2.1 (Downstream Core) - CRITICAL
   - Wave 3: Effort 7.4.1 (WebSocket Integration)

2. **Transform & Resource Agent** (Days 1-3)
   - Wave 1: Effort 7.1.2 (Transform Pipeline)
   - Wave 2: Efforts 7.2.2 & 7.2.3 (Applier & Conflict)
   - Wave 4: Effort 7.4.2 (Heartbeat)

3. **Upstream Agent** (Days 2-3)
   - Wave 2: Efforts 7.3.1 & 7.3.2 (Status & Events)
   - Can assist with testing in Waves 3-4

### Daily Standup Points

**Day 1**:
- Agents 1 & 2 work on Wave 1 in parallel
- No dependencies, pure parallel work
- End of day: Integration test sync engine with transform

**Day 2**:
- Agent 1 starts critical downstream core
- Agent 2 waits for interfaces, then applier
- Agent 3 starts upstream work (parallel)

**Day 3**:
- Agent 1 completes downstream core
- Agent 2 completes applier and conflict
- Agent 3 completes upstream sync
- Integration testing begins

**Day 4**:
- Agent 1 implements WebSocket integration
- Agent 2 implements heartbeat monitoring
- Agent 3 assists with end-to-end testing

## Conflict Prevention Strategy

### File Ownership Matrix

| Package | Owner | Files |
|---------|-------|-------|
| `pkg/syncer/engine/` | Agent 1 | All files |
| `pkg/syncer/transformation/` | Agent 2 | All files |
| `pkg/syncer/downstream/` | Agents 1 & 2 | Coordinated |
| `pkg/syncer/upstream/` | Agent 3 | All files |
| `pkg/syncer/tunnel/` | Agent 1 & 2 | Sequential |

### Interface Boundaries

**Clear Separation**:
1. Engine defines interfaces (Wave 1)
2. Downstream/Upstream implement interfaces (Wave 2)
3. WebSocket connects implementations (Wave 3)
4. Heartbeat monitors connection (Wave 4)

### Merge Strategy

**No Cross-Merging**:
- All branches merge to main independently
- Each branch is atomic and complete
- Integration testing happens in main

## Risk Mitigation

### Critical Path Risks

1. **Sync Engine Delays** (7.1.1)
   - Mitigation: Start immediately, prioritize interfaces
   - Backup: Can stub interfaces for downstream work

2. **Downstream Core Complexity** (7.2.1)
   - Mitigation: Experienced agent assigned
   - Backup: Can split into smaller commits if needed

3. **WebSocket Integration** (7.4.1)
   - Mitigation: Start design early in Wave 2
   - Backup: Can implement mock connection first

### Technical Risks

1. **Informer Synchronization**
   - Risk: Race conditions between KCP and downstream
   - Mitigation: Careful locking, extensive testing

2. **Transformation Correctness**
   - Risk: Resources transformed incorrectly
   - Mitigation: Comprehensive unit tests per transformer

3. **Connection Stability**
   - Risk: WebSocket drops under load
   - Mitigation: Exponential backoff, circuit breaker

## Success Metrics

### Per-Wave Completion Criteria

**Wave 1 Complete When**:
- [ ] Sync engine processes work queue
- [ ] Transformation pipeline handles all types
- [ ] Unit tests pass for both efforts
- [ ] Integration test shows engine uses transforms

**Wave 2 Complete When**:
- [ ] Resources sync downstream successfully
- [ ] Status syncs upstream successfully
- [ ] Conflicts detected and resolved
- [ ] Events propagated correctly
- [ ] 70% test coverage achieved

**Wave 3 Complete When**:
- [ ] WebSocket connects to KCP
- [ ] Bidirectional communication works
- [ ] Reconnection logic tested
- [ ] End-to-end resource sync demonstrated

**Wave 4 Complete When**:
- [ ] Heartbeat maintains connection
- [ ] Health metrics exposed
- [ ] Monitoring dashboard data available
- [ ] Production readiness achieved

## Implementation Checklist

### Pre-Implementation
- [ ] All agents understand dependencies
- [ ] Worktrees created for each effort
- [ ] Phase 5 & 1 interfaces reviewed
- [ ] Communication channels established

### During Implementation
- [ ] Daily standup synchronization
- [ ] Line count monitoring per effort
- [ ] Integration points documented
- [ ] Test coverage maintained >70%

### Post-Implementation
- [ ] All branches pass CI/CD
- [ ] Integration tests successful
- [ ] Performance benchmarks met
- [ ] Documentation complete

## Conclusion

This wave-based plan optimizes Phase 7 implementation by:

1. **Maximizing Parallelization**: 2-3 agents working simultaneously where possible
2. **Protecting Critical Path**: Prioritizing sync engine → downstream → websocket
3. **Preventing Conflicts**: Clear package ownership and boundaries
4. **Enabling Fast Delivery**: 4-5 days vs 9 days sequential

The syncer is the heart of TMC - this plan ensures rapid, quality delivery of this critical functionality.