# Implementation Instructions: Wave2b-02 - Auth & Storage

## 🎯 Objective
Copy authentication and storage components from wave2b-virtual-to-be-split (~375 lines)

## 📋 Prerequisites
- Wave2b-01 virtual workspace foundation must be complete
- Source files available in `/workspaces/kcp-worktrees/phase2/wave2b-virtual-to-be-split/`

## ⚠️ CRITICAL: Implementation Approach
**YOU MUST COPY EXISTING FILES** - The to-be-split branch HAS full implementation ready to copy.
- Cherry-pick Wave2b-01 first: `git cherry-pick <Wave2b-01-commit-hash>`
- COPY files from `/workspaces/kcp-worktrees/phase2/wave2b-virtual-to-be-split/pkg/virtual/syncer/`
- Exact files: `auth.go` (185 lines) and `rest_storage.go` (190 lines)
- Total: 375 lines exactly

## 🔨 Implementation Tasks

### 1. Copy `pkg/virtual/syncer/auth.go` (185 lines - ACTUAL COUNT)

**Source Location:**
```bash
/workspaces/kcp-worktrees/phase2/wave2b-virtual-to-be-split/pkg/virtual/syncer/auth.go
```

**Key Components:**
- `Authorizer` struct with workspace validation
- `Authorize()` method for request authorization
- `extractWorkspace()` helper function
- `validateUserAccess()` validation logic
- RBAC integration checks

**Copy Command:**
```bash
cp /workspaces/kcp-worktrees/phase2/wave2b-virtual-to-be-split/pkg/virtual/syncer/auth.go \
   pkg/virtual/syncer/auth.go
```

### 2. Copy `pkg/virtual/syncer/rest_storage.go` (190 lines - ACTUAL COUNT)

**Source Location:**
```bash
/workspaces/kcp-worktrees/phase2/wave2b-virtual-to-be-split/pkg/virtual/syncer/rest_storage.go
```

**Key Components:**
- `SyncTargetStorage` struct implementing REST storage
- CRUD operations (Create, Get, List, Update, Delete, Watch)
- Resource conversion between virtual and physical
- Workspace-scoped operations
- Status subresource handling

**Copy Command:**
```bash
cp /workspaces/kcp-worktrees/phase2/wave2b-virtual-to-be-split/pkg/virtual/syncer/rest_storage.go \
   pkg/virtual/syncer/rest_storage.go
```

## 📝 Critical Integration Points

### Auth.go Integration
The authorizer must integrate with:
- Virtual workspace context extraction
- KCP workspace permissions
- User/ServiceAccount validation
- Attribute-based access control

**Key Methods:**
```go
func (a *Authorizer) Authorize(ctx context.Context, attrs authorizer.Attributes) (authorizer.Decision, string, error)
func extractWorkspace(ctx context.Context) (logicalcluster.Path, error)
func (a *Authorizer) validateUserAccess(user user.Info, workspace logicalcluster.Path, verb string) bool
```

### RestStorage.go Integration
The storage must handle:
- Virtual to physical transformation (using transformer from Wave2b-03)
- Workspace isolation in all operations
- Proper status subresource updates
- Watch event propagation

**Key Methods:**
```go
func (s *SyncTargetStorage) Create(ctx context.Context, obj runtime.Object, createValidation rest.ValidateObjectFunc, options *metav1.CreateOptions) (runtime.Object, error)
func (s *SyncTargetStorage) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error)
func (s *SyncTargetStorage) List(ctx context.Context, options *metainternalversion.ListOptions) (runtime.Object, error)
func (s *SyncTargetStorage) Watch(ctx context.Context, options *metainternalversion.ListOptions) (watch.Interface, error)
```

## ✅ Validation Steps

1. **File Verification**
   ```bash
   # Verify files copied correctly
   ls -la pkg/virtual/syncer/auth.go
   ls -la pkg/virtual/syncer/rest_storage.go
   
   # Check line counts
   wc -l pkg/virtual/syncer/auth.go      # Should be 185 lines
   wc -l pkg/virtual/syncer/rest_storage.go  # Should be 190 lines
   ```

2. **Compile Check**
   ```bash
   go build ./pkg/virtual/syncer/...
   ```

3. **Import Verification**
   Ensure all imports are satisfied:
   - Authorization interfaces
   - REST storage interfaces
   - KCP workspace types
   - Transformation interfaces (may need stubs until Wave2b-03)

4. **Line Count Verification**
   ```bash
   /workspaces/kcp-shared-tools/tmc-pr-line-counter.sh -c $(git branch --show-current)
   ```
   Target: ~375 lines

## 🔄 Commit Structure

```bash
# Commit 1: Add authentication
git add pkg/virtual/syncer/auth.go
git commit -s -S -m "feat(virtual): add authentication for virtual workspace

- Implement workspace-aware authorization
- Add user access validation
- Integrate with KCP RBAC
- Ensure workspace isolation"

# Commit 2: Add REST storage
git add pkg/virtual/syncer/rest_storage.go
git commit -s -S -m "feat(virtual): add REST storage implementation

- Implement CRUD operations for SyncTarget
- Add workspace-scoped storage
- Handle virtual to physical transformation
- Support watch operations and status updates"
```

## ⚠️ Important Reminders

- **DO NOT** modify the copied files unless fixing imports
- **DO** verify workspace isolation is maintained
- **DO** ensure all methods handle context properly
- **DO** check that transformation interfaces align
- **DO NOT** exceed 375 lines total
- **DO** preserve all comments and documentation

## 🎯 Success Metrics

- [ ] Both files copied successfully
- [ ] Code compiles without errors
- [ ] Auth validates workspace correctly
- [ ] Storage maintains workspace isolation
- [ ] All CRUD operations present
- [ ] Exactly 375 lines of code
- [ ] Ready to integrate with Wave2b-03 transformation