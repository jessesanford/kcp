<!--

Thanks for creating a pull request!
If this is your first time, please make sure to review CONTRIBUTING.MD.

-->

## Summary

This PR implements PR3 of the Decision Maker split - the comprehensive placement decision validator component. The validator provides atomic validation functionality for placement decisions with extensive validation logic and comprehensive test coverage.

**Key Features:**
- Complete `DecisionValidator` interface for validating placement decisions
- Structural validation of decision components (IDs, timestamps, scores, workspace selections)
- Resource constraint validation with configurable overcommit protection  
- Policy compliance validation for labels and region restrictions
- Multi-level conflict detection (resource, affinity, policy conflicts)
- Configurable validation timeouts and thresholds
- Comprehensive error handling and structured logging
- Extensive test coverage with 16 test scenarios covering all validation paths

**Architecture:**
- Clean separation from decision making logic
- Interface-based design for extensibility
- Proper context handling for timeouts and cancellation
- Structured conflict reporting with severity levels and resolution suggestions

**Line Count:** 1059 lines (511 implementation + 548 tests) with net +894 lines due to type refactoring

## What Type of PR Is This?

/kind feature

## Related Issue(s)

Part of TMC Reimplementation Plan 2 - Decision Maker component split into smaller, focused PRs.

## Release Notes

```
Added comprehensive placement decision validation component with:
- Structural, resource, and policy compliance validation
- Multi-level conflict detection and reporting
- Configurable validation parameters and timeouts
- Full test coverage for all validation scenarios
```