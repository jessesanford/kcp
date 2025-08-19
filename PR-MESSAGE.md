## Summary

This PR combines the core APIResource types and helper methods for TMC's Phase 5 API Foundation. This merges parts 1-2 of 3 splits from the oversized p5w1-apiresource-types branch (originally 1,170 lines).

**Core Features:**
- Core NegotiatedAPIResource type definitions with proper KCP integration
- API registration and scheme setup
- Package documentation
- Foundation for cross-workspace API discovery and negotiation

**Helper Features:**
- Convenience methods for working with NegotiatedAPIResource objects
- Status management helpers for condition updates
- Group/version/resource parsing utilities
- Methods for checking API compatibility and overlap

## What Type of PR Is This?

/kind feature

## Related Issue(s)

Part of TMC Phase 5 API Foundation implementation

## Related PRs

This combines splits 1-2 of 3 from the oversized p5w1-apiresource-types:
- **Merged**: p5w1-apiresource-core (320 lines) - Core API types and registration
- **Merged**: p5w1-apiresource-helpers (269 lines) - Helper methods and status management
- **Next**: p5w1-apiresource-schema (568 lines) - Schema intersection and validation

Together these 3 PRs replace the oversized p5w1-apiresource-types branch.

## Dependencies

Core and helpers merged together - no external dependencies

## Release Notes

```release-note
Added NegotiatedAPIResource types and helper methods for TMC cross-workspace API discovery and negotiation
```