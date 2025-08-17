# p6w1-cluster-controller Split Implementation Plan

## CONTEXT: p6w1-cluster-controller Size Violation Remediation

**Original Branch**: `feature/tmc-completion/p6w1-cluster-controller`
**Total Lines**: 1,311 lines (511 over 800 limit)
**Location**: `/workspaces/kcp-worktrees/phase6/tmc-completion/worktrees/feature-tmc-completion-p6w1-cluster-controller`

## Current Implementation Analysis

### Component Structure Review:
1. **Core Controller** (`controller.go`): 518 lines
   - Controller types and structures (lines 45-111)
   - Controller initialization (lines 115-176)
   - Queue management and processing (lines 178-293)
   - Reconciliation logic (lines 321-477)
   - Helper functions (lines 479-518)

2. **Cluster Manager** (`cluster_controller.go`): 461 lines
   - ClusterManager with validation (lines 28-139)
   - Cluster preparation and defaults (lines 141-179)
   - SyncTarget spec generation (lines 181-250)
   - Health checking system (lines 272-400)
   - Capability discovery (lines 402-461)

3. **Status Management** (`status.go`): 332 lines
   - Condition constants (lines 25-70)
   - StatusManager implementation (lines 72-257)
   - Helper utilities (lines 259-332)

### Architecture Assessment

**Design Strengths:**
- Clear separation of concerns between controller, manager, and status
- Well-structured reconciliation phases
- Proper KCP pattern adherence

**Split Opportunities:**
- Controller foundations vs reconciliation logic
- Cluster validation/preparation vs health/discovery
- Core status management vs condition utilities

## SPLIT PLAN FOR p6w1-cluster-controller

### NEW SPLIT STRUCTURE WITH NAMING CONVENTION:

#### Original Branch Rename:
- **Old**: `feature/tmc-completion/p6w1-cluster-controller`
- **New**: `feature/tmc-completion/p6w1-cluster-controller-to-be-split`
- **Worktree**: `/workspaces/kcp-worktrees/phase6/tmc-completion/worktrees/feature-tmc-completion-p6w1-cluster-controller-to-be-split`

#### Split Branches:

| Split # | Branch Name | Content | Target Lines | Purpose |
|---------|------------|---------|--------------|---------|
| 1 | `feature/tmc-completion/p6w1-cluster-1-split-from-p6w1-cluster-controller` | Core controller setup + types | ~450 | Controller initialization, types, and basic reconciliation |
| 2 | `feature/tmc-completion/p6w1-cluster-2-split-from-p6w1-cluster-controller` | Cluster management + validation | ~430 | Cluster manager, validation, and preparation |
| 3 | `feature/tmc-completion/p6w1-cluster-3-split-from-p6w1-cluster-controller` | Status + health management | ~430 | Status management, health checking, capability discovery |

### Detailed Split Breakdown:

#### Split 1: Core Controller Foundation (~450 lines)
**Files to include:**
- `controller.go` - Lines 1-320 (core controller without advanced reconciliation)
  - Package and imports
  - ClusterRegistration types (45-111)
  - Controller struct and initialization (115-191)
  - Basic queue management (192-249)
  - Core process function (250-320)

**Rationale:** Establishes the fundamental controller structure with types and basic processing pipeline.

#### Split 2: Cluster Management and Validation (~430 lines)
**Files to include:**
- `cluster_controller.go` - Lines 1-250 (management and validation)
  - ClusterManager struct and initialization (28-59)
  - Comprehensive validation logic (61-139)
  - Cluster preparation and defaults (141-179)
  - SyncTarget spec generation (181-250)
- `controller.go` - Lines 321-403 (pending/registered reconciliation)
  - reconcile function
  - reconcilePendingCluster
  - reconcileRegisteredCluster

**Rationale:** Groups cluster registration workflow and validation logic together.

#### Split 3: Status and Health Management (~430 lines)
**Files to include:**
- `status.go` - Complete file (332 lines)
  - All status management utilities
- `cluster_controller.go` - Lines 272-461 (health and capabilities)
  - ClusterHealthChecker (272-400)
  - ClusterCapabilityDiscovery (402-461)
- `controller.go` - Lines 404-518 (ready/failed reconciliation + helpers)
  - reconcileReadyCluster
  - reconcileFailedCluster
  - Helper functions

**Rationale:** Consolidates all status, health, and capability management.

