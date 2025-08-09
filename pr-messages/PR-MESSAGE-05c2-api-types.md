## Summary

This PR replaces the minimal TMC API stubs with full-featured ClusterRegistration and WorkloadPlacement API types, providing comprehensive cluster management and workload placement capabilities.

**Key Components:**
- Complete `ClusterRegistration` API with status conditions and capabilities
- Full `WorkloadPlacement` API with advanced placement policies
- KCP APIExport integration for workspace-aware APIs
- CRD generation and APIResourceSchema definitions

## What Type of PR Is This?

/kind api-change
/kind feature

## Implementation Details

**ClusterRegistration Enhancements:**
- Added `ClusterCapabilities` for cluster resource advertising
- Implemented comprehensive status conditions (Ready, Available, Healthy)
- Support for location-based cluster organization
- Resource capacity and utilization tracking

**WorkloadPlacement Features:**
- Advanced cluster selection with label selectors
- Location-based placement policies
- Configurable cluster count and distribution strategies
- Placement condition tracking and status reporting

**KCP Integration:**
- APIExport configuration for TMC workspace
- APIResourceSchema definitions for both APIs
- Proper workspace isolation and logical cluster support
- Generated CRDs for standard Kubernetes environments

## Test Plan

- [ ] API validation tests (follow-up PR)
- [ ] KCP APIExport integration tests (follow-up PR)  
- [ ] CRD installation and validation (follow-up PR)
- [ ] Workspace isolation verification (follow-up PR)

## Dependencies

- Based on main branch
- Includes generated client code updates
- Part of TMC API foundation series

## Breaking Changes

Replaces minimal API stubs - not backward compatible with previous stub implementation.

## Notes

- **Line Count**: 1016 lines (45% over target) - includes generated SDK and CRD code
- **Test Coverage**: 0% (tests delivered in separate PR for size management)
- This PR completes the TMC API foundation for cluster management

🤖 Generated with [Claude Code](https://claude.ai/code)

Co-Authored-By: Claude <noreply@anthropic.com>