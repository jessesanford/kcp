# Bug Report: Missing Kubernetes Logging Feature Gates

## Summary
The KCP server panics on startup when TMC features are enabled due to missing Kubernetes logging feature gates that were accidentally removed during merge conflict resolution.

## Root Cause Analysis

### The Bug
**Location**: Commit `92688635` - "merge: resolve conflicts in feature flags"
**Date**: August 19, 2025
**Impact**: KCP crashes with panic when starting with TMC features enabled

### Error Message
```
panic: feature "ContextualLogging" is not registered in FeatureGate
```

### What Happened
During the massive 225-branch merge operation, when resolving conflicts in `/pkg/features/kcp_features.go`, the following critical Kubernetes logging feature gates were accidentally removed:

```go
// These were removed incorrectly:
logsapi.LoggingBetaOptions: {
    {Version: version.MustParse("1.26"), Default: true, PreRelease: featuregate.Beta},
},

logsapi.ContextualLogging: {
    {Version: version.MustParse("1.26"), Default: true, PreRelease: featuregate.Alpha},
},
```

### Why It Happened
1. **Upstream KCP** (main branch) has these logging feature gates properly registered
2. **TMC feature-flags branch** (`feature/tmc-impl4/00-feature-flags`) also had them correctly
3. During merge conflict resolution in commit `92688635`, when consolidating TMC features with existing features, these logging gates were accidentally removed
4. The automated merge script used `-X theirs` strategy which may have incorrectly resolved this particular conflict

### The Fix Applied
Added proper registration of Kubernetes logging feature gates in the `init()` function:

```go
func init() {
    utilruntime.Must(utilfeature.DefaultMutableFeatureGate.Add(defaultVersionedGenericControlPlaneFeatureGates))
    // Add Kubernetes logging feature gates that are expected by the logging system
    utilruntime.Must(logsapiv1.AddFeatureGates(utilfeature.DefaultMutableFeatureGate))
}
```

This ensures that:
- `ContextualLogging`
- `LoggingAlphaOptions` 
- `LoggingBetaOptions`

Are all properly registered before KCP attempts to validate and apply logging configuration.

## Affected Branches
The bug does NOT exist in any individual upstream branch. It was introduced during the merge process when combining:
- `feature/tmc-impl4/00-feature-flags` (had logging gates ✓)
- `feature/tmc-impl4/01-base-controller` 
- `feature/tmc-impl4/05-rbac`
- `feature/tmc-impl4/06-auth`
- `feature/tmc-impl4/18-syncer-core`

## Prevention
For future large-scale merges:
1. **Never remove existing feature gates** during conflict resolution unless explicitly intended
2. **Test server startup** after each phase of merging
3. **Preserve all logging-related configurations** from upstream Kubernetes
4. **Use more careful merge strategies** for critical configuration files like feature gates

## Verification
After the fix:
- ✅ KCP starts without panic
- ✅ TMC features can be enabled
- ✅ Logging system initializes correctly
- ✅ All feature gates are properly registered

## Files Modified
- `/workspaces/kcp-worktrees/tmc-full-merge-test/pkg/features/kcp_features.go` - Added logging feature gate registration