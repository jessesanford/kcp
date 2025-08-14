# Implementation Instructions: Wave2b-03 - Transformation & Tests

## 🎯 Objective
Copy transformation logic and comprehensive tests from wave2b-virtual-to-be-split (~499 lines)

## 📋 Prerequisites
- Wave2b-01 virtual workspace foundation must be complete
- Source files available in `/workspaces/kcp-worktrees/phase2/wave2b-virtual-to-be-split/`

## ⚠️ CRITICAL: Implementation Approach
**YOU MUST COPY EXISTING FILES** - The to-be-split branch HAS full implementation ready to copy.
- Cherry-pick Wave2b-01 first: `git cherry-pick <Wave2b-01-commit-hash>`
- COPY files from `/workspaces/kcp-worktrees/phase2/wave2b-virtual-to-be-split/pkg/virtual/syncer/`
- Exact files: `transformation.go` (222 lines) and `virtual_workspace_test.go` (277 lines)
- Total: 499 lines exactly

## 🔨 Implementation Tasks

### 1. Copy `pkg/virtual/syncer/transformation.go` (222 lines - ACTUAL COUNT)

**Source Location:**
```bash
/workspaces/kcp-worktrees/phase2/wave2b-virtual-to-be-split/pkg/virtual/syncer/transformation.go
```

**Key Components:**
- `Transformer` interface definition
- `SyncTargetTransformer` struct
- `VirtualToPhysical()` conversion method
- `PhysicalToVirtual()` conversion method
- Helper functions for name/namespace mapping

**Copy Command:**
```bash
cp /workspaces/kcp-worktrees/phase2/wave2b-virtual-to-be-split/pkg/virtual/syncer/transformation.go \
   pkg/virtual/syncer/transformation.go
```

### 2. Copy `pkg/virtual/syncer/virtual_workspace_test.go` (277 lines - ACTUAL COUNT)

**Source Location:**
```bash
/workspaces/kcp-worktrees/phase2/wave2b-virtual-to-be-split/pkg/virtual/syncer/virtual_workspace_test.go
```

**Test Coverage Areas:**
- Virtual workspace creation
- Discovery mechanism
- Authentication flows
- Storage CRUD operations
- Bidirectional transformation
- Error handling scenarios

**Copy Command:**
```bash
cp /workspaces/kcp-worktrees/phase2/wave2b-virtual-to-be-split/pkg/virtual/syncer/virtual_workspace_test.go \
   pkg/virtual/syncer/virtual_workspace_test.go
```

## 📝 Critical Transformation Logic

### Virtual to Physical Conversion
```go
func (t *SyncTargetTransformer) VirtualToPhysical(virtual runtime.Object) (runtime.Object, error) {
    // 1. Type assertion for SyncTarget
    // 2. Create ConfigMap representation
    // 3. Transform spec to ConfigMap data
    // 4. Add workspace labels
    // 5. Return physical object
}
```

**Key Transformations:**
- Name mapping: `virtual-name` → `synctarget-physical-name`
- Namespace: None (cluster-scoped) → workspace-specific namespace
- Data encoding: SyncTarget spec → ConfigMap data fields
- Label injection: Add `kcp.io/workspace` label

### Physical to Virtual Conversion
```go
func (t *SyncTargetTransformer) PhysicalToVirtual(physical runtime.Object) (runtime.Object, error) {
    // 1. Type assertion for ConfigMap
    // 2. Create SyncTarget representation
    // 3. Transform ConfigMap data to spec
    // 4. Derive status from annotations
    // 5. Return virtual object
}
```

**Key Transformations:**
- Name unmapping: `synctarget-physical-name` → `virtual-name`
- Data decoding: ConfigMap data → SyncTarget spec
- Status derivation: ConfigMap annotations → SyncTarget status
- Workspace validation: Ensure correct workspace isolation

## ✅ Validation Steps

1. **File Verification**
   ```bash
   # Verify files copied correctly
   ls -la pkg/virtual/syncer/transformation.go
   ls -la pkg/virtual/syncer/virtual_workspace_test.go
   
   # Check line counts
   wc -l pkg/virtual/syncer/transformation.go  # Should be 222 lines
   wc -l pkg/virtual/syncer/virtual_workspace_test.go  # Should be 277 lines
   ```

2. **Run Tests**
   ```bash
   go test ./pkg/virtual/syncer/... -v
   ```
   All tests should pass

3. **Test Round-Trip Transformation**
   ```bash
   go test -run TestTransformation ./pkg/virtual/syncer/... -v
   ```
   Verify bidirectional transformation preserves data

4. **Line Count Verification**
   ```bash
   /workspaces/kcp-shared-tools/tmc-pr-line-counter.sh -c $(git branch --show-current)
   ```
   Target: ~499 lines

## 🔄 Commit Structure

```bash
# Commit 1: Add transformation logic
git add pkg/virtual/syncer/transformation.go
git commit -s -S -m "feat(virtual): add resource transformation for virtual workspace

- Implement bidirectional transformation
- Handle virtual to physical conversion
- Map status between representations
- Ensure workspace isolation in transformations"

# Commit 2: Add comprehensive tests
git add pkg/virtual/syncer/virtual_workspace_test.go
git commit -s -S -m "test: add comprehensive tests for virtual workspace

- Test virtual workspace creation
- Validate discovery mechanism
- Test authentication flows
- Verify storage operations
- Test transformation logic"
```

## ⚠️ Important Reminders

- **DO NOT** modify transformation logic unless fixing imports
- **DO** ensure round-trip transformation works
- **DO** verify no data loss in conversion
- **DO** maintain workspace isolation
- **DO NOT** exceed 499 lines total
- **DO** ensure >80% test coverage

## 🧪 Test Execution Guide

### Run Specific Test Suites
```bash
# Test workspace creation
go test -run TestVirtualWorkspaceCreation ./pkg/virtual/syncer/...

# Test discovery
go test -run TestDiscoveryMechanism ./pkg/virtual/syncer/...

# Test authentication
go test -run TestAuthentication ./pkg/virtual/syncer/...

# Test storage operations
go test -run TestStorageOperations ./pkg/virtual/syncer/...

# Test transformation
go test -run TestTransformation ./pkg/virtual/syncer/...
```

### Coverage Report
```bash
go test -cover ./pkg/virtual/syncer/...
```

## 🎯 Success Metrics

- [ ] Both files copied successfully
- [ ] All tests pass
- [ ] Transformation is bidirectional
- [ ] No data loss in round-trip
- [ ] Test coverage >80%
- [ ] Exactly 499 lines of code
- [ ] Virtual workspace fully functional