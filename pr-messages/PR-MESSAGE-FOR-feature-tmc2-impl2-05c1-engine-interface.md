## Summary

This PR introduces the foundational PlacementEngine interface and basic round-robin implementation for TMC (Transparent Multi-Cluster). This is the **first split** of an oversized placement engine PR (1,718 lines → 473 lines) to meet KCP PR size requirements.

**Key Components:**
- `PlacementEngine` interface defining cluster placement algorithms
- `RoundRobinEngine` implementation with stateful round-robin cluster selection  
- Minimal TMC API stubs (`ClusterRegistration`, `WorkloadPlacement`) for interface compatibility
- Generated deepcopy methods, CRDs, and scheme registration

**Agent Integration:** This PR unblocks Agent 1's integration work by providing the essential placement interface they depend on. Full API types will be delivered in subsequent PRs.

## What Type of PR Is This?

/kind feature

## Related Issue(s)

Part of TMC Reimplementation Plan 2 - Emergency PR Split

## Release Notes

```yaml
apiVersion: v1
kind: ConfigMap  
metadata:
  name: release-notes
data:
  note: |
    Adds PlacementEngine interface for TMC cluster placement algorithms with 
    round-robin implementation. Includes minimal API stubs to enable controller
    integration while full API types are developed in follow-up PRs.
```