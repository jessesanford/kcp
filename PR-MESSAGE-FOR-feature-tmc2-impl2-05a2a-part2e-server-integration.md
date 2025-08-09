<!--

Thanks for creating a pull request!
If this is your first time, please make sure to review CONTRIBUTING.MD.

-->

## Summary

This PR implements the TMC placement controller with comprehensive KCP server integration. It provides the core controller infrastructure needed for transparent multi-cluster workload placement, including proper KCP workspace isolation, reconciler patterns, and server lifecycle management.

**Key Components:**
- **Placement Controller**: Full KCP-aware controller with workspace isolation
- **Placement Reconciler**: Advanced reconciliation logic for workload placement decisions  
- **Server Integration**: Proper registration with KCP controller lifecycle
- **REST Mapper Integration**: Dynamic resource discovery and handling

**Architecture Highlights:**
- Follows KCP architectural patterns with logical cluster awareness
- Implements proper error handling and status conditions
- Integrates seamlessly with existing KCP server infrastructure
- Supports dynamic REST mapping for workload resource types

This is part of the TMC implementation split from an oversized PR (4079 lines → 736 lines).

## What Type of PR Is This?

/kind feature

## Related Issue(s)

Part of TMC Reimplementation Plan 2 - Phase 5A2A (Controller Infrastructure)

## Release Notes

```
Add TMC placement controller with KCP server integration for transparent multi-cluster workload placement
```