# Execute Critical Split Implementation for vw-05b Overflow Remediation

## CONTEXT: vw-05b Size Violation Remediation
The vw-05b branch exceeded the 800-line hard limit with 1,372 lines. This workflow will properly split it into compliant branches with clear tracking, all organized within the phase4/virtual-workspaces directory structure.

## SPLIT PLAN FOR vw-05b (1,372 lines total)

### Original vw-05b Components:
1. **RBAC Evaluator** (`pkg/virtual/auth/rbac.go`): 588 lines
2. **Cache System** (`pkg/virtual/auth/cache.go`): 784 lines
3. **Policy Engine** (not implemented): ~150 lines estimated
4. **Metrics** (not implemented): ~71 lines estimated

### NEW SPLIT STRUCTURE WITH PROPER NAMING:

#### Original Branch Rename:
- **Old**: `vw-05b` → **New**: `vw-05b-to-be-split`
- **Worktree**: `/workspaces/kcp-worktrees/phase4/virtual-workspaces/vw-05b-to-be-split`

#### Split Branches:
1. **vw-05b-1-split-from-vw-05b**: RBAC Core (588 lines)
2. **vw-05b-2-split-from-vw-05b**: Cache System (784 lines)
3. **vw-05b-3-split-from-vw-05b**: Policy & Metrics (221 lines)

## ORCHESTRATION PROTOCOL - MANDATORY EXECUTION ORDER

### Phase 0: Worktree Reorganization and Preparation

#### Step 1: Rename Original Oversized Branch
```bash
cd /workspaces/kcp-worktrees/vw-05b

# Rename local branch
git branch -m vw-05b vw-05b-to-be-split

# Delete old remote tracking branch
git push origin --delete vw-05b

# Push renamed branch to remote
git push -u origin vw-05b-to-be-split

# Verify branch rename
git branch -vv | grep "vw-05b-to-be-split.*origin/vw-05b-to-be-split"

# Create phase4/virtual-workspaces directory structure if needed
mkdir -p /workspaces/kcp-worktrees/phase4/virtual-workspaces

# Move worktree to proper location
cd /workspaces/kcp-worktrees
mv vw-05b phase4/virtual-workspaces/vw-05b-to-be-split

# Verify worktree is still valid
cd phase4/virtual-workspaces/vw-05b-to-be-split
git status
```

#### Step 2: Create Split Worktrees in Same Directory Structure
```bash
# Ensure we're in the correct parent directory
cd /workspaces/kcp-worktrees/phase4/virtual-workspaces

# Source worktree tools
source /workspaces/kcp-shared-tools/setup-worktree-env.sh

# Create worktrees for splits - ALL IN phase4/virtual-workspaces
cd /workspaces/kcp
git worktree add /workspaces/kcp-worktrees/phase4/virtual-workspaces/vw-05b-1-split-from-vw-05b -b vw-05b-1-split-from-vw-05b origin/main
git worktree add /workspaces/kcp-worktrees/phase4/virtual-workspaces/vw-05b-2-split-from-vw-05b -b vw-05b-2-split-from-vw-05b origin/main
git worktree add /workspaces/kcp-worktrees/phase4/virtual-workspaces/vw-05b-3-split-from-vw-05b -b vw-05b-3-split-from-vw-05b origin/main

# Push branches to establish remote tracking
cd /workspaces/kcp-worktrees/phase4/virtual-workspaces/vw-05b-1-split-from-vw-05b
git push -u origin vw-05b-1-split-from-vw-05b

cd /workspaces/kcp-worktrees/phase4/virtual-workspaces/vw-05b-2-split-from-vw-05b
git push -u origin vw-05b-2-split-from-vw-05b

cd /workspaces/kcp-worktrees/phase4/virtual-workspaces/vw-05b-3-split-from-vw-05b
git push -u origin vw-05b-3-split-from-vw-05b

# Verify all worktrees are in the correct location
ls -la /workspaces/kcp-worktrees/phase4/virtual-workspaces/ | grep vw-05b
# Should show:
# vw-05b-to-be-split/
# vw-05b-1-split-from-vw-05b/
# vw-05b-2-split-from-vw-05b/
# vw-05b-3-split-from-vw-05b/
```