## EXECUTION PROTOCOL - MANDATORY SEQUENTIAL ORDER

### Phase 0: Worktree Preparation

#### Step 1: Rename Original Oversized Branch
```bash
# Navigate to the oversized branch worktree
cd /workspaces/kcp-worktrees/phase6/tmc-completion/worktrees/feature-tmc-completion-p6w1-cluster-controller

# Verify current branch
git branch -vv | grep "^\*"

# Rename local branch to indicate it's being split
git branch -m feature/tmc-completion/p6w1-cluster-controller feature/tmc-completion/p6w1-cluster-controller-to-be-split

# Delete old remote tracking branch
git push origin --delete feature/tmc-completion/p6w1-cluster-controller

# Push renamed branch to remote
git push -u origin feature/tmc-completion/p6w1-cluster-controller-to-be-split

# Rename worktree directory to match branch name
cd /workspaces/kcp-worktrees/phase6/tmc-completion/worktrees
mv feature-tmc-completion-p6w1-cluster-controller feature-tmc-completion-p6w1-cluster-controller-to-be-split

# Verify worktree is still functional
cd feature-tmc-completion-p6w1-cluster-controller-to-be-split
git status
```

#### Step 2: Create Split Worktrees
```bash
# Navigate to correct phase directory
cd /workspaces/kcp-worktrees/phase6/tmc-completion/worktrees

# Source worktree tools
source /workspaces/kcp-shared-tools/setup-worktree-env.sh

# Create worktrees for each split
cd /workspaces/kcp

# Create split 1
git worktree add /workspaces/kcp-worktrees/phase6/tmc-completion/worktrees/feature-tmc-completion-p6w1-cluster-1-split-from-p6w1-cluster-controller \
  -b feature/tmc-completion/p6w1-cluster-1-split-from-p6w1-cluster-controller origin/main

# Create split 2
git worktree add /workspaces/kcp-worktrees/phase6/tmc-completion/worktrees/feature-tmc-completion-p6w1-cluster-2-split-from-p6w1-cluster-controller \
  -b feature/tmc-completion/p6w1-cluster-2-split-from-p6w1-cluster-controller origin/main

# Create split 3
git worktree add /workspaces/kcp-worktrees/phase6/tmc-completion/worktrees/feature-tmc-completion-p6w1-cluster-3-split-from-p6w1-cluster-controller \
  -b feature/tmc-completion/p6w1-cluster-3-split-from-p6w1-cluster-controller origin/main

# Push branches to establish remote tracking
cd /workspaces/kcp-worktrees/phase6/tmc-completion/worktrees/feature-tmc-completion-p6w1-cluster-1-split-from-p6w1-cluster-controller
git push -u origin feature/tmc-completion/p6w1-cluster-1-split-from-p6w1-cluster-controller

cd /workspaces/kcp-worktrees/phase6/tmc-completion/worktrees/feature-tmc-completion-p6w1-cluster-2-split-from-p6w1-cluster-controller
git push -u origin feature/tmc-completion/p6w1-cluster-2-split-from-p6w1-cluster-controller

cd /workspaces/kcp-worktrees/phase6/tmc-completion/worktrees/feature-tmc-completion-p6w1-cluster-3-split-from-p6w1-cluster-controller
git push -u origin feature/tmc-completion/p6w1-cluster-3-split-from-p6w1-cluster-controller
```

### Phase 1: Sequential Split Implementation

#### SPLIT 1: Core Controller Foundation
```bash
cd /workspaces/kcp-worktrees/phase6/tmc-completion/worktrees/feature-tmc-completion-p6w1-cluster-1-split-from-p6w1-cluster-controller

# Create directory structure
mkdir -p pkg/reconciler/workload/cluster

# Copy controller.go with only core functionality (lines 1-320)
# This includes types, initialization, and basic processing

# Measure
/workspaces/kcp-shared-tools/tmc-pr-line-counter.sh -c feature/tmc-completion/p6w1-cluster-1-split-from-p6w1-cluster-controller

# Commit
git add .
git commit -s -S -m "feat(controller): add TMC cluster controller foundation - split 1/3

- Implement ClusterRegistration types and structures
- Add controller initialization with KCP patterns
- Establish queue management and processing pipeline
- Set up basic reconciliation framework

Part 1 of 3 splits from p6w1-cluster-controller (originally 1311 lines)"

git push origin feature/tmc-completion/p6w1-cluster-1-split-from-p6w1-cluster-controller
```

