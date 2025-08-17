## Summary

This PR adds evaluation and scheduling interfaces for TMC's cross-workspace placement engine. This is part 2 of 3 splits from the oversized p5w3-placement-interfaces branch (originally 1,430 lines).

- Comprehensive policy evaluator with CEL support
- Advanced scheduler interface with multiple strategies
- Support for workspace-aware scheduling decisions
- Integration with placement scoring system

## What Type of PR Is This?

/kind feature

## Related Issue(s)

Part of TMC Phase 5 API Foundation implementation

## Related PRs

This is split 2 of 3 from the oversized p5w3-placement-interfaces:
- **Previous**: p5w3-placement-core (618 lines) - Core interfaces
- **This PR**: p5w3-placement-eval (577 lines) - Evaluation and scheduling
- **Next**: p5w3-placement-strategy (235 lines) - Strategy patterns

## Dependencies

Depends on: feature/tmc-completion/p5w3-placement-core (should be merged first)

## Release Notes

```release-note
Added placement evaluation and scheduling interfaces with CEL policy support for TMC cross-workspace placement
```