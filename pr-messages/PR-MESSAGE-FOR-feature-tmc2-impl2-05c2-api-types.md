## Summary

This PR provides the **complete TMC API types** replacing the minimal stubs from the first PR. This is the **second split** of an oversized placement engine PR (480 lines) to meet KCP size requirements.

**Key Components:**
- Complete `ClusterRegistration` API with full cluster management capabilities
- Complete `WorkloadPlacement` API with comprehensive placement specifications
- Shared types for conditions, constraints, and selectors
- Generated CRDs, deepcopy methods, and client code

**Dependencies:** Builds on feature/tmc2-impl2/05c1-engine-interface

**Agent Integration:** Provides the full API types needed by the resource-aware placement engine and advanced TMC features.

## What Type of PR Is This?

/kind feature

## Related Issue(s)

Part of TMC Reimplementation Plan 2 - Emergency PR Split (2/4)

## Release Notes

```yaml
apiVersion: v1
kind: ConfigMap  
metadata:
  name: release-notes
data:
  note: |
    Adds complete TMC API types for cluster registration and workload placement,
    replacing minimal stubs. Includes full cluster management capabilities,
    placement specifications, and generated client code.
```