#### Step 3: Document Split Relationship
```bash
# Document the split relationship
cat > /workspaces/kcp-worktrees/tmc-planning/VW-05B-SPLIT-TRACKING.md << 'EOF'
# vw-05b Split Tracking

## Original Branch (OVERSIZED - TO BE ARCHIVED)
- Branch: vw-05b-to-be-split (renamed from vw-05b)
- Size: 1,372 lines (572 over 800 limit)
- Status: TO BE SPLIT - DO NOT MERGE
- Location: /workspaces/kcp-worktrees/phase4/virtual-workspaces/vw-05b-to-be-split

## Split Branches (REPLACEMENTS)
All located in: /workspaces/kcp-worktrees/phase4/virtual-workspaces/

| Branch | Purpose | Target Lines | Location |
|--------|---------|--------------|----------|
| vw-05b-1-split-from-vw-05b | RBAC Core | 588 | phase4/virtual-workspaces/vw-05b-1-split-from-vw-05b |
| vw-05b-2-split-from-vw-05b | Cache System | 784 | phase4/virtual-workspaces/vw-05b-2-split-from-vw-05b |
| vw-05b-3-split-from-vw-05b | Policy & Metrics | 221 | phase4/virtual-workspaces/vw-05b-3-split-from-vw-05b |

## Naming Convention Explanation
- `-to-be-split` suffix: Marks oversized branch pending split
- `-split-from-vw-05b` suffix: Identifies splits originating from vw-05b
- All splits maintain phase4/virtual-workspaces directory structure

Total Split Coverage: 1,593 lines (includes new policy/metrics)
Original Overflow: 572 lines over 800 limit
EOF

# Commit tracking document
cd /workspaces/kcp-worktrees/tmc-planning
git add VW-05B-SPLIT-TRACKING.md
git commit -s -S -m "doc: track vw-05b split with proper naming conventions"
git push origin feature/tmc-planning
```

### Phase 1: Sequential Split Implementation

#### SPLIT 1: vw-05b-1-split-from-vw-05b (RBAC Core)

**Implementation Instructions:**

1. Verify location and protection:
```bash
pwd  # MUST show: /workspaces/kcp-worktrees/phase4/virtual-workspaces/vw-05b-1-split-from-vw-05b
git branch -vv | grep "vw-05b-1-split-from-vw-05b.*origin/vw-05b-1-split-from-vw-05b"
```

2. Copy RBAC implementation from original branch:
```bash
# Ensure we're in the correct worktree
cd /workspaces/kcp-worktrees/phase4/virtual-workspaces/vw-05b-1-split-from-vw-05b

# Create directory structure
mkdir -p pkg/virtual/auth

# Copy the RBAC file from the renamed branch
git show vw-05b-to-be-split:pkg/virtual/auth/rbac.go > pkg/virtual/auth/rbac.go
```

3. Measure immediately: 588 lines expected
```bash
/workspaces/kcp-shared-tools/tmc-pr-line-counter.sh -c vw-05b-1-split-from-vw-05b
```

4. Create PR message:
```bash
cat > PR-MESSAGE-FOR-vw-05b-1-split-from-vw-05b.md << 'EOF'
## Summary
This PR implements the RBAC evaluator component, split from the oversized vw-05b branch.

- Part 1 of 3 splits from vw-05b-to-be-split (originally 1,372 lines)
- Implements role-based access control evaluation
- Maintains workspace isolation and KCP patterns

## Related Splits
- This PR: vw-05b-1-split-from-vw-05b (RBAC Core)
- Next: vw-05b-2-split-from-vw-05b (Cache System)
- Next: vw-05b-3-split-from-vw-05b (Policy & Metrics)

Together these three PRs replace the oversized vw-05b branch.
EOF
```

5. Commit and push:
```bash
git add pkg/virtual/auth/rbac.go PR-MESSAGE-FOR-vw-05b-1-split-from-vw-05b.md
git commit -s -S -m "feat(auth): implement RBAC evaluator - split 1/3 from vw-05b"
git push origin vw-05b-1-split-from-vw-05b
```

#### SPLIT 2: vw-05b-2-split-from-vw-05b (Cache System) - CRITICAL SIZE WARNING

⚠️ **CRITICAL WARNING: This split is 784 lines - VERY close to 800 limit!**

**Implementation Instructions:**

1. Verify location and protection:
```bash
pwd  # MUST show: /workspaces/kcp-worktrees/phase4/virtual-workspaces/vw-05b-2-split-from-vw-05b
git branch -vv | grep "vw-05b-2-split-from-vw-05b.*origin/vw-05b-2-split-from-vw-05b"
```

