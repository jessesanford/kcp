<!--

Thanks for creating a pull request!
If this is your first time, please make sure to review CONTRIBUTING.MD.

-->

## Summary

This PR implements the TMC (Transparent Multi-Cluster) virtual workspace following KCP patterns, providing the foundation for serving TMC APIs through KCP's virtual workspace framework.

**Key Components:**
- **TMC Virtual Workspace Builder**: Creates virtual workspace with proper path prefix handling for `/services/tmc/clusters/*/apis/tmc.kcp.io/*` URLs
- **URL Parsing and Routing**: Handles wildcard (`*`) and specific cluster requests with proper cluster context injection
- **Authorization Layer**: Implements workspace isolation and TMC-specific permission checks following KCP patterns
- **Configuration Options**: Provides configurable root path prefix and enable/disable controls
- **Comprehensive Testing**: Full test coverage including URL parsing, authorization, and integration tests

**TMC Virtual Workspace Features:**
- Supports both wildcard cluster requests (`/services/tmc/clusters/*/apis/tmc.kcp.io/v1alpha1/*`) and cluster-specific requests
- Implements proper KCP virtual workspace delegation patterns
- Provides workspace isolation and security boundaries
- Follows KCP authorization and readiness check patterns
- Designed for future integration with actual TMC API implementations

This establishes the virtual workspace infrastructure that will enable TMC APIs to be served through KCP's virtual workspace system with proper security, routing, and cluster context handling.

## What Type of PR Is This?

/kind feature

## Related Issue(s)

Part of TMC Reimplementation Plan 2 - Virtual Workspace Architecture (Phase 1)

## Release Notes

```
Add TMC virtual workspace implementation following KCP patterns for serving TMC APIs through virtual workspace framework with proper authorization and cluster routing.
```