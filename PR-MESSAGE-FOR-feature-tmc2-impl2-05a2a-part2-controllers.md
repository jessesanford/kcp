<!--

Thanks for creating a pull request!
If this is your first time, please make sure to review CONTRIBUTING.MD.

-->

## Summary

This PR implements Part 2 of the TMC 05a2a-api-foundation split, adding workload placement controllers and server integration components. This includes the core controller patterns, reconciliation logic, feature gates, and server lifecycle management.

**Key Components:**
- **Workload Placement Controller**: Full controller implementation with KCP patterns
- **Placement Reconciler**: Business logic for placement decisions
- **TMC Feature Gates**: Comprehensive feature flag system with dependencies  
- **Server Integration**: Controller lifecycle and startup hooks
- **Dynamic REST Mapper**: Support for TMC resource mapping

**Architecture Highlights:**
- Follows KCP controller patterns with proper workspace isolation
- Implements committer pattern for efficient status updates
- Uses typed workqueues for type safety
- Includes comprehensive error handling and observability
- Feature-gated behind @jessesanford/v0.1 alpha flags

## What Type of PR Is This?

/kind feature

## Related Issue(s)

Part of TMC implementation plan. Continues API foundation work from Part 1 (05a2a-part1-api-types).

## Release Notes

```
Add TMC workload placement controllers and server integration

- Workload placement controller with KCP integration patterns
- Placement reconciler with proper resource lifecycle management  
- TMC feature gates with dependency validation
- Server integration for controller startup and lifecycle
- Dynamic REST mapper support for TMC resources

All features are alpha and gated behind feature flags.
```