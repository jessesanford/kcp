<!--

Thanks for creating a pull request!
If this is your first time, please make sure to review CONTRIBUTING.MD.

-->

## Summary

This PR introduces the foundation types and interfaces for the TMC Decision Maker component, providing the core data structures and contracts that all other Decision Maker functionality will build upon.

**Key Features:**
- **DecisionMaker Interface**: Main contract for placement decision coordination
- **Core Types**: PlacementRequest, PlacementDecision, WorkspacePlacement structures  
- **CEL Integration**: Types for CEL expression evaluation and results
- **Override System**: Types for manual placement override functionality
- **Validation Framework**: Interfaces for decision validation and conflict detection
- **Audit Support**: Types for decision recording and historical tracking
- **Comprehensive Tests**: 551 lines of tests covering all type validation

**Architecture Highlights:**
- Full KCP workspace isolation support via `logicalcluster.Name`
- Integration with scheduler API types for seamless coordination
- Extensible design supporting multiple decision algorithms
- Rich metadata for debugging and audit trails

This is **Part 1 of 5** in the Decision Maker component split, focusing solely on types and interfaces to provide a stable foundation for subsequent implementation PRs.

## What Type of PR Is This?

/kind feature

## Related Issue(s)

Part of TMC Phase 8 Decision Maker implementation

## Release Notes

```
Add Decision Maker types and interfaces for TMC placement decisions
```