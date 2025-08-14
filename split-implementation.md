# Split Implementation: Wave2b-01 - Virtual Workspace Foundation

## Overview
**Branch:** `feature/tmc-syncer-02b-virtual-base`  
**Target Size:** 473 lines  
**Dependencies:** Wave1 API Types required  
**Can Run In Parallel:** No - this is the virtual workspace foundation

## Files to Copy

These files should be copied from `/workspaces/kcp-worktrees/phase2/wave2b-virtual-to-be-split/`:

### 1. **pkg/virtual/syncer/doc.go** (29 lines)
Package documentation and overview of the virtual workspace implementation.

### 2. **pkg/virtual/syncer/context.go** (46 lines)
Context management for virtual workspace operations.

### 3. **pkg/virtual/syncer/virtual_workspace.go** (201 lines)
Core virtual workspace implementation including:
- VirtualWorkspace struct
- Registration with KCP
- Resource provider interface
- Workspace initialization

### 4. **pkg/virtual/syncer/discovery.go** (197 lines)
Discovery mechanism for virtual resources:
- Discovery provider implementation
- API resource listing
- Group/version management
- Resource capability advertisement

## Implementation Checklist

### Pre-Implementation
- [ ] Ensure Wave1 API types are available
- [ ] Create branch from main
- [ ] Set up package structure

### Implementation Steps

1. **Create Package Structure**
   ```bash
   mkdir -p pkg/virtual/syncer
   ```

2. **Copy Core Files**
   ```bash
   # Copy documentation
   cp /workspaces/kcp-worktrees/phase2/wave2b-virtual-to-be-split/pkg/virtual/syncer/doc.go \
      pkg/virtual/syncer/doc.go
   
   # Copy context management
   cp /workspaces/kcp-worktrees/phase2/wave2b-virtual-to-be-split/pkg/virtual/syncer/context.go \
      pkg/virtual/syncer/context.go
   
   # Copy virtual workspace core
   cp /workspaces/kcp-worktrees/phase2/wave2b-virtual-to-be-split/pkg/virtual/syncer/virtual_workspace.go \
      pkg/virtual/syncer/virtual_workspace.go
   
   # Copy discovery mechanism
   cp /workspaces/kcp-worktrees/phase2/wave2b-virtual-to-be-split/pkg/virtual/syncer/discovery.go \
      pkg/virtual/syncer/discovery.go
   ```

3. **Verify Package Imports**
   Review each file to ensure imports are correct:
   - Virtual workspace framework imports
   - KCP client imports
   - Discovery interface imports

4. **Test Compilation**
   ```bash
   go build ./pkg/virtual/syncer/...
   ```

5. **Register Virtual Workspace**
   Add registration code to integrate with KCP's virtual workspace system:
   ```go
   // In appropriate initialization code
   virtualWorkspace := syncer.NewVirtualWorkspace(...)
   server.AddVirtualWorkspace(virtualWorkspace)
   ```

### Key Components to Verify

#### Virtual Workspace Structure
```go
type VirtualWorkspace struct {
    name string
    provider ResourceProvider
    discovery DiscoveryProvider
    // ... other fields
}
```

#### Discovery Provider Interface
```go
type DiscoveryProvider interface {
    GroupResources() ([]metav1.APIGroup, []metav1.APIResource)
    ResourceEnabled(resource schema.GroupVersionResource) bool
}
```

#### Context Management
- Workspace context propagation
- Request context enrichment
- Authentication context handling

### Validation Steps

1. **Check Line Count**
   ```bash
   /workspaces/kcp-shared-tools/tmc-pr-line-counter.sh -c feature/tmc-syncer-02b-virtual-base
   ```
   Should be exactly 473 lines

2. **Verify Compilation**
   ```bash
   make build
   ```

3. **Test Virtual Workspace Registration**
   ```bash
   # After building, verify virtual workspace appears
   kubectl ws virtual
   ```

4. **Check Discovery**
   ```bash
   kubectl api-resources --context system:admin
   # Should show virtual resources once registered
   ```

### Commit Strategy

```bash
# Stage all virtual workspace foundation files
git add pkg/virtual/syncer/doc.go
git add pkg/virtual/syncer/context.go
git add pkg/virtual/syncer/virtual_workspace.go
git add pkg/virtual/syncer/discovery.go

# Commit foundation
git commit -s -S -m "feat(virtual): add virtual workspace foundation for syncer

- Implement core virtual workspace structure
- Add discovery provider for resource advertisement
- Set up context management for workspace isolation
- Follow KCP virtual workspace patterns"
```

### Post-Implementation
- [ ] Virtual workspace compiles
- [ ] Discovery mechanism works
- [ ] Context propagation verified
- [ ] Line count exactly 473
- [ ] No compilation errors
- [ ] Push branch and create PR

## Success Criteria

1. ✅ Virtual workspace follows KCP patterns
2. ✅ Discovery provider properly configured
3. ✅ Context management in place
4. ✅ Exactly 473 lines
5. ✅ Compiles successfully
6. ✅ Can be registered with KCP

## Potential Issues & Solutions

1. **Virtual Workspace Registration**
   - Must follow KCP's registration pattern
   - Check existing virtual workspaces for examples

2. **Discovery Issues**
   - Ensure GroupVersion is properly defined
   - Resources must be properly advertised

3. **Context Propagation**
   - Workspace context must flow through requests
   - Authentication must be preserved

## Dependencies

- **Requires:** Wave1 API Types
- **Required By:** Wave2b-02 and Wave2b-03
- **Blocks:** Authentication and transformation implementations

## Notes for Parallel Agents

- This is the foundation - must be completed first
- Wave2b-02 and Wave2b-03 can proceed in parallel after this
- Virtual workspace pattern is critical to get right
- Discovery mechanism enables resource visibility