2. Copy cache implementation from original branch:
```bash
# Ensure we're in the correct worktree
cd /workspaces/kcp-worktrees/phase4/virtual-workspaces/vw-05b-2-split-from-vw-05b

# Create directory structure
mkdir -p pkg/virtual/auth

# Copy the cache file from renamed branch
git show vw-05b-to-be-split:pkg/virtual/auth/cache.go > pkg/virtual/auth/cache.go
```

3. Measure IMMEDIATELY - MUST be ≤784 lines:
```bash
/workspaces/kcp-shared-tools/tmc-pr-line-counter.sh -c vw-05b-2-split-from-vw-05b
```

4. If over 784 lines, optimize:
- Remove verbose comments
- Consolidate error messages
- BUT maintain all functionality

5. Create PR message and commit:
```bash
cat > PR-MESSAGE-FOR-vw-05b-2-split-from-vw-05b.md << 'EOF'
## Summary
This PR implements the multi-layer cache system, split from the oversized vw-05b branch.

- Part 2 of 3 splits from vw-05b-to-be-split (originally 1,372 lines)
- Implements L1/L2/L3 caching with sharding
- High-performance authorization decision caching

## Related Splits
- Previous: vw-05b-1-split-from-vw-05b (RBAC Core)
- This PR: vw-05b-2-split-from-vw-05b (Cache System)
- Next: vw-05b-3-split-from-vw-05b (Policy & Metrics)
EOF

git add pkg/virtual/auth/cache.go PR-MESSAGE-FOR-vw-05b-2-split-from-vw-05b.md
git commit -s -S -m "feat(auth): implement multi-layer cache - split 2/3 from vw-05b"
git push origin vw-05b-2-split-from-vw-05b
```

#### SPLIT 3: vw-05b-3-split-from-vw-05b (Policy & Metrics)

**Implementation Instructions:**

1. Verify location and protection:
```bash
pwd  # MUST show: /workspaces/kcp-worktrees/phase4/virtual-workspaces/vw-05b-3-split-from-vw-05b
git branch -vv | grep "vw-05b-3-split-from-vw-05b.*origin/vw-05b-3-split-from-vw-05b"
```

2. Create NEW policy and metrics implementations:
```bash
cd /workspaces/kcp-worktrees/phase4/virtual-workspaces/vw-05b-3-split-from-vw-05b
mkdir -p pkg/virtual/auth
```

3. Implement pkg/virtual/auth/policy.go (~150 lines):
- Policy interface for rule evaluation
- Basic policy evaluator implementation
- Rule definitions and matching
- Integration points with RBAC from split 1

4. Measure after policy.go:
```bash
/workspaces/kcp-shared-tools/tmc-pr-line-counter.sh -c vw-05b-3-split-from-vw-05b
```

5. Implement pkg/virtual/auth/metrics.go (~71 lines):
- Prometheus metrics for auth operations
- Cache hit/miss rate tracking
- Authorization latency histograms
- Error rate counters

6. Final measurement:
```bash
/workspaces/kcp-shared-tools/tmc-pr-line-counter.sh -c vw-05b-3-split-from-vw-05b
```

7. Create PR message and commit:
```bash
cat > PR-MESSAGE-FOR-vw-05b-3-split-from-vw-05b.md << 'EOF'
## Summary
This PR completes the auth implementation with policy engine and metrics.

- Part 3 of 3 splits from vw-05b-to-be-split (originally 1,372 lines)
- Adds policy evaluation engine
- Implements Prometheus metrics for observability

## Related Splits
- Previous: vw-05b-1-split-from-vw-05b (RBAC Core)
- Previous: vw-05b-2-split-from-vw-05b (Cache System)
- This PR: vw-05b-3-split-from-vw-05b (Policy & Metrics)

Together these three PRs provide complete auth functionality, replacing vw-05b.
EOF

git add pkg/virtual/auth/policy.go pkg/virtual/auth/metrics.go PR-MESSAGE-FOR-vw-05b-3-split-from-vw-05b.md
git commit -s -S -m "feat(auth): implement policy engine and metrics - split 3/3 from vw-05b"
git push origin vw-05b-3-split-from-vw-05b
```

### Phase 2: Verification & Documentation

