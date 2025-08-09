<!--

Thanks for creating a pull request!
If this is your first time, please make sure to review CONTRIBUTING.MD.

-->

## Summary

This PR provides the integration layer for TMC virtual workspace with KCP's server infrastructure, establishing the hooks and configuration needed to register TMC virtual workspace with the KCP system.

**Key Components:**
- **TMC Server Options**: Adds server-level configuration options for enabling and configuring TMC virtual workspace
- **Integration Registration**: Provides registration functions for connecting TMC virtual workspace to KCP server infrastructure
- **Configuration Validation**: Comprehensive validation of TMC virtual workspace configuration parameters
- **Server Integration Tests**: Complete test coverage for server options, validation, and integration points

**Integration Features:**
- Server flags for enabling/disabling TMC virtual workspace (`--enable-tmc-virtual-workspace`)
- Configurable root path prefix (`--tmc-virtual-workspace-prefix`)
- Parameter validation for REST config, clients, and informers
- Integration hooks that will connect to actual TMC virtual workspace implementation from 01k branch
- Follows KCP server option and integration patterns

**Future Integration:**
This PR establishes the integration infrastructure that will connect the TMC virtual workspace implementation (from branch `feature/tmc2-impl2/01k-virtual-workspace`) with KCP's server system once both branches are merged.

The integration layer validates all required dependencies and provides proper error handling while maintaining a clean separation between the virtual workspace implementation and server configuration.

## What Type of PR Is This?

/kind feature

## Related Issue(s)

Part of TMC Reimplementation Plan 2 - Virtual Workspace Integration (Phase 1)
Depends on: #XXX (01k-virtual-workspace PR)

## Release Notes

```
Add TMC virtual workspace integration layer with KCP server infrastructure including configuration options, validation, and registration hooks.
```