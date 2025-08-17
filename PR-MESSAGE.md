## Summary

This PR adds schema intersection and validation logic for APIResource management. This is part 3 of 3 splits from the oversized p5w1-apiresource-types branch (originally 1,170 lines).

- Schema intersection algorithm for API compatibility checking
- Comprehensive validation rules for NegotiatedAPIResource objects
- API overlap detection and conflict resolution logic
- Support for multi-version API schema merging

## What Type of PR Is This?

/kind feature

## Related Issue(s)

Part of TMC Phase 5 API Foundation implementation

## Related PRs

This is split 3 of 3 from the oversized p5w1-apiresource-types:
- **Previous**: p5w1-apiresource-core (320 lines) - Core API types and registration
- **Previous**: p5w1-apiresource-helpers (269 lines) - Helper methods and status management
- **This PR**: p5w1-apiresource-schema (568 lines) - Schema intersection and validation

Together these 3 PRs replace the oversized p5w1-apiresource-types branch.

## Dependencies

Depends on:
- feature/tmc-completion/p5w1-apiresource-core (should be merged first)
- feature/tmc-completion/p5w1-apiresource-helpers (should be merged second)

## Release Notes

```release-note
Added schema intersection and validation logic for NegotiatedAPIResource compatibility checking
```