## Summary

This PR adds core APIResource types and registration for TMC's API negotiation system. This is part 1 of 3 splits from the oversized p5w1-apiresource-types branch (originally 1,170 lines).

- Core NegotiatedAPIResource type definitions with proper KCP integration
- API registration and scheme setup
- Package documentation
- Foundation for cross-workspace API discovery and negotiation

## What Type of PR Is This?

/kind feature

## Related Issue(s)

Part of TMC Phase 5 API Foundation implementation

## Related PRs

This is split 1 of 3 from the oversized p5w1-apiresource-types:
- **This PR**: p5w1-apiresource-core (320 lines) - Core API types and registration
- **Next**: p5w1-apiresource-helpers (269 lines) - Helper methods and status management
- **Next**: p5w1-apiresource-schema (568 lines) - Schema intersection and validation

Together these 3 PRs replace the oversized p5w1-apiresource-types branch.

## Dependencies

No dependencies - this is the foundational API definition

## Release Notes

```release-note
Added NegotiatedAPIResource types for TMC cross-workspace API discovery and negotiation
```