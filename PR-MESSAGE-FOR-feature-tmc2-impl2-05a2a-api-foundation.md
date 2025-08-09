<!--

Thanks for creating a pull request!
If this is your first time, please make sure to review CONTRIBUTING.MD.

-->

## Summary

This PR introduces the core TMC (Transparent Multi-Cluster) workload API foundation, establishing the fundamental API types needed for workload placement and cluster management in KCP.

**Key Components Added:**
- **TMC Feature Flag**: New `TMCAPIs` feature gate for controlled rollout
- **Core API Types**: Complete workload API v1alpha1 with 6 resource types:
  - `Location`: Represents physical cluster locations with capacity and constraints
  - `Placement`: Defines workload placement policies and target selection
  - `ResourceExport`: Handles resource discovery and capability advertisement
  - `ResourceImport`: Manages resource consumption and binding
  - `SyncTarget`: Represents target clusters for workload synchronization
  - `SyncTargetHeartbeat`: Provides cluster health and status monitoring

**Technical Details:**
- All API types follow KCP patterns with proper workspace isolation
- Comprehensive validation, defaulting, and status reporting
- Built-in support for conditions-based status management
- Generated deepcopy and defaults code included
- Proper Go module structure for SDK consumption

This establishes the foundation that future PRs will build upon for placement controllers, client SDK generation, and CRD installation.

## What Type of PR Is This?

/kind feature

## Related Issue(s)

Part of TMC Reimplementation Plan 2 - API Foundation Phase

## Release Notes

```markdown
New TMC workload APIs introduced with feature flag protection:
- Location, Placement, ResourceExport, ResourceImport, SyncTarget, SyncTargetHeartbeat
- Full KCP workspace isolation support
- Foundation for TMC placement controller functionality
```