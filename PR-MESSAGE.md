## Summary

This PR combines all three parts of the APIResource foundation for TMC's Phase 5 API Foundation. This merges all 3 splits from the oversized p5w1-apiresource-types branch (originally 1,170 lines).

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

**Schema Features:**
- Schema intersection algorithm for API compatibility checking
- Comprehensive validation rules for NegotiatedAPIResource objects
- API overlap detection and conflict resolution logic
- Support for multi-version API schema merging

## What Type of PR Is This?

/kind feature

## Related Issue(s)

Part of TMC Phase 5 API Foundation implementation

## Related PRs

This combines all 3 splits from the oversized p5w1-apiresource-types:
- **Merged**: p5w1-apiresource-core (320 lines) - Core API types and registration
- **Merged**: p5w1-apiresource-helpers (269 lines) - Helper methods and status management
- **Merged**: p5w1-apiresource-schema (568 lines) - Schema intersection and validation

Together these 3 PRs replace the oversized p5w1-apiresource-types branch.

## Dependencies

All three parts merged together - complete APIResource foundation

## Release Notes

```release-note
Added complete NegotiatedAPIResource foundation with types, helpers, and schema validation for TMC cross-workspace API discovery and negotiation
```