1. Verify all splits are in correct directory and within limits:
```bash
echo "=== Verifying worktree locations and sizes ==="

# Check directory structure
ls -la /workspaces/kcp-worktrees/phase4/virtual-workspaces/ | grep vw-05b

# Verify each split
for branch in vw-05b-1-split-from-vw-05b vw-05b-2-split-from-vw-05b vw-05b-3-split-from-vw-05b; do
  echo "=== $branch ==="
  cd /workspaces/kcp-worktrees/phase4/virtual-workspaces/$branch
  echo "Location: $(pwd)"
  echo "Branch tracking: $(git branch -vv)"
  echo "Line count:"
  /workspaces/kcp-shared-tools/tmc-pr-line-counter.sh -c $branch
  echo "Clean status: $(git status --porcelain | wc -l) uncommitted files"
  echo ""
done
```

2. Verify original branch is renamed:
```bash
cd /workspaces/kcp-worktrees/phase4/virtual-workspaces/vw-05b-to-be-split
git branch -vv | grep "vw-05b-to-be-split.*origin/vw-05b-to-be-split"
echo "Original vw-05b successfully renamed to vw-05b-to-be-split"
```

3. Update PR plan:
```bash
cd /workspaces/kcp-worktrees/tmc-planning
# Edit TMC-REIMPL-PLAN2-PR-PLAN.md to show:
# - vw-05b-to-be-split: DO NOT MERGE - SUPERSEDED
# - vw-05b-1-split-from-vw-05b: Ready for review
# - vw-05b-2-split-from-vw-05b: Ready for review
# - vw-05b-3-split-from-vw-05b: Ready for review
```

### Phase 3: Final Report Generation

## vw-05b Split Remediation - Final Report

### Original Violation:
- Original Branch: vw-05b (1,372 lines - 572 over limit)
- Renamed To: vw-05b-to-be-split
- Status: DO NOT MERGE - SUPERSEDED BY SPLITS
- Location: /workspaces/kcp-worktrees/phase4/virtual-workspaces/vw-05b-to-be-split

### Split Implementation Results:
| Branch | Purpose | Target | Location | Status |
|--------|---------|--------|----------|--------|
| vw-05b-1-split-from-vw-05b | RBAC Core | 588 | phase4/virtual-workspaces | Ready |
| vw-05b-2-split-from-vw-05b | Cache System | 784 | phase4/virtual-workspaces | Ready |
| vw-05b-3-split-from-vw-05b | Policy & Metrics | 221 | phase4/virtual-workspaces | Ready |

### Directory Organization:
✅ All worktrees maintained in: /workspaces/kcp-worktrees/phase4/virtual-workspaces/
✅ No scattered worktrees across filesystem
✅ Clear organizational structure preserved

### Naming Convention Compliance:
✅ Original branch renamed with `-to-be-split` suffix
✅ All splits use `-split-from-vw-05b` suffix
✅ Clear lineage tracking through naming
✅ Remote branches match local naming

### Functional Coverage:
✅ All original vw-05b functionality preserved
✅ Additional policy engine added in split 3
✅ Clean separation of concerns
✅ No duplicate code between splits

### PR Merge Sequence:
1. vw-05b-1-split-from-vw-05b (RBAC) - can merge independently
2. vw-05b-2-split-from-vw-05b (Cache) - can merge independently
3. vw-05b-3-split-from-vw-05b (Policy) - depends on split 1

### Post-Merge Actions:
1. Delete vw-05b-to-be-split branch and worktree
2. Update documentation to reference new structure
3. Apply this pattern to any future oversized branches

## SUCCESS CRITERIA - ALL MUST BE MET

✅ Original vw-05b renamed to vw-05b-to-be-split
✅ All 3 splits created with -split-from-vw-05b suffix
✅ All worktrees in phase4/virtual-workspaces directory
✅ vw-05b-1-split-from-vw-05b under 700 lines
✅ vw-05b-2-split-from-vw-05b under 800 lines (CRITICAL)
✅ vw-05b-3-split-from-vw-05b under 700 lines
✅ Complete functional coverage maintained
✅ All commits signed and pushed
✅ Zero uncommitted files
✅ PR plan updated with new structure
✅ Clear relationship tracking through naming

## CRITICAL EXECUTION NOTES

⚠️ **Directory Structure**: ALL worktrees MUST be in `/workspaces/kcp-worktrees/phase4/virtual-workspaces/`
⚠️ **Branch Renaming**: Original MUST be renamed with `-to-be-split` BEFORE creating splits
⚠️ **Split Naming**: ALL splits MUST use `-split-from-vw-05b` suffix
⚠️ **vw-05b-2 Size**: 784 lines - only 16 lines under limit - BE EXTREMELY CAREFUL
⚠️ **Sequential Execution**: NO parallel work - splits must be done one at a time