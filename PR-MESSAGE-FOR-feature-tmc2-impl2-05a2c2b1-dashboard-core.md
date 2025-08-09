<!--

Thanks for creating a pull request!
If this is your first time, please make sure to review CONTRIBUTING.MD.

-->

## Summary

Implements TMC dashboard observability core foundation as part of split PR strategy. This PR provides the fundamental metrics aggregation interfaces, cluster registration controller, and TMC binary foundation required for TMC observability infrastructure.

**Part 1 of 2** - Core aggregation and controller foundation (735 lines). Complemented by dashboard-assets PR for complete TMC observability functionality.

### Key Components Added:
- **TMC Controller Binary**: Bootstrap foundation for TMC controller manager
- **Cluster Registration Controller**: KCP-aware cluster management with workspace isolation
- **Metrics Aggregation Core**: Interfaces and implementation for multi-cluster metric aggregation
- **Feature Flags**: TMC feature gating with @jessesanford user and 0.1 version
- **Workload CRDs**: KCP workload management APIs and schemas

### Technical Implementation:
- Follows KCP architectural patterns with logical cluster awareness
- Implements workspace isolation and security boundaries
- Provides aggregation strategies (sum, avg, max, min) with feature gating
- Includes comprehensive error handling and validation
- Thread-safe implementation with proper synchronization

### Atomic Functionality:
- Core aggregation engine functional standalone
- Cluster registration controller operational
- TMC binary bootstrap ready for extension
- Feature flags properly configured and testable
- All CRDs and schemas properly integrated

## What Type of PR Is This?

/kind feature

## Related Issue(s)

Fixes # (TMC Observability Foundation)

## Release Notes

```
Add TMC observability core foundation with metrics aggregation and cluster management.

This change implements the core TMC observability infrastructure including:
- Metrics aggregation engine with multi-strategy support (sum/avg/max/min)
- Cluster registration controller with KCP workspace isolation
- TMC controller binary foundation
- Feature flags for gradual rollout (@jessesanford, v0.1)
- Workload management CRDs and API schemas

The implementation follows KCP architectural patterns and maintains workspace
security boundaries. All functionality is feature-flagged for controlled deployment.
```