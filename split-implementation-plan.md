# Split Implementation Plan: p6w3-webhooks Branch

## Executive Summary

**Branch**: feature/tmc-completion/p6w3-webhooks  
**Current Size**: 1,081 lines (281 lines over 800 limit)  
**Problem**: Branch exceeds maximum PR size limit and needs to be split into smaller, atomic branches  
**Solution**: Split into 2 functionally complete branches

## Current Structure Analysis

### File Breakdown
```
pkg/admission/webhooks/
├── synctarget_webhook.go      (353 lines) - SyncTarget admission webhook
├── placement_webhook.go       (442 lines) - WorkloadPlacement admission webhook  
├── webhook_server.go          (286 lines) - HTTP server and handlers
└── synctarget_webhook_test.go (307 lines) - Tests for SyncTarget webhook
```

### Functional Components

1. **SyncTarget Webhook** (353 lines)
   - Mutation logic for labels/annotations
   - Validation logic for SyncTarget resources
   - Helper functions for validation

2. **WorkloadPlacement Webhook** (442 lines)
   - Mutation logic for placement policies
   - Validation logic for placement constraints
   - Helper functions for affinity rules

3. **Webhook Server Infrastructure** (286 lines)
   - HTTP server setup with TLS
   - Request handling and routing
   - Health check endpoints
   - Admission request/response processing

4. **Tests** (307 lines)
   - SyncTarget webhook tests only
   - Missing tests for placement webhook
   - Missing tests for server infrastructure

## Proposed Split Structure

### Split Branch 1: p6w3-webhooks-1-split-from-p6w3-webhooks (~540 lines)
**Focus**: Core webhook infrastructure and SyncTarget webhook

**Files**:
- `pkg/admission/webhooks/webhook_server.go` (286 lines)
- `pkg/admission/webhooks/synctarget_webhook.go` (353 lines)  
- `pkg/admission/webhooks/synctarget_webhook_test.go` (307 lines)
- `pkg/admission/webhooks/webhook_server_test.go` (NEW ~150 lines)

**Total**: ~540 implementation lines + ~457 test lines

**Functionality**:
- Complete webhook server infrastructure
- Full SyncTarget admission webhook
- Health check endpoints
- Comprehensive tests for both components

### Split Branch 2: p6w3-webhooks-2-split-from-p6w3-webhooks (~442 lines)
**Focus**: WorkloadPlacement webhook

**Dependencies**: Requires Split Branch 1 to be merged first

**Files**:
- `pkg/admission/webhooks/placement_webhook.go` (442 lines)
- `pkg/admission/webhooks/placement_webhook_test.go` (NEW ~350 lines)
- `pkg/admission/plugins.go` (UPDATE ~20 lines) - Register placement webhook

**Total**: ~462 implementation lines + ~350 test lines

**Functionality**:
- Complete WorkloadPlacement admission webhook
- Integration with existing webhook server
- Comprehensive tests for placement validation

## Execution Protocol

### Phase 1: Create Split Branch 1
```bash
# Create new branch from main
git checkout main
git pull origin main
git checkout -b feature/tmc-completion/p6w3-webhooks-1-split-from-p6w3-webhooks

# Cherry-pick webhook server and synctarget webhook commits
git cherry-pick <commits-for-server-and-synctarget>

# Add missing server tests
# Create pkg/admission/webhooks/webhook_server_test.go

# Measure and verify size
/workspaces/kcp-shared-tools/tmc-pr-line-counter.sh -c feature/tmc-completion/p6w3-webhooks-1-split-from-p6w3-webhooks

# Push branch
git push origin feature/tmc-completion/p6w3-webhooks-1-split-from-p6w3-webhooks
```

### Phase 2: Create Split Branch 2
```bash
# Create new branch based on Split Branch 1
git checkout feature/tmc-completion/p6w3-webhooks-1-split-from-p6w3-webhooks
git checkout -b feature/tmc-completion/p6w3-webhooks-2-split-from-p6w3-webhooks

# Cherry-pick placement webhook commits
git cherry-pick <commits-for-placement>

# Add placement webhook tests
# Create pkg/admission/webhooks/placement_webhook_test.go

# Update plugin registration
# Edit pkg/admission/plugins.go

# Measure and verify size
/workspaces/kcp-shared-tools/tmc-pr-line-counter.sh -c feature/tmc-completion/p6w3-webhooks-2-split-from-p6w3-webhooks

# Push branch
git push origin feature/tmc-completion/p6w3-webhooks-2-split-from-p6w3-webhooks
```

### Phase 3: Verify and Clean Up
```bash
# Verify both branches build and test pass
make test

# Archive original oversized branch
git tag archive/p6w3-webhooks-original feature/tmc-completion/p6w3-webhooks
git push origin --tags

# Delete original branch after splits are merged
# git branch -D feature/tmc-completion/p6w3-webhooks
```

## Dependencies and Merge Order

1. **First PR**: `p6w3-webhooks-1-split-from-p6w3-webhooks`
   - Independent, can merge directly to main
   - Provides webhook server infrastructure
   - Includes SyncTarget webhook functionality

2. **Second PR**: `p6w3-webhooks-2-split-from-p6w3-webhooks`  
   - Depends on first PR being merged
   - Adds WorkloadPlacement webhook
   - Integrates with existing server

## Risk Mitigation

### Testing Requirements
- Each split branch must have >40% test coverage
- All existing tests must pass
- New tests must be added for missing coverage

### Integration Points
- Webhook registration in `pkg/admission/plugins.go`
- Server configuration in KCP startup
- Certificate management for TLS

### Rollback Plan
- Original branch tagged for reference
- Each split can be reverted independently
- Server infrastructure can run with partial webhooks

## Success Criteria

✅ **Split Branch 1**:
- [ ] Under 600 lines of implementation code
- [ ] All SyncTarget webhook functionality intact
- [ ] Server infrastructure complete
- [ ] Tests pass with >40% coverage
- [ ] Clean commit history

✅ **Split Branch 2**:
- [ ] Under 500 lines of implementation code
- [ ] All WorkloadPlacement webhook functionality intact
- [ ] Integrates cleanly with server from Branch 1
- [ ] Tests pass with >40% coverage
- [ ] Clean commit history

## Review Considerations

### For Split Branch 1 Review
- Focus on server security (TLS configuration)
- Validate SyncTarget mutation logic
- Check error handling in admission flow
- Review health check implementation

### For Split Branch 2 Review
- Focus on placement policy validation
- Validate constraint and affinity logic
- Check integration with server
- Review priority handling

## Timeline Estimate

- **Split Branch 1 Creation**: 2 hours
  - Cherry-pick commits: 30 min
  - Add server tests: 1 hour
  - Verify and measure: 30 min

- **Split Branch 2 Creation**: 2.5 hours
  - Cherry-pick commits: 30 min
  - Add placement tests: 1.5 hours
  - Update registration: 30 min

- **Total Time**: 4.5 hours

## Notes

1. The split maintains functional atomicity - each branch provides complete, usable functionality
2. Test coverage needs improvement in both splits
3. Consider adding integration tests in a follow-up PR
4. Documentation updates can be a separate PR if needed

---

**Prepared by**: KCP Code Reviewer Agent  
**Date**: 2025-08-17  
**Status**: Ready for Execution