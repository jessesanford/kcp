## Summary

This PR adds helper methods for APIResource management. This is part 2 of 3 splits from the oversized p5w1-apiresource-types branch (originally 1,170 lines).

- Convenience methods for working with NegotiatedAPIResource objects
- Status management helpers for condition updates
- Group/version/resource parsing utilities
- Methods for checking API compatibility and overlap

## What Type of PR Is This?

/kind feature

## Related Issue(s)

Part of TMC Phase 5 API Foundation implementation

## Related PRs

This is split 2 of 3 from the oversized p5w1-apiresource-types:
- **Previous**: p5w1-apiresource-core (320 lines) - Core API types and registration
- **This PR**: p5w1-apiresource-helpers (269 lines) - Helper methods and status management
- **Next**: p5w1-apiresource-schema (568 lines) - Schema intersection and validation

Together these 3 PRs replace the oversized p5w1-apiresource-types branch.

## Dependencies

Depends on: feature/tmc-completion/p5w1-apiresource-core (should be merged first)

## Release Notes

```release-note
Added helper methods for NegotiatedAPIResource management and status updates
```