#### SPLIT 2: Cluster Management and Validation
```bash
cd /workspaces/kcp-worktrees/phase6/tmc-completion/worktrees/feature-tmc-completion-p6w1-cluster-2-split-from-p6w1-cluster-controller

# Create directory structure
mkdir -p pkg/reconciler/workload/cluster

# Copy cluster_controller.go (lines 1-250) and reconciliation functions from controller.go

# Measure
/workspaces/kcp-shared-tools/tmc-pr-line-counter.sh -c feature/tmc-completion/p6w1-cluster-2-split-from-p6w1-cluster-controller

# Commit
git add .
git commit -s -S -m "feat(cluster): add cluster management and validation - split 2/3

- Implement ClusterManager with comprehensive validation
- Add cluster preparation and default handling
- Create SyncTarget specification generation
- Add pending and registered phase reconciliation

Part 2 of 3 splits from p6w1-cluster-controller (originally 1311 lines)"

git push origin feature/tmc-completion/p6w1-cluster-2-split-from-p6w1-cluster-controller
```

#### SPLIT 3: Status and Health Management
```bash
cd /workspaces/kcp-worktrees/phase6/tmc-completion/worktrees/feature-tmc-completion-p6w1-cluster-3-split-from-p6w1-cluster-controller

# Create directory structure
mkdir -p pkg/reconciler/workload/cluster

# Copy status.go (complete), health checking from cluster_controller.go, and remaining reconciliation

# Measure
/workspaces/kcp-shared-tools/tmc-pr-line-counter.sh -c feature/tmc-completion/p6w1-cluster-3-split-from-p6w1-cluster-controller

# Commit
git add .
git commit -s -S -m "feat(status): add status and health management - split 3/3

- Implement comprehensive status management
- Add cluster health checking system
- Create capability discovery service
- Complete ready and failed phase reconciliation

Part 3 of 3 splits from p6w1-cluster-controller (originally 1311 lines)"

git push origin feature/tmc-completion/p6w1-cluster-3-split-from-p6w1-cluster-controller
```

## Dependencies and Merge Order

### Merge Sequence:
1. **Split 1** (controller foundation) - Base implementation, no dependencies
2. **Split 2** (cluster management) - Depends on Split 1 for types
3. **Split 3** (status/health) - Depends on Splits 1 & 2 for complete functionality

### Integration Points:
- Split 1 provides the core types used by all other splits
- Split 2 extends reconciliation logic started in Split 1
- Split 3 completes the reconciliation phases and adds observability

## Success Criteria

### Size Compliance:
- ✅ Split 1: ~450 lines (within 800 limit)
- ✅ Split 2: ~430 lines (within 800 limit)
- ✅ Split 3: ~430 lines (within 800 limit)
- ✅ Total coverage: 1,310 lines (matches original)

### Functional Completeness:
- ✅ Each split is independently testable
- ✅ No functionality lost in splitting
- ✅ Clear logical boundaries between splits
- ✅ Proper dependency chain maintained

### KCP Pattern Adherence:
- ✅ Maintains workspace isolation
- ✅ Follows controller patterns
- ✅ Preserves reconciliation flow
- ✅ Keeps proper error handling

## Risk Mitigation

### Potential Issues:
1. **Import cycles**: Carefully structured to avoid circular dependencies
2. **Test coverage**: Each split will need focused unit tests
3. **Integration testing**: Final split should include integration tests

### Mitigation Strategies:
- Keep types in Split 1 to avoid import issues
- Add minimal test stubs in each split
- Full integration tests in Split 3

## Post-Split Actions

1. **Archive original branch**: Mark p6w1-cluster-controller-to-be-split as superseded
2. **Update PR plan**: Reflect new three-part structure
3. **Create tracking document**: Document split completion
4. **Clean up worktrees**: Remove original oversized worktree after merge

## Execution Timeline

- Phase 0: ~10 minutes (worktree preparation)
- Split 1: ~15 minutes (core controller)
- Split 2: ~15 minutes (cluster management)
- Split 3: ~15 minutes (status/health)
- Verification: ~5 minutes
- **Total**: ~60 minutes

This split plan ensures compliance with the 800-line limit while maintaining functional cohesion and testability.